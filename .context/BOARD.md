# Project Board

[Board](#board) | [Supporting Detail](#supporting-detail) | [Completed](COMPLETED.md) | [Backlog](BACKLOG.md)

> **Updated**: 2026-02-25 (Session 71, revalidated) | **89% complete.**
> **Completed work**: See [COMPLETED.md](COMPLETED.md) — Phases 1–8c + L0 + L1 + L2.2–L2.7 + L3 (incl. L3.15) + L4.1/L4.3 + L5.1–L5.6/L5.9 + L6.1–L6.6/L6.8–L6.10 (470+ active tests, 32 skipped)
> **Archived boards**: `.context/archived/`

---

## Goals

> Atomic architectural principles. Every task is evaluated against each goal independently.

| Goal | Statement |
|------|-----------|
| **G0** | **Speed** — 50-120x faster than Python. Sub-ms search, <200ms startup, <50MB memory. No O(n) on hot paths. |
| **G1** | **Parity** — Zero behavioral divergence from Python. Test fixtures are source of truth. |
| **G2** | **Two Binaries, Clean Split** — `aoa` works standalone with zero deps. `aoa-recon` is optional; when installed it enhances `aoa` through a defined bridge. `aoa` must never depend on `aoa-recon` being present. |
| **G3** | **Agent-First** — Drop-in shim for grep/egrep/find. Three Unix modes: direct (`grep pat file`), pipe (`cmd | grep pat`), index (`grep pat` → O(1) daemon). Same flags, same output format, same exit codes. Agents never know it's not GNU grep. |
| **G4** | **Clean Architecture** — Hexagonal. Domain logic dependency-free. External concerns behind interfaces. No feature entanglement. |
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

**Current**: Core engine complete (search, learner, dashboard, grep parity). Recon scanning operational with caching and investigation tracking. Two-binary distribution published to npm (`@mvpscale/aoa` + `@mvpscale/aoa-recon` v0.1.7). Dimensional engine with 5 active tiers (security, performance, quality, architecture, observability), 136 YAML rules across 21 dimensions. CI/release pipeline proven across 5 releases.

**Approach**: TDD. Each layer validated before the next. Completed work archived to keep the board focused on what's next.

---

## Board

### P0: Critical Bugs (fix before any new work)

> See [BUGFIX.md](BUGFIX.md) for full details and pattern analysis.

| Layer | ID | G0 | G1 | G2 | G3 | G4 | G5 | G6 | Dep | Cf | St | Va | Task | Value | Va Detail |
|:------|:---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:----|:--:|:--:|:--:|:-----|:------|:----------|
| **P0** | [B7](#b7) | | | | x | | | | - | 🟢 | 🟢 | 🟢 | Remove "recon cached" from pure aOa logs | Pure mode must not mention recon | Old scanner deleted entirely -- zero recon code in lean path |
| **P0** | [B9](#b9) | | | | x | | | x | - | 🟢 | 🟢 | 🟢 | Recon tab shows install prompt in pure mode | Users know how to install recon | Install prompt only path when recon not enabled |
| **P0** | [B10](#b10) | x | | | | x | | | - | 🟢 | 🟢 | 🟢 | Gate `warmReconCache()` in `Reindex()` behind recon availability | Recon scanner runs after reindex even when recon not installed | Superseded -- warmReconCache() deleted entirely |
| **P0** | [B11](#b11) | x | | | | x | | | - | 🟢 | 🟢 | 🟢 | Gate `updateReconForFile` in file watcher behind recon availability | Old scanner runs on every file change even without recon | Superseded -- updateReconForFile() deleted entirely |
| **P0** | [B14](#b14) | | | | x | | | x | - | 🟢 | 🟢 | 🟢 | Remove truncation on debrief text — assistant and thinking | Users see full text in debrief tab | 500-char truncation removed from user input text |
| **P0** | [B15](#b15) | x | | | | x | | | - | 🟢 | 🟢 | 🟢 | Fix `BuildFileSymbols` called for entire index on every file change | Single file edit rebuilds all 4000 file symbols under lock, blocks dashboard | Superseded -- BuildFileSymbols calls deleted with old scanner |
| **P0** | [B17](#b17) | | | | x | x | | | - | 🟢 | 🟢 | 🟢 | Add debug mode (`AOA_DEBUG=1`) — log all runtime events | No way to diagnose runtime issues; file changes, searches, hangs invisible | AOA_DEBUG=1 enables timestamped debug logging at all key event points |

### Active Board

| Layer | ID | G0 | G1 | G2 | G3 | G4 | G5 | G6 | Dep | Cf | St | Va | Task | Value | Va Detail |
|:------|:---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:----|:--:|:--:|:--:|:-----|:------|:----------|
| **L0** | | | | | | | | | | | | | *All 12 tasks complete — see COMPLETED.md* | | |
| **L1** | | | | | | | | | | | | | *All 8 tasks complete — see COMPLETED.md* | | |
| [L2](#layer-2) | [L2.1](#l21) | x | | | | x | x | | - | 🟢 | 🟢 | 🟡 | Wire file watcher — `Watch()` in app.go, change→reparse→reindex | Dynamic re-indexing without restart | Unit: 10 tests (4 app + 6 adapter). **Gap**: no integration test through fsnotify event pipeline |
| **L3** | | | | | | | | | | | | | *All tasks complete — see COMPLETED.md* | | |
| [L3](#layer-3) | [L3.15](#l315) | | x | | x | | | | - | 🟢 | 🟢 | 🟢 | GNU grep native parity — 3 modes, 22 flags, stdin/files/index routing | Drop-in grep replacement for AI agents | 135 parity tests (77 internal + 58 Unix). Revalidated 2026-02-25 |
| [L4](#layer-4) | [L4.2](#l42) | | | x | | | | | L4.1 | 🟡 | 🟢 | 🟡 | Grammar CLI + build CI — `aoa grammar list/install/info` | Easy grammar distribution | CLI works. **Pivoted**: download irrelevant after G2 split — aoa-recon compiles in all 509 grammars. Revalidated 2026-02-25 |
| [L4](#layer-4) | [L4.4](#l44) | | | x | x | | | | L4.3 | 🟢 | ⚪ | ⚪ | Installation docs — `go install` or download binary | Friction-free onboarding | New user installs and runs in <2 minutes |
| [L5](#layer-5) | [L5.7](#l57) | | | | | | | | L5.1 | 🟢 | 🟢 | 🟡 | Performance tier — 26 rules across 5 dimensions (resources, concurrency, query, memory, hot_path) | Second tier coverage | All 5 dims wired in dashboard. **Gap**: per-rule detection validation. Revalidated 2026-02-25 |
| [L5](#layer-5) | [L5.8](#l58) | | | | | | | | L5.1 | 🟢 | 🟢 | 🟡 | Quality tier — 24 rules across 4 dimensions (errors, complexity, dead_code, conventions) | Third tier coverage | All 4 dims wired in dashboard. **Gap**: per-rule detection validation. Revalidated 2026-02-25 |
| [L5](#layer-5) | [L5.10](#l510) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Add dimension scores to search results (`S:-1 P:0 C:+2`) | Scores visible inline | Scores appear in grep/egrep output |
| [L5](#layer-5) | [L5.11](#l511) | | | | x | | | | L5.5 | 🟢 | ⚪ | ⚪ | Dimension query support — `--dimension=security --risk=high` | Filter by dimension | CLI filters by tier and severity |
| [L5](#layer-5) | [L5.12](#l512) | | | | | | | x | L5.9 | 🟢 | 🟢 | 🟡 | Recon tab — dimensional engine with interim scanner fallback | Dashboard dimensional view | API works. **Gap**: dashboard UI upgrade for bitmask scores |
| [L5](#layer-5) | [L5.13](#l513) | | | | | | | x | L5.12 | 🟢 | 🟢 | 🟡 | Recon dashboard overhaul — 5 focus modes, tier redesign, code toggle, copy prompt | Recon tab is actionable, not just a finding list | **Gap**: browser-only validation |
| [L5](#layer-5) | [L5.14](#l514) | x | | | | x | | | L5.12 | 🟢 | 🟢 | 🟡 | Recon cache + incremental updates — pre-compute at startup, SubtractFile/AddFile on file change | Zero per-poll scan cost, instant API response | **Gap**: no unit tests for incremental path |
| [L5](#layer-5) | [L5.15](#l515) | | | | | | | x | L5.14 | 🟢 | 🟢 | 🟡 | Investigation tracking — per-file investigated status, persistence, auto-expiry | Users can mark files as reviewed, auto-clears on change | **Gap**: no unit tests |
| [L5](#layer-5) | [L5.16](#l516) | | | | | | | x | L5.1 | 🟢 | 🟢 | 🟡 | Security dimension expansion — YAML rework complete, universal concept layer, 37 rules loaded | Complete 5-dimension security tier | All rules load + pass. **Gap**: per-rule detection validation. Revalidated 2026-02-25 |
| [L5](#layer-5) | [L5.17](#l517) | | | | | | | x | L5.1 | 🟢 | 🟢 | 🟡 | Architecture dimension expansion — YAML rework complete, universal concept layer | Complete 3-dimension architecture tier | All rules load + pass. **Gap**: per-rule detection validation |
| [L5](#layer-5) | [L5.18](#l518) | | | | | | | x | L5.1 | 🟢 | 🟢 | 🟡 | Observability dimension expansion — YAML rework complete, universal concept layer | Complete 2-dimension observability tier | All rules load + pass. **Gap**: per-rule detection validation |
| [L5](#layer-5) | [L5.19](#l519) | | | | | | | x | L5.1 | 🟢 | 🟢 | 🟡 | ~~Compliance tier~~ — **Pivoted**: tier removed, concepts absorbed into security tier (config dimension). `TierReserved` preserves bitmask slot. No YAML file. | ~~Compliance coverage~~ | Superseded. Revalidated 2026-02-25 |
| [L6](#layer-6) | [L6.7](#l67) | | | | | | | x | L6.6 | 🟢 | 🟢 | 🟡 | Dashboard Recon tab install prompt — "npm install aoa-recon" when not detected | Users know how to unlock Recon | **Gap**: browser-only validation |
| [L6](#layer-6) | [L6.8](#l68) | | | x | | | | | L6.2 | 🟢 | 🟢 | 🟢 | npm package structure — wrapper + platform packages + JS shims | Zero-friction install via npm | Published: `@mvpscale/aoa` v0.1.7 (2026-02-22). Revalidated 2026-02-25 |
| [L6](#layer-6) | [L6.9](#l69) | | | x | | | | | L6.4, L6.8 | 🟢 | 🟢 | 🟢 | npm recon package structure — wrapper + platform packages | Zero-friction recon install | Published: `@mvpscale/aoa-recon` v0.1.7 (2026-02-22). Revalidated 2026-02-25 |
| [L6](#layer-6) | [L6.10](#l610) | | | x | | | | | L6.8, L6.9 | 🟢 | 🟢 | 🟢 | CI/Release — workflow builds both binaries, publishes to npm | Tag → build → publish, fully automated | 5 successful releases (v0.1.3–v0.1.7). Revalidated 2026-02-25 |
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

`Rebuild()` on `SearchEngine`. `onFileChanged()` handles add/modify/delete. 10 unit tests: 4 in app (new/modify/delete/unsupported) + 6 in adapter (detect change/new/delete, ignore non-code, reindex latency, stop cleanup). Revalidated 2026-02-25.

**Gap**: No integration test through fsnotify event pipeline — needs `TestDaemon_FileWatcher_ReindexOnEdit`.

**Files**: `internal/domain/index/search.go`, `internal/app/app.go`, `internal/app/watcher.go`, `internal/domain/index/rebuild_test.go`, `internal/app/watcher_test.go`

---

### Layer 3

#### L3.15

**GNU grep native parity** — 🟢🟢🟢 Complete + validated. Revalidated 2026-02-25.

Three-route architecture: file args → `grepFiles()`, stdin pipe → `grepStdin()`, neither → daemon index search → fallback `/usr/bin/grep`. 22 native flags covering 100% of observed AI agent usage.

**Validation**: 135 automated parity tests across two suites:
- `test/migration/grep_parity_test.go` (77 tests): internal search engine vs fixture index — flags, combinations, edge cases, coverage matrix
- `test/migration/unix_grep_parity_test.go` (58 tests): CLI output format vs `/usr/bin/grep` — exit codes, stdin/file/index routing, real-world agent invocations (Claude, Gemini), snapshots

**Files**: `cmd/aoa/cmd/grep.go`, `egrep.go`, `tty.go`, `grep_native.go`, `grep_fallback.go`, `grep_exit.go`, `output.go`

---

### Layer 4

**Layer 4: Distribution**

#### L4.2

**Grammar CLI + Build CI** — 🟢 Code complete, 🟡 Pivoted. Revalidated 2026-02-25.

`aoa grammar list/info/install/path` commands work. 57-grammar manifest embedded. CI workflow defined.

**Pivoted**: After G2 split, `aoa-recon` compiles in all 509 grammars via go-sitter-forest. Dynamic grammar download is no longer needed — users install `aoa-recon` and get everything. The `aoa grammar install` download TODO is effectively dead code. Consider moving to backlog.

**Files**: `cmd/aoa/cmd/grammar.go`, `internal/adapters/treesitter/manifest.go`, `grammars/manifest.json`, `.github/workflows/build-grammars.yml`

#### L4.4

**Installation docs** — ⚪ Not started

Two install paths: `go install` (from source) vs binary download (lean + grammar packs). Post-install: `aoa init`, `aoa daemon start`.

**Files**: `README.md`

---

### Layer 5

**Layer 5: Dimensional Analysis (Bitmask engine, Recon tab)**

> Early warning system. 5 active tiers, 21 dimensions. Dimensional engine with 136 YAML rules across all tiers. **YAML rework complete**: Universal concept layer (15 concepts, 509 languages), declarative `structural:` blocks, `lang_map` eliminated from all rules. Performance tier (26 rules, 5 dims) and Quality tier (24 rules, 4 dims) fully populated. Compliance tier pivoted — concepts absorbed into security. Revalidated 2026-02-25.
>
> **ADR**: [Declarative YAML Rules](../decisions/2026-02-23-declarative-yaml-rules.md) -- spec (rework done)
> **Research**: [Bitmask analysis](../docs/research/bitmask-dimensional-analysis.md) | [AST vs LSP](../docs/research/asv-vs-lsp.md) | [Dimensional taxonomy](details/2026-02-23-dimensional-taxonomy.md)

#### L5.7

**Performance tier** — 🟢 Complete, 🟡 Per-rule detection validation gap. Revalidated 2026-02-25.

26 rules across 5 dimensions, all wired in dashboard. YAML rework done (declarative structural blocks, no lang_map, universal concepts).

**Dimensions** (all complete):
- **Resources** (5 rules): `defer_in_loop`, `open_without_close`, etc.
- **Concurrency** (5 rules): `lock_in_loop`, `goroutine_in_loop`, `channel_in_loop`, `goroutine_leak`, `sync_primitive_usage`
- **Query** (5 rules): `query_in_loop`, `exec_in_loop`, `unbounded_query`, `db_without_transaction`, `raw_sql_string`
- **Memory** (5 rules): `allocation_in_loop`, `append_in_loop`, `string_concat_in_loop`, `large_buffer_alloc`, `regex_compile_in_loop`
- **Hot Path** (6 rules): `reflection_call`, `json_marshal_in_loop`, `fmt_sprint_in_loop`, `sleep_in_handler`, `sort_in_loop`, `map_lookup_in_loop`

**Gap**: No per-rule detection validation tests (rules load and parse, but individual detection accuracy untested).

**Files**: `recon/rules/performance.yaml`, `internal/adapters/web/static/app.js` (RECON_TIERS)

#### L5.8

**Quality tier** — 🟢 Complete, 🟡 Per-rule detection validation gap. Revalidated 2026-02-25.

24 rules across 4 dimensions, all wired in dashboard. YAML rework done (declarative structural blocks, no lang_map, universal concepts).

**Dimensions** (all complete):
- **Errors** (7 rules): `ignored_error`, `panic_in_lib`, `unchecked_type_assertion`, `error_not_checked`, `empty_catch_block`, `error_without_context`, `deprecated_stdlib`
- **Complexity** (6 rules): `long_function`, `nesting_depth`, `too_many_params`, `large_switch`, `god_function`, `wide_function`
- **Dead Code** (5 rules): `unreachable_code`, `commented_out_code`, `unused_import`, `empty_function_body`, `disabled_test`
- **Conventions** (6 rules): `exported_no_doc`, `init_side_effects`, `magic_number`, `boolean_param`, `deeply_nested_callback`, `inconsistent_receiver`

**Gap**: No per-rule detection validation tests (rules load and parse, but individual detection accuracy untested).

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

Old warmReconCache/updateReconForFile deleted in Session 71 (clean separation). Recon now gated behind `aoa recon init` + `.aoa/recon.enabled` marker. When enabled, ReconBridge handles scanning via separate aoa-recon binary.

**Gap**: No unit tests for incremental path.

**Files**: `internal/app/recon_bridge.go`, `internal/adapters/web/recon.go`, `internal/app/watcher.go`

#### L5.15

**Investigation tracking** — 🟢 Complete, 🟡 No unit tests

Per-file investigated status with persistence (`.aoa/recon-investigated.json`), 1-week auto-expiry, auto-clear on file change. `POST /api/recon-investigate` endpoint. Dashboard investigated tier with solo mode.

**Gap**: No unit tests for investigation methods.

**Files**: `internal/app/app.go`, `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

#### L5.16

**Security dimension expansion** — 🟢 YAML rework complete, 🟡 Per-rule validation gap. Revalidated 2026-02-25.

37 rules across all 5 security dimensions. Universal concept layer eliminates per-rule `lang_map`. All rules use declarative `structural:` blocks per ADR. LangMap removed from Rule struct, yamlRule, convertRule(), and all hardcoded fallback rules.

**Complete**:
- `security.yaml` -- 37 rules with proper `structural:` blocks, no `lang_map`, universal header
- `rules_security.go` -- 6 hardcoded fallback rules, LangMap removed
- `lang_map.go` -- rewritten with `conceptDefaults` (all 509 langs) + `langOverrides` (10 langs)
- `walker.go` -- simplified `resolveMatchConcept()` to single `Resolve()` call
- Dashboard wires all 5 security dimensions

**Gap**: No per-rule detection validation tests (rules load and parse, but individual detection accuracy untested).

**Files**: `recon/rules/security.yaml`, `internal/domain/analyzer/rules_security.go`, `internal/domain/analyzer/lang_map.go`, `internal/adapters/treesitter/walker.go`

#### L5.17

**Architecture dimension expansion** — 🟢 YAML rework complete, 🟡 Per-rule validation gap

Rules across all 3 architecture dimensions with declarative `structural:` blocks. Universal concept layer, no `lang_map`. Universal header.

**Complete**: `architecture.yaml` reworked, dashboard dimensions (antipatterns, imports, api_surface).

**Gap**: No per-rule detection validation tests.

**Files**: `recon/rules/architecture.yaml`, `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

#### L5.18

**Observability dimension expansion** — 🟢 YAML rework complete, 🟡 Per-rule validation gap

Rules across both observability dimensions with declarative `structural:` blocks. Universal concept layer, no `lang_map`. Universal header.

**Complete**: `observability.yaml` reworked, dashboard dimensions (debug, silent_failures).

**Gap**: No per-rule detection validation tests.

**Files**: `recon/rules/observability.yaml`, `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

#### L5.19

**Compliance tier** — **Pivoted / Superseded**. Revalidated 2026-02-25.

Compliance tier was removed from the codebase. `TierReserved` in `types.go` preserves the bitmask slot ("formerly compliance — slot preserved for bitmask compat"). No `compliance.yaml` file exists. Compliance concepts (CVE patterns, licensing, data handling) were absorbed into the security tier's config dimension.

Dashboard `RECON_TIERS` has 5 active tiers (security, performance, quality, architecture, observability) — compliance is not among them.

---

### Layer 6

**Layer 6: Distribution v2**

> aoa (pure Go, 11 MB) + aoa-recon (CGo, 361 MB). npm distribution.

#### L6.7

**Dashboard Recon tab install prompt** — 🟢 Complete, 🟡 Browser-only

`recon_available` field in API. Install prompt with `npm install -g aoa-recon`. "Lite mode" indicator.

**Files**: `internal/adapters/web/recon.go`, `internal/adapters/web/static/app.js`

#### L6.8 + L6.9

**npm packages** — 🟢🟢🟢 Published. Revalidated 2026-02-25.

10 npm packages: 2 wrappers + 8 platform-specific. esbuild/turbo pattern with JS postinstall shim. Published as `@mvpscale/aoa` and `@mvpscale/aoa-recon` v0.1.7 (2026-02-22).

**Files**: `npm/aoa/`, `npm/aoa-recon/`, 8× `npm/aoa-{platform}/`

#### L6.10

**CI/Release** — 🟢🟢🟢 Tested end-to-end. Revalidated 2026-02-25.

Release workflow: 8 matrix jobs (2 binaries × 4 platforms), GitHub release, npm publish. 5 successful releases (v0.1.3–v0.1.7).

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
| Search engine (O(1) inverted index) | 26/26 parity tests, 4 search modes, trigram content search (~60us on 500 files), case-sensitive default (G1). **G0 perf**: regex trigram extraction (5s->8ms), symbol search gated on metadata (186ms->4us). All ops sub-25ms. |
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
| File watcher (L2) | `onFileChanged` -> re-parse -> `Rebuild` -> `SaveIndex` -> `clearFileInvestigated`. Recon gated behind `.aoa/recon.enabled`. |

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
| [Declarative YAML Rules ADR](decisions/2026-02-23-declarative-yaml-rules.md) | Spec for dimensional rules rework -- schema, constraints, Go types |
| [Dimensional Taxonomy](details/2026-02-23-dimensional-taxonomy.md) | 142 questions across 21 dimensions, 6 tiers |
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
