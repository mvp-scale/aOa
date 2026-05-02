# Assumed Top-Level Event Types (v2.1.70-inferred)

Derived from `internal/adapters/claude/reader.go:172-185`.

## Recognized — emit canonical events

| `type`      | Emits                                                             |
|-------------|-------------------------------------------------------------------|
| `user`      | `EventUserInput`, `EventToolResult`                               |
| `assistant` | `EventAIThinking`, `EventAIResponse`, `EventToolInvocation` (per tool_use) |
| `system`    | `EventSystemMeta`                                                  |

## Acknowledged but unhandled — counted in `UnknownTypes`

The parser comment at `reader.go:180` lists these as known-unhandled types:

- `progress`
- `file-history-snapshot`
- `queue-operation`
- `etc.` (open-ended)

No translation is performed. They are silently dropped after being counted.

## Content block types (within `message.content` arrays)

Derived from `internal/adapters/tailer/parser.go:204-301`.

| Block `type`  | Read by parser |
|---------------|----------------|
| `text`        | yes            |
| `thinking`    | yes            |
| `tool_use`    | yes            |
| `tool_result` | yes            |

Other block types (e.g., `image`) are mentioned in the parser comment
(`parser.go:203`) but not actively consumed — they are silently skipped.

## System event subtypes

Derived from `parser.go:135-138` and `reader.go:330-341`. The parser reads
`subtype` and `durationMs` but does not branch on subtype value. Comments
reference:

- `turn_duration`

No other subtypes are explicitly handled.
