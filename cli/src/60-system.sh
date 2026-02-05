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
#   cmd_start       Start aOa Docker services
#   cmd_stop        Stop aOa Docker services
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
#   (cmd_quickstart removed - use /aoa-start skill instead)
#   cmd_learn       Discover new patterns
#
# =============================================================================

# =============================================================================
# Environment Export (for shell integration)
# =============================================================================

cmd_env() {
    # Output export statements for shell integration
    # Usage: eval "$(aoa env)" - sets AOA_URL in current shell
    # Claude Code and hooks will inherit this, avoiding file reads

    local host="localhost"
    local port="8080"

    # Read from .env (source of truth)
    if [ -f "$AOA_HOME/.env" ]; then
        host=$(grep "^AOA_DOCKER_HOST=" "$AOA_HOME/.env" 2>/dev/null | cut -d'=' -f2 || echo "localhost")
        port=$(grep "^AOA_DOCKER_PORT=" "$AOA_HOME/.env" 2>/dev/null | cut -d'=' -f2 || echo "8080")
    fi

    echo "export AOA_URL=\"http://${host}:${port}\""
    echo "export AOA_DOCKER_HOST=\"${host}\""
    echo "export AOA_DOCKER_PORT=\"${port}\""
}

cmd_port() {
    # Change aOa port - updates .env, restarts Docker, informs user
    # Usage: aoa port <new_port>

    local new_port="$1"

    # Show current port if no argument
    if [ -z "$new_port" ]; then
        local current_port=$(grep "^AOA_DOCKER_PORT=" "$AOA_HOME/.env" 2>/dev/null | cut -d'=' -f2 || echo "8080")
        echo -e "${CYAN}${BOLD}⚡ aOa Port${NC}"
        echo
        echo -e "  Current: ${BOLD}${current_port}${NC}"
        echo
        echo -e "  ${DIM}To change: aoa port <new_port>${NC}"
        return 0
    fi

    # Validate port number
    if ! [[ "$new_port" =~ ^[0-9]+$ ]] || [ "$new_port" -lt 1024 ] || [ "$new_port" -gt 65535 ]; then
        echo -e "${RED}Invalid port: ${new_port}${NC}"
        echo -e "${DIM}Must be a number between 1024-65535${NC}"
        return 1
    fi

    echo -e "${CYAN}${BOLD}⚡ Changing aOa Port to ${new_port}${NC}"
    echo

    # Update .env
    echo -n "  Updating .env................. "
    sed -i "s/^AOA_DOCKER_PORT=.*/AOA_DOCKER_PORT=${new_port}/" "$AOA_HOME/.env"
    echo -e "${GREEN}✓${NC}"

    # Restart Docker
    echo -n "  Restarting Docker............. "
    cmd_stop > /dev/null 2>&1
    cmd_start > /dev/null 2>&1
    echo -e "${GREEN}✓${NC}"

    echo
    echo -e "${GREEN}${BOLD}✓ Port changed to ${new_port}${NC}"
    echo
    echo -e "  ${YELLOW}→ Open a new terminal${NC} for Claude to use the new port."
    echo -e "  ${DIM}Or run: source ~/.bashrc${NC}"
}

# =============================================================================
# Service Control
# =============================================================================

cmd_start() {
    echo -e "${CYAN}${BOLD}⚡ Starting aOa Services${NC}"
    echo

    local instance_name="aoa-${USER}"
    local gateway_port="${GATEWAY_PORT:-8080}"

    # Read installation mode from .env
    local use_compose=0
    if [ -f "$AOA_HOME/.env" ]; then
        source "$AOA_HOME/.env"
        use_compose="${USE_COMPOSE:-0}"
    fi

    # Check if already running
    if curl -s --connect-timeout 1 "http://localhost:${gateway_port}/health" > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓ Services already running on port ${gateway_port}${NC}"
        return 0
    fi

    # Start based on installation mode
    if [ "$use_compose" -eq 1 ]; then
        # Docker Compose mode
        local compose_count=$(cd "$AOA_HOME" && docker compose -p "$instance_name" ps -q 2>/dev/null | wc -l)
        if [ "$compose_count" -gt 0 ]; then
            echo -e "  ${DIM}Starting docker-compose services (${instance_name})...${NC}"
            cd "$AOA_HOME" && docker compose -p "$instance_name" start
        else
            echo -e "  ${DIM}Starting docker-compose services (${instance_name})...${NC}"
            cd "$AOA_HOME" && docker compose -p "$instance_name" up -d
        fi
    else
        # Unified container mode
        echo -e "  ${DIM}Starting unified container (${instance_name})...${NC}"
        docker start "$instance_name" 2>/dev/null || {
            echo -e "  ${RED}✗ Container '${instance_name}' not found${NC}"
            echo -e "  ${DIM}Run install.sh first to create the container${NC}"
            return 1
        }
    fi

    # Wait for services
    echo -n "  Waiting for services"
    for i in {1..10}; do
        if curl -s --connect-timeout 1 "http://localhost:${gateway_port}/health" > /dev/null 2>&1; then
            echo
            echo -e "  ${GREEN}✓ Services running on port ${gateway_port}${NC}"
            return 0
        fi
        echo -n "."
        sleep 1
    done
    echo
    echo -e "  ${YELLOW}! Services may still be starting${NC}"
}

cmd_stop() {
    echo -e "${CYAN}${BOLD}⚡ Stopping aOa Services${NC}"
    echo

    local instance_name="aoa-${USER}"
    local stopped=false

    # Read installation mode from .env
    local use_compose=0
    if [ -f "$AOA_HOME/.env" ]; then
        source "$AOA_HOME/.env"
        use_compose="${USE_COMPOSE:-0}"
    fi

    # Stop based on installation mode
    if [ "$use_compose" -eq 1 ]; then
        # Docker Compose mode
        local compose_count=$(cd "$AOA_HOME" && docker compose -p "$instance_name" ps -q 2>/dev/null | wc -l)
        if [ "$compose_count" -gt 0 ]; then
            echo -e "  ${DIM}Stopping docker-compose services (${instance_name})...${NC}"
            cd "$AOA_HOME" && docker compose -p "$instance_name" stop
            stopped=true
        fi
    else
        # Unified container mode
        if docker ps -q -f name="$instance_name" 2>/dev/null | grep -q .; then
            echo -e "  ${DIM}Stopping unified container (${instance_name})...${NC}"
            docker stop "$instance_name" > /dev/null 2>&1
            stopped=true
        fi
    fi

    if $stopped; then
        echo -e "  ${GREEN}✓ Services stopped${NC}"
    else
        echo -e "  ${DIM}No running services found for ${instance_name}${NC}"
    fi

    echo
    echo -e "${DIM}Tip: To change port, edit ${AOA_HOME}/.env and restart:${NC}"
    echo -e "${DIM}  AOA_DOCKER_PORT=8081${NC}"
    echo -e "${DIM}  aoa stop && aoa start${NC}"
}

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
        gateway_port=$(grep "^AOA_DOCKER_PORT=" "$env_file" 2>/dev/null | cut -d'=' -f2 || echo "8080")
    fi

    # Show Docker configuration from .env
    echo -e "${BOLD}Docker Configuration:${NC} ${DIM}(from .env)${NC}"
    echo -e "  PROJECTS_ROOT:   ${projects_root} → /userhome"
    echo -e "  AOA_DOCKER_PORT: ${gateway_port}"
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
    echo -e "${DIM}See: .env in aOa installation for configuration${NC}"
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

    echo -e "${BOLD}aOa Health${NC}"
    echo -e "────────────────────────────────────────"
    echo ""

    # =========================================================================
    # SERVICES
    # =========================================================================
    echo -e "${BOLD}Services${NC}"

    # Find the aoa container (could be aoa, aoa-corey, aoa-bray, etc.)
    local aoa_container=$(docker ps --filter "name=aoa" --format "{{.Names}}" 2>/dev/null | head -1)

    # Check Docker
    echo -n "  Docker:        "
    if [ -n "$aoa_container" ]; then
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

    # Check Redis via API (actual service connectivity, not just container)
    echo -n "  Redis:         "
    local redis_status=$(echo "$idx_health" | jq -r '.redis.connected // false')
    if [ "$redis_status" = "true" ]; then
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

    # Check hooks (essential: gateway + status-line)
    echo -n "  Hooks:         "
    local hook_count=0
    [ -f "$project_root/.claude/hooks/aoa-gateway.py" ] && hook_count=$((hook_count + 1))
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
        if grep -qi "aoa grep\|aoa search" "$project_root/CLAUDE.md" 2>/dev/null; then
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

    # Check semantic compression (outline angle, use detected container)
    echo -n "  Outline:       "
    if [ -n "$aoa_container" ] && docker exec "$aoa_container" python3 -c "import tree_sitter" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} Semantic compression ready"
    else
        echo -e "${YELLOW}!${NC} Semantic compression unavailable"
        warnings=$((warnings + 1))
    fi

    echo ""

    # =========================================================================
    # YOUR AOA (value summary)
    # =========================================================================
    local project_id=$(get_project_id)
    if [ -n "$project_id" ]; then
        echo -e "${BOLD}Your aOa${NC}"

        # Get file count from index health
        local file_count=0
        local symbol_count=0
        if [ -n "$idx_health" ]; then
            file_count=$(echo "$idx_health" | jq -r '.local.files // 0' 2>/dev/null)
            symbol_count=$(echo "$idx_health" | jq -r '.local.symbols // 0' 2>/dev/null)
        fi

        # Get domain stats
        local domain_stats=$(curl -s --max-time 1 "${INDEX_URL}/domains/stats?project_id=${project_id}" 2>/dev/null)
        local domain_count=0
        local term_count=0
        if [ -n "$domain_stats" ]; then
            domain_count=$(echo "$domain_stats" | jq -r '.domain_count // 0' 2>/dev/null)
            term_count=$(echo "$domain_stats" | jq -r '.term_count // 0' 2>/dev/null)
        fi

        # Get savings
        local metrics=$(curl -s --max-time 1 "${INDEX_URL}/metrics?project_id=${project_id}" 2>/dev/null)
        local tokens_saved=0
        if [ -n "$metrics" ]; then
            tokens_saved=$(echo "$metrics" | jq -r '.savings.tokens // 0' 2>/dev/null)
        fi
        local tokens_fmt="$tokens_saved"
        [ "$tokens_saved" -ge 1000 ] && tokens_fmt="$((tokens_saved / 1000))k"

        echo -e "  Codebase:  ${file_count} files, ${symbol_count} targets"
        echo -e "  Patterns:  ${domain_count} domains, ${term_count} terms"
        echo -e "  Savings:   ${GREEN}${tokens_fmt}${NC} tokens saved"
        echo ""
    fi

    # =========================================================================
    # SUMMARY
    # =========================================================================
    echo -e "────────────────────────────────────────"
    if $all_ok && [ "$warnings" -eq 0 ]; then
        echo -e "${GREEN}Everything working. Claude is getting smarter.${NC}"
    elif $all_ok; then
        echo -e "${YELLOW}Operational with ${warnings} warning(s)${NC}"
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

# NOTE: cmd_quickstart removed - no longer needed
# KeywordMatcher handles domain tagging in real-time from Redis

cmd_learn() {
    # aoa learn          - Show domain candidates and prompt for tag generation
    # aoa learn --store  - Store learned patterns (from stdin JSON)
    # aoa learn --show   - Show stored learned patterns

    local project_id=$(get_project_id)
    local project_param=""
    [ -n "$project_id" ] && project_param="?project_id=${project_id}"

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
                echo -e "${DIM}Run /aoa-start skill to generate domains.${NC}"
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
        echo -e "Run ${CYAN}/aoa-start${NC} skill first to set up domains."
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
    # aoa domains - Domain management and status
    # Usage: aoa domains [SUBCOMMAND] [OPTIONS]
    #
    # Subcommands:
    #   init     - Initialize skeleton domains from JSON stdin
    #   build    - Add terms+keywords to one domain
    #   add      - Add a single new domain
    #   refresh  - Re-generate a stale domain
    #   (none)   - Show domain status (default)

    local MAGENTA='\033[0;35m'
    local project_id=$(get_project_id)
    local project_root=$(get_project_root)

    # Check for subcommands first
    case "${1:-}" in
        init)
            # aoa domains init - bulk create skeletons from JSON file or stdin
            shift
            local json_input=""
            local file_path=""

            # Check for --file flag
            if [ "${1:-}" = "--file" ] || [ "${1:-}" = "-f" ]; then
                file_path="${2:-}"
                if [ -z "$file_path" ] || [ ! -f "$file_path" ]; then
                    echo -e "${RED}Error: File not found: ${file_path}${NC}" >&2
                    return 1
                fi
                json_input=$(cat "$file_path")
            else
                # Read from stdin
                json_input=$(cat)
            fi

            if [ -z "$json_input" ]; then
                echo -e "${RED}Error: No JSON input provided${NC}" >&2
                echo "Usage: aoa domains init --file path/to/domains.json" >&2
                return 1
            fi
            local result=$(curl -s -X POST "${INDEX_URL}/domains/init-skeleton" \
                -H "Content-Type: application/json" \
                -d "{\"project_id\":\"${project_id}\",\"domains\":${json_input}}")
            local count=$(echo "$result" | jq -r '.domains_created // 0')
            local error=$(echo "$result" | jq -r '.error // empty')
            if [ -n "$error" ]; then
                echo -e "${RED}Error: ${error}${NC}" >&2
                return 1
            fi
            echo "$count"
            return 0
            ;;
        build)
            # aoa domains build @name - add terms+keywords to one domain
            # aoa domains build --all - build all @*.json files in .aoa/domains/
            shift
            local domain_name="${1:-}"

            # Handle --all flag
            if [ "$domain_name" = "--all" ]; then
                local count=0
                local total=0
                local failed=0
                local missing=0
                local intelligence_file=".aoa/domains/intelligence.json"

                # Validate against intelligence.json if it exists
                if [ -f "$intelligence_file" ]; then
                    local expected_domains=$(jq -r '.[].name' "$intelligence_file" 2>/dev/null)
                    local missing_domains=""
                    for domain in $expected_domains; do
                        local file=".aoa/domains/${domain}.json"
                        if [ ! -f "$file" ]; then
                            missing_domains="$missing_domains $domain"
                            missing=$((missing + 1))
                        fi
                    done

                    if [ "$missing" -gt 0 ]; then
                        echo -e "${RED}${BOLD}⚠ Missing ${missing} domain files:${NC}"
                        for d in $missing_domains; do
                            printf "  ${RED}✗${NC} %s.json\n" "$d"
                        done
                        echo -e "\n${DIM}Run /aoa-start to regenerate, or create files manually.${NC}"
                        return 1
                    fi
                fi

                # Count files
                for f in .aoa/domains/@*.json; do
                    [ -f "$f" ] && total=$((total + 1))
                done

                if [ "$total" -eq 0 ]; then
                    echo -e "${YELLOW}No domain files found${NC}"
                    return 0
                fi

                echo -e "${CYAN}${BOLD}⚡ Building ${total} domains${NC}"

                for f in .aoa/domains/@*.json; do
                    [ ! -f "$f" ] && continue
                    local name=$(basename "$f" .json)
                    local result=$(cmd_domains build "$name" 2>/dev/null)
                    if [ $? -eq 0 ]; then
                        count=$((count + 1))
                        printf "  ${GREEN}✓${NC} %s (%s keywords)\n" "$name" "$result"
                    else
                        failed=$((failed + 1))
                        printf "  ${RED}✗${NC} %s\n" "$name"
                    fi
                done

                echo -e "\n${GREEN}✓${NC} ${BOLD}${count}${NC} domains enriched"
                [ "$failed" -gt 0 ] && echo -e "${RED}${failed} failed${NC}"
                return 0
            fi

            if [ -z "$domain_name" ]; then
                echo -e "${RED}Error: Domain name required${NC}" >&2
                echo "Usage: aoa domains build @search" >&2
                echo "       aoa domains build --all" >&2
                return 1
            fi
            # Support per-domain files: .aoa/domains/@name.json
            local enrichment_file=".aoa/domains/${domain_name}.json"
            # Fallback to shared file for backwards compatibility
            [ ! -f "$enrichment_file" ] && enrichment_file=".aoa/domains/enrichment.json"
            if [ ! -f "$enrichment_file" ]; then
                echo -e "${RED}Error: No enrichment file found at ${enrichment_file}${NC}" >&2
                return 1
            fi
            local json_input=$(cat "$enrichment_file")
            if [ -z "$json_input" ]; then
                echo -e "${RED}Error: Enrichment file is empty${NC}" >&2
                return 1
            fi
            # Validate domain field matches argument
            local file_domain=$(echo "$json_input" | jq -r '.domain // empty')
            if [ -z "$file_domain" ]; then
                echo -e "${RED}Error: Enrichment file missing 'domain' field${NC}" >&2
                echo -e "${DIM}Expected: {\"domain\": \"@name\", \"terms\": {...}}${NC}" >&2
                return 1
            fi
            if [ "$file_domain" != "$domain_name" ]; then
                echo -e "${RED}Error: Domain mismatch: expected ${domain_name}, got ${file_domain}${NC}" >&2
                return 1
            fi
            # Extract terms from the validated JSON
            local terms_json=$(echo "$json_input" | jq -c '.terms // {}')
            if [ "$terms_json" = "{}" ] || [ "$terms_json" = "null" ]; then
                echo -e "${RED}Error: Enrichment file has no terms${NC}" >&2
                return 1
            fi
            local result=$(curl -s -X POST "${INDEX_URL}/domains/enrich" \
                -H "Content-Type: application/json" \
                -d "{\"project_id\":\"${project_id}\",\"domain\":\"${domain_name}\",\"term_keywords\":${terms_json}}")
            local error=$(echo "$result" | jq -r '.error // empty')
            if [ -n "$error" ]; then
                echo -e "${RED}Error: ${error}${NC}" >&2
                return 1
            fi
            local keywords_added=$(echo "$result" | jq -r '.keywords_added // 0')
            # Auto-rebuild KeywordMatcher to link keywords to files
            curl -sf -X POST "${INDEX_URL}/keywords/rebuild?project_id=${project_id}" > /dev/null 2>&1

            # Cleanup: delete domain file after successful load (Redis is source of truth)
            if [ -f "$enrichment_file" ] && [ "$enrichment_file" != ".aoa/domains/enrichment.json" ]; then
                rm -f "$enrichment_file"
            fi

            echo "$keywords_added"
            return 0
            ;;
        add)
            # aoa domains add - add a single new domain from JSON stdin
            shift
            local json_input=$(cat)
            if [ -z "$json_input" ]; then
                echo -e "${RED}Error: No JSON input provided${NC}" >&2
                echo "Usage: echo '{\"name\":\"@new\",...}' | aoa domains add" >&2
                return 1
            fi
            # Wrap single domain in array for init-skeleton endpoint
            local result=$(curl -s -X POST "${INDEX_URL}/domains/init-skeleton" \
                -H "Content-Type: application/json" \
                -d "{\"project_id\":\"${project_id}\",\"domains\":[${json_input}]}")
            local count=$(echo "$result" | jq -r '.domains_created // 0')
            local error=$(echo "$result" | jq -r '.error // empty')
            if [ -n "$error" ]; then
                echo -e "${RED}Error: ${error}${NC}" >&2
                return 1
            fi
            echo "$count"
            return 0
            ;;
        refresh)
            # aoa domains refresh @name - mark domain for re-generation
            shift
            local domain_name="${1:-}"
            if [ -z "$domain_name" ]; then
                echo -e "${RED}Error: Domain name required${NC}" >&2
                echo "Usage: aoa domains refresh @search" >&2
                return 1
            fi
            # Mark domain as unenriched so it gets rebuilt
            local result=$(curl -s -X POST "${INDEX_URL}/domains/unenrich" \
                -H "Content-Type: application/json" \
                -d "{\"project_id\":\"${project_id}\",\"domain\":\"${domain_name}\"}")
            local error=$(echo "$result" | jq -r '.error // empty')
            if [ -n "$error" ]; then
                echo -e "${RED}Error: ${error}${NC}" >&2
                return 1
            fi
            echo "ok"
            return 0
            ;;
        pending)
            # aoa domains pending - show unenriched domains needing work
            # Returns: one domain name per line, or nothing if all done
            # Use with: aoa domains pending 3 (get batch of 3)
            shift
            local batch_size="${1:-3}"

            # Get pending domains from API
            local result=$(curl -sf "${INDEX_URL}/domains/pending?project_id=${project_id}&limit=${batch_size}" 2>/dev/null)
            if [ -z "$result" ]; then
                echo -e "${RED}Error: Could not fetch pending domains${NC}" >&2
                return 1
            fi

            # Output one domain name per line
            echo "$result" | jq -r '.domains[]?' 2>/dev/null
            return 0
            ;;
        clear-pending)
            # aoa domains clear-pending - clear the haiku learning pending flag
            # Called by /aoa-rebalance skill when it starts (reliable trigger pattern)
            shift
            local result=$(curl -sf -X POST "${INDEX_URL}/domains/haiku-pending" \
                -H "Content-Type: application/json" \
                -d "{\"project_id\": \"${project_id}\", \"clear\": true}" 2>/dev/null)
            if [ -z "$result" ]; then
                echo -e "${RED}Error: Could not clear pending flag${NC}" >&2
                return 1
            fi
            local cleared=$(echo "$result" | jq -r '.cleared // false')
            if [ "$cleared" = "true" ]; then
                echo -e "${GREEN}✓${NC} Cleared pending flag"
                return 0
            else
                echo -e "${YELLOW}No pending flag set${NC}"
                return 0
            fi
            ;;
        link)
            # aoa domains link - rebuild KeywordMatcher to link domains to files
            # Called after building domains to enable semantic tags in search results
            shift
            local result=$(curl -sf -X POST "${INDEX_URL}/keywords/rebuild?project_id=${project_id}" 2>/dev/null)
            if [ -z "$result" ]; then
                echo -e "${RED}Error: Could not rebuild keyword matcher${NC}" >&2
                return 1
            fi
            local status=$(echo "$result" | jq -r '.status // "error"')
            local keywords=$(echo "$result" | jq -r '.keywords // 0')
            local domains=$(echo "$result" | jq -r '.domains // 0')
            local elapsed=$(echo "$result" | jq -r '.elapsed_ms // 0')
            if [ "$status" = "ok" ] || [ "$status" = "initialized" ]; then
                echo -e "${GREEN}✓${NC} Linked ${CYAN}${keywords}${NC} keywords across ${MAGENTA}${domains}${NC} domains ${DIM}(${elapsed}ms)${NC}"
                return 0
            else
                local error=$(echo "$result" | jq -r '.error // "Unknown error"')
                echo -e "${RED}Error: ${error}${NC}" >&2
                return 1
            fi
            ;;
        clean)
            # aoa domains clean - delete @*.json files for domains already in Redis
            # Called after intelligence completes to clean up processed domain files
            shift
            local domains_dir="${project_root}/.aoa/domains"
            if [ ! -d "$domains_dir" ]; then
                echo "No domains directory"
                return 0
            fi

            # Get list of enriched domains from Redis
            local enriched=$(curl -sf "${INDEX_URL}/domains/list?project_id=${project_id}" 2>/dev/null \
                | jq -r '.domains[] | select(.enriched == true) | .name' 2>/dev/null)

            if [ -z "$enriched" ]; then
                echo "No enriched domains in Redis"
                return 0
            fi

            local cleaned=0
            local skipped=0
            for domain_file in "$domains_dir"/@*.json; do
                [ -f "$domain_file" ] || continue
                local domain_name=$(basename "$domain_file" .json)
                # Check if this domain is enriched in Redis
                if echo "$enriched" | grep -qx "$domain_name"; then
                    rm -f "$domain_file" && ((cleaned++)) || true
                else
                    ((skipped++)) || true
                fi
            done

            if [ "$cleaned" -gt 0 ]; then
                echo -e "${GREEN}✓${NC} Cleaned ${cleaned} domain files"
            fi
            if [ "$skipped" -gt 0 ]; then
                echo -e "  ${DIM}${skipped} files skipped (not yet in Redis)${NC}"
            fi
            if [ "$cleaned" -eq 0 ] && [ "$skipped" -eq 0 ]; then
                echo "No domain files to clean"
            fi
            ;;
        load-intent)
            # aoa domains load-intent - load domains from intent.json and add to context tier
            shift
            local intent_file="${project_root}/.aoa/domains/intent.json"
            if [ ! -f "$intent_file" ]; then
                echo -e "${RED}Error: No intent.json found at ${intent_file}${NC}" >&2
                return 1
            fi

            # Read and process each domain
            local count=0
            local domains=$(cat "$intent_file")
            local total=$(echo "$domains" | jq 'length')

            for i in $(seq 0 $((total - 1))); do
                local domain=$(echo "$domains" | jq ".[$i]")
                local name=$(echo "$domain" | jq -r '.domain')
                local desc=$(echo "$domain" | jq -r '.description // "Intent-generated domain"')
                local terms=$(echo "$domain" | jq -c '.terms | keys')
                local keywords=$(echo "$domain" | jq -c '.terms')

                # Call add-context API
                local result=$(curl -sf -X POST "${INDEX_URL}/domains/add-context" \
                    -H "Content-Type: application/json" \
                    -d "{\"project_id\":\"${project_id}\",\"name\":\"${name}\",\"description\":\"${desc}\",\"terms\":${terms},\"keywords\":${keywords}}" 2>/dev/null)

                if echo "$result" | jq -e '.success' > /dev/null 2>&1; then
                    ((count++))
                fi
            done

            # Cleanup intent.json after successful load
            rm -f "$intent_file"

            echo -e "${GREEN}✓${NC} Added ${CYAN}${count}${NC} domains to context tier"
            return 0
            ;;
    esac

    # Default: show domain status
    local json_output=false
    local names_only=false
    local limit=24

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --json|-j)
                json_output=true
                shift
                ;;
            --names)
                names_only=true
                shift
                ;;
            -n|--limit)
                limit="${2:-20}"
                shift 2
                ;;
            --help|-h)
                echo "Usage: aoa domains [SUBCOMMAND] [OPTIONS]"
                echo ""
                echo "Domain management and status."
                echo ""
                echo "Subcommands:"
                echo "  init              Initialize skeleton domains from JSON stdin"
                echo "  build @name       Add terms+keywords to one domain"
                echo "  link              Link domains to files (rebuild keyword matcher)"
                echo "  add               Add a single new domain from JSON stdin"
                echo "  refresh @name     Mark domain for re-generation"
                echo "  pending [N]       List unenriched domains (default: 3)"
                echo "  load-intent       Load domains from .aoa/domains/intent.json"
                echo ""
                echo "Options (for status display):"
                echo "  --json, -j        Output as JSON"
                echo "  --names           Output domain names only (one per line)"
                echo "  -n, --limit N     Show top N domains (default: 20)"
                echo "  --help, -h        Show this help message"
                echo ""
                echo "Examples:"
                echo "  aoa domains                    # Show domain status"
                echo "  aoa domains --names            # Just domain names"
                echo "  echo '[...]' | aoa domains init   # Init skeletons"
                echo "  aoa domains refresh @search    # Mark for rebuild"
                echo "  aoa domains pending            # Show 3 unenriched domains"
                echo "  aoa domains pending 5          # Show 5 unenriched domains"
                return 0
                ;;
            *)
                shift
                ;;
        esac
    done

    local project_id=$(get_project_id)

    # Check if project is initialized
    if [ -z "$project_id" ]; then
        echo -e "${CYAN}${BOLD}⚡ aOa Domains${NC}"
        echo ""
        echo -e "${DIM}No project initialized. Run 'aoa init' first.${NC}"
        return 0
    fi

    # Get domain stats
    local stats=$(curl -s "${INDEX_URL}/domains/stats?project_id=${project_id}")

    # Check if API is reachable
    if [ -z "$stats" ]; then
        echo -e "${CYAN}${BOLD}⚡ aOa Domains${NC}"
        echo ""
        echo -e "${RED}Cannot connect to aOa services at ${INDEX_URL}${NC}"
        echo -e "${DIM}Check that Docker is running: docker ps${NC}"
        return 1
    fi

    local domain_count=$(echo "$stats" | jq -r '.domains // 0')
    local total_terms=$(echo "$stats" | jq -r '.total_terms // 0')
    local total_hits=$(echo "$stats" | jq -r '.total_hits // 0')
    local prompt_count=$(echo "$stats" | jq -r '.prompt_count // 0')
    # GL-083: Rebalance-based system - fetch configurable threshold (QoL-2)
    local thresholds=$(curl -s "${INDEX_URL}/config/thresholds?project_id=${project_id}")
    local rebalance_threshold=$(echo "$thresholds" | jq -r '.thresholds.rebalance // 25 | floor')
    # Guard against division by zero if API fails
    [ -z "$rebalance_threshold" ] || [ "$rebalance_threshold" -eq 0 ] 2>/dev/null && rebalance_threshold=25
    local rebalance_progress=$((prompt_count % rebalance_threshold))
    # GL-054: Intelligence Angle (legacy - may be removed)
    local tokens_invested=$(echo "$stats" | jq -r '.tokens_invested // 0')
    # GL-059.1: Source counts
    local seeded_count=$(echo "$stats" | jq -r '.seeded_count // 0')
    local learned_count=$(echo "$stats" | jq -r '.learned_count // 0')
    # GL-085: Enrichment status
    local enriched_count=$(echo "$stats" | jq -r '.enrichment.enriched // 0')
    local enrichment_total=$(echo "$stats" | jq -r '.enrichment.total // 0')
    local enrichment_complete=$(echo "$stats" | jq -r '.enrichment.complete // false')

    # Get tokens saved from metrics endpoint
    local metrics=$(curl -s "${INDEX_URL}/metrics?project_id=${project_id}")
    local tokens_saved=$(echo "$metrics" | jq -r '.savings.tokens // 0')

    if [ "$domain_count" -eq 0 ] 2>/dev/null; then
        echo -e "${CYAN}${BOLD}⚡ aOa Domains${NC}"
        echo ""
        echo -e "${DIM}No domains found. Run '/aoa-start' to initialize.${NC}"
        return 0
    fi

    # Format hits compactly
    local hits_display
    if [ "$total_hits" -ge 1000 ]; then
        hits_display=$(awk "BEGIN {printf \"%.1fk\", $total_hits/1000}")
    else
        hits_display="$total_hits"
    fi

    # Get domains with full terms for display
    local domains_data=$(curl -s "${INDEX_URL}/domains/list?project_id=${project_id}&limit=${limit}&include_terms=true&include_created=true" 2>/dev/null)

    # GL-090: Get tier counts from ALL domains (not just display limit) for accurate totals
    local all_domains=$(curl -s "${INDEX_URL}/domains/list?project_id=${project_id}&limit=100" 2>/dev/null)
    local core_count=$(echo "$all_domains" | jq '[.domains[]? | select(.tier == "core")] | length')
    local context_count=$(echo "$all_domains" | jq '[.domains[]? | select(.tier == "context")] | length')
    local now=$(date +%s)

    # ─────────────────────────────────────────────────────────────────────────
    # Names Only Output Mode (for Haiku prompts)
    # ─────────────────────────────────────────────────────────────────────────
    if [ "$names_only" = true ]; then
        echo "$all_domains" | jq -r '.domains[]?.name' | sort
        return 0
    fi

    # ─────────────────────────────────────────────────────────────────────────
    # JSON Output Mode
    # ─────────────────────────────────────────────────────────────────────────
    if [ "$json_output" = true ]; then
        # Combine stats and domains into single JSON object
        jq -n \
            --argjson stats "$stats" \
            --argjson domains "$domains_data" \
            --arg project "$project_id" \
            '{
                project_id: $project,
                summary: {
                    domains: ($stats.domains // 0),
                    total_terms: ($stats.total_terms // 0),
                    total_hits: ($stats.total_hits // 0),
                    tokens_invested: ($stats.tokens_invested // 0),
                    learning_calls: ($stats.learning_calls // 0)
                },
                learning: {
                    prompt_count: ($stats.prompt_count // 0),
                    prompt_threshold: ($stats.prompt_threshold // 10),
                    last_learn: ($stats.last_learn // 0),
                    domains_learned: ($stats.domains_learned_list // []),
                    terms_learned: ($stats.terms_learned_list // [])
                },
                tuning: {
                    tune_count: ($stats.tune_count // 0),
                    tune_threshold: ($stats.tune_threshold // 100),
                    last_tune: ($stats.last_tune // 0),
                    last_results: {
                        kept: ($stats.tune_kept_last // 0),
                        added: ($stats.tune_added_last // 0),
                        removed: ($stats.tune_removed_last // 0)
                    }
                },
                domains: ($domains.domains // [])
            }'
        return 0
    fi

    # ─────────────────────────────────────────────────────────────────────────
    # aOa Domains Section (stats header + domain list)
    # GL-059.5: Show Generic vs Learned counts
    # GL-085: Show enrichment progress
    # ─────────────────────────────────────────────────────────────────────────
    echo ""
    # GL-085: Show enrichment progress OR rebalance based on state
    local progress_display
    local in_enrichment=false
    if [ "$enrichment_total" -gt 0 ] && [ "$enrichment_complete" != "true" ]; then
        progress_display="${YELLOW}intelligence ${enriched_count}/${enrichment_total}${NC}"
        in_enrichment=true
    else
        progress_display="Rebalance: ${YELLOW}${rebalance_progress}/${rebalance_threshold}${NC}"
    fi

    # Adjust header based on state
    # GL-090: Show tier breakdown (core/context) for cap verification
    local tier_display=""
    if [ "$core_count" -gt 0 ] || [ "$context_count" -gt 0 ]; then
        tier_display=" ${DIM}(${core_count} core, ${context_count} context)${NC}"
    fi

    if [ "$in_enrichment" = true ] && [ "$total_terms" -eq 0 ]; then
        # Skeleton phase - no terms yet
        echo -e "${CYAN}${BOLD}⚡ aOa Domains${NC}  ${MAGENTA}${domain_count}${NC} skeletons${tier_display} ${DIM}│${NC} ${progress_display}"
    else
        echo -e "${CYAN}${BOLD}⚡ aOa Domains${NC}  ${MAGENTA}${domain_count}${NC} domains${tier_display} ${DIM}│${NC} ${CYAN}${total_terms}${NC} terms ${DIM}│${NC} ${GREEN}${hits_display}${NC} hits ${DIM}│${NC} ${progress_display}"
    fi
    echo -e "${DIM}───────────────────────────────────────────────────────────────────────────────────────${NC}"

    # Column header - competitive ranking format (fixed widths for alignment)
    # Format: rank(2) + gap(2) + domain(24) + gap(1) + hits(6) + gap(2) + terms
    printf "${DIM}%2s  %-24s %6s  %s${NC}\n" "#" "DOMAIN" "HITS" "TERMS"

    # Display each domain - sorted by hits (competitive ranking)
    local rank=1
    echo "$domains_data" | jq -r '.domains | sort_by(-.hits) | .[]? | "\(.name)|\(.hits // 0)|\(.enriched // false)|\(.terms // [] | .[0:5] | join(" "))"' 2>/dev/null | while IFS='|' read -r name hits enriched terms; do
        # Truncate domain name if too long (24 chars to fit column)
        local name_trunc="${name:0:24}"

        # Tier indicator: top 24 = core (green), rest = context (yellow)
        local tier_color="${GREEN}"
        [ "$rank" -gt 24 ] && tier_color="${YELLOW}"

        # Show hits and terms
        local hits_fmt
        [ "$hits" -ge 1000 ] && hits_fmt=$(awk "BEGIN {printf \"%.1fk\", $hits/1000}") || hits_fmt="$hits"
        printf "${tier_color}%2d${NC}  ${MAGENTA}%-24s${NC} %6s  ${CYAN}%s${NC}\n" "$rank" "$name_trunc" "$hits_fmt" "$terms"
        ((rank++))
    done

    # Show remaining count if more than displayed
    if [ "$domain_count" -gt "$limit" ] 2>/dev/null; then
        local remaining=$((domain_count - limit))
        echo -e "${DIM}+${remaining} more domains${NC}"
    fi

    # ─────────────────────────────────────────────────────────────────────────
    # Recently Learned Section (GL-054)
    # ─────────────────────────────────────────────────────────────────────────

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

        # Get learned domains list
        local domains_learned=$(echo "$stats" | jq -r '.domains_learned_list // [] | join(" ")')

        # Show Recently Learned section
        if [ -n "$domains_learned" ] && [ "$domains_learned" != "" ]; then
            echo ""
            echo -e "${CYAN}${BOLD}⚡ Recently Learned${NC} ${DIM}(${learn_display})${NC}"
            echo -e "${DIM}───────────────────────────────────────────────────────────────────────────────────────${NC}"
            printf "${DIM}%-22s %5s  %s${NC}\n" "DOMAIN" "HITS" "TERMS"
            # Display each domain with its ACTUAL terms and hits (fetch from API)
            for domain in $domains_learned; do
                # Get this domain's terms and hits from the full domains list
                local domain_info=$(echo "$domains_data" | jq -r --arg d "$domain" '.domains[]? | select(.name == $d) | "\(.hits // 0)|\(.terms // [] | .[0:5] | join(" "))"' 2>/dev/null)
                if [ -z "$domain_info" ] || [ "$domain_info" = "|" ]; then
                    # Fallback: fetch directly if not in top 20
                    domain_info=$(curl -s "${INDEX_URL}/domains/list?project_id=${project_id}&limit=50&include_terms=true" 2>/dev/null | jq -r --arg d "$domain" '.domains[]? | select(.name == $d) | "\(.hits // 0)|\(.terms // [] | .[0:5] | join(" "))"' 2>/dev/null)
                fi
                local domain_hits=$(echo "$domain_info" | cut -d'|' -f1)
                local domain_terms=$(echo "$domain_info" | cut -d'|' -f2)
                local hits_fmt
                [ "${domain_hits:-0}" -ge 1000 ] 2>/dev/null && hits_fmt=$(awk "BEGIN {printf \"%.1fk\", ${domain_hits}/1000}") || hits_fmt="${domain_hits:-0}"
                # Truncate domain name if too long
                local domain_trunc="${domain:0:20}"
                # Use + prefix to reinforce learned iconography
                printf "${YELLOW}+${NC} ${MAGENTA}%-20s${NC} ${DIM}%5s${NC}  ${CYAN}%s${NC}\n" "$domain_trunc" "$hits_fmt" "${domain_terms:-...}"
            done
        else
            echo -e "${DIM}Last learn: ${learn_display} (no new domains found)${NC}"
        fi

        # Show tune results if any (GL-055)
        if [ "$tune_kept" -gt 0 ] || [ "$tune_added" -gt 0 ] || [ "$tune_removed" -gt 0 ]; then
            echo -e "${DIM}↻ Last tune: kept ${tune_kept}, added ${tune_added}, removed ${tune_removed}${NC}"
        fi
    fi

    # ─────────────────────────────────────────────────────────────────────────
    # Dynamic Intent Section (GL-078: Goal-focused semantic tagging)
    # ─────────────────────────────────────────────────────────────────────────

    # Fetch recent prompt records (goal + tags) - get more to show total count
    local project_param=""
    [ -n "$project_id" ] && project_param="project_id=${project_id}"
    local prompts_data=$(curl -s "${INDEX_URL}/domains/goal-history?${project_param}&limit=100" 2>/dev/null)

    # Get total goal count
    local total_goals=$(echo "$prompts_data" | jq -r '.count // 0' 2>/dev/null)

    if [ "$total_goals" -gt 0 ]; then
        echo ""
        echo -e "${CYAN}${BOLD}⚡ Dynamic Intent${NC} ${DIM}(${total_goals} goals)${NC}"
        echo -e "${DIM}───────────────────────────────────────────────────────────────────────────────────────${NC}"

        # Display last 2 goals with their tags (most recent first - API returns newest first)
        echo "$prompts_data" | jq -r '
            .prompts // [] | .[0:2] | .[] |
            "\u001b[33m\u001b[1m→ \(.goal)\u001b[0m\n  \u001b[35m\(.tags | map(.tag) | join(" "))\u001b[0m"
        ' 2>/dev/null
    fi

    # ─────────────────────────────────────────────────────────────────────────
    # Intelligence Angle Footer (GL-054)
    # ─────────────────────────────────────────────────────────────────────────

    # Format tokens invested (2 decimals for k, 3 for M to show movement)
    local invested_display
    if [ "$tokens_invested" -ge 1000000 ]; then
        invested_display=$(awk "BEGIN {printf \"%.3fM\", $tokens_invested/1000000}")
    elif [ "$tokens_invested" -ge 1000 ]; then
        invested_display=$(awk "BEGIN {printf \"%.2fk\", $tokens_invested/1000}")
    else
        invested_display="$tokens_invested"
    fi

    # Format tokens saved (2 decimals for k, 3 for M to show movement)
    local saved_display
    if [ "$tokens_saved" -ge 1000000 ]; then
        saved_display=$(awk "BEGIN {printf \"%.3fM\", $tokens_saved/1000000}")
    elif [ "$tokens_saved" -ge 1000 ]; then
        saved_display=$(awk "BEGIN {printf \"%.2fk\", $tokens_saved/1000}")
    else
        saved_display="$tokens_saved"
    fi

    echo ""
    echo -e "${CYAN}${BOLD}⚡ Intelligence Angle${NC}"
    echo -e "${DIM}───────────────────────────────────────────────────────────────────────────────────────${NC}"
    echo -e "${invested_display} invested ${DIM}│${NC} ${GREEN}${saved_display}${NC} saved ${DIM}│${NC} ${CYAN}aOa learns → you build faster.${NC} ${BOLD}${YELLOW}This is the way.${NC}"
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
        project_param="?project_id=${project_id}"
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

    echo -e "${DIM}Run /aoa-start skill to set up domains${NC}"
}

# =============================================================================
# GL-091: Test Mode Configuration
# =============================================================================

cmd_config() {
    local project_id=$(get_project_id)

    case "${1:-}" in
        thresholds)
            shift
            local mode="${1:-}"

            if [ -z "$mode" ]; then
                # Show current thresholds (TC-04: updated for Session 64)
                echo -e "${CYAN}${BOLD}⚡ aOa Thresholds${NC}"
                echo ""
                local result=$(curl -s "${INDEX_URL}/config/thresholds?project_id=${project_id}")
                local scrape=$(echo "$result" | jq -r '.thresholds.scrape // 5 | floor')
                local rebalance=$(echo "$result" | jq -r '.thresholds.rebalance // 50 | floor')
                local autotune=$(echo "$result" | jq -r '.thresholds.autotune // 100 | floor')
                local promotion=$(echo "$result" | jq -r '.thresholds.promotion // 100 | floor')
                local demotion=$(echo "$result" | jq -r '.thresholds.demotion // 500 | floor')
                local prune=$(echo "$result" | jq -r '.thresholds.prune_floor // 0.3')
                local decay=$(echo "$result" | jq -r '.thresholds.decay_rate // 0.95')

                echo -e "${DIM}Triggers (stops):${NC}"
                printf "  %-20s every %s\n" "Scrape:" "${scrape}"
                printf "  %-20s every %s\n" "Rebalance:" "${rebalance}"
                printf "  %-20s every %s\n" "Autotune:" "${autotune}"
                echo ""
                echo -e "${DIM}Qualification (checked during autotune):${NC}"
                printf "  %-20s %s\n" "Promotion:" "≥${promotion} hits → context→core"
                printf "  %-20s %s\n" "Demotion:" "${demotion} intents without hit → core→context"
                printf "  %-20s %s\n" "Prune:" "<${prune} hits → removed"
                printf "  %-20s %s\n" "Decay:" "${decay} per cycle"
                return 0
            fi

            if [ "$mode" = "test" ] || [ "$mode" = "prod" ]; then
                local result=$(curl -s -X POST "${INDEX_URL}/config/thresholds" \
                    -H "Content-Type: application/json" \
                    -d "{\"project_id\":\"${project_id}\",\"mode\":\"${mode}\"}")
                local success=$(echo "$result" | jq -r '.success // false')
                if [ "$success" = "true" ]; then
                    echo -e "${GREEN}✓ Thresholds set to ${mode} mode${NC}"
                    if [ "$mode" = "test" ]; then
                        echo -e "${DIM}  Triggers: Scrape 5, Rebalance 10, Autotune 25${NC}"
                        echo -e "${DIM}  Qualify:  Promote ≥25, Demote 100, Prune <0.3, Decay 0.90${NC}"
                    else
                        echo -e "${DIM}  Triggers: Scrape 5, Rebalance 50, Autotune 100${NC}"
                        echo -e "${DIM}  Qualify:  Promote ≥100, Demote 500, Prune <0.3, Decay 0.95${NC}"
                    fi
                else
                    echo -e "${RED}Failed to set thresholds${NC}"
                    return 1
                fi
            else
                echo "Usage: aoa config thresholds [test|prod]"
                echo ""
                echo "  test    Use compressed thresholds (10x faster)"
                echo "  prod    Use production thresholds (default)"
                echo ""
                echo "Run without argument to show current values."
                return 1
            fi
            ;;
        *)
            echo "Usage: aoa config <setting>"
            echo ""
            echo "Settings:"
            echo "  thresholds [test|prod]  Set validation thresholds"
            ;;
    esac
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

    # Handle --json flag first
    if [[ "${1:-}" == "--json" ]] || [[ "${1:-}" == "-j" ]]; then
        echo "$metrics" | jq .
        return 0
    fi

    # Parse metrics
    local hit_pct=$(echo "$metrics" | jq -r '.rolling.hit_at_5_pct // 0')
    local evaluated=$(echo "$metrics" | jq -r '.rolling.evaluated // 0')
    local hits=$(echo "$metrics" | jq -r '.rolling.hits // 0')
    local tokens_saved=$(echo "$metrics" | jq -r '.savings.tokens // 0')
    local time_sec_low=$(echo "$metrics" | jq -r '.savings.time_sec_low // 0')
    local time_sec_high=$(echo "$metrics" | jq -r '.savings.time_sec_high // 0')
    local trend=$(echo "$metrics" | jq -r '.trend // "unknown"')

    # Format tokens with 2 decimals (standard format)
    format_tokens() {
        local n=$1
        if [ "$n" -ge 1000000000 ]; then
            awk "BEGIN {printf \"%.2fB\", $n/1000000000}"
        elif [ "$n" -ge 1000000 ]; then
            awk "BEGIN {printf \"%.2fM\", $n/1000000}"
        elif [ "$n" -ge 1000 ]; then
            awk "BEGIN {printf \"%.2fk\", $n/1000}"
        else
            echo "$n"
        fi
    }
    local tokens_fmt=$(format_tokens $tokens_saved)

    # Format hit percentage
    local hit_int=$(printf "%.0f" "$hit_pct")

    # Format time (simple: just primary unit)
    format_time_simple() {
        local sec=$1
        if [ "$sec" -ge 3600 ]; then
            awk "BEGIN {printf \"%.0fh\", $sec / 3600}"
        elif [ "$sec" -ge 60 ]; then
            awk "BEGIN {printf \"%.0fm\", $sec / 60}"
        else
            echo "${sec}s"
        fi
    }
    local time_low_int=$(printf "%.0f" "$time_sec_low")
    local time_high_int=$(printf "%.0f" "$time_sec_high")
    local t_low=$(format_time_simple $time_low_int)
    local t_high=$(format_time_simple $time_high_int)
    local time_display
    if [ "$t_low" = "$t_high" ]; then
        time_display="~${t_low}"
    else
        time_display="${t_low}-${t_high}"
    fi

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
    echo -e "  Tokens:       ${GREEN}↓${tokens_fmt}${NC}"
    echo -e "  Time:         ${GREEN}⚡${time_display}${NC}"
    echo ""
    echo -e "${DIM}Full JSON: aoa metrics --json${NC}"
}

cmd_history() {
    local limit="${1:-20}"

    local result=$(curl -s "${STATUS_URL}/history?limit=${limit}")
    local count=$(echo "$result" | jq -r '.events | length // 0')

    if [ "$count" -eq 0 ]; then
        echo -e "${DIM}No history recorded yet${NC}"
        return 0
    fi

    printf "${CYAN}${BOLD}📜 %s events${NC}\n" "$count"
    echo "$result" | jq -r '.events[] |
        if .type == "request" then
            "  [\(.ts | strftime("%H:%M:%S"))] \(.model) in:\(.input) out:\(.output) $\(.cost)"
        elif .type == "model_switch" then
            "  [\(.ts | strftime("%H:%M:%S"))] -> \(.model)"
        elif .type == "block" then
            "  [\(.ts | strftime("%H:%M:%S"))] BLOCKED \(.block_type)"
        else
            "  [\(.ts | strftime("%H:%M:%S"))] \(.type)"
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

cmd_wipe() {
    # Full project data wipe with confirmation
    local project_id=$(get_project_id)
    local project_root=$(get_project_root)
    local home_file="$project_root/.aoa/home.json"

    if [ -z "$project_id" ]; then
        echo -e "${RED}✗ No project ID found${NC}"
        echo -e "${DIM}Run 'aoa init' first${NC}"
        return 1
    fi

    # Show warning
    echo -e "${YELLOW}${BOLD}⚠  WARNING: Full Project Wipe${NC}"
    echo ""
    echo -e "This will remove ALL data for this project:"
    echo -e "  ${DIM}•${NC} Domains (seeded and learned)"
    echo -e "  ${DIM}•${NC} Intent history"
    echo -e "  ${DIM}•${NC} Semantic tags and enrichment"
    echo -e "  ${DIM}•${NC} Learning counters and stats"
    echo ""
    echo -e "${DIM}From: ${home_file}${NC}"
    echo -e "  project_root: ${CYAN}${project_root}${NC}"
    echo -e "  project_id:   ${DIM}${project_id}${NC}"
    echo ""

    # Check for --force flag
    if [[ "$1" == "--force" || "$1" == "-f" ]]; then
        echo -e "${DIM}Skipping confirmation (--force)${NC}"
    else
        echo -n -e "Type ${BOLD}yes${NC} to confirm: "
        read -r confirm
        if [[ "$confirm" != "yes" ]]; then
            echo -e "${DIM}Aborted${NC}"
            return 0
        fi
    fi

    echo ""
    echo -e "${DIM}Wiping project data...${NC}"

    # 1. Clear job queue
    local jobs_result=$(curl -s -X POST "${INDEX_URL}/jobs/clear" \
        -H "Content-Type: application/json" \
        -d "{\"project_id\": \"${project_id}\"}" 2>/dev/null)
    local jobs_cleared=$(echo "$jobs_result" | jq -r '.cleared // 0' 2>/dev/null || echo "0")
    echo -e "  ${DIM}•${NC} Cleared ${jobs_cleared} jobs"

    # 2. Wipe Redis data (DELETE /project/<project_id>)
    local result=$(curl -s -X DELETE "${INDEX_URL}/project/${project_id}" 2>/dev/null)
    local success=$(echo "$result" | jq -r '.success // false')
    local deleted=$(echo "$result" | jq -r '.redis_deleted.total // 0')

    if [[ "$success" == "true" ]]; then
        echo -e "  ${DIM}•${NC} Wiped ${deleted} Redis keys"
    fi

    # 3. Rebuild keyword matcher to clear in-memory cache
    curl -sf -X POST "${INDEX_URL}/keywords/rebuild?project_id=${project_id}" > /dev/null 2>&1
    echo -e "  ${DIM}•${NC} Cleared keyword cache"

    # 4. Clear local domain files
    local domains_dir="$project_root/.aoa/domains"
    if [ -d "$domains_dir" ]; then
        rm -rf "$domains_dir"/*
        echo -e "  ${DIM}•${NC} Cleared local domain files"
    fi

    echo ""
    echo -e "${GREEN}✓${NC} Wipe complete"
    echo -e "${DIM}Run /aoa-start skill to reinitialize${NC}"
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
