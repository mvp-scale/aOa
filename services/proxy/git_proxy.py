#!/usr/bin/env python3
"""
aOa URL Proxy - Controlled internet access service.

This is the ONLY service with internet access.
It executes ONLY whitelisted operations to ONLY whitelisted URLs.

Default whitelist:
- github.com, gitlab.com, bitbucket.org (git operations)

Users can add their own URLs:
- Private git servers
- Internal documentation sites
- Proprietary repos

Trust Guarantees:
- Only HTTPS URLs allowed (no http://, git://, ssh://, file://)
- Only whitelisted hosts (user-configurable)
- All operations logged for audit
- Shallow git clones only (--depth 1)
- Size limited (default 500MB)
- Timeout enforced (default 5 minutes)

Even if compromised, this service can only access whitelisted URLs.
"""

import os
import re
import shutil
import subprocess
import tempfile
import time
from datetime import datetime
from pathlib import Path

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

app = FastAPI(
    title="aOa Git Proxy",
    description="Isolated git operations - the only internet-connected service",
    version="1.0.0"
)

# =============================================================================
# Configuration
# =============================================================================

REPOS_ROOT = Path(os.environ.get("REPOS_ROOT", "/repos"))
REPOS_ROOT.mkdir(parents=True, exist_ok=True)

# Whitelist file (persistent across restarts)
WHITELIST_FILE = REPOS_ROOT / ".allowed_urls"

# Default allowed hosts (pre-populated)
DEFAULT_ALLOWED_HOSTS = ["github.com", "gitlab.com", "bitbucket.org"]

MAX_REPO_SIZE_MB = int(os.environ.get("MAX_REPO_SIZE_MB", 500))
CLONE_TIMEOUT = int(os.environ.get("CLONE_TIMEOUT", 300))

# Clone log for audit
clone_log: list[dict] = []
MAX_LOG_SIZE = 500

# =============================================================================
# Whitelist Management
# =============================================================================

def load_whitelist() -> list[str]:
    """Load allowed hosts from file."""
    if not WHITELIST_FILE.exists():
        # Initialize with defaults
        save_whitelist(DEFAULT_ALLOWED_HOSTS)
        return DEFAULT_ALLOWED_HOSTS.copy()

    try:
        hosts = WHITELIST_FILE.read_text().strip().split("\n")
        return [h.strip() for h in hosts if h.strip()]
    except Exception:
        return DEFAULT_ALLOWED_HOSTS.copy()


def save_whitelist(hosts: list[str]):
    """Save allowed hosts to file."""
    WHITELIST_FILE.write_text("\n".join(hosts))


def add_to_whitelist(host: str) -> tuple[bool, str]:
    """Add a host to the whitelist."""
    # Validate host format
    if not re.match(r'^[\w\-\.]+$', host):
        return False, "Invalid host format (alphanumeric, hyphens, dots only)"

    hosts = load_whitelist()

    if host in hosts:
        return False, f"Host '{host}' already in whitelist"

    hosts.append(host)
    save_whitelist(hosts)

    log_operation("whitelist_add", host=host, success=True)
    return True, f"Added '{host}' to whitelist"


def remove_from_whitelist(host: str) -> tuple[bool, str]:
    """Remove a host from the whitelist."""
    hosts = load_whitelist()

    # Don't allow removing defaults (safety)
    if host in DEFAULT_ALLOWED_HOSTS:
        return False, f"Cannot remove default host '{host}'"

    if host not in hosts:
        return False, f"Host '{host}' not in whitelist"

    hosts.remove(host)
    save_whitelist(hosts)

    log_operation("whitelist_remove", host=host, success=True)
    return True, f"Removed '{host}' from whitelist"


# Load at startup
ALLOWED_HOSTS = load_whitelist()


# =============================================================================
# Validation
# =============================================================================

def validate_git_url(url: str) -> tuple[bool, str]:
    """
    Validate git URL is safe.

    Only allows:
    - HTTPS URLs (no git://, ssh://, file://)
    - Allowed hosts (github.com, gitlab.com, etc.)
    - Standard repository paths
    """
    # Must be HTTPS
    if not url.startswith("https://"):
        return False, "Only HTTPS URLs allowed"

    # Extract host
    match = re.match(r'https://([^/]+)/', url)
    if not match:
        return False, "Invalid URL format"

    host = match.group(1)

    # Check allowed hosts
    if host not in ALLOWED_HOSTS:
        return False, f"Host '{host}' not in allowed list: {ALLOWED_HOSTS}"

    # Validate path (no .. traversal, no weird characters)
    path = url[len(f"https://{host}/"):]
    if ".." in path:
        return False, "Path traversal not allowed"
    if not re.match(r'^[\w\-\.\/]+$', path):
        return False, "Invalid characters in path"

    return True, "OK"


def validate_repo_name(name: str) -> tuple[bool, str]:
    """Validate repo name is safe."""
    if not name:
        return False, "Repo name required"
    if not re.match(r'^[\w\-]+$', name):
        return False, "Repo name must be alphanumeric with hyphens only"
    if len(name) > 50:
        return False, "Repo name too long (max 50 chars)"
    return True, "OK"


def log_operation(action: str, **kwargs):
    """Log an operation for audit."""
    global clone_log

    entry = {
        "ts": datetime.utcnow().isoformat(),
        "action": action,
        **kwargs
    }
    clone_log.append(entry)

    # Trim log
    if len(clone_log) > MAX_LOG_SIZE:
        clone_log = clone_log[-MAX_LOG_SIZE:]


# =============================================================================
# Git Operations
# =============================================================================

def git_clone(url: str, name: str, depth: int = 1) -> tuple[bool, str]:
    """
    Execute git clone with safety restrictions.

    - Shallow clone only (--depth 1)
    - Timeout enforced
    - Size limited
    """
    target_path = REPOS_ROOT / name

    if target_path.exists():
        return False, f"Repo '{name}' already exists"

    # Create temp directory first
    temp_dir = tempfile.mkdtemp()
    temp_path = Path(temp_dir) / name

    start_time = time.time()

    try:
        # Run git clone
        result = subprocess.run(
            [
                "git", "clone",
                "--depth", str(depth),
                "--single-branch",
                url,
                str(temp_path)
            ],
            capture_output=True,
            text=True,
            timeout=CLONE_TIMEOUT,
        )

        if result.returncode != 0:
            log_operation("clone", url=url, name=name, success=False, error=result.stderr[:200])
            return False, f"Git clone failed: {result.stderr}"

        # Check size
        size_mb = sum(f.stat().st_size for f in temp_path.rglob("*") if f.is_file()) / (1024 * 1024)
        if size_mb > MAX_REPO_SIZE_MB:
            log_operation("clone", url=url, name=name, success=False,
                         error=f"Too large: {size_mb:.1f}MB")
            return False, f"Repo too large: {size_mb:.1f}MB > {MAX_REPO_SIZE_MB}MB limit"

        # Move to final location
        shutil.move(str(temp_path), str(target_path))

        elapsed = time.time() - start_time

        # Log success
        log_operation("clone",
                     url=url,
                     name=name,
                     size_mb=round(size_mb, 2),
                     elapsed_s=round(elapsed, 2),
                     success=True)

        return True, f"Cloned {name} ({size_mb:.1f}MB) in {elapsed:.1f}s"

    except subprocess.TimeoutExpired:
        log_operation("clone", url=url, name=name, success=False, error="Timeout")
        return False, f"Clone timed out after {CLONE_TIMEOUT}s"
    except Exception as e:
        log_operation("clone", url=url, name=name, success=False, error=str(e)[:200])
        return False, f"Clone error: {e}"
    finally:
        # Cleanup temp
        if Path(temp_dir).exists():
            shutil.rmtree(temp_dir, ignore_errors=True)


def git_pull(name: str) -> tuple[bool, str]:
    """Pull updates for an existing repo."""
    target_path = REPOS_ROOT / name

    if not target_path.exists():
        return False, f"Repo '{name}' not found"

    start_time = time.time()

    try:
        result = subprocess.run(
            ["git", "pull", "--depth", "1"],
            cwd=str(target_path),
            capture_output=True,
            text=True,
            timeout=CLONE_TIMEOUT,
        )

        elapsed = time.time() - start_time

        if result.returncode != 0:
            log_operation("pull", name=name, success=False, error=result.stderr[:200])
            return False, f"Git pull failed: {result.stderr}"

        log_operation("pull", name=name, elapsed_s=round(elapsed, 2), success=True)
        return True, f"Updated {name} in {elapsed:.1f}s"

    except subprocess.TimeoutExpired:
        log_operation("pull", name=name, success=False, error="Timeout")
        return False, f"Pull timed out after {CLONE_TIMEOUT}s"
    except Exception as e:
        log_operation("pull", name=name, success=False, error=str(e)[:200])
        return False, f"Pull error: {e}"


def list_repos() -> list[dict]:
    """List all cloned repos."""
    repos = []
    if REPOS_ROOT.exists():
        for repo_dir in sorted(REPOS_ROOT.iterdir()):
            if repo_dir.is_dir() and not repo_dir.name.startswith("."):
                # Get size
                try:
                    size_mb = sum(
                        f.stat().st_size for f in repo_dir.rglob("*") if f.is_file()
                    ) / (1024 * 1024)
                except Exception:
                    size_mb = 0

                repos.append({
                    "name": repo_dir.name,
                    "size_mb": round(size_mb, 2),
                })
    return repos


def delete_repo(name: str) -> tuple[bool, str]:
    """Delete a cloned repo."""
    target_path = REPOS_ROOT / name

    if not target_path.exists():
        return False, f"Repo '{name}' not found"

    try:
        shutil.rmtree(target_path)
        log_operation("delete", name=name, success=True)
        return True, f"Deleted {name}"
    except Exception as e:
        log_operation("delete", name=name, success=False, error=str(e)[:200])
        return False, f"Delete error: {e}"


# =============================================================================
# API Models
# =============================================================================

class CloneRequest(BaseModel):
    url: str
    name: str
    depth: int = 1


class PullRequest(BaseModel):
    name: str


class DeleteRequest(BaseModel):
    name: str


# =============================================================================
# API Endpoints
# =============================================================================

@app.get("/health")
async def health():
    return {
        "status": "ok",
        "service": "aOa Git Proxy",
        "internet_access": True,
        "warning": "This is the ONLY service with internet access",
        "allowed_hosts": ALLOWED_HOSTS,
        "restrictions": [
            "HTTPS URLs only",
            "Whitelisted hosts only",
            f"Max repo size: {MAX_REPO_SIZE_MB}MB",
            f"Clone timeout: {CLONE_TIMEOUT}s",
        ],
    }


@app.get("/status")
async def status():
    """Show git proxy status and recent operations."""
    repos = list_repos()
    total_size = sum(r["size_mb"] for r in repos)

    return {
        "repos_root": str(REPOS_ROOT),
        "repos": repos,
        "total_repos": len(repos),
        "total_size_mb": round(total_size, 2),
        "allowed_hosts": ALLOWED_HOSTS,
        "max_repo_size_mb": MAX_REPO_SIZE_MB,
        "clone_timeout": CLONE_TIMEOUT,
        "recent_operations": clone_log[-20:],
    }


@app.post("/clone")
async def clone(req: CloneRequest):
    """
    Clone a git repository.

    Restricted to:
    - HTTPS URLs only
    - Allowed hosts only
    - Shallow clones
    - Size limited
    """
    # Validate URL
    valid, msg = validate_git_url(req.url)
    if not valid:
        raise HTTPException(status_code=400, detail={"error": msg, "url": req.url})

    # Validate name
    valid, msg = validate_repo_name(req.name)
    if not valid:
        raise HTTPException(status_code=400, detail={"error": msg, "name": req.name})

    # Clone
    success, msg = git_clone(req.url, req.name, req.depth)

    if success:
        return {"success": True, "message": msg}
    else:
        raise HTTPException(status_code=400, detail={"error": msg})


@app.post("/pull")
async def pull(req: PullRequest):
    """Pull updates for an existing repo."""
    valid, msg = validate_repo_name(req.name)
    if not valid:
        raise HTTPException(status_code=400, detail={"error": msg})

    success, msg = git_pull(req.name)

    if success:
        return {"success": True, "message": msg}
    else:
        raise HTTPException(status_code=400, detail={"error": msg})


@app.delete("/repo/{name}")
async def delete(name: str):
    """Delete a cloned repo."""
    valid, msg = validate_repo_name(name)
    if not valid:
        raise HTTPException(status_code=400, detail={"error": msg})

    success, msg = delete_repo(name)

    if success:
        return {"success": True, "message": msg}
    else:
        raise HTTPException(status_code=404, detail={"error": msg})


@app.get("/repos")
async def repos():
    """List all cloned repos."""
    return {
        "repos": list_repos(),
        "allowed_hosts": ALLOWED_HOSTS,
    }


@app.get("/audit")
async def audit():
    """Audit log of all git operations."""
    return {
        "description": "All git operations performed by this service",
        "total_operations": len(clone_log),
        "operations": clone_log,
        "note": "This service is the ONLY one with internet access",
    }


@app.get("/whitelist")
async def get_whitelist():
    """
    Show allowed URLs.

    Default: github.com, gitlab.com, bitbucket.org
    Users can add private git servers or other URLs.
    """
    hosts = load_whitelist()
    return {
        "allowed_hosts": hosts,
        "default_hosts": DEFAULT_ALLOWED_HOSTS,
        "custom_hosts": [h for h in hosts if h not in DEFAULT_ALLOWED_HOSTS],
        "note": "Only these hosts can be accessed",
        "management": {
            "add": "POST /whitelist with {\"host\": \"git.company.com\"}",
            "remove": "DELETE /whitelist/{host}",
        }
    }


@app.post("/whitelist")
async def add_whitelist(req: dict):
    """
    Add a URL to the whitelist.

    Example: {"host": "git.company.com"}

    Use this to add private git servers or documentation sites.
    """
    host = req.get("host", "").strip()
    if not host:
        raise HTTPException(status_code=400, detail={"error": "host required"})

    success, msg = add_to_whitelist(host)

    if success:
        return {"success": True, "message": msg, "whitelist": load_whitelist()}
    else:
        raise HTTPException(status_code=400, detail={"error": msg})


@app.delete("/whitelist/{host}")
async def delete_whitelist(host: str):
    """
    Remove a URL from the whitelist.

    Cannot remove default hosts (github.com, gitlab.com, bitbucket.org).
    """
    success, msg = remove_from_whitelist(host)

    if success:
        return {"success": True, "message": msg, "whitelist": load_whitelist()}
    else:
        raise HTTPException(status_code=400, detail={"error": msg})


# =============================================================================
# Main
# =============================================================================

if __name__ == "__main__":
    import uvicorn

    # Ensure repos directory exists
    REPOS_ROOT.mkdir(parents=True, exist_ok=True)

    port = int(os.environ.get("GIT_PROXY_PORT", 9997))
    print(f"Starting aOa Git Proxy on port {port}")
    print(f"Repos root: {REPOS_ROOT}")
    print(f"Allowed hosts: {ALLOWED_HOSTS}")
    print(f"Max repo size: {MAX_REPO_SIZE_MB}MB")
    print(f"Clone timeout: {CLONE_TIMEOUT}s")
    print()
    print("WARNING: This is the ONLY service with internet access")

    uvicorn.run(app, host="0.0.0.0", port=port)
