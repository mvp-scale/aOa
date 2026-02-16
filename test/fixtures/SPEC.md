# aOa Behavioral Specification (F-02)

> Extracted from Python codebase 2026-02-15 via parallel research agents.
> Source: `services/domains/learner.py`, `services/index/indexer.py`, `services/jobs/worker.py`, `.claude/hooks/aoa-gateway.py`

---

## Constants (Canonical — Code, Not Docs)

```
DECAY_RATE          = 0.90    # 10% decay per cycle
AUTOTUNE_INTERVAL   = 50     # Every 50 intents
PRUNE_FLOOR         = 0.3    # Remove context domains below this
DEDUP_MIN_TOTAL     = 100    # Min cohits before dedup acts
CORE_DOMAINS_MAX    = 24     # Hard cap on core tier
CONTEXT_DOMAINS_MAX = 20     # Soft cap on context tier
PROMOTION_MIN_RATIO = 0.5    # Min cohit ratio for keyword→term
MIN_PROMOTION_OBS   = 3      # Min observations before promotion
NOISE_THRESHOLD     = 1000   # Blocklist keywords above this
PRESERVE_THRESHOLD  = 5      # Never remove domains with hits >= 5
```

**WARNING**: Documentation files reference DECAY_RATE=0.80, AUTOTUNE=100, PRUNE=0.5. These are OUTDATED. Code values above are canonical.

---

## Autotune: 21 Steps (run_math_tune)

Triggered every 50 intents (`prompt_count % AUTOTUNE_INTERVAL == 0`).

### Phase 1: Domain Lifecycle (Steps 1-7)

| # | Step | Behavior |
|---|------|----------|
| 1 | Prune noisy terms | Remove terms appearing in >30% of indexed files |
| 2 | Flag stale domains | hits_last_cycle==0 + active → stale (stale_cycles++) |
| 3 | Deprecate persistent stale | stale + stale_cycles>=2 → deprecated |
| 4 | Reactivate domains | hits_last_cycle>0 + not active → active (reset stale_cycles) |
| 5 | Flag thin domains | <2 remaining terms → deprecated |
| 6 | Remove deprecated seeded | When learned_count>=32, remove deprecated seeded domains |
| 7 | Snapshot cycle hits | Atomic: copy hits → hits_last_cycle for all domains |

### Phase 2: Two-Tier Curation (Steps 8-13)

| # | Step | Behavior |
|---|------|----------|
| 8 | Decay domain hits | `hits = hits * 0.90` (stored as **float**, NOT int-truncated) |
| 9a | Dedup keywords | Lua on cohit:kw_term, total>=100, winner keeps keyword |
| 9b | Dedup terms | Lua on cohit:term_domain, total>=100, winner keeps term |
| 10 | Rank domains | Sort all by hits descending |
| 11a | Promote context→core | Rank 0-23: set tier="core" |
| 11b | Prune low-value context | Rank 24+, hits<0.3: remove_domain() cascade |
| 11c | Demote core→context | Rank 24+, hits>=0.3: set tier="context", trim keywords to 5/term |
| 12 | Update tune tracking | Set last_tune timestamp, reset tune_count |
| 13 | Promotion check | Move staged domains to core if cohit ratio >= threshold |

### Phase 3: Hit Count Maintenance (Steps 14-18)

| # | Step | Behavior |
|---|------|----------|
| 14 | Cleanup stale proposals | Remove proposals with 0 hits after 50 prompts |
| 15 | Decay bigrams | `int(count * 0.90)`, delete if <=0 |
| 16 | Decay file_hits | `int(count * 0.90)`, delete if <=0 |
| 17 | Decay cohit:kw_term | `int(count * 0.90)`, delete if <=0 |
| 18 | Decay cohit:term_domain | `int(count * 0.90)`, delete if <=0 |

### Phase 4: Keyword/Term Freshness (Steps 19-21)

| # | Step | Behavior |
|---|------|----------|
| 19 | Blocklist noisy keywords | count > 1000 → add to blocklist, delete from keyword_hits |
| 20 | Decay keyword_hits | `int(count * 0.90)`, delete if <=0 |
| 21 | Decay term_hits | `int(count * 0.90)`, delete if <=0 |

### Float Precision Rules

| Map | Decay Formula | Truncation |
|-----|--------------|------------|
| domain_meta.hits | `hits * 0.90` | **NO** — stored as float |
| All others | `int(float(count) * 0.90)` | **YES** — int() toward zero |

### Critical Ordering Dependencies

1. **Snapshot BEFORE decay** (7→8): stale detection needs pre-decay hits
2. **Blocklist BEFORE keyword decay** (19→20): filter noise at raw counts
3. **Domain decay BEFORE dedup** (8→9): dedup compares post-decay counts
4. **Dedup BEFORE ranking** (9→10): removed losers don't participate
5. **Ranking BEFORE promote/demote** (10→11): tier from rank position

---

## observe() — Unified Signal Entry Point

**Signature**: `observe(keywords, terms, domains, keyword_terms, term_domains)`

| Parameter | Type | State Mutation |
|-----------|------|---------------|
| keywords | []string | keyword_hits[kw] += 1 |
| terms | []string | term_hits[term] += 1 |
| domains | []string | domain_meta[d].hits += 1, total_hits += 1 |
| keyword_terms | [](string,string) | keyword_hits, term_hits, cohit:kw_term all += 1 |
| term_domains | [](string,string) | cohit:term_domain += 1 |

### Signal Sources

| Source | Trigger | observe() Call | Condition |
|--------|---------|---------------|-----------|
| File read | POST /intent with line range | keyword_terms + domains | range < 500 lines |
| Conversation | Stop → scrape job | keywords only | bigram count >= 6 |
| Gap keyword | Search returns 0 results | record_gap_keyword() | Always |

### Range Gate

- Read event `limit < 500` → fire observe signal
- Read event `limit >= 500` → skip (full-file scan, not intentional focus)

---

## Bigram Extraction

**Tokenization**: `re.findall(r'\b[a-z][a-z0-9_]+\b', text.lower())`

**Rules**:
- Lowercase all text first
- First char must be `[a-z]`
- Remaining: `[a-z0-9_]+`
- Bigram = `word[i]:word[i+1]` (colon separator)
- Threshold: count >= 6 to be stored
- Stored in: `bigrams` hash and `recent_bigrams` hash

---

## Tokenization (Search Index)

**Split regex**: `[/_\-.\\s]+` (slash, underscore, hyphen, dot, whitespace)

**CamelCase split** (secondary): `[A-Z]?[a-z]+|[A-Z]+(?=[A-Z][a-z]|\d|\W|$)|\d+`

**Examples**:
- `app.post` → `["app", "post"]`
- `tree-sitter` → `["tree", "sitter"]`
- `getUserToken` → `["get", "user", "token"]`
- `APIKey` → `["api", "key"]`

**Min length**: 2 characters (shorter tokens discarded)
**Case**: All tokens lowercased for storage and lookup

---

## Output Format

```
file:scope[range]:line content  @domain  #tag1 #tag2 #tag3
```

**ANSI colors**: bold white (file), yellow (scope), dim (line), magenta (@domain), cyan (#tags)

**Spacing**:
- Colon separates file:scope:line
- Space between line number and content
- **Two spaces** before @domain
- **Two spaces** before #tags
- Max 3 tags displayed

**Scope format**: `Parent().method()[start-end]` or `function()[start-end]` or `<module>`

---

## Domain Lifecycle State Machine

```
active ──(0 hits 1 cycle)──→ stale ──(0 hits 2nd cycle)──→ deprecated
  ↑                            │
  └────(any hits)──────────────┘
```

- **Deprecated + seeded + learned_count>=32** → removed
- **<2 remaining terms** → deprecated
- **Context tier + hits<0.3** → removed (cascade cleanup)

### Cascade Cleanup (remove_domain)

1. Get domain terms
2. Delete domain meta + terms set
3. For each term: delete keyword_index entries, keywords set, claimants
4. Remove from domains set

### Keyword Trimming on Demotion (CD-02)

When core → context: trim to top 5 keywords per term (by score in sorted set)

---

## Competitive Displacement

1. Decay all domain hits (*0.90)
2. Dedup via cohit analysis (DEDUP_MIN_TOTAL=100)
3. Rank all domains by hits descending
4. Top 24 = core, promote if needed
5. Rank 24+, hits<0.3 = remove (cascade)
6. Rank 24+, hits>=0.3 = context, demote if was core (trim keywords)

**Tie-breaking**: Not explicitly defined in Python (dict ordering). Go must define deterministic tie-break (alphabetical recommended).

---

## Dedup Algorithm (DEDUP_LUA)

```
Input: cohit hash (e.g., cohit:kw_term)
  Keys: "entity:container" → count

1. Group entries by entity (left side of colon)
2. Filter to entities appearing in 2+ containers
3. Filter to entities with total cohits >= DEDUP_MIN_TOTAL (100)
4. For each qualifying entity:
   - Sort containers by count descending
   - Winner = highest count container
   - Losers = all other containers
5. Return: entity|winner=count|loser1=count,loser2=count

Action on losers:
  - kw_term: remove keyword from loser term's keywords set
  - term_domain: remove term from loser domain's terms set, update pointer to winner
```

---

## Grep Flag Matrix

| Flag | Behavior | Routes to |
|------|----------|-----------|
| -a, --and | AND mode (comma-separated) | /multi |
| -i | Case insensitive | re.IGNORECASE |
| -w | Word boundary | API param |
| -e PAT | Multiple patterns (OR) | Space accumulation |
| -E, -P | Regex mode | egrep |
| -c | Count only output | Integer |
| -q | Quiet (exit code only) | 0=found, 1=not |
| -m NUM | Limit results | Default 20 |
| --json | JSON output | Raw response |
| -r,-R,-n,-H,-F,-G,-l,-o,-s | No-ops | Ignored |
| --since, --before, --today | Time filters | Unix timestamp |
| --include, --exclude | File glob filters | Regex conversion |

**Smart routing**: Metacharacters (`.`, `*`, `+`, `?`, `^`, `$`, `[`, `\`, `(`) auto-route to egrep.
**Pipe conversion**: `foo|bar` → `foo bar` (OR search).

---

## Interface Gaps (from Goal Alignment Agent)

### Must fix before Phase 2:

1. **DomainMeta**: Add `LastHitAt uint32`, `Source string`, `CreatedAt int64`
2. **LearnerState**: Add `KeywordBlocklist map[string]bool`
3. **New type**: `SearchOptions` struct for grep flags/modes

### Must fix before Phase 4:

4. **Dedup**: Port DEDUP_LUA to pure Go, validate against Python output
5. **Compaction**: Define when/how bbolt cleans up after decay
6. **Autotune context**: Capture 21-step algorithm as documented state machine
