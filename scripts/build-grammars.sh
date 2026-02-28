#!/usr/bin/env bash
# build-grammars.sh — Compile tree-sitter grammar .so/.dylib files from
# alexaandru/go-sitter-forest C source (509 grammars).
#
# Usage:
#   ./scripts/build-grammars.sh python go typescript   # specific
#   ./scripts/build-grammars.sh --pack core            # P1 tier
#   ./scripts/build-grammars.sh --pack common          # P2 tier
#   ./scripts/build-grammars.sh --all                  # all manifest grammars
#
# Output: dist/grammars/{lang}-{os}-{arch}.so (or .dylib on macOS)
#
# Prerequisites:
#   - gcc (or cc)
#   - go-sitter-forest in Go module cache (go mod download)
set -euo pipefail

# Detect platform.
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
esac
PLATFORM="${OS}-${ARCH}"

if [ "$OS" = "darwin" ]; then
  EXT=".dylib"
  SHARED_FLAG="-dynamiclib"
else
  EXT=".so"
  SHARED_FLAG="-shared"
fi

OUTDIR="${OUTDIR:-dist/grammars}"
mkdir -p "$OUTDIR"

CC="${CC:-gcc}"
GOMODCACHE=$(go env GOMODCACHE)
FOREST_BASE="$GOMODCACHE/github.com/alexaandru/go-sitter-forest"

# Verify go-sitter-forest exists in the module cache.
if ! ls -d "$FOREST_BASE"/* >/dev/null 2>&1; then
  echo "go-sitter-forest not found in module cache."
  echo "Run: go mod download"
  exit 1
fi

# Pack definitions (grammar names per tier).
P1_CORE="python javascript typescript tsx go rust java c cpp bash json"
P2_COMMON="c_sharp ruby php kotlin yaml html css toml markdown dockerfile sql"
P3_EXTENDED="scala lua svelte hcl swift r vue dart elixir erlang groovy graphql clojure gleam cmake make nix"
P4_SPECIALIST="ocaml verilog haskell cuda zig julia d fortran nim objc vhdl purescript odin ada elm fennel glsl hlsl"

find_grammar_dir() {
  local lang="$1"
  # Find the latest version of this grammar in the module cache.
  local dir
  dir=$(ls -d "$FOREST_BASE/${lang}@"* 2>/dev/null | sort -V | tail -1)
  if [ -n "$dir" ] && [ -d "$dir" ]; then
    echo "$dir"
  fi
}

compile_grammar() {
  local lang="$1"
  local outfile="${OUTDIR}/${lang}-${PLATFORM}${EXT}"

  local src_dir
  src_dir=$(find_grammar_dir "$lang")

  if [ -z "$src_dir" ]; then
    echo "  SKIP  $lang (not found in go-sitter-forest)"
    return 1
  fi

  # parser.c is at the top level in go-sitter-forest.
  if [ ! -f "$src_dir/parser.c" ]; then
    echo "  SKIP  $lang (no parser.c in $src_dir)"
    return 1
  fi

  # Collect source files: parser.c + optional scanner.c
  local sources=("$src_dir/parser.c")
  if [ -f "$src_dir/scanner.c" ]; then
    sources+=("$src_dir/scanner.c")
  fi

  echo -n "  BUILD $lang ... "
  if $CC $SHARED_FLAG -fPIC -O2 -I"$src_dir" -o "$outfile" "${sources[@]}" 2>/dev/null; then
    local size
    size=$(stat --format=%s "$outfile" 2>/dev/null || stat -f%z "$outfile")
    echo "ok ($(( size / 1024 )) KB)"
    return 0
  else
    echo "FAILED"
    return 1
  fi
}

# Parse arguments.
GRAMMARS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --pack)
      shift
      case "${1:-}" in
        core)       GRAMMARS+=($P1_CORE) ;;
        common)     GRAMMARS+=($P2_COMMON) ;;
        extended)   GRAMMARS+=($P3_EXTENDED) ;;
        specialist) GRAMMARS+=($P4_SPECIALIST) ;;
        all)        GRAMMARS+=($P1_CORE $P2_COMMON $P3_EXTENDED $P4_SPECIALIST) ;;
        *)          echo "Unknown pack: ${1:-}"; exit 1 ;;
      esac
      shift
      ;;
    --all)
      GRAMMARS+=($P1_CORE $P2_COMMON $P3_EXTENDED $P4_SPECIALIST)
      shift
      ;;
    --help|-h)
      echo "Usage: $0 [--pack core|common|extended|specialist|all] [--all] [lang ...]"
      exit 0
      ;;
    *)
      GRAMMARS+=("$1")
      shift
      ;;
  esac
done

if [ ${#GRAMMARS[@]} -eq 0 ]; then
  echo "No grammars specified."
  echo "Usage: $0 [--pack core|common|extended|all] [--all] [lang ...]"
  exit 1
fi

echo "Platform: $PLATFORM"
echo "Output:   $OUTDIR/"
echo "Compiler: $CC"
echo "Source:   $FOREST_BASE/"
echo "Grammars: ${#GRAMMARS[@]}"
echo ""

OK=0
FAIL=0
for lang in "${GRAMMARS[@]}"; do
  if compile_grammar "$lang"; then
    OK=$((OK + 1))
  else
    FAIL=$((FAIL + 1))
  fi
done

echo ""
echo "Done: $OK built, $FAIL failed"
