# v2.1.70 — Inferred Contract

**Status:** inferred (NOT validated)
**Source:** mechanically derived from `internal/adapters/{tailer,claude}/` source code
**Captured:** 2026-05-02

## What this is

A reconstruction of the Claude Code session-format contract that aOa's parser
was originally designed to handle, derived by reading what fields the parser
extracts and what shapes it expects. We never captured a live session sample
at Claude Code v2.1.70, so this is a reconstruction of *design intent*, not a
verified record of what 2.1.70 actually emitted.

The "v2.1.70" tag is a placeholder for "the era during which this parser was
written" — the actual Claude Code version range our parser was targeting is
not known with certainty.

## What this is NOT

- Not a record of observed behavior at v2.1.70
- Not a source of truth for compliance checks
- Not authoritative — `versions/v2.1.126-observed/` is the source of truth

## Why preserve it

Three reasons:

1. **Drift baseline.** Without a "before" picture, "after" has nothing to diff against. The drift between this and `v2.1.126-observed/` is the integration debt list.
2. **Design-intent record.** Documents what the parser was *meant* to handle, separate from what Claude Code actually emits today.
3. **Future debugging.** If a parser bug surfaces, "the parser assumed X, but Claude Code emits Y" is the kind of question this folder answers.

## Files in this folder

| File                | Contents                                                  |
|---------------------|-----------------------------------------------------------|
| `README.md`         | this file                                                 |
| `assumed-types.md`  | top-level `type` values the parser knows about            |
| `assumed-fields.md` | per-event-type field maps (envelope, message, tool input) |
