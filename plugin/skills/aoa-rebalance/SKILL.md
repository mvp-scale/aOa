---
name: aoa-rebalance
description: Generate intent domains from accumulated usage signals
allowed-tools: Task, Bash, Write
---

# aOa Rebalance

Display:
```
⚡ aOa Rebalancing Domains

Analyzing usage signals for new domains...
This runs in the background. Continue working.
```

## Execute

Spawn ONE background Task:
- description: `⚡ aOa Rebalancing domains`
- run_in_background: true
- model: haiku

Prompt:
```
You are generating intent domains. Complete ONLY these 3 steps.

CRITICAL: Do NOT explore the codebase. Do NOT read files. Do NOT run any commands not listed below. If a command fails, return "✗ failed" and STOP. Do NOT retry, debug, or investigate.

## Step 1: Gather Data
Run: aoa bigrams --recent
Run: aoa domains --names

## Step 2: Generate Domains
Using ONLY the bigrams and domain names output from Step 1, generate 3 new domains.

Example bigrams and what they suggest:
- "hit:tracking" → @metrics domain with terms: tracking, hits, counting
- "domain:hits" → @domains domain with terms: hits, learning, promotion

Write to .aoa/domains/intent.json:
[
  {
    "domain": "@domain_name",
    "description": "brief description",
    "terms": {
      "term1": ["kw1", "kw2", "kw3", "kw4", "kw5"],
      "term2": ["kw1", "kw2", "kw3", "kw4", "kw5"]
    }
  }
]

Rules:
- Exactly 3 NEW domains (do NOT duplicate existing domain names)
- 5-7 terms per domain, SINGLE WORDS only
- 5-7 keywords per term, SINGLE WORDS only, NO underscores, NO phrases
- Extract keywords from BOTH sides of bigrams

## Step 3: Load Domains
Run: aoa domains rebalance

Return ONLY: "✓ 3 domains added"
```

## Complete

When done, display:
```
───────────────────────────────────────
⚡ Intent added

3 domains │ Competes by hits
```
