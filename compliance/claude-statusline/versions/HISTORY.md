# Claude Code Status Line — Version History

Narrative changelog of changes to the status line stdin schema, settings
shape, and env var requirements.

Confidence implied by phrasing: "added" / "shipped" = high; "appears to" /
"likely" = medium; "best guess" = low.

---

## v2.1.70-inferred → v2.1.126-observed

**Gap of unknown size.** The "v2.1.70" label is a placeholder for the era
during which the hook was written — no captured sample exists.

This transition is **stable on the consumed surface**. All 21 stdin paths
the hook reads are still populated at v2.1.126, and the
`.claude/settings.local.json` `statusLine` key shape is unchanged.
`CLAUDE_PROJECT_DIR` is still set by Claude Code before invocation.

### Verified stable

- All 21 consumed stdin paths still populated (cost, context_window, rate_limits, model, session_id, version, cwd)
- `statusLine.{type, command}` shape in `.claude/settings.local.json`
- `CLAUDE_PROJECT_DIR` env var continues to be set

### Unverified at v2.1.126

- Whether Claude Code added new stdin fields beyond what the hook reads.
  Plausible, given the session JSONL gained ~11 new envelope fields in
  the same era (`cwd`, `gitBranch`, `entrypoint`, `userType`,
  `isSidechain`, etc.). Some of those may also appear here.

### How to close the gap

Capture raw stdin via the procedure in `claude-statusline/README.md`,
drop into `versions/v2.1.126-observed/sample.json`, and re-run the test.

---

## Adding a new entry

1. Capture `versions/v{NEW}-observed/` (manifest.json, observations.md, ideally sample.json).
2. Append a `## v{prev}-observed → v{new}-observed` section here.
3. For each change, write 1-2 lines explaining the *what* and the *likely why*.
4. Keep entries terse; defer detail to `versions/v{X}-observed/observations.md`.
