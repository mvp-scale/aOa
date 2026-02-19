# Work Board

[Board](#board) | [Supporting Detail](#supporting-detail) | [Completed](.context/COMPLETED.md) | [Backlog](.context/BACKLOG.md)

> **Updated**: 2026-02-19 (Session 48) | **Phase**: Dashboard value metrics & 5-tab restructure
> **Completed work**: See [COMPLETED.md](.context/COMPLETED.md) — Phases 1–8c (~260 active tests, 291 declared, 88% context)

---

## Goals

> Atomic architectural principles. Every task is evaluated against each goal independently.

| Goal | Statement |
|------|-----------|
| **G0** | **Speed** — 50-120x faster than Python. Sub-millisecond search, <200ms startup, <50MB memory. |
| **G1** | **Parity** — Zero behavioral divergence from Python. Test fixtures are the source of truth. |
| **G2** | **Single Binary** — One `aoa` binary. Zero Docker, zero runtime deps, zero install friction. |
| **G3** | **Agent-First** — Replace `grep`/`find` transparently for AI agents. Minimize prompt education tax. |
| **G4** | **Clean Architecture** — Hexagonal. Domain logic is dependency-free. External concerns behind interfaces. |
| **G5** | **Self-Learning** — Adaptive pattern recognition. observe(), autotune, competitive displacement. |
| **G6** | **Value Proof** — Surface measurable savings. Context runway, tokens saved, sessions extended. |

---

## Board Structure

> Layered architecture. Each layer builds on the one below. TDD — validation gates at every layer.

### Layers

| Layer | Name | Purpose | Gate Method |
|-------|------|---------|-------------|
| **L0** | Value Engine | Burn rate, context runway, attribution signals | Runway API returns valid projections; attribution rubric covers all tool actions |
| **L1** | Dashboard | 5-tab layout, mockup implementation, hero narratives | All 5 tabs render with live data; mockup parity validated in browser |
| **L2** | Infra Gaps | File watcher wiring, bbolt lock fix, missing CLI flags | `aoa init` works while daemon runs; file changes trigger re-index |
| **L3** | Migration | Parallel run Python vs Go, parity proof | 100 queries × 5 projects = zero divergence; benchmark confirms speedup |
| **L4** | Distribution | Goreleaser, grammar loader, install docs | `go install` or binary download works on linux/darwin × amd64/arm64 |
| **L5** | Dimensional Analysis | Bitmask engine, 6-tier scanning, Recon tab | Security tier catches known vulns in test projects; query time < 10ms |

### Columns

| Column | Purpose |
|--------|---------|
| **Layer** | Layer grouping. Links to layer detail below. |
| **ID** | Task identifier (layer.step). Links to task reference below. |
| **G0-G6** | Goal alignment. `x` = serves this goal. Blank = not relevant. |
| **Dep** | ID of blocking task, or `-` |
| **Cf** | Confidence — see indicator reference below |
| **St** | Status — see indicator reference below |
| **Va** | Validation state — see indicator reference below |
| **Task** | What we're doing |
| **Value** | Why we're doing this |
| **Va Detail** | How we prove it |

### Indicator Reference

| Indicator | Cf (Confidence) | St (Status) | Va (Validation) |
|:---------:|:----------------|:------------|:----------------|
| ⚪ | — | Not started | Not yet validated |
| 🔵 | — | In progress | — |
| 🟢 | Confident | Complete | Validated |
| 🟡 | Uncertain | Pending | Needs test strategy |
| 🔴 | Lost/Blocked | Blocked | Failed |

> 🟢🟢🟢 = done. Task moves to COMPLETED.md.

---

## Mission

**North Star**: One binary that makes every AI agent faster by replacing slow, expensive tool calls with O(1) indexed search — and proves it with measurable savings.

**Current**: Core engine complete (search, learner, session integration, CLI, dashboard). ~260 active tests (291 declared). Now building the value metrics layer and restructuring the dashboard to surface savings.

**Approach**: TDD. Each layer validated before the next. Completed work archived to keep the board focused on what's next.

**Design Decisions Locked** (Session 48):
- **aOa Score** — Deferred. Not defined until data is flowing. Intel tab cleaned of coverage/confidence/momentum card.
- **Arsenal = Value Proof Over Time** — Not a config/status page. Session-centric value evidence: actual vs counterfactual token usage, learning curve, per-session savings. System status is compact/secondary.
- **Session as unit of measurement** — Each session gets a summary record (ID, date, prompts, reads, guided ratio, tokens saved, counterfactual). Multiple sessions per day. Chart is daily rollup, table is individual sessions.
- **Counterfactual is defensible** — "Without aOa" = sum of full file sizes for guided reads + observed unguided costs. Not fabricated.

**Needs Discussion** (before L1 implementation):
- **Alias strategy** — Goal is replacing `grep` itself. `grep auth` → `aoa grep auth` transparently. Graceful degradation on unsupported flags?
- **Real-time conversation** — Legacy Python showed real-time; Go dashboard with 2s poll should do better. Needs investigation.

---

## Board

| Layer | ID | G0 | G1 | G2 | G3 | G4 | G5 | G6 | Dep | Cf | St | Va | Task | Value | Va Detail |
|:------|:---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:----|:--:|:--:|:--:|:-----|:------|:----------|
| [L0](#layer-0) | [L0.1](#l01) | x | | | | | | x | - | 🟢 | ⚪ | ⚪ | Burn rate accumulator — rolling window tokens/min | Foundation for all savings metrics | Unit test: accumulator tracks rate over 5m window |
| [L0](#layer-0) | [L0.2](#l02) | | | | | | | x | - | 🟢 | ⚪ | ⚪ | Context window max lookup — map model tag to window size | Needed for runway projection | Lookup returns correct max for claude-3, gpt-4 |
| [L0](#layer-0) | [L0.3](#l03) | | | | | | | x | L0.1 | 🟢 | ⚪ | ⚪ | Dual projection — with-aOa vs without-aOa burn rates | The core value comparison | Two projections diverge correctly under test load |
| [L0](#layer-0) | [L0.4](#l04) | | | | | x | | x | L0.3 | 🟢 | ⚪ | ⚪ | Context runway API — `/api/runway` with both projections | Dashboard and CLI can show runway | API returns JSON with both projections and delta |
| [L0](#layer-0) | [L0.5](#l05) | | | | | x | | x | L0.3 | 🟢 | ⚪ | ⚪ | Session summary persistence — per-session metrics in bbolt | Arsenal value proof, survives restart | Session record persists with tokens saved, guided ratio, counterfactual |
| [L0](#layer-0) | [L0.6](#l06) | | | | | | x | x | - | 🟢 | ⚪ | ⚪ | Verify autotune fires every 50 prompts | Trust the learning cycle is working | Integration test: 50 prompts → autotune triggers |
| [L0](#layer-0) | [L0.7](#l07) | | | | | | x | x | L0.6 | 🟢 | ⚪ | ⚪ | Autotune activity event — "cycle N, +P/-D/~X" | Visible learning progress in activity feed | Activity entry appears with correct promote/demote counts |
| [L0](#layer-0) | [L0.8](#l08) | | | | | | | x | - | 🟢 | ⚪ | ⚪ | Write/Edit attrib = "productive" | Credit productive work correctly | Write/Edit tool events get "productive" attrib |
| [L0](#layer-0) | [L0.9](#l09) | | | | | | | x | - | 🟢 | ⚪ | ⚪ | Glob attrib = "unguided" + estimated token cost | Show cost of not using aOa | Glob events get "unguided" + token estimate |
| [L0](#layer-0) | [L0.10](#l010) | | | | | | | x | - | 🟢 | ⚪ | ⚪ | Grep (Claude) impact = estimated token cost | Show cost of not using aOa | Claude grep events show estimated tokens |
| [L0](#layer-0) | [L0.11](#l011) | | | | | | x | | - | 🟢 | ⚪ | ⚪ | Learn activity event — observe signals summary | Visible learning in feed | Activity entry: "+N keywords, +M terms, +K domains" |
| [L0](#layer-0) | [L0.12](#l012) | | | | | | | x | - | 🟢 | ⚪ | ⚪ | Target capture — preserve full query syntax, no normalization | Accurate activity display | Target column shows raw query as entered |
| [L1](#layer-1) | [L1.1](#l11) | | | | | | | x | - | 🟢 | ⚪ | ⚪ | Rename tabs: Overview→Live, Learning→Intel, Conversation→Debrief | Brand alignment — tabs named by user intent | Tabs render with new names |
| [L1](#layer-1) | [L1.2](#l12) | | | | | | | x | L1.1 | 🟢 | ⚪ | ⚪ | Add Recon tab (stub) — dimensional placeholder | Reserve the tab slot for v2 | Tab renders with placeholder content |
| [L1](#layer-1) | [L1.3](#l13) | | | | | | | x | L1.1 | 🟢 | ⚪ | ⚪ | Add Arsenal tab — value proof over time, session history, savings chart | Prove aOa's ROI across sessions | Shows actual vs counterfactual, learning curve, session table |
| [L1](#layer-1) | [L1.4](#l14) | | | | | | | | L1.1 | 🟢 | ⚪ | ⚪ | 5-tab header layout — responsive at <800px | Works on all screen sizes | Tabs don't wrap or overflow at narrow width |
| [L1](#layer-1) | [L1.5](#l15) | | | | | x | | x | L0.5, L1.3 | 🟢 | ⚪ | ⚪ | Arsenal API — `/api/sessions` for session summaries + `/api/config` | Backend for Arsenal charts and system strip | API returns session array with savings data + system config |
| [L1](#layer-1) | [L1.6](#l16) | x | | | | | | x | L0.4 | 🟢 | ⚪ | ⚪ | Live tab hero — context runway as primary display | Lead with the value prop | Hero shows "47 min remaining" with dual projection |
| [L1](#layer-1) | [L1.7](#l17) | | | | | | | x | L0.4 | 🟢 | ⚪ | ⚪ | Live tab metrics panel — savings-oriented cards | Replace vanity metrics with value | Cards show rolling avg speed, tokens saved, guided ratio |
| [L1](#layer-1) | [L1.8](#l18) | | | | | | | x | L0.9, L0.10 | 🟢 | ⚪ | ⚪ | Dashboard: render token cost for Grep/Glob | Show unguided cost inline | Red-coded cost appears in activity impact column |
| [L2](#layer-2) | [L2.1](#l21) | x | | | | x | x | | - | 🟢 | ⚪ | ⚪ | Wire file watcher — `Watch()` in app.go, change→reparse→reindex | Dynamic re-indexing without restart | File edit triggers re-index within 2s |
| [L2](#layer-2) | [L2.2](#l22) | x | | | | x | | | - | 🟢 | ⚪ | ⚪ | Fix bbolt lock contention — in-process reindex via socket command | `aoa init` works while daemon runs | Init succeeds with daemon running |
| [L2](#layer-2) | [L2.3](#l23) | | x | | x | | | | - | 🟢 | ⚪ | ⚪ | Implement `--invert-match` / `-v` flag for grep/egrep | Complete grep flag parity | `-v` excludes matching lines, parity test passes |
| [L3](#layer-3) | [L3.1](#l31) | | x | | | | | | - | 🟢 | ⚪ | ⚪ | Parallel run on 5 test projects — Python and Go side-by-side | Prove equivalence at scale | Both systems produce identical output |
| [L3](#layer-3) | [L3.2](#l32) | | x | | | | | | L3.1 | 🟢 | ⚪ | ⚪ | Diff search results: 100 queries/project, zero divergence | Search parity proof | `diff` output = 0 for all 500 queries |
| [L3](#layer-3) | [L3.3](#l33) | | x | | | | | | L3.1 | 🟡 | ⚪ | ⚪ | Diff learner state: 200 intents, zero tolerance | Learner parity proof | JSON diff of state = empty |
| [L3](#layer-3) | [L3.4](#l34) | x | | | | | | | L3.1 | 🟢 | ⚪ | ⚪ | Benchmark comparison — search, autotune, startup, memory | Confirm 50-120x speedup targets | Go beats Python on all 4 metrics |
| [L3](#layer-3) | [L3.5](#l35) | | | x | | | | | L3.1 | 🟢 | ⚪ | ⚪ | Migration docs — stop Python, install Go, migrate data | Clean upgrade path | Existing user migrates without data loss |
| [L4](#layer-4) | [L4.1](#l41) | | | x | | | | | - | 🟢 | ⚪ | ⚪ | Purego .so loader for runtime grammar loading | Extend language coverage without recompile | Load .so, parse file, identical to compiled-in |
| [L4](#layer-4) | [L4.2](#l42) | | | x | | | | | L4.1 | 🟡 | ⚪ | ⚪ | Grammar downloader CI — compile .so, host on GitHub Releases | Easy grammar distribution | Download + load 20 grammars from releases |
| [L4](#layer-4) | [L4.3](#l43) | | | x | | | | | - | 🟢 | ⚪ | ⚪ | Goreleaser — linux/darwin × amd64/arm64 | Cross-platform binaries | Binaries build for all 4 platforms |
| [L4](#layer-4) | [L4.4](#l44) | | | x | x | | | | L4.3 | 🟢 | ⚪ | ⚪ | Installation docs — `go install` or download binary | Friction-free onboarding | New user installs and runs in <2 minutes |
| [L5](#layer-5) | [L5.1](#l51) | | | | | x | | | - | 🟢 | ⚪ | ⚪ | Design structural query YAML schema (AST + lang_map + AC text) | Foundation for all dimensional patterns | Schema validates structural and text definitions |
| [L5](#layer-5) | [L5.2](#l52) | x | | | | x | | | L5.1 | 🟢 | ⚪ | ⚪ | Tree-sitter AST walker — match patterns against parsed AST | Structural detection engine | Walks AST, returns bit positions, ~100-500μs/file |
| [L5](#layer-5) | [L5.3](#l53) | x | | | | x | | | L5.1 | 🟢 | ⚪ | ⚪ | AC text scanner — compile patterns, scan raw source | Text detection engine | Single-pass AC, returns bit positions, ~15μs/file |
| [L5](#layer-5) | [L5.4](#l54) | | | | | x | | | L5.2 | 🟡 | ⚪ | ⚪ | Language mapping layer — normalize node names across 28 langs | Cross-language uniformity | Same query matches Go + Python + JS + Rust |
| [L5](#layer-5) | [L5.5](#l55) | x | | | | x | | | L5.2, L5.3 | 🟢 | ⚪ | ⚪ | Bitmask composer — merge structural + text bits, weighted severity | The scoring engine | Bitmask per method, score = weighted sum of set bits |
| [L5](#layer-5) | [L5.6](#l56) | | | | | | | | L5.1 | 🟡 | ⚪ | ⚪ | Security tier — 5 dims, 67 questions (injection, secrets, auth, crypto, path) | First dimensional tier | Catches known vulns in test projects |
| [L5](#layer-5) | [L5.7](#l57) | | | | | | | | L5.1 | 🟡 | ⚪ | ⚪ | Performance tier — 4 dims (queries, memory, concurrency, resource leaks) | Second tier | Flags N+1, unbounded allocs |
| [L5](#layer-5) | [L5.8](#l58) | | | | | | | | L5.1 | 🟡 | ⚪ | ⚪ | Quality tier — 4 dims (complexity, error handling, dead code, conventions) | Third tier | God functions, ignored errors |
| [L5](#layer-5) | [L5.9](#l59) | | | | | | | | L5.5 | 🟢 | ⚪ | ⚪ | Wire analyzer into `aoa init` — scan all files, store bitmasks in bbolt | Connect engine to pipeline | Bitmasks persist, available to search + dashboard |
| [L5](#layer-5) | [L5.10](#l510) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Add dimension scores to search results (`S:-1 P:0 C:+2`) | Scores visible inline | Scores appear in grep/egrep output |
| [L5](#layer-5) | [L5.11](#l511) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Dimension query support — `--dimension=security --risk=high` | Filter by dimension | CLI filters by tier and severity |
| [L5](#layer-5) | [L5.12](#l512) | | | | | | | x | L5.9 | 🟢 | ⚪ | ⚪ | Recon tab — NER-style dimensional view, drill-down, severity scoring | Dashboard dimensional view | Mockup parity validated in browser |

---

## Supporting Detail

### Layer 0

**Layer 0: Value Engine (Burn rate, context runway, attribution)**

> The metrics backend that powers all value messaging. Without this, the dashboard has data but no story.
> **Quality Gate**: `/api/runway` returns valid dual projections; all 15 attribution rows produce correct attrib/impact.

#### L0.1

**Burn rate accumulator**

Rolling window (5-minute) token-per-minute rate tracker. Feeds both the runway projection and the "tokens saved" metric.

**Files**: `internal/app/app.go`

#### L0.2

**Context window max lookup**

Map model tags from session JSONL (e.g., `claude-3-opus-20240229`) to their context window maximum. Used to compute runway as percentage and minutes remaining.

**Files**: `internal/app/app.go`

#### L0.3

**Dual projection**

Two burn rate projections: with-aOa (actual observed) vs without-aOa (counterfactual based on unguided tool call costs). The delta between them is the savings story.

**Files**: `internal/app/app.go`

#### L0.4

**Context runway API**

`/api/runway` returns JSON: `{ with_aoa: { minutes: 47, tokens_remaining: 84200 }, without_aoa: { minutes: 12, tokens_remaining: 84200 }, delta_minutes: 35 }`

**Files**: `web/server.go`, `socket/protocol.go`

#### L0.5

**Session summary persistence**

Per-session metrics record written to bbolt when session ends or daemon stops. Schema:

```
session_summary {
  session_id, date, start, end, duration_min,
  prompts, tool_reads, tool_writes, tool_searches,
  tool_grep, tool_glob, tool_bash,
  guided_reads, unguided_reads, guided_ratio,
  tokens_in, tokens_out, tokens_cache,
  tokens_actual, tokens_counterfact, tokens_saved,
}
```

Stored in `session_summaries` bucket, keyed by session ID. Arsenal reads this for daily rollups, learning curve, and value charts. Replaces the original "weekly counter" concept — individual session records are more flexible.

**Files**: `adapters/bbolt/store.go`, `internal/app/app.go`

#### L0.6

**Autotune verification**

Verify the 50-prompt autotune cycle fires reliably in the integrated system (not just unit tests). Important for AT-04 and learn event confidence.

**Files**: `internal/app/app.go`

#### L0.7

**Autotune activity event**

Activity feed entry: `Autotune | aOa | cycle 3 | +2 promoted, -1 demoted, ~8 decayed`. Visible evidence of the learning system working.

**Files**: `internal/app/app.go`

#### L0.8

**Write/Edit attrib**

Claude's Write/Edit tool events tagged as "productive" — desired work, aOa not involved. Green in dashboard.

**Files**: `internal/app/app.go`

#### L0.9

**Glob attrib**

Claude's Glob tool events tagged as "unguided" + estimated token cost. Red in dashboard. Heuristic: file size × ~0.25 tokens/byte (pending research on JSONL token exposure — open question #6).

**Files**: `internal/app/app.go`

#### L0.10

**Grep (Claude) impact**

Claude's native Grep tool events show estimated token cost. Same heuristic as L0.9.

**Files**: `internal/app/app.go`

#### L0.11

**Learn activity event**

Activity feed entry: `Learn | aOa | observe | +4 keywords, +2 terms, +1 domain`. Shows learning happening in real time.

**Files**: `internal/app/app.go`

#### L0.12

**Target capture**

Preserve the full query/path syntax from tool calls as-is in the activity target column. No normalization — show exactly what was searched or accessed.

**Files**: `internal/app/app.go`

---

### Layer 1

**Layer 1: Dashboard (5-tab layout, mockup implementation)**

> Transform the dashboard from data display to value narrative. Each tab tells a story.
> **Quality Gate**: All 5 tabs render with live data, hero sections drive value messaging, mockup parity confirmed.

**Tab structure:**
- **Live** (was Overview) — Context runway hero, savings stats, activity feed
- **Recon** (new) — Dimensional analysis view (stub until L5)
- **Intel** (was Learning) — Domain rankings, intent score, n-gram metrics
- **Debrief** (was Conversation) — Two-column conversation feed, tool actions
- **Arsenal** (new) — Value proof over time: session savings chart, learning curve, session history table, compact system status

**Design standard (locked Session 48):**
- **Hero row**: `min-height: 160px`, `flex: 2` wrapper + `flex: 1` metrics. `gap: 16px`. Gradient: `conic-gradient(green, blue, purple, green)` rotating at 6s. Card padding: `18px 24px`, gap `8px`. Identity font 22px, headline 17px, support 13px.
- **Hero metrics**: 2×2 grid with arrows. `font-size: 22px` values, `11px` labels.
- **Stats grid**: `repeat(N, 1fr)`, `gap: 12px`. Card: `padding: 16px`, `border-radius: 12px`. Value: `26px`. Label: `12px`. Sub: `11px`. N varies by tab (5-6).
- **Headline pattern**: `{Identity} {outcome} . . . {separator} {exclusion}.`
- **Three-tier narrative**: Hero (claim) → Stats grid (evidence) → Data (detail).
- **Nav**: 52px height. **Footer**: 36px height. Both identical across all tabs.
- **Recon sidebar**: Card-styled (`border-radius: 16px`), grid-aligned with first stat card (`repeat(5, 1fr)` matching stats grid). Pill-based dimension toggles (color-coded by tier, wrapping), toggle switches per tier.
- **Intel**: Domain table matches embedded dashboard: `#`, `@Domain` (purple), `Hits` (float, green, right-aligned), `Terms` (pills with hot/warm/cold states). N-gram sections: Bigrams (cyan), Cohits KW→Term (green), Cohits Term→Domain (purple).

**Static mockups** (validated Session 48): `_live_mockups/{live,recon,intel,debrief,arsenal}.html`

#### L1.1

**Tab rename**

Overview→Live, Learning→Intel, Conversation→Debrief. Update nav bar, tab click handlers, active state.

**Files**: `static/index.html`

#### L1.2

**Recon tab stub**

Placeholder tab for dimensional analysis. Shows "Dimensional scanning available in v2" with link to research docs. Reserve the slot so Arsenal isn't the last tab.

**Files**: `static/index.html`

#### L1.3

**Arsenal tab — Value Proof Over Time**

Arsenal's identity: "Here's the proof aOa is working for you." Session-centric, not config-centric.

Layout (top to bottom):
1. **Hero** — Cumulative savings headline. Metrics: tokens saved → sessions extended, unguided cost → guided ratio.
2. **Stats grid** (6 cards) — Tokens Saved, Unguided Cost, Sessions, Extended, Guided Ratio, Read Velocity.
3. **Savings chart** (full width) — Daily rollup. Dual bars: red (without aOa) + green (with aOa). Gap = savings. Multiple sessions per day aggregated.
4. **Bottom split** — Session history table (3/5 width, scrollable, individual sessions with hex IDs) | Learning curve canvas + compact system strip (2/5 width).

Session table columns: Session (hex ID), Date+Time, Duration, Prompts, Reads, Guided %, Saved, Waste, Reads/Prompt.

System status is a compact horizontal strip (not a card): running dot, PID, uptime, memory, DB size, files indexed.

**Files**: `static/index.html`

#### L1.4

**5-tab responsive header**

Tabs should not wrap or overflow at <800px. Consider abbreviated labels or icon-only at narrow widths.

**Files**: `static/index.html`

#### L1.5

**Arsenal API**

Two endpoints:
- `/api/sessions` — Returns array of session summary records from bbolt. Each has: session_id, date, duration, prompts, reads, guided_ratio, tokens_saved, tokens_actual, tokens_counterfact. Used for savings chart (daily rollup), session history table, and learning curve.
- `/api/config` — Returns: project root, DB path, socket path, daemon PID, uptime, memory, DB size, files indexed. Used for the compact system strip.

**Files**: `web/server.go`

#### L1.6

**Live tab hero — context runway**

Primary display: "47 min remaining." Secondary: "Without aOa: 12 min." The delta IS the value prop. Replaces current hero that shows prompt/domain counts.

**Files**: `static/index.html`

#### L1.7

**Live tab metrics panel**

Replace prompts/domains cards with: Rolling avg search speed, Tokens saved (session), Guided ratio (%), Sessions extended (week). Value-oriented, not vanity.

**Files**: `static/index.html`

#### L1.8

**Token cost display for unguided tools**

When Claude uses Grep/Glob, show estimated token cost in the activity impact column. Red-coded. Makes the cost of NOT using aOa visible.

**Files**: `static/index.html`

---

### Layer 2

**Layer 2: Infrastructure Gaps (File watcher, bbolt lock, CLI completeness)**

> Fix the known gaps that prevent production-grade operation.
> **Quality Gate**: `aoa init` works while daemon runs; file changes trigger re-index within 2s; `grep -v` works.

#### L2.1

**Wire file watcher**

`fsnotify.Watcher` and `treesitter.Parser` are both built and tested. The pipeline (file change → re-parse → update index in bbolt) needs to be connected in `app.go`. `Watch()` is never called today.

**Files**: `internal/app/app.go`, `internal/adapters/fsnotify/watcher.go`

#### L2.2

**Fix bbolt lock contention**

`aoa init` fails while daemon holds the bbolt lock. Solution: in-process reindex command via socket (`aoa daemon reindex`). Daemon receives command, re-indexes while holding the lock, reports progress.

**Files**: `internal/adapters/socket/server.go`, `cmd/aoa/cmd/daemon.go`

#### L2.3

**Implement --invert-match**

The `-v` flag for grep/egrep is the last missing flag from the parity table. Exclude matching lines from results.

**Files**: `cmd/aoa/cmd/grep.go`, `cmd/aoa/cmd/egrep.go`, `internal/domain/index/search.go`

---

### Layer 3

**Layer 3: Migration & Validation (Parallel run, parity proof)**

> Run both systems side-by-side and prove equivalence before the Python version is retired.
> **Quality Gate**: 500 queries across 5 projects = zero divergence. Go beats Python on all benchmarks.

#### L3.1

**Parallel run on 5 projects**

Select 5 diverse test projects (varying size, language mix, domain spread). Run both Python and Go in parallel, capture all outputs.

**Files**: `test/migration/*.sh`

#### L3.2

**Search diff**

100 queries per project, automated diff of results. Cover all search modes: literal, OR, AND, regex, case-insensitive, word-boundary, count, include/exclude.

**Files**: `test/migration/search-diff.sh`

#### L3.3

**Learner state diff**

After 200 intents of observation, diff the learner state (domains, terms, keywords, hits, bigrams). Zero tolerance for divergence. DomainMeta.Hits is float64 — the precision rule matters here.

**Files**: `test/migration/state-diff.sh`

#### L3.4

**Benchmark comparison**

Head-to-head: search latency, autotune latency, startup time, memory footprint. Confirm 50-120x speedup and 8x memory reduction.

**Files**: `test/benchmarks/compare.sh`

#### L3.5

**Migration docs**

Step-by-step: stop Python daemon, install Go binary, migrate bbolt data (or re-index), verify. Cover rollback if needed.

**Files**: `MIGRATION.md`

---

### Layer 4

**Layer 4: Distribution (Single binary for all platforms)**

> Ship it. One binary per platform, instant install, zero friction.
> **Quality Gate**: Binary works on linux/darwin × amd64/arm64. `go install` path works. Grammar download is optional.

#### L4.1

**Purego .so loader**

Runtime grammar loading via purego (no CGo at load time). Allows extending language coverage without recompiling the binary.

**Files**: `internal/adapters/treesitter/loader.go`

#### L4.2

**Grammar downloader CI**

CI pipeline: compile .so files for each grammar, host on GitHub Releases. `aoa grammar download <lang>` fetches and installs.

**Files**: `.github/workflows/build-grammars.yml`

#### L4.3

**Goreleaser**

Cross-compilation for 4 platforms. GitHub Release automation. Checksum files.

**Files**: `.goreleaser.yml`

#### L4.4

**Installation docs**

Two paths: `go install github.com/corey/aoa/cmd/aoa@latest` or download binary from releases. Include post-install: `aoa init`, `aoa daemon start`, alias setup.

**Files**: `README.md`

---

### Layer 5

**Layer 5: Dimensional Analysis (Bitmask engine, Recon tab)**

> Early warning system — yellow flags, not verdicts. Surfaces concerns via per-line bitmask scanning using tree-sitter AST + Aho-Corasick. Users can acknowledge, dismiss, or ignore findings.
>
> **Prerequisite**: Multi-language tree-sitter integration must be mature (L4 grammar loader working, validated across languages).
>
> **Quality Gate**: Security tier catches known vulns in test projects. Full project query < 10ms. ~250-290 questions across 6 tiers.
>
> **Research**: [Bitmask analysis](.context/research/bitmask-dimensional-analysis.md) | [AST vs LSP viability](.context/research/asv_vs_lsp.md)

**6 Tiers (22 dimensions):**

| Tier | Color | Dimensions |
|------|-------|-----------|
| Security | Red | Injection, Secrets, Auth Gaps, Cryptography, Path Traversal |
| Performance | Yellow | Query Patterns, Memory, Concurrency, Resource Leaks |
| Quality | Blue | Complexity, Error Handling, Dead Code, Conventions |
| Compliance | Purple | CVE Patterns, Licensing, Data Handling |
| Architecture | Cyan | Import Health, API Surface, Anti-patterns |
| Observability | Green | Silent Failures, Debug Artifacts |

**Execution pipeline**: Compute at index time (tree-sitter parse + AC scan + regex → bitmask → bbolt). Query time reads pre-computed bitmasks (< 10ms for entire project). See research doc for full pipeline detail.

#### L5.1

**Structural query YAML schema**

Define the format for detection patterns: AST structural rules, AC text patterns, lang_map entries, severity weights, bit positions.

**Files**: `dimensions/schema.yaml`

#### L5.2

**Tree-sitter AST walker**

Walk the already-parsed AST for structural patterns (call_with_arg, call_inside_loop, assignment_with_literal, etc.). ~8-10 parameterized pattern types cover the majority of questions.

**Files**: `internal/domain/analyzer/walker.go`

#### L5.3

**AC text scanner**

Compile ~115 text patterns into single AC automaton at startup. One pass per file over raw source bytes. Returns (pattern_id, byte_offset) pairs mapped to bit positions.

**Files**: `internal/domain/analyzer/text_scan.go`

#### L5.4

**Language mapping layer**

Normalize AST node names across 28 languages. ~280 entries (28 langs × 10 concepts). Most are identical or near-identical. ~20% of questions need per-language overrides.

**Files**: `internal/domain/analyzer/lang_map.go`

#### L5.5

**Bitmask composer**

Merge structural + text + regex bits into per-method bitmask. Compute weighted severity score (critical=3, high=2, medium=1). ~38 bytes per method across all dimensions.

**Files**: `internal/domain/analyzer/score.go`

#### L5.6

**Security tier**

67 questions across 5 categories: injection (16), secrets (13), auth gaps (14), cryptography (12), path traversal (12). Full question set defined in research doc.

**Files**: `dimensions/security/*.yaml`

#### L5.7

**Performance tier**

~50-60 questions: query patterns, memory, concurrency, resource leaks. N+1, unbounded alloc, mutex over I/O, unclosed handles.

**Files**: `dimensions/performance/*.yaml`

#### L5.8

**Quality tier**

~45-55 questions: complexity, error handling, dead code, conventions. God functions, ignored errors, deep nesting, magic numbers.

**Files**: `dimensions/quality/*.yaml`

#### L5.9

**Wire analyzer into init**

During `aoa init` (and file-watch re-index), run the dimensional engine alongside symbol extraction. Store per-method bitmasks and scores in bbolt "dimensions" bucket. ~0.2-0.6ms overhead per file.

**Files**: `internal/app/app.go`

#### L5.10

**Dimension scores in search results**

Append `S:-23 P:0 Q:-4` to search output. Negative = debt. Zero = clean. Visible in every grep/egrep result.

**Files**: `internal/domain/index/format.go`

#### L5.11

**Dimension query support**

`aoa grep --dimension=security --risk=high <query>` filters results to only show methods with security findings above threshold.

**Files**: `cmd/aoa/cmd/grep.go`

#### L5.12

**Recon tab**

NER-style dimensional view: tier toggle sidebar (6 tiers, color-coded), file→method drill-down, severity scoring, acknowledge/dismiss per finding. Mockup validated in `_live_mockups/recon.html`.

**Files**: `static/index.html`

---

### What Works (Preserve)

| Component | Notes |
|-----------|-------|
| Search engine (O(1) inverted index) | 26/26 parity tests, 4 search modes, content scanning. Do not change search logic. |
| Learner (21-step autotune) | 5/5 fixture parity, float64 precision on DomainMeta.Hits. Do not change decay/prune constants. |
| Session Prism (Claude JSONL reader) | Defensive parsing, UUID dedup, compound message decomposition. Battle-tested. |
| Tree-sitter parser (28 languages) | Symbol extraction working for Go, Python, JS/TS + 24 generic. Reuse ASTs for L5. |
| Socket protocol | JSON-over-socket IPC. Concurrent clients. Extend, don't replace. |
| Activity rubric (30 tests) | Color-coded attribs, impact formatting. Extend for new tool types. |

### What We're NOT Doing

| Item | Rationale |
|------|-----------|
| Neural 1-bit embeddings | Investigated, deprioritized. Deterministic AST+AC gives better signal with full interpretability. |
| WebSocket push (dashboard) | 2s poll is sufficient. Upgrade deferred — complexity not justified yet. |
| Multi-project simultaneous daemon | Single-project scope per daemon instance. Multi-project is a v3 concern. |
| LSP integration | AST is sufficient for early warning. LSP adds 100x cost for 20% more precision. See research. |

### Key Documents

| Document | Purpose |
|----------|---------|
| [COMPLETED.md](.context/COMPLETED.md) | Archived phases 1-8c with validation notes |
| [Bitmask Analysis](.context/research/bitmask-dimensional-analysis.md) | Security worked example, execution pipeline, cross-language uniformity |
| [AST vs LSP](.context/research/asv_vs_lsp.md) | Viability assessment, per-dimension confidence ratings |
| [Feedback Outline](research/feedback/OUTLINE.md) | User feedback on all system components |
| [CLAUDE.md](CLAUDE.md) | Agent instructions, architecture reference, build commands |

### Quick Reference

| Resource | Location |
|----------|----------|
| Build | `go build ./cmd/aoa/` |
| Test | `go test ./...` |
| CI check | `make check` |
| Database | `{ProjectRoot}/.aoa/aoa.db` |
| Socket | `/tmp/aoa-{sha256(root)[:12]}.sock` |
| Dashboard | `http://localhost:{port}` (port in `.aoa/http.port`) |
| Session logs | `~/.claude/projects/{encoded-path}/*.jsonl` |
| Mockups | `_live_mockups/{live,recon,intel,debrief,arsenal}.html` |
