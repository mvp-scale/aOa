---
name: aoa-setup
description: Generate semantic domains from codebase structure
allowed-tools: Read, Write
---

# AoA Setup

## Display

Show only these messages, nothing else:
```
⚡ aOa Setup
```

Then after reading structure.txt:
```
  Analyzing [N] files...
```

Then after writing JSON:
```
  ✓ [N] domains saved

  Next: aoa quickstart
```

## Process

1. Read `.aoa/structure.txt`
2. Count files, display "Analyzing [N] files..."
3. Generate 20-32 domains from paths/filenames
4. Write JSON to `.aoa/project-domains.json`
5. Display completion

## Constraints

- **ONLY** read `.aoa/structure.txt`
- **NEVER** read source files
- **NO** Task delegation
- **NO** explanations, reasoning, or verbose output
- **NO** echoing file contents or JSON back to user

## Schema
```json
[{"name":"@domain","description":"...","terms":{"term_name":["kw1","kw2","kw3","kw4","kw5","kw6","kw7"]}}]
```

## Quality Guide

**Terms** = Intent labels (what is the developer trying to accomplish?)
- GOOD: `token_lifecycle`, `cart_checkout`, `webhook_ingestion`, `cache_invalidation`
- BAD: `process_data` (too generic), `handle_auth` (just a function name)

**Keywords** = Actual identifiers from the codebase (matched during search)
- GOOD: `token`, `jwt`, `refresh`, `expire`, `validate`, `bearer`, `decode`
- BAD: `token_validation` (underscore), `t` (too short), `processing` (too generic)

## Rules

| Element | Requirement |
|---------|-------------|
| Domains | 20-32 covering full codebase |
| Terms/domain | 5-10, intent-driven, underscores OK |
| Keywords/term | 7-10, single words, 3+ chars, NO underscores |
