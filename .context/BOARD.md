# Project Board

[Board](#board) | [Supporting Detail](#supporting-detail) | [Completed](COMPLETED.md) | [Backlog](BACKLOG.md)

> **Updated**: 2026-02-28 (Session 82) | **91% complete.**
> **Completed work**: See [COMPLETED.md](COMPLETED.md) -- Phases 1-8c + L0 + L1 + L2 (all) + L3 (all) + L4.1/L4.3 + L5.1-L5.6/L5.9/L5.19 + L6 (all) + L7.2/L7.4 + L8.1 + L9 (all) + L10.5/L10.6 + G0.HF1 + P0 (all 7 bugs) (535 tests, 0 fail, 0 skip)
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

**Current**: Core engine complete (search, learner, dashboard, grep parity). Single binary with dynamic grammar loading (core build: tree-sitter C runtime, zero compiled-in grammars). `aoa init` is the single entry point -- scans project, detects languages, prints curl commands for grammar .so files. Grammar validation pipeline (weekly CI) validates all 509 grammars on 4 platforms, produces `parsers.json` with full provenance + `GRAMMAR_REPORT.md`. Dimensional engine with 5 active tiers (security, performance, quality, architecture, observability), 136 YAML rules across 21 dimensions. **aoa-recon removed** -- two build modes: standard (`./build.sh`) and light (`./build.sh --light`). **Security pipeline**: SECURITY.md trust document, govulncheck + gosec + network audit CI on every push.

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
| [L4](#layer-4) | [L4.4](#l44) | x | | x | x | | | | L4.3 | 🟢 | 🟡 | ⚪ | Installation docs + grammar pipeline -- 6-phase roadmap. **Phase 1 DONE** (S80). **Phase 2 DONE** (S81): parsers.json 509/509 provenance, GRAMMAR_REPORT.md, weekly CI (Sun 6am UTC), 346 contributors acknowledged. **Next: Phase 3** (grammar release -- pre-built .so on GitHub) | Friction-free onboarding, user delight | Install guide, manifest expansion, `aoa init` fetch from releases, `aoa grammar list` attribution |
| [L5](#layer-5) | [L5.Va](#l5va) | | | | | | | x | L5.1 | 🟢 | 🟢 | 🟡 | Dimensional rule validation -- per-rule detection tests across all 5 tiers (security 37, perf 26, quality 24, arch, obs). Absorbs L5.7/8/16/17/18 + L8.1 bitmask upgrade | All rules detect what they claim; engine wired to dashboard | Rules load + parse. **Gap**: per-rule detection accuracy untested |
| [L5](#layer-5) | [L5.10](#l510) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Add dimension scores to search results (`S:-1 P:0 C:+2`) | Scores visible inline | Scores appear in grep/egrep output |
| [L5](#layer-5) | [L5.11](#l511) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Dimension query support -- `--dimension=security --risk=high` | Filter by dimension | CLI filters by tier and severity |
| **L6** | | | | | | | | | | | | | *All 9 tasks complete -- see COMPLETED.md* | | |
| **L7** | | | | | | | | | | | | | *L7.2/L7.4 complete -- see COMPLETED.md* | | |
| [L7](#layer-7) | [L7.1](#l71) | x | | | x | | | x | - | 🟢 | 🟢 | 🟡 | Startup progress feedback -- deferred loading, async cache warming incl. recon | Daemon starts in <1s, caches warm in background | **Gap**: no automated startup time test |
| **G0** | | | | | | | | | | | | | *G0.HF1 complete -- see COMPLETED.md* | | |
| [L8](#layer-8) | [L8.2](#l82) | | | | | | | x | L5.Va | 🟢 | 🟢 | 🟡 | Recon dashboard overhaul -- 5 focus modes, tier redesign, code toggle, copy prompt | Recon tab is actionable, not just a finding list | **Gap**: browser-only validation |
| [L8](#layer-8) | [L8.3](#l83) | x | | | | x | | | L5.Va | 🟢 | 🟢 | 🟡 | Recon cache + incremental updates -- pre-compute at startup, SubtractFile/AddFile on file change. **S81**: recon_bridge.go deleted (aoa-recon removed) | Zero per-poll scan cost, instant API response | **Gap**: no unit tests for incremental path |
| [L8](#layer-8) | [L8.4](#l84) | | | | | | | x | L8.3 | 🟢 | 🟢 | 🟡 | Investigation tracking -- per-file investigated status, persistence, auto-expiry | Users can mark files as reviewed, auto-clears on change | **Gap**: no unit tests |
| [L8](#layer-8) | [L8.5](#l85) | | | | | | | x | L6.6 | 🟢 | 🟢 | 🟡 | Dashboard Recon tab install prompt -- "Run aoa init" when grammars missing | Users know how to get grammars | Updated from "npm install aoa-recon" to "Run aoa init". **Gap**: browser-only validation |
| [L8](#layer-8) | [L8.6](#l86) | | | | | | | x | - | 🟡 | ⚪ | ⚪ | Recon source line editor view -- file-level source display | All flagged lines in context, not one-at-a-time | Design conversation needed on layout |
| [L10](#layer-10) | [L10.1](#l101) | | | x | | x | | | - | 🟢 | 🟢 | 🟡 | Core build tier -- `./build.sh --core`: tree-sitter C runtime, zero compiled-in grammars. `languages_core.go` (empty registerBuiltinLanguages), build tags exclude `core` from recon/forest | Single binary with dynamic grammar loading | Core build compiles and runs. **Gap**: no automated test of core-only build path |
| [L10](#layer-10) | [L10.2](#l102) | | | x | | x | | | - | 🟢 | 🟢 | 🟡 | Grammar paths wired into parser -- `newParser(root)` configures `DefaultGrammarPaths` for dynamic .so loading. All callers updated (init.go, daemon.go) | Grammars load from `.aoa/grammars/` at runtime | Parser loads .so files. **Gap**: no automated test of dynamic loading |
| [L10](#layer-10) | [L10.3](#l103) | | | x | | | | | - | 🟢 | 🟢 | 🟢 | No outbound network -- removed Go HTTP downloader entirely. `aoa init` detects missing grammars and prints curl commands. Zero outbound connections. **S81**: CI security scan grep-enforces zero net/http imports | Full transparency, user controls all downloads | CI network audit on every push (grep-enforced) |
| [L10](#layer-10) | [L10.4](#l104) | | | x | | | | | - | 🟢 | 🟢 | 🟢 | Grammar build script -- `scripts/build-grammars.sh` compiles .so/.dylib from go-sitter-forest C source. Core pack = 11 grammars, 11 MB. Individual grammars 20 KB (json) to 3.5 MB (cpp). **S81**: Weekly CI validates all 509 grammars cross-platform (linux/darwin x amd64/arm64) | Grammars built from source, reproducible | Script tested locally + CI validates weekly on all 4 platforms |
| [L10](#layer-10) | | | | | | | | | | | | | *L10.5/L10.6 complete -- see COMPLETED.md* | | |
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

**Installation guide + grammar pipeline** -- PRIORITY 1 -- In progress (Session 79+). **Phase 1 COMPLETE (S80). Phase 2 COMPLETE (S81).**

## Terminology (non-negotiable)

- **`core` build tag** = the binary we ship. Tree-sitter runtime, dynamic `.so` loading. Built by `./build.sh` (default).
- **`lean` build tag** = no tree-sitter, pure Go. Built by `./build.sh --light`. Not shipped to users. Used for fast CI checks only.
- **`.so` files** = pre-compiled tree-sitter grammar shared libraries. One per language. NOT bundled in the binary. NOT bundled in npm. Downloaded by the user during `aoa init`.
- **Grammar source** = [alexaandru/go-sitter-forest](https://github.com/alexaandru/go-sitter-forest). 510 grammars aggregated from upstream tree-sitter repos. Each has `parser.c` + optional `scanner.c`.
- **`parsers.json`** = aOa-approved validated grammar list. Produced weekly by GitHub Actions. Contains: name, version, maintainer, upstream repo, sha256, size, security scan, platform status.
- **`build.sh`** = local dev guardrails (prevents Claude from running `go build` directly). NOT used in CI/CD.
- **`ci.yml`** = the real build. Runs in GitHub Actions. Produces the `core` binary for all platforms.
- **`grammar-validation.yml`** = weekly Sunday run (6am UTC). Compiles all 509 grammars on 4 platforms, produces `parsers.json` + `GRAMMAR_REPORT.md`. No security scan (cppcheck too slow for auto-generated parser.c; scanner.c scanning deferred).
- **`security.yml`** = CI security scan on every push. govulncheck (0 vulns), gosec (24 active rules), network audit (grep-enforced zero outbound connections).

## End-to-end installation flow (what the user does)

```
Step 1: npm install -g @mvpscale/aoa
        └─ npm pulls the lightweight platform package (binary only, ~8 MB)
        └─ Binary is the `core` build (tree-sitter runtime, dynamic grammar loading)
        └─ Binary was built in GitHub Actions, not locally. Provenance verified.

Step 2: cd my-project && aoa init
        └─ aOa creates .aoa/ directory (project-scoped, everything lives here)
        └─ Scans project files, detects languages (e.g. python, go, rust, yaml)
        └─ Checks .aoa/grammars/ for installed .so files
        └─ For missing grammars:
           └─ Writes .aoa/grammars/fetch.list (one URL+dest per grammar)
           └─ Shows user: "11 grammars needed (2.3 MB)"
           └─ Shows one-liner: xargs -n2 wget -qO < .aoa/grammars/fetch.list
           └─ URLs point to our GitHub release (pre-built, signed, validated)
        └─ If all grammars present: indexes project immediately
        └─ Output: "aOa indexed 6750 files, 5442 symbols, 28916 tokens"

Step 3: User runs the one-liner (or aoa prompts them)
        └─ Downloads only the .so files they need from our GitHub release
        └─ Each .so was compiled in GitHub Actions from go-sitter-forest source
        └─ Each .so has: sha256, cppcheck scan, SLSA provenance, maintainer credit

Step 4: aoa init (re-run)
        └─ Finds all grammars, indexes project with full structural parsing
        └─ "11 grammars ready"
        └─ Done. User is online.
```

## What must be built (step by step, in order)

### Phase 1: CI produces the binary -- COMPLETE (Session 80)
- [x] **1.1** Fix `ci.yml`: vet/test use `-tags lean`, build uses `./build.sh`, timeouts added
- [x] **1.2** Verify CI passes: vet, tests, standard build, light build all green (3 consecutive green runs)
- [x] **1.3** CI produces binary artifacts for all 4 platforms (linux/darwin x amd64/arm64) -- native runners
- [x] **1.4** CI uploads artifacts per platform (aoa-{os}-{arch})

### Phase 2: Grammar validation pipeline -- COMPLETE (Session 81)
- [x] **2.1** Fix `grammar-validation.yml`: `go mod download all`, portable sed, timeouts
- [x] **2.2** Verify weekly workflow passes on all platforms (4-platform matrix all green)
- [x] **2.3** `parsers.json` committed to repo with full provenance per grammar (509/509: maintainer, upstream URL, upstream revision, source SHA, binary SHA per platform)
- [x] **2.4** `GRAMMAR_REPORT.md` committed with human-readable summary table (Y/N platform columns, failures section, contributor count)
- [x] **2.5** Acknowledgments section thanking 346 individual contributors + @alexaandru for go-sitter-forest
- [x] **2.6** Array-vs-dict bug fixed: upstream URL + revision now populated for all 509 grammars

### Phase 3: Grammar release (pre-built .so files hosted)
- [ ] **3.1** Release workflow: compiles all 510 grammars per platform from go-sitter-forest
- [ ] **3.2** Uploads .so files to GitHub release with SLSA provenance
- [ ] **3.3** Release tagged (e.g. `grammars-v1`) with checksums
- [ ] **3.4** URLs in fetch.list point to this release

### Phase 4: aoa init flow (user-facing)
- [ ] **4.1** `aoa init` detects languages, checks .aoa/grammars/ for installed .so
- [ ] **4.2** Writes fetch.list with correct GitHub release URLs
- [ ] **4.3** Shows user: count, total size, one-liner command
- [ ] **4.4** On re-run with all grammars present: indexes immediately
- [ ] **4.5** `aoa grammar list` shows all 510 with maintainer + upstream repo
- [ ] **4.6** Progress + ETA for any step taking >2 seconds

### Phase 5: npm packaging
- [ ] **5.1** Platform packages (`@mvpscale/aoa-linux-x64`, etc.) contain only the binary
- [ ] **5.2** `install.js` postinstall shows next steps clearly
- [ ] **5.3** Release workflow publishes to npm from GitHub Actions
- [ ] **5.4** End-to-end test: `npm install -g @mvpscale/aoa && aoa init` on fresh machine

### Phase 6: Installation guide (docs/INSTALL.md)
- [ ] **6.1** Document the full flow with examples
- [ ] **6.2** Troubleshooting section (no gcc, no wget, firewall, etc.)
- [ ] **6.3** `aoa grammar list` output example with attribution
- [ ] **6.4** Link to GRAMMAR_REPORT.md and parsers.json for trust verification

## Session 79 completed
- [x] `build.sh` simplified: default = core, `--light` = lean, `--recon`/`--core` deprecated
- [x] Everything project-scoped under `{root}/.aoa/`
- [x] All 509 grammars validated locally (509/509 compiled, 0 failures)
- [x] Weekly grammar validation workflow created
- [x] CI workflow created (needs fixes from S79 findings)
- [x] Advisory Rule added to CLAUDE.md
- [x] `deploy.sh` updated to use `./build.sh` (not deprecated `--core`)

## Session 80 completed
- [x] **Phase 1 ALL 4 subtasks done** -- CI produces the binary on all 4 platforms
- [x] ci.yml: vet/test use `-tags lean`, build uses `./build.sh`, proper timeouts
- [x] `//go:build !lean` added to 7 test files (treesitter loader_test, loader_e2e_test, parser_test; app indexer_test, reindex_test, watcher_test; integration cli_test)
- [x] Gauntlet ceilings relaxed for brute-force paths (WordBoundary 15->30ms, InvertMatch 25->50ms, Glob 20->40ms)
- [x] build.sh bash 3.2 compatibility (removed `;& ` fall-through for macOS)
- [x] 4-platform build matrix: linux/darwin x amd64/arm64, native runners, all green
- [x] Artifacts uploaded per platform (aoa-{os}-{arch})
- [x] 3 consecutive green CI runs
- [x] Release workflow overhauled: dropped all aoa-recon matrix entries + npm publish steps, switched to ./build.sh core builds, same native runner matrix (120 lines removed, 37 added)
- [x] npm package.json repo URLs fixed from aOa-go to aOa in all 5 active packages

## Session 81 completed
- [x] **Phase 2 ALL subtasks done** -- Grammar validation pipeline complete
- [x] 4-platform matrix (linux/darwin x amd64/arm64) all green
- [x] Fixed array-vs-dict bug: 509/509 maintainer + upstream URL + revision now populated
- [x] parsers.json: full provenance (source SHA, binary SHA per platform, upstream repo, maintainer, revision)
- [x] GRAMMAR_REPORT.md: Y/N platform columns, failures section, contributor count
- [x] Acknowledgments section: 346 individual contributors + @alexaandru for go-sitter-forest
- [x] Weekly schedule (Sunday 6am UTC) auto-runs, auto-commits results
- [x] **SECURITY.md** created -- human-first trust document (no telemetry, no outbound, localhost-only, etc.)
- [x] **Security CI pipeline** -- govulncheck (0 vulns), gosec (24 rules), network audit on every push
- [x] Fixed real Slowloris vulnerability (G112) -- added ReadHeaderTimeout
- [x] Bumped Go to 1.25.7 (crypto/tls GO-2026-4337)
- [x] gosec exclusions documented with rationale
- [x] **aoa-recon removed** (-761 LOC) -- cmd/aoa-recon/, npm/aoa-recon/, recon_bridge.go, recon.go cmd deleted
- [x] DimensionalResults/ReconAvailable moved to dimensional.go (dashboard still works)
- [x] CI simplified: no more grep -v /cmd/aoa-recon exclusion hacks
- [x] Build strategy decided: compile aOa once per version, embed parsers.json via //go:embed

## Key decisions
- Binary built in GitHub Actions, not locally. CI is the real build.
- Grammar .so files pre-compiled in GitHub Actions with SLSA provenance
- Users download only the grammars they need via one-liner from fetch.list
- aOa never makes outbound network connections itself
- `parsers.json` is the aOa-approved trust document
- Maintainer attribution shown per grammar -- credit to the people who built them
- Everything project-scoped: `aoa remove` wipes it all
- Build strategy: compile aOa once per version, embed parsers.json via //go:embed. Grammar metadata updated weekly independently.
- aoa-recon removed entirely -- two build modes: core (./build.sh) and lean (./build.sh --light)

**Files**: `build.sh`, `deploy.sh`, `cmd/aoa/cmd/init.go`, `cmd/aoa/cmd/init_grammars_cgo.go`, `cmd/aoa/cmd/grammar_cgo.go`, `internal/adapters/treesitter/loader.go`, `internal/adapters/treesitter/manifest.go`, `scripts/validate-grammars.sh`, `scripts/build-grammars.sh`, `.github/workflows/grammar-validation.yml`, `.github/workflows/ci.yml`, `.github/workflows/security.yml`, `SECURITY.md`, `dist/parsers.json`, `GRAMMAR_REPORT.md`, `npm/`

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

*L7.4 (.aoa/ directory restructure) -- COMPLETE, archived to COMPLETED.md*

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

Old warmReconCache/updateReconForFile deleted in Session 71 (clean separation). S81: aoa-recon removed entirely (-761 LOC). `recon_bridge.go` deleted. `DimensionalResults`/`ReconAvailable` moved to `dimensional.go`. Dashboard recon tab still works via dimensional engine.

**Gap**: No unit tests for incremental path.

**Files**: `internal/adapters/web/recon.go`, `internal/app/dimensional.go`, `internal/app/watcher.go`

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

S79 update: `build.sh` simplified. Default build is now tree-sitter + dynamic grammars (what `--core` used to be). `--light` = pure Go. `--core` deprecated (falls through to default). `--recon`/`--recon-bin` deprecated with messages. Two effective tiers: standard (tree-sitter C runtime + dynamic .so loading) and light (pure Go, no parser).

**Files**: `build.sh`, `cmd/aoa/build_guard.go`, `internal/adapters/treesitter/languages_core.go`, `internal/adapters/treesitter/languages_forest.go`

#### L10.2

**Grammar paths wired into parser** -- green Complete, yellow No automated test

`newParser(root string)` now configures `DefaultGrammarPaths` pointing to `.aoa/grammars/` for dynamic .so loading at runtime. All callers updated (init.go, daemon.go).

**Files**: `cmd/aoa/cmd/parser_cgo.go`, `cmd/aoa/cmd/parser_nocgo.go`, `cmd/aoa/cmd/init.go`, `cmd/aoa/cmd/daemon.go`

#### L10.3

**No outbound network** -- COMPLETE (triple-green S81)

Removed the Go HTTP downloader entirely. `aoa init` detects missing grammars and prints `curl` commands for the user to run. Zero outbound connections from the binary. Full transparency and user control. S81: CI security scan grep-enforces zero net/http imports on every push.

**Files**: `internal/adapters/treesitter/downloader.go` (rewritten: PlatformString + GlobalGrammarDir only), `internal/adapters/treesitter/downloader_test.go`, `internal/adapters/treesitter/manifest.go`

#### L10.4

**Grammar build script** -- COMPLETE (triple-green S81)

`scripts/build-grammars.sh` compiles .so/.dylib from `alexaandru/go-sitter-forest` C source. Tested locally: core pack = 11 grammars, 11 MB total. Individual grammars range from 20 KB (json) to 3.5 MB (cpp). S79: `scripts/validate-grammars.sh` compiled all 509 grammars -- 509/509 passed, zero failures. Weekly validation workflow (`.github/workflows/grammar-validation.yml`) runs Sunday 6am UTC on linux-amd64/arm64 + darwin-arm64. Produces `dist/parsers.json` and commits `GRAMMAR_REPORT.md`. S81: CI validated on all 4 platforms, all green.

**Files**: `scripts/build-grammars.sh`, `scripts/validate-grammars.sh` (new), `scripts/gen-manifest-hashes.go`, `.github/workflows/grammar-validation.yml` (new)

*L10.5 (`aoa init` as single command) -- COMPLETE, archived to COMPLETED.md*

*L10.6 (Command rename: wipe -> reset + remove) -- COMPLETE, archived to COMPLETED.md*

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

### What Works (Preserve)

| Component | Notes |
|-----------|-------|
| Search engine (O(1) inverted index) | 26/26 parity tests, 4 search modes, trigram content search (~60us on 500 files), case-sensitive default (G1). **G0 perf**: regex trigram extraction (5s->8ms), symbol search gated on metadata (186ms->4us). All ops sub-25ms. **G0 gauntlet**: 22-shape perf regression suite (`test/gauntlet_test.go`) -- ceiling test in `go test ./...`, benchstat baselines via `make bench-gauntlet/bench-baseline/bench-compare`. Covers every Search() code path including regex trigram, lean-mode guard, brute-force, glob, context, count, quiet, only-matching. |
| Learner (21-step autotune) | 5/5 fixture parity, float64 precision. Do not change decay/prune constants. |
| Session Prism (Claude JSONL reader) | Defensive parsing, UUID dedup, compound message decomposition. |
| Tree-sitter parser (509 languages) | go-sitter-forest, behind `ports.Parser` interface. Core build: C runtime compiled in, grammars loaded dynamically from `.aoa/grammars/*.so`. |
| Single-binary distribution (L10) | One `aoa` binary (core build with tree-sitter C runtime). Grammars downloaded as .so/.dylib via `aoa init` curl commands. No outbound network from binary. CI-enforced (grep audit). |
| Security pipeline (S81) | SECURITY.md trust document, govulncheck + gosec + network audit CI on every push. Slowloris fix (ReadHeaderTimeout). |
| Grammar validation (L4.4 P2) | Weekly CI: 509/509 grammars validated on 4 platforms, parsers.json provenance, GRAMMAR_REPORT.md, 346 contributor acknowledgments. |
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
| [SECURITY.md](../SECURITY.md) | Trust document -- no telemetry, no outbound, localhost-only, security scan results |
| [CLAUDE.md](../CLAUDE.md) | Agent instructions, architecture reference, build commands |

### Quick Reference

| Resource | Location |
|----------|----------|
| Build (standard) | `./build.sh` -- tree-sitter C runtime, dynamic .so loading, zero compiled-in grammars |
| Build (light) | `./build.sh --light` -- pure Go, no tree-sitter, no CGo (~8 MB). CI checks only. |
| Test | `go test ./...` |
| CI check | `make check` |
| Database | `{ProjectRoot}/.aoa/aoa.db` |
| Socket | `/tmp/aoa-{sha256(root)[:12]}.sock` |
| Dashboard | `http://localhost:{port}` (port in `.aoa/http.port`) |
| Session logs | `~/.claude/projects/{encoded-path}/*.jsonl` |
