# Index

> **Updated**: 2026-02-26 (Session 76)

## Active Layer

**L5** (Dimensional Analysis) -- YAML rework complete. Universal concept layer (15 concepts, 509 languages), declarative structural blocks, lang_map eliminated. L5.7/8/16-19 all green with per-rule validation gaps. L5.10/11 not started (dimension scores + query support). Walker expression_list fix shipped.

**L8** (Recon) -- Recon-specific tasks moved from L5/L6/L7. L8.1-L8.5 all green with Va gaps. L8.6 not started.

**L9** (Telemetry) -- **Complete.** All 9 tasks (L9.0-L9.8) triple-green. ContentMeter, tool detail capture, persisted results, subagent tailing, shadow engine, shim counterfactual, burst throughput, dashboard display. 14 new unit tests. L9.6 pivoted from bash parsing to shim-level counterfactual. Dashboard: hero row, 6-card live stats grid, version in footer.

**G0** (Speed) -- Two critical violations found and fixed in Session 71. Regex search now uses trigram extraction (5s->8ms). Symbol search gated on metadata presence (186ms->4us). Full gauntlet all sub-25ms. 22-shape automated regression suite (`test/gauntlet_test.go`) with benchstat baselines prevents future regressions. Session 72: L7.2 binary encoding shipped and archived (964MB->~50MB bbolt, 28.7s->sub-second load, 20x smaller token storage). AOA_SHIM=1 env var added for explicit Unix shim mode, fixing 3 grep/egrep shim bugs. Shim scripts in init.go fixed to include `export AOA_SHIM=1`. Session 73: Build process hotfix -- `build.sh` as sole entry point, compile-time build guard, `recon` build tag opt-in, binary 366MB->8MB.

## Unblocked Tasks

Tasks with no blocking dependencies (or all deps satisfied):

| ID | Cf | St | Va | Task |
|----|:--:|:--:|:--:|------|
| L4.4 | 🟢 | ⚪ | ⚪ | Installation docs |
| L5.7 | 🟢 | 🟢 | 🟡 | Performance tier (gap: per-rule detection validation) |
| L5.8 | 🟢 | 🟢 | 🟡 | Quality tier (gap: per-rule detection validation) |
| L5.10 | 🟢 | ⚪ | ⚪ | Dimension scores in search results |
| L5.11 | 🟢 | ⚪ | ⚪ | Dimension query support |
| L5.16 | 🟢 | 🟢 | 🟡 | Security expansion (YAML rework complete, per-rule validation gap) |
| L5.17 | 🟢 | 🟢 | 🟡 | Architecture expansion (YAML rework complete, per-rule validation gap) |
| L5.18 | 🟢 | 🟢 | 🟡 | Observability expansion (YAML rework complete, per-rule validation gap) |
| L5.19 | 🟢 | 🟢 | 🟡 | Compliance tier (pivoted/superseded, absorbed into security) |
| L7.1 | 🟢 | 🟢 | 🟡 | Startup progress (gap: timing test) |
| L7.4 | 🟢 | 🟢 | 🟢 | .aoa/ directory restructure -- COMPLETE |
| G0.HF1 | 🟢 | 🟢 | 🟢 | Build process fix (hotfix) -- build.sh entry point, build guard, 366MB->8MB -- COMPLETE |
| L8.1 | 🟢 | 🟢 | 🟡 | Recon tab (gap: bitmask upgrade) |
| L8.2 | 🟢 | 🟢 | 🟡 | Recon dashboard overhaul (gap: browser-only) |
| L8.3 | 🟢 | 🟢 | 🟡 | Recon cache + incremental (gap: unit tests) |
| L8.4 | 🟢 | 🟢 | 🟡 | Investigation tracking (gap: unit tests) |
| L8.5 | 🟢 | 🟢 | 🟡 | Recon install prompt (gap: browser-only) |
| L8.6 | 🟡 | ⚪ | ⚪ | Recon source line editor view |
| L9.0 | 🟢 | 🟢 | 🟡 | Inline tool result char capture -- COMPLETE (gap: no unit test) |
| L9.1 | 🟢 | 🟢 | 🟢 | ContentMeter struct -- COMPLETE |
| L9.2 | 🟢 | 🟢 | 🟢 | Tool call detail capture -- COMPLETE |
| L9.3 | 🟢 | 🟢 | 🟢 | Persisted tool result sizes -- COMPLETE |
| L9.4 | 🟢 | 🟢 | 🟢 | Subagent JSONL tailing -- COMPLETE |
| L9.5 | 🟢 | 🟢 | 🟢 | Counterfactual shadow engine -- COMPLETE |
| L9.6 | 🟢 | 🟢 | 🟢 | Shim counterfactual (pivoted) -- COMPLETE |
| L9.7 | 🟢 | 🟢 | 🟢 | Burst throughput & per-turn velocity -- COMPLETE |
| L9.8 | 🟢 | 🟢 | 🟢 | Dashboard shadow savings -- COMPLETE |

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
| Board Table | 83-123 | `offset=83, limit=41` |
| Supporting Detail | 124-479 | `offset=124, limit=356` |
| - Layer 2 | 128-142 | `offset=128, limit=15` |
| - Layer 4 | 146-156 | `offset=146, limit=11` |
| - Layer 5 | 160-263 | `offset=160, limit=104` |
| - Layer 7 | 267-318 | `offset=267, limit=52` |
| - Layer 8 | 322-382 | `offset=322, limit=61` |
| - Layer 9 | 386-478 | `offset=386, limit=93` |
| What Works | 481-503 | `offset=481, limit=23` |
| What We're NOT Doing | 505-513 | `offset=505, limit=9` |
| Key Documents | 514-526 | `offset=514, limit=13` |
| Quick Reference | 527-540 | `offset=527, limit=14` |

## Layer Status

| Layer | Total | Done | Open | Status |
|-------|-------|------|------|--------|
| P0 | 7 | 7 | 0 | Complete -- archived to COMPLETED.md |
| L0 | 12 | 12 | 0 | Complete |
| L1 | 8 | 8 | 0 | Complete |
| L2 | 7 | 7 | 0 | Complete -- all archived to COMPLETED.md |
| L3 | 15 | 15 | 0 | Complete -- L3.15 archived to COMPLETED.md |
| L4 | 4 | 2 | 1 | L4.4 not started. L4.2 superseded -> BACKLOG.md |
| L5 | 15 | 9 | 6 | Active -- L5.7/8/16-19 green (Va gaps), L5.10/11 not started |
| L6 | 9 | 9 | 0 | Complete -- all archived to COMPLETED.md |
| L7 | 3 | 3 | 0 | Complete -- L7.1 (Va gap), L7.2 archived, L7.4 triple-green |
| G0 | 1 | 1 | 0 | Complete -- G0.HF1 build process hotfix (triple-green) |
| L8 | 6 | 5 | 1 | Recon -- L8.1-5 green (Va gaps), L8.6 not started |
| L9 | 9 | 9 | 0 | Complete -- L9.0-L9.8 all green (L9.0 Va gap: no unit test, L9.1-L9.8 triple-green) |

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
| `Makefile` | All targets go through build.sh -- build, build-recon, check, bench-gauntlet, bench-baseline, bench-compare |
