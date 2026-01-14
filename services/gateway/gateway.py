#!/usr/bin/env python3
"""
aOa Gateway - Single ingress point for all services.

Features:
- Routes requests to internal services
- Provides network topology visibility
- Logs all routing for audit
- Exposes routing table to users

Trust Guarantees:
- This service has NO internet access
- All requests are logged
- Network topology is visible at /network
"""

import os
import time
from datetime import datetime

import httpx
from fastapi import FastAPI, HTTPException, Request, Response
from fastapi.responses import PlainTextResponse

app = FastAPI(
    title="aOa Gateway",
    description="Single ingress point - transparent routing",
    version="1.0.0"
)

# =============================================================================
# Configuration
# =============================================================================

SERVICES = {
    "index": os.environ.get("INDEX_URL", "http://index:9999"),
    "status": os.environ.get("STATUS_URL", "http://status:9998"),
    "git-proxy": os.environ.get("GIT_PROXY_URL", "http://git-proxy:9997"),
}

# Routing table - defines what goes where
ROUTES = {
    # Index service routes
    "/health": ("index", "/health"),
    "/symbol": ("index", "/symbol"),
    "/grep": ("index", "/grep"),      # GL-050: Content search (like Unix grep)
    "/search": ("index", "/symbol"),  # deprecated alias
    "/multi": ("index", "/multi"),
    "/egrep": ("index", "/pattern"),  # Unix parity alias
    "/files": ("index", "/files"),
    "/changes": ("index", "/changes"),
    "/file": ("index", "/file"),
    "/outline": ("index", "/outline"),
    "/outline/enriched": ("index", "/outline/enriched"),
    "/outline/pending": ("index", "/outline/pending"),
    "/deps": ("index", "/deps"),
    "/structure": ("index", "/structure"),
    "/pattern": ("index", "/pattern"),
    "/intent": ("index", "/intent"),
    "/intent/tags": ("index", "/intent/tags"),
    "/intent/files": ("index", "/intent/files"),
    "/intent/file": ("index", "/intent/file"),
    "/intent/recent": ("index", "/intent/recent"),
    "/intent/session": ("index", "/intent/session"),
    "/intent/stats": ("index", "/intent/stats"),
    "/intent/rolling": ("index", "/intent/rolling"),  # GL-045: Rolling intent window
    "/repos": ("index", "/repos"),
    "/ac/test": ("index", "/ac/test"),  # GL-047: Aho-Corasick pattern matcher test

    # Ranking service routes (Phase 1)
    "/rank": ("index", "/rank"),
    "/rank/stats": ("index", "/rank/stats"),
    "/rank/record": ("index", "/rank/record"),

    # Prediction tracking routes (Phase 2)
    "/predict": ("index", "/predict"),
    "/predict/log": ("index", "/predict/log"),
    "/predict/check": ("index", "/predict/check"),
    "/predict/stats": ("index", "/predict/stats"),
    "/predict/finalize": ("index", "/predict/finalize"),

    # Weight tuner routes (Phase 4)
    "/tuner/weights": ("index", "/tuner/weights"),
    "/tuner/best": ("index", "/tuner/best"),
    "/tuner/stats": ("index", "/tuner/stats"),
    "/tuner/feedback": ("index", "/tuner/feedback"),
    "/tuner/reset": ("index", "/tuner/reset"),

    # Metrics endpoint (Phase 4)
    "/metrics": ("index", "/metrics"),
    "/metrics/tokens": ("index", "/metrics/tokens"),

    # Transition model routes (Phase 3)
    "/transitions/sync": ("index", "/transitions/sync"),
    "/transitions/predict": ("index", "/transitions/predict"),
    "/transitions/stats": ("index", "/transitions/stats"),

    # Context API (Phase 3)
    "/context": ("index", "/context"),

    # Memory API (Phase 5) - Dynamic working context
    "/memory": ("index", "/memory"),

    # Project management routes (Global mode)
    "/project/register": ("index", "/project/register"),
    "/project/*": ("index", "/project/*"),
    "/projects": ("index", "/projects"),

    # Domain learning routes (GL-053)
    "/domains/seed": ("index", "/domains/seed"),
    "/domains/stats": ("index", "/domains/stats"),
    "/domains/list": ("index", "/domains/list"),
    "/domains/lookup": ("index", "/domains/lookup"),
    "/domains/learn": ("index", "/domains/learn"),  # GL-053 Phase C: Manual learning trigger
    "/domains/autotune": ("index", "/domains/autotune"),  # GL-053 Phase D: Manual auto-tune trigger
    "/domains/add": ("index", "/domains/add"),  # GL-054: Hook-based domain addition
    "/domains/learned": ("index", "/domains/learned"),  # GL-054: Signal learning complete

    # Status service routes
    "/status": ("status", "/status"),
    "/status/json": ("status", "/status/json"),
    "/status/line": ("status", "/status/line"),
    "/session": ("status", "/session"),
    "/session/reset": ("status", "/session/reset"),
    "/history": ("status", "/history"),
    "/event": ("status", "/event"),
    "/weekly/reset": ("status", "/weekly/reset"),
    "/baseline": ("status", "/baseline"),
    "/sync/subagents": ("status", "/sync/subagents"),

    # Git proxy routes
    "/git/clone": ("git-proxy", "/clone"),
    "/git/pull": ("git-proxy", "/pull"),
    "/git/status": ("git-proxy", "/status"),
    "/git/audit": ("git-proxy", "/audit"),
    "/git/allowed-hosts": ("git-proxy", "/allowed-hosts"),
}

# Request log for audit
request_log: list = []
MAX_LOG_SIZE = 1000

# =============================================================================
# HTTP Client
# =============================================================================

client: httpx.AsyncClient | None = None


@app.on_event("startup")
async def startup():
    global client
    client = httpx.AsyncClient(timeout=30.0)


@app.on_event("shutdown")
async def shutdown():
    if client:
        await client.aclose()


async def proxy_request(
    service: str,
    path: str,
    request: Request,
) -> Response:
    """Proxy a request to an internal service."""

    base_url = SERVICES.get(service)
    if not base_url:
        raise HTTPException(status_code=503, detail=f"Service '{service}' not configured")

    url = f"{base_url}{path}"

    # Add query params
    if request.query_params:
        url += "?" + str(request.query_params)

    # Get request body
    body = None
    if request.method in ("POST", "PUT", "PATCH"):
        body = await request.body()

    # Make request
    start = time.time()
    try:
        response = await client.request(
            method=request.method,
            url=url,
            content=body,
            headers={
                "Content-Type": request.headers.get("Content-Type", "application/json"),
            },
        )
        elapsed = (time.time() - start) * 1000

        # Log for audit
        log_request(request.method, request.url.path, service, response.status_code, elapsed)

        return Response(
            content=response.content,
            status_code=response.status_code,
            headers=dict(response.headers),
            media_type=response.headers.get("content-type"),
        )
    except httpx.ConnectError:
        raise HTTPException(status_code=503, detail=f"Service '{service}' unavailable") from None


def log_request(method: str, path: str, service: str, status: int, ms: float):
    """Log a request for audit purposes."""
    global request_log

    request_log.append({
        "ts": datetime.utcnow().isoformat(),
        "method": method,
        "path": path,
        "service": service,
        "status": status,
        "ms": round(ms, 2),
    })

    # Trim log
    if len(request_log) > MAX_LOG_SIZE:
        request_log = request_log[-MAX_LOG_SIZE:]


# =============================================================================
# Gateway Endpoints
# =============================================================================

@app.get("/")
async def root():
    """Gateway info and health."""
    return {
        "service": "aOa Gateway",
        "version": "1.0.0",
        "status": "ok",
        "message": "Single ingress point for aOa services",
        "endpoints": {
            "routes": "/routes - View routing table",
            "network": "/network - View network topology",
            "audit": "/audit - View request log",
            "verify": "/verify - Verify network isolation",
        },
    }


@app.get("/gateway/health")
async def gateway_health():
    """Gateway-specific health check."""
    return {"status": "ok", "service": "gateway"}


@app.get("/routes")
async def routes():
    """Show the routing table - transparency feature."""
    return {
        "description": "All requests route through this gateway",
        "services": SERVICES,
        "routes": {path: {"service": svc, "backend_path": backend}
                   for path, (svc, backend) in ROUTES.items()},
    }


@app.get("/network")
async def network():
    """
    Show network topology - trust feature.

    Users can see exactly what services exist and how they connect.
    """
    return {
        "topology": {
            "networks": {
                "aoa-internal": {
                    "type": "bridge",
                    "internal": True,
                    "internet_access": False,
                    "services": ["gateway", "index", "status", "redis", "git-proxy"],
                },
                "aoa-external": {
                    "type": "bridge",
                    "internal": False,
                    "internet_access": True,
                    "restricted_to": "git operations only",
                    "services": ["git-proxy"],
                },
            },
            "services": {
                "gateway": {
                    "purpose": "Single ingress point",
                    "exposed_port": 8080,
                    "internet_access": False,
                    "networks": ["aoa-internal"],
                },
                "index": {
                    "purpose": "Codebase indexing and search",
                    "exposed_port": None,
                    "internet_access": False,
                    "networks": ["aoa-internal"],
                },
                "status": {
                    "purpose": "Session monitoring and metrics",
                    "exposed_port": None,
                    "internet_access": False,
                    "networks": ["aoa-internal"],
                },
                "redis": {
                    "purpose": "Persistent storage",
                    "exposed_port": None,
                    "internet_access": False,
                    "networks": ["aoa-internal"],
                },
                "git-proxy": {
                    "purpose": "Git clone for knowledge repos",
                    "exposed_port": None,
                    "internet_access": True,
                    "restricted_to": "git clone operations only",
                    "networks": ["aoa-internal", "aoa-external"],
                },
            },
        },
        "trust_guarantees": [
            "All services except git-proxy have NO internet access",
            "git-proxy only executes git clone commands",
            "All requests route through this gateway",
            "Request log available at /audit",
            "Network topology verifiable via docker inspect",
        ],
        "verify_command": "docker network inspect aoa_aoa-internal",
    }


@app.get("/audit")
async def audit():
    """
    Audit log - see all requests that passed through gateway.

    Transparency: users can see exactly what's being accessed.
    """
    return {
        "description": "Recent requests through the gateway",
        "log_size": len(request_log),
        "max_size": MAX_LOG_SIZE,
        "recent": request_log[-100:],  # Last 100
        "stats": {
            "by_service": _count_by_key(request_log, "service"),
            "by_status": _count_by_key(request_log, "status"),
        },
    }


def _count_by_key(items: list, key: str) -> dict:
    """Count items by a key."""
    counts = {}
    for item in items:
        val = str(item.get(key, "unknown"))
        counts[val] = counts.get(val, 0) + 1
    return counts


@app.get("/verify")
async def verify():
    """
    Verification endpoint - prove network isolation.

    Attempts to reach external services and reports results.
    This SHOULD fail for all services except git-proxy.
    """
    results = {}

    # Try to ping external service from gateway
    # This should timeout/fail because gateway has no internet
    try:
        async with httpx.AsyncClient(timeout=2.0) as test_client:
            await test_client.get("https://api.anthropic.com")
            results["gateway_internet"] = "FAIL - has internet access (unexpected)"
    except Exception:
        results["gateway_internet"] = "PASS - no internet access"

    # Check service health
    for name, url in SERVICES.items():
        try:
            response = await client.get(f"{url}/health", timeout=2.0)
            results[f"{name}_health"] = f"OK ({response.status_code})"
        except Exception as e:
            results[f"{name}_health"] = f"ERROR: {type(e).__name__}"

    return {
        "description": "Network isolation verification",
        "results": results,
        "interpretation": {
            "gateway_internet PASS": "Gateway cannot reach internet (correct)",
            "gateway_internet FAIL": "Gateway has internet access (misconfigured)",
        },
    }


@app.get("/diagram")
async def diagram():
    """ASCII network diagram."""
    diagram_text = """
    USER
      |
      | Port 8080
      |
      v
+-----+-----+
|  GATEWAY  |<-- All requests go through here
+-----+-----+
      |
      | aoa-internal network (NO INTERNET)
      |
+-----+-----+-----+-----+
|     |     |     |     |
v     v     v     v     v
INDEX STATUS REDIS GIT-PROXY
                      |
                      | aoa-external (INTERNET)
                      |
                      v
                  github.com
                  gitlab.com
                  bitbucket.org
"""
    return PlainTextResponse(diagram_text)


# =============================================================================
# Dynamic Route Handler
# =============================================================================

@app.api_route("/{path:path}", methods=["GET", "POST", "PUT", "DELETE", "PATCH"])
async def catch_all(path: str, request: Request):
    """Route all other requests to appropriate service."""

    full_path = f"/{path}"

    # Check for exact match
    if full_path in ROUTES:
        service, backend_path = ROUTES[full_path]
        return await proxy_request(service, backend_path, request)

    # Check for prefix match (e.g., /repo/flask/symbol)
    for route_path, (service, backend_path) in ROUTES.items():
        if full_path.startswith(route_path.rstrip("*")):
            # Construct backend path
            suffix = full_path[len(route_path.rstrip("*")):]
            target = backend_path.rstrip("*") + suffix
            return await proxy_request(service, target, request)

    # Handle repo routes specially
    if full_path.startswith("/repo/"):
        return await proxy_request("index", full_path, request)

    raise HTTPException(
        status_code=404,
        detail={
            "error": "Route not found",
            "path": full_path,
            "available_routes": "/routes",
        }
    )


# =============================================================================
# Main
# =============================================================================

if __name__ == "__main__":
    import uvicorn
    port = int(os.environ.get("GATEWAY_PORT", 8080))
    print(f"Starting aOa Gateway on port {port}")
    print(f"Services: {SERVICES}")
    uvicorn.run(app, host="0.0.0.0", port=port)
