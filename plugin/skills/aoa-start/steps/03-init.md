# Step 03: Initialize and Queue

## Goal
Load domains into Redis and queue enrichment jobs.

## Actions

### 1. Initialize domains
```bash
aoa domains init --file .aoa/domains/intelligence.json
```

### 2. Queue enrichment jobs
```bash
aoa jobs enrich
```

### 3. Check status
```bash
aoa jobs status
```

## Success
Shows N pending jobs (should be 24).

## Next
`steps/04-expand.md`
