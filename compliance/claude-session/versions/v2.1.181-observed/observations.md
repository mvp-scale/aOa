# v2.1.181 — Observed Drift vs v2.1.178

Captured 2026-06-18 by the generative conformance harness
(`compliance/conformance/run.sh --all`), which drove 5 controlled `claude -p`
sessions on the installed version 2.1.181 (minimal, bash-tool, edit-tool,
iterations, agent) — all `version="2.1.181"`.

## Headline: no drift

**v2.1.178 → v2.1.181 is a pure version bump for the session surface.**
`breaking = false`, `additive = false`. Across the generated sessions:

- **No new event type** — enumerated `assistant`, `user`, `attachment`,
  `ai-title`, `last-prompt`, `queue-operation` (same set; `system` /
  `permission-mode` / `mode` not triggered headless, carried forward).
- **No new / removed / renamed envelope field** — every field present is
  already catalogued (`slug`, `promptSource`, `agentId`, `attributionAgent`,
  `interruptedMessageId`, `pendingWorkflowCount`, …). Every **CONSUMED** field
  (`type`, `uuid`, `timestamp`, `version`, `sessionId`, `message`, …) is present
  and correctly typed.
- **No new usage field** — same 10 (`input_tokens`, `output_tokens`,
  `cache_read_input_tokens`, `cache_creation_input_tokens`, `cache_creation`,
  `server_tool_use`, `service_tier`, `inference_geo`, `iterations`, `speed`).
- **No new content block** — `text`, `thinking`, `tool_use`, `tool_result`.
- **No new toolUseResult shape** — `Bash`, `Edit`, `Edit(create)`, `Agent`
  (with `resolvedModel`) all present and unchanged.

## Closed-loop checks (harness)

- `usage_match` ✓ EXACT — the `claude -p` result block usage equalled the
  in-file `message.usage` (`{i:4221, o:5, cr:14111, cc:1767}`).
- `iterations` empty in single-inference turns (unchanged, `[DROPPED]`).
- `agent_tokens` WARN (376× under) — this is the harness's own `char/4`
  subagent estimate, **not** a contract issue and **not** the structured-field
  path; the subagent-attribution leak is tracked under L18 (see
  `v2.1.178-observed/observations.md` §4), independent of schema alignment.

## Surfaces not re-observed at 181 (carried forward, inferred)

`system` + subtypes (`turn_duration`/`away_summary`/`local_command`),
`permission-mode`, `mode`, `queue-operation` `enqueue.content`, and the
`ToolSearch`/`TaskCreate`/`TaskUpdate` toolUseResult shapes are not driveable by
the headless harness; their shapes carry forward from v2.1.178 unchanged.

## Disposition

Nothing to classify — no new signal appeared. The snapshot is a clean re-stamp
of v2.1.178 at 2.1.181, re-pointing the baseline so the compliance suite
certifies against the installed version. No `CONTRACT.md` field rows change;
only the header version/date.
