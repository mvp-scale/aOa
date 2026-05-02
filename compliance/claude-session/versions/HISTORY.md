# Claude Code Session Format — Version History

A narrative changelog of Claude Code's session JSONL format, with one or two
lines per change explaining what we think Claude Code did and why. The exact
release version where each item landed is unknown — we have one inferred
baseline and one observed snapshot, with an unbounded gap between them.

Confidence is implied by phrasing: "added", "shipped" = high; "appears to",
"likely" = medium; "best guess" = low. When in doubt, treat as low.

---

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

## Adding a new entry

When Claude Code ships a new minor/major version and the compliance test
flags drift:

1. Capture `versions/v{NEW}-observed/` (manifest.json, observations.md, README.md).
2. Append a new section to this file: `## v{prev}-observed → v{new}-observed`.
3. For each change, write 1-2 lines explaining the *what* and the *likely why*.
4. Keep entries terse — this file should be skimmable, not exhaustive. Send
   the reader to `versions/v{X}-observed/observations.md` for detail.
