# Index

> **Updated**: 2026-02-25 (Session 73)

## Active Layer

**L5** (Dimensional Analysis) -- YAML rework complete. Universal concept layer (15 concepts, 509 languages), declarative structural blocks, lang_map eliminated. L5.7/8/16-19 all green with per-rule validation gaps. L5.10/11 not started (dimension scores + query support). Walker expression_list fix shipped.

**L8** (Recon) -- New layer. Recon-specific tasks moved from L5/L6/L7. L8.1-L8.5 all green with Va gaps. L8.6 not started.

**G0** (Speed) -- Two critical violations found and fixed in Session 71. Regex search now uses trigram extraction (5s->8ms). Symbol search gated on metadata presence (186ms->4us). Full gauntlet all sub-25ms. 22-shape automated regression suite (`test/gauntlet_test.go`) with benchstat baselines prevents future regressions. Session 72: L7.2 binary encoding shipped and archived (964MB->~50MB bbolt, 28.7s->sub-second load, 20x smaller token storage). AOA_SHIM=1 env var added for explicit Unix shim mode, fixing 3 grep/egrep shim bugs. Shim scripts in init.go fixed to include `export AOA_SHIM=1`.

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
| L8.1 | 🟢 | 🟢 | 🟡 | Recon tab (gap: bitmask upgrade) |
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
| Board Table | 83-110 | `offset=83, limit=28` |
| Supporting Detail | 113-372 | `offset=113, limit=260` |
| - Layer 2 | 115-131 | `offset=115, limit=17` |
| - Layer 4 | 133-145 | `offset=133, limit=13` |
| - Layer 5 | 147-291 | `offset=147, limit=145` |
| - Layer 6 | 294-308 | `offset=294, limit=15` |
| - Layer 7 | 310-340 | `offset=310, limit=31` |
| - Layer 8 | 342-420 | `offset=342, limit=79` |
| What Works | 422-440 | `offset=422, limit=19` |
| What We're NOT Doing | 396-403 | `offset=396, limit=8` |
| Key Documents | 405-415 | `offset=405, limit=11` |
| Quick Reference | 417-429 | `offset=417, limit=13` |

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
| L8 | 6 | 5 | 1 | Recon -- L8.1-5 green (Va gaps), L8.6 not started |

## Active Documents

| Task | Type | Document | Status |
|------|------|----------|--------|
| L5.7-L5.19 | Reference | `details/2026-02-23-dimensional-taxonomy.md` | Complete -- 142 questions across 21 dimensions |
| L5.7-L5.19 | ADR | `decisions/2026-02-23-declarative-yaml-rules.md` | Accepted -- declarative YAML schema, 6 constraints |
| -- | detail | `details/2026-02-25-session-71-g0-perf-and-recon-separation.md` | Complete -- G0 perf fixes, recon separation, P0 closure |

## Key Files

| File | Purpose |
|------|---------|
| `.context/BOARD.md` | Source of truth -- task table + supporting detail |
| `.context/CURRENT.md` | Session checklist |
| `.context/COMPLETED.md` | Archived completed work |
| `.context/BACKLOG.md` | Deferred items |
| `CLAUDE.md` | Agent instructions, architecture, build commands |
| `Makefile` | build, build-pure, build-recon, check, bench-gauntlet, bench-baseline, bench-compare targets |
