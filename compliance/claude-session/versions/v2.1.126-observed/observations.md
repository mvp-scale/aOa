# Observations — Claude Code v2.1.126

Drift report comparing what was observed live in v2.1.126 against
`v2.1.70-inferred/`. This is the queue of integration gaps.

## Summary

| Drift category               | Count | Severity |
|------------------------------|-------|----------|
| New top-level event types    | 4     | parser counts as `UnknownTypes` — silent drop |
| New top-level envelope fields| 11    | unconsumed, but harmless on known types |
| New message-level fields     | 5     | unconsumed (assistant `stop_*`, etc.) |
| New `usage` fields           | 5     | unconsumed token-economics signals |
| Missing required fields      | 0     | all v2.1.70-inferred fields still present |
| Type changes                 | 0     | no observed type drift on consumed fields |
| Topology changes             | 0     | session dir layout unchanged |

Topology and file format are stable. All drift is **additive** — Claude Code
has added new event types and fields, but has not removed or restructured the
ones aOa already consumes. The parser's defensive design (multi-path field
probing, `UnknownTypes` bucket, oversize-line skip) means none of this drift
crashes anything, but it does mean signal is being silently dropped.

## 1. New top-level event types (severity: integration gap)

These types appear in v2.1.126 sessions and are silently counted in
`ExtractionHealth.UnknownTypes`:

### `permission-mode`
- Top-level fields: `permissionMode`, `sessionId`, `type`
- Carries the session's permission mode (e.g., default/acceptEdits/etc.)
- **Use case**: would let aOa correlate permission mode with user behavior, tool denial rates

### `attachment`
- Top-level fields: `attachment`, `cwd`, `entrypoint`, `gitBranch`, `isSidechain`, `parentUuid`, `sessionId`, `timestamp`, `type`, `userType`, `uuid`, `version`
- Same envelope shape as `user`/`assistant` events — has `uuid`, `timestamp`, `parentUuid`
- Carries an `attachment` payload (file/image/etc. attached to a message)
- **Use case**: file attachments are first-class signals for observe (file refs)

### `ai-title`
- Top-level fields: `aiTitle`, `sessionId`, `type`
- AI-generated session title
- **Use case**: low value for learner; could be useful for dashboard session naming

### `last-prompt`
- Top-level fields: `lastPrompt`, `leafUuid`, `sessionId`, `type`
- Marker pointing at the last user prompt's UUID (`leafUuid`) — likely a
  navigation/resume hint
- **Use case**: could improve session-resume detection in tailer

## 2. New top-level envelope fields (severity: information available, currently dropped)

These fields appear on already-recognized event types and are silently
ignored by `parser.go:128-138`:

| Field                       | Appears on                | Possible signal                          |
|-----------------------------|---------------------------|------------------------------------------|
| `cwd`                       | user, assistant, system, attachment | working-dir tracking            |
| `entrypoint`                | user, assistant, system, attachment | how the session was launched    |
| `gitBranch`                 | user, assistant, system, attachment | branch correlation              |
| `isSidechain`               | user, assistant, system, attachment | subagent/sidechain marker (overlap with our `Source: subagent`?) |
| `userType`                  | user, assistant, system, attachment | external/internal user          |
| `requestId`                 | assistant                 | API correlation ID                       |
| `promptId`                  | user                      | prompt grouping ID                       |
| `sourceToolAssistantUUID`   | user                      | links a `user` (tool_result) event back to the `assistant` (tool_use) that triggered it — **direct correlation field, currently we synthesize this via `tool_use_id`** |
| `toolUseResult`             | user                      | **structured per-tool result**, separate from inline `tool_result` content blocks. Tool-specific shape — see §6. |
| `messageCount`              | system                    | session message count                    |
| `permissionMode`            | user                      | per-message permission mode              |

`sourceToolAssistantUUID` is the most interesting — it makes the user→assistant
back-link explicit instead of having us correlate through `tool_use_id`.

## 3. New message-level fields (assistant)

| Field           | Notes                                                        |
|-----------------|--------------------------------------------------------------|
| `id`            | Anthropic API message ID                                     |
| `type`          | `"message"` literal (Anthropic API convention)               |
| `stop_reason`   | `end_turn`/`tool_use`/etc. — useful for completion analysis  |
| `stop_sequence` | matched stop sequence if any                                 |
| `stop_details`  | nested object — newer field, contents not yet enumerated     |

## 4. New `usage` fields

We currently consume: `input_tokens`, `output_tokens`, `cache_read_input_tokens`,
`cache_creation_input_tokens`, `service_tier`.

Newly observed but unconsumed:

| Field             | Likely meaning                                                    |
|-------------------|-------------------------------------------------------------------|
| `cache_creation`  | nested object: `{ephemeral_1h_input_tokens, ephemeral_5m_input_tokens}` — TTL-bucket breakdown. Flat `cache_creation_input_tokens` is the **sum** and still emitted. Both are present; neither is deprecated. |
| `inference_geo`   | inference region — empty in local sessions, populated for routed traffic |
| `iterations`      | **array** (not int) — empty `[]` in observed sessions; type contradicts intuition |
| `server_tool_use` | nested: `{web_search_requests, web_fetch_requests}` — token counts for server-side tools |
| `speed`           | string enum — observed `"standard"`; likely priority/throughput tier |

Verified: `cache_creation_input_tokens` is **not** deprecated — it carries the
sum of the new nested `cache_creation` buckets. Our parser still reads the
correct total.

## 5. `toolUseResult` envelope field (highest-value drop)

The `toolUseResult` field on `user` events carries a **structured per-tool
result** that is currently entirely dropped by the parser. Shape varies by
the source tool:

| Source tool       | Shape                                                                                                  |
|-------------------|--------------------------------------------------------------------------------------------------------|
| `Bash`            | `{stdout, stderr, interrupted, isImage, noOutputExpected}`                                             |
| `Edit`            | `{filePath, newString, oldString, originalFile, replaceAll, structuredPatch, userModified}`            |
| Agent (subagent)  | `{agentId, agentType, content, prompt, status, toolStats, totalDurationMs, totalTokens, totalToolUseCount, usage}` |
| other (Read/etc.) | `{file, type}`                                                                                          |

Highest-value signals currently being dropped:

- **`Edit.userModified`** — boolean indicating the user manually edited
  Claude's edit. Direct feedback signal for code-quality learning.
- **`Edit.structuredPatch`** — structured diff of what actually changed.
  Could feed code intelligence beyond what `Edit.input` carries.
- **`Bash.interrupted`** — flag for user-killed commands. Negative signal.
- **`Agent.*`** — full subagent recap including `totalTokens`, `toolStats`,
  `totalDurationMs`. Right now we tail subagent JSONLs separately and
  reconstruct this — `toolUseResult` may be the canonical artifact.

## 6. New system event subtype

`system.subtype` previously only carried `turn_duration` (timing). v2.1.126
adds:

| Subtype        | Payload          | Notes                                                                      |
|----------------|------------------|----------------------------------------------------------------------------|
| `away_summary` | `content` (str)  | Resume-summary text emitted when Claude Code reattaches to a paused session |

The summary text is potentially valuable for the learner's bigram extraction
(it reflects user-stated intent in compressed form) — currently dropped.

## 7. Stable areas (no drift)

- Topology (`projects/{encoded}/{S}.jsonl`, `{S}/subagents/`, `{S}/tool-results/`)
- File format (newline-delimited JSON, UTF-8)
- Envelope identity fields (`type`, `uuid`, `timestamp`, `version`, `sessionId`)
- Message content block types (`text`, `thinking`, `tool_use`, `tool_result`)
- Tool input field names (`file_path`, `command`, `pattern`)
- System event subtype `turn_duration` (still emitted alongside the new `away_summary`)

## 8. Recommended follow-up

Not part of this compliance work — listed for whoever picks up the parser
update layer:

Ranked roughly by signal-per-effort:

1. **Consume `toolUseResult`** — biggest win. `Edit.userModified` is a direct user-feedback signal for code-quality learning; `Bash.interrupted` is a negative signal; agent recaps consolidate subagent telemetry.
2. **Add translators** for `attachment`, `permission-mode`, `last-prompt`, `ai-title` (or explicitly silence the low-value ones)
3. **Consume `sourceToolAssistantUUID`** — replaces synthesized correlation in `translateUser` with the explicit back-link
4. **Branch on `system.subtype`** — `away_summary.content` carries user-intent text in compressed form, useful for the learner's bigram extraction
5. **Capture `stop_reason`** — useful for completion-quality signals (refusals, max_tokens, etc.)
6. **Decide on `isSidechain`** — does it overlap with our existing `Source: subagent` tagging, or is it orthogonal?
7. **Read `cache_creation` nested object** — flat sum is correct, but the 1h/5m breakdown is useful for cache-efficiency analysis
