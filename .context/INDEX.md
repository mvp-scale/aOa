# Index

> **Updated**: 2026-02-23 (Session 66)

## Active Layer

**L5** (Dimensional Analysis) and **L7** (Onboarding UX) -- parallel work. L5 has completed recon cache/investigation/dashboard but dimensional tiers and query support remain. L7 has startup feedback done but DB optimization, editor view, and .aoa/ restructure are open.

## Unblocked Tasks

Tasks with no blocking dependencies (or all deps satisfied):

| ID | Cf | St | Va | Task |
|----|:--:|:--:|:--:|------|
| L2.1 | 🟢 | 🟢 | 🟡 | Wire file watcher (gap: integration test) |
| L3.15 | 🟢 | 🟢 | 🟡 | GNU grep native parity (gap: parity test suite) |
| L4.2 | 🟡 | 🟢 | 🟡 | Grammar CLI (gap: download not implemented) |
| L4.4 | 🟢 | ⚪ | ⚪ | Installation docs |
| L5.7 | 🟡 | ⚪ | ⚪ | Performance tier rules |
| L5.8 | 🟡 | ⚪ | ⚪ | Quality tier rules |
| L5.10 | 🟢 | ⚪ | ⚪ | Dimension scores in search results |
| L5.11 | 🟢 | ⚪ | ⚪ | Dimension query support |
| L5.12 | 🟢 | 🟢 | 🟡 | Recon tab (gap: bitmask upgrade) |
| L5.13 | 🟢 | 🟢 | 🟡 | Recon dashboard overhaul (gap: browser-only) |
| L5.14 | 🟢 | 🟢 | 🟡 | Recon cache + incremental (gap: unit tests) |
| L5.15 | 🟢 | 🟢 | 🟡 | Investigation tracking (gap: unit tests) |
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
| Board Table | 83-110 | `offset=83, limit=28` |
| Supporting Detail | 113-345 | `offset=113, limit=233` |
| - Layer 2 | 115-130 | `offset=115, limit=16` |
| - Layer 3 | 133-144 | `offset=133, limit=12` |
| - Layer 4 | 147-168 | `offset=147, limit=22` |
| - Layer 5 | 171-250 | `offset=171, limit=80` |
| - Layer 6 | 253-282 | `offset=253, limit=30` |
| - Layer 7 | 285-345 | `offset=285, limit=61` |
| What Works | 348-363 | `offset=348, limit=16` |
| What We're NOT Doing | 367-374 | `offset=367, limit=8` |
| Key Documents | 376-384 | `offset=376, limit=9` |
| Quick Reference | 386-398 | `offset=386, limit=13` |

## Layer Status

| Layer | Total | Done | Open | Status |
|-------|-------|------|------|--------|
| L0 | 12 | 12 | 0 | Complete |
| L1 | 8 | 8 | 0 | Complete |
| L2 | 7 | 6 | 1 | L2.1 validation gap |
| L3 | 14 | 13 | 1 | L3.15 validation gap |
| L4 | 4 | 2 | 2 | L4.2 partial, L4.4 not started |
| L5 | 15 | 7 | 8 | Active -- recon done, tiers/query open |
| L6 | 10 | 6 | 4 | L6.7 done, npm/CI validation gaps |
| L7 | 4 | 1 | 3 | Active -- startup done, 3 open |

## Active Documents

| Task | Type | Document | Status |
|------|------|----------|--------|
| -- | -- | (none yet) | -- |

## Key Files

| File | Purpose |
|------|---------|
| `.context/BOARD.md` | Source of truth -- task table + supporting detail |
| `.context/CURRENT.md` | Session checklist |
| `.context/COMPLETED.md` | Archived completed work |
| `.context/BACKLOG.md` | Deferred items |
| `CLAUDE.md` | Agent instructions, architecture, build commands |
| `Makefile` | build, build-pure, build-recon, check targets |
