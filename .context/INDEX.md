# Index

> **Updated**: 2026-02-24 (Session 69)

## Active Layer

**L5** (Dimensional Analysis) -- YAML rework complete. Universal concept layer (15 concepts, 509 languages), declarative structural blocks, lang_map eliminated. L5.16-L5.19 now green. L5.7/L5.8 still need rules for empty dimensions. L7 deferred.

## Unblocked Tasks

Tasks with no blocking dependencies (or all deps satisfied):

| ID | Cf | St | Va | Task |
|----|:--:|:--:|:--:|------|
| L2.1 | 🟢 | 🟢 | 🟡 | Wire file watcher (gap: integration test) |
| L3.15 | 🟢 | 🟢 | 🟡 | GNU grep native parity (gap: parity test suite) |
| L4.2 | 🟡 | 🟢 | 🟡 | Grammar CLI (gap: download not implemented) |
| L4.4 | 🟢 | ⚪ | ⚪ | Installation docs |
| L5.7 | 🟡 | 🔵 | 🟡 | Performance tier (YAML rework done, 3 dims need rules) |
| L5.8 | 🟡 | 🔵 | 🟡 | Quality tier (YAML rework done, 2 dims need rules) |
| L5.10 | 🟢 | ⚪ | ⚪ | Dimension scores in search results |
| L5.11 | 🟢 | ⚪ | ⚪ | Dimension query support |
| L5.12 | 🟢 | 🟢 | 🟡 | Recon tab (gap: bitmask upgrade) |
| L5.13 | 🟢 | 🟢 | 🟡 | Recon dashboard overhaul (gap: browser-only) |
| L5.14 | 🟢 | 🟢 | 🟡 | Recon cache + incremental (gap: unit tests) |
| L5.15 | 🟢 | 🟢 | 🟡 | Investigation tracking (gap: unit tests) |
| L5.16 | 🟢 | 🟢 | 🟡 | Security expansion (YAML rework complete, per-rule validation gap) |
| L5.17 | 🟢 | 🟢 | 🟡 | Architecture expansion (YAML rework complete, per-rule validation gap) |
| L5.18 | 🟢 | 🟢 | 🟡 | Observability expansion (YAML rework complete, per-rule validation gap) |
| L5.19 | 🟢 | 🟢 | 🟡 | Compliance tier (YAML rework complete, per-rule validation gap) |
| L6.7 | 🟢 | 🟢 | 🟡 | Recon install prompt (gap: browser-only) |
| L7.1 | 🟢 | 🟢 | 🟡 | Startup progress (gap: timing test) |
| L7.2 | 🟡 | ⚪ | ⚪ | Database storage optimization |
| L7.3 | 🟡 | ⚪ | ⚪ | Recon source line editor view |
| L7.4 | 🟢 | ⚪ | ⚪ | .aoa/ directory restructure |

## Blocked Tasks

| ID | Blocked By | Task |
|----|-----------|------|
| L6.8 | L6.2 (done) | npm package structure -- dep satisfied but unpublished |
| L6.9 | L6.4 (done), L6.8 | npm recon packages -- blocked on L6.8 publish |
| L6.10 | L6.8, L6.9 | CI/Release -- blocked on npm packages |

Note: L6.8's dependency L6.2 is complete, so L6.8 is technically unblocked for code work but the validation gap is "not yet published to npm." L6.9 and L6.10 chain from there.

## Board Pointers

Line ranges into BOARD.md for targeted reads:

| Section | Lines | Range |
|---------|-------|-------|
| Header | 1-7 | `offset=1, limit=7` |
| Goals | 11-23 | `offset=11, limit=13` |
| Board Structure | 27-69 | `offset=27, limit=43` |
| Mission | 73-79 | `offset=73, limit=7` |
| Board Table | 83-114 | `offset=83, limit=32` |
| Supporting Detail | 117-416 | `offset=117, limit=300` |
| - Layer 2 | 119-134 | `offset=119, limit=16` |
| - Layer 3 | 137-148 | `offset=137, limit=12` |
| - Layer 4 | 151-172 | `offset=151, limit=22` |
| - Layer 5 | 175-320 | `offset=175, limit=146` |
| - Layer 6 | 323-352 | `offset=323, limit=30` |
| - Layer 7 | 355-416 | `offset=355, limit=62` |
| What Works | 418-435 | `offset=418, limit=18` |
| What We're NOT Doing | 437-445 | `offset=437, limit=9` |
| Key Documents | 446-457 | `offset=446, limit=12` |
| Quick Reference | 458-470 | `offset=458, limit=13` |

## Layer Status

| Layer | Total | Done | Open | Status |
|-------|-------|------|------|--------|
| L0 | 12 | 12 | 0 | Complete |
| L1 | 8 | 8 | 0 | Complete |
| L2 | 7 | 6 | 1 | L2.1 validation gap |
| L3 | 14 | 13 | 1 | L3.15 validation gap |
| L4 | 4 | 2 | 2 | L4.2 partial, L4.4 not started |
| L5 | 19 | 7 | 12 | Active -- YAML rework complete, L5.16-19 green, L5.7/8 need dim rules, L5.10/11 not started |
| L6 | 10 | 6 | 4 | L6.7 done, npm/CI validation gaps |
| L7 | 4 | 1 | 3 | Deferred -- startup done, 3 open |

## Active Documents

| Task | Type | Document | Status |
|------|------|----------|--------|
| L5.7-L5.19 | Reference | `details/2026-02-23-dimensional-taxonomy.md` | Complete -- 142 questions across 21 dimensions |
| L5.7-L5.19 | ADR | `decisions/2026-02-23-declarative-yaml-rules.md` | Accepted -- declarative YAML schema, 6 constraints |

## Key Files

| File | Purpose |
|------|---------|
| `.context/BOARD.md` | Source of truth -- task table + supporting detail |
| `.context/CURRENT.md` | Session checklist |
| `.context/COMPLETED.md` | Archived completed work |
| `.context/BACKLOG.md` | Deferred items |
| `CLAUDE.md` | Agent instructions, architecture, build commands |
| `Makefile` | build, build-pure, build-recon, check targets |
