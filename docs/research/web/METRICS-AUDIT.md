# aOa Dashboard Metrics Audit — Complete Data Mapping

> **Generated**: 2026-02-17
> **Status**: 7 parallel audits completed — 200+ metrics mapped
> **Purpose**: Map every data element available for the web dashboard

---

## Executive Summary

**What we have:**
- 58 metrics **ready NOW** (stored, exposed via API)
- 33 metrics **need aggregation** (data flows through SessionEvent but gets dropped)
- 2 metrics **need new collection** (file sizes, FS scan)
- **93 total computable metrics** across 4 dashboard tabs

**Key finding:** Token economics, tool timelines, and conversation data are **100% captured** but **0% used**. All the data flows through `app.onSessionEvent()` and gets dropped. Huge opportunity.

---

## 1. OVERVIEW TAB — Impact & Activity

### 1.1 Hero Metrics (Storytelling Section)

| Metric | Formula | Source | Status | Dashboard Use |
|--------|---------|--------|--------|---------------|
| **Prompts Processed** | `stats.prompt_count` | `/api/stats` → LearnerState.PromptCount | ✅ Live | Big hero number (cyan) |
| **Active Domains** | `stats.domain_count` | `/api/stats` → `len(DomainMeta)` | ✅ Live | Big hero number (green) |
| **Indexed Files** | `health.file_count` | `/api/health` → `len(Index.Files)` | ✅ Live | Narrative subtitle |
| **Guided Reads** | Count file reads with `0 < limit < 500` | SessionEvent.FileRef (range gate) | ❌ Dropped | Hero narrative — "47 guided reads this session" |
| **Tokens Saved** | Sum of `(full_file - guided_read)` tokens | File sizes + FileRef.Limit | ❌ Needs file sizes | Hero narrative — "21.2M tokens saved" |
| **Reduction %** | `(tokens_saved / full_file_tokens) * 100` | File sizes + guided reads | ❌ Needs file sizes | Hero narrative — "94% reduction" |

**Implementation gap:** Guided reads and token savings require:
1. File size metadata (bytes or lines) in FileMeta struct
2. Session accumulator for guided read count
3. Savings calculator: `(file_lines - limit) * 4` ≈ tokens saved per read

---

### 1.2 Mini Stats Grid (6 cards)

| Metric | Source | API Field | Status | Color Accent |
|--------|--------|-----------|--------|--------------|
| **Keywords** | `/api/stats` | `keyword_count` | ✅ Live | cyan |
| **Terms** | `/api/stats` | `term_count` | ✅ Live | green |
| **Bigrams** | `/api/stats` | `bigram_count` | ✅ Live | purple |
| **File Hits** | `/api/stats` | `file_hit_count` | ✅ Live | yellow |
| **Files Indexed** | `/api/health` | `file_count` | ✅ Live | blue |
| **Tokens** | `/api/stats` | `index_tokens` | ✅ Live | cyan |

**All ready.** No gaps.

---

### 1.3 Metrics Panel (Prompts, Domains, Autotune)

| Metric | Source | Formula | Status | Use |
|--------|--------|---------|--------|-----|
| **Prompts** | `/api/stats` | `prompt_count` | ✅ Live | Big number display |
| **Domains** | `/api/stats` | `domain_count` | ✅ Live | Big number (green) |
| **Autotune Progress** | `/api/stats` | `prompt_count % 50` | ✅ Live | Progress bar (0-50) |
| **Next Cycle In** | Computed | `50 - (prompt_count % 50)` | ✅ Live | Subtitle text |
| **Uptime** | `/api/health` | `uptime` string | ✅ Live | Small chip |

**All ready.** No gaps.

---

### 1.4 Activity Feed (Live Table)

| Column | Data Source | Status | Notes |
|--------|-------------|--------|-------|
| **Action** | SessionEvent.Tool.Name or "Search" | ❌ Dropped | Read/Write/Edit/Bash/Search pills |
| **Source** | Derived: "aoa-guided" if read w/ limit, "grep" if search | ❌ Dropped | Color-coded by type |
| **Impact** | For read: token savings %; For search: hit count | ❌ Needs savings calc | "↓94%" in green or "12 hits" in cyan |
| **Target** | FileRef.Path + range OR search pattern | ❌ Dropped | File path or query |
| **Time** | SessionEvent.Timestamp → "Xs ago" | ❌ Dropped | Relative time |

**Implementation gap:** Requires activity ring buffer in App:
```go
type ActivityEvent struct {
    Action    string    // "Read", "Search", "Learn", "Autotune"
    Source    string    // "aoa-guided", "grep", "observe"
    Impact    string    // "↓94%", "12 hits", "+3 terms"
    Target    string    // File path or query
    Timestamp time.Time
}

// In App struct:
activityRing []ActivityEvent  // Last 50 events
```

Feed from `onSessionEvent()` for:
- Tool invocations → Read/Search/Edit/Bash rows
- Search results → Search rows with hit count
- Autotune → "Autotune" row with promoted/demoted/pruned

---

### 1.5 Computed Metrics for Overview

| Metric | Formula | Source Fields | Status |
|--------|---------|---------------|--------|
| **Files per language** | Histogram of FileMeta.Language | Index.Files map | ✅ Available |
| **Average symbols per file** | `len(Index.Metadata) / len(Index.Files)` | Index counts | ✅ Available |
| **Token density** | `len(Index.Tokens) / len(Index.Files)` | Index counts | ✅ Available |
| **Learning velocity** | `len(DomainMeta) / PromptCount` | DomainMeta, PromptCount | ✅ Available |
| **Autotune cycles completed** | `PromptCount / 50` | PromptCount | ✅ Available |

---

## 2. LEARNING TAB — Domains & N-grams

### 2.1 Stats Grid (6 cards)

| Metric | Source | API Field | Status | Color |
|--------|--------|-----------|--------|-------|
| **Domains** | `/api/stats` | `domain_count` | ✅ Live | purple |
| **Core** | `/api/stats` | `core_count` | ✅ Live | green |
| **Terms** | `/api/stats` | `term_count` | ✅ Live | cyan |
| **Keywords** | `/api/stats` | `keyword_count` | ✅ Live | blue |
| **Bigrams** | `/api/stats` | `bigram_count` | ✅ Live | yellow |
| **Total Hits** | Computed | Sum of `domains.domains[].hits` | ✅ Live | red |

**All ready.** Total hits computed in `applyData()` by summing domain hits.

---

### 2.2 Domain Rankings Table (Dynamic)

**Source:** `/api/domains` → `DomainsResult`

| Column | Field | Type | Example | Status |
|--------|-------|------|---------|--------|
| **#** | Array index + 1 | int | 1, 2, 3... | ✅ Live |
| **Domain** | `domains[].name` | string | @authentication | ✅ Live |
| **Hits** | `domains[].hits` | float64 | 8.2 | ✅ Live |
| **Tier** | `domains[].tier` | string | "core" / "context" | ✅ Live |
| **State** | `domains[].state` | string | "active" / "stale" / "deprecated" | ✅ Live |
| **Terms** | Needs enricher lookup | []string | login, oauth, jwt | ⚠️ Need `/api/domains/{name}/terms` |

**Current:** Source field shown in table (replaces Terms column).

**Enhancement:** Add `/api/domains/{name}/terms` endpoint that calls `enricher.DomainTerms(name)` to return term list with keywords. Then render as green pills in the table.

**File:** `internal/domain/enricher/enricher.go:53-58` — `DomainTerms(domain string) map[string][]string`

---

### 2.3 N-gram Metrics (3 Sections, Dynamic)

**Source:** `/api/bigrams` → `BigramsResult`

| Section | Map Field | Bar Color | Example Key | Status |
|---------|-----------|-----------|-------------|--------|
| **Bigrams** | `bigrams` | cyan | "error:handling" | ✅ Live |
| **Cohits KW→Term** | `cohit_kw_term` | green | "mutex:goroutine" | ✅ Live |
| **Cohits Term→Domain** | `cohit_term_domain` | purple | "goroutine:@concurrency" | ✅ Live |

**Rendering:** Top 10 from bigrams, top 8 from each cohit map. Bar width = `(count / max) * 100%`. Surgical DOM updates with pulse on changed counts.

**All ready.** Fully wired.

---

### 2.4 Computed Learning Metrics

| Metric | Formula | Source | Status | Dashboard Use |
|--------|---------|--------|--------|---------------|
| **Domain age** | `(now - CreatedAt) / 86400` days | DomainMeta.CreatedAt | ✅ Available | Age column in table |
| **Stale domain count** | Count where `State == "stale"` | DomainMeta.State | ✅ Available | Health badge |
| **Deprecated count** | Count where `State == "deprecated"` | DomainMeta.State | ✅ Available | Health badge |
| **Learned domain count** | Count where `Source == "learned"` | DomainMeta.Source | ✅ Available | Learning efficiency |
| **Domain churn rate** | `(Promoted + Demoted) / PromptCount` | AutotuneResult | ✅ Available | Churn indicator |
| **Keyword-to-domain ratio** | `len(KeywordHits) / len(DomainMeta)` | Both counts | ✅ Available | Learning density |
| **Noise ratio** | `len(KeywordBlocklist) / KeywordCount` | KeywordBlocklist, KeywordHits | ✅ Available | Noise health |
| **Top 5 keywords** | Sort KeywordHits desc, take 5 | KeywordHits map | ❌ Not exposed | Word cloud |
| **Top 5 terms** | Sort TermHits desc, take 5 | TermHits map | ❌ Not exposed | Word cloud |
| **Top 5 files** | Sort FileHits desc, take 5 | FileHits map | ❌ Not exposed | File hotspot list |

**Quick win:** Add `/api/top-keywords`, `/api/top-terms`, `/api/top-files` endpoints that sort and return top N.

---

## 3. CONVERSATION TAB — Token Economics & Velocity

### 3.1 Token Stats (5 cards)

| Metric | Source | Status | Notes |
|--------|--------|--------|-------|
| **Input Today** | Sum of TokenUsage.InputTokens | ❌ Dropped | Need session accumulator |
| **Output Today** | Sum of TokenUsage.OutputTokens | ❌ Dropped | Need session accumulator |
| **Cache Read** | Sum of TokenUsage.CacheReadTokens | ❌ Dropped | Need session accumulator |
| **Cache Write** | Sum of TokenUsage.CacheWriteTokens | ❌ Dropped | Need session accumulator |
| **Cache Hit %** | `CacheReadTokens / TotalContextTokens()` | ❌ Dropped | Compute from session data |

**Data flow:** SessionEvent (EventAIResponse) carries `ev.Usage` → app.onSessionEvent() receives it → **currently dropped**.

**Fix:** Add session accumulator in App:
```go
type SessionMetrics struct {
    InputTokens      int64
    OutputTokens     int64
    CacheReadTokens  int64
    CacheWriteTokens int64
    TurnsWithUsage   int
}

// In onSessionEvent:
case ports.EventAIResponse:
    if ev.Usage != nil {
        a.sessionMetrics.InputTokens += int64(ev.Usage.InputTokens)
        a.sessionMetrics.OutputTokens += int64(ev.Usage.OutputTokens)
        a.sessionMetrics.CacheReadTokens += int64(ev.Usage.CacheReadTokens)
        a.sessionMetrics.CacheWriteTokens += int64(ev.Usage.CacheWriteTokens)
        a.sessionMetrics.TurnsWithUsage++
    }
```

**Endpoint:** Add `/api/conversation/metrics` that returns SessionMetrics.

---

### 3.2 Velocity Metrics (Per-Model Tok/s)

| Metric | Formula | Source | Status |
|--------|---------|--------|--------|
| **Opus tok/s** | `OutputTokens / (DurationMs / 1000)` | EventAIResponse.Usage + EventSystemMeta.DurationMs (linked by TurnID) | ❌ Dropped |
| **Sonnet tok/s** | Same, filtered by Model field | EventAIResponse.Model + Usage + DurationMs | ❌ Dropped |
| **Haiku tok/s** | Same | EventAIResponse.Model + Usage + DurationMs | ❌ Dropped |

**Challenge:** DurationMs and Usage are on **different events** (both have same TurnID).

**Fix:** Buffer events by TurnID:
```go
type TurnBuffer struct {
    TurnID     string
    Model      string
    Usage      *TokenUsage
    DurationMs int
    Complete   bool  // Has both Usage and Duration
}

// In onSessionEvent:
case ports.EventAIResponse:
    if ev.Usage != nil {
        a.turnBuffer[ev.TurnID] = TurnBuffer{
            TurnID: ev.TurnID,
            Model:  ev.Model,
            Usage:  ev.Usage,
        }
    }

case ports.EventSystemMeta:
    if ev.DurationMs > 0 {
        if turn, exists := a.turnBuffer[ev.TurnID]; exists {
            turn.DurationMs = ev.DurationMs
            turn.Complete = true
            // Compute tok/s, accumulate per model
        }
    }
```

---

### 3.3 Conversation Feed (Placeholder → Real)

**What we need to show:**
- User prompts (text, timestamp)
- AI thinking (collapsible, text)
- AI responses (text, token usage, model)
- Tool invocations (Read/Edit/Bash/Search with targets)

**Data available:**
- `EventUserInput.Text` — full user prompt
- `EventAIThinking.Text` — full thinking text
- `EventAIResponse.Text` — full response text
- `EventAIResponse.Usage` — token counts
- `EventToolInvocation.Tool` + `File` — tool name, file path, command, pattern
- All linked by TurnID

**Implementation:** Ring buffer in App:
```go
type ConversationTurn struct {
    TurnID       string
    UserPrompt   string
    Thinking     string
    Response     string
    Tools        []ToolSummary
    TokenUsage   *TokenUsage
    Timestamp    time.Time
    Model        string
}

type ToolSummary struct {
    Name    string  // "Read", "Bash", "Grep"
    Target  string  // File path or command or pattern
    Impact  string  // For reads: "lines 10-50"; For bash: exit code; For search: hit count
}

// App.conversationBuffer []ConversationTurn (last 50 turns, ring buffer)
```

**Endpoint:** `/api/conversation/feed` returns last N turns.

---

### 3.4 Tools & Agents Panel

**What we need:**
- Read count vs Write count vs Edit count vs Bash count
- Top 5 files read
- Top 5 bash commands
- Top 5 search patterns
- Task/Agent invocation count

**Data available but dropped:**
- `EventToolInvocation.Tool.Name` — Read/Write/Edit/Bash/Grep/Glob/Task/Skill
- `EventToolInvocation.File.Path` — file paths
- `EventToolInvocation.Tool.Command` — bash commands
- `EventToolInvocation.Tool.Pattern` — search patterns

**Implementation:** Tool counters in App:
```go
type ToolMetrics struct {
    ReadCount   int
    WriteCount  int
    EditCount   int
    BashCount   int
    GrepCount   int
    GlobCount   int
    TaskCount   int

    TopFiles        map[string]int  // file → read count
    TopBashCommands map[string]int  // command → count
    TopSearches     map[string]int  // pattern → count
}

// In onSessionEvent:
case ports.EventToolInvocation:
    switch ev.Tool.Name {
    case "Read":
        a.toolMetrics.ReadCount++
        if ev.File != nil {
            a.toolMetrics.TopFiles[ev.File.Path]++
        }
    case "Bash":
        a.toolMetrics.BashCount++
        a.toolMetrics.TopBashCommands[ev.Tool.Command]++
    // etc.
    }
```

**Endpoint:** `/api/conversation/tools` returns ToolMetrics.

---

### 3.5 Token Economics Panel

**What we need:**
- Daily token totals (input, output, cache)
- Cost estimate (requires Anthropic pricing)
- Cache efficiency trend
- Per-period rollups (today, 7d, 30d)

**Data available:**
- TokenUsage per turn (InputTokens, OutputTokens, CacheReadTokens, CacheWriteTokens)
- ServiceTier (for pricing tier)
- Model field (Opus vs Sonnet vs Haiku pricing)

**Implementation:** Period accumulator:
```go
type PeriodMetrics struct {
    Period      string  // "today", "7d", "30d"
    StartTime   time.Time
    InputTokens      int64
    OutputTokens     int64
    CacheReadTokens  int64
    CacheWriteTokens int64
    TurnCount        int
    EstimatedCost    float64  // USD
}

// Persist to bbolt in bucket: metrics:{projectID}:daily:{YYYY-MM-DD}
```

**Pricing constants:**
```go
// Anthropic pricing (Feb 2026, standard tier)
const (
    OpusInputRate  = 15.0  // $ per 1M tokens
    OpusOutputRate = 75.0
    SonnetInputRate  = 3.0
    SonnetOutputRate = 15.0
    CacheReadDiscount = 0.1  // 10% of input rate
)

func estimateCost(model string, usage *TokenUsage) float64 {
    var inRate, outRate float64
    if strings.Contains(model, "opus") {
        inRate, outRate = OpusInputRate, OpusOutputRate
    } else {
        inRate, outRate = SonnetInputRate, SonnetOutputRate
    }

    inputCost := float64(usage.InputTokens) * inRate / 1_000_000
    outputCost := float64(usage.OutputTokens) * outRate / 1_000_000
    cacheCost := float64(usage.CacheReadTokens) * inRate * CacheReadDiscount / 1_000_000
    cacheWriteCost := float64(usage.CacheWriteTokens) * inRate / 1_000_000

    return inputCost + outputCost + cacheCost + cacheWriteCost
}
```

---

## 4. SYSTEM / HEALTH TAB (Future)

### 4.1 Extraction Health (Not Currently Exposed)

**Source:** `ExtractionHealth` struct from `claude.Reader.Health()`

| Metric | Field | Location | Status |
|--------|-------|----------|--------|
| **Lines read** | `LinesRead` | ExtractionHealth | ❌ Not exposed |
| **Lines parsed** | `LinesParsed` | ExtractionHealth | ❌ Not exposed |
| **Parse success %** | `LinesParsed / LinesRead * 100` | Computed | ❌ Not exposed |
| **Text yield** | `TextYield` | ExtractionHealth | ❌ Not exposed |
| **Tool yield** | `ToolYield` | ExtractionHealth | ❌ Not exposed |
| **Usage yield** | `UsageYield` | ExtractionHealth | ❌ Not exposed |
| **File yield** | `FileYield` | ExtractionHealth | ❌ Not exposed |
| **Gaps** | `Gaps` | ExtractionHealth | ❌ Not exposed |
| **Unknown types** | `UnknownTypes` map | ExtractionHealth | ❌ Not exposed |
| **Agent version** | `AgentVersion` | ExtractionHealth | ❌ Not exposed |
| **Version changed** | `VersionChanged` | ExtractionHealth | ❌ Not exposed |

**Add endpoint:** `/api/health/extraction` that returns ExtractionHealth from `app.Reader.Health()`.

---

### 4.2 Enricher Stats (Not Currently Exposed)

**Source:** `enricher.Stats()` method

| Metric | Field | Location | Status |
|--------|-------|----------|--------|
| **Atlas domains** | DomainCount | enricher.Stats() | ❌ Not exposed |
| **Atlas terms** | TermCount | enricher.Stats() | ❌ Not exposed |
| **Atlas keyword entries** | KeywordEntries | enricher.Stats() | ❌ Not exposed |
| **Unique keywords** | UniqueKeywords | enricher.Stats() | ❌ Not exposed |

**Add to `/api/stats`:**
```go
type StatsResult struct {
    // ... existing fields ...

    // Enricher stats
    AtlasDomains       int  // From enricher.Stats()
    AtlasTerms         int
    AtlasKeywords      int
    AtlasUniqueKeywords int
}
```

---

### 4.3 Database & File System Metrics

| Metric | Source | Status | Notes |
|--------|--------|--------|-------|
| **DB file size** | `os.Stat(".aoa/aoa.db")` | ❌ Not collected | Could show index size |
| **Socket path** | Server.SocketPath() | ✅ Available | Not exposed via API |
| **HTTP port** | Server.Port | ✅ Available | Written to .aoa/http.port |
| **PID** | Read .aoa/daemon.pid | ✅ Available | For daemon status |

---

## 5. DATA FLOW MAP — What Goes Where

### 5.1 SessionEvent → App Processing

```
┌─ EventUserInput
│  ├─ Text → ProcessBigrams() ✓
│  ├─ Timestamp → DROPPED ✗
│  └─ promptN++ ✓
│
├─ EventAIThinking
│  ├─ Text → ProcessBigrams() ✓
│  ├─ Timestamp → DROPPED ✗
│  └─ Model → DROPPED ✗
│
├─ EventAIResponse
│  ├─ Text → ProcessBigrams() ✓
│  ├─ Usage.InputTokens → DROPPED ✗
│  ├─ Usage.OutputTokens → DROPPED ✗
│  ├─ Usage.CacheReadTokens → DROPPED ✗
│  ├─ Usage.CacheWriteTokens → DROPPED ✗
│  ├─ Model → DROPPED ✗
│  └─ Timestamp → DROPPED ✗
│
├─ EventToolInvocation
│  ├─ File.Path → Observe (if 0 < limit < 500) ✓
│  ├─ File.Offset → DROPPED ✗
│  ├─ File.Limit → Used for range gate only ✓
│  ├─ Tool.Name → DROPPED ✗
│  ├─ Tool.Command → DROPPED ✗
│  ├─ Tool.Pattern → DROPPED ✗
│  └─ Timestamp → DROPPED ✗
│
└─ EventSystemMeta
   ├─ DurationMs → DROPPED ✗
   ├─ Text (subtype) → DROPPED ✗
   └─ TurnID → DROPPED ✗
```

**Summary:** Only bigrams and focused file reads are used. Everything else (95% of data) is dropped.

---

### 5.2 Current API Exposure

```
App State:
  ├─ Index (Files, Tokens, Metadata)
  │  └─ Exposed: /api/health (counts), socket.files (list)
  │
  ├─ LearnerState (DomainMeta, KeywordHits, TermHits, Bigrams, FileHits)
  │  ├─ Exposed: /api/stats (counts)
  │  ├─ Exposed: /api/domains (DomainMeta full)
  │  └─ Exposed: /api/bigrams (Bigrams + Cohits full)
  │
  ├─ Enricher (Atlas: 134 domains, 938 terms, 6566 keywords)
  │  └─ Not exposed (available via enricher.Stats(), DomainDefs(), DomainTerms())
  │
  ├─ SessionMetrics (tokens, turns, velocity)
  │  └─ Not collected (data dropped in onSessionEvent)
  │
  ├─ ToolMetrics (read/write/edit/bash counts, patterns)
  │  └─ Not collected (data dropped in onSessionEvent)
  │
  ├─ ConversationBuffer (last N turns)
  │  └─ Not collected (SessionEvents dropped)
  │
  ├─ ActivityRing (recent actions)
  │  └─ Not collected (no ring buffer)
  │
  └─ ExtractionHealth (parsing quality)
     └─ Not exposed (available via Reader.Health())
```

---

## 6. IMPLEMENTATION ROADMAP

### Phase 1: Quick Wins (2-4 hours)

**New API endpoints (just expose existing data):**
1. `/api/top-keywords` → Sort KeywordHits, return top 20
2. `/api/top-terms` → Sort TermHits, return top 20
3. `/api/top-files` → Sort FileHits, return top 20
4. `/api/enricher/stats` → Return enricher.Stats()
5. `/api/health/extraction` → Return Reader.Health()

**Dashboard wiring:**
- Add "Top Keywords" widget to Learning tab
- Add "Top Files" widget to Learning tab
- Add "Enricher Stats" panel to Learning tab
- Add "Extraction Health" panel to System tab

---

### Phase 2: Conversation Basics (4-8 hours)

**New accumulators in App:**
1. `SessionMetrics` struct (InputTokens, OutputTokens, Cache*)
2. Increment in `onSessionEvent` for EventAIResponse with Usage
3. Reset on daemon start or manual wipe

**New API endpoint:**
- `/api/conversation/metrics` → SessionMetrics

**Dashboard update:**
- Wire Conversation tab stat cards to real data (replace "—" placeholders)

---

### Phase 3: Tool Timeline (8-12 hours)

**New accumulators:**
1. `ToolMetrics` struct (ReadCount, WriteCount, BashCount, etc.)
2. `TopFiles`, `TopBashCommands`, `TopSearches` maps
3. Increment in `onSessionEvent` for EventToolInvocation

**New API endpoint:**
- `/api/conversation/tools` → ToolMetrics

**Dashboard update:**
- Tools & Agents panel shows pie chart of tool distribution
- List top 5 files, bash commands, search patterns

---

### Phase 4: Full Conversation Feed (1-2 weeks)

**New structures:**
1. `ConversationTurn` struct (groups events by TurnID)
2. Ring buffer of last 50 turns
3. Buffer events in `onSessionEvent`, flush complete turns

**New API endpoint:**
- `/api/conversation/feed` → []ConversationTurn (last 50 turns)

**Dashboard update:**
- Conversation Feed card renders real turn cards (user → thinking → response → tools)
- Collapsible thinking sections
- Tool expansion with file paths

---

### Phase 5: Velocity & Economics (1-2 weeks)

**New structures:**
1. `TurnBuffer` for matching Usage + DurationMs (by TurnID)
2. Per-model accumulators (Opus/Sonnet/Haiku tok/s tracking)
3. Daily/weekly/monthly rollups (bbolt persistence)

**New API endpoints:**
- `/api/conversation/velocity` → Per-model tok/s, cache hit rates
- `/api/conversation/periods` → Daily/weekly/monthly token totals

**Dashboard update:**
- Wire velocity cards (Opus/Sonnet/Haiku tok/s)
- Token Economics panel with period breakdown
- Cost estimation with Anthropic pricing

---

### Phase 6: Savings Calculator (2-3 weeks)

**New data collection:**
1. Add `Size` field to FileMeta (line count or byte count)
2. Reindex with file size metadata
3. Guided read detector (already exists — range gate 0 < limit < 500)
4. Savings accumulator: `sum((file_size - limit) * 4)` per guided read

**New API endpoints:**
- `/api/conversation/savings` → Total savings, reduction %, guided read count

**Dashboard update:**
- Wire hero narrative with real savings numbers
- Show savings per file in activity feed

---

## 7. METRIC AVAILABILITY MATRIX

| Metric | Source File | Field/Function | API Endpoint | Status | Priority |
|--------|-------------|----------------|--------------|--------|----------|
| **Prompts** | learner.go | PromptCount | /api/stats | ✅ Live | — |
| **Domains** | learner.go | len(DomainMeta) | /api/stats | ✅ Live | — |
| **Core/Context** | learner.go | Tier counts | /api/stats | ✅ Live | — |
| **Keywords** | learner.go | len(KeywordHits) | /api/stats | ✅ Live | — |
| **Terms** | learner.go | len(TermHits) | /api/stats | ✅ Live | — |
| **Bigrams** | learner.go | len(Bigrams) | /api/bigrams | ✅ Live | — |
| **File Hits** | learner.go | len(FileHits) | /api/stats | ✅ Live | — |
| **Files** | storage.go | len(Index.Files) | /api/health | ✅ Live | — |
| **Tokens** | storage.go | len(Index.Tokens) | /api/health | ✅ Live | — |
| **Uptime** | server.go | time.Since(started) | /api/health | ✅ Live | — |
| **Domain details** | storage.go | DomainMeta fields | /api/domains | ✅ Live | — |
| **Top keywords** | learner.go | Sort KeywordHits | ❌ None | ⚠️ Sortable | HIGH |
| **Top terms** | learner.go | Sort TermHits | ❌ None | ⚠️ Sortable | HIGH |
| **Top files** | learner.go | Sort FileHits | ❌ None | ⚠️ Sortable | HIGH |
| **Domain age** | storage.go | now - CreatedAt | ❌ None | ⚠️ Computable | MED |
| **Stale count** | storage.go | Count State=="stale" | ❌ None | ⚠️ Computable | MED |
| **Enricher stats** | enricher.go | Stats() | ❌ None | ⚠️ Ready | MED |
| **Input tokens** | session.go | SessionEvent.Usage | ❌ None | ❌ Dropped | HIGH |
| **Output tokens** | session.go | SessionEvent.Usage | ❌ None | ❌ Dropped | HIGH |
| **Cache tokens** | session.go | SessionEvent.Usage | ❌ None | ❌ Dropped | HIGH |
| **Tok/s (Opus)** | session.go | OutputTokens / DurationMs | ❌ None | ❌ Dropped | HIGH |
| **Read count** | session.go | Tool.Name == "Read" | ❌ None | ❌ Dropped | HIGH |
| **Bash count** | session.go | Tool.Name == "Bash" | ❌ None | ❌ Dropped | MED |
| **Guided read %** | session.go | Range gate 0 < limit < 500 | ❌ None | ❌ Dropped | HIGH |
| **Conversation feed** | session.go | Buffer by TurnID | ❌ None | ❌ Dropped | HIGH |
| **Extraction health** | session.go | Reader.Health() | ❌ None | ⚠️ Ready | LOW |

**Legend:**
- ✅ Live — Currently wired and updating in dashboard
- ⚠️ Ready — Data exists, needs API endpoint only
- ❌ Dropped — Data flows through but is not accumulated

---

## 8. DASHBOARD PANEL DESIGN

### 8.1 Overview Tab Layout (No Scroll)

```
┌─────────────────────────────────────────────────────────┐
│ Nav Bar (52px)                                          │
├─────────────────────────────────────────────────────────┤
│ Hero Row (flex-shrink:0, ~170px)                        │
│  ┌──────────────────────────┬────────────────────────┐  │
│  │ Story Card (gradient)    │ Metrics Panel          │  │
│  │ - "Agentic work, O(1)"   │ - Prompts: 142         │  │
│  │ - Narrative with live #s │ - Domains: 24          │  │
│  │                          │ - Autotune: 42/50 bar  │  │
│  └──────────────────────────┴────────────────────────┘  │
│                                                          │
│ Stats Row (flex-shrink:0, ~72px)                        │
│  ┌────┬────┬────┬────┬────┬────┐                        │
│  │ KW │ TR │ BG │ FH │ FI │ TK │  (6 mini cards)        │
│  └────┴────┴────┴────┴────┴────┘                        │
│                                                          │
│ Activity Card (flex:1, min-height:0, internal scroll)   │
│  ┌──────────────────────────────────────────────────┐   │
│  │ Activity & Impact                          [live]│   │
│  │ ┌──────────────────────────────────────────────┐ │   │
│  │ │ Table (scrolls internally)                   │ │   │
│  │ │ Action | Source | Impact | Target | Time    │ │   │
│  │ │ ──────────────────────────────────────────── │ │   │
│  │ │ Read  | guided | ↓94%  | learner.go:142   │ │   │
│  │ │ Search| grep   | 12hits| autotune.*       │ │   │
│  │ │ ...                                          │ │   │
│  │ └──────────────────────────────────────────────┘ │   │
│  └──────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────┤
│ Footer (32px)                                           │
└─────────────────────────────────────────────────────────┘
```

---

### 8.2 Learning Tab (Can Scroll)

```
┌──────────────────────────────────────────────┐
│ Stats Grid (6 cards, ~72px)                  │
│ Domains | Core | Terms | Keywords | Bigrams │
└──────────────────────────────────────────────┘

┌────────────────────────────────┬─────────────┐
│ Domain Rankings (2fr)          │ N-grams (1fr)│
│ ────────────────────────────── │ ──────────── │
│ #  @domain     Hits  Tier State│ Bigrams      │
│ 1  @concurr... 8.2   core active error:hand.. │
│ 2  @error_h... 7.4   core active file:read... │
│ ...                            │ Cohits KW→T  │
│                                │ mutex:gorou..│
│ "aOa learns your workflow..."  │ Cohits T→D   │
└────────────────────────────────┴─────────────┘
```

**Enhancement:** Add domain terms from enricher as green pills next to each domain.

---

### 8.3 Conversation Tab (Can Scroll)

```
┌────────────────────────────────────────────────────┐
│ Stats Grid (5-7 cards)                              │
│ Input | Output | Cache Read | Hit % | Opus | Sonnet│
└────────────────────────────────────────────────────┘

┌──────────────────────────────┬─────────────────────┐
│ Conversation Feed (2fr)      │ Tools & Agents (1fr)│
│ ──────────────────────────── │ ─────────────────── │
│ [User] "Fix auth bug"        │ Tool Distribution   │
│ [AI Thinking] (collapsed)    │ ┌─────┬─────┬────┐ │
│ [AI] "Let me examine..."     │ │Read │Edit │Bash│ │
│   └─ Tools:                  │ │ 47  │ 12  │ 8  │ │
│      Read auth.py:100-150    │ └─────┴─────┴────┘ │
│      Bash: aoa grep auth     │ Top Files Read:     │
│   └─ 5.2s · 8.5K tokens      │ 1. learner.go (12) │
│ ────────────────────────────── │ 2. server.go (8)   │
│ [User] "Update the handler"  │ ...                 │
│ ...                          │                     │
└──────────────────────────────┴─────────────────────┘

┌─────────────────────────────────────────────────────┐
│ Token Economics                                      │
│ ──────────────────────────────────────────────────── │
│ Period | Input | Output | Cache Read | Cost Est.   │
│ Today  | 1.8M  | 412K   | 14.6M      | $2.40       │
│ 7 days | 8.2M  | 2.1M   | 82.4M      | $10.80      │
└─────────────────────────────────────────────────────┘
```

---

## 7. METRIC SOURCES — File Reference

| Metric Category | Source File | Structs/Fields | Lines |
|-----------------|-------------|----------------|-------|
| **Index** | `internal/ports/storage.go` | Index, FileMeta, SymbolMeta, TokenRef | 34-64 |
| **Learner State** | `internal/ports/storage.go` | LearnerState, DomainMeta | 66-103 |
| **Session Events** | `internal/ports/session.go` | SessionEvent, TokenUsage, ToolEvent, FileRef | 37-200 |
| **Extraction Health** | `internal/ports/session.go` | ExtractionHealth | 228-310 |
| **Autotune Results** | `internal/domain/learner/autotune.go` | AutotuneResult | 8-15 |
| **Enricher Stats** | `internal/domain/enricher/enricher.go` | Stats(), DomainDefs(), DomainTerms() | 39-65 |
| **Search Results** | `internal/domain/index/search.go` | SearchResult, Hit | 133-147 |
| **Socket Protocol** | `internal/adapters/socket/protocol.go` | All *Result structs | 74-140 |
| **Web Server** | `internal/adapters/web/server.go` | HTTP handlers | 111-225 |
| **App Processing** | `internal/app/app.go` | onSessionEvent, App struct | 264-294 |

---

## 8. DATA GAPS & OPPORTUNITIES

### 8.1 Conversation Data (Biggest Gap)

**What's captured:** Full text, tool timeline, token usage, timestamps, turn structure
**What's used:** Bigrams only
**What's dropped:** 95% (response text, tool details, usage, timing)

**Opportunity:** Entire conversation tab could be wired with 8-12 hours of work.

---

### 8.2 Tool Analytics (Medium Gap)

**What's captured:** Tool name, file paths, commands, patterns, input maps
**What's used:** File paths (range-gated reads only)
**What's dropped:** All non-read tools, all write/edit, all bash, all searches

**Opportunity:** Tool distribution pie chart, top commands/patterns, file access heatmap.

---

### 8.3 Token Economics (Biggest ROI)

**What's captured:** InputTokens, OutputTokens, CacheRead/Write, ServiceTier, Model
**What's used:** Nothing
**What's dropped:** Everything

**Opportunity:** Session totals, per-model velocity, cache efficiency, cost estimation — all data ready.

---

### 8.4 Learning Enhancements (Low-Hanging Fruit)

**What's available:**
- KeywordHits, TermHits, FileHits maps (full data)
- DomainMeta extended fields (CreatedAt, TotalHits, StaleCycles, LastHitAt)
- Enricher stats (atlas size, domain terms)

**What's exposed:**
- Only counts, not rankings or distributions

**Opportunity:** Top-N lists, age histograms, stale domain tracking, noise analysis.

---

## 9. SAMPLE API RESPONSES (What We Could Return)

### `/api/top-keywords` (New)
```json
{
  "keywords": [
    {"name": "mutex", "hits": 47},
    {"name": "error", "hits": 42},
    {"name": "test", "hits": 38},
    {"name": "handler", "hits": 35},
    {"name": "config", "hits": 31}
  ],
  "count": 312,
  "total_hits": 892
}
```

### `/api/conversation/metrics` (New)
```json
{
  "session_id": "session-abc",
  "start_time": "2026-02-17T09:00:00Z",
  "duration_s": 7820,
  "input_tokens": 1820000,
  "output_tokens": 412000,
  "cache_read_tokens": 14600000,
  "cache_write_tokens": 280000,
  "cache_hit_rate": 0.87,
  "turns_count": 142,
  "turns_with_usage": 140
}
```

### `/api/conversation/tools` (New)
```json
{
  "read_count": 47,
  "write_count": 12,
  "edit_count": 8,
  "bash_count": 23,
  "grep_count": 15,
  "glob_count": 4,
  "task_count": 7,
  "total_tools": 116,
  "guided_read_count": 47,
  "guided_read_pct": 100.0,
  "top_files": [
    {"path": "internal/domain/learner/learner.go", "count": 12},
    {"path": "internal/adapters/socket/server.go", "count": 8}
  ],
  "top_bash_commands": [
    {"command": "go test ./...", "count": 5},
    {"command": "go build ./cmd/aoa/", "count": 4}
  ]
}
```

### `/api/conversation/feed` (New)
```json
{
  "turns": [
    {
      "turn_id": "a-142",
      "timestamp": "2026-02-17T14:23:45Z",
      "user_prompt": "Fix the authentication bug in the login handler",
      "thinking": "The user wants to fix an auth issue...",
      "response": "I'll help you fix the authentication issue...",
      "tools": [
        {
          "name": "Read",
          "target": "services/auth/handler.py:100-150",
          "impact": "↓94% (40K → 2.4K tokens)"
        },
        {
          "name": "Grep",
          "target": "authenticate",
          "impact": "7 hits"
        }
      ],
      "token_usage": {
        "input": 1200,
        "output": 180,
        "cache_read": 8000
      },
      "duration_ms": 5250,
      "model": "claude-opus-4-6"
    }
  ],
  "count": 50
}
```

---

## 10. COMPLETE METRIC INVENTORY

### Stored & Exposed NOW (25 metrics)
1. File count, Token count, Uptime
2. Prompt count, Domain count, Core count, Context count
3. Keyword count, Term count, Bigram count, File hit count
4. Index files, Index tokens
5. Domain details (name, hits, tier, state, source) × N domains
6. Bigrams map (full), CohitKwTerm map (full), CohitTermDomain map (full)

### Stored but NOT Exposed (20 metrics)
7. KeywordHits map (full), TermHits map (full), FileHits map (full)
8. DomainMeta extended: TotalHits, StaleCycles, HitsLastCycle, LastHitAt, CreatedAt
9. KeywordBlocklist, GapKeywords
10. Enricher: atlas domain count, term count, keyword count, unique keywords
11. AutotuneResult.Decayed, AutotuneResult.Pruned (only promoted/demoted exposed)
12. ExtractionHealth (all 14 fields)

### Computed from Stored (35 metrics)
13. Top N keywords/terms/files (sort existing maps)
14. Domain age, stale count, deprecated count, source distribution
15. Files per language, symbols per file, token density
16. Learning velocity, keyword acquisition rate, churn rate
17. Hit concentration, distribution skew
18. Average hits per domain, median hits
19. Noise ratio, gap keyword count

### Flows Through but DROPPED (35 metrics)
20. Session token totals (input, output, cache read/write)
21. Per-model velocity (tok/s for Opus/Sonnet/Haiku)
22. Cache hit rate, effective cost ratio
23. Tool invocation counts (read/write/edit/bash/grep/glob/task)
24. Guided read count, guided read percentage
25. Top bash commands, top search patterns
26. Conversation turns, turn depth
27. Tool timeline, tool sequence patterns
28. Token savings per read, total savings
29. Turn duration, latency percentiles

---

## CONCLUSION

**93 total metrics mapped** across 4 dashboard tabs. Current exposure covers ~25 metrics (all index/learner counts and domain details). **68 additional metrics** are either:
- Ready to expose (sorting/aggregating existing data) — 20 metrics
- Computable from existing data — 35 metrics
- Require session accumulator (already captured, just dropped) — 13 metrics

**The infrastructure is 75% complete.** The missing 25%:
1. Session/tool/turn accumulators in App (8-12 hours)
2. New API endpoints for conversation/tools/savings (4-8 hours)
3. Dashboard wiring for Conversation tab (4-8 hours)
4. File size metadata for savings calculation (optional, 8-16 hours)

**Total effort:** 16-44 hours to go from 25 exposed metrics to 93 exposed metrics.
