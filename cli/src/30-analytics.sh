# =============================================================================
# SECTION 30: Analytics & Behavioral Commands
# =============================================================================
#
# PURPOSE
#   Behavioral analysis unique to aOa. These commands learn from usage
#   patterns to predict which files you'll need next. No Unix equivalent.
#
# DEPENDENCIES
#   - 01-constants.sh: INDEX_URL, colors
#   - 02-utils.sh: get_project_id()
#
# COMMANDS PROVIDED
#   cmd_hot        Show frequently accessed files
#   cmd_enrich     Add/update tags for a file
#   cmd_touched    Recently touched files
#   cmd_focus      Files related to current focus
#   cmd_predict    Predict next files based on intent
#   cmd_outline    Show file structure (functions, classes)
#   cmd_cc         Claude Code session commands (prompts, history)
#
# =============================================================================

cmd_hot() {
    # aoa hot [limit]     - Show frequently accessed "hot" files
    local limit="${1:-10}"

    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/hot?limit=${limit}${project_param}")
    local count=$(echo "$result" | jq -r '.files | length // 0' 2>/dev/null || echo "0")

    printf "${CYAN}${BOLD}🔥 %s hot files${NC}\n" "$count"

    if [ "$count" -gt 0 ] 2>/dev/null; then
        echo "$result" | jq -r '.files[] | "  \(.path) (\(.hits // 0) hits)"' 2>/dev/null || true
    fi
    return 0
}

cmd_bigrams() {
    # aoa bigrams [--recent] [--min N] [--limit N]  - Show bigrams (usage signals)
    local min_count=""
    local limit=50
    local recent=false

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --recent|-r)
                recent=true
                shift
                ;;
            --min|-m)
                min_count="${2:-6}"
                shift 2
                ;;
            --limit|-n)
                limit="${2:-50}"
                shift 2
                ;;
            *)
                shift
                ;;
        esac
    done

    local project_id=$(get_project_id)
    if [ -z "$project_id" ]; then
        echo -e "${RED}No project initialized${NC}" >&2
        return 1
    fi

    # Build URL with optional params
    local url="${INDEX_URL}/cc/bigrams?project_id=${project_id}&limit=${limit}"
    if [ "$recent" = true ]; then
        url="${url}&recent=true"
    fi
    if [ -n "$min_count" ]; then
        url="${url}&min_count=${min_count}"
    fi

    local result=$(curl -s "$url")

    # Output format: bigram (count)
    echo "$result" | jq -r '.bigrams[]? | "\(.bigram) (\(.count))"' 2>/dev/null
    return 0
}

cmd_enrich() {
    # aoa enrich --hot    - AI-enrich hot files that lack semantic tags
    # aoa enrich <file>   - AI-enrich a specific file
    local hot_mode=false
    local file=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --hot|-h)
                hot_mode=true
                shift
                ;;
            *)
                file="$1"
                shift
                ;;
        esac
    done

    local project_id=$(get_project_id)

    if $hot_mode; then
        # Get hot files and check which need AI enrichment
        printf "${CYAN}${BOLD}⚡ aOa Enrich${NC} ${DIM}│${NC} Checking hot files for AI tags...\n"
        echo ""

        local predictions=$(curl -s "${INDEX_URL}/predict?limit=10" 2>/dev/null)
        local needs_tags=""
        local count=0

        while IFS= read -r filepath; do
            [ -z "$filepath" ] && continue

            # Check if file has AI-stored tags
            local tags_json=$(curl -s "${INDEX_URL}/outline/tags?file=${filepath}" 2>/dev/null)
            local tag_count=$(echo "$tags_json" | jq -r '.tags | keys | length // 0' 2>/dev/null)

            if [ "$tag_count" -eq 0 ] || [ -z "$tag_count" ]; then
                count=$((count + 1))
                needs_tags+="$filepath\n"
                printf "  ${YELLOW}○${NC} %s ${DIM}(needs AI tags)${NC}\n" "$filepath"
            else
                printf "  ${GREEN}✓${NC} %s ${DIM}(%s tags)${NC}\n" "$filepath" "$tag_count"
            fi
        done < <(echo "$predictions" | jq -r '.files[].path' 2>/dev/null)

        echo ""
        if [ $count -gt 0 ]; then
            printf "${BOLD}%d file(s) need AI enrichment.${NC}\n" "$count"
            echo ""
            printf "${DIM}To enrich, say in Claude: \"enrich these hot files\"${NC}\n"
            printf "${DIM}Or enrich one: aoa enrich <filepath>${NC}\n"
            echo ""
            echo "FILES_NEEDING_ENRICHMENT:"
            echo -e "$needs_tags" | head -10
        else
            printf "${GREEN}All hot files have AI tags!${NC}\n"
        fi
    elif [ -n "$file" ]; then
        # Enrich a specific file - show outline for Claude to tag
        printf "${CYAN}${BOLD}⚡ aOa Enrich${NC} ${DIM}│${NC} %s\n" "$file"
        echo ""

        local outline=$(curl -s "${INDEX_URL}/outline?file=${file}" 2>/dev/null)
        local sym_count=$(echo "$outline" | jq -r '.count // 0' 2>/dev/null)

        if [ "$sym_count" -gt 0 ]; then
            echo "Symbols to tag:"
            echo "$outline" | jq -r '.symbols[] | "  \(.kind) \(.name) [\(.start_line)-\(.end_line)]"' 2>/dev/null
            echo ""
            echo "TAG_REQUEST:"
            echo "$outline" | jq -c '{file: .file, language: .language, symbols: [.symbols[] | {name: .name, kind: .kind, line: .start_line, end_line: .end_line, signature: .signature}]}'
        else
            printf "${YELLOW}No targets found in file.${NC}\n"
        fi
    else
        echo "Usage: aoa enrich --hot       # Check hot files for AI tags"
        echo "       aoa enrich <file>      # Prepare file for AI tagging"
        echo ""
        echo "Examples:"
        echo "  aoa enrich --hot"
        echo "  aoa enrich src/auth/handler.py"
    fi
}

cmd_touched() {
    # aoa touched [since]   - Files touched in current session or time period
    local since="${1:-session}"

    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    # Parse time string
    local since_param="since=session"
    if [[ "$since" =~ ^([0-9]+)([smhd])$ ]]; then
        local num="${BASH_REMATCH[1]}"
        local unit="${BASH_REMATCH[2]}"
        local seconds
        case "$unit" in
            s) seconds=$num ;;
            m) seconds=$((num * 60)) ;;
            h) seconds=$((num * 3600)) ;;
            d) seconds=$((num * 86400)) ;;
        esac
        since_param="since=${seconds}"
    elif [ "$since" = "session" ]; then
        since_param="since=session"
    elif [ "$since" = "today" ]; then
        since_param="since=86400"
    fi

    local result=$(curl -s "${INDEX_URL}/intent/recent?${since_param}&limit=100${project_param}")

    # Extract unique files from intent records
    local files=$(echo "$result" | jq -r '[.intents[]?.files[]?] | unique | .[]' 2>/dev/null || true)
    local count=0
    if [ -n "$files" ]; then
        count=$(echo "$files" | wc -l | tr -d ' ')
    fi

    printf "${CYAN}${BOLD}✋ %s files touched${NC}\n" "$count"
    if [ -n "$files" ]; then
        echo "$files" | while read -r file; do
            [ -n "$file" ] && echo "  $file"
        done
    fi
    return 0
}

cmd_focus() {
    # aoa focus     - Show current working set from memory
    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="?project_id=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/memory${project_param}")
    local count=$(echo "$result" | jq -r '.working_set | length // 0' 2>/dev/null || echo "0")

    printf "${CYAN}${BOLD}🎯 %s files in focus${NC}\n" "$count"

    if [ "$count" -gt 0 ] 2>/dev/null; then
        echo "$result" | jq -r '.working_set[]' 2>/dev/null | while read -r file; do
            echo "  $file"
        done
    fi
    return 0
}

cmd_outline() {
    local file=""
    local enrich=false
    local enrich_all=false
    local pending=false
    local hot_mode=false
    local json_output=false
    local store=false
    local show_tags=false

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --enrich)
                enrich=true
                shift
                ;;
            --enrich-all)
                enrich_all=true
                shift
                ;;
            --pending)
                pending=true
                shift
                ;;
            --hot)
                hot_mode=true
                shift
                ;;
            --json|-j)
                json_output=true
                shift
                ;;
            --store)
                store=true
                shift
                ;;
            --tags|-t)
                show_tags=true
                shift
                ;;
            -*)
                echo "Unknown option: $1"
                return 1
                ;;
            *)
                file="$1"
                shift
                ;;
        esac
    done

    # Handle --store (post enriched tags from stdin)
    if $store; then
        local project_id=$(get_project_id)
        local json_input=$(cat)

        # Add project_id if not present
        if [ -n "$project_id" ]; then
            json_input=$(echo "$json_input" | jq --arg pid "$project_id" '. + {project_id: $pid}')
        fi

        local result=$(echo "$json_input" | curl -s -X POST "${INDEX_URL}/outline/enriched" \
            -H "Content-Type: application/json" \
            -d @-)

        if $json_output; then
            echo "$result" | jq .
        else
            local success=$(echo "$result" | jq -r '.success // false')
            if [ "$success" = "true" ]; then
                local tags_count=$(echo "$result" | jq -r '.tags_indexed // 0')
                local symbols_count=$(echo "$result" | jq -r '.symbols_processed // 0')
                echo -e "${GREEN}✓${NC} Stored ${tags_count} tags across ${symbols_count} targets"

                # Record to intent system for activity tracking
                local file_path=$(echo "$json_input" | jq -r '.file // ""')
                local tags=$(echo "$json_input" | jq -c '[.symbols[].tags[]?] | unique | .[0:5]')
                if [ -n "$file_path" ] && [ "$tags" != "[]" ]; then
                    curl -s -X POST "${INDEX_URL}/intent" \
                        -H "Content-Type: application/json" \
                        -d "{\"tool\": \"Outline\", \"files\": [\"$file_path\"], \"tags\": $tags, \"project_id\": \"$project_id\"}" > /dev/null 2>&1
                fi
            else
                local err=$(echo "$result" | jq -r '.error // "Unknown error"')
                echo -e "${RED}✗${NC} ${err}"
                return 1
            fi
        fi
        return 0
    fi

    # Handle --hot (hot files that need AI tags - prioritized list for lazy enricher)
    if $hot_mode; then
        local project_id=$(get_project_id)

        # Get a larger pool of hot files (50), filter to those needing tags
        local predictions=$(curl -s "${INDEX_URL}/predict?limit=50" 2>/dev/null)
        local needs_tags=""
        local tagged_count=0
        local needs_count=0

        while IFS= read -r filepath; do
            [ -z "$filepath" ] && continue
            # Skip line-range references (file.py:100-200)
            [[ "$filepath" == *:*-* ]] && continue

            # Check if file has AI-stored tags
            local tags_json=$(curl -s "${INDEX_URL}/outline/tags?file=${filepath}" 2>/dev/null)
            local tag_count=$(echo "$tags_json" | jq -r '.tags | keys | length // 0' 2>/dev/null)

            if [ "$tag_count" -eq 0 ] || [ -z "$tag_count" ]; then
                needs_count=$((needs_count + 1))
                needs_tags+="$filepath\n"
            else
                tagged_count=$((tagged_count + 1))
            fi
        done < <(echo "$predictions" | jq -r '.files[].path' 2>/dev/null)

        # JSON output for agents
        if $json_output; then
            echo -e "$needs_tags" | head -20 | jq -R -s 'split("\n") | map(select(length > 0)) | {hot_pending: ., count: length}'
            return 0
        fi

        echo -e "${CYAN}${BOLD}⚡ aOa Outline - Hot Files${NC}"
        echo ""
        echo -e "  Need tags:  ${YELLOW}${needs_count}${NC}"
        echo -e "  Already:    ${GREEN}${tagged_count}${NC}"
        echo ""

        if [ "$needs_count" -gt 0 ]; then
            echo -e "${BOLD}Hot files needing AI tags:${NC}"
            echo -e "$needs_tags" | head -10 | while read -r f; do
                [ -n "$f" ] && echo "  $f"
            done
            [ "$needs_count" -gt 10 ] && echo -e "  ${DIM}... and $((needs_count - 10)) more${NC}"
            echo ""
            echo "HOT_PENDING:"
            echo -e "$needs_tags" | head -10
        else
            echo -e "${GREEN}All hot files have AI tags!${NC}"
        fi
        return 0
    fi

    # Handle --pending (quick status check)
    if $pending; then
        local project_id=$(get_project_id)
        local project_param=""
        if [ -n "$project_id" ]; then
            project_param="?project_id=${project_id}"
        fi

        local result=$(curl -s "${INDEX_URL}/outline/pending${project_param}")

        # JSON output for agents/scripts
        if $json_output; then
            echo "$result" | jq .
            return 0
        fi

        local pending_count=$(echo "$result" | jq -r '.pending_count // 0')
        local up_to_date=$(echo "$result" | jq -r '.up_to_date_count // 0')
        local total=$(echo "$result" | jq -r '.total_files // 0')

        echo -e "${CYAN}${BOLD}⚡ aOa Outline Status${NC}"
        echo ""
        echo -e "  Pending:  ${YELLOW}${pending_count}${NC} need tags"
        echo -e "  Tagged:   ${GREEN}${up_to_date}${NC}"
        echo -e "  Total:    ${total}"

        if [ "$pending_count" -gt 0 ]; then
            echo ""
            echo -e "${DIM}In Claude Code, say: \"tag the codebase\"${NC}"
        else
            echo ""
            echo -e "${DIM}Search tags: aoa search \"#validation\"${NC}"
        fi
        return 0
    fi

    # Handle --enrich-all (batch mode - now "deep outline")
    if $enrich_all; then
        echo -e "${CYAN}${BOLD}⚡ aOa Outline - Deep Tagging${NC}"
        echo ""

        # Get current project context
        local project_id=$(get_project_id)
        local project_param=""
        if [ -n "$project_id" ]; then
            project_param="?project_id=${project_id}"
        fi

        # Get pending files from the service
        local result=$(curl -s "${INDEX_URL}/outline/pending${project_param}")

        local pending_count=$(echo "$result" | jq -r '.pending_count // 0')
        local up_to_date=$(echo "$result" | jq -r '.up_to_date_count // 0')
        local total=$(echo "$result" | jq -r '.total_files // 0')
        local ms=$(echo "$result" | jq -r '.ms // 0')

        printf "Scanned project in ${GREEN}%.1fms${NC}\n" "$ms"
        echo ""
        echo -e "  Total files:   ${BOLD}${total}${NC}"
        echo -e "  Tagged:        ${GREEN}${up_to_date}${NC}"
        echo -e "  Need tags:     ${YELLOW}${pending_count}${NC}"
        echo ""

        if [ "$pending_count" -eq 0 ]; then
            echo -e "${GREEN}All files are tagged!${NC}"
            echo ""
            echo -e "${DIM}Search: aoa search \"#validation\"${NC}"
            return 0
        fi

        echo -e "${BOLD}Files needing tags:${NC}"
        echo "$result" | jq -r '.pending[:10][] | "  \(.file) (\(.reason))"' 2>/dev/null

        if [ "$pending_count" -gt 10 ]; then
            echo -e "  ${DIM}... and $((pending_count - 10)) more${NC}"
        fi

        echo ""
        echo -e "${BOLD}To tag these files:${NC}"
        echo ""
        echo "  In Claude Code, say: \"tag the codebase\""
        echo ""
        return 0
    fi

    if [ -z "$file" ]; then
        echo "Usage: aoa outline <file>"
        echo "       aoa outline <file> --tags"
        echo "       aoa outline --pending"
        echo ""
        echo "Options:"
        echo "  --tags, -t    Show semantic tags for each symbol"
        echo "  --pending     Show files needing semantic tags"
        echo "  --enrich-all  Show detailed tagging status"
        echo "  --json, -j    Output as JSON"
        echo ""
        echo "To add semantic tags, say in Claude Code: \"tag the codebase\""
        echo ""
        echo "Examples:"
        echo "  aoa outline src/index.ts"
        echo "  aoa outline src/index.ts --tags"
        echo "  aoa outline --pending"
        return 1
    fi

    # Get current project context
    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project_id=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/outline?file=${file}${project_param}")

    # JSON output for agents/scripts
    if $json_output; then
        echo "$result" | jq .
        return 0
    fi

    # Check for errors
    local err=$(echo "$result" | jq -r '.error // empty')
    if [ -n "$err" ]; then
        echo -e "${RED}${err}${NC}"
        local msg=$(echo "$result" | jq -r '.message // empty')
        [ -n "$msg" ] && echo -e "${DIM}${msg}${NC}"
        return 1
    fi

    local count=$(echo "$result" | jq -r '.count // 0')
    local ms=$(echo "$result" | jq -r '.ms // 0')
    local lang=$(echo "$result" | jq -r '.language // "unknown"')

    # Header
    if $enrich; then
        printf "${CYAN}${BOLD}⚡ %s targets${NC} ${DIM}│${NC} ${GREEN}%.2fms${NC} ${DIM}│${NC} ${YELLOW}%s${NC} ${DIM}│${NC} ${BLUE}enrichment requested${NC}\n" "$count" "$ms" "$lang"
    elif $show_tags; then
        printf "${CYAN}${BOLD}⚡ %s targets${NC} ${DIM}│${NC} ${GREEN}%.2fms${NC} ${DIM}│${NC} ${YELLOW}%s${NC} ${DIM}│${NC} ${CYAN}with tags${NC}\n" "$count" "$ms" "$lang"
    else
        printf "${CYAN}${BOLD}⚡ %s targets${NC} ${DIM}│${NC} ${GREEN}%.2fms${NC} ${DIM}│${NC} ${YELLOW}%s${NC}\n" "$count" "$ms" "$lang"
    fi
    echo ""

    # Output symbols with line ranges (and tags if requested)
    if $show_tags; then
        # Fetch tags for this file from the inverted index
        local tags_json=$(curl -s "${INDEX_URL}/outline/tags?file=${file}${project_param}" 2>/dev/null)

        # Use jq to format output with tags inline
        echo "$result" | jq -r --argjson tags "$tags_json" '
            .symbols[] |
            .signature as $sig |
            .kind as $kind |
            .name as $name |
            .start_line as $start |
            .end_line as $end |
            ($sig // "\($kind) \($name)") as $header |
            ($tags.tags[$name] // []) | join(" ") as $sym_tags |
            if ($sym_tags | length) > 0 then
                "  \($header) [\($start)-\($end)] \($sym_tags)"
            else
                "  \($header) [\($start)-\($end)] (no tags)"
            end
        ' 2>/dev/null
    else
        # Show signature (definition header) for each symbol
        # For routes, show parsed name (POST /path) instead of raw decorator
        echo "$result" | jq -r '.symbols[] |
            if .kind == "route" then
                "  \(.name) [\(.start_line)-\(.end_line)]"
            else
                "  \(.signature // "\(.kind) \(.name)") [\(.start_line)-\(.end_line)]"
            end
        ' 2>/dev/null
    fi

    # If tagging requested, output instructions for Claude
    if $enrich; then
        echo ""
        echo -e "${BLUE}─── Semantic Tagging ───${NC}"
        echo ""
        echo -e "${DIM}To tag these targets, Claude should:${NC}"
        echo -e "${DIM}1. Analyze the targets above${NC}"
        echo -e "${DIM}2. Generate 2-5 semantic tags per target via Haiku${NC}"
        echo -e "${DIM}3. Store via: POST /outline/enriched with symbols array${NC}"
        echo ""
        # Output JSON for Claude to use
        echo "TAG_REQUEST:"
        echo "$result" | jq -c '{file: .file, language: .language, symbols: [.symbols[] | {name: .name, kind: .kind, line: .start_line, end_line: .end_line, signature: .signature}]}'
    fi
}

cmd_outline_status() {
    # Show outline tagging status
    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="?project_id=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/outline/pending${project_param}")
    local pending_count=$(echo "$result" | jq -r '.pending_count // 0')
    local up_to_date=$(echo "$result" | jq -r '.up_to_date_count // 0')
    local total=$(echo "$result" | jq -r '.total_files // 0')

    echo -e "${CYAN}${BOLD}⚡ aOa Outline${NC}"
    echo ""
    echo -e "  Pending:  ${YELLOW}${pending_count}${NC} files need semantic tags"
    echo -e "  Tagged:   ${GREEN}${up_to_date}${NC} files"
    echo -e "  Total:    ${total} files"
    echo ""

    if [ "$pending_count" -eq 0 ]; then
        echo -e "${GREEN}All files are tagged!${NC}"
        echo ""
        echo "Try: aoa search \"#authentication\" to find auth code"
        return 0
    fi

    echo -e "${BOLD}How to tag:${NC}"
    echo ""
    echo "  In Claude Code, say: \"tag the codebase\""
    echo ""
    echo "  Claude will:"
    echo "    1. Get pending files from /outline/pending"
    echo "    2. Batch files into groups of 15"
    echo "    3. Generate semantic tags per symbol via Haiku"
    echo "    4. Store tags for searchable access"
    echo ""
    echo -e "${DIM}Then search: aoa search \"#validation\" to find by tag${NC}"
}

cmd_cc() {
    # aoa cc <subcommand>  - Claude Code session commands
    local subcmd="${1:-help}"
    shift 2>/dev/null || true

    case "$subcmd" in
        prompts|p)
            # aoa cc prompts [--limit N] [--json] [--raw]
            local limit=25
            local json_output=false
            local raw_output=false

            while [[ $# -gt 0 ]]; do
                case "$1" in
                    --limit|-l) limit="$2"; shift 2 ;;
                    --json|-j) json_output=true; shift ;;
                    --raw|-r) raw_output=true; shift ;;
                    *) shift ;;
                esac
            done

            local project_path=$(get_project_root)
            if [ -z "$project_path" ]; then
                echo -e "${RED}Error: Not in an aOa-initialized project${NC}"
                return 1
            fi

            local result=$(curl -s "${INDEX_URL}/cc/prompts?limit=${limit}&project_path=${project_path}")

            if $json_output; then
                echo "$result" | jq .
                return 0
            fi

            if $raw_output; then
                # Raw output for hooks - no header, no numbering
                echo "$result" | jq -r '.prompts[]' 2>/dev/null | head -"$limit"
                return 0
            fi

            local count=$(echo "$result" | jq -r '.count // 0')
            echo -e "${CYAN}${BOLD}⚡ CC Prompts${NC} │ Last ${count}"
            echo ""

            # Show full prompts with numbering
            echo "$result" | jq -r '.prompts[]' 2>/dev/null | head -"$limit" | nl -w3 -s'. ' | while IFS= read -r line; do
                echo "  $line"
            done
            ;;

        sessions|s)
            # aoa cc sessions [--limit N] [--json]
            local limit=10
            local json_output=false

            while [[ $# -gt 0 ]]; do
                case "$1" in
                    --limit|-l) limit="$2"; shift 2 ;;
                    --json|-j) json_output=true; shift ;;
                    *) shift ;;
                esac
            done

            local project_path=$(get_project_root)
            if [ -z "$project_path" ]; then
                echo -e "${RED}Error: Not in an aOa-initialized project${NC}"
                return 1
            fi

            local result=$(curl -s "${INDEX_URL}/cc/sessions?limit=${limit}&project_path=${project_path}")

            if $json_output; then
                echo "$result" | jq .
                return 0
            fi

            local count=$(echo "$result" | jq -r '.count // 0')
            echo -e "${CYAN}${BOLD}⚡ CC Sessions${NC} │ Last ${count}"
            echo ""

            # 120 char width layout
            # SESSION=23 │ OUTPUT=20 │ TPS=18 │ CALLS=17 │ TOOLS=27

            # Headers
            echo -e "${DIM}SESSION                ${DIM}│${DIM} OUTPUT (tok/s)     ${DIM}│${DIM} TPS              ${DIM}│${DIM} CALLS           ${DIM}│${DIM} TOOLS${NC}"
            echo -e "${DIM}START       DUR     P  ${DIM}│${DIM}   O      S      H  ${DIM}│${DIM}   O     S     H  ${DIM}│${DIM}   O    S    H   ${DIM}│${DIM}  B    R    E    W    T    M${NC}"
            echo -e "${DIM}───────────────────────${DIM}┼${DIM}────────────────────${DIM}┼${DIM}──────────────────${DIM}┼${DIM}─────────────────${DIM}┼${DIM}───────────────────────────${NC}"

            # Data rows - build each section to exact width then join
            echo "$result" | jq -r '.sessions[] | [
                .date,
                (if (.duration_min // 0) >= 60 then "\(((.duration_min // 0) / 60) | floor)h\(((.duration_min // 0) % 60))m" else "\(.duration_min // 0)m" end),
                (if (.prompt_count // 0) > 0 then (.prompt_count // 0) else "-" end),
                (if (.output_tps.O // 0) > 0 then (.output_tps.O // 0) else "-" end),
                (if (.output_tps.S // 0) > 0 then (.output_tps.S // 0) else "-" end),
                (if (.output_tps.H // 0) > 0 then (.output_tps.H // 0) else "-" end),
                (if (.effective_tps.O // 0) > 0 then (if (.effective_tps.O // 0) >= 1000 then "\(((.effective_tps.O // 0) / 1000 * 10 | floor) / 10)k" else "\((.effective_tps.O // 0) | floor)" end) else "-" end),
                (if (.effective_tps.S // 0) > 0 then (if (.effective_tps.S // 0) >= 1000 then "\(((.effective_tps.S // 0) / 1000 * 10 | floor) / 10)k" else "\((.effective_tps.S // 0) | floor)" end) else "-" end),
                (if (.effective_tps.H // 0) > 0 then (if (.effective_tps.H // 0) >= 1000 then "\(((.effective_tps.H // 0) / 1000 * 10 | floor) / 10)k" else "\((.effective_tps.H // 0) | floor)" end) else "-" end),
                (if (.calls.O // 0) > 0 then (.calls.O // 0) else "-" end),
                (if (.calls.S // 0) > 0 then (.calls.S // 0) else "-" end),
                (if (.calls.H // 0) > 0 then (.calls.H // 0) else "-" end),
                (if (.tools.B // 0) > 0 then (.tools.B // 0) else "-" end),
                (if (.tools.R // 0) > 0 then (.tools.R // 0) else "-" end),
                (if (.tools.E // 0) > 0 then (.tools.E // 0) else "-" end),
                (if (.tools.W // 0) > 0 then (.tools.W // 0) else "-" end),
                (if (.tools.T // 0) > 0 then (.tools.T // 0) else "-" end),
                (if (.tools.M // 0) > 0 then (.tools.M // 0) else "-" end)
            ] | @tsv' 2>/dev/null | head -"$limit" | while IFS=$'\t' read -r date dur p out_o out_s out_h tps_o tps_s tps_h call_o call_s call_h bb rr ee ww tt mm; do
                # Build each section as fixed-width string then pad to exact section width
                local sec1=$(printf "%-10s %6s %5s" "$date" "$dur" "$p")
                local sec2=$(printf "%5s %6s %7s" "$out_o" "$out_s" "$out_h")
                local sec3=$(printf "%5s %5s %5s" "$tps_o" "$tps_s" "$tps_h")
                local sec4=$(printf "%4s %4s %4s" "$call_o" "$call_s" "$call_h")
                local sec5=$(printf "%4s %4s %4s %4s %4s %4s" "$bb" "$rr" "$ee" "$ww" "$tt" "$mm")

                # Pad each section to EXACT width: 23 │ 20 │ 18 │ 17 │ rest
                sec1=$(printf "%-23s" "$sec1")
                sec2=$(printf "%-20s" "$sec2")
                sec3=$(printf "%-18s" "$sec3")
                sec4=$(printf "%-17s" "$sec4")

                printf "%s${DIM}│${NC}%s${DIM}│${NC}%s${DIM}│${NC}%s${DIM}│${NC}%s\n" \
                    "$sec1" "$sec2" "$sec3" "$sec4" "$sec5"
            done

            # Legend
            echo -e "${DIM}───────────────────────${DIM}┼${DIM}────────────────────${DIM}┼${DIM}──────────────────${DIM}┼${DIM}─────────────────${DIM}┼${DIM}───────────────────────────${NC}"
            echo -e "${DIM}P=Prompts              ${DIM}│${DIM} thinking+text+tool ${DIM}│${DIM} OUTPUT+cache     ${DIM}│${DIM}                 ${DIM}│${DIM} B=Bash  R=Read  E=Edit${NC}"
            echo -e "${DIM}                       ${DIM}│${DIM} (generation)       ${DIM}│${DIM} (effective)      ${DIM}│${DIM}                 ${DIM}│${DIM} W=Write T=Task  M=MCP${NC}"
            echo ""
            echo -e "${YELLOW}O=Opus  S=Sonnet  H=Haiku │ Wall-clock gauge (session logs) │ Not absolute benchmarks${NC}"
            echo ""
            ;;

        stats|st)
            # aoa cc stats [--json]
            local json_output=false

            while [[ $# -gt 0 ]]; do
                case "$1" in
                    --json|-j) json_output=true; shift ;;
                    *) shift ;;
                esac
            done

            local project_path=$(get_project_root)
            if [ -z "$project_path" ]; then
                echo -e "${RED}Error: Not in an aOa-initialized project${NC}"
                return 1
            fi

            local result=$(curl -s "${INDEX_URL}/cc/stats?project_path=${project_path}")

            if $json_output; then
                echo "$result" | jq .
                return 0
            fi

            # Extract period data
            local has_today=$(echo "$result" | jq -r '.has_data.today')
            local has_7d=$(echo "$result" | jq -r '.has_data["7d"]')
            local has_30d=$(echo "$result" | jq -r '.has_data["30d"]')

            echo -e "${CYAN}${BOLD}⚡ CC Stats${NC}"
            echo ""

            # Model Distribution
            printf "${DIM}%-40s │ %20s %20s %20s${NC}\n" "MODELS" "TODAY" "7 DAYS" "30 DAYS"
            echo -e "${DIM}─────────────────────────────────────────┼─────────────────────────────────────────────────────────────────${NC}"

            echo "$result" | jq -r '.model_distribution | to_entries[] | [.key, .value.today, .value["7d"], .value["30d"]] | @tsv' 2>/dev/null | while IFS=$'\t' read -r model today d7 d30; do
                printf "%-40s │ %20s %20s %20s\n" "$model" \
                    "$([ "$today" != "0" ] && echo "$today" || echo "")" \
                    "$([ "$d7" != "0" ] && echo "$d7" || echo "")" \
                    "$([ "$d30" != "0" ] && echo "$d30" || echo "")"
            done

            echo ""
            printf "${DIM}%-40s │ %20s %20s %20s${NC}\n" "PERF" "TODAY" "7 DAYS" "30 DAYS"
            echo -e "${DIM}─────────────────────────────────────────┼─────────────────────────────────────────────────────────────────${NC}"

            # Velocity
            local vel_today=$(echo "$result" | jq -r '.periods.today.velocity // 0')
            local vel_7d=$(echo "$result" | jq -r '.periods["7d"].velocity // 0')
            local vel_30d=$(echo "$result" | jq -r '.periods["30d"].velocity // 0')
            printf "%-40s │ %20s %20s %20s\n" "Opus tokens/sec" \
                "$([ "$has_today" = "true" ] && echo "$vel_today" || echo "")" \
                "$([ "$has_7d" = "true" ] && echo "$vel_7d" || echo "")" \
                "$([ "$has_30d" = "true" ] && echo "$vel_30d" || echo "")"

            # Cache Hit
            local cache_today=$(echo "$result" | jq -r '.periods.today.cache_hit // 0')
            local cache_7d=$(echo "$result" | jq -r '.periods["7d"].cache_hit // 0')
            local cache_30d=$(echo "$result" | jq -r '.periods["30d"].cache_hit // 0')
            printf "%-40s │ %20s %20s %20s\n" "Cache hit %" \
                "$([ "$has_today" = "true" ] && echo "${cache_today}%" || echo "")" \
                "$([ "$has_7d" = "true" ] && echo "${cache_7d}%" || echo "")" \
                "$([ "$has_30d" = "true" ] && echo "${cache_30d}%" || echo "")"

            echo ""
            printf "${DIM}%-40s │ %20s %20s %20s${NC}\n" "TOKENS" "TODAY" "7 DAYS" "30 DAYS"
            echo -e "${DIM}─────────────────────────────────────────┼─────────────────────────────────────────────────────────────────${NC}"

            # Token rows
            local in_today=$(echo "$result" | jq -r '.periods.today.input_tokens // 0')
            local in_7d=$(echo "$result" | jq -r '.periods["7d"].input_tokens // 0')
            local in_30d=$(echo "$result" | jq -r '.periods["30d"].input_tokens // 0')
            printf "%-40s │ %20s %20s %20s\n" "Input" \
                "$([ "$has_today" = "true" ] && _format_tokens "$in_today" || echo "")" \
                "$([ "$has_7d" = "true" ] && _format_tokens "$in_7d" || echo "")" \
                "$([ "$has_30d" = "true" ] && _format_tokens "$in_30d" || echo "")"

            local out_today=$(echo "$result" | jq -r '.periods.today.output_tokens // 0')
            local out_7d=$(echo "$result" | jq -r '.periods["7d"].output_tokens // 0')
            local out_30d=$(echo "$result" | jq -r '.periods["30d"].output_tokens // 0')
            printf "%-40s │ %20s %20s %20s\n" "Output" \
                "$([ "$has_today" = "true" ] && _format_tokens "$out_today" || echo "")" \
                "$([ "$has_7d" = "true" ] && _format_tokens "$out_7d" || echo "")" \
                "$([ "$has_30d" = "true" ] && _format_tokens "$out_30d" || echo "")"

            local cr_today=$(echo "$result" | jq -r '.periods.today.cache_read // 0')
            local cr_7d=$(echo "$result" | jq -r '.periods["7d"].cache_read // 0')
            local cr_30d=$(echo "$result" | jq -r '.periods["30d"].cache_read // 0')
            printf "%-40s │ %20s %20s %20s\n" "Cache Read" \
                "$([ "$has_today" = "true" ] && _format_tokens "$cr_today" || echo "")" \
                "$([ "$has_7d" = "true" ] && _format_tokens "$cr_7d" || echo "")" \
                "$([ "$has_30d" = "true" ] && _format_tokens "$cr_30d" || echo "")"

            local cw_today=$(echo "$result" | jq -r '.periods.today.cache_write // 0')
            local cw_7d=$(echo "$result" | jq -r '.periods["7d"].cache_write // 0')
            local cw_30d=$(echo "$result" | jq -r '.periods["30d"].cache_write // 0')
            printf "%-40s │ %20s %20s %20s\n" "Cache Write" \
                "$([ "$has_today" = "true" ] && _format_tokens "$cw_today" || echo "")" \
                "$([ "$has_7d" = "true" ] && _format_tokens "$cw_7d" || echo "")" \
                "$([ "$has_30d" = "true" ] && _format_tokens "$cw_30d" || echo "")"

            echo -e "${DIM}─────────────────────────────────────────┴─────────────────────────────────────────────────────────────────${NC}"
            echo -e "${DIM}Blank = no data for period${NC}"
            ;;

        conversation|conv|c)
            # aoa cc conversation [--limit N] [--watch] - Show conversation flow
            local limit=5
            local json_output=false
            local watch_mode=false

            while [[ $# -gt 0 ]]; do
                case "$1" in
                    --limit|-l) limit="$2"; shift 2 ;;
                    --json|-j) json_output=true; shift ;;
                    --watch|-w) watch_mode=true; shift ;;
                    *) shift ;;
                esac
            done

            local project_path=$(get_project_root)
            if [ -z "$project_path" ]; then
                echo -e "${RED}Error: Not in an aOa-initialized project${NC}"
                return 1
            fi

            _show_conversation() {
                local limit="$1"
                local project_path="$2"

                # Get recent conversation (last 2 hours)
                local since=$(date -u -d '2 hours ago' '+%Y-%m-%dT%H:%M:%S.000Z' 2>/dev/null || date -u -v-2H '+%Y-%m-%dT%H:%M:%S.000Z' 2>/dev/null || echo "")
                local url="${INDEX_URL}/cc/conversation?limit=500&project_path=${project_path}"
                [ -n "$since" ] && url="${url}&since=${since}"
                local result=$(curl -s "$url")

                # Group and format turns (oldest first, newest at bottom)
                # Sort by timestamp first (API returns mixed order from multiple files)
                local turn_count=0
                echo "$result" | jq -r '
                  # Sort by timestamp to ensure chronological order
                  .texts | sort_by(.ts) |
                  # Group into turns - prompt STARTS each turn (user speaks, claude responds)
                  reduce .[] as $item (
                    {turns: [], current: []};
                    if $item.type == "prompt" then
                      # Prompt starts new turn - save previous turn if non-empty
                      (if .current | length > 0 then
                        {turns: (.turns + [.current]), current: [$item]}
                      else
                        {turns: .turns, current: [$item]}
                      end)
                    else
                      # Add thinking/output to current turn
                      {turns: .turns, current: (.current + [$item])}
                    end
                  ) |
                  # Add final turn (current conversation)
                  (if .current | length > 0 then .turns + [.current] else .turns end) |
                  # Take newest N turns, display oldest first (newest at bottom)
                  (.[-'"$limit"':]) |
                  length as $total |
                  to_entries[] |
                  # Mark the last turn as NOW
                  (if .key == ($total - 1) then "---NOW---" else "---TURN \(.key + 1)---" end),
                  # Prompt first (user speaks)
                  (.value | map(select(.type == "prompt")) | .[0] | "PROMPT\t\(.text // "")"),
                  # Then Claude responds (outputs)
                  (.value | map(select(.type == "output")) | .[] | "OUTPUT\t\(.text)"),
                  # Then thinking (nested under outputs)
                  (.value | map(select(.type == "thinking")) | .[] | "THINK\t\(.text)")
                ' 2>/dev/null | while IFS=$'\t' read -r type text; do
                    if [[ "$type" == "---NOW---" ]]; then
                        echo -e "${CYAN}${BOLD}─── ▶ NOW ────────────────────────────────────────────────────────────────────────${NC}"
                        continue
                    elif [[ "$type" == ---TURN* ]]; then
                        local turn_num="${type#---TURN }"
                        turn_num="${turn_num%---}"
                        echo -e "${DIM}─── ${turn_num} ─────────────────────────────────────────────────────────────────────────${NC}"
                        continue
                    fi

                    local short_text="${text:0:120}"
                    [ ${#text} -gt 120 ] && short_text="${short_text}..."

                    case "$type" in
                        PROMPT)
                            echo -e "${YELLOW}You ▸${NC} ${short_text}"
                            echo ""
                            ;;
                        OUTPUT)
                            echo -e "${GREEN}Claude ▸${NC} ${short_text}"
                            ;;
                        THINK)
                            echo -e "${DIM}    └─ ${short_text}${NC}"
                            ;;
                    esac
                done
            }

            if $json_output; then
                local since=$(date -u -d '2 hours ago' '+%Y-%m-%dT%H:%M:%S.000Z' 2>/dev/null || date -u -v-2H '+%Y-%m-%dT%H:%M:%S.000Z' 2>/dev/null || echo "")
                local url="${INDEX_URL}/cc/conversation?limit=500&project_path=${project_path}"
                [ -n "$since" ] && url="${url}&since=${since}"
                curl -s "$url" | jq .
                return 0
            fi

            if $watch_mode; then
                echo -e "${CYAN}${BOLD}⚡ CC Conversation${NC} │ Watching (Ctrl+C to stop)"
                echo ""
                while true; do
                    clear
                    echo -e "${CYAN}${BOLD}⚡ CC Conversation${NC} │ Last ${limit} turns │ $(date '+%H:%M:%S')"
                    echo ""
                    _show_conversation "$limit" "$project_path"
                    echo ""
                    echo -e "${DIM}Watching... (Ctrl+C to stop)${NC}"
                    sleep 2
                done
            else
                echo -e "${CYAN}${BOLD}⚡ CC Conversation${NC} │ Last ${limit} turns"
                echo ""
                _show_conversation "$limit" "$project_path"
                echo ""
                echo -e "${DIM}Use --watch to auto-refresh${NC}"
            fi
            ;;

        history|h)
            # Alias for prompts with more context
            cmd_cc prompts --limit "${1:-50}"
            ;;

        *)
            echo -e "${CYAN}${BOLD}aoa cc${NC} - Claude Code session insights"
            echo ""
            echo "Usage:"
            echo "  aoa cc prompts [--limit N]       Recent user prompts (--raw for hooks)"
            echo "  aoa cc conversation [--limit N]  Prompt→thinking→output flow (bigram source)"
            echo "  aoa cc sessions [--limit N]      Per-session metrics with model throughput"
            echo "  aoa cc stats                     Health dashboard (today/7d/30d)"
            echo ""
            echo "Examples:"
            echo "  aoa cc prompts                   Last 25 prompts"
            echo "  aoa cc conversation              Last 5 conversation turns (prompt+think+output)"
            echo "  aoa cc sessions                  Last 10 sessions with OUTPUT/TPS per model"
            echo "  aoa cc stats                     Model distribution, velocity, cache, tokens"
            ;;
    esac
}

# Helper to format token counts (e.g., 1500 -> 1.5k, 1500000 -> 1.5M)
_format_tokens() {
    local n=$1
    if [ "$n" -ge 1000000 ]; then
        echo "$(echo "scale=1; $n / 1000000" | bc)M"
    elif [ "$n" -ge 1000 ]; then
        echo "$(echo "scale=1; $n / 1000" | bc)k"
    else
        echo "$n"
    fi
}

# Fixed-width formatting helpers for table alignment
# Left-pad string to exact width (right-align)
_rpad() {
    local str="$1" width="$2"
    local len=${#str}
    if [ $len -ge $width ]; then
        echo "${str:0:$width}"
    else
        printf "%${width}s" "$str"
    fi
}

# Right-pad string to exact width (left-align)
_lpad() {
    local str="$1" width="$2"
    local len=${#str}
    if [ $len -ge $width ]; then
        echo "${str:0:$width}"
    else
        printf "%-${width}s" "$str"
    fi
}

# Format number to fixed width with K/M suffix
_fmt_num() {
    local n="$1" width="$2"
    local formatted
    if [ "$n" -ge 1000000 ] 2>/dev/null; then
        formatted="$(echo "scale=1; $n / 1000000" | bc)M"
    elif [ "$n" -ge 1000 ] 2>/dev/null; then
        formatted="$(echo "scale=1; $n / 1000" | bc)k"
    else
        formatted="$n"
    fi
    _rpad "$formatted" "$width"
}

# Format throughput rate with t/s suffix to fixed width
_fmt_rate() {
    local n="$1" width="$2"
    local formatted
    if [ -z "$n" ] || [ "$n" = "0" ] || [ "$n" = "null" ]; then
        _rpad "" "$width"
        return
    fi
    if [ "${n%.*}" -ge 1000 ] 2>/dev/null; then
        formatted="$(echo "scale=1; $n / 1000" | bc)k t/s"
    else
        formatted="${n} t/s"
    fi
    _rpad "$formatted" "$width"
}

# Format duration to fixed width
_fmt_dur() {
    local mins="$1" width="$2"
    local formatted
    if [ -z "$mins" ] || [ "$mins" = "0" ] || [ "$mins" = "null" ]; then
        _rpad "" "$width"
        return
    fi
    if [ "$mins" -ge 60 ] 2>/dev/null; then
        formatted="$((mins / 60))h$((mins % 60))m"
    else
        formatted="${mins}m"
    fi
    _rpad "$formatted" "$width"
}

