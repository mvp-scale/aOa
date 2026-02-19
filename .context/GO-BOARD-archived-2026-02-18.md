# aOa GO-BOARD

> **Updated**: 2026-02-18 | **Phase**: Phase 8d/8e — Value Metrics & Dashboard Restructure | **Status**: 315+ tests passing, 0 failing | **Context**: 88%
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
| **Recon** | *(new)* | Dimensional intelligence — security/perf/quality/compliance/architecture/observability drill-down | "Where are concerns in my codebase?" |
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
| T-06 | Frontend | Relative timestamps — "now", "12s", "3m" (no "ago" suffix) | High | Done | 🟢 | - | `static/index.html` | Timestamps update live, weighted distribution, shared `relativeTime()` helper |
| T-07 | Frontend | Activity table: fixed columns, no jitter — `table-layout:fixed`, 2/3 left + 1/3 target | High | Done | 🟢 | - | `static/index.html` | Columns: time 8%, action 10%, source 10%, attrib 15%, impact 24%, target 33% |
| T-08 | Frontend | Activity table: responsive — drop time+target at 900px, keep action/source/attrib/impact (20/20/25/35) | High | Done | 🟢 | T-07 | `static/index.html` | 4-column layout at narrow width, balanced spacing |
| T-09 | Frontend | Negative feedback loop — unguided Grep/Glob: red pills, red attrib, red target, wasted token estimate in impact | High | Done | 🟢 | - | `static/index.html` | Grep/Glob rows read as costly end-to-end; productive=green, unguided=red |

| T-10 | Frontend | Standardized hero section — 160px min-height, persuasion headline + support line + cause→effect hero metrics | High | Done | 🟢 | - | all mockups | Hero row height, structure, typography consistent across all 5 tabs |
| T-11 | Frontend | Hero persuasion engine — JSON-driven Identity/Outcome/Separator/Exclusion rotating headlines | High | Done | 🟢 | T-10 | `static/hero.json`, all mockups | 60 combos per tab (5 identities × 3 stories × 4 separators) |
| T-12 | Frontend | Recon: column-based dimension indicators — tier headers replace per-row pill badges | High | Done | 🟢 | - | `_throwaway_mockups/recon.html` | Column headers with clickable tier toggles |
| T-13 | Frontend | Recon: tree breadcrumb navigation — Root › folder › file integrated into tree card header | High | Done | 🟢 | T-12 | `_throwaway_mockups/recon.html` | Every segment clickable, "Root" always links back |
| T-14 | Frontend | Recon: tier toggle persistence — click column header or sidebar tier to toggle on/off, persists in localStorage | High | Done | 🟢 | T-12 | `_throwaway_mockups/recon.html` | Toggle state survives page reload |

**Dashboard design standard (v2):**
- **Hero row**: 160px min-height, 2:1 flex ratio (hero card + hero metrics)
- **Hero card**: gradient-border wrapper → label (static) → hero headline (persuasion, rotates) → hero support (single data line, dot separators)
- **Hero headline pattern**: `{Identity} {outcome} . . . {separator} {exclusion}.` — Identity 22px bold gradient, outcome 17px semi-bold, separator cyan, exclusion dim. Line 2 indented 33%.
- **Hero metrics**: 2×2 cause→effect grid with arrows. Shows how one metric drives the next.
- **Hero data**: `static/hero.json` — shared identities/separators, 3 outcome/exclusion story pairs per tab
- **Support line**: single flowing line with `·` dot separators, colored value numbers. Data-driven, not persuasion.
- **Three-tier page narrative**: Hero row (claim) → Stats grid (evidence) → Data (operational detail)
- **Padding**: 24px all sides, gap 20px between sections. Responsive stacks at 900px.

**Mockup status:**
- `live.html` — persuasion hero, cause→effect metrics (guided→savings→tokens→sessions), flowing support line, autotune moved to stats grid
- `recon.html` — hero row inside .main (sidebar-aligned), column-based dim indicators, tree breadcrumb, tier toggle on/off with localStorage persistence
- `intel.html` — hero row added above traffic card, cause→effect metrics (score→coverage, core→confidence)
- `debrief.html` — hero row with wrapper, cause→effect metrics (input→output, cache→saved)
- `arsenal.html` — hero row with wrapper, cause→effect metrics (files→symbols, latency→autotune)

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
| AT-07 | Frontend | Dashboard: color-code attribs — green for "productive", **red for "unguided"** (negative feedback loop) | Medium | Done | 🟢 | AT-01 | `static/index.html` | Productive=green, unguided=red, Grep/Glob pills red, mock data updated |
| AT-08 | Frontend | Dashboard: render token cost impact for Grep/Glob | Medium | TODO | 🟢 | AT-02, AT-03 | `static/index.html` | Token cost rendered in impact column |
| AT-09 | Backend | Target capture — preserve full query syntax as-is, no normalization (regex, multi-arg, etc.) | High | TODO | 🟢 | - | `internal/app/app.go` | Regex queries stored verbatim, not normalized |

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
| 6 | **Glob token cost visibility** | Needs research | Claude's Glob/Read operations consume large token counts (entire files read into context). Do the JSONL session logs expose the token count for each tool result? If yes, we can show actual cost in the "unguided" impact column. If no, we need a heuristic (file size × ~0.25 tokens/byte). This research unblocks AT-02 and AT-08 with real numbers instead of estimates. |

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

## v2: Dimensional Analysis — Recon

**Prerequisite:** Multi-language tree-sitter integration must be mature first. The dimensional engine piggybacks on tree-sitter's AST parse — it needs a richer parser with proven coverage across multiple languages before we can validate cross-language pattern matching. This phase starts after Phase 10 and multi-language tree-sitter validation are complete.

**Research:** Detailed design, worked examples, and viability assessment in:
- `.context/research/bitmask-dimensional-analysis.md` — 67 security questions, per-line bitmask worked example, execution pipeline, cross-language uniformity analysis
- `.context/research/asv_vs_lsp.md` — AST vs LSP comparison, viability verdict across all 6 tiers (~250-290 questions total)

**Goal:** Structural code analysis via tree-sitter AST pattern matching + AC text scanning. Early warning system — yellow flags, not verdicts. Surfaces concerns as navigable dimension tags (NER-style entity detection applied to code). 6 tiers, 22 dimensions — all detectable by scanning code files only. Users can acknowledge, dismiss, or ignore findings.

**6 Tiers:**

| Tier | Color | Dimensions | What it catches |
|------|-------|-----------|----------------|
| **Security** | Red | Injection, Secrets, Auth Gaps, Cryptography, Path Traversal | SQL/cmd/XSS injection, hardcoded keys, missing auth, weak hash, symlink |
| **Performance** | Yellow | Query Patterns, Memory, Concurrency, Resource Leaks | N+1, unbounded alloc, mutex over I/O, unclosed handles |
| **Quality** | Blue | Complexity, Error Handling, Dead Code, Conventions | God functions, ignored errors, unreachable code, magic numbers |
| **Compliance** | Purple | CVE Patterns, Licensing, Data Handling | Known vuln patterns, copyleft conflicts, PII in logs |
| **Architecture** | Cyan | Import Health, API Surface, Anti-patterns | Circular deps, layer violations, global state, hardcoded config |
| **Observability** | Green | Silent Failures, Debug Artifacts | Swallowed errors, leftover print statements, TODO markers |

**Detection architecture (dual engine):**

| Layer | Engine | Detects | Speed | Example |
|-------|--------|---------|-------|---------|
| **Structural** | Tree-sitter AST walker | Patterns that require code structure | ~100-500μs/file | SQL injection (concat → query call), N+1 (loop → db call), god function (branch count) |
| **Text** | Aho-Corasick automaton | Simple literal patterns | ~15μs/file | Hardcoded secrets, known-bad function names, TODO markers |

Both engines produce bits in the same bitmask. The Recon tab doesn't care which engine found it.

**Structural query format** — one definition per detection, language-agnostic with a thin node-name mapping:

```yaml
- id: sql_injection
  severity: critical
  dimension: security
  query:
    match: call_expression
    receiver_contains: [db, query, exec, prepare]
    has_arg:
      type: binary_expression
      contains_string: ["SELECT", "INSERT", "UPDATE", "DELETE"]
      contains_identifier: true
  lang_map:
    go: { call: call_expression, string: interpreted_string_literal }
    python: { call: call, string: string }
    javascript: { call: call_expression, string: template_string }
```

**Tree-sitter is already built** — 28 languages compiled in, tested. ASTs are already produced at `aoa init` for symbol extraction. The detection engine reuses those ASTs — no re-parse cost.

### Tasks

| ID | Area | Task | Priority | Status | Conf | Deps | Files | Test Strategy |
|----|------|------|:--------:|:------:|:----:|------|-------|---------------|
| D-01 | Schema | Design structural query YAML schema (AST patterns + lang_map + AC text patterns) | Critical | TODO | 🟢 | - | `dimensions/schema.yaml` | Schema validates both structural and text detection definitions |
| D-02 | Engine | Tree-sitter AST walker — match structural patterns against parsed AST | Critical | TODO | 🟢 | D-01 | `internal/domain/analyzer/walker.go` | Walks AST, matches query definitions, returns bit positions |
| D-03 | Engine | AC text scanner — compile text patterns into automaton, scan raw source | High | TODO | 🟢 | D-01 | `internal/domain/analyzer/text_scan.go` | Reuses `adapters/ahocorasick/`, returns bit positions |
| D-04 | Engine | Language mapping layer — normalize AST node names across 28 languages | High | TODO | 🟡 | D-02 | `internal/domain/analyzer/lang_map.go` | Same structural query matches Go + Python + JS + Rust |
| D-05 | Analyzer | Bitmask composer — merge structural + text bits, compute weighted severity score | Critical | TODO | 🟢 | D-02, D-03 | `internal/domain/analyzer/score.go` | Bitmask per file/method, score = weighted sum of set bits |
| D-06 | Dimensions | Security tier (5 dims: injection, secrets, auth, crypto, path traversal) | High | TODO | 🟡 | D-01 | `dimensions/security/*.yaml` | Catches known vulns in test projects |
| D-07 | Dimensions | Performance tier (4 dims: queries, memory, concurrency, resource leaks) | High | TODO | 🟡 | D-01 | `dimensions/performance/*.yaml` | Flags structural perf issues |
| D-08 | Dimensions | Quality tier (4 dims: complexity, error handling, dead code, conventions) | Medium | TODO | 🟡 | D-01 | `dimensions/quality/*.yaml` | Scores correlate with code review findings |
| D-08b | Dimensions | Compliance tier (3 dims: CVE patterns, licensing, data handling) | Medium | TODO | 🟡 | D-01 | `dimensions/compliance/*.yaml` | CVE matches, license conflicts, PII exposure |
| D-08c | Dimensions | Architecture tier (3 dims: import health, API surface, anti-patterns) | Medium | TODO | 🟡 | D-01 | `dimensions/architecture/*.yaml` | Circular deps, layer violations, global state |
| D-08d | Dimensions | Observability tier (2 dims: silent failures, debug artifacts) | Low | TODO | 🟡 | D-01 | `dimensions/observability/*.yaml` | Swallowed errors, leftover debug statements |
| D-09 | Integration | Wire analyzer into `aoa init` — scan all files, store bitmasks in bbolt | High | TODO | 🟢 | D-05 | `internal/app/app.go` | Bitmasks persist, available to search + dashboard |
| D-10 | Format | Add dimension scores to search results (`S:-1 P:0 C:+2`) | High | TODO | 🟢 | D-05 | `internal/domain/index/format.go` | Scores in output |
| D-11 | Query | Dimension query support (`--dimension=security --risk=high`) | High | TODO | 🟢 | D-05 | `cmd/aoa/cmd/grep.go` | Filter by dimension |
| D-12 | Frontend | Recon tab — NER-style dimensional view: dimension toggle sidebar, file→method drill-down, severity scoring | High | TODO | 🟢 | D-09 | `static/index.html` | Mockup validated in `_throwaway_mockups/recon.html` |

**Extensibility:** New dimensions are just new YAML directories. Add `dimensions/testing/*.yaml` or `dimensions/accessibility/*.yaml` — the engine picks them up. No code changes. New tiers appear in the sidebar automatically.

**Research — Neural 1-bit embeddings (deferred):**
Investigated and deprioritized. Pre-trained embedding models (nomic-embed 137M, mxbai-embed 335M) encode semantic similarity, not security/quality properties — signal-to-noise ratio is poor for vulnerability detection. Probing the embedding space (XOR + popcount) showed vocabulary changes fire the same magnitude of signal as structural changes. A fine-tuned classifier would work but requires thousands of labeled examples per detection. The deterministic tree-sitter + AC approach gives better signal with full interpretability. Revisit only if the AC/AST pattern library hits a ceiling on novel code shapes.

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

---

## Session Log

### 2026-02-18: Hero Section Standardization & Recon UX Polish

**Scope:** Dashboard UX design system. All 5 mockups updated. No production code changes.

**Hero persuasion engine designed and implemented:**
- Defined copywriting framework for hero cards: Identity / Outcome / Pause / Separator / Exclusion
- Created `static/hero.json` — shared identity pool (10x Developers, Relentless Builders, Precision Engineers, Full-Stack Architects, High-Velocity Teams), 4 separators (minus the, instead of, bypassing, without), 3 curated outcome/exclusion story pairs per tab
- Typography hierarchy: identity 22px bold gradient, outcome 17px semi-bold, cyan pause dots, separator cyan 17px, exclusion dim. Line 2 indented 33% from left
- Randomized composition: 60 unique combinations per tab on each page load

**Hero row standardized across all 5 tabs:**
- 160px min-height, 2:1 flex ratio (hero card + hero metrics)
- Hero card: label (static) → headline (persuasion, rotates) → support (single flowing data line with dot separators)
- Hero metrics: 2×2 cause→effect grid with arrows — each tab tells its own causal story
- Three-tier page narrative established: Hero (claim) → Stats grid (evidence) → Data (operational detail)

**Live tab hero refactored:**
- Removed runway block and old narrative, replaced with persuasion headline + single support line
- Hero metrics panel: replaced vertical metric list with cause→effect grid (guided reads → avg savings, tokens saved → sessions extended)
- Autotune moved from hero metrics to stats grid (replaced Active Domains)
- Padding/gap standardized to 24px/20px

**Recon UX polish:**
- Moved hero row inside `.main` to align with sidebar (was outside `.layout`)
- Column-based dimension indicators: replaced per-row pill badges (11 Sec, 7 Perf...) with header columns + number-only rows
- Tree breadcrumb integrated into card header: Root › folder › file — every segment clickable
- Tier toggle on column headers and sidebar: click to toggle entire tier on/off
- localStorage persistence for all dimension toggle state
- Removed redundant "project root" breadcrumb element
- Fixed padding/gap inconsistency (was 20px/16px, now 24px/20px)

**Intel spacing fixed:**
- Removed stacked margin-bottom on hero-row, traffic-card, stat-grid that was doubling the gap from `.main`'s `gap: 20px`

---

### 2026-02-18: Strategy Session — Feedback, Value Metrics, Dashboard Restructure, Mockups

**Scope:** Product direction alignment. No code changes to production. New tasks added to board. Static mockups created.

**Feedback captured:**
- Created `research/feedback/OUTLINE.md` — full system map (dashboard tabs/sections, CLI commands, backend systems) with founder notes on each section
- Legacy system responses ingested for 5 strategic questions: value metrics, tab naming, alias strategy, conversation feed, intent score

**Decisions made and added to board:**

*Phase 8d — Value Metrics:*
- Lead metric: **context runway** — `(window_max - current_usage) / burn_rate` → "47 min remaining. Without aOa: 12 min."
- Secondary: **sessions extended** weekly rollup — `tokens_saved / avg_session_cost`
- Kill: time-saved ranges (undermine trust). Token counts stay but de-emphasized.
- 8 tasks added (V-01 through V-08)

*Phase 8e — 5-Tab Tactical Layout:*
- Tabs renamed: **Live** (was Overview) / **Recon** (new) / **Intel** (was Learning) / **Debrief** (was Conversation) / **Arsenal** (new)
- Each tab named after what the user is doing when they click it
- 5 tasks added (T-01 through T-05)

**Open questions (on board, pending next session):**
- **#3 Alias strategy** — goal is replacing `grep` itself, not adding shortcuts. `grep auth` → `aoa grep auth` transparently. Eliminates 250-token prompt education tax in sub-agents.
- **#4 Real-time conversation** — legacy Python showed real-time despite shell/curl limits; Go dashboard with 2s poll should do better. Needs investigation.
- **#5 Intent score visualization** — formula `coverage × confidence × momentum` (0-100). Lives in domain rankings (Intel tab) as the sorting mechanism. Traffic light vs number display still open.

**Board cleanup:**
- Moved all completed phases (1–8c) to `.context/COMPLETED.md` with expanded why/validation notes
- Board trimmed from ~545 lines to ~251 lines
- Added "Open Questions" section for unresolved strategic items

**Static mockups created** (`_throwaway_mockups/`):
- `live.html` — context runway hero, value-oriented stats grid, full activity table
- `recon.html` — command reference, interactive search demo with 70 symbols across 10 query categories
- `intel.html` — intent score traffic light, domain rankings with composite score, n-gram metrics
- `debrief.html` — two-column conversation feed (no yellow border), NOW bar, tool action chips
- `arsenal.html` — daemon status, project config, alias checklist with `eval "$(aoa shell-init)"`, perf metrics
- All 5 pages: correct nav (Live/Recon/Intel/Debrief/Arsenal), consistent width (`padding: 24px`, `gap: 20px`), dark/light theme, "Generate Mock Data" button, inter-page links
- Old mockups archived to `_throwaway_mockups/_remove/`

**Next session focus:** Review mockups in browser, give feedback on layout/content, then begin implementation starting with tab rename (T-01) or value metrics backend (V-01).
