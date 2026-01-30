#!/usr/bin/env python3
"""
aOa Status Service
Real-time Claude Code session monitoring backed by Redis.

Tracks: model, tokens, cache, context, cost, weekly usage, time.
Now also syncs subagent activity from Claude session logs.

Usage:
    pip install flask redis requests
    python status_service.py
"""

import json
import os
import re
import threading
import time
from dataclasses import asdict, dataclass, field
from datetime import datetime
from pathlib import Path

import redis
import requests
from flask import Flask, Response, jsonify, request

app = Flask(__name__)

# =============================================================================
# Configuration
# =============================================================================

REDIS_URL = os.environ.get('REDIS_URL', 'redis://localhost:6379/0')
PORT = int(os.environ.get('STATUS_PORT', 9998))

# Global instances (initialized in main())
manager = None
syncer = None

# Model pricing (per 1M tokens) - as of January 2026
PRICING = {
    # Latest models (4.5 generation)
    'claude-opus-4.5': {'input': 15.00, 'output': 75.00, 'cache_read': 1.50, 'cache_write': 18.75},
    'claude-sonnet-4.5': {'input': 3.00, 'output': 15.00, 'cache_read': 0.30, 'cache_write': 3.75},
    'claude-haiku-4': {'input': 0.25, 'output': 1.25, 'cache_read': 0.025, 'cache_write': 0.3125},

    # Legacy 4.0 models
    'claude-opus-4': {'input': 15.00, 'output': 75.00, 'cache_read': 1.50, 'cache_write': 18.75},
    'claude-sonnet-4': {'input': 3.00, 'output': 15.00, 'cache_read': 0.30, 'cache_write': 3.75},

    # Aliases
    'opus-4.5': {'input': 15.00, 'output': 75.00, 'cache_read': 1.50, 'cache_write': 18.75},
    'sonnet-4.5': {'input': 3.00, 'output': 15.00, 'cache_read': 0.30, 'cache_write': 3.75},
    'opus-4': {'input': 15.00, 'output': 75.00, 'cache_read': 1.50, 'cache_write': 18.75},
    'sonnet-4': {'input': 3.00, 'output': 15.00, 'cache_read': 0.30, 'cache_write': 3.75},
    'haiku-4': {'input': 0.25, 'output': 1.25, 'cache_read': 0.025, 'cache_write': 0.3125},
}

# Context window sizes
CONTEXT_LIMITS = {
    'claude-opus-4': 200000,
    'claude-sonnet-4': 200000,
    'claude-haiku-4': 200000,
    'opus-4': 200000,
    'sonnet-4': 200000,
    'haiku-4': 200000,
}

# Redis keys
class Keys:
    SESSION = "aoa:session"           # Hash: session state
    METRICS = "aoa:metrics"           # Hash: running totals
    HISTORY = "aoa:history"           # List: recent events
    DAILY = "aoa:daily:{date}"        # Hash: daily stats
    WEEKLY = "aoa:weekly"             # Hash: weekly tracking
    PROJECT = "aoa:project:{name}"    # Hash: per-project totals
    AGENT_SYNC = "aoa:agent_sync"     # Hash: agent file positions


# =============================================================================
# Subagent Sync - Intent patterns for tagging
# =============================================================================

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
]

# Tool to tag mapping
TOOL_TAGS = {
    'Read': ['reading'],
    'Edit': ['editing'],
    'Write': ['writing'],
    'Grep': ['searching'],
    'Glob': ['searching'],
    'Bash': ['executing'],
}


# ============================================================================
# Pattern Library Loading (RAM-cached at import time for ultra-speed)
# ============================================================================

def _load_pattern_configs():
    """Load pattern configs from universal-domains.json (v2 format).

    GL-074: Migrated from legacy semantic-patterns.json + domain-patterns.json
    to unified universal-domains.json with three-level hierarchy:
    @domain -> semantic_term -> matches[]

    Called once at import time. Returns optimized lookup structures.
    """
    semantic_patterns = {}  # semantic_term -> {patterns: set, tag: @domain, priority: int}
    domain_keywords = {}    # match -> @domain
    class_suffixes = {}     # suffix -> tag (kept for backwards compat)

    # Find config directory
    config_paths = [
        Path(__file__).parent.parent.parent / "config",  # services/status/ -> aOa/config/
        Path("/app/config"),  # Docker mount
        Path("/home") / os.environ.get("USER", "user") / ".aoa" / "config",  # ~/.aoa/config/
    ]

    universal_file = None
    for config_dir in config_paths:
        if config_dir.exists():
            candidate = config_dir / "universal-domains.json"
            if candidate.exists():
                universal_file = candidate
                break

    if not universal_file:
        return semantic_patterns, domain_keywords, class_suffixes

    try:
        data = json.loads(universal_file.read_text())
        # Handle both array format (v2) and object format (with _meta)
        domains = data if isinstance(data, list) else data.get("domains", [])

        for domain in domains:
            domain_name = domain.get("name", "")  # e.g., "@authentication"
            terms = domain.get("terms", {})

            # v2 format: terms is dict of semantic_term -> matches[]
            if isinstance(terms, dict):
                for semantic_term, matches in terms.items():
                    # Build semantic patterns
                    patterns = {m.lower() for m in matches}
                    semantic_patterns[semantic_term] = {
                        "patterns": patterns,
                        "tag": domain_name,
                        "priority": 3
                    }

                    # Build match -> domain lookup
                    for match in matches:
                        domain_keywords[match.lower()] = domain_name

            # Flat format fallback
            elif isinstance(terms, list):
                for match in terms:
                    domain_keywords[match.lower()] = domain_name

    except (OSError, json.JSONDecodeError):
        pass

    return semantic_patterns, domain_keywords, class_suffixes


# Load patterns at import time (RAM-cached, runs once)
SEMANTIC_PATTERNS, DOMAIN_KEYWORDS, CLASS_SUFFIXES = _load_pattern_configs()


def _tokenize(text: str) -> set:
    """Tokenize a string into matchable parts."""
    tokens = set()
    parts = re.split(r'[/_\-.\s]+', text)

    for part in parts:
        if not part:
            continue
        part_lower = part.lower()
        tokens.add(part_lower)

        # Split camelCase
        camel_parts = re.findall(r'[A-Z]?[a-z]+|[A-Z]+(?=[A-Z][a-z]|\d|\W|$)|\d+', part)
        for cp in camel_parts:
            tokens.add(cp.lower())

    return tokens


def _match_semantic_tags(tokens: set) -> set:
    """Match tokens against semantic patterns."""
    tags = set()

    for priority in [1, 2, 3]:
        for _cat_name, cat_data in SEMANTIC_PATTERNS.items():
            if cat_data["priority"] != priority:
                continue
            patterns = cat_data["patterns"]
            for token in tokens:
                for pattern in patterns:
                    if token.startswith(pattern) or token == pattern:
                        tags.add(cat_data["tag"])
                        break

    return tags


def _match_domain_tags(tokens: set, full_text: str) -> set:
    """Match tokens against domain keywords."""
    tags = set()
    full_lower = full_text.lower()

    for keyword, tag in DOMAIN_KEYWORDS.items():
        if keyword in tokens or keyword in full_lower:
            tags.add(tag)

    return tags


def _match_class_suffix(filename: str) -> set:
    """Match class/file suffixes."""
    tags = set()
    basename = Path(filename).stem if '/' in filename else filename.split('.')[0]

    for suffix, tag in CLASS_SUFFIXES.items():
        # Case-insensitive suffix matching
        if basename.lower().endswith(suffix.lower()):
            tags.add(tag)
            break

    return tags


class SubagentSyncer:
    """
    Syncs subagent activity from Claude session logs to aOa intent tracking.

    Watches ~/.claude/projects/*/agent-*.jsonl files, extracts tool_use events,
    infers tags, and POSTs to /intent endpoint.
    """

    def __init__(self, redis_client, intent_url: str = "http://localhost:9999"):
        self.redis = redis_client
        self.intent_url = intent_url
        # Use CLAUDE_SESSIONS env var (Docker mount), fallback to host path
        claude_base = os.environ.get('CLAUDE_SESSIONS', os.path.expanduser('~/.claude'))
        self.claude_dir = Path(claude_base) / 'projects'
        self.lock = threading.Lock()
        self.last_sync = 0
        self.sync_interval = 5  # seconds

    def get_file_position(self, file_path: str) -> int:
        """Get last read position for a file."""
        pos = self.redis.hget(Keys.AGENT_SYNC, file_path)
        return int(pos) if pos else 0

    def set_file_position(self, file_path: str, position: int):
        """Store last read position for a file."""
        self.redis.hset(Keys.AGENT_SYNC, file_path, position)

    def infer_tags(self, file_path: str, tool_name: str) -> list[str]:
        """Infer semantic intent tags from file path and tool name.

        Uses pattern library (JSON configs) for intelligent tagging.
        """
        tags = set()

        # Add tool-based tags
        if tool_name in TOOL_TAGS:
            tags.update(TOOL_TAGS[tool_name])

        # Tokenize file path
        tokens = _tokenize(file_path)

        # Match semantic patterns (action verbs)
        semantic_tags = _match_semantic_tags(tokens)
        # Strip # prefix for consistency with legacy format
        tags.update(t.lstrip('#') for t in semantic_tags)

        # Match domain keywords
        domain_tags = _match_domain_tags(tokens, file_path)
        tags.update(t.lstrip('#') for t in domain_tags)

        # Match class/file suffixes
        suffix_tags = _match_class_suffix(file_path)
        tags.update(t.lstrip('#') for t in suffix_tags)

        # Fallback: If no semantic tags, use legacy patterns
        if len(tags) <= len(TOOL_TAGS.get(tool_name, [])):
            for pattern, pattern_tags in INTENT_PATTERNS:
                if re.search(pattern, file_path, re.IGNORECASE):
                    tags.update(pattern_tags)
                    break

        return list(tags)

    def parse_agent_file(self, file_path: Path) -> tuple[list[dict], dict]:
        """Parse new lines from an agent log file, extract tool_use events and costs."""
        events = []
        costs = {
            'input_tokens': 0,
            'output_tokens': 0,
            'cache_read_tokens': 0,
            'tool_calls': 0,
            'search_tools': 0,  # Grep, Glob - "old way"
            'read_tools': 0,    # Read calls
            'first_timestamp': None,
            'last_timestamp': None,
        }
        str_path = str(file_path)

        try:
            last_pos = self.get_file_position(str_path)
            file_size = file_path.stat().st_size

            # Skip if no new data
            if file_size <= last_pos:
                return events, costs

            with open(file_path) as f:
                f.seek(last_pos)
                for line in f:
                    try:
                        data = json.loads(line.strip())
                        msg = data.get('message', {})
                        content = msg.get('content', [])
                        session_id = data.get('sessionId', '')
                        agent_id = data.get('agentId', '')
                        timestamp = data.get('timestamp', '')

                        # Track timestamps for duration calculation
                        if timestamp:
                            if costs['first_timestamp'] is None:
                                costs['first_timestamp'] = timestamp
                            costs['last_timestamp'] = timestamp

                        # Extract token usage from assistant messages
                        usage = msg.get('usage', {})
                        if usage:
                            costs['input_tokens'] += usage.get('input_tokens', 0)
                            costs['output_tokens'] += usage.get('output_tokens', 0)
                            costs['cache_read_tokens'] += usage.get('cache_read_input_tokens', 0)

                        if isinstance(content, list):
                            for item in content:
                                if item.get('type') == 'tool_use':
                                    tool_name = item.get('name', '')
                                    tool_input = item.get('input', {})
                                    tool_use_id = item.get('id', '')

                                    # Track tool types for baseline calculation
                                    costs['tool_calls'] += 1
                                    if tool_name in ('Grep', 'Glob'):
                                        costs['search_tools'] += 1
                                    elif tool_name == 'Read':
                                        costs['read_tools'] += 1

                                    # Extract file path from tool input
                                    file_accessed = tool_input.get('file_path',
                                                    tool_input.get('path', ''))

                                    if file_accessed or tool_name in ('Bash', 'Grep', 'Glob'):
                                        events.append({
                                            'tool': tool_name,
                                            'file': file_accessed,
                                            'tool_input': tool_input,
                                            'session_id': session_id,
                                            'agent_id': agent_id,
                                            'tool_use_id': tool_use_id,
                                            'timestamp': timestamp,
                                        })
                    except json.JSONDecodeError:
                        continue

                # Update position
                self.set_file_position(str_path, f.tell())

        except Exception as e:
            print(f"Error parsing {file_path}: {e}")

        return events, costs

    def sync_project(self, project_slug: str) -> dict:
        """Sync all agent files for a project, tracking baseline costs."""
        project_dir = self.claude_dir / project_slug
        if not project_dir.exists():
            return {'synced': 0, 'events': 0, 'baseline': {}}

        synced_files = 0
        total_events = 0

        # Aggregate baseline costs across all agents
        baseline = {
            'total_tokens': 0,
            'input_tokens': 0,
            'output_tokens': 0,
            'tool_calls': 0,
            'search_tools': 0,  # Grep/Glob - these could be replaced by aOa
            'read_tools': 0,
            'potential_savings': {
                'tools': 0,       # Search tools that aOa could replace
                'tokens_est': 0,  # Estimated token savings
            }
        }

        for agent_file in project_dir.glob('agent-*.jsonl'):
            events, costs = self.parse_agent_file(agent_file)
            synced_files += 1

            # Aggregate baseline metrics
            baseline['input_tokens'] += costs['input_tokens']
            baseline['output_tokens'] += costs['output_tokens']
            baseline['total_tokens'] += costs['input_tokens'] + costs['output_tokens']
            baseline['tool_calls'] += costs['tool_calls']
            baseline['search_tools'] += costs['search_tools']
            baseline['read_tools'] += costs['read_tools']

            # Estimate potential savings:
            # - Each Grep/Glob could be replaced by 1 aOa search
            # - Estimated 500 tokens saved per search tool replaced
            # - Each Read after search could be more targeted
            baseline['potential_savings']['tools'] += costs['search_tools']
            baseline['potential_savings']['tokens_est'] += costs['search_tools'] * 500

            for event in events:
                # Infer tags
                tags = self.infer_tags(event['file'], event['tool'])

                # Add grep/glob patterns as additional context
                if event['tool'] in ('Grep', 'Glob'):
                    pattern = event['tool_input'].get('pattern', '')
                    if pattern:
                        pattern_tags = self.infer_tags(pattern, event['tool'])
                        tags.extend(pattern_tags)

                # Dedupe tags
                tags = list(set(tags))

                # POST to intent endpoint
                try:
                    payload = {
                        'tool': event['tool'],
                        'files': [event['file']] if event['file'] else [],
                        'tags': tags,
                        'session_id': event['session_id'],
                        'tool_use_id': event['tool_use_id'],
                        'source': f"agent:{event['agent_id']}",
                    }
                    requests.post(f"{self.intent_url}/intent", json=payload, timeout=1)
                    total_events += 1
                except Exception:
                    pass  # Don't block on intent failures

        return {
            'synced': synced_files,
            'events': total_events,
            'baseline': baseline
        }

    def sync_all(self) -> dict:
        """Sync all projects that have agent files, aggregate baseline costs."""
        now = time.time()

        # T-005: Move rate limit check inside lock to prevent TOCTOU race
        with self.lock:
            if now - self.last_sync < self.sync_interval:
                return {'skipped': True, 'reason': 'rate_limited'}
            self.last_sync = now
            results = {}
            total_baseline = {
                'total_tokens': 0,
                'tool_calls': 0,
                'search_tools': 0,
                'read_tools': 0,
                'potential_savings_tokens': 0,
            }

            if not self.claude_dir.exists():
                return {'error': f'Claude dir not found: {self.claude_dir}'}

            for project_dir in self.claude_dir.iterdir():
                if project_dir.is_dir():
                    project_slug = project_dir.name
                    proj_result = self.sync_project(project_slug)
                    results[project_slug] = proj_result

                    # Aggregate baseline across all projects
                    if 'baseline' in proj_result:
                        b = proj_result['baseline']
                        total_baseline['total_tokens'] += b.get('total_tokens', 0)
                        total_baseline['tool_calls'] += b.get('tool_calls', 0)
                        total_baseline['search_tools'] += b.get('search_tools', 0)
                        total_baseline['read_tools'] += b.get('read_tools', 0)
                        total_baseline['potential_savings_tokens'] += b.get('potential_savings', {}).get('tokens_est', 0)

            # Store aggregated baseline in Redis for metrics endpoint
            # Use hincrby for cumulative tracking (not hmset which overwrites)
            if total_baseline['total_tokens'] > 0:
                self.redis.hincrby('aoa:baseline', 'total_tokens', total_baseline['total_tokens'])
                self.redis.hincrby('aoa:baseline', 'tool_calls', total_baseline['tool_calls'])
                self.redis.hincrby('aoa:baseline', 'search_tools', total_baseline['search_tools'])
                self.redis.hincrby('aoa:baseline', 'potential_savings_tokens', total_baseline['potential_savings_tokens'])
            self.redis.hset('aoa:baseline', 'last_sync', int(now))

            return {
                'projects': results,
                'baseline': total_baseline,
                'synced_at': int(now),
            }

# =============================================================================
# Data Models
# =============================================================================

@dataclass
class TokenUsage:
    input_tokens: int = 0
    output_tokens: int = 0
    cache_read_tokens: int = 0
    cache_write_tokens: int = 0

    @property
    def total(self) -> int:
        return self.input_tokens + self.output_tokens

    @property
    def cache_hit_rate(self) -> float:
        total_input = self.input_tokens + self.cache_read_tokens
        if total_input == 0:
            return 0.0
        return self.cache_read_tokens / total_input

@dataclass
class SessionState:
    model: str = "unknown"
    context_used: int = 0
    context_max: int = 200000
    tokens: TokenUsage = field(default_factory=TokenUsage)
    session_cost: float = 0.0
    total_cost: float = 0.0
    session_start: float = 0.0
    last_activity: float = 0.0
    request_count: int = 0
    weekly_usage_pct: float = 0.0
    project: str = "default"

@dataclass
class StatusLine:
    """The formatted status line."""
    model: str
    context: str          # "42k/200k"
    tokens_in: str        # "12.4k"
    tokens_out: str       # "3.2k"
    cache_pct: str        # "89%"
    session_cost: str     # "$0.47"
    total_cost: str       # "$8.23"
    weekly_pct: str       # "78%"
    weekly_bar: str       # "████████░░"
    duration: str         # "00:34:12"

    def format(self) -> str:
        return (
            f"aOa ─ {self.model} │ "
            f"ctx: {self.context} │ "
            f"in: {self.tokens_in} out: {self.tokens_out} │ "
            f"cache: {self.cache_pct} │ "
            f"session: {self.session_cost} │ "
            f"total: {self.total_cost} │ "
            f"weekly: {self.weekly_bar} {self.weekly_pct} │ "
            f"{self.duration}"
        )

# =============================================================================
# Status Manager
# =============================================================================

class StatusManager:
    def __init__(self, redis_url: str):
        self.r = redis.from_url(redis_url, decode_responses=True)
        # T-007: Thread lock for concurrent access from Flask + daemon threads
        self.lock = threading.RLock()
        self._ensure_session()

    def _ensure_session(self):
        """Initialize session if needed."""
        if not self.r.hexists(Keys.SESSION, 'session_start'):
            now = time.time()
            self.r.hset(Keys.SESSION, mapping={
                'model': 'unknown',
                'context_used': 0,
                'input_tokens': 0,
                'output_tokens': 0,
                'cache_read_tokens': 0,
                'cache_write_tokens': 0,
                'session_cost': 0.0,
                'request_count': 0,
                'session_start': now,
                'last_activity': now,
                'project': 'default',
            })

    # =========================================================================
    # Event Recording
    # =========================================================================

    def record_request(
        self,
        model: str,
        input_tokens: int,
        output_tokens: int,
        cache_read_tokens: int = 0,
        cache_write_tokens: int = 0,
        context_used: int = 0,
    ):
        """Record a Claude API request."""
        # T-007: Lock to prevent race conditions between Flask threads
        with self.lock:
            now = time.time()

            # Calculate cost
            pricing = PRICING.get(model, PRICING['sonnet-4'])
            cost = (
                (input_tokens / 1_000_000) * pricing['input'] +
                (output_tokens / 1_000_000) * pricing['output'] +
                (cache_read_tokens / 1_000_000) * pricing['cache_read'] +
                (cache_write_tokens / 1_000_000) * pricing['cache_write']
            )

            # Update session
            pipe = self.r.pipeline()
            pipe.hset(Keys.SESSION, 'model', model)
            pipe.hset(Keys.SESSION, 'context_used', context_used)
            pipe.hset(Keys.SESSION, 'last_activity', now)
            pipe.hincrby(Keys.SESSION, 'input_tokens', input_tokens)
            pipe.hincrby(Keys.SESSION, 'output_tokens', output_tokens)
            pipe.hincrby(Keys.SESSION, 'cache_read_tokens', cache_read_tokens)
            pipe.hincrby(Keys.SESSION, 'cache_write_tokens', cache_write_tokens)
            pipe.hincrbyfloat(Keys.SESSION, 'session_cost', cost)
            pipe.hincrby(Keys.SESSION, 'request_count', 1)

            # Update totals
            pipe.hincrbyfloat(Keys.METRICS, 'total_cost', cost)
            pipe.hincrby(Keys.METRICS, 'total_requests', 1)
            pipe.hincrby(Keys.METRICS, 'total_input_tokens', input_tokens)
            pipe.hincrby(Keys.METRICS, 'total_output_tokens', output_tokens)

            # Daily tracking
            today = datetime.now().strftime('%Y-%m-%d')
            daily_key = Keys.DAILY.format(date=today)
            pipe.hincrbyfloat(daily_key, 'cost', cost)
            pipe.hincrby(daily_key, 'requests', 1)
            pipe.expire(daily_key, 86400 * 30)  # Keep 30 days

            # Weekly tracking
            pipe.hincrbyfloat(Keys.WEEKLY, 'cost', cost)
            pipe.hincrby(Keys.WEEKLY, 'requests', 1)

            # History
            event = json.dumps({
                'type': 'request',
                'model': model,
                'input': input_tokens,
                'output': output_tokens,
                'cache_read': cache_read_tokens,
                'cost': round(cost, 4),
                'ts': now,
            })
            pipe.lpush(Keys.HISTORY, event)
            pipe.ltrim(Keys.HISTORY, 0, 999)
            # R-005: 30-day TTL as fallback safety net if ltrim ever fails
            pipe.expire(Keys.HISTORY, 2592000)

            pipe.execute()

            return cost

    def record_model_switch(self, model: str):
        """Record a model change."""
        self.r.hset(Keys.SESSION, 'model', model)
        event = json.dumps({
            'type': 'model_switch',
            'model': model,
            'ts': time.time(),
        })
        self.r.lpush(Keys.HISTORY, event)

    def record_block(self, block_type: str, duration_seconds: int = 0):
        """Record a rate limit block."""
        event = json.dumps({
            'type': 'block',
            'block_type': block_type,
            'duration': duration_seconds,
            'ts': time.time(),
        })
        self.r.lpush(Keys.HISTORY, event)
        self.r.hincrby(Keys.METRICS, 'blocks_total', 1)

    def set_weekly_estimate(self, percentage: float):
        """Set estimated weekly usage percentage."""
        self.r.hset(Keys.WEEKLY, 'estimated_pct', percentage)

    def reset_session(self):
        """Reset session stats (keep totals)."""
        now = time.time()
        self.r.hset(Keys.SESSION, mapping={
            'context_used': 0,
            'input_tokens': 0,
            'output_tokens': 0,
            'cache_read_tokens': 0,
            'cache_write_tokens': 0,
            'session_cost': 0.0,
            'request_count': 0,
            'session_start': now,
            'last_activity': now,
        })

    def reset_weekly(self):
        """Reset weekly stats (call on week boundary)."""
        self.r.delete(Keys.WEEKLY)

    # =========================================================================
    # Status Retrieval
    # =========================================================================

    def get_session(self) -> SessionState:
        """Get current session state."""
        data = self.r.hgetall(Keys.SESSION)
        metrics = self.r.hgetall(Keys.METRICS)
        weekly = self.r.hgetall(Keys.WEEKLY)

        model = data.get('model', 'unknown')

        return SessionState(
            model=model,
            context_used=int(data.get('context_used', 0)),
            context_max=CONTEXT_LIMITS.get(model, 200000),
            tokens=TokenUsage(
                input_tokens=int(data.get('input_tokens', 0)),
                output_tokens=int(data.get('output_tokens', 0)),
                cache_read_tokens=int(data.get('cache_read_tokens', 0)),
                cache_write_tokens=int(data.get('cache_write_tokens', 0)),
            ),
            session_cost=float(data.get('session_cost', 0)),
            total_cost=float(metrics.get('total_cost', 0)),
            session_start=float(data.get('session_start', time.time())),
            last_activity=float(data.get('last_activity', time.time())),
            request_count=int(data.get('request_count', 0)),
            weekly_usage_pct=float(weekly.get('estimated_pct', 0)),
            project=data.get('project', 'default'),
        )

    def get_status_line(self) -> StatusLine:
        """Get formatted status line."""
        session = self.get_session()

        # Format helpers
        def fmt_tokens(n: int) -> str:
            if n >= 1000:
                return f"{n/1000:.1f}k"
            return str(n)

        def fmt_cost(c: float) -> str:
            return f"${c:.2f}"

        def fmt_duration(start: float) -> str:
            elapsed = int(time.time() - start)
            hours, remainder = divmod(elapsed, 3600)
            minutes, seconds = divmod(remainder, 60)
            return f"{hours:02d}:{minutes:02d}:{seconds:02d}"

        def fmt_bar(pct: float, width: int = 10) -> str:
            filled = int(pct / 100 * width)
            return '█' * filled + '░' * (width - filled)

        # Context
        ctx_used_k = session.context_used // 1000
        ctx_max_k = session.context_max // 1000
        context = f"{ctx_used_k}k/{ctx_max_k}k"

        # Cache hit rate
        cache_pct = int(session.tokens.cache_hit_rate * 100)

        # Weekly percentage
        weekly_pct = min(100, session.weekly_usage_pct)

        return StatusLine(
            model=session.model.replace('claude-', ''),
            context=context,
            tokens_in=fmt_tokens(session.tokens.input_tokens),
            tokens_out=fmt_tokens(session.tokens.output_tokens),
            cache_pct=f"{cache_pct}%",
            session_cost=fmt_cost(session.session_cost),
            total_cost=fmt_cost(session.total_cost),
            weekly_pct=f"{int(weekly_pct)}%",
            weekly_bar=fmt_bar(weekly_pct),
            duration=fmt_duration(session.session_start),
        )

    def get_history(self, limit: int = 50) -> list[dict]:
        """Get recent events."""
        events = self.r.lrange(Keys.HISTORY, 0, limit - 1)
        return [json.loads(e) for e in events]


# =============================================================================
# Flask API
# =============================================================================

manager: StatusManager | None = None

@app.route('/health')
def health():
    return jsonify({'status': 'ok', 'service': 'aoa-status'})

@app.route('/status')
def status():
    """Get formatted status line."""
    line = manager.get_status_line()
    return Response(line.format(), mimetype='text/plain')

@app.route('/status/json')
def status_json():
    """Get full status as JSON."""
    session = manager.get_session()
    return jsonify({
        'model': session.model,
        'context_used': session.context_used,
        'context_max': session.context_max,
        'tokens': {
            'input': session.tokens.input_tokens,
            'output': session.tokens.output_tokens,
            'cache_read': session.tokens.cache_read_tokens,
            'cache_write': session.tokens.cache_write_tokens,
            'cache_hit_rate': round(session.tokens.cache_hit_rate, 3),
        },
        'cost': {
            'session': round(session.session_cost, 4),
            'total': round(session.total_cost, 4),
        },
        'weekly_usage_pct': session.weekly_usage_pct,
        'session_duration_seconds': int(time.time() - session.session_start),
        'request_count': session.request_count,
        'project': session.project,
    })

@app.route('/status/line')
def status_line_only():
    """Get just the status line components."""
    line = manager.get_status_line()
    return jsonify(asdict(line))

@app.route('/session')
def session():
    """Get session info."""
    session = manager.get_session()
    return jsonify(asdict(session))

@app.route('/session/reset', methods=['POST'])
def session_reset():
    """Reset session stats."""
    manager.reset_session()
    return jsonify({'status': 'ok', 'message': 'Session reset'})

@app.route('/history')
def history():
    """Get recent events."""
    limit = int(request.args.get('limit', 50))
    events = manager.get_history(limit)
    return jsonify({'events': events})

@app.route('/event', methods=['POST'])
def record_event():
    """Record an event from Claude Code."""
    data = request.json
    event_type = data.get('type')

    if event_type == 'request':
        cost = manager.record_request(
            model=data.get('model', 'unknown'),
            input_tokens=data.get('input_tokens', 0),
            output_tokens=data.get('output_tokens', 0),
            cache_read_tokens=data.get('cache_read_tokens', 0),
            cache_write_tokens=data.get('cache_write_tokens', 0),
            context_used=data.get('context_used', 0),
        )
        return jsonify({'status': 'ok', 'cost': cost})

    elif event_type == 'model_switch':
        manager.record_model_switch(data.get('model', 'unknown'))
        return jsonify({'status': 'ok'})

    elif event_type == 'block':
        manager.record_block(
            block_type=data.get('block_type', 'unknown'),
            duration_seconds=data.get('duration', 0),
        )
        return jsonify({'status': 'ok'})

    elif event_type == 'weekly_estimate':
        manager.set_weekly_estimate(data.get('percentage', 0))
        return jsonify({'status': 'ok'})

    else:
        return jsonify({'status': 'error', 'message': f'Unknown event type: {event_type}'}), 400

@app.route('/weekly/reset', methods=['POST'])
def weekly_reset():
    """Reset weekly stats."""
    manager.reset_weekly()
    return jsonify({'status': 'ok', 'message': 'Weekly stats reset'})


# =============================================================================
# Subagent Sync Endpoints
# =============================================================================

@app.route('/sync/subagents', methods=['POST'])
def sync_subagents():
    """
    Trigger subagent activity sync.

    Scans all agent-*.jsonl files, extracts tool_use events,
    infers tags, sends to intent tracking, and calculates baseline costs.
    """
    if syncer is None:
        return jsonify({'error': 'Syncer not initialized'}), 503

    results = syncer.sync_all()
    return jsonify(results)


@app.route('/baseline')
def get_baseline():
    """
    Get baseline cost metrics from subagent activity.

    Returns aggregated token usage, tool calls, and potential savings
    if aOa had been used instead of Grep/Glob.
    """
    baseline = manager.r.hgetall('aoa:baseline')

    if not baseline:
        return jsonify({
            'baseline': {},
            'message': 'No baseline data yet. Trigger /sync/subagents first.'
        })

    # Convert string values to integers (Redis returns strings with decode_responses=True)
    return jsonify({
        'baseline': {
            'total_tokens': int(baseline.get('total_tokens', 0)),
            'tool_calls': int(baseline.get('tool_calls', 0)),
            'search_tools': int(baseline.get('search_tools', 0)),
            'potential_savings_tokens': int(baseline.get('potential_savings_tokens', 0)),
            'last_sync': int(baseline.get('last_sync', 0)),
        }
    })


# =============================================================================
# Main
# =============================================================================

def main():
    global manager, syncer

    print("Starting aOa Status Service")
    print(f"Redis: {REDIS_URL}")
    print(f"Port: {PORT}")

    manager = StatusManager(REDIS_URL)

    # Initialize subagent syncer
    # Use INDEX_URL from environment (matches docker-compose config)
    intent_url = os.environ.get('INDEX_URL', 'http://localhost:9999')
    try:
        syncer = SubagentSyncer(
            redis_client=manager.r,
            intent_url=intent_url
        )
        print("Subagent syncer initialized")
        print(f"  Claude dir: {syncer.claude_dir}")
        print(f"  Sync interval: {syncer.sync_interval}s")

        # Start background sync thread
        def background_sync():
            while True:
                try:
                    time.sleep(syncer.sync_interval)
                    syncer.sync_all()
                except Exception as e:
                    print(f"Background sync error: {e}")

        sync_thread = threading.Thread(target=background_sync, daemon=True)
        sync_thread.start()
        print("Background sync thread started")
    except Exception as e:
        print(f"Syncer initialization failed: {e}")
        syncer = None

    app.run(host='0.0.0.0', port=PORT, threaded=True)

if __name__ == '__main__':
    main()
