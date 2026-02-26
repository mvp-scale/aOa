# Throughput & Conversation Speed Telemetry Model

> How aOa captures and calculates session throughput metrics.
> Written 2026-02-26 during implementation of tool result size tracking.

---

## Part 1: Claude Code's Data Hierarchy

Claude Code writes to `~/.claude/` with this structure. Understanding this is
prerequisite to capturing complete telemetry.

### File Tree

```
~/.claude/
├── projects/{encoded-path}/          # Per-project, contains sessions
│   └── {sessionId}/                  # UUID-based directory per session
│       ├── {sessionId}.jsonl         # PRIMARY: main conversation transcript
│       ├── subagents/                # Parallel agent forks
│       │   └── agent-{agentId}.jsonl # Each subagent gets its own JSONL
│       └── tool-results/             # Large tool outputs stored separately
│           └── toolu_{toolId}.txt    # Raw tool output text
│
├── file-history/{sessionId}/         # File version tracking
│   └── {hash}@v1, @v2, ...          # Edit snapshots per file
│
├── tasks/{sessionId}/                # Task tracking per session
├── todos/{sessionId}-agent-{agentId}.json  # Todo lists per agent
├── plans/{slug}.md                   # Named plans (whimsical slugs)
├── debug/{sessionId}/                # Debug output
├── session-env/{sessionId}/          # Environment snapshots
├── history.jsonl                     # Global cross-session message log
├── stats-cache.json                  # Pre-computed usage statistics
└── settings.json                     # User configuration
```

### What aOa currently reads

```
~/.claude/projects/{encoded-path}/{sessionId}.jsonl   ← ONLY THIS
```

We read one file — the main session JSONL. We do NOT currently read:
- `subagents/agent-*.jsonl` — subagent transcripts (Task tool forks)
- `tool-results/toolu_*.txt` — persisted large tool outputs
- `file-history/` — file edit snapshots
- `history.jsonl` — cross-session history

### What each source contains

| Source | Content | Volume | Currently Captured |
|--------|---------|--------|--------------------|
| Main JSONL | User text, assistant text, thinking, tool_use, tool_result, usage, timestamps | 500KB–12MB per session | Yes (tailer) |
| Subagent JSONL | Full conversation of spawned agents (Task tool) | 100KB–2MB per agent | No — only the final result text |
| tool-results/*.txt | Raw output when tool results exceed inline size | Varies (can be large) | No |
| file-history/ | File snapshots at each edit | 47MB total across all sessions | No |
| stats-cache.json | Daily activity, token usage by model | Small, pre-aggregated | No |

---

## Part 2: What a Session Looks Like in JSONL

A typical multi-turn exchange produces this sequence of JSONL lines:

```
Line 1: {"type":"user",      "message":{"role":"user",      "content":[{type:"text", text:"Fix the auth bug"}]}}
Line 2: {"type":"assistant",  "message":{"role":"assistant",  "content":[
           {type:"thinking", thinking:"Let me look at the auth module..."},
           {type:"text",     text:"I'll check the auth handler."},
           {type:"tool_use", id:"toolu_abc", name:"Read", input:{file_path:"/src/auth.go"}}
         ], "usage":{"input_tokens":1200, "output_tokens":85, "cache_read_input_tokens":98000}}}
Line 3: {"type":"user",      "message":{"role":"user",      "content":[
           {type:"tool_result", tool_use_id:"toolu_abc", content:"package auth\n\nfunc Login()..."}
         ]}}
Line 4: {"type":"assistant",  "message":{"role":"assistant",  "content":[
           {type:"thinking", thinking:"The bug is on line 42..."},
           {type:"text",     text:"Found it. The token expiry check is inverted."},
           {type:"tool_use", id:"toolu_def", name:"Edit", input:{file_path:"/src/auth.go", ...}}
         ], "usage":{"input_tokens":800, "output_tokens":120, ...}}}
Line 5: {"type":"user",      "message":{"role":"user",      "content":[
           {type:"tool_result", tool_use_id:"toolu_def", content:"File edited successfully."}
         ]}}
Line 6: {"type":"system",    "subtype":"turn_duration", "durationMs":14500, "parentUuid":"..."}
```

### What aOa extracts from each line

```
Line 1 (user):
  UserText = "Fix the auth bug"                          → 16 chars

Line 2 (assistant):
  ThinkingText = "Let me look at the auth module..."     → 38 chars
  AssistantText = "I'll check the auth handler."         → 30 chars
  ToolUse: name="Read", id="toolu_abc"
  Usage: output_tokens=85

Line 3 (user with tool_result):
  tool_use_id = "toolu_abc"
  content = "package auth\nfunc Login()..."              → 1,842 chars (JUST the text)
  ToolResultSizes = {"toolu_abc": 1842}

Line 4 (assistant):
  ThinkingText = "The bug is on line 42..."              → 25 chars
  AssistantText = "Found it. The token expiry..."        → 50 chars
  ToolUse: name="Edit", id="toolu_def"
  Usage: output_tokens=120

Line 5 (user with tool_result):
  tool_use_id = "toolu_def"
  content = "File edited successfully."                  → 25 chars
  ToolResultSizes = {"toolu_def": 25}

Line 6 (system):
  DurationMs = 14500
```

**Key point**: The `content` field in a tool_result is extracted as raw text length.
No JSON keys, no braces, no `tool_use_id` — just the actual output string.
When content is an array of blocks, we sum only the `text` fields.

---

## Part 3: Current Calculations

### Conversation Speed (visible dialogue only)

```
totalTextChars = 0
for each turn:
    totalTextChars += len(turn.text)            # user prompts + assistant responses
    totalTextChars += len(turn.thinking_text)    # model reasoning

convTokens = totalTextChars / 4
conversationSpeed = convTokens / elapsedSeconds
```

### Throughput (all content flowing through the session)

```
textBasedTokens = totalTextChars / 4
outputTokens    = sum of API usage.output_tokens across turns
resultTokens    = sum of tool result_chars across all actions / 4

throughputTokens = max(outputTokens, textBasedTokens) + resultTokens
throughput       = throughputTokens / elapsedSeconds
```

### The Elapsed Time Problem

Denominator is session wall time — includes idle time (thinking, AFK, lunch).

```
elapsedSec priority:
  1. rw.total_duration_ms / 1000    (from status line hook — most accurate)
  2. now - session_start_ts          (from first usage event)
  3. max(timestamp) - min(timestamp) (from turn timestamps — fallback)
```

Both metrics drop during idle periods. A session with 10K tokens of work
spread over 2 hours shows ~1.4 tok/s even though bursts were much faster.

---

## Part 4: What's Captured vs What's Missing

### Currently captured

| Stream | Source | Measurement | Accuracy |
|--------|--------|-------------|----------|
| User text | main JSONL `text` blocks | `len(text)` chars | Exact (cleaned of system tags) |
| Assistant text | main JSONL `text` blocks | `len(text)` chars | Exact |
| Thinking text | main JSONL `thinking` blocks | `len(thinking)` chars | Exact |
| Tool results | main JSONL `tool_result.content` | `len(content)` chars | Exact for inline results |
| API output tokens | main JSONL `usage.output_tokens` | Integer from API | Exact (real tokenizer) |
| Turn duration | main JSONL `system` events | `durationMs` | Exact |
| Turn timestamps | main JSONL `timestamp` field | ISO 8601 | Exact |

### NOT captured — the gaps

| Stream | Where it lives | Why it matters |
|--------|---------------|----------------|
| **Subagent content** | `subagents/agent-*.jsonl` | Task tool spawns full conversations — we only see the final result string, not the agent's internal reads/greps/thinking |
| **Persisted tool results** | `tool-results/toolu_*.txt` | When tool output exceeds inline size, Claude stores it here — we measure 0 chars for these |
| **File edit diffs** | `file-history/{hash}@v{N}` | We know an Edit happened but not how many chars changed |
| **Plan mode content** | `plans/{slug}.md` | Planning text that doesn't flow through the main JSONL |
| **Idle-adjusted timing** | Derivable from turn timestamps | Active work rate vs wall clock rate |
| **Per-turn rates** | Derivable from turn data | Individual turn velocity, not just session average |

---

## Part 5: Proposed Unified Telemetry Capture

### Principle

Capture raw character counts with timestamps at the point of extraction.
Never convert to tokens at capture time — store chars and let display convert.
Every measurement is a `(chars, timestamp)` pair.

### The ContentMeter

A single accumulator struct, one per session, recording every content stream:

```go
// ContentMeter captures raw character counts with timestamps for every
// content stream in a session. The universal unit is characters.
// Conversion to tokens (÷4) or any other unit happens at display time only.
type ContentMeter struct {
    // --- Conversation streams (visible dialogue) ---
    UserChars      int64     // total chars of user prompt text
    UserLastTs     time.Time // when last user text was captured

    AssistantChars int64     // total chars of assistant response text
    AssistantLastTs time.Time

    ThinkingChars  int64     // total chars of model reasoning
    ThinkingLastTs time.Time

    // --- Tool streams (infrastructure) ---
    ToolResultChars   int64     // total chars of inline tool results
    ToolResultLastTs  time.Time
    ToolResultCount   int       // number of tool results captured

    ToolPersistedChars int64    // total chars from tool-results/*.txt files
    ToolPersistedLastTs time.Time

    // --- Agent streams (subagent forks) ---
    SubagentChars     int64     // total chars from subagent JSONL transcripts
    SubagentLastTs    time.Time
    SubagentCount     int       // number of subagent sessions

    // --- Edit streams (code changes) ---
    EditChars         int64     // total chars of edit diffs (old + new)
    EditLastTs        time.Time

    // --- API-reported tokens (not chars — kept separate) ---
    APIOutputTokens   int       // sum of usage.output_tokens
    APIInputTokens    int       // sum of usage.input_tokens
    APICacheReadTokens int      // sum of cache_read tokens

    // --- Timing ---
    SessionStart      time.Time // first event timestamp
    ActiveMs          int64     // sum of turn durations (excludes idle)
    TurnCount         int       // number of completed turns

    // --- Per-turn snapshots (ring buffer, last N turns) ---
    Turns []TurnSnapshot
}

type TurnSnapshot struct {
    Timestamp       time.Time
    DurationMs      int
    UserChars       int   // chars of user text this turn
    AssistantChars  int   // chars of assistant text this turn
    ThinkingChars   int   // chars of thinking this turn
    ToolResultChars int   // chars of tool results this turn
    SubagentChars   int   // chars from subagents this turn
    APIOutputTokens int   // API-reported output tokens this turn
    ActionCount     int   // number of tool actions this turn
}
```

### Derived metrics at display time

All derived from the raw char counts. Never stored — always computed fresh:

```
# Conversation Speed (dialogue only)
convChars = UserChars + AssistantChars + ThinkingChars
conversationSpeed = (convChars / 4) / (wallTimeSeconds)

# Throughput (all content)
textTokens   = convChars / 4
resultTokens = (ToolResultChars + ToolPersistedChars + SubagentChars) / 4
throughput   = (max(APIOutputTokens, textTokens) + resultTokens) / wallTimeSeconds

# Burst Throughput (active work only, excludes idle)
burstThroughput = throughputTokens / (ActiveMs / 1000)

# Per-Turn Velocity (from TurnSnapshot ring)
for each turn:
    turnChars = turn.UserChars + turn.AssistantChars + turn.ThinkingChars + turn.ToolResultChars
    turnRate  = (turnChars / 4) / (turn.DurationMs / 1000)

# Content Mix (what fraction of throughput is tool results vs dialogue)
dialoguePct = convChars / (convChars + ToolResultChars + SubagentChars)
toolPct     = 1 - dialoguePct
```

### What changes in the pipeline

```
Current:
  main JSONL → tailer → reader → app (partial char counts) → dashboard

Proposed:
  main JSONL ──────────────┐
  subagent JSONLs ─────────┼──→ tailer → reader → app.ContentMeter → dashboard
  tool-results/*.txt ──────┤                        (raw chars + timestamps)
  file-history/ ───────────┘
```

The tailer already watches for new JSONL files. Extending it to also:
1. Scan `subagents/` directory for new agent-*.jsonl files
2. Measure `tool-results/toolu_*.txt` file sizes when they appear
3. Optionally diff `file-history/` snapshots

### Implementation phases

**Phase 0 (done)**: Inline tool result chars from main JSONL. This is what we
shipped today — captures ~80% of tool result volume.

**Phase 1**: Add `ContentMeter` struct to app.go. Wire existing char captures
into it (UserChars, AssistantChars, ThinkingChars, ToolResultChars, API tokens).
Add TurnSnapshot ring. Add ActiveMs from turn durations. No new data sources —
just restructuring what we already capture into the unified struct.

**Phase 2**: Read `tool-results/*.txt` file sizes. When the tailer sees a
tool_result with a `tool_use_id` but 0 inline content, check if
`tool-results/toolu_{id}.txt` exists and measure its size. Fills the
ToolPersistedChars gap.

**Phase 3**: Tail subagent JSONLs. Extend the tailer to discover and read
`subagents/agent-*.jsonl` files. Apply the same parser. Sum all content
into SubagentChars. This captures the full internal work of Task tool calls.

**Phase 4**: Burst throughput and per-turn velocity. Use ActiveMs (sum of
turn durations) as denominator for burst rate. Populate TurnSnapshot ring
for per-turn sparkline visualization.

---

## Part 6: Tool Call Capture & Counterfactual Shadow

### The Value Pattern

aOa replaces standard tool calls (G3: agents never know it's not GNU grep).
When it does, it's more efficient. The value proof (G6) requires showing:

> "This tool call cost X tokens. With aOa, it would have cost Y tokens. Delta = X - Y."

We already do this for **reads** — `readSavings()` compares a full file read
(counterfactual) against an aOa-guided partial read (actual). The result:
`counterfactTokensSaved`, `burnRateCounterfact`, runway delta.

The gap: we don't do this for **grep, find, glob, or bash commands**.

### What we already capture per tool call

From the JSONL `tool_use` block, the tailer extracts:

```
ToolUse {
    Name:     "Grep"                           // tool type
    ID:       "toolu_abc123"                   // for result correlation
    FilePath: "/home/corey/aOa-go/internal"    // search path
    Pattern:  "func.*Search"                   // search pattern
    Command:  ""                               // (for Bash tools)
    Offset:   0                                // (for Read tools)
    Limit:    0                                // (for Read tools)
}
```

And from the correlated `tool_result`, we now capture `ResultChars = 32,847`.

So we know: **Grep for "func.*Search" in internal/ returned 32,847 chars.**

### What we DON'T capture yet

The counterfactual: **What would aOa have returned for the same query?**

### The Shadow Pattern

When a tool call arrives that aOa can serve, run the aOa equivalent in a
background goroutine. Don't block the main pipeline. Just measure the output size.

```
Tool call arrives: Grep "func.*Search" in internal/

Main pipeline (unchanged):
  → record ResultChars = 32,847 from Claude's tool_result

Shadow thread (new, async, non-blocking):
  → run aOa search("func.*Search", {path: "internal/"})
  → measure output: 847 chars (precise symbol hits, no noise)
  → record: ShadowChars = 847

Counterfactual delta: 32,847 - 847 = 32,000 chars saved
                      = ~8,000 tokens saved per grep call
```

### Which tool calls can be shadowed

| Tool | Claude's approach | aOa equivalent | Shadowable? |
|------|-------------------|----------------|-------------|
| **Grep** | `rg` over raw files, returns matching lines with context | O(1) index lookup, returns symbols + domains | Yes — direct comparison |
| **Glob** | File system glob, returns path list | Index file lookup | Yes — direct comparison |
| **Read** (full file) | Reads entire file into context | Guided partial read via index knowledge | Yes — already doing this (readSavings) |
| **Read** (partial) | Reads N lines from offset | Same but with smarter offset/limit | Yes — already doing this |
| **Bash** (grep/find/locate) | Shell command, raw output | Parse the command, route to aOa search | Partially — need to parse bash commands |
| **Bash** (other) | Arbitrary shell commands | N/A | No — can't shadow arbitrary commands |
| **Edit/Write** | File modifications | N/A | No — these are outputs, not queries |
| **Task** | Subagent spawning | N/A | No — but can shadow the subagent's internal tool calls |

### The ToolShadow struct

```go
// ToolShadow records one tool call with its actual and counterfactual result sizes.
type ToolShadow struct {
    Timestamp     time.Time
    ToolName      string    // "Grep", "Glob", "Read", "Bash"
    ToolID        string    // for correlation with result

    // What the tool call asked for
    Pattern       string    // grep/glob pattern
    FilePath      string    // target path
    Command       string    // bash command (if applicable)

    // What actually happened
    ActualChars   int       // chars returned by Claude's tool execution

    // What aOa would have returned (filled async by shadow thread)
    ShadowChars   int       // chars aOa's equivalent would return
    ShadowRan     bool      // true if shadow was executed
    ShadowMs      int64     // how long the shadow took

    // The delta
    CharsSaved    int       // ActualChars - ShadowChars (positive = savings)
    TokensSaved   int       // CharsSaved / 4
}
```

### Ring buffer, not unbounded

Store the last 100 shadows in a ring buffer (same pattern as activity feed).
Aggregate totals into the ContentMeter. Dashboard shows:
- Per-tool-call shadow comparison in the Actions table
- Running total of counterfactual savings
- "aOa would have saved X tokens on this Grep" inline

### Implementation phases

**Phase 0 (done)**: Capture `ResultChars` per tool call. We have the actual cost.

**Phase 1**: Capture full tool call details (Pattern, FilePath, Command) in
`TurnAction` and persist through to the dashboard. We already extract these
in the tailer — just need to thread them through to `TurnActionResult`.

**Phase 2**: Shadow grep/glob calls. When a Grep or Glob tool invocation
arrives with a pattern, dispatch an async aOa search with the same query.
Measure output size. Store in ToolShadow. Non-blocking — the main pipeline
never waits for the shadow.

**Phase 3**: Shadow bash grep/find commands. Parse common bash patterns:
`grep -r "pattern" path`, `find . -name "*.go"`, `rg pattern path`.
Route to aOa's search/files equivalents. Harder — requires command parsing.

**Phase 4**: Dashboard integration. Show shadow comparison in the Actions
table. Add a "Savings" column: `32K → 847 (↓97%)`. Roll up into session
totals: "aOa could save ~40K tokens/session on grep alone."

### Connection to existing counterfactual system

The shadow pattern extends what `readSavings()` already does:

```
Current (reads only):
  Read tool_use → readSavings(path, limit) → counterfactTokensSaved

Extended (all queryable tools):
  Any tool_use → shadowRun(tool) → ToolShadow.TokensSaved → counterfactTokensSaved
```

The `burnRateCounterfact` tracker, `RunwayProjection().DeltaMinutes`, and
`TokensSaved` all benefit from more accurate counterfactual data. Currently
they only reflect read savings. With shadows, they reflect ALL tool savings.

---

## Part 7: Correlation Pipeline (current state, Phase 0)

```
JSONL tool_result block
  → tailer/parser.go: extracts content text length → ToolResultSizes[tool_use_id]
    → claude/reader.go: emits EventToolResult with sizes map
      → app.go: correlates tool_use_id back to TurnAction → sets ResultChars
        → ConversationTurns(): copies ResultChars into TurnActionResult
          → socket/protocol.go: serializes as "result_chars" in JSON
            → app.js: sums result_chars across all actions for throughput calc
```

---

## Part 8: Real Session Data (this session, 2026-02-26)

At time of writing:
- Session wall time: ~32,662 seconds (~9 hours, includes prior work)
- API output_tokens: 1,721
- Text chars (user + assistant + thinking): ~3,455
- Tool result chars: ~48,045
- Text-based tokens: 863
- Result tokens: 12,011
- throughputTokens = max(1721, 863) + 12011 = 13,732
- Throughput: 13,732 / 32,662 = **0.4 tok/s** (diluted by 9 hours of wall time)

The low number reflects the idle time problem. During active work the burst rate
is dramatically higher — the 48K chars of tool results arrived in ~5 minutes of
actual tool execution.
