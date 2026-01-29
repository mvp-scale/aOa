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
    # CLI-001: Check for API failure
    if [ -z "$result" ]; then
        echo "Error: API unavailable at ${INDEX_URL}" >&2
        return 1
    fi
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
    # CLI-001: Check for API failure
    if [ -z "$result" ]; then
        echo "Error: API unavailable at ${INDEX_URL}" >&2
        return 1
    fi

    # Filter to directory if specified
    if [ "$dir" != "." ]; then
        result=$(echo "$result" | jq --arg dir "$dir" '.results |= map(select(.path | startswith($dir)))')
    fi

    local count=$(echo "$result" | jq -r '.results | length')
    local dirs=$(echo "$result" | jq -r '[.results[].path | split("/")[:-1] | join("/")] | unique | length')

    # Unix tree implementation with proper │ ├── └── characters
    echo "."
    echo "$result" | jq -r '.results[].path' | sort | awk '
    {
        paths[NR-1] = $0
        n = NR

        # Extract all parent directories
        path = $0
        while (match(path, /\/[^\/]*$/)) {
            path = substr(path, 1, RSTART - 1)
            if (path != "") dirs[path] = 1
        }
    }
    END {
        # Combine dirs (D) and files (F)
        m = 0
        dir_count = 0
        for (d in dirs) { items[m++] = d "\tD"; dir_count++ }
        for (i = 0; i < n; i++) items[m++] = paths[i] "\tF"

        # Sort by path
        for (i = 0; i < m-1; i++) {
            for (j = i+1; j < m; j++) {
                split(items[i], a, "\t")
                split(items[j], b, "\t")
                if (a[1] > b[1]) {
                    tmp = items[i]; items[i] = items[j]; items[j] = tmp
                }
            }
        }

        # Count children per parent
        for (i = 0; i < m; i++) {
            split(items[i], parts, "\t")
            path = parts[1]
            if (match(path, /\/[^\/]*$/))
                parent = substr(path, 1, RSTART - 1)
            else
                parent = "."
            children[parent]++
        }

        # Print tree
        for (i = 0; i < m; i++) {
            split(items[i], parts, "\t")
            path = parts[1]
            type = parts[2]

            # Get parent and name
            if (match(path, /\/[^\/]*$/)) {
                parent = substr(path, 1, RSTART - 1)
                name = substr(path, RSTART + 1)
            } else {
                parent = "."
                name = path
            }

            # Is this the last child of parent?
            seen[parent]++
            is_last = (seen[parent] == children[parent])

            # Build prefix by walking up ancestors
            depth = split(path, segs, "/") - 1
            prefix = ""

            # For each depth level, check if ancestor was last in its parent
            tmp = ""
            for (d = 0; d < depth; d++) {
                tmp = (d == 0) ? segs[1] : tmp "/" segs[d+1]
                if (was_last[tmp])
                    prefix = prefix "    "
                else
                    prefix = prefix "│   "
            }

            # Branch character
            branch = is_last ? "└── " : "├── "

            # Print (no bold, no trailing /)
            printf "%s%s%s\n", prefix, branch, name
            if (type == "D") was_last[path] = is_last
        }

        # Summary at bottom
        printf "\n%d directories, %d files\n", dir_count, n
    }'
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
    # CLI-001: Check for API failure
    if [ -z "$result" ]; then
        echo "Error: API unavailable at ${INDEX_URL}" >&2
        return 1
    fi
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
    # CLI-001: Check for API failure
    if [ -z "$result" ]; then
        echo "Error: API unavailable at ${INDEX_URL}" >&2
        return 1
    fi

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
    # CLI-001: Check for API failure
    if [ -z "$result" ]; then
        echo "Error: API unavailable at ${INDEX_URL}" >&2
        return 1
    fi

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
