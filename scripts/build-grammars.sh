#!/usr/bin/env bash
# build-grammars.sh — Compile tree-sitter grammar .so/.dylib files from
# go-sitter-forest C source.
#
# Usage:
#   ./scripts/build-grammars.sh python go typescript   # specific grammars
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

# Find go-sitter-forest in the Go module cache.
GOMODCACHE=$(go env GOMODCACHE)
FOREST_DIR=$(find "$GOMODCACHE" -maxdepth 1 -name 'github.com' -type d)/smacker
if [ ! -d "$FOREST_DIR" ]; then
  echo "go-sitter-forest not found in module cache. Run: go mod download"
  exit 1
fi
FOREST_BASE=$(ls -d "$GOMODCACHE/github.com/smacker/go-tree-sitter"* 2>/dev/null | sort -V | tail -1)

# Pack definitions (grammar names per tier).
P1_CORE="python javascript typescript tsx go rust java c cpp bash json"
P2_COMMON="c_sharp ruby php kotlin yaml html css toml markdown dockerfile sql"
P3_EXTENDED="scala lua svelte hcl swift r vue dart elixir erlang groovy graphql clojure gleam cmake make nix"
P4_SPECIALIST="ocaml verilog haskell cuda zig julia d fortran nim objc vhdl purescript odin ada elm fennel glsl hlsl"

# Map grammar names to go-sitter-forest package directory names.
# Most grammars use their name directly. Override exceptions here.
grammar_dir_name() {
  local lang="$1"
  case "$lang" in
    c_sharp)    echo "csharp" ;;
    cpp)        echo "cpp" ;;
    tsx)        echo "tsx" ;;
    typescript) echo "typescript" ;;
    *)          echo "$lang" ;;
  esac
}

# Map grammar names to C symbol names.
c_symbol_name() {
  local lang="$1"
  echo "tree_sitter_${lang//-/_}"
}

compile_grammar() {
  local lang="$1"
  local dirname
  dirname=$(grammar_dir_name "$lang")
  local outfile="${OUTDIR}/${lang}-${PLATFORM}${EXT}"

  # Find the grammar source directory.
  local src_dir=""
  # Try go-sitter-forest layout first (smacker/go-tree-sitter).
  if [ -n "$FOREST_BASE" ] && [ -d "$FOREST_BASE/$dirname" ]; then
    src_dir="$FOREST_BASE/$dirname"
  fi

  if [ -z "$src_dir" ]; then
    echo "  SKIP  $lang (source not found)"
    return 1
  fi

  # Find parser.c — it may be at the top level or in src/.
  local parser_c=""
  if [ -f "$src_dir/parser.c" ]; then
    parser_c="$src_dir/parser.c"
  elif [ -f "$src_dir/src/parser.c" ]; then
    parser_c="$src_dir/src/parser.c"
  else
    echo "  SKIP  $lang (no parser.c found in $src_dir)"
    return 1
  fi

  local src_base
  src_base=$(dirname "$parser_c")

  # Collect source files: parser.c + optional scanner.c/scanner.cc.
  local sources=("$parser_c")
  if [ -f "$src_base/scanner.c" ]; then
    sources+=("$src_base/scanner.c")
  fi

  # Include path: parser.h is typically alongside parser.c.
  local include_dir="$src_base"

  echo -n "  BUILD $lang ... "
  if $CC $SHARED_FLAG -fPIC -O2 -I"$include_dir" -o "$outfile" "${sources[@]}" 2>/dev/null; then
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
echo "Grammars: ${#GRAMMARS[@]}"
echo ""

OK=0
FAIL=0
for lang in "${GRAMMARS[@]}"; do
  if compile_grammar "$lang"; then
    ((OK++))
  else
    ((FAIL++))
  fi
done

echo ""
echo "Done: $OK built, $FAIL failed"
