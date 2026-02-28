# aOa Completed Work

> Moved from BOARD.md to keep the active board focused. Each phase includes what was built, why, and how it was validated.

---

## Phase 1: Foundation & Test Harness

**What:** Project structure, hexagonal architecture, test fixtures, behavioral parity framework.

**Why:** Clean-room Go rewrite guided by behavioral specs, not Python code. Fixtures are the source of truth — 5 learner snapshots, 26 search queries, 13-file index.

**Validation:** 80 tests passing. Parity framework loads fixtures and asserts exact match.

**Key decisions:**
- Hexagonal architecture (ports/adapters) — domain logic is dependency-free
- `testify/assert` and `testify/require` for test framework
- Module path: `github.com/corey/aoa`

---

## Phase 2: Core Search Engine

**What:** O(1) indexed search with all four modes (literal, OR, AND, regex). Tree-sitter parser (28 languages, CGo compiled-in grammars). bbolt persistence. fsnotify watcher adapter. Unix socket daemon.

**Why:** Search is the core verb. O(1) lookup via inverted index gives sub-millisecond response. Tree-sitter provides structural understanding (functions, classes, methods) — not just text matching.

**Validation:** 26/26 search parity tests — zero divergence from Python output. All search modes produce identical results.

**Key files:** `internal/domain/index/search.go`, `internal/adapters/treesitter/parser.go`, `internal/adapters/bbolt/store.go`

---

## Phase 3: Universal Domain Structure

**What:** 134 semantic domains across 15 focus areas. Embedded via `//go:embed`. Enricher with O(1) keyword→term→domain lookup (~20ns).

**Why:** Domains are the semantic layer that makes aOa more than grep. 938 terms, 6566 keywords provide the vocabulary for understanding what code does, not just what it contains.

**Validation:** Enricher benchmarked at ~20ns per lookup. Shared keywords across domains handled correctly. Runtime noise filter prevents false positives.

**Key files:** `atlas/v1/` (15 JSON files), `internal/domain/enricher/enricher.go`

---

## Phase 4: Learning System

**What:** `observe()` signal processing, 21-step autotune algorithm, competitive displacement (top 24 core domains), cohit dedup, bigram extraction, keyword blocklist.

**Why:** The learner adapts to the developer's workflow. Every 50 prompts, autotune decays stale signals, promotes active domains, and prunes noise. Competitive displacement ensures the top 24 domains represent current intent.

**Validation:** 5/5 fixture parity with zero float divergence. Autotune benchmarked at ~2.5μs (2000x margin under 5ms target).

**Critical precision rules:**
- `DomainMeta.Hits` is **float64** — NO int truncation during decay
- All other hit maps truncate via `math.Trunc(float64(count) * 0.90)` to match Python `int()`
- Constants: `DecayRate=0.90`, `AutotuneInterval=50`, `PruneFloor=0.3`, `CoreDomainsMax=24`

**Key files:** `internal/domain/learner/learner.go`, `observe.go`, `autotune.go`, `bigrams.go`, `dedup.go`, `displace.go`

---

## Phase 5: Session Integration

**What:** Session Prism — Claude JSONL → tailer → parser → claude.Reader → canonical events. Signal chain: UserInput→bigrams, ToolInvocation→range gate→file_hits. Status line hook.

**Why:** The daemon tails Claude Code session logs (no hooks needed for learning). The Session Prism decomposes compound Claude messages into atomic events (thinking + text + tool_uses → separate events). Agent-agnostic canonical event model (7 event kinds).

**Validation:** 83 tests. Event decomposition tested against real Claude JSONL samples. ExtractionHealth tracks yield, gaps, version changes.

**Key files:** `internal/adapters/tailer/`, `internal/adapters/claude/reader.go`, `internal/ports/session.go`

---

## Phase 6: CLI & Hardening

**What:** 11 CLI commands (grep, egrep, find, locate, tree, config, health, wipe, daemon, init, open). Daemonized background process, PID management, lock contention diagnostics, orphan cleanup.

**Why:** Full Unix CLI experience. Every error message is actionable — if the daemon is running and you try `aoa init`, you get told exactly what to do. Stale socket detection, graceful shutdown, SIGTERM→SIGKILL fallback.

**Validation:** 42 integration tests covering end-to-end CLI flows (daemon start/stop, search via socket, init, health, config, wipe).

**Key files:** `cmd/aoa/cmd/` (all command files), `cmd/aoa/cmd/errors.go` (diagnostic helpers)

---

## Phase 7a: Web Dashboard

**What:** Embedded HTML dashboard (dark/light theme, responsive, surgical DOM updates). 4 API endpoints: health, stats, domains, bigrams. Removed 4 CLI commands (domains, intent, bigrams, stats) → moved to dashboard. Mock mode (`?mock`). Universal pulse animation.

**Why:** Full browser canvas replaces CLI stat commands. Embedded in the binary via `//go:embed` — no external files, no CDN, no dependencies.

**Validation:** Mock mode allows development and demo without a running daemon.

---

## Phase 7b: Dashboard Metrics Infrastructure

**What:** 6 new API endpoints. Session event accumulators (tokens, tools, conversation ring buffer). All 3 tabs live with real data. Activity & Impact table. Activity ring buffer with search observer timing.

**Why:** The dashboard needed real data, not just static views. Ring buffers provide bounded-memory storage for activity and conversation feeds. 309 tests passing.

**API Endpoints (11 total):**

| Endpoint | Returns |
|----------|---------|
| `GET /` | Embedded dashboard HTML |
| `GET /api/health` | File count, token count, uptime |
| `GET /api/stats` | Prompt count, domain/keyword/term/bigram counts |
| `GET /api/domains` | Domains sorted by hits, core count, term names |
| `GET /api/bigrams` | Bigrams + cohit KW→Term + cohit Term→Domain |
| `GET /api/conversation/metrics` | Input/output/cache tokens, turns, hit rate |
| `GET /api/conversation/tools` | Per-tool counters, top file reads/bash/grep |
| `GET /api/conversation/feed` | Last 20 conversation turns |
| `GET /api/top-keywords` | Top 15 keywords by hit count |
| `GET /api/top-terms` | Top 15 terms by hit count |
| `GET /api/top-files` | Top 15 files by hit count |
| `GET /api/activity/feed` | Last 20 activity entries |

---

## Phase 8a: Activity & Impact — Fix Pass

**What:** Behavioral parity with legacy `intent` command. 17 tasks (A-01 through A-17), all completed. 30 activity rubric tests passing.

**Why:** The activity table is the primary feedback loop — it shows what aOa did, what it attributed, and what impact it had. Legacy had this right; Go port needed to match exactly.

**Key fixes:**
- Source capitalization: `"claude"` → `"Claude"`
- Attrib: `"and"` → `"multi-and"` for AND search mode
- Bash `./aoa` commands filtered (already captured as Search by observer)
- Bash commands without file context filtered (e.g., `git status`)
- Read targets: strip project root, show relative path + `:offset-limit`
- Removed `guidedPaths` map (wrong approach — was path-matching search results)
- Implemented savings-based attribution: `file_size/4` vs `output_size/4`, ≥50% threshold
- Impact format for guided reads: `↓N% (Xk → Yk)`
- Three-part target color: `<cyan+bold>aOa</cyan+bold> <green>grep</green> <plain>query</plain>`
- Two-tone attrib rendering: "aOa" cyan+bold, "guided" green
- Three-part impact rendering: `<cyan+bold>N hits</cyan+bold> <dim>│</dim> <green>X.XXms</green>`

**Validation:** `internal/app/activity_test.go` — 30 tests covering all action/source/attrib/impact combinations: searchAttrib (4), searchTarget (2), source casing, impact default, guided savings, no-savings, path stripping (4), bash filtering (3), full rubric (13), readSavings (4), ring buffer.

---

## Phase 8b: Conversation Tab — Narrative Redesign

**What:** Complete overhaul from flat metric cards to two-column threaded narrative. 12 tasks (C-01 through C-12), all completed.

**Why:** Conversation should read like a conversation, not a dashboard of numbers. Two-column layout: messages on left (user→thinking→response), actions on right (tool chips with targets and impact).

**Key features built:**
- Turn grouping via `currentBuilder` — all assistant events between user inputs merge into one row
- Thinking text captured (truncated to 500 chars, click to expand)
- Per-turn token usage (InputTokens/OutputTokens)
- In-progress turn shown live (dashboard shows turn being built)
- Auto-scroll to bottom, "Jump to now" button, NOW bar
- Agent-agnostic labels ("Assistant" not "Claude", model shown as tag)

**Wire protocol:** `ConversationTurnResult` expanded with `thinking_text`, `actions[]`, `input_tokens`, `output_tokens`. `TurnActionResult` new struct.

---

## Phase 8b-2: Learning Tab — Live Signal Visualization

**What:** Term pills flash green on hit (30s fade), sorted by keyword hit popularity. Number glow on change. N-gram row flash on update.

**Why:** The learning tab should show learning *happening*, not just counts. When a keyword maps to a term and its count increases, you see exactly where the signal flowed.

**Key features:**
- `DomainTermHitCounts()` — sums keyword hits per term for change detection
- `DomainTermNames()` sorted by keyword hit popularity (fixed Go map random iteration)
- Blue text-shadow glow on stat number changes (4s fade)
- Green background flash on n-gram rows (3s fade)
- Up to 10 terms per row (increased from 5)

---

## Phase 8c: Global Branding & Color System

**What:** Consistent semantic color language across all surfaces.

| Color | Semantic Role | Usage |
|-------|--------------|-------|
| **CYAN+BOLD** | aOa brand identity | `aOa` everywhere — headers, source column, targets |
| **CYAN** | System/UI accent | Attrib values, tags, hit counts |
| **GREEN** | Positive/value | Savings arrows, timing, "guided", grep/egrep commands |
| **YELLOW** | Caution/in-progress | User prompts, learning state, warnings |
| **RED** | Error/critical | Errors, disconnected, critical context usage |
| **PURPLE/MAGENTA** | Domain names | @authentication, @search — always purple |
| **DIM/MUTE** | De-emphasized | Separators, secondary info, 0 hits, thinking blocks |

---

## Completed Attribution Table Rows

| # | Action | Source | Attrib | aOa Impact | Target |
|---|--------|--------|--------|------------|--------|
| 1 | Search | aOa | `indexed` | N hits, M files \| Xms | aOa grep \<query\> |
| 2 | Search | aOa | `multi-or` | N hits, M files \| Xms | aOa grep \<q1\> \<q2\> |
| 3 | Search | aOa | `multi-and` | N hits, M files \| Xms | aOa grep -a \<q1\>,\<q2\> |
| 4 | Search | aOa | `regex` | N hits, M files \| Xms | aOa egrep \<pattern\> |
| 5 | Read (ranged, >=50% savings) | Claude | `aOa guided` | ↓90% (44k → 4k) | file:offset-end |
| 6 | Read (ranged, <50% savings) | Claude | `-` | ↓44% (2.5k → 1.4k) | file:offset-end |
| 7 | Read (whole file) | Claude | `-` | `-` | file |
| 12 | Bash (aOa command) | *filtered* | — | — | — |
| 13 | Bash (no file) | *filtered* | — | — | — |

## Completed Grep/Egrep Flags

All flags below are implemented and tested:

| Flag | Short | Grep | Egrep | SearchOptions | Status |
|------|-------|------|-------|---------------|--------|
| `--and` | `-a` | Yes | No | `AndMode` | Done |
| `--count` | `-c` | Yes | Yes | `CountOnly` | Done |
| `--ignore-case` | `-i` | Yes | No | `Mode="case_insensitive"` | Done |
| `--word-regexp` | `-w` | Yes | No | `WordBoundary` | Done |
| `--quiet` | `-q` | Yes | Yes | `Quiet` | Done |
| `--max-count` | `-m` | Yes | Yes | `MaxCount` | Done |
| `--extended-regexp` | `-E` | Yes | n/a | Routes to egrep | Done |
| `--regexp` | `-e` | Yes | Yes | Multi-pattern OR/regex | Done |
| `--include` | — | Yes | Yes | `IncludeGlob` | Done |
| `--exclude` | — | Yes | Yes | `ExcludeGlob` | Done |
| `--recursive` | `-r` | no-op | no-op | — | Done (hidden) |
| `--line-number` | `-n` | no-op | no-op | — | Done (hidden) |
| `--with-filename` | `-H` | no-op | no-op | — | Done (hidden) |
| `--fixed-strings` | `-F` | no-op | — | — | Done (hidden) |
| `--files-with-matches` | `-l` | no-op | — | — | Done (hidden) |

---

## Success Metrics (All Verified)

| Metric | Target | Actual |
|--------|--------|--------|
| Search latency | <0.5ms | Verified |
| Autotune latency | <5ms | 2.5μs (2000x margin) |
| Startup time | <200ms | Verified |
| Memory footprint | <50MB | Verified |
| Behavioral parity | 100% | 26/26 search, 5/5 learner |
| Tests passing | 300+ | 315+ |
| Hook elimination | 0-1 | Optional status line only |
| Haiku elimination | 0 | Universal atlas |

---

## P0: Critical Bugs (Session 72 -- archived 2026-02-25)

All 7 bugs fixed. Old recon scanner deleted entirely, recon gated behind `aoa recon init`, debug mode implemented, truncation fixed. Clean separation of aoa-pure from recon complete.

| ID | Task | Resolution | Va Detail |
|:---|:-----|:-----------|:----------|
| B7 | Remove "recon cached" from pure aOa logs | Old scanner deleted entirely -- zero recon code in lean path | Triple-green |
| B9 | Recon tab shows install prompt in pure mode | Install prompt only path when recon not enabled | Triple-green |
| B10 | Gate `warmReconCache()` in `Reindex()` behind recon availability | Superseded -- warmReconCache() deleted entirely | Triple-green |
| B11 | Gate `updateReconForFile` in file watcher behind recon availability | Superseded -- updateReconForFile() deleted entirely | Triple-green |
| B14 | Remove truncation on debrief text -- assistant and thinking | 500-char truncation removed from user input text | Triple-green |
| B15 | Fix `BuildFileSymbols` called for entire index on every file change | Superseded -- BuildFileSymbols calls deleted with old scanner | Triple-green |
| B17 | Add debug mode (`AOA_DEBUG=1`) -- log all runtime events | AOA_DEBUG=1 enables timestamped debug logging at all key event points | Triple-green |

---

## L3.15: GNU grep native parity (Session 72 -- archived 2026-02-25)

Three-route architecture: file args -> `grepFiles()`, stdin pipe -> `grepStdin()`, neither -> daemon index search -> fallback `/usr/bin/grep`. 22 native flags covering 100% of observed AI agent usage.

**Validation**: 135 automated parity tests across two suites:
- `test/migration/grep_parity_test.go` (77 tests): internal search engine vs fixture index -- flags, combinations, edge cases, coverage matrix
- `test/migration/unix_grep_parity_test.go` (58 tests): CLI output format vs `/usr/bin/grep` -- exit codes, stdin/file/index routing, real-world agent invocations (Claude, Gemini), snapshots

**Files**: `cmd/aoa/cmd/grep.go`, `egrep.go`, `tty.go`, `grep_native.go`, `grep_fallback.go`, `grep_exit.go`, `output.go`

---

## L6.8 + L6.9: npm packages (Session 72 -- archived 2026-02-25)

10 npm packages: 2 wrappers + 8 platform-specific. esbuild/turbo pattern with JS postinstall shim. Published as `@mvpscale/aoa` and `@mvpscale/aoa-recon` v0.1.7 (2026-02-22).

**Files**: `npm/aoa/`, `npm/aoa-recon/`, 8x `npm/aoa-{platform}/`

---

## L6.10: CI/Release (Session 72 -- archived 2026-02-25)

Release workflow: 8 matrix jobs (2 binaries x 4 platforms), GitHub release, npm publish. 5 successful releases (v0.1.3-v0.1.7).

**Files**: `.github/workflows/release.yml`

---

## L2.1: Wire file watcher (Session 72 -- archived 2026-02-25)

`Watch()` in app.go, `onFileChanged()` handles add/modify/delete, `Rebuild()` on SearchEngine. 13 tests: 4 app (new/modify/delete/unsupported) + 6 adapter (detect change/new/delete, ignore non-code, reindex latency, stop cleanup) + 3 integration (new file auto-reindex, modify auto-reindex, delete auto-reindex). Full daemon -> fsnotify -> parse -> index -> search pipeline. Poll-based assertions (up to 3s) avoid flaky timing.

**Files**: `internal/domain/index/search.go`, `internal/app/app.go`, `internal/app/watcher.go`, `internal/domain/index/rebuild_test.go`, `internal/app/watcher_test.go`, `test/integration/cli_test.go`

---

## L7.2: Database storage optimization (Session 72 -- archived 2026-02-25)

Replaced JSON serialization with binary posting lists + gob for the bbolt search index. Format versioning (`_version` key): v0=JSON (legacy), v1=binary/gob. `SaveIndex` always writes v1. `LoadIndex` detects version and branches. Lazy v0->v1 migration -- first load reads v0, next save writes v1, all subsequent loads use fast binary path.

**Results**: 50K tokens / 500K refs encodes to 3.7 MB binary (vs ~75 MB JSON = ~20x smaller). Parallel decode preserved. All 25 bbolt tests pass.

**Key change**: TokenRefs encoded as little-endian uint32(FileID) + uint16(Line) = 6 bytes vs `{"FileID":1234,"Line":56}` = 25 bytes JSON.

**Files**: `internal/adapters/bbolt/encoding.go`, `internal/adapters/bbolt/store.go`, `internal/adapters/bbolt/store_test.go`

---

## L9: Telemetry (Session 76 -- archived 2026-02-26)

**What:** Unified content metering, tool call detail capture, counterfactual shadow engine. 9 tasks (L9.0-L9.8), all triple-green.

**Why:** Prove aOa saves tokens on every search tool call. Measure actual content volume across all streams (user, assistant, thinking, tool results, persisted results, subagents). Show throughput, burst velocity, and per-action counterfactual savings on the dashboard.

**Design:** [Throughput Telemetry Model](details/2026-02-26-throughput-telemetry-model.md) — raw character count is the universal unit, never convert to tokens at capture time, display converts (÷4).

**Key components:**
- **L9.0**: Inline tool result char capture — `ToolResultSizes` on parser events, `ResultChars` on TurnAction. 5 unit tests for extraction accuracy (string, array, fallback, zero, multi).
- **L9.1**: ContentMeter — unified `(chars, timestamp)` accumulator, ring buffer of 50 TurnSnapshots. 8 unit tests.
- **L9.2**: Tool call detail capture — Pattern, FilePath, Command on TurnAction/TurnActionResult, dashboard tooltips.
- **L9.3**: Persisted tool result sizes — tailer resolves `tool-results/toolu_{id}.txt` on disk.
- **L9.4**: Subagent JSONL tailing — discovers `subagents/agent-*.jsonl`, Source field, IsSubagent on events.
- **L9.5**: Counterfactual shadow engine — ToolShadow + ShadowRing (100-entry), async Grep/Glob dispatch. 6 unit tests.
- **L9.6**: Shim counterfactual (pivoted from bash parsing) — TotalMatchChars in SearchResult, observer savings delta.
- **L9.7**: Burst throughput & per-turn velocity — BurstTokensPerSec + TurnVelocities in ContentMeter, Debrief tab.
- **L9.8**: Dashboard shadow savings display — action rows, hero support line, stat cards. Plus Session 76 QOL: Intel/Debrief tab narrative redesign, task tool titles, session log cleanup, grep token fix, deploy.sh.

**Validation:** 19 unit tests across contentmeter_test.go (8), shadow_test.go (6), tailer_test.go (5). All features verified live on dashboard.

**Key files:** `internal/app/contentmeter.go`, `internal/app/shadow.go`, `internal/adapters/tailer/parser.go`, `internal/ports/session.go`, `internal/adapters/claude/reader.go`, `internal/app/app.go`, `internal/adapters/web/static/app.js`

---

## L5.19: Compliance Tier (superseded, archived 2026-02-26)

**Pivoted**: Compliance tier removed. Concepts (CVE patterns, licensing, data handling) absorbed into security tier's config dimension. `TierReserved` preserves bitmask slot.

---

## L8.1: Recon Tab (absorbed into L5.Va, archived 2026-02-26)

**What:** Interim pattern scanner with 10 detectors + `long_function`. `GET /api/recon` returns folder->file->findings tree. Tier toggles, breadcrumb nav, code-only file filtering.

**Absorbed**: Bitmask dashboard wiring gap merged into consolidated L5.Va (dimensional rule validation).

**Files**: `internal/adapters/recon/scanner.go`, `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

---

## Session Log

### 2026-02-17: Content Search + Tag Correction + Activity Enrichment

**Scope:** Phase 8a continued — `aoa grep` content scanning, tag system correction, search observer enrichment, activity & impact refinements.

**Content search (`aoa grep` body scanning):**
- `internal/domain/index/content.go` (NEW): `scanFileContents()` scans indexed file contents on disk for grep-style matches. Deduplicates against symbol hits (same file+line). Skips files >1MB.
- `buildContentMatcher()` handles all modes: literal (case-insensitive), regex, AND (comma-separated), word-boundary.
- `buildFileSpans()` + `findEnclosingSymbol()` — pre-computes per-file symbol ranges, resolves innermost enclosing function/method/class for each content hit.
- Symbol hits: `Kind: "symbol"`, full domain + tags + symbol signature.
- Content hits: `Kind: "content"`, enclosing symbol + range, tags (terms), NO domain.
- Two-section display — symbol hits first, content hits second. Header shows `(N symbol, M content)` breakdown.
- 7 new content tests: FindsBodyMatch, DedupWithSymbol, RegexMode, NoDomainButHasTerms, NestedEnclosingSymbol, SkipsLargeFiles, MissingFile.

**Tag system correction (keywords → terms):**
- Bug: `generateTags()` returned raw index tokens (keywords) instead of atlas terms.
- Fix: Built `keywordToTerms` reverse lookup by inverting `Domain.Terms` map. `resolveTerms()` maps raw tokens through atlas. All 26 parity tests pass.

**Search observer enrichment:**
- Bug: Observer only extracted signals from query tokens; content hit domains and tags were ignored.
- Fix: `signalCollector` now collects from 4 sources: query tokens, top 10 hit domains, hit tags, and hit content/symbol keywords. All fed to learner.
- `enricher.LookupTerm()` added for term→domain reverse lookup.

### 2026-02-17: Conversation Redesign + Data Pipeline + Learning Visualization

**Scope:** Phase 8b (Conversation tab), 8b-2 (Learning tab), partial 8a (FileMeta.Size, TurnAction)

**Key fix: Turn grouping** — Replaced per-TurnID `turnBuffer` map with single `currentBuilder`. All assistant events between user inputs accumulate into one exchange. Eliminated fragmented 5-6 row turns.

**Key fix: Term dancing** — `DomainTermNames()` was iterating a Go map (random order each call). Now sorts by keyword hit popularity with alphabetical tiebreaker. Stable and deterministic.

**Known gaps identified:**
- File watcher (`fsnotify`) built but not wired — `Watch()` never called, no dynamic re-indexing
- `aoa init` can't run while daemon holds bbolt lock — need in-process reindex command

---

## L7.4: .aoa/ directory restructure (Session 73 -- archived 2026-02-28)

**What:** `Paths` struct (18 fields), `EnsureDirs`, `Migrate` (7 files), 1MB log rotation, 13 files updated. Clean project state directory; logs don't grow forever.

**Validation:** 7 unit tests, live migration verified on 1.3GB database, all builds clean.

**Files:** `internal/app/paths.go`, `internal/app/paths_test.go`

---

## G0.HF1: Build process fix (Session 73 -- archived 2026-02-28)

**What:** `build.sh` as sole entry point, compile-time build guard (`cmd/aoa/build_guard.go`), flipped build tags (`recon` opt-in), Makefile rewrite, CLAUDE.md Build Rule. Binary 366MB->8MB.

**Validation:** `build.sh` enforced, `go build` panics with guard, 8MB binary verified.

**Files:** `build.sh`, `cmd/aoa/build_guard.go`, `Makefile`, `CLAUDE.md`

---

## L10.5: `aoa init` as single command (Session 75 -- archived 2026-02-28)

**What:** Scans project, detects languages via `ExtensionToLanguage`, shows curl commands for missing grammars, indexes with available grammars. Removed `aoa-recon` dependency entirely. TSX fix: each grammar gets its own .so file. Added `--no-grammars` flag.

**Validation:** Init runs end-to-end, no aoa-recon needed.

**Files:** `cmd/aoa/cmd/init.go`, `cmd/aoa/cmd/init_grammars_cgo.go`, `cmd/aoa/cmd/init_grammars_nocgo.go`, `internal/adapters/treesitter/extensions.go`, `internal/adapters/treesitter/loader.go`

---

## L10.6: Command rename -- wipe -> reset + remove (Session 75 -- archived 2026-02-28)

**What:** `aoa reset` clears data (what wipe used to do). `aoa remove` stops daemon and deletes `.aoa/` entirely. `aoa wipe` kept as hidden alias for backward compatibility.

**Validation:** Commands registered and working.

**Files:** `cmd/aoa/cmd/reset.go`, `cmd/aoa/cmd/remove.go`, `cmd/aoa/cmd/wipe.go`

---

## Session 81: Security Pipeline + aoa-recon Removal (2026-02-28)

**What:** SECURITY.md trust document, CI security scan (govulncheck + gosec + network audit), Slowloris fix, Go 1.25.7 bump, aoa-recon binary removed (-761 LOC), L4.4 Phase 2 grammar validation pipeline complete.

**Key accomplishments:**
- SECURITY.md: Human-first document answering SecOps questions (no telemetry, no outbound, no auto-updates, localhost-only daemon)
- CI security scan on every push: govulncheck (0 vulns), gosec (24 active rules), network audit (grep-enforced zero outbound connections)
- Fixed real Slowloris vulnerability (G112) -- added ReadHeaderTimeout to HTTP server
- Bumped Go to 1.25.7 (fixed crypto/tls GO-2026-4337)
- aoa-recon removed: cmd/aoa-recon/, npm/aoa-recon/, recon_bridge.go, recon.go cmd deleted. DimensionalResults/ReconAvailable moved to dimensional.go
- L4.4 Phase 2: parsers.json 509/509 provenance, GRAMMAR_REPORT.md, weekly CI, 346 contributor acknowledgments
- Build strategy decided: compile once per version, embed parsers.json via //go:embed
- L10.3 triple-green: CI grep-enforces zero net/http imports
- L10.4 triple-green: weekly CI validates all 509 grammars on 4 platforms

**10 commits shipped.**
