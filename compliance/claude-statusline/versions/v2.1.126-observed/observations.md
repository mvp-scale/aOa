# Observations — Status Line at v2.1.126

## Summary

| Drift category                          | Status     |
|-----------------------------------------|------------|
| Required fields renamed/removed         | none observed |
| Required fields type-changed            | none observed |
| New stdin fields (additive)             | unknown — requires raw stdin capture |
| `statusLine` settings key renamed       | none observed |
| `CLAUDE_PROJECT_DIR` env var dropped    | none observed |

## What is verified

The 21 stdin paths the hook reads are still populated at v2.1.126. The
status line renders non-zero values for fields where data is present
(cost, context_window, rate_limits all functional in active sessions).

The `.aoa/hook/context.jsonl` derived snapshot confirms that
`session_id`, `version`, `model.id`, and the cost/context_window fields
are received and parsed correctly:

```json
{
  "ts": 1777737742,
  "ctx_used": 151783,
  "ctx_max": 1000000,
  "used_pct": 15,
  "remaining_pct": 85,
  "total_cost_usd": 4.939862,
  "total_duration_ms": 2588506,
  "total_api_duration_ms": 991431,
  "model": "claude-opus-4-7[1m]",
  "session_id": "9a910065-9172-4baa-8819-5f0f9695752a",
  "version": "2.1.126"
}
```

## What requires live capture to verify

- **Field-level additions** at the top level of stdin JSON
- **Sub-field additions** under `cost`, `context_window`, `model`, `rate_limits`
- **New top-level keys** Claude Code may have started emitting (e.g.,
  permission mode, attachment count, sidechain markers — all of which
  appeared in the session JSONL)

The session JSONL drift was largely additive (cwd, gitBranch, entrypoint,
userType, isSidechain). It is plausible the status line stdin has
similar additive drift we'd benefit from consuming.

## Recommended follow-up

1. **Capture raw stdin** following the procedure in `claude-statusline/README.md`. Adds the missing `sample.json` and unlocks Pass 4 (live-shape enumeration) in the test.
2. **Compare to documentation** if Anthropic publishes the status line schema (currently undocumented).
3. **Watch for `permissionMode` / `gitBranch` parity** — if those appeared in session JSONL at v2.1.126, they likely appear here too. Worth consuming once verified.
