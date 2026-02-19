# Work Board

[Board](#board) | [Supporting Detail](#supporting-detail) | [Completed](.context/COMPLETED.md) | [Backlog](.context/BACKLOG.md)

> **Updated**: 2026-02-19 (Session 55) | **Phase**: L2 complete — Infrastructure gaps closed (invert-match, file watcher, bbolt lock fix)
> **Completed work**: See [COMPLETED.md](.context/COMPLETED.md) — Phases 1–8c + L0 + L1 + L2 (388 active tests)

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
| 🟢 | Confident | Complete | Automated test proves it (unit or integration) |
| 🟡 | Uncertain | Pending | Partial — manual/browser only, or unit but no integration. See Va Detail for gap. |
| 🔴 | Lost/Blocked | Blocked | Failed |

> 🟢🟢🟢 = done. Task moves to COMPLETED.md.

---

## Mission

**North Star**: One binary that makes every AI agent faster by replacing slow, expensive tool calls with O(1) indexed search — and proves it with measurable savings.

**Current**: Hot file cache + version display (Session 55). FileCache pre-reads all indexed files into memory at startup, eliminating disk I/O from the search path. `aoa --version` and daemon start/stop now show git version + build date via ldflags. `make build` target added. Search observer cascade identified: re-tokenizing content hit text through enricher produced 137 keywords/89 terms/76 domains from a single-token query — observer patched to use pre-resolved Tags instead of re-tokenizing. Next: refactor search observer to use correct signal pipeline (direct-increment domains/terms from hits, content text through ProcessBigrams with threshold gating, same pipeline for session log grep results).

**Approach**: TDD. Each layer validated before the next. Completed work archived to keep the board focused on what's next.

**Design Decisions Locked** (Session 48):
- **aOa Score** — Deferred. Not defined until data is flowing. Intel tab cleaned of coverage/confidence/momentum card.
- **Arsenal = Value Proof Over Time** — Not a config/status page. Session-centric value evidence: actual vs counterfactual token usage, learning curve, per-session savings. System status is compact/secondary.
- **Session as unit of measurement** — Each session gets a summary record (ID, date, prompts, reads, guided ratio, tokens saved, counterfactual). Multiple sessions per day. Chart is daily rollup, table is individual sessions.
- **Counterfactual is defensible** — "Without aOa" = sum of full file sizes for guided reads + observed unguided costs. Not fabricated.

**Design Decisions Locked** (Session 49):
- **Token cost heuristic** — `bytes / 4 = estimated tokens`. Standard industry approximation (1 token ≈ 4 characters; code is ASCII so bytes ≈ characters). Used for counterfactual cost of unguided Grep/Glob and for savings calculations. Resolves open question #6.
- **Session boundary = session ID change** — Each Claude JSONL file carries a session ID. When the reader sees a new session ID, flush the summary for the previous session and start accumulating for the new one. Revisiting an old session loads its existing summary from bbolt and appends. No timeout heuristics.

**Design Decisions Locked** (Session 50):
- **Dashboard split into 3 files** — `index.html` (354 lines, HTML shell), `style.css` (552 lines, all CSS), `app.js` (1022 lines, all logic). `embed.go` uses `//go:embed static` to pick up all three. Monolithic single-file approach abandoned after agent reliability issues with large files.
- **Soft glow animation system** — Replaced all harsh flash/glow animations with CSS transition-based diffuse effects. `soft-glow` (box-shadow, 2.5s ease-out) on ngram counts; `text-glow` (text-shadow, 2.5s ease-out) on hit values and ngram names; `.lit` class on term pills triggers a CSS `transition: background/color/box-shadow 2.5s ease-out` fade. No harsh instant-on effects anywhere.
- **Domain table: no Tier column** — Removed. Table is `#`, `@Domain`, `Hits`, `Terms`. Tier is surfaced via the domain name and stats grid (Core count), not a separate column.
- **Tab state in URL hash** — `#live`, `#recon`, `#intel`, `#debrief`, `#arsenal`. Restored on page load. Direct links to tabs work.
- **Tab-aware polling** — Each tab only fetches its relevant endpoints. Live: runway + stats + activity. Intel: stats + domains + bigrams. Debrief: conv/metrics + conv/feed. Arsenal: sessions + config + runway. Recon: health only. 3s interval.

**Design Decisions Locked** (Session 51):
- **Ngram limits: 10/5/5** — Bigrams capped at 10, Cohit KW→Term at 5, Cohit Term→Domain at 5. Section spacing 24px.
- **Debrief exchange grouping** — Consecutive user+assistant turns paired into "exchanges". Turn numbering counts exchanges, not individual messages. Oldest at top, newest at bottom (scroll lands at NOW).
- **Thinking as nested sub-element** — Always visible (no click-to-expand). Indented 16px with purple left border. Each thinking chunk is a separate line (backend separates with `\n`). 11px italic font, muted color. Thinking text truncation raised to 2000 chars.
- **Markdown rendering in Debrief** — Lightweight `renderMd()`: code blocks, inline code (cyan), bold, italic, line breaks. Applied to assistant text and thinking lines.
- **Token estimates on all message types** — User: `~text.length/4` (heuristic, prefixed with `~`). Thinking: `~text.length/4`. Assistant: actual `output_tokens` from API (no prefix). No double-counting.
- **Debrief actions: Save/Tok columns** — Two fixed-width (38px) right-aligned columns per turn. `Save`: green `↓N%` when aOa guided, blank otherwise. `Tok`: green when guided (reduced cost), red when unguided (full cost), blank for productive. Column headers at turn level with hover titles.
- **Full-file reads classified as unguided** — Read with limit=0 now gets `attrib="unguided"` and `tokens=fileSize/4` from index. Previously had no classification.
- **TurnAction carries Attrib/Tokens/Savings** — Backend `TurnAction` and `TurnActionResult` now include `attrib` (string), `tokens` (int), `savings` (int). Same data flows to both Live activity and Debrief actions.

**Design Decisions Locked** (Session 52):
- **Responsive compaction pattern** — At mobile breakpoint (<800px), hero and stats cards shed text and show values only. Hero: remove support text, keep headline + value. Stats cards: value only, no labels (labels visible in desktop view). This pattern repeats across all 5 tabs. Priority is to maximize space for the main value prop content below (domain table, ngrams, conversation feed, session history). Recon is hold-state, acceptable as-is.
- **Intel mobile** — Domain rankings and ngram sections must remain readable at mobile width. Current layout crowds them below oversized hero+stats. Compaction of hero+stats solves this.

**Design Decisions Locked** (Session 55):
- **Hot file cache** — `FileCache` in `internal/domain/index/filecache.go`. Pre-reads all eligible indexed files into `[]string` lines at startup via `WarmFromIndex()`. Binary filtering: extension blocklist + null byte check + MIME detection. 512KB per-file limit, 250MB configurable budget. `GetLines()` under RLock for concurrent searches. Re-warms on `Rebuild()` (file watcher triggers). 8 tests.
- **Version display via ldflags** — `internal/version/version.go` with `Version` + `BuildDate` vars, injected by `make build` via `-ldflags -X`. `aoa --version`, daemon start/stop all show version.
- **Search observer signal pipeline (approved, not yet fully implemented)** — Two paths, same analysis: (1) CLI search: top N hits → direct-increment domains/terms already on hits, content text → `ProcessBigrams` for bigram/cohit threshold gating (count ≥ 6 before promotion). (2) Session log: Claude's grep/egrep/search tool results → same pipeline, non-blocking via tailer goroutine. Current observer patched to stop re-tokenizing content through enricher; full pipeline refactor is next.

**Design Decisions Locked** (Session 54):
- **Attribution color system** — Three visual lanes: green = aOa guided (savings), red = unguided (cost), purple = creation. Unguided entries show red across attrib pill, impact text, and target text. Guided entries show green impact (savings display). No time claims on unguided entries — only token cost stated factually; red color communicates "not optimized" without accusing.
- **Creative words for Write/Edit** — Replaced static "productive" with cycling vocabulary: crafted, authored, forged, innovated. Purple pill, distinct from green (guided) and red (unguided). Each Write/Edit gets one word anchored to that entry; next creation may get a different word. Celebrates the creation moment.
- **Time savings: celebrate success, don't kick failure** — Time savings only shown on guided reads (provable counterfactual). Unguided reads show token cost only (no time claim). Cumulative savings tell the positive story over time. aOa doesn't claim value for net-new creation (Write/Edit).
- **Time savings algorithm (approved, not yet implemented)** — Two paths: simple (`tokens_saved * 0.0075s`) as fallback, dynamic (compute `ms/token` from tailer's `DurationMs` + `OutputTokens`, P25 percentile across recency windows, produce a range). Go has infrastructure advantage: tailer already extracts duration and token data in real-time, no separate JSONL re-parse needed.
- **Glob cost estimation uses `filepath.Match`** — Rewrote `estimateGlobCost`/`estimateGlobTokens` to accept `(dir, pattern)`. Uses `filepath.Match` for glob patterns against relative index paths, falls back to `HasPrefix` for directory-only lookups.

**Needs Discussion** (before L3):
- **Alias strategy** — Goal is replacing `grep` itself. `grep auth` → `aoa grep auth` transparently. Graceful degradation on unsupported flags?
- **Real-time conversation** — Legacy Python showed real-time; Go dashboard with 3s poll should do better. Needs investigation.

---

## Board

| Layer | ID | G0 | G1 | G2 | G3 | G4 | G5 | G6 | Dep | Cf | St | Va | Task | Value | Va Detail |
|:------|:---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:----|:--:|:--:|:--:|:-----|:------|:----------|
| [L0](#layer-0) | [L0.1](#l01) | x | | | | | | x | - | 🟢 | 🟢 | 🟢 | Burn rate accumulator — rolling window tokens/min | Foundation for all savings metrics | Unit: `burnrate_test.go` — 6 tests (empty, single, multi, eviction, partial, reset) |
| [L0](#layer-0) | [L0.2](#l02) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Context window max lookup — map model tag to window size | Needed for runway projection | Unit: `models_test.go` — 2 tests (known models, unknown default) |
| [L0](#layer-0) | [L0.3](#l03) | | | | | | | x | L0.1 | 🟢 | 🟢 | 🟢 | Dual projection — with-aOa vs without-aOa burn rates | The core value comparison | Unit: `runway_test.go` — 3 tests (divergence under load, zero-rate edge, model lookup) |
| [L0](#layer-0) | [L0.4](#l04) | | | | | x | | x | L0.3 | 🟢 | 🟢 | 🟢 | Context runway API — `/api/runway` with both projections | Dashboard and CLI can show runway | Unit: `runway_test.go` + HTTP: `web/server_test.go` TestRunwayEndpoint — JSON shape with both projections |
| [L0](#layer-0) | [L0.5](#l05) | | | | | x | | x | L0.3 | 🟢 | 🟢 | 🟢 | Session summary persistence — per-session metrics in bbolt | Arsenal value proof, survives restart | Unit: `bbolt/store_test.go` (4 tests: save/load/list/overwrite) + `session_test.go` (5 tests: boundary detect, flush, restore) |
| [L0](#layer-0) | [L0.6](#l06) | | | | | | x | x | - | 🟢 | 🟢 | 🟢 | Verify autotune fires every 50 prompts | Trust the learning cycle is working | Unit: `autotune_integration_test.go` — 50 searches → autotune triggers, activity entry emitted |
| [L0](#layer-0) | [L0.7](#l07) | | | | | | x | x | L0.6 | 🟢 | 🟢 | 🟢 | Autotune activity event — "cycle N, +P/-D/~X" | Visible learning progress in activity feed | Unit: `autotune_integration_test.go` — asserts activity entry with promote/demote/decay counts |
| [L0](#layer-0) | [L0.8](#l08) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Write/Edit creative attrib (purple) | Celebrate creation moments | Unit: `activity_test.go` TestWriteEditCreative + TestCreativeWordCycles — cycling crafted/authored/forged/innovated |
| [L0](#layer-0) | [L0.9](#l09) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Glob attrib = "unguided" + estimated token cost | Show cost of not using aOa | Unit: `activity_test.go` TestGlobAttrib — Glob events tagged `attrib="unguided"`, impact contains token estimate |
| [L0](#layer-0) | [L0.10](#l010) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Grep (Claude) impact = estimated token cost | Show cost of not using aOa | Unit: `activity_test.go` TestGrepImpact — Claude Grep events show `~Ntok` in impact |
| [L0](#layer-0) | [L0.11](#l011) | | | | | | x | | - | 🟢 | 🟢 | 🟢 | Learn activity event — observe signals summary | Visible learning in feed | Unit: `activity_test.go` TestLearnEvent + `autotune_integration_test.go` — entry contains "+N keywords, +M terms" |
| [L0](#layer-0) | [L0.12](#l012) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Target capture — preserve full query syntax, no normalization | Accurate activity display | Unit: `activity_test.go` TestTargetCapture — all-flags, regex+boundary, simple query preserved verbatim |
| [L1](#layer-1) | [L1.1](#l11) | | | | | | | x | - | 🟢 | 🟢 | 🟢 | Rename tabs: Overview→Live, Learning→Intel, Conversation→Debrief | Brand alignment — tabs named by user intent | Manual: user confirmed (Session 52) — Live/Recon/Intel/Debrief/Arsenal tabs visible in browser |
| [L1](#layer-1) | [L1.2](#l12) | | | | | | | x | L1.1 | 🟢 | 🟢 | 🟢 | Add Recon tab (stub) — dimensional placeholder | Reserve the tab slot for v2 | Manual: user confirmed (Session 52) — `#recon` tab renders hero + placeholder stub |
| [L1](#layer-1) | [L1.3](#l13) | | | | | | | x | L1.1 | 🟢 | 🟢 | 🟢 | Add Arsenal tab — value proof over time, session history, savings chart | Prove aOa's ROI across sessions | Manual: user confirmed (Session 52) — `#arsenal` tab visible, framing up. Backend: `web/server_test.go` TestSessionsEndpoint |
| [L1](#layer-1) | [L1.4](#l14) | | | | | | | | L1.1 | 🟢 | 🟢 | 🟢 | 5-tab responsive layout — mobile compaction at <=520px | Works on all screen sizes | Manual: user confirmed (Session 53) — hero card hidden, stats as value-only chips, glow consistent across breakpoints. CSS: `style.css` @media 520px block. Mockups validated first in `_live_mockups/`. |
| [L1](#layer-1) | [L1.5](#l15) | | | | | x | | x | L0.5, L1.3 | 🟢 | 🟢 | 🟢 | Arsenal API — `/api/sessions` + `/api/config` | Backend for Arsenal charts and system strip | Unit: `web/server_test.go` TestSessionsEndpoint + TestConfigEndpoint — JSON shape validated |
| [L1](#layer-1) | [L1.6](#l16) | x | | | | | | x | L0.4 | 🟢 | 🟢 | 🟢 | Live tab hero — context runway as primary display | Lead with the value prop | Manual: user confirmed (Session 53) — hero + metrics panel render correctly with data, consistent across responsive breakpoints. Backend: `runway_test.go`. |
| [L1](#layer-1) | [L1.7](#l17) | | | | | | | x | L0.4 | 🟢 | 🟢 | 🟢 | Live tab metrics panel — savings-oriented cards | Replace vanity metrics with value | Manual: user confirmed (Session 53) — 6 stats cards render, glow effect consistent across breakpoints. Backend: API tests. |
| [L1](#layer-1) | [L1.8](#l18) | | | | | | | x | L0.9, L0.10 | 🟢 | 🟢 | 🟢 | Dashboard: render token cost for unguided Read/Grep/Glob | Show unguided cost inline | Pipeline proven live (Session 53): tailer→activity→API→dashboard confirmed with guided Read showing `↓99% (12.1k → 100)`. Unit: `activity_test.go` TestActivityReadNoSavings (full-file Read shows `~500 tokens`), TestActivityRubric rows 7/10/11 (Read/Grep/Glob all show `~N tokens`). Fixed: full-file Read impact was `-`, now shows estimated cost. |
| [L2](#layer-2) | [L2.1](#l21) | x | | | | x | x | | - | 🟢 | 🟢 | 🟡 | Wire file watcher — `Watch()` in app.go, change→reparse→reindex | Dynamic re-indexing without restart | Unit: `watcher_test.go` (4 tests: new/modify/delete/unsupported call `onFileChanged` directly) + `rebuild_test.go` (1 test). **Gap**: no integration test through fsnotify event pipeline — needs `TestDaemon_FileWatcher_ReindexOnEdit` |
| [L2](#layer-2) | [L2.2](#l22) | x | | | | x | | | - | 🟢 | 🟢 | 🟢 | Fix bbolt lock contention — in-process reindex via socket command | `aoa init` works while daemon runs | Integration: `TestInit_DaemonRunning_DelegatesToReindex` (init succeeds via socket), `TestInit_DaemonRunning_ReportsStats` (output has file/symbol/token counts), `TestInit_DaemonRunning_ThenSearchFindsNewSymbol` (new file found after reindex). Unit: `indexer_test.go` (3), `reindex_test.go` (1) |
| [L2](#layer-2) | [L2.3](#l23) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | Implement `--invert-match` / `-v` flag for grep/egrep | Complete grep flag parity | Integration: `TestGrep_InvertMatch` (CLI `-v` via daemon), `TestGrep_InvertMatch_CountOnly` (`-v -c`), `TestEgrep_InvertMatch` (`-v` regex). Unit: `invert_test.go` (8 tests: literal/regex/OR/AND/content/count/quiet/glob) |
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
> **Quality Gate**: ✅ `/api/runway` returns valid dual projections; all attribution rows produce correct attrib/impact. 24 new tests, 284 total passing.

#### L0.1

**Burn rate accumulator** — 🟢 Complete

Rolling window (5-minute) `BurnRateTracker` with `Record`, `RecordAt`, `TokensPerMin`, `TotalTokens`, `Reset`. 6 unit tests (empty, single, multi-sample, eviction, partial eviction, reset).

**Files**: `internal/app/burnrate.go`, `internal/app/burnrate_test.go`

#### L0.2

**Context window max lookup** — 🟢 Complete

`ContextWindowSize()` map with Claude 3/3.5/4 family entries, 200k default for unknowns. 2 tests.

**Files**: `internal/app/models.go`, `internal/app/models_test.go`

#### L0.3

**Dual projection** — 🟢 Complete

`burnRate` (actual) and `burnRateCounterfact` (what-if) trackers wired into `onSessionEvent`. Counterfactual records the delta from guided reads (savings >= 50%). `sessionReadCount` and `sessionGuidedCount` tracked.

**Files**: `internal/app/app.go`

#### L0.4

**Context runway API** — 🟢 Complete

`GET /api/runway` returns `RunwayResult` JSON with model, context window, burn rates, both runway projections, delta, and tokens saved. `RunwayProjection()` on `AppQueries` interface. 3 unit tests + 1 HTTP endpoint test.

**Files**: `internal/adapters/socket/protocol.go`, `internal/adapters/socket/server.go`, `internal/adapters/web/server.go`, `internal/adapters/web/server_test.go`, `internal/app/app.go`, `internal/app/runway_test.go`

#### L0.5

**Session summary persistence** — 🟢 Complete

`SessionSummary` struct in ports. `SaveSessionSummary`/`LoadSessionSummary`/`ListSessionSummaries` on Storage interface. bbolt `sessions` bucket implementation. Session boundary detection via `handleSessionBoundary()` — flush on session ID change, restore on revisit. Flush in `Stop()`. 4 bbolt tests + 5 session boundary tests.

**Files**: `internal/ports/storage.go`, `internal/adapters/bbolt/store.go`, `internal/adapters/bbolt/store_test.go`, `internal/app/app.go`, `internal/app/session_test.go`

#### L0.6

**Autotune verification** — 🟢 Complete

Integration test fires 50 searches through `searchObserver`, confirms autotune triggers and produces activity entry.

**Files**: `internal/app/autotune_integration_test.go`

#### L0.7

**Autotune activity event** — 🟢 Complete

After autotune fires in `searchObserver`, pushes `ActivityEntry{Action: "Autotune", Attrib: "cycle N", Impact: "+P promoted, -D demoted, ~X decayed"}`. Verified by integration test.

**Files**: `internal/app/app.go`

#### L0.8

**Write/Edit attrib** — 🟢 Complete

Write/Edit tool events tagged with cycling creative words (`crafted`, `authored`, `forged`, `innovated`) in purple pills. Replaced static "productive" (Session 54). Updated rubric rows 8/9 (regex match), dedicated test `TestActivityWriteEditCreative`, and new `TestActivityCreativeWordCycles` verifying 4-word cycle order.

**Files**: `internal/app/app.go`, `internal/app/activity_test.go`, `internal/adapters/web/static/app.js`

#### L0.9

**Glob attrib** — 🟢 Complete

Glob tool events tagged `attrib = "unguided"` with estimated token cost. `estimateGlobCost(dir, pattern)` rewritten (Session 54) to use `filepath.Match` for glob patterns against relative index paths, with `HasPrefix` fallback for directory-only lookups. Target display now prefers pattern over directory. Updated rubric row 11, dedicated test `TestActivityGlobUnguided`, and new `TestActivityGlobPattern` for pattern-based matching.

**Files**: `internal/app/app.go`, `internal/app/activity_test.go`

#### L0.10

**Grep (Claude) impact** — 🟢 Complete

Claude Grep events show estimated scan cost via `estimateGrepCost()` (total indexed bytes / 4). Updated rubric row 10 and dedicated test.

**Files**: `internal/app/app.go`, `internal/app/activity_test.go`

#### L0.11

**Learn activity event** — 🟢 Complete

Two sources: (1) `searchObserver` pushes `Learn` with keyword/term/domain counts, (2) range-gated file reads push `Learn` with `+1 file: <path>`. Verified by integration test + dedicated unit test.

**Files**: `internal/app/app.go`, `internal/app/activity_test.go`

#### L0.12

**Target capture** — 🟢 Complete

`searchTarget()` preserves full flag syntax verbatim (already correct). Verification test added with all-flags, regex+boundary, and simple query cases.

**Files**: `internal/app/activity_test.go`

---

### Layer 1

**Layer 1: Dashboard (5-tab layout, mockup implementation)**

> Transform the dashboard from data display to value narrative. Each tab tells a story.
> **Quality Gate**: ✅ All 5 tabs render with live API data. `/api/sessions` and `/api/config` both tested. 286 tests passing.

**Tab structure:**
- **Live** (was Overview) — Context runway hero, savings stats, activity feed
- **Recon** (new) — Dimensional analysis view (stub until L5)
- **Intel** (was Learning) — Domain rankings, intent score, n-gram metrics
- **Debrief** (was Conversation) — Exchange-grouped conversation feed (user→thinking→assistant), markdown rendered, Save/Tok value columns in actions
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

#### L1.1–L1.8

**5-tab SPA** — 🟢 Complete

Full dashboard rewrite delivered as 3 files in `internal/adapters/web/static/`:

- **`index.html`** (354 lines) — HTML shell. Nav with 5 button-tabs, tab content divs (`tab-live`, `tab-recon`, `tab-intel`, `tab-debrief`, `tab-arsenal`), footer.
- **`style.css`** (552 lines) — Unified CSS. Dark/light themes, hero gradient animation, stats grid, activity table, domain table, n-gram bars, conversation feed, arsenal charts, recon placeholder, responsive breakpoints. Soft glow animation system (`soft-glow`, `text-glow`, `.lit` on term pills).
- **`app.js`** (1022 lines) — Tab switching (URL hash), tab-aware 3s polling, hero story rotation, per-tab renderers with change-tracking visual effects.

**Per-tab**:
- **Live**: Runway projection hero (dual projection), 6 stats cards, activity feed table with colored pills
- **Recon**: Hero + stats placeholders + "available in future release" card
- **Intel**: Domain rankings (no Tier column, term pills with `.lit` glow on hit, surgical DOM updates), N-gram metrics (6 bigrams / 4 cohit KW→Term / 4 cohit Term→Domain, no scroll, soft-glow on count changes)
- **Debrief**: Exchange-grouped conversation feed (user→thinking→assistant paired into turns, chronological order), inline thinking (always visible, nested italic, per-chunk lines), markdown rendering, token estimates on all message types, actions column with Save/Tok value columns (guided green, unguided red)
- **Arsenal**: Savings chart (div-based dual bars), session history table (mini guided-ratio bar), learning curve canvas (HiDPI), system status strip

**New backend** (`L1.5`): `SessionSummaryResult`, `SessionListResult`, `ProjectConfigResult` types. `SessionList()` + `ProjectConfig()` on `AppQueries`. `GET /api/sessions` + `GET /api/config` on web server. `dbPath` and `started` fields added to App struct. 2 new endpoint tests.

**Files**: `internal/adapters/web/static/index.html`, `style.css`, `app.js`, `embed.go`, `internal/adapters/socket/protocol.go`, `server.go`, `internal/adapters/web/server.go`, `server_test.go`, `internal/app/app.go`

---

### Layer 2

**Layer 2: Infrastructure Gaps (File watcher, bbolt lock, CLI completeness)**

> Fix the known gaps that prevent production-grade operation.
> **Quality Gate**: ✅ `aoa init` works while daemon runs; file changes trigger re-index; `grep -v` works. 22 new tests (17 unit + 6 integration − 1 replaced), 308 total passing. Integration tests: `TestInit_DaemonRunning_DelegatesToReindex`, `TestInit_DaemonRunning_ThenSearchFindsNewSymbol`, `TestGrep_InvertMatch`, `TestGrep_InvertMatch_CountOnly`, `TestEgrep_InvertMatch`.

#### L2.1

**Wire file watcher** — 🟢 Complete

`Rebuild()` method added to `SearchEngine` (reconstructs `refToTokens`, `tokenDocFreq`, `keywordToTerms` from current index state). `Parser *treesitter.Parser` added to `App` struct. `onFileChanged()` handles add/modify/delete: acquires mutex, checks extension support, computes relative path, removes old entries if modifying, parses with tree-sitter, adds tokens, calls `Rebuild()` + `SaveIndex()`. `removeFileFromIndex()` cleans all 3 index maps (Metadata, Tokens, Files). `Watch()` called in `Start()` (non-fatal on failure). 5 tests: rebuild after mutation, new file, modify file, delete file, unsupported extension.

**Files**: `internal/domain/index/search.go` (Rebuild), `internal/app/app.go` (Parser field, Start wiring), `internal/app/watcher.go` (new: onFileChanged, removeFileFromIndex), `internal/domain/index/rebuild_test.go` (new), `internal/app/watcher_test.go` (new)

#### L2.2

**Fix bbolt lock contention** — 🟢 Complete

`BuildIndex()` extracted from `init.go` into `internal/app/indexer.go` as shared function with `IndexResult` struct. `MethodReindex` socket command added with `ReindexResult` type. `Reindex()` on `AppQueries` interface — builds index outside mutex (IO-heavy walk/parse), then swaps maps in-place under lock and calls `Rebuild()`. Client `Reindex()` uses 120s timeout via new `callWithTimeout()` helper. `init.go` rewritten: delegates to daemon via socket when running (no lock error), falls back to direct `BuildIndex()` + bbolt otherwise. 4 tests: counts, large file skip, ignored dirs, full reindex cycle.

**Files**: `internal/app/indexer.go` (new: BuildIndex, IndexResult, skipDirs), `internal/adapters/socket/protocol.go` (MethodReindex, ReindexResult), `internal/adapters/socket/server.go` (Reindex on AppQueries, handleReindex), `internal/adapters/socket/client.go` (Reindex, callWithTimeout), `internal/app/app.go` (Reindex impl), `cmd/aoa/cmd/init.go` (rewritten), `internal/app/indexer_test.go` (new), `internal/app/reindex_test.go` (new)

#### L2.3

**Implement --invert-match** — 🟢 Complete

`InvertMatch bool` added to `SearchOptions`. `-v`/`--invert-match` flag registered in both `grep.go` and `egrep.go`, wired into opts. `invertSymbolHits()` method on `SearchEngine` computes the complement: builds set of matched `(fileID, line)` pairs, then collects all symbols NOT in that set (respecting glob filters). Content scanning flips the matcher result when `InvertMatch` is set. `-v` added to `searchTarget()` display. 8 tests: literal, regex, OR, AND, content, count-only, quiet, with-glob.

**Files**: `internal/ports/storage.go` (InvertMatch field), `cmd/aoa/cmd/grep.go` (-v flag), `cmd/aoa/cmd/egrep.go` (-v flag), `internal/domain/index/search.go` (invertSymbolHits), `internal/domain/index/content.go` (matcher flip), `internal/app/app.go` (searchTarget), `internal/domain/index/invert_test.go` (new)

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
| Search engine (O(1) inverted index) | 26/26 parity tests, 4 search modes, content scanning, `Rebuild()` for live mutation. Hot file cache (8 tests). `fileSpans` precomputed. Do not change search logic. |
| Learner (21-step autotune) | 5/5 fixture parity, float64 precision on DomainMeta.Hits. Do not change decay/prune constants. |
| Session Prism (Claude JSONL reader) | Defensive parsing, UUID dedup, compound message decomposition. Battle-tested. |
| Tree-sitter parser (28 languages) | Symbol extraction working for Go, Python, JS/TS + 24 generic. Reuse ASTs for L5. |
| Socket protocol | JSON-over-socket IPC. Concurrent clients. `Reindex` command with extended timeout. Extend, don't replace. |
| Value engine (L0, 24 new tests) | Burn rate, runway projection, session persistence, activity enrichments. All wired. |
| Activity rubric (41 tests) | Three-lane color system: green (guided savings), red (unguided cost — pill, impact, target), purple (creative words for Write/Edit). Learn/Autotune enrichments. |
| Dashboard (L1, 5-tab SPA) | 3-file split: `index.html` + `style.css` + `app.js`. Tab-aware polling. Soft glow animations. All tabs render live data. |
| File watcher (L2, 5 new tests) | `fsnotify` → `onFileChanged` → re-parse → `Rebuild()` → `SaveIndex()`. Add/modify/delete. |
| Invert-match (L2, 8 new tests) | `-v` flag for grep/egrep. Symbol complement + content matcher flip. All 4 modes. |
| Reindex protocol (L2, 4 new tests) | `BuildIndex()` shared function. `aoa init` delegates to daemon or runs direct. No more lock errors. |

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
| Build | `make build` (with version ldflags) or `go build ./cmd/aoa/` |
| Test | `go test ./...` |
| CI check | `make check` |
| Database | `{ProjectRoot}/.aoa/aoa.db` |
| Socket | `/tmp/aoa-{sha256(root)[:12]}.sock` |
| Dashboard | `http://localhost:{port}` (port in `.aoa/http.port`) |
| Session logs | `~/.claude/projects/{encoded-path}/*.jsonl` |
| Mockups | `_live_mockups/{live,recon,intel,debrief,arsenal}.html` |
