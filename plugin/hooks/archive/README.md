# Legacy Hooks Archive

These hooks were consolidated into `aoa-gateway.py` for simplified maintenance.

## Archived Files

- `aoa-intent-*.py` - Intent capture, summary, prefetch (now handled by gateway)
- `aoa-predict-context.py` - Context prediction (now handled by gateway)
- `aoa-auto-outline.py` - Auto outline trigger (deprecated)
- `aoa-enforce-search.py` - Search enforcement (now handled by gateway --event=enforce)

## Current Architecture

Single unified hook: `aoa-gateway.py` with event-based routing:
- `--event=prompt` (UserPromptSubmit)
- `--event=enforce` (PreToolUse for Grep/Glob)
- `--event=prefetch` (PreToolUse for Read/Edit/Write)
- `--event=tool` (PostToolUse)

Status line remains separate: `aoa-status-line.sh`
