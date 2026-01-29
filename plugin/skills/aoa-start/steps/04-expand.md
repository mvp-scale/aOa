# Step 04: Parallel Domain Expansion

## Goal
Spawn 4 Haiku Tasks to write 24 @domain.json files in parallel.

## Model
Haiku (fast execution)

## Orchestrator Action
Spawn 4 Tasks IN ONE RESPONSE:

| Task | Description | Domains |
|------|-------------|---------|
| 1 | ⚡ Domains 1-6 | First 6 from intelligence.json |
| 2 | ⚡ Domains 7-12 | Next 6 |
| 3 | ⚡ Domains 13-18 | Next 6 |
| 4 | ⚡ Domains 19-24 | Last 6 |

## Task Parameters
- model: haiku
- subagent_type: general-purpose
- run_in_background: false

## Haiku Task Responsibilities
Each Haiku writes 6 @domain.json files using parallel Write calls.

File format:
```json
{
  "domain": "@name",
  "terms": {
    "group1": ["word1", "word2", "word3", "word4", "word5", "word6", "word7"],
    "group2": ["word1", "word2", "word3", "word4", "word5", "word6", "word7"],
    "group3": ["word1", "word2", "word3", "word4", "word5", "word6", "word7"],
    "group4": ["word1", "word2", "word3", "word4", "word5", "word6", "word7"],
    "group5": ["word1", "word2", "word3", "word4", "word5", "word6", "word7"]
  }
}
```

Rules: 5 groups, 7 keywords each, single lowercase words relevant to domain.

## Success
All 24 @domain.json files exist in /home/corey/aOa/.aoa/domains/

## Target Time
~15 seconds (parallel execution)
