# v2.1.70 — Status Line Inferred Contract

**Status:** inferred (NOT validated)
**Source:** mechanically extracted from `hooks/aoa-status-line.sh`
**Captured:** 2026-05-02

## What this is

The set of stdin JSON paths that `aoa-status-line.sh` reads. Derived by
grepping every `jq` invocation in the script. This is the contract aOa
was built against — there's no captured sample at v2.1.70, so we can't
validate it against real Claude Code output of that era.

The "v2.1.70" label is a placeholder for "the era during which this hook
was written" — actual Claude Code version range is unknown.

## Files

| File                | Contents                                          |
|---------------------|---------------------------------------------------|
| `README.md`         | this file                                         |
| `assumed-fields.md` | every stdin field path the hook reads             |
