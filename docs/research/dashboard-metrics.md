# Dashboard Metrics Reference

True origin → calculation → storage for every hero and stats metric across 5 tabs.

**JSONL location**: `~/.claude/projects/{encoded-path}/*.jsonl`
**Tailer parser**: `internal/adapters/tailer/parser.go`
**Event handler**: `internal/app/app.go` `onSessionEvent()`

### Data Sources

| Source | Location | How Ingested | What It Provides |
|--------|----------|-------------|------------------|
| **Session JSONL** | `~/.claude/projects/{encoded-path}/*.jsonl` | Tailer (polling, byte-offset) → claude.Reader → `onSessionEvent()` | Per-turn tokens, tool invocations, conversation text, model, session boundaries |
| **Status Line Hook** | `.aoa/context.jsonl` | Hook appends on each status update → fsnotify → `onContextFileChanged()` | Real context window position, total cost USD, session/API duration, lines added/removed, model, version |
| **Project Files** | Project root (recursive walk) | fsnotify → `onFileChanged()` | Index files/tokens, recon findings |
| **Learner State** | `.aoa/aoa.db` (bbolt) | In-memory learner, persisted at autotune/session end | Domains, terms, keywords, bigrams, hit counts |

### Status Line Hook Data (`.aoa/context.jsonl`)

The status line hook receives Claude Code's stdin JSON on every status update (~300ms debounce). It captures fields **not available** in session JSONL and appends them to `.aoa/context.jsonl`. The daemon reads this file (read-only) via fsnotify and stores the last 5 snapshots in a ring buffer.

| Field | Type | Description | Available Elsewhere? |
|-------|------|-------------|---------------------|
| `ctx_used` | `int64` | Current context window token count (input + cache_creation + cache_read) | No — session JSONL has per-turn usage, not cumulative position |
| `ctx_max` | `int64` | Context window size (e.g., 200000) | No — daemon guesses from model name |
| `used_pct` | `float64` | Context utilization percentage (from Claude Code) | No |
| `remaining_pct` | `float64` | Remaining context percentage | No |
| `total_cost_usd` | `float64` | Real session cost calculated by Claude Code | No — daemon estimates from pricing table |
| `total_duration_ms` | `int64` | Total session wall-clock duration | No |
| `total_api_duration_ms` | `int64` | Total time spent in API calls | No |
| `total_lines_added` | `int` | Lines of code added this session | No |
| `total_lines_removed` | `int` | Lines of code removed this session | No |
| `model` | `string` | Model ID (e.g., `claude-opus-4-6`) | Yes (session JSONL) |
| `session_id` | `string` | Session UUID | Yes (session JSONL) |
| `version` | `string` | Claude Code CLI version | Yes (session JSONL) |
| `ts` | `int64` | Unix timestamp when captured | — |

**File management**: Hook appends freely, self-truncates to 5 lines when file exceeds 20 lines (tail + atomic mv). Daemon is read-only — never writes to this file. Max file size: ~1.2KB.

---

## Key Metrics by Tab — Full Catalog

Position-mapped to the dashboard layout. Each tab has:
- **Hero Card** — big headline (left 2/3), with a **Support Line** below it
- **Hero Metrics** — 2x2 grid (right 1/3): TL → TR / BL → BR
- **Stats Grid** — 5 or 6 stat cards below

Status: **[LIVE]** = exists today, **[NEW]** = derivable from JSONL data we already have, **[CALC]** = new calculation on existing captured data.

The **Rec#** column is the recommended position. The **Story** column explains why that metric belongs there — what narrative it serves, what the user should feel when they see it.

---

### Live Tab — "Right now, aOa is extending your session and saving you money."

The Live tab answers: *How healthy is my session? Is aOa helping? How much runway do I have left?*

| Position | Current Metric | Status | Rec# | Recommended Metric | Story |
|----------|---------------|--------|------|--------------------|-------|
| **Hero Card** | Angle of Attack (rotating headline) | **[LIVE]** | 1 | **Runway** (minutes remaining) | The single most urgent number. "You have 47 min left." User sees time as currency — this is the clock. Rotating headline stays as flavor text above. |
| **Hero Support** | runway, domains, prompts, counterfactual | **[LIVE]** | 2 | **Burn Rate** · **Context %** · **Model** · **Turn #** | Four-stat support line. Burn rate = velocity of spend, Context % = real utilization from status line hook (`ctx_used/ctx_max`), Model = what engine, Turn # = where we are. Tells the operational story in one glance. |
| **Hero TL** | Time Saved | **[LIVE]** | 3 | **Tokens Saved** | Lead with the concrete number. Tokens saved is the raw proof of value — "aOa saved you 42k tokens." It's the first thing an observer would want to verify. |
| **Hero TR** | Tokens Saved | **[LIVE]** | 4 | **Cost Saved $** | Convert tokens to dollars. "$0.63 saved" is more visceral than "42k tokens." Requires pricing calc. This is the money story. |
| **Hero BL** | Guided % | **[LIVE]** | 5 | **Time Saved** | Move time saved here — it's the human-felt benefit. "2.3 min saved" means the user physically waited less. Pairs with cost above: money + time. |
| **Hero BR** | Extended Runway | **[LIVE]** | 6 | **Extended Runway** | Keep this — "you gained +8 min" closes the hero story. The hero block now reads: saved tokens → saved money → saved time → gained runway. A complete value arc. |
| **Stats 1** | Guided Ratio | **[LIVE]** | 7 | **Context Utilization %** | How full is the context window? This is the fuel gauge. Answers "should I be worried?" Green at 30%, yellow at 60%, red at 85%. Most actionable stat. **Source**: `.aoa/context.jsonl` → `used_pct` (real data from Claude Code status line hook, not estimated). |
| **Stats 2** | Avg Savings/Read | **[LIVE]** | 8 | **Burn Rate** | Tokens/min. If context utilization is the fuel gauge, burn rate is the speedometer. Together they answer "when will I run out?" |
| **Stats 3** | Searches | **[LIVE]** | 9 | **Guided Ratio** | % of reads that were guided. This is the quality indicator — is aOa actually steering Claude? Higher = better training. |
| **Stats 4** | Files Indexed | **[LIVE]** | 10 | **Output Speed** | Tokens/sec generation rate. Users feel this — are responses coming fast or slow? Derived from `output_tokens / durationMs`. A performance pulse. |
| **Stats 5** | Autotune | **[LIVE]** | 11 | **Cache Hit %** | Rolling cache efficiency. Shows Anthropic's cache is working — when high, user is spending less per turn. Directly maps to cost savings. |
| **Stats 6** | Context Burn | **[LIVE]** | 12 | **Autotune** | Keep autotune progress (N/50) — it's the learning heartbeat. Shows the system is alive and calibrating. Last position because it's internal/secondary. |
| *(overflow)* | — | — | — | **Searches** | Moved out of grid — already shown in hero support line as Turn #. Could be in support line or tooltip. |
| *(overflow)* | — | — | — | **Avg Savings/Read** | Moved out — good detail metric for a tooltip or expanded view. Not front-page compelling on its own. |
| *(overflow)* | — | — | — | **Files Indexed** | Moved out — static number, doesn't change during a session. Better in Arsenal/System Status. |
| *(overflow)* | — | — | — | **Turn Velocity** | `turns/min` — available for support line or tooltip. Shows session pacing. |
| *(overflow)* | — | — | — | **User Think Time** | Avg seconds between turns — could surface in debrief or as tooltip. Shows user engagement pacing. |
| *(overflow)* | — | — | — | **Compaction Count** | Times context was auto-compacted. Warning indicator — high count means you're pushing limits. |

**Recommended story flow**: Runway clock (urgency) → Saved tokens/$$/time (value proof) → Extended runway (payoff) → Context health + burn rate + quality + speed + cache + learning (operational dashboard).

---

### Recon Tab — "Your codebase has been scanned. Here's the security posture."

The Recon tab answers: *How clean is my code? Where are the problems? How serious are they?*

| Position | Current Metric | Status | Rec# | Recommended Metric | Story |
|----------|---------------|--------|------|--------------------|-------|
| **Hero Card** | Recon (rotating headline) | **[LIVE]** | 1 | **Security Score** | A single 0–100 composite number. `100 − (weightedFindings / files)`. This is the grade — "your codebase scores 87/100." Instant posture assessment. |
| **Hero Support** | Static description text | **[LIVE]** | 2 | **Files Scanned** · **Findings/KLOC** · **Clean %** · **Dimensions Active** | Four-stat support line. Files scanned = coverage, Findings/KLOC = normalized density (comparable across projects), Clean % = positive framing, Active dimensions = scan depth. |
| **Hero TL** | Files Scanned | **[LIVE]** | 3 | **Files Scanned** | Keep — this is coverage. "We looked at 247 files." Establishes credibility of the scan. |
| **Hero TR** | Findings | **[LIVE]** | 4 | **Findings** | Keep — total finding count. Pairs with files scanned: "247 files, 38 findings." |
| **Hero BL** | Critical | **[LIVE]** | 5 | **Critical** | Keep — the alarm number. Red, prominent. "3 critical." User knows immediately if action is needed. |
| **Hero BR** | Clean Files | **[LIVE]** | 6 | **Clean File %** | Switch from count to percentage — "89% clean" is more meaningful than "221 clean." Tells a positive story alongside the critical count. |
| **Stats 1** | Files Scanned | **[LIVE]** | 7 | **Findings/KLOC** | Normalized density. Comparable across projects and over time. "2.1 findings per 1000 lines" — is that good? Users learn to benchmark. Replaces raw files scanned (now in hero). |
| **Stats 2** | Findings | **[LIVE]** | 8 | **Critical** | Red-colored critical count. Duplicated from hero on purpose — this is the call to action. Replaces raw findings (now in hero). |
| **Stats 3** | Critical | **[LIVE]** | 9 | **Warnings** | Yellow-colored warning count. Severity spectrum: critical → warnings → clean. Natural reading order. |
| **Stats 4** | Warnings | **[LIVE]** | 10 | **Clean File %** | Positive counterbalance. After seeing criticals and warnings, "89% clean" reassures. Green-colored. |
| **Stats 5** | Dimensions | **[LIVE]** | 11 | **Dimensions Active** | Keep — shows scan configuration depth. "4 of 5 active." Purple. |
| *(new slot)* | — | — | 12 | **Critical Density** | `criticals / files` — avg criticals per file. Trend-able over time. Could replace one of the above if 6-column grid is adopted. |
| *(overflow)* | — | — | — | **Security Score** | If not used as hero main number, works as Stats 1. |
| *(overflow)* | — | — | — | **Top Hot Files** | Ranked list of files by weighted severity — better as a list widget below the grid, not a stat card. |
| *(overflow)* | — | — | — | **Language Risk Map** | Per-language finding rates — visualization below, not a stat number. |

**Recommended story flow**: Security grade (posture at a glance) → Coverage + density + clean% (context) → Critical + warnings (action items) → Clean% + dimensions (depth) → Drill into tree view below.

---

### Intel Tab — "aOa is learning your project. Here's what it knows."

The Intel tab answers: *How deeply does aOa understand my codebase? Is it getting smarter? What domains has it discovered?*

| Position | Current Metric | Status | Rec# | Recommended Metric | Story |
|----------|---------------|--------|------|--------------------|-------|
| **Hero Card** | Intel (rotating headline) | **[LIVE]** | 1 | **Domains Discovered** (big number) | The breadth of understanding. "aOa has mapped 18 semantic domains." Shows intelligence, not just raw data. The system *understands* your project. |
| **Hero Support** | domain/term/keyword/bigram counts, total hits | **[LIVE]** | 2 | **Core** · **Terms** · **Keywords** · **Total Hits** | Four-stat support. Core = depth of focus, Terms = vocabulary, Keywords = raw signals, Total Hits = learning volume. Gives the full signal chain in one line. |
| **Hero TL** | Domains | **[LIVE]** | 3 | **Core Domains** | The quality measure. Not just "how many domains" but "how many are fully learned." Core domains are battle-tested, promoted by competitive displacement. |
| **Hero TR** | Core | **[LIVE]** | 4 | **Domain Velocity** | `domains / promptCount` — how fast is aOa learning? Rising = getting smarter. Falling = converging. This is the learning *rate*, not just the count. |
| **Hero BL** | Terms Matched | **[LIVE]** | 5 | **Term Coherence** | `termsWithDomainHits / totalTerms × 100` — "78% of terms map to domains." Shows the signal isn't noise — terms are resolving into meaningful domains. Quality indicator. |
| **Hero BR** | Bigrams | **[LIVE]** | 6 | **Keyword→Domain Rate** | `domainsFromKeywords / totalKeywords × 100` — "34% of keywords became domains." The conversion funnel: raw observations → structured knowledge. |
| **Stats 1** | Domains | **[LIVE]** | 7 | **Domains** | Keep — purple. Total domain count. Foundation number. |
| **Stats 2** | Core | **[LIVE]** | 8 | **Core** | Keep — green. Promoted domains. The "best of" count. |
| **Stats 3** | Terms | **[LIVE]** | 9 | **Terms** | Keep — cyan. Semantic vocabulary size. |
| **Stats 4** | Keywords | **[LIVE]** | 10 | **Learning Rate** | `Δ(keywords) / 50` — keywords added per autotune cycle. Replace raw keyword count (available in support line) with a rate. Shows momentum. |
| **Stats 5** | Bigrams | **[LIVE]** | 11 | **Bigrams** | Keep — yellow. Conversational pattern count. |
| **Stats 6** | Total Hits | **[LIVE]** | 12 | **Prune Pressure** | Count of keywords/terms near the prune floor (<1.0 hits). "12 terms at risk." Shows the decay/survival dynamic — the system is curating, not just accumulating. |
| *(overflow)* | — | — | — | **Keywords** | Raw count moved out of stats grid — already in support line. Available in tooltip. |
| *(overflow)* | — | — | — | **Total Hits** | Moved out — sum of domain hits is useful but abstract. Better as tooltip on domains. |
| *(overflow)* | — | — | — | **Core Stability** | Track tier flips per autotune. Trend metric for expanded view. |
| *(overflow)* | — | — | — | **Bigram Density** | `bigrams / turnCount` — available in tooltip or expanded view. |
| *(overflow)* | — | — | — | **Domain Freshness** | Time since last hit per domain — requires timestamp tracking. Future feature for domain list. |
| *(overflow)* | — | — | — | **Signal Mix** | Breakdown of observation sources (search/grep/read/conversation). Pie chart below the grid. |
| *(overflow)* | — | — | — | **Top Domain Trend** | Sparkline of top-5 domain hits over autotune cycles. Requires historical snapshots. |

**Recommended story flow**: Domains discovered (breadth) → Core + velocity + coherence + conversion (learning quality) → Raw counts (foundation) → Learning rate + prune pressure (system dynamics). The tab tells the story of a system that *thinks*, not just counts.

---

### Debrief Tab — "Here's what this session cost and how efficiently it ran."

The Debrief tab answers: *How much did this session cost? Was it efficient? How did Claude perform?*

| Position | Current Metric | Status | Rec# | Recommended Metric | Story |
|----------|---------------|--------|------|--------------------|-------|
| **Hero Card** | Debrief (rotating headline) | **[LIVE]** | 1 | **Session Cost $** | Money. The single most important number for accountability. "$4.82 this session." Users immediately know what they spent. Everything else is detail. **Source**: prefers real `total_cost_usd` from status line hook; falls back to pricing table estimate. |
| **Hero Support** | turn count, total tokens, cache hit rate, tokens saved | **[LIVE]** | 2 | **Turns** · **Total Tokens** · **Cache Savings $** · **Output Speed** | Four-stat support. Turns = session length, Total tokens = volume, Cache savings = money NOT spent, Output speed = performance feel. |
| **Hero TL** | Input Tokens | **[LIVE]** | 3 | **Input Tokens** | Keep — the context cost. This is what the user is "feeding" Claude. Cyan. |
| **Hero TR** | Output Tokens | **[LIVE]** | 4 | **Output Tokens** | Keep — what Claude produced. Green. The two together show the I/O ratio. |
| **Hero BL** | Cache Reused | **[LIVE]** | 5 | **Cache Savings $** | Convert cache reuse to dollars saved. "$1.24 saved by cache" is more compelling than "82k cache tokens." The efficiency story in money. |
| **Hero BR** | Tokens Saved | **[LIVE]** | 6 | **Cost/Turn** | `sessionCost / turnCount` — "$0.19/turn." Gives a unit cost. Users can reason about it: "each question costs me $0.19." Powerful for budgeting. |
| **Stats 1** | Input Tokens | **[LIVE]** | 7 | **Throughput** | Overall session token production rate. `total_output_tokens / elapsed_session_seconds`. Counts every token Claude produced — main conversation, subagent tasks, tool-heavy turns, everything. "128 tok/s" means the session is churning through work. This is the raw horsepower number. **[CALC]** |
| **Stats 2** | Output Tokens | **[LIVE]** | 8 | **Conv Speed** | Conversational dialogue pace. Only measures the human↔AI exchange: user prompts + thinking + assistant responses. `Σ(conversation_chars) / 4 / conversation_wall_time`. Excludes tool execution and subagent overhead. "34 tok/s" reflects how fast the dialogue itself flows — the rhythm the user actually feels. **[CALC]** |
| **Stats 3** | Cache Read | **[LIVE]** | 9 | **Avg Turn Duration** | Seconds per turn. "18s avg." Complements output speed — one is throughput, this is latency. Both matter. |
| **Stats 4** | Cache Write | **[LIVE]** | 10 | **Tool Density** | Tools per turn. "3.2 tools/turn" shows how much work Claude does per prompt. High = complex tasks, low = conversational. |
| **Stats 5** | Cache Hit % | **[LIVE]** | 11 | **Amplification Ratio** | `outputChars / inputChars` — "47x amplification." For every 1 char the user types, Claude produces 47. Shows leverage. |
| *(new slot)* | — | — | 12 | **Model Mix** | "94% Opus / 4% Sonnet / 2% Haiku" — shows which engines ran. Color-coded. Affects cost interpretation. |
| *(overflow)* | — | — | — | **Cache Hit %** | Moved out of stats grid — near 100% in practice (Anthropic's prompt cache is aggressive), making it noise rather than signal. Available in hero support line or tooltip. |
| *(overflow)* | — | — | — | **Output Speed** (raw generation) | Raw Claude token generation rate (`output_tokens / durationMs` per turn, median). Replaced by the two-speed split: Throughput (everything) and Conv Speed (dialogue only). Available as tooltip on either speed metric. |
| *(overflow)* | — | — | — | **Input Tokens** | Raw count moved out of stats grid — already in hero metric TL. Tooltip or detail view. |
| *(overflow)* | — | — | — | **Output Tokens** | Raw count moved out — already in hero metric TR. Tooltip or detail view. |
| *(overflow)* | — | — | — | **Cache Read / Cache Write** | Raw counts moved out — replaced by Cache Savings $. Available in expanded view. |
| *(overflow)* | — | — | — | **Thinking Ratio** | `thinkingTokens / outputTokens` — interesting but niche. Tooltip on output tokens. |
| *(overflow)* | — | — | — | **Tool Success Rate** | `(bash − interrupted) / bash %` — interesting for debugging. Detail view. |
| *(overflow)* | — | — | — | **Compaction Count/Savings** | Context compaction events. Warning-level indicator — surface as alert badge, not stat card. |
| *(overflow)* | — | — | — | **Longest Turn / P50 Turn** | Duration distribution — better as a chart in the conversation feed area. |
| *(overflow)* | — | — | — | **Sub-Agent Spawns / Delegation Ratio** | Agent token overhead — detail metric for power users. Tooltip or expanded view. |
| *(overflow)* | — | — | — | **Web Search Count** | Niche. Tooltip on tool density or detail view. |

**Recommended story flow**: Session cost (accountability) → I/O tokens (what went in/out) → Cache savings + cost/turn (efficiency) → Throughput + conv speed + duration + tool density + amplification + model mix (performance profile). Two speed metrics tell different stories: throughput is raw horsepower (total work done), conv speed is dialogue rhythm (what the user feels). The tab evolves from "what did I spend" → "was it worth it" → "how did it perform."

---

### Arsenal Tab — "Across all sessions, here's the cumulative value aOa delivers."

The Arsenal tab answers: *Over my entire history with aOa, what's the total value? Is it getting better?*

| Position | Current Metric | Status | Rec# | Recommended Metric | Story |
|----------|---------------|--------|------|--------------------|-------|
| **Hero Card** | Arsenal (rotating headline) | **[LIVE]** | 1 | **Total $ Saved by aOa** | The lifetime value headline. "$14.82 saved across 42 sessions." This is the ROI number — why aOa exists. Token savings + cache savings + cost avoidance, all in dollars. |
| **Hero Support** | tokens saved, time saved, sessions extended, session count, guided ratio, counterfactual | **[LIVE]** | 2 | **Sessions** · **Lifetime Tokens Saved** · **Lifetime Time Saved** · **ROI Multiplier** | Four-stat support. Sessions = scale, Tokens saved = proof, Time saved = human benefit, ROI = "for every $1 spent, aOa saved $X." The complete value argument. |
| **Hero TL** | Tokens Saved | **[LIVE]** | 3 | **Lifetime Cost Avoidance $** | Tokens saved converted to dollars not spent. "$8.40 in avoided token costs." More tangible than raw token counts. |
| **Hero TR** | Sessions Extended | **[LIVE]** | 4 | **Sessions Extended** | Keep — total minutes of runway gained across all sessions. "+34 min of runway." Time is the scarce resource. |
| **Hero BL** | Unguided Cost | **[LIVE]** | 5 | **Lifetime Cache Savings $** | "$6.42 saved by caching." The second value stream. Between cost avoidance (TL) and cache savings (BL), the user sees two independent money-saving mechanisms. |
| **Hero BR** | Guided Ratio | **[LIVE]** | 6 | **Session Efficiency Score** | Composite 0–100: `(guidedRatio × 0.4 + cacheHitRate × 0.3 + savingsRate × 0.3) × 100`. A grade across all sessions. "Your efficiency: 74/100." Shows improvement trajectory. |
| **Stats 1** | Tokens Saved | **[LIVE]** | 7 | **Guided Ratio** (lifetime) | Overall % of reads that were guided. Green. The quality baseline — "72% of all reads were optimized." |
| **Stats 2** | Unguided Cost | **[LIVE]** | 8 | **Unguided Cost** | Keep — red-colored waste indicator. Tokens consumed by unguided reads. The "what you'd lose without aOa" number. Contrast with green guided ratio. |
| **Stats 3** | Sessions | **[LIVE]** | 9 | **Sessions** | Keep — blue. Total session count. Scale indicator. |
| **Stats 4** | Sessions Extended | **[LIVE]** | 10 | **Avg Prompts/Session** | Prompts per session. Shows session depth — "avg 23 prompts/session." Users can compare their sessions. Replace sessions extended (now in hero). |
| **Stats 5** | Guided Ratio | **[LIVE]** | 11 | **Edit Acceptance %** | `(edits − userModified) / edits × 100` — "91% of edits accepted as-is." Trust metric — Claude is writing code the user keeps. |
| **Stats 6** | Read Velocity | **[LIVE]** | 12 | **Read Velocity** | Keep — reads per prompt. Efficiency of information retrieval. Yellow. |
| *(overflow)* | — | — | — | **Command Success Rate** | `(bash − interrupted) / bash %` — reliability metric. Detail view or tooltip. |
| *(overflow)* | — | — | — | **File Read Hotspots** | Top N files by lifetime reads — list widget, not stat card. |
| *(overflow)* | — | — | — | **Tool Usage Distribution** | Stacked bar breakdown — visualization below the grid. |
| *(overflow)* | — | — | — | **Tokens Saved Trend** | Per-session sparkline — goes in the learning curve chart area. |
| *(overflow)* | — | — | — | **Guided Ratio Trend** | Per-session trend line — already exists in learning curve chart. |
| *(overflow)* | — | — | — | **Avg Session Duration** | Detail metric. Available in session history table. |
| *(overflow)* | — | — | — | **CLI Version History** | Niche. System status or tooltip. |
| *(overflow)* | — | — | — | **Lifetime Cost** (total spent) | Available as tooltip on Total $ Saved — "saved $14.82 of $48.30 total." |

**Recommended story flow**: Total $ saved (the punchline) → Cost avoidance + sessions extended + cache savings + efficiency score (four pillars of value) → Guided ratio + waste + sessions + depth + trust + velocity (operational proof). The tab is the closing argument: "aOa pays for itself."

---

### Cross-Tab / Global Metrics

These metrics span multiple tabs or represent the overall aOa value proposition.

| Metric | Source | Calculation | Best Surface | Story |
|--------|--------|-------------|-------------|-------|
| **Total $ Spent** | All token counts × model pricing | Opus: $15/$75 per 1M; Sonnet: $3/$15; Haiku: $0.80/$4. Cache read = 10% of input | Arsenal hero tooltip | Baseline for ROI calculation |
| **Total $ Saved by aOa** | Token savings + cache savings + cost avoidance | `tokenSavings$ + cacheSavings$ + costAvoidance$` | Arsenal hero card | The punchline. Why aOa exists |
| **ROI Multiplier** | Value delivered / cost | `$ saved / $ spent` | Arsenal hero support | "For every $1 of tokens, aOa saved $0.31" |
| **Session Streak** | Consecutive sessions with guided ratio > 60% | Count of back-to-back efficient sessions | Arsenal — gamification badge | Engagement hook. Reward consistency |
| **Learning Velocity** | Domains over total prompts | Slope of `domainCount` vs `promptCount` | Intel hero support | Shows aOa getting smarter over time |
| **Angle of Attack Score** | Composite across all dimensions | Weighted: Intel depth + Arsenal efficiency + Recon coverage + Debrief cost | Live hero card subtitle | Single KPI — the meta-metric |

---

### Pricing Reference (for cost calculations)

| Model | Input (per 1M) | Output (per 1M) | Cache Read (per 1M) | Cache Write (per 1M) |
|-------|----------------|------------------|----------------------|----------------------|
| claude-opus-4-6 | $15.00 | $75.00 | $1.50 | $18.75 |
| claude-sonnet-4-6 | $3.00 | $15.00 | $0.30 | $3.75 |
| claude-haiku-4-5 | $0.80 | $4.00 | $0.08 | $1.00 |

---

### Data Availability Summary

| Data Point | Already Captured | In JSONL | In Status Hook | Needs New Code |
|-----------|:---:|:---:|:---:|:---:|
| Token counts per turn | Y | Y | — | — |
| Turn duration (ms) | Y | Y | — | — |
| Tool invocations + params | Y | Y | — | — |
| User/AI message text | Y | Y | — | — |
| Model per turn | Y | Y | Y | — |
| Cache read/write counts | Y | Y | — | — |
| **Context window position** | **Y** | — | **Y** | — |
| **Context window max** | **Y** | — | **Y** | — |
| **Context utilization %** | **Y** | — | **Y** | — |
| **Real session cost USD** | **Y** | — | **Y** | — |
| **Session wall-clock duration** | **Y** | — | **Y** | — |
| **API call duration** | **Y** | — | **Y** | — |
| **Lines added/removed** | **Y** | — | **Y** | — |
| **Claude Code version** | **Y** | Y | **Y** | — |
| Context compaction events | — | Y | — | Extract from system events |
| Sub-agent token usage | — | Y | — | Extract from toolUseResult |
| Throughput (tok/sec) | — | Y | Y | `total_output_tokens / elapsed_seconds` — uses session metrics + timestamps or `total_duration_ms` from status hook |
| Conv speed (tok/sec) | — | Y | — | `Σ(user+thinking+response chars) / 4 / conversation_wall_time` — filter feed to conversation events only |
| User think time | — | Y | — | Timestamp gap analysis |
| Edit acceptance rate | — | Y | — | Read toolUseResult.userModified |
| Bash success rate | — | Y | — | Read toolUseResult.interrupted/stderr |
| Web search frequency | — | Y | — | Read usage.server_tool_use |
| Cost/pricing calculations | Y | — | Y | Real cost from hook; estimate as fallback |
| Domain freshness timestamps | — | — | — | Add timestamp to observe() calls |
| Historical domain snapshots | — | — | — | Snapshot DomainMeta at each autotune |
| Signal source tagging | — | — | — | Tag observe() with source enum |

---

## Detailed Metric Definitions (existing)

## Live Tab — `/api/runway` + `/api/stats`

### Hero Card

| Metric | JSONL Origin | Calculation | Stored |
|--------|-------------|-------------|--------|
| **Tokens Saved** | `message.usage.input_tokens`, `output_tokens`, `cache_read_input_tokens` on assistant messages | For each range-gated Read (0 < limit < 500): `fileBytes/4 − (limit×20)`. Accumulated when savings ≥ 50%. Sum of prior sessions + current. | `SessionSummary.TokensSaved` → bbolt; current: `app.counterfactTokensSaved` |
| **Time Saved** | Same token fields + turn `durationMs` (system event) | `tokensSaved × P50(ms/token)` where P50 is median ms/token from last 30 min of valid turns (min 5 samples). Fallback: 7.5 ms/token. | `SessionSummary.TimeSavedMs` → bbolt; current: `app.sessionTimeSavedMs` |
| **Guided Ratio** | `ToolInvocation` Read events with `limit` field | `guidedReads / totalReads`. A read is "guided" if offset+limit partial read saves ≥ 50% vs full file. | `SessionSummary.GuidedRatio` → bbolt |
| **Extended Runway** | Same token fields (burn rate inputs) | `(remaining / actualBurnRate) − (remaining / counterfactBurnRate)` in minutes. Counterfact rate = actual minus guided-read token savings. | Ephemeral — recalculated per request |
| **Runway** | Same token fields | `(contextWindowMax − lifetimeTotalTokens) / TokensPerMin`. `contextWindowMax` from status line hook `ctx_max` (real), fallback to hardcoded model map (`internal/app/models.go`), default 200k. | Ephemeral |
| **Counterfact Runway** | Same token fields | Same formula but using counterfactual burn rate (what would burn without guided reads) | Ephemeral |

### Stats Grid

| Metric | Origin | Calculation | Stored |
|--------|--------|-------------|--------|
| **Context Util%** | Status line hook `used_pct` | Real `ctx_used / ctx_max × 100` from Claude Code. Fallback: `-` if no snapshot. | Ring buffer (last 5 snapshots) |
| **Burn Rate** | `message.usage.*_tokens` | `sum(tokens in 5-min window) / windowDuration.Minutes()` via `BurnRateTracker` | Ephemeral rolling window |
| **Guided Ratio** | Read `ToolInvocation` events | `guidedReads / totalReads` across all sessions + current | bbolt sessions + ephemeral |
| **Output Speed** | `ms_per_token` from `RateTracker` | `1000 / ms_per_token` tokens/sec | Ephemeral |
| **Cache Hit%** | `message.usage.cache_read_input_tokens` | `CacheReadTokens / (InputTokens + CacheReadTokens)` | `SessionMetrics.CacheHitRate()` |
| **Autotune Progress** | `user` message events | `promptN % 50` — resets at each autotune cycle | Ephemeral |

---

## Recon Tab — `/api/recon`

Source is the **project file system** — recon does not read from JSONL.

### Hero Card + Stats Grid

| Metric | True Origin | Calculation | Stored |
|--------|-------------|-------------|--------|
| **Files Scanned** | Project file system (walked by `aoa-recon enhance`) | Count of code files processed (filtered by extension, skips node_modules etc.) | `FileAnalysis` → bbolt `dimensions` bucket |
| **Total Findings** | Source file content (AC text scan + tree-sitter AST walk) | Count of all `RuleFinding` across all files | bbolt `dimensions` bucket |
| **Critical** | Same file content | Count where `Severity == SevCritical` (weight=10) | bbolt `dimensions` bucket |
| **Warnings** | Same file content | Count where `Severity == SevWarning` (weight=3) or `SevHigh` (weight=7) | bbolt `dimensions` bucket |
| **Clean Files** | Same file content | Count of files with zero findings | Computed at API time from bbolt data |
| **Active Dimensions** | Browser localStorage | Count of dimension toggles the user has enabled | `localStorage('aoa-recon-dims')` — never leaves browser |

---

## Intel Tab — `/api/stats` + `/api/domains`

All Intel metrics originate from text in the JSONL session file, processed through the enricher atlas.

### Hero Card + Stats Grid

| Metric | JSONL Origin | Calculation | Stored |
|--------|-------------|-------------|--------|
| **Domains** | Search queries, grep patterns, Read file paths, conversation text | `len(DomainMeta)` — each unique `@domain` resolved by enricher atlas from tokens | Learner state → bbolt |
| **Core** | Same | Count of domains where `Tier == "core"` (promoted by autotune competitive displacement) | Learner state → bbolt |
| **Terms** | Same | `len(TermHits)` — unique semantic terms resolved from tokens via atlas | Learner state → bbolt |
| **Keywords** | Same | `len(KeywordHits)` — unique raw tokens that survived pruning | Learner state → bbolt |
| **Bigrams** | `user` message text, AI `thinking` text, AI `text` response | `len(Bigrams)` — co-occurring word pairs from conversation text | Learner state → bbolt |
| **Total Hits** | Same | `sum(domain.Hits)` for all domains — each `Observe()` call adds 1.0, decayed at autotune by ×0.90 (float64, no truncation) | Client-side sum from `/api/domains` |

**What feeds Observe():**
- Search result hits → domains + terms from matched symbol tags
- Grep pattern tokens → keywords → enricher → terms → domains
- Range-gated Read file paths → `FileHits`
- Conversation text (user + thinking + response) → bigrams only

---

## Debrief Tab — `/api/conversation/metrics` + `/api/runway` + `/api/conversation/tools`

Token metrics from the `usage` object on Claude API responses. Cost metrics prefer real `total_cost_usd` from status line hook, with pricing table estimate as fallback.

### Hero Card + Stats Grid

| Metric | JSONL Field | Calculation | Stored |
|--------|------------|-------------|--------|
| **Input Tokens** | `message.usage.input_tokens` on assistant entries | Direct accumulation per `EventAIResponse` | Ephemeral `sessionMetrics.InputTokens`; flushed to `SessionSummary` → bbolt at session end |
| **Output Tokens** | `message.usage.output_tokens` | Direct accumulation | Same |
| **Cache Read** | `message.usage.cache_read_input_tokens` | Direct accumulation | Same |
| **Cache Write** | `message.usage.cache_creation_input_tokens` | Direct accumulation | Same |
| **Turn Count** | `user` message entries | Incremented on each `EventUserInput` | Ephemeral `sessionMetrics.TurnCount` |
| **Total Tokens** | Same four `usage` fields | `InputTokens + OutputTokens` (client-side sum; cache tokens excluded) | Client-side only |
| **Cache Hit Rate** | Same `usage` fields | `CacheReadTokens / (InputTokens + CacheReadTokens)` — returns 0.0 if total is 0 | Computed in `SessionMetrics.CacheHitRate()` |

---

## Arsenal Tab — `/api/sessions` + `/api/config` + `/api/runway`

### Hero Card + Stats Grid

| Metric | True Origin | Calculation | Stored |
|--------|-------------|-------------|--------|
| **Tokens Saved** | `message.usage.*_tokens` + file sizes from index | `fileBytes/4 − limit×20` per guided read, accumulated lifetime across sessions | `SessionSummary.TokensSaved` → bbolt; summed client-side across all sessions + current |
| **Time Extended** | Same token fields + turn duration from JSONL | `(remaining/actualRate) − (remaining/counterfactRate)` in minutes | Ephemeral per request |
| **Unguided Cost** | `ToolInvocation` Read events (all reads, guided + not) | `(totalReads − guidedReads) × 200 tokens` — flat 200 tok/read estimate for unguided reads | Client-side calculation from bbolt session totals |
| **Guided Ratio** | Read `ToolInvocation` events with limit field | `sum(guided_read_count) / sum(read_count)` across all sessions | `SessionSummary.GuidedRatio` → bbolt |
| **Sessions** | Session boundary = new session ID in JSONL filename | Count of persisted `SessionSummary` records | bbolt `sessions` bucket |
| **Read Velocity** | Read `ToolInvocation` events + user messages | `sum(read_count) / sum(prompt_count)` across all sessions | Client-side from bbolt session totals |

### System Status Strip

| Metric | True Origin | Calculation | Stored |
|--------|-------------|-------------|--------|
| **Uptime** | Daemon process start time | `time.Since(app.started)` | Ephemeral — set when daemon starts |
| **DB Path** | CLI flag `--db` or default `.aoa/aoa.db` | String display | `app.dbPath` |
| **Index Files** | Project file system (walked at init/reindex) | `len(idx.Files)` | Index → bbolt |
| **Index Tokens** | Same file walk + tokenization | `len(idx.Tokens)` — unique tokens across all indexed files | Index → bbolt |
| **Project ID** | `sha256(projectRoot)[:12]` | First 12 chars displayed | Derived at startup |

---

## Master Origin Table

### Session JSONL (`~/.claude/projects/.../*.jsonl`)

| JSONL Field | What It Is | Feeds |
|-------------|-----------|-------|
| `message.usage.input_tokens` | Tokens in Claude's context window for that turn | Burn rate, runway, session totals, cache hit rate |
| `message.usage.output_tokens` | Tokens Claude generated | Burn rate, runway, session totals, time-saved rate |
| `message.usage.cache_read_input_tokens` | Tokens served from prompt cache | Cache hit rate, session totals |
| `message.usage.cache_creation_input_tokens` | Tokens written to prompt cache | Session totals |
| `tool_use.input.file_path` + `offset` + `limit` | Read tool arguments | Guided ratio, tokens saved, learner FileHits |
| `tool_use.input.pattern` (grep/egrep) | Grep pattern string | Learner keywords → terms → domains via enricher |
| User message `content.text` | User's typed message | Learner bigrams, PromptCount |
| AI `content[thinking].thinking` | Claude's internal reasoning text | Learner bigrams only |
| AI `content[text].text` | Claude's response text | Learner bigrams only |
| `message.model` | Model name (e.g. `claude-opus-4-6`) | Context window size lookup |
| Session JSONL filename / `sessionId` | UUID in path | Session boundary detection |
| Project file system (not JSONL) | Source code files | Recon findings, index files/tokens |

### Status Line Hook (`.aoa/context.jsonl`)

| Field | What It Is | Feeds |
|-------|-----------|-------|
| `ctx_used` | Real context window token position | Live: Context Util%, support line (122k/200k) |
| `ctx_max` | Real context window max | Live: Context Util% denominator, overrides model lookup |
| `used_pct` | Real utilization % from Claude Code | Live: Stats 1, support line |
| `remaining_pct` | Remaining context % | Live: color coding (green/yellow/red thresholds) |
| `total_cost_usd` | Real session cost from Claude | Debrief: Session Cost hero, cost/turn; Arsenal: ROI calc |
| `total_duration_ms` | Session wall-clock time | Available for session pacing metrics |
| `total_api_duration_ms` | Time spent in API calls | Available for API efficiency (api_time / wall_time) |
| `total_lines_added` | Code lines added this session | Available for productivity metrics |
| `total_lines_removed` | Code lines removed this session | Available for churn metrics |
| `ts` | Capture timestamp | Rate calculations (context velocity, cost velocity) |

---

## Claude Session File Elements

Complete schema of every field and event type found in `~/.claude/projects/{encoded-path}/*.jsonl`. Each line is a JSON object. Fields vary by event type.

### Top-Level Fields (all events)

| Field | Type | Description |
|-------|------|-------------|
| `sessionId` | `string` (UUID) | Unique session identifier, matches the JSONL filename |
| `type` | `string` | Event type: `"user"`, `"assistant"`, `"progress"`, `"queue-operation"`, `"system"` |
| `timestamp` | `string` (ISO 8601) | When the event was recorded |

### Top-Level Fields (most events, absent on `queue-operation`)

| Field | Type | Description |
|-------|------|-------------|
| `parentUuid` | `string \| null` | UUID of the previous event in the conversation chain; `null` for the first message |
| `uuid` | `string` (UUID) | Unique identifier for this event |
| `isSidechain` | `boolean` | Whether this event is on a side conversation branch (e.g., agent sub-task) |
| `userType` | `string` | Always `"external"` for CLI sessions |
| `cwd` | `string` | Working directory when event was emitted |
| `version` | `string` | Claude Code CLI version (e.g., `"2.1.49"`) |
| `gitBranch` | `string` | Active git branch at time of event |
| `slug` | `string` | Human-readable session name (e.g., `"luminous-juggling-thunder"`) |

### Conditional Top-Level Fields

| Field | Type | Appears On | Description |
|-------|------|-----------|-------------|
| `message` | `object` | `user`, `assistant` | The conversation message payload (see Message Object below) |
| `requestId` | `string` | `assistant` | Claude API request ID (e.g., `"req_011CYL..."`) |
| `data` | `object` | `progress` | Sub-agent/tool execution progress payload (see Progress Data below) |
| `toolUseID` | `string \| null` | `progress` | Tool use ID this progress event relates to |
| `parentToolUseID` | `string \| null` | `progress` | Parent tool use ID (for nested tool calls) |
| `toolUseResult` | `object` | `user` (tool results) | Structured result from a tool execution (see Tool Use Result below) |
| `sourceToolAssistantUUID` | `string` | `user` (tool results) | UUID of the assistant event that invoked the tool |
| `operation` | `string` | `queue-operation` | Queue action: `"enqueue"`, `"dequeue"`, `"remove"` |
| `content` | `string` | `queue-operation`, `system` | JSON string (queue-op) or XML/text (system) content |
| `subtype` | `string` | `system` | System event kind: `"turn_duration"`, `"local_command"`, `"microcompact_boundary"` |
| `durationMs` | `number` | `system` (turn_duration) | Duration of the completed turn in milliseconds |
| `isMeta` | `boolean` | `system` | Whether this is a meta/internal system event |
| `level` | `string` | `system` | Log level: `"info"` |
| `permissionMode` | `string` | `user` | Active permission mode when user sent message (e.g., `"acceptEdits"`) |
| `planContent` | `string` | `user` | Content of a plan when user submits one for execution |
| `todos` | `array` | `user` | Task/todo list state at time of message (can be empty `[]`) |
| `error` | `object` | `assistant` (errors) | Error details when a request fails |
| `isApiErrorMessage` | `boolean` | `assistant` (errors) | `true` when the assistant message is a synthetic API error |
| `microcompactMetadata` | `object` | `system` (microcompact) | Context compaction details (see below) |

---

### Event Types

#### `type: "user"` — User messages and tool results

Two variants:

**1. User text input** — user typed a message:
```json
{
  "type": "user",
  "message": { "role": "user", "content": "the user's message text" },
  "permissionMode": "acceptEdits",
  "planContent": "...",
  "todos": []
}
```

**2. Tool result** — system returning a tool's output:
```json
{
  "type": "user",
  "message": {
    "role": "user",
    "content": [{ "tool_use_id": "toolu_...", "type": "tool_result", "content": "..." }]
  },
  "toolUseResult": { ... },
  "sourceToolAssistantUUID": "..."
}
```

#### `type: "assistant"` — Claude's responses

Each assistant event contains a `message` object with one content block. A single API response is split across multiple JSONL entries — one per content block (thinking, text, tool_use). All share the same `message.id` and `requestId`.

```json
{
  "type": "assistant",
  "requestId": "req_...",
  "message": { "model": "...", "id": "msg_...", "role": "assistant", "content": [...], "usage": {...} }
}
```

**Error variant** — when the API call fails or context is too long:
```json
{
  "type": "assistant",
  "error": { ... },
  "isApiErrorMessage": true,
  "message": { "model": "<synthetic>", "content": [{"type": "text", "text": "Prompt is too long"}] }
}
```

#### `type: "progress"` — Sub-agent and tool execution progress

Streams intermediate output from running tools. Contains a `data` object with sub-type:

| `data.type` | Description |
|-------------|-------------|
| `bash_progress` | Streaming stdout/stderr from a Bash command |
| `agent_progress` | Sub-agent (Task tool) intermediate messages |
| `hook_progress` | Hook script execution progress |
| `waiting_for_task` | Waiting for a background task to complete |

#### `type: "queue-operation"` — Task queue management

Tracks background task lifecycle:
```json
{
  "type": "queue-operation",
  "operation": "enqueue",
  "content": "{\"task_id\":\"a7af43d\",\"tool_use_id\":\"toolu_...\",\"description\":\"...\",\"task_type\":\"local_agent\"}"
}
```
Operations: `enqueue` (task started), `dequeue` (task picked up), `remove` (task finished/cancelled).

#### `type: "system"` — System/meta events

| Subtype | Fields | Description |
|---------|--------|-------------|
| `turn_duration` | `durationMs` | Marks end of a turn with its wall-clock duration in ms |
| `local_command` | `content` (XML) | User invoked a slash command (e.g., `/usage`, `/help`) |
| `microcompact_boundary` | `microcompactMetadata` | Context window was auto-compacted to save tokens |

---

### Message Object (`message`)

Present on `user` and `assistant` events.

| Field | Type | Description |
|-------|------|-------------|
| `role` | `string` | `"user"` or `"assistant"` |
| `content` | `string \| array` | User text (string) or array of content blocks |
| `model` | `string` | Model used (assistant only): `"claude-opus-4-6"`, `"claude-sonnet-4-6"`, `"claude-haiku-4-5-20251001"`, or `"<synthetic>"` for errors |
| `id` | `string` | Claude API message ID (e.g., `"msg_015T..."`) |
| `type` | `string` | Always `"message"` |
| `stop_reason` | `string \| null` | `null` (streaming), `"tool_use"`, `"stop_sequence"`, `"end_turn"` |
| `stop_sequence` | `string \| null` | The stop sequence that triggered, if any |
| `usage` | `object` | Token usage for this response (see Usage Object) |
| `context_management` | `object \| null` | Context management metadata (rarely populated) |

### Content Block Types (`message.content[]`)

| `type` | Key Fields | Description |
|--------|-----------|-------------|
| `text` | `text` | Claude's response text or user message |
| `thinking` | `thinking`, `signature` | Claude's internal reasoning (extended thinking). `signature` is a cryptographic verification string |
| `tool_use` | `id`, `name`, `input`, `caller` | Claude invoking a tool. `caller.type` is always `"direct"` |
| `tool_result` | `tool_use_id`, `content` | Result returned from a tool execution |

### Usage Object (`message.usage`)

| Field | Type | Description |
|-------|------|-------------|
| `input_tokens` | `number` | Tokens in Claude's input context for this turn |
| `output_tokens` | `number` | Tokens Claude generated |
| `cache_read_input_tokens` | `number` | Tokens served from prompt cache (cache hit) |
| `cache_creation_input_tokens` | `number` | Tokens written to prompt cache (cache miss/creation) |
| `cache_creation` | `object` | Breakdown: `{ ephemeral_5m_input_tokens, ephemeral_1h_input_tokens }` |
| `service_tier` | `string \| null` | API service tier: `"standard"` or `null` |
| `inference_geo` | `string \| null` | Inference geography (currently `"not_available"`) |
| `server_tool_use` | `object` | Server-side tool usage: `{ web_search_requests, web_fetch_requests }` |
| `iterations` | `any \| null` | Reserved, currently `null` |
| `speed` | `any \| null` | Reserved, currently `null` |

---

### Tool Use Result Object (`toolUseResult`)

Attached to `user` events that carry tool results. Shape varies by tool:

| Tool | Key Fields | Description |
|------|-----------|-------------|
| **Read** | `type`, `filePath`, `content`, `structuredPatch`, `originalFile` | File read result with content |
| **Write** | `type`, `filePath`, `structuredPatch`, `originalFile` | File creation result |
| **Edit** | `type`, `filePath`, `oldString`, `newString`, `replaceAll`, `userModified`, `structuredPatch`, `originalFile` | File edit with before/after strings |
| **Bash** | `stdout`, `stderr`, `interrupted`, `isImage`, `noOutputExpected` | Command execution output |
| **Glob** | `mode`, `numFiles`, `filenames`, `numLines` | File pattern match results |
| **Grep** | `mode`, `numFiles`, `filenames`, `numLines` | Content search results |
| **Task** | `status`, `prompt`, `agentId`, `content`, `totalDurationMs`, `totalTokens`, `totalToolUseCount`, `usage` | Sub-agent completion with token accounting |
| **TaskCreate** | `task.id`, `task.subject` | New task created |
| **TaskUpdate** | `updatedFields`, `statusChange` | Task status change |
| **TaskOutput** | `backgroundTaskId`, `retrieval_status` | Background task output retrieval |
| **TaskStop** | `success`, `task_id`, `task_type`, `command` | Background task termination |
| **AskUserQuestion** | `questions`, `answers` | User Q&A interaction |

---

### Progress Data Object (`data` in progress events)

| Field | Type | Appears On | Description |
|-------|------|-----------|-------------|
| `type` | `string` | All | `"bash_progress"`, `"agent_progress"`, `"hook_progress"`, `"waiting_for_task"` |
| `message` | `object` | `agent_progress`, `bash_progress` | Nested event from the sub-agent/tool (contains its own `type`, `message`, `uuid`, `timestamp`) |
| `normalizedMessages` | `array` | `agent_progress` | Processed message chain |
| `prompt` | `string` | `agent_progress` | Original prompt sent to the sub-agent |
| `output` | `string` | `bash_progress` | Incremental stdout/stderr text |
| `fullOutput` | `string` | `bash_progress` | Complete stdout/stderr accumulated so far |
| `command` | `string` | `bash_progress` | The bash command being executed |
| `totalBytes` | `number` | `bash_progress` | Total bytes of output so far |
| `totalLines` | `number` | `bash_progress` | Total lines of output so far |
| `elapsedTimeSeconds` | `number` | `bash_progress` | Seconds since command started |
| `timeoutMs` | `number` | `bash_progress` | Timeout configured for the command |
| `hookEvent` | `string` | `hook_progress` | Hook event name |
| `hookName` | `string` | `hook_progress` | Hook script name |
| `agentId` | `string` | `agent_progress` | Sub-agent identifier |
| `taskId` | `string` | `waiting_for_task` | Task being waited on |
| `taskDescription` | `string` | `waiting_for_task` | Description of the task being waited on |
| `taskType` | `string` | `waiting_for_task` | Type of task (e.g., `"local_agent"`) |

---

### Microcompact Metadata (`microcompactMetadata`)

| Field | Type | Description |
|-------|------|-------------|
| `trigger` | `string` | What triggered compaction: `"auto"` |
| `preTokens` | `number` | Token count before compaction |
| `tokensSaved` | `number` | Tokens removed by compaction |
| `compactedToolIds` | `array[string]` | Tool use IDs whose results were compacted |
| `clearedAttachmentUUIDs` | `array[string]` | Attachment UUIDs cleared during compaction |

---

### Queue Operation Content (parsed from `content` JSON string)

| Field | Type | Description |
|-------|------|-------------|
| `task_id` | `string` | Short hash identifier for the queued task |
| `tool_use_id` | `string` | Tool use ID that spawned this task |
| `description` | `string` | Human-readable task description |
| `task_type` | `string` | `"local_agent"` (sub-agent) or `"local_bash"` (background shell) |
