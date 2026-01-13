#!/usr/bin/env python3
"""
aoa-enforce-search.py - PreToolUse hook to redirect Grep/Glob to aOa

Blocks Grep and Glob tool calls with helpful guidance to use aoa grep instead.
This enforces the aOa-first search pattern for better performance.
"""

import json
import sys


def main():
    try:
        input_data = json.load(sys.stdin)
    except (json.JSONDecodeError, Exception):
        # If we can't parse input, allow the tool to proceed
        sys.exit(0)

    tool_name = input_data.get("tool_name", "")
    tool_input = input_data.get("tool_input", {})

    # Only intercept Grep and Glob
    if tool_name not in ("Grep", "Glob"):
        sys.exit(0)

    # Build helpful guidance based on what they're trying to do
    if tool_name == "Grep":
        pattern = tool_input.get("pattern", "<pattern>")
        guidance = f"""Use aoa grep instead of Grep. It's 10-100x faster.

Your search: {pattern}

Syntax:
  aoa grep {pattern}           # Symbol lookup (instant)
  aoa grep "term1 term2"       # Multi-term OR search
  aoa grep -a term1,term2      # Multi-term AND search
  aoa egrep "regex"            # Regex (working set only)

Gotchas:
  - Dots/hyphens break tokens: voip.ms → search 'voip' or use -a voip,ms
  - Quotes = OR search, not phrase match
  - Use -a for AND logic

Run commands cleanly - no 2>/dev/null"""

    else:  # Glob
        pattern = tool_input.get("pattern", "<pattern>")
        guidance = f"""Use aoa find/locate instead of Glob. It's faster.

Your search: {pattern}

Commands:
  aoa find "{pattern}"         # Find files by pattern
  aoa locate <name>            # Fast filename search
  aoa tree [dir]               # Directory structure

Run commands cleanly - no 2>/dev/null"""

    # Deny the tool use with guidance
    output = {
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": guidance
        }
    }

    print(json.dumps(output))
    sys.exit(0)

if __name__ == "__main__":
    main()
