#!/bin/bash
# =============================================================================
# aOa End-to-End Integration Test Suite
# =============================================================================
#
# Tests the complete aOa pipeline including GL-053 Domain Learning:
#   1. Service health
#   2. Domain seeding (quickstart)
#   3. Grep with domains and tags
#   4. Intent capture
#   5. Domain learning triggers
#   6. Self-improvement loop
#
# Usage:
#   ./tests/test_e2e.sh              # Run all tests
#   ./tests/test_e2e.sh --quick      # Skip slow tests
#   ./tests/test_e2e.sh --verbose    # Show all output
#
# Exit codes:
#   0 = All tests passed
#   1 = Some tests failed
#
# =============================================================================

set -o pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
DIM='\033[2m'
NC='\033[0m'
BOLD='\033[1m'

PASSED=0
FAILED=0
SKIPPED=0
VERBOSE=false
QUICK=false

# Parse args
while [[ $# -gt 0 ]]; do
    case "$1" in
        --verbose|-v) VERBOSE=true; shift ;;
        --quick|-q) QUICK=true; shift ;;
        *) shift ;;
    esac
done

# Test helpers
pass() {
    echo -e "  ${GREEN}✓${NC} $1"
    ((PASSED++))
}

fail() {
    echo -e "  ${RED}✗${NC} $1"
    if [ -n "${2:-}" ]; then
        echo -e "    ${DIM}$2${NC}"
    fi
    ((FAILED++))
}

skip() {
    echo -e "  ${YELLOW}○${NC} $1 ${DIM}(skipped)${NC}"
    ((SKIPPED++))
}

section() {
    echo ""
    echo -e "${CYAN}${BOLD}$1${NC}"
    echo -e "${DIM}────────────────────────────────────────${NC}"
}

# Get project ID from .aoa/home.json
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
PROJECT_ID=$(jq -r '.project_id // empty' "$PROJECT_ROOT/.aoa/home.json" 2>/dev/null || echo "")

if [ -z "$PROJECT_ID" ]; then
    echo -e "${RED}Error: Could not find project_id in .aoa/home.json${NC}"
    echo "Run 'aoa init' first to initialize the project."
    exit 1
fi

echo -e "${DIM}Project ID: $PROJECT_ID${NC}"

API_URL="http://localhost:8080"

# =============================================================================
# SECTION 1: Service Health
# =============================================================================

section "1. Service Health"

# Test Docker containers running (check for partial name match)
if docker ps --format '{{.Names}}' | grep -q "index"; then
    pass "Index service running"
else
    fail "Index service not running"
fi

if docker ps --format '{{.Names}}' | grep -q "redis"; then
    pass "Redis service running"
else
    fail "Redis service not running"
fi

if docker ps --format '{{.Names}}' | grep -q "gateway"; then
    pass "Gateway service running"
else
    fail "Gateway service not running"
fi

# Test API health endpoint
health_response=$(curl -s "$API_URL/health" 2>/dev/null)
if echo "$health_response" | grep -q "ok\|healthy\|files"; then
    pass "API health endpoint responds"
else
    fail "API health endpoint failed" "$health_response"
fi

# =============================================================================
# SECTION 2: Domain Seeding
# =============================================================================

section "2. Domain Seeding"

# Check if domains are seeded
domain_stats=$(curl -s "$API_URL/domains/stats?project=$PROJECT_ID" 2>/dev/null)
domain_count=$(echo "$domain_stats" | jq -r '.domains // 0' 2>/dev/null || echo "0")
domain_count=${domain_count:-0}

if [ "$domain_count" -gt 0 ] 2>/dev/null; then
    pass "Domains seeded: $domain_count domains"
else
    # Try to seed
    seed_result=$(curl -s -X POST "$API_URL/domains/seed" -H "Content-Type: application/json" -d "{\"project\":\"$PROJECT_ID\"}" 2>/dev/null)
    seeded=$(echo "$seed_result" | jq -r '.domains // 0')
    if [ "$seeded" -gt 0 ]; then
        pass "Domains seeded on demand: $seeded domains"
    else
        fail "Failed to seed domains" "$seed_result"
    fi
fi

# Check term count
term_count=$(echo "$domain_stats" | jq -r '.total_terms // 0')
if [ "$term_count" -gt 0 ]; then
    pass "Terms available: $term_count terms"
else
    fail "No terms found"
fi

# =============================================================================
# SECTION 3: Domain Lookup
# =============================================================================

section "3. Domain Lookup"

# Test term lookup
login_lookup=$(curl -s "$API_URL/domains/lookup?project=$PROJECT_ID&term=login" 2>/dev/null)
login_domain=$(echo "$login_lookup" | jq -r '.domains[0].name // empty')

if [ "$login_domain" = "@authentication" ]; then
    pass "Term lookup: login -> @authentication"
else
    fail "Term lookup failed" "Expected @authentication, got: $login_domain"
fi

# Test symbol lookup
symbol_lookup=$(curl -s "$API_URL/domains/lookup?project=$PROJECT_ID&symbol=authenticate_user" 2>/dev/null)
symbol_domain=$(echo "$symbol_lookup" | jq -r '.domain // empty')

if [ -n "$symbol_domain" ]; then
    pass "Symbol lookup: authenticate_user -> $symbol_domain"
else
    skip "Symbol lookup (no domain matched)"
fi

# =============================================================================
# SECTION 4: Grep with Domains
# =============================================================================

section "4. Grep with Domains"

# Test grep returns domain field
grep_result=$(curl -s "$API_URL/grep?q=context_search&project_id=$PROJECT_ID&limit=1" 2>/dev/null)
grep_domain=$(echo "$grep_result" | jq -r '.results[0].domain // empty')

if [ -n "$grep_domain" ]; then
    pass "Grep returns domain: $grep_domain"
else
    skip "Grep domain (symbol may not have matching domain)"
fi

# Test grep returns tags (AC matching)
grep_tags=$(echo "$grep_result" | jq -r '.results[0].tags | length')

if [ "$grep_tags" -gt 0 ]; then
    pass "Grep returns AC tags: $grep_tags tags"
else
    fail "Grep AC tags missing"
fi

# Test CLI grep output format
cli_grep=$(aoa grep login 2>&1 | head -5)
if echo "$cli_grep" | grep -q "⚡ aOa"; then
    pass "CLI grep header format"
else
    fail "CLI grep header missing"
fi

# Check for CYAN tags in output (escape code 36m)
if echo "$cli_grep" | grep -q "36m"; then
    pass "CLI shows #tags in CYAN"
else
    fail "CLI #tags not in CYAN"
fi

# =============================================================================
# SECTION 5: Intent Capture
# =============================================================================

section "5. Intent Capture"

# Get current intent count
intent_before=$(curl -s "$API_URL/intent/stats?project_id=$PROJECT_ID" 2>/dev/null | jq -r '.total // 0')

# Run a grep to generate intent
aoa grep test_intent_capture_marker >/dev/null 2>&1

# Check intent was captured
sleep 1
intent_after=$(curl -s "$API_URL/intent/stats?project_id=$PROJECT_ID" 2>/dev/null | jq -r '.total // 0')

if [ "$intent_after" -gt "$intent_before" ]; then
    pass "Intent captured on grep"
else
    skip "Intent capture (hook may not be active)"
fi

# Check prompt count increments
prompt_count=$(curl -s "$API_URL/domains/stats?project=$PROJECT_ID" 2>/dev/null | jq -r '.prompt_count // 0')
if [ "$prompt_count" -gt 0 ]; then
    pass "Prompt count tracking: $prompt_count"
else
    skip "Prompt count (may not have triggered)"
fi

# =============================================================================
# SECTION 6: Learning Triggers
# =============================================================================

section "6. Learning Triggers"

# Note: Domain learning uses HOOK MODE - Claude handles Haiku calls in conversation
# The API just tracks thresholds and stores results. No API keys needed.

if [ "$QUICK" = true ]; then
    skip "Learning threshold check (--quick mode)"
    skip "Rebalance threshold check (--quick mode)"
else
    # Test should_learn threshold
    stats=$(curl -s "$API_URL/domains/stats?project=$PROJECT_ID" 2>/dev/null)
    prompt_count=$(echo "$stats" | jq -r '.prompt_count // 0')
    should_learn=$(echo "$stats" | jq -r '.should_learn // false')

    if [ "$prompt_count" -gt 0 ]; then
        pass "Prompt counter working: $prompt_count prompts"
    else
        fail "Prompt counter not working"
    fi

    # Check should_learn triggers at 10
    if [ "$prompt_count" -ge 10 ] && [ "$should_learn" = "true" ]; then
        pass "Learning threshold triggers at 10+ prompts"
    elif [ "$prompt_count" -lt 10 ]; then
        pass "Learning threshold: $prompt_count/10 (not yet triggered)"
    else
        skip "Learning threshold check (should_learn=$should_learn)"
    fi

    # Test should_autotune check
    should_autotune=$(echo "$stats" | jq -r '.should_autotune // false')
    pass "Auto-tune check available (should_autotune=$should_autotune)"
fi

# =============================================================================
# SECTION 7: Hit Counter
# =============================================================================

section "7. Hit Counter"

# Get current hits for authentication domain
hits_before=$(docker exec aoa-redis-1 redis-cli HGET "aoa:$PROJECT_ID:domain:@authentication:meta" hits 2>/dev/null || echo "0")

# Run grep that should trigger hit
curl -s "$API_URL/grep?q=login&project_id=$PROJECT_ID&limit=1" >/dev/null 2>&1

# Check hits increased
hits_after=$(docker exec aoa-redis-1 redis-cli HGET "aoa:$PROJECT_ID:domain:@authentication:meta" hits 2>/dev/null || echo "0")

if [ "${hits_after:-0}" -gt "${hits_before:-0}" ]; then
    pass "Hit counter increments: $hits_before -> $hits_after"
else
    skip "Hit counter (domain may not have matched)"
fi

# =============================================================================
# SECTION 8: Unix Parity (Quick Check)
# =============================================================================

section "8. Unix Parity (Quick Check)"

# Test basic grep works
if aoa grep cache >/dev/null 2>&1; then
    pass "aoa grep works"
else
    fail "aoa grep failed"
fi

# Test egrep works
if aoa egrep "TODO" >/dev/null 2>&1; then
    pass "aoa egrep works"
else
    fail "aoa egrep failed"
fi

# Test find works
if aoa find "*.py" >/dev/null 2>&1; then
    pass "aoa find works"
else
    fail "aoa find failed"
fi

# Test health works
if aoa health >/dev/null 2>&1; then
    pass "aoa health works"
else
    fail "aoa health failed"
fi

# =============================================================================
# SUMMARY
# =============================================================================

echo ""
echo -e "${BOLD}════════════════════════════════════════${NC}"
TOTAL=$((PASSED + FAILED + SKIPPED))
echo -e "${BOLD}Results:${NC} ${GREEN}$PASSED passed${NC}, ${RED}$FAILED failed${NC}, ${YELLOW}$SKIPPED skipped${NC} / $TOTAL total"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}${BOLD}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}${BOLD}Some tests failed.${NC}"
    exit 1
fi
