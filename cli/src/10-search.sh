# =============================================================================
# SECTION 10: Core Search Commands - HOT PATH
# =============================================================================
#
# PURPOSE
#   Primary search interface. These commands run constantly and are optimized
#   for O(1) response time via pre-built inverted index.
#
# PERFORMANCE
#   grep:  <5ms  - Hash lookup in inverted index, pre-cached content
#   egrep: <20ms - Regex scan on working set (~30-50 recent files)
#
# DEPENDENCIES
#   - 01-constants.sh: INDEX_URL, colors
#   - 02-utils.sh: get_project_id()
#
# COMMANDS PROVIDED
#   cmd_grep       O(1) indexed symbol search (primary)
#   cmd_egrep      Regex search on working set
#   cmd_search     Alias for cmd_grep
#   cmd_multi      Alias for cmd_grep -a
#
# UNIX PARITY
#   Supports: -i, -w, -c, -q, -e, -E, -r, -n, -H, -F
#   See CLAUDE.md for full parity table
#
# =============================================================================

cmd_grep() {
    # aoa grep <term>           - Symbol search (O(1) indexed)
    # aoa grep "a b c"          - Multi-term OR search, ranked
    # aoa grep -a term1,term2   - Multi-term AND (all terms required)
    # aoa grep -i <term>        - Case insensitive
    # aoa grep --since 1h       - Files modified in last hour
    # aoa grep --today          - Files modified today (last 24h)
    # aoa grep --json           - Output as JSON
    # aoa grep -c               - Count only
    # aoa grep -q               - Quiet (exit code only)
    local and_mode=false
    local case_insensitive=false
    local word_boundary=false
    local json_output=false
    local count_only=false
    local quiet=false
    local query=""
    local mode="recent"
    local limit="20"
    local since=""
    local before=""

    # Parse flags
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -a|--and)
                and_mode=true
                shift
                ;;
            -i|--ignore-case)
                case_insensitive=true
                shift
                ;;
            -w|--word)
                word_boundary=true
                shift
                ;;
            --json)
                json_output=true
                shift
                ;;
            -c|--count)
                count_only=true
                shift
                ;;
            -q|--quiet)
                quiet=true
                shift
                ;;
            -l|--limit)
                limit="$2"
                shift 2
                ;;
            -m|--mode)
                mode="$2"
                shift 2
                ;;
            --since)
                since="$2"
                shift 2
                ;;
            --before)
                before="$2"
                shift 2
                ;;
            --today)
                since="24h"
                shift
                ;;
            -E|--extended-regexp)
                # Unix parity: grep -E routes to egrep
                shift
                cmd_egrep "$@"
                return $?
                ;;
            -r|-R|--recursive)
                # No-op: aOa is always recursive (searches entire index)
                shift
                ;;
            -n|--line-number)
                # No-op: aOa always shows line numbers
                shift
                ;;
            -H|--with-filename)
                # No-op: aOa always shows filenames
                shift
                ;;
            -F|--fixed-strings)
                # No-op: aOa grep is already literal/symbol-based
                shift
                ;;
            -e|--regexp)
                # Unix parity: grep -e foo -e bar → OR search
                if [ -z "$query" ]; then
                    query="$2"
                else
                    query="$query $2"
                fi
                shift 2
                ;;
            -*)
                echo "Unknown flag: $1"
                return 1
                ;;
            *)
                if [ -z "$query" ]; then
                    query="$1"
                fi
                shift
                ;;
        esac
    done

    # Parse time strings to seconds
    parse_time_to_seconds() {
        local time_str="$1"
        if [[ "$time_str" =~ ^([0-9]+)([smhd])$ ]]; then
            local num="${BASH_REMATCH[1]}"
            local unit="${BASH_REMATCH[2]}"
            case "$unit" in
                s) echo "$num" ;;
                m) echo $((num * 60)) ;;
                h) echo $((num * 3600)) ;;
                d) echo $((num * 86400)) ;;
            esac
        else
            echo "$time_str"
        fi
    }

    local since_seconds=""
    local before_seconds=""
    [ -n "$since" ] && since_seconds=$(parse_time_to_seconds "$since")
    [ -n "$before" ] && before_seconds=$(parse_time_to_seconds "$before")

    if [ -z "$query" ]; then
        echo "Usage: aoa grep <term> [options]"
        echo ""
        echo "Options:"
        echo "  -a, --and          AND mode: all comma-separated terms required"
        echo "  -i, --ignore-case  Case insensitive search"
        echo "  -w, --word         Word boundary match"
        echo "  -l, --limit N      Limit results (default: 20)"
        echo "  --since TIME       Files modified since TIME (e.g., 1h, 7d, 30m)"
        echo "  --before TIME      Files modified before TIME"
        echo "  --today            Shortcut for --since 24h"
        echo "  --json             Output as JSON"
        echo "  -c, --count        Show count only"
        echo "  -q, --quiet        Quiet mode (exit code only)"
        echo ""
        echo "Examples:"
        echo "  aoa grep handleAuth          # Symbol search"
        echo "  aoa grep \"auth token\"        # OR search (either term)"
        echo "  aoa grep -a auth,session     # AND search (all terms)"
        echo "  aoa grep auth --since 1h     # Modified in last hour"
        echo "  aoa grep auth --today        # Modified today"
        echo "  aoa grep auth --json         # JSON output"
        echo "  aoa grep auth -c             # Count only"
        return 1
    fi

    # === Smart Routing: detect regex patterns and route appropriately ===

    # Simple pipe OR: foo|bar or foo\|bar → convert to space OR
    # This is the #1 friction point - users expect | to mean OR
    if [[ "$query" =~ ^[a-zA-Z0-9_]+([\\]?\|[a-zA-Z0-9_]+)+$ ]]; then
        # Convert pipes (escaped or not) to spaces for OR search
        local converted="${query//\\|/ }"
        converted="${converted//|/ }"
        echo -e "${DIM}(| → OR search: ${converted})${NC}"
        query="$converted"

    # Glob pattern: starts with * or looks like *.ext → suggest aoa find
    elif [[ "$query" == \** || "$query" == *'**'* ]]; then
        echo -e "${YELLOW}Tip:${NC} Use 'aoa find' for file patterns"
        echo -e "  ${DIM}aoa find \"$query\"${NC}"
        return 1

    # Complex regex: contains metacharacters that won't work in symbol search
    # Route to egrep for actual regex matching
    # Check for: . * + ? ^ $ [ ] { } ( ) \
    elif [[ "$query" == *'.'* || "$query" == *'*'* || "$query" == *'+'* || \
            "$query" == *'?'* || "$query" == *'^'* || "$query" == *'$'* || \
            "$query" == *'['* || "$query" == *'\'* || "$query" == *'('* ]] && \
         [[ ! "$query" =~ ^[a-zA-Z0-9_\ ]+$ ]]; then
        # GL-046 Unified: Silent routing - no notice needed, output is identical
        cmd_egrep "$query"
        return $?
    fi

    # === End Smart Routing ===

    # Get current project context
    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project=${project_id}"
    fi

    local result
    local ms
    local count

    if [ "$and_mode" = true ]; then
        # AND mode: use /multi endpoint
        export AOA_SEARCH_TYPE="multi-and"
        local json_terms=$(echo "$query" | tr ',' '\n' | jq -R . | jq -s .)

        local body="{\"terms\": ${json_terms}, \"mode\": \"${mode}\", \"limit\": ${limit}"
        [ -n "$project_id" ] && body="${body}, \"project\": \"${project_id}\""
        [ -n "$since_seconds" ] && body="${body}, \"since\": ${since_seconds}"
        [ -n "$before_seconds" ] && body="${body}, \"before\": ${before_seconds}"
        body="${body}}"

        result=$(curl -s -X POST "${INDEX_URL}/multi" \
            -H "Content-Type: application/json" \
            -d "$body")

        ms=$(echo "$result" | jq -r '.ms // 0')
        count=$(echo "$result" | jq -r '.results | length')

        # Handle output flags
        if [ "$quiet" = true ]; then
            [ "$count" -gt 0 ] && return 0 || return 1
        elif [ "$json_output" = true ]; then
            echo "$result"
        elif [ "$count_only" = true ]; then
            echo "$count"
        else
            # GL-040: Enhanced grep with ranked results and outlines for top 5
            display_ranked_grep_results "$result" "$query" "$ms" "$count"
        fi
    else
        # GL-048: Space-separated terms → OR search via /multi endpoint
        # Unix parity: "foo bar" means search for foo OR bar
        if [[ "$query" == *" "* ]]; then
            # Split on spaces and use /multi for OR search
            export AOA_SEARCH_TYPE="multi-or"
            local json_terms=$(echo "$query" | tr ' ' '\n' | grep -v '^$' | jq -R . | jq -s .)

            local body="{\"terms\": ${json_terms}, \"mode\": \"${mode}\", \"limit\": ${limit}, \"operator\": \"or\""
            [ -n "$project_id" ] && body="${body}, \"project\": \"${project_id}\""
            [ -n "$since_seconds" ] && body="${body}, \"since\": ${since_seconds}"
            [ -n "$before_seconds" ] && body="${body}, \"before\": ${before_seconds}"
            body="${body}}"

            result=$(curl -s -X POST "${INDEX_URL}/multi" \
                -H "Content-Type: application/json" \
                -d "$body")
        else
            # Fast symbol search with /symbol endpoint (O(1) indexed)
            export AOA_SEARCH_TYPE="indexed"
            local encoded_query=$(printf '%s' "$query" | jq -sRr @uri)

            # Build query params
            local params="q=${encoded_query}&mode=${mode}&limit=${limit}${project_param}"
            [ "$case_insensitive" = true ] && params="${params}&ci=1"
            [ "$word_boundary" = true ] && params="${params}&word=1"
            [ -n "$since_seconds" ] && params="${params}&since=${since_seconds}"
            [ -n "$before_seconds" ] && params="${params}&before=${before_seconds}"

            result=$(curl -s "${INDEX_URL}/symbol?${params}")
        fi
        ms=$(echo "$result" | jq -r '.ms // 0')
        count=$(echo "$result" | jq -r '.results | length')

        # Handle output flags
        if [ "$quiet" = true ]; then
            [ "$count" -gt 0 ] && return 0 || return 1
        elif [ "$json_output" = true ]; then
            echo "$result"
        elif [ "$count_only" = true ]; then
            echo "$count"
        else
            # GL-040: Enhanced grep with ranked results and outlines for top 5
            display_ranked_grep_results "$result" "$query" "$ms" "$count"
        fi
    fi
}

# GL-046.4: O(1) grep display - uses enriched API response, no curl loops
# Shows all results ranked by intent, in grep-like compact format
display_ranked_grep_results() {
    local result="$1"
    local query="$2"
    local ms="$3"
    local count="$4"

    # If no results, show simple message
    if [ "$count" -lt 1 ]; then
        printf "${CYAN}${BOLD}⚡ aOa${NC} ${DIM}│${NC} 0 hits ${DIM}│${NC} ${GREEN}%.2fms${NC}\n" "$ms"
        return 0
    fi

    # Count unique files
    local file_count=$(echo "$result" | jq -r '[.results[].file] | unique | length')

    # Get rolling intent from API response (GL-045)
    # Note: API returns tags with # prefix already, don't double it
    local rolling_intent=$(echo "$result" | jq -r '.rolling_intent // [] | join(" ")' 2>/dev/null)

    # Header: hits | files | timing | intent tags
    printf "${CYAN}${BOLD}⚡ aOa${NC} ${DIM}│${NC} ${BOLD}%s${NC} hits ${DIM}│${NC} %s files ${DIM}│${NC} ${GREEN}%.2fms${NC}" "$count" "$file_count" "$ms"
    if [ -n "$rolling_intent" ] && [ "$rolling_intent" != "" ]; then
        printf " ${DIM}│${NC} ${CYAN}%s${NC}" "$rolling_intent"
    fi
    printf "\n\n"

    # GL-047.8: Hierarchical format for AI-readable context
    # Format: file:Parent().method(params)[range]:line content
    # Each line is self-contained - no state tracking needed
    echo "$result" | jq -r '
        # Helper: extract params from signature like "def foo(self, x: int) -> str" -> "(x)"
        def extract_params:
            if . == null then "()"
            elif test("\\(.*\\)") then
                # Extract content between ( and ), drop self, simplify types
                capture("\\((?<p>[^)]*)\\)").p
                | split(",")
                | map(gsub("^\\s+|\\s+$"; ""))  # trim
                | map(select(. != "self" and . != ""))  # drop self
                | map(split(":")[0] | gsub("^\\s+|\\s+$"; ""))  # drop type annotations
                | "(" + join(", ") + ")"
            else "()"
            end;

        .results[] |
        # Extract params from signature
        (.signature | extract_params) as $params |
        # Build hierarchical scope: Parent().method(params)[range] or symbol(params)[range] or <module>
        (
            if .parent_name then
                # Has parent: Parent().method(params)[method-range]
                "\(.parent_name)().\(.symbol // "<anon>")\($params)[\(.start_line // .line)-\(.end_line // .line)]"
            elif .symbol then
                # No parent: symbol(params)[range]
                "\(.symbol)\($params)[\(.start_line // .line)-\(.end_line // .line)]"
            else
                # Module level
                "<module>"
            end
        ) as $scope |
        # Build tags (max 3)
        ((.tags // []) | .[0:3] | join(" ")) as $tags |
        # Format: file:scope:line content  #tags
        "\u001b[1m\(.file)\u001b[0m:\u001b[33m\($scope)\u001b[0m:\u001b[2m\(.line)\u001b[0m" +
        (if .content then " \(.content)" else "" end) +
        (if ($tags | length) > 0 then "  \u001b[36m\($tags)\u001b[0m" else "" end)
    '
}

# Keep old function name for backwards compatibility
display_ranked_grep_results_verbose() {
    display_ranked_grep_results "$@"
}

# GL-045: Display semantic grep results (content search with structural context)
display_semantic_grep_results() {
    local result="$1"
    local query="$2"
    local ms="$3"
    local files_matched="$4"

    # Get search intent and total matches
    local search_intent=$(echo "$result" | jq -r '.search_intent | join(" ")' 2>/dev/null)
    local total_matches=$(echo "$result" | jq -r '.total_matches // 0' 2>/dev/null)

    # Header: hits | files | timing | intent tags
    if [ -n "$search_intent" ] && [ "$search_intent" != "" ]; then
        printf "${CYAN}${BOLD}⚡ aOa${NC} ${DIM}│${NC} ${BOLD}%s${NC} hits ${DIM}│${NC} %s files ${DIM}│${NC} ${GREEN}%.2fms${NC} ${DIM}│${NC} ${CYAN}%s${NC}\n" "$total_matches" "$files_matched" "$ms" "$search_intent"
    else
        printf "${CYAN}${BOLD}⚡ aOa${NC} ${DIM}│${NC} ${BOLD}%s${NC} hits ${DIM}│${NC} %s files ${DIM}│${NC} ${GREEN}%.2fms${NC}\n" "$total_matches" "$files_matched" "$ms"
    fi

    # If no results, done
    if [ "$files_matched" -lt 1 ]; then
        return 0
    fi

    echo ""

    # Display results - already sorted by intent score from API
    local shown=0
    local max_files=5
    [ "$files_matched" -gt 20 ] && max_files=3

    # Process each result file
    echo "$result" | jq -c '.results[]' 2>/dev/null | while read -r file_result; do
        [ $shown -ge $max_files ] && break

        local file=$(echo "$file_result" | jq -r '.file')
        local file_tags=$(echo "$file_result" | jq -r '.file_tags | .[0:3] | map("#" + .) | join(" ")' 2>/dev/null)
        local score=$(echo "$file_result" | jq -r '.score // 0' 2>/dev/null)

        # File header with tags
        if [ -n "$file_tags" ] && [ "$file_tags" != "" ]; then
            printf "${BOLD}%s${NC}  ${DIM}%s${NC}\n" "$file" "$file_tags"
        else
            printf "${BOLD}%s${NC}\n" "$file"
        fi

        # Display matches with their containing symbols
        local match_count=$(echo "$file_result" | jq '.matches | length')
        local idx=0

        echo "$file_result" | jq -c '.matches[]' 2>/dev/null | while read -r match; do
            local line=$(echo "$match" | jq -r '.line')
            local text=$(echo "$match" | jq -r '.text')
            local symbol=$(echo "$match" | jq -c '.symbol // null')
            local symbol_tags=$(echo "$match" | jq -r '.symbol_tags | .[0:2] | join(" ")' 2>/dev/null)

            idx=$((idx + 1))
            local prefix="├─"
            [ $idx -eq $match_count ] && prefix="└─"

            if [ "$symbol" != "null" ]; then
                local sym_name=$(echo "$symbol" | jq -r '.name')
                local sym_kind=$(echo "$symbol" | jq -r '.kind')
                local sym_start=$(echo "$symbol" | jq -r '.start_line')
                local sym_end=$(echo "$symbol" | jq -r '.end_line')

                if [ -n "$symbol_tags" ] && [ "$symbol_tags" != "" ]; then
                    printf "  %s ${CYAN}%s${NC} %s [%s-%s]  ${DIM}%s${NC}\n" "$prefix" "$sym_kind" "$sym_name" "$sym_start" "$sym_end" "$symbol_tags"
                else
                    printf "  %s ${CYAN}%s${NC} %s [%s-%s]\n" "$prefix" "$sym_kind" "$sym_name" "$sym_start" "$sym_end"
                fi
                printf "  │     ${DIM}L%s: %s${NC}\n" "$line" "$text"
            else
                # No containing symbol, show line directly
                printf "  %s ${DIM}L%s: %s${NC}\n" "$prefix" "$line" "$text"
            fi
        done

        echo ""
        shown=$((shown + 1))
    done

    # Show remaining files
    local remaining=$((files_matched - shown))
    if [ $remaining -gt 0 ]; then
        printf "${DIM}─── %d more files ───${NC}\n" "$remaining"

        local idx=0
        local max_remaining=10
        echo "$result" | jq -r '.results['"$max_files"':] | .[0:'"$max_remaining"'] | .[] | "\(.file) (\(.match_count) hits)"' 2>/dev/null | while read -r line; do
            printf "  %s\n" "$line"
            idx=$((idx + 1))
        done

        local hidden=$((remaining - max_remaining))
        [ $hidden -gt 0 ] && printf "  ${DIM}... and %d more${NC}\n" "$hidden"
    fi
}

# Deprecated: use cmd_grep instead
cmd_search() {
    cmd_grep "$@"
}

# Deprecated: use cmd_grep -a instead
cmd_multi() {
    cmd_grep -a "$@"
}

# =============================================================================
# Extended Regex Search (egrep)
# =============================================================================
# Regex search on working set (~30-50 recent files).
# Use grep for full indexed search.

cmd_egrep() {
    # aoa egrep "regex"                    - Simple regex search
    # aoa egrep '{"name": "regex"}'        - Named pattern (legacy JSON format)
    # aoa egrep "regex" --repo flask       - Search in knowledge repo
    # aoa egrep "regex" --since 7d         - Filter by time
    local pattern=""
    local repo=""
    local since=""

    # Unix parity: handle flags before pattern
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --repo) repo="$2"; shift 2 ;;
            --since) since="$2"; shift 2 ;;
            # Unix parity: accept common flags as no-ops
            -i|--ignore-case) shift ;;  # regex is case-sensitive by default, no-op for now
            -r|-R|--recursive) shift ;; # always recursive
            -n|--line-number) shift ;;  # always shows line numbers
            -H|--with-filename) shift ;; # always shows filenames
            -c|--count) shift ;;        # not implemented, no-op
            -q|--quiet) shift ;;        # not implemented, no-op
            -e|--regexp)
                # -e pattern syntax
                if [ -z "$pattern" ]; then
                    pattern="$2"
                else
                    pattern="$pattern|$2"  # Combine with OR for egrep
                fi
                shift 2
                ;;
            -*)
                # Unknown flag - skip silently for compatibility
                shift
                ;;
            *)
                # First non-flag argument is the pattern
                if [ -z "$pattern" ]; then
                    pattern="$1"
                fi
                shift
                ;;
        esac
    done

    if [ -z "$pattern" ]; then
        echo "Usage: aoa egrep <regex> [--repo name] [--since time]"
        echo ""
        echo "Examples:"
        echo '  aoa egrep "TODO|FIXME"                # Simple regex'
        echo '  aoa egrep "def\\s+handle\\w+"         # Function pattern'
        echo '  aoa egrep "class\\s+\\w+" --since 7d  # With time filter'
        echo '  aoa egrep "Blueprint" --repo flask    # In knowledge repo'
        echo ""
        echo "Note: Searches working set (~30-50 recent files), not full index."
        echo "      Use 'aoa grep' for full indexed search."
        return 1
    fi

    # Parse since time string
    local since_seconds=""
    if [ -n "$since" ]; then
        if [[ "$since" =~ ^([0-9]+)([smhd])$ ]]; then
            local num="${BASH_REMATCH[1]}"
            local unit="${BASH_REMATCH[2]}"
            case "$unit" in
                s) since_seconds=$num ;;
                m) since_seconds=$((num * 60)) ;;
                h) since_seconds=$((num * 3600)) ;;
                d) since_seconds=$((num * 86400)) ;;
            esac
        else
            since_seconds="$since"
        fi
    fi

    # Handle both simple string and JSON pattern formats
    local patterns
    if [[ "$pattern" == "{"* ]]; then
        # Already JSON format
        patterns="$pattern"
    else
        # Simple string - wrap in JSON with "match" key
        local escaped=$(printf '%s' "$pattern" | jq -Rs .)
        patterns="{\"match\": ${escaped}}"
    fi

    # Build request body
    export AOA_SEARCH_TYPE="regex"
    local project_id=$(get_project_id)
    local body="{\"patterns\": ${patterns}"
    [ -n "$repo" ] && body="${body}, \"repo\": \"${repo}\""
    [ -n "$since_seconds" ] && body="${body}, \"since\": ${since_seconds}"
    [ -n "$project_id" ] && body="${body}, \"project\": \"${project_id}\""
    body="${body}}"

    local url="${INDEX_URL}/pattern"
    [ -n "$repo" ] && url="${INDEX_URL}/repo/${repo}/pattern"

    local result=$(curl -s -X POST "$url" \
        -H "Content-Type: application/json" \
        -d "$body")

    local err=$(echo "$result" | jq -r '.error // empty')
    if [ -n "$err" ]; then
        echo -e "${RED}${err}${NC}"
        return 1
    fi

    local ms=$(echo "$result" | jq -r '.stats.ms')
    local files_searched=$(echo "$result" | jq -r '.stats.files_searched')
    local files_matched=$(echo "$result" | jq -r '.stats.files_matched')

    # Count total hits and unique files (same as grep)
    local hits=$(echo "$result" | jq '[.results | to_entries[].value | length] | add // 0')
    local file_count=$(echo "$result" | jq -r '[.results | to_entries[].value[].file] | unique | length')

    # Get rolling intent from API response (GL-046 unified - same as grep)
    # Note: API returns tags with # prefix already, don't double it
    local rolling_intent=$(echo "$result" | jq -r '.rolling_intent // [] | join(" ")' 2>/dev/null)

    # Header: IDENTICAL to grep - hits | files | timing | intent tags
    printf "${CYAN}${BOLD}⚡ aOa${NC} ${DIM}│${NC} ${BOLD}%s${NC} hits ${DIM}│${NC} %s files ${DIM}│${NC} ${GREEN}%.2fms${NC}" "$hits" "$file_count" "$ms"
    if [ -n "$rolling_intent" ] && [ "$rolling_intent" != "" ]; then
        printf " ${DIM}│${NC} ${CYAN}%s${NC}" "$rolling_intent"
    fi
    printf "\n\n"

    # GL-046 Unified Output: Same format as grep
    # Deduplicate by file:line to match grep behavior
    echo "$result" | jq -r '
        [.results | to_entries[].value[]] |
        unique_by("\(.file):\(.line)") |
        .[] |
        # Build symbol info (same as grep)
        (if .symbol then
            if .end_line then "\(.symbol)[\(.line)-\(.end_line)]"
            else "\(.symbol)[\(.line)]"
            end
        else "<module>"
        end) as $sym |
        # Build tags (max 3, same as grep) - tags already have # prefix from API
        ((.tags // []) | .[0:3] | join(" ")) as $tags |
        # Format: file:line: symbol content  #tags (IDENTICAL to grep)
        "\u001b[1m\(.file)\u001b[0m\u001b[2m:\(.line):\u001b[0m \u001b[33m\($sym)\u001b[0m" +
        (if .context then " \u001b[2m\(.context)\u001b[0m" else "" end) +
        (if ($tags | length) > 0 then "  \u001b[36m\($tags)\u001b[0m" else "" end)
    '
}

# Deprecated: use cmd_egrep instead
cmd_pattern() {
    cmd_egrep "$@"
}
