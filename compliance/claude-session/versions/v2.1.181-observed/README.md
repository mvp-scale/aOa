# v2.1.181 — Observed Contract

**Status:** observed (validated against generated session data)
**Source:** generative conformance harness (`compliance/conformance/run.sh
--all`) over installed version 2.1.181 — 5 controlled `claude -p` sessions
(minimal, bash-tool, edit-tool, iterations, agent), all `version="2.1.181"`.
**Captured:** 2026-06-18

## What this is

The shape of Claude Code's session JSONL as emitted by version **2.1.181**,
captured by generating controlled sessions and enumerating them. This is the new
baseline for the compliance check, replacing `v2.1.178-observed`.

Only the schema shape (event types, field names, value types) is captured into
`manifest.json` — no session payload is persisted.

## Files in this folder

| File              | Contents                                                                  |
|-------------------|---------------------------------------------------------------------------|
| `README.md`       | this file                                                                 |
| `manifest.json`   | machine-readable schema snapshot (types, fields, content blocks, drift)   |
| `observations.md` | human-readable drift report — what's NEW vs `v2.1.178-observed/`           |

## Headline

**No drift.** v2.1.178 → v2.1.181 is a pure version bump for the session
surface: no new/removed/renamed event type, envelope field, usage field, content
block, or toolUseResult shape, and every CONSUMED field present and correctly
typed (`breaking = false`, `additive = false`). The snapshot re-points the
baseline so the suite certifies against the installed version. See
`observations.md`.

## How to refresh

See `compliance/RUNBOOK.md` for the repeatable realignment procedure (L20.2).
