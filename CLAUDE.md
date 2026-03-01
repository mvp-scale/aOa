# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Isolation Rule

**aOa is a standalone project.** Do NOT import, reference, copy, or depend on anything from outside the `aOa/` directory. No imports from the parent `aOa/` codebase. All dependencies come from Go modules (`go.mod`) or are written fresh here. This is a clean-room rewrite guided by behavioral specs and test fixtures in `test/fixtures/`, not by copying Python code.

## Communication Rule

**Always reference work by task ID.** Use board IDs (L4.4-3.1, L10.3, L5.Va) not abstract labels ("Phase 3", "the grammar work"). Task IDs are the shared language — anything else creates ambiguity.

## Advisory Rule

**When you see a better approach, say so before writing code.** Reference the specific goal it aligns to (G0–G6 from `.context/GOALS.md`) and explain why. For example: "This speaks to G3 (Agent-First) — there's an approach using X that would be simpler. Want to explore it?" Let the user choose. Do not silently go along with an approach you know is suboptimal, and do not unilaterally switch approaches without explaining why. Build trust through transparent recommendations, not through compliance or surprise rewrites.

## Build Rule

**All builds MUST use `./build.sh` or `make build`.** Running `go build` directly is forbidden — a compile-time guard (`cmd/aoa/build_guard.go`) enforces this by panicking at startup. Do not bypass, remove, or modify the build guard. Do not run `go build ./cmd/aoa/` under any circumstances. Do not pass `-tags` to `go build` directly. The build script is the single source of truth for how binaries are produced.

- `./build.sh` — standard build (tree-sitter runtime, dynamic grammars)
- `./build.sh --light` — light build (no tree-sitter, pure Go, minimal)
- `./build.sh --recon` — deprecated (code retained, not active)
- `./build.sh --recon-bin` — deprecated (code retained, not active)

**Everything is project-scoped.** All data, grammars, shims, and config live under `{ProjectRoot}/.aoa/`. Nothing goes in `~/.aoa/`. `aoa remove` wipes everything cleanly.

## Goals

**Read `.context/GOALS.md` before any planning or architectural decision.** Every plan, code change, and design choice must align with the goals defined there. If a proposed change violates any goal, redesign before proceeding.

## Project Context

### .context/ Layout

```
.context/
  GOALS.md        # Project goals — read before any planning or design decision
  INDEX.md        # Derived index — line pointers, unblocked/blocked, layer status
  CURRENT.md      # Session checklist — done/in-progress/next
  BOARD.md        # Source of truth — task table + supporting detail
  COMPLETED.md    # Archived completed work
  BACKLOG.md      # Deferred items
  decisions/      # ADRs (date-prefixed)
  details/        # Research & discovery (date-prefixed)
  archived/       # Session bridges, old boards
```

**Board**: `.context/BOARD.md` is the project board. Check it for task status and open work.

**Context loading**: Read `.context/INDEX.md` first for fast orientation. It has line pointers into BOARD.md so you can read targeted sections instead of the full file.

### Beacon Agent

Beacon manages project continuity across `.context/`. Trigger with "Hey Beacon".

| Trigger | What Beacon Does |
|---------|-----------------|
| "where are we", "status" | Reads INDEX + CURRENT, presents state |
| "update the board" | Batch-updates BOARD -> INDEX -> CURRENT |
| "capture", "write this up" | Writes to decisions/ or details/ |
| "new session" | Archives CURRENT, bumps session counter |
| "move to completed" | Moves triple-green tasks to COMPLETED.md |

**Speed rule**: Most work happens WITHOUT Beacon. Update CURRENT.md directly as you work. Beacon spawns for board-level operations only.

## Build and Test Commands

```bash
./build.sh                    # Build the binary (tree-sitter + dynamic grammars)
./build.sh --light            # Light build (no tree-sitter, pure Go)
go vet ./...                  # Static analysis
go test ./...                 # Run all tests
go test ./... -v              # Verbose (shows skip reasons)
go test ./internal/domain/learner/ -run TestAutotune -v   # Single test/package
go test ./... -bench=. -benchmem -run=^$                  # Benchmarks only
make check                    # Local CI: vet + lint + test (run before committing)
```

**CRITICAL: Never run `go build ./cmd/aoa/` directly.** A compile-time guard (`build_guard.go`) will panic. All builds go through `./build.sh` or `make build`.

The module path is `github.com/corey/aoa`. The binary entry point is `cmd/aoa/main.go`.

## Architecture

Hexagonal (ports/adapters) architecture. All domain logic is dependency-free. External concerns are behind interfaces in `internal/ports/`.

```
cmd/aoa/              Cobra CLI (grep, egrep, find, locate, tree, config,
                      health, wipe, daemon, init, open)

internal/
  ports/              Interfaces + shared data types (Index, LearnerState,
                      SearchOptions, SessionEvent, TokenRef, SymbolMeta)
  domain/
    index/            Search engine: O(1) token lookup, OR/AND/regex modes,
                      tokenizer, domain enrichment, result formatting
    learner/          Learning system: observe(), 21-step autotune, competitive
                      displacement, bigram extraction, cohit dedup
    enricher/         Atlas keyword->term->domain resolution (O(1) map lookup)
    status/           Status line generation + file write
  adapters/
    bbolt/            Persistence (project-scoped buckets, JSON serialization)
    socket/           Unix socket daemon (JSON-over-socket, /tmp/aoa-{hash}.sock)
    web/              HTTP dashboard (embedded HTML, JSON API, localhost-only)
    tailer/           Session log tailer (defensive JSONL parser, UUID dedup)
    claude/           Claude Code adapter (Session Prism: raw JSONL -> canonical events)
    treesitter/       28-language structural parser (CGo, compiled-in grammars)
    fsnotify/         File watcher (recursive, debounced, filtered)
    ahocorasick/      Multi-pattern string matching
  app/                Wiring: App struct owns all components, Config for init

atlas/v1/             134 semantic domains embedded via //go:embed (15 JSON files)
test/fixtures/        Behavioral parity data (search queries, learner snapshots)
hooks/                Status line hook for Claude Code integration
```

### Signal Flow

The daemon tails Claude Code session logs (no hooks needed for learning):

```
Claude JSONL -> tailer -> parser -> claude.Reader -> app.onSessionEvent()
  UserInput     -> promptN++, bigrams, status line
  AIThinking    -> bigrams
  AIResponse    -> bigrams
  ToolInvocation -> range gate (limit 0-500) -> file_hits -> observe
```

Searches also generate learning signals via `searchObserver` in `app.go`.

### Thread Safety

`App.mu` serializes all learner access. The Learner itself is NOT thread-safe -- the caller (App) must hold the mutex. Socket server handles concurrent clients; searches are concurrent until they hit the observer.

### Key Paths

| Purpose | Path |
|---------|------|
| Database | `{ProjectRoot}/.aoa/aoa.db` (bbolt, project-scoped) |
| Status line | `{ProjectRoot}/.aoa/status.json` |
| Socket | `/tmp/aoa-{sha256(root)[:12]}.sock` |
| HTTP port | `{ProjectRoot}/.aoa/http.port` (port number for dashboard) |
| Dashboard | `http://localhost:{port}` (localhost-only, auto-refreshing) |
| Session logs | `~/.claude/projects/{encoded-path}/*.jsonl` (read-only) |

## Behavioral Parity

Zero-tolerance divergence from Python. Test fixtures in `test/fixtures/` are the source of truth.

Critical precision rules:
- `DomainMeta.Hits` is **float64** -- NO int truncation during decay
- All other hit maps (KeywordHits, TermHits, etc.) truncate via `math.Trunc(float64(count) * 0.90)` to match Python `int()`
- Constants: `DecayRate=0.90`, `AutotuneInterval=50`, `PruneFloor=0.3`, `CoreDomainsMax=24`

## Test Structure

- **Unit tests**: colocated with source (`internal/domain/learner/learner_test.go`)
- **Parity tests**: `test/parity_test.go` + `test/helpers_test.go` load fixtures and assert exact match
- **Fixtures**: `test/fixtures/search/` (26 queries, 13-file index) and `test/fixtures/learner/` (5 state snapshots, 3 event streams)
- Uses `testify/assert` and `testify/require`
