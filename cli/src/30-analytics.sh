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
#
# =============================================================================

cmd_hot() {
    # aoa hot [limit]     - Show frequently accessed "hot" files
    local limit="${1:-10}"

    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project=${project_id}"
    fi

    local result=$(curl -s "${INDEX_URL}/predict?limit=${limit}${project_param}")
    local count=$(echo "$result" | jq -r '.predictions | length // 0' 2>/dev/null || echo "0")

    printf "${CYAN}${BOLD}🔥 %s hot files${NC}\n" "$count"

    if [ "$count" -gt 0 ] 2>/dev/null; then
        echo "$result" | jq -r '.predictions[] | "  \(.file) (\(.score | . * 100 | floor)%)"' 2>/dev/null || true
    fi
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
        project_param="&project=${project_id}"
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
        project_param="?project=${project_id}"
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

cmd_predict() {
    # aoa predict [file]   - Predict next files based on current context
    local file="$1"

    local project_id=$(get_project_id)
    local project_param=""
    if [ -n "$project_id" ]; then
        project_param="&project=${project_id}"
    fi

    local url="${INDEX_URL}/transitions/predict?limit=5${project_param}"
    if [ -n "$file" ]; then
        url="${url}&file=${file}"
    fi

    local result=$(curl -s "$url")
    local count=$(echo "$result" | jq -r '.predictions | length // 0' 2>/dev/null || echo "0")

    if [ -n "$file" ]; then
        printf "${CYAN}${BOLD}🔮 %s predictions for %s${NC}\n" "$count" "$file"
    else
        printf "${CYAN}${BOLD}🔮 %s predictions${NC}\n" "$count"
    fi

    if [ "$count" -gt 0 ] 2>/dev/null; then
        echo "$result" | jq -r '.predictions[] | "  \(.file) (\(.confidence | . * 100 | floor)% confidence)"' 2>/dev/null || true
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
            json_input=$(echo "$json_input" | jq --arg pid "$project_id" '. + {project: $pid}')
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
            project_param="?project=${project_id}"
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
            project_param="?project=${project_id}"
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
        project_param="&project=${project_id}"
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
        echo "$result" | jq -r '.symbols[] | "  \(.signature // "\(.kind) \(.name)") [\(.start_line)-\(.end_line)]"' 2>/dev/null
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
        project_param="?project=${project_id}"
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

