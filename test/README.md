# aOa Test Pipeline

> Last updated: 2026-02-27

---

## Where Tests Live (and Why)

Tests follow Go convention: **unit tests colocated with source, cross-cutting tests here in `test/`**. This isn't accidental — Go's package visibility rules require unit tests to live alongside the code they test so they can access internal types.

```
test/                               Cross-cutting tests (parity, perf, CLI, migration)
├── parity_test.go                  Behavioral parity vs Python (fixture-driven)
├── helpers_test.go                 Shared fixture loaders
├── benchmark_test.go               7 performance benchmarks with G1 speed targets
├── gauntlet_test.go                22-shape search regression matrix
├── integration/
│   └── cli_test.go                 65+ CLI integration tests (daemon, init, wipe, grep)
├── migration/
│   ├── grep_parity_test.go         100+ grep/egrep flag combinations vs fixture index
│   └── unix_grep_parity_test.go    50+ output format tests vs GNU grep
├── fixtures/
│   ├── SPEC.md                     264-line canonical behavioral specification
│   ├── search/
│   │   ├── index-state.json        13-file fixture index (134 symbols, 8 domains)
│   │   └── queries.json            26 search queries with expected results
│   ├── learner/
│   │   ├── 00-fresh.json           Fresh learner state (0 prompts)
│   │   ├── 01-fifty-intents.json   After 50 intents
│   │   ├── 02-hundred-intents.json After 100 intents
│   │   ├── 03-two-hundred.json     After 200 intents
│   │   ├── 04-post-wipe.json       After wipe reset
│   │   ├── events-01-to-50.json    Event stream (intents 1-50)
│   │   ├── events-51-to-100.json   Event stream (intents 51-100)
│   │   ├── events-101-to-200.json  Event stream (intents 101-200)
│   │   └── compute_fixtures.py     Python script to regenerate fixtures
│   └── observe/                    Signal processing test data (placeholder)
└── testdata/
    └── benchmarks/                 Baseline files for benchstat comparison

internal/domain/                    16 unit test files (core logic)
├── learner/                        learner_test, autotune_test, bigrams_test,
│                                   dedup_test, displace_test, observe_test
├── index/                          index_test, tokenizer_test, format_test,
│                                   content_test, invert_test, rebuild_test, filecache_test
├── enricher/                       enricher_test (atlas loading, 134 domains)
├── analyzer/                       score_test, rules_test, rules_security_test,
│                                   lang_map_test, types_test
└── status/                         status_test

internal/adapters/                  18 unit test files (external boundaries)
├── treesitter/                     parser_test, loader_test, walker_test,
│                                   loader_e2e_test, debug_ignored_test
├── bbolt/                          store_test (persistence, crash recovery)
├── socket/                         server_test (JSON-over-socket protocol)
├── web/                            server_test (HTTP dashboard, JSON API)
├── fsnotify/                       watcher_test (file change detection)
├── tailer/                         tailer_test (JSONL parsing)
├── claude/                         reader_test (Session Prism translation)
├── ahocorasick/                    matcher_test (multi-pattern matching)
└── recon/                          engine_test (//go:build recon only)

internal/app/                       13 integration test files (wiring layer)
├── activity_test, session_test, indexer_test, reindex_test, watcher_test
├── autotune_integration_test, burnrate_test, contentmeter_test
├── models_test, paths_test, ratetracker_test, runway_test, shadow_test
```

**Total: ~54 test files, 250+ test functions.**

---

## How It Aligns

The test structure mirrors the hexagonal architecture:

| Layer | Location | What's Tested | Isolation |
|-------|----------|---------------|-----------|
| **Domain** | `internal/domain/*_test.go` | Pure logic: search, learning, enrichment, scoring | No I/O, no network, no disk. Fast. |
| **Adapters** | `internal/adapters/*_test.go` | Boundaries: bbolt, sockets, HTTP, file watching, parsers | Uses temp dirs, mock interfaces |
| **App** | `internal/app/*_test.go` | Wiring: components connected, signals flowing | Uses real bbolt in temp dirs |
| **Parity** | `test/parity_test.go` | Go output matches Python exactly | Loads shared fixtures |
| **Gauntlet** | `test/gauntlet_test.go` | 22-shape regression canary, catches 1000x+ slowdowns | 500-file synthetic index |
| **Benchmarks** | `test/benchmark_test.go` | G1 speed targets (16-500x faster than Python) | Isolated per-benchmark |
| **Integration** | `test/integration/cli_test.go` | Full binary: daemon lifecycle, CLI commands, file watcher | Compiles binary, temp project dirs |
| **Migration** | `test/migration/*.go` | Drop-in grep replacement: flag matrix + GNU output format | Fixture index + real grep comparison |

---

## How to Execute

### Quick Check (Before Every Commit)

```bash
make check
```

This runs the full local CI gate in order: `go vet` → `golangci-lint` → `go test ./...` → `./build.sh` → binary size validation (max 20 MB). If this passes, CI will pass.

### Run Everything

```bash
go test ./...                       # All tests (skipped tests are expected)
go test ./... -v                    # Verbose (shows skip reasons)
go test ./... -v 2>&1 | grep -E "SKIP|PASS|FAIL"   # Summary view
```

### Run by Layer

```bash
# Domain unit tests (fastest — pure logic, no I/O)
go test ./internal/domain/...

# Adapter tests (fast — temp dirs, mock interfaces)
go test ./internal/adapters/...

# App wiring tests (medium — real bbolt, temp dirs)
go test ./internal/app/...

# Parity + gauntlet (fixture-driven correctness)
go test ./test/ -v

# CLI integration (slower — compiles binary, starts daemons)
go test ./test/integration/ -v

# Grep migration (flag matrix + GNU grep comparison)
go test ./test/migration/ -v
```

### Run a Single Test

```bash
go test ./internal/domain/learner/ -run TestAutotune -v
go test ./test/ -run TestSearchParity -v
go test ./test/integration/ -run TestDaemon_StartStop -v
```

### Performance

```bash
make bench                          # All benchmarks
make bench-gauntlet                 # 22-shape regression suite (6 runs, benchstat)
make bench-baseline                 # Save baseline to test/testdata/benchmarks/
make bench-compare                  # Compare current vs saved baseline
```

### Coverage

```bash
make coverage                       # Prints per-function coverage, cleans up
```

---

## Full End-to-End Test Order

When doing a formal regression run (pre-release, post-refactor, or after a long session), execute in this order. Each phase builds confidence for the next.

### Phase 1: Static Analysis

```bash
go vet ./...
golangci-lint run ./...
```

Catches type errors, dead code, lint violations. Zero-cost, instant feedback. If this fails, nothing else matters.

### Phase 2: Domain Unit Tests

```bash
go test ./internal/domain/... -v
```

Tests the core logic layer — search engine, learner, enricher, tokenizer, scoring. These are pure functions with no I/O. If domain logic is broken, everything above it is broken too. **This is the foundation.**

Key tests:
- `learner/autotune_test.go` — 21-step autotune math, float precision matching Python
- `index/tokenizer_test.go` — camelCase, dotted, hyphenated tokenization
- `index/content_test.go` — Content search, body matching
- `enricher/enricher_test.go` — Atlas loading (134 domains, 938 terms, 3407 keywords)

### Phase 3: Adapter Unit Tests

```bash
go test ./internal/adapters/... -v
```

Tests the boundary layer — persistence, network, file watching, parsing. Uses temp directories and mock interfaces. If adapters are broken, the app can't talk to the outside world.

Key tests:
- `bbolt/store_test.go` — Index/learner persistence, crash recovery
- `socket/server_test.go` — JSON-over-socket protocol
- `web/server_test.go` — HTTP dashboard, JSON API
- `tailer/tailer_test.go` — Defensive JSONL parsing
- `claude/reader_test.go` — Session Prism event translation

### Phase 4: App Wiring Tests

```bash
go test ./internal/app/... -v
```

Tests component integration — indexer + parser + watcher + store wired together. Uses real bbolt in temp directories. If wiring is broken, the daemon won't function.

Key tests:
- `session_test.go` — Session boundaries, event tracking, learner persistence
- `autotune_integration_test.go` — Autotune firing via search observer (50-search trigger)
- `watcher_test.go` — File watcher → reindex → search finds new symbols
- `runway_test.go` — Context window projection, burn rate calculations

### Phase 5: Behavioral Parity

```bash
go test ./test/ -v
```

Validates Go output matches Python exactly using shared fixtures. This is the **zero-tolerance** layer — if parity breaks, the rewrite has diverged from spec.

Key tests:
- `TestSearchParity` — 26 fixture queries against 13-file index, exact match
- `TestSearchGauntlet_G0Ceiling` — 22 query shapes, ceiling validation (canary for regressions)

Skipped tests (features not yet implemented):
- `TestAutotuneParity_*` — Learner state snapshots at 50/100/200 intents
- `TestObserveSignalParity`, `TestDedupParity`, `TestDisplacementParity`
- `TestEnrichmentParity`, `TestBigramParity`
- `TestFullSessionParity_200Intents`, `TestSearchDiff_100Queries`

### Phase 6: Grep Migration Parity

```bash
go test ./test/migration/ -v
```

Validates aOa as a drop-in grep replacement. Two sub-layers:

- **`grep_parity_test.go`** — 100+ tests covering every grep/egrep flag and combination against the fixture index. Individual flags (-i, -w, -v, -c, -q, -m, -a, -E, -e, --include, --exclude), flag combos, edge cases (short tokens, Unicode, camelCase tokenization), context lines, only-matching.

- **`unix_grep_parity_test.go`** — 50+ tests comparing output format against real GNU grep. Output format (single file, multi file, line numbers), positional file arguments, stdin piping, exit codes (0/1/2), color/TTY behavior, combined real-world flag patterns.

### Phase 7: CLI Integration

```bash
go test ./test/integration/ -v
```

The heaviest tests — compiles the actual binary, creates temp project directories, starts/stops daemons, runs CLI commands. Tests the full user-facing surface.

Key areas:
- **V-05**: Standalone commands (tree, config)
- **V-06**: Init (happy path, empty dir, reinit, delegated, locked DB)
- **V-07**: Wipe (direct, via daemon, no data, locked DB)
- **V-08**: Daemon lifecycle (start/stop, double start, stale socket, locked DB)
- **V-09**: Search (grep, egrep, no daemon, no query)
- **V-10**: Query commands (health, find, locate, open)
- **V-11**: No-daemon error cases (table-driven)
- **V-12**: Error message quality
- **V-13**: L2 features (invert match, reindex finds new symbols)
- **L2.1**: File watcher auto-reindex (create, modify, delete)
- **Dashboard**: HTTP endpoints, HTML serving, port cleanup

### Phase 8: Performance (Optional, Pre-Release)

```bash
make bench                          # All 7 benchmarks against G1 targets
make bench-gauntlet                 # 22-shape regression suite
```

G1 speed targets (vs Python baseline):
- Search: <500µs (16-30x faster than Python 8-15ms)
- Observe: <10µs (300-500x faster than Python 3-5ms)
- Autotune: <5ms (50-120x faster than Python 250-600ms)
- Index file: <20ms (2.5-10x faster than Python 50-200ms)
- Cold start: <200ms (15-40x faster than Python 3-8s)
- Memory: <50MB heap for 500-file index

---

## Build Tags

Most tests run in the standard build. Special cases:

| Tag | Files | When |
|-----|-------|------|
| `//go:build recon` | `internal/adapters/recon/engine_test.go` | Only with `./build.sh --recon` |
| `//go:build !lean` | `internal/adapters/treesitter/walker_test.go`, `debug_ignored_test.go` | Excluded from lean builds |
| *(none)* | Everything else (50+ files) | Always runs |

---

## CI Pipeline

**GitHub Actions** (`.github/workflows/ci.yml`):
- Triggers on push to main and all PRs
- Steps: `go vet ./...` → `go test ./...` → CGO_ENABLED=0 build → 15 MB size gate
- Pure Go validation with strict size gating

**Local CI** (`make check`):
- Same gates plus golangci-lint and 20 MB size limit
- Run before every commit

---

## Test Philosophy

**Fixtures are truth.** Test fixtures in `test/fixtures/` were captured from the running Python aOa. They define correct behavior — not documentation, not assumptions, not what we think should happen. When Go output doesn't match a fixture, the Go code is wrong.

**Colocated by design.** Unit tests live next to their source because Go requires it for package-internal access. Cross-cutting tests live in `test/` because they span packages. Don't consolidate — the separation is load-bearing.

**Skipped tests are roadmap.** The 10 skipped parity tests in `test/parity_test.go` map directly to unimplemented features (learner replay, dedup, displacement, enrichment, bigrams). When a feature ships, its parity test gets unskipped.

**Performance is a feature.** The gauntlet's 22-shape ceiling test runs in `go test ./...` — it's a canary, not a benchmark. If any search path regresses 1000x+, the ceiling test fails and blocks the commit. Detailed benchmarks are separate (`make bench`).
