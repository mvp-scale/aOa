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

_grep_help() {
    echo -e "${CYAN}${BOLD}aoa grep${NC} - Indexed symbol search (exact token match)"
    echo ""
    echo -e "${BOLD}USAGE${NC}"
    echo "  aoa grep <term> [file-pattern] [options]"
    echo "  aoa grep \"term1 term2\"              OR search (either term)"
    echo "  aoa grep -a term1,term2             AND search (all terms required)"
    echo ""
    echo -e "${BOLD}HOW IT WORKS${NC}"
    echo "  aoa grep searches a pre-built symbol index. It matches exact"
    echo "  tokens, not substrings. \"auth\" finds \"auth\" but not \"authentication\"."
    echo "  For substring/regex matching, use: aoa egrep \"pattern\""
    echo ""
    echo -e "${BOLD}OUTPUT FORMAT${NC}"
    echo "  Not just file:line. Each result includes the containing symbol,"
    echo "  its line range, semantic domain, and intent tags:"
    echo ""
    echo "  handler.py:AuthService().login()[10-45]:12 def login(u):  @auth  #security"
    echo ""
    echo "  file       handler.py            Source file"
    echo "  scope      AuthService().login()  Class and method containing the match"
    echo "  range      [10-45]               Symbol spans lines 10 to 45"
    echo "  line       :12                   Exact line of the match"
    echo "  content    def login(u):         The matched source line"
    echo "  @domain    @auth                 Semantic domain (area of code)"
    echo "  #tags      #security             Intent tags (what you're working on)"
    echo ""
    echo -e "${BOLD}SEARCH OPTIONS${NC}"
    echo "  -a, --and             AND mode: all comma-separated terms required"
    echo "  -i, --ignore-case     Case insensitive search"
    echo "  -w, --word-regexp     Word boundary match"
    echo "  -e, --regexp PATTERN  Multiple patterns (OR): -e foo -e bar"
    echo "  -E, --extended-regexp Routes to egrep for regex matching"
    echo "  -P, --perl-regexp     Routes to egrep for regex matching"
    echo ""
    echo -e "${BOLD}OUTPUT OPTIONS${NC}"
    echo "  -c, --count           Show match count only"
    echo "  -q, --quiet           Quiet mode (exit code only)"
    echo "  -m, --max-count NUM   Limit results (default: 20)"
    echo "  --json                Output as JSON"
    echo ""
    echo -e "${BOLD}FILE FILTERS${NC}"
    echo "  aoa grep term *.py          Positional file pattern"
    echo "  --include=GLOB              Include only matching files"
    echo "  --exclude=GLOB              Exclude matching files (skipped)"
    echo ""
    echo -e "${BOLD}TIME FILTERS${NC}"
    echo "  --since TIME          Files modified since TIME (1h, 7d, 30m)"
    echo "  --before TIME         Files modified before TIME"
    echo "  --today               Shortcut for --since 24h"
    echo ""
    echo -e "${BOLD}UNIX COMPAT (accepted, no effect)${NC}"
    echo "  -r, -R, --recursive   Always recursive"
    echo "  -n, --line-number     Always shows line numbers"
    echo "  -H, --with-filename   Always shows filenames"
    echo "  -h, --no-filename     Cannot suppress filenames"
    echo "  -F, --fixed-strings   Already literal search"
    echo "  -G, --basic-regexp    Already literal search"
    echo "  -l                    Default behavior (files with matches)"
    echo "  -o, --only-matching   Shows full line context"
    echo "  -s, --no-messages     Errors always suppressed"
    echo "  -A, -B, -C NUM       Context lines (not applicable to index)"
    echo "  --color, --colour     Always colored"
    echo ""
    echo -e "${BOLD}EXAMPLES${NC}"
    echo "  aoa grep handleAuth              Single symbol lookup"
    echo "  aoa grep \"auth token session\"    OR search (any term)"
    echo "  aoa grep -a auth,session         AND search (all terms)"
    echo "  aoa grep -ri auth --include=*.py Case insensitive, Python only"
    echo "  aoa grep auth --since 1h         Modified in last hour"
    echo "  aoa grep auth --today --json     Today's results as JSON"
    echo ""
    echo -e "${BOLD}SEE ALSO${NC}"
    echo "  aoa egrep \"regex\"    Substring/regex search (working set)"
    echo "  aoa find \"*.py\"     File discovery by pattern"
    echo "  aoa locate name     Fast filename search"
}

cmd_grep() {
    local and_mode=false
    local case_insensitive=false
    local word_boundary=false
    local json_output=false
    local count_only=false
    local quiet=false
    local query=""
    local file_filter=""
    local mode="recent"
    local limit="20"
    local since=""
    local before=""

    # _grep_flag: process a single short flag. Returns 0 if handled.
    _grep_flag() {
        case "$1" in
            a) and_mode=true ;;
            i) case_insensitive=true ;;
            w) word_boundary=true ;;
            c) count_only=true ;;
            q) quiet=true ;;
            E|P) return 3 ;; # Routes to egrep, can't combine
            r|R|n|H|F|G|l|o|s|h|v) ;; # No-ops / not applicable
            e) return 1 ;; # Needs argument, can't combine
            *) return 2 ;; # Truly unknown
        esac
        return 0
    }

    # Parse flags
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --help)
                _grep_help
                return 0
                ;;
            -a|--and)
                and_mode=true
                shift
                ;;
            -i|--ignore-case|--no-ignore-case)
                case_insensitive=true
                shift
                ;;
            -w|--word-regexp)
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
            -q|--quiet|--silent)
                quiet=true
                shift
                ;;
            -l|--limit|-m|--max-count)
                limit="$2"
                shift 2
                ;;
            --max-count=*|--limit=*)
                limit="${1#*=}"
                shift
                ;;
            --mode)
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
            -E|--extended-regexp|-P|--perl-regexp)
                # Unix parity: grep -E/-P routes to egrep
                shift
                cmd_egrep "$@"
                return $?
                ;;
            -r|-R|--recursive|--dereference-recursive)
                shift ;; # No-op: always recursive
            -n|--line-number)
                shift ;; # No-op: always shows line numbers
            -H|--with-filename|-h|--no-filename)
                shift ;; # No-op: always shows filenames
            -F|--fixed-strings|-G|--basic-regexp)
                shift ;; # No-op: already literal search
            -o|--only-matching)
                shift ;; # No-op: shows full line context
            -s|--no-messages)
                shift ;; # No-op: errors handled internally
            -v|--invert-match)
                shift ;; # Not applicable to index search
            -A|-B|-C)
                # Context lines: accept and skip the NUM argument
                shift 2 ;;
            --context|--before-context|--after-context)
                shift 2 ;;
            -[0-9]|-[0-9][0-9]|-[0-9][0-9][0-9])
                shift ;; # grep -NUM context shorthand
            --color|--colour|--color=*|--colour=*)
                shift ;; # No-op: always colored
            --label|--label=*|--binary-files|--binary-files=*)
                # Skip flag + value
                if [[ "$1" != *=* ]]; then shift; fi
                shift
                ;;
            -e|--regexp|--regexp=*)
                # Unix parity: grep -e foo -e bar → OR search
                if [[ "$1" == --regexp=* ]]; then
                    local val="${1#--regexp=}"
                    if [ -z "$query" ]; then query="$val"; else query="$query $val"; fi
                    shift
                else
                    if [ -z "$query" ]; then query="$2"; else query="$query $2"; fi
                    shift 2
                fi
                ;;
            -f|--file)
                # grep -f FILE: not supported, skip
                shift 2
                ;;
            --include|--include=*)
                if [[ "$1" == --include=* ]]; then
                    file_filter="${1#--include=}"
                else
                    file_filter="$2"
                    shift
                fi
                shift
                ;;
            --exclude|--exclude=*|--exclude-dir|--exclude-dir=*|--exclude-from)
                # Accept and skip
                if [[ "$1" != *=* ]]; then
                    shift
                    [[ "${1:-}" != -* ]] 2>/dev/null && shift
                else
                    shift
                fi
                ;;
            -[a-zA-Z][a-zA-Z]*)
                # Combined short flags like -ri, -rn, -riH
                local flags="${1#-}"
                shift
                local unknown_in_combo=""
                for (( ci=0; ci<${#flags}; ci++ )); do
                    local ch="${flags:$ci:1}"
                    _grep_flag "$ch"
                    local rc=$?
                    if [ $rc -eq 2 ]; then
                        unknown_in_combo="$ch"
                    elif [ $rc -eq 3 ]; then
                        # -E/-P in combo: route remaining args to egrep
                        cmd_egrep "$@"
                        return $?
                    fi
                done
                if [ -n "$unknown_in_combo" ]; then
                    echo -e "${DIM}Unknown flag: -${unknown_in_combo}. Run: aoa grep --help${NC}"
                    return 1
                fi
                ;;
            -*)
                echo -e "${DIM}Unknown: $1. Run: aoa grep --help${NC}"
                return 1
                ;;
            *)
                # GL-051: First positional arg is query, rest are file patterns
                if [ -z "$query" ]; then
                    query="$1"
                elif [ -z "$file_filter" ]; then
                    file_filter="$1"
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
        _grep_help
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
        project_param="&project_id=${project_id}"
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
        [ -n "$file_filter" ] && body="${body}, \"filter\": \"${file_filter}\""
        body="${body}}"

        result=$(curl -s -X POST "${INDEX_URL}/multi" \
            -H "Content-Type: application/json" \
            -d "$body")
        # CLI-001: Check for API failure
        if [ -z "$result" ]; then
            echo "Error: API unavailable at ${INDEX_URL}" >&2
            return 1
        fi

        # CLI-009: Single jq call for ms and count
        read -r ms count < <(echo "$result" | jq -r '[.ms // 0, (.results | length)] | @tsv')

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
            # CLI-010: Single jq instead of tr|grep|jq|jq
            local json_terms=$(echo "$query" | jq -R 'split(" ") | map(select(length > 0))')

            local body="{\"terms\": ${json_terms}, \"mode\": \"${mode}\", \"limit\": ${limit}, \"operator\": \"or\""
            [ -n "$project_id" ] && body="${body}, \"project\": \"${project_id}\""
            [ -n "$since_seconds" ] && body="${body}, \"since\": ${since_seconds}"
            [ -n "$before_seconds" ] && body="${body}, \"before\": ${before_seconds}"
            [ -n "$file_filter" ] && body="${body}, \"filter\": \"${file_filter}\""
            body="${body}}"

            result=$(curl -s -X POST "${INDEX_URL}/multi" \
                -H "Content-Type: application/json" \
                -d "$body")
            # CLI-001: Check for API failure
            if [ -z "$result" ]; then
                echo "Error: API unavailable at ${INDEX_URL}" >&2
                return 1
            fi
        else
            # GL-050: Content search with /grep endpoint (searches file contents like Unix grep)
            export AOA_SEARCH_TYPE="content"
            local encoded_query=$(printf '%s' "$query" | jq -sRr @uri)

            # Build query params
            local params="q=${encoded_query}&limit=${limit}${project_param}"
            [ "$case_insensitive" = true ] && params="${params}&ci=1"
            [ "$word_boundary" = true ] && params="${params}&word=1"
            [ -n "$since_seconds" ] && params="${params}&since=${since_seconds}"
            [ -n "$before_seconds" ] && params="${params}&before=${before_seconds}"
            [ -n "$file_filter" ] && params="${params}&filter=$(printf '%s' "$file_filter" | jq -sRr @uri)"

            result=$(curl -s "${INDEX_URL}/grep?${params}")
            # CLI-001: Check for API failure
            if [ -z "$result" ]; then
                echo "Error: API unavailable at ${INDEX_URL}" >&2
                return 1
            fi
        fi
        # CLI-009: Single jq call for ms and count
        read -r ms count < <(echo "$result" | jq -r '[.ms // 0, (.results | length)] | @tsv')

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

    # If no results, show guidance
    if [ "$count" -lt 1 ]; then
        printf "${CYAN}${BOLD}⚡ aOa${NC} ${DIM}│${NC} 0 hits ${DIM}│${NC} ${GREEN}%.2fms${NC}\n" "$ms"
        echo ""
        echo -e "${DIM}grep matches exact tokens. \"auth\" won't match \"authentication\".${NC}"
        echo -e "${DIM}Try:  aoa egrep \"${query}\"    ${NC}${DIM}# substring/regex search${NC}"
        echo -e "${DIM}      aoa grep \"${query} ...\" ${NC}${DIM}# add related terms (OR search)${NC}"
        return 0
    fi

    # PERF: Single jq call outputs header info (line 1) then formatted results
    # GL-088: Simplified - just file_count on first line, no rolling_intent
    local output
    output=$(echo "$result" | jq -r '
        # Helper: extract params from signature like "def foo(self, x: int) -> str" -> "(x)"
        def extract_params:
            if . == null then "()"
            elif test("\\(.*\\)") then
                capture("\\((?<p>[^)]*)\\)").p
                | split(",")
                | map(gsub("^\\s+|\\s+$"; ""))
                | map(select(. != "self" and . != ""))
                | map(split(":")[0] | gsub("^\\s+|\\s+$"; ""))
                | "(" + join(", ") + ")"
            else "()"
            end;

        # First line: file_count only
        ([.results[].file] | unique | length | tostring),

        # Remaining lines: formatted results
        (.results | unique_by(.file + ":" + (.line | tostring)) | .[] |
            (.signature | extract_params) as $params |
            (
                if .parent_name then
                    "\(.parent_name)().\(.symbol // "<anon>")\($params)[\(.start_line // .line)-\(.end_line // .line)]"
                elif .symbol then
                    "\(.symbol)\($params)[\(.start_line // .line)-\(.end_line // .line)]"
                else
                    "<module>"
                end
            ) as $scope |
            (if .domain then .domain else "" end) as $domain |
            ((.tags // []) | .[0:3] | map(if startswith("@") then . else "#" + . end) | join(" ")) as $tags |
            "\u001b[1m\(.file)\u001b[0m:\u001b[33m\($scope)\u001b[0m:\u001b[2m\(.line)\u001b[0m" +
            (if .content then " \(.content)" else "" end) +
            (if ($domain | length) > 0 then "  \u001b[35m\($domain)\u001b[0m" else "" end) +
            (if ($tags | length) > 0 then "  \u001b[36m\($tags)\u001b[0m" else "" end)
        )
    ')

    # CLI-011: Parse header using bash builtins (no head/tail subprocesses)
    local header_line="${output%%$'\n'*}"
    local file_count="${header_line%%	*}"

    # Print header - GL-088: simplified, no trailing tags
    printf "${CYAN}${BOLD}⚡ aOa${NC} ${DIM}│${NC} ${BOLD}%s${NC} hits ${DIM}│${NC} %s files ${DIM}│${NC} ${GREEN}%.2fms${NC}\n\n" "$count" "$file_count" "$ms"

    # CLI-011: Print results skipping header (bash builtin)
    echo "${output#*$'\n'}"
}

# CLI-007: Removed display_ranked_grep_results_verbose (dead code)

# GL-045: Display semantic grep results (DEAD CODE - kept for reference, never called)
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
        local file_tags=$(echo "$file_result" | jq -r '.file_tags | .[0:3] | map(if startswith("@") then . else "#" + . end) | join(" ")' 2>/dev/null)
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
    # aoa egrep -i "regex"                 - Case insensitive
    # aoa egrep "regex" *.py               - Filter to file pattern (GL-051 Unix parity)
    local pattern=""
    local repo=""
    local since=""
    local case_insensitive=false
    local file_filter=""

    # Unix parity: handle flags before pattern
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --repo) repo="$2"; shift 2 ;;
            --since) since="$2"; shift 2 ;;
            # GL-050: Unix parity - case insensitive flag
            -i|--ignore-case) case_insensitive=true; shift ;;
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
                # GL-051: First positional arg is pattern, rest are file filters
                if [ -z "$pattern" ]; then
                    pattern="$1"
                elif [ -z "$file_filter" ]; then
                    file_filter="$1"
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

    # Phase 3B: Use optimized /grep endpoint with regex=true (same as grep)
    export AOA_SEARCH_TYPE="regex"
    local project_id=$(get_project_id)
    local encoded_pattern=$(printf '%s' "$pattern" | jq -sRr @uri)

    # Build query params - same as grep but with regex=true
    local params="q=${encoded_pattern}&regex=true"
    [ -n "$project_id" ] && params="${params}&project_id=${project_id}"
    [ "$case_insensitive" = true ] && params="${params}&ci=1"
    [ -n "$since_seconds" ] && params="${params}&since=${since_seconds}"
    [ -n "$file_filter" ] && params="${params}&filter=$(printf '%s' "$file_filter" | jq -sRr @uri)"

    local url="${INDEX_URL}/grep?${params}"
    # TODO: repo support would need different handling

    local result=$(curl -s "$url")
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

    # GL-050: Universal Output - use SAME display function as grep
    local ms=$(echo "$result" | jq -r '.ms')
    local count=$(echo "$result" | jq '.total_matches // (.results | length)')
    display_ranked_grep_results "$result" "$pattern" "$ms" "$count"
}

# Deprecated: use cmd_egrep instead
cmd_pattern() {
    cmd_egrep "$@"
}
