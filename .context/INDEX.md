# Index

> **Updated**: 2026-02-28 (Session 82)

## Active Layer

**L4** (Distribution) -- L4.4 in progress -- 6-phase roadmap. **Phase 1 COMPLETE (S80)**, **Phase 2 COMPLETE (S81)**: parsers.json 509/509 provenance, GRAMMAR_REPORT.md, weekly CI, 346 contributors. L4.2 superseded -> BACKLOG.md. **Next: Phase 3** (grammar release -- pre-built .so on GitHub).

**L5** (Dimensional Analysis) -- YAML rework complete. L5.Va consolidates all per-rule validation (was L5.7/8/16/17/18 + L8.1) into one task. L5.19 superseded (archived). L5.10/11 not started (dimension scores + query support).

**L8** (Recon) -- L8.1 absorbed into L5.Va. L8.2-L8.5 green with Va gaps (browser-only / unit tests). L8.5 updated: "Run aoa init" instead of "npm install aoa-recon". L8.6 not started. **S81**: aoa-recon removed (-761 LOC), recon_bridge.go deleted.

**L9** (Telemetry) -- **Complete, archived.** All 9 tasks triple-green. See COMPLETED.md.

**L10** (Dynamic Grammar Distribution) -- Single-binary architecture replacing two-binary model. L10.3/L10.4/L10.5/L10.6 triple-green (L10.5/L10.6 archived). L10.1/L10.2/L10.7 green (Va gaps). L10.8 now unblocked (L10.4 complete). L10.9 blocked on L10.8.

**G0** (Speed) -- G0.HF1 triple-green, archived to COMPLETED.md.

## Unblocked Tasks

Tasks with no blocking dependencies (or all deps satisfied):

| ID | Cf | St | Va | Task |
|----|:--:|:--:|:--:|------|
| L4.4 | 🟢 | 🟡 | ⚪ | Installation docs + grammar pipeline -- 6-phase roadmap. **Phase 1+2 DONE**. **Next: Phase 3** (grammar release -- pre-built .so on GitHub) |
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
| L10.8 | 🟢 | ⚪ | ⚪ | Build all 509 grammars + GitHub release (**NEWLY UNBLOCKED** -- L10.4 complete) |

## Blocked Tasks

| ID | Cf | St | Va | Task | Blocked By |
|----|:--:|:--:|:--:|------|------------|
| L10.9 | 🟢 | ⚪ | ⚪ | End-to-end test on fresh project | L10.8 (needs downloadable grammars) |

## Board Pointers

Line ranges into BOARD.md for targeted reads:

| Section | Lines | Range |
|---------|-------|-------|
| Header | 1-7 | `offset=1, limit=7` |
| Goals | 11-24 | `offset=11, limit=14` |
| Board Structure | 27-70 | `offset=27, limit=44` |
| Mission | 76-84 | `offset=76, limit=9` |
| Board Table | 86-117 | `offset=86, limit=32` |
| Supporting Detail | 120-480 | `offset=120, limit=361` |
| - Layer 2 | 122-138 | `offset=122, limit=17` |
| - Layer 4 | 140-285 | `offset=140, limit=146` |
| - Layer 5 | 287-331 | `offset=287, limit=45` |
| - Layer 7 | 333-363 | `offset=333, limit=31` |
| - Layer 8 | 365-417 | `offset=365, limit=53` |
| - Layer 10 | 419-481 | `offset=419, limit=63` |
| What Works | 483-507 | `offset=483, limit=25` |
| What We're NOT Doing | 509-516 | `offset=509, limit=8` |
| Key Documents | 518-530 | `offset=518, limit=13` |
| Quick Reference | 532-544 | `offset=532, limit=13` |

## Layer Status

| Layer | Total | Done | Open | Status |
|-------|-------|------|------|--------|
| P0 | 7 | 7 | 0 | Complete -- archived to COMPLETED.md |
| L0 | 12 | 12 | 0 | Complete |
| L1 | 8 | 8 | 0 | Complete |
| L2 | 7 | 7 | 0 | Complete -- all archived to COMPLETED.md |
| L3 | 15 | 15 | 0 | Complete -- L3.15 archived to COMPLETED.md |
| L4 | 4 | 2 | 1 | L4.4 in progress -- **Phase 1+2 COMPLETE**. Phase 3 next (grammar release). L4.2 superseded -> BACKLOG.md |
| L5 | 15 | 10 | 3 | Active -- L5.Va (consolidated validation), L5.10/11 not started. L5.19 superseded. |
| L6 | 9 | 9 | 0 | Complete -- all archived to COMPLETED.md. **Superseded by L10.** |
| L7 | 3 | 3 | 0 | Complete -- L7.1 (Va gap), L7.2/L7.4 archived to COMPLETED.md |
| G0 | 1 | 1 | 0 | Complete -- G0.HF1 archived to COMPLETED.md |
| L8 | 6 | 2 | 4 | Recon -- L8.1 absorbed into L5.Va. L8.2-5 green (Va gaps), L8.6 not started. S81: aoa-recon removed |
| L9 | 9 | 9 | 0 | Complete -- archived to COMPLETED.md |
| L10 | 9 | 4 | 5 | Dynamic Grammar Distribution. L10.3/L10.4/L10.5/L10.6 triple-green (L10.5/L10.6 archived). L10.1/L10.2/L10.7 green (Va gaps). L10.8 unblocked. L10.9 blocked on L10.8. |

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
