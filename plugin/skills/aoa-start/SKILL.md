---
name: aoa-start
description: Initialize aOa with semantic domain analysis
allowed-tools: Task, Read, Bash, Write
---

# aOa Intelligence

Show:
```
⚡ aOa Intelligence

Analyzing your codebase to build semantic understanding.
This is a one-time setup that takes ~2 minutes.

───────────────────────────────────────────────────────
```

---

## Phase 1: Understanding Your Code

### Step 1a: Gather (Haiku)

Spawn ONE Task:
- description: `⚡ Scanning project structure`
- model: haiku

Prompt: `Run aoa tree and aoa tree services. Return the combined output.`

### Step 1b: Think (You - Opus)

Based on the tree output, mentally generate 24 semantic domains.
DO NOT write them yet - just decide what they should be.

Format: `[{"name": "@lowercase", "description": "2-4 sentences of INTENT"}]`

### Step 1c: Write & Init (Haiku)

Spawn ONE Task:
- description: `⚡ Creating 24 semantic domains`
- model: haiku

Prompt (include the 24 domains you generated):
```
Write this JSON to /home/corey/aOa/.aoa/domains/intelligence.json:
[THE 24 DOMAINS JSON HERE]

Then run:
aoa domains init --file .aoa/domains/intelligence.json
aoa jobs enrich
```

---

## Phase 2: Generating Domain Files

**Spawn ALL 4 Tasks in ONE response.**

| Task | Description | Model |
|------|-------------|-------|
| 1 | ⚡ Writing domains 1-6 | haiku |
| 2 | ⚡ Writing domains 7-12 | haiku |
| 3 | ⚡ Writing domains 13-18 | haiku |
| 4 | ⚡ Writing domains 19-24 | haiku |

Each Haiku writes 6 @domain.json files with 5 term groups of 7 keywords.

---

## Phase 3: Enrichment

Spawn ONE Task:
- description: `⚡ Enriching 24 domains`
- model: haiku

Prompt:
```
Run 3 parallel batches:

Batch 1:
aoa domains build @search & aoa domains build @indexer & aoa domains build @cli & aoa domains build @gateway & aoa domains build @redis & aoa domains build @intent & aoa domains build @hooks & aoa domains build @domains & wait

Batch 2:
aoa domains build @jobs & aoa domains build @status & aoa domains build @ranking & aoa domains build @session & aoa domains build @docker & aoa domains build @proxy & aoa domains build @config & aoa domains build @skills & wait

Batch 3:
aoa domains build @analytics & aoa domains build @prefetch & aoa domains build @outline & aoa domains build @testing & aoa domains build @build & aoa domains build @install & aoa domains build @health & aoa domains build @learner & wait

Then run:
aoa jobs process 24
aoa jobs status
```

---

## Complete

Show:
```
───────────────────────────────────────────────────────
⚡ aOa Intelligence Complete

24 semantic domains now understand your codebase.
Try: aoa grep <term>
```
