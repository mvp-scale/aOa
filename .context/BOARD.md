# Board

[Board](#board) | [Supporting Detail](#supporting-detail) | [Completed](COMPLETED.md) | [Backlog](BACKLOG.md)

> **Updated**: 2026-02-23 (Session 66) | **86% complete.**
> **Completed work**: See [COMPLETED.md](COMPLETED.md) — Phases 1–8c + L0 + L1 + L2.2–L2.7 + L3 + L4.1/L4.3 + L5.1–L5.6/L5.9 + L6.1–L6.6 (470+ active tests, 32 skipped)
> **Archived boards**: `.context/archived/`

---

## Goals

> Atomic architectural principles. Every task is evaluated against each goal independently.

| Goal | Statement |
|------|-----------|
| **G0** | **Speed** — 50-120x faster than Python. Sub-millisecond search, <200ms startup, <50MB memory. |
| **G1** | **Parity** — Zero behavioral divergence from Python. Test fixtures are the source of truth. |
| **G2** | **Single Binary** — One `aoa` binary. Zero Docker, zero runtime deps, zero install friction. |
| **G3** | **Agent-First** — Replace `grep`/`find` transparently for AI agents. Minimize prompt education tax. |
| **G4** | **Clean Architecture** — Hexagonal. Domain logic is dependency-free. External concerns behind interfaces. |
| **G5** | **Self-Learning** — Adaptive pattern recognition. observe(), autotune, competitive displacement. |
| **G6** | **Value Proof** — Surface measurable savings. Context runway, tokens saved, sessions extended. |

---

## Board Structure

> Layered architecture. Each layer builds on the one below. TDD — validation gates at every layer.

### Layers

| Layer | Name | Purpose | Gate Method |
|-------|------|---------|-------------|
| **L0** | Value Engine | Burn rate, context runway, attribution signals | Runway API returns valid projections; attribution rubric covers all tool actions |
| **L1** | Dashboard | 5-tab layout, mockup implementation, hero narratives | All 5 tabs render with live data; mockup parity validated in browser |
| **L2** | Infra Gaps | File watcher, bbolt lock, CLI flags, sub-ms content search | `aoa init` works while daemon runs; file changes trigger re-index; `aoa grep` ≤1ms |
| **L3** | Migration | Parallel run Python vs Go, parity proof | 100 queries × 5 projects = zero divergence; benchmark confirms speedup |
| **L4** | Distribution | Goreleaser, grammar loader, install docs | `go install` or binary download works on linux/darwin × amd64/arm64 |
| **L5** | Dimensional Analysis | Bitmask engine, 6-tier scanning, Recon tab | Security tier catches known vulns in test projects; query time < 10ms |
| **L6** | Distribution v2 | Two-binary split, npm packaging, zero-friction install | `npm install -g aoa` works; `npm install -g aoa-recon` lights up Recon tab |
| **L7** | Onboarding UX | First-run experience, progress feedback, project state hygiene | User sees meaningful progress during startup; `.aoa/` is clean and self-documenting |

### Columns

| Column | Purpose |
|--------|---------|
| **Layer** | Layer grouping. Links to layer detail below. |
| **ID** | Task identifier (layer.step). Links to task reference below. |
| **G0-G6** | Goal alignment. `x` = serves this goal. Blank = not relevant. |
| **Dep** | ID of blocking task, or `-` |
| **Cf** | Confidence — see indicator reference below |
| **St** | Status — see indicator reference below |
| **Va** | Validation state — see indicator reference below |
| **Task** | What we're doing |
| **Value** | Why we're doing this |
| **Va Detail** | How we prove it |

### Indicator Reference

| Indicator | Cf (Confidence) | St (Status) | Va (Validation) |
|:---------:|:----------------|:------------|:----------------|
| ⚪ | — | Not started | Not yet validated |
| 🔵 | — | In progress | — |
| 🟢 | Confident | Complete | Automated test proves it (unit or integration) |
| 🟡 | Uncertain | Pending | Partial — manual/browser only, or unit but no integration. See Va Detail for gap. |
| 🔴 | Lost/Blocked | Blocked | Failed |

> 🟢🟢🟢 = done. Task moves to COMPLETED.md.

---

## Mission

**North Star**: One binary that makes every AI agent faster by replacing slow, expensive tool calls with O(1) indexed search -- and proves it with measurable savings.

**Current**: Core engine complete (search, learner, dashboard, grep parity). Recon scanning operational with caching and investigation tracking. Two-binary distribution built but not yet published. Focus shifting to dimensional rule expansion, operational polish, and onboarding.

**Approach**: TDD. Each layer validated before the next. Completed work archived to keep the board focused on what's next.

---

## Board

| Layer | ID | G0 | G1 | G2 | G3 | G4 | G5 | G6 | Dep | Cf | St | Va | Task | Value | Va Detail |
|:------|:---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:----|:--:|:--:|:--:|:-----|:------|:----------|
| **L0** | | | | | | | | | | | | | *All 12 tasks complete — see COMPLETED.md* | | |
| **L1** | | | | | | | | | | | | | *All 8 tasks complete — see COMPLETED.md* | | |
| [L2](#layer-2) | [L2.1](#l21) | x | | | | x | x | | - | 🟢 | 🟢 | 🟡 | Wire file watcher — `Watch()` in app.go, change→reparse→reindex | Dynamic re-indexing without restart | Unit: 5 tests. **Gap**: no integration test through fsnotify event pipeline |
| **L3** | | | | | | | | | | | | | *All tasks complete — see COMPLETED.md* | | |
| [L3](#layer-3) | [L3.15](#l315) | | x | | x | | | | - | 🟢 | 🟢 | 🟡 | GNU grep native parity — 3 modes, 22 flags, stdin/files/index routing | Drop-in grep replacement for AI agents | Smoke tests pass. **Gap**: no automated parity test suite |
| [L4](#layer-4) | [L4.2](#l42) | | | x | | | | | L4.1 | 🟡 | 🟢 | 🟡 | Grammar CLI + build CI — `aoa grammar list/install/info` | Easy grammar distribution | CLI works. **Gap**: actual download not implemented |
| [L4](#layer-4) | [L4.4](#l44) | | | x | x | | | | L4.3 | 🟢 | ⚪ | ⚪ | Installation docs — `go install` or download binary | Friction-free onboarding | New user installs and runs in <2 minutes |
| [L5](#layer-5) | [L5.7](#l57) | | | | | | | | L5.1 | 🟡 | ⚪ | ⚪ | Performance tier — 4 dims (queries, memory, concurrency, resource leaks) | Second tier | Flags N+1, unbounded allocs |
| [L5](#layer-5) | [L5.8](#l58) | | | | | | | | L5.1 | 🟡 | ⚪ | ⚪ | Quality tier — 4 dims (complexity, error handling, dead code, conventions) | Third tier | God functions, ignored errors |
| [L5](#layer-5) | [L5.10](#l510) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Add dimension scores to search results (`S:-1 P:0 C:+2`) | Scores visible inline | Scores appear in grep/egrep output |
| [L5](#layer-5) | [L5.11](#l511) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Dimension query support — `--dimension=security --risk=high` | Filter by dimension | CLI filters by tier and severity |
| [L5](#layer-5) | [L5.12](#l512) | | | | | | | x | L5.9 | 🟢 | 🟢 | 🟡 | Recon tab — dimensional engine with interim scanner fallback | Dashboard dimensional view | API works. **Gap**: dashboard UI upgrade for bitmask scores |
| [L5](#layer-5) | [L5.13](#l513) | | | | | | | x | L5.12 | 🟢 | 🟢 | 🟡 | Recon dashboard overhaul — 5 focus modes, tier redesign, code toggle, copy prompt | Recon tab is actionable, not just a finding list | **Gap**: browser-only validation |
| [L5](#layer-5) | [L5.14](#l514) | x | | | | x | | | L5.12 | 🟢 | 🟢 | 🟡 | Recon cache + incremental updates — pre-compute at startup, SubtractFile/AddFile on file change | Zero per-poll scan cost, instant API response | **Gap**: no unit tests for incremental path |
| [L5](#layer-5) | [L5.15](#l515) | | | | | | | x | L5.14 | 🟢 | 🟢 | 🟡 | Investigation tracking — per-file investigated status, persistence, auto-expiry | Users can mark files as reviewed, auto-clears on change | **Gap**: no unit tests |
| [L6](#layer-6) | [L6.7](#l67) | | | | | | | x | L6.6 | 🟢 | 🟢 | 🟡 | Dashboard Recon tab install prompt — "npm install aoa-recon" when not detected | Users know how to unlock Recon | **Gap**: browser-only validation |
| [L6](#layer-6) | [L6.8](#l68) | | | x | | | | | L6.2 | 🟢 | 🟢 | 🟡 | npm package structure — wrapper + platform packages + JS shims | Zero-friction install via npm | **Gap**: not yet published to npm |
| [L6](#layer-6) | [L6.9](#l69) | | | x | | | | | L6.4, L6.8 | 🟢 | 🟢 | 🟡 | npm recon package structure — wrapper + platform packages | Zero-friction recon install | **Gap**: not yet published to npm |
| [L6](#layer-6) | [L6.10](#l610) | | | x | | | | | L6.8, L6.9 | 🟡 | 🟢 | 🟡 | CI/Release — workflow builds both binaries, publishes to npm | Tag → build → publish, fully automated | **Gap**: workflow untested end-to-end |
| [L7](#layer-7) | [L7.1](#l71) | x | | | x | | | x | - | 🟢 | 🟢 | 🟡 | Startup progress feedback — deferred loading, async cache warming incl. recon | Daemon starts in <1s, caches warm in background | **Gap**: no automated startup time test |
| [L7](#layer-7) | [L7.2](#l72) | x | | | | x | | | - | 🟡 | ⚪ | ⚪ | Database storage optimization — replace JSON blobs with binary encoding | 964MB bbolt, 28.7s load. Target <3s | Profile load time; compare encoding formats |
| [L7](#layer-7) | [L7.3](#l73) | | | | | | | x | - | 🟡 | ⚪ | ⚪ | Recon source line editor view — file-level source display | All flagged lines in context, not one-at-a-time | Design conversation needed on layout |
| [L7](#layer-7) | [L7.4](#l74) | | | | x | x | | | - | 🟢 | ⚪ | ⚪ | .aoa/ directory restructure — subdirs for log/run/recon/hook, log rotation, delete dead dirs | Clean project state dir; logs don't grow forever | All paths resolve; daemon.log rotates; migration handles existing installs |

---

## Supporting Detail

### Layer 2

**Layer 2: Infrastructure Gaps**

> **Quality Gate**: ✅ All L2 tasks complete except L2.1 validation gap.

#### L2.1

**Wire file watcher** — 🟢 Complete, 🟡 Validation gap

`Rebuild()` on `SearchEngine`. `onFileChanged()` handles add/modify/delete. 5 unit tests (rebuild, new file, modify, delete, unsupported extension).

**Gap**: No integration test through fsnotify event pipeline — needs `TestDaemon_FileWatcher_ReindexOnEdit`.

**Files**: `internal/domain/index/search.go`, `internal/app/app.go`, `internal/app/watcher.go`, `internal/domain/index/rebuild_test.go`, `internal/app/watcher_test.go`

---

### Layer 3

#### L3.15

**GNU grep native parity** — 🟢 Complete, 🟡 Needs automated parity suite

Three-route architecture: file args → `grepFiles()`, stdin pipe → `grepStdin()`, neither → daemon index search → fallback `/usr/bin/grep`. 22 native flags covering 100% of observed AI agent usage.

**Gap**: Smoke tests pass but no automated parity test suite comparing output against `/usr/bin/grep`.

**Files**: `cmd/aoa/cmd/grep.go`, `egrep.go`, `tty.go`, `grep_native.go`, `grep_fallback.go`, `grep_exit.go`, `output.go`

---

### Layer 4

**Layer 4: Distribution**

#### L4.2

**Grammar CLI + Build CI** — 🟢 Code complete, 🟡 Download not implemented

`aoa grammar list/info/install/path` commands work. 57-grammar manifest embedded. CI workflow defined.

**Gap**: `aoa grammar install` doesn't download from GitHub Releases — shows what would be installed.

**Files**: `cmd/aoa/cmd/grammar.go`, `internal/adapters/treesitter/manifest.go`, `grammars/manifest.json`, `.github/workflows/build-grammars.yml`

#### L4.4

**Installation docs** — ⚪ Not started

Two install paths: `go install` (from source) vs binary download (lean + grammar packs). Post-install: `aoa init`, `aoa daemon start`.

**Files**: `README.md`

---

### Layer 5

**Layer 5: Dimensional Analysis (Bitmask engine, Recon tab)**

> Early warning system. 6 tiers, 22 dimensions. Security tier complete, performance and quality tiers not started.
>
> **Research**: [Bitmask analysis](../docs/research/bitmask-dimensional-analysis.md) | [AST vs LSP](../docs/research/asv-vs-lsp.md)

#### L5.7

**Performance tier** — ⚪ Not started

~50-60 questions: query patterns, memory, concurrency, resource leaks.

**Files**: `dimensions/performance/*.yaml`

#### L5.8

**Quality tier** — ⚪ Not started

~45-55 questions: complexity, error handling, dead code, conventions.

**Files**: `dimensions/quality/*.yaml`

#### L5.10

**Dimension scores in search results** — ⚪ Not started

Append `S:-23 P:0 Q:-4` to grep/egrep output.

**Files**: `internal/domain/index/format.go`

#### L5.11

**Dimension query support** — ⚪ Not started

`aoa grep --dimension=security --risk=high <query>` filters by tier and severity.

**Files**: `cmd/aoa/cmd/grep.go`

#### L5.12

**Recon tab** — 🟢 Complete (interim scanner), 🟡 Bitmask upgrade pending

Interim pattern scanner with 10 detectors + `long_function`. `GET /api/recon` returns folder→file→findings tree. Tier toggles, breadcrumb nav, code-only file filtering.

**Gap**: Full bitmask engine (L5.1–L5.5) not yet wired to dashboard. AST-based patterns, AC scanner, cross-language uniformity still on interim scanner.

**Files**: `internal/adapters/recon/scanner.go`, `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

#### L5.13

**Recon dashboard overhaul** — 🟢 Complete, 🟡 Browser-only validation

8 features delivered across Sessions 65–66: source line peek, tier noise gating, scan freshness, 5 focus modes, tier color redesign, code toggle with source cache, Copy Prompt, column alignment.

**Gap**: No automated tests — browser-only validation.

**Files**: `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`, `internal/adapters/web/static/style.css`

#### L5.14

**Recon cache + incremental updates** — 🟢 Complete, 🟡 No unit tests

Pre-compute scan at startup via `warmReconCache()`. Incremental updates via `SubtractFile`/`ScanFile`/`AddFile` on file watcher events. `CachedReconResult()` serves from memory (RWMutex-protected).

**Gap**: No unit tests for incremental path (SubtractFile/AddFile correctness).

**Files**: `internal/app/app.go`, `internal/adapters/recon/scanner.go`, `internal/app/watcher.go`

#### L5.15

**Investigation tracking** — 🟢 Complete, 🟡 No unit tests

Per-file investigated status with persistence (`.aoa/recon-investigated.json`), 1-week auto-expiry, auto-clear on file change. `POST /api/recon-investigate` endpoint. Dashboard investigated tier with solo mode.

**Gap**: No unit tests for investigation methods.

**Files**: `internal/app/app.go`, `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

---

### Layer 6

**Layer 6: Distribution v2**

> aoa (pure Go, 11 MB) + aoa-recon (CGo, 361 MB). npm distribution.

#### L6.7

**Dashboard Recon tab install prompt** — 🟢 Complete, 🟡 Browser-only

`recon_available` field in API. Install prompt with `npm install -g aoa-recon`. "Lite mode" indicator.

**Files**: `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

#### L6.8 + L6.9

**npm packages** — 🟢 Structure complete, 🟡 Not yet published

10 npm packages: 2 wrappers + 8 platform-specific. esbuild/turbo pattern with JS postinstall shim.

**Files**: `npm/aoa/`, `npm/aoa-recon/`, 8× `npm/aoa-{platform}/`

#### L6.10

**CI/Release** — 🟢 Workflow defined, 🟡 Untested end-to-end

Release workflow: 8 matrix jobs (2 binaries × 4 platforms), GitHub release, npm publish.

**Files**: `.github/workflows/release.yml`

---

### Layer 7

**Layer 7: Onboarding UX & Operational Polish**

> Progress feedback, project state hygiene, performance.

#### L7.1

**Startup progress feedback** — 🟢 Complete, 🟡 No automated timing test

Deferred all heavy IO to background after socket/HTTP are up. `WarmCaches()` runs index load → learner → file cache → recon scan with step-by-step logging. Daemon responds in 0.1s.

**Gap**: No automated startup time assertion.

**Files**: `internal/app/app.go`, `internal/domain/index/search.go`, `cmd/aoa/cmd/daemon.go`

#### L7.2

**Database storage optimization** — ⚪ Not started

`aoa.db` is 964MB. `LoadIndex()` deserializes JSON in ~28.7s.

Investigation: bbolt read vs JSON unmarshal, alternative encodings (gob/msgpack/protobuf), per-file buckets, compression. Target: <3s index load.

**Files**: `internal/adapters/bbolt/store.go`

#### L7.3

**Recon source line editor view** — ⚪ Not started

File-level source display with all flagged lines in context (editor-like, severity badges inline). Design conversation needed.

**Files**: `internal/adapters/web/static/app.js`, `style.css`, `internal/adapters/web/recon.go`

#### L7.4

**.aoa/ directory restructure** — ⚪ Not started

Reorganize `.aoa/` from flat dump to structured subdirectories:

```
.aoa/
  aoa.db              top-level (persistent database)
  status.json         top-level (hook reads this)
  log/
    daemon.log        rotated: >1MB → daemon.log.1 on startup
  run/
    daemon.pid        ephemeral, deleted on clean shutdown
    http.port         ephemeral, deleted on clean shutdown
  recon/
    investigated.json investigation markers
  hook/
    context.jsonl     hook-written context snapshots
    usage.txt         user-pasted /usage output
  grammars/           purego loader search path (empty by default)
```

Delete dead `domains/` directory. Add `ensureSubdirs()` at startup. Migration moves files from old flat paths on first run. ~6 Go files to update.

**Files**: `cmd/aoa/cmd/daemon.go`, `internal/app/app.go`, `internal/app/watcher.go`, `internal/adapters/web/server.go`

---

### What Works (Preserve)

| Component | Notes |
|-----------|-------|
| Search engine (O(1) inverted index) | 26/26 parity tests, 4 search modes, trigram content search (~60µs on 500 files), case-sensitive default (G1). |
| Learner (21-step autotune) | 5/5 fixture parity, float64 precision. Do not change decay/prune constants. |
| Session Prism (Claude JSONL reader) | Defensive parsing, UUID dedup, compound message decomposition. |
| Tree-sitter parser (509 languages) | go-sitter-forest, behind `ports.Parser` interface, lives in `aoa-recon` binary. |
| Two-binary distribution (L6) | `aoa` (pure Go, 11 MB) + `aoa-recon` (CGo, 361 MB). ReconBridge auto-discovery. |
| Socket protocol | JSON-over-socket IPC. Concurrent clients. `Reindex` with extended timeout. |
| Value engine (L0) | Burn rate, runway projection, session persistence, activity enrichments. |
| Activity rubric | Three-lane color system. Learned column. Autotune enrichments. |
| Dashboard (L1, 5-tab SPA) | 3-file split. Tab-aware polling. Soft glow animations. |
| Recon cache (L5.14) | Pre-computed at startup, incremental on file change. Zero per-poll cost. |
| Investigation tracking (L5.15) | Per-file status. Persisted. 1-week auto-expiry. Auto-cleared on change. |
| File watcher (L2) | `onFileChanged` → re-parse → `Rebuild` → `SaveIndex` → `updateReconForFile` → `clearFileInvestigated`. |

---

### What We're NOT Doing

| Item | Rationale |
|------|-----------|
| Neural 1-bit embeddings | Investigated, deprioritized. Deterministic AST+AC gives better signal with full interpretability. |
| WebSocket push (dashboard) | 2s poll is sufficient. Upgrade deferred — complexity not justified yet. |
| Multi-project simultaneous daemon | Single-project scope per daemon instance. Multi-project is a v3 concern. |
| LSP integration | AST is sufficient for early warning. LSP adds 100x cost for 20% more precision. See research. |

### Key Documents

| Document | Purpose |
|----------|---------|
| [COMPLETED.md](COMPLETED.md) | Archived phases 1-8c + all 🟢🟢🟢 tasks with validation notes |
| [Bitmask Analysis](../docs/research/bitmask-dimensional-analysis.md) | Security worked example, execution pipeline, cross-language uniformity |
| [AST vs LSP](../docs/research/asv-vs-lsp.md) | Viability assessment, per-dimension confidence ratings |
| [Sub-ms grep research](../docs/research/sub-ms-grep.md) | Trigram index approach, 5 alternatives evaluated |
| [CLAUDE.md](../CLAUDE.md) | Agent instructions, architecture reference, build commands |

### Quick Reference

| Resource | Location |
|----------|----------|
| Build (full) | `make build` — CGo, all grammars (~76 MB) |
| Build (pure) | `make build-pure` or `CGO_ENABLED=0 go build ./cmd/aoa/` — pure Go (~8 MB) |
| Build (recon) | `make build-recon` or `go build ./cmd/aoa-recon/` — CGo, tree-sitter (~73 MB) |
| Test | `go test ./...` |
| CI check | `make check` |
| Database | `{ProjectRoot}/.aoa/aoa.db` |
| Socket | `/tmp/aoa-{sha256(root)[:12]}.sock` |
| Dashboard | `http://localhost:{port}` (port in `.aoa/http.port`) |
| Session logs | `~/.claude/projects/{encoded-path}/*.jsonl` |
