# =============================================================================
# SECTION 60: System & Diagnostics
# =============================================================================
#
# PURPOSE
#   System management, health checks, and diagnostic tools. Used for
#   troubleshooting, monitoring, and understanding aOa state.
#
# DEPENDENCIES
#   - 01-constants.sh: INDEX_URL, STATUS_URL, colors
#   - 02-utils.sh: get_project_id(), get_project_root()
#
# COMMANDS PROVIDED
#   cmd_health      Check service health
#   cmd_info        Show project/index info
#   cmd_services    Show running services
#   cmd_memory      Memory usage stats
#   cmd_stats       Index statistics
#   cmd_baseline    Token baseline calculations
#   cmd_metrics     Performance metrics
#   cmd_history     Command history
#   cmd_reset       Reset index state
#   cmd_whitelist   Manage file whitelist
#   cmd_quickstart  Interactive setup wizard
#   cmd_learn       Discover new patterns
#
# =============================================================================

# =============================================================================
# Utility Commands
# =============================================================================

cmd_info() {
    echo -e "${CYAN}${BOLD}⚡ aOa Indexing Configuration${NC}"
    echo ""

    # Show aOa home
    echo -e "${BOLD}aOa Installation:${NC}"
    echo -e "  Home: ${AOA_HOME}"
    echo -e "  Data: ${AOA_DATA}"
    echo ""

    # Read from .env file if it exists (root directory, where docker-compose reads it)
    local env_file="${AOA_HOME}/.env"
    local projects_root="${HOME}"
    local gateway_port="8080"

    if [ -f "$env_file" ]; then
        projects_root=$(grep "^PROJECTS_ROOT=" "$env_file" 2>/dev/null | cut -d'=' -f2 || echo "$HOME")
        gateway_port=$(grep "^GATEWAY_PORT=" "$env_file" 2>/dev/null | cut -d'=' -f2 || echo "8080")
    fi

    # Show Docker configuration from .env
    echo -e "${BOLD}Docker Configuration:${NC} ${DIM}(from .env)${NC}"
    echo -e "  PROJECTS_ROOT:   ${projects_root} → /userhome"
    echo -e "  GATEWAY_PORT:    ${gateway_port}"
    echo -e "  Claude sessions: ${projects_root}/.claude ${DIM}(auto-derived)${NC}"
    echo ""
    echo -e "  ${DIM}Edit .env in aOa root to change, then restart Docker${NC}"
    echo ""

    # Show registered projects
    echo -e "${BOLD}Registered Projects:${NC}"
    local projects_file="${AOA_DATA}/projects.json"
    if [ -f "$projects_file" ] && [ "$(jq 'length' "$projects_file" 2>/dev/null)" != "0" ]; then
        jq -r '.[] | "  [\(.id | .[0:8])] \(.name) → \(.path)"' "$projects_file" 2>/dev/null
    else
        echo -e "  ${DIM}(none - run 'aoa init' in a project)${NC}"
    fi
    echo ""

    # Show current project context
    local project_root=$(get_project_root)
    local project_id=$(get_project_id)
    if [ -n "$project_root" ]; then
        echo -e "${BOLD}Current Project:${NC}"
        echo -e "  Root: ${project_root}"
        echo -e "  ID:   ${project_id:-not initialized}"

        # Check if initialized
        if [ -z "$project_id" ]; then
            echo -e "  ${YELLOW}→ Run 'aoa init' to enable aOa for this project${NC}"
        fi
    else
        echo -e "${BOLD}Current Project:${NC}"
        echo -e "  ${DIM}Not in a git repository${NC}"
    fi
    echo ""

    # Show what gets indexed
    echo -e "${BOLD}What Gets Indexed:${NC}"
    echo -e "  ✓ Files in registered project roots"
    echo -e "  ✓ Knowledge repos (repos/ directory)"
    echo -e "  ✓ Claude session history (~/.claude)"
    echo ""
    echo -e "${BOLD}What Is Skipped:${NC}"
    echo -e "  ✗ node_modules, .git, __pycache__, dist, build, etc."
    echo -e "  ✗ Files outside registered projects"
    echo -e "  ✗ Unrecognized file extensions"
    echo ""
    echo -e "${DIM}See: .aoa/config.json for full configuration${NC}"
}

cmd_services() {
    echo -e "${CYAN}${BOLD}"
    cat << 'EOF'
╔══════════════════════════════════════════════════════════════════════╗
║                         aOa Attack Map                               ║
╠══════════════════════════════════════════════════════════════════════╣
║                                                                      ║
║  ┌─────────────────────────────────────────────────────────────┐    ║
║  │                     GATEWAY (:8080)                         │    ║
║  │              Single entry point for all angles              │    ║
║  └─────────────────────────────────────────────────────────────┘    ║
║                              │                                       ║
║          ┌───────────────────┼───────────────────┐                  ║
║          ▼                   ▼                   ▼                  ║
║  ┌───────────────┐   ┌───────────────┐   ┌───────────────┐         ║
║  │    INDEX      │   │    STATUS     │   │   GIT-PROXY   │         ║
║  │    :9999      │   │    :9998      │   │    :9997      │         ║
║  │               │   │               │   │               │         ║
║  │ • Symbol      │   │ • Sessions    │   │ • Clone repos │         ║
║  │ • Ranking     │   │ • History     │   │ • Allowlist   │         ║
║  │ • Intent      │   │ • Metrics     │   │               │         ║
║  │ • Memory      │   │               │   │               │         ║
║  │ • Tuner       │   │               │   │               │         ║
║  └───────────────┘   └───────────────┘   └───────────────┘         ║
║          │                                                          ║
║          ▼                                                          ║
║  ┌───────────────┐                                                  ║
║  │    REDIS      │                                                  ║
║  │    :6379      │                                                  ║
║  │               │                                                  ║
║  │ • Scores      │                                                  ║
║  │ • Transitions │                                                  ║
║  │ • Predictions │                                                  ║
║  └───────────────┘                                                  ║
║                                                                      ║
╠══════════════════════════════════════════════════════════════════════╣
║  THE FIVE ANGLES                                                     ║
╠══════════════════════════════════════════════════════════════════════╣
║                                                                      ║
║  ⚡ SYMBOL         O(1) symbol lookup across codebase                ║
║     aoa search <term>                                                ║
║                                                                      ║
║  🎯 INTENT         Track tool calls, extract behavior patterns       ║
║     aoa intent recent                                                ║
║                                                                      ║
║  🧠 STRIKE         Predictive context, dynamic working memory        ║
║     aoa context "fix auth bug"                                       ║
║                                                                      ║
║  📊 SIGNAL         Multi-term ranking, pattern matching              ║
║     aoa multi auth,session                                           ║
║                                                                      ║
║  📁 INTEL          External reference repos, isolated search         ║
║     aoa repo <name> search <term>                                    ║
║                                                                      ║
╚══════════════════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"

    # Show live stats
    echo -e "${BOLD}Live Status${NC}"
    echo ""

    # Health check
    local index_ok=false
    local status_ok=false
    local redis_ok=false

    curl -s --connect-timeout 1 "http://localhost:8080/health" > /dev/null 2>&1 && index_ok=true
    curl -s --connect-timeout 1 "http://localhost:8080/status" > /dev/null 2>&1 && status_ok=true

    # Check Redis (works in both unified and compose modes)
    if docker exec aoa redis-cli ping > /dev/null 2>&1; then
        redis_ok=true
    elif docker exec aoa-redis-1 redis-cli ping > /dev/null 2>&1; then
        redis_ok=true
    fi

    if $index_ok; then
        echo -e "  Index:  ${GREEN}✓${NC} Running"
    else
        echo -e "  Index:  ${RED}✗${NC} Not responding"
    fi

    if $status_ok; then
        echo -e "  Status: ${GREEN}✓${NC} Running"
    else
        echo -e "  Status: ${RED}✗${NC} Not responding"
    fi

    if $redis_ok; then
        echo -e "  Redis:  ${GREEN}✓${NC} Connected"
    else
        echo -e "  Redis:  ${RED}✗${NC} Not connected"
    fi

    echo ""

    # Quick stats
    local memory_result=$(curl -s "http://localhost:8080/memory?format=compact" 2>/dev/null)
    if [ -n "$memory_result" ]; then
        local files=$(echo "$memory_result" | jq -r '.files_analyzed' 2>/dev/null)
        local ms=$(echo "$memory_result" | jq -r '.ms' 2>/dev/null)
        echo -e "  Memory: ${files} active files, ${GREEN}${ms}ms${NC} latency"
    fi

    local health_result=$(curl -s "http://localhost:8080/health" 2>/dev/null)
    if [ -n "$health_result" ]; then
        local symbols=$(echo "$health_result" | jq -r '.local.symbols' 2>/dev/null)
        local idx_files=$(echo "$health_result" | jq -r '.local.files' 2>/dev/null)
        echo -e "  Index:  ${idx_files} files, ${symbols} targets"
    fi
}

cmd_memory() {
    local format="${1:-prose}"

    case "$format" in
        -c|--compact|compact)
            format="compact"
            ;;
        -s|--structured|structured|json)
            format="structured"
            ;;
        -p|--prose|prose|*)
            format="prose"
            ;;
    esac

    local result=$(curl -s "http://localhost:8080/memory?format=${format}")

    if [ "$format" = "structured" ]; then
        echo "$result" | jq .
    else
        local memory=$(echo "$result" | jq -r '.memory')
        local ms=$(echo "$result" | jq -r '.ms')
        local files=$(echo "$result" | jq -r '.files_analyzed')

        echo -e "${CYAN}${BOLD}⚡ aOa Working Memory${NC} ${DIM}│${NC} ${files} files ${DIM}│${NC} ${GREEN}${ms}ms${NC}"
        echo ""
        echo "$memory"
    fi
}

cmd_health() {
    local project_root=$(get_project_root)
    local all_ok=true
    local warnings=0

    echo -e "${BOLD}aOa Health Check${NC}"
    echo -e "────────────────────────────────────────"
    echo ""

    # =========================================================================
    # SERVICES
    # =========================================================================
    echo -e "${BOLD}Services${NC}"

    # Check Docker
    echo -n "  Docker:        "
    if docker ps --filter "name=aoa" --format "{{.Names}}" 2>/dev/null | grep -q "aoa"; then
        echo -e "${GREEN}✓${NC} Container running"
    else
        echo -e "${RED}✗${NC} Container not found"
        all_ok=false
    fi

    # Check Index service
    echo -n "  Index:         "
    local idx_health=""
    if curl -s --connect-timeout 2 "${INDEX_URL}/health" > /dev/null 2>&1; then
        idx_health=$(curl -s "${INDEX_URL}/health")
        local mode=$(echo "$idx_health" | jq -r '.mode // "legacy"')
        if [ "$mode" = "global" ]; then
            local project_count=$(echo "$idx_health" | jq '.projects | length // 0')
            echo -e "${GREEN}✓${NC} Running (${project_count} project(s))"
        else
            local local_files=$(echo "$idx_health" | jq -r '.local.files // 0')
            local local_symbols=$(echo "$idx_health" | jq -r '.local.symbols // 0')
            echo -e "${GREEN}✓${NC} ${local_files} files, ${local_symbols} targets"
        fi
    else
        echo -e "${RED}✗${NC} Not responding"
        all_ok=false
    fi

    # Check Redis
    echo -n "  Redis:         "
    if docker exec aoa redis-cli ping > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Connected"
    elif docker exec aoa-redis-1 redis-cli ping > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Connected"
    else
        echo -e "${YELLOW}!${NC} Not connected ${DIM}(predictions disabled)${NC}"
        warnings=$((warnings + 1))
    fi

    echo ""

    # =========================================================================
    # PROJECT CONFIGURATION
    # =========================================================================
    echo -e "${BOLD}Project Configuration${NC}"

    # Check if initialized
    echo -n "  Initialized:   "
    if [ -f "$project_root/.aoa/home.json" ]; then
        local project_id=$(jq -r '.project_id // "none"' "$project_root/.aoa/home.json" 2>/dev/null)
        echo -e "${GREEN}✓${NC} ${DIM}${project_id:0:8}...${NC}"
    else
        echo -e "${RED}✗${NC} Not initialized ${DIM}(run 'aoa init')${NC}"
        all_ok=false
    fi

    # Check hooks (essential: intent-capture + status-line)
    echo -n "  Hooks:         "
    local hook_count=0
    [ -f "$project_root/.claude/hooks/aoa-intent-capture.py" ] && hook_count=$((hook_count + 1))
    [ -f "$project_root/.claude/hooks/aoa-status-line.sh" ] && hook_count=$((hook_count + 1))

    if [ "$hook_count" -eq 2 ]; then
        echo -e "${GREEN}✓${NC} Essential hooks installed"
    elif [ "$hook_count" -gt 0 ]; then
        echo -e "${YELLOW}!${NC} ${hook_count}/2 essential hooks ${DIM}(partial)${NC}"
        warnings=$((warnings + 1))
    else
        echo -e "${RED}✗${NC} No hooks found"
        all_ok=false
    fi

    # Check CLAUDE.md
    echo -n "  CLAUDE.md:     "
    if [ -f "$project_root/CLAUDE.md" ]; then
        if grep -q "aoa search" "$project_root/CLAUDE.md" 2>/dev/null; then
            echo -e "${GREEN}✓${NC} Present with aOa instructions"
        else
            echo -e "${YELLOW}!${NC} Present ${DIM}(missing aOa instructions)${NC}"
            warnings=$((warnings + 1))
        fi
    else
        echo -e "${YELLOW}!${NC} Not found ${DIM}(optional)${NC}"
        warnings=$((warnings + 1))
    fi

    echo ""

    # =========================================================================
    # FUNCTIONALITY
    # =========================================================================
    echo -e "${BOLD}Functionality${NC}"

    # Test search
    echo -n "  Search:        "
    local search_result=$(curl -s --connect-timeout 2 "${INDEX_URL}/symbol?q=test" 2>/dev/null)
    if [ -n "$search_result" ]; then
        local ms=$(echo "$search_result" | jq -r '.ms // "?"')
        echo -e "${GREEN}✓${NC} Working ${DIM}(${ms}ms)${NC}"
    else
        echo -e "${RED}✗${NC} Not working"
        all_ok=false
    fi

    # Check intent capture
    echo -n "  Intent:        "
    local intent_result=$(curl -s --connect-timeout 2 "${INDEX_URL}/intent/recent?limit=1" 2>/dev/null)
    if [ -n "$intent_result" ]; then
        local total=$(echo "$intent_result" | jq -r '.stats.total_records // 0')
        local tags=$(echo "$intent_result" | jq -r '.stats.unique_tags // 0')
        echo -e "${GREEN}✓${NC} ${total} recorded, ${tags} tags"
    else
        echo -e "${YELLOW}!${NC} No data ${DIM}(fresh install)${NC}"
        warnings=$((warnings + 1))
    fi

    # Check semantic compression (outline angle)
    echo -n "  Outline:       "
    if docker exec aoa python3 -c "import tree_sitter" > /dev/null 2>&1 || \
       docker exec aoa-index-1 python3 -c "import tree_sitter" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Semantic compression ready"
    else
        echo -e "${YELLOW}!${NC} Semantic compression unavailable"
        warnings=$((warnings + 1))
    fi

    echo ""

    # =========================================================================
    # SUMMARY
    # =========================================================================
    echo -e "────────────────────────────────────────"
    if $all_ok && [ "$warnings" -eq 0 ]; then
        echo -e "Status: ${GREEN}✓ All systems operational${NC}"
    elif $all_ok; then
        echo -e "Status: ${YELLOW}! Operational with ${warnings} warning(s)${NC}"
    else
        echo -e "Status: ${RED}✗ Issues detected${NC}"
        echo -e "${DIM}Run 'aoa init' to configure this project${NC}"
    fi
}

# Infer tags from symbol name (free pattern-based tagging)
infer_tags_from_name() {
    local name="$1"
    local kind="$2"
    local tags=()

    # Convert camelCase/PascalCase to words
    local words=$(echo "$name" | sed 's/\([a-z]\)\([A-Z]\)/\1 \2/g' | sed 's/_/ /g' | tr '[:upper:]' '[:lower:]')

    # Common action verbs → tags
    [[ "$words" =~ ^(get|fetch|load|read) ]] && tags+=("#read")
    [[ "$words" =~ ^(set|save|write|store|update|put) ]] && tags+=("#write")
    [[ "$words" =~ ^(delete|remove|clear) ]] && tags+=("#delete")
    [[ "$words" =~ ^(create|add|insert|new|make|build) ]] && tags+=("#create")
    [[ "$words" =~ ^(handle|process|on) ]] && tags+=("#handler")
    [[ "$words" =~ ^(validate|check|verify|is|has|can) ]] && tags+=("#validation")
    [[ "$words" =~ ^(parse|extract|convert|transform) ]] && tags+=("#transform")
    [[ "$words" =~ ^(init|setup|configure|start|boot) ]] && tags+=("#init")
    [[ "$words" =~ ^test ]] && tags+=("#test")

    # Domain keywords → tags
    [[ "$words" =~ auth ]] && tags+=("#auth")
    [[ "$words" =~ user ]] && tags+=("#user")
    [[ "$words" =~ login|logout|session ]] && tags+=("#session")
    [[ "$words" =~ token|jwt|oauth ]] && tags+=("#token")
    [[ "$words" =~ api|endpoint|route ]] && tags+=("#api")
    [[ "$words" =~ database|db|sql|query ]] && tags+=("#database")
    [[ "$words" =~ cache|redis ]] && tags+=("#cache")
    [[ "$words" =~ file|path|dir ]] && tags+=("#filesystem")
    [[ "$words" =~ config|setting|option ]] && tags+=("#config")
    [[ "$words" =~ error|exception|fail ]] && tags+=("#error")
    [[ "$words" =~ log|debug|trace ]] && tags+=("#logging")
    [[ "$words" =~ http|request|response ]] && tags+=("#http")
    [[ "$words" =~ json|xml|yaml ]] && tags+=("#serialization")
    [[ "$words" =~ encrypt|decrypt|hash|secret ]] && tags+=("#security")
    [[ "$words" =~ search|find|filter|sort ]] && tags+=("#search")
    [[ "$words" =~ render|display|view|template ]] && tags+=("#render")
    [[ "$words" =~ email|mail|send|notify ]] && tags+=("#notification")
    [[ "$words" =~ queue|job|task|worker ]] && tags+=("#async")
    [[ "$words" =~ metric|stat|count|measure ]] && tags+=("#metrics")
    [[ "$words" =~ index|symbol|outline ]] && tags+=("#index")

    # Kind-based tags
    [[ "$kind" == "class" ]] && tags+=("#class")
    [[ "$kind" == "function" && "$words" =~ service$ ]] && tags+=("#service")
    [[ "$kind" == "function" && "$words" =~ manager$ ]] && tags+=("#manager")
    [[ "$kind" == "function" && "$words" =~ helper$ ]] && tags+=("#utility")
    [[ "$kind" == "function" && "$words" =~ util$ ]] && tags+=("#utility")

    # Return unique tags as JSON array (max 5)
    if [ ${#tags[@]} -gt 0 ]; then
        printf '%s\n' "${tags[@]}" | sort -u | head -5 | jq -R . | jq -s .
    else
        echo "[]"
    fi
}

# Helper function for parallel quickstart processing (must be top-level for export -f)
_quickstart_process_file() {
    local file="$1"
    local loop_param=""
    [ -n "$project_id" ] && loop_param="&project=${project_id}"

    # Get outline
    local result=$(curl -s "${INDEX_URL}/outline?file=${file}${loop_param}")
    local err=$(echo "$result" | jq -r '.error // empty')

    if [ -n "$err" ]; then
        touch "${progress_dir}/error_$(echo "$file" | md5sum | cut -c1-8)"
        return 1
    fi

    local symbols_json=$(echo "$result" | jq -c '[.symbols[] | {name: .name, kind: .kind, start_line: .start_line, end_line: .end_line}]')

    # Infer tags
    local infer_payload=$(echo "$symbols_json" | jq '{symbols: [.[] | {name: .name, kind: .kind}]}')
    local infer_result=$(curl -s -X POST "${INDEX_URL}/patterns/infer" \
        -H "Content-Type: application/json" \
        -d "$infer_payload")
    local inferred_tags=$(echo "$infer_result" | jq -c '.tags // []')

    # Build enriched payload using jq (simpler, faster)
    local enriched_symbols=$(echo "$result" | jq -c --argjson tags "$inferred_tags" '
        [.symbols | to_entries | .[] | {
            name: .value.name,
            kind: .value.kind,
            line: .value.start_line,
            end_line: .value.end_line,
            tags: ($tags[.key] // [])
        }]')

    local mark_payload=$(jq -n \
        --arg file "$file" \
        --arg project "$project_id" \
        --argjson symbols "$enriched_symbols" \
        '{file: $file, project: $project, symbols: $symbols}')

    local http_code=$(curl -s -w "%{http_code}" -o /dev/null -X POST "${INDEX_URL}/outline/enriched" \
        -H "Content-Type: application/json" \
        -d "$mark_payload")

    if [ "$http_code" = "200" ]; then
        touch "${progress_dir}/done_$(echo "$file" | md5sum | cut -c1-8)"
    else
        touch "${progress_dir}/error_$(echo "$file" | md5sum | cut -c1-8)"
    fi
}

cmd_quickstart() {
    # aoa quickstart           - Initial tagging (only pending files)
    # aoa quickstart --force   - Re-tag ALL files (refresh patterns)
    # aoa quickstart --reset   - Clear enrichment data and re-tag

    local force=false
    local reset=false

    # Parse flags
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --force|-f) force=true; shift ;;
            --reset|-r) reset=true; force=true; shift ;;
            *) shift ;;
        esac
    done

    # Check services first
    if ! curl -s --connect-timeout 2 "${INDEX_URL}/health" > /dev/null 2>&1; then
        echo -e "${RED}✗ aOa services not running${NC}"
        echo -e "${DIM}Start with: docker start aoa${NC}"
        return 1
    fi

    # Get project info
    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="?project=${project_id}"
    fi

    # Handle --reset: Clear all enrichment data for this project
    if $reset; then
        echo -e "${YELLOW}Clearing enrichment data...${NC}"
        local redis_container=$(docker ps --format "{{.Names}}" | grep -E "redis" | head -1)
        if [ -n "$redis_container" ]; then
            local pattern="enriched:${project_id:-default}:*"
            local keys=$(docker exec "$redis_container" redis-cli KEYS "$pattern" 2>/dev/null)
            if [ -n "$keys" ]; then
                echo "$keys" | while read key; do
                    docker exec "$redis_container" redis-cli DEL "$key" > /dev/null 2>&1
                done
                local count=$(echo "$keys" | wc -l)
                echo -e "${GREEN}✓${NC} Cleared $count enrichment records"
            else
                echo -e "${DIM}No enrichment data found${NC}"
            fi
        fi
        echo ""
    fi

    # Seed universal domains (GL-053) - first time only
    local seed_result=$(curl -s -X POST "${INDEX_URL}/domains/seed" \
        -H "Content-Type: application/json" \
        -d "{\"project\": \"${project_id:-default}\"}" 2>/dev/null)
    local seed_domains=$(echo "$seed_result" | jq -r '.domains // 0')
    local seed_terms=$(echo "$seed_result" | jq -r '.terms // 0')
    local seed_msg=$(echo "$seed_result" | jq -r '.message // ""')

    if [ "$seed_domains" -gt 0 ] && [ "$seed_msg" != "Already seeded" ]; then
        echo -e "${GREEN}✓${NC} Seeded ${BOLD}${seed_domains}${NC} domains (${seed_terms} terms)"
        echo ""
    fi

    # Get pending files for semantic compression
    local pending_result
    if $force; then
        pending_result=$(curl -s "${INDEX_URL}/files${project_param}")
        local all_files=$(echo "$pending_result" | jq -r '.files // []')
        pending_result=$(echo "$all_files" | jq '{pending: [.[] | {file: .}], pending_count: length, up_to_date_count: 0}')
    else
        pending_result=$(curl -s "${INDEX_URL}/outline/pending${project_param}")
    fi

    local pending_count=$(echo "$pending_result" | jq -r '.pending_count // 0')
    local up_to_date=$(echo "$pending_result" | jq -r '.up_to_date_count // 0')

    # Calculate workers: 1 per 50 files, min 4, max 10
    local workers=$((pending_count / 50))
    [ $workers -lt 4 ] && workers=4
    [ $workers -gt 10 ] && workers=10

    echo -e "${CYAN}${BOLD}⚡ aOa Quickstart${NC}"
    $force && echo -e "${YELLOW}(force mode)${NC}"
    echo ""

    if [ "$pending_count" -eq 0 ]; then
        echo -e "${GREEN}✓${NC} All ${up_to_date} files are compressed"
        echo ""
        echo -e "${DIM}To refresh: aoa quickstart --force${NC}"
        return 0
    fi

    echo -e "  Processing ${BOLD}${pending_count}${NC} files (${workers} workers)"
    echo ""

    # Create temp directory to track progress
    local progress_dir=$(mktemp -d)
    local files_list="${progress_dir}/files.txt"
    echo "$pending_result" | jq -r '.pending[].file' > "$files_list"

    # Export variables for parallel processing
    export INDEX_URL
    export project_id
    export progress_dir
    export -f _quickstart_process_file

    # Start parallel processing in background
    cat "$files_list" | xargs -P "$workers" -I {} bash -c '_quickstart_process_file "$@"' _ {} &
    local xargs_pid=$!

    # Progress bar settings
    local bar_width=40

    # Worker stage tracking
    local -a tasks=("Parsing" "Extracting" "Indexing" "Mapping" "Compressing")
    local -a thresholds=()
    for ((w=1; w<=workers; w++)); do
        thresholds+=($((w * 100 / workers)))
    done
    local printed_stages=0
    local need_fresh_line=false

    # Show progress while processing
    while kill -0 $xargs_pid 2>/dev/null; do
        local done_count=$(ls -1 "${progress_dir}"/done_* 2>/dev/null | wc -l)
        local error_count=$(ls -1 "${progress_dir}"/error_* 2>/dev/null | wc -l)
        local processed=$((done_count + error_count))

        if [ $pending_count -gt 0 ]; then
            local pct=$((processed * 100 / pending_count))
            local filled=$((pct * bar_width / 100))
            local empty=$((bar_width - filled))

            local bar=""
            for ((i=0; i<filled; i++)); do bar+="█"; done
            for ((i=0; i<empty; i++)); do bar+="░"; done

            # Update progress bar
            if $need_fresh_line; then
                printf "  [${GREEN}%s${NC}] %3d%% │ %d/%d" "$bar" "$pct" "$processed" "$pending_count"
                need_fresh_line=false
            else
                printf "\r  [${GREEN}%s${NC}] %3d%% │ %d/%d" "$bar" "$pct" "$processed" "$pending_count"
            fi

            # Check if any new worker stages completed
            while [ $printed_stages -lt $workers ] && [ $pct -ge ${thresholds[$printed_stages]} ]; do
                local task_idx=$((printed_stages % 5))
                echo ""
                echo -e "    ${GREEN}✓${NC} Worker $((printed_stages + 1)): ${tasks[$task_idx]}"
                printed_stages=$((printed_stages + 1))
                need_fresh_line=true
            done
        fi

        sleep 0.1
    done

    # Wait for completion and get final counts
    wait $xargs_pid 2>/dev/null
    local done_count=$(ls -1 "${progress_dir}"/done_* 2>/dev/null | wc -l)
    local error_count=$(ls -1 "${progress_dir}"/error_* 2>/dev/null | wc -l)

    # Final progress update
    if $need_fresh_line; then
        printf "  [${GREEN}"
    else
        printf "\r  [${GREEN}"
    fi
    for ((i=0; i<bar_width; i++)); do printf "█"; done
    printf "${NC}] 100%% │ %d/%d\n" "$pending_count" "$pending_count"

    # Print any remaining worker stages
    while [ $printed_stages -lt $workers ]; do
        local task_idx=$((printed_stages % 5))
        echo -e "    ${GREEN}✓${NC} Worker $((printed_stages + 1)): ${tasks[$task_idx]}"
        printed_stages=$((printed_stages + 1))
    done

    # Cleanup
    rm -rf "$progress_dir"

    echo ""
    echo ""

    # =========================================================================
    # VALUE PROPOSITION
    # =========================================================================

    echo -e "${GREEN}✓${NC} ${BOLD}${done_count} files${NC} semantically compressed"
    echo ""

    if [ "$error_count" -gt 0 ]; then
        echo -e "  ${YELLOW}!${NC} ${error_count} files had errors"
        echo ""
    fi

    echo -e "  ${DIM}Before:${NC} grep 'handleAuth' → 12 matches → read 8 files → find method"
    echo -e "  ${DIM}Cost: 50,000 tokens, 30 seconds${NC}"
    echo ""

    echo -e "  ${CYAN}With aOa:${NC}"
    echo -e "    aoa grep handleAuth"
    echo ""
    echo -e "    ${GREEN}auth/service.py${NC}:AuthService().${GREEN}handleAuth${NC}(request)[${CYAN}47-89${NC}]  ${DIM}#auth #validation${NC}"
    echo ""
    echo -e "    Class. Method. Line range. Tags."
    echo -e "    AI reads ${BOLD}42 lines${NC}, not 8 files."
    echo -e "    ${GREEN}Cost: 800 tokens, 5ms${NC}"
    echo ""

    echo -e "  ${BOLD}Savings: 98% fewer tokens. Minutes → milliseconds.${NC}"
    echo ""

    echo -e "${DIM}Ready. Try: aoa grep <anything>${NC}"
}

cmd_learn() {
    # aoa learn          - Show domain candidates and prompt for tag generation
    # aoa learn --store  - Store learned patterns (from stdin JSON)
    # aoa learn --show   - Show stored learned patterns

    local project_id=$(get_project_id)
    local project_param=""
    [ -n "$project_id" ] && project_param="?project=${project_id}"

    # Check for flags
    case "$1" in
        --store)
            # Read JSON from stdin and store as learned patterns
            local input=$(cat)
            if [ -z "$input" ]; then
                echo "Error: No input provided. Expected JSON: {\"patterns\": {\"keyword\": \"#tag\", ...}}"
                return 1
            fi

            local result=$(echo "$input" | curl -s -X POST "${INDEX_URL}/patterns/learned${project_param}" \
                -H "Content-Type: application/json" \
                -d @-)

            local stored=$(echo "$result" | jq -r '.stored // 0')
            if [ "$stored" -gt 0 ]; then
                echo -e "${GREEN}✓${NC} Stored ${stored} learned patterns"
            else
                echo -e "${YELLOW}!${NC} No patterns stored"
            fi
            return 0
            ;;

        --show)
            # Show stored learned patterns
            local result=$(curl -s "${INDEX_URL}/patterns/learned${project_param}")
            local count=$(echo "$result" | jq -r '.patterns | length')

            echo -e "${CYAN}${BOLD}⚡ Learned Patterns${NC}"
            echo ""

            if [ "$count" -eq 0 ]; then
                echo -e "${DIM}No learned patterns stored yet.${NC}"
                echo -e "${DIM}Run 'aoa quickstart' then 'aoa learn' to generate.${NC}"
            else
                echo -e "${BOLD}${count}${NC} patterns:"
                echo ""
                echo "$result" | jq -r '.patterns | to_entries[] | "  \(.key) → \(.value)"'
            fi
            return 0
            ;;
    esac

    # Default: Show candidates and prompt for tag generation
    echo -e "${CYAN}${BOLD}⚡ aOa Learn${NC}"
    echo ""

    # Get stored candidates
    local result=$(curl -s "${INDEX_URL}/patterns/candidates${project_param}")
    local candidates=$(echo "$result" | jq -r '.candidates')
    local count=$(echo "$candidates" | jq 'length')
    local suggested=$(echo "$result" | jq -r '.suggested_domain // ""')
    local total_symbols=$(echo "$result" | jq -r '.total_symbols // 0')

    if [ "$count" -eq 0 ] || [ "$candidates" = "{}" ]; then
        echo -e "${DIM}No domain candidates found.${NC}"
        echo ""
        echo -e "Run ${CYAN}aoa quickstart${NC} first to scan your codebase."
        return 0
    fi

    echo -e "Found ${BOLD}${count}${NC} project-specific terms from ${total_symbols} symbols"
    if [ -n "$suggested" ] && [ "$suggested" != "" ]; then
        echo -e "Suggested domain: ${YELLOW}${suggested}${NC}"
    fi
    echo ""

    echo -e "${BOLD}Top candidates (word → frequency):${NC}"
    echo ""
    echo "$candidates" | jq -r 'to_entries | sort_by(-.value) | .[:20][] | "  \(.key): \(.value)"'
    echo ""

    echo -e "────────────────────────────────────────"
    echo -e "${BOLD}Generate Domain Tags${NC}"
    echo ""
    echo -e "To create project-specific tags, say to Claude:"
    echo ""
    echo -e "  ${CYAN}\"Generate domain tags for my codebase\"${NC}"
    echo ""
    echo -e "Claude will analyze these candidates and create"
    echo -e "keyword→tag mappings stored in Redis."
    echo ""
    echo -e "${DIM}Or manually: aoa learn --store < patterns.json${NC}"
}

cmd_domains() {
    # aoa domains - Show domain learning status and top domains with terms
    local project_id=$(get_project_id)
    local MAGENTA='\033[0;35m'

    # Get domain stats
    local stats=$(curl -s "${INDEX_URL}/domains/stats?project=${project_id}")

    local domain_count=$(echo "$stats" | jq -r '.domains // 0')
    local total_terms=$(echo "$stats" | jq -r '.total_terms // 0')
    local total_hits=$(echo "$stats" | jq -r '.total_hits // 0')
    local prompt_count=$(echo "$stats" | jq -r '.prompt_count // 0')
    local prompt_threshold=$(echo "$stats" | jq -r '.prompt_threshold // 10')
    local last_learn=$(echo "$stats" | jq -r '.last_learn // 0')
    local terms_learned=$(echo "$stats" | jq -r '.terms_learned_last // 0')
    local last_autotune=$(echo "$stats" | jq -r '.last_autotune // 0')
    local seconds_to_autotune=$(echo "$stats" | jq -r '.seconds_to_autotune // 0')
    local merged_last=$(echo "$stats" | jq -r '.autotune_merged_last // 0')
    local pruned_last=$(echo "$stats" | jq -r '.autotune_pruned_last // 0')
    # GL-054: Intelligence Angle
    local tokens_invested=$(echo "$stats" | jq -r '.tokens_invested // 0')
    local learning_calls=$(echo "$stats" | jq -r '.learning_calls // 0')

    # Get tokens saved from metrics endpoint
    local metrics=$(curl -s "${INDEX_URL}/metrics?project_id=${project_id}")
    local tokens_saved=$(echo "$metrics" | jq -r '.savings.tokens // 0')

    if [ "$domain_count" -eq 0 ] 2>/dev/null; then
        echo -e "${CYAN}${BOLD}⚡ aOa Domains${NC}"
        echo ""
        echo -e "${DIM}No domains seeded yet. Run 'aoa quickstart' to initialize.${NC}"
        return 0
    fi

    # Format hits compactly
    local hits_display
    if [ "$total_hits" -ge 1000 ]; then
        hits_display=$(awk "BEGIN {printf \"%.1fk\", $total_hits/1000}")
    else
        hits_display="$total_hits"
    fi

    # Format auto-tune time
    local tune_display=""
    if [ "$seconds_to_autotune" -gt 0 ] 2>/dev/null; then
        local hours=$((seconds_to_autotune / 3600))
        local mins=$(( (seconds_to_autotune % 3600) / 60 ))
        [ "$hours" -gt 0 ] && tune_display="${hours}h" || tune_display="${mins}m"
    else
        tune_display="ready"
    fi

    # Compact header: all key info on one line
    echo -e "${CYAN}${BOLD}⚡ aOa Domains${NC}  ${CYAN}${domain_count}${NC} domains ${DIM}│${NC} ${total_terms} terms ${DIM}│${NC} ${hits_display} hits ${DIM}│${NC} Learn: ${YELLOW}${prompt_count}/${prompt_threshold}${NC} ${DIM}│${NC} Tune: ${tune_display}"

    # Get domains with full terms for display
    local domains_data=$(curl -s "${INDEX_URL}/domains/list?project=${project_id}&limit=10&include_terms=true&include_created=true" 2>/dev/null)
    local now=$(date +%s)

    echo ""
    echo -e "${DIM}DOMAIN               HITS    TERMS${NC}"

    # Display each domain with its top terms
    echo "$domains_data" | jq -r '.domains[]? | "\(.name)|\(.hits)|\(.created // 0)|\(.terms // [] | .[0:6] | join(" "))"' 2>/dev/null | while IFS='|' read -r name hits created terms; do
        local hits_fmt
        [ "$hits" -ge 1000 ] && hits_fmt=$(awk "BEGIN {printf \"%.1fk\", $hits/1000}") || hits_fmt="$hits"

        local name_padded=$(printf "%-18s" "$name")
        local hits_padded=$(printf "%5s" "$hits_fmt")

        # Check if domain is new (created in last hour)
        local is_new=""
        if [ "$created" -gt 0 ] 2>/dev/null && [ $((now - created)) -lt 3600 ]; then
            is_new="${GREEN}+${NC}"
        fi

        echo -e "${is_new}${MAGENTA}${name_padded}${NC} ${DIM}${hits_padded}${NC}    ${CYAN}${terms}${NC}"
    done

    # ─────────────────────────────────────────────────────────────────────────
    # Recently Learned Section (GL-054)
    # ─────────────────────────────────────────────────────────────────────────
    echo ""
    echo -e "${DIM}───────────────────────────────────────────────────────────────────────────────────────${NC}"

    if [ "$last_learn" -gt 0 ] 2>/dev/null; then
        local learn_ago=$(( now - last_learn ))
        local learn_display=""
        if [ "$learn_ago" -lt 60 ]; then
            learn_display="${learn_ago}s ago"
        elif [ "$learn_ago" -lt 3600 ]; then
            learn_display="$((learn_ago / 60))m ago"
        elif [ "$learn_ago" -lt 86400 ]; then
            learn_display="$((learn_ago / 3600))h ago"
        else
            learn_display="$((learn_ago / 86400))d ago"
        fi

        # Get learned details
        local domains_learned=$(echo "$stats" | jq -r '.domains_learned_list // [] | join(" ")')
        local terms_learned_list=$(echo "$stats" | jq -r '.terms_learned_list // [] | .[0:4] | join(" ")')

        # Show Recently Learned section
        if [ -n "$domains_learned" ] && [ "$domains_learned" != "" ]; then
            echo -e "${CYAN}${BOLD}⚡ Recently Learned${NC} ${DIM}(${learn_display})${NC}"
            # Display each domain with its terms
            for domain in $domains_learned; do
                printf "  ${MAGENTA}%-18s${NC} ${CYAN}%s${NC}\n" "$domain" "$terms_learned_list"
            done
        elif [ -n "$terms_learned_list" ] && [ "$terms_learned_list" != "" ]; then
            echo -e "${CYAN}${BOLD}⚡ Recently Learned${NC} ${DIM}(${learn_display})${NC}"
            echo -e "  ${CYAN}${terms_learned_list}${NC}"
        else
            echo -e "${DIM}Last learn: ${learn_display} (no new domains found)${NC}"
        fi

        # Show auto-tune results if any
        if [ "$merged_last" -gt 0 ] || [ "$pruned_last" -gt 0 ]; then
            echo -e "  ${DIM}↻ Tuned: merged ${merged_last}, pruned ${pruned_last}${NC}"
        fi
    fi

    # What's next hint
    if [ "$prompt_count" -ge 7 ] 2>/dev/null; then
        local remaining=$((prompt_threshold - prompt_count))
        echo -e "${DIM}→ NEXT Learning in ${remaining} prompts${NC}"
    fi

    # ─────────────────────────────────────────────────────────────────────────
    # Intelligence Angle Footer (GL-054)
    # ─────────────────────────────────────────────────────────────────────────
    echo -e "${DIM}───────────────────────────────────────────────────────────────────────────────────────${NC}"

    # Format tokens invested
    local invested_display
    if [ "$tokens_invested" -ge 1000000 ]; then
        invested_display=$(awk "BEGIN {printf \"%.1fM\", $tokens_invested/1000000}")
    elif [ "$tokens_invested" -ge 1000 ]; then
        invested_display=$(awk "BEGIN {printf \"%.1fk\", $tokens_invested/1000}")
    else
        invested_display="$tokens_invested"
    fi

    # Format tokens saved
    local saved_display
    if [ "$tokens_saved" -ge 1000000 ]; then
        saved_display=$(awk "BEGIN {printf \"%.1fM\", $tokens_saved/1000000}")
    elif [ "$tokens_saved" -ge 1000 ]; then
        saved_display=$(awk "BEGIN {printf \"%.1fk\", $tokens_saved/1000}")
    else
        saved_display="$tokens_saved"
    fi

    echo -e "${CYAN}${BOLD}⚡ Intelligence Angle${NC} ${DIM}│${NC} ${invested_display} tokens invested ${DIM}│${NC} ${GREEN}${saved_display}${NC} saved ${DIM}│${NC} ${CYAN}aOa learns → more time to innovate${NC}"
}

cmd_stats() {
    echo -e "${CYAN}${BOLD}⚡ aOa Stats${NC}"
    echo ""

    # Check services
    if ! curl -s --connect-timeout 2 "${INDEX_URL}/health" > /dev/null 2>&1; then
        echo -e "${RED}✗ aOa services not running${NC}"
        return 1
    fi

    # Get project info
    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="?project=${project_id}"
    fi

    # Get index health
    local health=$(curl -s "${INDEX_URL}/health")
    local files=$(echo "$health" | jq -r '.local.files // 0')
    local symbols=$(echo "$health" | jq -r '.local.symbols // 0')

    # Get outline stats
    local pending_result=$(curl -s "${INDEX_URL}/outline/pending${project_param}")
    local pending_count=$(echo "$pending_result" | jq -r '.pending_count // 0')
    local tagged_count=$(echo "$pending_result" | jq -r '.up_to_date_count // 0')

    # Calculate coverage
    local total_files=$((tagged_count + pending_count))
    local coverage=0
    if [ "$total_files" -gt 0 ]; then
        coverage=$((tagged_count * 100 / total_files))
    fi

    # Get intent stats
    local intent_param=""
    [ -n "$project_id" ] && intent_param="?project_id=${project_id}"
    local intent_stats=$(curl -s "${INDEX_URL}/intent/stats${intent_param}")
    local intents=$(echo "$intent_stats" | jq -r '.total_records // 0')
    local unique_tags=$(echo "$intent_stats" | jq -r '.unique_tags // 0')

    # Get prediction stats
    local predict_stats=$(curl -s "${INDEX_URL}/predict/stats${intent_param}")
    local hit_rate=$(echo "$predict_stats" | jq -r '.rolling.hit_at_5_pct // 0')
    local evaluated=$(echo "$predict_stats" | jq -r '.rolling.evaluated // 0')

    # Get savings
    local metrics=$(curl -s "${INDEX_URL}/metrics${project_param}")
    local tokens_saved=$(echo "$metrics" | jq -r '.savings.tokens // 0')

    # Display
    echo -e "${BOLD}Index${NC}"
    echo -e "  Files:     ${BOLD}${files}${NC}"
    echo -e "  Targets:   ${BOLD}${symbols}${NC}"
    echo -e "  Tagged:    ${GREEN}${tagged_count}${NC} (${coverage}%)"
    [ "$pending_count" -gt 0 ] && echo -e "  Pending:   ${YELLOW}${pending_count}${NC}"
    echo ""

    echo -e "${BOLD}Intents${NC}"
    echo -e "  Captured:  ${BOLD}${intents}${NC}"
    echo -e "  Tags:      ${BOLD}${unique_tags}${NC} unique"
    echo ""

    echo -e "${BOLD}Predictions${NC}"
    printf "  Hit rate:  ${GREEN}%.0f%%${NC} (last %d)\n" "$hit_rate" "$evaluated"
    echo ""

    if [ "$tokens_saved" -gt 0 ]; then
        local tokens_fmt=$(format_tokens $tokens_saved)
        echo -e "${BOLD}Savings${NC}"
        echo -e "  Tokens:    ${GREEN}↓${tokens_fmt}${NC}"
        echo ""
    fi

    echo -e "${DIM}Run 'aoa quickstart' to tag pending files${NC}"
}

cmd_baseline() {
    echo -e "${BOLD}aOa Baseline Costs${NC}"
    echo -e "${DIM}Subagent activity tracked from session logs${NC}"
    echo

    local result=$(curl -s "${STATUS_URL}/baseline" 2>/dev/null)

    if [ -z "$result" ]; then
        echo -e "${RED}Could not connect to status service${NC}"
        return 1
    fi

    local total_tokens=$(echo "$result" | jq -r '.baseline.total_tokens // 0')
    local tool_calls=$(echo "$result" | jq -r '.baseline.tool_calls // 0')
    local search_tools=$(echo "$result" | jq -r '.baseline.search_tools // 0')
    local potential_savings=$(echo "$result" | jq -r '.baseline.potential_savings_tokens // 0')
    local last_sync=$(echo "$result" | jq -r '.baseline.last_sync // 0')

    if [ "$total_tokens" -eq 0 ]; then
        echo -e "${DIM}No baseline data yet.${NC}"
        echo -e "${DIM}Subagent sync runs automatically in the background.${NC}"
        return 0
    fi

    # Format tokens
    format_k() {
        local n=$1
        if [ "$n" -ge 1000 ]; then
            echo "$((n / 1000))k"
        else
            echo "$n"
        fi
    }

    local tokens_fmt=$(format_k $total_tokens)
    local savings_fmt=$(format_k $potential_savings)

    echo -e "  ${BOLD}Subagent Activity Observed:${NC}"
    echo -e "    Tool calls: ${CYAN}${tool_calls}${NC}"
    echo -e "    Tokens: ${CYAN}${tokens_fmt}${NC}"
    echo -e "    Grep/Glob used: ${YELLOW}${search_tools}${NC} times"
    echo

    if [ "$potential_savings" -gt 0 ]; then
        local pct=$((potential_savings * 100 / total_tokens))
        echo -e "  ${BOLD}Potential Savings with aOa:${NC}"
        echo -e "    Tokens: ${GREEN}↓${savings_fmt}${NC} ${DIM}(~${pct}% of subagent tokens)${NC}"
        echo -e "    Tool calls: ${GREEN}↓${search_tools}${NC} ${DIM}Grep/Glob → aoa search${NC}"
        echo
    fi

    if [ "$last_sync" -gt 0 ]; then
        local now=$(date +%s)
        local age=$((now - last_sync))
        echo -e "  ${DIM}Last sync: ${age}s ago${NC}"
    fi
    echo
}

cmd_metrics() {
    local project_id=$(get_project_id)
    local metrics=$(curl -s --connect-timeout 2 "${INDEX_URL}/metrics?project_id=${project_id}" 2>/dev/null)

    if [ -z "$metrics" ] || echo "$metrics" | jq -e '.error' > /dev/null 2>&1; then
        echo -e "${RED}Metrics not available${NC}"
        return 1
    fi

    # Parse metrics
    local hit_pct=$(echo "$metrics" | jq -r '.rolling.hit_at_5_pct // 0')
    local evaluated=$(echo "$metrics" | jq -r '.rolling.evaluated // 0')
    local hits=$(echo "$metrics" | jq -r '.rolling.hits // 0')
    local tokens_saved=$(echo "$metrics" | jq -r '.savings.tokens // 0')
    local time_saved=$(echo "$metrics" | jq -r '.savings.time_sec // 0')
    local trend=$(echo "$metrics" | jq -r '.trend // "unknown"')

    # Format hit percentage
    local hit_int=$(printf "%.0f" "$hit_pct")

    # Traffic light
    local color=$GREEN
    local light="🟢"
    if [ "$evaluated" -lt 3 ]; then
        color=$DIM
        light="⚪"
    elif [ "$hit_int" -lt 80 ]; then
        color=$YELLOW
        light="🟡"
    fi

    echo -e "${BOLD}aOa Prediction Metrics${NC}"
    echo ""
    echo -e "  Accuracy:     ${color}${light} ${hit_int}%${NC} ${DIM}(${evaluated} evaluated)${NC}"
    echo -e "  Hits:         ${hits}"
    echo -e "  Trend:        ${trend}"
    echo ""
    echo -e "${BOLD}Savings${NC}"
    echo -e "  Tokens:       ${GREEN}↓${tokens_saved}${NC}"
    echo -e "  Time:         ${GREEN}⚡${time_saved}s${NC}"
    echo ""
    echo -e "${DIM}Full JSON: aoa metrics --json${NC}"

    # Handle --json flag
    if [[ "${1:-}" == "--json" ]] || [[ "${1:-}" == "-j" ]]; then
        echo ""
        echo "$metrics" | jq .
    fi
}

cmd_history() {
    local limit="${1:-20}"

    curl -s "${STATUS_URL}/history?limit=${limit}" | jq -r '.events[] |
        if .type == "request" then
            "[\(.ts | strftime("%H:%M:%S"))] \(.model) in:\(.input) out:\(.output) $\(.cost)"
        elif .type == "model_switch" then
            "[\(.ts | strftime("%H:%M:%S"))] -> \(.model)"
        elif .type == "block" then
            "[\(.ts | strftime("%H:%M:%S"))] BLOCKED \(.block_type)"
        else
            "[\(.ts | strftime("%H:%M:%S"))] \(.type)"
        end
    '
}

cmd_reset() {
    local target="${1:-session}"

    case "$target" in
        session)
            curl -s -X POST "${STATUS_URL}/session/reset" | jq .
            echo -e "${GREEN}Session reset${NC}"
            ;;
        weekly)
            curl -s -X POST "${STATUS_URL}/weekly/reset" | jq .
            echo -e "${GREEN}Weekly stats reset${NC}"
            ;;
        *)
            echo "Usage: aoa reset [session|weekly]"
            return 1
            ;;
    esac
}

cmd_whitelist() {
    local subcmd="${1:-list}"
    shift || true

    case "$subcmd" in
        list|ls)
            curl -s "${INDEX_URL}/git/whitelist" | jq -r '
                "Default hosts:", (.default_hosts[] | "  \(.)"),
                "", "Custom hosts:",
                (if .custom_hosts | length > 0 then (.custom_hosts[] | "  \(.)") else "  (none)" end)
            '
            ;;
        add)
            local host="$1"
            if [ -z "$host" ]; then
                echo "Usage: aoa whitelist add <host>"
                echo "Example: aoa whitelist add git.company.com"
                return 1
            fi
            curl -s -X POST "${INDEX_URL}/git/whitelist" \
                -H "Content-Type: application/json" \
                -d "{\"host\": \"${host}\"}" | jq .
            ;;
        remove|rm)
            local host="$1"
            if [ -z "$host" ]; then
                echo "Usage: aoa whitelist remove <host>"
                return 1
            fi
            curl -s -X DELETE "${INDEX_URL}/git/whitelist/${host}" | jq .
            ;;
        *)
            echo -e "${BOLD}Whitelist Management${NC}"
            echo ""
            echo "Commands:"
            echo "  aoa whitelist list         Show allowed URLs"
            echo "  aoa whitelist add <host>   Add URL to whitelist"
            echo "  aoa whitelist remove <h>   Remove URL from whitelist"
            echo ""
            echo "Examples:"
            echo "  aoa whitelist add git.company.com"
            echo "  aoa whitelist add docs.internal.org"
            ;;
    esac
}
