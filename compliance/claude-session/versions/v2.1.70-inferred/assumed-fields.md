# Assumed Field Schema (v2.1.70-inferred)

Derived mechanically from `internal/adapters/tailer/parser.go` and
`internal/adapters/claude/reader.go`. This is the union of every JSON key the
parser probes.

## Envelope fields (top-level on every event)

From `parser.go:128-138`.

| Logical field | JSON keys probed                          | Parser type | Notes |
|---------------|-------------------------------------------|-------------|-------|
| Type          | `type`                                    | string      |       |
| UUID          | `uuid`, `id`                              | string      | dedup key |
| Timestamp     | `timestamp`                               | RFC3339     | several format fallbacks (`parser.go:385-396`) |
| Version       | `version`                                 | string      | Claude Code version |
| SessionID     | `sessionId`, `session_id`                 | string      |       |
| IsMeta        | `isMeta`                                  | bool        | filters meta events (`tailer.go:296-301`) |
| Subtype       | `subtype`                                 | string      | system events |
| ParentUUID    | `parentUuid`, `parent_uuid`, `parentId`   | string      | system event linkage |
| DurationMs    | `durationMs`                              | int         | system events |

**Top-level fields the parser does NOT consume** (envelope-level): all others.
Anything not listed above is silently dropped.

## Message body (`message` object — present on `user` and `assistant`)

From `parser.go:141-157`.

| Logical field | JSON path           | Parser type     |
|---------------|---------------------|-----------------|
| Role          | `message.role`      | string          |
| Model         | `message.model`     | string          |
| Content       | `message.content`   | string OR array |
| Usage         | `message.usage`     | object          |

Fallback: if `message` is missing, `extractContent` is called on the top-level
object using `type` as the role hint (`parser.go:159-162`).

## Token usage (`message.usage`)

From `parser.go:148-156`.

| Logical field    | JSON key                       | Parser type |
|------------------|--------------------------------|-------------|
| InputTokens      | `input_tokens`                 | int         |
| OutputTokens     | `output_tokens`                | int         |
| CacheReadTokens  | `cache_read_input_tokens`      | int         |
| CacheWriteTokens | `cache_creation_input_tokens`  | int         |
| ServiceTier      | `service_tier`                 | string      |

**Not consumed:** anything else under `usage`.

## Content block: `tool_use`

From `parser.go:227-250`.

| Logical field | JSON keys probed                                    | Source                |
|---------------|-----------------------------------------------------|-----------------------|
| Name          | `name`, `tool_name`                                 | block top-level       |
| ID            | `id`, `tool_use_id`                                 | block top-level       |
| Input         | `input` THEN `tool_input` (first non-nil wins)      | block top-level       |
| FilePath      | `file_path`, `path`, `notebook_path`, `filePath`    | inside Input          |
| Offset        | `offset`                                            | inside Input          |
| Limit         | `limit`                                             | inside Input          |
| Command       | `command`, `cmd`                                    | inside Input          |
| Pattern       | `pattern`, `query`, `q`                             | inside Input          |

## Content block: `tool_result`

From `parser.go:252-300`.

| Logical field      | JSON keys probed                                              | Notes |
|--------------------|---------------------------------------------------------------|-------|
| ToolUseID          | `tool_use_id`, `id`                                           |       |
| Inline text        | `text` (if 0 < len < 2000, appended to AssistantText)         |       |
| Content size/text  | `content` (string OR array of `{text}` blocks)                | sum of `len(text)` |

If inline content is 0 chars, the size is resolved from the persisted file at
`{sessionDir}/tool-results/{tool_use_id}.txt` (`tailer.go:445-480`).

## Content block: `thinking`

From `parser.go:219-224`.

| Logical field | JSON keys probed              |
|---------------|-------------------------------|
| Text          | `thinking`, `text`, `content` |

(First non-empty wins.)

## Tool name → file action mapping

From `reader.go:344-357`. Used to set `FileRef.Action` on
`EventToolInvocation`.

| Tool name             | Action     |
|-----------------------|------------|
| `Read`                | `read`     |
| `Write`               | `write`    |
| `Edit`, `NotebookEdit`| `edit`     |
| `Grep`, `Glob`        | `search`   |
| anything else         | `access`   |

## File path discovery

aOa expects the session directory to be derived as:

```
~/.claude/projects/{absolute-project-path-with-/-replaced-by-->}/
```

Inside that directory the parser expects:

- `{UUID}.jsonl` files at the top level (latest by mtime is the active session)
- `{UUID}/subagents/agent-*.jsonl` for subagent streams
- `{UUID}/tool-results/{tool_use_id}.txt` for persisted tool output

Source: `tailer.go:332` (subagent path), `tailer.go:457` (tool-results path),
`tailer.go:536-546` (project path encoding).
