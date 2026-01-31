# =============================================================================
# SECTION 50: Intent Tracking System
# =============================================================================
#
# PURPOSE
#   Tracks user intent through tool calls and queries. Used to predict
#   relevant files and understand workflow patterns over time.
#
# DEPENDENCIES
#   - 01-constants.sh: INDEX_URL, colors
#   - 02-utils.sh: get_project_id()
#
# COMMANDS PROVIDED
#   cmd_intent        Show recent activity and session metrics
#
# =============================================================================

# =============================================================================
# Intent Commands
# =============================================================================

cmd_intent() {
    # aoa intent - Show recent activity and session metrics
    # Usage: aoa intent [OPTIONS]
    # Flat command like aoa domains

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --help|-h)
                echo "Usage: aoa intent [OPTIONS]"
                echo ""
                echo "Show recent activity and session metrics."
                echo ""
                echo "Options:"
                echo "  -n, --limit N   Show N recent records (default: 20)"
                echo "  --help, -h      Show this help message"
                echo ""
                echo "Examples:"
                echo "  aoa intent              # Show 20 recent records"
                echo "  aoa intent -n 10        # Show 10 recent records"
                return 0
                ;;
            *)
                break
                ;;
        esac
    done

    # Pass all args to the display function
    cmd_intent_recent "$@"
}

cmd_intent_recent() {
    local limit=20

    # Parse -n flag for limit (consistent with aoa domains -n)
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -n|--limit)
                limit="$2"
                shift 2
                ;;
            -n*)
                limit="${1#-n}"
                shift
                ;;
            *)
                # First positional arg is also limit for backwards compat
                if [[ "$1" =~ ^[0-9]+$ ]]; then
                    limit="$1"
                fi
                shift
                ;;
        esac
    done

    local project_id=$(get_project_id)

    # Check if project is initialized
    if [ -z "$project_id" ]; then
        echo -e "${CYAN}${BOLD}⚡ aOa Activity${NC}"
        echo ""
        echo -e "${DIM}No project initialized. Run 'aoa init' first.${NC}"
        return 0
    fi

    # Get metrics for header
    local metrics=$(curl -s "${INDEX_URL}/metrics?project_id=${project_id}")

    # Check if API is reachable
    if [ -z "$metrics" ]; then
        echo -e "${CYAN}${BOLD}⚡ aOa Activity${NC}"
        echo ""
        echo -e "${RED}Cannot connect to aOa services at ${INDEX_URL}${NC}"
        echo -e "${DIM}Check that Docker is running: docker ps${NC}"
        return 1
    fi
    local tokens_saved=$(echo "$metrics" | jq -r '.savings.tokens // 0')
    local time_saved_sec=$(echo "$metrics" | jq -r '.savings.time_sec // 0')
    local hit_pct=$(echo "$metrics" | jq -r '.rolling.hit_at_5_pct // 0')
    local evaluated=$(echo "$metrics" | jq -r '.rolling.evaluated // 0')
    local hits=$(echo "$metrics" | jq -r '.rolling.hits // 0')

    # Get intent stats (includes first_seen for date display)
    local intent_stats=$(curl -s "${INDEX_URL}/intent/stats?project_id=${project_id}")
    local first_seen=$(echo "$intent_stats" | jq -r '.first_seen // 0')

    # Get domain learning stats (GL-054)
    local domain_stats=$(curl -s "${INDEX_URL}/domains/stats?project_id=${project_id}")
    local domain_count=$(echo "$domain_stats" | jq -r '.domains // 0')
    local prompt_count=$(echo "$domain_stats" | jq -r '.prompt_count // 0')
    local prompt_threshold=$(echo "$domain_stats" | jq -r '.prompt_threshold // 10')
    local seconds_to_autotune=$(echo "$domain_stats" | jq -r '.seconds_to_autotune // 0')

    # GL-088: Get hit-based tags (actual usage, not pattern-matched)
    local hit_stats=$(curl -s "${INDEX_URL}/intent/hits?project_id=${project_id}&limit=5")
    local top_domains=$(echo "$hit_stats" | jq -r '.domains[:3] | .[].name' 2>/dev/null | tr '\n' ' ')
    local top_terms=$(echo "$hit_stats" | jq -r '.terms[:3] | .[].name' 2>/dev/null | tr '\n' ' ')
    local recent_hits=$(echo "$hit_stats" | jq -r '.recent[:5] | join(" ")' 2>/dev/null)

    # Get recent intent records
    # Request 3x limit from API to ensure enough after filtering out records without files
    local api_limit=$((limit * 3))
    local result=$(curl -s "${INDEX_URL}/intent/recent?limit=${api_limit}&project_id=${project_id}")
    local total=$(echo "$result" | jq -r '.stats.total_records // 0')

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
    local hit_pct_int=$(printf "%.0f" "$hit_pct")

    # Get time range from metrics endpoint (calculated server-side with dynamic rates)
    local time_low=$(echo "$metrics" | jq -r '.savings.time_sec_low // 0' | awk '{printf "%.0f", $1}')
    local time_high=$(echo "$metrics" | jq -r '.savings.time_sec_high // 0' | awk '{printf "%.0f", $1}')

    # Format time range compactly
    format_time_compact() {
        local sec=$1
        if [ "$sec" -ge 3600 ] 2>/dev/null; then
            awk "BEGIN {printf \"%.1fh\", $sec / 3600}"
        elif [ "$sec" -ge 60 ] 2>/dev/null; then
            awk "BEGIN {printf \"%.0fm\", $sec / 60}"
        else
            echo "${sec}s"
        fi
    }

    local time_display=""
    if [ "$tokens_saved" -gt 0 ] 2>/dev/null && [ "$time_high" -gt 0 ] 2>/dev/null; then
        local t_low=$(format_time_compact $time_low)
        local t_high=$(format_time_compact $time_high)
        if [ "$t_low" = "$t_high" ]; then
            time_display="⚡~${t_low}"
        else
            time_display="⚡${t_low}-${t_high}"
        fi
    fi

    # Format first_seen as date (e.g., "Jan 11")
    local since_date=""
    if [ "$first_seen" -gt 0 ] 2>/dev/null; then
        since_date=$(date -d "@${first_seen}" "+%b %d" 2>/dev/null || date -r "${first_seen}" "+%b %d" 2>/dev/null)
    fi

    # Header - horizontal format matching aoa domains style
    local min_intents=30

    # Build savings display
    local savings_display=""
    if [ "$tokens_saved" -gt 0 ] 2>/dev/null; then
        local date_suffix=""
        [ -n "$since_date" ] && date_suffix=" (${since_date})"
        savings_display="Savings ${GREEN}↓${tokens_fmt}${NC}${date_suffix}"
    else
        savings_display="Savings ${DIM}(none yet)${NC}"
    fi

    # Build predictions display
    local pred_display=""
    if [ "$total" -lt "$min_intents" ] 2>/dev/null; then
        pred_display="Predictions: ${YELLOW}learning${NC}"
    elif [ "$hit_pct_int" -ge 80 ] 2>/dev/null; then
        pred_display="Predictions: ${GREEN}${hit_pct_int}%${NC}"
    else
        pred_display="Predictions: ${YELLOW}${hit_pct_int}%${NC}"
    fi

    # Assemble header line (only include time section if populated)
    local header="${CYAN}${BOLD}⚡ aOa Activity${NC}  ${savings_display}"
    if [ -n "$time_display" ]; then
        header="${header} ${DIM}│${NC} ${CYAN}${time_display}${NC}"
    fi
    header="${header} ${DIM}│${NC} ${pred_display}"
    echo -e "$header"
    echo -e "${DIM}─────────────────────────────────────────────────────────────────────────────────────────────${NC}"
    echo ""
    echo -e "ACTION     SOURCE   ATTRIB       aOa IMPACT                TAGS                                                    TARGET"

    # Process records - extract file_size (baseline) and output_size (actual)
    # Skip records with no files (noise) - require at least one real file
    echo "$result" | jq -r --arg project_root "$(get_project_root)" '.records[] |
        select(.files | length > 0) |
        select(.files[0] != null and .files[0] != "") |
        {
            tool: .tool,
            files: (.files[0] // "unknown"),
            file_count: (.files | length),
            tags: (.tags[0:6] | join(" ")),
            file_size: (if (.file_sizes and .files[0]) then .file_sizes[.files[0]] else null end),
            output_size: .output_size,
            confidence: .confidence
        } |
        # Use ASCII Unit Separator (0x1F) - designed for field separation, never in user input
        "\(.tool)\u001f\(.files)\u001f\(.tags)\u001f\(.file_size // 0)\u001f\(.output_size // 0)\u001f\(.file_count)\u001f\(.confidence // 0)"
    ' 2>/dev/null | head -n "$limit" | while IFS=$'\x1f' read -r tool file tags file_size output_size file_count confidence; do
        # Map tool to action and determine source
        local action="$tool"
        local source="Claude"

        case "$tool" in
            Bash)
                # Check for aOa commands - format: cmd:aoa:type:term:hits:time
                # Term may contain escaped colons (\:), so parse carefully
                if echo "$file" | grep -q "^cmd:aoa:"; then
                    source="aOa"
                    # Extract type (field 3)
                    local aoa_type=$(echo "$file" | cut -d: -f3)
                    # Map aOa command to display action
                    case "$aoa_type" in
                        grep|search|multi)  action="Search" ;;
                        egrep|pattern)      action="Search" ;;
                        find)               action="Find" ;;
                        tree)               action="Tree" ;;
                        locate)             action="Locate" ;;
                        head|tail|lines)    action="Read" ;;
                        hot)                action="Predict" ;;
                        touched)            action="Intent" ;;
                        focus)              action="Memory" ;;
                        predict)            action="Predict" ;;
                        outline)            action="Outline" ;;
                        *)                  action="Search" ;;
                    esac
                    # Extract hits:time from end (last 2 colon-separated fields)
                    local aoa_time=$(echo "$file" | rev | cut -d: -f1 | rev)
                    local aoa_hits=$(echo "$file" | rev | cut -d: -f2 | rev)
                    # Extract term: everything between field 3 and hits:time
                    # Remove "cmd:aoa:type:" prefix and ":hits:time" suffix
                    local aoa_term=$(echo "$file" | sed "s/^cmd:aoa:${aoa_type}://" | sed "s/:${aoa_hits}:${aoa_time}$//")
                    # URL-decode the term (handles all special chars: |, \, :, etc.)
                    aoa_term=$(python3 -c "from urllib.parse import unquote; print(unquote('$aoa_term'))" 2>/dev/null || echo "$aoa_term")
                    # Store for later use in attribution/impact
                    export AOA_SEARCH_TYPE="$aoa_type"
                    export AOA_SEARCH_HITS="$aoa_hits"
                    export AOA_SEARCH_TIME="$aoa_time"
                    # Store the full command for display (will be formatted at print time)
                    local display_term="${aoa_term:0:50}"
                    [ ${#aoa_term} -gt 50 ] && display_term="${display_term}..."
                    # Mark as aOa command for special formatting at print time
                    file="AOA_CMD:${display_term}"
                # Old format: cmd:aoa search term (space separated)
                elif echo "$file" | grep -q "^cmd:aoa "; then
                    action="Search"
                    source="aOa"
                    local aoa_term=$(echo "$file" | sed 's/^cmd:aoa [a-z]* //')
                    export AOA_SEARCH_TYPE="search"
                    export AOA_SEARCH_HITS=""
                    export AOA_SEARCH_TIME=""
                    file="\"${aoa_term}\""
                fi
                ;;
            Outline)
                source="aOa"
                ;;
            HaikuTag|Intent)
                action="Intent"
                source="aOa"
                ;;
            Predict)
                source="aOa"
                ;;
            Learn)
                # Domain learning event - format: learn:domains_added:tokens_invested
                action="Learn"
                source="aOa"
                # file contains: learn:3:201 (3 domains, 201 tokens)
                local learn_count=$(echo "$file" | cut -d: -f2)
                local learn_tokens=$(echo "$file" | cut -d: -f3)
                export AOA_LEARN_COUNT="$learn_count"
                export AOA_LEARN_TOKENS="$learn_tokens"
                ;;
            Tune)
                # Domain tuning event - format: tune:cycle:stale:pruned
                action="Tune"
                source="aOa"
                # file contains: tune:121:35:0 (cycle 121, 35 stale, 0 pruned)
                local tune_cycle=$(echo "$file" | cut -d: -f2)
                local tune_stale=$(echo "$file" | cut -d: -f3)
                local tune_pruned=$(echo "$file" | cut -d: -f4)
                export AOA_TUNE_CYCLE="$tune_cycle"
                export AOA_TUNE_STALE="$tune_stale"
                export AOA_TUNE_PRUNED="$tune_pruned"
                ;;
        esac

        # Calculate tokens from file size if available
        local actual base saved time_saved base_tokens

        # If we have file size, use it for more accurate baseline
        if [ "$file_size" -gt 0 ] 2>/dev/null; then
            # BASE = file size in bytes / 4 (Claude uses ~4 chars per token)
            base_tokens=$((file_size / 4))

            # Format with k suffix if > 1000
            if [ $base_tokens -ge 1000 ]; then
                local base_k=$(awk "BEGIN {printf \"%.1f\", $base_tokens/1000}")
                base="${base_k}k"
            else
                base="$base_tokens"
            fi
        else
            # Fallback to defaults if no file size
            base_tokens=0
            base="?"
        fi

        # ACTUAL and SAVED - only show REAL metrics, never fabricate
        case "$action" in
            Grep|Glob)
                # Grep/Glob bypass aOa - warn user to use aoa search instead
                actual="SLOW"
                saved="SLOW"
                time_saved="SLOW"
                ;;
            Outline)
                # AI-generated symbol-level tags - enrichment, not reduction
                actual="AI"
                saved="AI"
                time_saved="-"
                ;;
            HaikuTag)
                # AI-generated tags - enrichment, not reduction
                actual="AI"
                saved="AI"
                time_saved="-"
                ;;
            *)
                # Check if we have REAL output_size data
                if [ "$output_size" -gt 0 ] 2>/dev/null; then
                    # REAL data: calculate actual tokens from output_size
                    local actual_tokens=$((output_size / 4))

                    # Format actual with k suffix if > 1000
                    if [ $actual_tokens -ge 1000 ]; then
                        local actual_k=$(awk "BEGIN {printf \"%.1f\", $actual_tokens/1000}")
                        actual="${actual_k}k"
                    else
                        actual="$actual_tokens"
                    fi

                    # Calculate savings if we have baseline
                    if [ $base_tokens -gt 0 ]; then
                        local saved_tokens=$((base_tokens - actual_tokens))
                        if [ $saved_tokens -gt 0 ]; then
                            if [ $saved_tokens -ge 1000 ]; then
                                local saved_k=$(awk "BEGIN {printf \"%.1f\", $saved_tokens/1000}")
                                saved="${saved_k}k"
                            else
                                saved="$saved_tokens"
                            fi
                        else
                            saved="0"
                        fi
                    else
                        saved="-"
                    fi
                    time_saved="-"
                else
                    # No output_size captured yet - be honest
                    actual="-"
                    saved="-"
                    time_saved="-"
                fi
                ;;
        esac

        # Format saved with k suffix if numeric and > 1000
        if [[ "$saved" =~ ^[0-9]+$ ]] && [ "$saved" -ge 1000 ]; then
            local saved_k=$(awk "BEGIN {printf \"%.1f\", $saved/1000}")
            saved="${saved_k}k"
        fi

        # Format file path (project-relative)
        local target=$(echo "$file" | sed "s|$(get_project_root)/||" 2>/dev/null || echo "$file")

        # Calculate attribution and impact
        local impact attribution

        # Special handling for aOa native operations - tell meaningful stories
        if [ "$action" = "Predict" ]; then
            # Use confidence field (0-1 float) if available
            if [ -n "$confidence" ] && [ "$confidence" != "0" ] && [ "$confidence" != "null" ]; then
                # Convert float to percentage (e.g., 0.45 -> 45%)
                local conf_pct=$(awk "BEGIN {printf \"%.0f\", $confidence * 100}")
                attribution="${CYAN}${conf_pct}%${NC} conf"
            else
                attribution="${CYAN}predicted${NC}"
            fi
            # Show predicted file count
            if [ "$file_count" -gt 1 ] 2>/dev/null; then
                impact="${CYAN}${file_count} files${NC} suggested"
            else
                impact="${CYAN}1 file${NC} suggested"
            fi
            # TARGET: show keywords (tags without #) instead of first file
            local keywords=$(echo "$tags" | sed 's/#//g' | xargs)
            target="keywords: ${keywords}"
        elif [ "$action" = "Intent" ]; then
            # Haiku semantic tagging
            attribution="${CYAN}semantic${NC}"
            local tag_count=$(echo "$tags" | wc -w)
            impact="${CYAN}${tag_count} tags${NC} generated"
        elif [ "$action" = "Outline" ]; then
            # Target-level enrichment
            attribution="${CYAN}targets${NC}"
            impact="${CYAN}enriched${NC}"
        elif [ "$action" = "Search" ] || [ "$action" = "Find" ] || [ "$action" = "Locate" ] || [ "$action" = "Tree" ]; then
            # aOa search/discovery operations - show command type as attribution
            # Detect pattern search: if grep took >5ms, it was routed to egrep (pattern search)
            local search_type="$AOA_SEARCH_TYPE"
            if [[ "$search_type" == "grep" ]] && [[ -n "$AOA_SEARCH_TIME" ]]; then
                local time_int=$(printf "%.0f" "$AOA_SEARCH_TIME" 2>/dev/null || echo "0")
                [ "$time_int" -gt 5 ] && search_type="egrep"
            fi
            case "$search_type" in
                indexed)      attribution="${CYAN}indexed${NC}" ;;
                multi-or)     attribution="${CYAN}multi-or${NC}" ;;
                multi-and)    attribution="${CYAN}multi-and${NC}" ;;
                regex)        attribution="${CYAN}regex${NC}" ;;
                grep|search)  attribution="${CYAN}indexed${NC}" ;;
                egrep|pattern) attribution="${CYAN}regex${NC}" ;;
                multi)        attribution="${CYAN}multi-and${NC}" ;;
                find)         attribution="${CYAN}files${NC}" ;;
                tree)         attribution="${CYAN}structure${NC}" ;;
                locate)       attribution="${CYAN}filename${NC}" ;;
                head|tail|lines) attribution="${CYAN}content${NC}" ;;
                hot)          attribution="${CYAN}frequency${NC}" ;;
                touched)      attribution="${CYAN}session${NC}" ;;
                focus)        attribution="${CYAN}context${NC}" ;;
                predict)      attribution="${CYAN}predicted${NC}" ;;
                *)            attribution="${CYAN}indexed${NC}" ;;
            esac
            # Truncate time to 2 decimal places
            local time_display=$(printf "%.2f" "$AOA_SEARCH_TIME" 2>/dev/null || echo "$AOA_SEARCH_TIME")
            # Impact shows hits and timing
            if [ -n "$AOA_SEARCH_HITS" ] && [ "$AOA_SEARCH_HITS" != "0" ]; then
                impact="${CYAN}${BOLD}${AOA_SEARCH_HITS} hits${NC} ${DIM}│${NC} ${GREEN}${time_display}ms${NC}"
            else
                impact="${DIM}0 hits${NC}"
            fi
        # Determine attribution based on source and savings
        # aOa brand color is CYAN (consistent with status line)
        elif [ "$source" = "aOa" ]; then
            attribution="${CYAN}${BOLD}aOa${NC}"
        else
            attribution="-"  # Default, may be upgraded to "aOa guided" below
        fi

        if [ "$action" = "Learn" ]; then
            # Domain learning event
            attribution="${GREEN}+${AOA_LEARN_COUNT} domains${NC}"
            impact="${CYAN}${AOA_LEARN_TOKENS} tokens${NC} invested"
            tags="${CYAN}#learning${NC}"
            target="${BOLD}${YELLOW}This is the way.${NC}"
        elif [ "$action" = "Tune" ]; then
            # Domain tuning event
            attribution="${CYAN}cycle ${AOA_TUNE_CYCLE}${NC}"
            impact="${DIM}${AOA_TUNE_STALE} stale, ${AOA_TUNE_PRUNED} pruned${NC}"
            tags="${CYAN}#tuning${NC}"
            target="${BOLD}${YELLOW}This is the way.${NC}"
        elif [ "$action" = "Predict" ] || [ "$action" = "Intent" ] || [ "$action" = "Outline" ] || [ "$action" = "Search" ] || [ "$action" = "Find" ] || [ "$action" = "Locate" ] || [ "$action" = "Tree" ] || [ "$action" = "Memory" ]; then
            : # Impact already set above for aOa native operations
        elif [ "$saved" = "-" ]; then
            impact="-"
        elif [ "$saved" = "SLOW" ]; then
            # Glob/Grep warning - short to fit 25-char column
            impact="${YELLOW}slow${NC} ${DIM}→ aoa grep${NC}"
        elif [ "$saved" = "AI" ]; then
            if [ "$action" = "Outline" ]; then
                impact="${CYAN}symbol tags${NC}"
            else
                impact="${CYAN}semantic tags${NC}"
            fi
        else
            # Check if we have REAL savings data (both baseline AND actual output)
            if [ "$output_size" -gt 0 ] 2>/dev/null && [ "$base_tokens" -gt 0 ]; then
                # REAL measured savings - calculate percentage
                local actual_tokens=$((output_size / 4))
                local saved_tokens=$((base_tokens - actual_tokens))
                if [ $saved_tokens -gt 0 ]; then
                    local pct=$((saved_tokens * 100 / base_tokens))
                    # aOa-enabled: significant reduction means aOa guided Claude to read only what it needed
                    if [ $pct -ge 50 ]; then
                        # High savings = aOa guided (this is the value prop!)
                        # CYAN "aOa" (brand color) + GREEN "guided"
                        attribution="${CYAN}${BOLD}aOa${NC} ${GREEN}guided${NC}"
                        impact="${GREEN}${BOLD}↓${pct}%${NC} (${base} → ${actual})"
                    else
                        # Modest savings
                        impact="${GREEN}${pct}%${NC} (${base} → ${actual})"
                    fi
                else
                    # Full file read - standard Claude operation
                    impact="-"
                fi
            else
                # No measurement data - standard operation
                impact="-"
            fi
        fi

        # Truncate tags to 55 chars (room for 5-6 tags typically)
        if [ ${#tags} -gt 55 ]; then
            tags="${tags:0:52}..."
        fi

        # Color the source if it's aOa (brand consistency)
        local source_display
        if [ "$source" = "aOa" ]; then
            source_display="${CYAN}${BOLD}aOa${NC}"
        else
            source_display="$source"
        fi

        # Manual padding for color code compatibility
        # Strip ANSI codes to get visible length for source
        local source_visible=$(echo -e "$source_display" | sed 's/\x1b\[[0-9;]*m//g')
        local source_len=${#source_visible}
        local source_pad=$((8 - source_len))

        # Strip ANSI codes to get visible length for attribution
        local attrib_visible=$(echo -e "$attribution" | sed 's/\x1b\[[0-9;]*m//g')
        local attrib_len=${#attrib_visible}
        local attrib_pad=$((12 - attrib_len))

        # Strip ANSI codes for impact
        local impact_visible=$(echo -e "$impact" | sed 's/\x1b\[[0-9;]*m//g')
        local impact_len=${#impact_visible}
        local impact_pad=$((25 - impact_len))

        printf "%-10s " "$action"
        echo -ne "$source_display"
        [ $source_pad -gt 0 ] && printf "%${source_pad}s" ""
        echo -ne " $attribution"
        [ $attrib_pad -gt 0 ] && printf "%${attrib_pad}s" ""
        echo -ne " $impact"
        [ $impact_pad -gt 0 ] && printf "%${impact_pad}s" ""
        # Tags need echo -e to render color codes (e.g., for Tune/Learn events)
        # Strip ANSI codes for padding calculation
        local tags_visible=$(echo -e "$tags" | sed 's/\x1b\[[0-9;]*m//g')
        local tags_len=${#tags_visible}
        local tags_pad=$((55 - tags_len))
        echo -n " "
        echo -ne "$tags"
        [ $tags_pad -gt 0 ] && printf "%${tags_pad}s" ""
        echo -n " "
        # Format aOa commands with branding: aOa in CYAN, command in GREEN
        if [[ "$target" == AOA_CMD:* ]]; then
            local aoa_display="${target#AOA_CMD:}"
            # Parse: aoa <cmd> [rest]
            if [[ "$aoa_display" =~ ^aoa\ +([a-z]+)(\ +.*)? ]]; then
                local cmd="${BASH_REMATCH[1]}"
                local rest="${BASH_REMATCH[2]:-}"
                echo -e "${CYAN}aOa${NC} ${GREEN}${cmd}${NC}${rest}"
            else
                echo -e "${CYAN}aOa${NC} ${aoa_display#aoa }"
            fi
        else
            # Use echo -e for targets with color codes (e.g., Tune/Learn messages)
            echo -e "$target"
        fi
    done

    # Intent Angle footer - GL-088: clean value statement, no counters
    echo ""
    echo -e "${CYAN}${BOLD}⚡ Intent Angle${NC}"
    echo -e "${DIM}─────────────────────────────────────────────────────────────────────────────────────────────${NC}"

    # Show domains + top hits (if any) + value statement
    if [ -n "$recent_hits" ] && [ "$recent_hits" != "null" ] && [ "$recent_hits" != "" ]; then
        echo -e "${CYAN}${domain_count} domains${NC} ${DIM}│${NC} ${MAGENTA}${recent_hits}${NC} ${DIM}│${NC} ${CYAN}aOa learns your workflow →${NC} ${BOLD}${YELLOW}Claude reads only what it needs.${NC}"
    else
        echo -e "${CYAN}${domain_count} domains${NC} ${DIM}│${NC} ${CYAN}aOa learns your workflow →${NC} ${BOLD}${YELLOW}Claude reads only what it needs.${NC}"
    fi
}
