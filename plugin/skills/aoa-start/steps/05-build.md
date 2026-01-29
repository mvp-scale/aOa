# Step 05: Build All Domains (Enrichment)

## Goal
Build all 24 domains and process the job queue.

## Model
Haiku orchestrator (fast execution)

## Action: Build in Parallel Batches

Run build commands in 3 batches of 8 (parallel within each batch):

### Batch 1 (8 parallel)
```bash
aoa domains build @name1 & aoa domains build @name2 & aoa domains build @name3 & aoa domains build @name4 & aoa domains build @name5 & aoa domains build @name6 & aoa domains build @name7 & aoa domains build @name8 & wait
```

### Batch 2 (8 parallel)
```bash
aoa domains build @name9 & aoa domains build @name10 & ... & wait
```

### Batch 3 (8 parallel)
```bash
aoa domains build @name17 & aoa domains build @name18 & ... & wait
```

**Note:** Use actual domain names from intelligence.json.

## Process All Jobs
```bash
aoa jobs process 24
```

## Status Check
```bash
aoa jobs status
```

## Success
Shows 24 complete, 0 pending.

## Target Time
~20 seconds (parallel batches)
