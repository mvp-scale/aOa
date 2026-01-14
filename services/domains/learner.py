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
    STARTER_HITS = 10  # Initial hits for new domains
    STARTER_SCORE = 0.5  # Initial term confidence
    PRESERVE_THRESHOLD = 20  # Keep domains with hits >= this during tune

    def __init__(self, project_id: str, redis_url: Optional[str] = None):
        """Initialize with project identifier."""
        self.project_id = project_id
        self.redis = RedisClient(url=redis_url)
        self._anthropic = None

    @property
    def anthropic_client(self):
        """Lazy-initialize Anthropic client for direct mode."""
        if self._anthropic is None and ANTHROPIC_AVAILABLE:
            api_key = os.environ.get('ANTHROPIC_API_KEY')
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
        self.redis.client.set(self._key("learning_pending"), "1")

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
        self.redis.client.set(self._key("tuning_pending"), "1")

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
            self.redis.client.delete(self._key("terms_learned_list"))
            self.redis.client.rpush(self._key("terms_learned_list"), *terms_list[:20])  # Cap at 20
        if domains_list:
            self.redis.client.delete(self._key("domains_learned_list"))
            self.redis.client.rpush(self._key("domains_learned_list"), *domains_list[:5])  # Cap at 5

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

    # =========================================================================
    # Domain Storage
    # =========================================================================

    def get_all_domains(self) -> list[str]:
        """Get all domain names for this project."""
        key = self._key("domains")
        return list(self.redis.client.smembers(key))

    def add_domain(self, domain: Domain, source: str = "learned") -> None:
        """Add a new domain with metadata and terms.

        Args:
            domain: Domain object with name, description, confidence, terms
            source: "seeded" for quickstart domains, "learned" for Haiku-discovered
        """
        # Add to domain set
        domains_key = self._key("domains")
        self.redis.client.sadd(domains_key, domain.name)

        # Store metadata with lifecycle fields (GL-059.1)
        meta_key = self._domain_key(domain.name, "meta")
        self.redis.client.hset(meta_key, mapping={
            "description": domain.description,
            "confidence": domain.confidence,
            "hits": self.STARTER_HITS,
            "created": int(time.time()),
            # GL-059.1: Lifecycle fields
            "source": source,  # "seeded" | "learned"
            "state": "active",  # "active" | "stale" | "deprecated"
            "stale_cycles": 0,
            "hits_last_cycle": 0,
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

    def increment_domain_hits(self, name: str) -> int:
        """Increment hit counter for a domain."""
        meta_key = self._domain_key(name, "meta")
        return self.redis.client.hincrby(meta_key, "hits", 1)

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
        return int(self.redis.client.hget(meta_key, "hits_last_cycle") or 0)

    def snapshot_cycle_hits(self) -> None:
        """Snapshot current hits to hits_last_cycle for all domains, then reset."""
        domains = self.get_all_domains()
        pipe = self.redis.client.pipeline()
        for name in domains:
            meta_key = self._domain_key(name, "meta")
            # Get current hits
            hits = self.redis.client.hget(meta_key, "hits") or 0
            # Store as last cycle hits
            pipe.hset(meta_key, "hits_last_cycle", hits)
        pipe.execute()

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

    def increment_term_hits(self, terms: list[str]) -> None:
        """Increment hit counters for terms that matched."""
        if not terms:
            return
        term_hits_key = self._key("term_hits")
        pipe = self.redis.client.pipeline()
        for term in terms:
            pipe.hincrby(term_hits_key, term, 1)
        pipe.execute()

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

    def remove_domain(self, name: str) -> None:
        """Remove a domain and all its data."""
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

        # Remove domain from term mappings
        for term in terms:
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
                'hits': int(meta.get('hits', 0)),
                'confidence': float(meta.get('confidence', 0)),
                'created': int(meta.get('created', 0)),
                'terms': terms_sorted,
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
            hits_last = int(meta.get('hits_last_cycle', 0))
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

        # 5. Update tune tracking
        self.set_last_tune()
        self.reset_tune_count()
        self.clear_tuning_pending()
        self.set_tune_results(
            kept=summary['domains_active'],
            added=0,
            removed=summary['terms_pruned']
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
                hits = int(meta.get('hits', 0))
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

    def get_stats(self) -> dict:
        """Get domain learning statistics."""
        domains = self.get_all_domains()
        total_terms = 0
        total_hits = 0

        for d in domains:
            terms = self.get_domain_terms(d)
            total_terms += len(terms)
            meta = self.get_domain_meta(d)
            total_hits += int(meta.get("hits", 0))

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
