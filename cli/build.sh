#!/bin/bash
# =============================================================================
# aOa CLI Build Script
# =============================================================================
#
# Concatenates src/*.sh into a single cli/aoa file.
# The numbered prefixes ensure correct dependency order.
#
# Usage:
#   ./build.sh          Build cli/aoa from src/*.sh
#   ./build.sh test     Build and run basic tests
#
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_DIR="${SCRIPT_DIR}/src"
OUTPUT="${SCRIPT_DIR}/aoa"

echo "Building aOa CLI..."

# Verify src directory exists
if [ ! -d "$SRC_DIR" ]; then
    echo "Error: src/ directory not found"
    exit 1
fi

# Count source files
SRC_COUNT=$(ls -1 "$SRC_DIR"/*.sh 2>/dev/null | wc -l)
if [ "$SRC_COUNT" -eq 0 ]; then
    echo "Error: No .sh files found in src/"
    exit 1
fi

# Concatenate in order (numeric prefixes ensure correct order)
cat "$SRC_DIR"/*.sh > "$OUTPUT"
chmod +x "$OUTPUT"

# Report
LINES=$(wc -l < "$OUTPUT")
echo "Built: ${OUTPUT}"
echo "  Sources: ${SRC_COUNT} files"
echo "  Output:  ${LINES} lines"

# Optional test mode
if [ "$1" = "test" ]; then
    echo ""
    echo "Running basic tests..."

    # Test 1: CLI loads without error
    if "$OUTPUT" health >/dev/null 2>&1; then
        echo "  [PASS] health command"
    else
        echo "  [FAIL] health command"
        exit 1
    fi

    # Test 2: grep works
    if "$OUTPUT" grep cache >/dev/null 2>&1; then
        echo "  [PASS] grep command"
    else
        echo "  [FAIL] grep command"
        exit 1
    fi

    # Test 3: egrep works
    if "$OUTPUT" egrep "TODO" >/dev/null 2>&1; then
        echo "  [PASS] egrep command"
    else
        echo "  [FAIL] egrep command"
        exit 1
    fi

    echo ""
    echo "All tests passed!"
fi
