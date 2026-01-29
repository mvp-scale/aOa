# Architecture Reference

This file documents why the skill works the way it does.

## The Constraint

**Subagents cannot spawn other subagents.**

The Task tool is NOT available to subagents. If a subagent tries to spawn tasks, it fails silently and falls back to doing work itself (sequentially).

## Why Inline Execution

The skill runs **inline in the main conversation** (no `context: fork`).

This means:
- It CAN spawn Task agents (4 parallel Haiku)
- It executes in the user's context
- The user sees progress in real-time

## Execution Flow

```
Main Conversation
├─ /aoa-test invoked
├─ Phase 1: Main does intelligence.json (~45s)
├─ Phase 2: Main spawns 4 parallel Haiku (~15s)
│  ├─ Haiku 1: domains 1-6
│  ├─ Haiku 2: domains 7-12
│  ├─ Haiku 3: domains 13-18
│  └─ Haiku 4: domains 19-24
└─ Phase 3: Main runs enrichment (~20s)
```

## Parallel Task Spawning

To spawn tasks in parallel, the main conversation must make multiple Task tool calls **in a single response**.

```
Response contains:
- Task(model=haiku, description="Domains 1-6", ...)
- Task(model=haiku, description="Domains 7-12", ...)
- Task(model=haiku, description="Domains 13-18", ...)
- Task(model=haiku, description="Domains 19-24", ...)
```

All 4 run concurrently. Main waits for all to complete.

## Sources

- [Claude Code Sub-Agents Docs](https://code.claude.com/docs/en/sub-agents)
- [Claude Code Skills Docs](https://code.claude.com/docs/en/skills)
