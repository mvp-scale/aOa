#!/usr/bin/env python3
"""
aOa Gateway - Single entry point for all Claude Code hooks.

Events:
  --event=prompt   UserPromptSubmit: status, predictions, learning trigger
  --event=tool     PostToolUse: intent capture, domain counter
  --event=enforce  PreToolUse: redirect Grep/Glob to aoa
  --event=prefetch PreToolUse: prefetch related files

Usage: python3 aoa-gateway.py --event=<event> < stdin_json
"""

import argparse
import json
import os
import sys
import time
from pathlib import Path
from urllib.error import URLError
from urllib.request import Request, urlopen

# =============================================================================
# Configuration
# =============================================================================

AOA_URL = os.environ.get("AOA_URL", "http://localhost:8080")
HOOK_DIR = Path(__file__).parent
PROJECT_ROOT = HOOK_DIR.parent.parent
AOA_HOME = PROJECT_ROOT / ".aoa" / "home.json"
PROJECT_ID = ""

if AOA_HOME.exists():
    try:
        PROJECT_ID = json.loads(AOA_HOME.read_text()).get("project_id", "")
    except (json.JSONDecodeError, OSError):
        pass

# ANSI colors
CYAN, GREEN, YELLOW, RED = "\033[96m", "\033[92m", "\033[93m", "\033[91m"
BOLD, DIM, NC = "\033[1m", "\033[2m", "\033[0m"

# =============================================================================
# Shared Utilities
# =============================================================================

def api_get(path: str, timeout: float = 2) -> dict | None:
    """GET request to aOa API."""
    try:
        req = Request(f"{AOA_URL}{path}")
        with urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode())
    except (URLError, Exception):
        return None


def api_post(path: str, data: dict, timeout: float = 2) -> dict | None:
    """POST request to aOa API."""
    try:
        req = Request(
            f"{AOA_URL}{path}",
            data=json.dumps(data).encode(),
            headers={"Content-Type": "application/json"},
            method="POST"
        )
        with urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode())
    except (URLError, Exception):
        return None


def output_context(context: str, event: str = "UserPromptSubmit"):
    """Output additionalContext for Claude."""
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": event,
            "additionalContext": context
        }
    }))


def output_deny(reason: str):
    """Deny tool use with guidance."""
    print(json.dumps({
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "deny",
            "permissionDecisionReason": reason
        }
    }))

# =============================================================================
# Event Handlers
# =============================================================================

def handle_prompt(data: dict):
    """
    UserPromptSubmit: Show status, predict files, check learning.

    Combines: aoa-intent-summary.py + aoa-predict-context.py + learning check
    """
    start = time.time()

    # Get intent stats for status line
    stats = api_get("/intent/recent?limit=50")
    if not stats:
        return

    total = stats.get("stats", {}).get("total_records", 0)
    if total == 0:
        print(f"{CYAN}{BOLD}⚡ aOa{NC} {DIM}│{NC} calibrating...")
        return

    # Get accuracy
    metrics = api_get("/metrics") or {}
    rolling = metrics.get("rolling", {})
    hit_pct = rolling.get("hit_at_5_pct", 0)
    evaluated = rolling.get("evaluated", 0)

    # Format accuracy indicator
    if evaluated < 3:
        acc = f"{YELLOW}calibrating...{NC}"
    elif hit_pct >= 80:
        acc = f"{GREEN}🟢 {BOLD}{int(hit_pct)}%{NC}"
    else:
        acc = f"{YELLOW}🟡 {BOLD}{int(hit_pct)}%{NC}"

    # Recent tags
    tags = set()
    for r in stats.get("records", [])[:10]:
        for t in r.get("tags", []):
            tags.add(t.replace("#", ""))
    tags_str = " ".join(list(tags)[:5]) or "calibrating..."

    elapsed = (time.time() - start) * 1000
    print(f"{CYAN}{BOLD}⚡ aOa{NC} {acc} {DIM}│{NC} {total} intents {DIM}│{NC} {GREEN}{elapsed:.1f}ms{NC} {DIM}│{NC} {YELLOW}{tags_str}{NC}")

    # Check if domain learning is pending
    domain_stats = api_get(f"/domains/stats?project={PROJECT_ID}")
    if domain_stats and domain_stats.get("learning_pending"):
        # Get recent activity for context
        recent = api_get(f"/intent/recent?limit=20&project_id={PROJECT_ID}") or {}
        records = recent.get("records", [])

        # Extract files and tags from recent activity
        recent_files = []
        recent_tags = set()
        for r in records:
            for f in r.get("files", []):
                if not f.startswith("pattern:") and not f.startswith("cmd:"):
                    recent_files.append(f)
            for t in r.get("tags", []):
                recent_tags.add(t)

        files_summary = "\n".join(f"  - {f}" for f in recent_files[:15])
        tags_summary = ", ".join(list(recent_tags)[:10])
        existing = domain_stats.get("domains_learned_list", [])
        existing_domains = ", ".join(existing[:10]) if existing else "None"

        # Output learning instructions for Claude
        learning_prompt = f"""## aOa Domain Learning (Auto-triggered)

aOa has collected enough activity to learn new semantic domains.

**Recent files worked on:**
{files_summary}

**Recent tags:** {tags_summary}
**Existing domains:** {existing_domains}

**Your task:** Spawn a Haiku Task to analyze these patterns and discover 1-3 new semantic domains.

Use this prompt for Haiku:
```
Given this developer activity, identify 1-3 HIGH-LEVEL semantic domains.

Files: {', '.join(recent_files[:10])}
Tags: {tags_summary}

Existing domains (don't duplicate): {existing_domains}

Return JSON only:
{{"domains": [{{"name": "@domain_name", "description": "brief description", "terms": ["term1", "term2", "term3"]}}]}}
```

After Haiku responds, POST results to aOa:
```bash
curl -s -X POST "localhost:8080/domains/add" -H "Content-Type: application/json" -d '{{"project": "{PROJECT_ID}", "domains": <haiku_response>}}'
```

Then clear the pending flag:
```bash
curl -s -X POST "localhost:8080/domains/learned" -H "Content-Type: application/json" -d '{{"project": "{PROJECT_ID}"}}'
```
"""
        output_context(learning_prompt)
        return  # Don't output status line when learning is triggered

    # Predict files from prompt keywords (TODO: migrate from predict-context.py)
    prompt = data.get("prompt", "")
    if prompt and total >= 5:
        # TODO: Extract keywords, get predictions, output context
        pass


def handle_tool(data: dict):
    """
    PostToolUse: Capture intent, increment domain counter.

    Combines: aoa-intent-capture.py functionality
    """
    tool = data.get("tool_name", "unknown")
    tool_input = data.get("tool_input", {})
    session_id = data.get("session_id", "unknown")
    tool_use_id = data.get("tool_use_id")

    # Extract files from tool input
    files = []
    for key in ["file_path", "path", "file", "notebook_path"]:
        if key in tool_input and tool_input[key]:
            files.append(tool_input[key])
            break

    # Infer tags from file paths (simplified)
    tags = []
    for f in files:
        if "test" in f.lower():
            tags.append("#testing")
        if "auth" in f.lower():
            tags.append("#authentication")

    # Record intent
    if files or tags:
        api_post("/intent", {
            "session_id": session_id,
            "project_id": PROJECT_ID,
            "tool": tool,
            "files": files,
            "tags": tags,
            "tool_use_id": tool_use_id
        }, timeout=2)


def handle_enforce(data: dict):
    """
    PreToolUse (Grep|Glob): Redirect to aoa grep/find.

    From: aoa-enforce-search.py
    """
    tool = data.get("tool_name", "")
    tool_input = data.get("tool_input", {})

    if tool == "Grep":
        pattern = tool_input.get("pattern", "<pattern>")
        output_deny(f"""Use aoa grep instead of Grep. It's 10-100x faster.

Your search: {pattern}

Syntax:
  aoa grep {pattern}           # Symbol lookup (instant)
  aoa grep "term1 term2"       # Multi-term OR search
  aoa grep -a term1,term2      # Multi-term AND search
  aoa egrep "regex"            # Regex (working set only)""")

    elif tool == "Glob":
        pattern = tool_input.get("pattern", "<pattern>")
        output_deny(f"""Use aoa find/locate instead of Glob. It's faster.

Your search: {pattern}

Commands:
  aoa find "{pattern}"         # Find files by pattern
  aoa locate <name>            # Fast filename search
  aoa tree [dir]               # Directory structure""")


def handle_prefetch(data: dict):
    """
    PreToolUse (Read|Edit|Write): Prefetch related files.

    From: aoa-intent-prefetch.py
    """
    # TODO: Migrate prefetch logic
    pass

# =============================================================================
# Main Entry Point
# =============================================================================

def main():
    parser = argparse.ArgumentParser(description="aOa Gateway Hook")
    parser.add_argument("--event", required=True,
                        choices=["prompt", "tool", "enforce", "prefetch"],
                        help="Hook event type")
    args = parser.parse_args()

    # Read stdin
    try:
        data = json.load(sys.stdin)
    except (json.JSONDecodeError, Exception):
        data = {}

    # Route to handler
    handlers = {
        "prompt": handle_prompt,
        "tool": handle_tool,
        "enforce": handle_enforce,
        "prefetch": handle_prefetch,
    }

    handler = handlers.get(args.event)
    if handler:
        handler(data)


if __name__ == "__main__":
    main()
