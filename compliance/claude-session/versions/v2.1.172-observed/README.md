# v2.1.172 — Observed Contract

**Status:** observed (validated against live session data)
**Source:** live capture from `~/.claude/projects/-home-corey-aOa-go/` (session
`404204ee`), supplemented by adjacent v2.1.170 sessions for the
subagent/toolUseResult surface.
**Captured:** 2026-06-11
**Sample size:** 217 events (100 assistant, 52 user, 12 permission-mode,
12 mode, 12 ai-title, 11 last-prompt, 10 attachment, 6 queue-operation,
2 system)

## What this is

The shape of Claude Code's session JSONL as actually emitted by version
**2.1.172**, captured by enumeration over a live session. This is the new
source of truth for the compliance check, replacing `v2.1.126-observed` as the
baseline. Re-observation closes the 46-version gap that L20 exists to catch.

Only the schema shape (event types, field names, value types) is captured into
`manifest.json` — no session payload is persisted.

## Files in this folder

| File              | Contents                                                                  |
|-------------------|---------------------------------------------------------------------------|
| `README.md`       | this file                                                                 |
| `manifest.json`   | machine-readable schema snapshot (types, fields, content blocks, drift)   |
| `observations.md` | human-readable drift report — what's NEW vs `v2.1.126-observed/`           |

## Headline

Additive drift only — no breaking change. Seven new surfaces (2 event types,
2 envelope fields, 1 system subtype, 1 usage-semantics change, 3 toolUseResult
shapes). The only item with material downstream value is `usage.iterations`
becoming populated, routed to L18 token accuracy. See `observations.md`.

## How to refresh

See `compliance/RUNBOOK.md` for the repeatable realignment procedure (L20.2).
