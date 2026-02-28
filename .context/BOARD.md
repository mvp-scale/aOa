# Project Board

[Board](#board) | [Supporting Detail](#supporting-detail) | [Completed](COMPLETED.md) | [Backlog](BACKLOG.md)

> **Updated**: 2026-02-27 (Session 79) | **89% complete.**
> **Completed work**: See [COMPLETED.md](COMPLETED.md) -- Phases 1-8c + L0 + L1 + L2 (all) + L3 (all) + L4.1/L4.3 + L5.1-L5.6/L5.9/L5.19 + L6 (all) + L7.2 + L8.1 + L9 (all) + P0 (all 7 bugs) (535 tests, 0 fail, 0 skip)
> **Archived boards**: `.context/archived/`

---

## Goals

> Atomic architectural principles. Every task is evaluated against each goal independently.

| Goal | Statement |
|------|-----------|
| **G0** | **Speed** -- 50-120x faster than Python. Sub-ms search, <200ms startup, <50MB memory. No O(n) on hot paths. |
| **G1** | **Parity** -- Zero behavioral divergence from Python. Test fixtures are source of truth. |
| **G2** | **Single Binary, Dynamic Grammars** -- One `aoa` binary (core build: tree-sitter C runtime, zero compiled-in grammars). Grammars downloaded as .so/.dylib files via `aoa init` curl commands. No outbound network from the binary. Full transparency and user control. |
| **G3** | **Agent-First** -- Drop-in shim for grep/egrep/find. Three Unix modes: direct (`grep pat file`), pipe (`cmd | grep pat`), index (`grep pat` -> O(1) daemon). Same flags, same output format, same exit codes. Agents never know it's not GNU grep. |
| **G4** | **Clean Architecture** -- Hexagonal. Domain logic dependency-free. External concerns behind interfaces. No feature entanglement. |
| **G5** | **Self-Learning** -- Adaptive pattern recognition. observe(), autotune, competitive displacement. |
| **G6** | **Value Proof** -- Surface measurable savings. Context runway, tokens saved, sessions extended. |

---

## Board Structure

> Layered architecture. Each layer builds on the one below. TDD -- validation gates at every layer.

### Layers

| Layer | Name | Purpose | Gate Method |
|-------|------|---------|-------------|
| **L0** | Value Engine | Burn rate, context runway, attribution signals | Runway API returns valid projections; attribution rubric covers all tool actions |
| **L1** | Dashboard | 5-tab layout, mockup implementation, hero narratives | All 5 tabs render with live data; mockup parity validated in browser |
| **L2** | Infra Gaps | File watcher, bbolt lock, CLI flags, sub-ms content search | `aoa init` works while daemon runs; file changes trigger re-index; `aoa grep` <=1ms |
| **L3** | Migration | Parallel run Python vs Go, parity proof | 100 queries x 5 projects = zero divergence; benchmark confirms speedup |
| **L4** | Distribution | Goreleaser, grammar loader, install docs | `go install` or binary download works on linux/darwin x amd64/arm64 |
| **L5** | Dimensional Analysis | Bitmask engine, 6-tier scanning, rule expansion | Security tier catches known vulns in test projects; query time < 10ms |
| **L6** | Distribution v2 | Two-binary split, npm packaging, zero-friction install | `npm install -g aoa` works; `npm install -g aoa-recon` lights up Recon tab. **Superseded by L10 single-binary model.** |
| **L7** | Onboarding UX | First-run experience, progress feedback, project state hygiene | User sees meaningful progress during startup; `.aoa/` is clean and self-documenting |
| **L8** | Recon | Scanning dashboard, investigation tracking, source view | Recon tab shows findings; cache is instant; investigation tracks reviewed files |
| **L9** | Telemetry | Unified content metering, tool shadow counterfactual, burst throughput | Every content stream measured; counterfactual proves aOa savings on every tool call |
| **L10** | Dynamic Grammar Distribution | Core build + dynamic .so loading, `aoa init` as single entry point, grammar build pipeline | One binary, zero compiled-in grammars, user-controlled grammar download via curl |

### Columns

| Column | Purpose |
|--------|---------|
| **Layer** | Layer grouping. Links to layer detail below. |
| **ID** | Task identifier (layer.step). Links to task reference below. |
| **G0-G6** | Goal alignment. `x` = serves this goal. Blank = not relevant. |
| **Dep** | ID of blocking task, or `-` |
| **Cf** | Confidence -- see indicator reference below |
| **St** | Status -- see indicator reference below |
| **Va** | Validation state -- see indicator reference below |
| **Task** | What we're doing |
| **Value** | Why we're doing this |
| **Va Detail** | How we prove it |

### Indicator Reference

| Indicator | Cf (Confidence) | St (Status) | Va (Validation) |
|:---------:|:----------------|:------------|:----------------|
| white | -- | Not started | Not yet validated |
| blue | -- | In progress | -- |
| green | Confident | Complete | Automated test proves it (unit or integration) |
| yellow | Uncertain | Pending | Partial -- manual/browser only, or unit but no integration. See Va Detail for gap. |
| red | Lost/Blocked | Blocked | Failed |

> Triple-green = done. Task moves to COMPLETED.md.

---

## Mission

**North Star**: One binary that makes every AI agent faster by replacing slow, expensive tool calls with O(1) indexed search -- and proves it with measurable savings.

**Current**: Core engine complete (search, learner, dashboard, grep parity). Recon scanning integrated into single binary with dynamic grammar loading (core build: tree-sitter C runtime, zero compiled-in grammars). `aoa init` is the single entry point -- scans project, detects languages, prints curl commands for grammar .so files. Grammar build pipeline (`scripts/build-grammars.sh`) compiles from go-sitter-forest C source. Dimensional engine with 5 active tiers (security, performance, quality, architecture, observability), 136 YAML rules across 21 dimensions. Commands renamed: `wipe` -> `reset`/`remove`.

**Approach**: TDD. Each layer validated before the next. Completed work archived to keep the board focused on what's next.

---

## Board

### Active Board

| Layer | ID | G0 | G1 | G2 | G3 | G4 | G5 | G6 | Dep | Cf | St | Va | Task | Value | Va Detail |
|:------|:---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:----|:--:|:--:|:--:|:-----|:------|:----------|
| **L0** | | | | | | | | | | | | | *All 12 tasks complete -- see COMPLETED.md* | | |
| **L1** | | | | | | | | | | | | | *All 8 tasks complete -- see COMPLETED.md* | | |
| **L2** | | | | | | | | | | | | | *All tasks complete -- see COMPLETED.md* | | |
| **L3** | | | | | | | | | | | | | *All tasks complete -- see COMPLETED.md* | | |
| [L4](#layer-4) | [L4.4](#l44) | x | | x | x | | | | L4.3 | 🟢 | 🟡 | ⚪ | Installation docs + grammar pipeline -- npm install, aoa init, grammar compilation from go-sitter-forest with provenance | Friction-free onboarding, user delight | Build all 510, GH Actions provenance, install guide |
| [L5](#layer-5) | [L5.Va](#l5va) | | | | | | | x | L5.1 | 🟢 | 🟢 | 🟡 | Dimensional rule validation -- per-rule detection tests across all 5 tiers (security 37, perf 26, quality 24, arch, obs). Absorbs L5.7/8/16/17/18 + L8.1 bitmask upgrade | All rules detect what they claim; engine wired to dashboard | Rules load + parse. **Gap**: per-rule detection accuracy untested |
| [L5](#layer-5) | [L5.10](#l510) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Add dimension scores to search results (`S:-1 P:0 C:+2`) | Scores visible inline | Scores appear in grep/egrep output |
| [L5](#layer-5) | [L5.11](#l511) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Dimension query support -- `--dimension=security --risk=high` | Filter by dimension | CLI filters by tier and severity |
| **L6** | | | | | | | | | | | | | *All 9 tasks complete -- see COMPLETED.md* | | |
| [L7](#layer-7) | [L7.1](#l71) | x | | | x | | | x | - | 🟢 | 🟢 | 🟡 | Startup progress feedback -- deferred loading, async cache warming incl. recon | Daemon starts in <1s, caches warm in background | **Gap**: no automated startup time test |
| [L7](#layer-7) | [L7.4](#l74) | | | | x | x | | | - | 🟢 | 🟢 | 🟢 | .aoa/ directory restructure -- `Paths` struct (18 fields), `EnsureDirs`, `Migrate` (7 files), 1MB log rotation, 13 files updated | Clean project state dir; logs don't grow forever | 7 unit tests, live migration verified on 1.3GB database, all builds clean |
| G0 | G0.HF1 | x | | | | | | | - | 🟢 | 🟢 | 🟢 | **Build process fix (hotfix)** -- `build.sh` as sole entry point, compile-time build guard, flipped build tags (`recon` opt-in), Makefile rewrite, CLAUDE.md Build Rule. Binary 366MB->8MB. | Standard binary is pure Go, no CGo bleed | `build.sh` enforced, `go build` panics with guard, 8MB binary verified |
| [L8](#layer-8) | [L8.2](#l82) | | | | | | | x | L5.Va | 🟢 | 🟢 | 🟡 | Recon dashboard overhaul -- 5 focus modes, tier redesign, code toggle, copy prompt | Recon tab is actionable, not just a finding list | **Gap**: browser-only validation |
| [L8](#layer-8) | [L8.3](#l83) | x | | | | x | | | L5.Va | 🟢 | 🟢 | 🟡 | Recon cache + incremental updates -- pre-compute at startup, SubtractFile/AddFile on file change | Zero per-poll scan cost, instant API response | **Gap**: no unit tests for incremental path |
| [L8](#layer-8) | [L8.4](#l84) | | | | | | | x | L8.3 | 🟢 | 🟢 | 🟡 | Investigation tracking -- per-file investigated status, persistence, auto-expiry | Users can mark files as reviewed, auto-clears on change | **Gap**: no unit tests |
| [L8](#layer-8) | [L8.5](#l85) | | | | | | | x | L6.6 | 🟢 | 🟢 | 🟡 | Dashboard Recon tab install prompt -- "Run aoa init" when grammars missing | Users know how to get grammars | Updated from "npm install aoa-recon" to "Run aoa init". **Gap**: browser-only validation |
| [L8](#layer-8) | [L8.6](#l86) | | | | | | | x | - | 🟡 | ⚪ | ⚪ | Recon source line editor view -- file-level source display | All flagged lines in context, not one-at-a-time | Design conversation needed on layout |
| [L10](#layer-10) | [L10.1](#l101) | | | x | | x | | | - | 🟢 | 🟢 | 🟡 | Core build tier -- `./build.sh --core`: tree-sitter C runtime, zero compiled-in grammars. `languages_core.go` (empty registerBuiltinLanguages), build tags exclude `core` from recon/forest | Single binary with dynamic grammar loading | Core build compiles and runs. **Gap**: no automated test of core-only build path |
| [L10](#layer-10) | [L10.2](#l102) | | | x | | x | | | - | 🟢 | 🟢 | 🟡 | Grammar paths wired into parser -- `newParser(root)` configures `DefaultGrammarPaths` for dynamic .so loading. All callers updated (init.go, daemon.go) | Grammars load from `.aoa/grammars/` at runtime | Parser loads .so files. **Gap**: no automated test of dynamic loading |
| [L10](#layer-10) | [L10.3](#l103) | | | x | | | | | - | 🟢 | 🟢 | 🟡 | No outbound network -- removed Go HTTP downloader entirely. `aoa init` detects missing grammars and prints curl commands. Zero outbound connections | Full transparency, user controls all downloads | Curl commands print correctly. **Gap**: no automated test |
| [L10](#layer-10) | [L10.4](#l104) | | | x | | | | | - | 🟢 | 🟢 | 🟡 | Grammar build script -- `scripts/build-grammars.sh` compiles .so/.dylib from go-sitter-forest C source. Core pack = 11 grammars, 11 MB. Individual grammars 20 KB (json) to 3.5 MB (cpp) | Grammars built from source, reproducible | Script tested locally. **Gap**: cross-platform CI not yet wired |
| [L10](#layer-10) | [L10.5](#l105) | | | x | | | | | - | 🟢 | 🟢 | 🟢 | `aoa init` as single command -- scans project, detects languages, shows curl for missing grammars, indexes with available. Removed `aoa-recon` dependency. TSX fix (own .so file) | One command to set up any project | Init runs end-to-end, no aoa-recon needed |
| [L10](#layer-10) | [L10.6](#l106) | | | x | | x | | | - | 🟢 | 🟢 | 🟢 | Command rename: `wipe` -> `reset` + `remove`. `aoa reset` clears data, `aoa remove` stops daemon + deletes .aoa/. `wipe` kept as hidden alias | Clearer UX, destructive actions separated | Commands registered and working |
| [L10](#layer-10) | [L10.7](#l107) | | | x | | | | | - | 🟢 | 🟢 | 🟡 | deploy.sh updated -- uses `./build.sh --core` instead of lean build | Deploy produces correct binary | **Gap**: not tested on fresh machine |
| [L10](#layer-10) | [L10.8](#l108) | | | x | | | | | L10.4 | 🟢 | ⚪ | ⚪ | Build all 509 grammars + GitHub release `grammars-v1` with .so for all 4 platforms | Pre-built grammars available for download | CI builds cross-platform, release assets downloadable |
| [L10](#layer-10) | [L10.9](#l109) | | | x | | | | | L10.8 | 🟢 | ⚪ | ⚪ | End-to-end test on fresh project -- `aoa init` -> curl grammars -> scan -> dashboard | Full flow works from zero state | Fresh project scans successfully with downloaded grammars |

---

## Supporting Detail

### Layer 2

**Layer 2: Infrastructure Gaps**

> **Quality Gate**: All L2 tasks complete. L2.1 validated 2026-02-25.

#### L2.1

**Wire file watcher** -- green Complete, green Validated

`Rebuild()` on `SearchEngine`. `onFileChanged()` handles add/modify/delete. 13 tests: 4 in app (new/modify/delete/unsupported) + 6 in adapter (detect change/new/delete, ignore non-code, reindex latency, stop cleanup) + 3 integration (new file auto-reindex, modify auto-reindex, delete auto-reindex). Validated 2026-02-25.

Integration tests (`test/integration/cli_test.go`): `TestFileWatcher_NewFile_AutoReindex`, `TestFileWatcher_ModifyFile_AutoReindex`, `TestFileWatcher_DeleteFile_AutoReindex`. Full daemon → fsnotify → parse → index → search pipeline. Poll-based assertions (up to 3s) avoid flaky timing.

**Files**: `internal/domain/index/search.go`, `internal/app/app.go`, `internal/app/watcher.go`, `internal/domain/index/rebuild_test.go`, `internal/app/watcher_test.go`

---

### Layer 4

**Layer 4: Distribution**

#### L4.4

**Installation docs + grammar pipeline** -- In progress (Session 79)

Single install path: `npm install -g @mvpscale/aoa` (lightweight, binary only). Grammar `.so` files are pre-compiled from tree-sitter source via GitHub Actions with SLSA provenance. `aoa init` detects project languages and installs only the grammars needed.

**Grammar source**: [alexaandru/go-sitter-forest](https://github.com/alexaandru/go-sitter-forest) — aggregates 510 tree-sitter grammars from upstream repos. Each grammar has `parser.c` + optional `scanner.c`. Compiled with `gcc -shared -fPIC -O2`. Maintainer attribution displayed per grammar.

**User flow**:
1. `npm install -g @mvpscale/aoa` — installs binary
2. `aoa init` — detects languages, compiles/fetches missing grammars, indexes project
3. `aoa grammar list` — shows all available grammars with maintainer + source repo
4. `aoa grammar install <lang|pack|all>` — install specific grammars

**Remaining work**:
- [ ] Build all 510 grammars locally, verify every one compiles and loads
- [ ] GitHub Actions workflow to compile all grammars per platform with provenance
- [ ] Update manifest with all 510 grammars (maintainer, upstream repo)
- [ ] `aoa init` flow: detect → install → index, with progress + attribution
- [ ] `aoa grammar list` shows maintainer and source for each grammar
- [ ] User config file (`.aoa/languages`) for manual grammar selection
- [ ] Installation guide document
- [ ] npm package updated (binary only, lightweight)

**Files**: `cmd/aoa/cmd/init_grammars_cgo.go`, `cmd/aoa/cmd/grammar_cgo.go`, `internal/adapters/treesitter/manifest.go`, `.github/workflows/grammars.yml`, `docs/INSTALL.md`

---

### Layer 5

**Layer 5: Dimensional Analysis (Bitmask engine, rule expansion)**

> Early warning system. 5 active tiers, 21 dimensions. Dimensional engine with 136 YAML rules across all tiers. **YAML rework complete**: Universal concept layer (15 concepts, 509 languages), declarative `structural:` blocks, `lang_map` eliminated from all rules. Performance tier (26 rules, 5 dims) and Quality tier (24 rules, 4 dims) fully populated. Compliance tier pivoted -- concepts absorbed into security. Revalidated 2026-02-25.
>
> **ADR**: [Declarative YAML Rules](../decisions/2026-02-23-declarative-yaml-rules.md) -- spec (rework done)
> **Research**: [Bitmask analysis](../docs/research/bitmask-dimensional-analysis.md) | [AST vs LSP](../docs/research/asv-vs-lsp.md) | [Dimensional taxonomy](details/2026-02-23-dimensional-taxonomy.md)

#### L5.Va

**Dimensional rule validation** -- green Engine + rules complete, yellow Per-rule detection accuracy untested

Consolidated from L5.7 (performance), L5.8 (quality), L5.16 (security), L5.17 (architecture), L5.18 (observability), and L8.1 (bitmask dashboard wiring). L5.19 (compliance) superseded — absorbed into security tier.

All tiers have YAML rework complete (declarative `structural:` blocks, universal concept layer, no `lang_map`). Rules load, parse, and wire to dashboard. What's missing: tests proving each rule detects what it claims on real code samples.

**Tiers** (all rules authored, all wired to dashboard):
- **Security** (37 rules, 5 dims): `security.yaml` + 6 hardcoded fallbacks
- **Performance** (26 rules, 5 dims): resources, concurrency, query, memory, hot_path
- **Quality** (24 rules, 4 dims): errors, complexity, dead_code, conventions
- **Architecture** (3 dims): antipatterns, imports, api_surface
- **Observability** (2 dims): debug, silent_failures

**What validation needs**: For each rule, a test fixture (synthetic code snippet) + assertion that the rule fires on the snippet and does NOT fire on clean code. Can be a single test file per tier with table-driven subtests.

**Files**: `recon/rules/*.yaml`, `internal/domain/analyzer/`, `internal/adapters/treesitter/walker.go`, `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

#### L5.10

**Dimension scores in search results** -- Not started

Append `S:-23 P:0 Q:-4` to grep/egrep output.

**Files**: `internal/domain/index/format.go`

#### L5.11

**Dimension query support** -- Not started

`aoa grep --dimension=security --risk=high <query>` filters by tier and severity.

**Files**: `cmd/aoa/cmd/grep.go`

---

### Layer 7

**Layer 7: Onboarding UX & Operational Polish**

> Progress feedback, project state hygiene, performance.

#### L7.1

**Startup progress feedback** -- green Complete, yellow No automated timing test

Deferred all heavy IO to background after socket/HTTP are up. `WarmCaches()` runs index load -> learner -> file cache -> recon scan with step-by-step logging. Daemon responds in 0.1s.

**Gap**: No automated startup time assertion.

**Files**: `internal/app/app.go`, `internal/domain/index/search.go`, `cmd/aoa/cmd/daemon.go`

#### L7.2

**Database storage optimization** -- Complete (Session 72)

Replaced JSON serialization with binary posting lists + gob for the bbolt search index. Format versioning (`_version` key): v0=JSON (legacy), v1=binary/gob. `SaveIndex` always writes v1. `LoadIndex` detects version and branches. Lazy migration -- first load reads v0, next save writes v1, all subsequent loads use fast binary path.

**Results**: 50K tokens / 500K refs encodes to 3.7 MB binary (vs ~75 MB JSON = ~20x smaller). Parallel decode preserved. All 25 bbolt tests pass.

**Key change**: TokenRefs encoded as little-endian uint32(FileID) + uint16(Line) = 6 bytes vs `{"FileID":1234,"Line":56}` = 25 bytes JSON.

**Files**: `internal/adapters/bbolt/encoding.go` (new -- binary codec), `internal/adapters/bbolt/store.go` (format versioning + migration), `internal/adapters/bbolt/store_test.go` (5 new tests + 4 benchmarks)

#### L7.4

**.aoa/ directory restructure** -- COMPLETE (Session 73)

`Paths` struct with 18 pre-computed path fields, `NewPaths()` constructor, `EnsureDirs()` (creates 7 subdirs), `Migrate()` (moves 7 files from flat layout, removes dead `domains/`), `CleanEphemeral()`. Replaced 25+ scattered `filepath.Join` calls across 13 files. Line-based `trimDaemonLog` replaced with 1MB size-based rotation (`os.Rename` to `.1`). Hook script updated for `hook/` subdirectory.

```
.aoa/
  aoa.db              persistent database (top-level)
  status.json         hook reads this (top-level)
  log/daemon.log      rotated: >1MB -> daemon.log.1
  run/daemon.pid      ephemeral
  run/http.port       ephemeral
  recon/enabled       marker file
  recon/investigated.json
  hook/context.jsonl  hook-written context snapshots
  hook/usage.txt      pasted /usage output
  bin/                recon binary fallback
  grammars/           tree-sitter grammar .so files
```

**Validation**: 7 unit tests (NewPaths, EnsureDirs, Migrate_FreshInstall, Migrate_OldLayout, Migrate_Idempotent, Migrate_NoOverwrite, Migrate_DomainsCleanup). Live migration verified on 1.3GB database. All builds and vet clean.

**Files**: `internal/app/paths.go` (new), `internal/app/paths_test.go` (new), `internal/app/app.go`, `internal/app/watcher.go`, `internal/app/recon_bridge.go`, `internal/app/watcher_test.go`, `internal/app/activity_test.go`, `cmd/aoa/cmd/daemon.go`, `cmd/aoa/cmd/init.go`, `cmd/aoa/cmd/config.go`, `cmd/aoa/cmd/wipe.go`, `cmd/aoa/cmd/recon.go`, `cmd/aoa/cmd/open.go`, `cmd/aoa/cmd/grammar_cgo.go`, `hooks/aoa-status-line.sh`, `test/integration/cli_test.go`

---

### Layer 8

**Layer 8: Recon**

> Scanning dashboard, investigation tracking, source view. Powered by the dimensional engine (L5).

#### L8.2

**Recon dashboard overhaul** -- green Complete, yellow Browser-only validation

8 features delivered across Sessions 65-66: source line peek, tier noise gating, scan freshness, 5 focus modes, tier color redesign, code toggle with source cache, Copy Prompt, column alignment. Session 78: fixed `investigated_files` missing from `/api/recon` response (both dimensional results path and empty-data path).

**Gap**: No automated tests -- browser-only validation.

**Files**: `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`, `internal/adapters/web/static/style.css`

#### L8.3

**Recon cache + incremental updates** -- green Complete, yellow No unit tests

Old warmReconCache/updateReconForFile deleted in Session 71 (clean separation). Recon now gated behind `aoa recon init` + `.aoa/recon.enabled` marker. When enabled, ReconBridge handles scanning via separate aoa-recon binary.

**Gap**: No unit tests for incremental path. Session 78: `build.sh --recon-bin` fixed (was missing `-tags "recon"`, failing to compile `recon.NewEngine`). `aoa-recon` binary now builds and connects successfully.

**Files**: `internal/app/recon_bridge.go`, `internal/adapters/web/recon.go`, `internal/app/watcher.go`

#### L8.4

**Investigation tracking** -- green Complete, yellow No unit tests

Per-file investigated status with persistence (`.aoa/recon-investigated.json`), 1-week auto-expiry, auto-clear on file change. `POST /api/recon-investigate` endpoint. Dashboard investigated tier with solo mode.

**Gap**: No unit tests for investigation methods.

**Files**: `internal/app/app.go`, `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

#### L8.5

**Dashboard Recon tab install prompt** -- green Complete, yellow Browser-only

Updated from "npm install aoa-recon" to "Run aoa init" prompt. Condition fixed to show prompt whenever no data exists (regardless of recon_available flag).

**Files**: `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

#### L8.6

**Recon source line editor view** -- Not started

File-level source display with all flagged lines in context (editor-like, severity badges inline). Design conversation needed.

**Files**: `internal/adapters/web/static/app.js`, `style.css`, `internal/adapters/web/recon.go`

---

### Layer 10

**Layer 10: Dynamic Grammar Distribution**

> Single-binary architecture. Core build includes tree-sitter C runtime but zero compiled-in grammars. Grammars downloaded as platform-specific .so/.dylib files. `aoa init` is the sole entry point. No outbound network from the binary -- grammar downloads are user-executed curl commands. Supersedes the L6 two-binary npm model.

#### L10.1

**Core build tier** -- green Complete, yellow No automated test

`./build.sh --core` mode: compiles tree-sitter C runtime into the binary but includes zero grammars. `languages_core.go` provides empty `registerBuiltinLanguages()`. Build tags: `languages_forest.go` and `build_guard.go` exclude `core`. Three build tiers: standard (pure Go, no parser), core (C runtime + dynamic loading), recon (all 509 grammars compiled in).

**Files**: `build.sh`, `cmd/aoa/build_guard.go`, `internal/adapters/treesitter/languages_core.go` (new), `internal/adapters/treesitter/languages_forest.go`

#### L10.2

**Grammar paths wired into parser** -- green Complete, yellow No automated test

`newParser(root string)` now configures `DefaultGrammarPaths` pointing to `.aoa/grammars/` for dynamic .so loading at runtime. All callers updated (init.go, daemon.go).

**Files**: `cmd/aoa/cmd/parser_cgo.go`, `cmd/aoa/cmd/parser_nocgo.go`, `cmd/aoa/cmd/init.go`, `cmd/aoa/cmd/daemon.go`

#### L10.3

**No outbound network** -- green Complete, yellow No automated test

Removed the Go HTTP downloader entirely. `aoa init` detects missing grammars and prints `curl` commands for the user to run. Zero outbound connections from the binary. Full transparency and user control.

**Files**: `internal/adapters/treesitter/downloader.go` (rewritten: PlatformString + GlobalGrammarDir only), `internal/adapters/treesitter/downloader_test.go`, `internal/adapters/treesitter/manifest.go`

#### L10.4

**Grammar build script** -- green Complete, yellow Cross-platform CI not wired

`scripts/build-grammars.sh` compiles .so/.dylib from `alexaandru/go-sitter-forest` C source found in the Go module cache. Tested locally: core pack = 11 grammars, 11 MB total. Individual grammars range from 20 KB (json) to 3.5 MB (cpp).

**Files**: `scripts/build-grammars.sh` (new), `scripts/gen-manifest-hashes.go` (new)

#### L10.5

**`aoa init` as single command** -- COMPLETE

Scans project, detects languages via `ExtensionToLanguage`, shows curl commands for missing grammars, indexes with available grammars. Removed `aoa-recon` dependency entirely -- no more "run aoa recon init" tip. TSX fix: removed `soFileOverrides["tsx"] = "typescript"` -- each grammar gets its own .so file. Added `--no-grammars` flag to skip grammar detection.

**Files**: `cmd/aoa/cmd/init.go`, `cmd/aoa/cmd/init_grammars_cgo.go` (new), `cmd/aoa/cmd/init_grammars_nocgo.go` (new), `internal/adapters/treesitter/extensions.go`, `internal/adapters/treesitter/loader.go`, `internal/adapters/treesitter/loader_test.go`

#### L10.6

**Command rename: wipe -> reset + remove** -- COMPLETE

`aoa reset` clears data (what wipe used to do). `aoa remove` stops daemon and deletes `.aoa/` entirely. `aoa wipe` kept as hidden alias for backward compatibility.

**Files**: `cmd/aoa/cmd/reset.go` (new), `cmd/aoa/cmd/remove.go` (new), `cmd/aoa/cmd/wipe.go` (hidden alias), `cmd/aoa/cmd/root.go`

#### L10.7

**deploy.sh updated** -- green Complete, yellow Not tested on fresh machine

Now uses `./build.sh --core` instead of lean build.

**Files**: `deploy.sh`

#### L10.8

**Build all 509 grammars + GitHub release** -- Not started

Run `scripts/build-grammars.sh --all` to compile all grammars. Create GitHub release `grammars-v1` with .so files for linux/darwin x amd64/arm64. Wire CI workflow for cross-platform builds.

#### L10.9

**End-to-end test on fresh project** -- Not started

Full flow: `aoa init` on a fresh project -> curl grammars -> scan -> dashboard shows results. Validates the entire dynamic grammar pipeline from zero state.

---

---

### What Works (Preserve)

| Component | Notes |
|-----------|-------|
| Search engine (O(1) inverted index) | 26/26 parity tests, 4 search modes, trigram content search (~60us on 500 files), case-sensitive default (G1). **G0 perf**: regex trigram extraction (5s->8ms), symbol search gated on metadata (186ms->4us). All ops sub-25ms. **G0 gauntlet**: 22-shape perf regression suite (`test/gauntlet_test.go`) -- ceiling test in `go test ./...`, benchstat baselines via `make bench-gauntlet/bench-baseline/bench-compare`. Covers every Search() code path including regex trigram, lean-mode guard, brute-force, glob, context, count, quiet, only-matching. |
| Learner (21-step autotune) | 5/5 fixture parity, float64 precision. Do not change decay/prune constants. |
| Session Prism (Claude JSONL reader) | Defensive parsing, UUID dedup, compound message decomposition. |
| Tree-sitter parser (509 languages) | go-sitter-forest, behind `ports.Parser` interface. Core build: C runtime compiled in, grammars loaded dynamically from `.aoa/grammars/*.so`. |
| Single-binary distribution (L10) | One `aoa` binary (core build with tree-sitter C runtime). Grammars downloaded as .so/.dylib via `aoa init` curl commands. No outbound network from binary. |
| Socket protocol | JSON-over-socket IPC. Concurrent clients. `Reindex` with extended timeout. |
| Value engine (L0) | Burn rate, runway projection, session persistence, activity enrichments. |
| Activity rubric | Three-lane color system. Learned column. Autotune enrichments. |
| Dashboard (L1, 5-tab SPA) | 3-file split. Tab-aware polling. Soft glow animations. |
| GNU grep parity (L3.15) | 135 parity tests (77 internal + 58 Unix). 3-route architecture. 22 native flags. |
| npm distribution (L6.8-L6.10) | 10 npm packages, CI/release pipeline, 5 successful releases (v0.1.3-v0.1.7). **Superseded by L10 single-binary model.** |
| Recon cache (L8.3) | Pre-computed at startup, incremental on file change. Zero per-poll cost. |
| Investigation tracking (L8.4) | Per-file status. Persisted. 1-week auto-expiry. Auto-cleared on change. |
| ContentMeter (L9.1) | Unified char accumulator, 50-turn ring buffer, 8 unit tests. |
| Shadow engine (L9.5) | ToolShadow + 100-entry ShadowRing, async Grep/Glob dispatch, 6 unit tests. |
| Telemetry pipeline (L9) | Full pipeline: content metering, tool details, persisted results, subagent tailing, shadow counterfactual, burst throughput, dashboard display. |
| File watcher (L2) | `onFileChanged` -> re-parse -> `Rebuild` -> `SaveIndex` -> `clearFileInvestigated`. Recon gated behind `.aoa/recon.enabled`. |

---

### What We're NOT Doing

| Item | Rationale |
|------|-----------|
| Neural 1-bit embeddings | Investigated, deprioritized. Deterministic AST+AC gives better signal with full interpretability. |
| WebSocket push (dashboard) | 2s poll is sufficient. Upgrade deferred -- complexity not justified yet. |
| Multi-project simultaneous daemon | Single-project scope per daemon instance. Multi-project is a v3 concern. |
| LSP integration | AST is sufficient for early warning. LSP adds 100x cost for 20% more precision. See research. |

### Key Documents

| Document | Purpose |
|----------|---------|
| [COMPLETED.md](COMPLETED.md) | Archived phases 1-8c + all triple-green tasks with validation notes |
| [Declarative YAML Rules ADR](decisions/2026-02-23-declarative-yaml-rules.md) | Spec for dimensional rules rework -- schema, constraints, Go types |
| [Dimensional Taxonomy](details/2026-02-23-dimensional-taxonomy.md) | 142 questions across 21 dimensions, 6 tiers |
| [Throughput Telemetry Model](details/2026-02-26-throughput-telemetry-model.md) | Data hierarchy, calculations, ContentMeter spec, shadow pattern |
| [Bitmask Analysis](../docs/research/bitmask-dimensional-analysis.md) | Security worked example, execution pipeline, cross-language uniformity |
| [AST vs LSP](../docs/research/asv-vs-lsp.md) | Viability assessment, per-dimension confidence ratings |
| [Sub-ms grep research](../docs/research/sub-ms-grep.md) | Trigram index approach, 5 alternatives evaluated |
| [CLAUDE.md](../CLAUDE.md) | Agent instructions, architecture reference, build commands |

### Quick Reference

| Resource | Location |
|----------|----------|
| Build (standard) | `./build.sh` -- pure Go, no CGo (~8 MB) |
| Build (core) | `./build.sh --core` -- tree-sitter C runtime, zero compiled-in grammars, dynamic .so loading |
| Build (recon) | `./build.sh --recon` -- CGo, all 509 grammars compiled in (~361 MB) |
| Test | `go test ./...` |
| CI check | `make check` |
| Database | `{ProjectRoot}/.aoa/aoa.db` |
| Socket | `/tmp/aoa-{sha256(root)[:12]}.sock` |
| Dashboard | `http://localhost:{port}` (port in `.aoa/http.port`) |
| Session logs | `~/.claude/projects/{encoded-path}/*.jsonl` |
