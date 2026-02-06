#!/bin/bash
# =============================================================================
# aOa End-to-End Integration Test Suite
# =============================================================================
#
# Tests the complete aOa system: API endpoints, CLI commands, domains,
# intent tracking, configuration, and domain lifecycle.
#
# Usage:
#   ./tests/test_e2e.sh              # Run all sections
#   ./tests/test_e2e.sh --quick      # Skip slow lifecycle tests
#   ./tests/test_e2e.sh --section 4  # Run only section 4 (CLI)
#   ./tests/test_e2e.sh --verbose    # Show API response bodies
#
# Exit codes:
#   0 = All tests passed (some may be skipped)
#   1 = Some tests failed
#
# =============================================================================

set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/helpers.sh"

# Parse args
QUICK=false
VERBOSE=false
ONLY_SECTION=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --quick|-q)   QUICK=true; shift ;;
        --verbose|-v) VERBOSE=true; shift ;;
        --section|-s) ONLY_SECTION="$2"; shift 2 ;;
        *) shift ;;
    esac
done

verbose() {
    [ "$VERBOSE" = true ] && echo -e "    ${DIM}$1${NC}"
}

# =============================================================================
# SECTION 1: Service Health
# =============================================================================

test_service_health() {
    section "1. Service Health"

    # 1.1: Health endpoint responds with JSON
    local health
    health=$(api_get "/health")
    if [ -n "$health" ] && echo "$health" | jq -e '.' > /dev/null 2>&1; then
        pass "Health endpoint returns valid JSON"
        verbose "$health"
    else
        fail "Health endpoint" "empty or invalid response"
        return
    fi

    # 1.2: Files indexed
    local files
    files=$(echo "$health" | jq -r '.local.files // .files // 0' 2>/dev/null)
    if [ "$files" -gt 0 ] 2>/dev/null; then
        pass "Index has files ($files)"
    else
        fail "Index files" "expected >0, got: $files"
    fi

    # 1.3: Symbols indexed
    local symbols
    symbols=$(echo "$health" | jq -r '.local.symbols // .symbols // 0' 2>/dev/null)
    if [ "$symbols" -gt 0 ] 2>/dev/null; then
        pass "Index has symbols ($symbols)"
    else
        fail "Index symbols" "expected >0, got: $symbols"
    fi

    # 1.4: Docker mode detected
    if [ "$DOCKER_MODE" != "unknown" ]; then
        pass "Docker mode: $DOCKER_MODE ($AOA_CONTAINER)"
    else
        fail "Docker mode" "could not determine monolithic vs compose"
    fi

    # 1.5: Redis reachable
    local ping
    ping=$(redis_cli PING 2>/dev/null)
    if [ "$ping" = "PONG" ]; then
        pass "Redis PING via $DOCKER_MODE container"
    else
        skip "Redis PING" "could not reach redis-cli"
    fi
}

# =============================================================================
# SECTION 2: Core Search API
# =============================================================================

test_core_search_api() {
    section "2. Core Search API"

    # 2.1: /symbol?q=
    local result
    result=$(api_get "/symbol?q=def")
    if [ -n "$result" ] && echo "$result" | jq -e '.results' > /dev/null 2>&1; then
        pass "/symbol returns results array"
        verbose "$(echo "$result" | jq -r '.results | length') results"
    else
        fail "/symbol" "missing results field"
    fi

    # 2.2: /symbol returns timing
    assert_json_has_field "$result" ".ms // .time_ms" "/symbol returns timing"

    # 2.3: /symbol results have file field
    local first_file
    first_file=$(echo "$result" | jq -r '.results[0].file // empty' 2>/dev/null)
    if [ -n "$first_file" ]; then
        pass "/symbol results have file field"
    else
        skip "/symbol file field" "no results for 'def'"
    fi

    # 2.4: /symbol results have line field
    local first_line
    first_line=$(echo "$result" | jq -r '.results[0].line // empty' 2>/dev/null)
    if [ -n "$first_line" ]; then
        pass "/symbol results have line field"
    else
        skip "/symbol line field" "no results"
    fi

    # 2.5: /grep?q= (content search)
    result=$(api_get "/grep?q=import")
    if [ -n "$result" ] && echo "$result" | jq -e '.results' > /dev/null 2>&1; then
        pass "/grep returns results"
    else
        fail "/grep" "missing results field"
    fi

    # 2.6: /multi (multi-term search)
    result=$(curl -sf -X POST "${API_URL}/multi" \
        -H "Content-Type: application/json" \
        -d "{\"terms\":[\"def\",\"self\"],\"limit\":5,\"project\":\"${PROJECT_ID}\"}" 2>/dev/null)
    if [ -n "$result" ] && echo "$result" | jq -e '.results' > /dev/null 2>&1; then
        pass "/multi returns results"
    else
        fail "/multi" "missing results field"
    fi

    # 2.7: /grep?regex=true (egrep via /grep)
    result=$(api_get "/grep?q=def&regex=true&limit=3")
    if [ -n "$result" ] && echo "$result" | jq -e '.results' > /dev/null 2>&1; then
        pass "/grep?regex=true returns results"
    else
        fail "/grep?regex=true" "invalid response"
    fi

    # 2.8: /symbol with domain tags in response
    result=$(api_get "/symbol?q=auth")
    local has_tags
    has_tags=$(echo "$result" | jq '[.results[]? | select(.tags != null and (.tags | length > 0))] | length' 2>/dev/null)
    if [ "${has_tags:-0}" -gt 0 ] 2>/dev/null; then
        pass "/symbol results include domain tags"
    else
        skip "/symbol domain tags" "no tagged results for 'auth'"
    fi
}

# =============================================================================
# SECTION 3: File Operations API
# =============================================================================

test_file_operations_api() {
    section "3. File Operations API"

    # 3.1: /files returns list
    local result
    result=$(api_get "/files?limit=5")
    if [ -n "$result" ] && echo "$result" | jq -e '.results // .files' > /dev/null 2>&1; then
        pass "/files returns file list"
    else
        fail "/files" "missing results"
    fi

    # 3.2: /files results have path field
    local first_path
    first_path=$(echo "$result" | jq -r '.results[0].path // .files[0].path // .results[0] // empty' 2>/dev/null)
    if [ -n "$first_path" ]; then
        pass "/files entries have path ($first_path)"
    else
        fail "/files path field" "no path in first result"
    fi

    # 3.3: /file?path= returns content
    if [ -n "$first_path" ]; then
        local file_result
        file_result=$(api_get "/file?path=${first_path}&lines=1-10")
        if [ -n "$file_result" ] && echo "$file_result" | jq -e '.' > /dev/null 2>&1; then
            pass "/file returns content for $first_path"
        else
            fail "/file content" "empty response for $first_path"
        fi
    else
        skip "/file content" "no file path available"
    fi

    # 3.4-3.6: /outline for a Python file
    local py_file
    py_file=$(api_get "/files?match=*.py&limit=1" | jq -r '.results[0].path // .files[0].path // .results[0] // empty' 2>/dev/null)
    if [ -n "$py_file" ]; then
        local outline
        outline=$(api_get "/outline?file=${py_file}")
        if [ -n "$outline" ] && echo "$outline" | jq -e '.symbols' > /dev/null 2>&1; then
            pass "/outline returns symbols for $py_file"

            # 3.5: symbols have kind
            local kind
            kind=$(echo "$outline" | jq -r '.symbols[0].kind // empty' 2>/dev/null)
            if [ -n "$kind" ]; then
                pass "/outline symbols have kind ($kind)"
            else
                skip "/outline kind" "no symbols in file"
            fi

            # 3.6: language field
            assert_json_has_field "$outline" ".language" "/outline reports language"
        else
            fail "/outline" "missing symbols field"
            skip "/outline kind" "no outline data"
            skip "/outline language" "no outline data"
        fi
    else
        skip "/outline" "no .py file in index"
        skip "/outline kind" "no .py file"
        skip "/outline language" "no .py file"
    fi
}

# =============================================================================
# SECTION 4: CLI Commands
# =============================================================================

test_cli_commands() {
    section "4. CLI Commands"

    # 4.1: aoa health
    if aoa health > /dev/null 2>&1; then
        pass "aoa health"
    else
        fail "aoa health" "non-zero exit"
    fi

    # 4.2: aoa grep <term>
    local result
    result=$(aoa grep def 2>&1)
    if [[ "$result" == *"hits"* ]] || [[ "$result" == *"aOa"* ]]; then
        pass "aoa grep <term>"
    else
        fail "aoa grep" "unexpected output: ${result:0:80}"
    fi

    # 4.3: aoa grep "multi term" (OR)
    result=$(aoa grep "def class" 2>&1)
    if [[ "$result" == *"hits"* ]] || [[ "$result" == *"aOa"* ]]; then
        pass "aoa grep multi-term OR"
    else
        fail "aoa grep multi-term" "unexpected output"
    fi

    # 4.4: aoa grep -a term1,term2 (AND)
    result=$(aoa grep -a def,self 2>&1)
    if [[ "$result" == *"hits"* ]] || [[ "$result" == *"aOa"* ]]; then
        pass "aoa grep -a AND mode"
    else
        fail "aoa grep -a" "unexpected output"
    fi

    # 4.5: aoa egrep "pattern"
    result=$(aoa egrep "TODO" 2>&1)
    if [[ $? -eq 0 ]] || [[ "$result" == *"hits"* ]]; then
        pass "aoa egrep"
    else
        fail "aoa egrep" "error in output"
    fi

    # 4.6: aoa find "*.py"
    result=$(aoa find "*.py" 2>&1)
    if [[ "$result" == *".py"* ]] || [[ "$result" == *"files"* ]] || [[ "$result" == *"match"* ]]; then
        pass "aoa find pattern"
    else
        fail "aoa find" "unexpected output"
    fi

    # 4.7: aoa locate
    result=$(aoa locate indexer 2>&1)
    if [[ "$result" == *"indexer"* ]] || [[ "$result" == *"match"* ]]; then
        pass "aoa locate"
    else
        fail "aoa locate" "unexpected output"
    fi

    # 4.8: aoa tree
    result=$(aoa tree 2>&1)
    if [[ "$result" == *"├"* ]] || [[ "$result" == *"directories"* ]] || [[ "$result" == *"files"* ]]; then
        pass "aoa tree"
    else
        fail "aoa tree" "unexpected output"
    fi

    # 4.9: aoa stats
    result=$(aoa stats 2>&1)
    if [[ "$result" == *"Stats"* ]] || [[ "$result" == *"Files"* ]] || [[ "$result" == *"files"* ]]; then
        pass "aoa stats"
    else
        fail "aoa stats" "unexpected output: ${result:0:80}"
    fi

    # 4.10: aoa config
    result=$(aoa config 2>&1)
    if [[ "$result" == *"Rebalance"* ]] || [[ "$result" == *"rebalance"* ]] || [[ "$result" == *"Config"* ]] || [[ "$result" == *"config"* ]]; then
        pass "aoa config"
    else
        fail "aoa config" "unexpected output: ${result:0:80}"
    fi

    # 4.11: aoa domains
    result=$(aoa domains 2>&1)
    if [[ "$result" == *"Domains"* ]] || [[ "$result" == *"domains"* ]] || [[ "$result" == *"aOa"* ]]; then
        pass "aoa domains"
    else
        fail "aoa domains" "unexpected output"
    fi

    # 4.12: aoa domains --json
    result=$(aoa domains --json 2>&1)
    if echo "$result" | jq -e '.' > /dev/null 2>&1; then
        pass "aoa domains --json returns valid JSON"
    else
        # When no domains exist, CLI may output text instead of JSON
        if [[ "$result" == *"No domains"* ]] || [[ "$result" == *"domain_count"* ]]; then
            skip "aoa domains --json" "no domains seeded, CLI outputs text"
        else
            fail "aoa domains --json" "invalid JSON: ${result:0:80}"
        fi
    fi

    # 4.13: aoa intent
    result=$(aoa intent 2>&1)
    if [[ $? -eq 0 ]]; then
        pass "aoa intent"
    else
        fail "aoa intent" "non-zero exit"
    fi

    # 4.14: aoa bigrams
    result=$(aoa bigrams 2>&1)
    if [[ $? -eq 0 ]]; then
        pass "aoa bigrams"
    else
        fail "aoa bigrams" "non-zero exit"
    fi
}

# =============================================================================
# SECTION 5: Intent Tracking
# =============================================================================

test_intent_tracking() {
    section "5. Intent Tracking"

    # 5.1: /intent/recent
    local result
    result=$(api_get "/intent/recent?limit=5")
    if [ -n "$result" ] && echo "$result" | jq -e '.' > /dev/null 2>&1; then
        pass "/intent/recent returns JSON"
        verbose "$(echo "$result" | jq -r '.records // .intents // [] | length') records"
    else
        fail "/intent/recent" "invalid response"
    fi

    # 5.2: /intent/stats
    local stats
    stats=$(api_get "/intent/stats")
    if [ -n "$stats" ] && echo "$stats" | jq -e '.' > /dev/null 2>&1; then
        pass "/intent/stats returns JSON"
    else
        fail "/intent/stats" "invalid response"
        return
    fi

    # 5.3: total is integer
    local total
    total=$(echo "$stats" | jq -r '.total_records // .total // 0' 2>/dev/null)
    if [[ "$total" =~ ^[0-9]+$ ]]; then
        pass "/intent/stats total is integer ($total)"
    else
        fail "/intent/stats total" "expected integer, got: $total"
    fi

    # 5.4: /intent/hits
    local hits
    hits=$(api_get "/intent/hits?limit=5")
    if [ -n "$hits" ] && echo "$hits" | jq -e '.' > /dev/null 2>&1; then
        pass "/intent/hits returns JSON"
    else
        skip "/intent/hits" "endpoint may not be active"
    fi

    # 5.5: /intent/rolling
    local rolling
    rolling=$(api_get "/intent/rolling")
    if [ -n "$rolling" ] && echo "$rolling" | jq -e '.' > /dev/null 2>&1; then
        pass "/intent/rolling returns JSON"
    else
        skip "/intent/rolling" "endpoint may not be active"
    fi
}

# =============================================================================
# SECTION 6: Domain Management
# =============================================================================

test_domain_management() {
    section "6. Domain Management"

    # 6.1: /domains/stats
    local stats
    stats=$(api_get "/domains/stats")
    if [ -n "$stats" ] && echo "$stats" | jq -e '.' > /dev/null 2>&1; then
        pass "/domains/stats returns JSON"
        verbose "$stats"
    else
        fail "/domains/stats" "invalid response"
        return
    fi

    # 6.2: domain count is integer
    local count
    count=$(echo "$stats" | jq -r '.domains // 0' 2>/dev/null)
    if [[ "$count" =~ ^[0-9]+$ ]]; then
        pass "/domains/stats domain count ($count)"
    else
        fail "/domains/stats count" "expected integer, got: $count"
    fi

    # 6.3: prompt_count present
    assert_json_has_field "$stats" ".prompt_count" "/domains/stats has prompt_count"

    # 6.4: enrichment block present
    if echo "$stats" | jq -e '.enrichment' > /dev/null 2>&1; then
        pass "/domains/stats has enrichment block"
    else
        skip "/domains/stats enrichment" "enrichment block missing"
    fi

    # 6.5: /domains/list returns array
    local list
    list=$(api_get "/domains/list")
    if echo "$list" | jq -e '.domains | type == "array"' > /dev/null 2>&1; then
        pass "/domains/list returns array"
    elif echo "$list" | jq -e 'type == "array"' > /dev/null 2>&1; then
        pass "/domains/list returns array (top-level)"
    else
        fail "/domains/list" "expected array"
    fi

    # 6.6: list entries have name field (structure test, not content)
    local first_name
    first_name=$(echo "$list" | jq -r '.domains[0].name // .[0].name // empty' 2>/dev/null)
    if [ -n "$first_name" ]; then
        pass "/domains/list entries have name field"
    else
        if [ "$count" -eq 0 ] 2>/dev/null; then
            skip "/domains/list name" "no domains seeded"
        else
            fail "/domains/list name" "missing name field"
        fi
    fi

    # 6.7: /domains/lookup
    local lookup
    lookup=$(api_get "/domains/lookup?term=test")
    if [ -n "$lookup" ] && echo "$lookup" | jq -e '.' > /dev/null 2>&1; then
        pass "/domains/lookup returns JSON"
    else
        skip "/domains/lookup" "endpoint unavailable"
    fi

    # 6.8: /domains/enrichment-status
    local enrich
    enrich=$(api_get "/domains/enrichment-status")
    if [ -n "$enrich" ] && echo "$enrich" | jq -e '.' > /dev/null 2>&1; then
        pass "/domains/enrichment-status returns JSON"
    else
        skip "/domains/enrichment-status" "endpoint unavailable"
    fi
}

# =============================================================================
# SECTION 7: Configuration
# =============================================================================

test_configuration() {
    section "7. Configuration"

    # 7.1: /config/thresholds returns data
    local result
    result=$(api_get "/config/thresholds")
    if [ -n "$result" ] && echo "$result" | jq -e '.thresholds' > /dev/null 2>&1; then
        pass "/config/thresholds returns thresholds object"
        verbose "$result"
    else
        fail "/config/thresholds" "missing thresholds field"
        return
    fi

    # 7.2: rebalance is numeric
    local rebalance
    rebalance=$(echo "$result" | jq -r '.thresholds.rebalance // empty' 2>/dev/null)
    if [[ "$rebalance" =~ ^[0-9]+\.?[0-9]*$ ]]; then
        pass "rebalance threshold ($rebalance)"
    else
        fail "rebalance threshold" "expected numeric, got: $rebalance"
    fi

    # 7.3: autotune is numeric
    local autotune
    autotune=$(echo "$result" | jq -r '.thresholds.autotune // empty' 2>/dev/null)
    if [[ "$autotune" =~ ^[0-9]+\.?[0-9]*$ ]]; then
        pass "autotune threshold ($autotune)"
    else
        fail "autotune threshold" "expected numeric, got: $autotune"
    fi

    # 7.4: decay_rate between 0 and 1
    local decay
    decay=$(echo "$result" | jq -r '.thresholds.decay_rate // empty' 2>/dev/null)
    if [ -n "$decay" ]; then
        local in_range
        in_range=$(echo "$decay > 0 && $decay <= 1" | bc -l 2>/dev/null || echo "0")
        if [ "$in_range" = "1" ]; then
            pass "decay_rate in range ($decay)"
        else
            fail "decay_rate" "expected 0 < rate <= 1, got: $decay"
        fi
    else
        fail "decay_rate" "missing"
    fi

    # 7.5: prune_floor is numeric
    local prune
    prune=$(echo "$result" | jq -r '.thresholds.prune_floor // empty' 2>/dev/null)
    if [[ "$prune" =~ ^[0-9]+\.?[0-9]*$ ]]; then
        pass "prune_floor ($prune)"
    else
        fail "prune_floor" "expected numeric, got: $prune"
    fi

    # 7.6: project_id echoed back
    assert_json_has_field "$result" ".project_id" "/config/thresholds echoes project_id"
}

# =============================================================================
# SECTION 8: Bigrams
# =============================================================================

test_bigrams() {
    section "8. Bigrams"

    # 8.1: /cc/bigrams returns valid JSON
    local result
    result=$(api_get "/cc/bigrams?limit=10")
    if [ -n "$result" ] && echo "$result" | jq -e '.' > /dev/null 2>&1; then
        pass "/cc/bigrams returns valid JSON"
    else
        skip "/cc/bigrams" "endpoint returned empty or invalid"
        return
    fi

    # 8.2: bigrams array exists
    if echo "$result" | jq -e '.bigrams | type == "array"' > /dev/null 2>&1; then
        pass "/cc/bigrams has bigrams array"
    else
        fail "/cc/bigrams array" "missing bigrams field"
    fi

    # 8.3: bigram entries have structure (if any exist)
    local bigram_count
    bigram_count=$(echo "$result" | jq '.bigrams | length' 2>/dev/null)
    if [ "$bigram_count" -gt 0 ] 2>/dev/null; then
        local has_field
        has_field=$(echo "$result" | jq -r '.bigrams[0].bigram // .bigrams[0].pair // empty' 2>/dev/null)
        if [ -n "$has_field" ]; then
            pass "bigram entries have identifier ($bigram_count entries)"
        else
            fail "bigram entry structure" "missing bigram/pair field"
        fi
    else
        skip "bigram entry structure" "no bigrams recorded yet"
    fi
}

# =============================================================================
# SECTION 9: Domain Lifecycle (skip with --quick)
# =============================================================================

test_domain_lifecycle() {
    section "9. Domain Lifecycle"

    if [ "$QUICK" = true ]; then
        skip "Domain lifecycle (all)" "--quick mode"
        return
    fi

    # 9.1: Redis reachable
    local ping
    ping=$(redis_cli PING 2>/dev/null)
    if [ "$ping" != "PONG" ]; then
        skip "Domain lifecycle (all)" "Redis not reachable"
        return
    fi
    pass "Redis reachable"

    # 9.2: Project domain keys exist
    local domain_keys
    domain_keys=$(redis_cli KEYS "aoa:${PROJECT_ID}:domain:*" 2>/dev/null | head -5)
    if [ -n "$domain_keys" ]; then
        pass "Domain keys in Redis"
    else
        skip "Domain keys" "no domain data"
    fi

    # 9.3: prompt_count in Redis
    local pc
    pc=$(redis_cli GET "aoa:${PROJECT_ID}:prompt_count" 2>/dev/null)
    if [ -n "$pc" ] && [[ "$pc" =~ ^[0-9]+$ ]]; then
        pass "prompt_count in Redis ($pc)"
    else
        skip "prompt_count" "not set yet"
    fi

    # 9.4: /domains/autotune endpoint responds
    local autotune_result
    autotune_result=$(api_post "/domains/autotune" "{\"project_id\":\"${PROJECT_ID}\"}")
    if [ -n "$autotune_result" ] && echo "$autotune_result" | jq -e '.' > /dev/null 2>&1; then
        pass "/domains/autotune responds"
    else
        skip "/domains/autotune" "endpoint unavailable"
    fi

    # 9.5: /domains/tune/math endpoint responds
    local tune_result
    tune_result=$(api_post "/domains/tune/math" "{\"project_id\":\"${PROJECT_ID}\"}")
    if [ -n "$tune_result" ] && echo "$tune_result" | jq -e '.' > /dev/null 2>&1; then
        pass "/domains/tune/math responds"
    else
        skip "/domains/tune/math" "endpoint unavailable"
    fi

    # 9.6-9.7: Test mode thresholds
    local set_test
    set_test=$(api_post "/config/thresholds" "{\"project_id\":\"${PROJECT_ID}\",\"mode\":\"test\"}")
    if [ -n "$set_test" ]; then
        local test_rebalance
        test_rebalance=$(echo "$set_test" | jq -r '.thresholds.rebalance // empty' 2>/dev/null)
        if [ "$test_rebalance" = "20" ]; then
            pass "Test mode rebalance=20"
        else
            fail "Test mode rebalance" "expected 20, got: $test_rebalance"
        fi

        local test_autotune
        test_autotune=$(echo "$set_test" | jq -r '.thresholds.autotune // empty' 2>/dev/null)
        if [ "$test_autotune" = "10" ]; then
            pass "Test mode autotune=10"
        else
            fail "Test mode autotune" "expected 10, got: $test_autotune"
        fi
    else
        fail "Set test mode" "empty response"
        fail "Test mode autotune" "could not set test mode"
    fi

    # 9.8: Restore prod mode
    local set_prod
    set_prod=$(api_post "/config/thresholds" "{\"project_id\":\"${PROJECT_ID}\",\"mode\":\"prod\"}")
    local prod_rebalance
    prod_rebalance=$(echo "$set_prod" | jq -r '.thresholds.rebalance // empty' 2>/dev/null)
    if [ "$prod_rebalance" = "100" ]; then
        pass "Prod mode restored (rebalance=100)"
    else
        fail "Prod mode restore" "expected 100, got: $prod_rebalance"
    fi
}

# =============================================================================
# SECTION 10: Metrics
# =============================================================================

test_metrics() {
    section "10. Metrics"

    # 10.1: /metrics returns JSON
    local result
    result=$(api_get "/metrics")
    if [ -n "$result" ] && echo "$result" | jq -e '.' > /dev/null 2>&1; then
        pass "/metrics returns valid JSON"
        verbose "$result"
    else
        fail "/metrics" "empty or invalid JSON"
        return
    fi

    # 10.2: savings block
    if echo "$result" | jq -e '.savings' > /dev/null 2>&1; then
        pass "/metrics has savings block"
    else
        skip "/metrics savings" "missing savings field"
    fi

    # 10.3: savings.tokens is integer
    local tokens
    tokens=$(echo "$result" | jq -r '.savings.tokens // 0' 2>/dev/null)
    if [[ "$tokens" =~ ^-?[0-9]+$ ]]; then
        pass "/metrics savings.tokens ($tokens)"
    else
        fail "/metrics savings.tokens" "expected integer, got: $tokens"
    fi

    # 10.4: total_intents
    if echo "$result" | jq -e '.total_intents // .intents' > /dev/null 2>&1; then
        pass "/metrics has total_intents"
    else
        skip "/metrics total_intents" "field missing"
    fi

    # 10.5: rolling block
    if echo "$result" | jq -e '.rolling' > /dev/null 2>&1; then
        pass "/metrics has rolling block"
    else
        skip "/metrics rolling" "missing rolling field"
    fi
}

# =============================================================================
# SECTION 11: Observer Pattern - Hit Tracking (TDD for Session 80)
# =============================================================================
#
# Maps to Future State Table (BOARD.md Session 78/79).
# Tests verify correct counter increments per entry point.
#
# [REGRESSION] = should PASS now (guards existing behavior)
# [F4]  = FAILS until cohit:kw_term decay implemented
# [F5]  = FAILS until keyword_hits + term_hits decay implemented
# [F6]  = FAILS until keyword noise filter implemented
# [W1]  = FAILS until search path wired through observe()
#
# =============================================================================

test_observer_pattern() {
    section "11. Observer Pattern (Hit Tracking)"

    # Require Redis
    local ping
    ping=$(redis_cli PING 2>/dev/null)
    if [ "$ping" != "PONG" ]; then
        skip "Observer pattern (all)" "Redis not reachable"
        return
    fi

    # --- Setup: Find a domain + term for search tests ---
    local domain_list
    domain_list=$(api_get "/domains/list")
    local domain_name
    domain_name=$(echo "$domain_list" | jq -r '(.domains[0].name // .[0].name // empty)' 2>/dev/null)
    if [ -z "$domain_name" ]; then
        skip "Observer pattern (all)" "no domains seeded"
        return
    fi

    # Find a search term that produces results with domain enrichment.
    # Domain terms are semantic labels, not symbols -- so we search for a
    # known symbol and check if the response has domain tags.
    local search_term=""
    local search_result=""
    local terms_key="aoa:${PROJECT_ID}:domain:${domain_name}:terms"

    # Try up to 3 random terms from the domain
    for _attempt in 1 2 3; do
        local candidate
        candidate=$(redis_cli SRANDMEMBER "$terms_key" 2>/dev/null)
        [ -z "$candidate" ] && continue
        search_result=$(api_get "/symbol?q=${candidate}")
        local hit_count
        hit_count=$(echo "$search_result" | jq -r '.total_matches // 0' 2>/dev/null)
        if [ "${hit_count:-0}" -gt 0 ] 2>/dev/null; then
            search_term="$candidate"
            break
        fi
    done

    # ---- 11.A: Search Path ----

    if [ -z "$search_term" ]; then
        skip "Search increments term_hits" "no domain term produced search results"
        skip "Search increments domain total_hits" "no domain term produced search results"
        skip "Search increments keyword_hits via observe()" "no domain term produced search results"
        skip "Search maintains cohit:kw_term" "no domain term produced search results"
    else
        # 11.1 [REGRESSION]: Search increments term_hits
        local th_before th_after
        th_before=$(redis_cli HGET "aoa:${PROJECT_ID}:term_hits" "$search_term" 2>/dev/null)
        th_before=${th_before:-0}
        api_get "/symbol?q=${search_term}" > /dev/null 2>&1
        th_after=$(redis_cli HGET "aoa:${PROJECT_ID}:term_hits" "$search_term" 2>/dev/null)
        th_after=${th_after:-0}
        if [ "$th_after" -gt "$th_before" ] 2>/dev/null; then
            pass "Search increments term_hits ($th_before -> $th_after)"
        else
            fail "Search increments term_hits" "expected > $th_before, got $th_after"
        fi

        # 11.2 [REGRESSION]: Search increments domain total_hits
        local dh_before dh_after
        dh_before=$(redis_cli HGET "aoa:${PROJECT_ID}:domain:${domain_name}:meta" "total_hits" 2>/dev/null)
        dh_before=${dh_before:-0}
        api_get "/symbol?q=${search_term}" > /dev/null 2>&1
        dh_after=$(redis_cli HGET "aoa:${PROJECT_ID}:domain:${domain_name}:meta" "total_hits" 2>/dev/null)
        dh_after=${dh_after:-0}
        if [ "$dh_after" -gt "$dh_before" ] 2>/dev/null; then
            pass "Search increments domain total_hits ($dh_before -> $dh_after)"
        else
            fail "Search increments domain total_hits" "expected > $dh_before, got $dh_after"
        fi

        # 11.3 [W1]: Search increments keyword_hits for query term
        local kh_before kh_after
        kh_before=$(redis_cli HGET "aoa:${PROJECT_ID}:keyword_hits" "$search_term" 2>/dev/null)
        kh_before=${kh_before:-0}
        api_get "/symbol?q=${search_term}" > /dev/null 2>&1
        kh_after=$(redis_cli HGET "aoa:${PROJECT_ID}:keyword_hits" "$search_term" 2>/dev/null)
        kh_after=${kh_after:-0}
        if [ "$kh_after" -gt "$kh_before" ] 2>/dev/null; then
            pass "Search increments keyword_hits via observe() ($kh_before -> $kh_after)"
        else
            fail "Search increments keyword_hits via observe()" "expected > $kh_before, got $kh_after [W1]"
        fi

        # 11.4 [REGRESSION]: Search tracks cohit:kw_term
        local cohit_key="aoa:${PROJECT_ID}:cohit:kw_term"
        local cohit_before cohit_after
        cohit_before=$(redis_cli HLEN "$cohit_key" 2>/dev/null)
        cohit_before=${cohit_before:-0}
        api_get "/symbol?q=${search_term}" > /dev/null 2>&1
        cohit_after=$(redis_cli HLEN "$cohit_key" 2>/dev/null)
        cohit_after=${cohit_after:-0}
        if [ "$cohit_after" -ge "$cohit_before" ] 2>/dev/null; then
            pass "Search maintains cohit:kw_term ($cohit_before -> $cohit_after)"
        else
            fail "Search cohit:kw_term" "decreased: $cohit_before -> $cohit_after"
        fi
    fi

    # ---- 11.B: Submit-Tags Path ----

    if [ -n "$search_term" ]; then
        # 11.5 [REGRESSION]: Submit-tags increments domain total_hits
        local st_before st_after
        st_before=$(redis_cli HGET "aoa:${PROJECT_ID}:domain:${domain_name}:meta" "total_hits" 2>/dev/null)
        st_before=${st_before:-0}
        api_post "/domains/submit-tags" "{\"project_id\":\"${PROJECT_ID}\",\"tags\":[\"${search_term}\"]}" > /dev/null 2>&1
        st_after=$(redis_cli HGET "aoa:${PROJECT_ID}:domain:${domain_name}:meta" "total_hits" 2>/dev/null)
        st_after=${st_after:-0}
        if [ "$st_after" -gt "$st_before" ] 2>/dev/null; then
            pass "Submit-tags increments domain total_hits ($st_before -> $st_after)"
        else
            fail "Submit-tags increments domain total_hits" "expected > $st_before, got $st_after"
        fi

        # 11.6 [W4]: Submit-tags does NOT increment keyword_hits (domain-only path)
        local st_kh_before st_kh_after
        st_kh_before=$(redis_cli HGET "aoa:${PROJECT_ID}:keyword_hits" "__submit_tag_kw_check__" 2>/dev/null)
        st_kh_before=${st_kh_before:-0}
        api_post "/domains/submit-tags" "{\"project_id\":\"${PROJECT_ID}\",\"tags\":[\"${search_term}\"]}" > /dev/null 2>&1
        st_kh_after=$(redis_cli HGET "aoa:${PROJECT_ID}:keyword_hits" "__submit_tag_kw_check__" 2>/dev/null)
        st_kh_after=${st_kh_after:-0}
        if [ "$st_kh_after" = "$st_kh_before" ]; then
            pass "Submit-tags does not increment keyword_hits (correct)"
        else
            fail "Submit-tags incremented keyword_hits" "should be domain-only path"
        fi
    fi

    # ---- 11.C: Decay (F4/F5) ----
    # Seed known test values, run autotune, verify decay applied

    # 11.7 [F4]: cohit:kw_term decay
    redis_cli HSET "aoa:${PROJECT_ID}:cohit:kw_term" "__test__:__decay_cohit__" "100" > /dev/null 2>&1
    api_post "/domains/tune/math" "{\"project_id\":\"${PROJECT_ID}\"}" > /dev/null 2>&1
    local cohit_val
    cohit_val=$(redis_cli HGET "aoa:${PROJECT_ID}:cohit:kw_term" "__test__:__decay_cohit__" 2>/dev/null)
    cohit_val=${cohit_val:-100}
    if [ "$cohit_val" -lt 100 ] 2>/dev/null; then
        pass "Autotune decays cohit:kw_term (100 -> $cohit_val)"
    else
        fail "Autotune decays cohit:kw_term" "expected < 100, got $cohit_val [F4]"
    fi
    redis_cli HDEL "aoa:${PROJECT_ID}:cohit:kw_term" "__test__:__decay_cohit__" > /dev/null 2>&1

    # 11.8 [F5]: keyword_hits decay
    redis_cli HSET "aoa:${PROJECT_ID}:keyword_hits" "__test_decay_kw__" "100" > /dev/null 2>&1
    api_post "/domains/tune/math" "{\"project_id\":\"${PROJECT_ID}\"}" > /dev/null 2>&1
    local kh_decay_val
    kh_decay_val=$(redis_cli HGET "aoa:${PROJECT_ID}:keyword_hits" "__test_decay_kw__" 2>/dev/null)
    kh_decay_val=${kh_decay_val:-100}
    if [ "$kh_decay_val" -lt 100 ] 2>/dev/null; then
        pass "Autotune decays keyword_hits (100 -> $kh_decay_val)"
    else
        fail "Autotune decays keyword_hits" "expected < 100, got $kh_decay_val [F5]"
    fi
    redis_cli HDEL "aoa:${PROJECT_ID}:keyword_hits" "__test_decay_kw__" > /dev/null 2>&1

    # 11.9 [F5]: term_hits decay
    redis_cli HSET "aoa:${PROJECT_ID}:term_hits" "__test_decay_term__" "100" > /dev/null 2>&1
    api_post "/domains/tune/math" "{\"project_id\":\"${PROJECT_ID}\"}" > /dev/null 2>&1
    local th_decay_val
    th_decay_val=$(redis_cli HGET "aoa:${PROJECT_ID}:term_hits" "__test_decay_term__" 2>/dev/null)
    th_decay_val=${th_decay_val:-100}
    if [ "$th_decay_val" -lt 100 ] 2>/dev/null; then
        pass "Autotune decays term_hits (100 -> $th_decay_val)"
    else
        fail "Autotune decays term_hits" "expected < 100, got $th_decay_val [F5]"
    fi
    redis_cli HDEL "aoa:${PROJECT_ID}:term_hits" "__test_decay_term__" > /dev/null 2>&1

    # 11.10 [REGRESSION]: Domain hits decay still works
    # Use known domain_name from setup (avoids KEYS formatting issues)
    local decay_meta_key="aoa:${PROJECT_ID}:domain:${domain_name}:meta"
    redis_cli HSET "$decay_meta_key" "hits" "50" > /dev/null 2>&1
    local tune_response
    tune_response=$(api_post "/domains/tune/math" "{\"project_id\":\"${PROJECT_ID}\"}")
    local dh_decay_val
    dh_decay_val=$(redis_cli HGET "$decay_meta_key" "hits" 2>/dev/null)
    dh_decay_val=$(echo "${dh_decay_val:-50}" | awk '{printf "%d", $1}')
    if [ "$dh_decay_val" -lt 50 ] 2>/dev/null; then
        pass "Domain hits decay regression (50 -> $dh_decay_val)"
    else
        # Show tune response for debugging
        local tune_err
        tune_err=$(echo "$tune_response" | jq -r '.error // empty' 2>/dev/null)
        if [ -n "$tune_err" ]; then
            fail "Domain hits decay regression" "autotune error: $tune_err"
        else
            fail "Domain hits decay regression" "expected < 50, got $dh_decay_val"
        fi
    fi

    # ---- 11.D: Noise Filter (F6) ----

    # 11.11 [F6]: Keywords > threshold get blocklisted
    redis_cli HSET "aoa:${PROJECT_ID}:keyword_hits" "__test_noise__" "1001" > /dev/null 2>&1
    api_post "/domains/tune/math" "{\"project_id\":\"${PROJECT_ID}\"}" > /dev/null 2>&1
    local is_blocked
    is_blocked=$(redis_cli SISMEMBER "aoa:${PROJECT_ID}:keyword_blocklist" "__test_noise__" 2>/dev/null)
    if [ "$is_blocked" = "1" ]; then
        pass "Autotune blocklists noisy keyword (>1000 hits)"
    else
        fail "Autotune blocklists noisy keyword" "expected in keyword_blocklist [F6]"
    fi

    # 11.12 [F6]: Blocklisted keyword removed from keyword_hits
    local noise_still_exists
    noise_still_exists=$(redis_cli HEXISTS "aoa:${PROJECT_ID}:keyword_hits" "__test_noise__" 2>/dev/null)
    if [ "$noise_still_exists" = "0" ]; then
        pass "Blocklisted keyword removed from keyword_hits"
    else
        fail "Blocklisted keyword removed from keyword_hits" "still present [F6]"
    fi

    # Cleanup F6 test data
    redis_cli HDEL "aoa:${PROJECT_ID}:keyword_hits" "__test_noise__" > /dev/null 2>&1
    redis_cli SREM "aoa:${PROJECT_ID}:keyword_blocklist" "__test_noise__" > /dev/null 2>&1

    # ---- 11.E: Negative Tests (Future State Boundaries) ----

    # 11.13: Submit-tags does NOT increment term_hits (domain-only per future state)
    local neg_th_before neg_th_after
    neg_th_before=$(redis_cli HGET "aoa:${PROJECT_ID}:term_hits" "__nonexistent_term__" 2>/dev/null)
    neg_th_before=${neg_th_before:-0}
    api_post "/domains/submit-tags" "{\"project_id\":\"${PROJECT_ID}\",\"tags\":[\"__nonexistent_term__\"]}" > /dev/null 2>&1
    neg_th_after=$(redis_cli HGET "aoa:${PROJECT_ID}:term_hits" "__nonexistent_term__" 2>/dev/null)
    neg_th_after=${neg_th_after:-0}
    if [ "$neg_th_after" = "$neg_th_before" ]; then
        pass "Submit-tags does not increment term_hits (correct boundary)"
    else
        fail "Submit-tags incremented term_hits" "should be domain-only path"
    fi

    # 11.14: Submit-tags does NOT increment cohit:kw_term
    local neg_cohit_before neg_cohit_after
    neg_cohit_before=$(redis_cli HLEN "aoa:${PROJECT_ID}:cohit:kw_term" 2>/dev/null)
    neg_cohit_before=${neg_cohit_before:-0}
    api_post "/domains/submit-tags" "{\"project_id\":\"${PROJECT_ID}\",\"tags\":[\"__nonexistent_term__\"]}" > /dev/null 2>&1
    neg_cohit_after=$(redis_cli HLEN "aoa:${PROJECT_ID}:cohit:kw_term" 2>/dev/null)
    neg_cohit_after=${neg_cohit_after:-0}
    if [ "$neg_cohit_after" = "$neg_cohit_before" ]; then
        pass "Submit-tags does not increment cohit:kw_term (correct boundary)"
    else
        fail "Submit-tags incremented cohit:kw_term" "should be domain-only path"
    fi
}

# =============================================================================
# MAIN
# =============================================================================

main() {
    echo -e "${CYAN}${BOLD}"
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║          aOa End-to-End Integration Test Suite                  ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    preflight

    local sections=(
        test_service_health
        test_core_search_api
        test_file_operations_api
        test_cli_commands
        test_intent_tracking
        test_domain_management
        test_configuration
        test_bigrams
        test_domain_lifecycle
        test_metrics
        test_observer_pattern
    )

    for i in "${!sections[@]}"; do
        local num=$((i + 1))
        if [ -n "$ONLY_SECTION" ] && [ "$ONLY_SECTION" != "$num" ]; then
            continue
        fi
        ${sections[$i]}
    done

    summary
    exit $?
}

main "$@"
