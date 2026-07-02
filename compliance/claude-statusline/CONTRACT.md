# Claude Code Status Line Integration Contract

**Status:** active
**Current observed version:** 2.1.181
**Last validated:** 2026-06-18

This document specifies aOa's three-part integration with Claude Code's
status line system. The contract is undocumented by Anthropic and may
change across Claude Code releases. The runnable check is
`compliance_test.go`; the narrative changelog is `versions/HISTORY.md`.

When this contract is violated, the status line silently shows zeros — no
error, no log. The compliance check exists to surface that drift.

**Marker legend**

- `[CONSUMED]` — script reads this and renders it
- `[DROPPED]` — present in stdin (or settings) but the script ignores it
- `[REQUIRED]` — without this, the integration is broken

---

## 1. Stdin JSON schema

Claude Code invokes the hook command (registered in §2) and pipes a JSON
object to stdin. The script (`hooks/aoa-status-line.sh`) reads the
following paths via `jq`. Any other top-level fields are unread.

### 1.1 Top-level fields

| Path                | Type    | Status      | Default if absent | Notes |
|---------------------|---------|-------------|-------------------|-------|
| `cwd`               | string  | [CONSUMED]  | empty             | working dir for git status, dashboard port lookup |
| `session_id`        | string  | [CONSUMED]  | empty             | written to `.aoa/hook/context.jsonl` |
| `version`           | string  | [CONSUMED]  | empty             | Claude Code version, written to context.jsonl |

### 1.2 `model` object

| Path                | Type    | Status      | Default if absent | Notes |
|---------------------|---------|-------------|-------------------|-------|
| `model.id`          | string  | [CONSUMED]  | empty             | written to context.jsonl |
| `model.display_name`| string  | [CONSUMED]  | "Unknown"         | rendered as the model segment |

### 1.3 `cost` object

| Path                          | Type   | Status     | Default | Notes |
|-------------------------------|--------|------------|---------|-------|
| `cost.total_cost_usd`         | number | [CONSUMED] | 0       | rendered as `$X.XX` |
| `cost.total_lines_added`      | int    | [CONSUMED] | 0       | line-changes segment |
| `cost.total_lines_removed`    | int    | [CONSUMED] | 0       | line-changes segment |
| `cost.total_duration_ms`      | int    | [CONSUMED] | 0       | session timing segment |
| `cost.total_api_duration_ms`  | int    | [CONSUMED] | 0       | API timing segment |

### 1.4 `context_window` object

| Path                                              | Type   | Status     | Default  | Notes |
|---------------------------------------------------|--------|------------|----------|-------|
| `context_window.context_window_size`              | int    | [CONSUMED] | 200000   | model max context |
| `context_window.used_percentage`                  | number | [CONSUMED] | 0        | written to context.jsonl |
| `context_window.remaining_percentage`             | number | [CONSUMED] | 0        | written to context.jsonl |
| `context_window.total_input_tokens`               | int    | [CONSUMED] | 0        | session-tokens segment |
| `context_window.total_output_tokens`              | int    | [CONSUMED] | 0        | session-tokens segment |
| `context_window.current_usage`                    | object | [CONSUMED] | null     | sub-fields below |
| `context_window.current_usage.input_tokens`       | int    | [CONSUMED] | 0        | summed for ctx display |
| `context_window.current_usage.cache_creation_input_tokens` | int | [CONSUMED] | 0 | summed for ctx display |
| `context_window.current_usage.cache_read_input_tokens`     | int | [CONSUMED] | 0 | summed for ctx display |

### 1.5 `rate_limits` object

| Path                                          | Type   | Status     | Default | Notes |
|-----------------------------------------------|--------|------------|---------|-------|
| `rate_limits.five_hour.used_percentage`       | number | [CONSUMED] | 0       | rate_5h segment |
| `rate_limits.five_hour.resets_at`             | int    | [CONSUMED] | 0       | epoch seconds for countdown |
| `rate_limits.seven_day.used_percentage`       | number | [CONSUMED] | 0       | rate_7d segment |
| `rate_limits.seven_day.resets_at`             | int    | [CONSUMED] | 0       | epoch seconds for countdown |

### 1.6 Additive fields (observed at 2.1.181, not consumed)

Raw stdin capture at 2.1.181 surfaced 11 fields the hook does **not** read.
They are additive — no break to the consumed contract — but recorded so a
future *removal* of one we start consuming would register as drift.

| Path                                          | Status    | Notes |
|-----------------------------------------------|-----------|-------|
| `transcript_path`                             | [DROPPED] | path to session JSONL |
| `effort.level`                                | [DROPPED] | reasoning effort (e.g. "high") |
| `session_name`                                | [DROPPED] | human-readable session label |
| `workspace.current_dir`                       | [DROPPED] | overlaps `cwd` |
| `workspace.project_dir`                       | [DROPPED] | project root |
| `workspace.added_dirs`                        | [DROPPED] | extra workspace dirs |
| `output_style.name`                           | [DROPPED] | active output style |
| `exceeds_200k_tokens`                         | [DROPPED] | bool context-size flag |
| `fast_mode`                                   | [DROPPED] | bool fast-mode flag |
| `thinking.enabled`                            | [DROPPED] | bool thinking flag |
| `context_window.current_usage.output_tokens` | [DROPPED] | per-turn output tokens |

### 1.7 Defensive coding

All `jq` reads use `// <default>` fallbacks (`hooks/aoa-status-line.sh:45-76`).
A missing field renders as zero rather than crashing the hook. This means
**field renames and removals are silent failures** — exactly what this
compliance check catches.

---

## 2. Settings file shape (`.claude/settings.local.json`)

aOa writes its hook registration into the project-local settings file.
The shape we depend on:

```json
{
  "statusLine": {
    "type": "command",
    "command": "bash \"$CLAUDE_PROJECT_DIR/.aoa/hooks/aoa-status-line.sh\""
  }
}
```

| Path                  | Status      | Notes |
|-----------------------|-------------|-------|
| `statusLine`          | [REQUIRED]  | top-level key Claude Code reads to find a status-line hook |
| `statusLine.type`     | [REQUIRED]  | must be `"command"` (other types unsupported) |
| `statusLine.command`  | [REQUIRED]  | shell command Claude Code invokes |

**Source:** `cmd/aoa/cmd/init.go:385-390` (write), `:369` (read), `:515-524` (delete).

If Claude Code renames the `statusLine` key, our hook stops registering and
no status line shows. If they change the supported `type` enum, our
`"command"` value silently no-ops.

### 2.1 Co-existence

aOa's wiring is non-destructive:

- If `statusLine` already exists with a non-aOa command, init refuses to overwrite it (`init.go:369-380`).
- On `aoa remove`, only the aOa entry is removed; other keys are preserved (`init.go:515-541`).

---

## 3. Environment

| Var                  | Status      | Set by      | Notes |
|----------------------|-------------|-------------|-------|
| `CLAUDE_PROJECT_DIR` | [REQUIRED]  | Claude Code | Hook resolves all relative paths from this. Falls back to `$(pwd)` if absent (`aoa-status-line.sh:22`). Fallback works for manual invocation but not when Claude invokes from an unrelated cwd. |

---

## 4. Drift detection rules

| Drift                                                       | Severity | Detection                                          |
|-------------------------------------------------------------|----------|----------------------------------------------------|
| Stdin JSON path renamed/removed (e.g., `cost.total_cost_usd` → `cost.totalCostUsd`) | FAIL  | feed synthetic JSON, check for non-zero render |
| New top-level field in stdin                                | WARN     | observed sample diff vs CONTRACT.md               |
| `statusLine` key renamed in settings.json                   | FAIL     | observed by integration test outside CI scope     |
| `statusLine.type` enum tightened (e.g., `command` removed)  | FAIL     | observed by hook silently not running             |
| `CLAUDE_PROJECT_DIR` not set                                | FAIL     | hook falls back to `$(pwd)`, may resolve wrong project |

WARN signals are reported as `t.Logf` in the test. FAIL signals fail the test.

---

## 5. Versioning

- `versions/v{X}-inferred/` — what `aoa-status-line.sh` reads (mechanical extraction)
- `versions/v{X}-observed/` — what Claude Code v{X} actually pipes (live capture)

Live capture procedure: see `README.md` "Capturing live stdin" section.
