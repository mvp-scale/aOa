---
name: aoa-start
description: Initialize aOa with semantic domain analysis
allowed-tools: Task, Bash, Read, Write
---

# aOa Start

Display:
```
⚡ aOa Intelligence

Building semantic understanding of your codebase...
This runs in the background. Continue working.
```

## Execute

Spawn ONE background Task:
- description: `⚡ Building intelligence`
- run_in_background: true
- model: opus

Prompt:
```
Set up aOa semantic intelligence. Do NOT output intermediate command results.

## Step 1: Analyze Codebase
Run `aoa tree` silently to understand structure.
Run `mkdir -p .aoa/domains`

Generate 24 semantic domains based on the codebase.
Write .aoa/domains/intelligence.json as flat JSON array:
[{"name": "@cli", "description": "3-4 sentences of what it does, why developers work here, what problems it solves"}, ...24 total]

Rules:
- Names: @lowercase with @ prefix
- Descriptions: 3-4 rich sentences of developer INTENT
- NO terms, NO keywords in this file

Run: aoa domains init --file .aoa/domains/intelligence.json

## Step 2: Process Jobs Loop
The init command queued 24 ENRICH jobs. Process them in batches:

LOOP while `aoa jobs pending 3` returns domains:
  1. Get up to 3 pending domain names
  2. For EACH domain, read its description from intelligence.json, then write:
     .aoa/domains/@{name}.json with readable format (one term per line):
     {
       "domain": "@name",
       "terms": {
         "term1": ["kw1", "kw2", "kw3", "kw4", "kw5", "kw6", "kw7"],
         "term2": ["kw1", "kw2", "kw3", "kw4", "kw5", "kw6", "kw7"],
         ...
       }
     }
     Generate 5-7 meaningful terms with 7 keywords each.
  3. Run: aoa jobs process 3
  4. Run: aoa domains link
  5. Continue until no pending jobs

## Step 3: Link and Verify
Run: aoa domains link
Run: aoa jobs
Run: aoa domains | head -5

Return ONLY: "✓ 24 domains ready"
```

## Complete

When done, display:
```
───────────────────────────────────────
⚡ Ready

24 domains │ aoa grep <term>
```
