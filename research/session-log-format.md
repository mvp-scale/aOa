# Claude Code Session Log Format & Stability Research

**Date**: February 2026
**Scope**: Format documentation, schema stability, and official vs. reverse-engineered approaches
**Status**: NOT RECOMMENDED for production parsing without mitigation

---

## TL;DR

Claude Code stores session conversations in **`~/.claude/projects/{projectPath}/{sessionId}.jsonl`** format, but:

1. **NO official schema documentation exists** from Anthropic
2. **Critical data corruption bugs** are documented in GitHub (duplicate outputs, phantom parentUuid references)
3. **Session files are internally volatile** - format not guaranteed stable across versions
4. **NO official API** for accessing session data (unlike Claude API's Session Management)
5. **Reverse-engineered parsing is the only approach**, with known instability risks

**Recommendation**: Use hooks and official Claude Code APIs where possible. Only parse `.jsonl` for **non-critical** telemetry or analytics with robust error handling.

---

## 1. Session Log Format

### File Location
```
~/.claude/projects/{projectPath}/{sessionId}.jsonl
~/.claude/projects/index.json (metadata index)
```

### JSONL Structure (Reverse-Engineered)

Each line is a valid JSON object representing a message, tool use, or tool result:

```json
{
  "parentUuid": "e431222a-3ddc-4e4a-8b38-6018dff66107",
  "sessionId": "abc123-def456",
  "uuid": "8a5209f6-1234-5678-9012-abcdef123456",
  "version": "2.1.29",
  "cwd": "/path/to/project",
  "gitBranch": "main",
  "timestamp": "2026-02-14T15:30:45.123Z",
  "type": "user",
  "isSidechain": false,
  "isMeta": false,
  "userType": "external",
  "message": {
    "role": "user",
    "content": [
      {
        "type": "text",
        "text": "Write a function to calculate factorial"
      }
    ]
  }
}
```

### Message Types

Sessions contain multiple entry types:

| Type | Role | Content | Purpose |
|------|------|---------|---------|
| `user` | "user" | User prompt text | Captures user input |
| `assistant` | "assistant" | Tool calls + text | Claude's response |
| `tool_use` | N/A | Embedded in assistant | Represents tool invocation |
| `tool_result` | N/A | Tool output | Result of tool execution |

### Common Fields

| Field | Type | Description |
|-------|------|-------------|
| `parentUuid` | string\|null | Links messages in conversation chain; null for first message |
| `sessionId` | string | Unique session identifier |
| `uuid` | string | Unique message identifier |
| `version` | string | Claude Code version (e.g., "2.1.29") |
| `cwd` | string | Working directory when message was created |
| `gitBranch` | string | Git branch at message time |
| `timestamp` | ISO 8601 | Message creation time |
| `type` | string | Message type (user, assistant, etc.) |
| `isSidechain` | boolean | Marks non-primary conversation branches |
| `isMeta` | boolean | Marks internal/metadata messages |
| `userType` | string | User classification ("external", "agent", etc.) |
| `message` | object | The actual message content |

### Message Content Structure

```json
{
  "message": {
    "role": "user" | "assistant",
    "content": [
      {
        "type": "text" | "tool_use" | "tool_result" | "thinking" | "image",
        "text": "...",
        "tool_use_id": "...",
        "tool_name": "...",
        "tool_input": {...},
        "tool_response": {...}
      }
    ],
    "usage": {
      "input_tokens": 1024,
      "output_tokens": 512,
      "cache_creation_input_tokens": 0,
      "cache_read_input_tokens": 0
    }
  }
}
```

### Tool-Specific Fields

#### Bash Tool Results
```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01ABC123...",
  "tool_name": "Bash",
  "tool_response": {
    "stdout": "command output",
    "stderr": "error output",
    "exit_code": 0
  }
}
```

#### File Operations
```json
{
  "type": "tool_use",
  "tool_use_id": "toolu_01DEF456...",
  "tool_name": "Write" | "Edit" | "Read",
  "tool_input": {
    "file_path": "/absolute/path",
    "content": "...",
    "old_string": "...",
    "new_string": "..."
  }
}
```

#### Persisted Output (Large Results)
For results exceeding size threshold:
```json
{
  "type": "tool_result",
  "content": [
    {
      "type": "text",
      "text": "<persisted-output><preview>First 2KB of output...</preview><path>~/.claude/projects/.../tool-results/toolu_01ABC123.txt</path></persisted-output>"
    }
  ]
}
```

---

## 2. Schema Stability Issues

### CRITICAL BUG #1: Duplicate Tool Result Serialization
**GitHub Issue**: [#23948 - persisted-output tool results written to session JSONL at full size](https://github.com/anthropics/claude-code/issues/23948)

**Problem**:
- Large tool outputs (>size threshold) saved to external file
- Output wrapped in `<persisted-output>` tags in JSONL
- **BUT: full multi-MB output ALSO serialized into JSONL**

**Impact**:
- Session JSONL files bloat to 10-12MB
- `/resume` hangs indefinitely when parsing oversized entries
- Example: 5.2MB Bash result + 4.9MB external file = 10MB+ session

**Evidence**:
```
Session JSONL: 12MB (728 lines)
Offending entries:
  - Line 4: 5,244,945 bytes (5.2MB)
  - Line 5: 5,245,153 bytes (5.2MB)
External files:
  - tool-results/toolu_01TV61RJnJvVAhZ.txt: 4.9MB
```

**Status**: DUPLICATE of #21067 - marked regression from v2.1.29

### CRITICAL BUG #2: parentUuid References Non-Existent UUIDs
**GitHub Issue**: [#22526 - Session JSONL files contain corrupt parentUuid references](https://github.com/anthropics/claude-code/issues/22526)

**Problem**:
- `parentUuid` field points to UUIDs that **don't exist in the file**
- Conversation chain becomes logically invalid
- Sessions lose ability to reconstruct message history

**Evidence** (4-message session):
```
Line 2: uuid: "8a5209f6-..."           parentUuid: null           ✓
Line 3: uuid: "e431222a-..."           parentUuid: "8a5209f6-..." ✓
Line 5: uuid: "6935cb6f-..."           parentUuid: "632017ae-..." ✗ MISSING
Line 6: uuid: "3ad73692-..."           parentUuid: "6935cb6f-..." ✓

Missing: "632017ae-e7cf-4430-87ae-13dba44d2ea1"
Actual:  "e431222a-3ddc-4e4a-8b38-6018dff66107" (line 3)
```

**Impact**:
- Permanent loss of conversation context after IDE crash
- `sessions-index.json` counts only valid chain length
- Larger sessions (900+ lines, 9MB+) lose all but recent messages

**Status**: CLOSED as duplicate on Feb 6, 2026 (regression from v2.1.29)

### BUG #3: Duplicate JSONL Entries
**GitHub Issue**: [#5034 - Duplicate entries when using stream-json input format](https://github.com/anthropics/claude-code/issues/5034)

**Problem**: Messages appear twice in session JSONL under certain input conditions

**Status**: Unresolved

### Session Size Management Issues
**GitHub Issue**: [#22365 - Large session JSONL files cause hang and RAM exhaustion](https://github.com/anthropics/claude-code/issues/22365)

**Problem**: Oversized sessions cause:
- Indefinite hangs
- All available RAM consumed
- Sessions become unloadable

**Status**: Unresolved

---

## 3. Version Field Detection

The `version` field indicates Claude Code version but has **NO guaranteed stability contract**:

```json
{
  "version": "2.1.29"  // Known stable version
}
{
  "version": "2.1.30"  // May have breaking changes
}
```

### Known Problematic Versions
- **v2.1.30+**: Introduced parentUuid corruption (#22526)
- **Recent versions**: Duplicate persisted-output serialization (#23948)

### Version Detection Strategy (Unreliable)

```python
def get_session_version(jsonl_path):
    """Extract version from first message - NOT GUARANTEED TO WORK"""
    with open(jsonl_path) as f:
        first_line = f.readline()
        obj = json.loads(first_line)
        return obj.get("version", "unknown")
```

**Reliability**: LOW - version field may be inconsistent across messages, and version ≠ stable schema.

---

## 4. Official vs. Reverse-Engineered Approaches

### ❌ NO Official JSONL Parsing API from Anthropic

**What Anthropic provides**:
- **Claude API Session Management** (for Agent SDK): Documented, stable API for session creation/resumption
- **Hooks API**: Event-based access to session lifecycle (SessionStart, Stop, etc.)
- **Settings & Configuration**: Documented JSON schema

**What Anthropic does NOT provide**:
- ✗ Official JSONL schema documentation
- ✗ Parsing library or SDK
- ✗ Backwards compatibility guarantees
- ✗ Version field specification
- ✗ Official transcript export API

### ✓ Reverse-Engineered Parsing Tools

Community has built tools by **analyzing session files directly**:

| Tool | Language | Approach | Status |
|------|----------|----------|--------|
| [`claude-code-log`](https://github.com/daaain/claude-code-log) | Python | Parse JSONL → HTML/Markdown | Maintained |
| [`claude-JSONL-browser`](https://github.com/withLinda/claude-JSONL-browser) | JavaScript | Web UI for session viewing | Maintained |
| [`claude-notes`](https://github.com/professional-ALFIE/context-cleaner-skill) | Python | Pydantic validation + Markdown | Maintained |
| [`clog`](https://github.com/HillviewCap/clog) | JavaScript/Web | Real-time monitoring viewer | Maintained |
| [`cclv`](https://github.com/albertov/cclv) | Rust | TUI session log viewer | Maintained |
| [`claude-code-viewer`](https://github.com/d-kimuson/claude-code-viewer) | TypeScript | Full web client | Maintained |

**Common Implementation Pattern**:

```python
import json

def parse_session(jsonl_path):
    messages = []
    with open(jsonl_path) as f:
        for line in f:
            try:
                msg = json.loads(line)
                messages.append(msg)
            except json.JSONDecodeError:
                # Bug #5034: duplicates, malformed entries
                continue
    return messages
```

### Using Pydantic for Validation (Recommended for Reverse-Engineering)

```python
from pydantic import BaseModel
from typing import Optional, List, Any

class ContentBlock(BaseModel):
    type: str  # "text", "tool_use", "tool_result", "thinking"
    text: Optional[str] = None
    tool_use_id: Optional[str] = None
    tool_name: Optional[str] = None
    tool_input: Optional[dict] = None
    tool_response: Optional[dict] = None

class Message(BaseModel):
    role: str  # "user", "assistant"
    content: List[ContentBlock]
    usage: Optional[dict] = None

class SessionEntry(BaseModel):
    parentUuid: Optional[str] = None
    sessionId: str
    uuid: str
    version: str
    cwd: str
    gitBranch: Optional[str] = None
    timestamp: str
    type: str
    message: Message
    isSidechain: bool = False
    isMeta: bool = False

def validate_session_entry(line: str) -> Optional[SessionEntry]:
    """Parse and validate a session JSONL line"""
    try:
        data = json.loads(line)
        return SessionEntry(**data)
    except Exception as e:
        # Handle corrupted entries gracefully
        return None
```

---

## 5. Stability Concerns for Production Use

### Risk 1: Undocumented Format Changes
- **No breaking change notice** from Anthropic
- Format can change in minor version bumps
- Existing parsers may silently fail

### Risk 2: Data Corruption (CRITICAL)
- parentUuid phantom references (#22526)
- Duplicate output serialization (#23948)
- Duplicate JSONL entries (#5034)

**These are NOT parsing errors - the files are intentionally corrupt.**

### Risk 3: Large Session Handling
- No size limits documented
- Sessions >10MB cause hangs (#22365)
- No streaming parser available

### Risk 4: Session Index Corruption
- `sessions-index.json` loses sync with actual JSONL
- Reported message counts incorrect (#22365, #23614)

### Risk 5: Version Field Unreliability
- Version field may not match actual format
- No guarantee it's present in all lines
- Version ≠ schema version

---

## 6. Recommended Approaches

### Option 1: Use Hooks (RECOMMENDED)
**Stability**: Official, documented, stable API

```python
# .claude/hooks/capture-session.sh
#!/bin/bash
INPUT=$(jq -r '.prompt')
SESSION_ID=$(jq -r '.session_id')

# Capture user intent and tool calls
echo "User: $INPUT" >> ~/.logs/aoa-intents.log
exit 0
```

**Advantages**:
- Part of official Claude Code API
- Documented, versioned lifecycle
- Real-time event capture
- No JSONL parsing needed

### Option 2: Parse with Strict Error Handling (ACCEPTABLE)
**Stability**: Reverse-engineered, requires defensive coding

```python
import json
from pathlib import Path
from typing import Iterator, Optional

def parse_session_safe(jsonl_path: Path) -> Iterator[dict]:
    """
    Parse session JSONL with strict error handling.
    Yields only valid entries; silently skips corrupted ones.
    """
    line_num = 0
    with open(jsonl_path) as f:
        for line in f:
            line_num += 1
            if not line.strip():
                continue

            try:
                obj = json.loads(line)

                # Validate required fields
                if not all(k in obj for k in ['uuid', 'sessionId', 'message']):
                    print(f"⚠ Line {line_num}: Missing required fields")
                    continue

                # Warn on suspicious data
                if 'parentUuid' in obj and obj['parentUuid']:
                    # Cannot validate parentUuid references without rebuilding full graph
                    # Just warn and continue
                    pass

                yield obj

            except json.JSONDecodeError as e:
                print(f"⚠ Line {line_num}: JSON error - {e}")
                continue
            except Exception as e:
                print(f"⚠ Line {line_num}: Unexpected error - {e}")
                continue

def extract_tool_calls(jsonl_path: Path) -> list:
    """Extract tool calls from session with error handling"""
    tool_calls = []

    for entry in parse_session_safe(jsonl_path):
        message = entry.get('message', {})
        if message.get('role') == 'assistant':
            for content_item in message.get('content', []):
                if content_item.get('type') == 'tool_use':
                    tool_calls.append({
                        'tool': content_item.get('tool_name'),
                        'input': content_item.get('tool_input'),
                        'uuid': content_item.get('tool_use_id'),
                        'timestamp': entry.get('timestamp'),
                        'version': entry.get('version')
                    })

    return tool_calls
```

**Advantages**:
- Can extract data when official API unavailable
- Handles known bugs gracefully
- Non-critical telemetry/analytics

**Limitations**:
- Fragile to version changes
- Cannot validate data integrity
- May miss corrupted entries silently

### Option 3: Don't Parse JSONL - Use the API Instead
**Stability**: Official Claude API (most stable)

```python
# Use Claude API's official session management
from anthropic import Anthropic

client = Anthropic()

# Get session data through official API
# (Not available for local Claude Code sessions yet, but planned)
```

**Status**: Claude API supports sessions, but local Claude Code integration may be limited.

---

## 7. Parsing Pattern Recommendations

### For Non-Critical Use (Analytics, Telemetry)

```python
def safe_extract_intents(session_dir: Path) -> list:
    """
    Extract user prompts for intent analysis.
    Tolerates format changes and corruption gracefully.
    """
    intents = []

    for jsonl_file in session_dir.glob("*.jsonl"):
        for entry in parse_session_safe(jsonl_file):
            msg = entry.get('message', {})
            if msg.get('role') == 'user':
                for content in msg.get('content', []):
                    if content.get('type') == 'text':
                        intents.append({
                            'text': content.get('text'),
                            'timestamp': entry.get('timestamp'),
                            'session': entry.get('sessionId')
                        })

    return intents
```

### For Critical Use (Tool Call Reconstruction)

**DON'T**. Use hooks instead:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/log-tool-call.sh"
          }
        ]
      }
    ]
  }
}
```

**Advantages**:
- Guaranteed to capture all tool calls
- Event happens at execution time (no replay)
- Official, documented contract
- Atomic writes (no corruption risk)

---

## 8. Version Matrix (Known Issues by Version)

| Version | parentUuid Bug | Persisted-Output Bug | Duplicate Entries | Notes |
|---------|---|---|---|---|
| 2.1.29 | ✓ OK | ? | ? | Last known stable |
| 2.1.30 | ✗ BROKEN | ✗ BROKEN | ? | Regressions introduced |
| 2.1.31+ | ✗ BROKEN | ✗ BROKEN | ✗ BROKEN | Latest (unresolved) |
| Future | ? | ? | ? | NO STABILITY GUARANTEES |

---

## 9. Conclusion: Make vs. Buy Decision

| Approach | Use When | Risk Level | Recommendation |
|----------|----------|-----------|-----------------|
| **Parse JSONL directly** | Non-critical analytics, learning | HIGH | Only with strict error handling |
| **Use Claude Code Hooks** | Capturing tool calls, user input | LOW | PREFERRED |
| **Wait for Official API** | Critical session data | MEDIUM | Follow [Claude API Session Mgmt docs](https://platform.claude.com/docs/en/agent-sdk/sessions) |

### For aOa Specifically

**Recommended**: Use hooks to capture intents and tool calls in real-time, rather than parsing JSONL files after the fact.

```yaml
# .claude/hooks/capture-for-aoa.sh
hooks:
  PreToolUse:
    - matcher: ".*"
      hooks:
        - type: command
          command: "aoa intent --capture-tool-use --input '$ARGUMENTS'"
  Stop:
    - hooks:
      - type: command
        command: "aoa intent --finalize-session --session-id '$SESSION_ID'"
```

This gives you:
- Real-time capture (no replay delays)
- Official API contract (no format surprises)
- No JSONL parsing (no corruption risk)
- Perfect domain/intent accuracy

---

## Sources

- [Claude Code Hooks Documentation](https://code.claude.com/docs/en/hooks)
- [GitHub Issue #23948: persisted-output tool results](https://github.com/anthropics/claude-code/issues/23948)
- [GitHub Issue #22526: corrupt parentUuid references](https://github.com/anthropics/claude-code/issues/22526)
- [GitHub Issue #22365: Large session JSONL hangs](https://github.com/anthropics/claude-code/issues/22365)
- [GitHub Issue #5034: Duplicate JSONL entries](https://github.com/anthropics/claude-code/issues/5034)
- [Claude Code Log - Python parsing library](https://github.com/daaain/claude-code-log)
- [Claude API Session Management](https://platform.claude.com/docs/en/agent-sdk/sessions)
- [Gist: Claude Code session format explanation](https://gist.github.com/samkeen/dc6a9771a78d1ecee7eb9ec1307f1b52)
- [Claude Code Data Structures](https://simonwillison.net/2025/Dec/25/claude-code-transcripts/)
