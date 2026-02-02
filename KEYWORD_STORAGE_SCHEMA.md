# aOa Keyword Storage Schema

## Overview

This document details all Redis keys involved in keyword tracking, their structure, lifecycle, and relationships.

---

## Core Tables

### 1. Gap Keywords (Orphans Waiting for Assignment)

**Purpose**: Track keywords found in search queries that returned 0 results

**Key**: `aoa:{project_id}:gap_keywords`

**Type**: Redis Set

**Populated By**:
- `learner.record_gap_keyword(keyword)` - when grep returns 0 results
- Called from: `indexer.py:3592` during `/grep` search

**Consumed By**:
- `learner.rebalance_keywords()` - processes gaps every 25 prompts
- Called from: `indexer.py:4565`

**Lifecycle**:
```
User searches for "xyz" (no results found)
    ↓
SADD aoa:proj:gap_keywords "xyz"
    ↓
(next rebalance at prompt 25)
    ↓
_find_best_term_for_keyword("xyz")
    ↓
Assign to term "abc"
    ↓
SREM aoa:proj:gap_keywords "xyz"
    ↓
SET aoa:proj:keyword:xyz:term "abc"
```

**Related**:
- `learner.get_gap_keywords(limit=30)` - fetch N gaps for rebalance
- `learner.clear_gap_keyword(keyword)` - remove from gaps after assignment

---

### 2. Keyword-to-Term Mapping

**Purpose**: Store which term owns a keyword (one-to-one mapping)

**Key**: `aoa:{project_id}:keyword:{keyword_lower}:term`

**Type**: Redis String (scalar)

**Value**: Term name (e.g., "session_management")

**Populated By**:
- `learner.add_keyword_to_term(keyword, term)` - during enrichment or rebalance
- Called from: `learner.py:2107` (rebalance) or `learner.py:630` (enrich)

**Used By**:
- `learner.get_term_for_keyword(keyword)` - quick lookup
- `matchers.KeywordMatcher` - build automaton from all keywords

**Lifecycle**:
```
enrichment: set {keyword, term}
    ↓
SET aoa:proj:keyword:auth:term "session_control"
    ↓
(persists until manual override or domain changes)
```

**Schema**:
```redis
SET aoa:proj:keyword:auth:term "session_control"
SET aoa:proj:keyword:login:term "session_control"
SET aoa:proj:keyword:session:term "session_control"
SET aoa:proj:keyword:deploy:term "devops"
SET aoa:proj:keyword:docker:term "devops"
```

---

### 3. Keyword-to-Domain Mapping

**Purpose**: Store which domain a keyword belongs to (via its term)

**Key**: `aoa:{project_id}:keyword:{keyword_lower}:domain`

**Type**: Redis String (scalar)

**Value**: Domain name (e.g., "@authentication")

**Populated By**:
- Implicit: Set during enrichment when term→domain mapping exists
- `learner.enrich_domain()` - caller stores this during term addition
- Called from: `domains_api.py:243` enrichment endpoint

**Used By**:
- Metadata queries (informational)
- Predictions (tag-based ranking)

**Schema**:
```redis
SET aoa:proj:keyword:auth:domain "@authentication"
SET aoa:proj:keyword:login:domain "@authentication"
SET aoa:proj:keyword:session:domain "@authentication"
SET aoa:proj:keyword:deploy:domain "@devops"
SET aoa:proj:keyword:docker:domain "@devops"
```

**Relation**:
```
keyword:term            keyword:domain
auth → session_control  auth → @authentication
        ↑
        term:domain mapping
        session_control → @authentication
```

---

### 4. Term Keywords (Reverse Index)

**Purpose**: Store all keywords belonging to a term with relevance scores

**Key**: `aoa:{project_id}:term:{term_name}:keywords`

**Type**: Redis Sorted Set (with scores)

**Score**: 1.0 initially, incremented by usage (cohit)

**Populated By**:
- `learner.add_term_keywords(term, keywords)` - during enrichment
- Called from: `learner.py:630`

**Used By**:
- Informational: List all keywords in a term
- Sorting: Rank keywords within term by score

**Lifecycle**:
```
Haiku enriches @authentication with term "session_control"
keywords = ["auth", "login", "session", "token"]
    ↓
ZADD aoa:proj:term:session_control:keywords {
  "auth": 1.0,
  "login": 1.0,
  "session": 1.0,
  "token": 1.0
}
    ↓
(later when keywords used in searches)
    ↓
ZINCRBY aoa:proj:term:session_control:keywords 1 "auth"  (now 2.0)
ZINCRBY aoa:proj:term:session_control:keywords 1 "session" (now 2.0)
```

**Schema**:
```redis
ZADD aoa:proj:term:session_control:keywords {
  "auth": 1.0,
  "login": 1.0,
  "session": 1.0,
  "token": 1.0
}

ZADD aoa:proj:term:devops:keywords {
  "deploy": 1.0,
  "docker": 1.0,
  "kubernetes": 1.0,
  "cicd": 1.0
}
```

---

### 5. Co-Occurrence Counters (Keyword-Term)

**Purpose**: Track how many times a keyword and term co-occur (appear together in search results)

**Key**: `aoa:{project_id}:cohit:kw_term`

**Type**: Redis Hash

**Field**: `{keyword}:{term}`

**Value**: Integer count

**Populated By**:
- `learner.increment_cohit(keyword, term, domain)` - during result formatting
- Called from: `indexer.py:627` during grep result enrichment

**Used By**:
- Rebalance: `learner._find_best_term_for_keyword()` - score candidates
- Analytics: Track keyword→term association strength

**Lifecycle**:
```
User searches for "auth"
Search results contain 20 files
Each result scanned by KeywordMatcher
    ↓
For each result mentioning "auth", "session", "token":
  HINCRBY aoa:proj:cohit:kw_term "auth:session_control" 1
  HINCRBY aoa:proj:cohit:kw_term "session:session_control" 1
  HINCRBY aoa:proj:cohit:kw_term "token:session_control" 1
    ↓
Result:
  "auth:session_control" = 18  (appeared in 18 of 20 results)
  "session:session_control" = 20
  "token:session_control" = 19
```

**Schema**:
```redis
HSET aoa:proj:cohit:kw_term {
  "auth:session_control": 18,
  "login:session_control": 15,
  "session:session_control": 20,
  "token:session_control": 19,
  "deploy:devops": 25,
  "docker:devops": 23
}
```

**Query**:
```redis
HGET aoa:proj:cohit:kw_term "auth:session_control"  # Returns: 18
HGETALL aoa:proj:cohit:kw_term  # All cohits
```

---

### 6. Co-Occurrence Counters (Term-Domain)

**Purpose**: Track how many times a term and domain co-occur

**Key**: `aoa:{project_id}:cohit:term_domain`

**Type**: Redis Hash

**Field**: `{term}:{domain}`

**Value**: Integer count

**Populated By**:
- `learner.increment_cohit(keyword, term, domain)` - same call as kw_term
- Called from: `indexer.py:627`

**Used By**:
- Analytics only (informational)
- Redundant since term→domain is fixed mapping

**Schema**:
```redis
HSET aoa:proj:cohit:term_domain {
  "session_control:@authentication": 80,
  "access_control:@authentication": 45,
  "devops:@devops": 120
}
```

---

### 7. Keyword Search Counters

**Purpose**: Track how many times each keyword was searched

**Key**: `aoa:{project_id}:keyword:{keyword_lower}:searches`

**Type**: Redis String (scalar counter)

**Value**: Integer

**Populated By**:
- `learner.increment_keyword_search(keyword)` - when keyword used in search
- Called from: `learner.py:2059`

**Used By**:
- Analytics: "How often was this keyword searched?"
- Ranking: boost frequently-searched keywords

**Lifecycle**:
```
User: aoa grep auth
    ↓
increment_keyword_search("auth")
    ↓
INCR aoa:proj:keyword:auth:searches  (now 1)

User: aoa grep auth   (again)
    ↓
increment_keyword_search("auth")
    ↓
INCR aoa:proj:keyword:auth:searches  (now 2)
```

**Schema**:
```redis
SET aoa:proj:keyword:auth:searches "42"
SET aoa:proj:keyword:login:searches "38"
SET aoa:proj:keyword:deploy:searches "15"
SET aoa:proj:keyword:docker:searches "12"
```

**Query**:
```redis
GET aoa:proj:keyword:auth:searches  # Returns: "42"
```

---

### 8. Keyword Access Counters

**Purpose**: Track how many files were accessed as a result of this keyword being used

**Key**: `aoa:{project_id}:keyword:{keyword_lower}:accesses`

**Type**: Redis String (scalar counter)

**Value**: Integer

**Populated By**:
- `learner.increment_keyword_access(keyword)` - when search leads to file access
- Called from: `learner.py:2064`

**Used By**:
- Analytics: "How productive is this keyword?"
- Ranking: boost keywords that lead to productive searches

**Lifecycle**:
```
User: aoa grep auth  (finds 20 files)
All 20 accessed by Claude
    ↓
For each file accessed:
  increment_keyword_access("auth")
    ↓
INCRBY aoa:proj:keyword:auth:accesses 20
```

**Schema**:
```redis
SET aoa:proj:keyword:auth:accesses "342"  (342 files accessed from auth searches)
SET aoa:proj:keyword:login:accesses "295"
SET aoa:proj:keyword:deploy:accesses "142"
```

---

### 9. Keyword Hit Counts

**Purpose**: Track total hits (result count) from searches using a keyword

**Key**: `aoa:{project_id}:keyword_hits`

**Type**: Redis Hash

**Field**: `{keyword_lower}`

**Value**: Integer count

**Populated By**:
- `learner.increment_keyword_hits(keywords, amount)` - during result formatting
- Called from: `indexer.py:627` during grep result enrichment

**Used By**:
- Rebalance: Calculate promotion_ratio = cohit(kw,term) / hits(kw)
- Analytics: Overall keyword productivity

**Schema**:
```redis
HSET aoa:proj:keyword_hits {
  "auth": 150,
  "login": 140,
  "session": 135,
  "token": 125,
  "deploy": 45,
  "docker": 42
}
```

**Query**:
```redis
HGET aoa:proj:keyword_hits "auth"  # Returns: "150"
HGETALL aoa:proj:keyword_hits  # All keyword hit counts
```

**Used In Promotion Ratio**:
```python
def calculate_promotion_ratio(keyword, term):
    cohit = get_cohit_kw_term(keyword, term)
    total_hits = get_keyword_hits(keyword)
    ratio = cohit / total_hits if total_hits > 0 else 0
    # ratio = 0.72 means: "When searching for this keyword,
    #                       this term appears in 72% of results"
    return ratio
```

---

### 10. Last Rebalance Timestamp

**Purpose**: Track when rebalance last ran (for throttling)

**Key**: `aoa:{project_id}:last_rebalance`

**Type**: Redis String (scalar)

**Value**: Unix timestamp (seconds since epoch)

**Populated By**:
- `learner.rebalance_keywords()` - at end of rebalance
- Called from: `indexer.py:4565`

**Used By**:
- `learner.should_rebalance()` - check if enough time has passed

**Lifecycle**:
```
Rebalance runs at prompt 25
    ↓
SET aoa:proj:last_rebalance "1704067200"  (unix timestamp)
    ↓
(next rebalance at prompt 50)
    ↓
GET aoa:proj:last_rebalance  # Check timestamp to throttle
```

**Schema**:
```redis
SET aoa:proj:last_rebalance "1704067200"
```

---

## Supporting Tables

### Intent Records

**Purpose**: Store all tool usage events for analysis and prediction

**Key**: `aoa:{project_id}:intent:recent`

**Type**: Redis Sorted Set

**Score**: Timestamp (for time-series ordering)

**Member**: JSON string with event details

**Populated By**:
- `indexer.py:5181` - POST /intent endpoint
- Called from: `intent-capture.py:335`

**Schema**:
```redis
ZADD aoa:proj:intent:recent 1704067200 '{
  "tool": "Grep",
  "files": ["src/auth.py", "src/login.py"],
  "tags": ["#authentication", "#security"],
  "output_size": 1250,
  "timestamp": 1704067200,
  "project_id": "local",
  "tool_use_id": "123e4567-e89b-12d3-a456-426614174000"
}'

ZADD aoa:proj:intent:recent 1704067205 '{
  "tool": "Read",
  "files": ["src/auth.py"],
  "tags": ["#authentication"],
  "timestamp": 1704067205,
  "project_id": "local"
}'
```

**Gap**: Missing `keywords` field! (Gap #2)

---

### Prediction Cache

**Purpose**: Cache prediction results for same keyword combinations

**Key**: `aoa:context:{sorted_keyword_list}`

**Type**: Redis String (JSON)

**TTL**: 3600 seconds (1 hour)

**Populated By**:
- `indexer.py:7100` - after /context result generated
- Called from: caching logic

**Schema**:
```redis
SET aoa:context:auth:bug:login 3600 '{
  "intent": "fix the auth bug in login",
  "keywords": ["auth", "bug", "login"],
  "tags_matched": ["#authentication", "#security"],
  "files": [
    {"path": "src/auth.py", "confidence": 0.92},
    {"path": "src/login.py", "confidence": 0.88}
  ],
  "ms": 12.5,
  "cached": true
}'
```

---

## Full Example: Keyword Lifecycle

### Setup Phase
```
Project created
Domain enriched with @authentication

Haiku provides:
  domain: "@authentication"
  terms:
    "session_control": ["auth", "login", "session", "token", "bearer"]
    "access_control": ["permission", "role", "grant", "deny"]
    "encryption": ["hash", "salt", "encrypt", "decrypt"]
```

### Enrichment
```
learner.enrich_domain("@authentication", {
  "session_control": ["auth", "login", "session", "token", "bearer"],
  "access_control": ["permission", "role", "grant", "deny"],
  "encryption": ["hash", "salt", "encrypt", "decrypt"]
})

Results in Redis:
  SET aoa:proj:keyword:auth:term "session_control"
  SET aoa:proj:keyword:auth:domain "@authentication"
  ZADD aoa:proj:term:session_control:keywords "auth" 1.0
  ... (repeat for all keywords)
```

### First Search
```
User: aoa grep auth

Results: 20 files found

Per-result processing:
  For each file:
    KeywordMatcher scans content
    Finds: ["auth", "session", "token"]

    increment_cohit("auth", "session_control", "@authentication")
    increment_cohit("session", "session_control", "@authentication")
    increment_cohit("token", "session_control", "@authentication")

Redis updates:
  HINCRBY aoa:proj:cohit:kw_term "auth:session_control" 1
  HINCRBY aoa:proj:cohit:kw_term "session:session_control" 1
  HINCRBY aoa:proj:cohit:kw_term "token:session_control" 1
  HINCRBY aoa:proj:keyword_hits "auth" 20
  INCR aoa:proj:keyword:auth:searches
```

### Multiple Searches
```
After 10 searches for "auth", 20 results each:

aoa:proj:cohit:kw_term:
  "auth:session_control": 185  (found in 185 out of 200 results)

aoa:proj:keyword_hits:
  "auth": 200  (total results from all searches)

aoa:proj:keyword:auth:searches:
  "42"  (was searched 42 times)

Promotion ratio:
  P(session_control | auth) = 185 / 200 = 0.925
  Meaning: Session control term appears in 92.5% of auth results
```

### Rebalance (if gap keyword discovered)
```
Scenario: User searches for "bearer" (not in enriched keywords yet)
Results: 0 files

Steps:
  1. SADD aoa:proj:gap_keywords "bearer"

After 25 prompts, rebalance runs:
  2. gaps = ["bearer"]

  3. _find_best_term_for_keyword("bearer")
     - Check all terms for overlap
     - "bearer" vs "token" → high overlap
     - "bearer" vs "session" → medium overlap
     - Best: "token" in "session_control"

  4. add_keyword_to_term("bearer", "session_control")
     - SET aoa:proj:keyword:bearer:term "session_control"
     - ZADD aoa:proj:term:session_control:keywords "bearer" 1.0

  5. SREM aoa:proj:gap_keywords "bearer"

  6. Rebuild KeywordMatcher automaton
```

---

## Query Patterns

### Get All Keywords for a Term
```redis
ZRANGE aoa:proj:term:session_control:keywords 0 -1 WITHSCORES
# Returns: [("auth", 1.0), ("login", 1.0), ("session", 1.0), ...]
```

### Get All Keywords in a Domain (via terms)
```
For each term in domain:
  ZRANGE aoa:proj:term:{term}:keywords 0 -1 WITHSCORES
```

### Find Term for a Keyword
```redis
GET aoa:proj:keyword:auth:term
# Returns: "session_control"
```

### Get Promotion Ratio for Keyword
```
cohit = HGET aoa:proj:cohit:kw_term "auth:session_control"
total = HGET aoa:proj:keyword_hits "auth"
ratio = cohit / total
```

### Find Best Term for Orphan Keyword
```
For each term in each domain:
  cohit = HGET aoa:proj:cohit:kw_term "{orphan}:{term}"
  char_overlap = compute_overlap(orphan, term)
  score = cohit * 0.8 + (char_overlap / 10) * 0.2
# Pick term with highest score
```

### Get Keyword Statistics
```redis
GET aoa:proj:keyword:auth:searches      # Search count
GET aoa:proj:keyword:auth:accesses      # File access count
HGET aoa:proj:keyword_hits "auth"       # Total hits
```

---

## Schema Summary Table

| Name | Key Pattern | Type | Lifecycle | Critical? |
|------|------------|------|-----------|-----------|
| Gap Keywords | `gap_keywords` | Set | Created: search 0 hits. Consumed: rebalance | ✓ |
| Keyword→Term | `keyword:{kw}:term` | String | Enrichment or rebalance. Persistent | ✓ |
| Keyword→Domain | `keyword:{kw}:domain` | String | Enrichment. Persistent | |
| Term Keywords | `term:{t}:keywords` | Sorted Set | Enrichment. Incremented by usage | ✓ |
| KW-Term Cohit | `cohit:kw_term` | Hash | Search formatting. Used in rebalance | ✓ |
| Term-Domain Cohit | `cohit:term_domain` | Hash | Search formatting. Informational | |
| Keyword Searches | `keyword:{kw}:searches` | String | Every search using keyword | |
| Keyword Accesses | `keyword:{kw}:accesses` | String | File accessed from search | |
| Keyword Hits | `keyword_hits` | Hash | Search formatting. Used in ratio | ✓ |
| Intent Records | `intent:recent` | Sorted Set | Every tool use. Used for prediction | |
| Last Rebalance | `last_rebalance` | String | Rebalance completion | |
| Prediction Cache | `aoa:context:{kws}` | String (JSON) | Context search results | |

---

## Data Integrity Notes

### Consistency Guarantees
- **Keyword-to-term**: One keyword → one term (SET enforces uniqueness)
- **Term keywords**: Multiple keywords → one term (ZADD allows duplicates)
- **Cohit counters**: Just counters, no consistency constraint
- **Intent records**: Time-ordered, no duplicates needed

### Potential Issues
- If keyword assignment changes (re-enrichment), old cohit data orphaned
- Gap keywords can persist forever if rebalance doesn't match them
- No cascade delete: changing term doesn't update keyword:term pointers

### Data Growth
- Per keyword: ~4-5 keys (term, domain, searches, accesses, cohit entries)
- Per 1000 keywords: ~5000-6000 Redis operations during enrichment
- Per search with 100 results: ~300-400 hincrby operations (3x keywords × results)

---

## Planned Improvements (Gaps)

### Gap #1: Add Search Result Tracking
```
Proposed Key: aoa:{project_id}:keyword:{kw}:search_results

Record when search returns results:
  ZADD aoa:proj:keyword:auth:search_results {
    {timestamp}: {result_count}
  }

Enables: Analysis of search productivity per keyword
```

### Gap #2: Add Keywords to Intent
```
Proposed Field in intent record:

{
  "tool": "Grep",
  "keywords": ["auth", "login"],  ← NEW
  "files": [...],
  "tags": [...]
}

Enables: Correlate searches to file accesses
```

### Gap #3: Use Cohit in Rebalance
```
Proposed: Include cohit in rebalance scoring

score(keyword, term) = cohit(keyword, term) * 0.8 +
                       char_overlap(keyword, term) * 0.2

Current: Uses char_overlap only (0 vs 1.0)
```

