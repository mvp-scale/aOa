# Claude Code Session Format — Version History

A narrative changelog of Claude Code's session JSONL format, with one or two
lines per change explaining what we think Claude Code did and why. The exact
release version where each item landed is unknown — we have one inferred
baseline and one observed snapshot, with an unbounded gap between them.

Confidence is implied by phrasing: "added", "shipped" = high; "appears to",
"likely" = medium; "best guess" = low. When in doubt, treat as low.

---

## v2.1.172-observed → v2.1.178-observed

**Observed 2026-06-18** via the generative conformance harness
(`conformance/run.sh --all`) over installed version 2.1.178 — 5 controlled
`claude -p` sessions + 1 subagent stream (49 events, all `version="2.1.178"`),
supplemented by the live v2.1.173 session for headless-undriveable surfaces.
Closes the 6-version gap. **Additive only** — no field removed, renamed, or
retyped (`breaking = false`). The parser still works unchanged; the two material
items are numeric, not structural, and both route to L18.

### New envelope fields

- **`agentId`** (added) — on `user`/`assistant`/`attachment`. Links a top-level
  event to a spawned subagent; value matches the subagent file shortid
  (`agent-{shortid}.jsonl`) and `.meta.json`. DROPPED. **The missing attribution
  link** the subagent-token leak needs — flagged for L18.
- **`attributionAgent`** (added) — on `assistant`. The subagent type that
  produced the event (`"general-purpose"`). Pairs with `agentId`. DROPPED.
- **`system.pendingWorkflowCount`** (v2.1.173) — per-session pending-workflow
  counter (`0`). Seen live at 173, not triggered by the 178 harness
  (inferred-unverified). DROPPED.

### New toolUseResult shapes (non-breaking)

- **`Agent.resolvedModel`** (added) — the concrete model the subagent ran on
  (`"claude-opus-4-8[1m]"`). 10-key Agent shape → 11. The Agent branch matches on
  `agentId`/`agentType`, so the extra key is harmlessly unread.
- **`Edit` `type="create"` shape** (added) — `{content, filePath, originalFile,
  structuredPatch, type, userModified}` for file creation; lacks
  `newString`/`oldString`. Still classifies as Edit; `content`/`type` unread.

### Usage semantics (route to L18)

- **`usage.iterations`** — **regression vs the v2.1.172 claim**. Recorded
  POPULATED at 172; EMPTY `[]` across all 178 harness events and the live 173
  session. Now conditionally-populated (multi-inference turns only). `[DROPPED]`,
  non-breaking — but the 172 "POPULATED" narrative is corrected.
- **Subagent token attribution leak** — the known soft spot reproduces at 178,
  ~414x under (est=56 vs `totalTokens`=8145 vs summed subagent usage=23177). The
  exact in-band `AgentTotalTokens` is parsed and forwarded but never read; the
  surfaced cost falls back to `char/4`. Attribution leak, not schema break. The
  new `agentId`/`attributionAgent` fields are the in-band link to fix it.

### Not observed at 178 (carried forward, inferred-unverified)

- `system` + subtypes (`turn_duration`/`away_summary`/`local_command`),
  `permission-mode`, `mode`, `queue-operation` `enqueue.content`,
  `ToolSearch`/`TaskCreate`/`TaskUpdate` shapes — not driveable headless. Add a
  slash-command + turn-boundary scenario to observe them at 178.

### Stable (no drift)

- Topology, file format, `user`/`assistant`/`system` structure, content block
  types, tool-input field names, usage field SET (10), message field set (9).
  Main-session 4-field token usage verified EXACT closed-loop
  (`{i:4083, o:5, cr:14111, cc:1767}`).

## v2.1.70-inferred → v2.1.126-observed

**Gap of unknown size.** The "v2.1.70" label is a placeholder — we don't have
a verified live capture from that era. Treat it as "what aOa was originally
designed to handle" rather than a real version baseline.

This transition is **purely additive** on the parts aOa consumes: no field
was removed, renamed, or had its type changed. All new surface area is
either new event types (silently dropped) or new fields on already-known
events (silently dropped). The parser still works; it just sees less than
it could.

### New top-level event types

- **`permission-mode`** (added) — emitted on permission-mode transitions
  (`default` / `acceptEdits` / `plan` / etc.). Fields: `permissionMode`,
  `sessionId`, `type`. No `uuid` or `timestamp` — a control-plane event,
  not a content event.
- **`attachment`** (added) — first-class envelope for file/image
  attachments. Has the full identity envelope (`uuid`, `timestamp`,
  `parentUuid`) and a payload field `attachment`. Likely added because
  attachments grew beyond what fits cleanly inside a `content` array.
- **`ai-title`** (added) — AI-generated session title. Visible in the
  desktop app sidebar. Pure UX feature; low value for the learner.
- **`last-prompt`** (added) — resume/checkpoint pointer. Carries
  `leafUuid` pointing back at the last user prompt. Likely tied to the
  session-resume feature for "continue from here" navigation.

### New top-level envelope fields (on `user`/`assistant`/`system`/`attachment`)

- **`cwd`, `gitBranch`, `entrypoint`, `userType`, `isSidechain`** (bulk-added)
  — looks like a single PR landing "track session context for every event."
  Probably driven by analytics + support debugging. `isSidechain` may
  overlap with our existing `Source: subagent` tagging — worth checking.
- **`requestId`** (added on `assistant`) — Anthropic API request ID.
  Almost certainly added for support correlation: a user reports a bug,
  support pulls the matching request from API logs.
- **`promptId`** (added on `user`) — prompt grouping ID. Multiple events
  likely share a `promptId` if they trace to the same prompt revision.
  Likely tied to "edit prompt and re-run" affordances.
- **`sourceToolAssistantUUID`** (added on `user`) — explicit user→assistant
  back-link for tool results. Before this we correlated via `tool_use_id`.
  Adding it suggests Anthropic decided the implicit linkage was fragile —
  extended thinking + tool batching probably surfaced ambiguity.
- **`toolUseResult`** (added on `user`) — **structured per-tool result at
  envelope level**, separate from inline `tool_result` content blocks.
  Tool-specific shape:
  - `Bash` → `{stdout, stderr, interrupted, isImage, noOutputExpected}`
  - `Edit` → `{filePath, newString, oldString, originalFile, replaceAll, structuredPatch, userModified}`
  - `Agent` → `{agentId, agentType, content, prompt, status, toolStats, totalDurationMs, totalTokens, totalToolUseCount, usage}`
  - other tools → `{file, type}`

  This is the most consequential drop. `userModified` on Edit, agent
  recaps, and bash interrupt flags are all signals our learner could use.
- **`messageCount`** (added on `system`) — per-session message count.
  Cheap analytics.
- **`permissionMode`** (added on `user`) — per-message permission mode.
  Pairs with the new `permission-mode` event type.

### New `message.usage` fields (assistant)

- **`cache_creation`** (added, nested object) — splits cache writes by TTL
  bucket: `{ephemeral_1h_input_tokens, ephemeral_5m_input_tokens}`. Almost
  certainly added when Anthropic shipped 1-hour caching alongside the
  existing 5-minute default. The flat `cache_creation_input_tokens`
  **still exists** as the sum — neither field is deprecated; they're
  parallel views.
- **`inference_geo`** (added) — inference region (e.g., `us-east-1`,
  `eu-west-1`). Empty string in local sessions, populated for routed
  traffic. Added for data-sovereignty compliance.
- **`server_tool_use`** (added, nested object) — token accounting for
  server-side tools. Shape: `{web_search_requests, web_fetch_requests}`.
  Added when Anthropic shipped server-side tools (web search, web fetch).
- **`iterations`** (added, **array**) — empty array `[]` in observed
  sessions. Type contradicts intuition — best guess is per-iteration
  metadata for extended thinking, populated only when relevant.
- **`speed`** (added, string enum) — observed value `"standard"`. Likely
  tracks priority/throughput tier (standard vs. priority).

### New `message` fields (assistant)

These are standard Anthropic API response fields that Claude Code now
passes through verbatim:

- **`id`** — Anthropic API message ID.
- **`type: "message"`** — API response type literal.
- **`stop_reason`** — `end_turn` / `tool_use` / `max_tokens` / etc. Useful
  signal for completion-quality analysis.
- **`stop_sequence`** — matched stop sequence if any.
- **`stop_details`** (newest, nested object) — added when Anthropic
  introduced finer-grained stop reasons (refusal, pause-for-input, etc.).

### New `system.subtype` value

- **`away_summary`** (added) — emitted when Claude Code reattaches to a
  paused session. Carries a `content` field with a one-paragraph
  resume-summary of where the user was. Useful intent signal that the
  learner could consume for bigram extraction.

### Stable (no drift)

- Topology: `~/.claude/projects/{encoded}/{S}.jsonl` + `{S}/{subagents,tool-results}/`
- File format: newline-delimited JSON, UTF-8
- Recognized event types: `user`, `assistant`, `system` still present and structurally unchanged
- Content block types: `text`, `thinking`, `tool_use`, `tool_result`
- Tool input field names: `file_path`, `command`, `pattern`, etc.
- `system.subtype = turn_duration` (still emitted alongside the new `away_summary`)

---

## v2.1.126-observed → v2.1.172-observed

**Observed 2026-06-11** from live session `404204ee` (217 events) plus adjacent
v2.1.170 sessions. Closes the 46-version gap L20 exists to catch. **Purely
additive** — no field removed, renamed, or retyped. The parser still works
unchanged; it sees less than it could. Every item below is acknowledged in the
contract and deferred to a sub-ID; the only one with material value
(`usage.iterations`) is routed to L18.

### New top-level event types

- **`mode`** (added) — `{type, mode, sessionId}`. Interaction-mode control-plane
  event, sibling of `permission-mode`, emitted alongside it on transitions.
  Observed value `"normal"`. No `uuid`/`timestamp`. Falls into `UnknownTypes`.
- **`queue-operation`** (confirmed) — was a known-unhandled-unconfirmed comment
  at v2.1.126; now confirmed with shape `{type, operation, content?, timestamp,
  sessionId}`. `operation ∈ {enqueue, dequeue, remove}`; `enqueue` carries
  `content` = the queued user input (e.g. a `/model` slash command typed while
  Claude was working). Still IGNORED.

### New envelope fields (on recognized types)

- **`user.promptSource`** (string, e.g. `"typed"`) — how the prompt entered the
  turn. DROPPED.
- **`system.level`** (string, `"info"`) — severity level for system events.
  DROPPED.
- **`slug`** (string, e.g. `"staged-floating-candy"`) — human-readable session
  slug on `user`/`assistant`/`attachment`; appears mid-session once assigned.
  DROPPED. (Caught live: this field first appeared partway through the capture
  session itself.)

### New system subtype

- **`local_command`** (added) — local slash-command echo. Carries `content`
  (command text) + the new `level`. Parser does not branch on subtype; `content`
  is read into `SystemContent` but only surfaced for `away_summary`, so the
  command text is captured-but-unused. `turn_duration` / `away_summary` persist.

### Usage semantics change (HIGH value — feeds L18)

- **`usage.iterations`** — was `[]` at v2.1.126, now **populated** with a
  per-inference-iteration token breakdown (`{input_tokens, output_tokens,
  cache_read_input_tokens, cache_creation_input_tokens, cache_creation, type}`).
  The parser already captures it into `MessageUsage.Iterations` but nothing reads
  it. This is the cleanest in-band source for L18's token-economics fix.
- **`inference_geo`** — value `""` → `"not_available"`. Cosmetic; still consumed.

### New toolUseResult shapes (non-breaking)

- **`ToolSearch`** `{matches, query, total_deferred_tools}`,
  **`TaskCreate`** `{task}`,
  **`TaskUpdate`** `{statusChange, success, taskId, updatedFields}`.
  None match the Bash/Edit/Agent signature keys, so all collapse to the parser's
  `"other"` catch-all (Raw preserved, no drop/crash).

### New on-disk artifact (topology, non-breaking)

- Subagent dirs now carry `agent-{shortid}.meta.json` sidecars beside each
  `agent-{shortid}.jsonl`. aOa globs `.jsonl`, so sidecars are ignored harmlessly.
  Relevant to L20.3 historical mining.

### Stable (no drift)

- Topology, file format, `user`/`assistant`/`system` structure, content block
  types, tool-input field names, usage field SET (10), message field set (9).

---

## Adding a new entry

When Claude Code ships a new minor/major version and the compliance test
flags drift:

1. Capture `versions/v{NEW}-observed/` (manifest.json, observations.md, README.md).
2. Append a new section to this file: `## v{prev}-observed → v{new}-observed`.
3. For each change, write 1-2 lines explaining the *what* and the *likely why*.
4. Keep entries terse — this file should be skimmable, not exhaustive. Send
   the reader to `versions/v{X}-observed/observations.md` for detail.
