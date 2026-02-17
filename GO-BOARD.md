# aOa GO-BOARD

> **Updated**: 2026-02-17 | **Phase**: Phase 8 — Web Dashboard Refinement | **Status**: 309+ tests passing, 0 failing | **Progress**: ~82%
> **Architecture**: Hexagonal (ports/adapters) + Session Prism | **Target**: Single binary, zero Docker
> **Module**: `github.com/corey/aoa` | **Binary**: `cmd/aoa/main.go`

---

## Quick Start

```bash
./aoa init                      # Index project
./aoa daemon start              # Returns terminal, prints dashboard URL
./aoa open                      # Opens http://localhost:{port} in browser
./aoa grep <query>              # Search (CLI remains operational-only)
./aoa egrep <pattern>           # Regex search
./aoa daemon stop               # Graceful shutdown
# Dashboard: http://localhost:{port}?mock  (mock mode)
```

**Build & Test:**
```bash
go build ./cmd/aoa/              # Build binary
go vet ./...                     # Static analysis
go test ./...                    # All tests (309 passing)
go test ./test/integration/ -v   # 42 integration tests
make check                       # Local CI: vet + lint + test
```

---

## Design Goals

| ID | Goal | aOa-go Solution |
|----|------|-----------------|
| G1 | O(1) Performance | All in-memory, observe() 5ns, autotune 2.5μs, zero hooks |
| G2 | Grep/egrep Parity | Type-safe flag parsing, identical output format |
| G3 | Domain Learning | Compiled binary, universal domains, event log |
| G4 | Hit Tracking | All counters decayed in autotune, noise filter on in-memory maps |
| G5 | Cohesive Architecture | Single binary, typed Go structs, hexagonal ports/adapters |
| G6 | Embedded Storage | bbolt, no network, transactions native |
| G7 | Memory Bounded | Explicit bounds, bbolt with compaction, project-scoped buckets |

## Legend

| Symbol | Meaning |
|--------|---------|
| 🟢 | Confident — clear implementation path |
| 🟡 | Uncertain — may need research |
| 🔴 | Blocked — needs research first |

| Status | Meaning |
|--------|---------|
| TODO | Not started |
| WIP | In progress |
| Done | Completed and validated |

---

## Completed Phases (1–7b)

All phases below are **done and validated**. Collapsed for clarity.

### Phase 1: Foundation & Test Harness — DONE
Project structure, hexagonal architecture, test fixtures (5 learner snapshots, 26 search queries, 13-file index), behavioral parity framework. 80 tests passing.

### Phase 2: Core Search Engine — DONE
O(1) indexed search, 26/26 parity tests, tree-sitter (28 languages, CGo), bbolt persistence, fsnotify watcher, Unix socket daemon. All search modes: literal, OR, AND, regex.

### Phase 3: Universal Domain Structure — DONE
134 domains across 15 focus areas embedded via `//go:embed`. 938 terms, 6566 keywords. Enricher: O(1) keyword→term→domain lookup (~20ns). Shared keywords, runtime noise filter.

### Phase 4: Learning System — DONE
`observe()`, 21-step autotune, competitive displacement (top 24 core). 5/5 fixture parity (zero float divergence). Cohit dedup, bigram extraction, keyword blocklist. Autotune ~2.5μs.

### Phase 5: Session Integration — DONE
Session Prism: Claude JSONL → tailer → parser → claude.Reader → canonical events. Signal chain: UserInput→bigrams, ToolInvocation→range gate→file_hits. Status line hook. 83 tests.

### Phase 6: CLI & Hardening — DONE
11 CLI commands (grep, egrep, find, locate, tree, config, health, wipe, daemon, init, open). Daemonized background process, PID management, lock contention diagnostics, orphan cleanup. 42 integration tests, all actionable error messages.

### Phase 7a: Web Dashboard — DONE
Embedded HTML dashboard (dark/light theme, responsive, surgical DOM updates). 4 API endpoints: health, stats, domains, bigrams. Removed 4 CLI commands (domains, intent, bigrams, stats) → dashboard. Mock mode (`?mock`). Universal pulse animation.

### Phase 7b: Dashboard Metrics Infrastructure — DONE
6 new API endpoints (conversation metrics/tools/feed, top keywords/terms/files, activity feed). Session event accumulators (tokens, tools, conversation ring buffer). All 3 tabs live with real data. Activity & Impact table with 7-column layout (Action, Source, Attrib, aOa Impact, Tags, Target, Time). Activity ring buffer with search observer timing. 309 tests passing.

**Key Endpoints (11 total):**
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
| `GET /api/activity/feed` | Last 20 activity entries (Action/Source/Attrib/Impact/Target) |

---

## Phase 8: Web Dashboard Refinement

**Goal:** Elevate the dashboard from functional to best-in-class. The legacy Python CLI showed rich, well-branded information at the command line. With a full browser canvas, we can do better. Three focus areas: Activity & Impact accuracy, Conversation narrative flow, and consistent aOa branding.

### Research References

Legacy behavioral specs used to derive Phase 8 requirements:

| Document | What It Contains |
|----------|-----------------|
| `research/legacy_cli/intent.png` | Screenshot of legacy `intent` command — the gold standard for Activity & Impact rendering |
| `research/legacy_cli/COLOR_PALET.md` | Full ANSI color palette + semantic color map (CYAN+BOLD=brand, GREEN=value, YELLOW=caution, MAGENTA=domains) |
| `research/legacy_cli/CC_THINKING.md` | How legacy extracts thinking blocks from Claude session JSONL (`.thinking` field asymmetry, content block types, display format) |

Key behavioral details from legacy `intent` renderer (`cli/src/50-intent.sh`):
- **Actions shown:** Everything with ≥1 file. Bash `./aoa` commands remapped to Search/Find/etc. Non-aOa Bash without file context filtered out.
- **"aOa guided":** Pure savings calculation — `file_size/4` vs `output_size/4`. Threshold ≥50% savings. NOT path-matching against search results.
- **Target format:** aOa commands = `<CYAN>aOa</CYAN> <GREEN>grep</GREEN> <plain>query</plain>`. All others = relative path (project root stripped), Read includes `:start-end` range.
- **Impact format:** Search = `<CYAN+BOLD>N hits</CYAN+BOLD> <DIM>│</DIM> <GREEN>X.XXms</GREEN>`. Guided reads = `<GREEN+BOLD>↓N%</GREEN+BOLD> (Xk → Yk)`. Zero hits = DIM.
- **Attrib vocabulary:** `indexed`, `multi-or`, `multi-and`, `regex`, `aOa guided`, `aOa`, `-` (and legacy-only: `semantic`, `targets`, `files`, `structure`, `filename`, `content`, `session`, `context`, `+N domains`, `cycle N`)

### 8a: Activity & Impact — Fix Pass

The Activity & Impact table (Overview tab) needs behavioral parity with the legacy `intent` command plus modern visual treatment.

**Completed in this session:**
- ✅ `FileMeta.Size` added to ports/storage — enables savings calculation from indexed file sizes
- ✅ `TurnAction` struct with Tool/Target/Range/Impact — structured tool action data
- ✅ `readSavings()` helper — computes `↓N%` from `FileMeta.Size` vs read limit
- ✅ Range-limited reads generate impact strings (e.g., `↓85%`)

| ID | Area | Task | Priority | Status | Conf | G2 | G5 | Deps | Files | Test Strategy |
|----|------|------|:--------:|:------:|:----:|:--:|:--:|------|-------|---------------|
| **BEHAVIORAL FIXES** |
| A-01 | Backend | Source capitalization: `"claude"` → `"Claude"` in backend | High | TODO | 🟢 | ✓ | | - | `internal/app/app.go` | Activity feed entries show "Claude" |
| A-02 | Backend | Attrib: `"and"` → `"multi-and"` for AND search mode | High | TODO | 🟢 | ✓ | | - | `internal/app/app.go` | AND search attrib = "multi-and" |
| A-03 | Backend | Filter Bash `./aoa` commands — already captured as Search by observer | Critical | TODO | 🟢 | ✓ | | - | `internal/app/app.go` | No duplicate Bash+Search entries |
| A-04 | Backend | Filter Bash commands without file context (e.g., `git status`) | High | TODO | 🟢 | | ✓ | A-03 | `internal/app/app.go` | Only file-bearing tool calls in feed |
| A-05 | Backend | Read targets: strip project root, show relative path + `:offset-limit` range | High | TODO | 🟢 | ✓ | | - | `internal/app/app.go` | Read target = `src/foo.go:200-400` |
| A-06 | Backend | All tool targets: strip project root prefix for relative paths | High | TODO | 🟢 | ✓ | | A-05 | `internal/app/app.go` | All targets use relative paths |
| **"aOa GUIDED" REWORK** |
| A-07 | Backend | Remove `guidedPaths` map — wrong approach (path-matching search results) | Critical | TODO | 🟢 | | ✓ | - | `internal/app/app.go` | Map and related code removed |
| A-08 | Backend | Implement savings-based attribution: `file_size` vs `output_size` | Critical | WIP | 🟢 | ✓ | | A-07 | `internal/app/app.go` | `FileMeta.Size` now available; `readSavings()` implemented |
| A-09 | Backend | Attrib = `"aOa guided"` when savings ≥ 50% (legacy threshold) | Critical | TODO | 🟢 | ✓ | | A-08 | `internal/app/app.go` | Reads with ≥50% token savings get attrib |
| A-10 | Backend | Impact format for guided reads: `↓N% (Xk → Yk)` with token approximation (bytes/4) | High | TODO | 🟢 | ✓ | | A-08 | `internal/app/app.go` | Impact shows savings arrow + before/after |
| **BRANDING & COLOR** |
| A-11 | Frontend | Target three-part color: `<cyan+bold>aOa</cyan+bold> <green>grep</green> <plain>query</plain>` | High | TODO | 🟢 | ✓ | | - | `static/index.html` | Target renders with 3 distinct colors |
| A-12 | Frontend | Source `aOa` styled cyan+bold (brand identity color) | High | TODO | 🟢 | ✓ | | - | `static/index.html` | Source column uses brand color |
| A-13 | Frontend | Attrib `aOa guided`: "aOa" in cyan+bold, "guided" in green | High | TODO | 🟢 | ✓ | | - | `static/index.html` | Two-tone attrib rendering |
| A-14 | Frontend | Impact hits: `<cyan+bold>N hits</cyan+bold> <dim>│</dim> <green>X.XXms</green>` | Medium | TODO | 🟢 | ✓ | | - | `static/index.html` | Three-part impact rendering |
| A-15 | Frontend | Impact `0 hits` rendered in dim/muted (not cyan) | Medium | TODO | 🟢 | ✓ | | - | `static/index.html` | Zero-hit style distinct from positive |
| A-16 | Frontend | Ensure all `aOa` text uses consistent casing and cyan+bold color | High | TODO | 🟢 | ✓ | | - | `static/index.html`, `app.go` | Brand audit across all surfaces |
| **TEST RUBRIC** |
| A-17 | Test | Build activity rubric: enumerate all action/source/attrib/impact combinations | High | TODO | 🟢 | ✓ | ✓ | - | test fixture or script | Every combination exercised and verified |

### 8b: Conversation Tab — Narrative Redesign ✅ DONE

Complete overhaul of the Conversation tab from flat metric cards to a two-column threaded narrative.

**What was built:**
- ✅ **Two-column feed card** — Yellow-bordered card, bottom-anchored scroll, bold yellow scrollbar
- ✅ **Turn grouping** — All assistant events (thinking + response + tools) between user inputs merge into one row via `currentBuilder`. No more fragmented 5-6 rows per exchange.
- ✅ **Left column: Conversation** — User line (yellow border), thinking line (purple, click to expand), assistant response (green, with model tag + timing + token count)
- ✅ **Right column: Actions** — Tool chips (color-coded by type) with target paths, range info, and impact badges. Footer with tool/edit/guided counts.
- ✅ **Thinking text captured** — `turnBuilder.ThinkingText` accumulates from `EventAIThinking`, truncated to 500 chars for display
- ✅ **Per-turn token usage** — `InputTokens`/`OutputTokens` accumulated per exchange, shown as "N tok" on assistant line
- ✅ **In-progress turn** — `ConversationTurns()` includes `currentBuilder` as first entry, so dashboard shows the turn being built live
- ✅ **Scroll management** — Auto-scroll to bottom on new turns, "Jump to now" button when scrolled up
- ✅ **NOW bar** — Green live indicator at bottom of feed
- ✅ **Tool usage panel removed** — Standalone tool distribution bar and top file reads/bash panels removed; tools shown per-turn in actions column
- ✅ **Agent-agnostic labels** — "Assistant" not "Claude"; model shown as tag (e.g., `opus-4-6`)

**Wire protocol expanded:**
- `ConversationTurnResult`: added `thinking_text`, `actions[]`, `input_tokens`, `output_tokens`
- `TurnActionResult`: new struct with `tool`, `target`, `range`, `impact`
- Frontend no longer fetches `/api/conversation/tools` (endpoint still exists for API consumers)

| ID | Area | Task | Priority | Status |
|----|------|------|:--------:|:------:|
| C-01 | Frontend | Redesign: conversation as threaded narrative | Critical | Done |
| C-02 | Frontend | Turn structure: User → thinking → response → tools | Critical | Done |
| C-03 | Backend | Extract thinking blocks from session events | High | Done |
| C-04 | Frontend | Real-time flow — in-progress turn visible | High | Done |
| C-05 | Frontend | Tool calls as inline action chips per turn | High | Done |
| C-06 | Frontend | User prompts: yellow accent | Medium | Done |
| C-07 | Frontend | Assistant responses: green accent | Medium | Done |
| C-08 | Frontend | Thinking blocks: dim, collapsible, click to expand | Medium | Done |
| C-09 | Frontend | Tool chips: action-colored pills | Medium | Done |
| C-10 | Frontend | Token metrics kept as stat cards above feed | Medium | Done |
| C-11 | Frontend | Removed standalone tool distribution/top panels | Medium | Done |
| C-12 | Frontend | Hero narrative retained above stat cards | Medium | Done |

### 8b-2: Learning Tab — Live Signal Visualization ✅ DONE

Enhanced the Learning tab with live visual feedback showing where learning signals are flowing.

**What was built:**
- ✅ **Term pills sorted by popularity** — Most-hit terms on left, least on right. Sorted by sum of keyword hits per term. Deterministic (no more random map iteration dance).
- ✅ **Term pills gray by default** — `var(--border-subtle)` background, `var(--mute)` text. Quiet baseline.
- ✅ **Term flash on hit** — When a keyword maps to a term and its count increases, that specific pill flashes bright green then fades back to gray over **30 seconds**. You see exactly where learning is happening.
- ✅ **Per-term hit counts in API** — `DomainInfo.TermHits map[string]int` sent to frontend for per-term change detection.
- ✅ **`DomainTermHitCounts()`** — New method sums keyword hits per term from learner state.
- ✅ **Number glow on change** — All stat card values (keywords, terms, bigrams, domain count, etc.) get a subtle blue text-shadow glow that fades over 4s when they change.
- ✅ **N-gram row flash** — Bigram, cohit kw→term, cohit term→domain rows get a green background flash (3s) and count glow (4s) when values change.
- ✅ **Up to 10 terms per row** — Increased from 5, smaller pills with flex-wrap + overflow hidden to stay in viewable area.

**Known gap: File watcher not wired** — The `fsnotify.Watcher` adapter is built and tested, `treesitter.Parser` is built and tested, but the file-watch → re-parse → update-index pipeline is not connected in `app.go`. The watcher is created and stopped but `Watch()` is never called. Dynamic re-indexing on file changes is a future task.

### 8c: Global Branding & Color System

Consistent semantic color language across all surfaces (dashboard, CLI, status line).

| Color | Semantic Role | CSS Variable | Usage |
|-------|--------------|-------------|-------|
| **CYAN+BOLD** | aOa brand identity | `var(--cyan)` + `font-weight: 600` | `aOa` everywhere — headers, source column, targets |
| **CYAN** | System/UI accent | `var(--cyan)` | Attrib values (indexed, multi-or, regex), tags, hit counts |
| **GREEN** | Positive/value | `var(--green)` | Savings arrows, timing, "guided", grep/egrep commands |
| **YELLOW** | Caution/in-progress | `var(--yellow)` | User prompts, learning state, warnings |
| **RED** | Error/critical | `var(--red)` | Errors, disconnected, critical context usage |
| **PURPLE/MAGENTA** | Domain names | `var(--purple)` | @authentication, @search — always purple |
| **DIM/MUTE** | De-emphasized | `var(--mute)` | Separators, secondary info, 0 hits, thinking blocks |
| **BOLD** | Emphasis | `font-weight: 700` | File paths, section headers, hit counts |

---

## Remaining Phases

### Phase 9: Migration & Validation

**Goal:** Run both systems in parallel, prove equivalence

| ID | Area | Task | Priority | Status | Conf | G1 | G2 | G3 | G4 | Deps | Files | Test Strategy |
|----|------|------|:--------:|:------:|:----:|:--:|:--:|:--:|:--:|------|-------|---------------|
| M-01 | Migrate | Parallel run on 5 test projects (Python and Go side-by-side) | Critical | TODO | 🟢 | | | | | - | `test/migration/*.sh` | Both produce identical output |
| M-02 | Search | Diff search results: 100 queries/project, zero divergence | Critical | TODO | 🟢 | ✓ | ✓ | | | M-01 | `test/migration/search-diff.sh` | `diff` output = 0 for all queries |
| M-03 | Learner | Diff learner state: 200 intents, zero tolerance | Critical | TODO | 🟡 | | | ✓ | ✓ | M-01 | `test/migration/state-diff.sh` | JSON diff of state = empty |
| M-04 | Bench | Benchmark comparison (search, autotune, startup, memory) | High | TODO | 🟢 | ✓ | | | | M-01 | `test/benchmarks/compare.sh` | Confirm 50-120x speedup targets |
| M-05 | Docs | Migration path (stop Python, install Go, migrate data) | High | TODO | 🟢 | | | | | M-01 | `MIGRATION.md` | Existing user migrates in <10 min |

### Phase 10: Distribution

**Goal:** Single binary, zero Docker, instant install

| ID | Area | Task | Priority | Status | Conf | G1 | G5 | Deps | Files | Test Strategy |
|----|------|------|:--------:|:------:|:----:|:--:|:--:|------|-------|---------------|
| R-01 | Parser | Purego .so loader for runtime grammar loading | Medium | TODO | 🟢 | ✓ | | - | `internal/adapters/treesitter/loader.go` | Load .so, parse file, identical to compiled-in |
| R-02 | Build | Grammar downloader (CI: compile .so, host on GitHub Releases) | High | TODO | 🟡 | | | R-01 | `.github/workflows/build-grammars.yml` | Download+load 20 grammars from releases |
| R-03 | Build | Goreleaser (linux/darwin × amd64/arm64) | High | TODO | 🟢 | | ✓ | - | `.goreleaser.yml` | Binaries build for all 4 platforms |
| R-04 | Docs | Installation docs (`go install` or download binary) | Medium | TODO | 🟢 | | | R-03 | `README.md` | New user installs and runs in <5 min |

### v2: Dimensional Analysis (Post-Release)

**Goal:** Multi-dimensional static analysis hints (security, performance, standards). Deferred.

| ID | Area | Task | Priority | Status | Conf | Deps | Files | Test Strategy |
|----|------|------|:--------:|:------:|:----:|------|-------|---------------|
| D-01 | Schema | Design dimension YAML schema (3 dims, 5 angles each) | Critical | TODO | 🟢 | - | `dimensions/schema.yaml` | Schema validates all dimension files |
| D-02 | Security | Build security dimension (~75-100 bits) | High | TODO | 🟡 | D-01 | `dimensions/security.yaml` | Catches known vulns in test projects |
| D-03 | Perf | Build performance dimension (~60-75 bits) | High | TODO | 🟡 | D-01 | `dimensions/performance.yaml` | Flags N+1, unbounded allocs |
| D-04 | Standards | Build standards dimension (~40-55 bits) | Medium | TODO | 🟡 | D-01 | `dimensions/standards.yaml` | Scores correlate with code review |
| D-05 | Compiler | Build aoa-dimgen compiler (YAML → binary, AC automaton) | Critical | TODO | 🟡 | D-01 | `cmd/aoa-dimgen/main.go` | All 3 dimensions compile, <100ms |
| D-06 | Analyzer | Implement Analyzer domain (ComputeBitmask, ScoreDimension) | Critical | TODO | 🟡 | D-05 | `internal/domain/analyzer/` | Bitmask ~100ns/line |
| D-07 | Format | Add dimension scores to search results (`S:-1 P:0 C:+2`) | High | TODO | 🟢 | D-06 | `internal/domain/index/format.go` | Scores in output |
| D-08 | Query | Dimension query support (`--dimension=security --risk=high`) | High | TODO | 🟢 | D-06 | `cmd/aoa/cmd/grep.go` | Filter by dimension |

---

## Architecture

```
cmd/aoa/              Cobra CLI (grep, egrep, find, locate, tree, config,
                      health, wipe, daemon, init, open)

internal/
  ports/              Interfaces + shared data types
  domain/
    index/            Search engine: O(1) token lookup, OR/AND/regex
    learner/          Learning system: observe(), 21-step autotune
    enricher/         Atlas keyword→term→domain resolution
    status/           Status line generation + file write
  adapters/
    bbolt/            Persistence (project-scoped buckets)
    socket/           Unix socket daemon (JSON-over-socket)
    web/              HTTP dashboard (embedded HTML, JSON API)
    tailer/           Session log tailer (defensive JSONL parser)
    claude/           Claude Code adapter (Session Prism)
    treesitter/       28-language structural parser (CGo)
    fsnotify/         File watcher (recursive, debounced)
    ahocorasick/      Multi-pattern string matching
  app/                Wiring: App struct, Config, lifecycle

atlas/v1/             134 semantic domains (15 JSON files, go:embed)
test/fixtures/        Behavioral parity data
hooks/                Status line hook
research/             Legacy CLI reference docs
```

### Signal Flow
```
Claude JSONL → tailer → parser → claude.Reader → app.onSessionEvent()
  UserInput     → flush currentBuilder → push user turn → promptN++, bigrams, status line
  AIThinking    → bigrams + buffer thinking text on currentBuilder
  AIResponse    → bigrams + buffer response text + per-turn & global token accumulators
  ToolInvocation → range gate → file_hits → observe
                 → TurnAction (tool/target/range/impact) on currentBuilder
                 → activity ring buffer (Action/Source/Attrib/Impact/Target)

Search (CLI) → searchObserver → learner signals + activity ring buffer

Dashboard poll (2s) → ConversationTurns() includes in-progress currentBuilder
                    → DomainTermNames() sorted by keyword hit popularity
                    → DomainTermHitCounts() for per-term flash detection
```

### Key Paths
| Purpose | Path |
|---------|------|
| Database | `{ProjectRoot}/.aoa/aoa.db` |
| Status line | `{ProjectRoot}/.aoa/status.json` |
| Socket | `/tmp/aoa-{sha256(root)[:12]}.sock` |
| HTTP port | `{ProjectRoot}/.aoa/http.port` |
| Dashboard | `http://localhost:{port}` |
| Session logs | `~/.claude/projects/{encoded-path}/*.jsonl` |

---

## Success Metrics

| Metric | Target | Status |
|--------|--------|--------|
| Search latency | <0.5ms | ✅ Verified |
| Autotune latency | <5ms | ✅ 2.5μs (2000x margin) |
| Startup time | <200ms | ✅ Verified |
| Memory footprint | <50MB | ✅ Verified |
| Behavioral parity | 100% | ✅ 26/26 search, 5/5 learner |
| Tests passing | 300+ | ✅ 309 |
| Hook elimination | 0-1 | ✅ Optional status line only |
| Haiku elimination | 0 | ✅ Universal atlas |

---

## Session Log

### 2026-02-17: Conversation Redesign + Data Pipeline + Learning Visualization

**Scope:** Phase 8b (Conversation tab), 8b-2 (Learning tab), partial 8a (FileMeta.Size, TurnAction)

**Backend changes:**
- `ports/storage.go`: Added `Size int64` to `FileMeta`
- `cmd/aoa/cmd/init.go`: Captures `info.Size()` during indexing
- `app/app.go`: Added `TurnAction` struct, `currentBuilder` exchange grouping (replaces per-TurnID fragmentation), `turnFromBuilder()`, `readSavings()`, `DomainTermHitCounts()`, `flushCurrentBuilder()`. `DomainTermNames()` now sorts by keyword hit popularity. In-progress turn included in `ConversationTurns()` response.
- `socket/protocol.go`: Added `TurnActionResult`, expanded `ConversationTurnResult` with `thinking_text`, `actions[]`, `input_tokens`, `output_tokens`. `DomainInfo` gained `TermHits map[string]int`.
- `socket/server.go`: `AppQueries` interface expanded with `DomainTermHitCounts()`. Both web and socket servers populate `TermHits`.

**Frontend changes:**
- Conversation tab: Complete HTML/CSS/JS overhaul — two-column feed card (messages left, actions right), yellow border, bottom-anchored scroll, NOW bar, jump-to-now button. Removed standalone Tool Usage card.
- Learning tab: Term pills gray by default, flash green on hit with 30s fade. Terms sorted by popularity (most-hit left). Up to 10 terms per row. Blue text-shadow glow on all stat number changes (4s). N-gram rows flash green background (3s) + count glow on change.
- Global: `set()` helper upgraded from color pulse to `num-glow` animation.

**Key fix: Turn grouping** — Replaced per-TurnID `turnBuffer` map with single `currentBuilder`. All assistant events between user inputs accumulate into one exchange. Eliminated fragmented 5-6 row turns.

**Key fix: Term dancing** — `DomainTermNames()` was iterating a Go map (random order each call). Now sorts by keyword hit popularity with alphabetical tiebreaker. Stable and deterministic.

**Known gaps identified:**
- File watcher (`fsnotify`) built but not wired — `Watch()` never called, no dynamic re-indexing
- Activity rubric needed — enumerate all action/source/attrib/impact combinations for testing
- `aoa init` can't run while daemon holds bbolt lock — need in-process reindex command
