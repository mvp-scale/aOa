# =============================================================================
# SECTION 20: File Discovery Commands
# =============================================================================
#
# PURPOSE
#   File operations with familiar Unix syntax. Find files by pattern, view
#   directory structure, and access file contents.
#
# DEPENDENCIES
#   - 01-constants.sh: INDEX_URL, colors
#   - 02-utils.sh: get_project_id()
#
# COMMANDS PROVIDED
#   cmd_find      Find files by glob pattern
#   cmd_tree      Show directory structure
#   cmd_locate    Fast filename search
#   cmd_head      Show first N lines
#   cmd_tail      Show last N lines
#   cmd_lines     Show specific line range
#   cmd_changes   Recently changed files
#   cmd_files     List indexed files
#
# =============================================================================

cmd_find() {
    # aoa find <pattern>         - Find files by glob pattern
    # aoa find -type py          - Find files by language
    # aoa find "*.py" --recent   - Recently modified Python files
    local pattern=""
    local lang=""
    local mode="recent"
    local limit="50"

    while [[ $# -gt 0 ]]; do
        case "$1" in
            -type|-t)
                lang="$2"
                shift 2
                ;;
            --recent|-r)
                mode="recent"
                shift
                ;;
            --alpha|-a)
                mode="alpha"
                shift
                ;;
            -l|--limit)
                limit="$2"
                shift 2
                ;;
            -*)
                echo "Unknown flag: $1"
                return 1
                ;;
            *)
                pattern="$1"
                shift
                ;;
        esac
    done

    if [ -z "$pattern" ] && [ -z "$lang" ]; then
        echo "Usage: aoa find <pattern> [options]"
        echo ""
        echo "Options:"
        echo "  -type, -t <lang>   Filter by language (py, js, ts, go, etc.)"
        echo "  --recent, -r       Sort by modification time (default)"
        echo "  --alpha, -a        Sort alphabetically"
        echo "  -l, --limit N      Limit results (default: 50)"
        echo ""
        echo "Examples:"
        echo "  aoa find '*.py'          # All Python files"
        echo "  aoa find -type py        # Same, by language"
        echo "  aoa find 'test*.py'      # Test files"
        echo "  aoa find 'src/**/*.ts'   # TypeScript in src/"
        return 1
    fi

    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    local url="${INDEX_URL}/files?mode=${mode}&limit=${limit}${project_param}"
    [ -n "$pattern" ] && url="${url}&match=${pattern}"
    [ -n "$lang" ] && url="${url}&lang=${lang}"

    local result=$(curl -s "$url")
    local count=$(echo "$result" | jq -r '.results | length')

    printf "${CYAN}${BOLD}📁 %s files${NC}\n" "$count"
    echo "$result" | jq -r '.results[] | "  \(.path)"'
}

cmd_tree() {
    # aoa tree [dir]     - Show directory structure
    local dir="${1:-.}"
    local depth="${2:-3}"

    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    # Use /files endpoint and build tree structure
    local result=$(curl -s "${INDEX_URL}/files?mode=alpha&limit=500${project_param}")

    # Filter to directory if specified
    if [ "$dir" != "." ]; then
        result=$(echo "$result" | jq --arg dir "$dir" '.results |= map(select(.path | startswith($dir)))')
    fi

    local count=$(echo "$result" | jq -r '.results | length')
    local dirs=$(echo "$result" | jq -r '[.results[].path | split("/")[:-1] | join("/")] | unique | length')

    printf "${CYAN}${BOLD}🌳 %s dirs, %s files${NC}\n" "$dirs" "$count"
    echo ""

    # Simple tree-like output
    echo "$result" | jq -r '.results[].path' | while read -r path; do
        local indent=$(echo "$path" | tr -cd '/' | wc -c)
        local name=$(basename "$path")
        printf "%*s%s\n" $((indent * 2)) "" "$name"
    done | head -50
}

cmd_locate() {
    # aoa locate <name>   - Fast filename search (uses symbol index)
    local name="$1"

    if [ -z "$name" ]; then
        echo "Usage: aoa locate <filename>"
        echo ""
        echo "Examples:"
        echo "  aoa locate routes       # Find files with 'routes' in name"
        echo "  aoa locate config.py    # Find config.py files"
        return 1
    fi

    # Search in filenames using /files endpoint
    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/files?match=*${name}*&limit=20${project_param}")
    local count=$(echo "$result" | jq -r '.results | length')

    printf "${CYAN}${BOLD}🔍 %s matches${NC}\n" "$count"
    echo "$result" | jq -r '.results[] | "  \(.path)"'
}

cmd_head() {
    # aoa head <file> [n]   - Show first n lines (default: 20)
    local file="$1"
    local lines="${2:-20}"

    if [ -z "$file" ]; then
        echo "Usage: aoa head <file> [lines]"
        echo ""
        echo "Examples:"
        echo "  aoa head cli/aoa        # First 20 lines"
        echo "  aoa head cli/aoa 50     # First 50 lines"
        return 1
    fi

    # Use /file endpoint with line range
    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/file?path=${file}&lines=1-${lines}${project_param}")

    local err=$(echo "$result" | jq -r '.error // empty')
    if [ -n "$err" ]; then
        echo -e "${RED}${err}${NC}"
        return 1
    fi

    printf "${DIM}%s (lines 1-%s)${NC}\n" "$file" "$lines"
    echo "$result" | jq -r '.content // .lines[]'
}

cmd_tail() {
    # aoa tail <file> [n]   - Show last n lines (default: 20)
    local file="$1"
    local lines="${2:-20}"

    if [ -z "$file" ]; then
        echo "Usage: aoa tail <file> [lines]"
        echo ""
        echo "Examples:"
        echo "  aoa tail cli/aoa        # Last 20 lines"
        echo "  aoa tail cli/aoa 50     # Last 50 lines"
        return 1
    fi

    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/file?path=${file}&lines=-${lines}${project_param}")

    local err=$(echo "$result" | jq -r '.error // empty')
    if [ -n "$err" ]; then
        echo -e "${RED}${err}${NC}"
        return 1
    fi

    printf "${DIM}%s (last %s lines)${NC}\n" "$file" "$lines"
    echo "$result" | jq -r '.content // .lines[]'
}

cmd_lines() {
    # aoa lines <file> <range>   - Show specific line range
    local file="$1"
    local range="$2"

    if [ -z "$file" ] || [ -z "$range" ]; then
        echo "Usage: aoa lines <file> <start-end>"
        echo ""
        echo "Examples:"
        echo "  aoa lines cli/aoa 100-150   # Lines 100-150"
        echo "  aoa lines cli/aoa 500-550   # Lines 500-550"
        return 1
    fi

    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/file?path=${file}&lines=${range}${project_param}")

    local err=$(echo "$result" | jq -r '.error // empty')
    if [ -n "$err" ]; then
        echo -e "${RED}${err}${NC}"
        return 1
    fi

    printf "${DIM}%s (lines %s)${NC}\n" "$file" "$range"
    echo "$result" | jq -r '.content // .lines[]'
}

cmd_changes() {
    local input="${1:-5m}"
    local since
    local display

    # Parse duration: 30s, 5m, 1h, 2d -> seconds
    if [[ "$input" =~ ^([0-9]+)([smhd])$ ]]; then
        local num="${BASH_REMATCH[1]}"
        local unit="${BASH_REMATCH[2]}"
        case "$unit" in
            s) since=$num; display="${num}s" ;;
            m) since=$((num * 60)); display="${num}m" ;;
            h) since=$((num * 3600)); display="${num}h" ;;
            d) since=$((num * 86400)); display="${num}d" ;;
        esac
    elif [[ "$input" =~ ^[0-9]+$ ]]; then
        # Plain number = seconds
        since=$input
        display="${input}s"
    else
        echo "Usage: aoa changes [duration]"
        echo ""
        echo "Duration format: 30s, 5m, 1h, 2d (default: 5m)"
        echo ""
        echo "Examples:"
        echo "  aoa changes          # Last 5 minutes"
        echo "  aoa changes 30m      # Last 30 minutes"
        echo "  aoa changes 1h       # Last hour"
        echo "  aoa changes 2d       # Last 2 days"
        return 1
    fi

    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/changes?since=${since}${project_param}")
    local count=$(echo "$result" | jq -r '.results | length // 0' 2>/dev/null || echo "0")

    printf "${CYAN}${BOLD}📝 %s changed files${NC} ${DIM}(last %s)${NC}\n" "$count" "$display"

    if [ "$count" -gt 0 ] 2>/dev/null; then
        echo "$result" | jq -r '.results[] | "  \(.file) (\(.changes) changes)"' 2>/dev/null || true
    fi
}

cmd_files() {
    local limit="${1:-20}"
    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/files?limit=${limit}${project_param}")
    local count=$(echo "$result" | jq -r '.results | length')

    printf "${CYAN}${BOLD}📁 %s files${NC}\n" "$count"
    echo "$result" | jq -r '.results[] | "  \(.path)"'
}
