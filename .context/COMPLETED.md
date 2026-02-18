# aOa Completed Work

> Moved from GO-BOARD.md to keep the active board focused. Each phase includes what was built, why, and how it was validated.

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
| 5 | Read (ranged, ≥50% savings) | Claude | `aOa guided` | ↓90% (44k → 4k) | file:offset-end |
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
