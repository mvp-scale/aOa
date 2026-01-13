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

    echo -e "${CYAN}${BOLD}⚡ aOa Quickstart${NC}"
    $force && echo -e "${YELLOW}(force mode - re-tagging all files)${NC}"
    echo ""

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
        # Get Redis container name dynamically
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

    # Get index health
    local health=$(curl -s "${INDEX_URL}/health")
    local files=$(echo "$health" | jq -r '.local.files // 0')
    local symbols=$(echo "$health" | jq -r '.local.symbols // 0')

    echo -e "${GREEN}✓${NC} Your codebase is indexed"
    echo -e "  ${BOLD}${files}${NC} files  │  ${BOLD}${symbols}${NC} targets"
    echo ""
    echo -e "${DIM}Fast search works right now:${NC}"
    echo -e "  ${CYAN}aoa grep <term>${NC}     Instant (<5ms)"
    echo -e "  ${CYAN}aoa egrep <regex>${NC}   Regex patterns"
    echo ""

    # Get pending files for semantic compression
    local pending_result
    if $force; then
        # Force mode: get ALL files (treat them all as pending)
        pending_result=$(curl -s "${INDEX_URL}/files${project_param}")
        local all_files=$(echo "$pending_result" | jq -r '.files // []')
        local file_count=$(echo "$all_files" | jq 'length')
        # Reformat to match pending structure
        pending_result=$(echo "$all_files" | jq '{pending: [.[] | {file: .}], pending_count: length, up_to_date_count: 0}')
    else
        pending_result=$(curl -s "${INDEX_URL}/outline/pending${project_param}")
    fi

    local pending_count=$(echo "$pending_result" | jq -r '.pending_count // 0')
    local up_to_date=$(echo "$pending_result" | jq -r '.up_to_date_count // 0')

    echo -e "────────────────────────────────────────"
    echo -e "${BOLD}Semantic Compression${NC}"
    echo ""

    if [ "$pending_count" -eq 0 ]; then
        echo -e "${GREEN}✓${NC} All ${up_to_date} files are compressed"
        echo ""
        echo -e "${DIM}View any file: aoa outline <file>${NC}"
        echo -e "${DIM}Search by meaning: aoa grep '#authentication'${NC}"
        echo ""
        echo -e "${DIM}To refresh with new patterns: aoa quickstart --force${NC}"
        return 0
    fi

    if $force; then
        echo -e "  Re-tagging: ${YELLOW}${pending_count}${NC} files"
    else
        echo -e "  Tagged:   ${GREEN}${up_to_date}${NC} files"
        echo -e "  Pending:  ${YELLOW}${pending_count}${NC} files"
    fi
    echo ""
    echo -e "${DIM}Semantic compression creates a structured outline of your code${NC}"
    echo -e "${DIM}(functions, classes, methods). This is FREE - runs locally.${NC}"
    echo ""

    # Show one example
    local example_file=$(echo "$pending_result" | jq -r '.pending[0].file // empty')
    if [ -n "$example_file" ]; then
        echo -e "${BOLD}Example:${NC} ${example_file}"
        echo ""
        # Get outline for this file (just a preview)
        local outline_param=""
        [ -n "$project_id" ] && outline_param="&project=${project_id}"
        local outline=$(curl -s "${INDEX_URL}/outline?file=${example_file}${outline_param}")
        local sym_count=$(echo "$outline" | jq -r '.count // 0')
        local ms=$(echo "$outline" | jq -r '.ms // 0')
        printf "  ${CYAN}⚡ ${sym_count} targets${NC} in ${GREEN}%.1fms${NC}\n" "$ms"
        echo "$outline" | jq -r '.symbols[:5][] | "    \(.kind) \(.name) [\(.start_line)-\(.end_line)]"' 2>/dev/null
        local total_syms=$(echo "$outline" | jq -r '.symbols | length')
        if [ "$total_syms" -gt 5 ]; then
            echo -e "    ${DIM}... and $((total_syms - 5)) more${NC}"
        fi
        echo ""
    fi

    # Ask user
    echo -e "────────────────────────────────────────"
    echo -e "${BOLD}Run semantic compression on all ${pending_count} files?${NC}"
    echo -e "${DIM}This is free - runs locally, no API calls${NC}"
    echo -e "${DIM}Estimated time: ~$(( pending_count / 20 + 1 )) seconds${NC}"
    echo ""
    read -p "Proceed? [Y/n] " -n 1 -r
    echo ""

    if [[ ! $REPLY =~ ^[Yy]$ ]] && [[ -n $REPLY ]]; then
        echo -e "${DIM}Skipped. Run 'aoa quickstart' anytime.${NC}"
        return 0
    fi

    echo ""
    echo -e "${BOLD}Processing ${pending_count} files...${NC}"
    echo ""

    # Process each pending file
    local processed=0
    local errors=0

    local loop_param=""
    [ -n "$project_id" ] && loop_param="&project=${project_id}"

    local total_tags=0
    local total_symbols=0

    # Collect word frequencies for domain analysis
    declare -A word_counts

    while IFS= read -r file; do
        [ -z "$file" ] && continue

        # Get outline for this file
        local result=$(curl -s "${INDEX_URL}/outline?file=${file}${loop_param}")
        local err=$(echo "$result" | jq -r '.error // empty')

        if [ -n "$err" ]; then
            errors=$((errors + 1))
            [ -n "$VERBOSE" ] && echo -e "\n${YELLOW}Failed to get outline:${NC} $file ($err)" >&2
        else
            # Extract symbols from outline
            local symbols_json=$(echo "$result" | jq -c '[.symbols[] | {name: .name, kind: .kind, start_line: .start_line, end_line: .end_line}]')
            local sym_count=$(echo "$symbols_json" | jq 'length')
            total_symbols=$((total_symbols + sym_count))

            # Collect word frequencies for domain analysis
            echo "$result" | jq -r '.symbols[].name' | while read -r sym_name; do
                local words=$(echo "$sym_name" | sed 's/\([a-z]\)\([A-Z]\)/\1 \2/g' | sed 's/_/ /g' | tr '[:upper:]' '[:lower:]')
                for word in $words; do
                    [ ${#word} -lt 3 ] && continue
                    [[ "$word" =~ ^(the|and|for|with|this|that|from|have|been|will|are|was|get|set|has|can|did|does|its|not|but|all|any|new|one|two|out|use|add|run|try|end|let|var|def|int|str|err|ctx|req|res|nil|key|val|idx|len|num|tmp|ptr|src|dst|max|min|sum|avg|etc)$ ]] && continue
                    word_counts["$word"]=$((${word_counts["$word"]:-0} + 1))
                done
            done

            # Call /patterns/infer API for batch tag inference
            local infer_payload=$(echo "$symbols_json" | jq '{symbols: [.[] | {name: .name, kind: .kind}]}')
            local infer_result=$(curl -s -X POST "${INDEX_URL}/patterns/infer" \
                -H "Content-Type: application/json" \
                -d "$infer_payload")
            local inferred_tags=$(echo "$infer_result" | jq -c '.tags // []')

            # Build symbols array with inferred tags
            local symbols_with_tags="["
            local first=true
            local idx=0

            while IFS= read -r sym_json; do
                [ -z "$sym_json" ] && continue

                local sym_name=$(echo "$sym_json" | jq -r '.name')
                local sym_kind=$(echo "$sym_json" | jq -r '.kind')
                local sym_line=$(echo "$sym_json" | jq -r '.start_line')
                local sym_end=$(echo "$sym_json" | jq -r '.end_line')

                # Get tags from inferred results (fallback to empty if index out of bounds)
                local tags=$(echo "$inferred_tags" | jq -c ".[$idx] // []")
                local tag_count=$(echo "$tags" | jq 'length')
                total_tags=$((total_tags + tag_count))
                idx=$((idx + 1))

                # Build symbol object with proper JSON escaping
                local sym_obj=$(jq -n \
                    --arg name "$sym_name" \
                    --arg kind "$sym_kind" \
                    --argjson line "$sym_line" \
                    --argjson end "$sym_end" \
                    --argjson tags "$tags" \
                    '{name: $name, kind: $kind, line: $line, end_line: $end, tags: $tags}')
                $first || symbols_with_tags+=","
                first=false
                symbols_with_tags+="$sym_obj"

            done < <(echo "$result" | jq -c '.symbols[]')

            symbols_with_tags+="]"

            # POST to /outline/enriched with inferred tags (use jq for proper escaping)
            local mark_payload=$(jq -n \
                --arg file "$file" \
                --arg project "$project_id" \
                --argjson symbols "$symbols_with_tags" \
                '{file: $file, project: $project, symbols: $symbols}')

            local response=$(curl -s -w "\n%{http_code}" -X POST "${INDEX_URL}/outline/enriched" \
                -H "Content-Type: application/json" \
                -d "$mark_payload")
            local http_code=$(echo "$response" | tail -n1)

            if [ "$http_code" = "200" ]; then
                processed=$((processed + 1))
            else
                errors=$((errors + 1))
                # Log failure for debugging (visible with VERBOSE=1)
                [ -n "$VERBOSE" ] && echo -e "\n${YELLOW}Failed to enrich:${NC} $file (HTTP $http_code)" >&2
            fi
        fi

        # Progress indicator every 5 files
        if [ $((processed % 5)) -eq 0 ]; then
            printf "\r  ${GREEN}✓${NC} ${processed}/${pending_count} files (${total_tags} tags)"
        fi

    done < <(echo "$pending_result" | jq -r '.pending[].file')

    printf "\r  ${GREEN}✓${NC} ${processed}/${pending_count} files, ${total_tags} tags generated\n"
    echo ""

    if [ "$errors" -gt 0 ]; then
        echo -e "${YELLOW}!${NC} ${errors} files had errors (run with VERBOSE=1 for details)"
    fi

    echo ""
    echo -e "${GREEN}✓${NC} Tagged ${BOLD}${processed}${NC} files (${BOLD}${total_tags}${NC} targets)"
    echo -e "  Your searches now understand code structure, not just text."
    echo ""

    # =========================================================================
    # Domain Analysis: Find project-specific words not in universal patterns
    # =========================================================================

    if [ ${#word_counts[@]} -gt 0 ]; then
        echo -e "────────────────────────────────────────"
        echo -e "${BOLD}Domain Analysis${NC}"
        echo ""

        # Known universal pattern words (from semantic-patterns.json + domain-patterns.json)
        # This is a subset - words we expect to see in any codebase
        local universal_words="get fetch load read set save write store update put delete remove clear create add insert new make build handle process validate check verify parse extract convert transform init setup configure start test auth user login logout session token jwt oauth api endpoint route database sql query cache redis file path dir config setting error exception fail log debug http request response json xml search find filter render display email send queue job task metric index symbol outline service manager helper util client server handler controller model schema"

        # Filter to words NOT in universal patterns, count >= 3
        # Build candidates as proper JSON using jq
        local candidates_json="{}"
        local candidate_count=0
        local top_words=""

        for word in "${!word_counts[@]}"; do
            local count=${word_counts[$word]}
            # Skip if count < 3 or word is in universal patterns
            [ "$count" -lt 3 ] && continue
            [[ " $universal_words " =~ " $word " ]] && continue

            # Build JSON properly with jq (handles special chars)
            candidates_json=$(echo "$candidates_json" | jq --arg w "$word" --argjson c "$count" '. + {($w): $c}')
            candidate_count=$((candidate_count + 1))

            # Track top 10 for display
            if [ -z "$top_words" ]; then
                top_words="${word} (${count})"
            elif [ $candidate_count -le 10 ]; then
                top_words="${top_words}, ${word} (${count})"
            fi
        done

        if [ "$candidate_count" -gt 0 ]; then

            # Try to suggest a domain based on top words
            local suggested_domain=""
            # Simple heuristics - could be enhanced
            [[ "$top_words" =~ claim|policy|premium|underwriter ]] && suggested_domain="Insurance"
            [[ "$top_words" =~ order|cart|checkout|product|sku ]] && suggested_domain="E-commerce"
            [[ "$top_words" =~ patient|diagnosis|prescription|medical ]] && suggested_domain="Healthcare"
            [[ "$top_words" =~ trade|stock|portfolio|ticker ]] && suggested_domain="Finance"
            [[ "$top_words" =~ vehicle|driver|trip|ride ]] && suggested_domain="Transportation"
            [[ "$top_words" =~ recipe|ingredient|menu|restaurant ]] && suggested_domain="Food & Dining"
            [[ "$top_words" =~ lesson|course|student|grade ]] && suggested_domain="Education"
            [[ "$top_words" =~ game|player|score|level ]] && suggested_domain="Gaming"
            [[ "$top_words" =~ song|playlist|artist|album ]] && suggested_domain="Music"
            [[ "$top_words" =~ video|stream|channel|watch ]] && suggested_domain="Video/Media"

            echo -e "  ${CYAN}Analyzed ${total_symbols} symbols${NC}"
            echo -e "  Found ${BOLD}${candidate_count}${NC} project-specific terms"
            echo ""
            echo -e "  ${DIM}Top terms:${NC} ${top_words}"

            if [ -n "$suggested_domain" ]; then
                echo -e "  ${DIM}Suggested domain:${NC} ${YELLOW}${suggested_domain}${NC}"
            fi
            echo ""

            # Store candidates in Redis
            local store_payload=$(jq -n \
                --arg project "$project_id" \
                --argjson candidates "$candidates_json" \
                --arg domain "$suggested_domain" \
                --argjson symbols "$total_symbols" \
                '{project: $project, candidates: $candidates, suggested_domain: $domain, total_symbols: $symbols}')

            local store_result=$(curl -s -X POST "${INDEX_URL}/patterns/candidates" \
                -H "Content-Type: application/json" \
                -d "$store_payload")

            local stored=$(echo "$store_result" | jq -r '.stored // 0')
            if [ "$stored" -gt 0 ]; then
                echo -e "  ${GREEN}✓${NC} Stored ${stored} candidates for future enrichment"
                echo -e "  ${DIM}Run 'aoa learn' to generate domain-specific tags${NC}"
            fi
        else
            echo -e "  ${DIM}No unique project-specific terms found.${NC}"
            echo -e "  ${DIM}Your codebase uses standard naming conventions.${NC}"
        fi
        echo ""
    fi

    echo -e "${DIM}Run 'aoa stats' for full index details${NC}"
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

cmd_help() {
