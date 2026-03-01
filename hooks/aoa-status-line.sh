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
#   .aoa/http.port         = dashboard port (for dashboard segment)
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
INTENTS=0; TOKENS_SAVED=0; TIME_SAVED_MS=0; BURN_RATE=0
GUIDED_RATIO=0; READ_COUNT=0; GUIDED_READ_COUNT=0
RUNWAY_MIN=0; DELTA_MIN=0; CACHE_HIT=0; SHADOW_SAVED=0
DOMAINS=0; AUTOTUNE_PROGRESS=0
MASTERED=0; OBSERVED=0; VOCABULARY=0; CONCEPTS=0; PATTERNS=0; EVIDENCE=0
INPUT_TOKENS=0; OUTPUT_TOKENS=0; FLOW=0

if [ -f "$STATUS_FILE" ]; then
    DAEMON_ONLINE=true
    # Live
    INTENTS=$(jq -r '.intents // 0' "$STATUS_FILE" 2>/dev/null)
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
    # Debrief
    INPUT_TOKENS=$(jq -r '.input_tokens // 0' "$STATUS_FILE" 2>/dev/null)
    OUTPUT_TOKENS=$(jq -r '.output_tokens // 0' "$STATUS_FILE" 2>/dev/null)
    FLOW=$(jq -r '.flow // 0' "$STATUS_FILE" 2>/dev/null)
    # Defaults
    INTENTS=${INTENTS:-0}; TOKENS_SAVED=${TOKENS_SAVED:-0}; TIME_SAVED_MS=${TIME_SAVED_MS:-0}
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
    if [ "$n" -ge 1000000 ]; then awk "BEGIN {printf \"%.2fM\", $n/1000000}"
    elif [ "$n" -ge 1000 ]; then  awk "BEGIN {printf \"%.2fk\", $n/1000}"
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
        local m=$((secs / 60)); local s=$((secs % 60))
        if [ "$s" -gt 0 ]; then echo "${m}m${s}s"; else echo "${m}m"; fi
    else
        local h=$((secs / 3600)); local m=$(( (secs % 3600) / 60 ))
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

render_intents() {
    local light intent_display
    if [ "$INTENTS" -lt 30 ] 2>/dev/null; then
        light="${GRAY}⚪${RESET}"; intent_display="learning"
    elif [ "$INTENTS" -lt 100 ] 2>/dev/null; then
        light="${YELLOW}🟡${RESET}"; intent_display="${INTENTS}"
    else
        light="${GREEN}🟢${RESET}"; intent_display="${INTENTS}"
    fi
    [ "$INTENTS" -ge 1000 ] 2>/dev/null && intent_display=$(format_tokens $INTENTS)
    echo "${light} ${intent_display}"
}

render_tokens_saved() {
    [ "$TOKENS_SAVED" -gt 0 ] 2>/dev/null || return
    echo "↓$(format_tokens $TOKENS_SAVED)"
}

render_time_saved_range() {
    [ "$TOKENS_SAVED" -gt 0 ] 2>/dev/null || return
    local low_fmt high_fmt
    low_fmt=$(format_time $TIME_SAVED_MS)
    high_fmt=$(format_time $((TOKENS_SAVED * 200)))
    [ "$TIME_SAVED_MS" -gt 0 ] 2>/dev/null && echo "${CYAN}⚡${low_fmt}-${high_fmt} saved${RESET}"
}

render_burn_rate() {
    gt0 "$BURN_RATE" || return
    echo "🔥$(format_tokens $(awk "BEGIN {printf \"%d\", $BURN_RATE}"))/min"
}

render_cost() {
    gt0 "$COST_USD" || return
    echo "$(format_dollars $COST_USD)"
}

render_guided_ratio() {
    [ "$READ_COUNT" -gt 0 ] 2>/dev/null || return
    echo "guided $(format_pct $GUIDED_RATIO)"
}

render_shadow_saved() {
    [ "$SHADOW_SAVED" -gt 0 ] 2>/dev/null || return
    echo "shadow ↓$(format_tokens $SHADOW_SAVED)"
}

render_cache_hit_rate() {
    gt0 "$CACHE_HIT" || return
    echo "cache $(format_pct $CACHE_HIT)"
}

render_read_count() {
    [ "$READ_COUNT" -gt 0 ] 2>/dev/null || return
    echo "${GUIDED_READ_COUNT}/${READ_COUNT} reads"
}

render_autotune() {
    echo "${AUTOTUNE_PROGRESS}/50"
}

render_lines_changed() {
    local total=$((LINES_ADD + LINES_REM))
    [ "$total" -gt 0 ] 2>/dev/null || return
    echo "${GREEN}+${LINES_ADD}${RESET}/${RED}-${LINES_REM}${RESET}L"
}

# ── Runway ──

render_runway() {
    gt0 "$RUNWAY_MIN" || return
    echo "runway $(format_minutes $RUNWAY_MIN)"
}

render_delta_minutes() {
    gt0 "$DELTA_MIN" || return
    echo "+$(format_minutes $DELTA_MIN)"
}

# ── Intel ──

render_domains() {
    [ "$DOMAINS" -gt 0 ] 2>/dev/null || return
    echo "${DOMAINS} domains"
}

render_mastered() {
    [ "$MASTERED" -gt 0 ] 2>/dev/null || return
    echo "${PURPLE}${MASTERED} mastered${RESET}"
}

render_observed() {
    [ "$OBSERVED" -gt 0 ] 2>/dev/null || return
    echo "${OBSERVED} observed"
}

render_vocabulary() {
    [ "$VOCABULARY" -gt 0 ] 2>/dev/null || return
    echo "$(format_tokens $VOCABULARY) keywords"
}

render_concepts() {
    [ "$CONCEPTS" -gt 0 ] 2>/dev/null || return
    echo "${CONCEPTS} concepts"
}

render_patterns() {
    [ "$PATTERNS" -gt 0 ] 2>/dev/null || return
    echo "$(format_tokens $PATTERNS) patterns"
}

render_evidence() {
    gt0 "$EVIDENCE" || return
    echo "$(format_float1 $EVIDENCE) evidence"
}

# ── Debrief ──

render_input_tokens() {
    [ "$INPUT_TOKENS" -gt 0 ] 2>/dev/null || return
    echo "in:$(format_tokens $INPUT_TOKENS)"
}

render_output_tokens() {
    [ "$OUTPUT_TOKENS" -gt 0 ] 2>/dev/null || return
    echo "out:$(format_tokens $OUTPUT_TOKENS)"
}

render_flow() {
    gt0 "$FLOW" || return
    echo "$(format_float1 $FLOW) tok/s"
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
    local port_file="$PROJECT_DIR/.aoa/http.port"
    [ -f "$port_file" ] || return
    local port=$(cat "$port_file" 2>/dev/null)
    [ -n "$port" ] || return
    local url="http://localhost:${port}"
    # OSC 8 terminal hyperlink
    echo "\033]8;;${url}\033\\${DIM}dashboard${RESET}\033]8;;\033\\"
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
[ ${#SEGMENTS[@]} -eq 0 ] && SEGMENTS=(intents tokens_saved time_saved_range context model)

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
    LINE2="${CYAN}${BOLD}⚡ aOa${RESET} ${DIM}offline${RESET}"
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
    LINE2="${CYAN}${BOLD}⚡ aOa${RESET}"
    first=true
    for seg in "${SEGMENTS[@]}"; do
        result=""
        case "$seg" in
            # Live
            intents)          result=$(render_intents) ;;
            tokens_saved)     result=$(render_tokens_saved) ;;
            time_saved_range) result=$(render_time_saved_range) ;;
            burn_rate)        result=$(render_burn_rate) ;;
            cost)             result=$(render_cost) ;;
            guided_ratio)     result=$(render_guided_ratio) ;;
            shadow_saved)     result=$(render_shadow_saved) ;;
            cache_hit_rate)   result=$(render_cache_hit_rate) ;;
            read_count)       result=$(render_read_count) ;;
            autotune)         result=$(render_autotune) ;;
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
            # Debrief
            input_tokens)     result=$(render_input_tokens) ;;
            output_tokens)    result=$(render_output_tokens) ;;
            flow)             result=$(render_flow) ;;
            # Right side
            context)          result=$(render_context) ;;
            model)            result=$(render_model) ;;
            dashboard)        result=$(render_dashboard) ;;
        esac
        if [ -n "$result" ]; then
            if [ "$first" = true ]; then
                LINE2="${LINE2} ${result}"
                first=false
            else
                LINE2="${LINE2} ${SEP} ${result}"
            fi
        fi
    done
fi

echo -e "${LINE2}"
