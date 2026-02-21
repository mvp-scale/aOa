# aOa Research: Session Parsing for Go Implementation

Research directory containing practical guides for parsing Claude Code session logs to extract aOa signals.

**Last Updated**: 2026-02-14

---

## Documents

### 1. session-parsing-patterns.md (PRIMARY REFERENCE)

**Complete guide to parsing Claude Code JSONL session logs**

- 893 lines, ~35KB
- Comprehensive reference implementation
- Python pseudocode (directly translatable to Go)

**Contains:**
- Event Type Catalog (user, assistant, system events)
- JSON Structure Reference (with complete examples)
- Field Mapping for aOa Signals (file access, search queries, intent)
- Parsing Pseudocode (two-pass algorithm, tool extraction, prompt cleaning)
- Session End Detection (3 methods: timeout, explicit marker, file mtime)
- Performance & Rate Limiting (ms benchmarks, buffering strategy)
- Edge Cases & Defensive Parsing (malformed JSON, missing fields, variants)
- Integration Points (hooks, batch jobs, CLI)
- Complete Example Code
- Testing & Validation

**Best for:** Understanding the full pipeline, reference implementation details

### 2. RESEARCH_SUMMARY.txt

**Executive summary of key findings**

- 140 lines
- Quick reference (no deep dives)
- 10-point findings list
- Next steps for Go implementation

**Contains:**
- Session storage locations and staleness warning
- Event types overview
- Signals available from each event type
- JSON field mappings quick reference
- Parsing patterns summary
- Session end detection methods
- Performance metrics
- Critical insight: Don't parse for real-time (use hooks instead)
- Integration points
- Sources examined
- Actionable outputs

**Best for:** Quick orientation, executive overview, next-steps checklist

### 3. session-log-format.md

**Detailed JSON schema documentation**

- Structural reference for each event type
- Field-by-field breakdown
- Storage paths and conventions

**Best for:** JSON schema validation, field reference

### 4. treesitter-ecosystem.md

**Background research on language/AST parsing** (related to broader indexing)

**Best for:** Context on how session logs relate to codebase indexing

---

## Quick Start

If you're implementing session parsing in Go:

1. **Start here**: Read RESEARCH_SUMMARY.txt (5 min)
2. **Deep dive**: Read session-parsing-patterns.md sections 1-5 (15 min)
3. **Implement**: Use "Parsing Pseudocode" section as template (30 min)
4. **Validate**: Use synthetic-sessions.jsonl as test fixture (in .context/benchmarks/fixtures/)
5. **Test**: Cross-check against services/session/metrics.py behavior

---

## Key Insights

### Critical Finding: Real-Time vs. Batch

**DON'T parse session files for real-time signals** - they are hours stale.

**Instead:**
- Real-time: Use hooks (`~/.claude/hooks/aoa-gateway.py`) to receive data via stdin
- Batch: Parse session files only for bigrams, analytics, file transitions (offline processing)

### Event Types

Four event types contain all signals:

| Type | Source | Use |
|------|--------|-----|
| `user` | User messages | Prompts, timestamps |
| `assistant` | Claude responses | Tool calls, token usage, models |
| `system` (turn_duration) | Metadata | Turn latency |
| `file-history-snapshot` | Backups | Skip (not needed) |

### Signals Extracted

From session logs, you can extract:

**File Access**:
- Absolute paths: `/home/corey/aOa/file.py`
- Partial reads: `/home/corey/aOa/file.py:100-150` (offset:limit)
- Tools: Read, Edit, Write

**Search Queries**:
- aOa commands: `aoa grep pattern`
- Glob patterns: `*.py`
- Bash commands (parsed for embedded aOa)

**Token Economics**:
- Input tokens, output tokens
- Cache creation (write cost), cache read (hit savings)
- Model used, turn duration

**Timing**:
- Timestamps (ISO 8601)
- Turn duration (ms to execute)
- Throughput (tokens/sec)

---

## Implementation Checklist for Go

From session-parsing-patterns.md section "NEXT STEPS FOR GO IMPLEMENTATION":

- [ ] JSONL line parser (handle malformed JSON gracefully)
- [ ] Two-pass parsing (collect turn_duration first, then process messages)
- [ ] Tool extraction regex (aoa commands from bash input)
- [ ] safe_get() helper (traverse nested JSON, return defaults)
- [ ] Session end detection (timeout-based recommended: 5 minutes)
- [ ] Buffering (100-200 lines or 5 second intervals)
- [ ] Performance: <50ms per session parse
- [ ] Validation against Python implementation
- [ ] Test fixture: synthetic-sessions.jsonl

---

## Sources

All research based on working implementations in aOa codebase:

**Core Implementation Files:**
- `/home/corey/aOa/services/session/metrics.py` (309 lines) - Full session parsing
- `/home/corey/aOa/services/session/reader.py` (258 lines) - Reader interface
- `/home/corey/aOa/services/ranking/session_parser.py` (660 lines) - Event extraction
- `/home/corey/aOa/.claude/hooks/aoa-gateway.py` (538 lines) - Real-time hook

**Supporting Documentation:**
- `.context/archive/2026-02-04-session-67.md` - Key insight about session staleness
- `.context/benchmarks/fixtures/synthetic-sessions.jsonl` - Test data

---

## Performance Benchmarks

From working implementation (metrics.py):

| Operation | Time | Notes |
|-----------|------|-------|
| Parse single turn | 1-5ms | Per assistant/user pair |
| Parse full session (50 turns) | 50-250ms | Sequential line iteration |
| Extract tool events | 0.5ms per tool | Regex-based |
| Validate paths | 0.1ms per path | os.path.exists() checks |
| Two-pass (durations first) | 10-20ms overhead | But enables turn filtering |

**Buffering Strategy:**
- Real-time hooks: No buffering (async POST)
- Batch parsing: 100-200 lines or 5 second intervals
- No rate limits on single session (sequential)

---

## Integration Points

### Real-Time (Hooks)

Location: `~/.claude/hooks/aoa-gateway.py`

Three hook types receive real-time data:

1. **UserPromptSubmit** - User prompts (stdin has `prompt` field)
2. **PostToolUse** - Tool calls (stdin has `tool_input`, `tool_response`)
3. **Stop** - Session ended (stdin has `transcript_path` to parse)

No parsing needed - data already extracted. POST directly to `/intent` API (async).

### Batch (Background Jobs)

Location: `services/ranking/session_parser.py`

Runs periodically to extract:
- File transition matrices (for prediction prefetching)
- Bigrams (for semantic domain learning)
- Usage statistics

Parses all session files, builds transitions, syncs to Redis.

### CLI (Inspection)

Location: `services/session/metrics.py` (SessionMetrics class)

Commands:
```bash
aoa cc sessions     # View recent sessions
aoa cc prompts      # View user prompts
aoa cc stats        # View token usage stats
```

---

## Edge Cases & Defensive Patterns

From session-parsing-patterns.md section "Edge Cases & Defensive Parsing":

**Malformed JSON**
- Every `json.loads()` wrapped in try/catch
- Skip malformed lines, don't crash
- Log errors but continue

**Missing Fields**
- Use `safe_get(dict, *keys, default=None)` for nested access
- Return sensible defaults for missing fields

**Timestamps**
- ISO 8601 format with timezone suffix (Z)
- Handle variations: `replace("Z", "+00:00")`

**File Paths**
- Validate absolute paths only (start with /)
- Reject suspiciously long paths (>500 chars)
- Handle pattern variations (globs, relative paths)

**Tool Name Variations**
- Normalize tool names (Read, Edit, Write, etc.)
- Create aliases for version variations

---

## Testing

### Unit Test Example

```python
def test_parse_real_session():
    from services.session.metrics import SessionMetrics

    metrics = SessionMetrics("/home/corey/aOa")
    session_files = metrics._get_session_files(limit=1)

    if not session_files:
        pytest.skip("No session files found")

    session = metrics.parse_session(session_files[0])

    assert session["session_id"]
    assert isinstance(session["prompts"], list)
    assert isinstance(session["turns"], list)
    assert session["total_input"] >= 0
    assert session["error"] is None
```

### Test Fixture

Location: `/home/corey/aOa/.context/benchmarks/fixtures/synthetic-sessions.jsonl`

11 synthetic sessions with:
- Complete event sequences
- Multiple tools (Read, Grep, Edit, Write, Glob)
- Token usage (input/output/cache)
- Expected results (ground_truth)

Use for validation before testing on real sessions.

---

## Questions & Decision Points

### Q: Why two-pass parsing?

**A**: Turn duration events come from `system` events with `parentUuid` references. Collecting them first (pass 1) ensures they're available when processing assistant messages (pass 2). Enables filtering incomplete turns.

### Q: Why not stream real-time from session files?

**A**: Session JSONL files are stale (Claude doesn't flush in real-time). Can be hours behind actual conversation. Hooks receive data via stdin immediately, making them reliable for real-time signals.

### Q: How do we detect session end?

**A**: Timeout-based (5 minutes without new messages) is recommended. File modification time can also work. Explicit markers not yet documented by Claude.

### Q: What about large files?

**A**: Tool input includes `offset` and `limit` fields for partial reads. aOa hooks capture the range (`file:offset-limit`). Files larger than 2MB can be read partially.

### Q: How fast can we parse?

**A**: ~1-5ms per turn (assistant + user pair). Full 50-turn session: 50-250ms. Two-pass adds 10-20ms overhead. Acceptable for background jobs.

---

## Next Phase

Once Go implementation is complete:

1. Validate against synthetic-sessions.jsonl
2. Test on real session files from ~/.claude/projects/
3. Measure latency (target: <50ms per session)
4. Integrate with aOa services (POST to /intent API)
5. Compare behavior against Python implementation

Then integrate with:
- Real-time hook system (for Go-based hooks)
- Background job queue (for batch enrichment)
- CLI inspection commands (for Go-based tools)

---

## References

**Document Index:**
1. session-parsing-patterns.md - Primary reference (893 lines)
2. RESEARCH_SUMMARY.txt - Executive summary (140 lines)
3. session-log-format.md - JSON schema reference
4. treesitter-ecosystem.md - Related research on language parsing

**Key Files:**
- Source implementations: services/session/, .claude/hooks/
- Test fixtures: .context/benchmarks/fixtures/synthetic-sessions.jsonl
- Documentation: .context/archive/2026-02-04-session-67.md
