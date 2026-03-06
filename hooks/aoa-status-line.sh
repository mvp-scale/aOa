#!/bin/bash
# =============================================================================
# aOa Status Line - Two-Line Display (configurable segments)
# =============================================================================
#
# Line 1: user:directory (branch) +add/-del cc_version
# Line 2: ⚡ aOa 🟢 48 │ ↓93k ⚡5m-52m saved │ ctx:60k/200k (30%) │ Opus 4.6
#
# Segments configured via .aoa/status-line.conf (one per line).
# If no conf exists, uses built-in defaults.
#
# Data sources:
#   stdin                  = Claude Code JSON (context_window, model, cwd, cost)
#   .aoa/status.json       = daemon-written metrics
#   .aoa/status-line.conf  = user segment config (optional)
#   .aoa/run/http.port     = dashboard port (for dashboard segment)
#
# =============================================================================

set -uo pipefail

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(pwd)}"
STATUS_FILE="$PROJECT_DIR/.aoa/status.json"
CONF_FILE="$PROJECT_DIR/.aoa/status-line.conf"

# ANSI colors
CYAN='\033[96m'
GREEN='\033[92m'
YELLOW='\033[93m'
RED='\033[91m'
GRAY='\033[90m'
BLUE='\033[94m'
PURPLE='\033[95m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'
MAGENTA='\033[95m'

SEP="${DIM}│${RESET}"

# === READ INPUT FROM CLAUDE CODE ===
input=$(cat)

# === PARSE CLAUDE CODE DATA ===
CURRENT_USAGE=$(echo "$input" | jq '.context_window.current_usage' 2>/dev/null)
CONTEXT_SIZE=$(echo "$input" | jq -r '.context_window.context_window_size // 200000' 2>/dev/null)
MODEL=$(echo "$input" | jq -r '.model.display_name // "Unknown"' 2>/dev/null)
CWD=$(echo "$input" | jq -r '.cwd // ""' 2>/dev/null)
LINES_ADD=$(echo "$input" | jq -r '.cost.total_lines_added // 0' 2>/dev/null)
LINES_REM=$(echo "$input" | jq -r '.cost.total_lines_removed // 0' 2>/dev/null)
COST_USD=$(echo "$input" | jq -r '.cost.total_cost_usd // 0' 2>/dev/null)
LINES_ADD=${LINES_ADD:-0}
LINES_REM=${LINES_REM:-0}
COST_USD=${COST_USD:-0}

# === READ DAEMON STATUS ===
DAEMON_ONLINE=false
INPUTS=0; TOKENS_SAVED=0; TIME_SAVED_MS=0; BURN_RATE=0
GUIDED_RATIO=0; READ_COUNT=0; GUIDED_READ_COUNT=0
RUNWAY_MIN=0; DELTA_MIN=0; CACHE_HIT=0; SHADOW_SAVED=0
DOMAINS=0; AUTOTUNE_PROGRESS=0
MASTERED=0; OBSERVED=0; VOCABULARY=0; CONCEPTS=0; PATTERNS=0; EVIDENCE=0
INPUT_TOKENS=0; OUTPUT_TOKENS=0; FLOW=0

if [ -f "$STATUS_FILE" ]; then
    DAEMON_ONLINE=true
    # Live
    INPUTS=$(jq -r '.intents // 0' "$STATUS_FILE" 2>/dev/null)
    TOKENS_SAVED=$(jq -r '.tokens_saved // 0' "$STATUS_FILE" 2>/dev/null)
    TIME_SAVED_MS=$(jq -r '.time_saved_ms // 0' "$STATUS_FILE" 2>/dev/null)
    BURN_RATE=$(jq -r '.burn_rate_per_min // 0' "$STATUS_FILE" 2>/dev/null)
    GUIDED_RATIO=$(jq -r '.guided_ratio // 0' "$STATUS_FILE" 2>/dev/null)
    READ_COUNT=$(jq -r '.read_count // 0' "$STATUS_FILE" 2>/dev/null)
    GUIDED_READ_COUNT=$(jq -r '.guided_read_count // 0' "$STATUS_FILE" 2>/dev/null)
    SHADOW_SAVED=$(jq -r '.shadow_saved // 0' "$STATUS_FILE" 2>/dev/null)
    CACHE_HIT=$(jq -r '.cache_hit_rate // 0' "$STATUS_FILE" 2>/dev/null)
    AUTOTUNE_PROGRESS=$(jq -r '.autotune_progress // 0' "$STATUS_FILE" 2>/dev/null)
    # Runway
    RUNWAY_MIN=$(jq -r '.runway_minutes // 0' "$STATUS_FILE" 2>/dev/null)
    DELTA_MIN=$(jq -r '.delta_minutes // 0' "$STATUS_FILE" 2>/dev/null)
    # Intel
    DOMAINS=$(jq -r '.domains // 0' "$STATUS_FILE" 2>/dev/null)
    MASTERED=$(jq -r '.mastered // 0' "$STATUS_FILE" 2>/dev/null)
    OBSERVED=$(jq -r '.observed // 0' "$STATUS_FILE" 2>/dev/null)
    VOCABULARY=$(jq -r '.vocabulary // 0' "$STATUS_FILE" 2>/dev/null)
    CONCEPTS=$(jq -r '.concepts // 0' "$STATUS_FILE" 2>/dev/null)
    PATTERNS=$(jq -r '.patterns // 0' "$STATUS_FILE" 2>/dev/null)
    EVIDENCE=$(jq -r '.evidence // 0' "$STATUS_FILE" 2>/dev/null)
    # Intel (derived)
    LEARNING_SPEED=$(jq -r '.learning_speed // 0' "$STATUS_FILE" 2>/dev/null)
    SIGNAL_CLARITY=$(jq -r '.signal_clarity // 0' "$STATUS_FILE" 2>/dev/null)
    CONVERSION=$(jq -r '.conversion // 0' "$STATUS_FILE" 2>/dev/null)
    # Debrief
    INPUT_TOKENS=$(jq -r '.input_tokens // 0' "$STATUS_FILE" 2>/dev/null)
    OUTPUT_TOKENS=$(jq -r '.output_tokens // 0' "$STATUS_FILE" 2>/dev/null)
    FLOW=$(jq -r '.flow // 0' "$STATUS_FILE" 2>/dev/null)
    PACE=$(jq -r '.pace // 0' "$STATUS_FILE" 2>/dev/null)
    TURN_TIME_MS=$(jq -r '.turn_time_ms // 0' "$STATUS_FILE" 2>/dev/null)
    LEVERAGE=$(jq -r '.leverage // 0' "$STATUS_FILE" 2>/dev/null)
    AMPLIFICATION=$(jq -r '.amplification // 0' "$STATUS_FILE" 2>/dev/null)
    COST_PER_EXCHANGE=$(jq -r '.cost_per_exchange // 0' "$STATUS_FILE" 2>/dev/null)
    CACHE_SAVED_USD=$(jq -r '.cache_saved_usd // 0' "$STATUS_FILE" 2>/dev/null)
    COST_SAVED_USD=$(jq -r '.cost_saved_usd // 0' "$STATUS_FILE" 2>/dev/null)
    TURN_COUNT=$(jq -r '.turn_count // 0' "$STATUS_FILE" 2>/dev/null)
    # L17.5: Lifetime totals
    LIFETIME_TOKENS_SAVED=$(jq -r '.lifetime_tokens_saved // 0' "$STATUS_FILE" 2>/dev/null)
    LIFETIME_TIME_SAVED_MS=$(jq -r '.lifetime_time_saved_ms // 0' "$STATUS_FILE" 2>/dev/null)
    LIFETIME_SESSIONS=$(jq -r '.lifetime_sessions // 0' "$STATUS_FILE" 2>/dev/null)
    # Defaults
    INPUTS=${INPUTS:-0}; TOKENS_SAVED=${TOKENS_SAVED:-0}; TIME_SAVED_MS=${TIME_SAVED_MS:-0}
fi

# === LINE 1: Environment Context ===
USERNAME="${USER:-$(whoami)}"

GIT_BRANCH=""
GIT_CHANGES=""
if [ -n "$CWD" ] && git -C "$CWD" rev-parse --git-dir >/dev/null 2>&1; then
    GIT_BRANCH=$(git -C "$CWD" symbolic-ref --short HEAD 2>/dev/null || git -C "$CWD" rev-parse --short HEAD 2>/dev/null)
    GIT_STAT=$(git -C "$CWD" diff --shortstat HEAD 2>/dev/null)
    if [ -n "$GIT_STAT" ]; then
        INSERTIONS=$(echo "$GIT_STAT" | grep -oE '[0-9]+ insertion' | grep -oE '[0-9]+' || echo "0")
        DELETIONS=$(echo "$GIT_STAT" | grep -oE '[0-9]+ deletion' | grep -oE '[0-9]+' || echo "0")
        [ -z "$INSERTIONS" ] && INSERTIONS=0
        [ -z "$DELETIONS" ] && DELETIONS=0
        if [ "$INSERTIONS" -gt 0 ] || [ "$DELETIONS" -gt 0 ]; then
            GIT_CHANGES="${GREEN}+${INSERTIONS}${RESET}/${RED}-${DELETIONS}${RESET}"
        fi
    fi
fi

CC_VERSION=$(ls -t "${HOME}/.local/share/claude/versions/" 2>/dev/null | head -1)
CC_VER_DISPLAY=""
if [ -n "$CC_VERSION" ]; then
    CC_VER_DISPLAY="${DIM}cc${RESET}${CYAN}${CC_VERSION}${RESET}"
fi

LINE1="${MAGENTA}${USERNAME}${RESET}:${CYAN}${CWD}${RESET}"
[ -n "$GIT_BRANCH" ] && LINE1="${LINE1} ${DIM}(${RESET}${YELLOW}${GIT_BRANCH}${RESET}${DIM})${RESET}"
[ -n "$GIT_CHANGES" ] && LINE1="${LINE1} ${GIT_CHANGES}"
[ -n "$CC_VER_DISPLAY" ] && LINE1="${LINE1} ${CC_VER_DISPLAY}"
echo -e "${LINE1}"

# === FORMATTING HELPERS ===

format_tokens() {
    local n=$1
    if [ "$n" -ge 1000000 ]; then awk "BEGIN {printf \"%.1fM\", $n/1000000}"
    elif [ "$n" -ge 1000 ]; then  awk "BEGIN {printf \"%.1fk\", $n/1000}"
    else echo "$n"; fi
}

format_tokens_fixed() {
    local n=$1
    if [ "$n" -ge 1000000 ]; then awk "BEGIN {printf \"%.0fM\", $n/1000000}"
    elif [ "$n" -ge 1000 ]; then  awk "BEGIN {printf \"%.0fk\", $n/1000}"
    else echo "$n"; fi
}

format_time() {
    local ms=$1
    local secs=$((ms / 1000))
    if [ "$secs" -lt 60 ]; then echo "${secs}s"
    elif [ "$secs" -lt 3600 ]; then
        local m=$(( (secs + 30) / 60 ))
        echo "${m}m"
    else
        local total_m=$(( (secs + 30) / 60 ))
        local h=$((total_m / 60)); local m=$((total_m % 60))
        if [ "$m" -gt 0 ]; then echo "${h}h${m}m"; else echo "${h}h"; fi
    fi
}

format_minutes() {
    local min=$(awk "BEGIN {printf \"%d\", $1}")
    if [ "$min" -lt 60 ]; then echo "${min}m"
    else
        local h=$((min / 60)); local m=$((min % 60))
        if [ "$m" -gt 0 ]; then echo "${h}h${m}m"; else echo "${h}h"; fi
    fi
}

gt0() { [ "$(awk "BEGIN {print ($1 > 0)}")" = "1" ]; }
format_pct() { awk "BEGIN {printf \"%.0f%%\", $1 * 100}"; }
format_dollars() { awk "BEGIN {printf \"\$%.2f\", $1}"; }
format_float1() { awk "BEGIN {printf \"%.1f\", $1}"; }

# === SEGMENT RENDERERS ===
# Each render_* outputs its text. Empty output = segment hidden.

# ── Live ──

render_input() {
    local light intent_display
    if [ "$INPUTS" -lt 30 ] 2>/dev/null; then
        light="${GRAY}⚪${RESET}"; intent_display="learning"
    elif [ "$INPUTS" -lt 100 ] 2>/dev/null; then
        light="${YELLOW}🟡${RESET}"; intent_display="${INPUTS}"
    else
        light="${GREEN}🟢${RESET}"; intent_display="${INPUTS}"
    fi
    [ "$INPUTS" -ge 1000 ] 2>/dev/null && intent_display=$(format_tokens $INPUTS)
    echo "${light} ${intent_display}"
}

# ── Live (dashboard: green/cyan/red/purple/green) ──

render_tokens_saved() {
    [ "$TOKENS_SAVED" -gt 0 ] 2>/dev/null || return
    echo "${GREEN}↓$(format_tokens $TOKENS_SAVED)${RESET}"
}

render_time_saved_range() {
    [ "$TIME_SAVED_MS" -gt 0 ] 2>/dev/null || return
    echo "${CYAN}$(format_time $TIME_SAVED_MS)${RESET}${DIM} saved${RESET}"
}

render_lifetime_saved() {
    [ "$LIFETIME_TOKENS_SAVED" -gt 0 ] 2>/dev/null || return
    echo "${GREEN}↓$(format_tokens $LIFETIME_TOKENS_SAVED)${RESET}${DIM} lifetime${RESET}"
}

render_lifetime_time_saved() {
    [ "$LIFETIME_TIME_SAVED_MS" -gt 0 ] 2>/dev/null || return
    echo "${CYAN}$(format_time $LIFETIME_TIME_SAVED_MS)${RESET}${DIM} lifetime${RESET}"
}

render_lifetime_sessions() {
    [ "$LIFETIME_SESSIONS" -gt 0 ] 2>/dev/null || return
    echo "${CYAN}${LIFETIME_SESSIONS}${RESET}${DIM} sessions${RESET}"
}

render_cost_saved() {
    gt0 "$COST_SAVED_USD" || return
    echo "${GREEN}$(format_dollars $COST_SAVED_USD)${RESET}${DIM} saved${RESET}"
}

render_burn_rate() {
    gt0 "$BURN_RATE" || return
    echo "🔥${RED}$(format_tokens $(awk "BEGIN {printf \"%d\", $BURN_RATE}"))${RESET}${DIM}/min${RESET}"
}

render_cost() {
    gt0 "$COST_USD" || return
    echo "${PURPLE}$(format_dollars $COST_USD)${RESET}"
}

render_guided_ratio() {
    [ "$READ_COUNT" -gt 0 ] 2>/dev/null || return
    echo "${GREEN}$(format_pct $GUIDED_RATIO)${RESET}${DIM} guided${RESET}"
}

render_shadow_saved() {
    [ "$SHADOW_SAVED" -gt 0 ] 2>/dev/null || return
    echo "${GREEN}↓$(format_tokens $SHADOW_SAVED)${RESET}${DIM} shadow${RESET}"
}

render_cache_hit_rate() {
    gt0 "$CACHE_HIT" || return
    echo "${PURPLE}$(format_pct $CACHE_HIT)${RESET}${DIM} cache${RESET}"
}

render_cache_saved() {
    gt0 "$CACHE_SAVED_USD" || return
    echo "${PURPLE}$(format_dollars $CACHE_SAVED_USD)${RESET}${DIM} cache${RESET}"
}

render_read_count() {
    [ "$READ_COUNT" -gt 0 ] 2>/dev/null || return
    echo "${GREEN}${GUIDED_READ_COUNT}/${READ_COUNT}${RESET}${DIM} reads${RESET}"
}

render_autotune() {
    echo "${YELLOW}${AUTOTUNE_PROGRESS}/50${RESET}"
}

render_lines_changed() {
    local total=$((LINES_ADD + LINES_REM))
    [ "$total" -gt 0 ] 2>/dev/null || return
    echo "${GREEN}+${LINES_ADD}${RESET}${DIM}/${RESET}${RED}-${LINES_REM}${RESET}${DIM}L${RESET}"
}

# ── Runway ──

render_runway() {
    gt0 "$RUNWAY_MIN" || return
    echo "${CYAN}$(format_minutes $RUNWAY_MIN)${RESET}${DIM} runway${RESET}"
}

render_delta_minutes() {
    gt0 "$DELTA_MIN" || return
    echo "${GREEN}+$(format_minutes $DELTA_MIN)${RESET}"
}

# ── Intel (dashboard: purple/blue/cyan/green/purple/yellow/red) ──

render_domains() {
    [ "$DOMAINS" -gt 0 ] 2>/dev/null || return
    echo "${PURPLE}${DOMAINS}${RESET}${DIM} domains${RESET}"
}

render_mastered() {
    [ "$MASTERED" -gt 0 ] 2>/dev/null || return
    echo "${PURPLE}${MASTERED}${RESET}${DIM} mastered${RESET}"
}

render_observed() {
    [ "$OBSERVED" -gt 0 ] 2>/dev/null || return
    echo "${BLUE}${OBSERVED}${RESET}${DIM} observed${RESET}"
}

render_vocabulary() {
    [ "$VOCABULARY" -gt 0 ] 2>/dev/null || return
    echo "${CYAN}$(format_tokens $VOCABULARY)${RESET}${DIM} keywords${RESET}"
}

render_concepts() {
    [ "$CONCEPTS" -gt 0 ] 2>/dev/null || return
    echo "${GREEN}${CONCEPTS}${RESET}${DIM} concepts${RESET}"
}

render_patterns() {
    [ "$PATTERNS" -gt 0 ] 2>/dev/null || return
    echo "${YELLOW}$(format_tokens $PATTERNS)${RESET}${DIM} patterns${RESET}"
}

render_evidence() {
    gt0 "$EVIDENCE" || return
    echo "${RED}$(format_float1 $EVIDENCE)${RESET}${DIM} evidence${RESET}"
}

render_learning_speed() {
    gt0 "$LEARNING_SPEED" || return
    echo "${GREEN}$(format_float1 $LEARNING_SPEED)${RESET}${DIM} d/prompt${RESET}"
}

render_signal_clarity() {
    gt0 "$SIGNAL_CLARITY" || return
    echo "${CYAN}$(format_pct $SIGNAL_CLARITY)${RESET}${DIM} signal${RESET}"
}

render_conversion() {
    gt0 "$CONVERSION" || return
    echo "${YELLOW}$(format_pct $CONVERSION)${RESET}${DIM} conv${RESET}"
}

# ── Debrief (dashboard: cyan/green/blue/green/cyan/yellow/purple/purple) ──

render_input_tokens() {
    [ "$INPUT_TOKENS" -gt 0 ] 2>/dev/null || return
    echo "${CYAN}$(format_tokens $INPUT_TOKENS)${RESET}${DIM} in${RESET}"
}

render_output_tokens() {
    [ "$OUTPUT_TOKENS" -gt 0 ] 2>/dev/null || return
    echo "${GREEN}$(format_tokens $OUTPUT_TOKENS)${RESET}${DIM} out${RESET}"
}

render_flow() {
    gt0 "$FLOW" || return
    echo "${BLUE}$(format_float1 $FLOW)${RESET}${DIM} tok/s${RESET}"
}

render_pace() {
    gt0 "$PACE" || return
    echo "${GREEN}$(format_float1 $PACE)${RESET}${DIM}/s${RESET}"
}

render_turn_time() {
    [ "$TURN_TIME_MS" -gt 0 ] 2>/dev/null || return
    echo "${CYAN}$(format_time $TURN_TIME_MS)${RESET}${DIM}/turn${RESET}"
}

render_leverage() {
    gt0 "$LEVERAGE" || return
    echo "${YELLOW}$(format_float1 $LEVERAGE)${RESET}${DIM} tools/turn${RESET}"
}

render_amplification() {
    gt0 "$AMPLIFICATION" || return
    echo "${PURPLE}$(format_float1 $AMPLIFICATION)x${RESET}"
}

render_cost_per_exchange() {
    gt0 "$COST_PER_EXCHANGE" || return
    echo "${PURPLE}$(format_dollars $COST_PER_EXCHANGE)${RESET}${DIM}/turn${RESET}"
}

render_turn_count() {
    [ "$TURN_COUNT" -gt 0 ] 2>/dev/null || return
    echo "${CYAN}${TURN_COUNT}${RESET}${DIM} turns${RESET}"
}

# ── Right side ──

render_context() {
    local total_tokens=0
    if [ "$CURRENT_USAGE" != "null" ] && [ -n "$CURRENT_USAGE" ]; then
        local input_tok=$(echo "$CURRENT_USAGE" | jq -r '.input_tokens // 0')
        local cache_create=$(echo "$CURRENT_USAGE" | jq -r '.cache_creation_input_tokens // 0')
        local cache_read=$(echo "$CURRENT_USAGE" | jq -r '.cache_read_input_tokens // 0')
        total_tokens=$((input_tok + cache_create + cache_read))
    fi
    local ctx_size=${CONTEXT_SIZE:-200000}
    [ "$ctx_size" -eq 0 ] 2>/dev/null && ctx_size=200000
    local pct=0
    [ "$ctx_size" -gt 0 ] && pct=$((total_tokens * 100 / ctx_size))
    local ctx_color=$GREEN
    if [ "$pct" -gt 84 ]; then ctx_color=$RED
    elif [ "$pct" -gt 70 ]; then ctx_color=$YELLOW; fi
    echo "ctx:${ctx_color}$(format_tokens $total_tokens)/$(format_tokens_fixed $ctx_size)${RESET} ${DIM}(${pct}%)${RESET}"
}

render_model() {
    echo "${MODEL}"
}

render_dashboard() {
    # Kept for backward compat if used as a standalone segment — now a no-op.
    # Dashboard link is rendered as part of the ⚡ aOa header instead.
    return
}

# aoa_header outputs "⚡ aOa <traffic_light>" — the aOa name is wrapped
# in an OSC 8 hyperlink to the dashboard when running. The traffic light
# always follows, showing learning maturity based on input count.
aoa_header() {
    local label="${CYAN}${BOLD}⚡ aOa${RESET}"
    local port_file="$PROJECT_DIR/.aoa/run/http.port"
    if [ -f "$port_file" ]; then
        local port=$(cat "$port_file" 2>/dev/null)
        if [ -n "$port" ]; then
            label="\033]8;;http://localhost:${port}\a${label}\033]8;;\a"
        fi
    fi

    # Traffic light: learning maturity based on input count
    local light input_display
    if [ "$INPUTS" -lt 30 ] 2>/dev/null; then
        light="${GRAY}⚪${RESET}"; input_display="learning"
    elif [ "$INPUTS" -lt 100 ] 2>/dev/null; then
        light="${YELLOW}🟡${RESET}"; input_display="${INPUTS}"
    else
        light="${GREEN}🟢${RESET}"; input_display="${INPUTS}"
    fi
    [ "$INPUTS" -ge 1000 ] 2>/dev/null && input_display=$(format_tokens $INPUTS)

    echo "${label} ${light} ${input_display}"
}

# === READ SEGMENT CONFIG ===
SEGMENTS=()
if [ -f "$CONF_FILE" ]; then
    while IFS= read -r line; do
        line="${line%%#*}"
        line=$(echo "$line" | tr -d '[:space:]')
        [ -n "$line" ] && SEGMENTS+=("$line")
    done < "$CONF_FILE"
fi
[ ${#SEGMENTS[@]} -eq 0 ] && SEGMENTS=(tokens_saved time_saved_range context model)

# === WRITE CONTEXT SNAPSHOT TO .aoa/context.jsonl ===
if [ -d "$PROJECT_DIR/.aoa" ]; then
    CONTEXT_JSONL="$PROJECT_DIR/.aoa/hook/context.jsonl"
    mkdir -p "$PROJECT_DIR/.aoa/hook"
    COST_DUR=$(echo "$input" | jq -r '.cost.total_duration_ms // 0' 2>/dev/null)
    COST_API_DUR=$(echo "$input" | jq -r '.cost.total_api_duration_ms // 0' 2>/dev/null)
    USED_PCT=$(echo "$input" | jq -r '.context_window.used_percentage // 0' 2>/dev/null)
    REMAIN_PCT=$(echo "$input" | jq -r '.context_window.remaining_percentage // 0' 2>/dev/null)
    CC_SESSION=$(echo "$input" | jq -r '.session_id // ""' 2>/dev/null)
    CC_VER=$(echo "$input" | jq -r '.version // ""' 2>/dev/null)
    CC_MODEL_ID=$(echo "$input" | jq -r '.model.id // ""' 2>/dev/null)
    TS=$(date +%s)
    local_total=0
    if [ "$CURRENT_USAGE" != "null" ] && [ -n "$CURRENT_USAGE" ]; then
        local_input=$(echo "$CURRENT_USAGE" | jq -r '.input_tokens // 0')
        local_cc=$(echo "$CURRENT_USAGE" | jq -r '.cache_creation_input_tokens // 0')
        local_cr=$(echo "$CURRENT_USAGE" | jq -r '.cache_read_input_tokens // 0')
        local_total=$((local_input + local_cc + local_cr))
    fi
    local_ctx_size=${CONTEXT_SIZE:-200000}
    printf '{"ts":%s,"ctx_used":%s,"ctx_max":%s,"used_pct":%s,"remaining_pct":%s,"total_cost_usd":%s,"total_duration_ms":%s,"total_api_duration_ms":%s,"total_lines_added":%s,"total_lines_removed":%s,"model":"%s","session_id":"%s","version":"%s"}\n' \
        "$TS" "$local_total" "$local_ctx_size" "$USED_PCT" "$REMAIN_PCT" \
        "$COST_USD" "$COST_DUR" "$COST_API_DUR" "$LINES_ADD" "$LINES_REM" \
        "$CC_MODEL_ID" "$CC_SESSION" "$CC_VER" >> "$CONTEXT_JSONL"
    if [ "$(wc -l < "$CONTEXT_JSONL" 2>/dev/null)" -gt 20 ] 2>/dev/null; then
        tail -5 "$CONTEXT_JSONL" > "${CONTEXT_JSONL}.tmp" && mv "${CONTEXT_JSONL}.tmp" "$CONTEXT_JSONL"
    fi
fi

# === BUILD LINE 2 ===
if [ "$DAEMON_ONLINE" = "false" ]; then
    LINE2="$(aoa_header) ${DIM}offline${RESET}"
    for seg in "${SEGMENTS[@]}"; do
        case "$seg" in
            context) LINE2="${LINE2} ${SEP} $(render_context)" ;;
            model)   LINE2="${LINE2} ${SEP} $(render_model)" ;;
            cost)    result=$(render_cost); [ -n "$result" ] && LINE2="${LINE2} ${SEP} ${result}" ;;
            lines_changed) result=$(render_lines_changed); [ -n "$result" ] && LINE2="${LINE2} ${SEP} ${result}" ;;
            dashboard) result=$(render_dashboard); [ -n "$result" ] && LINE2="${LINE2} ${SEP} ${result}" ;;
        esac
    done
else
    LINE2="$(aoa_header)"
    for seg in "${SEGMENTS[@]}"; do
        result=""
        case "$seg" in
            # Live
            tokens_saved)     result=$(render_tokens_saved) ;;
            time_saved_range) result=$(render_time_saved_range) ;;
            burn_rate)        result=$(render_burn_rate) ;;
            cost)             result=$(render_cost) ;;
            guided_ratio)     result=$(render_guided_ratio) ;;
            shadow_saved)     result=$(render_shadow_saved) ;;
            cache_hit_rate)   result=$(render_cache_hit_rate) ;;
            read_count)       result=$(render_read_count) ;;
            autotune)         result=$(render_autotune) ;;
            lifetime_saved)      result=$(render_lifetime_saved) ;;
            lifetime_time_saved) result=$(render_lifetime_time_saved) ;;
            lifetime_sessions)   result=$(render_lifetime_sessions) ;;
            lines_changed)    result=$(render_lines_changed) ;;
            # Runway
            runway)           result=$(render_runway) ;;
            delta_minutes)    result=$(render_delta_minutes) ;;
            # Intel
            domains)          result=$(render_domains) ;;
            mastered)         result=$(render_mastered) ;;
            observed)         result=$(render_observed) ;;
            vocabulary)       result=$(render_vocabulary) ;;
            concepts)         result=$(render_concepts) ;;
            patterns)         result=$(render_patterns) ;;
            evidence)         result=$(render_evidence) ;;
            learning_speed)   result=$(render_learning_speed) ;;
            signal_clarity)   result=$(render_signal_clarity) ;;
            conversion)       result=$(render_conversion) ;;
            # Debrief
            input_tokens)     result=$(render_input_tokens) ;;
            output_tokens)    result=$(render_output_tokens) ;;
            flow)             result=$(render_flow) ;;
            pace)             result=$(render_pace) ;;
            turn_time)        result=$(render_turn_time) ;;
            leverage)         result=$(render_leverage) ;;
            amplification)    result=$(render_amplification) ;;
            cost_per_exchange) result=$(render_cost_per_exchange) ;;
            cache_saved)      result=$(render_cache_saved) ;;
            cost_saved)       result=$(render_cost_saved) ;;
            turn_count)       result=$(render_turn_count) ;;
            # Right side
            context)          result=$(render_context) ;;
            model)            result=$(render_model) ;;
            dashboard)        result=$(render_dashboard) ;;
        esac
        if [ -n "$result" ]; then
            LINE2="${LINE2} ${SEP} ${result}"
        fi
    done
fi

echo -e "${LINE2}"
