#!/bin/bash
# =============================================================================
# tests/lib/helpers.sh - Shared test infrastructure for aOa test suites
# =============================================================================
#
# Provides:
#   - Docker mode auto-detection (monolithic vs compose)
#   - Redis CLI wrapper
#   - API request helpers with project_id injection
#   - JSON assertion helpers
#   - pass/fail/skip/section display
#   - Preflight checks
#
# Source this from test scripts:
#   source "$SCRIPT_DIR/lib/helpers.sh"
#
# =============================================================================

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
DIM='\033[2m'
NC='\033[0m'
BOLD='\033[1m'

# Counters
PASSED=0
FAILED=0
SKIPPED=0

# Configuration - read URL from home.json, fall back to env or default
_helpers_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_project_root="$(dirname "$(dirname "$_helpers_dir")")"
_home_url=$(jq -r '.aoa_url // empty' "$_project_root/.aoa/home.json" 2>/dev/null)
API_URL="${AOA_URL:-${_home_url:-http://localhost:8080}}"
PROJECT_ID=""
DOCKER_MODE="unknown"
AOA_CONTAINER=""
REDIS_CONTAINER=""

# =============================================================================
# Display helpers
# =============================================================================

pass() {
    echo -e "  ${GREEN}✓${NC} $1"
    PASSED=$((PASSED + 1))
}

fail() {
    echo -e "  ${RED}✗${NC} $1"
    if [ -n "${2:-}" ]; then
        echo -e "    ${DIM}$2${NC}"
    fi
    FAILED=$((FAILED + 1))
}

skip() {
    echo -e "  ${YELLOW}○${NC} $1 ${DIM}(skipped: ${2:-})${NC}"
    SKIPPED=$((SKIPPED + 1))
}

section() {
    echo ""
    echo -e "${CYAN}${BOLD}=== $1 ===${NC}"
}

# =============================================================================
# Docker mode detection
# =============================================================================

detect_docker_mode() {
    DOCKER_MODE="unknown"
    AOA_CONTAINER=""
    REDIS_CONTAINER=""

    # Check for monolithic container: pattern aoa-{username} (no hyphens in suffix)
    local mono
    mono=$(docker ps --format '{{.Names}}' 2>/dev/null | grep -E '^aoa-[a-zA-Z0-9]+$' | head -1)
    if [ -n "$mono" ]; then
        DOCKER_MODE="monolithic"
        AOA_CONTAINER="$mono"
        REDIS_CONTAINER="$mono"
        return 0
    fi

    # Check for compose containers: aoa-redis-1, aoa-gateway-1, etc.
    local compose_redis
    compose_redis=$(docker ps --format '{{.Names}}' 2>/dev/null | grep -E 'aoa.*redis' | head -1)
    if [ -n "$compose_redis" ]; then
        DOCKER_MODE="compose"
        REDIS_CONTAINER="$compose_redis"
        AOA_CONTAINER=$(docker ps --format '{{.Names}}' 2>/dev/null | grep -E 'aoa.*gateway' | head -1)
        return 0
    fi

    return 1
}

# =============================================================================
# Redis CLI wrapper
# =============================================================================

redis_cli() {
    if [ -z "$REDIS_CONTAINER" ]; then
        echo "ERROR: Redis container not detected" >&2
        return 1
    fi
    docker exec "$REDIS_CONTAINER" redis-cli "$@" 2>/dev/null
}

# =============================================================================
# Project ID
# =============================================================================

get_project_id() {
    local helpers_dir
    helpers_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local project_root
    project_root="$(dirname "$(dirname "$helpers_dir")")"
    jq -r '.project_id // empty' "$project_root/.aoa/home.json" 2>/dev/null || echo ""
}

# =============================================================================
# API helpers
# =============================================================================

api_get() {
    local path="$1"
    local extra="${2:-}"
    local sep="?"
    [[ "$path" == *"?"* ]] && sep="&"
    local url="${API_URL}${path}"
    [ -n "$PROJECT_ID" ] && url="${url}${sep}project_id=${PROJECT_ID}"
    [ -n "$extra" ] && url="${url}&${extra}"
    curl -sf --max-time 5 "$url" 2>/dev/null
}

api_post() {
    local path="$1"
    local body="$2"
    curl -sf --max-time 5 -X POST "${API_URL}${path}" \
        -H "Content-Type: application/json" \
        -d "$body" 2>/dev/null
}

# =============================================================================
# JSON assertion helpers
# =============================================================================

# Check that a jq path returns a non-empty, non-null value
assert_json_has_field() {
    local json="$1" path="$2" name="$3"
    local val
    val=$(echo "$json" | jq -r "$path // empty" 2>/dev/null)
    if [ -n "$val" ] && [ "$val" != "null" ]; then
        pass "$name"
    else
        fail "$name" "field $path missing or null"
    fi
}

# Check that a jq path returns an integer
assert_json_is_integer() {
    local json="$1" path="$2" name="$3"
    local val
    val=$(echo "$json" | jq -r "$path // empty" 2>/dev/null)
    if [[ "$val" =~ ^-?[0-9]+$ ]]; then
        pass "$name"
    else
        fail "$name" "expected integer at $path, got: $val"
    fi
}

# Check that a jq path returns a value greater than min
assert_json_gt() {
    local json="$1" path="$2" min="$3" name="$4"
    local val
    val=$(echo "$json" | jq -r "$path // 0" 2>/dev/null)
    if [ "$val" -gt "$min" ] 2>/dev/null; then
        pass "$name ($val)"
    else
        fail "$name" "expected >$min, got: $val"
    fi
}

# Check that a jq path returns a value equal to expected
assert_json_eq() {
    local json="$1" path="$2" expected="$3" name="$4"
    local val
    val=$(echo "$json" | jq -r "$path // empty" 2>/dev/null)
    if [ "$val" = "$expected" ]; then
        pass "$name ($val)"
    else
        fail "$name" "expected $expected, got: $val"
    fi
}

# =============================================================================
# Preflight
# =============================================================================

preflight() {
    if ! curl -sf --max-time 3 "${API_URL}/health" > /dev/null 2>&1; then
        echo -e "${RED}Error: aOa services not running at ${API_URL}${NC}"
        echo "Start services first: aoa start"
        exit 1
    fi

    detect_docker_mode
    PROJECT_ID=$(get_project_id)

    if [ -z "$PROJECT_ID" ]; then
        echo -e "${RED}Error: No project_id in .aoa/home.json${NC}"
        echo "Run 'aoa init' first."
        exit 1
    fi

    echo -e "${DIM}Docker: ${DOCKER_MODE} │ Container: ${AOA_CONTAINER:-none} │ Project: ${PROJECT_ID:0:8}...${NC}"
}

# =============================================================================
# Summary
# =============================================================================

summary() {
    local total=$((PASSED + FAILED + SKIPPED))
    echo ""
    echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════════════════════════${NC}"
    echo -e "${BOLD}Results:${NC}"
    echo -e "  ${GREEN}Passed:${NC}  $PASSED"
    echo -e "  ${RED}Failed:${NC}  $FAILED"
    echo -e "  ${YELLOW}Skipped:${NC} $SKIPPED"
    echo -e "  Total:   $total"
    echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════════════════════════${NC}"

    if [ $FAILED -gt 0 ]; then
        echo -e "${RED}${BOLD}SOME TESTS FAILED${NC}"
        return 1
    elif [ $SKIPPED -gt 0 ]; then
        echo -e "${YELLOW}${BOLD}ALL PASSED (some skipped)${NC}"
        return 0
    else
        echo -e "${GREEN}${BOLD}ALL TESTS PASSED${NC}"
        return 0
    fi
}
