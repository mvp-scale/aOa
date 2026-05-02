# Assumed Stdin Fields (v2.1.70-inferred)

Every stdin JSON path read by `hooks/aoa-status-line.sh`, extracted
mechanically by grepping `jq` invocations. Each entry includes the default
value used when the field is absent (a `jq // <default>` fallback).

## Top-level

| Path         | Default  |
|--------------|----------|
| `cwd`        | `""`     |
| `session_id` | `""`     |
| `version`    | `""`     |

## `model.*`

| Path                 | Default     |
|----------------------|-------------|
| `model.id`           | `""`        |
| `model.display_name` | `"Unknown"` |

## `cost.*`

| Path                         | Default |
|------------------------------|---------|
| `cost.total_cost_usd`        | `0`     |
| `cost.total_lines_added`     | `0`     |
| `cost.total_lines_removed`   | `0`     |
| `cost.total_duration_ms`     | `0`     |
| `cost.total_api_duration_ms` | `0`     |

## `context_window.*`

| Path                                                    | Default  |
|---------------------------------------------------------|----------|
| `context_window.context_window_size`                    | `200000` |
| `context_window.used_percentage`                        | `0`      |
| `context_window.remaining_percentage`                   | `0`      |
| `context_window.total_input_tokens`                     | `0`      |
| `context_window.total_output_tokens`                    | `0`      |
| `context_window.current_usage.input_tokens`             | `0`      |
| `context_window.current_usage.cache_creation_input_tokens` | `0`   |
| `context_window.current_usage.cache_read_input_tokens`  | `0`      |

## `rate_limits.*`

| Path                                          | Default |
|-----------------------------------------------|---------|
| `rate_limits.five_hour.used_percentage`       | `0`     |
| `rate_limits.five_hour.resets_at`             | `0`     |
| `rate_limits.seven_day.used_percentage`       | `0`     |
| `rate_limits.seven_day.resets_at`             | `0`     |

## Total: 21 paths read from stdin
