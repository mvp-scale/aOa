#!/usr/bin/env bash
# Regenerate docs/screenshots/{tab}-{view}.png from the live dashboard.
#
# Captures all 5 tabs (Live, Recon, Intel, Debrief, Arsenal) at two
# heights each (hero + main), matching the existing README image
# dimensions exactly. Output is drop-in: replaces docs/screenshots/*
# without any other manual step.
#
# Requirements:
#   - aoa daemon running (the dashboard must be reachable)
#   - chromium (snap or apt) — `sudo snap install chromium` or
#     `sudo apt install chromium-browser`
#
# Notes on snap chromium:
#   The snap-confined chromium can only write to paths under $HOME
#   (the repo qualifies). Writing to /tmp silently fails — the bytes
#   land in a snap-private overlay, not the real /tmp.
#
# Usage:
#   ./scripts/screenshots-dashboard.sh                 # uses default URL
#   DASHBOARD=http://localhost:19271 ./scripts/...     # override URL
#   OUT_DIR=/tmp/foo ./scripts/...                     # override output dir
#                                                       (must be under $HOME
#                                                       if using snap chromium)

set -euo pipefail

DASHBOARD="${DASHBOARD:-http://localhost:19270}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${OUT_DIR:-$REPO_ROOT/docs/screenshots}"

# Width matches existing screenshots (1587). Heights are deliberately
# generous — the trimmer (scripts/trim-screenshot.go) crops trailing
# background after capture, so each PNG ends at the last content row.
# Capture is driven by ?view=hero | ?view=main on the dashboard, which
# enables a CSS mode that hides the complementary slice.
WIDTH=1587
HERO_HEIGHT=600
MAIN_HEIGHT=3000   # generous; trimmer crops to content
RENDER_BUDGET_MS=4000  # one poll cycle for fresh data

TABS=(live recon intel debrief arsenal)
TRIM_TOOL="$REPO_ROOT/scripts/trim-screenshot.go"

# ---------- Resolve chromium ----------
CHROME=""
for cmd in chromium chromium-browser google-chrome chrome; do
  if command -v "$cmd" >/dev/null 2>&1; then
    CHROME="$cmd"
    break
  fi
done
if [ -z "$CHROME" ]; then
  echo "✗ No chromium/chrome binary found on PATH." >&2
  echo "  Install: sudo snap install chromium" >&2
  echo "       OR: sudo apt install chromium-browser" >&2
  exit 1
fi

# ---------- Daemon up? ----------
if ! curl -sSf "${DASHBOARD}/api/health" >/dev/null 2>&1; then
  echo "✗ Dashboard not responding at ${DASHBOARD}." >&2
  echo "  Start it: aoa daemon start" >&2
  exit 1
fi

# ---------- Capture ----------
mkdir -p "$OUT_DIR"
echo "Browser: $CHROME"
echo "Dashboard: $DASHBOARD"
echo "Output: $OUT_DIR"
echo ""

shoot() {
  local tab="$1"
  local view="$2"
  local height="$3"
  local out="$OUT_DIR/${tab}-${view}.png"
  # Query param activates body.screenshot-{view} mode; hash selects tab.
  local url="${DASHBOARD}/?view=${view}#${tab}"
  printf "  %-25s (%dx%d) → %s\n" "${tab}-${view}" "$WIDTH" "$height" "$(basename "$out")"
  "$CHROME" --headless \
    --disable-gpu \
    --no-sandbox \
    --hide-scrollbars \
    --window-size="${WIDTH},${height}" \
    --virtual-time-budget="$RENDER_BUDGET_MS" \
    --screenshot="$out" \
    "$url" 2>&1 \
    | grep -vE '^\[|Failed to call method|AppArmor|libva|Fontconfig|^$' \
    | grep -vE 'bytes written to file' \
    || true
  if [ ! -s "$out" ]; then
    echo "  ✗ ${out} not produced — check chromium permissions (snap may block writes outside \$HOME)" >&2
    return 1
  fi
  # Auto-trim trailing background to fit content.
  go run "$TRIM_TOOL" "$out" 2>&1 | sed 's/^/    /'
}

failed=0
for tab in "${TABS[@]}"; do
  shoot "$tab" "hero" "$HERO_HEIGHT" || failed=$((failed+1))
  shoot "$tab" "main" "$MAIN_HEIGHT" || failed=$((failed+1))
done

echo ""
if [ $failed -eq 0 ]; then
  echo "✓ All $((${#TABS[@]} * 2)) screenshots written to $OUT_DIR"
else
  echo "✗ $failed capture(s) failed" >&2
  exit 1
fi

# ---------- Summary ----------
echo ""
echo "Files:"
for tab in "${TABS[@]}"; do
  for view in hero main; do
    f="$OUT_DIR/${tab}-${view}.png"
    if [ -f "$f" ]; then
      size_kb=$(($(stat --format='%s' "$f") / 1024))
      dim=$(file "$f" | grep -oE '[0-9]+ x [0-9]+' | head -1)
      printf "  %-30s  %-12s  %sKB\n" "$(basename "$f")" "$dim" "$size_kb"
    fi
  done
done
