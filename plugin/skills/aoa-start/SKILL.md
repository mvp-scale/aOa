---
name: aoa-start
description: Initialize aOa with semantic domain analysis
allowed-tools: Task, Bash, Write
---

# aOa Start

## User Output

Show this welcome message:

```
⚡ Welcome to aOa

Analyzing your codebase to build semantic understanding.

This will:
  • Scan project structure
  • Generate 24 core semantic domains
  • Queue enrichment jobs for background processing

Jobs process automatically via hooks. Check progress: aoa jobs

Starting analysis...
```

Then launch ONE background Task (see below).

After launching, show:
```
───────────────────────────────────────────────────────
⚡ aOa intelligence started │ Check: aoa jobs
```

## Task Prompt

Launch a single background Task with these parameters:
- model: haiku
- run_in_background: true
- allowed_tools: ["Bash(aoa *)", "Bash(curl *)", "Write", "Read", "Glob"]

---

You are the aOa Intelligence Agent. Generate domains and queue them for enrichment.

## PHASE 1: Generate Domains

**Step 1:** Run `aoa tree` to get project structure.

**Step 2:** Plan your domains (24 total). For each domain, think:
- What unique area of the codebase does this represent?
- What would someone search for to find this code?

**Step 3:** Write rich descriptions. Each description should be 2-3 sentences that:
- Explain what the domain covers
- Mention key concepts and patterns
- Be specific enough to generate good terms later

**Step 4:** Validate before writing:
- [ ] 24 domains total?
- [ ] Each has a unique @name?
- [ ] Each has a rich description (not generic)?
- [ ] No overlapping domains?

**Step 5:** Write JSON array to `.aoa/domains/intelligence.json`:
```json
[
  {"name":"@search","description":"Code search and symbol lookup. Includes grep, egrep, find, and locate commands. Pattern matching and result ranking with O(1) instant indexing."},
  {"name":"@api","description":"REST API endpoints and HTTP handlers. Request routing, response formatting, and middleware."}
]
```

### Quality Examples (Domains)

GOOD domain:
```json
{"name":"@authentication","description":"User authentication and session management. Handles login, logout, JWT tokens, OAuth flows, and credential validation. Security-critical code paths."}
```

BAD domain:
```json
{"name":"@utils","description":"Utility functions."}
```
(Too generic, description won't help generate good terms)

---

## PHASE 2: Initialize and Queue

**Step 1:** Initialize domains in Redis:
```bash
aoa domains init --file .aoa/domains/intelligence.json
```

**Step 2:** Get project ID:
```bash
cat .aoa/home.json | jq -r '.project_id'
```

**Step 3:** Push enrichment jobs to queue. Build the JSON from your intelligence.json:
```bash
PROJECT_ID=$(cat .aoa/home.json | jq -r '.project_id')
DOMAINS=$(cat .aoa/domains/intelligence.json)
curl -s -X POST "localhost:8081/jobs/push/enrich" \
  -H "Content-Type: application/json" \
  -d "{\"project_id\":\"${PROJECT_ID}\",\"domains\":${DOMAINS}}"
```

**Step 4:** Verify jobs were queued:
```bash
aoa jobs status
```

You should see output like: `X pending │ 0 active │ 0 complete`

---

## PHASE 3: Process Queue (Enrichment Loop)

**CRITICAL: Loop until queue is empty. Never stop at a fixed number.**

```
WHILE aoa jobs shows pending > 0:
    1. Get next batch of pending jobs (up to 3)
    2. For each job, generate terms and keywords
    3. Write enrichment file and build domain
    4. Repeat until queue empty
```

**Step 1:** Check for pending work:
```bash
aoa jobs pending 3
```

If no output, queue is empty - you're done.

**Step 2:** For EACH domain returned, do ALL of the following:

  **2a. Generate terms and keywords** for this domain.

  Think about:
  - What are 5-10 distinct concepts within this domain?
  - For each concept, what specific code identifiers would match?
  - Avoid common words that cause noise (get, set, data, file, handle, process)

  **2b. Validate** before writing:
  - [ ] 5-10 terms?
  - [ ] 7-10 keywords per term?
  - [ ] Keywords are specific, not generic?

  **2c. Write** to `.aoa/domains/@domain_name.json`:
  ```json
  {
    "domain": "@authentication",
    "terms": {
      "token_lifecycle": ["jwt", "bearer", "refresh", "expire", "validate", "decode", "claims"],
      "session_management": ["session", "logout", "cookie", "persist", "invalidate", "timeout"],
      "credential_validation": ["password", "hash", "bcrypt", "verify", "credential", "secret"]
    }
  }
  ```

  **2d. Build:** Run `aoa domains build @domain_name`

**Step 3:** Process jobs from queue:
```bash
aoa jobs process 3
```

**Step 4:** GO BACK TO STEP 1 and check for more pending work.

**TERMINATION CONDITION:** Only stop when `aoa jobs pending` returns no output (queue empty).

### Quality Examples (Terms & Keywords)

GOOD terms: `token_lifecycle`, `session_management`, `credential_validation`
BAD terms: `handle_auth`, `process_data` (too generic, sound like function names)

GOOD keywords: `jwt`, `bearer`, `oauth`, `credential`, `bcrypt`
BAD keywords: `get`, `set`, `data`, `file`, `handle` (match everything)

**The test:** Would this keyword ONLY match code in THIS domain? If it could match general code anywhere, don't use it.

---

## Completion

When queue is empty, return: "Intelligence complete: X domains enriched"

Run `aoa jobs status` to confirm: `0 pending │ 0 active │ X complete`

---

## Recovery

If the process stops for any reason:
1. Run `aoa jobs status` to see what's pending
2. Run `aoa jobs pending` to see which domains need work
3. Resume from PHASE 3 Step 1

The queue persists in Redis - nothing is lost.

---

## Notes

- Queue is the source of truth - not file counts or hardcoded numbers
- Process in batches of 3 to avoid overwhelming the system
- Each `aoa jobs process` marks jobs complete in Redis
- If jobs fail, use `aoa jobs retry` to requeue them
