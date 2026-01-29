# Step 02: Generate Intelligence

## Goal
Create 24 semantic domains that capture developer intent for each area of the codebase.

## Model
Opus (quality thinking)

## Input
TREE_OUTPUT provided by Haiku orchestrator (already gathered).

Opus does NOT run `aoa tree` - the data is handed to it.

## Action
Write file: `/home/corey/aOa/.aoa/domains/intelligence.json`

## Format
```json
[
  {
    "name": "@lowercase",
    "description": "2-4 sentences answering: What was the developer trying to accomplish?"
  }
]
```

## Quality
- 24 domains exactly
- Names: @lowercase (e.g., @search, @redis, @intent)
- Descriptions capture INTENT, not just "what it does"

Good: "O(1) symbol lookup via inverted index. Powers grep, egrep, instant search. Tokenization, file watching, index rebuilds. Goal: sub-10ms response."

Weak: "Search functionality for the codebase."

## Success
File exists with 24 domain objects (name + description only).

## Return
List of 24 domain names (for Phase 2 to use).
