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
#   cmd_intent        Main intent command dispatcher
#   cmd_intent_recent Show recent intents
#   cmd_intent_tags   Show intent-derived tags
#   cmd_intent_files  Files touched by recent intent
#   cmd_intent_file   Intent history for a specific file
#   cmd_intent_stats  Intent statistics
#   cmd_intent_store  Store a new intent
#   cmd_rate          Rate a prediction (feedback loop)
#
# =============================================================================

# =============================================================================
# Intent Commands
# =============================================================================

cmd_intent() {
    local subcmd="${1:-recent}"
    shift || true

    case "$subcmd" in
        recent|r)
            cmd_intent_recent "$@"
            ;;
        tags|t)
            cmd_intent_tags "$@"
            ;;
        files|f)
            cmd_intent_files "$@"
            ;;
        file)
            cmd_intent_file "$@"
            ;;
        stats)
            cmd_intent_stats "$@"
            ;;
        store|s)
            cmd_intent_store "$@"
            ;;
        *)
            echo -e "${BOLD}Intent Tracking${NC}"
            echo ""
            echo "Commands:"
            echo "  aoa intent recent [since]   Recent intent records (e.g., 1h, 30m)"
            echo "  aoa intent tags             All tags with file counts"
            echo "  aoa intent files <tag>      Files associated with a tag"
            echo "  aoa intent file <path>      Tags associated with a file"
            echo "  aoa intent stats            Intent index statistics"
            echo "  aoa intent store <tags> [files...]  Store AI-generated intent tags"
            ;;
    esac
}

cmd_intent_recent() {
    local limit="${1:-20}"
    local project_id=$(get_project_id)

    # Get metrics for header
    local metrics=$(curl -s "${INDEX_URL}/metrics?project_id=${project_id}")
    local tokens_saved=$(echo "$metrics" | jq -r '.savings.tokens // 0')
    local time_saved_sec=$(echo "$metrics" | jq -r '.savings.time_sec // 0')
    local hit_pct=$(echo "$metrics" | jq -r '.rolling.hit_at_5_pct // 0')
    local evaluated=$(echo "$metrics" | jq -r '.rolling.evaluated // 0')
    local hits=$(echo "$metrics" | jq -r '.rolling.hits // 0')

    # Get intent stats (includes first_seen for date display)
    local intent_stats=$(curl -s "${INDEX_URL}/intent/stats?project_id=${project_id}")
    local first_seen=$(echo "$intent_stats" | jq -r '.first_seen // 0')

    # Get recent intent records
    local result=$(curl -s "${INDEX_URL}/intent/recent?limit=${limit}&project_id=${project_id}")
    local total=$(echo "$result" | jq -r '.stats.total_records // 0')

    # Format tokens with k suffix
    local tokens_k=$(awk "BEGIN {printf \"%.0f\", $tokens_saved/1000}")
    local hit_pct_int=$(printf "%.0f" "$hit_pct")

    # Calculate dynamic rate from rolling windows (5, 15, 30 min)
    local rate_data=$(python3 << 'PYEOF'
import json
import os
from pathlib import Path
from datetime import datetime, timedelta

home = os.path.expanduser('~')
projects_dir = Path(home) / '.claude' / 'projects'

if not projects_dir.exists():
    print(json.dumps({'rate_low': 2.0, 'rate_high': 5.0, 'samples': 0}))
    exit()

# Get recent session files
sessions = []
for project_dir in sorted(projects_dir.iterdir(), key=lambda p: p.stat().st_mtime, reverse=True)[:3]:
    sessions.extend(sorted(project_dir.glob('*.jsonl'), key=lambda p: p.stat().st_mtime, reverse=True)[:5])

now = datetime.now().astimezone()
windows = {'5min': [], '15min': [], '30min': []}

for session_file in sessions[:10]:
    try:
        messages = []
        with open(session_file, 'r') as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    event = json.loads(line)
                    if event.get('type') == 'assistant' and 'message' in event:
                        msg = event['message']
                        if 'usage' in msg and 'timestamp' in event:
                            ts = datetime.fromisoformat(event['timestamp'].replace('Z', '+00:00'))
                            tokens = msg['usage'].get('input_tokens', 0) + msg['usage'].get('output_tokens', 0)
                            messages.append({'ts': ts, 'tokens': tokens})
                except:
                    continue

        # Calculate rates between consecutive messages
        for i in range(1, len(messages)):
            try:
                duration_ms = (messages[i]['ts'] - messages[i-1]['ts']).total_seconds() * 1000
                tokens = messages[i]['tokens']
                age = (now - messages[i]['ts']).total_seconds() / 60  # minutes ago

                # Only include fast responses (< 15s) - pure LLM processing without tool delays
                # Filter: 100ms-15s duration, significant tokens, rate < 20ms/token
                if 100 < duration_ms < 15000 and tokens > 200:
                    rate = duration_ms / tokens
                    # Cap at 20ms/token - anything higher is tool/network overhead
                    if rate < 20:
                        if age <= 5:
                            windows['5min'].append(rate)
                        if age <= 15:
                            windows['15min'].append(rate)
                        if age <= 30:
                            windows['30min'].append(rate)
            except:
                continue
    except:
        continue

# Use P25 (faster end) since we're saving INPUT tokens which process faster
def percentile(lst, p):
    if not lst:
        return None
    s = sorted(lst)
    idx = int(len(s) * p / 100)
    return s[min(idx, len(s)-1)]

rates = []
for w in ['5min', '15min', '30min']:
    # P25 gives us the faster, cleaner samples
    p = percentile(windows[w], 25)
    if p is not None:
        rates.append(p)

if rates:
    rate_low = min(rates)
    rate_high = max(rates)
    samples = sum(len(v) for v in windows.values())
    print(json.dumps({'rate_low': round(rate_low, 2), 'rate_high': round(rate_high, 2), 'samples': samples}))
else:
    # Fallback: documented input processing rate (~2ms/token)
    print(json.dumps({'rate_low': 1.5, 'rate_high': 3.0, 'samples': 0}))
PYEOF
)

    local rate_low=$(echo "$rate_data" | jq -r '.rate_low')
    local rate_high=$(echo "$rate_data" | jq -r '.rate_high')
    local rate_samples=$(echo "$rate_data" | jq -r '.samples')

    # Calculate time range from dynamic rates
    local time_low=$(awk "BEGIN {printf \"%.0f\", $tokens_saved * $rate_low / 1000}")
    local time_high=$(awk "BEGIN {printf \"%.0f\", $tokens_saved * $rate_high / 1000}")

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

    # Header
    echo -e "${CYAN}${BOLD}aOa Activity${NC}                                                 Session"
    echo ""
    # Only show savings if we have REAL data (not fabricated)
    if [ "$tokens_saved" -gt 0 ] 2>/dev/null; then
        local savings_suffix=""
        if [ -n "$since_date" ]; then
            savings_suffix=" ${DIM}(since ${since_date})${NC}"
        fi
        if [ -n "$time_display" ]; then
            echo -e "${BOLD}SAVINGS${NC}         ${GREEN}↓${tokens_k}k tokens${NC} ${CYAN}${time_display}${NC}${savings_suffix}"
        else
            echo -e "${BOLD}SAVINGS${NC}         ${GREEN}↓${tokens_k}k tokens${NC}${savings_suffix}"
        fi
    else
        echo -e "${BOLD}SAVINGS${NC}         ${DIM}(no measured savings yet)${NC}"
    fi
    # Show prediction capability level (matches status line logic)
    local min_intents=30
    if [ "$total" -lt "$min_intents" ] 2>/dev/null; then
        echo -e "${BOLD}PREDICTIONS${NC}     ${YELLOW}Learning${NC} (${total}/${min_intents} intents toward full prediction)"
    elif [ "$hit_pct_int" -ge 80 ] 2>/dev/null; then
        echo -e "${BOLD}PREDICTIONS${NC}     ${GREEN}${hit_pct_int}% accuracy${NC} (${evaluated} predictions evaluated)"
    else
        echo -e "${BOLD}PREDICTIONS${NC}     ${YELLOW}${hit_pct_int}% accuracy${NC} (${evaluated} predictions, improving)"
    fi
    echo -e "${BOLD}HOW IT WORKS${NC}    ${CYAN}aOa finds exact locations${NC}, so Claude reads only what it needs"
    echo -e "                ${DIM}Time: rolling avg (5/15/30min) of input token processing${NC}"
    echo ""
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
            output_size: .output_size
        } |
        # Use ASCII Unit Separator (0x1F) - designed for field separation, never in user input
        "\(.tool)\u001f\(.files)\u001f\(.tags)\u001f\(.file_size // 0)\u001f\(.output_size // 0)\u001f\(.file_count)"
    ' 2>/dev/null | while IFS=$'\x1f' read -r tool file tags file_size output_size file_count; do
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
            # Extract confidence from tags (format: @XX%)
            local confidence=$(echo "$tags" | grep -oE '@[0-9]+%' | head -1 | tr -d '@')
            if [ -n "$confidence" ]; then
                attribution="${CYAN}${confidence}${NC} conf"
                # Remove confidence from tags to avoid duplication
                tags=$(echo "$tags" | sed 's/@[0-9]*%//' | sed 's/  / /g' | xargs)
            else
                attribution="${CYAN}predicted${NC}"
            fi
            # Show predicted file count
            if [ "$file_count" -gt 1 ] 2>/dev/null; then
                impact="${CYAN}${file_count} files${NC} suggested"
            else
                impact="${CYAN}1 file${NC} suggested"
            fi
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

        if [ "$action" = "Predict" ] || [ "$action" = "Intent" ] || [ "$action" = "Outline" ] || [ "$action" = "Search" ] || [ "$action" = "Find" ] || [ "$action" = "Locate" ] || [ "$action" = "Tree" ] || [ "$action" = "Memory" ]; then
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
        printf " %-55s " "$tags"
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
            echo "$target"
        fi
    done

    echo ""
    echo -e "${DIM}─────────────────────────────────────────────────────────────────────────────────────────────${NC}"
    echo ""
    echo -e "${DIM}${limit} of ${total} operations.  Use: watch -n 2 aoa intent${NC}"
}

cmd_intent_tags() {
    local project_id=$(get_project_id)
    local result=$(curl -s "${INDEX_URL}/intent/tags?project_id=${project_id}")

    echo -e "${BOLD}Intent Tags${NC}"
    echo ""

    echo "$result" | jq -r '.tags[] | "  \(.tag) (\(.count) files)"' 2>/dev/null || echo "  (no tags yet)"
}

cmd_intent_files() {
    local tag="$1"
    local project_id=$(get_project_id)

    if [ -z "$tag" ]; then
        echo "Usage: aoa intent files <tag>"
        echo "Example: aoa intent files authentication"
        return 1
    fi

    local result=$(curl -s "${INDEX_URL}/intent/files?tag=${tag}&project_id=${project_id}")

    local actual_tag=$(echo "$result" | jq -r '.tag')
    echo -e "${BOLD}Files for ${actual_tag}${NC}"
    echo ""

    echo "$result" | jq -r '.files[]' 2>/dev/null || echo "  (no files)"
}

cmd_intent_file() {
    local path="$1"
    local project_id=$(get_project_id)

    if [ -z "$path" ]; then
        echo "Usage: aoa intent file <path>"
        return 1
    fi

    local result=$(curl -s "${INDEX_URL}/intent/file?path=${path}&project_id=${project_id}")

    echo -e "${BOLD}Tags for ${path}${NC}"
    echo ""

    echo "$result" | jq -r '.tags[]' 2>/dev/null || echo "  (no tags)"
}

cmd_intent_stats() {
    local project_id=$(get_project_id)

    # Get metrics
    local metrics=$(curl -s "${INDEX_URL}/metrics?project_id=${project_id}")
    local hit_pct=$(echo "$metrics" | jq -r '.rolling.hit_at_5_pct // 0')
    local evaluated=$(echo "$metrics" | jq -r '.rolling.evaluated // 0')
    local hits=$(echo "$metrics" | jq -r '.rolling.hits // 0')
    local tokens_saved=$(echo "$metrics" | jq -r '.savings.tokens // 0')

    # Get intent stats
    local stats=$(curl -s "${INDEX_URL}/intent/stats?project_id=${project_id}")
    local total=$(echo "$stats" | jq -r '.total_records // 0')
    local unique_tags=$(echo "$stats" | jq -r '.unique_tags // 0')
    local unique_files=$(echo "$stats" | jq -r '.unique_files // 0')
    local first_seen=$(echo "$stats" | jq -r '.first_seen // 0')

    local hit_pct_int=$(printf "%.0f" "$hit_pct")
    local min_intents=30

    # Format first_seen as date
    local since_date=""
    if [ "$first_seen" -gt 0 ] 2>/dev/null; then
        since_date=$(date -d "@${first_seen}" "+%b %d" 2>/dev/null || date -r "${first_seen}" "+%b %d" 2>/dev/null)
    fi

    # Header
    echo -e "${CYAN}${BOLD}aOa Session Statistics${NC}                                      ${total} operations"
    echo ""
    echo -e "${BOLD}WHAT WE TRACK (REAL DATA)${NC}"
    # Show prediction capability level (matches status line logic)
    if [ "$total" -lt "$min_intents" ] 2>/dev/null; then
        echo -e "  Predictions:      ${YELLOW}Learning${NC} (${total}/${min_intents} toward full prediction)"
    elif [ "$hit_pct_int" -ge 80 ] 2>/dev/null; then
        echo -e "  Predictions:      ${GREEN}${hit_pct_int}% accuracy${NC} (${evaluated} evaluated)"
    else
        echo -e "  Predictions:      ${YELLOW}${hit_pct_int}% accuracy${NC} (${evaluated} evaluated, improving)"
    fi
    echo -e "  Operations:       ${total}"
    echo -e "  Unique files:     ${unique_files}"
    echo -e "  Unique tags:      ${unique_tags}"
    echo ""
    echo -e "${BOLD}TOKEN SAVINGS${NC}"
    if [ "$tokens_saved" -gt 0 ] 2>/dev/null; then
        local tokens_k=$(awk "BEGIN {printf \"%.1f\", $tokens_saved/1000}")
        if [ -n "$since_date" ]; then
            echo -e "  Measured:         ${GREEN}↓${tokens_k}k tokens${NC} ${DIM}(since ${since_date})${NC}"
        else
            echo -e "  Measured:         ${GREEN}↓${tokens_k}k tokens${NC}"
        fi
    else
        echo -e "  ${DIM}Not yet measured - requires capturing actual output tokens${NC}"
        echo -e "  ${DIM}Baseline (file sizes) is captured; actual output capture coming soon${NC}"
    fi
    echo ""
    echo -e "${DIM}─────────────────────────────────────────────────────────────────────────────────────────────${NC}"
    echo ""
    echo -e "${DIM}Run 'aoa intent' to see recent activity with per-operation details${NC}"
}

cmd_intent_store() {
    # Store AI-generated intent tags
    # Usage: aoa intent store "#tag1 #tag2 #tag3" [file1] [file2] ...
    local tags_str="$1"
    shift || true

    if [ -z "$tags_str" ]; then
        echo -e "${RED}Usage: aoa intent store \"#tag1 #tag2\" [file1] [file2]${NC}"
        echo -e "${DIM}Example: aoa intent store \"#auth #validation\" src/auth.py${NC}"
        return 1
    fi

    local project_id=$(get_project_id)
    local session_id="${AOA_SESSION_ID:-$(date +%Y%m%d)}"

    # Parse tags (space or comma separated)
    local tags_json=$(echo "$tags_str" | tr ',' ' ' | xargs -n1 | sed 's/^#*/#/' | jq -R . | jq -s .)

    # Collect files (remaining args, or use current context files)
    local files_json="[]"
    if [ $# -gt 0 ]; then
        files_json=$(printf '%s\n' "$@" | jq -R . | jq -s .)
    fi

    # Build and send request
    local payload=$(jq -n \
        --arg sid "$session_id" \
        --arg pid "$project_id" \
        --argjson tags "$tags_json" \
        --argjson files "$files_json" \
        '{session_id: $sid, project_id: $pid, tool: "Intent", tags: $tags, files: $files}')

    local result=$(curl -s -X POST "${INDEX_URL}/intent" \
        -H "Content-Type: application/json" \
        -d "$payload")

    if echo "$result" | jq -e '.success' > /dev/null 2>&1; then
        local tag_count=$(echo "$tags_json" | jq 'length')
        echo -e "${GREEN}✓${NC} Stored ${tag_count} tags"
    else
        echo -e "${RED}✗${NC} Failed to store tags"
        return 1
    fi
}

cmd_rate() {
    # Show token savings with estimated time savings
    # Uses conservative LLM processing rates based on documented performance

    echo -e "${CYAN}${BOLD}Time Savings Estimation${NC}"
    echo ""

    # Get current token savings from intent stats
    local project_id=$(get_project_id)
    local metrics=$(curl -s "${INDEX_URL}/metrics?project_id=${project_id}")
    local tokens_saved=$(echo "$metrics" | jq -r '.savings.tokens // 0')
    local tokens_k=$(awk "BEGIN {printf \"%.1f\", $tokens_saved/1000}")

    echo -e "${BOLD}YOUR TOKEN SAVINGS${NC}"
    if [ "$tokens_saved" -gt 0 ] 2>/dev/null; then
        echo -e "  Measured:         ${GREEN}↓${tokens_k}k tokens${NC}"
    else
        echo -e "  ${DIM}(no measured savings yet)${NC}"
    fi
    echo ""

    echo -e "${BOLD}TIME SAVINGS MODEL${NC}"
    echo -e "  ${DIM}LLMs process tokens at documented rates:${NC}"
    echo -e "    Input tokens:   ~100-500 tokens/second"
    echo -e "    Output tokens:  ~20-100 tokens/second"
    echo ""
    echo -e "  ${BOLD}Conservative estimate:${NC} 5-10ms per token (combined)"
    echo ""

    echo -e "${DIM}─────────────────────────────────────────────────────────────────────────────────────────────${NC}"
    echo ""

    if [ "$tokens_saved" -gt 0 ] 2>/dev/null; then
        # Calculate time savings range
        local time_low=$(awk "BEGIN {printf \"%.1f\", $tokens_saved * 5 / 1000}")   # 5ms/token
        local time_high=$(awk "BEGIN {printf \"%.1f\", $tokens_saved * 10 / 1000}") # 10ms/token

        echo -e "${BOLD}ESTIMATED TIME SAVINGS${NC}"
        echo -e "  Low estimate:     ${CYAN}~${time_low}s${NC} (at 5ms/token)"
        echo -e "  High estimate:    ${CYAN}~${time_high}s${NC} (at 10ms/token)"
        echo ""

        # Additional context for large savings
        if [ "$tokens_saved" -gt 100000 ]; then
            local mins_low=$(awk "BEGIN {printf \"%.1f\", $time_low / 60}")
            local mins_high=$(awk "BEGIN {printf \"%.1f\", $time_high / 60}")
            echo -e "  That's ${GREEN}${mins_low}-${mins_high} minutes${NC} of LLM processing avoided"
            echo ""
        fi

        echo -e "${DIM}Plus search speed: aOa search (~5ms) vs grep (~2-3 seconds)${NC}"
    else
        echo -e "${BOLD}EXAMPLE${NC}"
        echo -e "  If you save 22k tokens:"
        echo -e "    Low:  22k × 5ms  = ${CYAN}~110s${NC}"
        echo -e "    High: 22k × 10ms = ${CYAN}~220s${NC}"
    fi
    echo ""
    echo -e "${DIM}Note: Estimates based on typical Claude API processing speeds${NC}"
}

