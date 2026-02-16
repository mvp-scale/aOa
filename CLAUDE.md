# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Isolation Rule

**aOa-go is a standalone project.** Do NOT import, reference, copy, or depend on anything from outside the `aOa-go/` directory. No imports from the parent `aOa/` codebase. All dependencies come from Go modules (`go.mod`) or are written fresh here. This is a clean-room rewrite guided by behavioral specs and test fixtures in `test/fixtures/`, not by copying Python code.

## GO-BOARD

When referencing "the go board" or "GO-BOARD", this means `GO-BOARD.md` in this repo -- the project board for the Go port. Not the parent aOa board. Check it for current phase, task status, and session logs.

## Build and Test Commands

```bash
go build ./cmd/aoa/          # Build the binary (outputs ./aoa)
go vet ./...                  # Static analysis
go test ./...                 # Run all tests
go test ./... -v              # Verbose (shows skip reasons)
go test ./internal/domain/learner/ -run TestAutotune -v   # Single test/package
go test ./... -bench=. -benchmem -run=^$                  # Benchmarks only
make check                    # Local CI: vet + lint + test (run before committing)
```

The module path is `github.com/corey/aoa-go`. The binary entry point is `cmd/aoa/main.go`.

## Architecture

Hexagonal (ports/adapters) architecture. All domain logic is dependency-free. External concerns are behind interfaces in `internal/ports/`.

```
cmd/aoa/              Cobra CLI (grep, egrep, find, locate, tree, domains,
                      intent, bigrams, stats, config, health, wipe, daemon)

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
    socket/           Unix socket daemon (JSON-over-socket, /tmp/aoa-go-{hash}.sock)
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
| Status line | `{ProjectRoot}/.aoa/status-line.txt` |
| Socket | `/tmp/aoa-go-{sha256(root)[:12]}.sock` |
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
