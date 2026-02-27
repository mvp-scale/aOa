#!/usr/bin/env bash
# build.sh — The ONLY way to build aOa binaries.
# Direct "go build" is blocked. Use this script or "make build".
#
# Usage:
#   ./build.sh              # Standard build (no recon, no compiled grammars)
#   ./build.sh --recon      # Opt-in: include recon/dimensional analysis
#   ./build.sh --recon-bin  # Build standalone aoa-recon binary
set -euo pipefail

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS="-s -w -X github.com/corey/aoa/internal/version.Version=${VERSION} -X github.com/corey/aoa/internal/version.BuildDate=${BUILD_DATE}"

MODE="${1:-standard}"

case "$MODE" in
  --recon)
    echo "Building aoa WITH recon (opt-in)..."
    go build -tags "recon" -ldflags "$LDFLAGS" -o aoa ./cmd/aoa/
    SIZE=$(stat --format=%s aoa 2>/dev/null || stat -f%z aoa)
    echo "Built: aoa ($(( SIZE / 1048576 )) MB) [recon enabled]"
    ;;
  --recon-bin)
    echo "Building standalone aoa-recon..."
    go build -tags "recon" -ldflags "$LDFLAGS" -o aoa-recon ./cmd/aoa-recon/
    SIZE=$(stat --format=%s aoa-recon 2>/dev/null || stat -f%z aoa-recon)
    echo "Built: aoa-recon ($(( SIZE / 1048576 )) MB)"
    ;;
  standard|"")
    echo "Building aoa (standard — no recon)..."
    CGO_ENABLED=0 go build -tags "lean" -ldflags "$LDFLAGS" -o aoa ./cmd/aoa/
    SIZE=$(stat --format=%s aoa 2>/dev/null || stat -f%z aoa)
    SIZE_MB=$(( SIZE / 1048576 ))
    if [ "$SIZE" -gt 20971520 ]; then
      echo ""
      echo "FATAL: binary is ${SIZE_MB} MB — max 20 MB for standard build."
      echo "  Something dragged in CGo/treesitter/recon."
      echo "  Check build tags: //go:build !lean and //go:build recon"
      rm -f aoa
      exit 1
    fi
    echo "Built: aoa (${SIZE_MB} MB) [standard, no recon]"
    ;;
  *)
    echo "Unknown mode: $MODE"
    echo "Usage: ./build.sh [--recon|--recon-bin]"
    exit 1
    ;;
esac
