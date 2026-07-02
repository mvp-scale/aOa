#!/usr/bin/env bash
#
# capture-statusline.sh — refresh the status-line `sample.json` for recert (L20).
#
# WHY A PTY: `claude -p` (headless) renders NO status line, and
# `--no-session-persistence` / `--bare` only apply to `-p` (and `--bare` skips
# hooks, which would skip the very capture hook we need). So the status line can
# only be captured by driving an EPHEMERAL INTERACTIVE `claude` in a pseudo-tty,
# letting it complete one turn so the render is MATURE (post-API-response, all
# token fields populated), then exiting it GRACEFULLY (SIGINT) — no SIGKILL.
#
# The capture itself is done by the status-line hook (armed via a sentinel file).
# This script arms it, drives claude, waits for a mature sample, validates the
# version, copies it into versions/v<installed>-observed/sample.json, and cleans
# up after itself (sentinel, any temp hook patch, and the throwaway session).
#
# Usage:
#   compliance/conformance/capture-statusline.sh            # capture installed version
#   compliance/conformance/capture-statusline.sh --print    # just print to stdout, don't write the version dir
#
# Requires: claude (logged in), jq, script (util-linux), a PTY.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
AOA="$ROOT/.aoa"
HOOK="$AOA/hooks/aoa-status-line.sh"
SENTINEL="$AOA/.capture-statusline"
CAP="$AOA/statusline-capture.json"
PROJDIR="$HOME/.claude/projects/$(echo "$ROOT" | sed 's#/#-#g')"
PRINT_ONLY=0; [ "${1:-}" = "--print" ] && PRINT_ONLY=1

command -v claude >/dev/null || { echo "FATAL: claude not in PATH"; exit 2; }
command -v jq     >/dev/null || { echo "FATAL: jq not in PATH"; exit 2; }
command -v script >/dev/null || { echo "FATAL: 'script' (util-linux) needed for a PTY"; exit 2; }
[ -f "$HOOK" ] || { echo "FATAL: deployed hook not found ($HOOK) — run 'aoa init' first"; exit 2; }

VER="$(claude --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)"
[ -n "$VER" ] || { echo "FATAL: could not read 'claude --version'"; exit 2; }
echo "▶ capturing status-line stdin for Claude Code v$VER (interactive PTY)"

# --- ensure the deployed hook can capture; temp-swap in the source hook + auto-revert ---
PATCHED=0
if ! grep -q '.capture-statusline' "$HOOK"; then
    cp "$HOOK" "$HOOK.precapture"
    cp "$ROOT/hooks/aoa-status-line.sh" "$HOOK"   # source hook carries the capture block
    PATCHED=1
    echo "  (temp-deployed source hook for capture — reverts on exit; 'aoa init' makes it permanent)"
fi

CAPTURED_SID=""
cleanup() {
    rm -f "$SENTINEL"
    [ "$PATCHED" = 1 ] && mv -f "$HOOK.precapture" "$HOOK" 2>/dev/null
    # delete the throwaway session this capture created (avoid daemon ingest)
    [ -n "$CAPTURED_SID" ] && rm -f "$PROJDIR/$CAPTURED_SID.jsonl" 2>/dev/null
}
trap cleanup EXIT INT TERM

# --- arm (version-gated) and drive an ephemeral interactive claude ---
rm -f "$CAP"
printf '%s' "$VER" > "$SENTINEL"

mature() { jq -e '(.context_window.current_usage != null) and (.cost.total_cost_usd != null) and ((.context_window.total_output_tokens // 0) > 0)' "$1" >/dev/null 2>&1; }

# Drive a no-tool prompt in a PTY (default mode; "reply ok" triggers no tools,
# so no permission prompt to hang on). We POLL for a mature capture, then stop —
# the hook captures stdin BEFORE we kill, and the session file is discarded, so
# there is nothing to flush: a clean tree-kill is correct, no SIGINT dance needed.
script -qfc "claude 'reply with only the word: ok'" /dev/null >/dev/null 2>&1 &
DRIVER=$!
echo -n "  waiting for a mature render"
for i in $(seq 1 75); do
    sleep 1; echo -n "."
    if [ -f "$CAP" ] && mature "$CAP"; then echo " ✓ (${i}s)"; break; fi
done
kill "$DRIVER" 2>/dev/null; pkill -P "$DRIVER" 2>/dev/null; sleep 1
kill -9 "$DRIVER" 2>/dev/null; pkill -9 -P "$DRIVER" 2>/dev/null; wait "$DRIVER" 2>/dev/null

# --- validate ---
[ -f "$CAP" ] || { echo "✗ no capture produced — claude may need login, or no render fired"; exit 1; }
GOTVER="$(jq -r '.version // ""' "$CAP")"
CAPTURED_SID="$(jq -r '.session_id // ""' "$CAP")"
[ "$GOTVER" = "$VER" ] || { echo "✗ captured v$GOTVER, expected v$VER (version skew?) — not writing baseline"; exit 1; }
if ! mature "$CAP"; then
    echo "⚠ capture is IMMATURE (pre-first-API-response) — token fields not populated."
    echo "  Not writing baseline (an immature sample false-flags conditional fields as breaking)."
    exit 1
fi
echo "✓ mature v$VER sample: $(jq -r 'paths(scalars)|join(".")' "$CAP" | wc -l) fields"

if [ "$PRINT_ONLY" = 1 ]; then jq '.' "$CAP"; exit 0; fi

# --- write into the observed version dir ---
DEST="$ROOT/compliance/claude-statusline/versions/v$VER-observed"
mkdir -p "$DEST"
jq '.' "$CAP" > "$DEST/sample.json"
echo "✓ wrote $DEST/sample.json"
echo "  next: author $DEST/manifest.json (Pass4 needs the dir to have a manifest) — see README.md"
