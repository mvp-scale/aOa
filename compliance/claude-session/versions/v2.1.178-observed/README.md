# v2.1.178 — Observed Contract

**Status:** observed (validated against generated + live session data)
**Source:** generative conformance harness (`compliance/conformance/run.sh
--all --keep`) over installed version 2.1.178 — 5 controlled `claude -p`
sessions (minimal, bash-tool, edit-tool, iterations, agent) + 1 subagent
stream, supplemented by the live v2.1.173 repo session for the
`system`/`permission-mode`/`mode` surfaces the headless harness cannot drive.
**Captured:** 2026-06-18
**Sample size:** 49 events across 6 files (all `version="2.1.178"`), plus
carried-forward v2.1.173 system surface. Per-type counts are not directly
comparable to the 217-event organic v2.1.172 sample — the harness generates
small controlled sessions per surface.

## What this is

The shape of Claude Code's session JSONL as actually emitted by version
**2.1.178**, captured by generating controlled sessions and enumerating them.
This is the new source of truth for the compliance check, replacing
`v2.1.172-observed` as the baseline. Re-observation closes the 6-version gap
that L20 exists to catch.

Only the schema shape (event types, field names, value types) is captured into
`manifest.json` — no session payload is persisted.

## Files in this folder

| File              | Contents                                                                  |
|-------------------|---------------------------------------------------------------------------|
| `README.md`       | this file                                                                 |
| `manifest.json`   | machine-readable schema snapshot (types, fields, content blocks, drift)   |
| `observations.md` | human-readable drift report — what's NEW vs `v2.1.172-observed/`           |

## Headline

Additive drift only — no breaking change (`breaking = false`). Two new envelope
fields (`agentId`, `attributionAgent`) tying events to subagents, one new Agent
toolUseResult key (`resolvedModel`), and a new Edit `type=create` shape — all
DROPPED or absorbed. The two material items are numeric, not structural: the
v2.1.172 `usage.iterations` "POPULATED" claim no longer holds (empty at
173/178), and the known subagent-token attribution leak reproduces (~414x
under) — the right number is captured and the wrong one is reported. Both route
to L18. Several surfaces (`system`/`permission-mode`/`mode`/`ToolSearch`/`Task*`)
were not driveable headless and are carried forward as inferred-unverified. See
`observations.md`.

## How to refresh

See `compliance/RUNBOOK.md` for the repeatable realignment procedure (L20.2).
