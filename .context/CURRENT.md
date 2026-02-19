# aOa-go - Beacon

> **Session**: 55 | **Date**: 2026-02-19
> **Phase**: L2 complete, pre-L3 — Hot file cache shipped, observer pipeline redesign next

---

## Now

Refactor the search observer signal pipeline. Two paths, same analysis:

1. **CLI search (synchronous, must be fast)**: Top N hits → direct-increment domains/terms already on hits. Content text → `ProcessBigrams` for bigram/cohit threshold gating (count >= 6 before promotion). No enricher re-tokenization.

2. **Session log (asynchronous, non-blocking)**: Claude's grep/egrep/search tool results → same pipeline via tailer goroutine.

## Active

| # | Task | Status |
|---|------|--------|
| Observer pipeline | Refactor searchObserver: direct-increment domains/terms from hits, content → ProcessBigrams, threshold gate at 6 | Designed, not implemented |
| Session log search signals | Wire grep/egrep results from session JSONL through same pipeline | Designed, not implemented |

## Just Shipped (Session 55)

1. **Hot file cache** — `FileCache` pre-reads all indexed files into `[]string` at startup. Binary filtering (extension blocklist + null byte + MIME). 512KB/file, 250MB budget. Re-warms on `Rebuild()`. 8 tests.
2. **fileSpans caching** — `buildFileSpans()` precomputed in `NewSearchEngine()`/`Rebuild()` instead of per-search.
3. **Version display** — `internal/version/version.go` with ldflags injection. `aoa --version`, daemon start/stop show git hash + build date. `make build` target.
4. **Observer cascade fix** — Stopped re-tokenizing `hit.Content` through enricher (was producing 137 keywords from single-token query). Partial fix; full pipeline refactor is next.

## Key Decisions (This Session)

- **FileCache budget**: 250MB default, configurable via `Config.CacheMaxBytes`. Files sorted by LastModified desc for priority.
- **Observer signal pipeline (approved)**: Domains/terms from hits → direct increment. Content text → ProcessBigrams only. Threshold gate at 6. Same pipeline for CLI and session log paths.
- **No enricher on content text**: Content hits already have Tags (pre-resolved terms via `generateContentTags`). Re-tokenizing through enricher was redundant and noisy.

## Test Status

**388 passing, 0 failing**

- 8 new FileCache tests
- All existing tests preserved (content_test.go covers disk fallback path)

## Key Files

```
internal/version/version.go              -- Version vars + String(), ldflags target
internal/domain/index/filecache.go       -- FileCache: WarmFromIndex, GetLines, Invalidate, Stats
internal/domain/index/filecache_test.go  -- 8 tests
internal/domain/index/search.go          -- fileSpans + cache fields, SetCache(), warm in Rebuild()
internal/domain/index/content.go         -- Uses cache + cached fileSpans, readFileLines fallback
internal/app/app.go                      -- CacheMaxBytes config, cache creation, observer fix
cmd/aoa/cmd/daemon.go                    -- Version in start/stop output
cmd/aoa/cmd/root.go                      -- rootCmd.Version
Makefile                                 -- build target with ldflags
```

## Resume Command

```
Read .context/CURRENT.md
Read GO-BOARD.md
```

Then implement the search observer pipeline refactor (searchObserver in app.go).

---

## Project Snapshot

| Layer | Status | Tests |
|-------|--------|-------|
| Phases 1-8c | COMPLETE | 308 base |
| L0 - Value Engine | COMPLETE | 24 tests |
| L1 - Dashboard | COMPLETE | 5-tab SPA |
| L2 - Infra Gaps | COMPLETE | 22 tests |
| Session 55 | File cache + version | 8 new tests |
| L3 - Migration | TODO | - |

## Architecture Reference

```
cmd/aoa/              Cobra CLI (grep, egrep, find, locate, tree, config,
                      health, wipe, daemon, init, open)
internal/
  version/            Version vars (ldflags injection)
  ports/              Interfaces + shared types
  domain/
    index/            Search engine + FileCache + tokenizer + content scanning
    learner/          Learning system (observe, autotune, bigrams, cohits)
    enricher/         Atlas keyword→term→domain resolution
    status/           Status line generation
  adapters/
    bbolt/            Persistence
    socket/           Unix socket daemon
    web/              HTTP dashboard (embedded SPA)
    tailer/           Session log tailer
    claude/           Session Prism (JSONL → canonical events)
    treesitter/       28-language structural parser
    fsnotify/         File watcher
    ahocorasick/      Multi-pattern string matching
  app/                Wiring + lifecycle
atlas/v1/             134 semantic domains (embedded)
test/fixtures/        Behavioral parity data
```
