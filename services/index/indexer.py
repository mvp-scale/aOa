#!/usr/bin/env python3
"""
Codebase Indexer - Multi-Index Architecture
Fast symbol lookup with isolated local and knowledge repo indexes.

Architecture:
  - LOCAL index: Your project_id code (always active, default)
  - REPO indexes: Knowledge repos (only queried explicitly)

Usage:
    CODEBASE_ROOT=/path/to/code REPOS_ROOT=/path/to/repos python indexer.py
"""

import hashlib
import json
import os
import re
import subprocess
import threading
import time
from collections import OrderedDict, defaultdict, deque
from dataclasses import asdict, dataclass
from pathlib import Path

from flask import Flask, jsonify, request
from watchdog.events import FileSystemEventHandler
from watchdog.observers import Observer

# Aho-Corasick for O(n) multi-pattern matching (GL-047)
try:
    import ahocorasick
    AHOCORASICK_AVAILABLE = True
except ImportError:
    AHOCORASICK_AVAILABLE = False

# Tree-sitter for code outlines (165+ languages via language-pack)
try:
    from tree_sitter import Parser, Query, QueryCursor
    from tree_sitter_language_pack import get_language
    TREE_SITTER_AVAILABLE = True

    def get_ts_language(lang_name: str):
        """Get a tree-sitter language by name. Supports 165+ languages."""
        # Map common aliases to tree-sitter-language-pack names
        LANG_ALIASES = {
            'typescript': 'typescript',
            'javascript': 'javascript',
            'python': 'python',
            'go': 'go',
            'rust': 'rust',
            'java': 'java',
            'c': 'c',
            'cpp': 'cpp',
            'ruby': 'ruby',
            'php': 'php',
            'c_sharp': 'csharp',
            'csharp': 'csharp',
            'swift': 'swift',
            'kotlin': 'kotlin',
            'scala': 'scala',
            'bash': 'bash',
            'shell': 'bash',
            'html': 'html',
            'css': 'css',
            'json': 'json',
            'yaml': 'yaml',
            'toml': 'toml',
            'sql': 'sql',
            'lua': 'lua',
            'elixir': 'elixir',
            'haskell': 'haskell',
            'ocaml': 'ocaml',
            'r': 'r',
            'julia': 'julia',
            'dart': 'dart',
            'zig': 'zig',
            'nim': 'nim',
            'perl': 'perl',
            'markdown': 'markdown',
            'vue': 'vue',
            'svelte': 'svelte',
        }
        try:
            name = LANG_ALIASES.get(lang_name, lang_name)
            return get_language(name)
        except Exception:
            return None

except ImportError:
    TREE_SITTER_AVAILABLE = False
    def get_ts_language(x):
        return None

# Ranking module for predictive file scoring
import contextlib
import sys

sys.path.insert(0, '/app')  # For Docker
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))  # For local
try:
    from ranking import Scorer, WeightTuner
    RANKING_AVAILABLE = True
except ImportError:
    RANKING_AVAILABLE = False
    Scorer = None
    WeightTuner = None

# Domain learning for dynamic semantic tagging (GL-053)
try:
    from domains import DomainLearner, Domain
    DOMAINS_AVAILABLE = True
except ImportError:
    DOMAINS_AVAILABLE = False
    DomainLearner = None
    Domain = None

# GL-089: Job queue for background work
try:
    from jobs.queue import JobQueue, Job, JobType, create_enrich_job
    from jobs.worker import JobWorker, push_enrich_jobs
    JOBS_AVAILABLE = True
except ImportError:
    JOBS_AVAILABLE = False
    JobQueue = None
    JobWorker = None

# ============================================================================
# O1-013: Pre-compiled Regexes (avoid re.compile per call)
# ============================================================================
_TOKENIZE_SPLIT_RE = re.compile(r'[/_\-.\s]+')
_TOKENIZE_CAMEL_RE = re.compile(r'[A-Z]?[a-z]+|[A-Z]+(?=[A-Z][a-z]|\d|\W|$)|\d+')

# O1-012: LRU cache for compiled regex patterns
from functools import lru_cache

@lru_cache(maxsize=128)
def _compile_pattern(pattern: str, flags: int) -> re.Pattern:
    """Cache compiled regex patterns to avoid recompilation."""
    return re.compile(pattern, flags)

# ============================================================================
# Pattern Library for Intent Inference (RAM-cached at startup)
# ============================================================================

def _load_pattern_library():
    """Load semantic and domain pattern configs from project-domains.json.

    GL-084: Universal domains removed. Only project-specific domains are used.
    Generated via /aoa-setup skill.

    v2 Structure: @domain -> semantic_term -> matches[]

    Returns:
    - semantic_patterns: semantic_term -> {patterns: set, tag: @domain, priority: int}
    - domain_keywords: match -> @domain
    - term_to_domain: semantic_term -> @domain (for reverse lookup)
    - match_to_term: match -> semantic_term (for tagging)
    """
    semantic_patterns = {}  # semantic_term -> {patterns: set, tag: @domain, priority: int}
    domain_keywords = {}    # match -> @domain
    term_to_domain = {}     # semantic_term -> @domain
    match_to_term = {}      # match -> semantic_term

    # GL-084: Only project-domains.json - no universal fallback
    # Generated via /aoa-setup or aoa analyze
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
        # No project domains yet - run /aoa-setup to generate
        return semantic_patterns, domain_keywords

    try:
        data = json.loads(domains_file.read_text())
        # Handle both array format (v2) and object format (with _meta)
        domains = data if isinstance(data, list) else data.get('domains', [])

        for domain in domains:
            domain_name = domain.get('name', '')  # e.g., "@authentication"
            terms = domain.get('terms', {})

            # v2 format: terms is dict of semantic_term -> matches[]
            if isinstance(terms, dict):
                for semantic_term, matches in terms.items():
                    # Build semantic patterns (semantic_term -> matches)
                    patterns = {m.lower() for m in matches}
                    semantic_patterns[semantic_term] = {
                        'patterns': patterns,
                        'tag': domain_name,  # @authentication
                        'priority': 3
                    }
                    term_to_domain[semantic_term] = domain_name

                    # Build match lookups
                    for match in matches:
                        match_lower = match.lower()
                        domain_keywords[match_lower] = domain_name
                        match_to_term[match_lower] = semantic_term

            # Flat format fallback: terms is list of matches
            elif isinstance(terms, list):
                for match in terms:
                    domain_keywords[match.lower()] = domain_name

    except Exception:
        pass

    return semantic_patterns, domain_keywords

# Load patterns at startup (RAM-cached)
SEMANTIC_PATTERNS, DOMAIN_KEYWORDS = _load_pattern_library()


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
        self.pattern_meta = {}  # pattern -> {tag, source, category, priority}
        self._build_automaton()

    def _build_automaton(self):
        """Build unified automaton from all pattern sources."""
        if not AHOCORASICK_AVAILABLE:
            return

        self.automaton = ahocorasick.Automaton()

        # Add semantic patterns (action verbs like create, get, update)
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

        # Add domain patterns (tech keywords like redis, jwt, kafka)
        for keyword, tag in DOMAIN_KEYWORDS.items():
            keyword_lower = keyword.lower()
            if keyword_lower not in self.pattern_meta:  # Don't overwrite semantic
                self.pattern_meta[keyword_lower] = {
                    'tag': tag,
                    'source': 'domain',
                    'category': tag.lstrip('#'),  # Use tag as category
                    'priority': 3
                }
                self.automaton.add_word(keyword_lower, keyword_lower)

        # Finalize automaton for searching
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

        # Convert categories to #term tags (not @domain)
        tags = []
        for cat, _ in dense:
            # cat is the term_name, return as #term
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


# Initialize global matcher (built once at startup)
AC_MATCHER = AhoCorasickMatcher()


# ============================================================================
# GL-XXX: Redis-backed Keyword Matcher for semantic term tagging
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
        self.keyword_to_term = {}        # keyword → (term_name, domain_name) tuple
        self.domain_created_at = {}      # domain → created_at timestamp
        self._initialized = False
        # T-004: Lock to prevent find_tags() from running during rebuild
        self.rebuild_lock = threading.RLock()

    def rebuild(self):
        """Rebuild AC automaton from Redis term:* keys."""
        if not AHOCORASICK_AVAILABLE:
            return

        # T-004: Lock during rebuild to prevent find_tags() NPE
        with self.rebuild_lock:
            self.automaton = ahocorasick.Automaton()
            self.keyword_to_term = {}
            self.domain_created_at = {}

            try:
                r = self.redis.client if hasattr(self.redis, 'client') else self.redis

                # Load domain timestamps first
                domain_pattern = f"aoa:{self.project_id}:domain:*:meta"
                for key in r.scan_iter(domain_pattern):
                    key_str = key.decode() if isinstance(key, bytes) else key
                    parts = key_str.split(':')
                    if len(parts) >= 4:
                        domain_name = parts[3]  # @domain_name
                        created = r.hget(key_str, 'created')
                        if created:
                            created_val = created.decode() if isinstance(created, bytes) else created
                            self.domain_created_at[domain_name] = int(created_val)
                        else:
                            self.domain_created_at[domain_name] = 0  # Cold start

                # Scan Redis for term:{name}:domain → STRING mappings
                # Term names are also searchable as keywords
                term_domain_pattern = f"aoa:{self.project_id}:term:*:domain"
                for key in r.scan_iter(term_domain_pattern):
                    key_str = key.decode() if isinstance(key, bytes) else key
                    parts = key_str.split(':')
                    if len(parts) >= 5:  # aoa:project:term:name:domain
                        term_name = parts[3]  # The term name (also searchable as keyword)

                        # Get domain from STRING key
                        domain = r.get(key_str)
                        if domain:
                            domain_str = domain.decode() if isinstance(domain, bytes) else domain
                            # Store tuple: (term_name, domain_name) for term-level output
                            self.keyword_to_term[term_name.lower()] = (term_name, domain_str)
                            self.automaton.add_word(term_name.lower(), term_name.lower())

                # Also add keywords from term:*:keywords SETs
                for key in r.scan_iter(f"aoa:{self.project_id}:term:*:keywords"):
                    key_str = key.decode() if isinstance(key, bytes) else key
                    parts = key_str.split(':')
                    if len(parts) >= 5:  # aoa:project:term:name:keywords
                        term_name = parts[3]  # The term name

                        # Get parent term's domain from STRING key
                        domain_key = f"aoa:{self.project_id}:term:{term_name}:domain"
                        domain = r.get(domain_key)
                        if domain:
                            domain_str = domain.decode() if isinstance(domain, bytes) else domain

                            # Add all keywords from this term's keyword set
                            # O1-003: Use sscan instead of smembers for streaming iteration
                            cursor = 0
                            while True:
                                cursor, keywords = r.sscan(key_str, cursor=cursor, count=100)
                                for kw in keywords:
                                    kw_str = (kw.decode() if isinstance(kw, bytes) else kw).lower()
                                    if len(kw_str) >= 3 and kw_str not in self.keyword_to_term:
                                        # Store tuple: (term_name, domain_name) - keyword maps to its parent term
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
            file_last_accessed: Timestamp when file was last accessed (from intent tracking)
                               0 means unknown/never tracked, which allows cold start domains

        Returns:
            Dict with domain (str) and tags (list[str])
            e.g., {"domain": "@search", "tags": ["#indexing", "#query_processing"]}
        """
        if not self.automaton or not self._initialized:
            return {"domain": None, "tags": []}

        try:
            tags = set()
            domain_counts = {}  # Track which domain has most matches
            matched_keywords = []  # P3-2: Track matched keywords for hit counting
            content_lower = content.lower()

            for _, keyword in self.automaton.iter(content_lower):
                mapping = self.keyword_to_term.get(keyword)
                if not mapping:
                    continue

                term_name, domain_name = mapping

                # Check eligibility: cold start (0) or file accessed after domain created
                created_at = self.domain_created_at.get(domain_name, 0)

                # Eligibility rules:
                # 1. Cold start domains (created_at=0) always eligible
                # 2. Unknown file access (file_last_accessed=0) - show all domains (benefit of doubt)
                # 3. Known file access after domain created - eligible
                if created_at == 0 or file_last_accessed == 0 or file_last_accessed > created_at:
                    tags.add(term_name)
                    matched_keywords.append(keyword)  # P3-2: Collect matched keyword
                    # Track domain match count
                    domain_counts[domain_name] = domain_counts.get(domain_name, 0) + 1

            # P3-2: Hit tracking removed from hot path
            # Tracked via intent system at request level, not per-result

            # Pick domain with most matches
            top_domain = max(domain_counts, key=domain_counts.get) if domain_counts else None

            return {
                "domain": top_domain,
                "tags": list(tags)[:3],
                "matched_keywords": matched_keywords  # KW-001: Return for hit tracking
            }

        except Exception as e:
            print(f"[KeywordMatcher] find_tags error: {e}", flush=True)
            return {"domain": None, "tags": []}

    @property
    def keyword_count(self) -> int:
        """Number of keywords in the automaton."""
        return len(self.keyword_to_term)

    @property
    def is_available(self) -> bool:
        """Check if keyword matching is available."""
        return self._initialized and self.automaton is not None


# Global KeywordMatcher instance (lazy init)
KEYWORD_MATCHER: KeywordMatcher | None = None
# T-006: Lock for thread-safe initialization
_KEYWORD_MATCHER_LOCK = threading.Lock()


def get_keyword_matcher(project_id: str = None, intent_idx: 'IntentIndex | None' = None) -> KeywordMatcher | None:
    """Get or create the global KeywordMatcher instance.

    Handles project_id changes by rebuilding if needed.

    Args:
        project_id: Project ID for Redis key prefix
        intent_idx: IntentIndex with Redis client (uses global intent_index if not provided)
    """
    global KEYWORD_MATCHER
    proj = project_id or 'local'

    # Use provided intent_idx or fall back to global
    idx = intent_idx if intent_idx is not None else intent_index

    if idx and idx.redis:
        # T-006: Double-check pattern with lock for thread safety
        needs_init = KEYWORD_MATCHER is None
        needs_rebuild = KEYWORD_MATCHER is not None and KEYWORD_MATCHER.project_id != proj

        if needs_init or needs_rebuild:
            with _KEYWORD_MATCHER_LOCK:
                # Double-check after acquiring lock
                if KEYWORD_MATCHER is None:
                    # First initialization
                    KEYWORD_MATCHER = KeywordMatcher(proj, idx.redis)
                    KEYWORD_MATCHER.rebuild()
                elif KEYWORD_MATCHER.project_id != proj:
                    # Project changed, rebuild with new project
                    KEYWORD_MATCHER = KeywordMatcher(proj, idx.redis)
                    KEYWORD_MATCHER.rebuild()

    return KEYWORD_MATCHER


def _tokenize_search(text: str) -> set:
    """Tokenize search term into matchable parts."""
    tokens = set()
    # O1-013: Use pre-compiled regex
    parts = _TOKENIZE_SPLIT_RE.split(text.lower())
    for part in parts:
        if part:
            tokens.add(part)
            # Split camelCase - O1-013: Use pre-compiled regex
            camel_parts = _TOKENIZE_CAMEL_RE.findall(part)
            tokens.update(p.lower() for p in camel_parts)
    return tokens


def infer_search_intent(search_term: str) -> list:
    """Infer intent tags from a search term using pattern library.

    Example: "authenticate" -> ["#auth"]
    Example: "getUserById" -> ["#read", "#user"]
    """
    tags = set()
    tokens = _tokenize_search(search_term)

    # Match semantic patterns (action verbs)
    for cat_data in SEMANTIC_PATTERNS.values():
        for token in tokens:
            for pattern in cat_data['patterns']:
                if token.startswith(pattern) or token == pattern:
                    tags.add(cat_data['tag'])
                    break

    # Match domain keywords
    for keyword, tag in DOMAIN_KEYWORDS.items():
        if keyword in tokens or keyword in search_term.lower():
            tags.add(tag)

    return list(tags)


def calculate_intent_score(result_tags: list, search_intent: list) -> float:
    """Calculate similarity between result tags and search intent.

    Uses Jaccard-like scoring: intersection / union, weighted.
    Returns 0.0 to 1.0 (higher = better alignment).
    """
    if not search_intent:
        return 0.5  # No search intent, neutral score
    if not result_tags:
        return 0.0  # No tags, can't score

    search_set = set(search_intent)
    result_set = set(result_tags)

    intersection = len(search_set & result_set)
    len(search_set | result_set)

    # Boost for matching search intent (weighted towards search terms)
    match_ratio = intersection / len(search_set) if search_set else 0

    return match_ratio


# ============================================================================
# Phase 3B: Pre-Batch Enrichment - ONE Redis call for ALL results
# ============================================================================

def batch_fetch_enrichment_data(
    results: list[dict],
    intent_index: 'IntentIndex | None',
    project_id: str | None,
    idx: 'CodebaseIndex | None' = None
) -> tuple[dict, dict, dict]:
    """
    Pre-batch ALL enrichment data in ONE Redis pipeline call.

    Instead of N Redis calls (one per result), this fetches everything upfront.
    Benchmark: 130 ops goes from 14.76ms (loop) to 1.22ms (pipeline) = 12x faster.

    Args:
        results: List of search results with 'file', 'line', and optional '_file_id'
        intent_index: IntentIndex with Redis connection
        project_id: Project ID for Redis key prefix
        idx: CodebaseIndex for looking up symbols from metadata_store

    Returns:
        (file_tags_lookup, symbol_domains_lookup, file_accessed_lookup) - dicts for O(1) Python lookups
    """
    file_tags_lookup = {}
    symbol_domains_lookup = {}
    file_accessed_lookup = {}

    if not intent_index or not hasattr(intent_index, 'redis') or not intent_index.redis:
        return file_tags_lookup, symbol_domains_lookup, file_accessed_lookup

    proj = project_id or 'local'
    r = intent_index.redis.client if hasattr(intent_index.redis, 'client') else intent_index.redis

    # Collect unique files from results
    unique_files = list({res.get('file') for res in results if res.get('file')})

    # Collect unique (file, symbol) pairs by looking up metadata_store
    # This allows pre-batching domain lookups even though results don't have symbols yet
    unique_symbols = set()
    for res in results:
        file_path = res.get('file')
        line_num = res.get('line')
        file_id = res.get('_file_id')

        # Get file_id if not provided
        if file_id is None and idx and file_path in idx.path_to_id:
            file_id = idx.path_to_id[file_path]

        # Look up symbol from metadata_store
        if idx and file_id is not None and line_num:
            meta = idx.metadata_store.get((file_id, line_num))
            if meta:
                symbol = meta.get('symbol')
                if symbol:
                    unique_symbols.add((file_path, symbol))
                # Also check parent for fallback domain lookup
                parent = meta.get('parent_name')
                if parent:
                    unique_symbols.add((file_path, parent))

    unique_symbols = list(unique_symbols)

    if not unique_files and not unique_symbols:
        return file_tags_lookup, symbol_domains_lookup, file_accessed_lookup

    # Build and execute ONE pipeline
    pipe = r.pipeline()

    # Queue file_tags lookups
    for f in unique_files:
        pipe.smembers(f"aoa:{proj}:file_tags:{f}")

    # Queue symbol_domains lookups
    for f, sym in unique_symbols:
        pipe.smembers(f"aoa:{proj}:symbol_domains:{f}:{sym}")

    # Queue file_accessed lookups (for keyword matcher eligibility)
    for f in unique_files:
        pipe.zscore(f"aoa:{proj}:file_accessed", f)

    # Execute ALL in one network round-trip
    try:
        batch_results = pipe.execute()
    except Exception:
        return file_tags_lookup, symbol_domains_lookup, file_accessed_lookup

    # Parse file_tags results
    generic_tags = {'#executing', '#reading', '#editing', '#creating', '#searching',
                   '#delegating', '#shell', '#python', '#markdown', '#indexing',
                   '#hooks', '#test', '#grep', '#curl'}

    for i, f in enumerate(unique_files):
        tags = batch_results[i]
        if tags:
            decoded = [t.decode() if isinstance(t, bytes) else t for t in tags]
            semantic = [t for t in decoded if t not in generic_tags]
            file_tags_lookup[f] = sorted(semantic)[:3] if semantic else sorted(decoded)[:3]
        else:
            file_tags_lookup[f] = []

    # Parse symbol_domains results
    offset = len(unique_files)
    for i, (f, sym) in enumerate(unique_symbols):
        domains = batch_results[offset + i]
        if domains:
            domain = list(domains)[0]
            symbol_domains_lookup[(f, sym)] = domain.decode() if isinstance(domain, bytes) else domain

    # Parse file_accessed results (for keyword matcher eligibility)
    offset2 = len(unique_files) + len(unique_symbols)
    for i, f in enumerate(unique_files):
        accessed = batch_results[offset2 + i]
        file_accessed_lookup[f] = int(accessed) if accessed else 0

    return file_tags_lookup, symbol_domains_lookup, file_accessed_lookup


# ============================================================================
# GL-050: Universal Output - Single source of truth for all search results
# ============================================================================

def enrich_result(
    file_path: str,
    line_num: int,
    content: str,
    idx: 'CodebaseIndex',
    intent_index: 'IntentIndex | None' = None,
    project_id: str | None = None,
    file_tags_cache: dict | None = None,
    file_id: int | None = None,  # Task 3.5: Optional file_id for O(1) metadata lookup
    # Phase 3B: Pre-batched lookups (O(1) Python dict, no Redis at query time)
    file_tags_lookup: dict | None = None,
    symbol_domains_lookup: dict | None = None,
    file_accessed_lookup: dict | None = None
) -> dict:
    """
    Universal result enrichment - THE SINGLE SOURCE OF TRUTH.

    Both grep and egrep call this to get identical output format.

    Args:
        file_path: Relative path to file
        line_num: Line number of match
        content: The matched line content
        idx: CodebaseIndex with file_outlines
        intent_index: Optional IntentIndex for tags
        project_id: Project ID for tag lookup
        file_id: Optional file_id for O(1) metadata_store lookup

    Returns:
        Standardized result dict with consistent keys
    """
    # =================================================================
    # O(1) OPTIMIZATION: Task 3.5 - Try metadata_store first
    # =================================================================

    # Task 3.5a: If we have file_id, try O(1) metadata_store lookup
    metadata = None
    if file_id is not None:
        meta_key = (file_id, line_num)
        metadata = idx.metadata_store.get(meta_key)

    # Task 3.5b: If no file_id provided, try to get it from path_to_id
    if metadata is None and file_path in idx.path_to_id:
        file_id = idx.path_to_id[file_path]
        meta_key = (file_id, line_num)
        metadata = idx.metadata_store.get(meta_key)

    # Task 3.5c: Use metadata if available (O(1) path)
    if metadata:
        symbol_name = metadata.get('symbol')
        symbol_signature = metadata.get('signature')
        symbol_kind = metadata.get('symbol_kind')
        start_line = metadata.get('start_line')
        end_line = metadata.get('end_line')
        parent_name = metadata.get('parent_name')
        symbol_tags = metadata.get('tags') or []
        # Task 3.5c: Check for cached domain/terms (populated by learning)
        cached_domain = metadata.get('domain')
        cached_terms = metadata.get('terms')
    else:
        # Fallback: Get symbol context from pre-indexed outlines
        symbol_name = None
        symbol_signature = None
        symbol_kind = None
        start_line = None
        end_line = None
        parent_name = None
        symbol_tags = []
        cached_domain = None
        cached_terms = None

        symbols = idx.file_outlines.get(file_path, [])

        # Find INNERMOST enclosing symbol (smallest range that contains the line)
        # This ensures we get the method, not the class, when line is inside a method
        # Also track the PARENT (next smallest enclosing symbol) for hierarchy display
        enclosing = []  # All symbols that contain this line
        for sym in symbols:
            if sym.start_line <= line_num <= sym.end_line:
                sym_range = sym.end_line - sym.start_line
                enclosing.append((sym_range, sym))

        # Sort by range (smallest first = most specific)
        enclosing.sort(key=lambda x: x[0])

        if enclosing:
            # Best match is the innermost (smallest range)
            _, best_match = enclosing[0]
            symbol_name = best_match.name
            symbol_signature = best_match.signature
            symbol_kind = best_match.kind
            start_line = best_match.start_line
            end_line = best_match.end_line
            symbol_tags = best_match.tags or []

            # Check for stored parent_name first
            parent_name = getattr(best_match, 'parent_name', None)

            # If no stored parent, look for the next enclosing symbol (parent)
            if not parent_name and len(enclosing) > 1:
                _, parent_sym = enclosing[1]  # Second smallest = parent
                parent_name = parent_sym.name

    # Get file-level tags - Phase 3B: Use pre-batched lookup (O(1) Python dict)
    file_tags = []
    if file_tags_lookup is not None and file_path in file_tags_lookup:
        # Phase 3B: O(1) Python dict lookup (no Redis)
        file_tags = file_tags_lookup[file_path]
    elif intent_index and project_id:
        # Fallback: Redis lookup (only if pre-batch not provided)
        if file_tags_cache is not None and file_path in file_tags_cache:
            file_tags = file_tags_cache[file_path]
        else:
            file_tags = list(intent_index.tags_for_file(file_path, project_id))
            if file_tags_cache is not None:
                file_tags_cache[file_path] = file_tags

    # GL-084: Domain/Term hierarchy
    # - @domain (magenta): ONLY on symbol definition line (line == start_line)
    # - #term (cyan): On lines INSIDE symbol where keywords match content
    #
    # This creates the visual hierarchy:
    #   file.py:login()[10-45]:10 def login(user):  @authentication
    #   file.py:login()[10-45]:25   token = jwt.encode()  #token_lifecycle

    domain = None
    term_tags = []
    is_definition_line = start_line is not None and line_num == start_line

    # Phase 3B: Use pre-batched lookup for domains (O(1) Python dict)
    if symbol_domains_lookup is not None:
        # O(1) Python dict lookup (no Redis)
        if symbol_name:
            domain = symbol_domains_lookup.get((file_path, symbol_name))
        if not domain and parent_name:
            domain = symbol_domains_lookup.get((file_path, parent_name))
    elif intent_index and project_id:
        # Fallback: Redis lookup (only if pre-batch not provided)
        if symbol_name:
            symbol_domains = intent_index.domains_for_symbol(file_path, symbol_name, project_id)
            if symbol_domains:
                domain = symbol_domains[0]
        if not domain and parent_name:
            parent_domains = intent_index.domains_for_symbol(file_path, parent_name, project_id)
            if parent_domains:
                domain = parent_domains[0]

    # GL-XXX: Match keywords from Redis to get #term tags (replaces legacy SEMANTIC_PATTERNS)
    # Only for lines INSIDE a symbol (not the definition line)
    # Uses KeywordMatcher with timestamp-based eligibility
    if not is_definition_line:
        matcher = get_keyword_matcher(project_id, intent_index)
        if matcher and matcher.is_available:
            # Get file's last accessed time for eligibility filtering
            # Phase 3B: Use pre-batched lookup (O(1) Python dict)
            file_accessed = 0
            if file_accessed_lookup is not None and file_path in file_accessed_lookup:
                file_accessed = file_accessed_lookup[file_path]
            elif intent_index and project_id:
                # Fallback: Redis lookup
                file_accessed = intent_index.get_file_last_accessed(file_path, project_id)

            # Find matching domain and tags with eligibility filter
            match_result = matcher.find_tags(content, file_accessed)
            top_domain = match_result.get("domain")  # e.g., "@search"
            keyword_tags = match_result.get("tags", [])  # e.g., ["#indexing", "#query"]
            term_tags = keyword_tags  # Already in #term format
        elif SEMANTIC_PATTERNS:
            # Legacy fallback: file-based patterns
            import re
            content_tokens = set(re.findall(r'[a-z][a-z0-9_]*', content.lower()))
            for term_name, term_data in SEMANTIC_PATTERNS.items():
                patterns = term_data.get('patterns', set())
                if patterns & content_tokens:
                    term_tags.append(f"#{term_name}")
            term_tags = term_tags[:3]

    # GL-053: Fallback to AC matching if no tags
    all_tags = file_tags + symbol_tags + term_tags
    if not all_tags and AHOCORASICK_AVAILABLE:
        try:
            text_for_tags = content
            if symbol_name:
                text_for_tags = f"{symbol_name} {content}"
            ac_tags = AC_MATCHER.get_dense_tags(text_for_tags, threshold=1)
            all_tags = ac_tags[:5]
        except Exception:
            pass

    # Build universal result format
    result = {
        'file': file_path,
        'line': line_num,
        'content': content.strip()[:200],
        'tags': all_tags
    }

    # GL-084: Show @domain on definition lines OR from KeywordMatcher
    if domain and is_definition_line:
        result['domain'] = domain
    elif 'top_domain' in locals() and top_domain:
        result['domain'] = top_domain

    # Add symbol info if found (all or nothing - consistent shape)
    if symbol_name:
        result['symbol'] = symbol_name
        result['signature'] = symbol_signature
        result['kind'] = symbol_kind
        result['start_line'] = start_line
        result['end_line'] = end_line
        if parent_name:
            result['parent_name'] = parent_name  # CLI expects parent_name

    return result


def format_search_response(
    results: list[dict],
    ms: float,
    files_searched: int,
    search_intent: list | None = None,
    intent_index: 'IntentIndex | None' = None,
    project_id: str | None = None
) -> dict:
    """
    Universal response format - THE SINGLE SOURCE OF TRUTH.

    Both grep and egrep call this to get identical response structure.
    GL-050: No limit at API layer - inverted index is O(1), return all matches.
    Display layer (CLI) handles how many to show.

    Args:
        results: List of enriched result dicts
        ms: Elapsed time in milliseconds
        files_searched: Number of files examined
        search_intent: Inferred search intent tags
        intent_index: Optional IntentIndex for rolling intent
        project_id: Project ID for rolling intent

    Returns:
        Standardized response dict ready for jsonify()
    """
    # Score and sort by intent alignment
    # TODO: VALIDATE - If Redis already pre-sorts by intent/recency/frequency,
    # this sort is redundant and should be removed. No reason to sort twice.
    # See: services/ranking/ for pre-sorting logic
    if search_intent:
        for r in results:
            r['score'] = calculate_intent_score(r.get('tags', []), search_intent)
        results.sort(key=lambda r: r['score'], reverse=True)

    # GL-071: Enrich terms from top search results (100% non-blocking)
    # Top results validate term relevance - fire and forget
    if DOMAINS_AVAILABLE and project_id and results:
        # Snapshot top 10 for background processing
        top_results = results[:10]
        def _enrich_top_terms():
            try:
                learner = DomainLearner(project_id)
                seen_terms = set()
                seen_domains = set()
                seen_keywords = set()  # KW-002: Collect keywords for hit tracking
                matcher = get_keyword_matcher(project_id, intent_index) if intent_index else None

                # Collect all unique terms from top results
                for r in top_results:
                    for tag in r.get('tags', []):
                        term = tag.lstrip('#@').lower()
                        if term and len(term) >= 3:
                            seen_terms.add(term)
                    # GL-088: Also track domains from results
                    domain = r.get('domain', '')
                    if domain:
                        seen_domains.add(domain.lstrip('@'))
                    # KW-002: Collect keywords from content
                    if matcher and matcher.is_available:
                        match_result = matcher.find_tags(r.get('content', ''))
                        for kw in match_result.get('matched_keywords', []):
                            seen_keywords.add(kw)

                # UNIFY-001: Increment term hits and collect their domains
                for term in seen_terms:
                    learner.increment_term_hits(term)
                    # Collect domains with this term (don't increment yet)
                    domains_with_term = learner.get_domains_for_term(term)
                    for domain_name in domains_with_term:
                        seen_domains.add(domain_name)

                # KW-003: Increment keyword hits
                if seen_keywords:
                    learner.increment_keyword_hits(list(seen_keywords))

                # UNIFY-004: Batch increment ALL seen_domains (from results + terms)
                for domain in seen_domains:
                    learner.increment_domain_hits(domain)
            except Exception as e:
                print(f"[Enrich] Error: {e}", flush=True)  # HIT-002: observability
        # Fire and forget
        import threading
        threading.Thread(target=_enrich_top_terms, daemon=True).start()

    # Build response - return ALL results, display layer limits
    response = {
        'results': results,
        'ms': ms,
        'total_matches': len(results),
        'files_searched': files_searched,
        'files_matched': len(set(r['file'] for r in results))
    }

    # Add search intent if provided
    if search_intent:
        response['search_intent'] = search_intent

    # Add rolling intent if available
    if intent_index and project_id:
        rolling_tags = intent_index.get_rolling_intent(project_id, window=50)
        if rolling_tags:
            top_tags = sorted(rolling_tags.items(), key=lambda x: x[1], reverse=True)[:5]
            response['rolling_intent'] = [tag for tag, _ in top_tags]  # Tags already have # prefix

    return response


app = Flask(__name__)

# ============================================================================
# Data Structures
# ============================================================================

@dataclass
class Location:
    file: str
    line: int
    col: int
    symbol_type: str
    mtime: int
    # Symbol-level metadata for semantic compression tags
    symbol: str | None = None      # Symbol/function name (e.g., "handleAuth")
    symbol_kind: str | None = None # Kind (e.g., "function", "class")
    start_line: int | None = None  # GL-047.7: Where the symbol starts (for correct range display)
    end_line: int | None = None    # Where the symbol ends
    signature: str | None = None   # GL-047.7: Definition header (e.g., "def foo(x: int) -> str")
    # GL-047.8: Parent hierarchy for AI-readable context
    parent_name: str | None = None       # Parent class/function name (e.g., "OutlineParser")
    parent_signature: str | None = None  # Parent signature with params (e.g., "class OutlineParser()")
    parent_start: int | None = None      # Parent start line
    parent_end: int | None = None        # Parent end line
    content: str | None = None     # GL-046: Line content for O(1) display
    tags: list | None = None       # GL-047: AC-computed dense tags at index time

@dataclass
class FileMeta:
    path: str
    mtime: int
    size: int
    language: str
    content_hash: str

@dataclass
class ChangeRecord:
    file: str
    timestamp: int
    change_type: str  # added, modified, deleted
    lines_changed: list[int] | None = None


@dataclass
class IntentRecord:
    """Record of an intent capture from tool usage."""
    timestamp: int
    session_id: str
    tool: str
    files: list[str]
    tags: list[str]
    tool_use_id: str | None = None  # Claude's toolu_xxx correlation key
    project_id: str | None = None  # UUID for per-project_id isolation
    file_sizes: dict[str, int] | None = None  # File path -> size in bytes (for baseline calc)
    output_size: int | None = None  # Actual output size in bytes (for real savings calc)
    # GL-069.5: Rich locations - resolved symbols from file:line references
    # Format: [{"file": "path.py", "symbol": "func", "parent": "Class", "qualified": "path.py:Class.func()"}]
    locations: list[dict] | None = None


# ============================================================================
# GL-046.1: LRU Content Cache - O(1) file content access
# ============================================================================

class LRUContentCache:
    """Bounded LRU cache for file contents across all projects.

    Provides O(1) content access for search results without reading from disk.
    Shared across all projects with configurable memory cap.

    Memory math (500MB default):
    - Average line: ~50 bytes
    - Average file: ~500 lines = 25KB
    - 500MB / 25KB = ~20,000 files cached
    - Most codebases: 1-5K files = entire working set fits in cache
    """

    def __init__(self, max_size_mb: int = 500):
        self.max_size = max_size_mb * 1024 * 1024  # Convert to bytes
        self.current_size = 0
        # Key: (project_id, file_path) -> Value: list of lines
        self.cache: OrderedDict[tuple[str, str], list[str]] = OrderedDict()
        self.lock = threading.RLock()
        # Stats
        self.hits = 0
        self.misses = 0

    def get(self, project_id: str, file_path: str, file_root: Path) -> list[str] | None:
        """Get file content as lines. Returns None if file doesn't exist."""
        key = (project_id, file_path)

        with self.lock:
            if key in self.cache:
                self.cache.move_to_end(key)  # Mark as recently used
                self.hits += 1
                return self.cache[key]

        # Cache miss - read from disk
        self.misses += 1
        try:
            full_path = file_root / file_path
            content = full_path.read_text(encoding='utf-8', errors='ignore')
            lines = content.split('\n')
            self._add(key, lines)
            return lines
        except Exception:
            return None

    def get_line(self, project_id: str, file_path: str, line_num: int, file_root: Path) -> str | None:
        """Get a specific line from a file (1-indexed)."""
        lines = self.get(project_id, file_path, file_root)
        if lines and 0 < line_num <= len(lines):
            return lines[line_num - 1]
        return None

    def get_lines(self, project_id: str, file_path: str, line_nums: list[int], file_root: Path) -> dict[int, str]:
        """Get multiple lines from a file. Returns {line_num: content}."""
        result = {}
        lines = self.get(project_id, file_path, file_root)
        if lines:
            for ln in line_nums:
                if 0 < ln <= len(lines):
                    result[ln] = lines[ln - 1]
        return result

    def _add(self, key: tuple[str, str], lines: list[str]):
        """Add content to cache, evicting LRU entries if needed."""
        size = sum(len(line) for line in lines)

        with self.lock:
            # Evict until we have room
            while self.current_size + size > self.max_size and self.cache:
                evicted_key, evicted_lines = self.cache.popitem(last=False)
                self.current_size -= sum(len(line) for line in evicted_lines)

            self.cache[key] = lines
            self.current_size += size

    def invalidate(self, project_id: str, file_path: str):
        """Remove a file from cache (call on file change)."""
        key = (project_id, file_path)
        with self.lock:
            if key in self.cache:
                lines = self.cache.pop(key)
                self.current_size -= sum(len(line) for line in lines)

    def invalidate_project(self, project_id: str):
        """Remove all files for a project_id from cache."""
        with self.lock:
            keys_to_remove = [k for k in self.cache if k[0] == project_id]
            for key in keys_to_remove:
                lines = self.cache.pop(key)
                self.current_size -= sum(len(line) for line in lines)

    def stats(self) -> dict:
        """Return cache statistics."""
        with self.lock:
            total = self.hits + self.misses
            return {
                'size_mb': round(self.current_size / (1024 * 1024), 2),
                'max_size_mb': self.max_size // (1024 * 1024),
                'files_cached': len(self.cache),
                'hits': self.hits,
                'misses': self.misses,
                'hit_rate': round(self.hits / total * 100, 1) if total > 0 else 0
            }


# Global content cache instance (shared across all indexes)
CONTENT_CACHE_MB = int(os.environ.get('AOA_CONTENT_CACHE_MB', '500'))
content_cache = LRUContentCache(max_size_mb=CONTENT_CACHE_MB)


@dataclass
class OutlineSymbol:
    """A symbol extracted from code outline."""
    name: str
    kind: str  # function, class, method
    start_line: int
    end_line: int
    signature: str | None = None
    children: list['OutlineSymbol'] | None = None
    tags: list[str] | None = None  # AI-generated intent tags (via enrichment)


class OutlineParser:
    """Extract code structure using tree-sitter."""

    # Map file extensions to tree-sitter language names (97 supported)
    LANG_MAP = {
        # Tier 1: Core languages with full symbol extraction
        'python': 'python',
        'typescript': 'typescript',
        'javascript': 'javascript',
        'go': 'go',
        'rust': 'rust',
        'java': 'java',
        'c': 'c',
        'cpp': 'cpp',
        'ruby': 'ruby',
        'php': 'php',
        'swift': 'swift',
        'kotlin': 'kotlin',
        'scala': 'scala',
        'csharp': 'csharp',
        'c_sharp': 'csharp',
        # Tier 2: Additional languages with outline support
        'bash': 'bash',
        'shell': 'bash',
        'lua': 'lua',
        'elixir': 'elixir',
        'haskell': 'haskell',
        'ocaml': 'ocaml',
        'r': 'r',
        'julia': 'julia',
        'dart': 'dart',
        'zig': 'zig',
        'nim': 'nim',
        'perl': 'perl',
        'clojure': 'clojure',
        'erlang': 'erlang',
        # Tier 3: Extended languages
        'd': 'd',
        'cuda': 'cuda',
        'glsl': 'glsl',
        'hlsl': 'hlsl',
        'objc': 'objc',
        'ada': 'ada',
        'fortran': 'fortran',
        'gleam': 'gleam',
        'elm': 'elm',
        'purescript': 'purescript',
        'odin': 'odin',
        'v': 'v',
        'verilog': 'verilog',
        'vhdl': 'vhdl',
        'graphql': 'graphql',
        'terraform': 'terraform',
        'nix': 'nix',
        'fennel': 'fennel',
        'cmake': 'cmake',
        'make': 'make',
        'groovy': 'groovy',
        # Tier 4: Markup/config (basic outline)
        'html': 'html',
        'css': 'css',
        'json': 'json',
        'yaml': 'yaml',
        'toml': 'toml',
        'sql': 'sql',
        'markdown': 'markdown',
        'vue': 'vue',
        'svelte': 'svelte',
    }

    # Node types that represent symbols we want to extract (by language)
    SYMBOL_NODES = {
        'python': {
            'function_definition': 'function',
            'class_definition': 'class',
        },
        'typescript': {
            'function_declaration': 'function',
            'class_declaration': 'class',
            'method_definition': 'method',
            'arrow_function': 'function',
            'interface_declaration': 'interface',
        },
        'javascript': {
            'function_declaration': 'function',
            'class_declaration': 'class',
            'method_definition': 'method',
            'arrow_function': 'function',
        },
        'go': {
            'function_declaration': 'function',
            'method_declaration': 'method',
            'type_spec': 'type',  # type_spec contains the name directly
        },
        'rust': {
            'function_item': 'function',
            'impl_item': 'impl',
            'struct_item': 'struct',
            'enum_item': 'enum',
            'trait_item': 'trait',
        },
        'java': {
            'method_declaration': 'method',
            'class_declaration': 'class',
            'interface_declaration': 'interface',
        },
        'c': {
            'function_definition': 'function',
            'struct_specifier': 'struct',
        },
        'cpp': {
            'function_definition': 'function',
            'class_specifier': 'class',
            'struct_specifier': 'struct',
        },
        'ruby': {
            'method': 'method',
            'class': 'class',
            'module': 'module',
        },
        'php': {
            'function_definition': 'function',
            'method_declaration': 'method',
            'class_declaration': 'class',
            'interface_declaration': 'interface',
            'trait_declaration': 'trait',
        },
        'swift': {
            'function_declaration': 'function',
            'class_declaration': 'class',
            'struct_declaration': 'struct',
            'protocol_declaration': 'protocol',
        },
        'kotlin': {
            'function_declaration': 'function',
            'class_declaration': 'class',
            'object_declaration': 'object',
            'interface_declaration': 'interface',
        },
        'scala': {
            'function_definition': 'function',
            'class_definition': 'class',
            'object_definition': 'object',
            'trait_definition': 'trait',
        },
        'bash': {
            'function_definition': 'function',
        },
        'lua': {
            'function_declaration': 'function',
            'function_definition': 'function',
        },
        'elixir': {
            'call': 'function',  # def/defp are calls in Elixir AST
        },
        'haskell': {
            'function': 'function',
            'data': 'data',
            'type_synonym': 'type',
        },
        'dart': {
            'function_signature': 'function',
            'class_definition': 'class',
            'method_signature': 'method',
        },
        'zig': {
            'fn_decl': 'function',
            'struct_decl': 'struct',
        },
        'csharp': {
            'method_declaration': 'method',
            'class_declaration': 'class',
            'interface_declaration': 'interface',
            'struct_declaration': 'struct',
        },
        'groovy': {
            'method_declaration': 'method',
            'class_declaration': 'class',
        },
        # Extended language support (97 languages available)
        'nim': {
            'proc_declaration': 'function',
            'func_declaration': 'function',
            'type_declaration': 'type',
        },
        'ocaml': {
            'value_definition': 'function',
            'type_definition': 'type',
        },
        'julia': {
            'function_definition': 'function',
            'struct_definition': 'struct',
            'module_definition': 'module',
        },
        'erlang': {
            'function_clause': 'function',
        },
        'r': {
            'function_definition': 'function',
        },
        'perl': {
            'subroutine_declaration_statement': 'function',
            'package_statement': 'package',
        },
        'objc': {
            'class_interface': 'class',
            'class_implementation': 'class',
            'function_definition': 'function',
            'method_definition': 'method',
        },
        'ada': {
            'subprogram_body': 'function',
            'package_declaration': 'package',
        },
        'fortran': {
            'subroutine': 'function',
            'function': 'function',
            'module': 'module',
        },
        'gleam': {
            'function': 'function',
            'type_definition': 'type',
        },
        'elm': {
            'value_declaration': 'function',
            'type_declaration': 'type',
        },
        'purescript': {
            'function': 'function',
            'type': 'type',
        },
        'd': {
            'function_declaration': 'function',
            'class_declaration': 'class',
            'struct_declaration': 'struct',
        },
        'cuda': {
            'function_definition': 'function',
        },
        'glsl': {
            'function_definition': 'function',
        },
        'hlsl': {
            'function_definition': 'function',
        },
        'odin': {
            'procedure_declaration': 'function',
        },
        'v': {
            'function_declaration': 'function',
            'struct_declaration': 'struct',
        },
        'verilog': {
            'module_declaration': 'module',
        },
        'vhdl': {
            'entity_declaration': 'entity',
        },
        'sql': {
            'create_function_statement': 'function',
        },
        'graphql': {
            'type_definition': 'type',
            'field_definition': 'field',
        },
        'protobuf': {
            'message': 'message',
            'service': 'service',
        },
        'terraform': {
            'block': 'block',
        },
        'nix': {
            'function': 'function',
        },
        'fennel': {
            'fn': 'function',
        },
        'clojure': {
            'list_lit': 'expression',  # defn, defrecord are list expressions
        },
        'cmake': {
            'function_def': 'function',
            'macro_def': 'macro',
        },
        'make': {
            'rule': 'target',
        },
    }

    # Pattern queries for framework-specific symbols (routes, tests, handlers)
    # These capture patterns that node-type extraction misses
    # Note: predicates must be inside the pattern parentheses
    PATTERN_QUERIES = {
        'javascript': """
            ; Express routes: app.get('/path', handler), router.post('/path', handler)
            (call_expression
              function: (member_expression
                object: (identifier) @_router
                property: (property_identifier) @_method
                (#match? @_method "^(get|post|put|delete|patch|options|head|all|use)$"))
              arguments: (arguments
                (string) @path)) @route

            ; Jest/Mocha tests: describe('...', fn), it('...', fn), test('...', fn)
            (call_expression
              function: (identifier) @_test_fn
              (#match? @_test_fn "^(describe|it|test|beforeEach|afterEach|beforeAll|afterAll)$")
              arguments: (arguments
                (string) @test_name)) @test

            ; Event handlers: emitter.on('event', handler)
            (call_expression
              function: (member_expression
                property: (property_identifier) @_on_method
                (#match? @_on_method "^(on|once|addEventListener|addListener)$"))
              arguments: (arguments
                (string) @event_name)) @event_handler
        """,

        'typescript': """
            ; Express routes: app.get('/path', handler), router.post('/path', handler)
            (call_expression
              function: (member_expression
                object: (identifier) @_router
                property: (property_identifier) @_method
                (#match? @_method "^(get|post|put|delete|patch|options|head|all|use)$"))
              arguments: (arguments
                (string) @path)) @route

            ; Jest/Mocha tests: describe('...', fn), it('...', fn), test('...', fn)
            (call_expression
              function: (identifier) @_test_fn
              (#match? @_test_fn "^(describe|it|test|beforeEach|afterEach|beforeAll|afterAll)$")
              arguments: (arguments
                (string) @test_name)) @test

            ; Event handlers: emitter.on('event', handler)
            (call_expression
              function: (member_expression
                property: (property_identifier) @_on_method
                (#match? @_on_method "^(on|once|addEventListener|addListener)$"))
              arguments: (arguments
                (string) @event_name)) @event_handler
        """,

        'python': """
            ; Flask/FastAPI decorators: @app.route('/path'), @router.get('/path')
            (decorated_definition
              (decorator
                (call
                  function: (attribute
                    attribute: (identifier) @_method
                    (#match? @_method "^(route|get|post|put|delete|patch|options|head)$"))
                  arguments: (argument_list
                    (string) @path)))
              definition: (_) @_func) @route

            ; pytest tests: def test_something()
            (function_definition
              name: (identifier) @test_name
              (#match? @test_name "^test_")) @test
        """,
    }

    def __init__(self):
        self._parsers = {}
        self._queries = {}  # Cache compiled queries

    def get_parser(self, language: str):
        """Get or create a parser for the given language."""
        if not TREE_SITTER_AVAILABLE:
            return None

        ts_lang = self.LANG_MAP.get(language)
        if not ts_lang:
            return None

        if ts_lang not in self._parsers:
            lang_obj = get_ts_language(ts_lang)
            if not lang_obj:
                return None
            try:
                parser = Parser(lang_obj)
                self._parsers[ts_lang] = parser
            except Exception:
                return None

        return self._parsers.get(ts_lang)

    def get_query(self, language: str):
        """Get or create a compiled query for pattern matching."""
        if not TREE_SITTER_AVAILABLE:
            return None

        ts_lang = self.LANG_MAP.get(language)
        if not ts_lang:
            return None

        if ts_lang not in self._queries:
            query_str = self.PATTERN_QUERIES.get(ts_lang)
            if not query_str:
                return None
            lang_obj = get_ts_language(ts_lang)
            if not lang_obj:
                return None
            try:
                # Use Query() constructor (new API)
                query = Query(lang_obj, query_str)
                self._queries[ts_lang] = query
            except Exception:
                return None

        return self._queries.get(ts_lang)

    def _run_pattern_queries(self, tree, source: bytes, language: str) -> list['OutlineSymbol']:
        """Run pattern queries to extract framework-specific symbols."""
        query = self.get_query(language)
        if not query:
            return []

        symbols = []
        try:
            # Use QueryCursor for executing queries (new API)
            cursor = QueryCursor(query)
            captures = cursor.captures(tree.root_node)
        except Exception:
            return []

        # Group captures by their pattern type (@route, @test, @event_handler)
        # The captures dict has capture name as key and list of nodes as value
        for capture_name, nodes in captures.items():
            if capture_name.startswith('_'):
                # Skip internal captures (prefixed with _)
                continue

            for node in nodes:
                # Determine symbol kind and name based on capture type
                if capture_name == 'route':
                    # Find the path capture within this match
                    path_node = None
                    method_node = None
                    for child in node.children:
                        if child.type == 'member_expression':
                            for subchild in child.children:
                                if subchild.type == 'property_identifier':
                                    method_node = subchild
                        elif child.type == 'arguments':
                            for subchild in child.children:
                                if subchild.type == 'string':
                                    path_node = subchild
                                    break

                    if path_node and method_node:
                        method = source[method_node.start_byte:method_node.end_byte].decode('utf-8', errors='replace').upper()
                        path = source[path_node.start_byte:path_node.end_byte].decode('utf-8', errors='replace').strip('"\'')
                        name = f"{method} {path}"
                        symbols.append(OutlineSymbol(
                            name=name,
                            kind='route',
                            start_line=node.start_point[0] + 1,
                            end_line=node.end_point[0] + 1,
                            signature=source[node.start_byte:min(node.end_byte, node.start_byte + 80)].decode('utf-8', errors='replace').strip(),
                            children=[]
                        ))

                elif capture_name == 'test':
                    # Extract test name from the string argument
                    test_name_node = None
                    test_fn = None
                    for child in node.children:
                        if child.type == 'identifier':
                            test_fn = source[child.start_byte:child.end_byte].decode('utf-8', errors='replace')
                        elif child.type == 'arguments':
                            for subchild in child.children:
                                if subchild.type == 'string':
                                    test_name_node = subchild
                                    break

                    if test_name_node:
                        test_name = source[test_name_node.start_byte:test_name_node.end_byte].decode('utf-8', errors='replace').strip('"\'')
                        kind = 'test' if test_fn in ('it', 'test') else 'test_suite' if test_fn == 'describe' else 'test_hook'
                        name = f"{test_fn}: {test_name}"
                        symbols.append(OutlineSymbol(
                            name=name,
                            kind=kind,
                            start_line=node.start_point[0] + 1,
                            end_line=node.end_point[0] + 1,
                            signature=source[node.start_byte:min(node.end_byte, node.start_byte + 80)].decode('utf-8', errors='replace').strip(),
                            children=[]
                        ))

                elif capture_name == 'event_handler':
                    # Extract event name
                    event_name_node = None
                    for child in node.children:
                        if child.type == 'arguments':
                            for subchild in child.children:
                                if subchild.type == 'string':
                                    event_name_node = subchild
                                    break

                    if event_name_node:
                        event_name = source[event_name_node.start_byte:event_name_node.end_byte].decode('utf-8', errors='replace').strip('"\'')
                        name = f"on: {event_name}"
                        symbols.append(OutlineSymbol(
                            name=name,
                            kind='event',
                            start_line=node.start_point[0] + 1,
                            end_line=node.end_point[0] + 1,
                            signature=source[node.start_byte:min(node.end_byte, node.start_byte + 80)].decode('utf-8', errors='replace').strip(),
                            children=[]
                        ))

                elif capture_name == 'test_name':
                    # Python pytest: def test_something()
                    name = source[node.start_byte:node.end_byte].decode('utf-8', errors='replace')
                    # Get parent function for line info
                    parent = node.parent
                    if parent and parent.type == 'function_definition':
                        symbols.append(OutlineSymbol(
                            name=name,
                            kind='test',
                            start_line=parent.start_point[0] + 1,
                            end_line=parent.end_point[0] + 1,
                            signature=source[parent.start_byte:min(parent.end_byte, parent.start_byte + 80)].decode('utf-8', errors='replace').strip(),
                            children=[]
                        ))

                elif capture_name == 'path':
                    # Python Flask/FastAPI route - need to get parent decorated_definition
                    parent = node
                    while parent and parent.type != 'decorated_definition':
                        parent = parent.parent
                    if parent:
                        path = source[node.start_byte:node.end_byte].decode('utf-8', errors='replace').strip('"\'')
                        # Try to find method from decorator
                        # Supports both FastAPI (.get/.post) and Flask (methods=['GET'])
                        method = 'GET'  # Default to GET (most common)
                        for child in parent.children:
                            if child.type == 'decorator':
                                dec_text = source[child.start_byte:child.end_byte].decode('utf-8', errors='replace')
                                dec_lower = dec_text.lower()
                                # FastAPI style: @app.post("/path")
                                for m in ['get', 'post', 'put', 'delete', 'patch']:
                                    if f'.{m}(' in dec_lower:
                                        method = m.upper()
                                        break
                                # Flask style: @app.route("/path", methods=['POST'])
                                if "methods=" in dec_lower or "methods =" in dec_lower:
                                    for m in ['get', 'post', 'put', 'delete', 'patch']:
                                        if f"'{m}'" in dec_lower or f'"{m}"' in dec_lower:
                                            method = m.upper()
                                            break
                        name = f"{method} {path}"
                        symbols.append(OutlineSymbol(
                            name=name,
                            kind='route',
                            start_line=parent.start_point[0] + 1,
                            end_line=parent.end_point[0] + 1,
                            signature=source[parent.start_byte:min(parent.end_byte, parent.start_byte + 80)].decode('utf-8', errors='replace').strip(),
                            children=[]
                        ))

        return symbols

    def _get_node_name(self, node, source_bytes: bytes, language: str) -> str | None:
        """Extract the name of a symbol node."""
        # Different languages have different name child node types
        name_types = {
            # Core languages
            'python': ['identifier', 'name'],
            'typescript': ['identifier', 'property_identifier', 'type_identifier'],
            'javascript': ['identifier', 'property_identifier', 'type_identifier'],
            'go': ['identifier', 'field_identifier', 'type_identifier'],
            'rust': ['identifier', 'type_identifier'],
            'java': ['identifier'],
            'c': ['identifier', 'type_identifier'],
            'cpp': ['identifier', 'type_identifier'],
            'ruby': ['identifier', 'constant'],
            'php': ['name'],
            'swift': ['identifier', 'simple_identifier', 'type_identifier'],
            'kotlin': ['simple_identifier', 'identifier', 'type_identifier'],
            'scala': ['identifier'],
            'csharp': ['identifier'],
            'groovy': ['identifier'],
            'bash': ['word'],
            'lua': ['identifier', 'name'],
            'elixir': ['identifier', 'atom'],
            'haskell': ['identifier', 'variable', 'constructor'],
            'dart': ['identifier', 'type_identifier'],
            'zig': ['identifier'],
            # Extended languages
            'nim': ['identifier', 'symbol'],
            'ocaml': ['identifier', 'value_name', 'type_constructor'],
            'julia': ['identifier'],
            'erlang': ['atom', 'variable'],
            'r': ['identifier'],
            'perl': ['identifier', 'scalar'],
            'objc': ['identifier', 'type_identifier'],
            'ada': ['identifier'],
            'fortran': ['identifier', 'name'],
            'gleam': ['identifier', 'type_identifier'],
            'elm': ['identifier', 'lower_case_identifier', 'type_identifier'],
            'purescript': ['identifier', 'type_identifier'],
            'd': ['identifier', 'type_identifier'],
            'cuda': ['identifier', 'type_identifier'],
            'glsl': ['identifier', 'type_identifier'],
            'hlsl': ['identifier', 'type_identifier'],
            'odin': ['identifier'],
            'v': ['identifier', 'type_identifier'],
            'verilog': ['identifier', 'module_identifier'],
            'vhdl': ['identifier'],
            'sql': ['identifier'],
            'graphql': ['name'],
            'terraform': ['identifier'],
            'nix': ['identifier'],
            'fennel': ['symbol'],
            'clojure': ['sym_lit'],
            'cmake': ['identifier'],
            'make': ['word'],
        }

        types_to_check = name_types.get(language, ['identifier', 'name'])

        # First, check direct children
        for child in node.children:
            if child.type in types_to_check:
                return source_bytes[child.start_byte:child.end_byte].decode('utf-8', errors='replace')

        # If not found, search one level deeper (handles C/C++/CUDA function_declarator pattern)
        for child in node.children:
            for grandchild in child.children:
                if grandchild.type in types_to_check:
                    return source_bytes[grandchild.start_byte:grandchild.end_byte].decode('utf-8', errors='replace')

        return None

    # Body node types by language - where the actual code starts
    BODY_NODES = {
        'python': {'block'},
        'javascript': {'statement_block'},
        'typescript': {'statement_block'},
        'go': {'block'},
        'rust': {'block'},
        'java': {'block'},
        'c': {'compound_statement'},
        'cpp': {'compound_statement'},
        'ruby': {'body_statement', 'do_block'},
        'php': {'compound_statement'},
        'swift': {'function_body'},
        'kotlin': {'function_body'},
        'scala': {'block'},
        'bash': {'compound_statement'},
        'lua': {'block'},
    }

    def _get_signature(self, node, source_bytes: bytes, language: str = 'python', max_len: int = 500) -> str:
        """Extract full signature including multi-line parameters.

        Finds the body node and takes everything from the function start
        to just before the body begins.
        """
        body_types = self.BODY_NODES.get(language, {'block', 'statement_block', 'compound_statement'})

        # Find body node
        body_start = None
        for child in node.children:
            if child.type in body_types:
                body_start = child.start_byte
                break

        if body_start is not None:
            # Take everything before the body
            sig_bytes = source_bytes[node.start_byte:body_start]
        else:
            # Fallback: take first line up to max_len
            start = node.start_byte
            end = start
            while end < len(source_bytes) and end < start + max_len:
                if source_bytes[end:end+1] == b'\n':
                    break
                end += 1
            sig_bytes = source_bytes[start:end]

        # Clean up: decode, normalize whitespace, remove trailing : or {
        sig = sig_bytes.decode('utf-8', errors='replace')
        # Collapse multiple whitespace/newlines into single space
        sig = ' '.join(sig.split())
        # Remove trailing : or { (Python colon, C-style brace)
        sig = sig.rstrip(': {')
        return sig.strip()

    def parse_file(self, file_path: str, language: str) -> list[OutlineSymbol]:
        """Parse a file and return its outline."""
        try:
            with open(file_path, 'rb') as f:
                source = f.read()
        except OSError:
            return []
        return self.parse_content(source, language)

    def parse_content(self, source: bytes, language: str) -> list[OutlineSymbol]:
        """Parse content bytes and return outline. Use this with LRU cache."""
        parser = self.get_parser(language)
        if not parser:
            return []

        try:
            tree = parser.parse(source)
        except Exception:
            return []

        symbols = []
        symbol_types = self.SYMBOL_NODES.get(language, {})

        def walk(node, depth=0):
            if node.type in symbol_types:
                name = self._get_node_name(node, source, language)
                if name:
                    symbol = OutlineSymbol(
                        name=name,
                        kind=symbol_types[node.type],
                        start_line=node.start_point[0] + 1,  # 1-indexed
                        end_line=node.end_point[0] + 1,
                        signature=self._get_signature(node, source, language),
                        children=[]
                    )
                    symbols.append(symbol)

            for child in node.children:
                walk(child, depth + 1)

        walk(tree.root_node)

        # Run pattern queries for framework-specific symbols (routes, tests, handlers)
        pattern_symbols = self._run_pattern_queries(tree, source, language)
        symbols.extend(pattern_symbols)

        return symbols


# Global outline parser instance
outline_parser = OutlineParser()


class CodebaseIndex:
    """Single codebase index with inverted index, file metadata, and change log."""

    # Aggressive extension mapping - index everything, outline where tree-sitter available
    # Tier 1: Full tree-sitter support (rich outlines)
    # Tier 2: Basic indexing only (tokenization works, no structural outline)
    EXTENSIONS = {
        # === TIER 1: Tree-sitter supported (rich structural outline) ===
        # Core systems
        '.py': 'python',
        '.js': 'javascript', '.jsx': 'javascript', '.mjs': 'javascript', '.cjs': 'javascript',
        '.ts': 'typescript', '.tsx': 'typescript', '.mts': 'typescript',
        '.go': 'go',
        '.rs': 'rust',
        '.c': 'c', '.h': 'c',
        '.cpp': 'cpp', '.hpp': 'cpp', '.cc': 'cpp', '.cxx': 'cpp', '.hxx': 'cpp',
        '.java': 'java',
        '.cs': 'csharp',
        '.rb': 'ruby',
        '.php': 'php',
        '.swift': 'swift',
        '.kt': 'kotlin', '.kts': 'kotlin',
        '.scala': 'scala', '.sc': 'scala',
        '.lua': 'lua',
        '.ex': 'elixir', '.exs': 'elixir',
        '.hs': 'haskell', '.lhs': 'haskell',
        '.sh': 'bash', '.bash': 'bash', '.zsh': 'bash',
        '.sql': 'sql',
        '.html': 'html', '.htm': 'html',
        '.css': 'css', '.scss': 'scss', '.sass': 'scss', '.less': 'css',
        '.json': 'json', '.jsonc': 'json',
        '.yaml': 'yaml', '.yml': 'yaml',
        '.toml': 'toml',
        '.md': 'markdown', '.mdx': 'markdown',
        '.xml': 'xml', '.xsl': 'xml', '.xslt': 'xml',
        '.vue': 'vue',
        '.svelte': 'svelte',

        # === TIER 2: Indexing only (no tree-sitter, but still searchable) ===
        # JVM ecosystem
        '.groovy': 'groovy', '.gradle': 'groovy',
        '.clj': 'clojure', '.cljs': 'clojure', '.cljc': 'clojure', '.edn': 'clojure',
        # .NET ecosystem
        '.fs': 'fsharp', '.fsx': 'fsharp', '.fsi': 'fsharp',
        '.vb': 'vb',
        # Systems
        '.zig': 'zig',
        '.nim': 'nim',
        '.d': 'd',
        '.ada': 'ada', '.adb': 'ada', '.ads': 'ada',
        '.f90': 'fortran', '.f95': 'fortran', '.f03': 'fortran', '.f': 'fortran',
        '.cob': 'cobol', '.cbl': 'cobol',
        # Scripting
        '.pl': 'perl', '.pm': 'perl',
        '.r': 'r', '.R': 'r',
        '.jl': 'julia',
        '.tcl': 'tcl',
        '.awk': 'awk',
        '.sed': 'sed',
        # Functional
        '.ml': 'ocaml', '.mli': 'ocaml',
        '.erl': 'erlang', '.hrl': 'erlang',
        '.elm': 'elm',
        '.purs': 'purescript',
        '.rkt': 'racket',
        '.scm': 'scheme', '.ss': 'scheme',
        '.lisp': 'lisp', '.cl': 'lisp', '.el': 'elisp',
        # Web/mobile
        '.dart': 'dart',
        '.coffee': 'coffeescript',
        '.slim': 'slim',
        '.haml': 'haml',
        '.pug': 'pug', '.jade': 'pug',
        '.ejs': 'ejs',
        '.hbs': 'handlebars', '.handlebars': 'handlebars',
        '.mustache': 'mustache',
        '.twig': 'twig',
        '.liquid': 'liquid',
        # Data/config
        '.graphql': 'graphql', '.gql': 'graphql',
        '.proto': 'protobuf',
        '.thrift': 'thrift',
        '.avsc': 'avro',
        '.tf': 'terraform', '.tfvars': 'terraform',
        '.hcl': 'hcl',
        '.nix': 'nix',
        '.dhall': 'dhall',
        '.ini': 'ini', '.cfg': 'ini', '.conf': 'ini',
        '.env': 'dotenv',
        '.properties': 'properties',
        # DevOps/CI
        '.dockerfile': 'dockerfile',
        '.containerfile': 'dockerfile',
        '.jenkinsfile': 'groovy',
        '.makefile': 'make', '.mk': 'make',
        '.cmake': 'cmake',
        # Documentation
        '.rst': 'rst',
        '.adoc': 'asciidoc', '.asciidoc': 'asciidoc',
        '.tex': 'latex', '.latex': 'latex',
        '.org': 'org',
        # Misc
        '.diff': 'diff', '.patch': 'diff',
        '.log': 'log',
        '.csv': 'csv',
        '.tsv': 'tsv',
    }

    IGNORE_DIRS = {
        'node_modules', '.git', '__pycache__', 'target', 'dist',
        'build', '.next', '.nuxt', 'vendor', 'venv', '.venv',
        '.idea', '.vscode', 'coverage', '.cache', 'repos'
    }

    def __init__(self, root: str, name: str = 'local'):
        self.root = Path(root).resolve()
        self.name = name
        self.session_start = int(time.time())
        self.last_indexed = int(time.time())

        # Core data structures
        self.inverted_index: dict[str, list[Location]] = defaultdict(list)
        self.files: dict[str, FileMeta] = {}
        self.changes: list[ChangeRecord] = []

        # GL-050: Pre-computed symbol outlines per file (stored at index time)
        self.file_outlines: dict[str, list] = {}  # rel_path -> [OutlineSymbol, ...]

        # Dependency graph
        self.deps_outgoing: dict[str, list[str]] = defaultdict(list)
        self.deps_incoming: dict[str, list[str]] = defaultdict(list)

        # Thread safety
        self.lock = threading.RLock()

        # =================================================================
        # O(1) OPTIMIZATION: Three-Store Architecture (Session 41)
        # See: .context/O1-OPTIMIZATION_ARCH.md
        # =================================================================

        # Task 1.1: File Registry - bidirectional file_id <-> path mapping
        # Enables compact integer IDs in token_index instead of full path strings
        self.id_to_path: dict[int, str] = {}      # file_id -> rel_path
        self.path_to_id: dict[str, int] = {}      # rel_path -> file_id
        self.next_file_id: int = 1                # Auto-increment counter

        # Task 1.2: Token Index - O(1) lookup, lean pointers only
        # CRITICAL: Use (file_id, line) tuples, NOT Location objects
        # Memory: ~16 bytes/entry vs ~765 bytes for Location objects
        self.token_index: dict[str, list[tuple[int, int]]] = defaultdict(list)

        # Task 1.3: Content Store - file content stored ONCE per file
        # Lookup by file_id, then index by line number
        self.content_store: dict[int, list[str]] = {}  # file_id -> [line0, line1, ...]

        # Task 1.4: Metadata Store - symbol context + enrichment cache
        # Key: (file_id, line) -> symbol metadata + cached domain/terms
        self.metadata_store: dict[tuple[int, int], dict] = {}

        # Batch Optimization: In-memory mirrors of Redis data (no Redis at query time)
        # Populated when Redis is written, queried from Python memory
        self.file_tags_store: dict[int, list[str]] = {}  # file_id -> [tags]
        self.symbol_domains_store: dict[tuple[int, str], str] = {}  # (file_id, symbol) -> domain

    def get_language(self, path: Path) -> str:
        return self.EXTENSIONS.get(path.suffix.lower(), 'unknown')

    def should_index(self, path: Path) -> bool:
        """Check if file should be indexed."""
        # Use relative path from index root for ignore checks
        try:
            rel_path = path.relative_to(self.root)
            parts = rel_path.parts
        except ValueError:
            parts = path.parts

        if any(part.startswith('.') for part in parts):
            return False
        if any(ignored in parts for ignored in self.IGNORE_DIRS):
            return False
        return path.suffix.lower() in self.EXTENSIONS

    def tokenize(self, content: str) -> list[tuple[str, int, int]]:
        """Extract tokens with their positions."""
        tokens = []
        for line_num, line in enumerate(content.split('\n'), 1):
            for match in re.finditer(r'[a-zA-Z_][a-zA-Z0-9_]*', line):
                token = match.group()
                if len(token) >= 2:
                    tokens.append((token, line_num, match.start()))
        return tokens

    def _find_enclosing_symbol(self, line_num: int, symbols: list) -> dict | None:
        """GL-046.2: Find the most specific (smallest) symbol containing a line.

        Returns dict with symbol info AND parent info for hierarchy.
        GL-047.8: Added parent tracking for AI-readable context.
        """
        # Find all symbols containing this line
        candidates = []
        for sym in symbols:
            if sym.start_line <= line_num <= sym.end_line:
                candidates.append(sym)
        if not candidates:
            return None

        # Sort by range size (smallest first = most specific)
        candidates.sort(key=lambda s: s.end_line - s.start_line)

        # Best = smallest (innermost)
        best = candidates[0]

        # Parent = next smallest that contains best (if any)
        parent = None
        for sym in candidates[1:]:
            if sym.start_line <= best.start_line and best.end_line <= sym.end_line:
                parent = sym
                break

        result = {
            'name': best.name,
            'kind': best.kind,
            'start_line': best.start_line,
            'end_line': best.end_line,
            'signature': best.signature
        }

        # Add parent info if exists
        if parent:
            result['parent_name'] = parent.name
            result['parent_signature'] = parent.signature
            result['parent_start'] = parent.start_line
            result['parent_end'] = parent.end_line

        return result

    def index_file(self, path: Path) -> bool:
        """Index a single file."""
        try:
            content = path.read_text(encoding='utf-8', errors='ignore')
            stat = path.stat()
            mtime = int(stat.st_mtime)

            rel_path = str(path.relative_to(self.root))
            language = self.get_language(path)
            content_hash = hashlib.md5(content.encode()).hexdigest()[:16]

            # GL-046.2: Parse outline to get symbol info for each token location
            symbols = []
            if TREE_SITTER_AVAILABLE and language in outline_parser.LANG_MAP:
                try:
                    symbols = outline_parser.parse_file(str(path), language)
                except Exception:
                    pass  # Fall back to no symbol info

            # GL-050: Store outline for O(1) lookup during egrep
            self.file_outlines[rel_path] = symbols

            # GL-047: Pre-compute AC dense tags for each symbol body
            lines = content.split('\n')
            symbol_tags = {}  # (start_line, end_line) -> dense_tags
            if AC_MATCHER.is_available and symbols:
                for sym in symbols:
                    # Extract symbol body text
                    start = max(0, sym.start_line - 1)
                    end = min(len(lines), sym.end_line)
                    body = '\n'.join(lines[start:end])
                    # Compute dense tags (threshold=2 for meaningful density)
                    dense = AC_MATCHER.get_dense_tags(body, threshold=2)
                    if dense:
                        symbol_tags[(sym.start_line, sym.end_line)] = dense[:3]  # Top 3 tags
                # Debug: log how many symbols got tags
                if symbol_tags and 'indexer' in rel_path:
                    print(f"[GL-047] {rel_path}: {len(symbol_tags)} symbols with AC tags")

            with self.lock:
                if rel_path in self.files:
                    if self.files[rel_path].content_hash == content_hash:
                        return False
                    self._remove_file_from_index(rel_path)

                self.files[rel_path] = FileMeta(
                    path=rel_path,
                    mtime=mtime,
                    size=stat.st_size,
                    language=language,
                    content_hash=content_hash
                )

                # =================================================================
                # O(1) OPTIMIZATION: Populate Three-Store (Session 41)
                # =================================================================

                # Task 2.1: Add to file_registry (reuse existing ID or assign new)
                if rel_path in self.path_to_id:
                    file_id = self.path_to_id[rel_path]
                else:
                    file_id = self.next_file_id
                    self.next_file_id += 1
                    self.id_to_path[file_id] = rel_path
                    self.path_to_id[rel_path] = file_id

                # Task 2.2: Populate content_store (lines stored once per file)
                self.content_store[file_id] = lines  # 'lines' already split above

                for token, line_num, col in self.tokenize(content):
                    # Get line content (capped at 200 chars for memory efficiency)
                    line_content = lines[line_num - 1][:200] if line_num <= len(lines) else ''

                    # GL-046.2: Find enclosing symbol for this token
                    enclosing = self._find_enclosing_symbol(line_num, symbols)

                    # GL-047: Look up pre-computed AC tags for this symbol
                    ac_tags = None
                    if enclosing and symbol_tags:
                        key = (enclosing['start_line'], enclosing['end_line'])
                        ac_tags = symbol_tags.get(key)
                        # Debug: first match
                        if ac_tags and 'indexer' in rel_path and not hasattr(self, '_ac_debug_done'):
                            print(f"[GL-047 Debug] Found tags for {enclosing['name']} {key}: {ac_tags}")
                            print(f"[GL-047 Debug] ac_tags type: {type(ac_tags)}, value: {ac_tags}")
                            self._ac_debug_done = True

                    loc = Location(
                        file=rel_path,
                        line=line_num,
                        col=col,
                        symbol_type='token',
                        mtime=mtime,
                        content=line_content.strip(),  # GL-046: Store content for O(1) display
                        symbol=enclosing['name'] if enclosing else None,  # GL-046.2
                        symbol_kind=enclosing['kind'] if enclosing else None,  # GL-046.2
                        start_line=enclosing['start_line'] if enclosing else None,  # GL-047.7
                        end_line=enclosing['end_line'] if enclosing else None,  # GL-046.2
                        signature=enclosing['signature'] if enclosing else None,  # GL-047.7
                        # GL-047.8: Parent hierarchy
                        parent_name=enclosing.get('parent_name') if enclosing else None,
                        parent_signature=enclosing.get('parent_signature') if enclosing else None,
                        parent_start=enclosing.get('parent_start') if enclosing else None,
                        parent_end=enclosing.get('parent_end') if enclosing else None,
                        tags=ac_tags  # GL-047: Pre-computed AC dense tags
                    )
                    self.inverted_index[token].append(loc)
                    lower = token.lower()
                    if lower != token:
                        self.inverted_index[lower].append(loc)

                    # Task 2.3: Populate token_index with lean (file_id, line) tuples
                    self.token_index[token].append((file_id, line_num))
                    if lower != token:
                        self.token_index[lower].append((file_id, line_num))

                    # Task 2.4: Populate metadata_store with symbol context
                    # Only store if we have enclosing symbol (avoid storing for every token)
                    meta_key = (file_id, line_num)
                    if meta_key not in self.metadata_store:
                        self.metadata_store[meta_key] = {
                            'symbol': enclosing['name'] if enclosing else None,
                            'symbol_kind': enclosing['kind'] if enclosing else None,
                            'start_line': enclosing['start_line'] if enclosing else None,
                            'end_line': enclosing['end_line'] if enclosing else None,
                            'signature': enclosing['signature'] if enclosing else None,
                            'parent_name': enclosing.get('parent_name') if enclosing else None,
                            'tags': ac_tags,
                            'mtime': mtime,
                            # Task 2.4b: domain/terms populated later by learning
                            'domain': None,
                            'terms': None
                        }

                self._extract_deps(path, content, language)
                self.last_indexed = int(time.time())

            return True

        except Exception as e:
            print(f"Error indexing {path}: {e}")
            return False

    def _remove_file_from_index(self, rel_path: str):
        """Remove all entries for a file from the index."""
        # Original inverted_index cleanup
        for token, locations in list(self.inverted_index.items()):
            self.inverted_index[token] = [
                loc for loc in locations if loc.file != rel_path
            ]
            if not self.inverted_index[token]:
                del self.inverted_index[token]

        # =================================================================
        # O(1) OPTIMIZATION: Clean up Three-Store (Task 2.5)
        # =================================================================

        # Task 2.5a: Get file_id from registry
        file_id = self.path_to_id.get(rel_path)

        if file_id is not None:
            # Task 2.5b: Remove from token_index (filter by file_id)
            for token, positions in list(self.token_index.items()):
                self.token_index[token] = [
                    (fid, line) for fid, line in positions if fid != file_id
                ]
                if not self.token_index[token]:
                    del self.token_index[token]

            # Task 2.5c: Remove from content_store
            if file_id in self.content_store:
                del self.content_store[file_id]

            # Task 2.5d: Remove from metadata_store (filter by file_id)
            keys_to_remove = [k for k in self.metadata_store if k[0] == file_id]
            for key in keys_to_remove:
                del self.metadata_store[key]

            # Task 2.5e: Remove from file_registry (both dicts)
            del self.id_to_path[file_id]
            del self.path_to_id[rel_path]

        if rel_path in self.deps_outgoing:
            del self.deps_outgoing[rel_path]
        if rel_path in self.deps_incoming:
            del self.deps_incoming[rel_path]

    def _extract_deps(self, path: Path, content: str, language: str):
        """Extract import/dependency information."""
        rel_path = str(path.relative_to(self.root))
        imports = []

        if language in ('typescript', 'javascript'):
            for match in re.finditer(r'''(?:import\s+.*?from\s+|require\()['"]([^'"]+)['"]''', content):
                imports.append(match.group(1))
        elif language == 'python':
            for match in re.finditer(r'^(?:from\s+(\S+)|import\s+(\S+))', content, re.MULTILINE):
                imports.append(match.group(1) or match.group(2))
        elif language == 'rust':
            for match in re.finditer(r'^(?:use|mod)\s+([a-zA-Z_][a-zA-Z0-9_:]*)', content, re.MULTILINE):
                imports.append(match.group(1))

        if imports:
            self.deps_outgoing[rel_path] = imports
            for imp in imports:
                self.deps_incoming[imp].append(rel_path)

    def full_scan(self):
        """Scan entire codebase."""
        start = time.time()
        count = 0

        for path in self.root.rglob('*'):
            if path.is_file() and self.should_index(path) and self.index_file(path):
                count += 1

        elapsed = time.time() - start
        print(f"[{self.name}] Indexed {count} files in {elapsed:.2f}s ({len(self.inverted_index)} symbols)")

    def record_change(self, path: Path, change_type: str):
        """Record a file change."""
        try:
            rel_path = str(path.relative_to(self.root))
        except ValueError:
            rel_path = str(path)

        with self.lock:
            self.changes.append(ChangeRecord(
                file=rel_path,
                timestamp=int(time.time()),
                change_type=change_type
            ))

    def search(self, query: str, mode: str = 'recent', limit: int = 20,
               since: int = None, before: int = None, file_filter: str = None) -> list[dict]:
        """Search for a term with filename boosting and optional time/file filtering.

        Args:
            file_filter: Glob pattern to filter files (e.g., "*.py", "cli/", "indexer.py")
        """
        results = []

        with self.lock:
            if query in self.inverted_index:
                results.extend(self.inverted_index[query])
            lower = query.lower()
            if lower != query and lower in self.inverted_index:
                results.extend(self.inverted_index[lower])

        # GL-051: File pattern filtering (Unix grep parity)
        # Filter BEFORE scoring to ensure correct limit behavior
        if file_filter:
            if '*' in file_filter:
                # Glob pattern: *.py, **/*.ts, etc.
                filter_regex = file_filter.replace('.', r'\.').replace('**', '.*').replace('*', '[^/]*')
                filtered = [loc for loc in results if re.search(filter_regex, loc.file)]
            elif file_filter.endswith('/'):
                # Directory filter: cli/, services/
                filtered = [loc for loc in results if loc.file.startswith(file_filter)]
            else:
                # Filename contains: indexer.py, routes
                filtered = [loc for loc in results if file_filter in loc.file]
            results = filtered

        # Time filtering
        if since is not None or before is not None:
            filtered = []
            for loc in results:
                if since is not None and loc.mtime < since:
                    continue
                if before is not None and loc.mtime > before:
                    continue
                filtered.append(loc)
            results = filtered

        # Deduplicate by (file, line)
        seen = set()
        unique = []
        for loc in results:
            key = (loc.file, loc.line)
            if key not in seen:
                seen.add(key)
                unique.append(loc)

        # Score each result with filename boosting
        query_lower = query.lower()
        def score(loc):
            filename = loc.file.lower().split('/')[-1]  # Just the filename
            filepath = loc.file.lower()

            # Filename boost: files named after the query rank highest
            if query_lower in filename.replace('-', '').replace('_', ''):
                filename_boost = 1000
            elif query_lower in filename:
                filename_boost = 500
            elif query_lower in filepath:
                filename_boost = 100
            else:
                filename_boost = 0

            # Recency as secondary factor
            recency = loc.mtime

            return (filename_boost, recency)

        if mode == 'recent':
            unique.sort(key=score, reverse=True)
        else:
            unique.sort(key=lambda x: x.file)

        return [asdict(loc) for loc in unique[:limit]]

    # =================================================================
    # O(1) OPTIMIZATION: New search using token_index (Task 3.1)
    # =================================================================

    def search_o1(self, query: str, limit: int = 100, file_filter: str = None) -> list[dict]:
        """O(1) search using token_index - returns results for enrichment.

        This is the fast path for literal searches. Returns raw results
        that will be enriched by enrich_result() before display.

        Args:
            query: Token to search for
            limit: Max results to return
            file_filter: Optional glob pattern to filter files

        Returns:
            List of result dicts with: file, line, content, mtime
            (Symbol metadata added later by enrich_result)
        """
        results = []

        with self.lock:
            # Task 3.1a: O(1) lookup from token_index
            positions = self.token_index.get(query, [])

            # Also check lowercase version
            lower = query.lower()
            if lower != query:
                positions = positions + self.token_index.get(lower, [])

            # Task 3.1b: Build result dicts from three stores
            seen = set()
            for file_id, line_num in positions:
                # Deduplicate by (file_id, line)
                key = (file_id, line_num)
                if key in seen:
                    continue
                seen.add(key)

                # Task 3.1c: Get file path from registry
                file_path = self.id_to_path.get(file_id)
                if not file_path:
                    continue

                # Apply file filter if specified
                if file_filter:
                    if '*' in file_filter:
                        filter_regex = file_filter.replace('.', r'\.').replace('**', '.*').replace('*', '[^/]*')
                        if not re.search(filter_regex, file_path):
                            continue
                    elif file_filter.endswith('/'):
                        if not file_path.startswith(file_filter):
                            continue
                    else:
                        if file_filter not in file_path:
                            continue

                # Get content from content_store
                lines = self.content_store.get(file_id, [])
                content = lines[line_num - 1][:200] if 0 < line_num <= len(lines) else ''

                # Get mtime from files metadata
                file_meta = self.files.get(file_path)
                mtime = file_meta.mtime if file_meta else 0

                # Task 3.1d: Build result dict (same format as old search)
                results.append({
                    'file': file_path,
                    'line': line_num,
                    'content': content.strip(),
                    'mtime': mtime,
                    # Include file_id for metadata lookup during enrichment
                    '_file_id': file_id
                })

                if len(results) >= limit:
                    break

        # Score by filename boost (same as original search)
        query_lower = query.lower()
        def score(r):
            filename = r['file'].lower().split('/')[-1]
            filepath = r['file'].lower()
            if query_lower in filename.replace('-', '').replace('_', ''):
                return (1000, r['mtime'])
            elif query_lower in filename:
                return (500, r['mtime'])
            elif query_lower in filepath:
                return (100, r['mtime'])
            return (0, r['mtime'])

        results.sort(key=score, reverse=True)
        return results[:limit]

    def search_multi(self, terms: list[str], mode: str = 'recent', limit: int = 20,
                     since: int = None, before: int = None, file_filter: str = None) -> list[dict]:
        """Search for multiple terms, rank by density."""
        all_results = []
        for term in terms:
            all_results.extend(self.search(term, mode, limit * 2, since=since, before=before, file_filter=file_filter))

        file_scores: dict[str, tuple[int, int]] = {}
        for loc in all_results:
            if loc['file'] not in file_scores:
                file_scores[loc['file']] = (0, loc['mtime'])
            count, mtime = file_scores[loc['file']]
            file_scores[loc['file']] = (count + 1, max(mtime, loc['mtime']))

        sorted_files = sorted(
            file_scores.items(),
            key=lambda x: (x[1][0], x[1][1]),
            reverse=True
        )

        top_files = {f for f, _ in sorted_files[:limit]}
        return [loc for loc in all_results if loc['file'] in top_files][:limit]

    def changes_since(self, since: int) -> list[dict]:
        """Get changes since timestamp."""
        with self.lock:
            return [asdict(c) for c in self.changes if c.timestamp >= since]

    def list_files(self, pattern: str | None = None, mode: str = 'recent', limit: int = 50) -> list[dict]:
        """List files matching pattern."""
        with self.lock:
            results = list(self.files.values())

        if pattern:
            if '*' in pattern:
                regex = pattern.replace('.', r'\.').replace('*', '.*')
                results = [f for f in results if re.search(regex, f.path)]
            else:
                results = [f for f in results if pattern in f.path]

        if mode == 'recent':
            results.sort(key=lambda x: x.mtime, reverse=True)
        else:
            results.sort(key=lambda x: x.path)

        return [asdict(f) for f in results[:limit]]

    def get_stats(self) -> dict:
        """Get index statistics."""
        return {
            'name': self.name,
            'root': str(self.root),
            'files': len(self.files),
            'symbols': len(self.inverted_index),
            'last_indexed': self.last_indexed
        }

    def clear(self):
        """Clear the index."""
        with self.lock:
            self.inverted_index.clear()
            self.files.clear()
            self.changes.clear()
            self.deps_outgoing.clear()
            self.deps_incoming.clear()


# ============================================================================
# Index Manager - Manages local + repo indexes
# ============================================================================

class IndexManager:
    """Manages multiple isolated indexes: local project_id + knowledge repos.

    Supports two modes:
    - Legacy mode: Single local index from CODEBASE_ROOT
    - Global mode: Multiple project_id indexes from /config/projects.json
    """

    def __init__(self, local_root: str, repos_root: str, config_dir: str = None, indexes_dir: str = None):
        self.local_root = Path(local_root).resolve() if local_root else None
        self.repos_root = Path(repos_root).resolve()
        self.config_dir = Path(config_dir) if config_dir else None
        self.indexes_dir = Path(indexes_dir) if indexes_dir else None
        self.user_home = os.environ.get('USER_HOME', '/home')

        # Create repos directory if needed
        self.repos_root.mkdir(parents=True, exist_ok=True)

        # Determine mode
        self.global_mode = self.config_dir is not None and (self.config_dir / 'projects.json').exists()

        # Local index (legacy mode - your project_id)
        self.local: CodebaseIndex | None = None
        if self.local_root and self.local_root.exists():
            self.local = CodebaseIndex(str(self.local_root), name='local')

        # Project indexes (global mode - multiple projects)
        self.projects: dict[str, CodebaseIndex] = {}

        # Repo indexes (knowledge repos)
        self.repos: dict[str, CodebaseIndex] = {}

        # File watchers
        self.observers: dict[str, Observer] = {}

        self.lock = threading.RLock()

    def get_local(self, project_id: str = None) -> CodebaseIndex | None:
        """Get the appropriate index for a query.

        In global mode, returns the project_id index if project_id is provided.
        In legacy mode, returns the single local index.

        IMPORTANT: In global mode, if project_id is provided but not found,
        returns None to prevent cross-project_id data leakage.
        In legacy mode, always falls back to the single local index.
        """
        if project_id:
            # Project ID provided - check registered projects first
            if project_id in self.projects:
                return self.projects[project_id]
            # In legacy mode (single index), fall back to local
            # This is safe because there's only one index anyway
            if self.local:
                return self.local
            # In global mode, don't fall back to wrong index
            return None

        # No project_id ID - legacy mode fallback
        if self.local:
            return self.local
        # Return first project_id if available (legacy compatibility)
        if self.projects:
            return next(iter(self.projects.values()))
        return None

    def init_local(self):
        """Initialize and scan local index (legacy mode)."""
        if self.local:
            print(f"Initializing local index: {self.local_root}")
            self.local.full_scan()
            self._start_watcher('local', self.local)
        elif self.global_mode:
            print("Global mode: No single local index, using project_id indexes")
            self._load_projects()

    def _load_projects(self):
        """Load all registered projects from config."""
        if not self.config_dir:
            return

        projects_file = self.config_dir / 'projects.json'
        if not projects_file.exists():
            print("No projects.json found")
            return

        try:
            projects = json.loads(projects_file.read_text())
            print(f"Loading {len(projects)} registered projects...")

            for proj in projects:
                self._load_project(proj['id'], proj['name'], proj['path'])
        except Exception as e:
            print(f"Error loading projects: {e}")

    def _load_project(self, project_id: str, name: str, path: str) -> CodebaseIndex | None:
        """Load or create index for a project_id."""
        # Convert path to container path (user's home is mounted at /userhome)
        container_path = path.replace(self.user_home, '/userhome')

        if not Path(container_path).exists():
            print(f"  Project path not accessible: {path}")
            return None

        with self.lock:
            if project_id in self.projects:
                return self.projects[project_id]

            print(f"  Loading project_id: {name} ({project_id})")
            idx = CodebaseIndex(container_path, name=name)
            idx.full_scan()
            self.projects[project_id] = idx
            self._start_watcher(f"project_id:{project_id}", idx)
            print(f"    -> {len(idx.files)} files indexed")
            return idx

    def register_project(self, project_id: str, name: str, path: str) -> tuple[bool, str, int]:
        """Register and index a new project_id."""
        try:
            idx = self._load_project(project_id, name, path)
            if idx:
                return True, f"Project '{name}' registered", len(idx.files)
            else:
                return False, f"Could not access project_id path: {path}", 0
        except Exception as e:
            return False, f"Error registering project_id: {e}", 0

    def unregister_project(self, project_id: str) -> tuple[bool, str]:
        """Unregister a project_id and remove its index."""
        with self.lock:
            # Stop watcher
            self._stop_watcher(f"project_id:{project_id}")

            # Remove from index
            if project_id in self.projects:
                del self.projects[project_id]
                return True, "Project unregistered"
            else:
                return False, "Project not found"

    def init_repos(self):
        """Initialize indexes for existing repos."""
        if not self.repos_root.exists():
            return

        for repo_dir in self.repos_root.iterdir():
            if repo_dir.is_dir() and not repo_dir.name.startswith('.'):
                self._load_repo(repo_dir.name)

    def _load_repo(self, name: str) -> CodebaseIndex | None:
        """Load an existing repo into the index."""
        repo_path = self.repos_root / name
        if not repo_path.exists():
            return None

        with self.lock:
            if name in self.repos:
                return self.repos[name]

            print(f"Loading repo index: {name}")
            idx = CodebaseIndex(str(repo_path), name=name)
            idx.full_scan()
            self.repos[name] = idx
            self._start_watcher(name, idx)
            return idx

    def _start_watcher(self, name: str, idx: CodebaseIndex):
        """Start file watcher for an index."""
        handler = IndexerHandler(idx)
        observer = Observer()
        observer.schedule(handler, str(idx.root), recursive=True)
        observer.start()
        self.observers[name] = observer
        print(f"File watcher started for: {name}")

    def _stop_watcher(self, name: str):
        """Stop file watcher for an index."""
        if name in self.observers:
            self.observers[name].stop()
            self.observers[name].join()
            del self.observers[name]

    def add_repo(self, name: str, git_url: str) -> tuple[bool, str]:
        """Clone a git repo and index it."""
        repo_path = self.repos_root / name

        if repo_path.exists():
            return False, f"Repo '{name}' already exists"

        # Clone the repo
        try:
            print(f"Cloning {git_url} to {repo_path}...")
            result = subprocess.run(
                ['git', 'clone', '--depth', '1', git_url, str(repo_path)],
                capture_output=True,
                text=True,
                timeout=300
            )
            if result.returncode != 0:
                return False, f"Git clone failed: {result.stderr}"
        except subprocess.TimeoutExpired:
            return False, "Git clone timed out"
        except Exception as e:
            return False, f"Git clone error: {e}"

        # Index the repo
        idx = self._load_repo(name)
        if idx:
            return True, f"Repo '{name}' added with {len(idx.files)} files"
        else:
            return False, "Failed to index repo"

    def remove_repo(self, name: str) -> tuple[bool, str]:
        """Remove a repo and its index."""
        repo_path = self.repos_root / name

        with self.lock:
            # Stop watcher
            self._stop_watcher(name)

            # Remove from index
            if name in self.repos:
                del self.repos[name]

            # Remove files
            if repo_path.exists():
                import shutil
                shutil.rmtree(repo_path)
                return True, f"Repo '{name}' removed"
            else:
                return False, f"Repo '{name}' not found"

    def list_repos(self) -> list[dict]:
        """List all knowledge repos."""
        repos = []
        with self.lock:
            for _name, idx in self.repos.items():
                repos.append(idx.get_stats())
        return repos

    def get_repo(self, name: str) -> CodebaseIndex | None:
        """Get a repo index by name."""
        with self.lock:
            return self.repos.get(name)

    def shutdown(self):
        """Stop all watchers."""
        for name in list(self.observers.keys()):
            self._stop_watcher(name)


# ============================================================================
# File Watcher
# ============================================================================

class IndexerHandler(FileSystemEventHandler):
    def __init__(self, index: CodebaseIndex):
        self.index = index

    def on_modified(self, event):
        if event.is_directory:
            return
        path = Path(event.src_path)
        if self.index.should_index(path) and self.index.index_file(path):
            self.index.record_change(path, 'modified')
            # GL-046.1: Invalidate content cache on file change
            try:
                rel_path = str(path.relative_to(self.index.root))
                content_cache.invalidate(self.index.name, rel_path)
            except Exception:
                pass

    def on_created(self, event):
        if event.is_directory:
            return
        path = Path(event.src_path)
        if self.index.should_index(path) and self.index.index_file(path):
            self.index.record_change(path, 'added')

    def on_deleted(self, event):
        if event.is_directory:
            return
        path = Path(event.src_path)
        try:
            rel_path = str(path.relative_to(self.index.root))
            # GL-046.1: Invalidate content cache on file delete
            content_cache.invalidate(self.index.name, rel_path)
            with self.index.lock:
                if rel_path in self.index.files:
                    self.index._remove_file_from_index(rel_path)
                    del self.index.files[rel_path]
                    self.index.record_change(path, 'deleted')
        except Exception:
            pass


# ============================================================================
# Intent Index - Semantic layer over tool usage
# ============================================================================

class IntentIndex:
    """
    Bidirectional index for intent tracking with per-project_id isolation.

    Stores (per project_id):
    - tag -> files: Which files are associated with each intent tag
    - file -> tags: Which intent tags are associated with each file
    - timeline: Chronological list of all intent records (in-memory, session-scoped)

    Persists to Redis (survives restarts):
    - Cumulative metrics (total_intents, total_tokens_saved)
    - Tag-file associations
    - First seen timestamp
    """

    DEFAULT_PROJECT = "_global"  # Fallback for requests without project_id

    def __init__(self, redis_client=None):
        # All data structures are nested by project_id
        self.tag_to_files: dict[str, dict[str, set[str]]] = defaultdict(lambda: defaultdict(set))
        self.file_to_tags: dict[str, dict[str, set[str]]] = defaultdict(lambda: defaultdict(set))
        # A-002: Use deque with maxlen to cap memory usage (10k intents per project)
        self.timeline: dict[str, deque] = defaultdict(lambda: deque(maxlen=10000))
        self.session_intents: dict[str, dict[str, list[IntentRecord]]] = defaultdict(lambda: defaultdict(list))
        self.lock = threading.RLock()
        self.redis = redis_client  # Optional Redis for persistence

    def _project_key(self, project_id: str = None) -> str:
        """Get project_id key, using default for empty/None."""
        return project_id if project_id else self.DEFAULT_PROJECT

    def record(self, tool: str, files: list[str], tags: list[str], session_id: str,
               tool_use_id: str = None, project_id: str = None, file_sizes: dict[str, int] = None,
               output_size: int = None, locations: list[dict] = None):
        """Record an intent from a tool use."""
        proj = self._project_key(project_id)
        record = IntentRecord(
            timestamp=int(time.time()),
            session_id=session_id,
            tool=tool,
            files=files,
            tags=tags,
            tool_use_id=tool_use_id,
            project_id=project_id,
            file_sizes=file_sizes or {},
            output_size=output_size,
            locations=locations or []  # GL-069.5: Rich symbol locations
        )

        # Calculate token savings for this record (QoL-3: sum all files)
        tokens_saved = 0
        if output_size and output_size > 0 and file_sizes:
            total_baseline = sum(size for size in file_sizes.values() if size > 0)
            if total_baseline > 0:
                baseline_tokens = total_baseline // 4  # bytes to ~tokens
                actual_tokens = output_size // 4
                tokens_saved = max(0, baseline_tokens - actual_tokens)

        with self.lock:
            # Add to project_id-specific timeline
            self.timeline[proj].append(record)
            self.session_intents[proj][session_id].append(record)

            # Update project_id-specific bidirectional indexes
            for tag in tags:
                for f in files:
                    self.tag_to_files[proj][tag].add(f)
                    self.file_to_tags[proj][f].add(tag)

        # Persist to Redis (cumulative, survives restarts)
        # Note: self.redis is RedisClient wrapper, use .client for raw redis-py operations
        if self.redis:
            try:
                r = self.redis.client  # Get raw redis-py client
                redis_key = f"aoa:{proj}:metrics"
                # Increment cumulative counters
                r.hincrby(redis_key, 'total_intents', 1)
                if tokens_saved > 0:
                    r.hincrby(redis_key, 'total_tokens_saved', tokens_saved)
                # Set first_seen if not exists
                r.hsetnx(redis_key, 'first_seen', int(time.time()))
                # Update last_active
                r.hset(redis_key, 'last_active', int(time.time()))

                # Persist tag-file associations
                # R-012 + A-008: Add TTL to tag sets for auto-cleanup
                for tag in tags:
                    for f in files:
                        tag_key = f"aoa:{proj}:tags:{tag}"
                        file_tag_key = f"aoa:{proj}:file_tags:{f}"
                        r.sadd(tag_key, f)
                        # R-012: Cap tags per file at 50 (scard check)
                        if r.scard(file_tag_key) < 50:
                            r.sadd(file_tag_key, tag)
                        # 24-hour TTL on tag sets
                        r.expire(tag_key, 86400)
                        r.expire(file_tag_key, 86400)

                # Track file access timestamps for KeywordMatcher eligibility
                now = int(time.time())
                file_accessed_key = f"aoa:{proj}:file_accessed"
                for f in files:
                    r.zadd(file_accessed_key, {f: now})
                # R-011: 30-day TTL on file access ZSET
                r.expire(file_accessed_key, 2592000)
            except Exception as e:
                print(f"[IntentIndex] Redis persistence error: {e}", flush=True)

    def files_for_tag(self, tag: str, project_id: str = None) -> list[str]:
        """Get files associated with a tag."""
        proj = self._project_key(project_id)
        with self.lock:
            return list(self.tag_to_files[proj].get(tag, set()))

    def tags_for_file(self, file: str, project_id: str = None) -> list[str]:
        """Get tags for a file - O(1) lookup from pre-indexed SET."""
        proj = self._project_key(project_id)

        # Fast path: Direct SET lookup (O(1)) - tags stored by record()
        if self.redis:
            try:
                tags = self.redis.smembers(f"aoa:{proj}:file_tags:{file}")
                if tags:
                    # Filter out generic activity tags
                    generic = {'#executing', '#reading', '#editing', '#creating', '#searching',
                              '#delegating', '#shell', '#python', '#markdown', '#indexing',
                              '#hooks', '#test', '#grep', '#curl'}
                    decoded = [t.decode() if isinstance(t, bytes) else t for t in tags]
                    semantic = [t for t in decoded if t not in generic]
                    if semantic:
                        return sorted(semantic)[:3]
                    return sorted(decoded)[:3]
            except Exception:
                pass  # Fall through

        # Fallback to in-memory intent tags
        with self.lock:
            proj_file_to_tags = self.file_to_tags[proj]
            if file in proj_file_to_tags:
                return list(proj_file_to_tags[file])[:3]
            for f, tags in proj_file_to_tags.items():
                if f.endswith(file) or file in f:
                    return list(tags)[:3]
            return []

    def domains_for_symbol(self, file: str, symbol: str, project_id: str = None) -> list[str]:
        """GL-071.5: Get domains assigned to a specific symbol - O(1) Redis lookup."""
        if not self.redis or not symbol:
            return []

        proj = self._project_key(project_id)
        try:
            r = self.redis.client if hasattr(self.redis, 'client') else self.redis
            symbol_key = f"aoa:{proj}:symbol_domains:{file}:{symbol}"
            domains = r.smembers(symbol_key)
            if domains:
                decoded = [d.decode() if isinstance(d, bytes) else d for d in domains]
                return sorted(decoded)[:3]  # Limit to 3 most relevant
        except Exception:
            pass
        return []

    def get_file_last_accessed(self, file_path: str, project_id: str = None) -> int:
        """Get when file was last accessed via intent tracking.

        Used for timestamp-based eligibility in KeywordMatcher.
        Files accessed after a domain's created_at are eligible for that domain's tags.

        Returns:
            Unix timestamp of last access, or 0 if never tracked
        """
        if not self.redis:
            return 0

        proj = self._project_key(project_id)
        try:
            r = self.redis.client if hasattr(self.redis, 'client') else self.redis
            accessed = r.zscore(f"aoa:{proj}:file_accessed", file_path)
            return int(accessed) if accessed else 0
        except Exception:
            return 0

    def record_file_access(self, file_path: str, project_id: str = None) -> None:
        """Record when a file was accessed for timestamp eligibility.

        Called by intent tracking to update file access timestamps.
        """
        if not self.redis or not file_path:
            return

        proj = self._project_key(project_id)
        now = int(time.time())
        try:
            r = self.redis.client if hasattr(self.redis, 'client') else self.redis
            r.zadd(f"aoa:{proj}:file_accessed", {file_path: now})
        except Exception:
            pass

    def recent(self, since: int = None, limit: int = 50, project_id: str = None) -> list[dict]:
        """Get recent intent records."""
        proj = self._project_key(project_id)
        with self.lock:
            records = list(self.timeline[proj])  # deque doesn't support slicing
            if since:
                records = [r for r in records if r.timestamp >= since]
            records = records[-limit:]
            return [asdict(r) for r in reversed(records)]

    def session(self, session_id: str, project_id: str = None) -> list[dict]:
        """Get intent records for a session."""
        proj = self._project_key(project_id)
        with self.lock:
            return [asdict(r) for r in self.session_intents[proj].get(session_id, [])]

    def all_tags(self, project_id: str = None) -> list[tuple[str, int]]:
        """Get all tags with file counts, sorted by count."""
        proj = self._project_key(project_id)
        with self.lock:
            return sorted(
                [(tag, len(files)) for tag, files in self.tag_to_files[proj].items()],
                key=lambda x: x[1],
                reverse=True
            )

    def get_stats(self, project_id: str = None) -> dict:
        """Get intent index statistics including token savings.

        Combines:
        - Cumulative metrics from Redis (persisted across restarts)
        - Session metrics from memory (current session only)
        """
        proj = self._project_key(project_id)

        # Get cumulative metrics from Redis (persisted)
        cumulative_intents = 0
        cumulative_tokens_saved = 0
        first_seen = None
        redis_tags_count = 0

        if self.redis:
            try:
                r = self.redis.client  # Get raw redis-py client
                redis_key = f"aoa:{proj}:metrics"
                metrics = r.hgetall(redis_key)
                if metrics:
                    cumulative_intents = int(metrics.get('total_intents', 0))
                    cumulative_tokens_saved = int(metrics.get('total_tokens_saved', 0))
                    first_seen = int(metrics.get('first_seen', 0)) if metrics.get('first_seen') else None

                # Count persisted tags
                tag_keys = self.redis.keys(f"aoa:{proj}:tags:*")
                redis_tags_count = len(tag_keys) if tag_keys else 0
            except Exception:
                pass

        # Get session metrics from memory
        with self.lock:
            session_records = len(self.timeline[proj])
            session_tags = len(self.tag_to_files[proj])
            session_files = len(self.file_to_tags[proj])
            session_count = len(self.session_intents[proj])

            # Calculate session token savings
            session_baseline = 0
            session_actual = 0
            measured_count = 0

            for record in self.timeline[proj]:
                if record.output_size and record.output_size > 0 and record.file_sizes:
                    for _f, size in record.file_sizes.items():
                        if size > 0:
                            session_baseline += size // 4
                            session_actual += record.output_size // 4
                            measured_count += 1
                            break

            session_tokens_saved = max(0, session_baseline - session_actual)

        # Combine: cumulative (Redis) includes session (memory) via hincrby
        # So we use Redis values directly for totals
        total_tokens_saved = cumulative_tokens_saved if cumulative_tokens_saved > 0 else session_tokens_saved

        # Estimate time savings
        time_sec = total_tokens_saved * 0.0075

        return {
            'total_records': cumulative_intents if cumulative_intents > 0 else session_records,
            'unique_tags': max(redis_tags_count, session_tags),
            'unique_files': session_files,  # File count from memory is accurate for session
            'sessions': session_count,
            'project_id': project_id,
            'first_seen': first_seen,
            'savings': {
                'tokens': total_tokens_saved,
                'time_sec': round(time_sec, 1),
                'baseline': session_baseline,  # Session baseline for debugging
                'actual': session_actual,
                'measured_records': measured_count
            }
        }

    def get_rolling_intent(self, project_id: str = None, window: int = 50) -> dict[str, float]:
        """Get weighted tag scores from recent N intents.

        GL-045: Rolling intent window for smart result ranking.
        Count-based (not time-based) - robust to lunch breaks, sleep, etc.
        A-007: Merges memory (current session) with Redis (cross-session history).

        Args:
            project_id: Project to query
            window: Number of recent intents to consider (default 50, configurable)

        Returns:
            Dict[tag, score] where score is position-weighted (recent = higher)
        """
        proj = self._project_key(project_id)
        tag_scores: dict[str, float] = {}

        with self.lock:
            # Deques don't support slicing - convert to list first
            timeline = list(self.timeline[proj]) if self.timeline[proj] else []
            recent = timeline[-window:] if timeline else []

            # A-007: If memory timeline is sparse, supplement with Redis tag data
            memory_count = len(recent)

            for i, record in enumerate(recent):
                # Position-weighted: oldest = 0.3, newest = 1.0
                if len(recent) > 1:
                    weight = 0.3 + (0.7 * (i / (len(recent) - 1)))
                else:
                    weight = 1.0

                for tag in record.tags:
                    tag_scores[tag] = tag_scores.get(tag, 0) + weight

        # A-007: Merge Redis historical data if memory is sparse
        if self.redis and memory_count < window // 2:
            try:
                r = self.redis.client
                # Get tags from Redis with their file counts as proxy for importance
                tag_pattern = f"aoa:{proj}:tags:*"
                redis_tags = {}
                for key in r.scan_iter(tag_pattern, count=100):
                    tag = key.split(':')[-1]  # Extract tag from key
                    count = r.scard(key)  # Number of files with this tag
                    if count > 0:
                        redis_tags[tag] = count

                # Add Redis tags with lower weight (historical = 0.1-0.2)
                if redis_tags:
                    max_count = max(redis_tags.values())
                    for tag, count in redis_tags.items():
                        # Weight by relative frequency, capped at 0.2 (less than memory's 0.3 min)
                        redis_weight = 0.1 + (0.1 * (count / max_count))
                        # Only add if not already strong from memory
                        if tag not in tag_scores or tag_scores[tag] < redis_weight:
                            tag_scores[tag] = tag_scores.get(tag, 0) + redis_weight
            except Exception:
                pass  # Redis failure shouldn't break intent

        return tag_scores

    def get_file_affinity(self, project_id: str = None, window: int = 50, rolling: dict = None) -> tuple[dict[str, float], dict[str, float]]:
        """Get pre-computed affinity scores for files based on rolling intent.

        GL-045: Files that match recent intent tags get higher scores.
        O1-009/O1-010: Returns both affinity and rolling to avoid recomputation.

        Args:
            project_id: Project to query
            window: Number of recent intents to consider
            rolling: Optional pre-computed rolling intent (avoids recomputation)

        Returns:
            Tuple of (file_scores, rolling_intent) - reuse rolling for UI display
        """
        if rolling is None:
            rolling = self.get_rolling_intent(project_id, window)
        if not rolling:
            return {}, {}

        proj = self._project_key(project_id)
        file_scores: dict[str, float] = {}

        with self.lock:
            for tag, weight in rolling.items():
                for file in self.tag_to_files[proj].get(tag, set()):
                    file_scores[file] = file_scores.get(file, 0) + weight

        return file_scores, rolling

    # =========================================================================
    # GL-088: Hit Tracking - Track domain/term hits during grep searches
    # =========================================================================

    def increment_hits(self, domains: list[str], terms: list[str], project_id: str = None):
        """
        Batch increment hit counters for domains and terms found in search results.

        Called at the end of grep/multi searches to track which domains/terms
        are actually being accessed.

        Redis keys:
            aoa:{proj}:domain:{name}:hits → INT (total hits)
            aoa:{proj}:term:{term}:hits → INT (total hits)
            aoa:{proj}:hits:recent → ZSET (domain:term, score=timestamp)
        """
        if not self.redis:
            return

        proj = self._project_key(project_id)
        now = int(time.time())

        try:
            r = self.redis.client
            pipe = r.pipeline()

            # Increment domain hits
            for domain in domains:
                domain_clean = domain.lstrip('@')
                pipe.hincrby(f"aoa:{proj}:hits:domains", domain_clean, 1)
                pipe.zadd(f"aoa:{proj}:hits:recent", {f"@{domain_clean}": now})

            # Increment term hits
            for term in terms:
                term_clean = term.lstrip('#')
                pipe.hincrby(f"aoa:{proj}:hits:terms", term_clean, 1)
                pipe.zadd(f"aoa:{proj}:hits:recent", {f"#{term_clean}": now})

            # Trim recent hits to last 1000
            recent_key = f"aoa:{proj}:hits:recent"
            pipe.zremrangebyrank(recent_key, 0, -1001)
            # R-013: 24-hour TTL on recent hits ZSET
            pipe.expire(recent_key, 86400)

            pipe.execute()
        except Exception as e:
            print(f"[IntentIndex] Hit tracking error: {e}", flush=True)

    def get_top_hits(self, project_id: str = None, limit: int = 10) -> dict:
        """
        Get top-hit domains and terms from domain metadata.

        GL-090: Reads from domain:@name:meta structure used by domain learning.

        Returns:
            {
                'domains': [{'name': '@auth', 'hits': 42}, ...],
                'terms': [{'name': '#token', 'hits': 15}, ...],
                'recent': ['@auth', '#token', ...]  # Last N unique hits
            }
        """
        if not self.redis:
            return {'domains': [], 'terms': [], 'recent': []}

        proj = self._project_key(project_id)

        try:
            r = self.redis.client

            # Get domains from the domains set (GL-090 structure)
            domain_names = r.smembers(f"aoa:{proj}:domains")
            domains = []
            recent = []

            for name_bytes in domain_names:
                name = name_bytes.decode() if isinstance(name_bytes, bytes) else name_bytes
                # Get hits from domain metadata
                meta_key = f"aoa:{proj}:domain:{name}:meta"
                hits = r.hget(meta_key, "hits")
                if hits:
                    hit_count = int(float(hits))
                    if hit_count > 0:
                        domains.append({'name': name, 'hits': hit_count})
                        recent.append(name)

            domains.sort(key=lambda x: x['hits'], reverse=True)

            # Get terms from term keys (GL-090 structure)
            # O1-001: Use scan_iter instead of keys() to avoid blocking Redis
            terms = []
            for key in r.scan_iter(match=f"aoa:{proj}:term:*", count=100):
                key_str = key.decode() if isinstance(key, bytes) else key
                # Extract term name from key like "aoa:proj:term:search"
                term_name = key_str.split(":")[-1]
                # Get score (number of domains using this term)
                score = r.zcard(key_str)
                if score and score > 0:
                    terms.append({'name': f"#{term_name}", 'hits': score})

            terms.sort(key=lambda x: x['hits'], reverse=True)

            return {
                'domains': domains[:limit],
                'terms': terms[:limit],
                'recent': recent[:limit]
            }
        except Exception as e:
            print(f"[IntentIndex] Get hits error: {e}", flush=True)
            return {'domains': [], 'terms': [], 'recent': []}


# ============================================================================
# Global Index Manager
# ============================================================================

manager: IndexManager | None = None
intent_index: IntentIndex | None = None


# ============================================================================
# API Endpoints - Local Index (default)
# ============================================================================

@app.route('/health')
def health():
    local = manager.get_local()

    # Check Redis connectivity
    redis_connected = False
    if intent_index and intent_index.redis:
        try:
            intent_index.redis.client.ping()
            redis_connected = True
        except Exception:
            pass

    response = {
        'status': 'ok',
        'mode': 'global' if manager.global_mode else 'legacy',
        'redis': {'connected': redis_connected},
        'repos': [r.get_stats() for r in manager.repos.values()],
        'content_cache': content_cache.stats(),  # GL-046.1: LRU cache stats
        'ac_matcher': {  # GL-047: Aho-Corasick pattern matcher
            'available': AC_MATCHER.is_available,
            'pattern_count': AC_MATCHER.pattern_count
        }
    }

    if local:
        response['local'] = local.get_stats()

    if manager.projects:
        response['projects'] = [
            {
                'id': pid,
                'name': idx.name,
                'files': len(idx.files),
                'symbols': len(idx.inverted_index)
            }
            for pid, idx in manager.projects.items()
        ]

    return jsonify(response)


@app.route('/ac/test')
def ac_test():
    """
    GL-047: Test endpoint for Aho-Corasick pattern matcher.

    Usage: /ac/test?text=def get_cached_user_token()
    """
    text = request.args.get('text', 'def get_cached_user_token(user_id)')

    if not AC_MATCHER.is_available:
        return jsonify({
            'error': 'AC matcher not available',
            'available': False
        }), 503

    start = time.time()
    hits = AC_MATCHER.find_all(text)
    density = AC_MATCHER.density_by_category(text)
    dense_tags = AC_MATCHER.get_dense_tags(text, threshold=1)
    elapsed_ms = (time.time() - start) * 1000

    return jsonify({
        'text': text,
        'hits': [{'pattern': h[0], 'tag': h[1], 'source': h[2]} for h in hits],
        'density': density,
        'dense_tags': dense_tags,
        'pattern_count': AC_MATCHER.pattern_count,
        'elapsed_ms': round(elapsed_ms, 3)
    })


@app.route('/keywords/rebuild', methods=['POST'])
def keywords_rebuild():
    """
    Rebuild the KeywordMatcher automaton from Redis.

    Called after rebalance_keywords() in learner.py to refresh
    the AC automaton with new keyword→domain mappings.
    """
    global KEYWORD_MATCHER

    project_id = request.args.get('project_id')
    start = time.time()

    try:
        if KEYWORD_MATCHER:
            KEYWORD_MATCHER.rebuild()
            elapsed_ms = (time.time() - start) * 1000
            return jsonify({
                'status': 'ok',
                'keywords': KEYWORD_MATCHER.keyword_count,
                'domains': len(KEYWORD_MATCHER.domain_created_at),
                'elapsed_ms': round(elapsed_ms, 3)
            })
        elif intent_index and intent_index.redis:
            # Initialize if not yet created
            proj = project_id or 'local'
            KEYWORD_MATCHER = KeywordMatcher(proj, intent_index.redis)
            KEYWORD_MATCHER.rebuild()
            elapsed_ms = (time.time() - start) * 1000
            return jsonify({
                'status': 'initialized',
                'keywords': KEYWORD_MATCHER.keyword_count,
                'domains': len(KEYWORD_MATCHER.domain_created_at),
                'elapsed_ms': round(elapsed_ms, 3)
            })
        else:
            return jsonify({
                'status': 'error',
                'error': 'Redis not available for KeywordMatcher'
            }), 503
    except Exception as e:
        return jsonify({
            'status': 'error',
            'error': str(e)
        }), 500


@app.route('/symbol')
def symbol_search():
    start = time.time()
    try:
        q = request.args.get('q', '')
        mode = request.args.get('mode', 'recent')
        limit = int(request.args.get('limit', 20))
        project_id = request.args.get('project_id')  # Optional project ID
        since = request.args.get('since')  # Unix timestamp or seconds ago
        before = request.args.get('before')  # Unix timestamp or seconds ago
        file_filter = request.args.get('filter')  # GL-051: File pattern filter (Unix grep parity)
        rank_by_intent = request.args.get('rank', 'true').lower() != 'false'  # GL-045: opt-out

        # Convert time params to absolute timestamps
        now = int(time.time())
        since_ts = None
        before_ts = None
        if since:
            since_val = int(since)
            since_ts = since_val if since_val > 1000000000 else now - since_val
        if before:
            before_val = int(before)
            before_ts = before_val if before_val > 1000000000 else now - before_val

        idx = manager.get_local(project_id)
        if not idx:
            return jsonify({
                'error': 'No index available',
                'message': 'Run "aoa init" in a project_id to register it',
                'results': [],
                'ms': 0
            }), 404

        # Get more results than needed so we can rank and still return `limit`
        fetch_limit = limit * 3 if rank_by_intent else limit
        results = idx.search(q, mode, fetch_limit, since=since_ts, before=before_ts, file_filter=file_filter)

        # PERF: Cache file tags to avoid N+1 Redis calls
        file_tags_cache = {}

        # GL-045: Rank by rolling intent affinity
        rolling_tags = None
        if rank_by_intent and intent_index and results:
            # O1-009/O1-010: Get both affinity and rolling in one call (avoids recomputation)
            file_affinity, rolling_tags = intent_index.get_file_affinity(project_id, window=50)

            # Infer query intent from search term
            query_intent = set(infer_search_intent(q)) if q else set()

            # Score each result
            now = int(time.time())
            for r in results:
                file_path = r.get('file', '')
                score = 0.0

                # Rolling affinity (main signal)
                score += file_affinity.get(file_path, 0) * 2

                # Query intent match (boost files with matching tags)
                if file_path not in file_tags_cache:
                    file_tags_cache[file_path] = list(intent_index.tags_for_file(file_path, project_id))
                file_tags = set(file_tags_cache[file_path])
                query_overlap = len(query_intent & file_tags)
                score += query_overlap * 5

                # Recency boost (GL-045: recently modified files rank higher)
                mtime = r.get('mtime', 0)
                if mtime > 0:
                    age_hours = (now - mtime) / 3600
                    if age_hours < 1:
                        score += 3  # Modified in last hour
                    elif age_hours < 24:
                        score += 2  # Modified today
                    elif age_hours < 168:
                        score += 1  # Modified this week

                # Penalty for test/mock files (unless searching for tests)
                file_lower = file_path.lower()
                if 'test' not in q.lower() and any(p in file_lower for p in ['test', 'spec', 'mock', '__pycache__']):
                    score -= 3

                r['intent_score'] = round(score, 2)

            # Sort by intent score (highest first), then by original order
            results.sort(key=lambda r: r.get('intent_score', 0), reverse=True)

            # O1-009: rolling_tags already set by get_file_affinity() above

        # Trim to requested limit
        results = results[:limit]

        # GL-046.1: Enrich results with content from LRU cache (O(1) when warm)
        # O1-008: Use content_store as fallback to avoid disk I/O on cache miss
        # GL-046.3: Batch add tags to results (no per-file curl needed in CLI)
        for r in results:
            file_path = r.get('file', '')
            line_num = r.get('line', 0)
            if file_path and line_num > 0:
                line_content = content_cache.get_line(
                    project_id or 'local',
                    file_path,
                    line_num,
                    idx.root
                )
                # O1-008: Fallback to content_store (already in memory) if cache missed
                if line_content is None and file_path in idx.path_to_id:
                    file_id = idx.path_to_id[file_path]
                    lines = idx.content_store.get(file_id, [])
                    if 0 < line_num <= len(lines):
                        line_content = lines[line_num - 1]
                if line_content is not None:
                    # Strip leading whitespace, truncate for display
                    r['content'] = line_content.strip()[:100]

            # GL-046.3: Add file tags to each result (from intent tracking)
            if intent_index and file_path:
                if file_path not in file_tags_cache:
                    file_tags_cache[file_path] = list(intent_index.tags_for_file(file_path, project_id))
                r['intent_tags'] = file_tags_cache[file_path]

            # GL-047: AC tags are already in r['tags'] from Location.tags (computed at index time)
            # Keep both - intent_tags (activity-based) and tags (content-based density)

        response = {
            'results': results,
            'index': idx.name,
            'project_id': project_id,
            'ms': (time.time() - start) * 1000,
            'cache': content_cache.stats()  # Include cache stats for monitoring
        }

        # Include rolling intent tags if ranking was applied
        if rolling_tags:
            # Top 5 tags by score for display
            top_tags = sorted(rolling_tags.items(), key=lambda x: x[1], reverse=True)[:5]
            response['rolling_intent'] = [tag for tag, _ in top_tags]

        return jsonify(response)
    except Exception as e:
        return jsonify({
            'error': 'Search failed',
            'message': str(e),
            'results': [],
            'ms': (time.time() - start) * 1000
        }), 500


# ============================================================================
# API Endpoint - Semantic Grep (GL-045: Content search with structural context)
# ============================================================================

@app.route('/grep')
def semantic_grep():
    """Content search with structural context and intent ranking.

    Searches file contents like Unix grep, but:
    1. Shows which function/class contains each match
    2. Displays intent tags for file and symbol
    3. Ranks results by intent alignment with search term

    Query params:
        q: Search term (required)
        project_id: Project ID (optional)
        limit: Max results (default 50)
        regex: Enable regex mode (default false, like grep vs egrep)
    """
    start = time.time()

    q = request.args.get('q', '')
    project_id = request.args.get('project_id') or 'local'  # HIT-001: Default early
    # GL-050: Unix parity - case-sensitive by default, -i flag for insensitive
    case_insensitive = request.args.get('ci', '0') == '1'
    use_regex = request.args.get('regex', 'false').lower() == 'true'
    file_filter = request.args.get('filter')  # GL-051: File pattern filter (Unix grep parity)

    # GL-051: Compile file filter regex if provided
    file_filter_regex = None
    if file_filter:
        if '*' in file_filter:
            filter_pattern = file_filter.replace('.', r'\.').replace('**', '.*').replace('*', '[^/]*')
            file_filter_regex = re.compile(filter_pattern)
        # else: will use string matching in the loop

    if not q:
        return jsonify({'error': 'Missing search term', 'results': [], 'ms': 0}), 400

    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'error': 'No index available', 'results': [], 'ms': 0}), 404

    # 1. Infer intent from search term
    search_intent = infer_search_intent(q)

    project_id = project_id or 'local'

    # =================================================================
    # O(1) OPTIMIZATION: Task 3.2 - Dispatch based on search type
    # =================================================================

    if not use_regex and not case_insensitive:
        # Task 3.2b: LITERAL SEARCH - O(1) path via token_index
        # This is the fast path for exact token matches
        raw_results = idx.search_o1(q, limit=500, file_filter=file_filter)

        # Phase 3B: Pre-batch ALL enrichment data in ONE Redis pipeline
        # Benchmark: 130 ops goes from 14.76ms (loop) to 1.22ms (pipeline) = 12x faster
        file_tags_lookup, symbol_domains_lookup, file_accessed_lookup = batch_fetch_enrichment_data(
            results=raw_results,
            intent_index=intent_index,
            project_id=project_id,
            idx=idx
        )

        results = []
        files_with_hits = set()

        for r in raw_results:
            files_with_hits.add(r['file'])
            result = enrich_result(
                file_path=r['file'],
                line_num=r['line'],
                content=r['content'],
                idx=idx,
                intent_index=intent_index,
                project_id=project_id,
                file_id=r.get('_file_id'),  # Task 3.5: Pass file_id for O(1) metadata lookup
                # Phase 3B: Pre-batched lookups (O(1) Python dict, no Redis at query time)
                file_tags_lookup=file_tags_lookup,
                symbol_domains_lookup=symbol_domains_lookup,
                file_accessed_lookup=file_accessed_lookup
            )
            results.append(result)

        files_searched = len(idx.files)  # We "searched" the index, not individual files

    else:
        # Task 3.2c: REGEX SEARCH - O(n) path, scan content_store
        # Still faster than disk because content is in-memory

        # 2. Compile pattern - O1-012: Use LRU cached compilation
        flags = re.MULTILINE | (re.IGNORECASE if case_insensitive else 0)
        try:
            if use_regex:
                pattern = _compile_pattern(q, flags)
            else:
                # Case-insensitive literal search
                pattern = _compile_pattern(re.escape(q), flags)
        except re.error as e:
            return jsonify({'error': f'Invalid pattern: {e}', 'results': [], 'ms': 0}), 400

        # 3. Search using content_store (Task 3.3: in-memory, no disk reads)
        matches = []
        files_searched = 0
        files_with_hits = set()

        with idx.lock:
            # Task 3.3a: Iterate content_store instead of files + disk read
            for file_id, lines in idx.content_store.items():
                file_path = idx.id_to_path.get(file_id)
                if not file_path:
                    continue

                # GL-051: File pattern filtering
                if file_filter:
                    if file_filter_regex:
                        if not file_filter_regex.search(file_path):
                            continue
                    elif file_filter.endswith('/'):
                        if not file_path.startswith(file_filter):
                            continue
                    else:
                        if file_filter not in file_path:
                            continue

                files_searched += 1

                # Phase 3B FIX: Iterate lines directly - we already have line numbers!
                # Old approach joined lines then counted newlines = O(n) per match
                # New approach: enumerate gives us line_num for free = O(1)
                file_matches = []

                for line_num, line in enumerate(lines, 1):
                    match = pattern.search(line)
                    if match:
                        file_matches.append({
                            'line': line_num,
                            'text': line.strip()[:100],
                            'match': match.group()[:50]
                        })

                if file_matches:
                    files_with_hits.add(file_path)
                    meta = idx.files.get(file_path)
                    matches.append({
                        'file': file_path,
                        'file_id': file_id,  # Pass through for O(1) enrichment
                        'language': meta.language if meta else 'unknown',
                        'matches': file_matches
                    })

        # 4. Enrich results
        # Phase 3B: Flatten matches for pre-batch, then enrich
        flat_results = []
        for file_match in matches:
            rel_path = file_match['file']
            fid = file_match['file_id']  # Pass through for O(1) enrichment
            for m in file_match['matches']:
                flat_results.append({
                    'file': rel_path,
                    'line': m['line'],
                    'content': m['text'],
                    '_file_id': fid
                })

        # Pre-batch ALL enrichment data in ONE Redis pipeline
        file_tags_lookup, symbol_domains_lookup, file_accessed_lookup = batch_fetch_enrichment_data(
            results=flat_results,
            intent_index=intent_index,
            project_id=project_id,
            idx=idx
        )

        results = []
        for r in flat_results:
            result = enrich_result(
                file_path=r['file'],
                line_num=r['line'],
                content=r['content'],
                idx=idx,
                intent_index=intent_index,
                project_id=project_id,
                file_id=r.get('_file_id'),  # Same as grep path
                file_tags_lookup=file_tags_lookup,
                symbol_domains_lookup=symbol_domains_lookup,
                file_accessed_lookup=file_accessed_lookup
            )
            results.append(result)

    # 5. Record gap keyword if no results found (P1-1: wire learning loop)
    if not results and q and DomainLearner:
        try:
            learner = DomainLearner(project_id)
            learner.record_gap_keyword(q)
        except Exception:
            pass  # Don't fail search on learning errors

    # 6. GL-050: Universal Response - format through single function
    elapsed = (time.time() - start) * 1000
    response = format_search_response(
        results=results,
        ms=elapsed,
        files_searched=files_searched,
        search_intent=search_intent,
        intent_index=intent_index,
        project_id=project_id
    )

    return jsonify(response)


@app.route('/multi', methods=['GET', 'POST'])
def multi_search():
    start = time.time()

    # Support both GET (query params) and POST (JSON body)
    if request.method == 'GET':
        q = request.args.get('q', '')
        terms = q.split() if q else []
        mode = request.args.get('mode', 'recent')
        limit = int(request.args.get('limit', 20))
        project_id = request.args.get('project_id')
        since = request.args.get('since')
        before = request.args.get('before')
        file_filter = request.args.get('filter')  # GL-051
    else:
        data = request.json
        terms = data.get('terms', [])
        mode = data.get('mode', 'recent')
        limit = int(data.get('limit', 20))
        project_id = data.get('project_id')
        since = data.get('since')
        before = data.get('before')
        file_filter = data.get('filter')  # GL-051

    # Convert time params to absolute timestamps
    now = int(time.time())
    since_ts = None
    before_ts = None
    if since:
        since_val = int(since)
        since_ts = since_val if since_val > 1000000000 else now - since_val
    if before:
        before_val = int(before)
        before_ts = before_val if before_val > 1000000000 else now - before_val

    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'error': 'No index available', 'results': [], 'ms': 0}), 404

    results = idx.search_multi(terms, mode, limit, since=since_ts, before=before_ts, file_filter=file_filter)

    return jsonify({
        'results': results,
        'index': idx.name,
        'project_id': project_id,
        'ms': (time.time() - start) * 1000
    })


@app.route('/outline')
def get_outline():
    """Get code outline (functions, classes) for a file using tree-sitter."""
    start = time.time()

    file_path = request.args.get('file')
    project_id = request.args.get('project_id')

    if not file_path:
        return jsonify({'error': 'Missing file parameter', 'symbols': [], 'ms': 0}), 400

    if not TREE_SITTER_AVAILABLE:
        return jsonify({
            'error': 'tree-sitter not available',
            'message': 'Install tree-sitter and tree-sitter-languages',
            'symbols': [],
            'ms': 0
        }), 503

    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'error': 'No index available', 'symbols': [], 'ms': 0}), 404

    # Resolve file path
    full_path = Path(idx.root) / file_path if not Path(file_path).is_absolute() else Path(file_path)

    if not full_path.exists():
        return jsonify({'error': f'File not found: {file_path}', 'symbols': [], 'ms': 0}), 404

    # Get language from file extension
    language = idx.get_language(full_path)
    if language == 'unknown':
        return jsonify({
            'error': f'Unsupported language for: {file_path}',
            'symbols': [],
            'ms': (time.time() - start) * 1000
        }), 400

    # Parse and get outline
    symbols = outline_parser.parse_file(str(full_path), language)

    # Enrich symbols with tags from Redis
    enriched_symbols = []
    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))
        # Use request param or fallback to "local"
        project_key = project_id or "local"

        # O1-011: Single scan for ALL symbols in file, then distribute
        # Pattern: tag_meta:{project_key}:{file_path}:{symbol}:{tag}
        all_pattern = f"tag_meta:{project_key}:{file_path}:*"
        all_keys = list(r.scan_iter(match=all_pattern, count=500))

        # Build symbol -> tags map from scan results
        symbol_tags = defaultdict(list)
        for key in all_keys:
            parts = (key.decode() if isinstance(key, bytes) else key).split(':')
            if len(parts) >= 5:
                symbol_name = parts[3]  # tag_meta:proj:file:SYMBOL:tag
                tag = parts[4]
                symbol_tags[symbol_name].append(tag)

        # Assign tags to symbols
        for sym in symbols:
            sym_dict = asdict(sym)
            sym_dict['tags'] = symbol_tags.get(sym.name, [])
            enriched_symbols.append(sym_dict)
    except Exception as e:
        # Fallback: return symbols without tags
        print(f"Tag enrichment failed: {e}", flush=True)
        enriched_symbols = [asdict(s) for s in symbols]

    return jsonify({
        'file': str(file_path),
        'language': language,
        'symbols': enriched_symbols,
        'count': len(symbols),
        'ms': (time.time() - start) * 1000
    })


@app.route('/outline/enriched', methods=['POST'])
def mark_enriched():
    """Store semantic compression tags with counting (idempotent, tracks confidence).

    POST body: {
        file: string,
        project_id: string,
        symbols: [{name, kind, line, end_line, tags, domains?}]
    }

    Tags are stored as:
    - #term tags: tag_count/tag_meta keys (for counting/confidence)
    - @domain tags: aoa:{proj}:symbol_domains:{file}:{symbol} (for lookup in enrich_result)
    """
    data = request.json
    file_path = data.get('file')
    project_id = data.get('project_id')
    symbols = data.get('symbols', [])  # List of {name, kind, line, end_line, tags, domains?}

    if not file_path:
        return jsonify({'success': False, 'error': 'Missing file parameter'}), 400

    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'success': False, 'error': 'No index available'}), 404

    tags_indexed = 0
    tags_incremented = 0
    domains_assigned = 0
    mtime = int(time.time())
    project_key = project_id or 'default'

    # Use Redis for tag counting (dedup + confidence tracking)
    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))

        for sym in symbols:
            sym_name = sym.get('name', '')
            sym_kind = sym.get('kind', 'unknown')
            line = sym.get('line', 0)
            end_line = sym.get('end_line', line)
            tags = sym.get('tags', [])
            domains = sym.get('domains', [])  # GL-084: Domain tags for symbol_domains storage

            # GL-084: Store @domain tags in symbol_domains for enrich_result lookup
            # Key format: aoa:{proj}:symbol_domains:{file}:{symbol}
            if domains:
                symbol_domains_key = f"aoa:{project_key}:symbol_domains:{file_path}:{sym_name}"
                for domain in domains:
                    domain_tag = domain if domain.startswith('@') else f"@{domain}"
                    r.sadd(symbol_domains_key, domain_tag)
                    domains_assigned += 1

                # Track which symbols exist in this file
                file_symbols_key = f"aoa:{project_key}:file_symbols:{file_path}"
                r.sadd(file_symbols_key, sym_name)

            for tag in tags:
                # Skip @domain tags from regular tag storage (already handled above)
                if tag.startswith('@'):
                    continue

                # Key: tag_count:{project_id}:{file}:{symbol}:{tag}
                count_key = f"tag_count:{project_key}:{file_path}:{sym_name}:{tag}"

                # Increment count (creates key with value 1 if doesn't exist)
                new_count = r.incr(count_key)

                if new_count == 1:
                    # First time seeing this tag - add to inverted index
                    with idx.lock:
                        loc = Location(
                            file=file_path,
                            line=line,
                            col=0,
                            symbol_type='tag',
                            mtime=mtime,
                            symbol=sym_name,
                            symbol_kind=sym_kind,
                            end_line=end_line
                        )
                        idx.inverted_index[tag].append(loc)
                    tags_indexed += 1
                else:
                    # Already exists, just incremented count
                    tags_incremented += 1

                # Also store symbol metadata for retrieval
                meta_key = f"tag_meta:{project_key}:{file_path}:{sym_name}:{tag}"
                r.hset(meta_key, mapping={
                    'kind': sym_kind,
                    'line': line,
                    'end_line': end_line,
                    'updated': mtime
                })

        # Store enrichment timestamp
        enrich_key = f"enriched:{project_key}:{file_path}"
        r.hset(enrich_key, mapping={
            'enriched_at': mtime,
            'tags_count': tags_indexed + tags_incremented,
            'domains_count': domains_assigned
        })

    except Exception:
        # Fallback: just append without dedup (legacy behavior)
        with idx.lock:
            for sym in symbols:
                sym_name = sym.get('name', '')
                sym_kind = sym.get('kind', 'unknown')
                line = sym.get('line', 0)
                end_line = sym.get('end_line', line)
                tags = sym.get('tags', [])

                for tag in tags:
                    loc = Location(
                        file=file_path,
                        line=line,
                        col=0,
                        symbol_type='tag',
                        mtime=mtime,
                        symbol=sym_name,
                        symbol_kind=sym_kind,
                        end_line=end_line
                    )
                    idx.inverted_index[tag].append(loc)
                    tags_indexed += 1

    return jsonify({
        'success': True,
        'file': file_path,
        'tags_indexed': tags_indexed,
        'tags_incremented': tags_incremented,
        'domains_assigned': domains_assigned,
        'symbols_processed': len(symbols),
        'enriched_at': mtime
    })


@app.route('/outline/tags')
def get_symbol_tags():
    """Get semantic tags for symbols in a file, with confidence counts."""
    file_path = request.args.get('file', '')
    project_id = request.args.get('project_id')
    include_counts = request.args.get('counts', 'false').lower() == 'true'

    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'error': 'No index available', 'tags': {}}), 404

    project_key = project_id or 'default'
    tags_by_symbol = {}

    # Try to get counts from Redis
    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))

        # Scan for all tag counts for this file
        pattern = f"tag_count:{project_key}:{file_path}:*"
        cursor = 0
        while True:
            cursor, keys = r.scan(cursor, match=pattern, count=100)
            for key in keys:
                # Parse key: tag_count:{project_id}:{file}:{symbol}:{tag}
                parts = key.decode().split(':')
                if len(parts) >= 5:
                    symbol_name = parts[3]
                    tag = ':'.join(parts[4:])  # Handle tags with colons
                    count = int(r.get(key) or 1)

                    if symbol_name not in tags_by_symbol:
                        tags_by_symbol[symbol_name] = []

                    if include_counts:
                        tags_by_symbol[symbol_name].append({'tag': tag, 'count': count})
                    else:
                        if tag not in tags_by_symbol[symbol_name]:
                            tags_by_symbol[symbol_name].append(tag)

            if cursor == 0:
                break

    except Exception:
        # Fallback: use inverted index (no counts)
        with idx.lock:
            for tag, locations in idx.inverted_index.items():
                if not tag.startswith('#'):
                    continue

                for loc in locations:
                    if loc.file == file_path or loc.file.endswith(file_path):
                        symbol_name = loc.symbol or 'file'
                        if symbol_name not in tags_by_symbol:
                            tags_by_symbol[symbol_name] = []
                        if tag not in tags_by_symbol[symbol_name]:
                            tags_by_symbol[symbol_name].append(tag)

    return jsonify({
        'file': file_path,
        'tags': tags_by_symbol
    })


@app.route('/outline/pending')
def get_pending_enrichment():
    """Get files that need enrichment (modified since last enriched or never enriched)."""
    start = time.time()
    project_id = request.args.get('project_id')

    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'error': 'No index available', 'pending': [], 'ms': 0}), 404

    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))
    except Exception:
        r = None

    pending = []
    up_to_date = []

    for rel_path, meta in idx.files.items():
        # Check if file has been enriched
        enriched_at = 0
        if r:
            key = f"enriched:{project_id or 'default'}:{rel_path}"
            data = r.hgetall(key)
            if data and b'enriched_at' in data:
                enriched_at = int(data[b'enriched_at'])

        # Compare mtime to enriched_at
        if meta.mtime > enriched_at:
            pending.append({
                'file': rel_path,
                'language': meta.language,
                'mtime': meta.mtime,
                'enriched_at': enriched_at if enriched_at > 0 else None,
                'reason': 'never' if enriched_at == 0 else 'modified'
            })
        else:
            up_to_date.append(rel_path)

    # Sort pending by mtime (most recently modified first)
    pending.sort(key=lambda x: x['mtime'], reverse=True)

    return jsonify({
        'pending': pending,
        'pending_count': len(pending),
        'up_to_date_count': len(up_to_date),
        'total_files': len(idx.files),
        'ms': (time.time() - start) * 1000
    })


@app.route('/file')
def get_file_content():
    """Get file content with optional line range. Used by aoa head/lines/tail."""
    start = time.time()

    file_path = request.args.get('path')
    lines_param = request.args.get('lines')  # "1-50" or "-20" (tail) or "30" (head)
    project_id = request.args.get('project_id')

    if not file_path:
        return jsonify({'error': 'Missing path parameter'}), 400

    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'error': 'No index available'}), 404

    # Resolve path (same pattern as /outline)
    full_path = Path(idx.root) / file_path if not Path(file_path).is_absolute() else Path(file_path)

    if not full_path.exists():
        return jsonify({'error': f'File not found: {file_path}'}), 404

    # Read content
    try:
        content = full_path.read_text(encoding='utf-8', errors='ignore')
        all_lines = content.split('\n')
    except Exception as e:
        return jsonify({'error': f'Cannot read file: {e}'}), 500

    total_lines = len(all_lines)

    # Parse line range
    if lines_param:
        if lines_param.startswith('-'):
            # Tail: last N lines
            n = int(lines_param[1:])
            extracted = all_lines[-n:]
            line_range = (max(1, total_lines - n + 1), total_lines)
        elif '-' in lines_param:
            # Range: start-end
            parts = lines_param.split('-')
            start_l = int(parts[0]) - 1
            end_l = int(parts[1]) if len(parts) > 1 else total_lines
            extracted = all_lines[start_l:end_l]
            line_range = (start_l + 1, min(end_l, total_lines))
        else:
            # Head: first N lines
            n = int(lines_param)
            extracted = all_lines[:n]
            line_range = (1, min(n, total_lines))

        return jsonify({
            'content': '\n'.join(extracted),
            'lines': line_range,
            'total_lines': total_lines,
            'file': file_path,
            'ms': (time.time() - start) * 1000
        })
    else:
        return jsonify({
            'content': content,
            'lines': (1, total_lines),
            'total_lines': total_lines,
            'file': file_path,
            'ms': (time.time() - start) * 1000
        })


# ============================================================================
# GL-059.2: Symbol Lookup for Targeted Assignment
# ============================================================================

@app.route('/symbol/lookup', methods=['POST'])
def symbol_lookup():
    """
    Resolve file:line references to enclosing function/class symbols.

    GL-059.2: Used by gateway hook to target domain assignment at FUNCTIONS,
    not individual lines. This reduces noise (50 line reads = 1 domain).

    POST body: {"locations": ["file.py:123", "other.py:456"], "project_id": "optional-id"}
    Returns: {"symbols": [{"location": "file.py:123", "symbol": "func_name", ...}]}
    """
    start = time.time()
    data = request.get_json(silent=True) or {}
    locations = data.get('locations', [])
    project_id = data.get('project_id') or request.args.get('project_id')

    if not locations:
        return jsonify({'error': 'Missing locations array', 'symbols': []})

    # Get index for this project_id
    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'error': 'No index available', 'symbols': []})

    results = []
    seen_symbols = set()  # Dedupe: same function accessed multiple times = 1

    for loc in locations:
        if ':' not in loc:
            continue

        parts = loc.rsplit(':', 1)
        file_path = parts[0]
        try:
            line_num = int(parts[1].split('-')[0])  # Handle ranges like "280-290"
        except (ValueError, IndexError):
            continue

        # Get outline for this file
        symbols = idx.file_outlines.get(file_path, [])

        # Find innermost enclosing symbol (same logic as enrich_result)
        enclosing = []
        for sym in symbols:
            if sym.start_line <= line_num <= sym.end_line:
                sym_range = sym.end_line - sym.start_line
                enclosing.append((sym_range, sym))

        if not enclosing:
            continue

        # Sort by range (smallest first = most specific)
        enclosing.sort(key=lambda x: x[0])
        _, best_match = enclosing[0]

        # Build unique key for deduplication
        symbol_key = f"{file_path}:{best_match.name}"
        if symbol_key in seen_symbols:
            continue
        seen_symbols.add(symbol_key)

        # Find parent (for class.method style)
        parent_name = None
        if len(enclosing) > 1:
            _, parent = enclosing[1]
            parent_name = parent.name

        results.append({
            'location': loc,
            'file': file_path,
            'symbol': best_match.name,
            'kind': best_match.kind,
            'signature': best_match.signature,
            'start_line': best_match.start_line,
            'end_line': best_match.end_line,
            'parent': parent_name,
            # Format: file.py:func_name() or file.py:Class.method()
            'qualified': f"{file_path}:{parent_name}.{best_match.name}()" if parent_name else f"{file_path}:{best_match.name}()"
        })

    return jsonify({
        'symbols': results,
        'total': len(results),
        'deduplicated_from': len(locations),
        'ms': (time.time() - start) * 1000
    })


# ============================================================================
# Domain Pattern Candidates (Learned from quickstart)
# ============================================================================

@app.route('/patterns/candidates', methods=['GET', 'POST'])
def domain_candidates():
    """Store or retrieve project_id-specific domain keyword candidates.

    POST: Store word frequencies discovered during quickstart
    GET: Retrieve stored candidates for a project_id

    These are high-frequency words NOT in universal patterns -
    candidates for project_id-specific tags.
    """
    # Handle project_id param differently for GET vs POST
    if request.method == 'POST':
        data = request.get_json(silent=True) or {}
        project_id = request.args.get('project_id') or data.get('project_id', '')
    else:
        project_id = request.args.get('project_id', '')

    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))
    except Exception:
        return jsonify({'error': 'Redis not available'}), 503

    redis_key = f"aoa:{project_id or 'default'}:domain_candidates"

    if request.method == 'POST':
        if not data:
            return jsonify({'error': 'No data provided'}), 400

        candidates = data.get('candidates', {})  # {word: count}
        suggested_domain = data.get('suggested_domain', '')
        total_symbols = data.get('total_symbols', 0)

        # Store in Redis as hash
        r.delete(redis_key)  # Clear old data
        if candidates:
            r.hset(redis_key, mapping=candidates)
        r.hset(redis_key, '_meta_domain', suggested_domain)
        r.hset(redis_key, '_meta_symbols', str(total_symbols))
        r.hset(redis_key, '_meta_timestamp', str(int(time.time())))

        return jsonify({
            'success': True,
            'stored': len(candidates),
            'suggested_domain': suggested_domain
        })

    else:  # GET
        data = r.hgetall(redis_key)
        if not data:
            return jsonify({
                'candidates': {},
                'suggested_domain': '',
                'total_symbols': 0
            })

        # Parse stored data
        candidates = {}
        suggested_domain = ''
        total_symbols = 0
        timestamp = 0

        for k, v in data.items():
            key = k.decode() if isinstance(k, bytes) else k
            val = v.decode() if isinstance(v, bytes) else v

            if key == '_meta_domain':
                suggested_domain = val
            elif key == '_meta_symbols':
                total_symbols = int(val)
            elif key == '_meta_timestamp':
                timestamp = int(val)
            else:
                candidates[key] = int(val)

        # Sort by count descending
        sorted_candidates = dict(sorted(candidates.items(), key=lambda x: x[1], reverse=True))

        return jsonify({
            'candidates': sorted_candidates,
            'suggested_domain': suggested_domain,
            'total_symbols': total_symbols,
            'timestamp': timestamp
        })


@app.route('/patterns/learned', methods=['GET', 'POST'])
def learned_patterns():
    """Store or retrieve project_id-specific learned patterns.

    POST: Store keyword->tag mappings (from Claude analysis or user input)
    GET: Retrieve learned patterns for merging with universal patterns
    """
    if request.method == 'POST':
        data = request.get_json(silent=True) or {}
        project_id = request.args.get('project_id') or data.get('project_id', '')
    else:
        data = {}
        project_id = request.args.get('project_id', '')

    try:
        import redis
        r = redis.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))
    except Exception:
        return jsonify({'error': 'Redis not available'}), 503

    redis_key = f"aoa:{project_id or 'default'}:learned_patterns"

    if request.method == 'POST':
        if not data:
            return jsonify({'error': 'No data provided'}), 400

        patterns = data.get('patterns', {})  # {keyword: tag}

        # Store in Redis as hash
        if patterns:
            r.hset(redis_key, mapping=patterns)
            r.hset(redis_key, '_meta_timestamp', str(int(time.time())))

        return jsonify({
            'success': True,
            'stored': len(patterns)
        })

    else:  # GET
        data = r.hgetall(redis_key)
        if not data:
            return jsonify({'patterns': {}})

        patterns = {}
        for k, v in data.items():
            key = k.decode() if isinstance(k, bytes) else k
            val = v.decode() if isinstance(v, bytes) else v
            if not key.startswith('_meta_'):
                patterns[key] = val

        return jsonify({'patterns': patterns})


@app.route('/patterns/infer', methods=['POST'])
def infer_patterns():
    """Infer tags from symbol names using project-domains.json (v2 format).

    POST body: {"symbols": [{"name": "getUserById", "kind": "function"}, ...]}
    Returns: {"tags": [["#term_name", "@domain"], ...], "domains": ["@domain", ...]}

    v2 format matching:
    - Matches KEYWORDS from terms against symbol tokens
    - Returns #term_name when keyword matches
    - Returns @domain_name at parent/container level

    Hierarchy:
    - @domain: Applied to parent containers (class, module, route)
    - #term: Applied to specific symbols where keywords match
    """
    import re
    from pathlib import Path

    data = request.get_json(silent=True) or {}
    symbols = data.get('symbols', [])

    if not symbols:
        return jsonify({'tags': [], 'domains': []})

    # Load project-domains.json (v2 format)
    # GL-084: Keywords in terms enable semantic matching
    project_paths = [
        Path(os.environ.get('CODEBASE_ROOT', '.')) / '.aoa' / 'project-domains.json',
        Path('/app/.aoa/project-domains.json'),  # Docker mounted
    ]

    # Build lookup tables from v2 format:
    # keyword -> term_name (for #term tagging)
    # keyword -> domain_name (for @domain tagging)
    # term_name -> domain_name (for reverse lookup)
    keyword_to_term = {}      # "grep" -> "grep_operations"
    keyword_to_domain = {}    # "grep" -> "@search_engine"
    term_to_domain = {}       # "grep_operations" -> "@search_engine"

    for path in project_paths:
        if path.exists():
            try:
                domains_data = json.loads(path.read_text())
                # Handle array format (v2)
                domains = domains_data if isinstance(domains_data, list) else domains_data.get('domains', [])

                for domain in domains:
                    domain_name = domain.get('name', '')  # e.g., "@search_engine"
                    if not domain_name.startswith('@'):
                        domain_name = f"@{domain_name}"

                    terms = domain.get('terms', {})
                    if isinstance(terms, dict):
                        for term_name, keywords in terms.items():
                            term_to_domain[term_name] = domain_name
                            if isinstance(keywords, list):
                                for kw in keywords:
                                    kw_lower = kw.lower()
                                    keyword_to_term[kw_lower] = term_name
                                    keyword_to_domain[kw_lower] = domain_name
                break  # Found and loaded
            except Exception:
                pass

    def tokenize(text):
        """Split symbol name into matchable tokens."""
        tokens = set()
        # Split on common separators
        parts = re.split(r'[/_\-.\s]+', text)
        for part in parts:
            if not part:
                continue
            tokens.add(part.lower())
            # Also split camelCase
            camel_parts = re.findall(r'[A-Z]?[a-z]+|[A-Z]+(?=[A-Z][a-z]|\d|\W|$)|\d+', part)
            for cp in camel_parts:
                if len(cp) >= 2:  # Skip single chars
                    tokens.add(cp.lower())
        return tokens

    def match_keywords(tokens, full_text):
        """Match tokens against keyword library using EXACT token matching.

        Word boundary matching - "start" won't match inside "quickstart".

        Returns:
            terms: set of #term_name matches
            domains: set of @domain matches
        """
        terms = set()
        domains = set()

        # Only use exact token matches (no substring matching)
        for token in tokens:
            if token in keyword_to_term:
                terms.add(f"#{keyword_to_term[token]}")
                domains.add(keyword_to_domain[token])

        return terms, domains

    # Process each symbol
    result_tags = []
    result_domains = []

    for sym in symbols:
        name = sym.get('name', '')
        kind = sym.get('kind', '')

        tokens = tokenize(name)
        terms, domains = match_keywords(tokens, name)

        # GL-084: Separation of concerns
        # - tags: ONLY #term tags (for lines inside symbols)
        # - domains: @domain tags (stored separately in symbol_domains)
        #
        # At display time:
        # - Definition line shows @domain (from symbol_domains lookup)
        # - Inner lines show #term (from tags array or content matching)
        result_tags.append(list(terms)[:5])  # Always #terms, never @domains
        result_domains.append(list(domains)[:2])  # For symbol_domains storage

    return jsonify({
        'tags': result_tags,
        'domains': result_domains  # Stored in symbol_domains Redis keys
    })


# ============================================================================
# Project Management Endpoints (Global Mode)
# ============================================================================

@app.route('/project_id/register', methods=['POST'])
def register_project():
    """Register a new project_id for indexing."""
    data = request.json
    project_id = data.get('id')
    name = data.get('name')
    path = data.get('path')

    if not all([project_id, name, path]):
        return jsonify({'success': False, 'error': 'Missing required fields: id, name, path'}), 400

    success, message, files = manager.register_project(project_id, name, path)

    return jsonify({
        'success': success,
        'message': message,
        'files': files
    })


@app.route('/project/<project_id>', methods=['DELETE'])
def unregister_project(project_id):
    """Unregister a project, remove its index, and wipe all stored data."""
    # First, wipe all Redis data for this project_id
    redis_deleted = {'domains': 0, 'enrichment': 0, 'total': 0}
    try:
        import redis as redis_lib
        r = redis_lib.from_url(os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))

        # Use Lua script for atomic bulk delete
        lua_script = """
        local keys = redis.call('keys', ARGV[1])
        for i=1,#keys do
            redis.call('del', keys[i])
        end
        return #keys
        """

        # Clear aoa:* (domains, intents, metrics) and enriched:* (semantic compression)
        deleted_aoa = r.eval(lua_script, 0, f"aoa:{project_id}:*")
        deleted_enriched = r.eval(lua_script, 0, f"enriched:{project_id}:*")
        redis_deleted = {
            'domains': deleted_aoa,
            'enrichment': deleted_enriched,
            'total': deleted_aoa + deleted_enriched
        }
    except Exception:
        pass  # Redis may not be available, continue with index removal

    # Then unregister from memory
    success, message = manager.unregister_project(project_id)

    return jsonify({
        'success': success,
        'message': message,
        'redis_deleted': redis_deleted
    })


@app.route('/projects')
def list_projects():
    """List all registered projects."""
    projects = []
    for pid, idx in manager.projects.items():
        projects.append({
            'id': pid,
            'name': idx.name,
            'files': len(idx.files),
            'symbols': len(idx.inverted_index)
        })

    return jsonify({
        'projects': projects,
        'global_mode': manager.global_mode
    })


# ============================================================================
# Domain Learning Endpoints (GL-053)
# ============================================================================

# GL-084: _load_universal_domains() removed - use project-domains.json via /aoa-setup


# GL-053 Phase C: Continuous domain learning
_learning_lock = threading.Lock()
_learning_in_progress = set()  # Track which projects are currently learning


def _do_domain_learning(project_id: str):
    """
    Domain learning threshold reached - signal hook mode.

    aOa uses HOOKS to access Haiku, not direct API calls.
    This keeps aOa accessible without requiring API keys.

    GL-062: Fire-and-forget - pending flag and count reset happen in trigger,
    not here. This function just logs that learning was triggered.
    The hook will output instructions, but we don't wait for completion.
    Counter keeps going, and if Claude doesn't complete learning, it triggers
    again in another 10 prompts. This is resilient vs. blocking forever.
    """
    if not DOMAINS_AVAILABLE:
        return

    print(f"[DomainLearning] Ready for hook - {project_id}", flush=True)


def _trigger_domain_learning_if_needed(project_id: str):
    """
    Check if domain learning/tuning should be triggered, spawn background thread if so.

    Called from record_intent() to keep it non-blocking.

    Two thresholds:
    - Learning: every 10 prompts (incremental domain discovery)
    - Tuning: every 100 intents (regenerative optimization)
    """
    if not DOMAINS_AVAILABLE or not project_id:
        return

    try:
        learner = DomainLearner(project_id)

        # GL-083: Simplified - just track prompt count for rebalance
        prompt_count = learner.increment_prompt_count()

        # GL-083: Rebalance trigger (every 25 prompts) - replaces per-prompt learning
        if learner.should_rebalance() and prompt_count > 0:
            rebalance_result = learner.rebalance_keywords()
            # INT-003: Log even when added=0 for observability
            print(f"[Rebalance] {project_id}: +{rebalance_result.get('added', 0)} keywords, {rebalance_result.get('gaps_processed', 0)} gaps processed", flush=True)

        # P2B: Autotune trigger (every 100 prompts) - full domain lifecycle management
        # Uses get_threshold('autotune') for test mode support (100 in prod, 10 in test)
        # See .context/arch/06-autotune.md for full scope (10 operations)
        autotune_interval = int(learner.get_threshold('autotune'))
        if prompt_count > 0 and autotune_interval > 0 and prompt_count % autotune_interval == 0:
            tune_result = learner.run_math_tune()
            if tune_result.get('promoted') or tune_result.get('demoted') or tune_result.get('context_pruned'):
                print(f"[Autotune] {project_id}: promoted={tune_result.get('promoted', 0)}, "
                      f"demoted={tune_result.get('demoted', 0)}, pruned={tune_result.get('context_pruned', 0)}", flush=True)

    except Exception as e:
        print(f"[Rebalance] Trigger error: {e}", flush=True)


# GL-083: Removed _do_domain_tune - tuning now handled by rebalance_keywords()

# =============================================================================
# Project Analysis (GL-083)
# =============================================================================

@app.route('/analyze/project', methods=['POST'])
def analyze_project():
    """
    Analyze project via parallel Haiku to generate project-specific domains.

    GL-083: One-time analysis replaces per-prompt learning.

    POST body: {
        "project_id": "uuid",
        "project_root": "/path/to/project"
    }

    Returns: {
        "success": bool,
        "domains_count": int,
        "terms_count": int,
        "output_file": str (if saved)
    }
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id') or request.args.get('project_id')
    project_root = data.get('project_root')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    if not project_root:
        return jsonify({'error': 'Missing project_root parameter'}), 400

    try:
        learner = DomainLearner(project_id)

        # Check if Anthropic client is available
        if not learner.anthropic_client:
            return jsonify({
                'error': 'Anthropic API key not configured. Set AOA_ANTHROPIC_KEY environment variable.',
                'success': False
            }), 400

        # Get indexed files for this project
        project_files = []
        if project_id in _project_caches:
            cache = _project_caches[project_id]
            project_files = list(cache.keys())

        # Run parallel analysis
        result = learner.analyze_project(project_root, project_files)

        if not result.get('success'):
            return jsonify(result), 400

        # Save to project-domains.json in config directory
        import os
        config_dir = os.path.join(project_root, '.aoa')
        os.makedirs(config_dir, exist_ok=True)
        output_file = os.path.join(config_dir, 'project-domains.json')

        if learner.save_project_domains(result['domains'], output_file):
            result['output_file'] = output_file

        # Also seed these domains into Redis for immediate use
        for d in result['domains']:
            domain = Domain(
                name=d['name'],
                description=d.get('description', ''),
                confidence=0.9,
                terms=d.get('terms', [])
            )
            learner.add_domain(domain, source="analyzed")

        return jsonify({
            'success': True,
            'domains_count': result['domains_count'],
            'terms_count': result['terms_count'],
            'clusters_analyzed': result.get('clusters_analyzed', 0),
            'output_file': result.get('output_file', ''),
            'project_id': project_id
        })

    except Exception as e:
        import traceback
        traceback.print_exc()
        return jsonify({'error': str(e), 'success': False}), 500


@app.route('/domains/seed', methods=['POST'])
def seed_domains():
    """
    DEPRECATED: Universal domain seeding removed in GL-084.

    Use /aoa-setup skill to generate project-specific domains instead.
    """
    return jsonify({
        'error': 'Universal domain seeding removed. Use /aoa-setup to generate project-specific domains.',
        'success': False,
        'deprecated': True
    }), 410  # 410 Gone


# =========================================================================
# GL-085: Lazy Domain Enrichment Endpoints
# =========================================================================

@app.route('/domains/init-skeleton', methods=['POST'])
def domains_init_skeleton():
    """
    Initialize domains from skeleton (names + terms only, no keywords).

    GL-085: Called by /aoa-start skill. Sets enriched=false on all domains.
    Keywords are added lazily via hook-triggered enrichment.

    POST body: {
        "project_id": "xxx",
        "domains": [
            {"name": "@domain", "description": "...", "terms": ["term1", "term2"]}
        ]
    }
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    domains = data.get('domains', [])

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    if not domains:
        return jsonify({'error': 'No domains provided'}), 400

    if len(domains) > 40:
        return jsonify({'error': f'Too many domains ({len(domains)}), max 40'}), 400

    try:
        learner = DomainLearner(project_id)
        result = learner.init_skeleton(domains)
        return jsonify({
            'success': True,
            'project_id': project_id,
            **result
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/unenriched')
def domains_unenriched():
    """
    Get one domain that needs keyword enrichment.

    GL-085: Called by hook to find next domain to enrich.

    Returns: {"domain": {"name": "@x", "description": "...", "terms": [...]}}
    Or: {"domain": null} if all enriched
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        domain = learner.get_unenriched_domain()
        status = learner.get_enrichment_status()
        return jsonify({
            'domain': domain,
            'enrichment': status
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/pending')
def domains_pending():
    """
    Get list of unenriched domain names.

    GL-088: Used by `aoa domains pending` for batch processing.

    Query params:
      - project_id: required
      - limit: max domains to return (default: 10)

    Returns: {"domains": ["@name1", "@name2", ...], "enrichment": {...}}
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    limit = int(request.args.get('limit', 10))

    try:
        learner = DomainLearner(project_id)
        domains = learner.get_unenriched_domains(limit)
        status = learner.get_enrichment_status()
        return jsonify({
            'domains': domains,
            'enrichment': status
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/enrich', methods=['POST'])
def domains_enrich():
    """
    Add keywords to a domain's terms and mark as enriched.

    GL-085: Called after Haiku generates keywords for a domain.

    POST body: {
        "project_id": "xxx",
        "domain": "@domain_name",
        "term_keywords": {
            "term1": ["kw1", "kw2", ...],
            "term2": ["kw3", "kw4", ...]
        }
    }
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    domain_name = data.get('domain')
    term_keywords = data.get('term_keywords', {})

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    if not domain_name:
        return jsonify({'error': 'Missing domain parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        result = learner.enrich_domain(domain_name, term_keywords)
        status = learner.get_enrichment_status()

        # GL-085: Auto-tag files matching the new keywords
        files_tagged = 0
        symbols_tagged = 0
        all_keywords = []
        for keywords in term_keywords.values():
            all_keywords.extend(keywords)

        if all_keywords and intent_index and intent_index.redis:
            try:
                # Get the local index
                idx = manager.get_local(project_id)
                if idx:
                    # Search for each keyword and collect matching files/symbols
                    matched_locations = {}  # file -> set of symbols

                    for keyword in all_keywords[:50]:  # Limit to avoid slowdown
                        # Search symbol index for this keyword
                        matches = idx.search(keyword, mode='recent', limit=100)
                        for match in matches:
                            file_path = match.get('file', '')
                            symbol = match.get('name', '')
                            if file_path and symbol:
                                if file_path not in matched_locations:
                                    matched_locations[file_path] = set()
                                matched_locations[file_path].add(symbol)

                    # Tag the matched files/symbols
                    proj = intent_index._project_key(project_id)
                    r = intent_index.redis.client if hasattr(intent_index.redis, 'client') else intent_index.redis
                    domain_tag = domain_name if domain_name.startswith('@') else f"@{domain_name}"

                    for file_path, symbols in matched_locations.items():
                        # Tag file
                        file_tags_key = f"aoa:{proj}:file_tags:{file_path}"
                        r.sadd(file_tags_key, domain_tag)
                        files_tagged += 1

                        # Tag symbols
                        for symbol in symbols:
                            symbol_key = f"aoa:{proj}:symbol_domains:{file_path}:{symbol}"
                            r.sadd(symbol_key, domain_tag)
                            symbols_tagged += 1

                        # Track symbols in file
                        file_symbols_key = f"aoa:{proj}:file_symbols:{file_path}"
                        r.sadd(file_symbols_key, *symbols)

            except Exception:
                pass  # Don't fail enrichment if tagging fails

        return jsonify({
            'success': True,
            'project_id': project_id,
            **result,
            'enrichment': status,
            'files_tagged': files_tagged,
            'symbols_tagged': symbols_tagged
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/enrichment-status')
def domains_enrichment_status():
    """
    Get enrichment progress for status line display.

    GL-085: Shows X/Y domains enriched.
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        status = learner.get_enrichment_status()
        return jsonify(status)
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/unenrich', methods=['POST'])
def domains_unenrich():
    """
    Mark a domain as unenriched (for refresh/rebuild).

    POST body: {"project_id": "xxx", "domain": "@domain_name"}
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    domain_name = data.get('domain')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400
    if not domain_name:
        return jsonify({'error': 'Missing domain parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        learner.set_domain_enriched(domain_name, False)
        return jsonify({'success': True, 'domain': domain_name, 'enriched': False})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/enrichment-prompt')
def domains_enrichment_prompt():
    """
    Get the Haiku prompt for enriching one domain.

    GL-085: Returns prompt text for the hook to pass to Haiku Task.
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        domain = learner.get_unenriched_domain()

        if not domain:
            return jsonify({
                'prompt': None,
                'domain': None,
                'message': 'All domains enriched'
            })

        prompt = learner.get_enrichment_prompt(domain)
        return jsonify({
            'prompt': prompt,
            'domain': domain['name'],
            'terms': domain['terms']
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/stats')
def domains_stats():
    """Get domain statistics for a project_id."""
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        stats = learner.get_stats()
        # GL-085: Add enrichment status
        enrichment = learner.get_enrichment_status()
        stats['enrichment'] = enrichment
        return jsonify(stats)
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/list')
def domains_list():
    """List all domains with metadata, sorted by hits."""
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    limit = int(request.args.get('limit', 10))
    include_terms = request.args.get('include_terms', '').lower() == 'true'
    include_created = request.args.get('include_created', '').lower() == 'true'
    include_keywords = request.args.get('include_keywords', '').lower() == 'true'

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        domains_with_meta = learner.get_domains_with_meta()

        # Sort by hits descending
        domains_with_meta.sort(key=lambda d: d.get('hits', 0), reverse=True)

        # Format for output
        result = []
        for d in domains_with_meta[:limit]:
            # GL-085: Check enrichment status
            enriched = learner.is_domain_enriched(d['name'])

            entry = {
                'name': d['name'],
                'description': d.get('description', ''),
                'hits': d.get('hits', 0),
                'term_count': len(d.get('terms', [])),
                'confidence': d.get('confidence', 0),
                # GL-059.1: Source and state
                'source': d.get('source', 'seeded'),
                'state': d.get('state', 'active'),
                # GL-085: Enrichment status
                'enriched': enriched,
                # GL-090: Two-tier curation fields
                'tier': d.get('tier', 'core'),
                'total_hits': d.get('total_hits', 0),
                'last_hit_at': d.get('last_hit_at', 0),
            }
            if include_terms:
                terms = list(d.get('terms', []))[:10]  # Limit to 10 terms
                entry['terms'] = terms

                # GL-085: Optionally include keywords for each term
                if include_keywords:
                    term_keywords = {}
                    for term in terms:
                        keywords = learner.get_term_keywords(term)
                        if keywords:
                            term_keywords[term] = list(keywords)[:10]
                    entry['term_keywords'] = term_keywords

            if include_created:
                entry['created'] = d.get('created', 0)
            result.append(entry)

        return jsonify({'domains': result, 'total': len(domains_with_meta)})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/lookup')
def domains_lookup():
    """Look up domain for a symbol or term."""
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    term = request.args.get('term')
    symbol = request.args.get('symbol')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400
    if not term and not symbol:
        return jsonify({'error': 'Missing term or symbol parameter'}), 400

    try:
        learner = DomainLearner(project_id)

        if symbol:
            # Full symbol lookup (tokenize and aggregate)
            domain = learner.get_domain_for_symbol(symbol)
            return jsonify({
                'symbol': symbol,
                'domain': domain,
                'project_id': project_id
            })
        else:
            # Direct term lookup
            results = learner.lookup_term(term)
            return jsonify({
                'term': term,
                'domains': [{'name': name, 'score': score} for name, score in results],
                'project_id': project_id
            })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/learn', methods=['POST'])
def domains_learn():
    """
    Manually trigger domain learning for a project_id.

    GL-053 Phase C: Calls Haiku to discover domains and generate terms
    from recent intent data.

    POST body: {"project_id": "project_id-id"}
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id') or request.args.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)

        # Check if API key is available
        if not learner.anthropic_client:
            return jsonify({
                'error': 'ANTHROPIC_API_KEY not set - domain learning requires Haiku API access',
                'project_id': project_id
            }), 400

        # Get current stats before learning
        before_stats = learner.get_stats()

        # Trigger learning synchronously (for testing)
        _do_domain_learning(project_id)

        # Get stats after learning
        after_stats = learner.get_stats()

        return jsonify({
            'success': True,
            'project_id': project_id,
            'domains_before': before_stats['domains'],
            'domains_after': after_stats['domains'],
            'terms_before': before_stats['total_terms'],
            'terms_after': after_stats['total_terms'],
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/trigger-learn', methods=['POST'])
def domains_trigger_learn():
    """
    GL-070: Trigger hook-side learning by setting learning_pending flag.

    This lets the next UserPromptSubmit hook execute automatic domain generation.
    For testing or manual learning triggers.

    POST body: {"project_id": "project_id-id"}
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id') or request.args.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        learner.set_learning_pending()

        return jsonify({
            'success': True,
            'project_id': project_id,
            'learning_pending': True,
            'message': 'Next UserPromptSubmit will trigger hook-side domain learning'
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/self-learn', methods=['POST'])
def domains_self_learn():
    """
    GL-090: Orphan-based domain learning (replaces path-based junk).

    Checks if:
    1. Context tier has room (< CONTEXT_DOMAINS_MAX)
    2. Orphan tag count >= ORPHAN_THRESHOLD (30)

    If both true, returns orphan tags for Haiku to generate domains.
    Actual domain creation happens via hook or subsequent API call.

    POST body: {"project_id": "uuid"}

    Returns:
    - should_learn: true/false
    - orphans: list of orphan tags (if should_learn)
    - reason: why learning is/isn't needed
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id') or request.args.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)

        # Check 1: Does context tier have room?
        if not learner.can_add_context_domain():
            counts = learner.count_domains_by_tier()
            return jsonify({
                'should_learn': False,
                'reason': f"Context tier full ({counts.get('context', 0)}/{learner.CONTEXT_DOMAINS_MAX})",
                'tier_counts': counts
            })

        # Check 2: Get orphan tags (tags from recent intents that don't match any domain)
        orphans = _get_orphan_tags(project_id, limit=50)

        if len(orphans) < learner.ORPHAN_THRESHOLD:
            return jsonify({
                'should_learn': False,
                'reason': f"Not enough orphan tags ({len(orphans)}/{learner.ORPHAN_THRESHOLD})",
                'orphan_count': len(orphans)
            })

        # Both conditions met - learning should happen
        # Increment intent count for tracking
        learner.increment_intent_count()

        return jsonify({
            'should_learn': True,
            'reason': f"Found {len(orphans)} orphan tags, context tier has room",
            'orphans': orphans[:50],  # Top 50 orphans for Haiku
            'max_domains': min(2, learner.CONTEXT_DOMAINS_MAX - learner.count_domains_by_tier().get('context', 0)),
            'tier_counts': learner.count_domains_by_tier()
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


def _get_orphan_tags(project_id: str, limit: int = 50) -> list[str]:
    """
    Get tags from recent intents that don't match any existing domain term.

    GL-090: Orphan tags indicate semantic gaps - areas users work on
    that aren't covered by existing domains.
    """
    try:
        learner = DomainLearner(project_id)

        # Get all existing terms across all domains
        existing_terms = set()
        for domain_name in learner.get_all_domains():
            terms = learner.get_domain_terms(domain_name)
            existing_terms.update(terms)
            # Also add keywords for each term
            for term in terms:
                keywords = learner.get_term_keywords(term)
                existing_terms.update(keywords)

        # Get recent intent records
        records = intent_index.recent(None, 50, project_id)
        if not records:
            return []

        # Collect tags from intents
        tag_counts = {}
        for r in records:
            tags = r.get('tags', [])
            for tag in tags:
                # Skip domain tags (@...) and very short tags
                if tag.startswith('@') or tag.startswith('#') or len(tag) < 3:
                    continue
                # Clean the tag
                clean_tag = tag.lower().strip()
                # Skip if it matches an existing term
                if clean_tag in existing_terms:
                    continue
                tag_counts[clean_tag] = tag_counts.get(clean_tag, 0) + 1

        # P3-4: Boost with explicit orphan hit counts (from direct searches)
        orphan_hits = learner.get_orphan_hits()
        for tag, hits in orphan_hits.items():
            if tag in tag_counts:
                tag_counts[tag] += hits  # Boost existing tags
            elif tag not in existing_terms and len(tag) >= 3:
                tag_counts[tag] = hits  # Add new tags from direct searches

        # Sort by frequency (including hits) and return top N
        sorted_tags = sorted(tag_counts.items(), key=lambda x: -x[1])
        return [tag for tag, count in sorted_tags[:limit] if count >= 2]

    except Exception as e:
        print(f"[OrphanTags] Error: {e}", flush=True)
        return []


@app.route('/domains/add-context', methods=['POST'])
def domains_add_context():
    """
    GL-090: Add a new context-tier domain from Haiku-generated skeleton.

    POST body: {
        "project_id": "uuid",
        "name": "@domain_name",
        "description": "what this domain covers",
        "terms": ["term1", "term2"]  # optional initial terms
    }

    Called by hook after Haiku generates domain from orphan tags.
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    name = data.get('name', '').strip()
    description = data.get('description', '')
    terms = data.get('terms', [])

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400
    if not name or not name.startswith('@'):
        return jsonify({'error': 'Invalid domain name (must start with @)'}), 400

    try:
        learner = DomainLearner(project_id)

        # Check if context tier has room
        if not learner.can_add_context_domain():
            counts = learner.count_domains_by_tier()
            return jsonify({
                'error': f"Context tier full ({counts.get('context', 0)}/{learner.CONTEXT_DOMAINS_MAX})"
            }), 400

        # Check if domain already exists
        if name in learner.get_all_domains():
            return jsonify({'error': f"Domain {name} already exists"}), 400

        # Create domain in context tier
        domain = Domain(
            name=name,
            description=description,
            confidence=0.6,
            terms=terms
        )
        learner.add_domain(domain, source="intent", tier="context")

        return jsonify({
            'success': True,
            'domain': name,
            'tier': 'context',
            'tier_counts': learner.count_domains_by_tier()
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/autotune', methods=['POST'])
def domains_autotune():
    """
    Manually trigger domain auto-tune for a project_id.

    GL-053 Phase D: Calls Haiku to merge overlapping domains,
    prune low-value domains, and reassign ambiguous terms.

    POST body: {"project_id": "project_id-id"}
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id') or request.args.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)

        # Check if API key is available
        if not learner.anthropic_client:
            return jsonify({
                'error': 'ANTHROPIC_API_KEY not set - auto-tune requires Haiku API access',
                'project_id': project_id
            }), 400

        # Get current stats before auto-tune
        before_stats = learner.get_stats()

        # Run auto-tune
        result = learner.autotune_direct()

        # Get stats after auto-tune
        after_stats = learner.get_stats()

        return jsonify({
            'success': True,
            'project_id': project_id,
            'merged': result.get('merged', 0),
            'pruned': result.get('pruned', 0),
            'reassigned': result.get('reassigned', 0),
            'domains_before': before_stats['domains'],
            'domains_after': after_stats['domains'],
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/tune/math', methods=['POST'])
def domains_tune_math():
    """
    GL-059.3: Run pure math-based tuning - no Haiku needed.

    Algorithm:
    - Prune terms with >30% coverage (too generic)
    - Update domain lifecycle based on hits
    - Flag domains with <2 remaining terms

    POST body: {"project_id": "project_id-id"}
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id') or request.args.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)

        # Get current stats before tune
        before_stats = learner.get_stats()

        # Run math-based tune (GL-059.3)
        summary = learner.run_math_tune()

        # Get stats after tune
        after_stats = learner.get_stats()

        # Record Tune event in intent stream (GL-058)
        tune_cycle = learner.get_tune_count()
        stale_count = summary.get('domains_flagged_stale', 0)
        pruned_count = summary.get('terms_pruned', 0)
        intent_index.record(
            tool="Tune",
            files=[f"tune:{tune_cycle}:{stale_count}:{pruned_count}"],
            tags=["#tuning"],
            session_id="aoa-system",
            tool_use_id=None,
            project_id=project_id
        )

        return jsonify({
            'success': True,
            'project_id': project_id,
            'method': 'math',
            'terms_pruned': summary.get('terms_pruned', 0),
            'domains_active': summary.get('domains_active', 0),
            'domains_flagged_stale': summary.get('domains_flagged_stale', 0),
            'domains_deprecated': summary.get('domains_deprecated', 0),
            'domains_removed': summary.get('domains_removed', 0),  # GL-059.4
            # GL-090: Two-tier curation fields
            'decayed': summary.get('decayed', 0),
            'promoted': summary.get('promoted', 0),
            'demoted': summary.get('demoted', 0),
            'context_pruned': summary.get('context_pruned', 0),
            'domains_before': before_stats['domains'],
            'domains_after': after_stats['domains'],
            'terms_before': before_stats['total_terms'],
            'terms_after': after_stats['total_terms'],
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/tune', methods=['POST'])
def domains_tune():
    """
    Apply regenerative tune results from Haiku via hook (legacy).

    Note: GL-059.3 moves tuning to /domains/tune/math (pure math, no Haiku).

    POST body: {"project_id": "project_id-id", "result": {...haiku response...}}
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id') or request.args.get('project_id')
    result = data.get('result', {})

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    if not result:
        return jsonify({'error': 'Missing result parameter'}), 400

    try:
        learner = DomainLearner(project_id)

        # Get current stats before tune
        before_stats = learner.get_stats()

        # Apply tune
        summary = learner.apply_tune(result)

        # Get stats after tune
        after_stats = learner.get_stats()

        return jsonify({
            'success': True,
            'project_id': project_id,
            'kept': summary.get('kept', 0),
            'added': summary.get('added', 0),
            'removed': summary.get('removed', 0),
            'terms_updated': summary.get('terms_updated', 0),
            'domains_before': before_stats['domains'],
            'domains_after': after_stats['domains'],
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/tuned', methods=['POST'])
def domains_tuned():
    """
    Clear tuning pending flag after hook completes.

    GL-055: Called by hook after Haiku tune is processed.

    POST body: {"project_id": "project_id-id"}
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id') or request.args.get('project_id')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        learner.clear_tuning_pending()
        learner.reset_tune_count()
        learner.set_last_tune()

        return jsonify({
            'success': True,
            'project_id': project_id,
            'message': 'Tuning completed, counter reset'
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/add', methods=['POST'])
def domains_add():
    """
    Add domains discovered by Haiku via hook.

    POST body: {"project_id": "project_id-id", "domains": [{"name": "@domain", "terms": [...]}]}

    Validates input to protect Redis from malformed/hallucinated data.
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    domains_data = data.get('domains', [])
    source = data.get('source', 'learned')  # GL-083: 'analyzed' allows bulk loading

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    # Validation: domains must be a list
    if not isinstance(domains_data, list):
        return jsonify({'error': 'domains must be an array'}), 400

    # Validation: sanity check on count
    # GL-083: Allow more domains for 'analyzed' source (from aoa analyze)
    max_domains = 30 if source == 'analyzed' else 5
    if len(domains_data) > max_domains:
        return jsonify({'error': f'Too many domains ({len(domains_data)}), max {max_domains}'}), 400

    if len(domains_data) == 0:
        return jsonify({'error': 'No domains provided'}), 400

    try:
        learner = DomainLearner(project_id)
        added = []
        terms_added = []
        skipped = []

        for d in domains_data:
            # Validation: must be a dict
            if not isinstance(d, dict):
                skipped.append("non-dict entry")
                continue

            # Validation: name must be a short string
            name = d.get('name', '')
            if not isinstance(name, str) or len(name) < 2 or len(name) > 50:
                skipped.append(f"invalid name length: {name[:20]}")
                continue

            # Validation: name should be alphanumeric + underscore (after @)
            clean_name = name.lstrip('@').replace('_', '')
            if not clean_name.isalnum():
                skipped.append(f"invalid chars in name: {name}")
                continue

            # Normalize name
            if not name.startswith('@'):
                name = f"@{name}"

            # Validation: terms must be a list or dict (v2 format)
            raw_terms = d.get('terms', [])

            # A-004: GL-084 v2 format - terms is dict of {term_name: [keywords]}
            # Preserve the dict to store keywords separately
            terms_dict = None
            if isinstance(raw_terms, dict):
                terms_dict = raw_terms  # A-004: Preserve for keyword storage
                raw_terms = list(raw_terms.keys())
            elif not isinstance(raw_terms, list):
                skipped.append(f"terms not a list or dict for {name}")
                continue

            # Filter terms: must be strings, 2-30 chars, no spaces (single words)
            valid_terms = []
            for t in raw_terms[:50]:  # Allow more terms from v2 format (was 10)
                if isinstance(t, str) and 2 <= len(t) <= 30 and ' ' not in t:
                    valid_terms.append(t.lower())

            if len(valid_terms) == 0:
                skipped.append(f"no valid terms for {name}")
                continue

            # Validation passed - add domain
            domain = Domain(
                name=name,
                description=str(d.get('description', ''))[:200],  # Cap description
                confidence=min(1.0, max(0.0, float(d.get('confidence', 0.8)))),
                terms=valid_terms
            )
            learner.add_domain(domain, source=source)  # GL-083: Pass through source

            # A-004: Store keywords for each term if v2 format provided
            if terms_dict:
                r = learner.redis.client
                for term_name, keywords in terms_dict.items():
                    if term_name.lower() in valid_terms and isinstance(keywords, list):
                        kw_key = f"aoa:{project_id}:term:{term_name.lower()}:keywords"
                        valid_kws = [k.lower() for k in keywords if isinstance(k, str) and len(k) >= 3]
                        if valid_kws:
                            r.sadd(kw_key, *valid_kws)
                            # Also map term to domain for KeywordMatcher
                            domain_key = f"aoa:{project_id}:term:{term_name.lower()}:domain"
                            r.set(domain_key, name)

            added.append(name)
            terms_added.extend(valid_terms)

        # Record what was just added for "Recently Learned" display
        if added:
            learner.set_last_learn(
                terms_count=len(terms_added),
                terms_list=terms_added[:20],
                domains_list=added[:5]
            )

        return jsonify({
            'success': True,
            'project_id': project_id,
            'domains_added': added,
            'terms_added': len(terms_added),
            'skipped': skipped if skipped else None,
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/learned', methods=['POST'])
def domains_learned():
    """
    Signal that hook-based learning completed.
    Clears learning_pending flag, updates timestamp, tracks token investment.

    POST body: {
        "project_id": "project_id-id",
        "domains": [...],
        "terms": [...],
        "prompt_chars": 500,    # Optional: character count of Haiku prompt
        "response_chars": 800   # Optional: character count of Haiku response
    }
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    domains_list = data.get('domains', [])
    terms_list = data.get('terms', [])
    files_list = data.get('files', [])  # GL-071: Files to assign domains to
    locations_list = data.get('locations', [])  # GL-071.1: Rich symbol locations
    prompt_chars = data.get('prompt_chars', 0)
    response_chars = data.get('response_chars', 0)

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        learner.clear_learning_pending()
        learner.set_last_learn(
            terms_count=len(terms_list),
            terms_list=terms_list[:20],
            domains_list=domains_list[:5]
        )

        # Estimate tokens invested (~4 chars per token)
        # If not provided, estimate from content
        if prompt_chars == 0:
            # Estimate: ~600 char prompt template + domain/term content
            prompt_chars = 600 + len(str(domains_list)) + len(str(terms_list))
        if response_chars == 0:
            # Estimate: ~200 chars per domain in response
            response_chars = 200 * max(1, len(domains_list))

        estimated_input_tokens = prompt_chars // 4
        estimated_output_tokens = response_chars // 4
        learner.add_tokens_invested(estimated_input_tokens, estimated_output_tokens)

        total_tokens = estimated_input_tokens + estimated_output_tokens

        # Record Learn event in intent stream (GL-058)
        if domains_list:
            domain_tags = [f"#{d.lstrip('@')}" for d in domains_list[:5]]
            intent_index.record(
                tool="Learn",
                files=[f"learn:{len(domains_list)}:{total_tokens}"],
                tags=domain_tags,
                session_id="aoa-system",
                tool_use_id=None,
                project_id=project_id
            )

        # GL-069.8: Clear orphans that now match new terms
        # After learning adds new terms, check if any orphans now have matches
        orphans_cleared = 0
        if terms_list:
            orphans = learner.get_orphan_tags(limit=100)
            matched_orphans = []
            for orphan in orphans:
                # Check if orphan now matches a term (including new ones)
                domains = learner.get_domains_for_term(orphan)
                if domains:
                    matched_orphans.append(orphan)
            if matched_orphans:
                orphans_cleared = learner.clear_orphan_tags(matched_orphans)

        # GL-071: Assign domains to recent files
        # Roll out newly learned domains to files passed from hook
        files_assigned = 0
        if domains_list and files_list and intent_index and intent_index.redis:
            try:
                proj = intent_index._project_key(project_id)
                r = intent_index.redis.client if hasattr(intent_index.redis, 'client') else intent_index.redis

                for file_path in files_list:
                    # Skip invalid files
                    if not file_path or not isinstance(file_path, str):
                        continue
                    file_tags_key = f"aoa:{proj}:file_tags:{file_path}"
                    for domain in domains_list:
                        # Ensure domain has @ prefix
                        domain_tag = domain if domain.startswith('@') else f"@{domain}"
                        r.sadd(file_tags_key, domain_tag)
                    files_assigned += 1
            except Exception as e:
                # Don't fail the whole request if assignment fails
                pass

        # GL-071.4: Assign domains to symbols (symbol-level storage)
        # Key format: aoa:{proj}:symbol_domains:{file}:{symbol} -> set of @domains
        symbols_assigned = 0
        if domains_list and locations_list and intent_index and intent_index.redis:
            try:
                proj = intent_index._project_key(project_id)
                r = intent_index.redis.client if hasattr(intent_index.redis, 'client') else intent_index.redis

                for loc in locations_list:
                    file_path = loc.get('file', '')
                    symbol = loc.get('symbol', '')
                    parent = loc.get('parent', '')  # GL-071: Prefer parent for domain assignment
                    if not file_path or not symbol:
                        continue

                    # Store domains at PARENT level when available (route/class level)
                    # This allows child symbols to inherit parent's domain
                    target_symbol = parent if parent else symbol
                    symbol_key = f"aoa:{proj}:symbol_domains:{file_path}:{target_symbol}"
                    for domain in domains_list:
                        domain_tag = domain if domain.startswith('@') else f"@{domain}"
                        r.sadd(symbol_key, domain_tag)
                    symbols_assigned += 1

                    # Track which symbols exist in each file (for lookup)
                    file_symbols_key = f"aoa:{proj}:file_symbols:{file_path}"
                    r.sadd(file_symbols_key, target_symbol)
            except Exception as e:
                # Don't fail the whole request if assignment fails
                pass

        return jsonify({
            'success': True,
            'project_id': project_id,
            'learning_pending': False,
            'tokens_invested': total_tokens,
            'tokens_total': learner.get_tokens_invested(),
            'orphans_cleared': orphans_cleared,  # GL-069.8
            'files_assigned': files_assigned,  # GL-071
            'symbols_assigned': symbols_assigned,  # GL-071.4
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/submit-tags', methods=['POST'])
def domains_submit_tags():
    """
    GL-069.1: Per-prompt semantic tag matching.

    Receives tags generated by Claude for a prompt, matches them against
    existing domain terms, increments hits for matches, stores orphans.

    POST body:
        - {"project_id": "...", "tags": ["tag1", "tag2", ...]}  (legacy)
        - {"project_id": "...", "goal": "...", "tags": [{"tag": "name", "score": N}, ...]}  (GL-078)

    Returns:
        - matched: tags that hit existing domain terms
        - orphaned: tags stored for learning cycle
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    tags = data.get('tags', [])
    goal = data.get('goal')

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    # Validation: tags must be a list
    if not isinstance(tags, list):
        return jsonify({'error': 'tags must be an array'}), 400

    # GL-078: Handle scored tags format [{"tag": "name", "score": N}, ...]
    # Flatten to tag names for matching
    tag_names = []
    for tag in tags:
        if isinstance(tag, dict) and 'tag' in tag:
            tag_names.append(tag['tag'])
        elif isinstance(tag, str):
            tag_names.append(tag)

    # Validation: sanity check on count (7-10 tags per prompt)
    if len(tag_names) > 10:
        tag_names = tag_names[:10]  # Truncate, don't reject

    if len(tag_names) == 0:
        return jsonify({'matched': [], 'orphaned': [], 'message': 'No tags provided'})

    try:
        learner = DomainLearner(project_id)

        # GL-078: Store prompt record (goal + tags) for grouped display/learning
        if goal and tags:
            learner.add_prompt_record(goal, tags)

        # Match tags against existing terms
        result = learner.match_tags_to_terms(tag_names)

        return jsonify({
            'success': True,
            'project_id': project_id,
            'matched': result['matched'],
            'matched_count': len(result['matched']),
            'orphaned': result['orphaned'],
            'orphan_count': len(result['orphaned']),
            'total_orphans': learner.get_orphan_count(),
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/unmatched-tags')
def domains_unmatched_tags():
    """
    GL-069.6: Get orphan tags for learning cycle.

    Orphan tags are semantic tags that didn't match any existing domain terms.
    They represent unmet intent and inform domain discovery.

    Query params:
        project_id: project_id ID
        limit: max orphans to return (default 50)
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    limit = int(request.args.get('limit', 50))

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        orphans = learner.get_orphan_tags(limit=limit)

        return jsonify({
            'project_id': project_id,
            'orphans': orphans,
            'count': len(orphans),
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/domains/goal-history')
def domains_goal_history():
    """
    GL-078: Get recent prompt records (goal + tags) for display/learning.

    Returns the last N prompts with their goals and associated tags.
    Used for grouped display in `aoa domains` and for GL-072 learning.

    Query params:
        project_id: project_id ID
        limit: max prompts to return (default 10)
    """
    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning module not available'}), 500

    project_id = request.args.get('project_id')
    limit = int(request.args.get('limit', 10))

    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        learner = DomainLearner(project_id)
        prompts = learner.get_prompt_records(limit=limit)

        return jsonify({
            'project_id': project_id,
            'prompts': prompts,
            'count': len(prompts),
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/files')
def files_search():
    start = time.time()
    pattern = request.args.get('match')
    mode = request.args.get('mode', 'recent')
    limit = int(request.args.get('limit', 50))
    project_id = request.args.get('project_id')

    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'error': 'No index available', 'results': [], 'ms': 0}), 404

    results = idx.list_files(pattern, mode, limit)

    return jsonify({
        'results': results,
        'index': idx.name,
        'project_id': project_id,
        'ms': (time.time() - start) * 1000
    })

@app.route('/changes')
def changes():
    start = time.time()
    since_param = request.args.get('since', '300')
    project_id = request.args.get('project_id')

    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'error': 'No index available', 'added': [], 'modified': [], 'deleted': [], 'ms': 0}), 404

    if since_param == 'session':
        since = idx.session_start
    else:
        since = int(time.time()) - int(since_param)

    changes_list = idx.changes_since(since)

    added = [c['file'] for c in changes_list if c['change_type'] == 'added']
    modified = [{'file': c['file'], 'lines_changed': c.get('lines_changed', [])}
                for c in changes_list if c['change_type'] == 'modified']
    deleted = [c['file'] for c in changes_list if c['change_type'] == 'deleted']

    return jsonify({
        'added': added,
        'modified': modified,
        'deleted': deleted,
        'index': idx.name,
        'project_id': project_id,
        'ms': (time.time() - start) * 1000
    })

@app.route('/file')
def file_content():
    start = time.time()
    path = request.args.get('path', '')
    lines = request.args.get('lines')
    symbol = request.args.get('symbol')

    full_path = manager.get_local().root / path
    if not full_path.exists():
        return jsonify({'error': 'File not found'}), 404

    content = full_path.read_text(encoding='utf-8', errors='ignore')
    all_lines = content.split('\n')

    if lines:
        parts = lines.split('-')
        start_l = int(parts[0]) - 1
        end_l = int(parts[1]) if len(parts) > 1 else len(all_lines)
        extracted = '\n'.join(all_lines[start_l:end_l])
        return jsonify({
            'content': extracted,
            'lines': (start_l + 1, end_l),
            'ms': (time.time() - start) * 1000
        })
    elif symbol:
        for i, line in enumerate(all_lines):
            if symbol in line:
                start_l = max(0, i - 5)
                end_l = min(len(all_lines), i + 20)
                extracted = '\n'.join(all_lines[start_l:end_l])
                return jsonify({
                    'content': extracted,
                    'lines': (start_l + 1, end_l),
                    'ms': (time.time() - start) * 1000
                })
        return jsonify({'error': 'Symbol not found'}), 404
    else:
        return jsonify({
            'content': content,
            'lines': (1, len(all_lines)),
            'ms': (time.time() - start) * 1000
        })

@app.route('/file/meta')
def file_meta():
    """Get file metadata (size, language, mtime) for baseline calculations."""
    path = request.args.get('path', '')
    project_id = request.args.get('project_id')

    idx = manager.get_local(project_id)
    if not idx:
        return jsonify({'error': 'No index available'}), 404

    # Look up file in index
    if path in idx.files:
        meta = idx.files[path]
        return jsonify({
            'path': path,
            'size': meta.size,
            'language': meta.language,
            'mtime': meta.mtime,
            'tokens_estimate': meta.size // 4  # Claude uses ~4 chars per token
        })
    else:
        return jsonify({'error': 'File not in index'}), 404


@app.route('/deps')
def deps():
    start = time.time()
    file = request.args.get('file')
    direction = request.args.get('direction', 'outgoing')
    local = manager.get_local()

    if not file:
        return jsonify({'error': 'file parameter required'}), 400

    with local.lock:
        if direction == 'outgoing':
            results = local.deps_outgoing.get(file, [])
        else:
            results = local.deps_incoming.get(file, [])

    return jsonify({
        'dependencies': results,
        'direction': direction,
        'ms': (time.time() - start) * 1000
    })

@app.route('/structure')
def structure():
    start = time.time()
    focus = request.args.get('focus', '')
    depth = int(request.args.get('depth', 2))
    local = manager.get_local()

    root = local.root / focus if focus else local.root

    def build_tree(path: Path, current_depth: int) -> dict:
        if current_depth > depth:
            return None

        result = {'name': path.name, 'type': 'dir' if path.is_dir() else 'file'}

        if path.is_dir():
            children = []
            try:
                for child in sorted(path.iterdir()):
                    if child.name.startswith('.'):
                        continue
                    if child.name in CodebaseIndex.IGNORE_DIRS:
                        continue
                    subtree = build_tree(child, current_depth + 1)
                    if subtree:
                        children.append(subtree)
            except PermissionError:
                pass
            result['children'] = children

        return result

    tree = build_tree(root, 0)

    return jsonify({
        'tree': tree,
        'ms': (time.time() - start) * 1000
    })


# ============================================================================
# API Endpoints - Repo Management
# ============================================================================

@app.route('/repos', methods=['GET'])
def list_repos():
    """List all knowledge repos."""
    return jsonify({
        'repos': manager.list_repos()
    })

@app.route('/repos', methods=['POST'])
def add_repo():
    """Add a new knowledge repo."""
    data = request.json
    name = data.get('name')
    url = data.get('url')

    if not name or not url:
        return jsonify({'error': 'name and url required'}), 400

    # Sanitize name
    name = re.sub(r'[^a-zA-Z0-9_-]', '', name)
    if not name:
        return jsonify({'error': 'Invalid repo name'}), 400

    success, message = manager.add_repo(name, url)

    if success:
        return jsonify({'success': True, 'message': message})
    else:
        return jsonify({'success': False, 'error': message}), 400

@app.route('/repos/<name>', methods=['DELETE'])
def remove_repo(name):
    """Remove a knowledge repo."""
    success, message = manager.remove_repo(name)

    if success:
        return jsonify({'success': True, 'message': message})
    else:
        return jsonify({'success': False, 'error': message}), 404


# ============================================================================
# API Endpoints - Repo Search (isolated)
# ============================================================================

@app.route('/repo/<name>/symbol')
def repo_symbol_search(name):
    """Search in a specific repo only."""
    start = time.time()

    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    q = request.args.get('q', '')
    mode = request.args.get('mode', 'recent')
    limit = int(request.args.get('limit', 20))

    results = repo.search(q, mode, limit)

    return jsonify({
        'results': results,
        'index': name,
        'ms': (time.time() - start) * 1000
    })

@app.route('/repo/<name>/multi', methods=['POST'])
def repo_multi_search(name):
    """Multi-term search in a specific repo only."""
    start = time.time()

    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    data = request.json
    terms = data.get('terms', [])
    mode = data.get('mode', 'recent')
    limit = int(data.get('limit', 20))

    results = repo.search_multi(terms, mode, limit)

    return jsonify({
        'results': results,
        'index': name,
        'ms': (time.time() - start) * 1000
    })

@app.route('/repo/<name>/files')
def repo_files(name):
    """List files in a specific repo."""
    start = time.time()

    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    pattern = request.args.get('match')
    mode = request.args.get('mode', 'recent')
    limit = int(request.args.get('limit', 50))

    results = repo.list_files(pattern, mode, limit)

    return jsonify({
        'results': results,
        'index': name,
        'ms': (time.time() - start) * 1000
    })

@app.route('/repo/<name>/file')
def repo_file_content(name):
    """Get file content from a specific repo."""
    start = time.time()

    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    path = request.args.get('path', '')
    lines = request.args.get('lines')

    full_path = repo.root / path
    if not full_path.exists():
        return jsonify({'error': 'File not found'}), 404

    content = full_path.read_text(encoding='utf-8', errors='ignore')
    all_lines = content.split('\n')

    if lines:
        parts = lines.split('-')
        start_l = int(parts[0]) - 1
        end_l = int(parts[1]) if len(parts) > 1 else len(all_lines)
        extracted = '\n'.join(all_lines[start_l:end_l])
        return jsonify({
            'content': extracted,
            'lines': (start_l + 1, end_l),
            'index': name,
            'ms': (time.time() - start) * 1000
        })
    else:
        return jsonify({
            'content': content,
            'lines': (1, len(all_lines)),
            'index': name,
            'ms': (time.time() - start) * 1000
        })

@app.route('/repo/<name>/deps')
def repo_deps(name):
    """Get dependencies from a specific repo."""
    start = time.time()

    repo = manager.get_repo(name)
    if not repo:
        return jsonify({'error': f"Repo '{name}' not found"}), 404

    file = request.args.get('file')
    direction = request.args.get('direction', 'outgoing')

    if not file:
        return jsonify({'error': 'file parameter required'}), 400

    with repo.lock:
        if direction == 'outgoing':
            results = repo.deps_outgoing.get(file, [])
        else:
            results = repo.deps_incoming.get(file, [])

    return jsonify({
        'dependencies': results,
        'direction': direction,
        'index': name,
        'ms': (time.time() - start) * 1000
    })


# ============================================================================
# API Endpoints - Pattern Search (Agent-driven AC)
# ============================================================================

@app.route('/pattern', methods=['POST'])
def pattern_search():
    """DEPRECATED: Use /grep?regex=true instead. Multi-pattern search is no longer supported."""
    return jsonify({
        'error': 'Endpoint deprecated. Use /grep?regex=true for regex search.',
        'migration': 'aoa egrep "pattern" now uses /grep?regex=true'
    }), 410


@app.route('/repo/<name>/pattern', methods=['POST'])
def repo_pattern_search(name):
    """DEPRECATED: Use /grep?regex=true&repo=name instead."""
    return jsonify({
        'error': 'Endpoint deprecated. Use /grep?regex=true for regex search.',
        'migration': 'aoa egrep "pattern" --repo name now uses /grep?regex=true'
    }), 410


# ============================================================================
# API Endpoints - Intent Tracking
# ============================================================================


def resolve_file_locations(files: list[str], idx) -> list[dict]:
    """
    GL-069.5: Resolve file:line references to rich symbol locations.

    Takes a list of file paths (some with :line suffixes) and resolves them
    to enclosing function/class symbols using the index's outline data.

    Returns list of dicts with: file, symbol, kind, parent, qualified, start_line, end_line
    """
    if not files or not idx:
        return []

    locations = []
    seen_symbols = set()  # Dedupe: same function accessed multiple times = 1

    # Get index root for path normalization
    idx_root = str(idx.root) if hasattr(idx, 'root') else ''

    for file_ref in files:
        # Skip patterns and commands
        if file_ref.startswith('pattern:') or file_ref.startswith('cmd:'):
            continue

        # Extract file path and line number
        if ':' not in file_ref:
            continue

        parts = file_ref.rsplit(':', 1)
        file_path = parts[0]
        try:
            # Handle ranges like "100-150" or offsets like "100+"
            line_str = parts[1].split('-')[0].rstrip('+')
            line_num = int(line_str)
        except (ValueError, IndexError):
            continue

        # Normalize path: try multiple variants to find match
        # Index stores relative paths, but hooks send absolute paths
        symbols = idx.file_outlines.get(file_path, [])

        if not symbols:
            # Try stripping index root
            if idx_root and file_path.startswith(idx_root):
                rel_path = file_path[len(idx_root):].lstrip('/')
                symbols = idx.file_outlines.get(rel_path, [])

        if not symbols:
            # Try finding relative path by suffix matching
            # e.g., /home/user/proj/services/foo.py -> services/foo.py
            for indexed_path in idx.file_outlines.keys():
                if file_path.endswith('/' + indexed_path) or file_path == indexed_path:
                    symbols = idx.file_outlines[indexed_path]
                    file_path = indexed_path  # Use indexed path for qualified name
                    break

        if not symbols:
            continue

        # Find innermost enclosing symbol
        enclosing = []
        for sym in symbols:
            if sym.start_line <= line_num <= sym.end_line:
                sym_range = sym.end_line - sym.start_line
                enclosing.append((sym_range, sym))

        if not enclosing:
            continue

        # Sort by range (smallest first = most specific)
        enclosing.sort(key=lambda x: x[0])
        _, best_match = enclosing[0]

        # Dedupe by symbol
        symbol_key = f"{file_path}:{best_match.name}"
        if symbol_key in seen_symbols:
            continue
        seen_symbols.add(symbol_key)

        # Find parent (for class.method style)
        parent_name = None
        if len(enclosing) > 1:
            _, parent = enclosing[1]
            parent_name = parent.name

        locations.append({
            'file': file_path,
            'symbol': best_match.name,
            'kind': best_match.kind,
            'parent': parent_name,
            'start_line': best_match.start_line,
            'end_line': best_match.end_line,
            'qualified': f"{file_path}:{parent_name}.{best_match.name}()" if parent_name else f"{file_path}:{best_match.name}()"
        })

    return locations


@app.route('/intent', methods=['POST'])
def record_intent():
    """
    Record an intent from tool usage.

    POST body:
    {
        "tool": "Edit",
        "files": ["/path/to/file.py"],
        "tags": ["#authentication", "#editing"],
        "session_id": "abc123",
        "tool_use_id": "toolu_xxx",  # Claude's correlation key
        "project_id": "uuid-here"    # Per-project_id isolation
    }
    """
    data = request.json

    tool = data.get('tool', 'unknown')
    files = data.get('files', [])
    tags = data.get('tags', [])
    session_id = data.get('session_id', 'unknown')
    tool_use_id = data.get('tool_use_id')  # Claude's toolu_xxx ID
    project_id = data.get('project_id')  # UUID for per-project_id isolation
    file_sizes = data.get('file_sizes', {})  # File path -> size for baseline calc
    output_size = data.get('output_size')  # Actual output size for REAL savings calc

    # GL-069.5: Resolve file:line references to rich symbol locations
    idx = manager.get_local(project_id)
    locations = resolve_file_locations(files, idx) if idx else []

    intent_index.record(tool, files, tags, session_id, tool_use_id, project_id, file_sizes, output_size, locations)

    # GL-062: Feed scorer for prediction ranking
    # Record file access to build recency/frequency/tag data for predictions
    if RANKING_AVAILABLE and scorer is not None:
        for file_path in files:
            # Skip patterns and commands, only score real files
            if file_path.startswith('pattern:') or file_path.startswith('cmd:'):
                continue
            # Extract base path (remove :line-range suffixes)
            base_path = file_path.split(':')[0] if ':' in file_path else file_path
            try:
                scorer.record_access(base_path, tags=tags)
            except Exception:
                pass  # Don't block on scorer errors

    # GL-060.3: Match intent tags against domain terms, increment hits
    # GL-090 BUG-002 FIX: Also collect orphan tags for learning cycle
    debug_mode = os.environ.get("AOA_DEBUG") == "1"
    if debug_mode and tags:
        print(f"[INTENT DEBUG] Processing tags: {tags}, project_id: {project_id}", flush=True)

    if DOMAINS_AVAILABLE and project_id and tags:
        try:
            learner = DomainLearner(project_id)
            orphaned_tags = []
            for tag in tags:
                # Normalize tag (remove # prefix if present)
                term = tag.lstrip('#').lower()
                if len(term) < 2:
                    continue
                # Find domains that have this term
                domains_with_term = learner.get_domains_for_term(term)
                if debug_mode:
                    print(f"[INTENT DEBUG] tag='{tag}' -> term='{term}' -> domains={domains_with_term}", flush=True)
                if domains_with_term:
                    for domain_name in domains_with_term:
                        learner.increment_domain_hits(domain_name)
                else:
                    # Tag didn't match any domain - it's an orphan
                    orphaned_tags.append(term)
            # Store orphaned tags for learning cycle
            if orphaned_tags:
                if debug_mode:
                    print(f"[INTENT DEBUG] Orphaned tags (no domain match): {orphaned_tags}", flush=True)
                learner.add_orphan_tags(orphaned_tags)
        except Exception as e:
            if debug_mode:
                print(f"[INTENT DEBUG] Error in domain matching: {e}", flush=True)
            pass  # Don't block intent recording on domain errors

    # GL-053 Phase C: Trigger domain learning if threshold reached
    _trigger_domain_learning_if_needed(project_id)

    # GL-088: Check if enrichment should be triggered (every 25 prompts)
    enrichment_ready = False
    prompt_count = 0
    if DOMAINS_AVAILABLE and project_id:
        try:
            learner = DomainLearner(project_id)
            prompt_count = learner.get_prompt_count()
            # Signal enrichment_ready when prompt_count hits 25, 50, 75, etc.
            enrichment_ready = (prompt_count > 0 and prompt_count % 25 == 0)
        except Exception:
            pass

    return jsonify({
        'success': True,
        'enrichment_ready': enrichment_ready,
        'prompt_count': prompt_count
    })


@app.route('/intent/tags')
def intent_tags():
    """Get all intent tags with counts."""
    project_id = request.args.get('project_id')
    tags = intent_index.all_tags(project_id)
    return jsonify({
        'tags': [{'tag': t, 'count': c} for t, c in tags],
        'project_id': project_id
    })


@app.route('/intent/files')
def intent_files_for_tag():
    """Get files associated with a tag."""
    tag = request.args.get('tag', '')
    project_id = request.args.get('project_id')
    if not tag.startswith('#'):
        tag = '#' + tag

    files = intent_index.files_for_tag(tag, project_id)
    return jsonify({
        'tag': tag,
        'files': files,
        'project_id': project_id
    })


@app.route('/intent/file')
def intent_tags_for_file():
    """Get tags associated with a file."""
    file = request.args.get('path', '')
    project_id = request.args.get('project_id')
    tags = intent_index.tags_for_file(file, project_id)
    return jsonify({
        'file': file,
        'tags': tags,
        'project_id': project_id
    })


@app.route('/intent/recent')
def intent_recent():
    """Get recent intent records."""
    since = request.args.get('since')
    limit = int(request.args.get('limit', 50))
    project_id = request.args.get('project_id')

    since_ts = None
    if since:
        since_ts = int(time.time()) - int(since)

    records = intent_index.recent(since_ts, limit, project_id)
    return jsonify({
        'records': records,
        'stats': intent_index.get_stats(project_id)
    })


@app.route('/intent/session')
def intent_session():
    """Get intent records for a session."""
    session_id = request.args.get('id', '')
    project_id = request.args.get('project_id')
    records = intent_index.session(session_id, project_id)
    return jsonify({
        'session_id': session_id,
        'records': records,
        'project_id': project_id
    })


@app.route('/intent/stats')
def intent_stats():
    """Get intent index statistics."""
    project_id = request.args.get('project_id')
    return jsonify(intent_index.get_stats(project_id))


@app.route('/cc/prompts')
def cc_prompts():
    """Get recent user prompts from Claude Code session history.

    Query params:
        project_path: Project directory (default: CODEBASE_ROOT)
        limit: Number of prompts (default: 25)

    Returns clean user prompts with system-generated content stripped.
    """
    try:
        from metrics import SessionMetrics
    except ImportError:
        try:
            from session.metrics import SessionMetrics
        except ImportError:
            return jsonify({'error': 'SessionMetrics not available', 'prompts': []})

    try:
        limit = int(request.args.get('limit', 25))
        project_path = request.args.get('project_path', os.environ.get('CODEBASE_ROOT', '/app'))

        metrics = SessionMetrics(project_path)
        prompts = metrics.get_prompts(limit=limit)

        # Extract just the text for backward compatibility
        prompt_texts = [p.get('text', '') for p in prompts]

        return jsonify({
            'prompts': prompt_texts,
            'count': len(prompt_texts),
            'project_path': project_path
        })
    except Exception as e:
        return jsonify({'error': str(e), 'prompts': []})


@app.route('/cc/sessions')
def cc_sessions():
    """Get per-session metrics for 'aoa cc sessions' view.

    Query params:
        project_path: Project directory (default: CODEBASE_ROOT)
        limit: Number of sessions (default: 10)

    Returns session summaries with tokens, velocity, model counts, tool counts.
    """
    try:
        from metrics import SessionMetrics
    except ImportError:
        try:
            from session.metrics import SessionMetrics
        except ImportError:
            return jsonify({'error': 'SessionMetrics not available', 'sessions': []})

    try:
        limit = int(request.args.get('limit', 10))
        project_path = request.args.get('project_path', os.environ.get('CODEBASE_ROOT', '/app'))

        metrics = SessionMetrics(project_path)
        sessions = metrics.get_sessions_summary(limit=limit)

        return jsonify({
            'sessions': sessions,
            'count': len(sessions),
            'project_path': project_path
        })
    except Exception as e:
        return jsonify({'error': str(e), 'sessions': []})


@app.route('/cc/stats')
def cc_stats():
    """Get aggregated stats for 'aoa cc stats' view.

    Query params:
        project_path: Project directory (default: CODEBASE_ROOT)

    Returns stats broken down by today, 7d, 30d periods.
    """
    try:
        from metrics import SessionMetrics
    except ImportError:
        try:
            from session.metrics import SessionMetrics
        except ImportError:
            return jsonify({'error': 'SessionMetrics not available', 'periods': {}, 'model_distribution': {}})

    try:
        project_path = request.args.get('project_path', os.environ.get('CODEBASE_ROOT', '/app'))

        metrics = SessionMetrics(project_path)
        stats = metrics.get_stats()

        return jsonify(stats)
    except Exception as e:
        return jsonify({'error': str(e), 'periods': {}, 'model_distribution': {}})


@app.route('/intent/summary')
def intent_summary():
    """GL-088: Get work summary for last N prompts (for Haiku enrichment).

    Query params:
        project_id: Project to query
        limit: Number of recent intents (default 25)

    Returns:
        {
            'prompts': 25,
            'files': ['file1.py', 'file2.py', ...],  // Unique files touched
            'file_count': 15,
            'summary': 'user worked on authentication, search, API endpoints'
        }
    """
    project_id = request.args.get('project_id')
    limit = int(request.args.get('limit', 25))

    records = intent_index.recent(None, limit, project_id)

    # Collect unique files (excluding patterns and commands)
    files = set()
    tools_used = set()
    for r in records:
        tools_used.add(r.get('tool', 'unknown'))
        for f in r.get('files', []):
            if f.startswith('pattern:') or f.startswith('cmd:'):
                continue
            # Strip line references
            base_path = f.split(':')[0] if ':' in f else f
            if base_path and '/' in base_path:
                files.add(base_path)

    # Sort by recency (most recent files first)
    files_list = list(files)[:50]  # Limit to 50 files

    return jsonify({
        'prompts': len(records),
        'files': files_list,
        'file_count': len(files),
        'tools_used': list(tools_used),
        'project_id': project_id
    })


@app.route('/intent/rolling')
def intent_rolling():
    """GL-045: Get rolling intent window with tag scores.

    Query params:
        project_id: Project to query
        window: Number of recent intents (default 50)
    """
    project_id = request.args.get('project_id')
    window = int(request.args.get('window', 50))

    # O1-009/O1-010: Get both in one call
    affinity, rolling = intent_index.get_file_affinity(project_id, window)

    return jsonify({
        'rolling_tags': rolling,
        'file_affinity': affinity,
        'window': window,
        'project_id': project_id
    })


@app.route('/intent/hits')
def intent_hits():
    """GL-088: Get top-hit domains and terms from grep searches.

    Query params:
        project_id: Project to query
        limit: Max results per category (default 10)

    Returns:
        {
            'domains': [{'name': '@auth', 'hits': 42}, ...],
            'terms': [{'name': '#token', 'hits': 15}, ...],
            'recent': ['@auth', '#token', ...]  // Most recent unique hits
        }
    """
    project_id = request.args.get('project_id')
    limit = int(request.args.get('limit', 10))

    hits = intent_index.get_top_hits(project_id, limit)

    return jsonify({
        'domains': hits.get('domains', []),
        'terms': hits.get('terms', []),
        'recent': hits.get('recent', []),
        'project_id': project_id
    })


@app.route('/metrics/token-rate')
def metrics_token_rate():
    """
    Calculate actual ms_per_token rate from session history.

    Derives the real processing rate from Claude session timestamps and token counts.
    This rate can be used to estimate time savings from token savings.

    Returns:
        ms_per_token: Median milliseconds per token processed
        range: Min/max/p25/p75 for showing variability
        samples: Number of data points analyzed
        confidence: high (50+), medium (20+), or low (<20)
        methodology: How the rate was calculated
    """
    import os
    from pathlib import Path

    from services.ranking.session_parser import SessionLogParser

    # Find Claude projects directory
    home = os.path.expanduser('~')
    projects_dir = Path(home) / '.claude' / 'projects'

    if not projects_dir.exists():
        return jsonify({
            'ms_per_token': 0,
            'samples': 0,
            'confidence': 'none',
            'error': 'No Claude projects directory found'
        })

    # Get the most recent project_id directory
    project_dirs = sorted(projects_dir.iterdir(), key=lambda p: p.stat().st_mtime, reverse=True)
    if not project_dirs:
        return jsonify({
            'ms_per_token': 0,
            'samples': 0,
            'confidence': 'none',
            'error': 'No session data found'
        })

    # Use the most recent project_id for rate calculation
    parser = SessionLogParser(project_dirs[0])
    rate_data = parser.calculate_token_rate()

    return jsonify(rate_data)


# ============================================================================
# Prediction Tracking API - Phase 2 Session Correlation + Phase 4 Rolling Metrics
# ============================================================================

# Rolling window constants for Hit@5 calculation
ROLLING_WINDOW_HOURS = 24
ROLLING_WINDOW_SECONDS = ROLLING_WINDOW_HOURS * 3600

@app.route('/predict/log', methods=['POST'])
def log_prediction():
    """
    Log a prediction for later hit/miss comparison.

    POST body:
    {
        "session_id": "uuid-xxx",
        "predicted_files": ["/src/file1.py", "/src/file2.py"],
        "tags": ["python", "api"],
        "trigger_file": "/src/current.py",
        "confidence": 0.85
    }

    Phase 4: Also logs to rolling ZSET for Hit@5 calculation over 24h window.
    """
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    data = request.json
    session_id = data.get('session_id', 'unknown')
    predicted_files = data.get('predicted_files', [])
    tags = data.get('tags', [])
    trigger_file = data.get('trigger_file', '')
    confidence = data.get('confidence', 0.0)

    if not predicted_files:
        return jsonify({'success': True, 'logged': 0})

    try:
        # Store prediction in Redis with TTL (60 seconds - predictions expire)
        import time as time_module
        timestamp = time_module.time()
        timestamp_ms = int(timestamp * 1000)
        prediction_key = f"aoa:prediction:{session_id}:{timestamp_ms}"

        prediction_data = {
            'session_id': session_id,
            'timestamp_ms': timestamp_ms,
            'predicted_files': predicted_files,
            'tags': tags,
            'trigger_file': trigger_file,
            'confidence': confidence,
            'hit': None  # Will be set by /predict/check
        }

        # Store prediction with 24h TTL (matches rolling window for proper hit tracking)
        scorer.redis.client.setex(
            prediction_key,
            ROLLING_WINDOW_SECONDS,  # 24 hour TTL - predictions persist for rolling analysis
            json.dumps(prediction_data)
        )

        # Also add to session's prediction list for quick lookup
        # R-003: Cap list to 100 items to prevent unbounded growth
        session_predictions_key = f"aoa:predictions:{session_id}"
        scorer.redis.client.lpush(session_predictions_key, prediction_key)
        scorer.redis.client.ltrim(session_predictions_key, 0, 99)
        scorer.redis.client.expire(session_predictions_key, ROLLING_WINDOW_SECONDS)  # 24h TTL to match rolling window

        # Phase 4: Add to rolling predictions ZSET for Hit@5 calculation
        # Score = timestamp, Member = prediction_id
        # This persists beyond the 60s TTL for rolling metrics
        rolling_key = "aoa:rolling:predictions"
        scorer.redis.client.zadd(rolling_key, {prediction_key: timestamp})

        # Store prediction data in a hash that persists for rolling window
        rolling_data_key = f"aoa:rolling:data:{prediction_key}"
        scorer.redis.client.hset(rolling_data_key, mapping={
            'session_id': session_id,
            'timestamp': str(timestamp),
            'predicted_files': json.dumps(predicted_files[:5]),  # Top 5 for Hit@5
            'hit': '',  # Empty = not yet evaluated
        })
        scorer.redis.client.expire(rolling_data_key, ROLLING_WINDOW_SECONDS + 3600)  # 25h TTL

        # Cleanup: Remove predictions older than rolling window
        cutoff = timestamp - ROLLING_WINDOW_SECONDS
        scorer.redis.client.zremrangebyscore(rolling_key, 0, cutoff)

        return jsonify({
            'success': True,
            'logged': len(predicted_files),
            'prediction_key': prediction_key
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/predict/check', methods=['POST'])
def check_prediction_hit():
    """
    Check if a file access was predicted (called by intent-capture after Read).

    POST body:
    {
        "session_id": "uuid-xxx",
        "file": "/src/file.py"
    }

    Returns whether this file was in recent predictions.

    Phase 4: Also updates rolling data for Hit@5 calculation.
    A prediction batch is a "hit" if ANY of the top 5 files were read.
    """
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'hit': False, 'error': 'Redis not available'}), 503

    data = request.json
    session_id = data.get('session_id', 'unknown')
    file_path = data.get('file', '')
    project_id = data.get('project_id', '')  # UUID for per-project_id metrics

    if not file_path:
        return jsonify({'hit': False})

    try:
        # Get recent predictions for this session
        session_predictions_key = f"aoa:predictions:{session_id}"
        prediction_keys = scorer.redis.client.lrange(session_predictions_key, 0, 10)

        for pred_key in prediction_keys:
            pred_key_str = pred_key.decode() if isinstance(pred_key, bytes) else pred_key
            pred_data = scorer.redis.client.get(pred_key_str)
            if pred_data:
                prediction = json.loads(pred_data)
                if file_path in prediction.get('predicted_files', []):
                    # Record the hit - global (system monitoring)
                    scorer.redis.client.incr('aoa:metrics:hits')

                    # Record per-project_id hit count (NOT fabricated savings)
                    # Real savings are calculated when we have both baseline + actual output
                    if project_id:
                        scorer.redis.client.incr(f'aoa:{project_id}:metrics:hits')

                    # Phase 4: Mark the prediction batch as a hit in rolling data
                    rolling_data_key = f"aoa:rolling:data:{pred_key_str}"
                    current_hit = scorer.redis.client.hget(rolling_data_key, 'hit')
                    if current_hit is not None:
                        # Only mark as hit if not already evaluated
                        current_hit_str = current_hit.decode() if isinstance(current_hit, bytes) else current_hit
                        if current_hit_str == '':
                            scorer.redis.client.hset(rolling_data_key, 'hit', '1')

                    return jsonify({
                        'hit': True,
                        'prediction_key': pred_key_str,
                        'confidence': prediction.get('confidence', 0)
                    })

        # No hit - record miss (global)
        scorer.redis.client.incr('aoa:metrics:misses')
        if project_id:
            scorer.redis.client.incr(f'aoa:{project_id}:metrics:misses')

        # Phase 4: Mark any unevaluated predictions as misses after a file read
        # (This is conservative - we only mark miss if we checked and didn't find a hit)
        # Note: We don't mark as miss here because the user might still read a predicted file later

        return jsonify({'hit': False})

    except Exception as e:
        return jsonify({'hit': False, 'error': str(e)}), 500


@app.route('/predict/stats')
def prediction_stats():
    """
    Get prediction hit/miss statistics.

    Phase 4: Includes rolling Hit@5 over 24h window.
    """
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    project_id = request.args.get('project_id')

    try:
        # Legacy cumulative counters (per-project_id if project_id provided)
        if project_id:
            hits = int(scorer.redis.client.get(f'aoa:{project_id}:metrics:hits') or 0)
            misses = int(scorer.redis.client.get(f'aoa:{project_id}:metrics:misses') or 0)
        else:
            hits = int(scorer.redis.client.get('aoa:metrics:hits') or 0)
            misses = int(scorer.redis.client.get('aoa:metrics:misses') or 0)

        total = hits + misses
        hit_rate = (hits / total * 100) if total > 0 else 0

        # Phase 4: Calculate rolling Hit@5 over 24h window
        rolling_stats = calculate_rolling_hit_rate()

        return jsonify({
            # Legacy stats
            'hits': hits,
            'misses': misses,
            'total': total,
            'hit_rate': round(hit_rate, 1),
            # Phase 4 rolling stats
            'rolling': rolling_stats,
            'project_id': project_id
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


def calculate_rolling_hit_rate(window_hours: int = 24) -> dict:
    """
    Calculate Hit@5 over a rolling time window.

    Hit@5 = (prediction batches with at least 1 hit) / (total evaluated batches)

    Returns:
        dict with:
        - window_hours: The time window
        - total_predictions: Number of predictions in window
        - evaluated: Number of predictions that have been evaluated
        - hits: Number of prediction batches with at least 1 hit
        - hit_at_5: Hit@5 rate (0.0 to 1.0)
        - hit_at_5_pct: Hit@5 as percentage (0 to 100)
    """
    import time as time_module

    if not RANKING_AVAILABLE or scorer is None:
        return {'error': 'Redis not available'}

    try:
        now = time_module.time()
        window_start = now - (window_hours * 3600)

        # Get all predictions in the rolling window
        rolling_key = "aoa:rolling:predictions"
        prediction_keys = scorer.redis.client.zrangebyscore(
            rolling_key, window_start, now
        )

        total_predictions = len(prediction_keys)
        evaluated = 0
        hits = 0
        misses = 0

        for pred_key in prediction_keys:
            pred_key_str = pred_key.decode() if isinstance(pred_key, bytes) else pred_key
            rolling_data_key = f"aoa:rolling:data:{pred_key_str}"

            hit_value = scorer.redis.client.hget(rolling_data_key, 'hit')
            if hit_value is not None:
                hit_str = hit_value.decode() if isinstance(hit_value, bytes) else hit_value
                if hit_str == '1':
                    hits += 1
                    evaluated += 1
                elif hit_str == '0':
                    misses += 1
                    evaluated += 1
                # Empty string means not yet evaluated

        hit_at_5 = hits / evaluated if evaluated > 0 else 0.0

        return {
            'window_hours': window_hours,
            'total_predictions': total_predictions,
            'evaluated': evaluated,
            'pending': total_predictions - evaluated,
            'hits': hits,
            'misses': misses,
            'hit_at_5': round(hit_at_5, 4),
            'hit_at_5_pct': round(hit_at_5 * 100, 1),
        }

    except Exception as e:
        return {'error': str(e)}


@app.route('/predict/finalize', methods=['POST'])
def finalize_predictions():
    """
    Finalize stale predictions as misses.

    Predictions older than `max_age_seconds` (default 300 = 5 minutes) that
    haven't been marked as hits are marked as misses.

    POST body (optional):
    {
        "max_age_seconds": 300
    }

    Returns count of predictions finalized.
    """
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    import time as time_module

    data = request.json or {}
    max_age_seconds = data.get('max_age_seconds', 300)  # 5 minutes default

    try:
        now = time_module.time()
        cutoff = now - max_age_seconds

        # Get predictions older than max_age that haven't been evaluated
        rolling_key = "aoa:rolling:predictions"
        stale_keys = scorer.redis.client.zrangebyscore(
            rolling_key, 0, cutoff
        )

        finalized = 0
        for pred_key in stale_keys:
            pred_key_str = pred_key.decode() if isinstance(pred_key, bytes) else pred_key
            rolling_data_key = f"aoa:rolling:data:{pred_key_str}"

            hit_value = scorer.redis.client.hget(rolling_data_key, 'hit')
            if hit_value is not None:
                hit_str = hit_value.decode() if isinstance(hit_value, bytes) else hit_value
                if hit_str == '':
                    # Not yet evaluated - mark as miss
                    scorer.redis.client.hset(rolling_data_key, 'hit', '0')
                    finalized += 1

        return jsonify({
            'finalized': finalized,
            'checked': len(stale_keys),
            'max_age_seconds': max_age_seconds
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


# ============================================================================
# Weight Tuner API - Phase 4 Thompson Sampling
# ============================================================================

@app.route('/tuner/weights')
def tuner_weights():
    """
    Get current optimized weights via Thompson Sampling.

    Each call samples from the Beta distributions and returns the best arm.
    Use for exploration (learning which weights work best).

    Returns:
        {
            "weights": {"recency": 0.4, "frequency": 0.3, "tag": 0.3},
            "arm_idx": 2,
            "arm_name": "default"
        }
    """
    if tuner is None:
        return jsonify({'error': 'Tuner not available'}), 503

    try:
        weights = tuner.select_weights()
        arm_idx = weights.pop('_arm_idx', 0)
        arm = tuner.ARMS[arm_idx]

        return jsonify({
            'weights': weights,
            'arm_idx': arm_idx,
            'arm_name': arm.get('name', f'arm-{arm_idx}')
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/tuner/best')
def tuner_best():
    """
    Get the best performing weights (exploitation only, no exploration).

    Returns the arm with highest mean success rate.
    Use for production predictions once you have enough data.

    Returns:
        {
            "weights": {"recency": 0.5, "frequency": 0.3, "tag": 0.2},
            "arm_idx": 0,
            "mean": 0.78
        }
    """
    if tuner is None:
        return jsonify({'error': 'Tuner not available'}), 503

    try:
        best = tuner.get_best_weights()
        arm_idx = best.pop('_arm_idx', 0)
        mean = best.pop('_mean', 0.5)
        arm = tuner.ARMS[arm_idx]

        return jsonify({
            'weights': best,
            'arm_idx': arm_idx,
            'arm_name': arm.get('name', f'arm-{arm_idx}'),
            'mean': round(mean, 4)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/tuner/stats')
def tuner_stats():
    """
    Get statistics for all arms, sorted by mean success rate.

    Returns:
        {
            "arms": [
                {"arm_idx": 0, "name": "recency-heavy", "mean": 0.78, ...},
                ...
            ],
            "total_samples": 150
        }
    """
    if tuner is None:
        return jsonify({'error': 'Tuner not available'}), 503

    try:
        stats = tuner.get_stats()
        total_samples = sum(arm['samples'] for arm in stats)

        return jsonify({
            'arms': stats,
            'total_samples': total_samples
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/tuner/feedback', methods=['POST'])
def tuner_feedback():
    """
    Record hit/miss feedback for a specific arm.

    POST body:
    {
        "arm_idx": 2,
        "hit": true
    }

    Returns confirmation of the update.
    """
    if tuner is None:
        return jsonify({'error': 'Tuner not available'}), 503

    data = request.json or {}
    arm_idx = data.get('arm_idx')
    hit = data.get('hit', False)

    if arm_idx is None:
        return jsonify({'error': 'arm_idx required'}), 400

    try:
        tuner.record_feedback(hit=hit, arm_idx=arm_idx)

        # Get updated stats for this arm
        alpha, beta = tuner._get_arm_stats(arm_idx)

        return jsonify({
            'success': True,
            'arm_idx': arm_idx,
            'hit': hit,
            'new_alpha': alpha,
            'new_beta': beta,
            'new_mean': round(alpha / (alpha + beta), 4)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/tuner/reset', methods=['POST'])
def tuner_reset():
    """
    Reset all arm statistics to priors.

    Use with caution - this erases all learned data.
    """
    if tuner is None:
        return jsonify({'error': 'Tuner not available'}), 503

    try:
        tuner.reset()
        return jsonify({'success': True, 'message': 'All arms reset to priors'})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


# ============================================================================
# Metrics API - Phase 4 Unified Accuracy Dashboard
# ============================================================================

@app.route('/metrics')
def get_metrics():
    """
    Unified metrics endpoint showing accuracy, tuner performance, and trends.

    Query params:
        project_id: UUID for per-project_id metrics (optional, for future per-project_id support)

    Returns:
        {
            "hit_at_5": 0.72,
            "hit_at_5_pct": 72.0,
            "target": 0.90,
            "gap": 0.18,
            "trend": "improving",

            "rolling": {
                "window_hours": 24,
                "total_predictions": 150,
                "evaluated": 120,
                "hits": 86,
                "hit_at_5": 0.72
            },

            "tuner": {
                "best_arm": "recency-heavy",
                "best_weights": {"recency": 0.5, ...},
                "best_mean": 0.78,
                "total_samples": 150
            },

            "legacy": {
                "hits": 200,
                "misses": 100,
                "hit_rate": 66.7
            }
        }
    """
    # Accept project_id for future per-project_id metrics support
    # TODO: Implement per-project_id Redis key prefixing for metrics
    project_id = request.args.get('project_id')

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Ranking not available'}), 503

    try:
        # Get rolling stats
        rolling = calculate_rolling_hit_rate()

        # Get tuner stats
        tuner_stats = {}
        if tuner is not None:
            best = tuner.get_best_weights()
            arm_idx = best.pop('_arm_idx', 0)
            mean = best.pop('_mean', 0.5)
            arm = tuner.ARMS[arm_idx]
            all_stats = tuner.get_stats()

            tuner_stats = {
                'best_arm': arm.get('name', f'arm-{arm_idx}'),
                'best_arm_idx': arm_idx,
                'best_weights': best,
                'best_mean': round(mean, 4),
                'total_samples': sum(a['samples'] for a in all_stats),
            }

        # Legacy cumulative stats (per-project_id if project_id provided, else global)
        # Note: tokens_saved and time_saved_ms are DEPRECATED - they were fabricated estimates
        # Real savings require capturing actual output tokens (Phase 2)
        if project_id:
            hits = int(scorer.redis.client.get(f'aoa:{project_id}:metrics:hits') or 0)
            misses = int(scorer.redis.client.get(f'aoa:{project_id}:metrics:misses') or 0)
            # DEPRECATED: These were fake hardcoded estimates (1500 tokens/hit, 50ms/hit)
            # Real savings will be tracked via intent records with baseline + actual output
            tokens_saved = int(scorer.redis.client.get(f'aoa:{project_id}:savings:tokens:real') or 0)
        else:
            hits = int(scorer.redis.client.get('aoa:metrics:hits') or 0)
            misses = int(scorer.redis.client.get('aoa:metrics:misses') or 0)
            # DEPRECATED: These were fake hardcoded estimates
            tokens_saved = int(scorer.redis.client.get('aoa:savings:tokens:real') or 0)

        total = hits + misses
        legacy_rate = (hits / total * 100) if total > 0 else 0

        # Calculate main metrics
        hit_at_5 = rolling.get('hit_at_5', 0.0)
        target = 0.90

        # Determine trend (would need historical data for real trend)
        # For now, compare to legacy rate
        if rolling.get('evaluated', 0) > 10:
            if hit_at_5 > (legacy_rate / 100) + 0.05:
                trend = 'improving'
            elif hit_at_5 < (legacy_rate / 100) - 0.05:
                trend = 'declining'
            else:
                trend = 'stable'
        else:
            trend = 'insufficient_data'

        # Get real savings from intent index (file_size vs output_size measurements)
        intent_savings = {}
        total_intents = 0
        if intent_index:
            intent_stats = intent_index.get_stats(project_id)
            intent_savings = intent_stats.get('savings', {})
            total_intents = intent_stats.get('total_records', 0)

        # Total savings = Redis + Intent (Intent is the primary/real source)
        total_tokens_saved = tokens_saved + intent_savings.get('tokens', 0)
        # Get time_sec from intent_savings (calculated at 7.5ms per token)
        intent_time_sec = intent_savings.get('time_sec', 0)
        savings_data = {
            'tokens': total_tokens_saved,
            'baseline': intent_savings.get('baseline', 0),
            'actual': intent_savings.get('actual', 0),
            'measured_records': intent_savings.get('measured_records', 0),
            'time_sec': intent_time_sec,  # Estimated from token savings
        }

        return jsonify({
            # Primary metrics
            'hit_at_5': hit_at_5,
            'hit_at_5_pct': rolling.get('hit_at_5_pct', 0.0),
            'target': target,
            'target_pct': target * 100,
            'gap': round(target - hit_at_5, 4),
            'trend': trend,

            # Detailed rolling stats
            'rolling': rolling,

            # Tuner stats
            'tuner': tuner_stats,

            # Legacy stats (cumulative)
            'legacy': {
                'hits': hits,
                'misses': misses,
                'total': total,
                'hit_rate': round(legacy_rate, 1),
            },

            # Savings (cumulative) - include intent index real measurements
            'savings': savings_data,

            # Intent count for status line
            'total_intents': total_intents,
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/metrics/tokens')
def get_token_metrics():
    """
    Get token usage and cost statistics from Claude session logs.

    Returns:
        {
            "input_tokens": 1234567,
            "output_tokens": 234567,
            "cache_read_tokens": 567890,
            "total_tokens": 1469134,
            "message_count": 150,

            "cost": {
                "input": 18.52,
                "output": 17.59,
                "total": 36.11
            },

            "savings": {
                "from_cache": 7.65,
                "cache_hit_rate": 0.42
            }
        }
    """
    try:
        # Get project_id path from environment
        import os

        from ranking.session_parser import SessionLogParser
        project_path = os.environ.get('CODEBASE_ROOT', '/home/corey/aOa')

        parser = SessionLogParser(project_path)
        token_stats = parser.get_token_usage()

        return jsonify(token_stats)

    except Exception as e:
        return jsonify({'error': str(e)}), 500


# ============================================================================
# GL-091: Test Mode Thresholds
# ============================================================================

THRESHOLD_DEFAULTS = {
    'rebalance': 25,
    'autotune': 100,  # P2B: Autotune every 100 prompts (full lifecycle management)
    'promotion': 150,
    'demotion': 500,
    'prune_floor': 0.5
}

THRESHOLD_TEST = {
    'rebalance': 3,
    'autotune': 10,  # P2B: Autotune every 10 prompts in test mode
    'promotion': 15,
    'demotion': 50,
    'prune_floor': 0.5
}


@app.route('/config/thresholds', methods=['GET', 'POST'])
def config_thresholds():
    """
    GL-091: Get or set threshold configuration for test mode.

    GET: Returns current thresholds
    POST body: {"mode": "test"} or {"mode": "prod"} or {"thresholds": {...}}
    """
    project_id = request.args.get('project_id') or (request.json or {}).get('project_id')

    if not DOMAINS_AVAILABLE:
        return jsonify({'error': 'Domain learning not available'}), 500

    try:
        learner = DomainLearner(project_id) if project_id else None
        r = learner.redis.client if learner else None

        if request.method == 'GET':
            # Return current thresholds
            current = {}
            for key, default in THRESHOLD_DEFAULTS.items():
                if r:
                    val = r.get(f"aoa:config:{key}")
                    current[key] = float(val) if val else default
                else:
                    current[key] = default
            return jsonify({'thresholds': current, 'project_id': project_id})

        # POST - set thresholds
        data = request.json or {}
        mode = data.get('mode')
        custom = data.get('thresholds')

        if mode == 'test':
            thresholds = THRESHOLD_TEST
        elif mode == 'prod':
            thresholds = THRESHOLD_DEFAULTS
        elif custom:
            thresholds = custom
        else:
            return jsonify({'error': 'Specify mode (test/prod) or thresholds'}), 400

        # Store in Redis
        if r:
            for key, val in thresholds.items():
                r.set(f"aoa:config:{key}", val)

        return jsonify({
            'success': True,
            'mode': mode or 'custom',
            'thresholds': thresholds,
            'project_id': project_id
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/predict')
def predict_files():
    """
    Get predicted files with optional snippet prefetch.

    This is the main prediction endpoint for P2-005.
    Returns ranked files with first N lines of each file for context injection.

    Query params:
        tags: Comma-separated tags to filter/boost by (optional)
        keywords: Comma-separated keywords from prompt (optional, treated as tags)
        limit: Maximum files to return (default: 5)
        snippet_lines: Number of lines to prefetch per file (default: 20, 0 to disable)
        file: Trigger file for co-occurrence lookup (optional)

    Returns:
        {
            "files": [
                {
                    "path": "/src/api/routes.py",
                    "confidence": 0.85,
                    "snippet": "first 20 lines..."
                }
            ],
            "predictions": ["/src/api/routes.py", ...],  # Simple list for backward compat
            "ms": 4.2
        }
    """
    start = time.time()

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({
            'error': 'Ranking module not available',
            'files': [],
            'predictions': [],
            'ms': (time.time() - start) * 1000
        }), 503

    # Parse parameters
    tag_param = request.args.get('tags', request.args.get('tag', ''))
    keyword_param = request.args.get('keywords', '')
    file_param = request.args.get('file', '')

    # Combine tags and keywords
    tags = [t.strip().lstrip('#') for t in tag_param.split(',') if t.strip()]
    keywords = [k.strip() for k in keyword_param.split(',') if k.strip()]
    all_tags = list(set(tags + keywords))

    limit = int(request.args.get('limit', 5))
    snippet_lines = int(request.args.get('snippet_lines', 20))

    try:
        # Get ranked files from scorer
        results = scorer.get_ranked_files(tags=all_tags if all_tags else None, limit=limit * 2)

        # Get transition predictions if trigger file provided
        transition_preds = {}
        transition_boost = 0.0
        project_root = os.environ.get('CODEBASE_ROOT', '/codebase')
        host_root = '/home/corey/aOa'

        if file_param and SESSION_PARSER_AVAILABLE:
            try:
                # Try with the file param as-is first
                trans_results = SessionLogParser.predict_next(scorer.redis, file_param, limit=10)

                # If no results, try normalizing the path
                if not trans_results and file_param.startswith(host_root):
                    normalized = file_param[len(host_root) + 1:]  # Remove /home/corey/aOa/
                    trans_results = SessionLogParser.predict_next(scorer.redis, normalized, limit=10)
                elif not trans_results and file_param.startswith(project_root):
                    normalized = file_param[len(project_root) + 1:]  # Remove /codebase/
                    trans_results = SessionLogParser.predict_next(scorer.redis, normalized, limit=10)

                # Store predictions with both absolute and relative paths for matching
                for f, prob in trans_results:
                    transition_preds[f] = prob
                    # Also store absolute path variant
                    if not f.startswith('/'):
                        transition_preds[os.path.join(host_root, f)] = prob

                transition_boost = 0.3  # Boost factor for transition matches
            except Exception:
                pass  # Transitions are optional enhancement

        # Build response with snippets
        files = []
        seen_paths = set()

        for r in results:
            file_path = r['file']
            # Use calibrated confidence from scorer (P2-001)
            # Falls back to normalized score for backward compatibility
            confidence = r.get('confidence', min(r.get('score', 0.0) / 100.0, 1.0))

            # Boost confidence if file is also predicted by transitions
            if file_path in transition_preds:
                trans_prob = transition_preds[file_path]
                confidence = min(1.0, confidence + trans_prob * transition_boost)

            file_data = {
                'path': file_path,
                'confidence': round(confidence, 3)
            }

            # Read snippet if requested
            if snippet_lines > 0:
                snippet = read_file_snippet(file_path, snippet_lines)
                if snippet:
                    file_data['snippet'] = snippet

            files.append(file_data)
            seen_paths.add(file_path)

            if len(files) >= limit:
                break

        # Add high-probability transition predictions not in scorer results
        if transition_preds and len(files) < limit:
            for trans_file, trans_prob in sorted(transition_preds.items(),
                                                  key=lambda x: x[1], reverse=True):
                if trans_file not in seen_paths and trans_prob >= 0.1:
                    file_data = {
                        'path': trans_file,
                        'confidence': round(trans_prob * 0.8, 3),  # Scale down since not in scorer
                        'source': 'transition'
                    }
                    if snippet_lines > 0:
                        snippet = read_file_snippet(trans_file, snippet_lines)
                        if snippet:
                            file_data['snippet'] = snippet
                    files.append(file_data)
                    if len(files) >= limit:
                        break

        # Re-sort by confidence
        files.sort(key=lambda x: x['confidence'], reverse=True)
        files = files[:limit]

        return jsonify({
            'files': files,
            'predictions': [f['path'] for f in files],  # Backward compat
            'tags_used': all_tags,
            'trigger_file': file_param if file_param else None,
            'transition_matches': len([f for f in files if f['path'] in transition_preds]),
            'ms': round((time.time() - start) * 1000, 2)
        })

    except Exception as e:
        return jsonify({
            'error': str(e),
            'files': [],
            'predictions': [],
            'ms': (time.time() - start) * 1000
        }), 500


def read_file_snippet(file_path: str, max_lines: int = 20) -> str:
    """
    Read first N lines of a file for snippet prefetch.

    Returns empty string if file doesn't exist or can't be read.
    Handles common text files, skips binary files.
    """
    import os

    # Translate host paths to container paths
    # File paths in Redis are stored as /home/corey/aOa/... but in container they're at /codebase/...
    CODEBASE_ROOT = os.environ.get('CODEBASE_ROOT', '/codebase')
    HOST_PATH_PREFIX = '/home/corey/aOa'

    if file_path.startswith(HOST_PATH_PREFIX):
        file_path = file_path.replace(HOST_PATH_PREFIX, CODEBASE_ROOT, 1)

    # Resolve to absolute path if needed
    if not os.path.isabs(file_path):
        # Try common base paths
        for base in [CODEBASE_ROOT, os.getcwd()]:
            full_path = os.path.join(base, file_path)
            if os.path.exists(full_path):
                file_path = full_path
                break

    if not os.path.exists(file_path):
        return ''

    # Skip binary files by extension
    binary_exts = {'.pyc', '.so', '.o', '.a', '.exe', '.dll', '.bin', '.dat',
                   '.png', '.jpg', '.jpeg', '.gif', '.ico', '.pdf', '.zip', '.tar', '.gz'}
    _, ext = os.path.splitext(file_path)
    if ext.lower() in binary_exts:
        return ''

    try:
        with open(file_path, encoding='utf-8', errors='ignore') as f:
            lines = []
            for i, line in enumerate(f):
                if i >= max_lines:
                    break
                # Truncate very long lines
                if len(line) > 500:
                    line = line[:500] + '...\n'
                lines.append(line)
            return ''.join(lines)
    except OSError:
        return ''


# ============================================================================
# Ranking API - Predictive File Scoring
# ============================================================================

# Global scorer and tuner instances
scorer = None
tuner = None  # Phase 4: Thompson Sampling weight tuner


@app.route('/rank')
def rank_files():
    """
    Get files ranked by composite score (recency + frequency + tag affinity).

    Query params:
        tag: Comma-separated tags to filter/boost by (optional)
        limit: Maximum files to return (default: 10)
        db: Redis database number (for testing, optional)

    Returns:
        {
            "files": ["/src/api/routes.py", ...],
            "details": [{"file": "...", "score": 0.85, ...}, ...],
            "ms": 4.2
        }
    """
    start = time.time()

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({
            'error': 'Ranking module not available',
            'files': [],
            'details': [],
            'ms': (time.time() - start) * 1000
        }), 503

    # Parse parameters
    tag_param = request.args.get('tag', '')
    tags = [t.strip().lstrip('#') for t in tag_param.split(',') if t.strip()]
    limit = int(request.args.get('limit', 10))
    db = request.args.get('db')
    db = int(db) if db else None

    # Get ranked files
    try:
        results = scorer.get_ranked_files(tags=tags if tags else None, limit=limit, db=db)

        return jsonify({
            'files': [r['file'] for r in results],
            'details': results,
            'tags_used': tags,
            'ms': round((time.time() - start) * 1000, 2)
        })
    except Exception as e:
        return jsonify({
            'error': str(e),
            'files': [],
            'details': [],
            'ms': (time.time() - start) * 1000
        }), 500


@app.route('/rank/stats')
def rank_stats():
    """Get ranking system statistics."""
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Ranking module not available'}), 503

    return jsonify(scorer.get_stats())


@app.route('/rank/record', methods=['POST'])
def rank_record():
    """
    Record a file access for scoring.

    POST body:
        {
            "file": "/src/api/routes.py",
            "tags": ["api", "python"]
        }
    """
    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Ranking module not available'}), 503

    data = request.json or {}
    file_path = data.get('file')
    tags = data.get('tags', [])

    if not file_path:
        return jsonify({'error': 'file parameter required'}), 400

    scores = scorer.record_access(file_path, tags=tags)
    return jsonify({
        'recorded': file_path,
        'scores': scores
    })


# ============================================================================
# Transition Model API - Phase 3 Session Log Learning
# ============================================================================

# Global session parser instance
session_parser = None

try:
    from ranking.session_parser import SessionLogParser
    SESSION_PARSER_AVAILABLE = True
except ImportError:
    SESSION_PARSER_AVAILABLE = False


@app.route('/transitions/sync', methods=['POST'])
def sync_transitions():
    """
    Sync file transitions from Claude session logs to Redis.

    This parses ~/.claude/projects/*/agent-*.jsonl to extract
    file access patterns and store transition probabilities in Redis.

    POST body (optional):
    {
        "project_path": "/home/corey/aOa"  # default
    }

    Returns:
    {
        "keys_written": 57,
        "total_transitions": 94,
        "stats": {...}
    }
    """
    global session_parser

    if not SESSION_PARSER_AVAILABLE:
        return jsonify({'error': 'Session parser not available'}), 503

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    start = time.time()
    data = request.json or {}
    project_path = data.get('project_path', '/home/corey/aOa')

    try:
        session_parser = SessionLogParser(project_path)
        stats = session_parser.get_stats()
        result = session_parser.sync_to_redis(scorer.redis)

        return jsonify({
            'success': True,
            'keys_written': result['keys_written'],
            'total_transitions': result['total_transitions'],
            'stats': stats,
            'ms': round((time.time() - start) * 1000, 2)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/transitions/predict')
def predict_from_transitions():
    """
    Get predicted next files based on transition model.

    Query params:
        file: Current file being accessed (required)
        limit: Maximum predictions to return (default: 5)

    Returns:
    {
        "predictions": [
            {"file": ".context/BOARD.md", "probability": 0.7},
            {"file": "src/hooks/intent-prefetch.py", "probability": 0.1}
        ],
        "source_file": ".context/CURRENT.md",
        "ms": 1.2
    }
    """
    if not SESSION_PARSER_AVAILABLE:
        return jsonify({'error': 'Session parser not available'}), 503

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    start = time.time()
    current_file = request.args.get('file', '')
    limit = int(request.args.get('limit', 5))

    if not current_file:
        return jsonify({'error': 'file parameter required'}), 400

    try:
        predictions = SessionLogParser.predict_next(
            scorer.redis, current_file, limit=limit
        )

        return jsonify({
            'predictions': [
                {'file': f, 'probability': round(p, 4)}
                for f, p in predictions
            ],
            'source_file': current_file,
            'ms': round((time.time() - start) * 1000, 2)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/transitions/stats')
def transition_stats():
    """
    Get statistics about the transition model.

    Returns session parsing stats and Redis key counts.
    """
    if not SESSION_PARSER_AVAILABLE:
        return jsonify({'error': 'Session parser not available'}), 503

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    try:
        # Count transition keys in Redis
        transition_keys = scorer.redis.keys('aoa:transition:*')

        # Get session parser stats if initialized
        parser_stats = None
        if session_parser:
            parser_stats = session_parser.get_stats()

        return jsonify({
            'transition_keys': len(transition_keys),
            'parser_stats': parser_stats
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


# ============================================================================
# Context API - Natural Language Intent to Files (P3-003, P3-004)
# ============================================================================

# Stopwords for keyword extraction
STOPWORDS = {
    'the', 'a', 'an', 'is', 'are', 'was', 'were', 'be', 'been', 'being',
    'have', 'has', 'had', 'do', 'does', 'did', 'will', 'would', 'could',
    'should', 'may', 'might', 'must', 'shall', 'can', 'need', 'dare',
    'to', 'of', 'in', 'for', 'on', 'with', 'at', 'by', 'from', 'as',
    'into', 'through', 'during', 'before', 'after', 'above', 'below',
    'between', 'under', 'again', 'further', 'then', 'once', 'here',
    'there', 'when', 'where', 'why', 'how', 'all', 'each', 'few',
    'more', 'most', 'other', 'some', 'such', 'no', 'nor', 'not',
    'only', 'own', 'same', 'so', 'than', 'too', 'very', 'just',
    'and', 'but', 'if', 'or', 'because', 'until', 'while', 'this',
    'that', 'these', 'those', 'what', 'which', 'who', 'whom',
    'i', 'you', 'he', 'she', 'it', 'we', 'they', 'me', 'him', 'her',
    'us', 'them', 'my', 'your', 'his', 'its', 'our', 'their',
    'fix', 'add', 'update', 'change', 'modify', 'implement', 'create',
    'make', 'get', 'set', 'find', 'look', 'check', 'help', 'want',
    'try', 'work', 'use', 'file', 'code', 'function', 'class'
}

# Intent patterns from intent-capture.py (tag mapping)
INTENT_PATTERNS = [
    (r'auth|login|session|oauth|jwt|password', ['authentication', 'security']),
    (r'test[s]?[/_]|_test\.|\bspec[s]?\b|pytest|unittest', ['testing']),
    (r'config|settings|\.env|\.yaml|\.yml|\.json', ['configuration']),
    (r'api|endpoint|route|handler|controller', ['api']),
    (r'index|search|query|grep|find', ['search']),
    (r'model|schema|entity|db|database|migration|sql', ['data']),
    (r'component|view|template|page|ui|style|css|html', ['frontend']),
    (r'deploy|docker|k8s|ci|cd|pipeline|github', ['devops']),
    (r'error|exception|catch|throw|raise|fail', ['errors']),
    (r'log|debug|trace|print|console', ['logging']),
    (r'cache|redis|memory|store', ['caching']),
    (r'async|await|promise|thread|concurrent', ['async']),
    (r'hook|plugin|extension|middleware', ['hooks']),
    (r'doc|readme|comment|docstring', ['documentation']),
    (r'util|helper|common|shared|lib', ['utilities']),
    (r'ranking|score|predict|confidence', ['ranking']),
    (r'transition|session|pattern', ['transitions']),
]


def extract_keywords(text: str) -> list:
    """
    Extract meaningful keywords from natural language intent.

    Simple approach: tokenize, lowercase, filter stopwords.
    """
    import re
    # Tokenize: extract words
    tokens = re.findall(r'[a-zA-Z][a-zA-Z0-9_]*', text.lower())

    # Filter: remove stopwords, keep meaningful tokens
    keywords = [t for t in tokens if t not in STOPWORDS and len(t) > 2]

    # Dedupe while preserving order
    seen = set()
    unique = []
    for k in keywords:
        if k not in seen:
            seen.add(k)
            unique.append(k)

    return unique


def map_keywords_to_tags(keywords: list) -> list:
    """
    Map extracted keywords to intent tags.

    Matches keywords against INTENT_PATTERNS.
    """
    import re
    matched_tags = set()
    combined = ' '.join(keywords)

    for pattern, tags in INTENT_PATTERNS:
        if re.search(pattern, combined, re.IGNORECASE):
            matched_tags.update(tags)

    return list(matched_tags)


@app.route('/context', methods=['POST'])
def context_search():
    """
    Natural language intent -> ranked files + snippets.

    POST body:
    {
        "intent": "fix the auth bug in login",
        "limit": 5,
        "snippet_lines": 10,
        "trigger_file": ".context/CURRENT.md"  (optional)
    }

    Returns:
    {
        "intent": "fix the auth bug in login",
        "keywords": ["auth", "bug", "login"],
        "tags_matched": ["authentication", "security"],
        "files": [
            {
                "path": "src/auth/login.py",
                "confidence": 0.85,
                "snippet": "..."
            }
        ],
        "ms": 12.5
    }
    """
    start = time.time()

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Ranking not available'}), 503

    data = request.json or {}
    intent = data.get('intent', '')
    limit = int(data.get('limit', 5))
    snippet_lines = int(data.get('snippet_lines', 10))
    trigger_file = data.get('trigger_file', '')

    if not intent:
        return jsonify({'error': 'intent required'}), 400

    # Step 1: Extract keywords
    keywords = extract_keywords(intent)

    if not keywords:
        return jsonify({
            'error': 'No keywords extracted from intent',
            'intent': intent,
            'keywords': []
        }), 400

    # Step 1.5: Check cache (normalized keyword key)
    cache_key = f"aoa:context:{':'.join(sorted(keywords))}"
    try:
        cached = scorer.redis.client.get(cache_key)
        if cached:
            cached_result = json.loads(cached)
            cached_result['cached'] = True
            cached_result['ms'] = round((time.time() - start) * 1000, 2)
            return jsonify(cached_result)
    except Exception:
        pass  # Cache miss or error, continue

    # Step 2: Map keywords to tags
    tags_matched = map_keywords_to_tags(keywords)

    # Step 3: Get ranked files (using tags as boost)
    all_tags = list(set(keywords + tags_matched))
    results = scorer.get_ranked_files(tags=all_tags, limit=limit * 2)

    # Step 4: Get transition predictions if trigger file provided
    transition_preds = {}
    host_root = '/home/corey/aOa'
    if trigger_file and SESSION_PARSER_AVAILABLE:
        try:
            trans_results = SessionLogParser.predict_next(scorer.redis, trigger_file, limit=10)
            for f, prob in trans_results:
                transition_preds[f] = prob
                if not f.startswith('/'):
                    transition_preds[os.path.join(host_root, f)] = prob
        except Exception:
            pass

    # Step 5: Build response with snippets
    files = []
    seen_paths = set()

    for r in results:
        file_path = r['file']
        confidence = r.get('confidence', min(r.get('score', 0.0) / 100.0, 1.0))

        # Boost if in transition predictions
        if file_path in transition_preds:
            confidence = min(1.0, confidence + transition_preds[file_path] * 0.3)

        file_data = {
            'path': file_path,
            'confidence': round(confidence, 3)
        }

        if snippet_lines > 0:
            snippet = read_file_snippet(file_path, snippet_lines)
            if snippet:
                file_data['snippet'] = snippet

        files.append(file_data)
        seen_paths.add(file_path)

        if len(files) >= limit:
            break

    # Add high-probability transition predictions
    if transition_preds and len(files) < limit:
        for trans_file, trans_prob in sorted(transition_preds.items(),
                                              key=lambda x: x[1], reverse=True):
            if trans_file not in seen_paths and trans_prob >= 0.1:
                file_data = {
                    'path': trans_file,
                    'confidence': round(trans_prob * 0.8, 3),
                    'source': 'transition'
                }
                if snippet_lines > 0:
                    snippet = read_file_snippet(trans_file, snippet_lines)
                    if snippet:
                        file_data['snippet'] = snippet
                files.append(file_data)
                if len(files) >= limit:
                    break

    # Sort by confidence
    files.sort(key=lambda x: x['confidence'], reverse=True)
    files = files[:limit]

    # Build response
    result = {
        'intent': intent,
        'keywords': keywords,
        'tags_matched': tags_matched,
        'files': files,
        'trigger_file': trigger_file if trigger_file else None,
        'cached': False
    }

    # Cache result (1 hour TTL, skip snippets for cache efficiency)
    try:
        cache_data = {
            'intent': intent,
            'keywords': keywords,
            'tags_matched': tags_matched,
            'files': [{'path': f['path'], 'confidence': f['confidence']} for f in files],
            'trigger_file': trigger_file if trigger_file else None
        }
        scorer.redis.client.setex(cache_key, 3600, json.dumps(cache_data))
    except Exception:
        pass  # Cache write failure is non-fatal

    result['ms'] = round((time.time() - start) * 1000, 2)
    return jsonify(result)


# ============================================================================
# Memory API - Dynamic Working Context (Phase 5)
# ============================================================================

# Domain patterns for prose generation
DOMAIN_PATTERNS = {
    r'auth|login|session|oauth': 'authentication',
    r'api|endpoint|route|handler': 'API layer',
    r'test|spec|mock': 'testing',
    r'config|settings|env': 'configuration',
    r'index|search|query': 'search infrastructure',
    r'rank|score|predict': 'ranking system',
    r'hook|capture|intent': 'intent tracking',
    r'gate|proxy|route': 'gateway',
    r'redis|cache|store': 'data layer',
    r'doc|readme|md': 'documentation',
}


def time_band(seconds_ago: float) -> str:
    """Convert seconds ago to human-readable time band."""
    if seconds_ago < 60:
        return "just now"
    if seconds_ago < 180:
        return "moments ago"
    if seconds_ago < 600:
        return f"{int(seconds_ago / 60)}m ago"
    if seconds_ago < 1800:
        return "recently"
    if seconds_ago < 3600:
        return "earlier this session"
    return "earlier today"


def confidence_phrase(score: float) -> str:
    """Convert numeric confidence to natural phrase."""
    if score > 0.8:
        return "main focus"
    if score > 0.6:
        return "actively working on"
    if score > 0.4:
        return "recently touched"
    return "in context"


def detect_domain(file_path: str) -> str:
    """Detect domain from file path."""
    path_lower = file_path.lower()
    for pattern, domain in DOMAIN_PATTERNS.items():
        if re.search(pattern, path_lower):
            return domain
    # Fallback to directory name
    parts = file_path.split('/')
    if len(parts) > 1:
        return parts[-2] if parts[-2] not in ('src', 'lib', 'app') else parts[-1].split('.')[0]
    return "general"


def get_recent_tags(limit: int = 5) -> list[str]:
    """Get recent intent tags."""
    try:
        stats = intent_index.get_stats()
        tags = stats.get('tags', {})
        # Sort by count, take top N
        sorted_tags = sorted(tags.items(), key=lambda x: x[1], reverse=True)[:limit]
        return [t[0].lstrip('#') for t, _ in sorted_tags]
    except Exception:
        return []


@app.route('/memory')
def get_memory():
    """
    Dynamic working memory - current context as LLM-readable prose.

    Returns structured narrative of:
    - Current focus (what you're working on)
    - Active files (recently touched)
    - Predicted next files
    - Intent signals

    Query params:
        format: prose (default), structured, compact
        window: time window in minutes (default 20)

    Example response (prose):
        ## Working Memory

        You're currently focused on the ranking system, specifically the scorer.

        **Active Files** (last 20 minutes):
        - src/ranking/scorer.py (5 touches, 3m ago) - main focus
        - src/index/indexer.py (2 touches, 12m ago) - actively working on

        **Predicted Next**:
        - src/ranking/redis_client.py (65% likely)

        **Intent Signals**: #python, #editing, #search
    """
    start = time.time()

    fmt = request.args.get('format', 'prose')
    window_mins = int(request.args.get('window', 20))
    window_secs = window_mins * 60

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({
            'memory': 'Working memory unavailable (Redis not connected)',
            'format': fmt,
            'ms': round((time.time() - start) * 1000, 2)
        })

    try:
        now = time.time()

        # 1. Get recent files by recency
        recent_files = scorer.get_top_files_by_recency(limit=20)

        # Filter to window and enrich with data
        active_files = []
        for file_path, last_ts in recent_files:
            age = now - last_ts
            if age > window_secs:
                continue

            freq = scorer.get_frequency_score(file_path) or 0
            active_files.append({
                'path': file_path,
                'last_access': last_ts,
                'age_seconds': age,
                'time_band': time_band(age),
                'frequency': int(freq),
                'domain': detect_domain(file_path),
            })

        # 2. Detect primary focus
        focus_domain = "general"
        focus_file = None
        if active_files:
            # Most frequent in window is likely focus
            by_freq = sorted(active_files, key=lambda x: x['frequency'], reverse=True)
            focus_file = by_freq[0]['path']
            focus_domain = by_freq[0]['domain']

        # 3. Get predictions for next files
        predicted_next = []
        if focus_file and SESSION_PARSER_AVAILABLE:
            try:
                # Get relative path for transition lookup
                rel_path = focus_file
                if rel_path.startswith('/home/corey/aOa/'):
                    rel_path = rel_path[len('/home/corey/aOa/'):]

                preds = SessionLogParser.predict_next(scorer.redis, rel_path, limit=3)
                for pred_file, prob in preds:
                    predicted_next.append({
                        'path': pred_file,
                        'probability': round(prob * 100),
                    })
            except Exception:
                pass

        # 4. Get recent tags
        recent_tags = get_recent_tags(5)

        # 5. Calculate mode (read/write ratio from recent intents)
        mode = "exploring"
        try:
            records = intent_index.recent(limit=20)
            if records:
                writes = sum(1 for r in records if r.get('tool', '').lower() in ('write', 'edit', 'notebookedit'))
                reads = sum(1 for r in records if r.get('tool', '').lower() in ('read', 'glob', 'grep'))
                if writes > reads:
                    mode = "writing"
                elif reads > writes * 2:
                    mode = "reading"
                else:
                    mode = "mixed"
        except Exception:
            pass

        # Generate output based on format
        if fmt == 'compact':
            # Minimal token format
            file_list = ','.join(f"{os.path.basename(f['path'])}({f['frequency']}x,{f['time_band']})"
                                 for f in active_files[:5])
            pred_list = ','.join(f"{os.path.basename(p['path'])}({p['probability']}%)"
                                 for p in predicted_next)
            tag_list = ','.join(recent_tags)

            memory = f"FOCUS: {os.path.basename(focus_file or 'none')} ({focus_domain})\n"
            memory += f"ACTIVE: {file_list}\n"
            if pred_list:
                memory += f"NEXT: {pred_list}\n"
            memory += f"TAGS: {tag_list}\n"
            memory += f"MODE: {mode}"

        elif fmt == 'structured':
            # JSON with explanations
            return jsonify({
                'focus': {
                    'domain': focus_domain,
                    'file': focus_file,
                    'explanation': f"Based on highest frequency in last {window_mins}m"
                },
                'active_files': [{
                    'path': f['path'],
                    'frequency': f['frequency'],
                    'recency': f['time_band'],
                    'role': confidence_phrase(f['frequency'] / 10)
                } for f in active_files[:5]],
                'predicted_next': [{
                    'path': p['path'],
                    'probability': p['probability'],
                    'why': 'frequently follows current focus'
                } for p in predicted_next],
                'intent_signals': recent_tags,
                'mode': mode,
                'window_minutes': window_mins,
                'ms': round((time.time() - start) * 1000, 2)
            })

        else:
            # Prose format (default)
            lines = ["## Working Memory", ""]

            if focus_file:
                lines.append(f"You're currently focused on **{focus_domain}**, specifically `{os.path.basename(focus_file)}`.")
            else:
                lines.append("No recent file activity detected.")

            lines.append("")

            if active_files:
                lines.append(f"**Active Files** (last {window_mins} minutes):")
                for f in active_files[:5]:
                    role = confidence_phrase(f['frequency'] / 10)
                    lines.append(f"- `{f['path']}` ({f['frequency']}x, {f['time_band']}) - {role}")
                lines.append("")

            if predicted_next:
                lines.append("**Predicted Next**:")
                for p in predicted_next:
                    lines.append(f"- `{p['path']}` ({p['probability']}% likely)")
                lines.append("")

            if recent_tags:
                tag_str = ', '.join(f"#{t}" for t in recent_tags)
                lines.append(f"**Intent Signals**: {tag_str}")
                lines.append("")

            lines.append(f"**Mode**: {mode}")

            memory = '\n'.join(lines)

        return jsonify({
            'memory': memory,
            'format': fmt,
            'files_analyzed': len(active_files),
            'ms': round((time.time() - start) * 1000, 2)
        })

    except Exception as e:
        return jsonify({
            'error': str(e),
            'memory': '',
            'ms': round((time.time() - start) * 1000, 2)
        }), 500


# ============================================================================
# GL-089: Job Queue Endpoints
# ============================================================================

@app.route('/jobs/status')
def jobs_status():
    """Get job queue status for a project."""
    if not JOBS_AVAILABLE:
        return jsonify({'error': 'Job queue module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        q = JobQueue(project_id)
        return jsonify(q.status())
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/jobs/pending')
def jobs_pending():
    """Get pending jobs for a project."""
    if not JOBS_AVAILABLE:
        return jsonify({'error': 'Job queue module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    limit = int(request.args.get('limit', 10))

    try:
        q = JobQueue(project_id)
        jobs = q.pending_jobs(limit)
        return jsonify({
            'jobs': [{'id': j.id, 'type': j.type.value, 'payload': j.payload} for j in jobs],
            'count': len(jobs)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/jobs/failed')
def jobs_failed():
    """Get failed jobs for a project with error messages."""
    if not JOBS_AVAILABLE:
        return jsonify({'error': 'Job queue module not available'}), 500

    project_id = request.args.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    limit = int(request.args.get('limit', 10))

    try:
        q = JobQueue(project_id)
        jobs = q.failed_jobs(limit)
        return jsonify({
            'jobs': [{'id': j.id, 'type': j.type.value, 'payload': j.payload, 'error': j.error} for j in jobs],
            'count': len(jobs)
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/jobs/push', methods=['POST'])
def jobs_push():
    """Push jobs to the queue."""
    if not JOBS_AVAILABLE:
        return jsonify({'error': 'Job queue module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    jobs_data = data.get('jobs', [])
    if not jobs_data:
        return jsonify({'error': 'No jobs provided'}), 400

    try:
        q = JobQueue(project_id)
        jobs = []
        for jd in jobs_data:
            job = Job(
                id="",
                type=JobType(jd.get('type', 'enrich')),
                project_id=project_id,
                phase=jd.get('phase', 'intelligence'),
                payload=jd.get('payload', {})
            )
            jobs.append(job)

        count = q.push_many(jobs)
        return jsonify({'pushed': count, 'status': q.status()})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/jobs/push/enrich', methods=['POST'])
def jobs_push_enrich():
    """
    Push enrichment jobs for domains.

    POST body: {"project_id": "xxx", "domains": [{"name": "@x", "description": "..."}]}
    """
    if not JOBS_AVAILABLE:
        return jsonify({'error': 'Job queue module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    domains = data.get('domains', [])
    if not domains:
        return jsonify({'error': 'No domains provided'}), 400

    try:
        count = push_enrich_jobs(project_id, domains)
        q = JobQueue(project_id)
        return jsonify({'pushed': count, 'status': q.status()})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/jobs/process', methods=['POST'])
def jobs_process():
    """
    Process jobs from the queue.

    POST body: {"project_id": "xxx", "count": 3}
    """
    if not JOBS_AVAILABLE:
        return jsonify({'error': 'Job queue module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    count = int(data.get('count', 1))

    try:
        worker = JobWorker(project_id)
        processed = worker.process_batch(count)
        return jsonify({
            'processed': len(processed),
            'jobs': [{'id': j.id, 'type': j.type.value} for j in processed],
            'status': worker.queue.status()
        })
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/jobs/retry', methods=['POST'])
def jobs_retry():
    """Move failed jobs back to pending."""
    if not JOBS_AVAILABLE:
        return jsonify({'error': 'Job queue module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    try:
        q = JobQueue(project_id)
        count = q.retry_failed()
        return jsonify({'retried': count, 'status': q.status()})
    except Exception as e:
        return jsonify({'error': str(e)}), 500


@app.route('/jobs/clear', methods=['POST'])
def jobs_clear():
    """Clear job queues."""
    if not JOBS_AVAILABLE:
        return jsonify({'error': 'Job queue module not available'}), 500

    data = request.json or {}
    project_id = data.get('project_id')
    if not project_id:
        return jsonify({'error': 'Missing project_id parameter'}), 400

    queue_type = data.get('queue', 'complete')  # complete, failed, all

    try:
        q = JobQueue(project_id)
        if queue_type == 'all':
            result = q.clear_all()
        elif queue_type == 'complete':
            result = {'cleared': q.clear_complete()}
        else:
            return jsonify({'error': f'Unknown queue type: {queue_type}'}), 400

        return jsonify(result)
    except Exception as e:
        return jsonify({'error': str(e)}), 500


# ============================================================================
# Main
# ============================================================================

def main():
    global manager, intent_index, scorer, tuner

    codebase_root = os.environ.get('CODEBASE_ROOT', '')
    repos_root = os.environ.get('REPOS_ROOT', './repos')
    config_dir = os.environ.get('CONFIG_DIR', '/config')
    indexes_dir = os.environ.get('INDEXES_DIR', '/indexes')
    port = int(os.environ.get('PORT', 9999))

    # Detect global mode
    global_mode = not codebase_root and Path(config_dir).exists()

    print("=" * 60)
    print("aOa Index Service - Multi-Index Architecture")
    if global_mode:
        print("Mode: GLOBAL (multi-project_id)")
    else:
        print("Mode: LEGACY (single project_id)")
    print("=" * 60)

    # Initialize ranking scorer and weight tuner FIRST (need Redis for intent index)
    redis_client = None
    if RANKING_AVAILABLE:
        scorer = Scorer()
        if scorer.redis.ping():
            print("Ranking scorer initialized (Redis connected)")
            redis_client = scorer.redis
            # Phase 4: Initialize weight tuner
            tuner = WeightTuner(scorer.redis)
            print("Weight tuner initialized (8 arms)")
        else:
            print("Ranking scorer initialized (Redis not available)")
            scorer = None
            tuner = None
    else:
        print("Ranking module not available")

    # Initialize intent index with Redis for persistence
    intent_index = IntentIndex(redis_client=redis_client)
    if redis_client:
        print("Intent index initialized (Redis-backed, persists across restarts)")
    else:
        print("Intent index initialized (in-memory only)")

    if global_mode:
        print(f"Config directory: {config_dir}")
        print(f"Indexes directory: {indexes_dir}")
    else:
        print(f"Local codebase: {codebase_root}")
    print(f"Repos directory: {repos_root}")
    print()

    # Create index manager
    manager = IndexManager(
        codebase_root if codebase_root else None,
        repos_root,
        config_dir if global_mode else None,
        indexes_dir if global_mode else None
    )

    # Initialize indexes
    manager.init_local()
    manager.init_repos()

    print()
    if manager.local:
        print(f"Local: {len(manager.local.files)} files, {len(manager.local.inverted_index)} symbols")
    if manager.projects:
        print(f"Projects: {len(manager.projects)} project_id indexes loaded")
    print(f"Repos: {len(manager.repos)} knowledge repos loaded")
    print()

    try:
        print(f"Listening on http://0.0.0.0:{port}")
        app.run(host='0.0.0.0', port=port, threaded=True)
    finally:
        manager.shutdown()


if __name__ == '__main__':
    main()
