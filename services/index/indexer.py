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
    from jobs.queue import (
        JobQueue, Job, JobType,
        create_enrich_job, create_scrape_job, create_autotune_job
    )
    from jobs.worker import JobWorker, push_enrich_jobs
    JOBS_AVAILABLE = True
except ImportError:
    JOBS_AVAILABLE = False
    JobQueue = None
    JobWorker = None

# Pattern matchers (CH-01: extracted to matchers.py)
from matchers import (
    AC_MATCHER,
    AHOCORASICK_AVAILABLE,
    KeywordMatcher,
    get_keyword_matcher,
    set_intent_index,
    SEMANTIC_PATTERNS,
    DOMAIN_KEYWORDS,
)
import matchers  # For KEYWORD_MATCHER global access

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

    # S65-01 FIX: Sync hit tracking (no daemon thread)
    # Redis pipeline is fast (~2-5ms), no need for async complexity.
    # Data already in memory from search - just count before returning.
    if DOMAINS_AVAILABLE and project_id and results:
        try:
            learner = DomainLearner(project_id)
            seen_terms = set()
            seen_domains = set()

            # Collect terms and domains from top 10 results
            for r in results[:10]:
                for tag in r.get('tags', []):
                    term = tag.lstrip('#@').lower()
                    if term and len(term) >= 3:
                        seen_terms.add(term)
                # GL-088: Track domains from results
                # FIX SL-02: Keep @ prefix - Redis keys include it
                domain = r.get('domain', '')
                if domain:
                    seen_domains.add(domain if domain.startswith('@') else f"@{domain}")

            # Collect domains for each term
            for term in seen_terms:
                domains_with_term = learner.get_domains_for_term(term)
                for domain_name in domains_with_term:
                    seen_domains.add(domain_name)

            # Single pipeline for all increments (~2-5ms total)
            if seen_terms or seen_domains:
                pipe = learner.redis.client.pipeline()
                prompt_count = learner.get_prompt_count()

                # Term hits
                term_hits_key = f"aoa:{project_id}:term_hits"
                for term in seen_terms:
                    pipe.hincrby(term_hits_key, term, 1)

                # Domain hits (hits + total_hits + last_hit_at)
                for domain in seen_domains:
                    meta_key = f"aoa:{project_id}:domain:{domain}:meta"
                    pipe.hincrby(meta_key, "hits", 1)
                    pipe.hincrby(meta_key, "total_hits", 1)
                    pipe.hset(meta_key, "last_hit_at", prompt_count)

                pipe.execute()  # ONE round-trip
        except Exception as e:
            print(f"[HitTracking] Error: {e}", flush=True)

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

# Register domain routes blueprint (CH-01: extracted for maintainability)
from domains_api import domains_bp, init_domains_api
app.register_blueprint(domains_bp)

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
    project_id = request.args.get('project_id')
    start = time.time()

    try:
        # Use matchers module's global KEYWORD_MATCHER
        if matchers.KEYWORD_MATCHER:
            matchers.KEYWORD_MATCHER.rebuild()
            elapsed_ms = (time.time() - start) * 1000
            return jsonify({
                'status': 'ok',
                'keywords': matchers.KEYWORD_MATCHER.keyword_count,
                'domains': len(matchers.KEYWORD_MATCHER.domain_created_at),
                'elapsed_ms': round(elapsed_ms, 3)
            })
        elif intent_index and intent_index.redis:
            # Initialize via get_keyword_matcher (creates in matchers module)
            proj = project_id or 'local'
            matcher = get_keyword_matcher(proj, intent_index)
            if matcher:
                elapsed_ms = (time.time() - start) * 1000
                return jsonify({
                    'status': 'initialized',
                    'keywords': matcher.keyword_count,
                    'domains': len(matcher.domain_created_at),
                    'elapsed_ms': round(elapsed_ms, 3)
                })
            else:
                return jsonify({
                    'status': 'error',
                    'error': 'Could not initialize KeywordMatcher'
                }), 503
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


def _trigger_domain_learning_if_needed(project_id: str, intent_index: 'IntentIndex' = None):
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
            added = rebalance_result.get('added', 0)
            gaps = rebalance_result.get('gaps_processed', 0)
            print(f"[Rebalance] {project_id}: +{added} keywords, {gaps} gaps processed", flush=True)

            # CT-06: Record rebalance event in intent for visibility
            if intent_index:
                intent_index.record(
                    tool='@rebalance',
                    files=[],
                    tags=[f'#added:{added}', f'#gaps:{gaps}', '#system'],
                    session_id='aOa',
                    project_id=project_id
                )

        # P2B: Autotune trigger (every 100 prompts) - full domain lifecycle management
        # Uses get_threshold('autotune') for test mode support (100 in prod, 10 in test)
        # See .context/arch/06-autotune.md for full scope (10 operations)
        autotune_interval = int(learner.get_threshold('autotune'))
        if prompt_count > 0 and autotune_interval > 0 and prompt_count % autotune_interval == 0:
            tune_result = learner.run_math_tune()
            promoted = tune_result.get('promoted', 0)
            demoted = tune_result.get('demoted', 0)
            pruned = tune_result.get('context_pruned', 0)
            decayed = tune_result.get('decayed', 0)
            print(f"[Autotune] {project_id}: promoted={promoted}, demoted={demoted}, pruned={pruned}", flush=True)

            # CT-06: Record autotune event in intent for visibility (always, even if no changes)
            if intent_index:
                intent_index.record(
                    tool='@autotune',
                    files=[],
                    tags=[f'#promoted:{promoted}', f'#demoted:{demoted}', f'#pruned:{pruned}', f'#decayed:{decayed}', '#system'],
                    session_id='aOa',
                    project_id=project_id
                )

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
                        # RB-02: Track term:domain co-occurrence
                        learner.increment_cohit(term, term, domain_name)
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
    _trigger_domain_learning_if_needed(project_id, intent_index)

    # GL-088: Check if enrichment should be triggered (every N prompts based on threshold)
    enrichment_ready = False
    prompt_count = 0
    if DOMAINS_AVAILABLE and project_id:
        try:
            learner = DomainLearner(project_id)
            prompt_count = learner.get_prompt_count()
            # RB-14: Use configurable threshold (3 in test mode, 25 in prod)
            rebalance_threshold = int(learner.get_threshold('rebalance'))
            # Signal enrichment_ready when prompt_count hits threshold, 2x, 3x, etc.
            enrichment_ready = (prompt_count > 0 and prompt_count % rebalance_threshold == 0)
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


@app.route('/cc/conversation')
def cc_conversation():
    """Get conversation text since last scrape: prompts + thinking + output.

    Query params:
        project_path: Project directory (default: CODEBASE_ROOT)
        since: ISO timestamp - only return content after this time (default: all)
        limit: Max items to return (default: 100)

    Returns all text for bigram extraction:
    - User prompts
    - Assistant thinking (reasoning)
    - Assistant text (visible output)
    - latest_timestamp: Use as 'since' for next call to avoid duplicates
    """
    try:
        from metrics import SessionMetrics
    except ImportError:
        try:
            from session.metrics import SessionMetrics
        except ImportError:
            return jsonify({'error': 'SessionMetrics not available', 'texts': []})

    try:
        limit = int(request.args.get('limit', 100))
        since = request.args.get('since', '')  # ISO timestamp filter

        # Get HOST project path from /config/home.json (for session path encoding)
        project_path = request.args.get('project_path')
        if not project_path:
            try:
                with open('/codebase/.aoa/home.json') as f:
                    home_data = json.load(f)
                    project_path = home_data.get('project_root', os.environ.get('CODEBASE_ROOT', '/app'))
            except (FileNotFoundError, json.JSONDecodeError):
                project_path = os.environ.get('CODEBASE_ROOT', '/app')

        metrics = SessionMetrics(project_path)
        texts = []
        latest_timestamp = since  # Track newest timestamp seen

        # Parse recent sessions for full conversation
        for session_file in metrics._get_session_files(limit=5):
            try:
                with open(session_file) as f:
                    for line in f:
                        try:
                            data = json.loads(line)
                        except json.JSONDecodeError:
                            continue

                        # Filter by timestamp if 'since' provided
                        timestamp = data.get("timestamp", "")
                        if since and timestamp and timestamp <= since:
                            continue  # Skip already-processed content

                        # Track latest timestamp
                        if timestamp and timestamp > latest_timestamp:
                            latest_timestamp = timestamp

                        msg_type = data.get("type")
                        msg = data.get("message", {})

                        # User prompts
                        if msg_type == "user" and not data.get("isMeta"):
                            content = msg.get("content", "")
                            if isinstance(content, str) and len(content) > 5:
                                # Strip system-generated blocks
                                import re
                                clean = re.sub(r"<system-reminder>.*?</system-reminder>", "", content, flags=re.DOTALL)
                                clean = re.sub(r"<[^>]+>", "", clean)
                                clean = re.sub(r"\s+", " ", clean).strip()
                                if clean:
                                    texts.append({"type": "prompt", "text": clean, "ts": timestamp})

                        # Assistant thinking + text
                        elif msg_type == "assistant":
                            content = msg.get("content", [])
                            if isinstance(content, list):
                                for item in content:
                                    if isinstance(item, dict):
                                        item_type = item.get("type")
                                        if item_type == "thinking":
                                            thinking = item.get("thinking", "")
                                            if thinking and len(thinking) > 10:
                                                texts.append({"type": "thinking", "text": thinking, "ts": timestamp})
                                        elif item_type == "text":
                                            text = item.get("text", "")
                                            if text and len(text) > 10:
                                                texts.append({"type": "output", "text": text, "ts": timestamp})

                        if len(texts) >= limit:
                            break
            except (IOError, OSError):
                continue
            if len(texts) >= limit:
                break

        return jsonify({
            'texts': texts[:limit],
            'count': len(texts[:limit]),
            'latest_timestamp': latest_timestamp,  # Use as 'since' next time
            'project_path': project_path
        })
    except Exception as e:
        return jsonify({'error': str(e), 'texts': [], 'latest_timestamp': since})


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


@app.route('/cc/bigrams')
def cc_bigrams():
    """BG-04: Get bigrams for rebalance with noise filtering.

    Query params:
        project_id: Project to query
        min_count: Minimum hit count (default 6)
        limit: Max bigrams to return (default 100)

    Returns:
        {
            'bigrams': [{'bigram': 'hit:tracking', 'count': 55}, ...],
            'total': 9439,
            'filtered': 85
        }
    """
    project_id = request.args.get('project_id')
    min_count = int(request.args.get('min_count', 6))
    limit = int(request.args.get('limit', 100))

    if not project_id:
        return jsonify({'error': 'project_id required'}), 400

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    # Noise filter: common filler phrases and stopword pairs
    NOISE_PATTERNS = {
        'let:me', 'me:check', 'the:user', 'in:the', 'of:the', 'to:the',
        'with:the', 'from:the', 'at:the', 'on:the', 'for:the', 'and:the',
        'is:the', 'it:the', 'that:the', 'this:the', 'but:the', 'now:let',
        'me:to', 'i:will', 'i:need', 'you:can', 'we:need', 'we:can',
        'let:s', 'let:us', 'going:to', 'want:to', 'need:to', 'have:to',
        'looking:at', 'look:at', 'check:if', 'check:the', 'see:if', 'see:the',
        'home:corey', 'corey:aoa',  # Path artifacts
    }

    try:
        r = scorer.redis.client
        bigram_key = f'aoa:{project_id}:bigrams'

        # Get all bigrams
        raw = r.hgetall(bigram_key)
        total = len(raw)

        # Filter and sort
        filtered = []
        for bigram, count_bytes in raw.items():
            bigram_str = bigram.decode() if isinstance(bigram, bytes) else bigram
            count = int(count_bytes.decode() if isinstance(count_bytes, bytes) else count_bytes)

            # Skip if below threshold or noise
            if count < min_count:
                continue
            if bigram_str in NOISE_PATTERNS:
                continue

            filtered.append({'bigram': bigram_str, 'count': count})

        # Sort by count descending, take limit
        filtered.sort(key=lambda x: x['count'], reverse=True)
        result = filtered[:limit]

        return jsonify({
            'bigrams': result,
            'total': total,
            'filtered': len(result),
            'min_count': min_count
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500


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

def calculate_dynamic_rates() -> dict:
    """
    Calculate dynamic token processing rates from Claude session logs.

    Analyzes recent session files to determine actual LLM response rates,
    returning a range (low/high) based on P25 percentile across time windows.

    Returns:
        {
            'rate_low': float,   # ms per token (faster end)
            'rate_high': float,  # ms per token (slower end)
            'samples': int       # number of rate samples used
        }
    """
    import json as json_mod
    from datetime import datetime
    from pathlib import Path

    home = os.environ.get('HOME', os.path.expanduser('~'))
    projects_dir = Path(home) / '.claude' / 'projects'

    if not projects_dir.exists():
        return {'rate_low': 2.0, 'rate_high': 5.0, 'samples': 0}

    # Get recent session files
    sessions = []
    try:
        for project_dir in sorted(projects_dir.iterdir(), key=lambda p: p.stat().st_mtime, reverse=True)[:3]:
            sessions.extend(sorted(project_dir.glob('*.jsonl'), key=lambda p: p.stat().st_mtime, reverse=True)[:5])
    except Exception:
        return {'rate_low': 2.0, 'rate_high': 5.0, 'samples': 0}

    now = datetime.now().astimezone()
    windows = {'5min': [], '15min': [], '30min': []}

    for session_file in sessions[:10]:
        try:
            messages = []
            with open(session_file, 'r') as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    try:
                        event = json_mod.loads(line)
                        if event.get('type') == 'assistant' and 'message' in event:
                            msg = event['message']
                            if 'usage' in msg and 'timestamp' in event:
                                ts = datetime.fromisoformat(event['timestamp'].replace('Z', '+00:00'))
                                tokens = msg['usage'].get('input_tokens', 0) + msg['usage'].get('output_tokens', 0)
                                messages.append({'ts': ts, 'tokens': tokens})
                    except Exception:
                        continue

            # Calculate rates between consecutive messages
            for i in range(1, len(messages)):
                try:
                    duration_ms = (messages[i]['ts'] - messages[i-1]['ts']).total_seconds() * 1000
                    tokens = messages[i]['tokens']
                    age = (now - messages[i]['ts']).total_seconds() / 60  # minutes ago

                    # Only include fast responses (< 15s) - pure LLM processing without tool delays
                    if 100 < duration_ms < 15000 and tokens > 200:
                        rate = duration_ms / tokens
                        # Cap at 20ms/token - anything higher is tool/network overhead
                        if rate < 20:
                            if age <= 5:
                                windows['5min'].append(rate)
                            if age <= 15:
                                windows['15min'].append(rate)
                            if age <= 30:
                                windows['30min'].append(rate)
                except Exception:
                    continue
        except Exception:
            continue

    def percentile(lst, p):
        if not lst:
            return None
        s = sorted(lst)
        idx = int(len(s) * p / 100)
        return s[min(idx, len(s)-1)]

    rates = []
    for w in ['5min', '15min', '30min']:
        p = percentile(windows[w], 25)
        if p is not None:
            rates.append(p)

    if rates:
        return {
            'rate_low': round(min(rates), 2),
            'rate_high': round(max(rates), 2),
            'samples': sum(len(v) for v in windows.values())
        }
    else:
        # Fallback: documented input processing rate (~2ms/token)
        return {'rate_low': 1.5, 'rate_high': 3.0, 'samples': 0}


@app.route('/metrics')
def get_metrics():
    """
    Simplified metrics endpoint for status line.

    SH-04: Prediction system sunset - now based on stop_count.

    Query params:
        project_id: UUID for per-project metrics

    Returns:
        {
            "stop_count": 125,      # Total stops for this project
            "total_intents": 450,   # Total intent records
            "savings": {...},       # Token/time savings
            "rolling": {...}        # Stub for backward compat
        }

    Status line thresholds:
        <50 stops: gray (learning)
        50-250: yellow
        250+: green
    """
    project_id = request.args.get('project_id')

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Ranking not available'}), 503

    try:
        # Get stop count (the new primary metric)
        stop_count = 0
        if project_id:
            stop_count = int(scorer.redis.client.get(f'aoa:{project_id}:session:stop_count') or 0)

        # Get savings from intent index
        intent_savings = {}
        total_intents = 0
        if intent_index:
            intent_stats = intent_index.get_stats(project_id)
            intent_savings = intent_stats.get('savings', {})
            total_intents = intent_stats.get('total_records', 0)

        tokens_saved = intent_savings.get('tokens', 0)

        # Calculate dynamic rates from session logs
        dynamic_rates = calculate_dynamic_rates()
        rate_low = dynamic_rates.get('rate_low', 2.0)
        rate_high = dynamic_rates.get('rate_high', 5.0)

        # Calculate time range using dynamic rates (ms/token -> seconds)
        time_sec_low = tokens_saved * rate_low / 1000
        time_sec_high = tokens_saved * rate_high / 1000

        savings_data = {
            'tokens': tokens_saved,
            'baseline': intent_savings.get('baseline', 0),
            'actual': intent_savings.get('actual', 0),
            'measured_records': intent_savings.get('measured_records', 0),
            'time_sec_low': round(time_sec_low, 1),
            'time_sec_high': round(time_sec_high, 1),
            'time_sec': round(time_sec_high, 1),
            'rate_low': rate_low,
            'rate_high': rate_high,
            'rate_samples': dynamic_rates.get('samples', 0),
        }

        # Stub rolling stats for backward compatibility with status line
        rolling = {
            'window_hours': 24,
            'total_predictions': 0,
            'evaluated': 0,
            'pending': 0,
            'hits': 0,
            'misses': 0,
            'hit_at_5': 0.0,
            'hit_at_5_pct': 0.0,
        }

        return jsonify({
            # Primary metric: stop count
            'stop_count': stop_count,

            # Intent count
            'total_intents': total_intents,

            # Savings
            'savings': savings_data,

            # Stub for backward compat
            'rolling': rolling,
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
# TC-01: Unified Threshold Configuration (Session 64)
# ============================================================================
# Single source of truth: Redis keys aoa:config:{setting}
# Set via: aoa config thresholds test|prod

THRESHOLD_PROD = {
    'scrape': 5,
    'rebalance': 50,
    'autotune': 100,
    'promotion': 100,
    'demotion': 500,
    'prune_floor': 0.3,
    'decay_rate': 0.95
}

THRESHOLD_TEST = {
    'scrape': 5,
    'rebalance': 10,
    'autotune': 25,
    'promotion': 25,
    'demotion': 100,
    'prune_floor': 0.3,
    'decay_rate': 0.90
}

def get_thresholds(project_id: str = None) -> dict:
    """Get thresholds from Redis, falling back to prod defaults."""
    thresholds = THRESHOLD_PROD.copy()
    try:
        if RANKING_AVAILABLE and scorer is not None:
            r = scorer.redis.client
            for key in thresholds:
                val = r.get(f"aoa:config:{key}")
                if val is not None:
                    thresholds[key] = float(val)
    except Exception:
        pass
    return thresholds


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
            # Return current thresholds (TC-01: use get_thresholds)
            current = get_thresholds(project_id)
            return jsonify({'thresholds': current, 'project_id': project_id})

        # POST - set thresholds
        data = request.json or {}
        mode = data.get('mode')
        custom = data.get('thresholds')

        if mode == 'test':
            thresholds = THRESHOLD_TEST
        elif mode == 'prod':
            thresholds = THRESHOLD_PROD
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


# ============================================================================
# Session Stop Hook (SH-02)
# ============================================================================

@app.route('/session/stop', methods=['POST'])
def session_stop():
    """
    Stop hook handler: increment counter, trigger async actions at thresholds.

    POST body:
    {
        "session_id": "abc123",
        "project_id": "uuid-here"
    }

    Returns:
    {
        "stop_count": 5,
        "triggered": ["scrape"],  # Actions triggered at this stop
        "thresholds": {"scrape": 5, "rebalance": 25, "autotune": 100}
    }

    Thresholds:
    - Every 5 stops: Session scrape (bigrams + file hits) - async
    - Every 25 stops: Rebalance keywords - async
    - Every 100 stops: Autotune (decay, promote, demote, prune) - async
    """
    data = request.json or {}
    session_id = data.get('session_id', 'unknown')
    project_id = data.get('project_id')

    if not project_id:
        return jsonify({'error': 'project_id required'}), 400

    if not RANKING_AVAILABLE or scorer is None:
        return jsonify({'error': 'Redis not available'}), 503

    try:
        r = scorer.redis.client

        # Increment stop count (atomic)
        stop_key = f'aoa:{project_id}:session:stop_count'
        stop_count = r.incr(stop_key)

        # Set TTL on stop counter (24 hour session window)
        if stop_count == 1:
            r.expire(stop_key, 86400)

        # Check thresholds and queue actions (TC-01: read from Redis)
        triggered = []
        thresholds = get_thresholds(project_id)

        # Every N stops: Session scrape (bigrams + file hits)
        if stop_count % int(thresholds['scrape']) == 0:
            triggered.append('scrape')
            # Queue via JobQueue (SH-02c)
            if JOBS_AVAILABLE:
                queue = JobQueue(project_id)
                job = create_scrape_job(project_id, session_id, stop_count)
                queue.push(job)

        # Every N stops: Rebalance (10 test, 50 prod)
        if stop_count % int(thresholds['rebalance']) == 0:
            triggered.append('rebalance')
            # Set haiku pending flag for next prompt
            r.set(f'aoa:{project_id}:haiku_pending', '1')

        # Every N stops: Autotune (25 test, 100 prod)
        if stop_count % int(thresholds['autotune']) == 0:
            triggered.append('autotune')
            # Queue via JobQueue (SH-02c)
            if JOBS_AVAILABLE:
                queue = JobQueue(project_id)
                job = create_autotune_job(project_id, stop_count)
                queue.push(job)

        return jsonify({
            'success': True,
            'stop_count': stop_count,
            'triggered': triggered,
            'thresholds': thresholds
        })

    except Exception as e:
        return jsonify({'error': str(e)}), 500



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
            'jobs': [{'id': j.id, 'type': j.type.value, 'domain': j.payload.get('domain')} for j in processed],
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
    set_intent_index(intent_index)  # CH-01: Set for matchers module
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

    # Initialize domains API with runtime dependencies (CH-01)
    init_domains_api(manager, intent_index, DOMAINS_AVAILABLE, DomainLearner)

    try:
        print(f"Listening on http://0.0.0.0:{port}")
        app.run(host='0.0.0.0', port=port, threaded=True)
    finally:
        manager.shutdown()


if __name__ == '__main__':
    main()
