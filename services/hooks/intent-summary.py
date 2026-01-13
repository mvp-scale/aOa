#!/usr/bin/env python3
"""
aOa Intent Summary - UserPromptSubmit Hook

Shows branded intent summary when user submits a prompt.
Output: ⚡ aOa 87% │ 877 intents │ 0.1ms │ editing python searching
        ^^^^^^^^
        Accuracy is FIRST - bright and visible
"""

import contextlib
import json
import os
import sys
import time
from urllib.error import URLError
from urllib.request import Request, urlopen

AOA_URL = os.environ.get("AOA_URL", "http://localhost:8080")

# ANSI colors - brighter for key metrics
CYAN = "\033[96m"       # Bright cyan for aOa brand
GREEN = "\033[92m"      # Bright green for good accuracy
YELLOW = "\033[93m"     # Bright yellow for tags
RED = "\033[91m"        # Bright red for low accuracy
BOLD = "\033[1m"
DIM = "\033[2m"
RESET = "\033[0m"


def get_intent_stats():
    """Fetch intent stats from aOa."""
    start = time.time()

    try:
        req = Request(f"{AOA_URL}/intent/recent?since=3600&limit=50")
        with urlopen(req, timeout=2) as resp:
            data = json.loads(resp.read().decode('utf-8'))
    except (URLError, Exception):
        return None, 0

    elapsed_ms = (time.time() - start) * 1000
    return data, elapsed_ms


def get_accuracy():
    """Fetch prediction accuracy from aOa metrics."""
    try:
        req = Request(f"{AOA_URL}/metrics")
        with urlopen(req, timeout=1) as resp:
            data = json.loads(resp.read().decode('utf-8'))
            rolling = data.get('rolling', {})
            hit_pct = rolling.get('hit_at_5_pct', 0)
            evaluated = rolling.get('evaluated', 0)
            return hit_pct, evaluated
    except (URLError, Exception):
        return None, 0


def format_accuracy(hit_pct, evaluated):
    """Format accuracy with traffic lights."""
    if evaluated < 2:
        # Learning - grey (neutral, not broken)
        return f"{DIM}⚪{RESET}"
    elif evaluated < 3:
        # Calibrating - yellow light
        return f"{YELLOW}🟡{RESET}"
    else:
        # Ready - traffic light + percentage
        pct = int(hit_pct)
        if pct >= 80:
            return f"{GREEN}🟢 {BOLD}{pct}%{RESET}"
        else:
            # Yellow for anything below 80%
            return f"{YELLOW}🟡 {BOLD}{pct}%{RESET}"


def format_output(data: dict, elapsed_ms: float) -> str:
    """Format the branded output line."""
    stats = data.get('stats', {})
    records = data.get('records', [])

    total = stats.get('total_records', 0)

    # Get recent tags (last few records)
    recent_tags = set()
    for record in records[:10]:
        for tag in record.get('tags', []):
            recent_tags.add(tag.replace('#', ''))

    # Limit to 5 most relevant tags
    tags_str = ' '.join(list(recent_tags)[:5]) if recent_tags else 'calibrating...'

    # Get accuracy - THE KEY METRIC
    hit_pct, evaluated = get_accuracy()
    accuracy_str = format_accuracy(hit_pct, evaluated)

    # Build branded output - ACCURACY FIRST
    parts = [
        f"{CYAN}{BOLD}⚡ aOa{RESET} {accuracy_str}",  # Brand + accuracy together
        f"{total} intents",
        f"{GREEN}{elapsed_ms:.1f}ms{RESET}",
    ]

    header = f" {DIM}│{RESET} ".join(parts)
    tags_display = f"{YELLOW}{tags_str}{RESET}"

    return f"{header} {DIM}│{RESET} {tags_display}"


def main():
    # Read stdin (hook input) but we don't need it
    with contextlib.suppress(Exception):
        json.load(sys.stdin)

    data, elapsed_ms = get_intent_stats()

    if data is None:
        # aOa not running - silent
        return

    if data.get('stats', {}).get('total_records', 0) == 0:
        # No intents yet - show minimal output
        print(f"{CYAN}{BOLD}⚡ aOa{RESET} {DIM}│{RESET} calibrating... {DIM}(use Claude to build intent){RESET}")
        return

    output = format_output(data, elapsed_ms)
    print(output)


if __name__ == "__main__":
    main()
