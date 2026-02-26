# GH Telemetry Review: Layer 9 Architecture Analysis

> Written 2026-02-26. Analysis of coupling risk, hidden value signals,
> counterfactual expansion, business moat, and risk mitigation for L9.

---

## Problem

Layer 9 (Telemetry) is tapping deeper into Claude Code's undocumented
JSONL session format. The existing Session Prism (`internal/adapters/claude/reader.go`)
normalizes raw events to canonical `ports.SessionEvent` types. L9.0 is shipped.
L9.1-L9.8 extend into new territory: subagent JSONLs, persisted tool results,
burst throughput, and counterfactual shadowing. Each step increases coupling
surface to an unstable external format.

The question: How do we extract maximum value from session data while
building resilience against format changes?

## Goal Alignment

| Goal | Impact | Rationale |
|------|--------|-----------|
| G0 | = | No hot-path impact. Telemetry is async/background. Shadow engine must not block search. |
| G1 | = | Not applicable. Telemetry doesn't affect search parity. |
| G2 | = | All L9 work lives in the `aoa` binary. No new binary needed. |
| G3 | = | Telemetry is invisible to agents. No interface change. |
| G4 | + | ContentMeter consolidation improves architecture (scattered fields -> single struct). |
| G5 | + | Richer signals from subagents and tool details feed the learner. |
| G6 | ++ | This IS the value proof layer. Every L9 task directly serves G6. |

No goal conflicts. L9 is clean.

---

## 1. Abstraction Layer Analysis

### What You Already Have

The Session Prism is a two-layer abstraction:

```
Layer 1: tailer/parser.go    (raw JSONL -> tailer.SessionEvent)
Layer 2: claude/reader.go    (tailer.SessionEvent -> ports.SessionEvent)
```

Layer 1 (parser.go) is where format coupling lives. It uses `map[string]any`
extraction with multi-path field resolution (`getStringAny(block, "name", "tool_name")`).
This is already the right defensive pattern. It never crashes on unknown input.
It tries multiple field names for the same concept.

Layer 2 (reader.go) translates Claude-specific concepts to agent-agnostic
canonical events. This is a clean adapter pattern. The `ports.SessionEvent`
type is genuinely agent-agnostic (it says so in the doc comment and the
design reflects it).

### Is It Sufficient for L9?

**For L9.1-L9.2 (ContentMeter, tool details)**: Yes. These restructure
data already flowing through the pipeline. No new format coupling.

**For L9.3 (persisted tool results)**: Partial. Reading `tool-results/toolu_*.txt`
is a new coupling point -- not to the JSONL format, but to the directory
layout. This is a different kind of contract.

**For L9.4 (subagent tailing)**: This is the highest-risk extension. You're
opening a second (and potentially Nth) JSONL stream with the same parser.
The parser handles it, but the tailer (`tailer.go`) currently only watches
one file. Extending it means the tailer needs to manage multiple file cursors.

**For L9.5-L9.6 (shadow engine)**: No additional format coupling. The shadow
engine runs aOa's own search -- it consumes the tool call details already
extracted and runs local code. Clean.

### What's Missing: A Directory Contract Layer

The gap is not in JSONL parsing -- that's well-defended. The gap is in
**directory layout assumptions**. Currently hardcoded across multiple files:

```
~/.claude/projects/{encoded-path}/          # tailer.SessionDirForProject()
{sessionDir}/{sessionId}.jsonl              # tailer.findLatestJSONL()
```

L9.3 adds: `{sessionDir}/tool-results/toolu_{id}.txt`
L9.4 adds: `{sessionDir}/subagents/agent-{agentId}.jsonl`

These path patterns should be centralized. Not a full abstraction -- just
a single place that knows Claude's directory layout:

```go
// internal/adapters/claude/paths.go

// SessionLayout describes the directory structure of a Claude Code session.
// Centralizes all path assumptions about Claude's undocumented layout.
type SessionLayout struct {
    SessionDir string  // base: ~/.claude/projects/{encoded-path}/
}

func (l *SessionLayout) MainJSONL(sessionID string) string {
    return filepath.Join(l.SessionDir, sessionID+".jsonl")
}

func (l *SessionLayout) ToolResultFile(toolUseID string) string {
    return filepath.Join(l.SessionDir, "tool-results", toolUseID+".txt")
}

func (l *SessionLayout) SubagentGlob() string {
    return filepath.Join(l.SessionDir, "subagents", "agent-*.jsonl")
}

func (l *SessionLayout) LatestJSONL() (string, error) {
    // Current findLatestJSONL logic, but also returns session-scoped subdirs
}
```

This is the minimum viable abstraction. It:
1. Centralizes every path assumption in one file
2. Makes format changes grep-able (search `SessionLayout`)
3. Costs almost nothing to build or maintain
4. Does not over-abstract (no interfaces, no indirection)

**Recommendation**: Create `internal/adapters/claude/paths.go` before L9.3.
Move `SessionDirForProject()` from `tailer.go` into it. Add the new path
methods as L9.3 and L9.4 need them. One file, one responsibility.

### The ExtractionHealth System Is Your Canary

You already have `ports.ExtractionHealth` with `IsHealthy()`, gap counting,
unknown type tracking, and version change detection. This is excellent.

What it doesn't cover yet: **per-field extraction health**. Right now you
track "we got N text events" but not "we successfully extracted ToolResultSizes
from N of M tool_result blocks." For L9, add field-level yield counters:

```go
// In ExtractionHealth, add:
ToolResultYield   int  // tool_results where we extracted content size
ToolResultMissed  int  // tool_results where content was 0 or absent
```

This lets you detect the specific moment when Claude Code changes how
tool results are encoded -- before it silently breaks your throughput
metrics.

---

## 2. Hidden Value Signals

Looking at the full data hierarchy (main JSONL, subagent JSONLs, tool-results,
file-history, plans, tasks, todos), here are signals beyond throughput/tokens
that would make users say "I can't work without this":

### Tier 1: High-Value, Low-Effort (data already in the pipeline)

**A. Tool Choice Efficiency Score**

You already track every tool invocation in `ToolMetrics`. What you don't
surface: tool choice quality.

Pattern: If Claude does `Read` (full file, 5000 tokens) then immediately
`Read` (same file, 50 lines), that's a self-correction. The first read was
wasteful. Track these patterns:

```
- Read full -> Read partial (same file): self-correction, wasted tokens
- Grep broad -> Grep narrow (same pattern family): refinement chain
- Read -> Edit (same file, within 2 turns): productive pair
- Read -> no follow-up: exploratory (potentially wasted)
- Bash grep -> Grep tool (same pattern): tool switch (agent learning)
```

This produces a "Tool Efficiency" metric: what fraction of tool calls
led to productive follow-ups vs dead ends? Users who see "73% of your
reads led to edits" understand their workflow efficiency.

Implementation: Already possible from `ConversationTurn` ring buffer data.
Scan consecutive turns, match file paths and patterns, classify pairs.

**B. Context Window Utilization Curve**

You have `ContextSnapshot` from the hook with `CtxUsed`/`CtxMax` over time.
You have turn timestamps and token counts. Combine them:

```
Turn 1: 5% context used, 120 output tokens
Turn 15: 42% context used, 85 output tokens
Turn 30: 78% context used, 45 output tokens (degrading)
Turn 35: 92% context used, 12 output tokens (compressed, near-death)
```

The insight: **at what context utilization does output quality degrade?**
If output tokens per turn drops as context fills, you can predict the
"productivity cliff" -- the point where the session becomes inefficient
and should be restarted.

Display: "You have ~12 productive turns remaining" (not just "X minutes
runway" but "X quality turns").

This is the kind of signal that would make a power user say "I can't
work without knowing this."

**C. Thinking-to-Action Ratio**

You capture ThinkingChars and tool action counts per turn. The ratio
`ThinkingChars / ActionCount` tells you how much reasoning the model
does per action.

- Low ratio (short thinking, many actions) = confident, efficient work
- High ratio (long thinking, few actions) = uncertain, exploring
- High ratio + no actions = stuck (model is spinning)

Surface this as a "Confidence" indicator. When the model starts thinking
more and acting less, the user knows the task is getting harder and
might need intervention.

### Tier 2: Medium-Value, Medium-Effort (new data sources)

**D. Subagent Work Amplification (L9.4)**

When you tail subagent JSONLs, don't just sum chars. Track the
amplification factor:

```
Task tool invocation: "Research how X handles Y"
  Main conversation: 500 chars (the request + summary)
  Subagent work: 45,000 chars (reads, greps, thinking)
  Amplification: 90x
```

This proves that Task tool calls are expensive. If aOa could intercept
and serve the subagent's internal greps/reads, the savings multiply.
The amplification factor is the value proposition for "aOa inside
subagents."

**E. File Hotspot Map**

You already track `FileReads map[string]int`. Extend to a heatmap:

```
internal/app/app.go:       Read(12), Edit(4), Grep-hit(8)  = 24 touches
internal/domain/index/:    Read(3), Grep-hit(15)           = 18 touches
test/fixtures/:            Read(7)                          = 7 touches
```

The insight: which files are the session's gravity wells? These are
candidates for pre-loading, pre-caching, and priority indexing. The
learner already has file_hits -- surface this as "Session Focus Areas"
on the dashboard.

**F. Edit Velocity (from file-history)**

File-history snapshots (`~/.claude/file-history/{sessionId}/{hash}@v{N}`)
tell you how many versions of each file were created. `v1 -> v2 -> v3`
means 3 edit rounds. Compute:

```
Files edited: 8
Average versions per file: 2.3
Files with >3 versions: 2 (likely bugs or iteration)
Total lines changed: 847
```

This gives a "session productivity" metric that's more meaningful than
token throughput -- it measures actual code output.

### Tier 3: High-Value, High-Effort (requires new infrastructure)

**G. Cross-Session Pattern Recognition**

You persist session summaries in bbolt. Over time, you accumulate:
- Which tools are used most per project
- Which files are read/edited most
- Average session length before context exhaustion
- Common grep patterns (from `GrepPatterns map[string]int`)

Cross-session analysis: "In the last 10 sessions, you edited `app.go`
in 9 of them. Average session length has been decreasing. Your most
common grep is `func.*Search` -- aOa's index has that answer in 0.8ms
vs grep's 32K token response."

This is the moat (see Section 4). The more sessions you observe, the
more personalized the insights become.

---

## 3. Counterfactual Pattern Expansion

### Current State

`readSavings()` computes: "this partial read cost X tokens, a full file
read would have cost Y tokens, delta = Y - X." It feeds
`counterfactTokensSaved` and `burnRateCounterfact`.

This only covers Read tools with `limit > 0 && limit < 500`.

### L9.5: Grep/Glob Shadow (Already Designed)

The telemetry model describes this well. Async shadow, non-blocking,
ToolShadow ring buffer. The design is sound. One addition:

**Shadow should also measure result relevance, not just size.**

If Claude's Grep returns 32K chars and aOa returns 847 chars, the savings
are clear. But if aOa's 847 chars contain the same file+line hits that
Claude's agent then Reads, that's a quality proof. Track:

```go
type ToolShadow struct {
    // ... existing fields ...

    // Quality signal: did the shadow's results cover what Claude
    // actually used next? (filled retroactively from next turn's Reads)
    ShadowCoverage float64  // 0.0-1.0: fraction of next-turn reads
                             // that appeared in shadow results
}
```

When the next turn does `Read(file_from_grep_results)`, check if that
file appeared in the shadow results. If yes, increment coverage. This
turns the counterfactual from "we'd return fewer bytes" to "we'd return
the RIGHT fewer bytes."

### Making the Counterfactual More Universal

The REAL value proof is: "what would this entire session have cost without aOa?"

You can't shadow every tool call (edits, writes, bash). But you CAN
extrapolate from what you shadow:

```
Shadowed tool calls: 47 (grep + glob + read)
Average savings per shadowed call: 73%
Total shadowed savings: 156K tokens

Non-shadowable calls: 23 (edit, write, bash, task)
Non-shadowable token cost: 12K tokens (known from tool results)

Session total tokens: 450K
Shadowable fraction: 438K / 450K = 97%
Extrapolated savings: 156K tokens = 35% of session

Display: "aOa saved ~156K tokens this session (35% reduction)"
```

The key insight: **most session tokens are in reads, greps, and tool
results** -- all shadowable. Edits and writes are tiny by comparison.
You don't need to shadow everything to have a credible total.

Implementation: This is just arithmetic on existing data. The ContentMeter
gives you total chars by stream. The shadow engine gives you savings on
shadowable calls. The ratio gives you session-level value proof.

### L9.6 Risk Assessment (Bash Shadow)

Bash command parsing is fragile. Instead of a full bash parser, use a
simple regex extractor for the 5 most common patterns:

```go
var bashShadowPatterns = []*regexp.Regexp{
    regexp.MustCompile(`^grep\s+(-[a-zA-Z]*\s+)*"?([^"]+)"?\s+(.+)`),
    regexp.MustCompile(`^rg\s+(-[a-zA-Z-]*\s+)*"?([^"]+)"?\s+(.+)`),
    regexp.MustCompile(`^find\s+(\S+)\s+-name\s+"?([^"]+)"?`),
    regexp.MustCompile(`^find\s+(\S+)\s+-type\s+f`),
    regexp.MustCompile(`^cat\s+(.+)`),
}
```

If it matches, shadow it. If not, skip. Never crash on parse failure.
This catches 80% of bash search patterns with 20% of the complexity.

Mark L9.6 as confidence yellow -- it's worth doing but won't be
comprehensive, and that's fine.

---

## 4. Business Moat

### The Data Flywheel

L9 creates a data flywheel that's hard to replicate:

```
Session data -> ContentMeter -> value metrics
                                    |
                            Dashboard (visible savings)
                                    |
                            User retention (keeps using aOa)
                                    |
                            More session data (across sessions)
                                    |
                            Cross-session patterns (G5)
                                    |
                            Personalized insights (moat deepens)
```

**The moat is not the parser or the shadow engine.** Those are replicable.
The moat is the **accumulated session data and the patterns extracted
from it.** After 100 sessions, aOa knows:
- Your project's file access patterns
- Your most common search queries
- Your context window utilization patterns
- Your tool efficiency profile
- Your productivity cliff point

A competitor starting from zero has none of this.

### Three Moat Layers

**Layer 1: Switching Cost (Immediate)**

If users rely on the dashboard's throughput, runway, and savings metrics,
switching to a competitor means losing session history and calibrated
projections. The ContentMeter's historical data is the switching cost.

**Layer 2: Learning Advantage (G5)**

The learner's observe/autotune/competitive-displacement system improves
with every tool call. After 50 sessions, the learner has seen thousands
of file access patterns and can predict which files a grep query is really
looking for. A new tool starts with zero observations.

**Layer 3: Cross-Session Intelligence (Future)**

When you build cross-session pattern recognition (Tier 3 above), the
moat becomes: "aOa knows that when you grep for 'auth' in this project,
you always end up reading `internal/auth/handler.go`, so it pre-surfaces
that file." This is the `observe() -> autotune -> competitive displacement`
cycle operating at session scale. No competitor can replicate it without
the same observation history.

### Defensibility Rating by L9 Task

| Task | Moat Contribution | Why |
|------|-------------------|-----|
| L9.1 ContentMeter | Medium | Structured data is prerequisite for everything else |
| L9.2 Tool details | Low | Just display enhancement |
| L9.3 Persisted results | Medium | Completes the content picture |
| L9.4 Subagent tailing | High | Most competitors won't bother -- it's hard, but it reveals the true session cost |
| L9.5 Shadow engine | Very High | The counterfactual is the value proof. Without it, savings claims are theoretical |
| L9.6 Bash shadow | Medium | Extends L9.5 to a harder domain |
| L9.7 Burst throughput | Low-Medium | Nice metric but not defensible |
| L9.8 Dashboard display | Medium | Makes the moat visible -- users see the value |

---

## 5. Risk Mitigation

### Risk 1: Claude Code Format Change

**Current defenses (already strong):**
- `map[string]any` extraction, never typed struct unmarshal
- Multi-path field resolution (`getStringAny(block, "name", "tool_name")`)
- `ExtractionHealth` with gap detection, unknown type tracking, version monitoring
- 512KB line cap (protects against multi-MB tool outputs)
- UUID dedup (protects against duplicate events)
- Never crashes on unexpected input

**Additional defenses for L9:**

1. **Centralize path assumptions** (as described in Section 1). Create
   `internal/adapters/claude/paths.go` with `SessionLayout`. Cost: 30 min.

2. **Add field-level extraction metrics** to ExtractionHealth. Track
   `ToolResultYield` vs `ToolResultMissed` specifically. When yield drops,
   the format changed. Cost: 15 min.

3. **Version-gated behavior**. The tailer already tracks `AgentVersion`.
   If a known-breaking version is detected, log a warning but don't crash.
   Future: maintain a `knownFormats` map that adjusts extraction paths
   per version range. Don't build this now -- YAGNI until you hit an
   actual format break. But design the hook point:

   ```go
   // In parser.go, future hook point:
   // extractContentBlock could check ev.Version and branch.
   // For now, multi-path extraction handles it.
   ```

4. **Graceful degradation for L9.3 and L9.4**. If `tool-results/` directory
   doesn't exist or has no files, ToolPersistedChars stays 0. If
   `subagents/` doesn't exist, SubagentChars stays 0. The ContentMeter
   should never error -- only accumulate what's available. This is
   already the pattern (the parser returns zero values for missing fields).
   Just make sure the new code follows the same pattern.

### Risk 2: Performance Impact of Subagent Tailing (L9.4)

Subagent JSONLs can be large (up to 2MB each). Multiple subagents can
run concurrently. The tailer currently manages one file cursor. Extending
to N cursors needs care.

**Mitigation**: Don't tail subagents in real-time. Instead, **scan on
completion**. When the main JSONL shows a Task tool result (meaning the
subagent finished), stat the subagent's JSONL file and sum its size.
You don't need to parse every line -- just `os.Stat()` for total bytes
gives you SubagentChars with O(1) cost.

If you want richer signals (subagent's internal greps for shadow engine),
parse the subagent JSONL lazily in a background goroutine after the result
arrives. Never block the main tailer.

```go
// In onSessionEvent, when a Task tool result arrives:
case "Task":
    if ev.Tool.ToolID != "" {
        go a.scanSubagentAsync(ev.Tool.ToolID)
    }
```

### Risk 3: Shadow Engine Blocking Search (L9.5)

The shadow engine must NEVER interfere with real search performance (G0).

**Mitigation**: Shadow runs in a separate goroutine with a budget:

```go
func (a *App) shadowSearch(pattern string, path string) {
    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()

    // Use a separate search path that respects the context
    result := a.Engine.SearchWithContext(ctx, pattern, opts)
    // ... record shadow result ...
}
```

If the shadow takes >50ms (shouldn't happen with O(1) index), it's cancelled.
Main pipeline never waits. Shadow results are best-effort.

Actually -- simpler: the search engine is already concurrent (searches are
concurrent, only learner access is serialized via `App.mu`). The shadow
goroutine can just call `a.Engine.Search()` directly without holding the
mutex. The only thing to protect is writing the shadow result back to the
ToolShadow ring, which needs a brief mutex hold.

### Risk 4: Memory Growth from ContentMeter

The ContentMeter holds scalar accumulators (int64 fields) -- no growth risk.
The TurnSnapshot ring buffer is fixed-size (last N turns). The ToolShadow
ring buffer is fixed-size (last 100). No unbounded growth anywhere.

One thing to watch: the `turnBuffer map[string]*turnBuilder` in `app.go`
is already bounded by turn lifecycle (flushed on user input). Make sure
subagent-related builders follow the same pattern.

### Risk 5: Privacy/Security of Session Data

L9 reads Claude's session data (user prompts, model thinking, tool outputs).
This data stays local -- it's only displayed on the localhost-only dashboard.
But make sure:

- No session content is persisted in bbolt (only aggregate metrics)
- No session content is sent over the socket (only derived metrics)
- The ContentMeter stores character COUNTS, not character CONTENT
- The ToolShadow stores pattern and size, not the actual output text

This is already the design intent. Just document it explicitly in the
ContentMeter struct:

```go
// ContentMeter captures aggregate character counts, never raw content.
// No user text, model output, or tool content is stored -- only counts
// and timestamps. This is a privacy invariant.
```

---

## Decomposition: Recommended Build Order

Based on this analysis, the optimal execution sequence:

### Phase A: Foundation (L9.1 + claude/paths.go)

| Step | Task | Effort | Risk |
|------|------|--------|------|
| A.1 | Create `internal/adapters/claude/paths.go` with `SessionLayout` | 30 min | Known |
| A.2 | Move `SessionDirForProject()` from tailer.go to claude/paths.go | 15 min | Known |
| A.3 | Build `ContentMeter` struct in `internal/app/content_meter.go` | 45 min | Known |
| A.4 | Wire existing char captures (scattered in app.go) into ContentMeter | 1 hr | Known |
| A.5 | Unit tests for ContentMeter accumulation from synthetic events | 30 min | Known |

### Phase B: Tool Details + Persisted Results (L9.2 + L9.3)

| Step | Task | Effort | Risk |
|------|------|--------|------|
| B.1 | Thread Pattern/FilePath/Command through TurnAction -> TurnActionResult | 30 min | Known |
| B.2 | Dashboard: show tool details in Actions table | 30 min | Known |
| B.3 | Add `ToolResultFile()` to SessionLayout | 10 min | Known |
| B.4 | In tailer or reader: when inline content=0, stat persisted file | 45 min | Known |
| B.5 | Feed ToolPersistedChars into ContentMeter | 15 min | Known |
| B.6 | Add field-level extraction metrics to ExtractionHealth | 15 min | Known |

### Phase C: Shadow Engine (L9.5)

| Step | Task | Effort | Risk |
|------|------|--------|------|
| C.1 | Define ToolShadow struct + ring buffer in app.go | 30 min | Known |
| C.2 | Shadow goroutine for Grep tool calls | 1 hr | Known |
| C.3 | Shadow goroutine for Glob tool calls | 30 min | Known |
| C.4 | Wire shadow results into counterfactTokensSaved | 30 min | Known |
| C.5 | Unit test: synthetic grep call -> shadow -> delta | 45 min | Known |

### Phase D: Subagents + Burst (L9.4 + L9.7)

| Step | Task | Effort | Risk |
|------|------|--------|------|
| D.1 | Add `SubagentGlob()` to SessionLayout | 10 min | Known |
| D.2 | On Task tool result: stat subagent JSONL for size | 30 min | Known |
| D.3 | Feed SubagentChars into ContentMeter | 15 min | Known |
| D.4 | ActiveMs accumulator from system turn_duration events | 20 min | Known |
| D.5 | TurnSnapshot ring population | 30 min | Known |
| D.6 | Burst throughput calculation (ActiveMs denominator) | 15 min | Known |

### Phase E: Dashboard + Bash Shadow (L9.8 + L9.6)

| Step | Task | Effort | Risk |
|------|------|--------|------|
| E.1 | Per-action shadow column in Actions table | 45 min | Known |
| E.2 | Session savings rollup display | 30 min | Known |
| E.3 | Bash command regex extractor (5 patterns) | 45 min | Known |
| E.4 | Route matched bash commands to shadow engine | 30 min | Known |

---

## First Move

Create `internal/adapters/claude/paths.go`. It's 30 minutes, zero risk,
and every subsequent L9 task benefits from having path assumptions
centralized. Then build ContentMeter (A.3-A.5) since it's the single
struct that everything else wires into.

---

## Key Files Referenced

| File | Role in L9 |
|------|-----------|
| `/home/corey/aOa-go/internal/adapters/tailer/parser.go` | Format coupling point. Defensive map extraction. L9.3 extends this. |
| `/home/corey/aOa-go/internal/adapters/tailer/tailer.go` | File discovery and polling. L9.4 extends to subagents. |
| `/home/corey/aOa-go/internal/adapters/claude/reader.go` | Session Prism. Translates tailer events to canonical ports. |
| `/home/corey/aOa-go/internal/ports/session.go` | Canonical event types. ExtractionHealth. Add field-level metrics here. |
| `/home/corey/aOa-go/internal/app/app.go` | Event handler, burn rate, readSavings, counterfactual system. ContentMeter lives here (or new file). |
| `/home/corey/aOa-go/internal/app/observer.go` | Search observer, grep signal processing. Shadow engine adjacent. |
| `/home/corey/aOa-go/internal/app/burnrate.go` | BurnRateTracker pattern. ToolShadow ring follows same pattern. |
| `/home/corey/aOa-go/internal/adapters/socket/protocol.go` | Wire format. TurnActionResult needs Pattern/Command fields for L9.2. |
| `/home/corey/aOa-go/internal/adapters/web/static/app.js` | Dashboard. L9.8 shadow display, burst throughput sparkline. |

---

## Summary

1. **Abstraction**: The Session Prism is sufficient for JSONL parsing. The
   missing piece is a `SessionLayout` for directory path assumptions.
   Cheap to build, high payoff.

2. **Hidden value**: Tool efficiency scoring, context utilization curves,
   thinking-to-action ratio, and subagent amplification factors. All
   derivable from data you already capture or will capture in L9.

3. **Counterfactual**: The shadow engine design is sound. Add shadow
   coverage tracking (did shadow results predict next-turn reads?) for
   quality proof, not just size proof. Session-level extrapolation gives
   you "35% total savings" without shadowing every call.

4. **Moat**: The data flywheel. Accumulated session observations feed
   personalized insights that a competitor starting from zero cannot
   replicate. L9.5 (shadow engine) and L9.4 (subagent tailing) are the
   highest-moat tasks.

5. **Risk mitigation**: Centralize path assumptions, add field-level
   extraction health, scan subagents on completion (not real-time),
   shadow with timeout budget, document the privacy invariant (counts
   not content).
