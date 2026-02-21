# Claude Code Session Log Parsing Patterns

Research into extracting aOa signals from Claude Code JSONL session logs.

**Status**: Based on working implementations in aOa Python services
**Sources**: `services/session/metrics.py`, `services/session/reader.py`, `services/ranking/session_parser.py`
**Last Updated**: 2026-02-14

---

## Overview

Claude Code stores session transcripts in JSONL format at:
```
~/.claude/projects/{encoded-project-path}/*.jsonl
```

Project path encoding: `/home/corey/aOa` → `-home-corey-aOa`

Each session file is newline-delimited JSON. Parsing is defensively wrapped (malformed JSON lines skipped, errors logged but not thrown).

---

## Event Type Catalog

| Event Type | Role | Key Fields | Used For |
|-----------|------|-----------|----------|
| `user` | User message | `message.content`, `isMeta`, `timestamp` | User prompts, intent capture |
| `assistant` | Claude response | `message.content[]`, `message.usage`, `message.model` | Tool calls, token usage, timing |
| `system` (subtype: `turn_duration`) | System metadata | `durationMs`, `parentUuid` | Turn latency, throughput calculation |
| `file-history-snapshot` | File backup | File metadata | (Not used for signal extraction) |
| `summary` | Session summary | Session stats | (Not typically parsed) |

---

## JSON Structure Reference

### User Message Event

```jsonl
{
  "type": "user",
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-02-14T18:20:15.000Z",
  "message": {
    "content": "Research how to parse Claude Code session logs"
  },
  "isMeta": false
}
```

**Signal Extraction**:
- `message.content` → Raw user prompt (clean via regex removal of system blocks)
- `timestamp` → Conversation time
- `isMeta: true` → Skip (internal commands like `/clear`)

### Assistant Message Event

```jsonl
{
  "type": "assistant",
  "uuid": "660e8400-e29b-41d4-a716-446655440001",
  "timestamp": "2026-02-14T18:20:45.000Z",
  "message": {
    "model": "claude-opus-4-6-20260101",
    "content": [
      {
        "type": "text",
        "text": "I'll help you research session parsing patterns."
      },
      {
        "type": "tool_use",
        "id": "toolu_01234567890abcd",
        "name": "Read",
        "input": {
          "file_path": "/home/corey/aOa/services/session/metrics.py"
        }
      },
      {
        "type": "tool_use",
        "id": "toolu_01234567890abce",
        "name": "Bash",
        "input": {
          "command": "find /home/corey/aOa -type f -name '*.py' | head -10"
        }
      }
    ],
    "usage": {
      "input_tokens": 2048,
      "output_tokens": 512,
      "cache_creation_input_tokens": 100,
      "cache_read_input_tokens": 256
    }
  }
}
```

**Signal Extraction**:

**Tool Calls** (`content[].type == "tool_use"`):
- `name` → Tool identifier (Read, Edit, Write, Bash, Grep, Glob, WebFetch, etc.)
- `input.file_path` or `input.path` → File path (absolute)
- `input.offset`, `input.limit` → File range (for partial reads)
- `input.command` → Bash command (parse for `aoa grep`, file paths)
- `input.pattern` → Glob/grep pattern

**Token Usage** (`message.usage`):
- `input_tokens` → Context size
- `output_tokens` → Response generation
- `cache_creation_input_tokens` → Tokens cached (write cost)
- `cache_read_input_tokens` → Cache hits (read cost)

**Thinking** (`content[].type == "thinking"`):
- `content[].thinking` → Extended thinking text (if present)
- Used for actual output token calculation (not just `usage.output_tokens`)

### Turn Duration Event

```jsonl
{
  "type": "system",
  "subtype": "turn_duration",
  "timestamp": "2026-02-14T18:20:50.000Z",
  "parentUuid": "660e8400-e29b-41d4-a716-446655440001",
  "durationMs": 5250
}
```

**Signal Extraction**:
- `durationMs` → How long Claude took to respond (matches to assistant message via `parentUuid`)
- Used to calculate throughput: `tokens / (duration_ms / 1000)`

---

## Field Mapping for aOa Signals

### File Access Tracking

**Read Operations**:
```python
if tool == "Read":
    file_path = input.get("file_path")
    offset = input.get("offset")
    limit = input.get("limit")

    if offset and limit:
        signal = f"{file_path}:{offset}-{offset + limit}"
    else:
        signal = file_path
```

**Edit/Write Operations**:
```python
if tool in ("Edit", "Write"):
    file_path = input.get("file_path")
    # Records file was modified (write signal)
```

### Search Query Tracking

**Grep/Glob Patterns**:
```python
if tool == "Grep":
    pattern = input.get("pattern")
    # Extract from bash commands: aoa grep <pattern>

if tool == "Glob":
    pattern = input.get("pattern")
    # File discovery patterns
```

**From Bash Commands**:
```python
if tool == "Bash":
    command = input.get("command")
    # Regex: aoa\s+(grep|egrep|find|locate|tree|...)(.*?)($|\||&&|;|2>)
    # Extract: command type, flags (-a, -E), search term

    # Also parse direct file paths in commands:
    # /[\w\-_]+(?:/[\w.\-_]+)+\.(?:py|js|ts|...)\b
```

### Intent Capture

**Tool + Files**:
```python
intent = {
    "tool": "Read",
    "files": ["/home/corey/aOa/services/session/metrics.py"],
    "timestamp": "2026-02-14T18:20:45.000Z"
}
```

**Search Signals**:
```python
intent = {
    "tool": "Bash",
    "files": ["cmd:aoa:indexed:aoa grep session:15:4.73"],
    "tags": ["#session"]
}
# cmd:<tool>:<type>:<command>:<hits>:<time_ms>
```

**Token Economics**:
```python
intent = {
    "file_sizes": {"/file.py": 4096},
    "output_size": 2048,
    "file_sizes": {...}
}
# Used to estimate: input baseline, cache effectiveness
```

---

## Parsing Pseudocode

### Single Session Parse

```python
def parse_session(file_path):
    """Two-pass parse: collect durations first, then process messages."""

    result = {
        "prompts": [],
        "turns": [],
        "tool_counts": {},
        "start_time": None,
        "end_time": None,
        "total_input": 0,
        "total_output": 0,
    }

    turn_durations = {}  # uuid -> duration_ms
    all_lines = []

    # PASS 1: Collect turn durations (referenced by parentUuid)
    with open(file_path) as f:
        for line in f:
            all_lines.append(line)
            try:
                data = json.loads(line)
                if data.get("type") == "system" and data.get("subtype") == "turn_duration":
                    parent_uuid = data.get("parentUuid")
                    duration_ms = data.get("durationMs", 0)
                    if parent_uuid and duration_ms:
                        turn_durations[parent_uuid] = duration_ms
                        result["total_duration_ms"] += duration_ms
            except json.JSONDecodeError:
                continue

    # PASS 2: Process all messages with durations now available
    for line in all_lines:
        try:
            data = json.loads(line)
        except json.JSONDecodeError:
            continue

        msg_type = data.get("type")
        timestamp = data.get("timestamp")

        # Track time bounds
        if timestamp:
            if result["start_time"] is None or timestamp < result["start_time"]:
                result["start_time"] = timestamp
            if result["end_time"] is None or timestamp > result["end_time"]:
                result["end_time"] = timestamp

        # USER PROMPTS
        if msg_type == "user" and not data.get("isMeta"):
            content = data.get("message", {}).get("content", "")
            clean = clean_user_prompt(content)
            if clean:
                result["prompts"].append({
                    "text": clean,
                    "timestamp": timestamp
                })

        # ASSISTANT TURNS
        if msg_type == "assistant":
            msg = data.get("message", {})
            uuid = data.get("uuid", "")

            # Token usage
            usage = msg.get("usage", {})
            result["total_input"] += usage.get("input_tokens", 0)
            result["total_output"] += usage.get("output_tokens", 0)

            # Tool counts from content
            content = msg.get("content", [])
            for item in content:
                if item.get("type") == "tool_use":
                    tool = item.get("name", "unknown")
                    result["tool_counts"][tool] = result["tool_counts"].get(tool, 0) + 1

            # Only include completed turns (those with duration)
            duration_ms = turn_durations.get(uuid, 0)
            if duration_ms > 0:
                turn = {
                    "timestamp": timestamp,
                    "uuid": uuid,
                    "model": msg.get("model", "unknown"),
                    "input_tokens": usage.get("input_tokens", 0),
                    "output_tokens": usage.get("output_tokens", 0),
                    "duration_ms": duration_ms,
                }
                turn["tps"] = turn["output_tokens"] / (duration_ms / 1000) if duration_ms > 0 else 0
                result["turns"].append(turn)

    return result
```

### Extract Tool Events

```python
def extract_tool_events(data, session_id):
    """Extract all tool uses from an assistant message."""

    events = []
    msg = data.get("message", {})
    content = msg.get("content", [])
    timestamp = data.get("timestamp")

    for item in content:
        if item.get("type") != "tool_use":
            continue

        tool_use = {
            "session_id": session_id,
            "timestamp": timestamp,
            "tool": item.get("name"),
            "tool_use_id": item.get("id"),
            "input": item.get("input", {}),
        }

        # Extract files
        inp = tool_use["input"]
        files = []

        for key in ["file_path", "path", "notebook_path"]:
            if key in inp:
                path = inp[key]
                offset = inp.get("offset")
                limit = inp.get("limit")
                if offset and limit:
                    files.append(f"{path}:{offset}-{offset + limit}")
                else:
                    files.append(path)

        # Extract patterns
        if "pattern" in inp:
            files.append(f"pattern:{inp['pattern']}")

        tool_use["files"] = files
        events.append(tool_use)

    return events
```

### Clean User Prompt

```python
def clean_user_prompt(content):
    """Remove system-generated blocks from user prompts."""

    if not isinstance(content, str) or len(content) < 5:
        return None

    clean = content

    # Remove entire blocks (content AND tags)
    patterns = [
        r"<system-reminder>.*?</system-reminder>",
        r"<local-command-[^>]*>.*?</local-command-[^>]*>",
        r"<command-[^>]+>.*?</command-[^>]+>",
        r"<hook-[^>]+>.*?</hook-[^>]+>",
    ]

    for pattern in patterns:
        clean = re.sub(pattern, "", clean, flags=re.DOTALL)

    # Remove any remaining XML tags
    clean = re.sub(r"<[^>]+>", "", clean)

    # Collapse whitespace
    clean = re.sub(r"\s+", " ", clean).strip()

    return clean if len(clean) > 3 else None
```

### Extract aOa Commands from Bash

```python
def extract_aoa_commands(command_str):
    """Parse aOa commands from bash input."""

    pattern = r'\baoa\s+(grep|egrep|find|tree|locate|head|tail|lines|hot|touched|focus|outline|search|multi|pattern)(?:\s+(-[a-z]))?(?:\s+(.+?))?(?:\s*$|\s*\||\s*&&|\s*;|\s*2>)'

    matches = re.findall(pattern, command_str)
    if not matches:
        return []

    signals = []
    for match in matches:
        aoa_cmd = match[0]      # Command: grep, egrep, find, etc.
        aoa_flag = match[1] or ""
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
        else:
            search_type = aoa_cmd

        signals.append({
            "command": f"aoa {aoa_cmd}",
            "type": search_type,
            "term": aoa_term,
            "flag": aoa_flag,
        })

    return signals
```

---

## Session End Detection

### Method 1: Timeout (Recommended for Real-Time)

```python
def is_session_active(last_timestamp, timeout_seconds=300):
    """Check if session is still active."""
    elapsed = (datetime.now() - parse_iso_timestamp(last_timestamp)).total_seconds()
    return elapsed < timeout_seconds
```

**Behavior**:
- Real-time parsing: Wait 5 minutes after last message
- If new message arrives: Session active, continue parsing
- If timeout: Session ended, flush signals

### Method 2: Explicit End Marker

```python
def detect_session_end(line):
    """Check for explicit session end event."""
    try:
        data = json.loads(line)
        # Claude might emit a "session_end" or similar event
        # Currently not documented, but could be:
        if data.get("type") == "session_end":
            return True
        if data.get("type") == "system" and data.get("subtype") == "session_end":
            return True
    except json.JSONDecodeError:
        pass
    return False
```

### Method 3: File Modification Time

```python
def is_session_updated(file_path, min_interval_seconds=30):
    """Check if file has been modified recently."""
    try:
        mtime = os.path.getmtime(file_path)
        elapsed = time.time() - mtime
        return elapsed < min_interval_seconds
    except OSError:
        return False
```

**Real-Time Tailing Strategy**:
```
1. Detect new session file created
2. Monitor for updates (file mtime changes)
3. Parse new lines incrementally
4. When mtime hasn't changed for 5 minutes: flush and close
5. Watch for next session file
```

---

## Performance & Rate Limiting

### Parsing Speed

| Operation | Speed | Notes |
|-----------|-------|-------|
| Parse single turn | ~1-5ms | Per assistant/user pair |
| Parse full session (50 turns) | ~50-250ms | Sequential line iteration |
| Extract tool events | ~0.5ms per tool | Regex extraction |
| Authenticate paths | ~0.1ms per path | os.path.exists() checks |

### Buffering Strategy

```python
class SessionTailer:
    """Real-time session log tailer with buffering."""

    def __init__(self, file_path, flush_interval_seconds=5):
        self.file_path = file_path
        self.flush_interval = flush_interval_seconds
        self.buffer = []
        self.last_flush = time.time()

    def read_new_lines(self):
        """Read new lines from file since last read."""
        with open(self.file_path) as f:
            f.seek(self.last_position)  # Start from last known position
            for line in f:
                self.buffer.append(line)
            self.last_position = f.tell()  # Save position

    def should_flush(self):
        """Check if buffer should be flushed."""
        elapsed = time.time() - self.last_flush
        return elapsed > self.flush_interval or len(self.buffer) > 100

    def flush_signals(self):
        """Process buffered lines and POST to API."""
        events = []
        for line in self.buffer:
            try:
                data = json.loads(line)
                events.extend(self.extract_signals(data))
            except json.JSONDecodeError:
                continue

        if events:
            self.post_to_api(events)

        self.buffer = []
        self.last_flush = time.time()
```

### Rate Limiting Guidelines

**For aOa hooks** (real-time, from stdin):
- No buffering needed (hook receives data in real-time)
- POST immediately (async, non-blocking)
- No rate limits (single session, sequential)

**For batch parsing** (background job):
- Buffer 100-200 lines or 5 seconds
- POST in batches to reduce API calls
- Safe to throttle: 1 POST per 5 seconds

**Claude Code Session File Staleness**:
> Session JSONL files are **hours behind** actual conversation
> Don't rely on scraping session files for real-time signals
> Instead: Use hooks (receive real-time data via stdin)

---

## Edge Cases & Defensive Parsing

### Malformed JSON

```python
# ALWAYS wrap in try/except
for line in f:
    try:
        data = json.loads(line)
        # Process...
    except json.JSONDecodeError:
        continue  # Skip malformed lines, don't crash
    except Exception as e:
        logger.debug(f"Error: {e}")
        continue
```

### Missing Fields

```python
# Use safe_get() pattern
def safe_get(data, *keys, default=None):
    """Safely traverse nested dict."""
    result = data
    for key in keys:
        if isinstance(result, dict):
            result = result.get(key, default)
        else:
            return default
    return result if result is not None else default

# Usage
model = safe_get(msg, "model", default="unknown")
tokens = safe_get(usage, "input_tokens", default=0)
```

### Timestamp Parsing

```python
# ISO 8601 with timezone
timestamp = "2026-02-14T18:20:15.000Z"
dt = datetime.fromisoformat(timestamp.replace("Z", "+00:00"))

# Handle variations
try:
    if timestamp.endswith("Z"):
        dt = datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
    else:
        dt = datetime.fromisoformat(timestamp)
except (ValueError, TypeError):
    dt = None  # Can't parse, skip
```

### File Path Validation

```python
# Paths may be:
# - Absolute: /home/corey/aOa/file.py
# - Relative: ./file.py (rare)
# - Patterns: /src/**/*.py (glob)

def validate_file_path(path):
    """Check if path is absolute and reasonable."""
    if not path or not isinstance(path, str):
        return False
    if len(path) > 500:
        return False  # Suspiciously long
    return path.startswith("/")  # Absolute only
```

### Tool Name Variations

```python
# Normalize tool names (may vary by Claude version)
TOOL_ALIASES = {
    "Bash": ["Bash", "bash", "shell"],
    "Read": ["Read", "read", "file_read"],
    "Edit": ["Edit", "edit", "file_edit"],
    "Write": ["Write", "write", "file_write"],
    "Grep": ["Grep", "grep", "search"],
    "Glob": ["Glob", "glob", "find"],
}

def normalize_tool(tool_name):
    for canonical, aliases in TOOL_ALIASES.items():
        if tool_name in aliases:
            return canonical
    return tool_name
```

---

## Integration Points for aOa

### Hook: Real-Time Signal Capture

**Source**: `~/.claude/hooks/aoa-gateway.py`

Receives structured data via stdin:

```python
# UserPromptSubmit hook
data = {
    "session_id": "abc123...",
    "prompt": "Research how to parse..."
}

# PostToolUse hook
data = {
    "session_id": "abc123...",
    "tool_name": "Read",
    "tool_input": {"file_path": "/home/corey/aOa/services/session/metrics.py"},
    "tool_response": {"content": "...file contents..."}
}

# Stop hook (session ended)
data = {
    "session_id": "abc123...",
    "transcript_path": "/home/corey/.claude/projects/-home-corey-aOa/agent-2026-02-14-001.jsonl"
}
```

**No parsing needed** - data already extracted. Just POST to `/intent` API.

### Batch Job: Background Enrichment

**Source**: `services/ranking/session_parser.py`

Runs periodically to extract bigrams, file transitions, etc.

```python
# 1. Parse all session files
parser = SessionLogParser("/home/corey/aOa")
sessions = parser.list_sessions()

for session_file in sessions:
    events = parser.parse_session(session_file)
    reads = parser.extract_file_reads(events)

    # 2. Extract bigrams
    for i in range(len(reads) - 1):
        bigram = (reads[i], reads[i+1])
        # POST to /bigrams/increment

    # 3. Build transition matrix
    transitions = parser.build_transition_matrix()
    # Sync to Redis for predictions
```

### CLI: Session Inspection

```bash
# View recent sessions
aoa cc sessions

# View prompts
aoa cc prompts

# View token stats
aoa cc stats

# View per-turn metrics
aoa sessions --detail
```

Implemented via `SessionMetrics` class in `services/session/metrics.py`.

---

## Example: Complete Signal Extraction Flow

```python
import json
from pathlib import Path
from datetime import datetime

def extract_session_signals(session_file_path, project_id):
    """
    Complete signal extraction from a session file.

    Returns list of intent records ready for API POST.
    """

    signals = []
    turn_durations = {}
    all_lines = []

    # PASS 1: Collect metadata
    with open(session_file_path) as f:
        for line in f:
            all_lines.append(line)
            try:
                data = json.loads(line)
                if data.get("type") == "system" and data.get("subtype") == "turn_duration":
                    turn_durations[data.get("parentUuid")] = data.get("durationMs", 0)
            except json.JSONDecodeError:
                continue

    # PASS 2: Extract signals
    for line in all_lines:
        try:
            data = json.loads(line)
        except json.JSONDecodeError:
            continue

        msg_type = data.get("type")

        # USER PROMPT
        if msg_type == "user" and not data.get("isMeta"):
            content = data.get("message", {}).get("content", "")
            clean = clean_user_prompt(content)
            if clean:
                signals.append({
                    "type": "prompt",
                    "timestamp": data.get("timestamp"),
                    "project_id": project_id,
                    "text": clean,
                })

        # TOOL USE
        if msg_type == "assistant":
            msg = data.get("message", {})
            uuid = data.get("uuid")
            content = msg.get("content", [])

            for item in content:
                if item.get("type") != "tool_use":
                    continue

                tool = item.get("name", "unknown")
                inp = item.get("input", {})

                # Extract files
                files = []
                for key in ["file_path", "path", "notebook_path"]:
                    if key in inp and inp[key]:
                        path = inp[key]
                        offset = inp.get("offset")
                        limit = inp.get("limit")
                        if offset and limit:
                            files.append(f"{path}:{offset}-{offset + limit}")
                        else:
                            files.append(path)

                # Extract aOa command from bash
                if tool == "Bash" and "command" in inp:
                    aoa_cmds = extract_aoa_commands(inp["command"])
                    for cmd in aoa_cmds:
                        files.append(f"cmd:aoa:{cmd['type']}:{cmd['command']}")

                signals.append({
                    "type": "tool_use",
                    "timestamp": data.get("timestamp"),
                    "project_id": project_id,
                    "tool": tool,
                    "files": files,
                    "duration_ms": turn_durations.get(uuid, 0),
                })

    return signals
```

---

## Testing & Validation

### Unit Test: Parse Real Session

```python
def test_parse_real_session():
    from services.session.metrics import SessionMetrics

    metrics = SessionMetrics("/home/corey/aOa")
    session_files = metrics._get_session_files(limit=1)

    if not session_files:
        pytest.skip("No session files found")

    session = metrics.parse_session(session_files[0])

    # Assertions
    assert session["session_id"]
    assert isinstance(session["prompts"], list)
    assert isinstance(session["turns"], list)
    assert session["total_input"] >= 0
    assert session["total_output"] >= 0
    assert session["error"] is None
```

### Integration Test: End-to-End

```bash
# 1. Run a session
cd /home/corey/aOa
# ... do some work in Claude Code ...

# 2. Parse the session
python -c "
from services.session.metrics import SessionMetrics
m = SessionMetrics('/home/corey/aOa')
for s in m.get_sessions_summary(limit=1):
    print(f'Session: {s[\"session_id\"]}')
    print(f'Turns: {s[\"turn_count\"]}')
    print(f'Duration: {s[\"duration_min\"]}m')
    print(f'Cache: {s[\"cache_hit\"]}%')
"

# 3. Verify signals
curl http://localhost:8080/intent/recent?project_id=8f3d743a-3cc5-4e0c-8f65-2812655f0aae
```

---

## References

**aOa Implementation**:
- `services/session/metrics.py` - Comprehensive session parsing
- `services/session/reader.py` - Simpler reader interface
- `services/ranking/session_parser.py` - File transition extraction
- `.claude/hooks/aoa-gateway.py` - Real-time hook integration

**Claude Code**:
- Session location: `~/.claude/projects/{slug}/`
- File format: Newline-delimited JSON (JSONL)
- Encoding: `/home/corey/aOa` → `-home-corey-aOa`

**Key Insight**:
> Don't parse session files for real-time signals. Use hooks instead.
> Hooks receive data via stdin in real-time. Session files are hours stale.
> Parse session files only for: bigram extraction, analytics, offline analysis.
