# v2.1.126 — Observed Contract

**Status:** observed (validated against live session data)
**Source:** live capture from `~/.claude/projects/-home-corey-aOa-go/`
**Captured:** 2026-05-02
**Sample size:** 1 active session JSONL (83 events: 36 assistant, 23 user, 5 system, 5 attachment, 5 ai-title, 5 permission-mode, 4 last-prompt)

## What this is

The shape of Claude Code's session JSONL as actually emitted by version
**2.1.126**. Captured by enumeration over a live session file. This is the
source of truth for the compliance check.

No session content was redacted into a file fixture — instead, only the
schema shape (event types, field names, value types) was captured into
`manifest.json`. Real session data may contain sensitive context, so we
keep the structural metadata and discard the payload.

## Files in this folder

| File              | Contents                                                                  |
|-------------------|---------------------------------------------------------------------------|
| `README.md`       | this file                                                                 |
| `manifest.json`   | machine-readable schema snapshot (types, fields, content blocks)          |
| `observations.md` | human-readable drift report — what's NEW vs `v2.1.70-inferred/`            |

## How to refresh

```bash
go test -tags compliance ./compliance/claude-session/ -run TestObservedSchema -update
```

(Once the test is wired with `-update` support — initial capture was manual.)
