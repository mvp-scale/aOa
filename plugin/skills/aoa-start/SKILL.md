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

Analyzing your codebase to build semantic understanding for rapid navigation.

A background task will:
  • Scan project structure
  • Generate ~24 semantic domains
  • Enrich each with terms and keywords

No need to wait - continue working.
Status line shows progress: intelligence 3/24

Once complete, aOa shifts to the intent phase - learning from
your actions and continuously refining its understanding.

Starting background analysis...
```

Then launch ONE background task (see below).

After launching, show:
```
───────────────────────────────────────────────────────
⚡ aOa intelligence started │ Check: aoa domains
```

## Task Prompt

Launch a single background Task with these parameters:
- model: haiku
- run_in_background: true
- allowed_tools: ["Bash(aoa *)", "Write", "Read", "Glob"]

---

You are the aOa Intelligence Agent. Process domains silently in the background.

## PHASE 1: Generate Domains

**Step 1:** Run `aoa tree` to get project structure.

**Step 2:** Plan your domains (20-32 total). For each domain, think:
- What unique area of the codebase does this represent?
- What would someone search for to find this code?

**Step 3:** Write rich descriptions. Each description should be 2-3 sentences that:
- Explain what the domain covers
- Mention key concepts and patterns
- Be specific enough to generate good terms later

**Step 4:** Validate before writing:
- [ ] 20-32 domains total?
- [ ] Each has a unique @name?
- [ ] Each has a rich description (not generic)?
- [ ] No overlapping domains?

**Step 5:** Write JSON array to `.aoa/domains/intelligence.json`:
```json
[{"name":"@search","description":"Code search and symbol lookup. Includes grep, egrep, find, and locate commands. Pattern matching and result ranking with O(1) instant indexing."}]
```

**Step 6:** Run `aoa domains init --file .aoa/domains/intelligence.json`

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

## PHASE 2: Enrich Domains (One at a Time)

**CRITICAL: Process each domain completely before moving to the next.**
This updates the status line after each domain, giving users real-time progress.

**Step 1:** Get unenriched domains:
```bash
aoa domains --json | jq -r '.domains[] | select(.enriched != true) | .name'
```

**Step 2:** For EACH domain in the list, do ALL of the following before moving to the next:

  **2a. Generate terms and keywords** for this ONE domain.

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

  **2d. Build IMMEDIATELY:** Run `aoa domains build @domain_name`

  This updates the enrichment count. Status line now shows progress (e.g., `intelligence 5/24`).

**Step 3:** Move to the next domain. Repeat Step 2.

**DO NOT batch all JSON writes before building. Each domain must be built immediately after its JSON is written.**

### Quality Examples (Terms & Keywords)

GOOD terms: `token_lifecycle`, `session_management`, `credential_validation`
BAD terms: `handle_auth`, `process_data` (too generic, sound like function names)

GOOD keywords: `jwt`, `bearer`, `oauth`, `credential`, `bcrypt`
BAD keywords: `get`, `set`, `data`, `file`, `handle` (match everything)

**The test:** Would this keyword ONLY match code in THIS domain? If it could match general code anywhere, don't use it.

---

## Completion

When all domains are enriched, return: "Intelligence complete: X domains enriched"

---

## Notes

- This runs in background - user continues working
- Status line shows `intelligence X/24` progress (updates after each `aoa domains build`)
- Process domains ONE AT A TIME: generate → write → build → next
- DO NOT generate all JSONs first then build - this breaks progress updates
- When complete, status shifts to `intent` phase
