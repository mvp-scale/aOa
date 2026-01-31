#!/usr/bin/env python3
"""
Pattern Matchers - Aho-Corasick based pattern matching utilities.

Extracted from indexer.py for maintainability (CH-01).

Classes:
  - AhoCorasickMatcher: Multi-pattern matching for semantic/domain patterns
  - KeywordMatcher: Redis-backed keyword matching with timestamp eligibility

Usage:
    from matchers import AC_MATCHER, get_keyword_matcher, set_intent_index

    # For AhoCorasick pattern matching (file-based patterns)
    tags = AC_MATCHER.get_dense_tags(code_content)

    # For Redis-backed keyword matching (requires intent_index)
    set_intent_index(intent_index)  # Call once after intent_index created
    matcher = get_keyword_matcher(project_id)
    result = matcher.find_tags(content)
"""

import json
import os
import threading
from collections import defaultdict
from pathlib import Path

# Aho-Corasick for O(n) multi-pattern matching
try:
    import ahocorasick
    AHOCORASICK_AVAILABLE = True
except ImportError:
    AHOCORASICK_AVAILABLE = False


# ============================================================================
# Pattern Loading
# ============================================================================

def _load_pattern_library():
    """Load semantic and domain pattern configs from project-domains.json.

    GL-084: Universal domains removed. Only project-specific domains are used.
    Generated via /aoa-setup skill.

    v2 Structure: @domain -> semantic_term -> matches[]

    Returns:
    - semantic_patterns: semantic_term -> {patterns: set, tag: @domain, priority: int}
    - domain_keywords: match -> @domain
    """
    semantic_patterns = {}
    domain_keywords = {}

    # GL-084: Only project-domains.json - no universal fallback
    project_paths = [
        Path(os.environ.get('CODEBASE_ROOT', '.')) / '.aoa' / 'project-domains.json',
        Path('/app/.aoa/project-domains.json'),  # Docker mounted
    ]

    domains_file = None
    for path in project_paths:
        if path.exists():
            domains_file = path
            break

    if not domains_file:
        return semantic_patterns, domain_keywords

    try:
        data = json.loads(domains_file.read_text())
        domains = data if isinstance(data, list) else data.get('domains', [])

        for domain in domains:
            domain_name = domain.get('name', '')
            terms = domain.get('terms', {})

            # v2 format: terms is dict of semantic_term -> matches[]
            if isinstance(terms, dict):
                for semantic_term, matches in terms.items():
                    patterns = {m.lower() for m in matches}
                    semantic_patterns[semantic_term] = {
                        'patterns': patterns,
                        'tag': domain_name,
                        'priority': 3
                    }

                    for match in matches:
                        domain_keywords[match.lower()] = domain_name

            # Flat format fallback
            elif isinstance(terms, list):
                for match in terms:
                    domain_keywords[match.lower()] = domain_name

    except Exception:
        pass

    return semantic_patterns, domain_keywords


# Load patterns at startup (RAM-cached)
SEMANTIC_PATTERNS, DOMAIN_KEYWORDS = _load_pattern_library()


# ============================================================================
# AhoCorasickMatcher - File-based pattern matching
# ============================================================================

class AhoCorasickMatcher:
    """
    Unified pattern matcher using Aho-Corasick algorithm.

    GL-047: Provides O(n) multi-pattern matching where n = text length.
    Combines semantic patterns (action verbs) and domain patterns (tech keywords)
    into a single automaton for efficient density analysis.

    Usage:
        matcher = AhoCorasickMatcher()
        hits = matcher.find_all("def get_cached_user_token()")
        # Returns: [('get', '#read', 'semantic'), ('cache', '#cache', 'domain'), ...]

        density = matcher.density_by_domain(lines)
        # Returns: {'caching': 3, 'auth': 1, ...}
    """

    def __init__(self):
        self.automaton = None
        self.pattern_meta = {}
        self._build_automaton()

    def _build_automaton(self):
        """Build unified automaton from all pattern sources."""
        if not AHOCORASICK_AVAILABLE:
            return

        self.automaton = ahocorasick.Automaton()

        # Add semantic patterns
        for cat_name, cat_data in SEMANTIC_PATTERNS.items():
            tag = cat_data.get('tag', f'#{cat_name}')
            priority = cat_data.get('priority', 3)
            for pattern in cat_data.get('patterns', set()):
                pattern_lower = pattern.lower()
                self.pattern_meta[pattern_lower] = {
                    'tag': tag,
                    'source': 'semantic',
                    'category': cat_name,
                    'priority': priority
                }
                self.automaton.add_word(pattern_lower, pattern_lower)

        # Add domain patterns
        for keyword, tag in DOMAIN_KEYWORDS.items():
            keyword_lower = keyword.lower()
            if keyword_lower not in self.pattern_meta:
                self.pattern_meta[keyword_lower] = {
                    'tag': tag,
                    'source': 'domain',
                    'category': tag.lstrip('#'),
                    'priority': 3
                }
                self.automaton.add_word(keyword_lower, keyword_lower)

        if self.pattern_meta:
            self.automaton.make_automaton()

    def find_all(self, text: str) -> list:
        """
        Find all pattern matches in text.

        Returns list of (pattern, tag, source) tuples.
        O(n) where n = len(text), regardless of pattern count.
        """
        if not self.automaton or not text:
            return []

        text_lower = text.lower()
        results = []

        for end_idx, pattern in self.automaton.iter(text_lower):
            meta = self.pattern_meta.get(pattern, {})
            results.append((
                pattern,
                meta.get('tag', f'#{pattern}'),
                meta.get('source', 'unknown')
            ))

        return results

    def find_all_with_positions(self, text: str) -> list:
        """
        Find all pattern matches with their positions.

        Returns list of (start_idx, end_idx, pattern, tag, source) tuples.
        """
        if not self.automaton or not text:
            return []

        text_lower = text.lower()
        results = []

        for end_idx, pattern in self.automaton.iter(text_lower):
            start_idx = end_idx - len(pattern) + 1
            meta = self.pattern_meta.get(pattern, {})
            results.append((
                start_idx,
                end_idx + 1,
                pattern,
                meta.get('tag', f'#{pattern}'),
                meta.get('source', 'unknown')
            ))

        return results

    def density_by_category(self, text: str) -> dict:
        """
        Count pattern hits by category.

        Used for density analysis: high counts = this code IS ABOUT that concept.
        Returns dict of {category: count}.
        """
        if not self.automaton or not text:
            return {}

        text_lower = text.lower()
        counts = defaultdict(int)

        for _, pattern in self.automaton.iter(text_lower):
            meta = self.pattern_meta.get(pattern, {})
            category = meta.get('category', 'unknown')
            counts[category] += 1

        return dict(counts)

    def get_dense_tags(self, text: str, threshold: int = 2) -> list:
        """
        Get tags that appear frequently (density >= threshold).

        These represent what the code IS ABOUT, not just what it mentions.
        Returns list of #term tags sorted by frequency (highest first).

        GL-084: Returns #term_name, NOT @domain. Domains are handled separately.
        """
        density = self.density_by_category(text)
        dense = [(cat, count) for cat, count in density.items() if count >= threshold]
        dense.sort(key=lambda x: -x[1])

        tags = []
        for cat, _ in dense:
            tags.append(f'#{cat}')

        return tags

    @property
    def pattern_count(self) -> int:
        """Number of patterns in the automaton."""
        return len(self.pattern_meta)

    @property
    def is_available(self) -> bool:
        """Check if AC matching is available."""
        return self.automaton is not None and AHOCORASICK_AVAILABLE


# Global AC matcher (built once at startup)
AC_MATCHER = AhoCorasickMatcher()


# ============================================================================
# KeywordMatcher - Redis-backed keyword matching
# ============================================================================

class KeywordMatcher:
    """Build AC automaton from Redis keywords with timestamp-based eligibility.

    This replaces the legacy SEMANTIC_PATTERNS file-based approach.
    Keywords come from Redis term:* keys, providing real-time updates.

    Eligibility filtering ensures:
    - Cold start domains (created_at=0) apply everywhere
    - New domains only tag files accessed AFTER the domain was created
    """

    def __init__(self, project_id: str, redis_client):
        self.project_id = project_id
        self.redis = redis_client
        self.automaton = None
        self.keyword_to_term = {}
        self.domain_created_at = {}
        self._initialized = False
        self.rebuild_lock = threading.RLock()

    def rebuild(self):
        """Rebuild AC automaton from Redis term:* keys."""
        if not AHOCORASICK_AVAILABLE:
            return

        with self.rebuild_lock:
            self.automaton = ahocorasick.Automaton()
            self.keyword_to_term = {}
            self.domain_created_at = {}

            try:
                r = self.redis.client if hasattr(self.redis, 'client') else self.redis

                # Load domain timestamps
                domain_pattern = f"aoa:{self.project_id}:domain:*:meta"
                for key in r.scan_iter(domain_pattern):
                    key_str = key.decode() if isinstance(key, bytes) else key
                    parts = key_str.split(':')
                    if len(parts) >= 4:
                        domain_name = parts[3]
                        created = r.hget(key_str, 'created')
                        if created:
                            created_val = created.decode() if isinstance(created, bytes) else created
                            self.domain_created_at[domain_name] = int(created_val)
                        else:
                            self.domain_created_at[domain_name] = 0

                # Scan for term:*:domain mappings
                term_domain_pattern = f"aoa:{self.project_id}:term:*:domain"
                for key in r.scan_iter(term_domain_pattern):
                    key_str = key.decode() if isinstance(key, bytes) else key
                    parts = key_str.split(':')
                    if len(parts) >= 5:
                        term_name = parts[3]
                        domain = r.get(key_str)
                        if domain:
                            domain_str = domain.decode() if isinstance(domain, bytes) else domain
                            self.keyword_to_term[term_name.lower()] = (term_name, domain_str)
                            self.automaton.add_word(term_name.lower(), term_name.lower())

                # Add keywords from term:*:keywords SETs
                for key in r.scan_iter(f"aoa:{self.project_id}:term:*:keywords"):
                    key_str = key.decode() if isinstance(key, bytes) else key
                    parts = key_str.split(':')
                    if len(parts) >= 5:
                        term_name = parts[3]
                        domain_key = f"aoa:{self.project_id}:term:{term_name}:domain"
                        domain = r.get(domain_key)
                        if domain:
                            domain_str = domain.decode() if isinstance(domain, bytes) else domain
                            cursor = 0
                            while True:
                                cursor, keywords = r.sscan(key_str, cursor=cursor, count=100)
                                for kw in keywords:
                                    kw_str = (kw.decode() if isinstance(kw, bytes) else kw).lower()
                                    if len(kw_str) >= 3 and kw_str not in self.keyword_to_term:
                                        self.keyword_to_term[kw_str] = (term_name, domain_str)
                                        self.automaton.add_word(kw_str, kw_str)
                                if cursor == 0:
                                    break

                if self.keyword_to_term:
                    self.automaton.make_automaton()
                    self._initialized = True

            except Exception as e:
                print(f"[KeywordMatcher] Rebuild error: {e}", flush=True)

    def find_tags(self, content: str, file_last_accessed: int = 0) -> dict:
        """Find matching terms and domain in content via AC scan with eligibility filter.

        Args:
            content: Text to scan for keywords
            file_last_accessed: Timestamp when file was last accessed

        Returns:
            Dict with domain (str), tags (list[str]), matched_keywords (list[str])
        """
        if not self.automaton or not self._initialized:
            return {"domain": None, "tags": []}

        try:
            tags = set()
            domain_counts = {}
            matched_keywords = []
            content_lower = content.lower()

            for _, keyword in self.automaton.iter(content_lower):
                mapping = self.keyword_to_term.get(keyword)
                if not mapping:
                    continue

                term_name, domain_name = mapping
                created_at = self.domain_created_at.get(domain_name, 0)

                if created_at == 0 or file_last_accessed == 0 or file_last_accessed > created_at:
                    tags.add(term_name)
                    matched_keywords.append(keyword)
                    domain_counts[domain_name] = domain_counts.get(domain_name, 0) + 1

            top_domain = max(domain_counts, key=domain_counts.get) if domain_counts else None

            return {
                "domain": top_domain,
                "tags": list(tags)[:3],
                "matched_keywords": matched_keywords
            }

        except Exception as e:
            print(f"[KeywordMatcher] find_tags error: {e}", flush=True)
            return {"domain": None, "tags": []}

    def update_from_domains(self, domain_names, get_terms_fn, get_keywords_fn):
        """Update automaton with new domain data.

        Args:
            domain_names: List of domain names
            get_terms_fn: Function(domain) -> list of terms
            get_keywords_fn: Function(term) -> list of keywords
        """
        # Trigger a full rebuild - simpler than incremental update
        self.rebuild()

    @property
    def keyword_count(self) -> int:
        """Number of keywords in the automaton."""
        return len(self.keyword_to_term)

    @property
    def is_available(self) -> bool:
        """Check if keyword matching is available."""
        return self._initialized and self.automaton is not None


# ============================================================================
# Global KeywordMatcher management
# ============================================================================

KEYWORD_MATCHER: KeywordMatcher | None = None
_KEYWORD_MATCHER_LOCK = threading.Lock()

# Reference to intent_index (set via set_intent_index)
_intent_index = None


def set_intent_index(intent_idx):
    """Set the default intent_index for get_keyword_matcher fallback.

    Call this from indexer.py after creating intent_index:
        from matchers import set_intent_index
        set_intent_index(intent_index)
    """
    global _intent_index
    _intent_index = intent_idx


def get_keyword_matcher(project_id: str = None, intent_idx=None) -> KeywordMatcher | None:
    """Get or create the global KeywordMatcher instance.

    Handles project_id changes by rebuilding if needed.

    Args:
        project_id: Project ID for Redis key prefix
        intent_idx: IntentIndex with Redis client (uses default if not provided)
    """
    global KEYWORD_MATCHER
    proj = project_id or 'local'

    # Use provided intent_idx or fall back to module default
    idx = intent_idx if intent_idx is not None else _intent_index

    if idx and idx.redis:
        needs_init = KEYWORD_MATCHER is None
        needs_rebuild = KEYWORD_MATCHER is not None and KEYWORD_MATCHER.project_id != proj

        if needs_init or needs_rebuild:
            with _KEYWORD_MATCHER_LOCK:
                if KEYWORD_MATCHER is None:
                    KEYWORD_MATCHER = KeywordMatcher(proj, idx.redis)
                    KEYWORD_MATCHER.rebuild()
                elif KEYWORD_MATCHER.project_id != proj:
                    KEYWORD_MATCHER = KeywordMatcher(proj, idx.redis)
                    KEYWORD_MATCHER.rebuild()

    return KEYWORD_MATCHER
