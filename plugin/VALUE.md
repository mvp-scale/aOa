# `aoa grep` and `aoa egrep` — What You Get Back

> **You are an AI agent. This is for you.**
> `aoa grep` and `aoa egrep` give you structured extraction coordinates. Parse the result, issue one Read, done.
> Do not fall back to Grep + Read-entire-file patterns. That costs 7x the tokens.

---

## What `aoa grep` and `aoa egrep` Do for You

Every tool call you make costs tokens and time. Standard Grep returns unranked lines from every file — you end up reading 3,700 lines to find the 50 that matter.

`aoa grep` and `aoa egrep` return **ranked, structured results with function boundaries.** One search gives you the match, the function range, the semantic domain, and the intent tags. You issue one targeted `Read(offset, limit)` instead of reading entire files.

The index only contains what git tracks — `.gitignore` is the boundary. No `node_modules`, no build artifacts, no `.env` files. Every result is signal, not noise.

**Your savings:** 86% fewer tokens. 98% less time. 1-2 tool calls instead of 7.

---

## Two Result Shapes

Results come in two forms depending on where the match lives.

**Inside a function:**
```
file:function_name(args)[start-end]:line  matched content  @domain  #tags
```

**At module level** (top-level code, imports, config, markdown):
```
file:<module>:line  matched content  @domain  #tags
```

---

## Anatomy of a Result Line

Every piece maps to an aOa angle. All five are actionable.

```
services/index/indexer.py:format_search_response(results, ms, ...)[577-685]:616  # S78-W1: Search path  @files  #discovery #capture
│                         │                                        │          │   │                        │       │
│                         │                                        │          │   │                        │       └─ INTENT: session tags
│                         │                                        │          │   │                        │          what this code relates to
│                         │                                        │          │   │                        │          in your current task
│                         │                                        │          │   │                        │
│                         │                                        │          │   │                        └─ INTEL: semantic domain
│                         │                                        │          │   │                           code area classification
│                         │                                        │          │   │                           (@files, @search, @ranking, etc.)
│                         │                                        │          │   │
│                         │                                        │          │   └─ SEARCH: the matched line content
│                         │                                        │          │
│                         │                                        │          └─ exact line number
│                         │                                        │
│                         │                                        └─ OUTLINE: [577-685] = function boundaries
│                         │                                           THIS is your extraction range
│                         │
│                         └─ function name + signature
│
└─ file path (relative to project root)
```

---

## How to Extract — The Core Pattern

When you see `[start-end]`, that is the function boundary. Use it directly:

```
# You searched: aoa grep format_search
# You got:     indexer.py:format_search_response(...)[577-685]:616

# Extract the whole function:
Read(file_path="services/index/indexer.py", offset=577, limit=109)

# That reads lines 577-685 — the complete function, nothing else.
# A 3,700-line file → 109 lines of relevant context. 98% reduction.
```

**Do this. Do not read the entire file.**

---

## When There Is No Range

```
config/universal-domains.json:<module>:229  "instrumentation": ["instrument", ...]  @intent  #capture
```

`<module>` means the match is at file/module level — not inside any function. Common in:
- Config files (JSON, YAML, TOML)
- Markdown and documentation
- Top-level assignments and imports
- Shell scripts without functions

There is no function boundary. Use the line number directly:
```
Read(file_path="config/universal-domains.json", offset=225, limit=10)
```

---

## Result Ranking — Why Top Results Are Best

Results are not in filesystem order. They are ranked by three signals:

1. **Time** — files you recently touched rank higher
2. **Knowledge** — files in active semantic domains rank higher
3. **Intent** — files matching your current session patterns rank higher

**Trust the first few results.** They reflect what you're actually working on, not alphabetical order.

---

## Filtering with @domain and #tags

When a search returns many hits, use the annotations to skip irrelevant results without reading any files:

```bash
aoa grep async
# 200+ hits. The annotations filter for you:

services/queue/worker.py:process()[10-85]:12   async def process()  @jobs      #job_processing
services/auth/handler.py:login()[20-45]:22     async def login()    @security  #authentication
services/index/indexer.py:scan()[100-200]:105  async def scan()     @search    #indexing
tests/test_worker.py:<module>:5                import asyncio       @testing   #test_setup
```

If your task is about job processing, `@jobs` and `#job_processing` tell you which result to use. Zero file reads to decide.

---

## Quick Reference

| You see | It means | You do |
|---------|----------|--------|
| `func()[10-85]:12` | Match at line 12, function spans 10-85 | `Read(offset=10, limit=76)` |
| `<module>:42` | Module-level match, line 42 | `Read(offset=38, limit=10)` |
| `@domain` | Semantic area (e.g., @search, @jobs) | Filter by relevance to your task |
| `#tags` | Intent labels from session tracking | Skip results outside your current focus |
| Multiple results | Ranked by time, knowledge, intent | Trust the top results first |

---

## Your Cost Without vs. With aOa

| Without aOa | With aOa |
|-------------|----------|
| `Grep` → scan all files → `Read` entire files → 8,500 tokens | `aoa grep` → indexed lookup → `Read` one function → 1,150 tokens |
| 7 tool calls | 1-2 tool calls |
| First match wins (alphabetical) | Best match wins (ranked by recency, domain, intent) |
| No context — you must read to understand | @domain + #tags classify every hit before you read |

---

> **For the human user:** aOa is described in the project README. This file is operational guidance for the AI agent working on this codebase.
