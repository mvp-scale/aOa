#!/bin/bash
# =============================================================================
# aOa Status Line - Two-Line Progressive Display
# =============================================================================
#
# Line 1: user:directory (branch) +add/-del cc_version
# Line 2: ⚡ aOa 🟢 100% │ intents │ savings │ context │ Model
#
# =============================================================================

set -uo pipefail

MIN_INTENTS=30

# Find AOA config from .aoa/home.json
HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$HOOK_DIR")")"
AOA_HOME_FILE="$PROJECT_ROOT/.aoa/home.json"

if [ -f "$AOA_HOME_FILE" ]; then
    AOA_DATA=$(jq -r '.data_dir' "$AOA_HOME_FILE" 2>/dev/null)
    AOA_URL="${AOA_URL:-$(jq -r '.aoa_url // "http://localhost:8080"' "$AOA_HOME_FILE" 2>/dev/null)}"
    AOA_PROJECT_ID=$(jq -r '.project_id // ""' "$AOA_HOME_FILE" 2>/dev/null)
else
    # Fallback defaults
    AOA_DATA="${AOA_DATA:-/tmp/aoa}"
    AOA_URL="${AOA_URL:-http://localhost:8080}"
    AOA_PROJECT_ID=""
fi

STATUS_FILE="${AOA_STATUS_FILE:-$AOA_DATA/status.json}"

# ANSI colors
CYAN='\033[96m'
GREEN='\033[92m'
YELLOW='\033[93m'
RED='\033[91m'
GRAY='\033[90m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'
MAGENTA='\033[95m'

# === READ INPUT FROM CLAUDE CODE ===
input=$(cat)

# === PARSE CONTEXT WINDOW ===
CURRENT_USAGE=$(echo "$input" | jq '.context_window.current_usage' 2>/dev/null)
CONTEXT_SIZE=$(echo "$input" | jq -r '.context_window.context_window_size // 200000' 2>/dev/null)
MODEL=$(echo "$input" | jq -r '.model.display_name // "Unknown"' 2>/dev/null)
CWD=$(echo "$input" | jq -r '.cwd // ""' 2>/dev/null)

# === LINE 1: Environment Context ===
USERNAME="${USER:-$(whoami)}"

# Get git info if in a git repo
GIT_BRANCH=""
GIT_CHANGES=""
if [ -n "$CWD" ] && [ -d "$CWD/.git" ] || git -C "$CWD" rev-parse --git-dir >/dev/null 2>&1; then
    GIT_BRANCH=$(git -C "$CWD" symbolic-ref --short HEAD 2>/dev/null || git -C "$CWD" rev-parse --short HEAD 2>/dev/null)

    # Get insertions/deletions from staged + unstaged changes
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

# Get Claude Code version from filesystem (instant - no process spawn)
CC_VERSION=$(ls -t "${HOME}/.local/share/claude/versions/" 2>/dev/null | head -1)
CC_VER_DISPLAY=""
if [ -n "$CC_VERSION" ]; then
    CC_VER_DISPLAY="${DIM}cc${RESET}${CYAN}${CC_VERSION}${RESET}"
fi

# Build Line 1
LINE1="${MAGENTA}${USERNAME}${RESET}:${CYAN}${CWD}${RESET}"
if [ -n "$GIT_BRANCH" ]; then
    LINE1="${LINE1} ${DIM}(${RESET}${YELLOW}${GIT_BRANCH}${RESET}${DIM})${RESET}"
fi
if [ -n "$GIT_CHANGES" ]; then
    LINE1="${LINE1} ${GIT_CHANGES}"
fi
if [ -n "$CC_VER_DISPLAY" ]; then
    LINE1="${LINE1} ${DIM}${CC_VER_DISPLAY}${RESET}"
fi

# Format CWD (show last 2 path components) - for compact display
if [ -n "$CWD" ]; then
    CWD_SHORT=$(echo "$CWD" | rev | cut -d'/' -f1-2 | rev)
else
    CWD_SHORT=""
fi

# Get tokens
if [ "$CURRENT_USAGE" != "null" ] && [ -n "$CURRENT_USAGE" ]; then
    INPUT_TOKENS=$(echo "$CURRENT_USAGE" | jq -r '.input_tokens // 0')
    CACHE_CREATION=$(echo "$CURRENT_USAGE" | jq -r '.cache_creation_input_tokens // 0')
    CACHE_READ=$(echo "$CURRENT_USAGE" | jq -r '.cache_read_input_tokens // 0')
    TOTAL_TOKENS=$((INPUT_TOKENS + CACHE_CREATION + CACHE_READ))
else
    TOTAL_TOKENS=0
fi

# Ensure numeric
CONTEXT_SIZE=${CONTEXT_SIZE:-200000}
[ "$CONTEXT_SIZE" -eq 0 ] 2>/dev/null && CONTEXT_SIZE=200000
TOTAL_TOKENS=${TOTAL_TOKENS:-0}

# Calculate percentage
if [ "$CONTEXT_SIZE" -gt 0 ]; then
    PERCENT=$((TOTAL_TOKENS * 100 / CONTEXT_SIZE))
else
    PERCENT=0
fi

# Format tokens (e.g., 51k, 1.2M)
format_tokens() {
    # Always 2 decimals for consistent movement visibility
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

format_tokens_fixed() {
    # No decimals for fixed values (context size, etc.)
    local n=$1
    if [ "$n" -ge 1000000000 ]; then
        awk "BEGIN {printf \"%.0fB\", $n/1000000000}"
    elif [ "$n" -ge 1000000 ]; then
        awk "BEGIN {printf \"%.0fM\", $n/1000000}"
    elif [ "$n" -ge 1000 ]; then
        awk "BEGIN {printf \"%.0fk\", $n/1000}"
    else
        echo "$n"
    fi
}

# Format time (seconds to human readable)
format_time() {
    # Simple format for estimated ranges - just primary unit
    local sec=$1
    if [ "$sec" -ge 3600 ]; then
        awk "BEGIN {printf \"%.0fh\", $sec / 3600}"
    elif [ "$sec" -ge 60 ]; then
        awk "BEGIN {printf \"%.0fm\", $sec / 60}"
    else
        echo "${sec}s"
    fi
}

TOTAL_FMT=$(format_tokens $TOTAL_TOKENS)
CTX_SIZE_FMT=$(format_tokens_fixed $CONTEXT_SIZE)

# Context color
if [ "$PERCENT" -le 70 ]; then CTX_COLOR=$GREEN
elif [ "$PERCENT" -lt 85 ]; then CTX_COLOR=$YELLOW
else CTX_COLOR=$RED
fi

# === INTENT COUNT - will be set from /metrics response below ===
INTENTS=0

# === GET AOA METRICS (with timing) ===
START_TIME=$(date +%s%N)
METRICS=$(curl -s --max-time 0.3 "${AOA_URL}/metrics?project_id=${AOA_PROJECT_ID}" 2>/dev/null)
END_TIME=$(date +%s%N)

# Calculate response time in ms
if [ -n "$METRICS" ]; then
    RESPONSE_MS=$(( (END_TIME - START_TIME) / 1000000 ))
else
    RESPONSE_MS=0
fi

if [ -z "$METRICS" ]; then
    # aOa not running - minimal output (still show both lines)
    echo -e "${LINE1}"
    echo -e "${CYAN}${BOLD}⚡ aOa${RESET} ${DIM}offline${RESET} ${DIM}│${RESET} ctx:${CTX_COLOR}${TOTAL_FMT}/${CTX_SIZE_FMT}${RESET} ${DIM}(${PERCENT}%)${RESET} ${DIM}│${RESET} ${MODEL}"
    exit 0
fi

# Parse metrics
STOP_COUNT=$(echo "$METRICS" | jq -r '.stop_count // 0')
STOP_COUNT=${STOP_COUNT:-0}
TOKENS_SAVED=$(echo "$METRICS" | jq -r '.savings.tokens // 0')
TIME_SEC_LOW=$(echo "$METRICS" | jq -r '.savings.time_sec_low // 0')
TIME_SEC_HIGH=$(echo "$METRICS" | jq -r '.savings.time_sec_high // 0')
TIME_SEC_LOW_INT=$(printf "%.0f" "$TIME_SEC_LOW")
TIME_SEC_HIGH_INT=$(printf "%.0f" "$TIME_SEC_HIGH")
INTENTS=$(echo "$METRICS" | jq -r '.total_intents // 0')
INTENTS=${INTENTS:-0}

# === BUILD DISPLAY ===
SEP="${DIM}│${RESET}"

# Traffic light based on total_intents (learning progress)
# Synced with aoa intent thresholds: <30 learning, 30-100 adapting, 100+ trained
if [ "$INTENTS" -lt 30 ] 2>/dev/null; then
    LIGHT="${GRAY}⚪${RESET}"
    INTENT_DISPLAY="learning"
elif [ "$INTENTS" -lt 100 ] 2>/dev/null; then
    LIGHT="${YELLOW}🟡${RESET}"
    INTENT_DISPLAY="${INTENTS}"
else
    LIGHT="${GREEN}🟢${RESET}"
    INTENT_DISPLAY="${INTENTS}"
fi

# Format intents for display (1.2k for large numbers)
if [ "$INTENTS" -ge 100 ] && [ "$INTENTS" -ge 1000 ]; then
    INTENT_FMT=$(format_tokens $INTENTS)
    INTENT_DISPLAY="${INTENT_FMT}"
fi

# Middle section: savings OR learning
if [ "$TOKENS_SAVED" -gt 0 ] 2>/dev/null; then
    # Have savings - show them with time range
    TOKENS_SAVED_FMT=$(format_tokens $TOKENS_SAVED)
    TIME_LOW_FMT=$(format_time $TIME_SEC_LOW_INT)
    TIME_HIGH_FMT=$(format_time $TIME_SEC_HIGH_INT)
    if [ "$TIME_LOW_FMT" = "$TIME_HIGH_FMT" ]; then
        TIME_RANGE="~${TIME_LOW_FMT}"
    else
        TIME_RANGE="${TIME_LOW_FMT}-${TIME_HIGH_FMT}"
    fi
    MIDDLE="${GREEN}↓${TOKENS_SAVED_FMT}${RESET} ${CYAN}⚡${TIME_RANGE}${RESET} saved"
else
    # No savings yet - show learning
    MIDDLE="${DIM}learning${RESET}"
fi

# Check enrichment status for intelligence angle display
ENRICHMENT=$(curl -s --max-time 0.2 "${AOA_URL}/domains/enrichment-status?project_id=${AOA_PROJECT_ID}" 2>/dev/null)

# Handle empty/missing response - default to "no domains" state
if [ -z "$ENRICHMENT" ] || [ "$ENRICHMENT" = "null" ]; then
    ENRICHED=0
    ENRICHMENT_TOTAL=0
    ENRICHMENT_COMPLETE="false"
else
    ENRICHED=$(echo "$ENRICHMENT" | jq -r '.enriched // 0' 2>/dev/null)
    ENRICHMENT_TOTAL=$(echo "$ENRICHMENT" | jq -r '.total // 0' 2>/dev/null)
    ENRICHMENT_COMPLETE=$(echo "$ENRICHMENT" | jq -r '.complete // false' 2>/dev/null)
fi

# TU-09: Get prompt count to detect first rebalance (at 100 intents prod, 20 test)
PROMPT_COUNT=0
if [ -n "$ENRICHMENT" ] && [ "$ENRICHMENT" != "null" ]; then
    PROMPT_COUNT=$(echo "$ENRICHMENT" | jq -r '.prompt_count // 0' 2>/dev/null)
fi
PROMPT_COUNT=${PROMPT_COUNT:-0}
FIRST_REBALANCE_DONE=false
[ "$PROMPT_COUNT" -gt 0 ] 2>/dev/null && FIRST_REBALANCE_DONE=true

# Check if intent learning is pending (haiku-pending flag)
LEARNING_PENDING=false
PENDING_DATA=$(curl -s --max-time 0.2 "${AOA_URL}/domains/haiku-pending?project_id=${AOA_PROJECT_ID}" 2>/dev/null)
if [ -n "$PENDING_DATA" ] && [ "$PENDING_DATA" != "null" ]; then
    PENDING_FLAG=$(echo "$PENDING_DATA" | jq -r '.pending // false' 2>/dev/null)
    [ "$PENDING_FLAG" = "true" ] && LEARNING_PENDING=true
fi

# Right section: setup → learning → ready → intent learning → intent (clean transitions)
if [ "$ENRICHMENT_TOTAL" -eq 0 ] 2>/dev/null; then
    # No domains - prompt to run /aoa-start
    RIGHT="${YELLOW}setup → run /aoa-start${RESET}"
elif [ "$ENRICHMENT_COMPLETE" != "true" ]; then
    # Domains exist but not all enriched - show progress
    RIGHT="${YELLOW}learning (${ENRICHED}/${ENRICHMENT_TOTAL})${RESET}"
elif [ "$FIRST_REBALANCE_DONE" != "true" ]; then
    # Learning complete, waiting for first rebalance (25 prompts)
    RIGHT="${GREEN}ready${RESET} ${DIM}→${RESET} ${YELLOW}tracking${RESET}"
elif [ "$LEARNING_PENDING" = "true" ]; then
    # Intent learning triggered - show rebalance state
    RIGHT="${YELLOW}rebalancing${RESET}"
else
    # Active - clean, no tags
    RIGHT=""
fi

# === OUTPUT ===
# Line 1: Environment context
echo -e "${LINE1}"

# Line 2: aOa status
# Build line 2 - only append RIGHT section if non-empty
LINE2="${CYAN}${BOLD}⚡ aOa${RESET} ${LIGHT} ${INTENT_DISPLAY} ${SEP} ${MIDDLE} ${SEP} ctx:${CTX_COLOR}${TOTAL_FMT}/${CTX_SIZE_FMT}${RESET} ${DIM}(${PERCENT}%)${RESET} ${SEP} ${MODEL}"
[ -n "$RIGHT" ] && LINE2="${LINE2} ${SEP} ${RIGHT}"
echo -e "${LINE2}"
