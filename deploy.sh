#!/usr/bin/env bash
# deploy.sh — build + restart daemon in one clean step
# Usage: ./deploy.sh
set -euo pipefail

# 1. Build (tree-sitter runtime + dynamic grammar loading)
./build.sh

# 2. Install to PATH (if ~/bin exists)
if [ -d "$HOME/bin" ]; then ln -sf "$(pwd)/aoa" "$HOME/bin/aoa"; fi

# 3. Stop any running daemon (graceful first, force if needed)
PID=$(pgrep -f 'aoa daemon' 2>/dev/null || true)
if [ -n "$PID" ]; then
  ./aoa daemon stop 2>/dev/null && sleep 0.5 || {
    kill -9 "$PID" 2>/dev/null || true
    sleep 1
  }
fi

# 3. Clean stale socket
rm -f /tmp/aoa-*.sock

# 4. Start
./aoa daemon start
