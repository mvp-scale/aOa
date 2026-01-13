# =============================================================================
# SECTION 40: Knowledge Repository Commands
# =============================================================================
#
# PURPOSE
#   Manage external knowledge repositories (cloned from GitHub). These provide
#   searchable reference documentation for frameworks and libraries.
#
# DEPENDENCIES
#   - 01-constants.sh: INDEX_URL, colors
#   - 02-utils.sh: get_project_id()
#
# COMMANDS PROVIDED
#   cmd_repo        Main repo command dispatcher
#   cmd_repo_list   List available repos
#   cmd_repo_add    Clone and index a repo
#   cmd_repo_remove Remove a repo
#   cmd_repo_search Search within a repo
#   cmd_repo_multi  Multi-term repo search
#   cmd_repo_files  List repo files
#   cmd_repo_file   Get file content from repo
#   cmd_repo_deps   Show file dependencies
#
# =============================================================================

# =============================================================================
# Knowledge Repo Commands
# =============================================================================

cmd_repo() {
    local subcmd="${1:-list}"
    shift || true

    case "$subcmd" in
        list|ls)
            cmd_repo_list "$@"
            ;;
        add)
            cmd_repo_add "$@"
            ;;
        remove|rm)
            cmd_repo_remove "$@"
            ;;
        *)
            # Assume it's a repo name - dispatch to repo-specific commands
            local repo_name="$subcmd"
            local repo_cmd="${1:-help}"
            shift || true

            case "$repo_cmd" in
                search|s)
                    cmd_repo_search "$repo_name" "$@"
                    ;;
                multi|m)
                    cmd_repo_multi "$repo_name" "$@"
                    ;;
                files|f)
                    cmd_repo_files "$repo_name" "$@"
                    ;;
                file)
                    cmd_repo_file "$repo_name" "$@"
                    ;;
                deps)
                    cmd_repo_deps "$repo_name" "$@"
                    ;;
                *)
                    echo -e "${BOLD}Repo: ${repo_name}${NC}"
                    echo ""
                    echo "Commands:"
                    echo "  aoa repo ${repo_name} search <term>   Search in ${repo_name}"
                    echo "  aoa repo ${repo_name} multi <t1,t2>  Multi-term search"
                    echo "  aoa repo ${repo_name} files [pat]    List files"
                    echo "  aoa repo ${repo_name} file <path>    Get file content"
                    echo "  aoa repo ${repo_name} deps <file>    Get dependencies"
                    ;;
            esac
            ;;
    esac
}

cmd_repo_list() {
    echo -e "${BOLD}Knowledge Repos${NC}"
    echo ""

    local result=$(curl -s "${INDEX_URL}/repos")
    local count=$(echo "$result" | jq '.repos | length')

    if [ "$count" == "0" ]; then
        echo -e "${DIM}No knowledge repos. Add one with:${NC}"
        echo "  aoa repo add <name> <git-url>"
        return
    fi

    echo "$result" | jq -r '.repos[] | "  \(.name): \(.files) files, \(.symbols) symbols"'
}

cmd_repo_add() {
    local name="$1"
    local url="$2"

    if [ -z "$name" ] || [ -z "$url" ]; then
        echo "Usage: aoa repo add <name> <git-url>"
        echo ""
        echo "Examples:"
        echo "  aoa repo add flask https://github.com/pallets/flask"
        echo "  aoa repo add react https://github.com/facebook/react"
        return 1
    fi

    echo -e "${DIM}Cloning and indexing ${name}...${NC}"

    local result=$(curl -s -X POST "${INDEX_URL}/repos" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"${name}\", \"url\": \"${url}\"}")

    local success=$(echo "$result" | jq -r '.success // false')

    if [ "$success" == "true" ]; then
        local msg=$(echo "$result" | jq -r '.message')
        echo -e "${GREEN}${msg}${NC}"
    else
        local err=$(echo "$result" | jq -r '.error // "Unknown error"')
        echo -e "${RED}Failed: ${err}${NC}"
        return 1
    fi
}

cmd_repo_remove() {
    local name="$1"

    if [ -z "$name" ]; then
        echo "Usage: aoa repo remove <name>"
        return 1
    fi

    echo -e "${DIM}Removing ${name}...${NC}"

    local result=$(curl -s -X DELETE "${INDEX_URL}/repos/${name}")

    local success=$(echo "$result" | jq -r '.success // false')

    if [ "$success" == "true" ]; then
        echo -e "${GREEN}Repo '${name}' removed${NC}"
    else
        local err=$(echo "$result" | jq -r '.error // "Unknown error"')
        echo -e "${RED}Failed: ${err}${NC}"
        return 1
    fi
}

cmd_repo_search() {
    local repo_name="$1"
    local query="$2"
    local mode="${3:-recent}"
    local limit="${4:-20}"

    if [ -z "$query" ]; then
        echo "Usage: aoa repo ${repo_name} search <term> [mode] [limit]"
        return 1
    fi

    local result=$(curl -s "${INDEX_URL}/repo/${repo_name}/symbol?q=${query}&mode=${mode}&limit=${limit}")

    # Check for error
    local err=$(echo "$result" | jq -r '.error // empty')
    if [ -n "$err" ]; then
        echo -e "${RED}${err}${NC}"
        return 1
    fi

    local ms=$(echo "$result" | jq -r '.ms // 0')
    local count=$(echo "$result" | jq -r '.results | length')

    # Single punchy line with repo indicator
    printf "${CYAN}${BOLD}⚡ %s hits${NC} ${DIM}│${NC} ${GREEN}%.2fms${NC} ${DIM}│${NC} ${YELLOW}%s${NC}\n" "$count" "$ms" "$repo_name"

    echo "$result" | jq -r '.results[] | "  \(.file):\(.line)"' 2>/dev/null
}

cmd_repo_multi() {
    local repo_name="$1"
    local terms="$2"
    local mode="${3:-recent}"
    local limit="${4:-20}"

    if [ -z "$terms" ]; then
        echo "Usage: aoa repo ${repo_name} multi <term1,term2,...> [mode] [limit]"
        return 1
    fi

    local json_terms=$(echo "$terms" | tr ',' '\n' | jq -R . | jq -s .)

    local result=$(curl -s -X POST "${INDEX_URL}/repo/${repo_name}/multi" \
        -H "Content-Type: application/json" \
        -d "{\"terms\": ${json_terms}, \"mode\": \"${mode}\", \"limit\": ${limit}}")

    local err=$(echo "$result" | jq -r '.error // empty')
    if [ -n "$err" ]; then
        echo -e "${RED}${err}${NC}"
        return 1
    fi

    local ms=$(echo "$result" | jq -r '.ms // 0')

    echo -e "${DIM}Found in ${ms}ms (${repo_name}):${NC}"
    echo "$result" | jq -r '.results[] | "\(.file):\(.line)"' 2>/dev/null || echo "No results"
}

cmd_repo_files() {
    local repo_name="$1"
    local pattern="$2"
    local mode="${3:-recent}"
    local limit="${4:-30}"

    local url="${INDEX_URL}/repo/${repo_name}/files?mode=${mode}&limit=${limit}"
    [ -n "$pattern" ] && url="${url}&match=${pattern}"

    local result=$(curl -s "$url")

    local err=$(echo "$result" | jq -r '.error // empty')
    if [ -n "$err" ]; then
        echo -e "${RED}${err}${NC}"
        return 1
    fi

    echo "$result" | jq -r '.results[] | "\(.path) (\(.language))"'
}

cmd_repo_file() {
    local repo_name="$1"
    local path="$2"
    local lines="$3"

    if [ -z "$path" ]; then
        echo "Usage: aoa repo ${repo_name} file <path> [lines]"
        echo "  lines: e.g., 10-50"
        return 1
    fi

    local url="${INDEX_URL}/repo/${repo_name}/file?path=${path}"
    [ -n "$lines" ] && url="${url}&lines=${lines}"

    local result=$(curl -s "$url")

    local err=$(echo "$result" | jq -r '.error // empty')
    if [ -n "$err" ]; then
        echo -e "${RED}${err}${NC}"
        return 1
    fi

    echo "$result" | jq -r '.content'
}

cmd_repo_deps() {
    local repo_name="$1"
    local file="$2"
    local direction="${3:-outgoing}"

    if [ -z "$file" ]; then
        echo "Usage: aoa repo ${repo_name} deps <file> [outgoing|incoming]"
        return 1
    fi

    local result=$(curl -s "${INDEX_URL}/repo/${repo_name}/deps?file=${file}&direction=${direction}")

    local err=$(echo "$result" | jq -r '.error // empty')
    if [ -n "$err" ]; then
        echo -e "${RED}${err}${NC}"
        return 1
    fi

    echo -e "${BOLD}${direction} dependencies for ${file}:${NC}"
    echo "$result" | jq -r '.dependencies[]' 2>/dev/null || echo "  (none)"
}

