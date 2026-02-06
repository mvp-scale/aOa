#!/bin/bash
# =============================================================================
# aOa Complete Test Suite Runner
# =============================================================================
#
# Runs all test suites and reports combined totals.
#
# Usage:
#   ./tests/test_all.sh              # Run all suites
#   ./tests/test_all.sh --quick      # Pass --quick to e2e suite
#
# =============================================================================

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'
BOLD='\033[1m'

TOTAL_PASS=0
TOTAL_FAIL=0
TOTAL_SKIP=0
SUITE_RESULTS=()

run_suite() {
    local name="$1"
    local script="$2"
    shift 2

    echo ""
    echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}Running: ${name}${NC}"
    echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    local output
    output=$("$script" "$@" 2>&1)
    local exit_code=$?

    echo "$output"

    # Parse results from output (strip ANSI codes first)
    local clean
    clean=$(echo "$output" | sed 's/\x1b\[[0-9;]*m//g')
    local p f s
    p=$(echo "$clean" | grep -oE 'Passed:\s+[0-9]+' | grep -oE '[0-9]+' || echo "0")
    f=$(echo "$clean" | grep -oE 'Failed:\s+[0-9]+' | grep -oE '[0-9]+' || echo "0")
    s=$(echo "$clean" | grep -oE 'Skipped:\s+[0-9]+' | grep -oE '[0-9]+' || echo "0")

    TOTAL_PASS=$((TOTAL_PASS + p))
    TOTAL_FAIL=$((TOTAL_FAIL + f))
    TOTAL_SKIP=$((TOTAL_SKIP + s))

    local status="${GREEN}PASS${NC}"
    [ "$exit_code" -ne 0 ] && status="${RED}FAIL${NC}"

    SUITE_RESULTS+=("$(printf "  %-30s %b  (%d passed, %d failed, %d skipped)" "$name" "$status" "$p" "$f" "$s")")
}

# Header
echo -e "${CYAN}${BOLD}"
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║              aOa Complete Test Suite                            ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Run suites
run_suite "E2E Integration" "$SCRIPT_DIR/test_e2e.sh" "$@"
run_suite "Unix Parity"     "$SCRIPT_DIR/test_unix_parity.sh"

# Combined summary
echo ""
echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}Suite Results:${NC}"
for line in "${SUITE_RESULTS[@]}"; do
    echo -e "$line"
done
echo ""
echo -e "${BOLD}Combined:${NC}"
echo -e "  ${GREEN}Passed:${NC}  $TOTAL_PASS"
echo -e "  ${RED}Failed:${NC}  $TOTAL_FAIL"
echo -e "  ${YELLOW}Skipped:${NC} $TOTAL_SKIP"
TOTAL=$((TOTAL_PASS + TOTAL_FAIL + TOTAL_SKIP))
echo -e "  Total:   $TOTAL"
echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════════════════════════${NC}"

if [ $TOTAL_FAIL -gt 0 ]; then
    echo -e "${RED}${BOLD}SOME TESTS FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}${BOLD}ALL TESTS PASSED${NC}"
    exit 0
fi
