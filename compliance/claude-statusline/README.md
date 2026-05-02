# Claude Code Status Line — Integration Compliance

Validates the integration contract between aOa and Claude Code's status
line system. Three things constitute this contract:

1. **stdin JSON schema** — Claude Code invokes our hook script and pipes a
   JSON object to stdin. Our script reads ~20 specific paths from it.
2. **`.claude/settings.local.json` `statusLine` key** — we write a
   `{type, command}` object into this file to register the hook.
3. **`CLAUDE_PROJECT_DIR` env var** — Claude Code sets it before invoking
   the hook; we read it as `$CLAUDE_PROJECT_DIR`.

## Why this is here

The status line stdin schema is undocumented and can change across Claude
Code releases. If a field is renamed or restructured, the status line
silently shows zeros — no error, no log. This compliance suite makes that
drift visible.

## Files

```
claude-statusline/
  README.md            ← you are here
  CONTRACT.md          ← spec: stdin fields, settings.json shape, env vars
  compliance_test.go   ← runnable check (build tag: compliance)
  versions/
    HISTORY.md
    v2.1.70-inferred/   ← mechanical extraction from aoa-status-line.sh
    v2.1.126-observed/ ← captured live from a real Claude Code invocation
```

## Run

```bash
go test -tags compliance ./compliance/claude-statusline/... -v
```

## Capturing live stdin (for refreshing v{X}-observed)

Unlike session JSONL which we passively tail, the status line stdin is
ephemeral — Claude Code pipes it to the script and the script's stdin
closes. To capture a known-good sample:

1. **Wrap the hook temporarily** by editing `.claude/settings.local.json`
   `statusLine.command` to:
   ```bash
   bash -c 'tee /tmp/aoa-statusline-stdin.json | bash "$CLAUDE_PROJECT_DIR/.aoa/hooks/aoa-status-line.sh"'
   ```
2. Trigger a status-line refresh in Claude Code (any prompt or wait for the next tick).
3. **Restore** the original command (`bash "$CLAUDE_PROJECT_DIR/.aoa/hooks/aoa-status-line.sh"`).
4. Move `/tmp/aoa-statusline-stdin.json` into `versions/v{X}-observed/sample.json`.
5. Re-run the compliance test — observed fields will be enumerated.

The compliance test treats the sample file as optional — if it's absent,
the test runs with reduced fidelity (skips the live-shape pass).
