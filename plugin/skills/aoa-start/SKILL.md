---
name: aoa-start
description: Initialize aOa with semantic domain analysis
allowed-tools: Task, Bash, Write
---

# aOa Start

Display:
```
⚡ aOa Building Intelligence

Building semantic understanding of your codebase...
This runs in the background. Continue working.
```

## Execute

Spawn ONE background Task:
- description: `⚡ aOa Building intelligence`
- run_in_background: true
- model: opus

**NON-BLOCKING: After launching the task, STOP. Do not call TaskOutput. Do not wait. Return control to the user immediately. The completion notification arrives automatically.**

Prompt:
```
You are setting up aOa intelligence. Complete ONLY these 3 steps. Do NOT explore the codebase, read files, or run any commands not listed below.

## Step 1: Generate Intelligence
Run: aoa tree
Run: mkdir -p .aoa/domains

Using ONLY the aoa tree output, generate 24 semantic domains.
Write to .aoa/domains/intelligence.json as a flat JSON array:
[{"name": "@cli", "description": "3-4 sentences of what it does, why developers work here, what problems it solves"}, ...24 total]

Rules:
- Names: @lowercase with @ prefix
- Descriptions: 3-4 rich sentences of developer INTENT
- NO terms, NO keywords in this file

Run: aoa domains init --file .aoa/domains/intelligence.json

## Step 2: Generate Domain Files
For EACH of the 24 domains in intelligence.json, Write the file .aoa/domains/@{name}.json:
{
  "domain": "@name",
  "terms": {
    "term1": ["kw1", "kw2", "kw3", "kw4", "kw5", "kw6", "kw7"],
    "term2": ["kw1", "kw2", "kw3", "kw4", "kw5", "kw6", "kw7"],
    ...5-7 terms
  }
}
Generate terms and keywords based on the domain description from intelligence.json.

## Step 3: Build and Verify
Run: aoa domains build --all
Run: aoa domains clean
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
