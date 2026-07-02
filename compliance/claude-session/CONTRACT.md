# Claude Code Session Integration Contract

**Status:** active
**Current observed version:** 2.1.181
**Last validated:** 2026-06-18

This document is the integration contract between aOa and Claude Code's
on-disk session format. It documents the **full observed schema** at the
current version, with each field marked `[CONSUMED]` (the parser reads it),
`[DROPPED]` (the parser sees it but does nothing with it), or `[IGNORED]`
(the parser skips the entire event/block). The contract is neither
documented nor stable — Anthropic does not publish it, and it has
demonstrably changed across versions. This file is the spec; the runnable
check is `compliance_test.go`; the narrative changelog is `versions/HISTORY.md`.

When this contract is violated, aOa silently degrades (events fall into the
`UnknownTypes` bucket, fields read as empty strings, tool events are missed).
The compliance check exists to surface that drift before it becomes a
production gap.

**Marker legend**

- `[CONSUMED]` — parser reads this field/type and emits a canonical event or attribute
- `[DROPPED]` — field/type is present in the JSONL but the parser does not read it (signal we throw away)
- `[IGNORED]` — entire event type is unhandled; counted in `UnknownTypes` and dropped

---

## 1. Topology

### 1.1 Session directory location

```
~/.claude/projects/{encoded-project-path}/
```

`encoded-project-path` is the absolute project root with `/` replaced by `-`.
Example: `/home/corey/aOa-go` → `~/.claude/projects/-home-corey-aOa-go/`.

**Source:** `internal/adapters/tailer/tailer.go:536-546` (`SessionDirForProject`).

### 1.2 Per-session layout

For each session UUID `S`, the project directory contains:

```
{encoded-project-path}/
  {S}.jsonl                              # main session log (we tail this)
  {S}/                                   # session-scoped artifacts directory
    subagents/
      agent-{shortid}.jsonl              # per-subagent event stream (we tail these)
    tool-results/
      {tool_use_id}.txt                  # persisted large tool results (we read on demand)
```

**Source:**
- Main JSONL discovery: `tailer.go:494-530` (`findLatestJSONL`)
- Subagent discovery: `tailer.go:319-440` (`readSubagentLines`)
- Persisted tool results: `tailer.go:445-480` (`resolvePersistedToolResults`)

### 1.3 What we do NOT read

aOa intentionally does not depend on:

- `~/.claude/sessions/`
- `~/.claude/tasks/`
- `~/.claude/todos/`
- `~/.claude/plans/`
- `~/.claude/history.jsonl`
- `~/.claude/plugins/`
- `~/.claude/session-env/`
- `~/.claude/shell-snapshots/`
- `~/.claude/settings.json`
- Any peer file/directory under `~/.claude/`

If integration scope expands to these, this contract must be updated.

---

## 2. File format

- All session and subagent files are **newline-delimited JSON** (`.jsonl`).
- Each non-empty line is a single self-contained JSON object.
- UTF-8 encoded; UTF-8 BOM is tolerated and stripped (`parser.go:399-404`).
- Lines exceeding **512 KB** are skipped without parse failure (`tailer.go:253-259`).
  This is deliberate — Claude Code has historically emitted multi-MB tool output
  lines (issue #23948) and we treat oversize lines as discardable.
- Files are append-only between session resumes; truncation triggers an offset
  reset, not an error (`tailer.go:218-222`).

---

## 3. Event schema (what we consume)

aOa consumes events keyed by the top-level `type` field. The parser is
defensive: missing fields read as zero values, unknown types are counted in
`ExtractionHealth.UnknownTypes`. The translator (`reader.go`) maps Claude
events into agent-agnostic `ports.SessionEvent`s.

### 3.1 Top-level envelope fields

Fields observed at v2.1.126 across recognized types (`user`, `assistant`, `system`):

| Field                     | Status      | JSON keys tried (in order)              | Notes |
|---------------------------|-------------|------------------------------------------|-------|
| `type`                    | [CONSUMED]  | `type`                                   | required |
| `uuid`                    | [CONSUMED]  | `uuid`, `id`                             | dedup key |
| `timestamp`               | [CONSUMED]  | `timestamp` (ISO 8601)                   | required for content events |
| `version`                 | [CONSUMED]  | `version`                                | Claude Code version, for health |
| `sessionId`               | [CONSUMED]  | `sessionId`, `session_id`                | event linkage |
| `isMeta`                  | [CONSUMED]  | `isMeta`                                 | filters meta events (e.g., `/clear`) |
| `parentUuid`              | [CONSUMED]  | `parentUuid`, `parent_uuid`, `parentId`  | system event linkage |
| `subtype`                 | [CONSUMED]  | `subtype`                                | system events |
| `durationMs`              | [CONSUMED]  | `durationMs`                             | system events |
| `cwd`                     | [DROPPED]   | —                                        | session working directory |
| `gitBranch`               | [DROPPED]   | —                                        | git branch at event time |
| `entrypoint`              | [DROPPED]   | —                                        | how Claude Code was launched |
| `userType`                | [DROPPED]   | —                                        | external/internal user |
| `isSidechain`             | [DROPPED]   | —                                        | sidechain/subagent marker (may overlap with our `Source: subagent`) |
| `requestId`               | [DROPPED]   | —                                        | Anthropic API request ID (assistant only) |
| `promptId`                | [DROPPED]   | —                                        | prompt grouping ID (user only) |
| `sourceToolAssistantUUID` | [DROPPED]   | —                                        | explicit user→assistant back-link for tool results (user only) |
| `toolUseResult`           | [DROPPED]   | —                                        | structured per-tool result; see §3.7 |
| `messageCount`            | [DROPPED]   | —                                        | per-session message count (system only) |
| `permissionMode`          | [DROPPED]   | —                                        | per-message permission mode (user only) |
| `promptSource`            | [DROPPED]   | —                                        | (v2.1.172) how the prompt entered the turn ("typed" / queued / ...) — user only |
| `level`                   | [DROPPED]   | —                                        | (v2.1.172) severity level for system events ("info") — system only |
| `slug`                    | [DROPPED]   | —                                        | (v2.1.172) human-readable session slug — user/assistant/attachment |
| `agentId`                 | [DROPPED]   | —                                        | (v2.1.178) links a top-level event to a spawned subagent; value = subagent file shortid — user/assistant/attachment. HIGH-value L18 attribution link |
| `attributionAgent`        | [DROPPED]   | —                                        | (v2.1.178) subagent TYPE that produced the event ("general-purpose") — assistant only |
| `pendingWorkflowCount`    | [DROPPED]   | —                                        | (v2.1.173) per-session pending-workflow counter — system only |
| `interruptedMessageId`    | [DROPPED]   | —                                        | (v2.1.173) id of the assistant message interrupted by the user; present only on interrupt — user only |

**Source for [CONSUMED] paths:** `parser.go:128-138`.

### 3.2 Top-level `type` values (full observed set at v2.1.126)

| `type`            | Status      | Translator                         | Notes |
|-------------------|-------------|------------------------------------|-------|
| `user`            | [CONSUMED]  | `translateUser` (reader.go:198)    | emits `EventUserInput` + `EventToolResult` |
| `assistant`       | [CONSUMED]  | `translateAssistant` (reader.go:248)| emits `EventAIThinking`, `EventAIResponse`, `EventToolInvocation`/tool_use |
| `system`          | [CONSUMED]  | `translateSystem` (reader.go:330)  | emits `EventSystemMeta` |
| `permission-mode` | [CONSUMED]  | `translatePermissionMode`          | permission-mode transition (control-plane) |
| `attachment`      | [CONSUMED]  | `translateAttachment`              | file/image attachment with full envelope |
| `ai-title`        | [CONSUMED]  | `translateAITitle`                 | AI-generated session title (UX-only) |
| `last-prompt`     | [CONSUMED]  | `translateLastPrompt`              | resume/checkpoint pointer |
| `mode`            | [IGNORED]   | —                                  | (v2.1.172) interaction-mode control-plane event, sibling of `permission-mode`; `{type, mode, sessionId}` |
| `queue-operation` | [IGNORED]   | —                                  | (v2.1.172 confirmed) message-queue lifecycle; `enqueue.content` carries queued input |
| `progress`        | [IGNORED]   | —                                  | mentioned in reader.go comment; status unconfirmed |
| `file-history-snapshot` | [IGNORED] | —                                | mentioned in reader.go comment; status unconfirmed |

Any other value falls through to `UnknownTypes` (`reader.go:179-184`).

### 3.3 Message body (`message` object on `user`/`assistant`)

| Field   | JSON path                | Type            | Notes |
|---------|--------------------------|-----------------|-------|
| Role    | `message.role`           | string          | "user" \| "assistant" |
| Model   | `message.model`          | string          | assistant-only |
| Content | `message.content`        | string OR array | both shapes supported (`parser.go:174-200`) |
| Usage   | `message.usage`          | object          | assistant-only |

#### Content array element types (`parser.go:204-301`)

| `type`        | Fields read                                                                            |
|---------------|----------------------------------------------------------------------------------------|
| `text`        | `text` (string)                                                                        |
| `thinking`    | `thinking`, `text`, `content` (first non-empty wins)                                   |
| `tool_use`    | `name`/`tool_name`, `id`/`tool_use_id`, `input` or `tool_input` (object)               |
| `tool_result` | `tool_use_id`/`id`, `content` (string OR array OR via `text`)                          |

#### `tool_use.input` field probing (`parser.go:233-249`)

| Logical field | JSON keys tried                                          |
|---------------|----------------------------------------------------------|
| File path     | `file_path`, `path`, `notebook_path`, `filePath`         |
| Offset        | `offset`                                                 |
| Limit         | `limit`                                                  |
| Bash command  | `command`, `cmd`                                         |
| Pattern       | `pattern`, `query`, `q`                                  |

### 3.4 Token usage (`message.usage`)

Full observed shape at v2.1.126:

| Field                          | Status      | JSON key                         | Notes |
|--------------------------------|-------------|----------------------------------|-------|
| `input_tokens`                 | [CONSUMED]  | `input_tokens`                   |       |
| `output_tokens`                | [CONSUMED]  | `output_tokens`                  |       |
| `cache_read_input_tokens`      | [CONSUMED]  | `cache_read_input_tokens`        |       |
| `cache_creation_input_tokens`  | [CONSUMED]  | `cache_creation_input_tokens`    | flat sum of `cache_creation.*` |
| `service_tier`                 | [CONSUMED]  | `service_tier`                   |       |
| `cache_creation`               | [DROPPED]   | —                                | nested: `{ephemeral_1h_input_tokens, ephemeral_5m_input_tokens}` |
| `inference_geo`                | [DROPPED]   | —                                | inference region (empty in local sessions) |
| `server_tool_use`              | [DROPPED]   | —                                | nested: `{web_search_requests, web_fetch_requests}` |
| `iterations`                   | [DROPPED]   | —                                | array — **populated at v2.1.172** (was empty at v2.1.126); per-inference-iteration token breakdown. Captured into `MessageUsage.Iterations`, unused. High value for L18. |
| `speed`                        | [DROPPED]   | —                                | enum string (e.g., `"standard"`) |

**Source for [CONSUMED] paths:** `parser.go:148-156`.

### 3.5 System event subtypes

`system.subtype` is passed verbatim as `EventSystemMeta.Text`. The
parser does NOT branch on subtype value — it only reads `subtype` and
`durationMs` regardless of subtype.

| Subtype          | Status      | Subtype-specific payload                | Notes |
|------------------|-------------|------------------------------------------|-------|
| `turn_duration`  | [CONSUMED]  | `durationMs`                             | turn timing |
| `away_summary`   | [CONSUMED]  | `content` (string)                       | resume-summary text emitted on session reattach (read into `AwaySummary`) |
| `local_command`  | [DROPPED]   | `content`, `level`                       | (v2.1.172) local slash-command echo; `content` captured into `SystemContent` but only surfaced for `away_summary` |

The `content` field on `system` events is currently dropped. Other
subtypes are tolerated.

### 3.6 Message-level fields (`message`)

| Field            | Role        | Status      | Notes |
|------------------|-------------|-------------|-------|
| `role`           | both        | [CONSUMED]  | drives content extraction |
| `model`          | assistant   | [CONSUMED]  | model identifier |
| `content`        | both        | [CONSUMED]  | string OR array of content blocks |
| `usage`          | assistant   | [CONSUMED]  | see §3.4 |
| `id`             | assistant   | [DROPPED]   | Anthropic API message ID |
| `type`           | assistant   | [DROPPED]   | API response type literal (`"message"`) |
| `stop_reason`    | assistant   | [DROPPED]   | `end_turn` / `tool_use` / `max_tokens` / etc. |
| `stop_sequence`  | assistant   | [DROPPED]   | matched stop sequence if any |
| `stop_details`   | assistant   | [DROPPED]   | nested object (refusal, pause, etc.) |

### 3.7 `toolUseResult` envelope field (user events)

A user event with `toolUseResult` carries a tool's structured result at
envelope level — separate from inline `tool_result` content blocks.
**Currently entirely [DROPPED].** Tool-specific shapes observed at v2.1.126:

| Source tool       | Shape                                                                                                  |
|-------------------|--------------------------------------------------------------------------------------------------------|
| `Bash`            | `{stdout, stderr, interrupted, isImage, noOutputExpected}`                                             |
| `Edit`            | `{filePath, newString, oldString, originalFile, replaceAll, structuredPatch, userModified}`            |
| Agent (subagent)  | `{agentId, agentType, content, prompt, status, toolStats, totalDurationMs, totalTokens, totalToolUseCount, usage}` |
| `ToolSearch` (v2.1.172) | `{matches, query, total_deferred_tools}` — collapses to `other` branch              |
| `TaskCreate` (v2.1.172) | `{task}` — collapses to `other` branch                                              |
| `TaskUpdate` (v2.1.172) | `{statusChange, success, taskId, updatedFields}` — collapses to `other` branch      |
| other (Read/etc.) | `{file, type}`                                                                                          |

> **Note on `Agent.totalTokens`:** parsed into `ToolUseResultDetail.AgentTotalTokens`
> and copied to `ports.SessionEvent`, but **never read** by app code — the Agent
> activity row attributes tokens via `len(final_summary)/4` instead. This is the
> root of the L18 understatement (see `.context/details/2026-06-11-L20-numeric-conformance.md`).

This is the most signal-rich envelope-level field we don't read. See
`versions/v2.1.126-observed/observations.md` §7 for the recommended
follow-up.

### 3.8 System tag stripping (user text)

User-message text is run through `cleanSystemTags` (`parser.go:306-327`)
which strips:

- `<system-reminder>...</system-reminder>`
- `<task-notification>...</task-notification>`
- `<local-command-...>...</local-command-...>`
- `<command-...>...</command-...>`
- `<hook-...>...</hook-...>`
- `Full transcript available at:...`
- All residual `<...>` tags

Whitespace is collapsed to single spaces.

---

## 4. Drift detection rules

A drift signal is any of:

1. **Topology drift** (FAIL): session directory not at the expected path; `{S}.jsonl` and `{S}/` no longer co-located; `subagents/` or `tool-results/` moved or renamed.
2. **File-format drift** (FAIL): files are not newline-delimited JSON; lines are not valid JSON objects after BOM stripping.
3. **New top-level event type** (WARN): a `type` value appears that does not match the recognized set in §3.2.
4. **New top-level envelope field** (WARN): an unconsumed top-level field appears that may carry signal we should map.
5. **Missing required field** (FAIL): an event of a recognized type is missing a field listed as "required by aOa" in §3.1.
6. **Type change** (FAIL): a field's JSON type changes (e.g., `usage.input_tokens` becomes a string, `content` becomes a different shape).
7. **Renamed field that breaks all probe paths** (FAIL): a field is renamed to a key not in our probe list (e.g., `file_path` → something other than `path`/`notebook_path`/`filePath`).

WARN signals are reported by `compliance_test.go` as `t.Logf` — they do not
fail the test. They become the queue of integration work.

---

## 5. Versioning

This contract is versioned implicitly via `versions/`:

- `versions/v{X}-inferred/` — what aOa's code assumes (mechanically derived from `parser.go`/`reader.go`/`tailer.go`)
- `versions/v{X}-observed/` — what was actually observed in live session data at version `X`

When Claude Code ships a new version that introduces drift, capture a new
`v{X}-observed/` snapshot and update §3 here only after the parser is
adjusted to consume the new shape.
