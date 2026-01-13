#!/usr/bin/env python3
"""
aOa Intent Prefetch - PreToolUse Hook

Predicts related files before tool execution.
Only activates after 10+ recorded intents (avoids cold-start noise).
"""

import json
import os
import sys
from urllib.error import URLError
from urllib.request import Request, urlopen

AOA_URL = os.environ.get("AOA_URL", "http://localhost:8080")
MIN_INTENTS = 10  # Don't prefetch until we have enough data


def get_intent_count() -> int:
    """Check how many intents we have."""
    try:
        req = Request(f"{AOA_URL}/intent/stats")
        with urlopen(req, timeout=1) as resp:
            data = json.loads(resp.read().decode('utf-8'))
            return data.get('total_records', 0)
    except (URLError, Exception):
        return 0


def get_related_files(file_path: str) -> list:
    """Get files related to the given path via shared intent tags."""
    try:
        # Get tags for this file
        req = Request(f"{AOA_URL}/intent/file?path={file_path}")
        with urlopen(req, timeout=1) as resp:
            data = json.loads(resp.read().decode('utf-8'))
            tags = data.get('tags', [])

        if not tags:
            return []

        # Get files for the most common tag
        related = set()
        for tag in tags[:3]:  # Top 3 tags
            req = Request(f"{AOA_URL}/intent/files?tag={tag}")
            with urlopen(req, timeout=1) as resp:
                data = json.loads(resp.read().decode('utf-8'))
                for f in data.get('files', []):
                    if f != file_path:
                        related.add(f)

        return list(related)[:5]  # Top 5 related files

    except (URLError, Exception):
        return []


def main():
    try:
        data = json.load(sys.stdin)
    except (json.JSONDecodeError, Exception):
        return

    # Check if we have enough data
    if get_intent_count() < MIN_INTENTS:
        return

    # Extract file path from tool input
    tool_input = data.get('tool_input', {})
    file_path = tool_input.get('file_path') or tool_input.get('path')

    if not file_path:
        return

    related = get_related_files(file_path)

    if related:
        # Future: inject suggestions into Claude's context
        # For now, just output for debugging (visible in verbose mode)
        # print(f"[aOa] Related: {', '.join(related)}")
        pass


if __name__ == "__main__":
    main()
