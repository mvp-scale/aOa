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
import re
import sys
import time
from pathlib import Path
from urllib.error import URLError
from urllib.request import Request, urlopen

# GL-072: Add services path for SessionReader import
SERVICES_PATH = Path(__file__).parent.parent.parent / "services"
if str(SERVICES_PATH) not in sys.path:
    sys.path.insert(0, str(SERVICES_PATH))

try:
    from session.reader import SessionReader
    SESSION_READER_AVAILABLE = True
except ImportError:
    SESSION_READER_AVAILABLE = False

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

# Prediction settings
MIN_INTENTS = 5
MAX_SNIPPET_LINES = 15
MAX_FILES = 3

# Stopwords for keyword extraction
STOPWORDS = {
    'the', 'and', 'for', 'that', 'this', 'with', 'from', 'have', 'what', 'how',
    'can', 'you', 'are', 'please', 'help', 'want', 'need', 'make', 'use', 'get',
    'add', 'fix', 'update', 'change', 'create', 'delete', 'remove', 'show', 'find',
    'look', 'see', 'let', 'know', 'would', 'could', 'should', 'will', 'just',
    'like', 'also', 'more', 'some', 'any', 'all', 'new', 'now', 'about', 'into',
    # Additional common words that aren't useful as keywords
    'then', 'through', 'when', 'where', 'which', 'while', 'been', 'being', 'were',
    'they', 'them', 'their', 'there', 'these', 'those', 'your', 'yours', 'our',
    'has', 'had', 'does', 'did', 'doing', 'done', 'going', 'goes', 'went', 'come',
    'came', 'take', 'took', 'give', 'gave', 'made', 'said', 'tell', 'told', 'ask',
    'asked', 'why', 'yes', 'not', 'but', 'only', 'very', 'even', 'still', 'already',
    'again', 'back', 'here', 'there', 'over', 'under', 'before', 'after', 'between',
    'each', 'every', 'both', 'most', 'other', 'such', 'same', 'different', 'next',
    'first', 'last', 'many', 'much', 'few', 'less', 'own', 'way', 'thing', 'things',
    'something', 'anything', 'nothing', 'everything', 'someone', 'anyone', 'everyone',
    'watching', 'commands', 'command', 'run', 'running', 'work', 'working', 'works',
}

# =============================================================================
# Intent Capture (ported from aoa-intent-capture.py)
# =============================================================================

# Intent patterns: (regex, [tags])
INTENT_PATTERNS = [
    (r'auth|login|session|oauth|jwt|password', ['#authentication', '#security']),
    (r'test[s]?[/_]|_test\.|\bspec[s]?\b|pytest|unittest', ['#testing']),
    (r'config|settings|\.env|\.yaml|\.yml|\.json', ['#configuration']),
    (r'api|endpoint|route|handler|controller', ['#api']),
    (r'index|search|query|grep|find', ['#search']),
    (r'model|schema|entity|db|database|migration|sql', ['#data']),
    (r'component|view|template|page|ui|style|css|html', ['#frontend']),
    (r'deploy|docker|k8s|ci|cd|pipeline|github', ['#devops']),
    (r'error|exception|catch|throw|raise|fail', ['#errors']),
    (r'log|debug|trace|print|console', ['#logging']),
    (r'cache|redis|memory|store', ['#caching']),
    (r'async|await|promise|thread|concurrent', ['#async']),
    (r'hook|plugin|extension|middleware', ['#hooks']),
    (r'doc|readme|comment|docstring', ['#documentation']),
    (r'util|helper|common|shared|lib', ['#utilities']),
]

# Tool action tags
TOOL_TAGS = {
    'Read': '#reading',
    'Edit': '#editing',
    'Write': '#creating',
    'Bash': '#executing',
    'Grep': '#searching',
    'Glob': '#searching',
    'Task': '#delegating',
}


def _tokenize(text: str) -> set:
    """Tokenize a string into matchable parts."""
    tokens = set()
    parts = re.split(r'[/_\-.\s]+', text)
    for part in parts:
        if not part:
            continue
        part_lower = part.lower()
        tokens.add(part_lower)
        # Split camelCase
        camel_parts = re.findall(r'[A-Z]?[a-z]+|[A-Z]+(?=[A-Z][a-z]|\d|\W|$)|\d+', part)
        for cp in camel_parts:
            tokens.add(cp.lower())
    return tokens


def infer_tags(files: list, tool: str) -> list:
    """Infer semantic intent tags from file paths and tool."""
    tags = set()

    # Add tool action tag
    if tool in TOOL_TAGS:
        tags.add(TOOL_TAGS[tool])

    # Collect all tokens from all files
    combined_text = ""
    for f in files:
        if f.startswith('pattern:') or f.startswith('cmd:'):
            continue
        combined_text += " " + f

    # Match against intent patterns
    for pattern, pattern_tags in INTENT_PATTERNS:
        if re.search(pattern, combined_text, re.IGNORECASE):
            tags.update(pattern_tags)

    return list(tags)


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
            r'\baoa\s+(grep|egrep|find|tree|locate|head|tail|lines|hot|touched|focus|predict|outline|search|multi|pattern)'
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
# Prediction System (ported from aoa-predict-context.py)
# =============================================================================

def extract_keywords(prompt: str) -> list:
    """Extract likely file/symbol keywords from user's prompt."""
    # Find potential identifiers (camelCase, snake_case, etc.)
    words = re.findall(r'\b[a-zA-Z_][a-zA-Z0-9_]*\b', prompt.lower())

    # Filter stopwords and very short words
    keywords = [w for w in words if w not in STOPWORDS and len(w) > 2]

    # Also extract file-like patterns
    file_patterns = re.findall(r'[\w\-]+\.(py|js|ts|tsx|md|json|yaml|yml)', prompt.lower())
    for fp in file_patterns:
        name = fp.rsplit('.', 1)[0]
        if name and name not in keywords:
            keywords.append(name)

    # Dedupe while preserving order
    seen = set()
    unique = []
    for k in keywords:
        if k not in seen:
            seen.add(k)
            unique.append(k)

    return unique[:10]


def get_predictions(keywords: list) -> dict:
    """Call aOa /predict endpoint with extracted keywords."""
    if not keywords:
        return {'files': []}

    keyword_str = ','.join(keywords)
    return api_get(f"/predict?keywords={keyword_str}&limit={MAX_FILES}&snippet_lines={MAX_SNIPPET_LINES}") or {'files': []}


def format_prediction_context(files: list, keywords: list) -> str:
    """Format predicted files as additionalContext for Claude."""
    if not files:
        return ""

    project_root = str(PROJECT_ROOT)

    def rel_path(path):
        if path.startswith(project_root):
            return path[len(project_root):].lstrip('/')
        return path

    parts = ["## aOa Predicted Files", ""]
    parts.append(f"Based on keywords: {', '.join(keywords[:5])}")
    parts.append("")

    for f in files:
        path = rel_path(f.get('path', ''))
        confidence = f.get('confidence', 0)
        snippet = f.get('snippet', '')

        parts.append(f"### `{path}` ({confidence:.0%} confidence)")
        parts.append("")

        if snippet:
            ext = path.rsplit('.', 1)[-1] if '.' in path else ''
            lang = ext if ext in ['py', 'js', 'ts', 'tsx', 'json', 'yaml', 'md', 'sh'] else ''
            parts.append(f"```{lang}")
            parts.append(snippet.rstrip())
            parts.append("```")
            parts.append("")

    parts.append("*Consider these files if relevant to your task.*")
    return "\n".join(parts)


def log_prediction(session_id: str, files: list, keywords: list):
    """Log prediction for hit/miss tracking and intent display."""
    if not files:
        return

    file_paths = [f.get('path', '') for f in files]
    avg_confidence = sum(f.get('confidence', 0) for f in files) / len(files) if files else 0

    # Log to /predict/log for hit/miss tracking
    api_post("/predict/log", {
        'session_id': session_id,
        'predicted_files': file_paths,
        'tags': keywords[:5],
        'trigger_file': 'UserPromptSubmit',
        'confidence': avg_confidence
    }, timeout=1)

    # Record as Predict intent for aoa intent display
    # Tags should reflect what was predicted, not raw search keywords
    # Use infer_tags on predicted files for semantic meaning
    predicted_tags = infer_tags(file_paths[:5], 'Predict')
    predicted_tags.append(f"@{avg_confidence:.0%}")
    api_post("/intent", {
        'session_id': session_id,
        'project_id': PROJECT_ID,
        'tool': 'Predict',
        'files': file_paths[:5],
        'tags': predicted_tags
    }, timeout=1)


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

    # Check domain stats for both tuning and learning
    domain_stats = api_get(f"/domains/stats?project={PROJECT_ID}")

    # GL-059.3: Run math-based tuning FIRST (silent, no Haiku needed)
    # This runs before learning since it's automatic and doesn't block
    if domain_stats and domain_stats.get("tuning_pending"):
        tune_count = domain_stats.get("tune_count", 0)
        tune_result = api_post("/domains/tune/math", {"project": PROJECT_ID})

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

    # Check if domain learning is pending
    if domain_stats and domain_stats.get("learning_pending"):
        # GL-070: Hook-side automatic domain generation
        # Instead of outputting instructions for Claude, execute learning directly
        # This ensures 100% reliability - no dependence on Claude compliance

        # Get recent activity for context
        recent = api_get(f"/intent/recent?limit=20&project_id={PROJECT_ID}") or {}
        records = recent.get("records", [])

        # Extract keywords from activity and recent files (GL-071)
        recent_tags = set()
        file_keywords = set()
        recent_files = set()  # GL-071: Collect files for domain assignment
        recent_locations = []  # GL-071.1: Rich symbol locations for domain assignment
        seen_symbols = set()  # Dedupe locations
        for r in records:
            # GL-071: Extract files
            for f in r.get("files", []):
                if f and not f.startswith('.context/') and not f.startswith('learn:'):
                    recent_files.add(f)
            # Extract tags
            for t in r.get("tags", []):
                clean = t.lstrip('#').lower()
                if len(clean) >= 3 and clean not in {'reading', 'editing', 'creating', 'executing', 'search'}:
                    recent_tags.add(clean)
            # GL-071.1: Extract rich locations for symbol-level domain assignment
            for loc in r.get("locations", []):
                symbol = loc.get("symbol", "")
                file_path = loc.get("file", "")
                if symbol and file_path and len(symbol) >= 3:
                    # Dedupe by file:symbol
                    key = f"{file_path}:{symbol}"
                    if key not in seen_symbols:
                        seen_symbols.add(key)
                        recent_locations.append({
                            "file": file_path,
                            "symbol": symbol,
                            "kind": loc.get("kind", ""),
                            "parent": loc.get("parent"),
                            "start_line": loc.get("start_line"),
                            "end_line": loc.get("end_line")
                        })
                    # Also extract keywords from symbol names
                    words = re.split(r'[_\s]|(?<=[a-z])(?=[A-Z])', symbol)
                    for w in words:
                        if len(w) >= 3:
                            file_keywords.add(w.lower())

        # Get orphan tags (high-value signals)
        orphan_tags = []
        orphan_count = domain_stats.get("orphan_count", 0)
        if orphan_count > 0:
            orphan_data = api_get(f"/domains/orphans?project={PROJECT_ID}&limit=20")
            if orphan_data:
                orphan_tags = orphan_data.get("orphans", [])

        # Get existing domain names to avoid duplicates
        all_domains = api_get(f"/domains/list?project={PROJECT_ID}&limit=100") or {}
        existing_names = [d['name'] for d in all_domains.get('domains', [])]

        # GL-072: Use Haiku for semantic domain generation
        # Gather rich activity data for the prompt

        # 1. User prompts from SessionReader (if available)
        user_prompts = []
        if SESSION_READER_AVAILABLE:
            try:
                reader = SessionReader(str(PROJECT_ROOT))
                user_prompts = reader.get_recent_prompts(10)
                # Clean prompts - truncate and filter noise
                user_prompts = [p[:150] for p in user_prompts if len(p) > 15 and not p.startswith('/')][:8]
            except Exception:
                pass

        # 2. Files + symbols from intent locations (what was read)
        file_symbols = []
        for loc in recent_locations[:15]:
            symbol = loc.get("symbol", "")
            file_path = loc.get("file", "").replace(str(PROJECT_ROOT) + "/", "")
            if symbol and file_path:
                file_symbols.append(f"{file_path}:{symbol}()")

        # 3. Orphan tags already collected above

        # Build Haiku prompt if we have enough data
        has_data = len(user_prompts) >= 2 or len(file_symbols) >= 2 or len(orphan_tags) >= 2

        if has_data:
            # Format activity data for prompt
            prompts_str = "; ".join(user_prompts[:5]) if user_prompts else "(none)"
            files_str = ", ".join(file_symbols[:10]) if file_symbols else "(none)"
            orphans_str = ", ".join(orphan_tags[:15]) if orphan_tags else "(none)"
            existing_str = ", ".join(existing_names[:20])

            haiku_prompt = f"""## aOa Domain Learning (GL-072)

Create 1-3 code domains from this activity. Use Haiku model.

Domain = semantic label (@auth, @cache, @render)
Terms = 3-7 grep-able words per domain (single words preferred)
CRITICAL: All terms must be UNIQUE across domains - no duplicates

Activity:
- Prompts: {prompts_str}
- Reads: {files_str}
- Orphans: {orphans_str}

Existing (skip similar): {existing_str}

Bad: "increment_term_hits" | Good: "hits"
Bad: @term_hit_validation | Good: @prediction

VALIDATION before POST:
- Domain names: must start with @, lowercase, no spaces
- Terms: must be real words (3+ chars), no empty strings
- JSON: must be valid, parseable

After generating, POST result:
```bash
curl -s -X POST "localhost:8080/domains/add" -H "Content-Type: application/json" -d '{{"project": "{PROJECT_ID}", "domains": YOUR_DOMAINS}}'
curl -s -X POST "localhost:8080/domains/learned" -H "Content-Type: application/json" -d '{{"project": "{PROJECT_ID}"}}'
```

Output JSON: {{"domains":[{{"name":"@example","terms":["word1","word2","word3"]}}]}}"""

            output_context(haiku_prompt)
        else:
            # Not enough data, clear pending
            api_post("/domains/learned", {"project": PROJECT_ID}, timeout=2)

        # Don't return - continue to show status line

    # GL-069.1: Per-prompt semantic tag generation
    # Only when: domains exist, learning NOT pending, and we have a prompt
    prompt = data.get("prompt", "")
    if prompt and domain_stats and domain_stats.get("domains", 0) > 0:
        # Output minimal tag generation instructions
        tag_prompt = f"""## aOa Semantic Tags (Per-Prompt)

Generate 3-5 semantic tags for this prompt. Tags should capture the user's INTENT, not just keywords.

**User prompt:** {prompt[:200]}{'...' if len(prompt) > 200 else ''}

Output JSON then curl: {{"tags": ["tag1", "tag2", "tag3"]}}

```bash
curl -s -X POST "localhost:8080/domains/tags" -H "Content-Type: application/json" -d '{{"project": "{PROJECT_ID}", "tags": YOUR_TAGS}}'
```
"""
        output_context(tag_prompt)

    # Predict files from prompt keywords
    session_id = data.get("session_id", "unknown")

    if prompt and total >= MIN_INTENTS:
        keywords = extract_keywords(prompt)
        if keywords:
            predictions = get_predictions(keywords)
            files = predictions.get('files', [])

            if files:
                # Log prediction for hit/miss tracking
                log_prediction(session_id, files, keywords)

                # Format and output context for Claude
                context = format_prediction_context(files, keywords)
                if context:
                    output_context(context)


def handle_tool(data: dict):
    """
    PostToolUse: Capture intent with full file/tag extraction.

    Ported from: aoa-intent-capture.py
    """
    tool = data.get("tool_name", "unknown")
    session_id = data.get("session_id", "unknown")
    tool_use_id = data.get("tool_use_id")

    # Extract files and search tags from tool data
    files, search_tags = extract_files(data)

    # Infer semantic tags from file paths
    tags = infer_tags(files, tool)
    tags.extend(search_tags)

    # Get file sizes for baseline token calculation
    file_sizes = get_file_sizes(files)

    # Get actual output size
    output_size = get_output_size(data)

    # Record intent (fire-and-forget)
    if files or tags:
        api_post("/intent", {
            "session_id": session_id,
            "project_id": PROJECT_ID,
            "tool": tool,
            "files": files,
            "tags": tags,
            "tool_use_id": tool_use_id,
            "file_sizes": file_sizes,
            "output_size": output_size,
        }, timeout=2)

    # GL-062: Check if accessed files match predictions (for hit/miss tracking)
    # Only check for file-accessing tools
    if tool in ('Read', 'Edit', 'Write') and files:
        for file_path in files:
            if file_path.startswith('pattern:') or file_path.startswith('cmd:'):
                continue
            base_path = file_path.split(':')[0] if ':' in file_path else file_path
            api_post("/predict/check", {
                "session_id": session_id,
                "project_id": PROJECT_ID,
                "file": base_path
            }, timeout=1)


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
