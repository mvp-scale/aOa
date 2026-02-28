#!/usr/bin/env bash
# validate-grammars.sh — Compile every grammar from go-sitter-forest and
# produce parsers.json with compile status, size, and metadata.
#
# Usage:
#   ./scripts/validate-grammars.sh
#
# Output: dist/parsers.json + dist/grammars/*.so
set -euo pipefail

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

OUTDIR="dist/grammars"
RESULTS="dist/parsers.json"
mkdir -p "$OUTDIR"

CC="${CC:-gcc}"
GOMODCACHE=$(go env GOMODCACHE)
FOREST_BASE="$GOMODCACHE/github.com/alexaandru/go-sitter-forest"

if ! ls -d "$FOREST_BASE"/* >/dev/null 2>&1; then
  echo "go-sitter-forest not found. Run: go mod download"
  exit 1
fi

# Discover all grammar names.
GRAMMARS=()
for dir in "$FOREST_BASE"/*/; do
  name=$(basename "$dir" | sed 's/@.*//')
  GRAMMARS+=("$name")
done

# Deduplicate and sort.
readarray -t GRAMMARS < <(printf '%s\n' "${GRAMMARS[@]}" | sort -u)

TOTAL=${#GRAMMARS[@]}
echo "Platform: $PLATFORM"
echo "Compiler: $CC"
echo "Source:   $FOREST_BASE/"
echo "Output:   $OUTDIR/"
echo "Grammars: $TOTAL"
echo ""

OK=0
FAIL=0
SKIP=0

# Start JSON array.
echo "[" > "$RESULTS"
FIRST=true

for i in "${!GRAMMARS[@]}"; do
  lang="${GRAMMARS[$i]}"
  n=$((i + 1))

  # Find latest version.
  src_dir=$(ls -d "$FOREST_BASE/${lang}@"* 2>/dev/null | sort -V | tail -1)

  if [ -z "$src_dir" ] || [ ! -d "$src_dir" ]; then
    printf "  [%3d/%d] %-20s SKIP (not found)\n" "$n" "$TOTAL" "$lang"
    SKIP=$((SKIP + 1))
    continue
  fi

  if [ ! -f "$src_dir/parser.c" ]; then
    printf "  [%3d/%d] %-20s SKIP (no parser.c)\n" "$n" "$TOTAL" "$lang"
    SKIP=$((SKIP + 1))
    continue
  fi

  outfile="${OUTDIR}/${lang}${EXT}"
  sources=("$src_dir/parser.c")
  if [ -f "$src_dir/scanner.c" ]; then
    sources+=("$src_dir/scanner.c")
  fi

  # Extract version from path.
  version=$(echo "$src_dir" | grep -oP '@v?\K[^/]+$' || echo "unknown")

  # Compile.
  start_ms=$(date +%s%N)
  if $CC $SHARED_FLAG -fPIC -O2 -I"$src_dir" -o "$outfile" "${sources[@]}" 2>/tmp/gcc-err.txt; then
    end_ms=$(date +%s%N)
    elapsed_ms=$(( (end_ms - start_ms) / 1000000 ))
    size=$(stat --format=%s "$outfile" 2>/dev/null || stat -f%z "$outfile")
    size_kb=$(( size / 1024 ))
    sha=$(sha256sum "$outfile" | cut -d' ' -f1)

    printf "  [%3d/%d] %-20s ok  %4d KB  %dms\n" "$n" "$TOTAL" "$lang" "$size_kb" "$elapsed_ms"

    # Write JSON entry.
    if [ "$FIRST" = true ]; then
      FIRST=false
    else
      echo "," >> "$RESULTS"
    fi
    cat >> "$RESULTS" <<ENTRY
  {
    "name": "$lang",
    "version": "$version",
    "status": "ok",
    "platform": "$PLATFORM",
    "size_bytes": $size,
    "sha256": "$sha",
    "source": "github.com/alexaandru/go-sitter-forest/${lang}@${version}",
    "compile_ms": $elapsed_ms
  }
ENTRY
    OK=$((OK + 1))
  else
    end_ms=$(date +%s%N)
    elapsed_ms=$(( (end_ms - start_ms) / 1000000 ))
    err_line=$(head -1 /tmp/gcc-err.txt 2>/dev/null || echo "unknown error")
    printf "  [%3d/%d] %-20s FAIL  %dms  %s\n" "$n" "$TOTAL" "$lang" "$elapsed_ms" "$err_line"
    rm -f "$outfile"

    if [ "$FIRST" = true ]; then
      FIRST=false
    else
      echo "," >> "$RESULTS"
    fi
    cat >> "$RESULTS" <<ENTRY
  {
    "name": "$lang",
    "version": "$version",
    "status": "fail",
    "platform": "$PLATFORM",
    "error": "$(echo "$err_line" | sed 's/"/\\"/g')"
  }
ENTRY
    FAIL=$((FAIL + 1))
  fi
done

echo "" >> "$RESULTS"
echo "]" >> "$RESULTS"

echo ""
echo "Done: $OK compiled, $FAIL failed, $SKIP skipped (of $TOTAL)"
echo "Results: $RESULTS"
echo "Grammars: $OUTDIR/"

# Summary.
total_size=$(du -sh "$OUTDIR" | cut -f1)
echo "Total size: $total_size"
