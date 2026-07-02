# v2.1.178 — Observed Drift vs v2.1.172

Captured 2026-06-18 by the generative conformance harness
(`compliance/conformance/run.sh --all --keep`), which drove 5 controlled
`claude -p` sessions on the installed version 2.1.178 (minimal, bash-tool,
edit-tool, iterations, agent) plus one subagent stream
(`agent-a6afa089dd2c6736a.jsonl`) — 49 events total, all `version="2.1.178"`.
The live v2.1.173 repo session supplements the surfaces the headless harness
cannot drive (`system` / `permission-mode` / `mode`). This is the
re-observation pass closing the 6-version gap (v2.1.172 → v2.1.178).

The transition is **additive on everything aOa already consumes** — no field
was removed, renamed, or had its type changed (`breaking = false`). Every
CONSUMED field (`type`, `uuid`, `timestamp`, `version`, `sessionId`, `isMeta`,
`parentUuid`, `subtype`, `durationMs`; `message.role/model/content`;
`usage.{input,output,cache_read,cache_creation}_tokens` + `service_tier`;
`tool_use.{name,id,input}`; `tool_result.content`) is present and correctly
typed at 178. The parser still works; it sees less than it could — and on the
subagent-token path it sees the right number and reports the wrong one.

## 1. New top-level event types

**None.** No new `type` value appeared, and nothing from the v2.1.172 set was
removed or renamed. The headless harness enumerated `assistant`, `user`,
`attachment`, `ai-title`, `last-prompt`, `queue-operation`; `system` /
`permission-mode` / `mode` were simply not triggered (manual surfaces — see §6).

## 2. New envelope fields (on recognized types)

Two genuinely-new fields appeared at 178, both tied to subagent attribution.

| Field                | Type   | Observed value        | On types                    | Disposition |
|----------------------|--------|-----------------------|-----------------------------|-------------|
| `agentId`            | string | `a6afa089dd2c6736a`   | `user`, `assistant`, `attachment` | DROPPED |
| `attributionAgent`   | string | `general-purpose`     | `assistant`                 | DROPPED |

### `agentId` (NEW) — the missing attribution link
`agentId` ties a top-level event to a spawned subagent: its value matches the
subagent file shortid (`agent-a6afa089dd2c6736a.jsonl`) and the `.meta.json`
sidecar. It falls through as an unconsumed envelope field — no probe path in
`parser.go` reads it. **This is the highest-signal drop at 178.** It is exactly
the link the subagent-token leak (§4) is missing: with `agentId`, a main-session
Agent row could be reconciled against the real subagent usage instead of the
`char/4` estimate. Acknowledge-and-drop; flag for L18.

### `attributionAgent` (NEW) — the subagent type
Names the subagent type that produced the event (observed `"general-purpose"`).
Pairs with `agentId`. DROPPED.

A third field, **`system.pendingWorkflowCount`** (`0`), was observed in the live
v2.1.173 session but **not at true 178** (the harness emitted no system events).
Carried forward as inferred-unverified, DROPPED — a cheap per-session counter.

## 3. New toolUseResult shapes (non-breaking)

### `Agent` gains `resolvedModel`
The v2.1.172 Agent shape had 10 keys; 178 adds `resolvedModel`
(e.g. `"claude-opus-4-8[1m]"`), the concrete model the subagent ran on. The
parser matches the Agent branch on `agentId`/`agentType`, so the extra key is
harmlessly unread. The Agent token totals are all still present and correct
(`toolUseResult.totalTokens = 8145`, full `usage` block) — see §4.

### `Edit` gains a file-creation (`type = "create"`) variant
A Write / file-creation result now appears under the Edit family with shape
`{content, filePath, originalFile, structuredPatch, type, userModified}` — it
lacks `newString`/`oldString`. It still classifies as `SourceTool = "Edit"` via
the `structuredPatch`/`userModified`/`originalFile` signature; `EditNewString`
reads empty (genuinely absent), and the new `content` / `type = "create"` keys
are unread. Non-breaking — CONSUMED as Edit, new fields ignored.

`ToolSearch` / `TaskCreate` / `TaskUpdate` / `other` carry forward from v2.1.172
(not triggered by the headless harness) — inferred-unverified at 178.

## 4. Usage semantics — the subagent-token soft spot still leaks

### `usage.iterations` — REGRESSION vs the v2.1.172 claim
The v2.1.172 manifest recorded `iterations` as **POPULATED** (the cleanest
in-band source for L18 token attribution). At v2.1.178 — and at the live
v2.1.173 session — `iterations` is **empty `[]`** across every assistant event.
The `iterations` scenario confirmed it: two `echo` Bash calls completed in a
single inference turn, so the array never populated. Most likely `iterations`
is now emitted only for genuine multi-inference turns, and the harness never
generated one. `iterations` is `[DROPPED]`, so this is non-breaking — but the
"POPULATED at v2.1.172" narrative is now stale. **Corrected here to
conditionally-populated** (empty in single-inference turns); the L18 routing
note stands but the audit trail only exists under specific session conditions.

### Subagent token attribution — KNOWN SOFT SPOT, reproduces 414x
The agent scenario closes the loop on the prime numeric leak. Claude's own
roll-up reports `toolUseResult.totalTokens = 8145`; the summed subagent JSONL
usage is `23177`; aOa's surfaced estimate is `56` (a `chars/4` projection of a
short relayed summary string) — **414x under**, far worse than the documented
~10x WARN threshold. This is an **attribution leak, not a schema break**:
- `AgentTotalTokens` IS parsed (`parser.go` → `reader.go:306`) and forwarded to
  `ports.SessionEvent`, but no app code reads it.
- `RecordSubagentAPI(input, output)` captures real subagent usage into
  `ContentMeter`, but those accumulators have no non-test readers — the value is
  captured and thrown away.
- The surfaced cost falls back to `SubagentChars` (`char/4`).

The new `agentId`/`attributionAgent` fields (§2) are the structural link that
would let this be fixed. Captured here as the load-bearing counterexample to
"perfect alignment": **main-session usage is exact; subagent usage is not.**

### Main-session usage — EXACT (closed-loop verified)
The minimal scenario string-compared the `claude -p` result-block usage against
the last in-file `message.usage` from the same JSONL the tailer reads:
`{i:4083, o:5, cr:14111, cc:1767}` matched exactly. For the four scalar token
fields on the last main turn, alignment is genuine and lossless (whole-integer
`int()` copy, no rounding hazard). The equality is a 4-field projection on the
last turn only — it does not cover `service_tier`, the `cache_creation`
ephemeral breakdown, `iterations`, multi-turn aggregation, or subagent tokens.

## 5. Stable (no drift)

- Topology: `~/.claude/projects/{encoded}/{S}.jsonl` + `{S}/{subagents,tool-results}/`
- File format: newline-delimited JSON, UTF-8
- Recognized event types `user`/`assistant`/`system` structurally unchanged
- Content block types: `text`, `thinking`, `tool_use`, `tool_result`
- Tool input field names: `file_path`, `command`, `query`, etc.
- Usage field SET unchanged (10 fields); message field set unchanged (9 fields)
- Bash and Edit toolUseResult signatures still match their probe keys

## 6. Surfaces NOT observed at v2.1.178 (inferred, unverified)

The headless harness cannot drive these; their shapes carry forward from
v2.1.172/v2.1.173 and are **not** re-confirmed at true 178:

- `system` events and subtypes (`turn_duration`, `away_summary`, `local_command`)
- `permission-mode`, `mode`
- `queue-operation` `enqueue`-with-`content` (only the operation surface was seen)
- `ToolSearch` / `TaskCreate` / `TaskUpdate` toolUseResult shapes; the `other` shape

Recommend adding harness scenarios that trigger a slash command
(`local_command` + `queue-operation` enqueue) and a turn boundary
(`turn_duration`) to fully observe 178. Until then, treat §6 surfaces as
carried-forward, not verified.

## Disposition summary

| Drift                                   | Class             | aOa today                | Decision (L20.1) |
|-----------------------------------------|-------------------|--------------------------|------------------|
| `agentId` (user/assistant/attachment)   | new field         | dropped (UnknownTypes-adjacent) | acknowledge-drop (**L18 candidate** — attribution link) |
| `attributionAgent` (assistant)          | new field         | dropped                  | acknowledge-drop |
| `system.pendingWorkflowCount` (v2.1.173)| new field         | dropped                  | acknowledge-drop (inferred at 178) |
| `Agent.resolvedModel`                   | new shape key     | unread (Agent branch matches) | acknowledge-drop |
| `Edit` `type=create` shape              | new shape         | CONSUMED as Edit; new keys unread | acknowledge (non-breaking) |
| `usage.iterations` empty at 178/173     | changed semantics | captured, unused         | **correct narrative; feed L18 conditionally** |
| subagent token estimate (414x under)    | numeric leak      | char/4 surfaced; real value captured-and-dropped | **route to L18 (known soft spot)** |

No breaking drift. All schema decisions are "acknowledge in the contract, defer
consumption to a sub-ID." The two material items are both numeric, not
structural: the `iterations` regression (correct the 172 claim) and the
subagent-token attribution leak (route to L18, with `agentId` as the new
in-band link to fix it).
