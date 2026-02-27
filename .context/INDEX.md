# Index

> **Updated**: 2026-02-26 (Session 76)

## Active Layer

**L5** (Dimensional Analysis) -- YAML rework complete. L5.Va consolidates all per-rule validation (was L5.7/8/16/17/18 + L8.1) into one task. L5.19 superseded (archived). L5.10/11 not started (dimension scores + query support).

**L8** (Recon) -- L8.1 absorbed into L5.Va. L8.2-L8.5 green with Va gaps (browser-only / unit tests). L8.6 not started.

**L9** (Telemetry) -- **Complete, archived.** All 9 tasks triple-green. See COMPLETED.md.

**G0** (Speed) -- Two critical violations found and fixed in Session 71. Regex search now uses trigram extraction (5s->8ms). Symbol search gated on metadata presence (186ms->4us). Full gauntlet all sub-25ms. 22-shape automated regression suite (`test/gauntlet_test.go`) with benchstat baselines prevents future regressions. Session 72: L7.2 binary encoding shipped and archived (964MB->~50MB bbolt, 28.7s->sub-second load, 20x smaller token storage). AOA_SHIM=1 env var added for explicit Unix shim mode, fixing 3 grep/egrep shim bugs. Shim scripts in init.go fixed to include `export AOA_SHIM=1`. Session 73: Build process hotfix -- `build.sh` as sole entry point, compile-time build guard, `recon` build tag opt-in, binary 366MB->8MB.

## Unblocked Tasks

Tasks with no blocking dependencies (or all deps satisfied):

| ID | Cf | St | Va | Task |
|----|:--:|:--:|:--:|------|
| L4.4 | 🟢 | ⚪ | ⚪ | Installation docs |
| L5.Va | 🟢 | 🟢 | 🟡 | Dimensional rule validation -- all 5 tiers (absorbs L5.7/8/16/17/18 + L8.1) |
| L5.10 | 🟢 | ⚪ | ⚪ | Dimension scores in search results |
| L5.11 | 🟢 | ⚪ | ⚪ | Dimension query support |
| L7.1 | 🟢 | 🟢 | 🟡 | Startup progress (gap: timing test) |
| L7.4 | 🟢 | 🟢 | 🟢 | .aoa/ directory restructure -- COMPLETE |
| G0.HF1 | 🟢 | 🟢 | 🟢 | Build process fix (hotfix) -- COMPLETE |
| L8.2 | 🟢 | 🟢 | 🟡 | Recon dashboard overhaul (gap: browser-only) |
| L8.3 | 🟢 | 🟢 | 🟡 | Recon cache + incremental (gap: unit tests) |
| L8.4 | 🟢 | 🟢 | 🟡 | Investigation tracking (gap: unit tests) |
| L8.5 | 🟢 | 🟢 | 🟡 | Recon install prompt (gap: browser-only) |
| L8.6 | 🟡 | ⚪ | ⚪ | Recon source line editor view |

## Blocked Tasks

No tasks are currently blocked.

## Board Pointers

Line ranges into BOARD.md for targeted reads:

| Section | Lines | Range |
|---------|-------|-------|
| Header | 1-7 | `offset=1, limit=7` |
| Goals | 11-23 | `offset=11, limit=13` |
| Board Structure | 27-69 | `offset=27, limit=43` |
| Mission | 73-79 | `offset=73, limit=7` |
| Board Table | 83-108 | `offset=83, limit=26` |
| Supporting Detail | 111-300 | `offset=111, limit=190` |
| - Layer 2 | 113-129 | `offset=113, limit=17` |
| - Layer 4 | 131-143 | `offset=131, limit=13` |
| - Layer 5 | 145-189 | `offset=145, limit=45` |
| - Layer 7 | 191-244 | `offset=191, limit=54` |
| - Layer 8 | 246-300 | `offset=246, limit=55` |
| What Works | 303-325 | `offset=303, limit=23` |
| What We're NOT Doing | 327-335 | `offset=327, limit=9` |
| Key Documents | 336-348 | `offset=336, limit=13` |
| Quick Reference | 349-361 | `offset=349, limit=13` |

## Layer Status

| Layer | Total | Done | Open | Status |
|-------|-------|------|------|--------|
| P0 | 7 | 7 | 0 | Complete -- archived to COMPLETED.md |
| L0 | 12 | 12 | 0 | Complete |
| L1 | 8 | 8 | 0 | Complete |
| L2 | 7 | 7 | 0 | Complete -- all archived to COMPLETED.md |
| L3 | 15 | 15 | 0 | Complete -- L3.15 archived to COMPLETED.md |
| L4 | 4 | 2 | 1 | L4.4 not started. L4.2 superseded -> BACKLOG.md |
| L5 | 15 | 10 | 3 | Active -- L5.Va (consolidated validation), L5.10/11 not started. L5.19 superseded. |
| L6 | 9 | 9 | 0 | Complete -- all archived to COMPLETED.md |
| L7 | 3 | 3 | 0 | Complete -- L7.1 (Va gap), L7.2 archived, L7.4 triple-green |
| G0 | 1 | 1 | 0 | Complete -- G0.HF1 build process hotfix (triple-green) |
| L8 | 6 | 2 | 4 | Recon -- L8.1 absorbed into L5.Va. L8.2-5 green (Va gaps), L8.6 not started |
| L9 | 9 | 9 | 0 | Complete -- archived to COMPLETED.md |

## Active Documents

| Task | Type | Document | Status |
|------|------|----------|--------|
| L5.7-L5.19 | Reference | `details/2026-02-23-dimensional-taxonomy.md` | Complete -- 142 questions across 21 dimensions |
| L5.7-L5.19 | ADR | `decisions/2026-02-23-declarative-yaml-rules.md` | Accepted -- declarative YAML schema, 6 constraints |
| -- | detail | `details/2026-02-25-session-71-g0-perf-and-recon-separation.md` | Complete -- G0 perf fixes, recon separation, P0 closure |
| -- | detail | `details/QOL.md` | In progress -- dashboard QOL feedback (Live tab started) |
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
| `build.sh` | ONLY build entry point -- standard (pure Go, 8MB) and recon (CGo, full) builds |
| `deploy.sh` | Single-command deploy: build -> graceful stop -> clean socket -> start |
| `Makefile` | All targets go through build.sh -- build, build-recon, check, bench-gauntlet, bench-baseline, bench-compare |
