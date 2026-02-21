# aOa-go - Beacon

> **Session**: 63 | **Date**: 2026-02-20
> **Phase**: L6 complete — Two-binary distribution shipped (aoa + aoa-recon)

---

## Now

Validate the L6 two-binary integration end-to-end. aoa-pure is built and running.

## Next Steps

1. **Validate aoa-pure standalone** — Run `aoa init` on this project with the pure Go binary. Confirm file-level indexing (files + tokens, 0 symbols). Run `aoa grep` and verify file-level search works.

2. **Validate aoa-recon enhance** — Build aoa-recon, run `aoa-recon enhance --db .aoa/aoa.db --root .`. Verify symbols appear in index. Run same grep query — confirm symbol-level results.

3. **Validate auto-discovery** — Place both binaries in same directory. Start daemon. Confirm daemon log says "aoa-recon found". Hit `/api/recon` — confirm `recon_available: true`. Open dashboard — Recon tab shows scan results.

4. **Validate graceful degradation** — Remove aoa-recon. Restart daemon. Confirm `recon_available: false`. Dashboard Recon tab shows install prompt.

5. **Build up aoa-recon** — Once integration is validated, expand aoa-recon with:
   - Full bitmask engine (L5.1–L5.5) wired into enhance command
   - Scanner patterns beyond the current 10 lightweight detectors
   - `aoa-recon scan` command for standalone scan output (no daemon needed)
   - Per-file incremental via watcher bridge

## Just Shipped (Session 63)

1. **L6.1** — `ports.Parser` interface decouples domain from treesitter adapter
2. **L6.2** — `App.Parser` optional; `CGO_ENABLED=0 go build ./cmd/aoa/` → 7.5 MB pure Go binary
3. **L6.3** — `defaultCodeExtensions` + `TokenizeContentLine` fallback for file-level search without parser
4. **L6.4** — `cmd/aoa-recon/` with enhance, enhance-file, --version subcommands
5. **L6.5** — Scanner extracted to `internal/adapters/recon/scanner.go` (shared by web + CLI)
6. **L6.6** — `ReconBridge` auto-discovers aoa-recon (PATH → .aoa/bin/ → sibling)
7. **L6.7** — Dashboard Recon tab shows install prompt when recon not available
8. **L6.8–L6.9** — npm packages (wrapper + 8 platform packages, esbuild/turbo pattern)
9. **L6.10** — Release workflow (8 matrix jobs + npm publish)
10. **docs/build.md** — Full build, test, and distribution guide

## Key Decisions (This Session)

- **cgo build constraint** — `parser_cgo.go` / `parser_nocgo.go` for compile-time parser switching. No custom build tags.
- **git-lfs discovery model** — PATH → .aoa/bin/ → sibling. Zero config. Non-blocking goroutine for enhance.
- **Scanner shared, not duplicated** — `recon.Scan(idx, lines)` with `LineGetter` interface. Web handler is thin adapter.
- **npm esbuild/turbo pattern** — `optionalDependencies` with `os`/`cpu` constraints. One binary per platform download.
- **Consistent version** — Both binaries use `--version` flag (Cobra built-in), not subcommand.

## Test Status

**All passing, 0 failing** (go test ./...)

- New: `TestBuildIndex_NilParser_TokenizationOnly`
- All existing tests preserved

## Key Files

```
internal/ports/parser.go                 -- Parser interface (new)
internal/app/app.go                      -- Parser as interface field, recon bridge wiring
internal/app/indexer.go                  -- defaultCodeExtensions, tokenization fallback
internal/app/watcher.go                  -- Nil-guard parser, content tokenization, recon trigger
internal/app/recon_bridge.go             -- ReconBridge: discover + invoke aoa-recon (new)
internal/adapters/recon/scanner.go       -- Shared scanner package (new, extracted from web)
internal/adapters/web/recon.go           -- Thin handler calling shared scanner
cmd/aoa/cmd/parser_cgo.go               -- CGo parser factory (new)
cmd/aoa/cmd/parser_nocgo.go             -- No-CGo nil parser (new)
cmd/aoa/cmd/grammar_cgo.go              -- Grammar commands (CGo only)
cmd/aoa/cmd/grammar_nocgo.go            -- Grammar stub (no CGo)
cmd/aoa-recon/main.go                   -- aoa-recon binary (new)
npm/                                     -- 10 npm packages (new)
.github/workflows/release.yml            -- 8-matrix build + npm publish
docs/build.md                            -- Build & distribution guide
Makefile                                 -- build-pure, build-recon targets
```

## Resume Command

```
Read .context/CURRENT.md
Read docs/build.md
```

Then validate the integration: build-pure, init, grep, build-recon, enhance, grep again, daemon with auto-discovery.

---

## Project Snapshot

| Layer | Status | Tests |
|-------|--------|-------|
| Phases 1-8c | COMPLETE | 308 base |
| L0 - Value Engine | COMPLETE | 24 tests |
| L1 - Dashboard | COMPLETE | 5-tab SPA |
| L2 - Infra Gaps | COMPLETE | 22 tests |
| L3 - Migration | COMPLETE | 55 parity tests |
| L4 - Distribution | COMPLETE | Grammar loader + goreleaser |
| L5 - Dimensional Analysis | IN PROGRESS | L5.12 interim (pattern scanner) |
| L6 - Distribution v2 | COMPLETE | Two-binary split, npm, CI |

## Architecture Reference

```
cmd/aoa/              Cobra CLI — pure Go (CGO_ENABLED=0 capable)
cmd/aoa-recon/        Cobra CLI — CGo, tree-sitter + scanning

internal/
  ports/              Interfaces: Storage, Watcher, SessionReader, PatternMatcher, Parser
  domain/
    index/            Search engine + FileCache + tokenizer + content scanning
    learner/          Learning system (observe, autotune, bigrams, cohits)
    enricher/         Atlas keyword→term→domain resolution
    status/           Status line generation
  adapters/
    bbolt/            Persistence
    socket/           Unix socket daemon
    web/              HTTP dashboard (embedded SPA)
    recon/            Shared scanner (patterns + scan logic)
    tailer/           Session log tailer
    claude/           Session Prism (JSONL → canonical events)
    treesitter/       28-language structural parser (CGo)
    fsnotify/         File watcher
    ahocorasick/      Multi-pattern string matching
  app/                Wiring + lifecycle + ReconBridge

npm/                  10 npm packages (2 wrappers + 8 platform)
atlas/v1/             134 semantic domains (embedded)
test/fixtures/        Behavioral parity data
```
