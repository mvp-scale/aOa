---
name: aoa-rebalance
description: Generate intent domains from accumulated usage signals
allowed-tools: Task, Bash, Write
---

# aOa Rebalance

Display:
```
⚡ aOa Intent Learning

Analyzing usage signals for new domains...
This runs in the background. Continue working.
```

## Execute

Spawn ONE background Task:
- description: `⚡ Intent learning`
- run_in_background: true
- model: haiku

Prompt:
```
Generate 3 new semantic domains from bigram usage signals.

## Step 1: Clear trigger and gather data
Run these commands (do NOT show output):
```bash
aoa domains clear-pending
aoa bigrams --limit 50
aoa domains --names
```

## Step 2: Generate domains
The bigrams show what the developer is ACTUALLY working on based on accumulated usage patterns.

Example bigrams and what they suggest:
- "hit:tracking" → @metrics domain with terms: tracking, hits, counting
- "domain:hits" → @domains domain with terms: hits, learning, promotion
- "gap:analysis" → @analysis domain with terms: gap, research, investigation

Write to /home/corey/aOa/.aoa/domains/intent.json:
```json
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
```

Rules:
- Generate exactly 3 NEW domains based on bigram signals
- Each domain should have 5-7 terms
- Each term should have 5-7 keywords (derived from bigram words)
- Domain names: lowercase with underscores, start with @
- Terms: SINGLE WORDS only (e.g., "tracking", "validation", "generation")
- Keywords: SINGLE WORDS only, NO underscores, NO phrases
- Extract keywords from BOTH sides of the bigram (e.g., "hit:tracking" → hit, tracking)
- DO NOT duplicate existing domain names

## Step 3: Load domains
```bash
aoa domains load-intent
```

New domains enter at context tier and compete for top 24 by hits. Bad domains naturally fall off.

Return ONLY: "✓ 3 domains added"
```

## Complete

When done, display:
```
───────────────────────────────────────
⚡ Intent added

3 domains │ Competes by hits
```
