# aOa GO-BOARD

> **Updated**: 2026-02-18 | **Phase**: Phase 8d/8e — Value Metrics & Dashboard Restructure | **Status**: 315+ tests passing, 0 failing
> **Architecture**: Hexagonal (ports/adapters) + Session Prism | **Target**: Single binary, zero Docker
> **Module**: `github.com/corey/aoa` | **Binary**: `cmd/aoa/main.go`
> **Completed work**: See `.context/COMPLETED.md` for Phases 1–8c (all validated)

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
go test ./...                    # All tests (315+ passing)
go test ./test/integration/ -v   # 42 integration tests
make check                       # Local CI: vet + lint + test
```

---

## Design Goals

| ID | Goal | aOa-go Solution | Status |
|----|------|-----------------|--------|
| G1 | O(1) Performance | All in-memory, observe() 5ns, autotune 2.5μs, zero hooks | ✅ |
| G2 | Grep/egrep Parity | Type-safe flag parsing, identical output format | ✅ |
| G3 | Domain Learning | Compiled binary, universal domains, event log | ✅ |
| G4 | Hit Tracking | All counters decayed in autotune, noise filter on in-memory maps | ✅ |
| G5 | Cohesive Architecture | Single binary, typed Go structs, hexagonal ports/adapters | ✅ |
| G6 | Embedded Storage | bbolt, no network, transactions native | ✅ |
| G7 | Memory Bounded | Explicit bounds, bbolt with compaction, project-scoped buckets | ✅ |

## Legend

| Symbol | Meaning |
|--------|---------|
| 🟢 | Confident — clear implementation path |
| 🟡 | Uncertain — may need research |
| 🔴 | Blocked — needs research first |

---

## Phase 8d: Value Metrics — Context Runway & Savings

**Goal:** Replace vanity metrics (raw token counts, keyword counts) with metrics that communicate real value. Every hero card, stat grid, and metrics panel should answer: *"What did aOa save me?"*

**Guiding principle:** Lead with the metric every Claude Code user has physically felt — context filling up and the model forgetting things.

**Three metric tiers (ascending perceived value):**

| Tier | Metric | Formula | Display |
|------|--------|---------|---------|
| Vanity | Tokens saved | `Σ(full_size - guided_size)` | Small/secondary — keep but de-emphasize |
| Tangible | Sessions extended | `tokens_saved / avg_session_cost` | Weekly rollup: "aOa gave you 3 extra sessions this week" |
| **Visceral** | **Context runway** | `(window_max - current_usage) / burn_rate` | **Lead metric:** "47 min remaining. Without aOa: 12 min." |

| ID | Area | Task | Priority | Status | Conf | Deps | Files | Test Strategy |
|----|------|------|:--------:|:------:|:----:|------|-------|---------------|
| V-01 | Backend | Burn rate accumulator — rolling window of tokens consumed per minute | Critical | TODO | 🟢 | - | `internal/app/app.go` | Accumulator produces stable rate after 5+ data points |
| V-02 | Backend | Context window max lookup — map model tag to window size | High | TODO | 🟢 | - | `internal/app/app.go` | opus-4 → 200k, sonnet → 200k, haiku → 200k |
| V-03 | Backend | Dual projection — with-aOa vs without-aOa burn rates using guided savings delta | Critical | TODO | 🟢 | V-01 | `internal/app/app.go` | Two rates diverge when guided reads occur |
| V-04 | Backend | Context runway API — expose both projections via `/api/runway` | High | TODO | 🟢 | V-03 | `web/server.go`, `socket/protocol.go` | Returns minutes remaining with and without aOa |
| V-05 | Backend | Weekly rollup persistence — sessions-extended counter survives daemon restart | Medium | TODO | 🟡 | V-03 | `adapters/bbolt/store.go` | Counter persists across restart, resets weekly |
| V-06 | Frontend | Live tab hero — context runway as primary display ("47 min remaining") | Critical | TODO | 🟢 | V-04 | `static/index.html` | Hero shows dual projection with visual contrast |
| V-07 | Frontend | Live tab metrics panel — replace prompts/domains with savings-oriented metrics | High | TODO | 🟢 | V-04 | `static/index.html` | Panel shows token savings, avg query speed, sessions extended |
| V-08 | Frontend | Stats grid revision — cards show rolling avg speed, tokens saved, guided ratio | High | TODO | 🟢 | V-04 | `static/index.html` | Cards communicate value, not raw counters |

**Kill list:** Time-saved ranges (e.g., "3.9h-9.6h") — ranges communicate uncertainty, undermine trust. Pick one defensible number or drop entirely.

---

## Phase 8e: Dashboard Restructure — 5-Tab Tactical Layout

**Goal:** Expand from 3 tabs to 5. Each tab maps to what the user is *doing* when they click it.

| New Tab | Old Tab | Content | What User Is Doing |
|---------|---------|---------|-------------------|
| **Live** | Overview | Real-time intent feed, activity table, context runway, status | "What's happening right now?" |
| **Recon** | *(new)* | Search interface — grep/egrep/find in the browser | "I need to find something" |
| **Intel** | Learning | Domain rankings, n-gram metrics, intent score | "What has aOa learned?" |
| **Debrief** | Conversation | Session stats, conversation feed, token metrics | "What just happened?" |
| **Arsenal** | *(new)* | Config, aliases, daemon status, setup, port management | "How is my system configured?" |

| ID | Area | Task | Priority | Status | Conf | Deps | Files | Test Strategy |
|----|------|------|:--------:|:------:|:----:|------|-------|---------------|
| T-01 | Frontend | Rename tabs: Overview→Live, Learning→Intel, Conversation→Debrief | High | TODO | 🟢 | - | `static/index.html` | All tab labels, IDs, CSS classes updated |
| T-02 | Frontend | Add Recon tab (stub) — "use `ag <query>` in your terminal" placeholder | Medium | TODO | 🟢 | T-01 | `static/index.html` | Tab renders, placeholder visible |
| T-03 | Frontend | Add Arsenal tab — daemon status, config display, alias instructions, port info | Medium | TODO | 🟢 | T-01 | `static/index.html` | Shows config data from `/api/health` |
| T-04 | Frontend | 5-tab header layout — ensure responsive at smaller widths | Medium | TODO | 🟢 | T-01 | `static/index.html` | Tabs don't overflow at 800px width |
| T-05 | Backend | Arsenal API — expose config, alias state, daemon info | Low | TODO | 🟢 | T-03 | `web/server.go` | `/api/config` returns project root, paths, alias status |

**Recon — future scope:** Full browser search UI. Stubbed for now with guidance card pointing to CLI.
**Arsenal — future scope:** Interactive `aoa init` in browser, alias toggle, .gitignore exception editor. Start read-only.

---

## Phase 8 — Remaining Attribution Work

**Attribution Philosophy:**
- **aOa gets credit** when it saves tokens or provides indexed speed
- **"productive"** for Write/Edit — desired development work, aOa not involved
- **"unguided"** for Claude's Grep/Glob — expensive operations where aOa indexed search is the alternative
- **System events** (Autotune, Learn) — aOa working in the background

### TODO Attribution Rows

| # | Action | Source | Attrib | aOa Impact | Target | Status |
|---|--------|--------|--------|------------|--------|--------|
| 8 | Write | Claude | `productive` | `-` | file | TODO |
| 9 | Edit | Claude | `productive` | `-` | file | TODO |
| 10 | Grep (Claude's) | Claude | `unguided` | est. Nk tokens | pattern | Partial — no token cost |
| 11 | Glob (Claude's) | Claude | `unguided` | est. Nk tokens | path/pattern | TODO |
| 14 | Autotune | aOa | `cycle N` | +P promoted, -D demoted, ~X decayed | — | TODO |
| 15 | Learn | aOa | `observe` | +N keywords, +M terms, +K domains | — | TODO |

### Tasks

| ID | Area | Task | Priority | Status | Conf | Deps | Files | Test Strategy |
|----|------|------|:--------:|:------:|:----:|------|-------|---------------|
| AT-06 | Backend | Verify autotune is firing correctly every 50 prompts | Critical | TODO | 🟢 | - | `internal/app/app.go` | Instrument autotune, confirm via session log |
| AT-04 | Backend | Autotune activity event: "Autotune \| aOa \| cycle N \| +P/-D/~X" | High | TODO | 🟢 | AT-06 | `internal/app/app.go` | Autotune emits activity entry |
| AT-02 | Backend | Glob attrib = "unguided" + estimated token cost | High | TODO | 🟢 | - | `internal/app/app.go` | Glob tool calls show token estimate |
| AT-01 | Backend | Write/Edit attrib = "productive" | Medium | TODO | 🟢 | - | `internal/app/app.go` | Write/Edit tool calls get attrib |
| AT-03 | Backend | Grep (Claude) impact = estimated token cost | Medium | TODO | 🟢 | - | `internal/app/app.go` | Grep tool calls show token estimate |
| AT-05 | Backend | Learn activity event (observe signals summary) | Low | TODO | 🟢 | - | `internal/app/app.go` | Observe emits summary activity entry |
| AT-07 | Frontend | Dashboard: color-code "productive" attrib (green) | Medium | TODO | 🟢 | AT-01 | `static/index.html` | Productive attrib styled green |
| AT-08 | Frontend | Dashboard: render token cost impact for Grep/Glob | Medium | TODO | 🟢 | AT-02, AT-03 | `static/index.html` | Token cost rendered in impact column |

### Flag Gap

| Flag | Short | Status |
|------|-------|--------|
| `--invert-match` | `-v` | Not implemented |

---

## Known Gaps

| Gap | Description | Priority |
|-----|-------------|----------|
| **File watcher not wired** | `fsnotify.Watcher` built and tested, `treesitter.Parser` built and tested, but `Watch()` never called in `app.go`. No dynamic re-indexing pipeline. | High |
| **bbolt lock contention** | `aoa init` fails while daemon holds the bbolt lock. Need in-process reindex command (socket command or `aoa daemon reindex`). | High |
| **Aho-Corasick stubbed** | `ports.PatternMatcher` interface exists, adapter dir exists, tests skipped. Future use for dimensional analysis bitmask scanning. | Low |

---

## Open Questions (Pending Discussion)

Items from feedback session that need alignment before becoming board tasks:

| # | Topic | Status | Summary |
|---|-------|--------|---------|
| 3 | **Alias strategy** | Needs answer | Goal: replace `grep` itself (not shortcuts). `grep auth` → `aoa grep auth` transparently. Eliminates 250-token prompt education tax. Graceful degradation on unsupported flags? |
| 4 | **Real-time conversation** | Needs investigation | Legacy Python showed real-time conversation despite shell/curl limitations. Go dashboard with 2s poll should do better — why isn't it? User to provide more direction. |
| 5 | **Intent score visualization** | Needs discussion | Formula: `coverage × confidence × momentum` (0-100). Lives in domain rankings (Intel tab) as the sorting/scoring mechanism. Traffic light vs number display. |

---

## Phase 9: Migration & Validation

**Goal:** Run both systems in parallel, prove equivalence

| ID | Area | Task | Priority | Status | Conf | Deps | Files | Test Strategy |
|----|------|------|:--------:|:------:|:----:|------|-------|---------------|
| M-01 | Migrate | Parallel run on 5 test projects (Python and Go side-by-side) | Critical | TODO | 🟢 | - | `test/migration/*.sh` | Both produce identical output |
| M-02 | Search | Diff search results: 100 queries/project, zero divergence | Critical | TODO | 🟢 | M-01 | `test/migration/search-diff.sh` | `diff` output = 0 for all queries |
| M-03 | Learner | Diff learner state: 200 intents, zero tolerance | Critical | TODO | 🟡 | M-01 | `test/migration/state-diff.sh` | JSON diff of state = empty |
| M-04 | Bench | Benchmark comparison (search, autotune, startup, memory) | High | TODO | 🟢 | M-01 | `test/benchmarks/compare.sh` | Confirm 50-120x speedup targets |
| M-05 | Docs | Migration path (stop Python, install Go, migrate data) | High | TODO | 🟢 | M-01 | `MIGRATION.md` | Existing user migrates cleanly |

## Phase 10: Distribution

**Goal:** Single binary, zero Docker, instant install

| ID | Area | Task | Priority | Status | Conf | Deps | Files | Test Strategy |
|----|------|------|:--------:|:------:|:----:|------|-------|---------------|
| R-01 | Parser | Purego .so loader for runtime grammar loading | Medium | TODO | 🟢 | - | `adapters/treesitter/loader.go` | Load .so, parse file, identical to compiled-in |
| R-02 | Build | Grammar downloader (CI: compile .so, host on GitHub Releases) | High | TODO | 🟡 | R-01 | `.github/workflows/build-grammars.yml` | Download+load 20 grammars from releases |
| R-03 | Build | Goreleaser (linux/darwin × amd64/arm64) | High | TODO | 🟢 | - | `.goreleaser.yml` | Binaries build for all 4 platforms |
| R-04 | Docs | Installation docs (`go install` or download binary) | Medium | TODO | 🟢 | R-03 | `README.md` | New user installs and runs cleanly |

## v2: Dimensional Analysis (Post-Release)

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

### Key Paths

| Purpose | Path |
|---------|------|
| Database | `{ProjectRoot}/.aoa/aoa.db` |
| Status line | `{ProjectRoot}/.aoa/status.json` |
| Socket | `/tmp/aoa-{sha256(root)[:12]}.sock` |
| HTTP port | `{ProjectRoot}/.aoa/http.port` |
| Dashboard | `http://localhost:{port}` |
| Session logs | `~/.claude/projects/{encoded-path}/*.jsonl` |
