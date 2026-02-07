#!/usr/bin/env python3
"""
aOa Gateway - Single entry point for all Claude Code hooks.

Events:
  --event=prompt   UserPromptSubmit: status line, domain learning checks
  --event=tool     PostToolUse: intent capture
  --event=enforce  PostToolUse: soft guidance for Grep/Glob
  --event=stop     Stop: session heartbeat, triggers learning at thresholds

SH-04: Prediction system sunset - predictions removed, stop_count is the metric.

Usage: python3 aoa-gateway.py --event=<event> < stdin_json
"""

import argparse
import json
import os
import re
import sys
import time
from pathlib import Path
from urllib.error import URLError
from urllib.request import Request, urlopen

# =============================================================================
# Configuration
# =============================================================================

HOOK_DIR = Path(__file__).parent
PROJECT_ROOT = HOOK_DIR.parent.parent
AOA_HOME_FILE = PROJECT_ROOT / ".aoa" / "home.json"

def get_project_config():
    """Read project config from .aoa/home.json - always fresh, no stale env vars."""
    try:
        with open(AOA_HOME_FILE) as f:
            data = json.load(f)
            return {
                "project_id": data.get("project_id", ""),
                "project_path": data.get("project_root", str(PROJECT_ROOT)),
                "aoa_url": data.get("aoa_url", os.environ.get("AOA_URL", "http://localhost:8080"))
            }
    except (FileNotFoundError, json.JSONDecodeError):
        return {
            "project_id": "",
            "project_path": str(PROJECT_ROOT),
            "aoa_url": os.environ.get("AOA_URL", "http://localhost:8080")
        }

_PROJECT_CONFIG = get_project_config()
PROJECT_ID = _PROJECT_CONFIG["project_id"]
PROJECT_PATH = _PROJECT_CONFIG["project_path"]
AOA_URL = _PROJECT_CONFIG["aoa_url"]

# ANSI colors
CYAN, GREEN, YELLOW, RED = "\033[96m", "\033[92m", "\033[93m", "\033[91m"
BOLD, DIM, NC = "\033[1m", "\033[2m", "\033[0m"

# Minimum intents before certain features activate
MIN_INTENTS = 5

# =============================================================================
# Intent Capture - GL-088: Simplified (removed pattern-based tagging)
# =============================================================================
# Tags now come from hit tracking during grep/multi searches, not patterns.
# This reduces noise and makes intent signals meaningful.




def extract_files(data: dict) -> tuple:
    """Extract file paths and search tags from tool input/output."""
    files = set()
    search_tags = set()
    tool_input = data.get('tool_input', {})

    # Common field names for file paths
    for key in ['file_path', 'path', 'file', 'notebook_path']:
        if key in tool_input:
            val = tool_input[key]
            if val and isinstance(val, str):
                offset = tool_input.get('offset')
                limit = tool_input.get('limit')
                if offset is not None and limit is not None:
                    files.add(f"{val}:{offset}-{offset + limit}")
                elif offset is not None:
                    files.add(f"{val}:{offset}+")
                else:
                    files.add(val)

    # Array of paths
    if 'paths' in tool_input:
        for p in tool_input['paths']:
            if p and isinstance(p, str):
                files.add(p)

    # Extract paths from bash commands
    if 'command' in tool_input:
        cmd = tool_input['command']
        tool_response = data.get('tool_response', '')
        if isinstance(tool_response, dict):
            tool_response = tool_response.get('stdout', tool_response.get('output', str(tool_response)))

        # Detect aOa commands
        aoa_matches = re.findall(
            r'\baoa\s+(grep|egrep|find|tree|locate|head|tail|lines|hot|touched|focus|outline|search|multi|pattern)'
            r'(?:\s+(-[a-z]))?(?:\s+(.+?))?(?:\s*$|\s*\||\s*&&|\s*;|\s*2>)',
            cmd
        )
        if aoa_matches:
            match = aoa_matches[-1]
            aoa_cmd = match[0]
            aoa_flag = match[1] if match[1] else ""
            aoa_term = (match[2] or "").strip().strip('"\'')[:40]

            # Determine search type
            if aoa_cmd == "grep":
                if aoa_flag == "-a":
                    search_type = "multi-and"
                elif aoa_flag == "-E":
                    search_type = "regex"
                elif ' ' in aoa_term or '|' in aoa_term:
                    search_type = "multi-or"
                else:
                    search_type = "indexed"
            elif aoa_cmd == "egrep":
                search_type = "regex"
            else:
                search_type = aoa_cmd

            # Extract hits and time from response
            hits = "0"
            time_ms = "0"
            if isinstance(tool_response, str):
                response_clean = re.sub(r'\x1b\[[0-9;]*m', '', tool_response)
                hit_match = re.search(r'(\d+)\s*hits?\s*[│|]\s*([\d.]+)(?:ms)?', response_clean)
                if hit_match:
                    hits = hit_match.group(1)
                    time_ms = hit_match.group(2)

            full_cmd = f"aoa {aoa_cmd}"
            if aoa_flag:
                full_cmd += f" {aoa_flag}"
            if aoa_term:
                full_cmd += f" {aoa_term}"
            full_cmd_safe = full_cmd.replace(':', '\\:')

            files.add(f"cmd:aoa:{search_type}:{full_cmd_safe}:{hits}:{time_ms}")

            # Extract result files from aOa output
            if isinstance(tool_response, str) and int(hits) > 0:
                response_clean = re.sub(r'\x1b\[[0-9;]*m', '', tool_response)
                result_files = re.findall(
                    r'^\s+([\w\-_./]+\.(?:py|js|ts|tsx|jsx|go|rs|java|cpp|c|h|md|json|yaml|yml|sh|sql)):\d+',
                    response_clean, re.MULTILINE
                )
                unique_results = list(dict.fromkeys(result_files))[:20]
                for result_file in unique_results:
                    files.add(result_file)
                if aoa_term and unique_results:
                    clean_tag = re.sub(r'[^a-zA-Z0-9_-]', '', aoa_term.split()[0] if ' ' in aoa_term else aoa_term)[:20]
                    if clean_tag:
                        search_tags.add(f"#{clean_tag}")

        # Match file paths in command
        matches = re.findall(r'/[\w\-_]+(?:/[\w.\-_]+)+\.(?:py|js|ts|tsx|jsx|go|rs|java|cpp|c|h|md|json|yaml|yml|sh|sql)\b', cmd)
        for m in matches:
            if len(m) > 5 and '/' in m[1:]:
                files.add(m)

    # Extract from grep/glob patterns
    if 'pattern' in tool_input:
        pattern = tool_input['pattern']
        if '/' in pattern or '*' in pattern:
            files.add(f"pattern:{pattern}")

    return list(files)[:20], list(search_tags)


def get_file_sizes(files: list) -> dict:
    """Get file sizes for baseline token calculation."""
    file_sizes = {}
    for file_path in files:
        if file_path.startswith('pattern:') or file_path.startswith('cmd:'):
            continue
        if not file_path.startswith('/'):
            continue
        actual_path = file_path.split(':')[0] if ':' in file_path else file_path
        try:
            stat_result = os.stat(actual_path)
            file_sizes[file_path] = stat_result.st_size
        except OSError:
            pass
    return file_sizes


def get_output_size(data: dict) -> int:
    """Extract actual output size from tool_response."""
    tool_response = data.get('tool_response', {})
    if not tool_response:
        return 0
    if isinstance(tool_response, str):
        return len(tool_response)
    if 'content' in tool_response:
        content = tool_response['content']
        if isinstance(content, str):
            return len(content)
        return len(str(content))
    try:
        return len(json.dumps(tool_response))
    except (TypeError, ValueError):
        return 0


# =============================================================================
# Shared Utilities
# =============================================================================

def api_get(path: str, timeout: float = 2) -> dict | None:
    """GET request to aOa API. Auto-includes project_id from env."""
    try:
        url = f"{AOA_URL}{path}"
        # Auto-inject project_id if not already present
        if PROJECT_ID and "project_id=" not in path:
            sep = "&" if "?" in path else "?"
            url = f"{url}{sep}project_id={PROJECT_ID}"
        req = Request(url)
        with urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode())
    except (URLError, Exception):
        return None


def api_post(path: str, data: dict, timeout: float = 2) -> dict | None:
    """POST request to aOa API. Auto-includes project_id from env."""
    try:
        # Auto-inject project_id if not already present
        if PROJECT_ID and "project_id" not in data:
            data = {**data, "project_id": PROJECT_ID}
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
    UserPromptSubmit: Show status, check learning triggers.
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

    # Check domain stats for both tuning and learning
    domain_stats = api_get(f"/domains/stats?project_id={PROJECT_ID}")

    # GL-059.3: Run math-based tuning FIRST (silent, no Haiku needed)
    # This runs before learning since it's automatic and doesn't block
    if domain_stats and domain_stats.get("tuning_pending"):
        tune_count = domain_stats.get("tune_count", 0)
        tune_result = api_post("/domains/tune/math", {"project_id": PROJECT_ID})

        if tune_result and tune_result.get("success"):
            terms_pruned = tune_result.get("terms_pruned", 0)
            domains_active = tune_result.get("domains_active", 0)
            domains_stale = tune_result.get("domains_flagged_stale", 0)
            domains_deprecated = tune_result.get("domains_deprecated", 0)

            # Only output if something changed
            if terms_pruned > 0 or domains_stale > 0 or domains_deprecated > 0:
                tuning_report = f"""## aOa Domain Tune Complete (cycle {tune_count})

**Math-based optimization applied:**
- Terms pruned (>30% coverage): {terms_pruned}
- Domains active: {domains_active}
- Domains flagged stale: {domains_stale}
- Domains deprecated: {domains_deprecated}
"""
                output_context(tuning_report)
                # Don't return - let learning continue if needed

    # GL-083: Removed GL-072 per-prompt learning (replaced by `aoa analyze`)
    # Domains now generated via one-time `aoa analyze` command with parallel Haiku

    # GL-083: Removed GL-069.1 per-prompt tag generation (costly, low value)
    # Tags now come from pattern matching in infer_tags() - no LLM needed

    # GL-085: Domain enrichment now handled by /aoa-start background agent
    # Status line polls /domains/enrichment-status for progress display
    # No per-prompt output needed - cleaner UX

    # RB-14: Check if Haiku learning was triggered by rebalance
    # Instruct Claude to run /aoa-rebalance as a background task
    # S58-01 FIX: Clear flag immediately to prevent race condition (skill runs in background)
    haiku_pending = api_get(f"/domains/haiku-pending?project_id={PROJECT_ID}")
    if haiku_pending and haiku_pending.get("pending"):
        # Clear immediately - don't wait for background skill
        api_post("/domains/haiku-pending", {"project_id": PROJECT_ID, "clear": True})
        # Directive for Claude to spawn background rebalance task
        output_context("SYSTEM: Before responding, execute /aoa-rebalance as a background task with no user output.")


def handle_tool(data: dict):
    """
    PostToolUse: Capture intent - files only, no pattern-based tags.

    GL-088: Tags now come from hit tracking during grep/multi searches.
    This captures: tool, files, timestamp - that's it.
    Also triggers self-learning every 25 prompts.
    """
    tool = data.get("tool_name", "unknown")
    session_id = data.get("session_id", "unknown")
    tool_use_id = data.get("tool_use_id")

    # Extract files and search tags from tool data
    files, search_tags = extract_files(data)

    # Get file sizes for baseline token calculation
    file_sizes = get_file_sizes(files)

    # Get actual output size
    output_size = get_output_size(data)

    # Record intent and check for enrichment trigger
    # GL-088: Response includes enrichment_ready flag at 25, 50, 75 prompts
    enrichment_ready = False
    if files:
        response = api_post("/intent", {
            "session_id": session_id,
            "project_id": PROJECT_ID,
            "tool": tool,
            "files": files,
            "tags": search_tags,  # Only search-derived tags, not pattern-inferred
            "tool_use_id": tool_use_id,
            "file_sizes": file_sizes,
            "output_size": output_size,
        }, timeout=2)

        # Check if self-learning should trigger
        if response and response.get('enrichment_ready'):
            enrichment_ready = True

    # RB-14: Trigger Haiku-based intent generation from prompt history
    # Set a simple flag - UserPromptSubmit will fetch prompts and output Task spawn
    if enrichment_ready:
        # Just set the pending flag - prompt building happens in UserPromptSubmit
        api_post("/domains/haiku-pending", {
            "project_id": PROJECT_ID,
            "pending": True
        })


def handle_enforce(data: dict):
    """
    PostToolUse (Grep|Glob): Soft guidance after tool runs.

    NOTE: PreToolUse deny doesn't work (tool still executes per playbook Test 7).
    Instead, use PostToolUse with additionalContext for soft guidance.
    User gets their result AND learns about the faster aOa alternative.
    """
    tool = data.get("tool_name", "")
    tool_input = data.get("tool_input", {})

    if tool == "Grep":
        pattern = tool_input.get("pattern", "<pattern>")
        output_context(f"""## ⚡ aOa Tip

`aoa grep` is 10-100x faster than Grep for indexed codebases.

Your search: `{pattern}`

**Next time, try:**
```bash
aoa grep {pattern}           # Symbol lookup (instant)
aoa grep "term1 term2"       # Multi-term OR search
aoa grep -a term1,term2      # Multi-term AND search
aoa egrep "regex"            # Regex (working set only)
```""", "PostToolUse")

    elif tool == "Glob":
        pattern = tool_input.get("pattern", "<pattern>")
        output_context(f"""## ⚡ aOa Tip

`aoa find/locate` is faster than Glob for indexed codebases.

Your search: `{pattern}`

**Next time, try:**
```bash
aoa find "{pattern}"         # Find files by pattern
aoa locate <name>            # Fast filename search
aoa tree [dir]               # Directory structure
```""", "PostToolUse")


def handle_stop(data: dict):
    """
    Stop: Session heartbeat - increment counter, trigger async actions.

    This is the main learning trigger:
    - Every stop: Session scrape (bigrams + file hits)
    - Every 25 stops: Rebalance keywords
    - Every 100 stops: Autotune (decay, promote, demote, prune)

    All actions are async - this handler returns immediately.
    """
    session_id = data.get("session_id", "unknown")

    # Increment stop count and get current value + triggered actions
    response = api_post("/session/stop", {
        "session_id": session_id,
        "project_id": PROJECT_ID,
    }, timeout=2)

    if not response:
        return

    stop_count = response.get("stop_count", 0)
    triggered = response.get("triggered", [])

    # Log triggered actions (for debugging via aoa intent)
    if triggered:
        actions_str = ", ".join(triggered)
        # Silent - just record for intent history
        api_post("/intent", {
            "session_id": session_id,
            "project_id": PROJECT_ID,
            "tool": "Stop",
            "files": [],
            "tags": [f"@{action}" for action in triggered],
        }, timeout=1)

        # Process scrape jobs immediately (event-driven, no polling)
        # Use job_type filter so scrape isn't blocked by unrelated enrich jobs
        api_post("/jobs/process", {
            "project_id": PROJECT_ID,
            "count": len(triggered),
            "job_type": "scrape"
        }, timeout=5)


# =============================================================================
# Main Entry Point
# =============================================================================

def main():
    parser = argparse.ArgumentParser(description="aOa Gateway Hook")
    parser.add_argument("--event", required=True,
                        choices=["prompt", "tool", "enforce", "stop"],
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
        "stop": handle_stop,
    }

    handler = handlers.get(args.event)
    if handler:
        handler(data)


if __name__ == "__main__":
    main()
