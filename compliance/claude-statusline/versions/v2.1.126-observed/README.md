# v2.1.126 — Status Line Observed Contract

**Status:** partially observed
**Captured:** 2026-05-02
**Source confirmed:** `.aoa/hook/context.jsonl` (derived snapshot, not raw stdin)

## What this is

Confirmed contract for Claude Code v2.1.126 status line stdin. Unlike
session JSONL, the status line stdin is ephemeral — Claude Code pipes it
in once per refresh and the script's stdin closes.

**What is verified at this version**:

- Fields the hook reads via `jq` are populated correctly (the status line
  renders non-zero values for them — see `manifest.json`).
- `session_id`, `version`, `model.id` confirmed present from the
  derived snapshot in `.aoa/hook/context.jsonl`:

  ```json
  {"session_id":"9a910065-9172-4baa-8819-5f0f9695752a",
   "version":"2.1.126",
   "model":"claude-opus-4-7[1m]"}
  ```

**What is NOT yet verified**:

- The full set of fields Claude Code emits beyond what we read. To
  enumerate "fields available but dropped" requires a live raw-stdin
  capture (see `claude-statusline/README.md` "Capturing live stdin").
- Type/shape changes within fields we read.

## Files

| File              | Contents                                      |
|-------------------|-----------------------------------------------|
| `README.md`       | this file                                     |
| `manifest.json`   | machine-readable schema snapshot              |
| `observations.md` | drift report (limited until live capture)     |
| `sample.json`     | (not yet captured — placeholder)              |

## To enrich this snapshot

Follow the live-capture procedure in `claude-statusline/README.md`. Drop
the captured `sample.json` into this directory and re-run the test —
Pass 4 (live shape) will then enumerate any new fields.
