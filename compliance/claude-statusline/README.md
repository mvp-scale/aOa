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
ephemeral and only renders in the **interactive** TUI — `claude -p` (headless)
emits no status line at all. So a sample can only be captured by driving an
**ephemeral interactive `claude` in a pseudo-terminal**, letting it complete one
turn so the render is **mature** (post-API-response, with the token/cost fields
populated), then reading what Claude pipes to the hook.

### Automated (recommended)

```bash
compliance/conformance/capture-statusline.sh
```

This drives the installed `claude` in a PTY and writes
`versions/v<installed-version>-observed/sample.json`. It is self-contained:

- **Hook capture** is OFF unless armed — the hook only writes a sample when
  `.aoa/.capture-statusline` exists (a version string in that file gates the
  capture to one version; empty = any). The script arms and disarms it.
- **Maturity gate** — it polls until the captured render has completed a turn
  (`current_usage != null`, `cost.total_cost_usd` present, `total_output_tokens
  > 0`). An immature pre-API-response sample would false-flag conditional fields
  as breaking in Pass4, so the script refuses to write one.
- **No pollution** — the throwaway session it creates is deleted on exit (so the
  daemon doesn't ingest it). The capture grabs stdin *before* claude is stopped,
  and the session file is discarded, so there is nothing to flush — no SIGKILL
  hazard. If the deployed hook predates the capture block, the script
  temp-patches it and reverts on exit (`aoa init` makes it permanent).

Flag notes: `--bare` is **not** usable (it skips hooks, including this one);
`--no-session-persistence` only applies to `-p`, not the interactive mode the
status line needs.

After it writes `sample.json`, author the sibling `manifest.json` (Pass4 needs
the version dir to have a manifest) and re-run the compliance test.

### Manual fallback

If you can't run the script (no `script`/PTY), arm the hook by hand:
`echo "<version>" > .aoa/.capture-statusline`, send a prompt in Claude Code,
wait for a render, then `mv .aoa/statusline-capture.json
versions/v<version>-observed/sample.json` and `rm .aoa/.capture-statusline`.

The compliance test treats the sample file as optional — if it's absent,
`Pass4` skips (reduced fidelity).
