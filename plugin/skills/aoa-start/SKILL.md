---
name: aoa-start
description: Initialize domain skeletons for lazy enrichment
allowed-tools: Task
---

# aOa Start

## User Output

Show:
```
⚡ aOa Start
  Initializing semantic domains...
```

Launch ONE task. When done, show:
```
  ✓ [N] domains initialized

  Terms and keywords build automatically as you work.
  Run `aoa domains` to see status.
```

## Task Prompt

Launch a single Task (model: haiku):

---

Initialize aOa domains. Execute silently:

**[1/2] Get structure**
Run: `aoa tree`

**[2/2] Generate and save domains**

From the tree output, generate 20-32 domains.

Schema:
```json
{"name":"@lowercase_name","description":"Three sentences. What this domain covers. Key concepts and patterns."}
```

Example:
```json
{"name":"@search","description":"Code search and symbol lookup. Includes grep, egrep, find, and locate commands. Pattern matching and result ranking."}
```

Save to Redis:
1. Write JSON array to `.aoa/domains-pending.json`
2. Run: `aoa domains init --file .aoa/domains-pending.json`

This returns the count of domains created.

Return ONLY the count (e.g., "24")

---

## Schema

Domains at init have:
- `name`: @lowercase
- `description`: 3 sentences (used later to query index for terms/keywords)

NO terms or keywords at init - those are built per-prompt using description to query the index.
