#!/usr/bin/env bash
# build.sh — The ONLY way to build aOa binaries.
# Direct "go build" is blocked. Use this script or "make build".
#
# Usage:
#   ./build.sh              # Standard build (tree-sitter runtime, dynamic grammars)
#   ./build.sh --light      # Light build (no tree-sitter, pure Go, minimal)
set -euo pipefail

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-s -w -X github.com/corey/aoa/internal/version.Version=${VERSION} -X github.com/corey/aoa/internal/version.BuildDate=${BUILD_DATE}"

MODE="${1:-standard}"

case "$MODE" in
  --light)
    echo "Building aoa (light — no tree-sitter)..."
    CGO_ENABLED=0 go build -tags "lean" -ldflags "$LDFLAGS" -o aoa ./cmd/aoa/
    SIZE=$(stat --format=%s aoa 2>/dev/null || stat -f%z aoa)
    SIZE_MB=$(( SIZE / 1048576 ))
    if [ "$SIZE" -gt 20971520 ]; then
      echo ""
      echo "FATAL: binary is ${SIZE_MB} MB — max 20 MB for light build."
      echo "  Something dragged in CGo/treesitter."
      echo "  Check build tags: //go:build !lean"
      rm -f aoa
      exit 1
    fi
    echo "Built: aoa (${SIZE_MB} MB) [light, no tree-sitter]"
    ;;
  --recon|--recon-bin)
    echo "DEPRECATED: $MODE is no longer supported."
    echo "  Recon functionality is being rearchitected."
    echo "  Use ./build.sh (standard) or ./build.sh --light"
    exit 1
    ;;
  --core)
    echo "DEPRECATED: --core is now the default build."
    echo "  Just run: ./build.sh"
    echo ""
    ;& # fall through to standard
  standard|"")
    echo "Building aoa..."
    go build -tags "core" -ldflags "$LDFLAGS" -o aoa ./cmd/aoa/
    SIZE=$(stat --format=%s aoa 2>/dev/null || stat -f%z aoa)
    SIZE_MB=$(( SIZE / 1048576 ))
    echo "Built: aoa (${SIZE_MB} MB)"
    ;;
  *)
    echo "Unknown mode: $MODE"
    echo "Usage: ./build.sh [--light]"
    exit 1
    ;;
esac
