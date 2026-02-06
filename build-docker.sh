#!/bin/bash
# build-docker.sh - Rebuild aOa Docker images
# Maintains parity between compose and unified modes
# Handles full container recreation with all mounts/env vars

set -e

# Colors
CYAN='\033[96m'
GREEN='\033[92m'
YELLOW='\033[93m'
RED='\033[91m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# Get script directory
AOA_HOME="$(cd "$(dirname "$0")" && pwd)"
AOA_DATA="$AOA_HOME/data"
cd "$AOA_HOME"

# Load configuration
if [ ! -f "$AOA_HOME/.env" ]; then
    echo -e "${RED}Error: .env not found. Run install.sh first.${NC}"
    exit 1
fi
source "$AOA_HOME/.env"

# Derive CLAUDE_SESSIONS (same logic as install.sh)
CLAUDE_SESSIONS=""
if [ -d "${PROJECTS_ROOT}/.claude" ]; then
    CLAUDE_SESSIONS="${PROJECTS_ROOT}/.claude"
elif [ -d "${HOME}/.claude" ]; then
    CLAUDE_SESSIONS="${HOME}/.claude"
fi

INSTANCE_NAME="aoa-${USER}"

echo -e "${CYAN}${BOLD}⚡ aOa Docker Build${NC}"
echo -e "${DIM}───────────────────────────────────────${NC}"
echo ""

# Accept choice as argument or prompt interactively
if [ -n "$1" ]; then
    choice="$1"
    echo "  Building mode: $choice"
else
    echo "  1) Compose mode (multi-container)"
    echo "  2) Unified mode (single container)"
    echo "  3) Both"
    echo ""
    read -p "Which to build? [1/2/3]: " choice
fi

build_compose() {
    echo ""
    echo -e "${DIM}Building compose images...${NC}"
    docker compose build --no-cache
    echo -e "${GREEN}✓ Compose images built${NC}"

    # Recreate containers
    echo -e "${DIM}Recreating compose services...${NC}"
    docker compose -p "$INSTANCE_NAME" down 2>/dev/null || true
    docker compose -p "$INSTANCE_NAME" up -d
    echo -e "${GREEN}✓ Compose services running${NC}"
}

build_unified() {
    echo ""
    echo -e "${DIM}Building unified image...${NC}"
    docker build --no-cache -t aoa .
    echo -e "${GREEN}✓ Unified image built${NC}"

    # Stop and remove old container
    echo -e "${DIM}Removing old container...${NC}"
    docker stop "$INSTANCE_NAME" 2>/dev/null || true
    docker rm "$INSTANCE_NAME" 2>/dev/null || true

    # Recreate container with all mounts and env vars
    echo -e "${DIM}Creating new container...${NC}"

    # Build docker run command (mirrors install.sh exactly)
    local run_cmd="docker run -d --name $INSTANCE_NAME"
    run_cmd+=" -p ${AOA_DOCKER_PORT}:8080"
    run_cmd+=" -v ${AOA_HOME}:/codebase:ro"
    run_cmd+=" -v ${PROJECTS_ROOT}:/userhome:ro"
    run_cmd+=" -v ${AOA_DATA}/repos:/repos:rw"
    run_cmd+=" -v ${AOA_DATA}/indexes:/indexes:rw"
    run_cmd+=" -v ${AOA_DATA}:/config:rw"
    run_cmd+=" -v ${AOA_HOME}/config:/app/config:ro"

    # Add Claude sessions mount if available
    if [ -n "$CLAUDE_SESSIONS" ] && [ -d "$CLAUDE_SESSIONS" ]; then
        run_cmd+=" -v ${CLAUDE_SESSIONS}:/claude-sessions:ro"
        run_cmd+=" -e CLAUDE_SESSIONS=/claude-sessions"
    fi

    run_cmd+=" -e USER_HOME=${PROJECTS_ROOT}"
    run_cmd+=" -e CONFIG_DIR=/config"
    run_cmd+=" -e AOA_CONTENT_CACHE_MB=${AOA_CONTENT_CACHE_MB:-500}"
    run_cmd+=" --restart unless-stopped"
    run_cmd+=" aoa"

    # Execute
    eval "$run_cmd" > /dev/null
    echo -e "${GREEN}✓ Container created${NC}"

    # Wait for health
    echo -n "  Waiting for services"
    for i in {1..10}; do
        if curl -s --connect-timeout 1 "http://localhost:${AOA_DOCKER_PORT}/health" > /dev/null 2>&1; then
            echo ""
            echo -e "${GREEN}✓ Services running on port ${AOA_DOCKER_PORT}${NC}"
            return 0
        fi
        echo -n "."
        sleep 1
    done
    echo ""
    echo -e "${YELLOW}! Services may still be starting${NC}"
}

case "$choice" in
    1) build_compose ;;
    2) build_unified ;;
    3) build_compose; build_unified ;;
    *)
        echo "Invalid choice. Use 1, 2, or 3."
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}${BOLD}Done.${NC}"
