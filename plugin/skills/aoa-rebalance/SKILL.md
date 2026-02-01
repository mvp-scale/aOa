---
name: aoa-rebalance
description: Generate intent domains from recent activity
allowed-tools: Task, Bash, Read, Write
---

# aOa Rebalance

Display:
```
⚡ aOa Intent Learning

Analyzing last 25 prompts for new domains...
This runs in the background. Continue working.
```

## Execute

Spawn ONE background Task:
- description: `⚡ Intent learning`
- run_in_background: true
- model: haiku

Prompt:
```
Generate 3 new semantic domains from recent developer prompts. Do NOT output intermediate command results.

## Step 1: Gather Prompts and Context
Run `aoa cc prompts --raw` to get the last 25 user prompts.
Run `aoa domains --json` to get existing domains (don't duplicate these).

## Step 2: Analyze Prompts and Generate Domains
Analyze the prompts to understand what the developer is DOING (not generic concepts).

Write .aoa/domains/intent.json as a flat JSON array with 3 NEW domains:
[
  {
    "domain": "@domain_name",
    "terms": {
      "term_name": ["keyword1", "keyword2", "keyword3", "keyword4", "keyword5"],
      "another_term": ["keyword1", "keyword2", "keyword3", "keyword4", "keyword5"]
    }
  }
]

Rules:
- Generate exactly 3 NEW domains based on prompt activity
- Each domain should have 5-7 terms
- Each term should have 5-7 keywords
- Domain names: lowercase with underscores, start with @
- Terms: SINGLE WORDS only (e.g., "tracking", "validation", "generation")
- Keywords: SINGLE WORDS only, NO underscores, NO phrases
- Focus on SPECIFIC user activities from the prompts
- DO NOT duplicate existing domains

## Step 3: Stage Proposals
Run: aoa domains stage
Run: aoa domains | head -10

The staged domains will accumulate hits. When they get enough hits through usage, they promote to core domains.

Return ONLY: "✓ 3 domains staged"
```

## Complete

When done, display:
```
───────────────────────────────────────
⚡ Intent staged

3 domains │ Promotes with usage
```
