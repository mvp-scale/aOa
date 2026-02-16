#!/usr/bin/env python3
"""
Compute learner state fixtures for aOa-go behavioral parity tests.

This script generates:
- Event streams (events-01-to-50.json, events-51-to-100.json, events-101-to-200.json)
- State snapshots (01-fifty-intents.json through 04-post-wipe.json)

All math is done in Python to match the Python reference implementation exactly.
Float precision: domain_meta.hits uses float (no truncation).
All other maps: int(float(count) * 0.90) — int-truncated toward zero.
"""

import json
import copy
import math

# ============================================================
# Constants (from SPEC.md — canonical, code-derived)
# ============================================================
DECAY_RATE = 0.90
AUTOTUNE_INTERVAL = 50
PRUNE_FLOOR = 0.3
DEDUP_MIN_TOTAL = 100
CORE_DOMAINS_MAX = 24
CONTEXT_DOMAINS_MAX = 20
PROMOTION_MIN_RATIO = 0.5
MIN_PROMOTION_OBS = 3
NOISE_THRESHOLD = 1000
PRESERVE_THRESHOLD = 5

# ============================================================
# Domain structure (8 domains, realistic but compact)
# ============================================================

DOMAIN_STRUCTURE = {
    "@authentication": {
        "tier": "core", "source": "seeded", "state": "active",
        "terms": {
            "login": ["login", "signin", "authenticate", "credentials", "sso"],
            "session": ["session", "cookie", "jwt", "bearer", "refresh"],
            "token": ["token", "access_token", "refresh_token", "oauth", "apikey"],
        }
    },
    "@api": {
        "tier": "core", "source": "seeded", "state": "active",
        "terms": {
            "handler": ["handler", "controller", "middleware", "interceptor", "resolver"],
            "endpoint": ["endpoint", "url", "path", "prefix", "base_url"],
            "route": ["route", "router", "dispatch", "mapping", "urlconf"],
        }
    },
    "@database": {
        "tier": "core", "source": "seeded", "state": "active",
        "terms": {
            "query": ["query", "select", "insert", "update", "delete"],
            "model": ["model", "schema", "entity", "table", "column"],
            "migration": ["migration", "migrate", "rollback", "seed", "fixture"],
        }
    },
    "@testing": {
        "tier": "core", "source": "seeded", "state": "active",
        "terms": {
            "test": ["test", "spec", "suite", "runner", "coverage"],
            "mock": ["mock", "stub", "fake", "spy", "double"],
            "assert": ["assert", "expect", "should", "verify", "check"],
        }
    },
    "@logging": {
        "tier": "context", "source": "seeded", "state": "active",
        "terms": {
            "logger": ["logger", "logfile", "loglevel", "syslog", "logrotate"],
            "debug": ["debug", "trace", "verbose", "debugger", "breakpoint"],
        }
    },
    "@caching": {
        "tier": "context", "source": "seeded", "state": "active",
        "terms": {
            "cache": ["cache", "redis", "memcached", "ttl", "expiry"],
            "invalidate": ["invalidate", "evict", "purge", "flush", "bust"],
        }
    },
    "@deployment": {
        "tier": "context", "source": "seeded", "state": "active",
        "terms": {
            "deploy": ["deploy", "release", "rollout", "canary", "bluegreen"],
            "pipeline": ["pipeline", "ci", "cd", "workflow", "stage"],
        }
    },
    "@monitoring": {
        "tier": "context", "source": "learned", "state": "active",
        "terms": {
            "metric": ["metric", "gauge", "counter", "histogram", "percentile"],
            "alert": ["alert", "alarm", "threshold", "notification", "pager"],
        }
    },
}

CREATED_AT = 1739500000  # Fixed timestamp for all seeded domains

# ============================================================
# Files used in events
# ============================================================
FILES = [
    "services/auth/handler.py",
    "services/auth/session.py",
    "services/auth/token.py",
    "services/api/routes.py",
    "services/api/middleware.py",
    "services/db/models.py",
    "services/db/queries.py",
    "tests/test_auth.py",
    "tests/test_api.py",
    "services/cache/store.py",
    "services/logging/logger.py",
    "services/monitoring/metrics.py",
]

# ============================================================
# Event generation
# ============================================================

def make_event(prompt_num, keywords, terms, domains, keyword_terms, term_domains, file_path, offset=1, limit=30):
    """Create a single observe event."""
    ev = {
        "prompt_number": prompt_num,
        "observe": {
            "keywords": keywords,
            "terms": terms,
            "domains": domains,
            "keyword_terms": keyword_terms,
            "term_domains": term_domains,
        },
    }
    if file_path:
        ev["file_read"] = {
            "file": file_path,
            "offset": offset,
            "limit": limit,
        }
    return ev


def generate_events_1_to_50():
    """Generate first 50 events. Heavy auth/api, medium db/test, low log/cache, zero deploy."""
    events = []

    # Pattern: distribute events across domains
    # @authentication: ~30 hits (events 1-5, 8-12, 16-20, 24-28, 32-36, 40-44)
    # @api: ~25 hits (events 2-6, 10-14, 18-22, 26-30, 34-38)
    # @database: ~15 hits (events 3, 7, 11, 15, 19, 23, 27, 31, 35, 39, 43, 47, 13, 17, 21)
    # @testing: ~12 hits (events 4, 9, 14, 20, 25, 30, 36, 41, 46, 8, 16, 24)
    # @logging: ~3 hits (events 10, 30, 50)
    # @caching: ~2 hits (events 15, 45)
    # @deployment: 0 hits
    # @monitoring: not introduced yet

    for i in range(1, 51):
        kws = []
        tms = []
        doms = []
        kw_terms = []
        tm_doms = []
        fp = None

        # Authentication events
        if i in [1,2,3,5,8,9,10,11,16,17,18,20,24,25,26,28,32,33,34,36,40,41,42,44,46,48,49,50,6,13]:
            # 30 events total
            kw = ["login", "session"] if i % 3 == 0 else ["login"] if i % 2 == 0 else ["token", "jwt"]
            kws.extend(kw)
            tms.append("login" if "login" in kw else "token" if "token" in kw else "session")
            doms.append("@authentication")
            for k in kw:
                t = "login" if k in ["login", "signin", "credentials"] else "session" if k in ["session", "cookie", "jwt"] else "token"
                kw_terms.append([k, t])
            for t in set(tms):
                tm_doms.append([t, "@authentication"])
            fp = FILES[0] if i % 3 == 0 else FILES[1] if i % 3 == 1 else FILES[2]

        # API events
        if i in [2,3,4,6,10,11,12,14,18,19,22,26,27,29,30,34,35,37,38,42,43,45,47,48,50]:
            # 25 events total
            kw = ["handler", "middleware"] if i % 4 == 0 else ["route"] if i % 3 == 0 else ["endpoint"]
            kws.extend(kw)
            tms.append("handler" if "handler" in kw else "route" if "route" in kw else "endpoint")
            doms.append("@api")
            for k in kw:
                t = "handler" if k in ["handler", "controller", "middleware"] else "route" if k in ["route", "router"] else "endpoint"
                kw_terms.append([k, t])
            for t in ["handler" if "handler" in [x for x in tms if x in ["handler"]] else "route" if "route" in tms else "endpoint"]:
                tm_doms.append([t, "@api"])
            if not fp:
                fp = FILES[3] if i % 2 == 0 else FILES[4]

        # Database events
        if i in [3,7,11,15,19,23,27,31,35,39,43,47,13,17,21]:
            # 15 events total
            kw = ["query", "select"] if i % 5 == 0 else ["model"]
            kws.extend(kw)
            tms.append("query" if "query" in kw else "model")
            doms.append("@database")
            for k in kw:
                t = "query" if k in ["query", "select", "insert"] else "model"
                kw_terms.append([k, t])
            tm_doms.append(["query" if "query" in kw else "model", "@database"])
            if not fp:
                fp = FILES[5] if i % 2 == 0 else FILES[6]

        # Testing events
        if i in [4,9,14,20,25,30,36,41,46,8,16,24]:
            # 12 events total
            kw = ["test", "assert"] if i % 5 == 0 else ["test", "mock"] if i % 3 == 0 else ["test"]
            kws.extend(kw)
            tms.append("test")
            doms.append("@testing")
            for k in kw:
                t = "test" if k in ["test", "spec", "suite"] else "mock" if k in ["mock", "stub"] else "assert"
                kw_terms.append([k, t])
            tm_doms.append(["test", "@testing"])
            if not fp:
                fp = FILES[7] if i % 2 == 0 else FILES[8]

        # Logging events (very few)
        if i in [10, 30, 50]:
            # 3 events total
            kw = ["logger"]
            kws.extend(kw)
            tms.append("logger")
            doms.append("@logging")
            kw_terms.append(["logger", "logger"])
            tm_doms.append(["logger", "@logging"])
            if not fp:
                fp = FILES[10]

        # Caching events (minimal)
        if i in [15, 45]:
            # 2 events total
            kw = ["cache", "redis"]
            kws.extend(kw)
            tms.append("cache")
            doms.append("@caching")
            for k in kw:
                kw_terms.append([k, "cache"])
            tm_doms.append(["cache", "@caching"])
            if not fp:
                fp = FILES[9]

        # Default file if none set
        if not fp:
            fp = FILES[0]

        # Remove duplicates in domains list
        doms = list(dict.fromkeys(doms))
        # Remove duplicate terms
        tms = list(dict.fromkeys(tms))

        events.append(make_event(i, kws, tms, doms, kw_terms, tm_doms, fp, offset=i*5, limit=30))

    return events


def generate_events_51_to_100():
    """Generate events 51-100. Similar pattern, introduce @monitoring at ~70, @deployment still zero."""
    events = []

    for i in range(51, 101):
        kws = []
        tms = []
        doms = []
        kw_terms = []
        tm_doms = []
        fp = None

        # Authentication: ~28 hits
        if i in [51,52,53,55,58,59,60,62,65,66,67,69,72,73,75,78,79,80,82,85,86,88,91,92,94,97,98,100]:
            kw = ["login", "session"] if i % 3 == 0 else ["login"] if i % 2 == 0 else ["token", "jwt"]
            kws.extend(kw)
            tms.append("login" if "login" in kw else "token" if "token" in kw else "session")
            doms.append("@authentication")
            for k in kw:
                t = "login" if k in ["login", "signin", "credentials"] else "session" if k in ["session", "cookie", "jwt"] else "token"
                kw_terms.append([k, t])
            for t in set(tms):
                tm_doms.append([t, "@authentication"])
            fp = FILES[0] if i % 3 == 0 else FILES[1] if i % 3 == 1 else FILES[2]

        # API: ~23 hits
        if i in [52,53,54,56,60,61,63,64,68,69,71,74,76,77,81,83,84,87,89,93,95,96,99]:
            kw = ["handler", "middleware"] if i % 4 == 0 else ["route"] if i % 3 == 0 else ["endpoint"]
            kws.extend(kw)
            tms.append("handler" if "handler" in kw else "route" if "route" in kw else "endpoint")
            doms.append("@api")
            for k in kw:
                t = "handler" if k in ["handler", "controller", "middleware"] else "route" if k in ["route", "router"] else "endpoint"
                kw_terms.append([k, t])
            tm_doms.append(["handler" if "handler" in kw else "route" if "route" in kw else "endpoint", "@api"])
            if not fp:
                fp = FILES[3] if i % 2 == 0 else FILES[4]

        # Database: ~14 hits
        if i in [53,57,61,65,69,73,77,81,85,89,93,97,63,67]:
            kw = ["query", "select"] if i % 5 == 0 else ["model"]
            kws.extend(kw)
            tms.append("query" if "query" in kw else "model")
            doms.append("@database")
            for k in kw:
                t = "query" if k in ["query", "select", "insert"] else "model"
                kw_terms.append([k, t])
            tm_doms.append(["query" if "query" in kw else "model", "@database"])
            if not fp:
                fp = FILES[5] if i % 2 == 0 else FILES[6]

        # Testing: ~12 hits
        if i in [54,59,64,70,75,80,86,91,96,58,66,74]:
            kw = ["test", "assert"] if i % 5 == 0 else ["test", "mock"] if i % 3 == 0 else ["test"]
            kws.extend(kw)
            tms.append("test")
            doms.append("@testing")
            for k in kw:
                t = "test" if k in ["test", "spec", "suite"] else "mock" if k in ["mock", "stub"] else "assert"
                kw_terms.append([k, t])
            tm_doms.append(["test", "@testing"])
            if not fp:
                fp = FILES[7] if i % 2 == 0 else FILES[8]

        # Logging: ~2 hits (very low, to test decay below prune floor eventually)
        if i in [60, 90]:
            kw = ["logger"]
            kws.extend(kw)
            tms.append("logger")
            doms.append("@logging")
            kw_terms.append(["logger", "logger"])
            tm_doms.append(["logger", "@logging"])
            if not fp:
                fp = FILES[10]

        # Caching: ~2 hits
        if i in [65, 95]:
            kw = ["cache"]
            kws.extend(kw)
            tms.append("cache")
            doms.append("@caching")
            kw_terms.append(["cache", "cache"])
            tm_doms.append(["cache", "@caching"])
            if not fp:
                fp = FILES[9]

        # @monitoring: introduced at event 70 (learned domain, ~10 hits from 70-100)
        if i in [70, 72, 75, 78, 82, 85, 88, 92, 95, 98]:
            kw = ["metric", "gauge"] if i % 3 == 0 else ["alert"] if i % 5 == 0 else ["metric"]
            kws.extend(kw)
            tms.append("metric" if "metric" in kw else "alert")
            doms.append("@monitoring")
            for k in kw:
                t = "metric" if k in ["metric", "gauge", "counter"] else "alert"
                kw_terms.append([k, t])
            tm_doms.append(["metric" if "metric" in kw else "alert", "@monitoring"])
            if not fp:
                fp = FILES[11]

        # Default file
        if not fp:
            fp = FILES[0]

        doms = list(dict.fromkeys(doms))
        tms = list(dict.fromkeys(tms))

        events.append(make_event(i, kws, tms, doms, kw_terms, tm_doms, fp, offset=i*5, limit=30))

    return events


def generate_events_101_to_200():
    """Generate events 101-200.
    @logging drops to 0 (tests prune after compound decay).
    @monitoring gains traction.
    Push 'test' keyword past 1000 hits (tests blocklist).
    """
    events = []

    for i in range(101, 201):
        kws = []
        tms = []
        doms = []
        kw_terms = []
        tm_doms = []
        fp = None

        # Authentication: ~30 hits (continued heavy)
        if i in list(range(101, 201, 3)) + list(range(102, 201, 7)):
            kw = ["login", "session"] if i % 3 == 0 else ["login"] if i % 2 == 0 else ["token"]
            kws.extend(kw)
            tms.append("login" if "login" in kw else "token")
            doms.append("@authentication")
            for k in kw:
                t = "login" if k in ["login", "signin", "credentials"] else "session" if k in ["session", "cookie", "jwt"] else "token"
                kw_terms.append([k, t])
            for t in set(tms):
                tm_doms.append([t, "@authentication"])
            fp = FILES[0] if i % 3 == 0 else FILES[1] if i % 3 == 1 else FILES[2]

        # API: ~25 hits
        if i in list(range(101, 201, 4)) + list(range(103, 201, 7)):
            kw = ["handler", "middleware"] if i % 4 == 0 else ["endpoint"]
            kws.extend(kw)
            tms.append("handler" if "handler" in kw else "endpoint")
            doms.append("@api")
            for k in kw:
                t = "handler" if k in ["handler", "controller", "middleware"] else "endpoint"
                kw_terms.append([k, t])
            tm_doms.append(["handler" if "handler" in kw else "endpoint", "@api"])
            if not fp:
                fp = FILES[3] if i % 2 == 0 else FILES[4]

        # Database: ~15 hits
        if i in list(range(105, 201, 7)) + list(range(103, 201, 13)):
            kw = ["query"] if i % 2 == 0 else ["model"]
            kws.extend(kw)
            tms.append("query" if "query" in kw else "model")
            doms.append("@database")
            for k in kw:
                t = "query" if k in ["query", "select"] else "model"
                kw_terms.append([k, t])
            tm_doms.append(["query" if "query" in kw else "model", "@database"])
            if not fp:
                fp = FILES[5] if i % 2 == 0 else FILES[6]

        # Testing: EVERY event hits "test" to push it past 1000 for blocklist test
        # We need "test" keyword to reach >1000 by event 200
        # After autotune at 50: test keyword decayed. Let's track carefully.
        # Strategy: hit "test" in every single event 101-200 (100 times)
        # Plus add extra test hits to boost count
        kw_test = ["test"]
        kws.extend(kw_test)
        tms.append("test")
        doms.append("@testing")
        kw_terms.append(["test", "test"])
        tm_doms.append(["test", "@testing"])
        if not fp:
            fp = FILES[7] if i % 2 == 0 else FILES[8]

        # Additionally, add 9 more "test" hits per event to push count up fast
        # We need > 1000. After autotune1 (50): test_kw was ~12, decayed to int(12*0.9)=10
        # Events 51-100 add ~12 more = 22, autotune2 decays to int(22*0.9)=19
        # Events 101-200: we add 10 per event = 1000 raw + 19 = 1019
        # Actually let me recalculate properly later. For now, add 10 per event.
        for _ in range(9):
            kws.append("test")
            kw_terms.append(["test", "test"])

        # Logging: 0 hits in this range (tests prune after compound decay)
        # @logging got 3 hits in 1-50, 2 hits in 51-100
        # After autotune1: hits = (3) * 0.9 = 2.7
        # 51-100 adds 2 more = 4.7
        # After autotune2: hits = 4.7 * 0.9 = 4.23
        # 101-150 adds 0 = 4.23
        # After autotune3: hits = 4.23 * 0.9 = 3.807
        # 151-200 adds 0 = 3.807
        # After autotune4: hits = 3.807 * 0.9 = 3.4263
        # Still above 0.3... Need more decay cycles or fewer initial hits
        # Actually @logging is context tier. With 3.4 hits it stays.
        # Let me reduce initial logging hits to make prune work.
        # Redesign: @logging gets only 2 hits in 1-50, 1 hit in 51-100, 0 in 101-200
        # After AT1: 2 * 0.9 = 1.8
        # +1 in 51-100 = 2.8
        # After AT2: 2.8 * 0.9 = 2.52
        # +0 in 101-150 = 2.52
        # After AT3: 2.52 * 0.9 = 2.268
        # +0 in 151-200 = 2.268
        # After AT4: 2.268 * 0.9 = 2.0412
        # Still above 0.3. Won't prune with these numbers.
        #
        # For prune to work, we need hits < 0.3 at context tier.
        # Let's make @logging get 1 hit total in 1-50, 0 in 51-100, 0 in 101-200.
        # After AT1: 1 * 0.9 = 0.9
        # After AT2: 0.9 * 0.9 = 0.81
        # After AT3: 0.81 * 0.9 = 0.729
        # After AT4: 0.729 * 0.9 = 0.6561
        # Still above 0.3 after 4 cycles. Need 7+ cycles or start lower.
        # With 1 hit: need hits * 0.9^n < 0.3 → 0.9^n < 0.3 → n > log(0.3)/log(0.9) ≈ 11.4
        # That's way too many cycles.
        #
        # Alternative: @logging gets 0 hits. Then stale→deprecated→removed.
        # After AT1: stale (0 hits_last_cycle)
        # After AT2: deprecated (stale_cycles >= 2)
        # But we want @deployment to be the zero-hit domain.
        #
        # New plan: Both @logging and @deployment test DIFFERENT lifecycle paths.
        # @deployment: 0 hits ever → stale at AT1, deprecated at AT2
        # @logging: Gets a few hits early, then none → PRESERVES because hits stay > 0.3
        #   BUT we need to test prune floor. So:
        #   @logging gets hits_last_cycle > 0 (reactivates if stale), but eventually
        #   the accumulated hits decay below 0.3 AFTER enough cycles.
        #
        # Let me just make @caching the prune candidate instead.
        # @caching: 2 hits in 1-50, 0 hits in 51-200
        # After AT1: 2 * 0.9 = 1.8
        # After AT2: 1.8 * 0.9 = 1.62
        # After AT3: 1.62 * 0.9 = 1.458
        # After AT4: 1.458 * 0.9 = 1.3122
        # Still too high. Prune won't trigger for 0.3 floor.
        #
        # Actually for prune to work we need: hits < 0.3 AND context tier AND rank >= 24
        # With only 8 domains, rank >= 24 is impossible (only 8 domains total).
        # So prune floor only matters with 24+ domains.
        # With 8 domains, all fit in core (< 24).
        #
        # BUT @deployment can be deprecated and removed via the stale lifecycle.
        # And @logging can be demoted from core to context when it has fewer hits.
        # Actually with 8 domains < 24, ALL domains are core tier.
        #
        # OK let me rethink. With CORE_DOMAINS_MAX = 24 and only 8 domains,
        # all 8 are core. No demotion, no prune floor test.
        #
        # The PRUNE test (step 11b) requires rank >= 24 AND hits < 0.3.
        # We can't test this with 8 domains unless we add more or reduce CORE_MAX.
        # But we can't change constants — they're fixed by the spec.
        #
        # SO: with 8 domains, we test:
        # 1. Stale detection (0 cycle hits → stale)
        # 2. Deprecated (stale_cycles >= 2)
        # 3. Domain removal (deprecated seeded — Step 6 requires learned_count >= 32 though)
        # 4. Decay math
        # 5. Ranking (but no demotion since all fit in top 24)
        # 6. Blocklist (keyword > 1000)
        #
        # Step 6: "Remove deprecated seeded" requires learned_count >= 32.
        # With only 1 learned domain (@monitoring), this won't trigger either.
        #
        # For the prune floor test, we need Step 11b:
        # "Rank 24+, hits < 0.3: remove_domain()"
        # This requires > 24 domains. The task says 8 domains for "verifiable math."
        #
        # Resolution: We document that prune floor (11b) and keyword trimming (11c)
        # are NOT testable with 8 domains (requires >24). The fixtures test all OTHER
        # autotune steps. A separate fixture set with 30+ domains would test 11b/11c.
        #
        # What we CAN test with 8 domains:
        # - Steps 1-7: domain lifecycle (stale, deprecated, reactivate)
        # - Step 8: decay (float precision)
        # - Steps 10-11a: rank + promote (context→core)
        # - Steps 14-21: decay all maps, blocklist
        # - @deployment: stale (AT1) → deprecated (AT2) → stays deprecated (no removal since learned_count < 32)
        # - @monitoring: introduced at 70, gains traction, promoted to core at AT2

        # Monitoring: ~15 hits in 101-200
        if i in list(range(101, 201, 7)) + list(range(104, 201, 11)):
            kw = ["metric", "counter"] if i % 3 == 0 else ["alert"] if i % 5 == 0 else ["metric"]
            kws.extend(kw)
            tms.append("metric" if "metric" in kw else "alert")
            doms.append("@monitoring")
            for k in kw:
                t = "metric" if k in ["metric", "gauge", "counter", "histogram"] else "alert"
                kw_terms.append([k, t])
            tm_doms.append(["metric" if "metric" in kw else "alert", "@monitoring"])
            if not fp:
                fp = FILES[11]

        # Default file
        if not fp:
            fp = FILES[0]

        doms = list(dict.fromkeys(doms))
        tms = list(dict.fromkeys(tms))

        events.append(make_event(i, kws, tms, doms, kw_terms, tm_doms, fp, offset=i*5, limit=30))

    return events

# ============================================================
# State machine
# ============================================================

def fresh_state():
    """Create fresh (empty) learner state."""
    return {
        "keyword_hits": {},
        "term_hits": {},
        "domain_meta": {},
        "cohit_kw_term": {},
        "cohit_term_domain": {},
        "bigrams": {},
        "file_hits": {},
        "keyword_blocklist": {},
        "gap_keywords": {},
        "prompt_count": 0,
    }


def init_domains(state):
    """Initialize domain meta for all seeded domains (not @monitoring which is learned)."""
    for domain_name, info in DOMAIN_STRUCTURE.items():
        if domain_name == "@monitoring":
            continue  # Learned domain, introduced later
        state["domain_meta"][domain_name] = {
            "hits": 0.0,
            "total_hits": 0,
            "tier": info["tier"],
            "source": info["source"],
            "state": info["state"],
            "stale_cycles": 0,
            "hits_last_cycle": 0.0,
            "last_hit_at": 0,
            "created_at": CREATED_AT,
        }


def apply_observe(state, event):
    """Apply a single observe event to the state."""
    obs = event["observe"]
    prompt = event["prompt_number"]

    # Increment keyword_hits
    for kw in obs["keywords"]:
        state["keyword_hits"][kw] = state["keyword_hits"].get(kw, 0) + 1

    # Increment term_hits
    for term in obs["terms"]:
        state["term_hits"][term] = state["term_hits"].get(term, 0) + 1

    # Increment domain hits
    for domain in obs["domains"]:
        if domain not in state["domain_meta"]:
            # Create learned domain on first encounter
            state["domain_meta"][domain] = {
                "hits": 0.0,
                "total_hits": 0,
                "tier": "context",
                "source": "learned",
                "state": "active",
                "stale_cycles": 0,
                "hits_last_cycle": 0.0,
                "last_hit_at": 0,
                "created_at": CREATED_AT + prompt,  # Offset for learned domains
            }
        dm = state["domain_meta"][domain]
        dm["hits"] += 1.0
        dm["total_hits"] += 1
        dm["last_hit_at"] = prompt

    # Increment cohit_kw_term
    for kw, term in obs["keyword_terms"]:
        key = f"{kw}:{term}"
        state["cohit_kw_term"][key] = state["cohit_kw_term"].get(key, 0) + 1
        # keyword_terms also increments keyword_hits and term_hits per spec
        state["keyword_hits"][kw] = state["keyword_hits"].get(kw, 0) + 1
        state["term_hits"][term] = state["term_hits"].get(term, 0) + 1

    # Increment cohit_term_domain
    for term, domain in obs["term_domains"]:
        key = f"{term}:{domain}"
        state["cohit_term_domain"][key] = state["cohit_term_domain"].get(key, 0) + 1

    # Increment file_hits
    if "file_read" in event:
        fp = event["file_read"]["file"]
        state["file_hits"][fp] = state["file_hits"].get(fp, 0) + 1

    state["prompt_count"] = prompt


def run_autotune(state):
    """Run the 21-step autotune algorithm."""

    # ---- Phase 1: Domain Lifecycle (Steps 1-7) ----

    # Step 1: Prune noisy terms (>30% of indexed files)
    # No file index in our fixtures, skip

    # Step 2: Flag stale domains (hits_last_cycle==0 + active → stale)
    # Also increment stale_cycles for already-stale domains with 0 hits
    # (required for stale→deprecated transition after 2+ cycles)
    for name, dm in list(state["domain_meta"].items()):
        if dm["hits_last_cycle"] == 0.0 and dm["state"] in ("active", "stale"):
            dm["state"] = "stale"
            dm["stale_cycles"] += 1

    # Step 3: Deprecate persistent stale (stale + stale_cycles >= 2 → deprecated)
    for name, dm in list(state["domain_meta"].items()):
        if dm["state"] == "stale" and dm["stale_cycles"] >= 2:
            dm["state"] = "deprecated"

    # Step 4: Reactivate domains (hits_last_cycle > 0 + not active → active, reset stale_cycles)
    for name, dm in list(state["domain_meta"].items()):
        if dm["hits_last_cycle"] > 0.0 and dm["state"] != "active":
            dm["state"] = "active"
            dm["stale_cycles"] = 0

    # Step 5: Flag thin domains (<2 remaining terms → deprecated)
    # Would need term tracking per domain; skip for now (all domains have 2+ terms)

    # Step 6: Remove deprecated seeded when learned_count >= 32
    learned_count = sum(1 for dm in state["domain_meta"].values() if dm["source"] == "learned")
    if learned_count >= 32:
        to_remove = [name for name, dm in state["domain_meta"].items()
                     if dm["state"] == "deprecated" and dm["source"] == "seeded"]
        for name in to_remove:
            del state["domain_meta"][name]

    # Step 7: Snapshot cycle hits (copy hits → hits_last_cycle for all domains)
    for name, dm in state["domain_meta"].items():
        dm["hits_last_cycle"] = dm["hits"]

    # ---- Phase 2: Two-Tier Curation (Steps 8-13) ----

    # Step 8: Decay domain hits (float, NO truncation)
    for name, dm in state["domain_meta"].items():
        dm["hits"] = dm["hits"] * DECAY_RATE

    # Step 9a: Dedup keywords (cohit_kw_term, total >= 100)
    # Step 9b: Dedup terms (cohit_term_domain, total >= 100)
    # Check if any entity has total >= DEDUP_MIN_TOTAL
    run_dedup(state, "cohit_kw_term")
    run_dedup(state, "cohit_term_domain")

    # Step 10: Rank domains by hits descending
    active_domains = [(name, dm) for name, dm in state["domain_meta"].items()
                      if dm["state"] != "deprecated"]
    # Sort by hits desc, then alphabetically for tie-breaking
    active_domains.sort(key=lambda x: (-x[1]["hits"], x[0]))

    # Step 11a: Promote context→core (rank 0-23)
    for idx, (name, dm) in enumerate(active_domains):
        if idx < CORE_DOMAINS_MAX:
            if dm["tier"] == "context":
                dm["tier"] = "core"  # Promotion!
        else:
            # Step 11b: Prune low-value context (rank 24+, hits < 0.3)
            if dm["hits"] < PRUNE_FLOOR:
                # remove_domain cascade
                del state["domain_meta"][name]
            else:
                # Step 11c: Demote core→context (rank 24+, hits >= 0.3)
                if dm["tier"] == "core":
                    dm["tier"] = "context"
                    # Trim keywords to 5 per term (handled in keyword maps)

    # Step 12: Update tune tracking (timestamp, reset tune_count)
    # Not tracked in our fixtures

    # Step 13: Promotion check (staged → core if cohit ratio >= threshold)
    # Not tracked in our fixtures

    # ---- Phase 3: Hit Count Maintenance (Steps 14-18) ----

    # Step 14: Cleanup stale proposals (0 hits after 50 prompts)
    # Not applicable in fixtures

    # Step 15: Decay bigrams
    decay_int_map(state["bigrams"])

    # Step 16: Decay file_hits
    decay_int_map(state["file_hits"])

    # Step 17: Decay cohit_kw_term
    decay_int_map(state["cohit_kw_term"])

    # Step 18: Decay cohit_term_domain
    decay_int_map(state["cohit_term_domain"])

    # ---- Phase 4: Keyword/Term Freshness (Steps 19-21) ----

    # Step 19: Blocklist noisy keywords (count > 1000)
    to_blocklist = [kw for kw, count in state["keyword_hits"].items() if count > NOISE_THRESHOLD]
    for kw in to_blocklist:
        state["keyword_blocklist"][kw] = True
        del state["keyword_hits"][kw]

    # Step 20: Decay keyword_hits
    decay_int_map(state["keyword_hits"])

    # Step 21: Decay term_hits
    decay_int_map(state["term_hits"])


def run_dedup(state, map_name):
    """Run dedup on a cohit map. Groups by entity (left of colon), checks total >= 100."""
    cohit_map = state[map_name]

    # Group by entity (left side of colon)
    entity_containers = {}
    for key, count in list(cohit_map.items()):
        parts = key.split(":")
        if len(parts) != 2:
            continue
        entity, container = parts
        if entity not in entity_containers:
            entity_containers[entity] = {}
        entity_containers[entity][container] = count

    # Filter to entities in 2+ containers with total >= DEDUP_MIN_TOTAL
    for entity, containers in entity_containers.items():
        if len(containers) < 2:
            continue
        total = sum(containers.values())
        if total < DEDUP_MIN_TOTAL:
            continue
        # Sort containers by count desc
        sorted_containers = sorted(containers.items(), key=lambda x: (-x[1], x[0]))
        # Winner keeps, losers removed
        winner = sorted_containers[0]
        losers = sorted_containers[1:]
        for loser_name, loser_count in losers:
            loser_key = f"{entity}:{loser_name}"
            if loser_key in cohit_map:
                del cohit_map[loser_key]


def decay_int_map(m):
    """Decay all values in a map: int(float(count) * 0.90), delete if <= 0."""
    to_delete = []
    for key in m:
        new_val = int(float(m[key]) * DECAY_RATE)
        if new_val <= 0:
            to_delete.append(key)
        else:
            m[key] = new_val
    for key in to_delete:
        del m[key]


# ============================================================
# Main computation
# ============================================================

def clean_floats(obj):
    """Round float values to avoid floating point artifacts like 12.500000000000002."""
    if isinstance(obj, dict):
        return {k: clean_floats(v) for k, v in obj.items()}
    elif isinstance(obj, list):
        return [clean_floats(v) for v in obj]
    elif isinstance(obj, float):
        # Round to 10 decimal places to remove fp noise, then clean trailing zeros
        rounded = round(obj, 10)
        # If it's effectively an integer, keep as float with .0
        if rounded == int(rounded) and abs(rounded) < 2**53:
            return float(int(rounded))
        return rounded
    return obj


def main():
    # Generate events
    events_1_50 = generate_events_1_to_50()
    events_51_100 = generate_events_51_to_100()
    events_101_200 = generate_events_101_to_200()

    # State 00: Fresh (with domains initialized)
    state = fresh_state()
    init_domains(state)

    # Save initial state for reference
    state_00 = copy.deepcopy(state)

    # ========== Apply events 1-50 ==========
    for event in events_1_50:
        apply_observe(state, event)

    # Autotune at prompt 50
    run_autotune(state)
    state_01 = copy.deepcopy(state)

    # ========== Apply events 51-100 ==========
    for event in events_51_100:
        apply_observe(state, event)

    # Autotune at prompt 100
    run_autotune(state)
    state_02 = copy.deepcopy(state)

    # ========== Apply events 101-200 ==========
    for event in events_101_200[:50]:  # 101-150
        apply_observe(state, event)

    # Autotune at prompt 150
    run_autotune(state)

    for event in events_101_200[50:]:  # 151-200
        apply_observe(state, event)

    # Autotune at prompt 200
    run_autotune(state)
    state_03 = copy.deepcopy(state)

    # ========== Post-wipe ==========
    state_04 = fresh_state()

    # ========== Write all files ==========

    # Write event streams
    write_json("events-01-to-50.json", events_1_50)
    write_json("events-51-to-100.json", events_51_100)
    write_json("events-101-to-200.json", events_101_200)

    # Write state snapshots
    write_json("00-fresh.json", state_00)
    write_json("01-fifty-intents.json", clean_floats(state_01))
    write_json("02-hundred-intents.json", clean_floats(state_02))
    write_json("03-two-hundred.json", clean_floats(state_03))
    write_json("04-post-wipe.json", state_04)

    # Print summary for verification
    print("=== State at 50 (after autotune 1) ===")
    print_domain_summary(state_01)
    print(f"  prompt_count: {state_01['prompt_count']}")
    print(f"  keyword_hits count: {len(state_01['keyword_hits'])}")
    print(f"  blocklist: {state_01['keyword_blocklist']}")
    print()

    print("=== State at 100 (after autotune 2) ===")
    print_domain_summary(state_02)
    print(f"  prompt_count: {state_02['prompt_count']}")
    print(f"  keyword_hits count: {len(state_02['keyword_hits'])}")
    print(f"  blocklist: {state_02['keyword_blocklist']}")
    print()

    print("=== State at 200 (after autotune 4) ===")
    print_domain_summary(state_03)
    print(f"  prompt_count: {state_03['prompt_count']}")
    print(f"  keyword_hits count: {len(state_03['keyword_hits'])}")
    print(f"  blocklist: {state_03['keyword_blocklist']}")
    print()

    # Print keyword hits for "test" to verify blocklist
    test_kw_at_150 = "test keyword not directly available (need pre-autotune state)"
    print(f"  'test' in keyword_hits at 200: {'test' in state_03['keyword_hits']}")
    print(f"  'test' in blocklist at 200: {'test' in state_03['keyword_blocklist']}")


def print_domain_summary(state):
    for name in sorted(state["domain_meta"].keys()):
        dm = state["domain_meta"][name]
        print(f"  {name}: hits={dm['hits']:.4f} total={dm['total_hits']} "
              f"tier={dm['tier']} state={dm['state']} stale_cycles={dm['stale_cycles']} "
              f"hlc={dm['hits_last_cycle']:.4f}")


def write_json(filename, data):
    import os
    path = os.path.join(os.path.dirname(os.path.abspath(__file__)), filename)
    with open(path, 'w') as f:
        json.dump(data, f, indent=2)
    print(f"Wrote {path}")


if __name__ == "__main__":
    main()

