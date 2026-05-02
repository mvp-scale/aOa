# aOa Integration Compliance

aOa integrates with Claude Code at several boundaries. Each one is an
**external contract**: Anthropic owns the schema, doesn't publish it, and
changes it across versions. When a contract drifts and we don't notice,
aOa silently degrades — events get dropped, status line fields go blank,
hook registration breaks.

This folder formalizes those contracts. Each subfolder covers one
integration surface, follows the same structure, and ships a runnable
compliance test.

## Integration surfaces

| # | Surface | Direction | Folder | Status |
|---|---------|-----------|--------|--------|
| 1 | Claude Code session JSONL (`~/.claude/projects/{S}/`) | Claude → aOa (we tail) | [`claude-session/`](claude-session/) | active |
| 2 | Status line stdin JSON + `.claude/settings.local.json` `statusLine` key + `CLAUDE_PROJECT_DIR` env var | Claude → aOa hook | [`claude-statusline/`](claude-statusline/) | active |

## Out of scope

These are *not* compliance concerns and live elsewhere:

- **`aoa peek/grep` output format** — we control both ends; spec lives in `cmd/aoa/cmd/init.go` (the CLAUDE.md guidance block) and is exercised by the search parity tests in `test/`.
- **CLAUDE.md guidance behavior** — we inject the `aoa-guidance` block; Claude reads it. The contract is *behavioral* (does the model follow grep→peek?), not schema. Validated via search-parity tests, not compliance tests.
- **Daemon socket protocol, bbolt schema, internal port interfaces** — internal to aOa.
- **Claude Code install path** (`~/.local/share/claude/versions/`) — already known to vary by install method (npm/homebrew/manual). The status-line script tolerates absence; documented as a known fragility in `claude-statusline/CONTRACT.md` rather than a real contract.

## Shared structure

Every subfolder follows the same shape:

```
{surface}/
  README.md                          ← purpose + how to run the test
  CONTRACT.md                        ← the spec, fields marked [CONSUMED]/[DROPPED]/[IGNORED]
  compliance_test.go                 ← three-pass runnable check (build tag `compliance`)
  versions/
    HISTORY.md                       ← narrative changelog: what changed and why
    v{X}-inferred/                   ← what aOa was DESIGNED for (mechanical from code)
    v{X}-observed/                   ← what version X actually emits (validated live)
```

## Run all compliance checks

```bash
go test -tags compliance ./compliance/... -v
```

Without the build tag, compliance tests are invisible to default `go test`.

## Adding a new surface

When aOa starts integrating with a new Claude Code feature (e.g., a hook
event, a new file Claude Code writes), create a new subfolder following the
shared structure and add it to the table above.
