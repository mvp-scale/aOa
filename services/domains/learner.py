#!/usr/bin/env python3
"""
Domain Learner - Discovers and learns semantic domains via Haiku.

This service manages dynamic domain learning:
1. Tracks prompt counts for batch triggers
2. Discovers domains from user behavior
3. Generates terms for domain matching
4. Stores everything in Redis for O(1) lookup

Two usage modes:
- Hook mode: Prepares context for Claude Task agents (zero API keys)
- Direct mode: Calls Anthropic API directly (requires ANTHROPIC_API_KEY)
"""

import json
import os
import time
from dataclasses import dataclass
from typing import Optional

# For direct API mode
try:
    import anthropic
    ANTHROPIC_AVAILABLE = True
except ImportError:
    ANTHROPIC_AVAILABLE = False

# Redis client - handle both Docker and local imports
import sys
_services_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, _services_dir)
sys.path.insert(0, '/app/services')  # Docker path
try:
    from ranking.redis_client import RedisClient
except ImportError:
    # Fallback: inline minimal Redis client
    import redis as redis_lib

    class RedisClient:
        def __init__(self, url=None):
            self.url = url or os.environ.get('REDIS_URL', 'redis://localhost:6379/0')
            self._client = None

        @property
        def client(self):
            if self._client is None:
                self._client = redis_lib.from_url(self.url, decode_responses=True)
            return self._client

        def ping(self):
            try:
                return self.client.ping()
            except Exception:
                return False

# Job queue for auto-queuing enrichment jobs
try:
    from jobs.queue import JobQueue, create_enrich_job
    JOBS_AVAILABLE = True
except ImportError:
    JOBS_AVAILABLE = False


@dataclass
class Domain:
    """A semantic domain with confidence and terms."""
    name: str
    description: str
    confidence: float  # 0.0 - 1.0
    terms: list[str]


class DomainLearner:
    """Manages domain discovery and learning via Haiku."""

    BATCH_SIZE = 10  # Prompts per learning batch
    TUNE_THRESHOLD = 100  # Intents per regenerative tune
    STARTER_HITS = 0  # Domains start fresh, accumulate hits from actual usage
    STARTER_SCORE = 0.5  # Initial term confidence
    PRESERVE_THRESHOLD = 20  # Keep domains with hits >= this during tune
    REBALANCE_THRESHOLD = 25  # A-001: GL-083 - Rebalance every 25 prompts

    # GL-090: Two-tier domain curation
    CORE_DOMAINS_MAX = 24  # Max core tier domains
    CONTEXT_DOMAINS_MAX = 20  # Max context tier domains
    DECAY_RATE = 0.80  # 20% decay per tune cycle
    PROMOTION_THRESHOLD = 150  # Promote context→core after 150 lifetime hits
    DEMOTION_STALENESS = 500  # Demote core→context after 500 intents with 0 hits
    ORPHAN_THRESHOLD = 30  # Need 30+ orphans to trigger domain creation
    PRUNE_FLOOR = 0.5  # Prune context domains with decayed hits below this
    AUTOTUNE_INTERVAL = 100  # P2B: Run math tune every 100 prompts (10 in test mode)

    def __init__(self, project_id: str, redis_url: Optional[str] = None):
        """Initialize with project identifier."""
        self.project_id = project_id
        self.redis = RedisClient(url=redis_url)
        self._anthropic = None

    def get_threshold(self, name: str) -> float:
        """GL-091: Get threshold from Redis config, with fallback to class constant.

        Allows test mode to use lower thresholds for faster validation.
        """
        val = self.redis.client.get(f"aoa:config:{name}")
        if val:
            return float(val)
        # Fallback to class constants
        defaults = {
            'rebalance': self.REBALANCE_THRESHOLD,
            'promotion': self.PROMOTION_THRESHOLD,
            'demotion': self.DEMOTION_STALENESS,
            'prune_floor': self.PRUNE_FLOOR,
            'autotune': self.AUTOTUNE_INTERVAL,
        }
        return defaults.get(name, 0)

    @property
    def anthropic_client(self):
        """Lazy-initialize Anthropic client for direct mode."""
        if self._anthropic is None and ANTHROPIC_AVAILABLE:
            # GL-083: Support AOA_ANTHROPIC_KEY (preferred) and ANTHROPIC_API_KEY (fallback)
            api_key = os.environ.get('AOA_ANTHROPIC_KEY') or os.environ.get('ANTHROPIC_API_KEY')
            if api_key:
                self._anthropic = anthropic.Anthropic(api_key=api_key)
        return self._anthropic

    # =========================================================================
    # Redis Key Helpers
    # =========================================================================

    def _key(self, suffix: str) -> str:
        """Generate Redis key with project prefix."""
        return f"aoa:{self.project_id}:{suffix}"

    def _domain_key(self, name: str, suffix: str) -> str:
        """Generate domain-specific Redis key."""
        return f"aoa:{self.project_id}:domain:{name}:{suffix}"

    def _term_key(self, term: str, suffix: str) -> str:
        """Generate term-specific Redis key."""
        return f"aoa:{self.project_id}:term:{term}:{suffix}"

    # =========================================================================
    # Prompt Counter (Batch Trigger)
    # =========================================================================

    def increment_prompt_count(self) -> int:
        """Increment and return current prompt count."""
        key = self._key("prompt_count")
        return self.redis.client.incr(key)

    def get_prompt_count(self) -> int:
        """Get current prompt count."""
        key = self._key("prompt_count")
        val = self.redis.client.get(key)
        return int(val) if val else 0

    def reset_prompt_count(self) -> None:
        """Reset prompt counter after batch processing."""
        key = self._key("prompt_count")
        self.redis.client.set(key, 0)

    def should_learn(self) -> bool:
        """Check if we've accumulated enough prompts for a batch."""
        return self.get_prompt_count() >= self.BATCH_SIZE

    # =========================================================================
    # Learning Pending Flag (Hook Signal)
    # =========================================================================

    def set_learning_pending(self) -> None:
        """Signal that learning is ready for hook to process."""
        # R-001 + A-003: Atomic set with TTL and NX (only set if not exists)
        # Prevents double-run if multiple triggers fire simultaneously
        self.redis.client.set(self._key("learning_pending"), "1", ex=3600, nx=True)

    def clear_learning_pending(self) -> None:
        """Clear pending flag after hook completes learning."""
        self.redis.client.delete(self._key("learning_pending"))

    def is_learning_pending(self) -> bool:
        """Check if learning is waiting for hook to process."""
        val = self.redis.client.get(self._key("learning_pending"))
        return val == b"1" or val == "1"

    # =========================================================================
    # Tune Counter (Regenerative Tune Trigger)
    # =========================================================================

    def increment_tune_count(self) -> int:
        """Increment and return current tune count (called on each intent)."""
        key = self._key("tune_count")
        return self.redis.client.incr(key)

    def get_tune_count(self) -> int:
        """Get current tune count."""
        key = self._key("tune_count")
        val = self.redis.client.get(key)
        return int(val) if val else 0

    def reset_tune_count(self) -> None:
        """Reset tune counter after regenerative tune."""
        key = self._key("tune_count")
        self.redis.client.set(key, 0)

    def should_tune(self) -> bool:
        """Check if we've accumulated enough intents for regenerative tune."""
        # Must have at least some domains to tune
        if not self.get_all_domains():
            return False
        return self.get_tune_count() >= self.TUNE_THRESHOLD

    def get_last_tune(self) -> int:
        """Get timestamp of last regenerative tune."""
        key = self._key("last_tune")
        val = self.redis.client.get(key)
        return int(val) if val else 0

    def set_last_tune(self) -> None:
        """Update tune timestamp to now."""
        key = self._key("last_tune")
        self.redis.client.set(key, int(time.time()))

    # =========================================================================
    # Tuning Pending Flag (Hook Signal)
    # =========================================================================

    def set_tuning_pending(self) -> None:
        """Signal that tuning is ready for hook to process."""
        # R-002: Add 1-hour TTL so flag auto-clears if tuning fails
        self.redis.client.setex(self._key("tuning_pending"), 3600, "1")

    def clear_tuning_pending(self) -> None:
        """Clear pending flag after hook completes tuning."""
        self.redis.client.delete(self._key("tuning_pending"))

    def is_tuning_pending(self) -> bool:
        """Check if tuning is waiting for hook to process."""
        val = self.redis.client.get(self._key("tuning_pending"))
        return val == b"1" or val == "1"

    # =========================================================================
    # Learning History (GL-054)
    # =========================================================================

    def get_last_learn(self) -> int:
        """Get timestamp of last learning cycle."""
        key = self._key("last_learn")
        val = self.redis.client.get(key)
        return int(val) if val else 0

    def set_last_learn(self, terms_count: int = 0, terms_list: list[str] = None, domains_list: list[str] = None) -> None:
        """Update learning timestamp and record what was learned."""
        self.redis.client.set(self._key("last_learn"), int(time.time()))
        self.redis.client.set(self._key("terms_learned_last"), terms_count)
        # Store actual terms and domains learned (for display)
        if terms_list:
            key = self._key("terms_learned_list")
            self.redis.client.delete(key)
            self.redis.client.rpush(key, *terms_list[:20])
            self.redis.client.ltrim(key, 0, 19)  # R-015: Redis-enforced cap at 20
        if domains_list:
            key = self._key("domains_learned_list")
            self.redis.client.delete(key)
            self.redis.client.rpush(key, *domains_list[:5])
            self.redis.client.ltrim(key, 0, 4)  # R-015: Redis-enforced cap at 5

    def get_terms_learned_last(self) -> int:
        """Get count of terms learned in last learning cycle."""
        key = self._key("terms_learned_last")
        val = self.redis.client.get(key)
        return int(val) if val else 0

    def get_learned_details(self) -> dict:
        """Get details of what was learned in last cycle."""
        return {
            'terms': list(self.redis.client.lrange(self._key("terms_learned_list"), 0, -1)),
            'domains': list(self.redis.client.lrange(self._key("domains_learned_list"), 0, -1)),
        }

    # =========================================================================
    # Token Investment Tracking (GL-054)
    # =========================================================================

    def add_tokens_invested(self, input_tokens: int, output_tokens: int) -> None:
        """Track tokens invested in domain learning."""
        total = input_tokens + output_tokens
        self.redis.client.incrby(self._key("tokens_invested"), total)
        self.redis.client.set(self._key("tokens_invested_last"), total)
        self.redis.client.incr(self._key("learning_calls"))

    def get_tokens_invested(self) -> int:
        """Get total tokens invested in domain learning."""
        val = self.redis.client.get(self._key("tokens_invested"))
        return int(val) if val else 0

    def get_tokens_invested_last(self) -> int:
        """Get tokens invested in last learning call."""
        val = self.redis.client.get(self._key("tokens_invested_last"))
        return int(val) if val else 0

    def get_learning_calls(self) -> int:
        """Get total number of learning API calls made."""
        val = self.redis.client.get(self._key("learning_calls"))
        return int(val) if val else 0

    def set_autotune_results(self, merged: int = 0, pruned: int = 0) -> None:
        """Record results of last auto-tune."""
        self.redis.client.set(self._key("autotune_merged_last"), merged)
        self.redis.client.set(self._key("autotune_pruned_last"), pruned)

    def get_autotune_results(self) -> dict:
        """Get results of last auto-tune."""
        return {
            'merged': int(self.redis.client.get(self._key("autotune_merged_last")) or 0),
            'pruned': int(self.redis.client.get(self._key("autotune_pruned_last")) or 0),
        }

    def set_last_autotune(self) -> None:
        """Update autotune timestamp to now."""
        self.redis.client.set(self._key("last_autotune"), int(time.time()))

    # =========================================================================
    # Domain Storage
    # =========================================================================

    def get_all_domains(self) -> list[str]:
        """Get all domain names for this project."""
        key = self._key("domains")
        return list(self.redis.client.smembers(key))

    def add_domain(self, domain: Domain, source: str = "learned", tier: str = None) -> None:
        """Add a new domain with metadata and terms.

        Args:
            domain: Domain object with name, description, confidence, terms
            source: "seeded" for quickstart domains, "learned" for Haiku-discovered
            tier: "core" or "context" (GL-090). Defaults based on source.
        """
        # GL-090: Determine tier based on source if not specified
        if tier is None:
            # Intelligence phase (skeleton/seeded) → core, intent phase → context
            tier = "core" if source in ("skeleton", "seeded") else "context"

        # Add to domain set
        domains_key = self._key("domains")
        self.redis.client.sadd(domains_key, domain.name)

        # Get current intent count for tracking
        intent_count = int(self.redis.client.get(self._key("intent_count")) or 0)

        # Store metadata with lifecycle fields (GL-059.1) and enrichment (GL-085)
        meta_key = self._domain_key(domain.name, "meta")
        self.redis.client.hset(meta_key, mapping={
            "description": domain.description,
            "confidence": domain.confidence,
            "hits": self.STARTER_HITS,
            "created": int(time.time()),
            # GL-059.1: Lifecycle fields
            "source": source,  # "seeded" | "learned" | "skeleton" | "intent"
            "state": "active",  # "active" | "stale" | "deprecated"
            "stale_cycles": 0,
            "hits_last_cycle": 0,
            # GL-085: Lazy enrichment
            "enriched": "false",  # "false" until keywords added
            # GL-090: Two-tier curation
            "tier": tier,  # "core" | "context"
            "total_hits": 0,  # Lifetime hits (never decayed, for promotion)
            "last_hit_at": intent_count,  # Intent count when last hit (for demotion)
        })

        # Update project-level source counters (GL-059.1)
        if source == "seeded":
            self.redis.client.incr(self._key("seeded_count"))
        else:
            self.redis.client.incr(self._key("learned_count"))

        # Store terms
        terms_key = self._domain_key(domain.name, "terms")
        if domain.terms:
            self.redis.client.sadd(terms_key, *domain.terms)

        # Create term->domain mappings with starter scores
        for term in domain.terms:
            term_key = self._key(f"term:{term}")
            self.redis.client.zadd(term_key, {domain.name: self.STARTER_SCORE})

    def get_domain_meta(self, name: str) -> dict:
        """Get domain metadata."""
        meta_key = self._domain_key(name, "meta")
        return self.redis.client.hgetall(meta_key)

    def get_domain_terms(self, name: str) -> set[str]:
        """Get terms for a domain."""
        terms_key = self._domain_key(name, "terms")
        return self.redis.client.smembers(terms_key)

    def get_domains_for_term(self, term: str) -> list[str]:
        """Get domains that have this term (GL-060.3: term -> domain lookup)."""
        term_key = self._key(f"term:{term}:domain")  # SCHEMA-001: correct key suffix
        domain = self.redis.client.get(term_key)
        # DEBUG: Log key lookups when AOA_DEBUG is set
        if os.environ.get("AOA_DEBUG") == "1":
            print(f"[LEARNER DEBUG] get_domains_for_term('{term}') -> key='{term_key}' -> domain={domain}", flush=True)
        if not domain:
            return []
        domain_str = domain.decode() if isinstance(domain, bytes) else domain
        return [domain_str] if domain_str else []

    def increment_domain_hits(self, name: str) -> int:
        """Increment hit counter for a domain.

        GL-090: Also tracks total_hits (lifetime) and last_hit_at (for demotion).
        """
        meta_key = self._domain_key(name, "meta")
        # Use prompt_count for last_hit_at tracking (incremented on each intent)
        prompt_count = self.get_prompt_count()

        # Increment both decayed hits and lifetime total_hits
        pipe = self.redis.client.pipeline()
        pipe.hincrby(meta_key, "hits", 1)
        pipe.hincrby(meta_key, "total_hits", 1)
        pipe.hset(meta_key, "last_hit_at", prompt_count)
        results = pipe.execute()

        # DEBUG: Log hit increments when AOA_DEBUG is set
        if os.environ.get("AOA_DEBUG") == "1":
            print(f"[LEARNER DEBUG] increment_domain_hits('{name}') -> hits={results[0]}, total_hits={results[1]}", flush=True)

        return results[0]  # Return current hits count

    # =========================================================================
    # GL-085: Lazy Domain Enrichment
    # =========================================================================

    def is_domain_enriched(self, name: str) -> bool:
        """Check if domain has been enriched with keywords."""
        meta_key = self._domain_key(name, "meta")
        val = self.redis.client.hget(meta_key, "enriched")
        return val == "true" or val == b"true"

    def set_domain_enriched(self, name: str, enriched: bool = True) -> None:
        """Mark domain as enriched (keywords have been added)."""
        meta_key = self._domain_key(name, "meta")
        self.redis.client.hset(meta_key, "enriched", "true" if enriched else "false")

    # =========================================================================
    # GL-090: Two-Tier Domain Curation
    # =========================================================================

    def get_domain_tier(self, name: str) -> str:
        """Get domain tier (core or context)."""
        meta_key = self._domain_key(name, "meta")
        tier = self.redis.client.hget(meta_key, "tier")
        return tier if tier else "core"  # Default to core for legacy domains

    def set_domain_tier(self, name: str, tier: str) -> None:
        """Set domain tier."""
        meta_key = self._domain_key(name, "meta")
        self.redis.client.hset(meta_key, "tier", tier)

    def count_domains_by_tier(self) -> dict:
        """Count domains in each tier."""
        counts = {"core": 0, "context": 0}
        for name in self.get_all_domains():
            tier = self.get_domain_tier(name)
            counts[tier] = counts.get(tier, 0) + 1
        return counts

    def get_domains_by_tier(self, tier: str) -> list[str]:
        """Get all domain names in a specific tier."""
        return [name for name in self.get_all_domains() if self.get_domain_tier(name) == tier]

    def promote_domain(self, name: str) -> bool:
        """Promote domain from context to core tier.

        GL-090: Called when total_hits >= PROMOTION_THRESHOLD.
        Returns True if promoted, False if already core or doesn't exist.
        """
        current_tier = self.get_domain_tier(name)
        if current_tier == "context":
            self.set_domain_tier(name, "core")
            return True
        return False

    def demote_domain(self, name: str) -> bool:
        """Demote domain from core to context tier.

        GL-090: Called when domain has 0 hits for DEMOTION_STALENESS intents.
        Returns True if demoted, False if already context or doesn't exist.
        """
        current_tier = self.get_domain_tier(name)
        if current_tier == "core":
            self.set_domain_tier(name, "context")
            return True
        return False

    def can_add_context_domain(self) -> bool:
        """Check if we can add another context domain (under max)."""
        counts = self.count_domains_by_tier()
        return counts.get("context", 0) < self.CONTEXT_DOMAINS_MAX

    def can_add_core_domain(self) -> bool:
        """Check if we can add another core domain (under max)."""
        counts = self.count_domains_by_tier()
        return counts.get("core", 0) < self.CORE_DOMAINS_MAX

    def apply_decay(self) -> int:
        """Apply decay to all domain hits.

        GL-090: Multiplies hits by DECAY_RATE (0.80).
        Returns count of domains decayed.

        R-006: Uses Lua script for atomic read-multiply-write to prevent race conditions.
        """
        # Lua script for atomic decay: read hits, multiply by decay rate, write back
        decay_script = """
        local meta_key = KEYS[1]
        local decay_rate = tonumber(ARGV[1])

        local hits = tonumber(redis.call('HGET', meta_key, 'hits') or 0)
        local new_hits = hits * decay_rate
        redis.call('HSET', meta_key, 'hits', new_hits)
        return 1
        """

        count = 0
        for name in self.get_all_domains():
            meta_key = self._domain_key(name, "meta")
            self.redis.client.eval(decay_script, 1, meta_key, self.DECAY_RATE)
            count += 1
        return count

    def get_intent_count(self) -> int:
        """Get current intent count for this project."""
        return int(self.redis.client.get(self._key("intent_count")) or 0)

    def increment_intent_count(self) -> int:
        """Increment and return intent count."""
        return self.redis.client.incr(self._key("intent_count"))

    def get_unenriched_domain(self) -> Optional[dict]:
        """
        Get one domain that needs enrichment.

        Returns dict with name, description, terms or None if all enriched.
        """
        for name in self.get_all_domains():
            if not self.is_domain_enriched(name):
                meta = self.get_domain_meta(name)
                terms = list(self.get_domain_terms(name))
                return {
                    'name': name,
                    'description': meta.get('description', ''),
                    'terms': terms
                }
        return None

    def get_unenriched_domains(self, limit: int = 10) -> list[str]:
        """
        Get names of all unenriched domains (up to limit).

        GL-088: Used by `aoa domains pending` for batch processing.
        Returns list of domain names that need enrichment.
        """
        unenriched = []
        for name in self.get_all_domains():
            if not self.is_domain_enriched(name):
                unenriched.append(name)
                if len(unenriched) >= limit:
                    break
        return unenriched

    def get_enrichment_status(self) -> dict:
        """Get enrichment progress: enriched count, total count."""
        domains = self.get_all_domains()
        total = len(domains)
        enriched = sum(1 for d in domains if self.is_domain_enriched(d))
        return {
            'enriched': enriched,
            'total': total,
            'pending': total - enriched,
            'complete': enriched == total and total > 0,
            'prompt_count': self.get_prompt_count()  # INT-001: for status line
        }

    def add_term_keywords(self, term: str, keywords: list[str]) -> int:
        """
        Store keywords for a term.

        GL-085: Keywords stored in Redis SET at term:{name}:keywords
        Returns number of keywords added.
        """
        if not keywords:
            return 0
        keywords_key = self._term_key(term, "keywords")
        # Normalize keywords: lowercase, filter short ones
        clean_keywords = [k.lower() for k in keywords if len(k) >= 3]
        if clean_keywords:
            return self.redis.client.sadd(keywords_key, *clean_keywords)
        return 0

    def get_term_keywords(self, term: str) -> set[str]:
        """Get keywords for a term."""
        keywords_key = self._term_key(term, "keywords")
        return self.redis.client.smembers(keywords_key)

    def enrich_domain(self, name: str, term_keywords: dict[str, list[str]]) -> dict:
        """
        Add keywords to a domain's terms and mark as enriched.

        Args:
            name: Domain name (e.g., "@authentication")
            term_keywords: Dict of {term_name: [keyword1, keyword2, ...]}

        Returns:
            Summary of what was added
        """
        terms_added = 0
        keywords_added = 0

        # Add terms to domain's term set
        terms_key = self._domain_key(name, "terms")
        term_names = list(term_keywords.keys())
        if term_names:
            self.redis.client.sadd(terms_key, *term_names)

        for term, keywords in term_keywords.items():
            count = self.add_term_keywords(term, keywords)
            if count > 0:
                terms_added += 1
                keywords_added += count

            # Store term → domain mapping (for domain lookup)
            term_domain_key = self._key(f"term:{term}:domain")
            self.redis.client.set(term_domain_key, name)

            # Create keyword → term reverse index (for grep tagging)
            # Keywords map to TERM names, not domain names
            # e.g., "boost" → "result_ranking" (not "boost" → "@search")
            for kw in keywords:
                clean_kw = kw.lower()
                if len(clean_kw) >= 3:
                    kw_key = self._key(f"keyword:{clean_kw}")
                    self.redis.client.zadd(kw_key, {term: self.STARTER_SCORE})

        # Mark domain as enriched
        self.set_domain_enriched(name, True)

        return {
            'domain': name,
            'terms_enriched': terms_added,
            'keywords_added': keywords_added
        }

    def init_skeleton(self, domains: list[dict]) -> dict:
        """
        Initialize domains from skeleton (names + terms only, no keywords).

        GL-085: Called by /aoa-start skill. Sets enriched=false on all.
        GL-090: Respects CORE_DOMAINS_MAX cap.
        GL-091: Auto-queues ENRICH jobs for each created domain.

        Args:
            domains: List of {name, description, terms[]}

        Returns:
            Summary of domains created
        """
        created = []
        descriptions = {}  # name -> description for job queue
        skipped = 0
        for d in domains:
            # GL-090: Check core tier cap before adding
            if not self.can_add_core_domain():
                skipped += 1
                continue

            name = d.get('name', '')
            if not name.startswith('@'):
                name = f"@{name}"

            # Skip if domain already exists
            if name in self.get_all_domains():
                skipped += 1
                continue

            domain = Domain(
                name=name,
                description=d.get('description', ''),
                confidence=0.5,  # Skeleton confidence
                terms=d.get('terms', [])
            )
            self.add_domain(domain, source="skeleton")
            created.append(name)
            descriptions[name] = d.get('description', '')

            # Create term→domain mappings so hits can be tracked immediately
            for term in d.get('terms', []):
                term_domain_key = self._key(f"term:{term}:domain")
                self.redis.client.set(term_domain_key, name)

        # GL-091: Auto-queue ENRICH jobs for created domains
        jobs_queued = 0
        if JOBS_AVAILABLE and created:
            try:
                q = JobQueue(self.project_id)
                jobs = [create_enrich_job(self.project_id, name, descriptions.get(name, ''))
                        for name in created]
                jobs_queued = q.push_many(jobs)
            except Exception as e:
                # Log but don't fail - jobs can be queued manually
                print(f"[DomainLearner] Warning: Could not auto-queue jobs: {e}")

        return {
            'domains_created': len(created),
            'domains': created,
            'skipped': skipped,
            'jobs_queued': jobs_queued
        }

    def rebuild_term_mappings(self) -> dict:
        """
        Rebuild term→domain mappings from existing domain data.

        Useful after Redis data loss or when mappings are missing.
        Creates: aoa:{project}:term:{term}:domain → domain_name
        """
        domains = self.get_all_domains()
        terms_mapped = 0
        domains_processed = 0

        for domain_name in domains:
            terms = self.get_domain_terms(domain_name)
            for term in terms:
                # Decode bytes if needed
                if isinstance(term, bytes):
                    term = term.decode()
                term_domain_key = self._key(f"term:{term}:domain")
                self.redis.client.set(term_domain_key, domain_name)
                terms_mapped += 1
            domains_processed += 1

        return {
            'domains_processed': domains_processed,
            'terms_mapped': terms_mapped
        }

    def get_enrichment_prompt(self, domain: dict) -> str:
        """
        Generate Haiku prompt for enriching one domain with keywords.

        GL-085: Small focused prompt for lazy enrichment.
        """
        name = domain.get('name', '')
        description = domain.get('description', '')
        terms = domain.get('terms', [])

        terms_list = '\n'.join(f"- {t}" for t in terms)

        return f"""Add keywords for domain {name}: {description}

Terms to enrich:
{terms_list}

For each term, provide 7-10 specific keywords that would match code in this domain.
Keywords must be:
- Single words only (no underscores)
- 3+ characters
- Actual identifiers from code (function names, variable names, etc.)
- NOT generic words (get, set, data, file, handle, process)

Return JSON only:
{{"term_name": ["keyword1", "keyword2", ...], ...}}"""

    # =========================================================================
    # GL-059.1: Lifecycle Management
    # =========================================================================

    def get_domain_source(self, name: str) -> str:
        """Get domain source: 'seeded' or 'learned'."""
        meta_key = self._domain_key(name, "meta")
        return self.redis.client.hget(meta_key, "source") or "seeded"

    def get_domain_state(self, name: str) -> str:
        """Get domain state: 'active', 'stale', or 'deprecated'."""
        meta_key = self._domain_key(name, "meta")
        return self.redis.client.hget(meta_key, "state") or "active"

    def set_domain_state(self, name: str, state: str) -> None:
        """Update domain state."""
        if state not in ("active", "stale", "deprecated"):
            raise ValueError(f"Invalid state: {state}")
        meta_key = self._domain_key(name, "meta")
        self.redis.client.hset(meta_key, "state", state)

    def increment_stale_cycles(self, name: str) -> int:
        """Increment stale cycle counter. Returns new value."""
        meta_key = self._domain_key(name, "meta")
        return self.redis.client.hincrby(meta_key, "stale_cycles", 1)

    def reset_stale_cycles(self, name: str) -> None:
        """Reset stale cycles to 0 (domain became active again)."""
        meta_key = self._domain_key(name, "meta")
        self.redis.client.hset(meta_key, "stale_cycles", 0)

    def get_hits_last_cycle(self, name: str) -> int:
        """Get hits from last tune cycle."""
        meta_key = self._domain_key(name, "meta")
        return int(float(self.redis.client.hget(meta_key, "hits_last_cycle") or 0))

    def snapshot_cycle_hits(self) -> None:
        """Snapshot current hits to hits_last_cycle for all domains, then reset.

        R-007: Uses Lua script for atomic copy to prevent race conditions.
        """
        # Lua script for atomic snapshot: read hits, copy to hits_last_cycle
        snapshot_script = """
        local meta_key = KEYS[1]

        local hits = redis.call('HGET', meta_key, 'hits') or '0'
        redis.call('HSET', meta_key, 'hits_last_cycle', hits)
        return 1
        """

        for name in self.get_all_domains():
            meta_key = self._domain_key(name, "meta")
            self.redis.client.eval(snapshot_script, 1, meta_key)

    def get_source_counts(self) -> dict:
        """Get counts of seeded vs learned domains.

        Handles legacy domains: if counters are 0 but domains exist,
        counts them by reading source field (defaults to 'seeded').
        """
        seeded = int(self.redis.client.get(self._key("seeded_count")) or 0)
        learned = int(self.redis.client.get(self._key("learned_count")) or 0)

        # Backfill for legacy domains without counters
        if seeded == 0 and learned == 0:
            domains = self.get_all_domains()
            if domains:
                for name in domains:
                    source = self.get_domain_source(name)
                    if source == "learned":
                        learned += 1
                    else:
                        seeded += 1
                # Cache for future calls
                if seeded > 0:
                    self.redis.client.set(self._key("seeded_count"), seeded)
                if learned > 0:
                    self.redis.client.set(self._key("learned_count"), learned)

        return {"seeded": seeded, "learned": learned}

    def increment_term_hits(self, terms, amount: int = 1) -> int:
        """
        Increment hit counters for term(s).

        Args:
            terms: Single term (str) or list of terms (list[str])
            amount: Amount to increment (only used for single term)

        Returns:
            New hit count (single term) or 0 (batch)
        """
        term_hits_key = self._key("term_hits")

        # Handle list of terms (batch mode)
        if isinstance(terms, list):
            if not terms:
                return 0
            pipe = self.redis.client.pipeline()
            for term in terms:
                pipe.hincrby(term_hits_key, term.lower(), 1)
            pipe.execute()
            return 0

        # Handle single term
        return self.redis.client.hincrby(term_hits_key, terms.lower(), amount)

    def get_term_hits(self, terms: list[str] = None) -> dict:
        """Get hit counts for terms. If terms=None, returns all."""
        term_hits_key = self._key("term_hits")
        if terms:
            pipe = self.redis.client.pipeline()
            for term in terms:
                pipe.hget(term_hits_key, term)
            results = pipe.execute()
            return {term: int(hits or 0) for term, hits in zip(terms, results)}
        else:
            all_hits = self.redis.client.hgetall(term_hits_key)
            return {term: int(hits) for term, hits in all_hits.items()}

    # =========================================================================
    # P3-1: Keyword Hit Tracking
    # =========================================================================

    def increment_keyword_hits(self, keywords: list[str], amount: int = 1) -> int:
        """
        Increment hit counters for keyword(s).

        P3-1: Finer-grained learning signal - tracks which specific keywords
        drive search results, not just terms.

        Args:
            keywords: List of keywords that matched
            amount: Amount to increment per keyword

        Returns:
            Number of keywords incremented
        """
        if not keywords:
            return 0
        keyword_hits_key = self._key("keyword_hits")
        pipe = self.redis.client.pipeline()
        for kw in keywords:
            pipe.hincrby(keyword_hits_key, kw.lower(), amount)
        pipe.execute()
        return len(keywords)

    def get_keyword_hits(self, keywords: list[str] = None) -> dict:
        """Get hit counts for keywords. If keywords=None, returns all."""
        keyword_hits_key = self._key("keyword_hits")
        if keywords:
            pipe = self.redis.client.pipeline()
            for kw in keywords:
                pipe.hget(keyword_hits_key, kw.lower())
            results = pipe.execute()
            return {kw: int(hits or 0) for kw, hits in zip(keywords, results)}
        else:
            all_hits = self.redis.client.hgetall(keyword_hits_key)
            return {kw: int(hits) for kw, hits in all_hits.items()}

    # =========================================================================
    # GL-069.1: Orphan Tag Storage
    # =========================================================================

    def add_orphan_tags(self, tags: list[str]) -> int:
        """
        Store unmatched tags as orphans for the learning cycle.

        Orphan tags are semantic tags that didn't match any existing domain terms.
        They represent unmet intent and inform domain discovery in the next
        learning cycle.

        Returns: number of orphan tags added
        """
        if not tags:
            return 0
        orphan_key = self._key("orphan_tags")
        # Use sorted set with timestamp as score for ordering
        now = time.time()
        # R-010: Clean up orphans older than 60 days
        cutoff = now - (60 * 24 * 3600)  # 60 days ago
        self.redis.client.zremrangebyscore(orphan_key, '-inf', cutoff)

        pipe = self.redis.client.pipeline()
        for tag in tags:
            # Normalize: lowercase, strip #
            clean_tag = tag.lower().lstrip('#')
            if len(clean_tag) >= 2:
                pipe.zadd(orphan_key, {clean_tag: now})
        pipe.expire(orphan_key, 604800)  # R-014: 7-day TTL fallback
        results = pipe.execute()
        return sum(1 for r in results if r) - 1  # -1 for expire result

    def get_orphan_tags(self, limit: int = 50) -> list[str]:
        """
        Get orphan tags for the learning cycle.

        Returns most recent orphan tags (newest first).
        """
        orphan_key = self._key("orphan_tags")
        # Get by score descending (newest first)
        return list(self.redis.client.zrevrange(orphan_key, 0, limit - 1))

    def clear_orphan_tags(self, tags: list[str] = None) -> int:
        """
        Clear orphan tags.

        If tags provided, only clear those specific tags (they were matched).
        If tags=None, clear all orphans (full reset).

        Returns: number of tags cleared
        """
        orphan_key = self._key("orphan_tags")
        if tags:
            pipe = self.redis.client.pipeline()
            for tag in tags:
                clean_tag = tag.lower().lstrip('#')
                pipe.zrem(orphan_key, clean_tag)
            results = pipe.execute()
            return sum(1 for r in results if r)
        else:
            count = self.redis.client.zcard(orphan_key)
            self.redis.client.delete(orphan_key)
            return count

    def get_orphan_count(self) -> int:
        """Get count of orphan tags waiting for learning."""
        orphan_key = self._key("orphan_tags")
        return self.redis.client.zcard(orphan_key)

    def increment_orphan_hits(self, tags: list[str]) -> int:
        """
        P3-3: Increment hit count for orphan tags.

        Orphan tags that get searched frequently should be prioritized
        for domain creation. This tracks usage before they're assigned.

        Args:
            tags: Orphan tags that were searched

        Returns:
            Number of orphans incremented
        """
        if not tags:
            return 0
        orphan_hits_key = self._key("orphan_hits")
        pipe = self.redis.client.pipeline()
        for tag in tags:
            clean_tag = tag.lower().lstrip('#')
            if len(clean_tag) >= 2:
                pipe.hincrby(orphan_hits_key, clean_tag, 1)
        pipe.execute()
        return len(tags)

    def get_orphan_hits(self) -> dict:
        """P3-3: Get hit counts for orphan tags (for priority sorting)."""
        orphan_hits_key = self._key("orphan_hits")
        all_hits = self.redis.client.hgetall(orphan_hits_key)
        return {tag: int(hits) for tag, hits in all_hits.items()}

    def add_prompt_record(self, goal: str, tags: list) -> bool:
        """
        GL-078: Store a prompt record (goal + tags) for grouped learning.

        Stores the last 10 prompts with their goals and tags as JSON.
        Used for display (last 2) and learning cycle (last 10).

        Args:
            goal: Developer's goal for this prompt
            tags: List of tag dicts [{"tag": "name", "score": N}, ...]

        Returns: True if stored successfully
        """
        import json
        import time

        prompts_key = self._key("prompt_records")
        record = {
            "goal": goal,
            "tags": tags,
            "timestamp": time.time()
        }

        try:
            # Add to front of list (LPUSH = newest first)
            self.redis.client.lpush(prompts_key, json.dumps(record))
            # Keep only last 10
            self.redis.client.ltrim(prompts_key, 0, 9)
            return True
        except Exception:
            return False

    def get_prompt_records(self, limit: int = 10) -> list[dict]:
        """
        GL-078: Get recent prompt records for display or learning.

        Args:
            limit: Max number of records (default 10)

        Returns: List of {"goal": "...", "tags": [...], "timestamp": N}
        """
        import json

        prompts_key = self._key("prompt_records")
        try:
            records = self.redis.client.lrange(prompts_key, 0, limit - 1)
            return [json.loads(r) for r in records]
        except Exception:
            return []

    def match_tags_to_terms(self, tags: list[str]) -> dict:
        """
        Match semantic tags against existing domain terms.

        For each tag, checks if it matches any domain term.
        Returns dict with 'matched' and 'orphaned' lists.
        Also increments hit counters for matched terms.
        """
        if not tags:
            return {'matched': [], 'orphaned': []}

        matched = []
        orphaned = []

        for tag in tags:
            clean_tag = tag.lower().lstrip('#')
            if len(clean_tag) < 2:
                continue

            # Check if tag matches any domain term
            domains = self.get_domains_for_term(clean_tag)
            if domains:
                matched.append(clean_tag)
            else:
                orphaned.append(clean_tag)

        # Increment hits for matched terms
        if matched:
            self.increment_term_hits(matched)

        # Store orphaned tags for learning cycle
        if orphaned:
            self.add_orphan_tags(orphaned)

        return {'matched': matched, 'orphaned': orphaned}

    def remove_domain(self, name: str) -> None:
        """Remove a domain and all its data.

        R-008: Includes cascade cleanup of keyword_index entries.
        """
        # Get terms first for cleanup
        terms = self.get_domain_terms(name)

        # Remove from domain set
        domains_key = self._key("domains")
        self.redis.client.srem(domains_key, name)

        # Remove metadata
        meta_key = self._domain_key(name, "meta")
        self.redis.client.delete(meta_key)

        # Remove terms set
        terms_key = self._domain_key(name, "terms")
        self.redis.client.delete(terms_key)

        # Remove domain from term mappings and cascade cleanup keywords
        index_key = self._key("keyword_index")
        for term in terms:
            # R-008: Clean up keyword_index entries for this term's keywords
            keywords_key = self._term_key(term, "keywords")
            keywords = self.redis.client.smembers(keywords_key)
            if keywords:
                # Remove keywords from keyword_index (they pointed to this term)
                self.redis.client.hdel(index_key, *keywords)
            # Delete the term's keywords set
            self.redis.client.delete(keywords_key)

            # Remove term->domain mapping
            term_key = self._key(f"term:{term}")
            self.redis.client.zrem(term_key, name)

    # =========================================================================
    # Term Lookup
    # =========================================================================

    def lookup_term(self, term: str) -> list[tuple[str, float]]:
        """
        Look up which domains a term maps to.

        Returns list of (domain_name, score) tuples, highest score first.
        """
        term_key = self._key(f"term:{term}")
        results = self.redis.client.zrevrange(term_key, 0, -1, withscores=True)
        return [(name, score) for name, score in results]

    def get_domain_for_symbol(self, symbol: str, track_hits: bool = False) -> Optional[str]:
        """
        Get the best matching domain for a symbol name.

        Tokenizes the symbol and aggregates domain scores.
        If track_hits=True, also increments hit counters for matched terms.
        """
        # Simple tokenization: split on non-alphanumeric
        import re
        tokens = re.split(r'[^a-zA-Z0-9]+', symbol.lower())
        tokens = [t for t in tokens if len(t) > 2]

        if not tokens:
            return None

        # Aggregate scores across all tokens
        scores: dict[str, float] = {}
        matched_terms: list[str] = []
        for token in tokens:
            results = self.lookup_term(token)
            if results:
                matched_terms.append(token)
                for domain, score in results:
                    scores[domain] = scores.get(domain, 0) + score

        if not scores:
            return None

        # Track term hits if requested
        if track_hits and matched_terms:
            self.increment_term_hits(matched_terms)

        # Return highest-scoring domain
        return max(scores.items(), key=lambda x: x[1])[0]

    # =========================================================================
    # Haiku Prompts (for hook mode)
    # =========================================================================

    def get_discovery_prompt(self, prompts: list[str], files: list[str],
                             existing_domains: list[str]) -> str:
        """
        Generate the domain discovery prompt for Haiku.

        Used in hook mode where Claude Task agent calls Haiku.
        """
        return f"""Given this developer session context, identify 1-3 HIGH-LEVEL semantic domains.

Session prompts (last {len(prompts)}):
{chr(10).join(f'- {p[:200]}' for p in prompts[:10])}

Files touched:
{chr(10).join(f'- {f}' for f in files[:20])}

Existing domains (don't duplicate):
{chr(10).join(f'- {d}' for d in existing_domains)}

Output valid JSON only:
{{"domains": [{{"name": "@domain_name", "confidence": 0.8, "description": "What this domain represents"}}]}}"""

    def get_terms_prompt(self, domains: list[str], file_samples: dict[str, str]) -> str:
        """
        Generate the term generation prompt for Haiku.
        """
        samples_text = "\n\n".join(
            f"=== {path} ===\n{content[:500]}"
            for path, content in list(file_samples.items())[:5]
        )
        return f"""Generate 5-8 key terms for each domain. Terms should be:
- Function names, variable patterns, keywords found in code
- Specific enough for exact matching
- Actually present in the code samples

Domains:
{chr(10).join(f'- {d}' for d in domains)}

Code samples:
{samples_text}

Output valid JSON only:
{{"@domain_name": ["term1", "term2", ...]}}"""

    def get_autotune_prompt(self, domains_with_meta: list[dict]) -> str:
        """
        Generate the auto-tune prompt for Haiku.
        """
        domains_text = "\n".join(
            f"- {d['name']}: hits={d.get('hits', 0)}, confidence={d.get('confidence', 0)}, "
            f"created={d.get('created', 0)}, terms={d.get('terms', [])}"
            for d in domains_with_meta
        )
        return f"""Review all domains and terms. Your job:
1. MERGE overlapping domains (>70% term overlap)
2. PRUNE domains with hits < 5 AND older than 3 days
3. DEDUPE terms that appear in multiple domains (boost winner, decay loser)

Current state:
{domains_text}

Output valid JSON only:
{{
  "merge": [{{"from": "@old_domain", "into": "@target_domain"}}],
  "prune": ["@unused_domain"],
  "reassign": {{"term": "@new_domain"}}
}}"""

    # =========================================================================
    # Direct API Mode (for testing)
    # =========================================================================

    def _call_haiku(self, prompt: str) -> Optional[dict]:
        """
        Call Haiku directly via Anthropic API.

        Only available when ANTHROPIC_API_KEY is set.
        Returns parsed JSON or None.
        Tracks token investment for transparency.
        """
        if not self.anthropic_client:
            return None

        try:
            response = self.anthropic_client.messages.create(
                model="claude-3-5-haiku-latest",
                max_tokens=1024,
                messages=[{"role": "user", "content": prompt}]
            )

            # Track tokens invested (GL-054: Intelligence Angle)
            if hasattr(response, 'usage'):
                self.add_tokens_invested(
                    response.usage.input_tokens,
                    response.usage.output_tokens
                )

            text = response.content[0].text
            # Extract JSON from response
            import re
            json_match = re.search(r'\{.*\}', text, re.DOTALL)
            if json_match:
                return json.loads(json_match.group())
        except Exception as e:
            print(f"Haiku call failed: {e}")
        return None

    def discover_domains_direct(self, prompts: list[str], files: list[str]) -> list[Domain]:
        """
        Discover domains using direct Haiku API call.

        Only works when ANTHROPIC_API_KEY is set.
        """
        existing = self.get_all_domains()
        prompt = self.get_discovery_prompt(prompts, files, existing)
        result = self._call_haiku(prompt)

        if not result or "domains" not in result:
            return []

        domains = []
        for d in result["domains"]:
            domains.append(Domain(
                name=d.get("name", "").strip(),
                description=d.get("description", ""),
                confidence=float(d.get("confidence", 0.5)),
                terms=[]
            ))
        return domains

    # =========================================================================
    # Keyword Tracking (GL-083 - Every-25 Rebalance)
    # =========================================================================

    def add_keyword_to_term(self, keyword: str, term: str) -> None:
        """Assign keyword to term (one keyword -> one term mapping)."""
        index_key = self._key("keyword_index")
        self.redis.client.hset(index_key, keyword.lower(), term)

    def get_term_for_keyword(self, keyword: str) -> str | None:
        """Get which term owns this keyword."""
        index_key = self._key("keyword_index")
        result = self.redis.client.hget(index_key, keyword.lower())
        # Redis returns strings with decode_responses=True
        return result if result else None

    def get_all_keywords(self) -> dict:
        """Get all keyword->term mappings."""
        index_key = self._key("keyword_index")
        result = self.redis.client.hgetall(index_key)
        # Redis returns strings with decode_responses=True
        return dict(result) if result else {}

    def record_gap_keyword(self, keyword: str) -> None:
        """Record a search keyword that found no domain match."""
        gap_key = self._key("gap_keywords")
        self.redis.client.sadd(gap_key, keyword.lower())
        # R-009: 30-day TTL to auto-cleanup stale gap keywords
        self.redis.client.expire(gap_key, 2592000)

    def get_gap_keywords(self, limit: int = 50) -> list[str]:
        """Get keywords that had no domain matches."""
        gap_key = self._key("gap_keywords")
        # Get random sample to avoid always processing same ones
        result = self.redis.client.srandmember(gap_key, limit)
        # Redis returns strings with decode_responses=True, no .decode() needed
        return list(result) if result else []

    def clear_gap_keyword(self, keyword: str) -> None:
        """Remove keyword from gaps after assignment."""
        gap_key = self._key("gap_keywords")
        self.redis.client.srem(gap_key, keyword.lower())

    def get_gap_keyword_count(self) -> int:
        """Count of unmatched keywords."""
        gap_key = self._key("gap_keywords")
        return self.redis.client.scard(gap_key) or 0

    def increment_keyword_search(self, keyword: str) -> int:
        """Track how many times a keyword was searched."""
        key = self._key(f"keyword:{keyword.lower()}:searches")
        return self.redis.client.incr(key)

    def increment_keyword_access(self, keyword: str) -> int:
        """Track how many times a keyword led to file access."""
        key = self._key(f"keyword:{keyword.lower()}:accesses")
        return self.redis.client.incr(key)

    def get_keyword_stats(self, keyword: str) -> dict:
        """Get search and access counts for a keyword."""
        search_key = self._key(f"keyword:{keyword.lower()}:searches")
        access_key = self._key(f"keyword:{keyword.lower()}:accesses")
        searches = self.redis.client.get(search_key)
        accesses = self.redis.client.get(access_key)
        return {
            "keyword": keyword,
            "searches": int(searches) if searches else 0,
            "accesses": int(accesses) if accesses else 0
        }

    # =========================================================================
    # Every-25 Rebalance (GL-083)
    # =========================================================================

    def should_rebalance(self) -> bool:
        """Check if we should run keyword rebalance."""
        threshold = int(self.get_threshold('rebalance'))
        return self.get_prompt_count() % threshold == 0

    def rebalance_keywords(self) -> dict:
        """
        Every-25 rebalance - pure Redis, no LLM needed.

        GL-083: Assigns gap keywords to best-fit terms based on co-occurrence.
        After rebalance, signals indexer to rebuild KeywordMatcher automaton.
        """
        stats = {'added': 0, 'stale_marked': 0, 'gaps_processed': 0}

        # 1. Process gap keywords
        gaps = self.get_gap_keywords(limit=30)
        stats['gaps_processed'] = len(gaps)

        for keyword in gaps:
            # Find best term based on existing domain terms
            best_term = self._find_best_term_for_keyword(keyword)
            if best_term:
                self.add_keyword_to_term(keyword, best_term)
                self.clear_gap_keyword(keyword)
                stats['added'] += 1

        # 2. Record rebalance timestamp
        self.redis.client.set(self._key("last_rebalance"), int(time.time()))

        # 3. Signal indexer to rebuild KeywordMatcher automaton
        # Non-blocking - grep will use stale data until rebuild completes
        if stats['added'] > 0:
            try:
                import requests
                index_url = os.environ.get('INDEX_URL', 'http://localhost:8080')
                requests.post(
                    f"{index_url}/keywords/rebuild",
                    params={'project_id': self.project_id},
                    timeout=1
                )
            except Exception:
                pass  # Non-blocking, don't fail rebalance if indexer unreachable

        return stats

    def _find_best_term_for_keyword(self, keyword: str) -> str | None:
        """
        Find best existing term to assign keyword to.

        Simple heuristic: find term with most character overlap.
        """
        keyword_lower = keyword.lower()
        best_term = None
        best_score = 0

        # Get all domains and their terms
        for domain_name in self.get_all_domains():
            terms = self.get_domain_terms(domain_name)
            for term in terms:
                term_lower = term.lower()
                # Score: substring match or character overlap
                if keyword_lower in term_lower or term_lower in keyword_lower:
                    score = len(keyword_lower) + len(term_lower)
                else:
                    # Character overlap
                    score = len(set(keyword_lower) & set(term_lower))

                if score > best_score:
                    best_score = score
                    best_term = term

        # Require minimum score to avoid bad matches
        return best_term if best_score >= 3 else None

    def get_last_rebalance(self) -> int:
        """Get timestamp of last rebalance."""
        val = self.redis.client.get(self._key("last_rebalance"))
        return int(val) if val else 0

    # =========================================================================
    # Auto-tune Operations (GL-053 Phase D)
    # =========================================================================

    def get_domains_with_meta(self) -> list[dict]:
        """Get all domains with their metadata and terms (sorted by hits)."""
        result = []
        # Get all term hits for sorting
        all_term_hits = self.get_term_hits()

        for name in self.get_all_domains():
            meta = self.get_domain_meta(name)
            terms = list(self.get_domain_terms(name))

            # Sort terms by hit count (highest first)
            terms_sorted = sorted(terms, key=lambda t: all_term_hits.get(t, 0), reverse=True)

            result.append({
                'name': name,
                'description': meta.get('description', ''),
                'hits': int(float(meta.get('hits', 0) or 0)),
                'confidence': float(meta.get('confidence', 0) or 0),
                'created': int(meta.get('created', 0) or 0),
                'source': meta.get('source', 'seeded'),
                'state': meta.get('state', 'active'),
                'terms': terms_sorted,
                # GL-090: Two-tier curation fields
                'tier': meta.get('tier', 'core'),
                'total_hits': int(float(meta.get('total_hits', 0) or 0)),
                'last_hit_at': int(float(meta.get('last_hit_at', 0) or 0)),
            })
        return result

    def apply_autotune(self, result: dict) -> dict:
        """
        Apply auto-tune results from Haiku.

        Expected format:
        {
            "merge": [{"from": "@old_domain", "into": "@target_domain"}],
            "prune": ["@unused_domain"],
            "reassign": {"term": "@new_domain"}
        }

        Returns summary of actions taken.
        """
        summary = {'merged': 0, 'pruned': 0, 'reassigned': 0}

        # Process merges: move terms from old to target, then delete old
        for merge in result.get('merge', []):
            from_domain = merge.get('from')
            into_domain = merge.get('into')
            if not from_domain or not into_domain:
                continue

            # Get terms from source domain
            source_terms = self.get_domain_terms(from_domain)

            # Add terms to target domain
            if source_terms:
                target_terms_key = self._domain_key(into_domain, "terms")
                self.redis.client.sadd(target_terms_key, *source_terms)

                # Update term->domain mappings
                for term in source_terms:
                    term_key = self._key(f"term:{term}")
                    # Remove old mapping, add new
                    self.redis.client.zrem(term_key, from_domain)
                    self.redis.client.zadd(term_key, {into_domain: self.STARTER_SCORE})

            # Remove source domain
            self.remove_domain(from_domain)
            summary['merged'] += 1

        # Process prunes: delete low-value domains
        for domain_name in result.get('prune', []):
            if domain_name in self.get_all_domains():
                self.remove_domain(domain_name)
                summary['pruned'] += 1

        # Process reassignments: move terms to new domains
        for term, new_domain in result.get('reassign', {}).items():
            term_key = self._key(f"term:{term}")
            # Get current mappings
            current = self.redis.client.zrange(term_key, 0, -1)
            # Remove from all current domains
            for old_domain in current:
                self.redis.client.zrem(term_key, old_domain)
                # Also remove from domain's term set
                old_terms_key = self._domain_key(old_domain, "terms")
                self.redis.client.srem(old_terms_key, term)
            # Add to new domain
            self.redis.client.zadd(term_key, {new_domain: self.STARTER_SCORE})
            new_terms_key = self._domain_key(new_domain, "terms")
            self.redis.client.sadd(new_terms_key, term)
            summary['reassigned'] += 1

        # Update auto-tune timestamp and record results
        self.set_last_autotune()
        self.set_autotune_results(merged=summary['merged'], pruned=summary['pruned'])

        return summary

    def autotune_direct(self) -> dict:
        """
        Run auto-tune using direct Haiku API call.

        Returns summary of actions taken.
        """
        if not self.anthropic_client:
            return {'error': 'ANTHROPIC_API_KEY not set'}

        domains_with_meta = self.get_domains_with_meta()
        if not domains_with_meta:
            return {'error': 'No domains to auto-tune'}

        prompt = self.get_autotune_prompt(domains_with_meta)
        result = self._call_haiku(prompt)

        if not result:
            return {'error': 'Haiku call failed'}

        return self.apply_autotune(result)

    # =========================================================================
    # Regenerative Tune (GL-055: Intent-based tuning)
    # =========================================================================

    def get_noisy_terms(self, threshold: int = 3) -> list[str]:
        """Get terms that appear in multiple domains (noisy/ambiguous)."""
        noisy = []
        # Scan all terms
        for domain in self.get_all_domains():
            for term in self.get_domain_terms(domain):
                results = self.lookup_term(term)
                if len(results) >= threshold:
                    noisy.append(term)
        return list(set(noisy))

    def get_recent_files_from_intents(self, limit: int = 100) -> list[str]:
        """Get recent unique files from intent history."""
        # This will be populated by the caller from /intent/recent API
        # For now, return empty - hook will provide this data
        return []

    def get_tune_prompt(self, recent_files: list[str], domains_with_meta: list[dict],
                        noisy_terms: list[str]) -> str:
        """
        Generate regenerative tune prompt for Haiku.

        Unlike autotune (patch-based), this asks for OPTIMAL structure.
        """
        # Format domains
        high_hit = [d for d in domains_with_meta if d.get('hits', 0) >= self.PRESERVE_THRESHOLD]
        low_hit = [d for d in domains_with_meta if d.get('hits', 0) < self.PRESERVE_THRESHOLD]

        high_hit_text = "\n".join(
            f"  - {d['name']}: {d.get('hits', 0)} hits, terms: {d.get('terms', [])[:5]}"
            for d in sorted(high_hit, key=lambda x: x.get('hits', 0), reverse=True)
        ) or "  (none)"

        low_hit_text = "\n".join(
            f"  - {d['name']}: {d.get('hits', 0)} hits, terms: {d.get('terms', [])[:5]}"
            for d in low_hit
        ) or "  (none)"

        # Format files (group by directory)
        file_dirs = {}
        for f in recent_files[:100]:
            parts = f.rsplit('/', 2)
            if len(parts) >= 2:
                dir_name = parts[-2] if len(parts) > 1 else '.'
                file_dirs.setdefault(dir_name, []).append(parts[-1])

        files_text = "\n".join(
            f"  - {dir_name}/: {', '.join(files[:5])}{'...' if len(files) > 5 else ''}"
            for dir_name, files in sorted(file_dirs.items(), key=lambda x: -len(x[1]))[:10]
        ) or "  (no recent files)"

        noisy_text = ", ".join(noisy_terms[:15]) or "(none)"

        return f"""You are optimizing a semantic domain system based on actual usage.

CURRENT STATE ({len(domains_with_meta)} domains):

High-value domains (hits >= {self.PRESERVE_THRESHOLD}, PRESERVE these):
{high_hit_text}

Low-value domains (candidates for merge/removal):
{low_hit_text}

Noisy terms (in 3+ domains, need assignment): {noisy_text}

RECENT USAGE (last {len(recent_files)} files):
{files_text}

YOUR TASK:
Design the OPTIMAL domain structure. Consider:
1. Keep high-value domains that are working
2. Merge or remove low-value domains
3. Assign noisy terms to their best single domain
4. Create specific terms (prefer "session_timeout" over generic "session")
5. Each domain should have 5-10 focused terms

Return JSON:
{{
  "domains": [
    {{
      "name": "@domain_name",
      "description": "what this domain represents",
      "terms": ["specific_term1", "specific_term2", "..."],
      "action": "keep|merge_into|new"
    }}
  ],
  "remove": ["@domain_to_delete"],
  "reasoning": "brief explanation of changes"
}}"""

    # =========================================================================
    # GL-059.3: Math-Based Noise Elimination
    # =========================================================================

    def run_math_tune(self) -> dict:
        """
        GL-059.3: Pure math-based tuning - no Haiku needed.

        Algorithm:
        1. Calculate term coverage: term appears in what % of indexed files
        2. Prune noisy terms (>30% coverage - too generic to be useful)
        3. Update domain lifecycle based on hits_last_cycle
        4. Flag domains with <2 remaining terms

        Returns:
            Summary of changes made
        """
        summary = {
            'terms_pruned': 0,
            'domains_flagged_stale': 0,
            'domains_deprecated': 0,
            'domains_active': 0,
        }

        domains = self.get_all_domains()
        if not domains:
            return summary

        # Get total file count from Redis (set by indexer)
        total_files = int(self.redis.client.get("aoa:total_indexed_files") or 1000)
        coverage_threshold = 0.30  # 30% = too generic

        # Track terms to prune globally
        terms_to_prune = set()

        for domain_name in domains:
            # Get domain terms and hits
            terms = self.get_domain_terms(domain_name)
            meta = self.get_domain_meta(domain_name)
            hits_last = int(float(meta.get('hits_last_cycle', 0)))
            state = meta.get('state', 'active')
            stale_cycles = int(meta.get('stale_cycles', 0))

            # 1. Check term coverage - prune noisy terms
            for term in list(terms):
                # Count files containing this term (from intent records)
                term_hits_key = self._key("term_hits")
                term_hit_count = int(self.redis.client.hget(term_hits_key, term) or 0)

                # If term appears in >30% of activity, it's too generic
                if total_files > 0 and term_hit_count / total_files > coverage_threshold:
                    terms_to_prune.add(term)
                    # Remove term from this domain
                    terms_key = self._domain_key(domain_name, "terms")
                    self.redis.client.srem(terms_key, term)
                    # Remove domain from term index
                    term_key = self._key(f"term:{term}")
                    self.redis.client.zrem(term_key, domain_name)
                    summary['terms_pruned'] += 1

            # 2. Update lifecycle based on hits_last_cycle
            remaining_terms = len(self.get_domain_terms(domain_name))

            if hits_last == 0:
                # No hits last cycle - move toward stale
                if state == 'active':
                    self.set_domain_state(domain_name, 'stale')
                    self.increment_stale_cycles(domain_name)
                    summary['domains_flagged_stale'] += 1
                elif state == 'stale':
                    new_cycles = self.increment_stale_cycles(domain_name)
                    if new_cycles >= 2:
                        # 2 stale cycles = deprecated
                        self.set_domain_state(domain_name, 'deprecated')
                        summary['domains_deprecated'] += 1
            else:
                # Had hits - reset to active
                if state != 'active':
                    self.set_domain_state(domain_name, 'active')
                    self.reset_stale_cycles(domain_name)
                summary['domains_active'] += 1

            # 3. Flag domains with too few terms
            if remaining_terms < 2 and state != 'deprecated':
                self.set_domain_state(domain_name, 'deprecated')
                summary['domains_deprecated'] += 1

        # 4. GL-059.4: Graduated Transition - retire seeded when learned is sufficient
        source_counts = self.get_source_counts()
        learned_count = source_counts.get('learned', 0)
        transition_threshold = 32  # Start retiring when 32+ learned domains

        summary['domains_removed'] = 0
        if learned_count >= transition_threshold:
            # Get all deprecated seeded domains and remove them
            for domain_name in list(domains):
                meta = self.get_domain_meta(domain_name)
                if meta.get('source') == 'seeded' and meta.get('state') == 'deprecated':
                    self.remove_domain(domain_name)
                    # Decrement seeded count
                    self.redis.client.decr(self._key("seeded_count"))
                    summary['domains_removed'] += 1

        # 5. Snapshot hits for next cycle and reset
        self.snapshot_cycle_hits()

        # =====================================================================
        # GL-090: Two-Tier Curation
        # =====================================================================

        summary['decayed'] = 0
        summary['promoted'] = 0
        summary['demoted'] = 0
        summary['context_pruned'] = 0

        # Use prompt_count for staleness calculation (same as last_hit_at tracking)
        prompt_count = self.get_prompt_count()

        # 6. Apply decay to all domain hits
        for domain_name in self.get_all_domains():
            meta_key = self._domain_key(domain_name, "meta")
            hits = float(self.redis.client.hget(meta_key, "hits") or 0)
            new_hits = hits * self.DECAY_RATE
            self.redis.client.hset(meta_key, "hits", new_hits)
            summary['decayed'] += 1

        # 7. Check promotions: context → core (high value)
        # GL-091: Use configurable threshold
        promotion_threshold = self.get_threshold('promotion')
        for domain_name in self.get_domains_by_tier("context"):
            meta = self.get_domain_meta(domain_name)
            total_hits = int(meta.get('total_hits', 0))
            if total_hits >= promotion_threshold:
                # Check if core tier has room
                counts = self.count_domains_by_tier()
                if counts.get("core", 0) < self.CORE_DOMAINS_MAX:
                    self.promote_domain(domain_name)
                    summary['promoted'] += 1

        # 8. Check demotions: core → context (stale)
        # GL-091: Use configurable threshold
        demotion_threshold = self.get_threshold('demotion')
        for domain_name in self.get_domains_by_tier("core"):
            meta = self.get_domain_meta(domain_name)
            last_hit_at = int(meta.get('last_hit_at', 0))
            intents_since_hit = prompt_count - last_hit_at

            if intents_since_hit >= demotion_threshold:
                self.demote_domain(domain_name)
                summary['demoted'] += 1

        # 9. Prune low-value context domains
        # GL-091: Use configurable threshold
        prune_floor = self.get_threshold('prune_floor')
        for domain_name in self.get_domains_by_tier("context"):
            meta = self.get_domain_meta(domain_name)
            hits = float(meta.get('hits', 0))
            if hits < prune_floor:
                self.remove_domain(domain_name)
                summary['context_pruned'] += 1

        # 10. Update tune tracking
        self.set_last_tune()
        self.reset_tune_count()
        self.clear_tuning_pending()
        self.set_tune_results(
            kept=summary['domains_active'],
            added=0,
            removed=summary['terms_pruned'] + summary['context_pruned']
        )

        return summary

    def apply_tune(self, result: dict) -> dict:
        """
        Apply regenerative tune results from Haiku (legacy).

        Note: GL-059.3 moves tuning to pure math via run_math_tune().
        This method kept for backward compatibility.

        Unlike apply_autotune (patches), this rebuilds the domain structure
        while preserving high-value domains.
        """
        summary = {'kept': 0, 'added': 0, 'removed': 0, 'terms_updated': 0}

        if not result or 'domains' not in result:
            return {'error': 'Invalid tune result'}

        current_domains = set(self.get_all_domains())
        new_domains = {d['name'] for d in result.get('domains', [])}

        # 1. Process domains from Haiku response
        for domain_data in result.get('domains', []):
            name = domain_data.get('name', '').strip()
            if not name:
                continue

            action = domain_data.get('action', 'keep')
            terms = domain_data.get('terms', [])
            description = domain_data.get('description', '')

            if name in current_domains:
                # Existing domain - update terms if provided
                if terms and action != 'remove':
                    # Update term mappings
                    for term in terms:
                        term_key = self._key(f"term:{term}")
                        # Remove from other domains, add to this one
                        self.redis.client.zadd(term_key, {name: self.STARTER_SCORE})
                        terms_key = self._domain_key(name, "terms")
                        self.redis.client.sadd(terms_key, term)
                        summary['terms_updated'] += 1
                    summary['kept'] += 1
            else:
                # New domain from tune
                domain = Domain(
                    name=name,
                    description=description,
                    confidence=0.7,
                    terms=terms
                )
                self.add_domain(domain)
                summary['added'] += 1

        # 2. Remove domains marked for removal
        for domain_name in result.get('remove', []):
            if domain_name in current_domains:
                # Check if high-value (extra protection)
                meta = self.get_domain_meta(domain_name)
                hits = int(float(meta.get('hits', 0) or 0))
                if hits < self.PRESERVE_THRESHOLD:
                    self.remove_domain(domain_name)
                    summary['removed'] += 1

        # 3. Update tune tracking
        self.set_last_tune()
        self.reset_tune_count()
        self.clear_tuning_pending()
        self.set_tune_results(
            kept=summary['kept'],
            added=summary['added'],
            removed=summary['removed']
        )

        return summary

    def set_tune_results(self, kept: int = 0, added: int = 0, removed: int = 0) -> None:
        """Store results of last tune for display."""
        self.redis.client.hset(self._key("tune_results"), mapping={
            "kept": kept,
            "added": added,
            "removed": removed,
            "timestamp": int(time.time())
        })

    def get_tune_results(self) -> dict:
        """Get results of last tune."""
        results = self.redis.client.hgetall(self._key("tune_results"))
        return {
            'kept': int(results.get('kept', 0)),
            'added': int(results.get('added', 0)),
            'removed': int(results.get('removed', 0)),
            'timestamp': int(results.get('timestamp', 0))
        }

    # =========================================================================
    # Stats
    # =========================================================================

    # =========================================================================
    # Project Analysis (GL-083)
    # =========================================================================

    def get_cluster_analysis_prompt(self, directory: str, files: list[str]) -> str:
        """
        Generate Haiku prompt for analyzing a directory cluster.

        GL-083: One-time project analysis replaces per-prompt learning.
        GL-084: Returns v2 format with 5-10 terms per domain, 7-10 keywords per term.
        """
        file_list = "\n".join(f"  - {f}" for f in files[:50])

        return f"""Generate 24 core semantic domains from this codebase structure.

Directory: {directory}
Files:
{file_list}

STRUCTURE:
- Domain (@name): A high-level capability area
- Terms: 5-10 intent labels per domain (underscores OK: #token_lifecycle)
- Keywords: 7-10 single-word identifiers per term (NO underscores - these are matched)

PURPOSE:
Terms are semantic handles for AI agents. When grep returns scattered results,
terms provide instant context. A term answers: "What is the developer trying
to accomplish?"

GOOD TERMS: #token_lifecycle, #stock_sync, #cart_checkout, #webhook_ingestion
BAD TERMS: #process_payment (too generic), #handle_user_auth (just a function name)

GOOD KEYWORDS: token, jwt, refresh, expire, validate, bearer, decode
BAD KEYWORDS: token_validation, user_auth (underscores), t (too short)

REQUIREMENTS:
- Generate 24 core domains covering the full codebase
- Each domain MUST have 5-10 terms
- Each term MUST have 7-10 keywords
- Keywords: single words only, 3+ characters, no underscores
- Terms: intent-driven, not function-name derivatives
- Keywords: actual identifiers, filenames, or tokens from the codebase

Respond with ONLY valid JSON (no markdown, no explanation):
{{
  "domains": [
    {{
      "name": "@domain_name",
      "description": "Capability this provides",
      "terms": {{
        "intent_label": ["keyword", "another", "single", "words", "only", "here", "matched"]
      }}
    }}
  ]
}}"""

    def analyze_cluster(self, directory: str, files: list[str]) -> list[dict]:
        """
        Analyze a single directory cluster via Haiku.

        Returns list of domain dicts or empty list on failure.
        """
        if not files:
            return []

        prompt = self.get_cluster_analysis_prompt(directory, files)
        result = self._call_haiku(prompt)

        if result and "domains" in result:
            return result["domains"]
        return []

    def analyze_project(self, project_root: str, file_list: list[str] = None) -> dict:
        """
        Analyze entire project via parallel Haiku calls.

        GL-083: One-time semantic analysis to generate project-domains.json.

        Args:
            project_root: Path to project
            file_list: Optional pre-computed file list

        Returns:
            {
                "success": bool,
                "domains": [...],
                "domains_count": int,
                "terms_count": int,
                "clusters_analyzed": int
            }
        """
        import concurrent.futures
        from pathlib import Path
        from collections import defaultdict

        # Group files by top-level directory
        clusters = defaultdict(list)

        if file_list:
            files = file_list
        else:
            # Get files from index
            files = self.get_recent_files_from_intents(limit=500)

        # Group by two-level directory for better granularity
        # e.g., "services/gateway" instead of just "services"
        for f in files:
            # Get relative path
            if f.startswith(project_root):
                rel = f[len(project_root):].lstrip("/")
            else:
                rel = f

            # Get directory grouping (two levels if available)
            parts = rel.split("/")
            if len(parts) > 2:
                # Use two-level: "services/gateway"
                dir_key = f"{parts[0]}/{parts[1]}"
            elif len(parts) > 1:
                # Single level: "cli"
                dir_key = parts[0]
            else:
                dir_key = "_root"

            clusters[dir_key].append(rel)

        # Filter to clusters with enough files
        valid_clusters = {k: v for k, v in clusters.items()
                        if len(v) >= 2 and not k.startswith(".")}

        if not valid_clusters:
            return {
                "success": False,
                "error": "No valid file clusters found",
                "domains": [],
                "domains_count": 0,
                "terms_count": 0
            }

        # Parallel Haiku analysis (max 5 concurrent)
        all_domains = []
        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            futures = {
                executor.submit(self.analyze_cluster, dir_name, files): dir_name
                for dir_name, files in list(valid_clusters.items())[:15]
            }

            for future in concurrent.futures.as_completed(futures):
                dir_name = futures[future]
                try:
                    domains = future.result()
                    all_domains.extend(domains)
                except Exception as e:
                    print(f"Cluster {dir_name} failed: {e}")

        # Dedupe and merge domains
        merged = self._merge_domains(all_domains)

        # Count terms (term clusters) and keywords
        total_terms = 0
        total_keywords = 0
        for d in merged:
            terms = d.get("terms", {})
            if isinstance(terms, dict):
                total_terms += len(terms)  # Number of term clusters
                total_keywords += sum(len(kws) for kws in terms.values())
            else:
                # Legacy flat array
                total_keywords += len(terms)

        return {
            "success": True,
            "domains": merged,
            "domains_count": len(merged),
            "terms_count": total_terms,
            "keywords_count": total_keywords,
            "clusters_analyzed": len(valid_clusters)
        }

    def _merge_domains(self, domains: list[dict]) -> list[dict]:
        """
        Merge and dedupe domains from multiple clusters.

        Combines domains with similar names, merges their term hierarchies.
        Supports both v2 format (terms as dict) and legacy (terms as list).
        """
        merged = {}

        for d in domains:
            name = d.get("name", "").lower().strip()
            if not name.startswith("@"):
                name = f"@{name}"

            terms_data = d.get("terms", {})

            # Handle legacy flat array format - convert to dict
            if isinstance(terms_data, list):
                terms_data = {"keywords": terms_data}

            if name in merged:
                # Merge term clusters
                existing_terms = merged[name].get("terms", {})
                for cluster_name, keywords in terms_data.items():
                    if cluster_name in existing_terms:
                        # Union keywords, limit to 20 per cluster
                        combined = list(set(existing_terms[cluster_name]) | set(keywords))
                        existing_terms[cluster_name] = combined[:20]
                    else:
                        existing_terms[cluster_name] = keywords[:20]
                merged[name]["terms"] = existing_terms
            else:
                # Limit clusters to 10 per domain, keywords to 20 per cluster
                limited_terms = {}
                for cluster_name, keywords in list(terms_data.items())[:10]:
                    limited_terms[cluster_name] = keywords[:20] if isinstance(keywords, list) else []

                merged[name] = {
                    "name": name,
                    "description": d.get("description", ""),
                    "terms": limited_terms
                }

        return list(merged.values())

    def save_project_domains(self, domains: list[dict], output_path: str) -> bool:
        """Save analyzed domains to project-domains.json."""
        try:
            import json
            with open(output_path, "w") as f:
                json.dump(domains, f, indent=2)
            return True
        except Exception as e:
            print(f"Failed to save domains: {e}")
            return False

    def get_stats(self) -> dict:
        """Get domain learning statistics."""
        domains = self.get_all_domains()
        total_terms = 0
        total_hits = 0

        for d in domains:
            terms = self.get_domain_terms(d)
            total_terms += len(terms)
            meta = self.get_domain_meta(d)
            total_hits += int(float(meta.get("hits", 0) or 0))

        tune_results = self.get_tune_results()
        learned_details = self.get_learned_details()

        # GL-059.1: Get source counts
        source_counts = self.get_source_counts()

        return {
            "project": self.project_id,
            "domains": len(domains),
            "total_terms": total_terms,
            "total_hits": total_hits,
            # GL-059.1: Domain sources
            "seeded_count": source_counts["seeded"],
            "learned_count": source_counts["learned"],
            # Learning (every 10 prompts)
            "prompt_count": self.get_prompt_count(),
            "prompt_threshold": self.BATCH_SIZE,
            "should_learn": self.should_learn(),
            "learning_pending": self.is_learning_pending(),
            "last_learn": self.get_last_learn(),
            "terms_learned_last": self.get_terms_learned_last(),
            "terms_learned_list": learned_details['terms'],
            "domains_learned_list": learned_details['domains'],
            # Tuning (every 100 intents)
            "tune_count": self.get_tune_count(),
            "tune_threshold": self.TUNE_THRESHOLD,
            "should_tune": self.should_tune(),
            "tuning_pending": self.is_tuning_pending(),
            "last_tune": self.get_last_tune(),
            "tune_kept_last": tune_results['kept'],
            "tune_added_last": tune_results['added'],
            "tune_removed_last": tune_results['removed'],
            # GL-054: Intelligence Angle
            "tokens_invested": self.get_tokens_invested(),
            "tokens_invested_last": self.get_tokens_invested_last(),
            "learning_calls": self.get_learning_calls(),
            # GL-069.1: Orphan tags
            "orphan_count": self.get_orphan_count(),
        }


if __name__ == "__main__":
    # Quick test
    learner = DomainLearner("test-project")

    print("Testing Redis connection...")
    if learner.redis.ping():
        print("✓ Redis connected")

        # Test domain operations
        test_domain = Domain(
            name="@test_domain",
            description="A test domain",
            confidence=0.8,
            terms=["test", "sample", "example"]
        )

        learner.add_domain(test_domain)
        print(f"✓ Added domain: {test_domain.name}")

        domains = learner.get_all_domains()
        print(f"✓ All domains: {domains}")

        result = learner.lookup_term("test")
        print(f"✓ Term lookup 'test': {result}")

        # Cleanup
        learner.remove_domain("@test_domain")
        print("✓ Cleaned up test domain")

        print("\nStats:", learner.get_stats())
    else:
        print("✗ Redis not available")
