#!/bin/bash
# =============================================================================
# aOa Status Line - Two-Line Display
# =============================================================================
#
# Line 1: user:directory (branch) +add/-del cc_version
# Line 2: ⚡ aOa 🟢 42 │ savings │ ctx:28k/200k (14%) │ Model │ @domains
#
# Data sources:
#   stdin  = Claude Code JSON (context_window, model, cwd)
#   .aoa/status.json = daemon-written learner state
#
# =============================================================================

set -uo pipefail

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(pwd)}"
STATUS_FILE="$PROJECT_DIR/.aoa/status.json"

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
if [ -n "$GIT_BRANCH" ]; then
    LINE1="${LINE1} ${DIM}(${RESET}${YELLOW}${GIT_BRANCH}${RESET}${DIM})${RESET}"
fi
if [ -n "$GIT_CHANGES" ]; then
    LINE1="${LINE1} ${GIT_CHANGES}"
fi
if [ -n "$CC_VER_DISPLAY" ]; then
    LINE1="${LINE1} ${CC_VER_DISPLAY}"
fi

# === TOKEN FORMATTING ===
format_tokens() {
    local n=$1
    if [ "$n" -ge 1000000 ]; then
        awk "BEGIN {printf \"%.2fM\", $n/1000000}"
    elif [ "$n" -ge 1000 ]; then
        awk "BEGIN {printf \"%.2fk\", $n/1000}"
    else
        echo "$n"
    fi
}

format_tokens_fixed() {
    local n=$1
    if [ "$n" -ge 1000000 ]; then
        awk "BEGIN {printf \"%.0fM\", $n/1000000}"
    elif [ "$n" -ge 1000 ]; then
        awk "BEGIN {printf \"%.0fk\", $n/1000}"
    else
        echo "$n"
    fi
}

# === CONTEXT WINDOW ===
if [ "$CURRENT_USAGE" != "null" ] && [ -n "$CURRENT_USAGE" ]; then
    INPUT_TOKENS=$(echo "$CURRENT_USAGE" | jq -r '.input_tokens // 0')
    CACHE_CREATION=$(echo "$CURRENT_USAGE" | jq -r '.cache_creation_input_tokens // 0')
    CACHE_READ=$(echo "$CURRENT_USAGE" | jq -r '.cache_read_input_tokens // 0')
    TOTAL_TOKENS=$((INPUT_TOKENS + CACHE_CREATION + CACHE_READ))
else
    TOTAL_TOKENS=0
fi

CONTEXT_SIZE=${CONTEXT_SIZE:-200000}
[ "$CONTEXT_SIZE" -eq 0 ] 2>/dev/null && CONTEXT_SIZE=200000
TOTAL_TOKENS=${TOTAL_TOKENS:-0}

if [ "$CONTEXT_SIZE" -gt 0 ]; then
    PERCENT=$((TOTAL_TOKENS * 100 / CONTEXT_SIZE))
else
    PERCENT=0
fi

TOTAL_FMT=$(format_tokens $TOTAL_TOKENS)
CTX_SIZE_FMT=$(format_tokens_fixed $CONTEXT_SIZE)

if [ "$PERCENT" -le 70 ]; then CTX_COLOR=$GREEN
elif [ "$PERCENT" -lt 85 ]; then CTX_COLOR=$YELLOW
else CTX_COLOR=$RED
fi

# === READ DAEMON STATUS ===
SEP="${DIM}│${RESET}"
INTENTS=0
DOMAINS=0
TOP_DOMAINS=""

if [ -f "$STATUS_FILE" ]; then
    INTENTS=$(jq -r '.intents // 0' "$STATUS_FILE" 2>/dev/null)
    DOMAINS=$(jq -r '.domains // 0' "$STATUS_FILE" 2>/dev/null)
    TOP_DOMAINS=$(jq -r '.top_domains // [] | map("@" + .) | join(" ")' "$STATUS_FILE" 2>/dev/null)
    INTENTS=${INTENTS:-0}
    DOMAINS=${DOMAINS:-0}
fi

# Traffic light: <30 learning, 30-100 adapting, 100+ trained
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

# Format large intent counts
if [ "$INTENTS" -ge 1000 ] 2>/dev/null; then
    INTENT_DISPLAY=$(format_tokens $INTENTS)
fi

# === OUTPUT ===
echo -e "${LINE1}"

LINE2="${CYAN}${BOLD}⚡ aOa${RESET} ${LIGHT} ${INTENT_DISPLAY} ${SEP} ${DOMAINS} domains ${SEP} ctx:${CTX_COLOR}${TOTAL_FMT}/${CTX_SIZE_FMT}${RESET} ${DIM}(${PERCENT}%)${RESET} ${SEP} ${MODEL}"

if [ -n "$TOP_DOMAINS" ] && [ "$TOP_DOMAINS" != "" ]; then
    LINE2="${LINE2} ${SEP} ${DIM}${TOP_DOMAINS}${RESET}"
fi

if [ ! -f "$STATUS_FILE" ]; then
    LINE2="${CYAN}${BOLD}⚡ aOa${RESET} ${DIM}offline${RESET} ${SEP} ctx:${CTX_COLOR}${TOTAL_FMT}/${CTX_SIZE_FMT}${RESET} ${DIM}(${PERCENT}%)${RESET} ${SEP} ${MODEL}"
fi

echo -e "${LINE2}"
