# Index

> **Updated**: 2026-03-01 (Session 85 -- board updated)

## Active Layer

**L4** (Distribution) -- L4.4 in progress -- 4-phase roadmap. **Phase 1 COMPLETE (S80)**, **Phase 2 COMPLETE (S81)**, **Phase 3 ~95% (S84)**: L4.4-3.1 through L4.4-3.5 done (pre-built .so distribution, UX rewrite, SHA-256 verification, `--update` full sync). L4.4-3.6 (fresh machine e2e) remains. Phase 4: npm + e2e. L4.2 superseded -> BACKLOG.md. L4.4-4.2 (npm postinstall) done.

**L5** (Dimensional Analysis) -- YAML rework complete. L5.Va consolidates all per-rule validation (was L5.7/8/16/17/18 + L8.1) into one task. L5.19 superseded (archived). L5.10/11 not started (dimension scores + query support). **S85**: `SignalScore()` (Formula P) validated against real-world data -- gate raised to 6. `TestRealWorldDistribution` fixture-driven test. 9 priority rules amplified. `error_not_checked` replaced by `empty_catch_body` (structural, cross-language).

**L8** (Recon) -- L8.1 absorbed into L5.Va. L8.2-L8.5 green with Va gaps (browser-only / unit tests). L8.6 not started. **S81**: aoa-recon removed. **S85**: 92% noise reduction (49,838 -> 4,149 findings). Three fixes: gitignore-aware indexing (79% of findings from vendored tree-sitter source), generated file detection (`languages_forest.go` = 509 FPs), `error_not_checked` replaced by `empty_catch_body` (4,406 FPs eliminated). Formula P scoring validated against real data, gate=6. `TestRealWorldDistribution` fixture test. `cmd/dim-dump/` tool for bbolt data export.

**L9** (Telemetry) -- **Complete, archived.** All 9 tasks triple-green. See COMPLETED.md.

**L10** (Dynamic Grammar Distribution) -- Single-binary architecture. L10.3/L10.4/L10.5/L10.6 triple-green (L10.5/L10.6 archived). L10.1/L10.2/L10.7 green (Va gaps). **L10.8 superseded by L4.4 Phase 3.** L10.9 now L4.4 Phase 4.4 (end-to-end test).

**G0** (Speed) -- G0.HF1 triple-green, archived to COMPLETED.md.

## Unblocked Tasks

Tasks with no blocking dependencies (or all deps satisfied):

| ID | Cf | St | Va | Task |
|----|:--:|:--:|:--:|------|
| L4.4 | 🟢 | 🟡 | ⚪ | Installation + onboarding pipeline -- Phase 1+2 DONE. Phase 3 ~95% (S84): 3.1-3.5 done, 3.6 remains |
| L4.4-3.1 | 🟢 | 🟢 | ⚪ | Update grammar-validation.yml -- commit .so/.dylib to `grammars/{platform}/`, Git LFS. **DONE S83** |
| L4.4-3.2 | 🟢 | 🟢 | ⚪ | Create `grammars/README.md` -- structure, platforms, SHA verification. **DONE S83** |
| L4.4-3.3 | 🟢 | 🟢 | ⚪ | `aoa init` UX rewrite -- fresh path (educational, trust-building), returning path (concise), download.sh SHA-256 via awk. **DONE S84** |
| L4.4-3.4 | 🟢 | 🟢 | ⚪ | Remove compile-from-source code, eliminated download.sha256 sidecar. **DONE S84** |
| L4.4-3.5 | 🟢 | 🟢 | ⚪ | `aoa init --update` full sync -- scan, generate, execute, index. One command. **DONE S84** |
| L4.4-3.6 | 🟢 | ⚪ | ⚪ | End-to-end verify on fresh machine |
| L5.Va | 🟢 | 🟢 | 🟡 | Dimensional rule validation -- all 5 tiers (absorbs L5.7/8/16/17/18 + L8.1) |
| L5.10 | 🟢 | ⚪ | ⚪ | Dimension scores in search results |
| L5.11 | 🟢 | ⚪ | ⚪ | Dimension query support |
| L7.1 | 🟢 | 🟢 | 🟡 | Startup progress (gap: timing test) |
| L8.2 | 🟢 | 🟢 | 🟡 | Recon dashboard overhaul (gap: browser-only) |
| L8.3 | 🟢 | 🟢 | 🟡 | Recon cache + incremental (gap: unit tests). S81: recon_bridge.go deleted |
| L8.4 | 🟢 | 🟢 | 🟡 | Investigation tracking (gap: unit tests) |
| L8.5 | 🟢 | 🟢 | 🟡 | Recon install prompt -- "Run aoa init" (gap: browser-only) |
| L8.6 | 🟡 | ⚪ | ⚪ | Recon source line editor view |
| L10.1 | 🟢 | 🟢 | 🟡 | Core build tier (gap: no automated test) |
| L10.2 | 🟢 | 🟢 | 🟡 | Grammar paths wired into parser (gap: no automated test) |
| L10.3 | 🟢 | 🟢 | 🟢 | No outbound network -- CI grep-enforced. **TRIPLE-GREEN** |
| L10.4 | 🟢 | 🟢 | 🟢 | Grammar build script -- weekly CI validates cross-platform. **TRIPLE-GREEN** |
| L10.7 | 🟢 | 🟢 | 🟡 | deploy.sh updated (gap: not tested on fresh machine) |

## Blocked Tasks

None. L10.8 superseded by L4.4 Phase 3. L10.9 absorbed into L4.4 Phase 4.4.

## Board Pointers

Line ranges into BOARD.md for targeted reads:

| Section | Lines | Range |
|---------|-------|-------|
| Header | 1-7 | `offset=1, limit=7` |
| Goals | 11-24 | `offset=11, limit=14` |
| Board Structure | 27-70 | `offset=27, limit=44` |
| Mission | 76-84 | `offset=76, limit=9` |
| Board Table | 86-117 | `offset=86, limit=32` |
| Supporting Detail | 120-534 | `offset=120, limit=415` |
| - Layer 2 | 128-144 | `offset=128, limit=17` |
| - Layer 4 | 146-317 | `offset=146, limit=172` |
| - Layer 5 | 320-365 | `offset=320, limit=46` |
| - Layer 7 | 368-397 | `offset=368, limit=30` |
| - Layer 8 | 400-472 | `offset=400, limit=73` |
| - Layer 10 | 474-533 | `offset=474, limit=60` |
| What Works | 536-559 | `offset=536, limit=24` |
| What We're NOT Doing | 562-569 | `offset=562, limit=8` |
| Key Documents | 571-583 | `offset=571, limit=13` |
| Quick Reference | 585-597 | `offset=585, limit=13` |

## Layer Status

| Layer | Total | Done | Open | Status |
|-------|-------|------|------|--------|
| P0 | 7 | 7 | 0 | Complete -- archived to COMPLETED.md |
| L0 | 12 | 12 | 0 | Complete |
| L1 | 8 | 8 | 0 | Complete |
| L2 | 7 | 7 | 0 | Complete -- all archived to COMPLETED.md |
| L3 | 15 | 15 | 0 | Complete -- L3.15 archived to COMPLETED.md |
| L4 | 4 | 2 | 1 | L4.4 in progress -- **Phase 1+2 COMPLETE. Phase 3 ~95% (S84)**: L4.4-3.1 through 3.5 done, L4.4-3.6 (fresh machine e2e) remains. L4.2 superseded -> BACKLOG.md |
| L5 | 15 | 10 | 3 | Active -- L5.Va (consolidated validation), L5.10/11 not started. L5.19 superseded. |
| L6 | 9 | 9 | 0 | Complete -- all archived to COMPLETED.md. **Superseded by L10.** |
| L7 | 3 | 3 | 0 | Complete -- L7.1 (Va gap), L7.2/L7.4 archived to COMPLETED.md |
| G0 | 1 | 1 | 0 | Complete -- G0.HF1 archived to COMPLETED.md |
| L8 | 6 | 2 | 4 | Recon -- L8.1 absorbed into L5.Va. L8.2-5 green (Va gaps), L8.6 not started. S81: aoa-recon removed |
| L9 | 9 | 9 | 0 | Complete -- archived to COMPLETED.md |
| L10 | 9 | 6 | 3 | Dynamic Grammar Distribution. L10.3/L10.4/L10.5/L10.6 triple-green (L10.5/L10.6 archived). L10.1/L10.2/L10.7 green (Va gaps). **L10.8 superseded by L4.4 Phase 3. L10.9 absorbed into L4.4 Phase 4.4.** |

## Active Documents

| Task | Type | Document | Status |
|------|------|----------|--------|
| L5.7-L5.19 | Reference | `details/2026-02-23-dimensional-taxonomy.md` | Complete -- 142 questions across 21 dimensions |
| L5.7-L5.19 | ADR | `decisions/2026-02-23-declarative-yaml-rules.md` | Accepted -- declarative YAML schema, 6 constraints |
| -- | detail | `details/2026-02-25-session-71-g0-perf-and-recon-separation.md` | Complete -- G0 perf fixes, recon separation, P0 closure |
| -- | detail | `details/QOL.md` | In progress -- dashboard QOL (Live, Intel, Arsenal done; Recon remains). Session 78: Arsenal v2 split-bar chart, per-model token breakdown, session history redesign. Split-bar ported to live dashboard, chart layout fixes, Learning Curve cost-per-prompt fix. Session 78 continued: chart card 340px row height, footer symmetry, 3-session smoothed cost-per-prompt |
| -- | mockup | `docs/mockups/arsenal-v2.html` | Complete -- split-bar design ported to live dashboard |
| -- | detail | `test/testdata/regression-2026-02-27.md` | Complete -- full regression baseline: 535 pass, 0 fail, 0 skip across 7 phases |
| -- | detail | `test/README.md` | Updated -- test pipeline docs: 8-phase execution order, where tests live, how to run |
| -- | detail | `details/2026-02-26-throughput-telemetry-model.md` | Complete -- telemetry data hierarchy, throughput/conv speed formulas, phased roadmap |
| -- | detail | `docs/research/dashboard-metrics.md` | Updated -- 138 lines, L9 data sources, shadow ring metrics, 17 inferrable metrics |

## Key Files

| File | Purpose |
|------|---------|
| `.context/BOARD.md` | Source of truth -- task table + supporting detail |
| `.context/CURRENT.md` | Session checklist |
| `.context/COMPLETED.md` | Archived completed work |
| `.context/BACKLOG.md` | Deferred items |
| `CLAUDE.md` | Agent instructions, architecture, build commands |
| `SECURITY.md` | Trust document -- no telemetry, no outbound, localhost-only, security scan results |
| `build.sh` | ONLY build entry point -- default (tree-sitter + dynamic grammars), --light (pure Go, 8MB) |
| `deploy.sh` | Single-command deploy: build -> graceful stop -> clean socket -> start |
| `scripts/build-grammars.sh` | Compile grammar .so/.dylib from go-sitter-forest C source |
| `.github/workflows/security.yml` | CI security scan: govulncheck + gosec + network audit |
| `.github/workflows/grammar-validation.yml` | Weekly grammar validation: 509 grammars, 4 platforms, parsers.json |
| `Makefile` | All targets go through build.sh -- build, check, bench-gauntlet, bench-baseline, bench-compare |
