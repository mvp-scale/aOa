# =============================================================================
# aOa - Unified Docker Image
# =============================================================================
# Single image containing all aOa services
#
# Build:   docker build -t aoa .
# Run:     docker run -d -p 8080:8080 \
#            -v $(pwd):/codebase:ro \
#            -v ./repos:/repos:rw \
#            -v ./.aoa:/config:rw \
#            -v ~/.claude:/claude-sessions:ro \
#            -v $HOME:/userhome:ro \
#            -e USER_HOME=$HOME \
#            aoa
#
# Services included:
#   - Gateway (port 8080, exposed)
#   - Index (internal)
#   - Status (internal)
#   - Proxy (internal)
#   - Redis (internal)
#
# Volume mounts:
#   /codebase       - aOa installation directory
#   /repos          - Knowledge repos
#   /config         - Configuration (.aoa folder)
#   /claude-sessions - Claude history
#   /userhome       - User's home (for multi-project access)
# =============================================================================

FROM python:3.11-slim

LABEL maintainer="aOa Team"
LABEL description="5 angles. 1 attack. Fast code search for Claude Code."
LABEL version="1.0.0"

# Install system dependencies (pre-built wheels used - no build-essential needed)
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    jq \
    redis-server \
    supervisor \
    && rm -rf /var/lib/apt/lists/*

# Install Python dependencies (all services)
RUN pip install --no-cache-dir \
    fastapi \
    uvicorn \
    httpx \
    flask \
    watchdog \
    redis \
    pydantic \
    requests \
    tree-sitter \
    tree-sitter-language-pack \
    pyahocorasick \
    anthropic

WORKDIR /app

# Default environment variables (can be overridden at runtime with -e)
ENV AOA_CONTENT_CACHE_MB=500

# Copy all services
COPY services/gateway/gateway.py /app/gateway/
COPY services/index/indexer.py /app/index/
COPY services/index/domains_api.py /app/index/
COPY services/index/matchers.py /app/index/
COPY services/ranking /app/ranking/
COPY services/domains /app/domains/
COPY services/jobs /app/jobs/
COPY services/session /app/session/
COPY services/status/status_service.py /app/status/
COPY services/proxy/git_proxy.py /app/proxy/

# Copy config files (GL-074: unified v2 format)
COPY config/universal-domains.json /app/config/

# Copy CLI (available inside container)
COPY cli/aoa /usr/local/bin/aoa
RUN chmod +x /usr/local/bin/aoa

# Create supervisord config
RUN mkdir -p /etc/supervisor/conf.d /var/log/supervisor

COPY <<'EOF' /etc/supervisor/conf.d/aoa.conf
[supervisord]
nodaemon=true
logfile=/var/log/supervisor/supervisord.log
pidfile=/var/run/supervisord.pid

[program:redis]
command=redis-server --appendonly yes --maxmemory 256mb --maxmemory-policy allkeys-lru --loglevel warning
autostart=true
autorestart=true
stdout_logfile=/var/log/supervisor/redis.log
stdout_logfile_maxbytes=1MB
stderr_logfile=/var/log/supervisor/redis-error.log

[program:index]
command=python /app/index/indexer.py
directory=/app/index
environment=CODEBASE_ROOT="%(ENV_CODEBASE_ROOT)s",REPOS_ROOT="/repos",CONFIG_DIR="/config",INDEXES_DIR="/indexes",CLAUDE_SESSIONS="/claude-sessions",REDIS_URL="redis://localhost:6379/0",PORT="9999",USER_HOME="%(ENV_USER_HOME)s",AOA_CONTENT_CACHE_MB="%(ENV_AOA_CONTENT_CACHE_MB)s"
autostart=true
autorestart=true
stdout_logfile=/var/log/supervisor/index.log
stdout_logfile_maxbytes=1MB
stderr_logfile=/var/log/supervisor/index-error.log

[program:status]
command=python /app/status/status_service.py
directory=/app/status
environment=REDIS_URL="redis://localhost:6379/0",STATUS_PORT="9998",CLAUDE_SESSIONS="/claude-sessions",INDEX_URL="http://localhost:9999"
autostart=true
autorestart=true
stdout_logfile=/var/log/supervisor/status.log
stdout_logfile_maxbytes=1MB
stderr_logfile=/var/log/supervisor/status-error.log

[program:proxy]
command=python /app/proxy/git_proxy.py
directory=/app/proxy
environment=REPOS_ROOT="/repos",WHITELIST_FILE="/config/whitelist.txt",MAX_REPO_SIZE_MB="500",CLONE_TIMEOUT="300",GIT_PROXY_PORT="9997"
autostart=true
autorestart=true
stdout_logfile=/var/log/supervisor/proxy.log
stdout_logfile_maxbytes=1MB
stderr_logfile=/var/log/supervisor/proxy-error.log

[program:gateway]
command=python /app/gateway/gateway.py
directory=/app/gateway
environment=INDEX_URL="http://localhost:9999",STATUS_URL="http://localhost:9998",PROXY_URL="http://localhost:9997"
autostart=true
autorestart=true
stdout_logfile=/var/log/supervisor/gateway.log
stdout_logfile_maxbytes=1MB
stderr_logfile=/var/log/supervisor/gateway-error.log
EOF

# Create data directories (matching docker-compose volumes)
RUN mkdir -p /codebase /repos /config /claude-sessions /indexes /userhome /var/log/aoa

# Fix git "dubious ownership" for mounted volumes
RUN git config --global --add safe.directory '*'

# Expose only the gateway port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/aoa.conf"]
