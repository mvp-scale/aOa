# v2.1.172 — Observed Drift vs v2.1.126

Captured 2026-06-11 from live session `404204ee` (217 events) plus adjacent
v2.1.170 sessions for the toolUseResult / subagent surface. This is the
re-observation pass for L20.1 across the 46-version gap (v2.1.126 → v2.1.172).

The transition is **additive on everything aOa already consumes** — no field
was removed, renamed, or had its type changed. Every drift item below is either
a new event type (dropped), a new field (dropped), or a semantics change on a
field we already capture. The parser still works; it sees less than it could.

## 1. New top-level event types

### `mode` (NEW) — `{type, mode, sessionId}`
A control-plane event, sibling to `permission-mode`. Carries the interaction
mode (observed value `"normal"`). No `uuid`/`timestamp`. Emitted alongside
`permission-mode` on mode transitions. **Currently falls into `UnknownTypes`.**
Low learner value; recommend acknowledge-and-drop.

### `queue-operation` (CONFIRMED) — `{type, operation, content?, timestamp, sessionId}`
Was listed as known-unhandled-unconfirmed at v2.1.126 (a reader.go comment).
Now confirmed present with a concrete shape. Tracks the message queue:
- `operation: "enqueue"` carries `content` = the queued user input (often a
  slash command like `/model` typed while Claude was working).
- `operation: "dequeue"` / `"remove"` carry no content.

**Still IGNORED.** The `enqueue.content` is real user intent that bypasses the
normal `user` event path — a candidate signal for the learner, but
deferred.

## 2. New envelope fields (on recognized types)

| Field             | Type   | Observed values   | Disposition |
|-------------------|--------|-------------------|-------------|
| `user.promptSource` | string | `"typed"`, absent | DROPPED — how the prompt entered the turn |
| `system.level`      | string | `"info"`          | DROPPED — severity level for system events |

Both are new at v2.1.172. Neither breaks any probe path. Low value;
acknowledge-and-drop pending a use case.

## 3. New system subtype

### `system.subtype = "local_command"` (NEW)
Emitted for local slash-command echoes. Carries `content` (the command text)
and the new `level` field. The parser does not branch on subtype — it reads
`content` into `SystemContent` but only surfaces it for `away_summary`. So the
command text is captured-but-unused here. Subtypes `turn_duration` and
`away_summary` from v2.1.126 still exist (not triggered in this sample).

## 4. Usage semantics change (HIGH value — feeds L18)

### `usage.iterations` — was `[]`, now POPULATED
At v2.1.126 the `iterations` array was always empty. At v2.1.172 it carries a
per-inference-iteration token breakdown:

```json
"iterations": [
  {"input_tokens": 2204, "output_tokens": 722,
   "cache_read_input_tokens": 15007, "cache_creation_input_tokens": 9164,
   "cache_creation": {"ephemeral_5m_input_tokens": 0, "ephemeral_1h_input_tokens": 9164},
   "type": "message"}
]
```

The parser already captures this into `MessageUsage.Iterations` (`[]any`) but
the comment still says "observed empty" and **nothing reads it**. When a single
assistant turn spans multiple inference iterations, the top-level `usage` block
is the relevant aggregate, but `iterations` is the audit trail for verifying
per-turn attribution. This is the cleanest in-band source for L18's token-economics
fix. **No code change here — flagged for L18.**

### `inference_geo` — `""` → `"not_available"`
Cosmetic value change. Still consumed as a string; no impact.

## 5. New toolUseResult shapes (non-breaking)

Three new per-tool envelope-result shapes appeared, none matching the parser's
Bash/Edit/Agent signature keys, so all collapse to `SourceTool = "other"`
(`Raw` preserved, typed fields empty):

| Tool        | Shape                                              |
|-------------|----------------------------------------------------|
| `ToolSearch`| `{matches, query, total_deferred_tools}`           |
| `TaskCreate`| `{task}`                                           |
| `TaskUpdate`| `{statusChange, success, taskId, updatedFields}`   |

Non-breaking — the "other" catch-all absorbs them. If any becomes interesting
(e.g. TaskUpdate status transitions for debrief), add a discriminated branch.

## 6. New on-disk artifact (topology, non-breaking)

Subagent directories now contain `agent-{shortid}.meta.json` sidecars next to
each `agent-{shortid}.jsonl`. aOa discovers subagents by the `.jsonl` glob, so
the sidecars are ignored harmlessly. Worth knowing for L20.3 historical mining.

## 7. Stable (no drift)

- Topology: `~/.claude/projects/{encoded}/{S}.jsonl` + `{S}/{subagents,tool-results}/`
- File format: newline-delimited JSON, UTF-8
- Recognized event types `user`/`assistant`/`system` structurally unchanged
- Content block types: `text`, `thinking`, `tool_use`, `tool_result`
- Tool input field names: `file_path`, `command`, `pattern`, etc.
- Usage field SET unchanged (10 fields); message field set unchanged (9 fields)

## Disposition summary

| Drift                          | Class            | aOa today        | Decision (L20.1) |
|--------------------------------|------------------|------------------|------------------|
| `mode` event                   | new event type   | UnknownTypes     | acknowledge-drop |
| `queue-operation` event        | new event type   | UnknownTypes     | acknowledge-drop (candidate) |
| `user.promptSource`            | new field        | dropped          | acknowledge-drop |
| `system.level`                 | new field        | dropped          | acknowledge-drop |
| `system.subtype=local_command` | new subtype      | content captured, unused | acknowledge-drop |
| `usage.iterations` populated   | changed semantics| captured, unused | **feed L18**     |
| ToolSearch/TaskCreate/TaskUpdate toolUseResult | new shapes | collapse to "other" | acknowledge-drop |

No breaking drift. All decisions are "acknowledge in the contract, defer
consumption to a sub-ID." The only item with material downstream value is
`usage.iterations`, routed to L18.
