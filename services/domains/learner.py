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
    REBALANCE_INTERVAL = 43200  # 12 hours in seconds
    STARTER_HITS = 10  # Initial hits for new domains
    STARTER_SCORE = 0.5  # Initial term confidence

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
    # Rebalance Timer
    # =========================================================================

    def get_last_rebalance(self) -> int:
        """Get timestamp of last rebalance."""
        key = self._key("last_rebalance")
        val = self.redis.client.get(key)
        return int(val) if val else 0

    def set_last_rebalance(self) -> None:
        """Update rebalance timestamp to now."""
        key = self._key("last_rebalance")
        self.redis.client.set(key, int(time.time()))

    def should_rebalance(self) -> bool:
        """Check if 12+ hours since last rebalance."""
        last = self.get_last_rebalance()
        if last == 0:
            return False  # Never rebalanced = nothing to rebalance
        return (time.time() - last) >= self.REBALANCE_INTERVAL

    # =========================================================================
    # Domain Storage
    # =========================================================================

    def get_all_domains(self) -> list[str]:
        """Get all domain names for this project."""
        key = self._key("domains")
        return list(self.redis.client.smembers(key))

    def add_domain(self, domain: Domain) -> None:
        """Add a new domain with metadata and terms."""
        # Add to domain set
        domains_key = self._key("domains")
        self.redis.client.sadd(domains_key, domain.name)

        # Store metadata
        meta_key = self._domain_key(domain.name, "meta")
        self.redis.client.hset(meta_key, mapping={
            "description": domain.description,
            "confidence": domain.confidence,
            "hits": self.STARTER_HITS,
            "created": int(time.time()),
        })

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

    def get_domain_for_symbol(self, symbol: str) -> Optional[str]:
        """
        Get the best matching domain for a symbol name.

        Tokenizes the symbol and aggregates domain scores.
        """
        # Simple tokenization: split on non-alphanumeric
        import re
        tokens = re.split(r'[^a-zA-Z0-9]+', symbol.lower())
        tokens = [t for t in tokens if len(t) > 2]

        if not tokens:
            return None

        # Aggregate scores across all tokens
        scores: dict[str, float] = {}
        for token in tokens:
            for domain, score in self.lookup_term(token):
                scores[domain] = scores.get(domain, 0) + score

        if not scores:
            return None

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

    def get_rebalance_prompt(self, domains_with_meta: list[dict]) -> str:
        """
        Generate the rebalance prompt for Haiku.
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
        """
        if not self.anthropic_client:
            return None

        try:
            response = self.anthropic_client.messages.create(
                model="claude-3-5-haiku-latest",
                max_tokens=1024,
                messages=[{"role": "user", "content": prompt}]
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

        return {
            "project": self.project_id,
            "domains": len(domains),
            "total_terms": total_terms,
            "total_hits": total_hits,
            "prompt_count": self.get_prompt_count(),
            "should_learn": self.should_learn(),
            "should_rebalance": self.should_rebalance(),
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
