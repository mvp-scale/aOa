"""
Session Log Parser for aOa

Parses Claude's session logs to extract file access patterns
and build transition probabilities for predictive prefetch.

Session logs location: ~/.claude/projects/[project-slug]/agent-*.jsonl
"""

import json
from collections import defaultdict
from datetime import datetime
from pathlib import Path

# Import Redis client if available
try:
    from .redis_client import RedisClient
    REDIS_AVAILABLE = True
except ImportError:
    REDIS_AVAILABLE = False


# Redis key prefix for transitions
PREFIX_TRANSITION = "aoa:transition"


class SessionLogParser:
    """Parse Claude session logs to extract file access patterns."""

    def __init__(self, project_path: str = None):
        """
        Initialize parser for a project.

        Args:
            project_path: Absolute path to the project root
        """
        import os

        if not project_path:
            project_path = os.environ.get('AOA_PROJECT_PATH', '/app')

        # Convert path to slug: /home/user/project -> -home-user-project
        self.project_slug = project_path.replace('/', '-')
        self.project_path = project_path

        # Support Docker volume mount via CLAUDE_SESSIONS env var
        claude_sessions = os.environ.get('CLAUDE_SESSIONS')
        if claude_sessions:
            # Running in Docker with mounted sessions
            # Use AOA_PROJECT_SLUG if set, otherwise try to find the right project
            env_slug = os.environ.get('AOA_PROJECT_SLUG')
            if env_slug:
                self.base_path = Path(claude_sessions) / 'projects' / env_slug
            else:
                # Try common project paths, or fall back to searching
                projects_dir = Path(claude_sessions) / 'projects'
                candidate_slugs = [
                    '-home-corey-aOa',  # Default for aOa project
                    self.project_slug,  # Computed slug
                ]
                self.base_path = None
                for slug in candidate_slugs:
                    candidate = projects_dir / slug
                    if candidate.exists():
                        self.base_path = candidate
                        break

                # If still not found, use first project dir with session files
                if not self.base_path and projects_dir.exists():
                    for proj_dir in projects_dir.iterdir():
                        if proj_dir.is_dir() and list(proj_dir.glob('*.jsonl')):
                            self.base_path = proj_dir
                            break

                if not self.base_path:
                    self.base_path = projects_dir / self.project_slug
        else:
            # Running locally
            self.base_path = Path.home() / '.claude' / 'projects' / self.project_slug

    def list_sessions(self) -> list[Path]:
        """List all agent session files."""
        if not self.base_path.exists():
            return []
        # Agent files contain actual tool calls
        return sorted(self.base_path.glob('agent-*.jsonl'))

    def parse_session(self, session_file: Path) -> list[dict]:
        """
        Extract tool use events from a session file.

        Args:
            session_file: Path to the .jsonl session file

        Returns:
            List of tool events with tool name, input, and timestamp
        """
        events = []
        try:
            with open(session_file, encoding='utf-8') as f:
                for line in f:
                    if not line.strip():
                        continue
                    try:
                        entry = json.loads(line)
                    except json.JSONDecodeError:
                        continue

                    # Only process assistant messages with tool calls
                    if entry.get('type') != 'assistant':
                        continue

                    message = entry.get('message', {})
                    content = message.get('content', [])
                    timestamp = entry.get('timestamp', '')

                    # Extract tool_use items from content
                    for item in content:
                        if not isinstance(item, dict):
                            continue
                        if item.get('type') != 'tool_use':
                            continue

                        events.append({
                            'tool': item.get('name', ''),
                            'input': item.get('input', {}),
                            'timestamp': timestamp,
                            'session_file': session_file.name
                        })
        except Exception as e:
            # Log but don't crash on malformed files
            print(f"Warning: Could not parse {session_file}: {e}")

        return events

    def extract_file_reads(self, events: list[dict]) -> list[str]:
        """
        Get ordered list of files read from events.

        Args:
            events: List of tool events from parse_session

        Returns:
            List of file paths in access order
        """
        reads = []
        for e in events:
            if e['tool'] == 'Read':
                file_path = e['input'].get('file_path', '')
                if file_path:
                    reads.append(file_path)
        return reads

    def extract_file_writes(self, events: list[dict]) -> list[str]:
        """
        Get list of files written/edited.

        Args:
            events: List of tool events

        Returns:
            List of file paths that were written or edited
        """
        writes = []
        for e in events:
            if e['tool'] in ('Write', 'Edit'):
                file_path = e['input'].get('file_path', '')
                if file_path:
                    writes.append(file_path)
        return writes

    def normalize_path(self, file_path: str, project_root: str = None) -> str:
        """
        Normalize file path to relative project path.

        Args:
            file_path: Absolute or relative path
            project_root: Project root to make paths relative to

        Returns:
            Relative path from project root, or original if outside project
        """
        if not project_root:
            project_root = self.project_path
        if file_path.startswith(project_root):
            rel = file_path[len(project_root):]
            if rel.startswith('/'):
                rel = rel[1:]
            return rel
        return file_path

    def build_transition_matrix(self, normalize: bool = True) -> dict[str, dict[str, int]]:
        """
        Build file transition counts across all sessions.

        For each pair of consecutive Read events (A, B), increment
        transitions[A][B] by 1.

        Args:
            normalize: If True, convert absolute paths to relative

        Returns:
            Dict mapping from_file -> {to_file: count}
        """
        transitions: dict[str, dict[str, int]] = defaultdict(lambda: defaultdict(int))

        for session_file in self.list_sessions():
            events = self.parse_session(session_file)
            files = self.extract_file_reads(events)

            if normalize:
                files = [self.normalize_path(f) for f in files]

            # Build transitions from consecutive reads
            for i in range(len(files) - 1):
                from_file = files[i]
                to_file = files[i + 1]
                # Skip self-transitions (re-reading same file)
                if from_file != to_file:
                    transitions[from_file][to_file] += 1

        return dict(transitions)

    def get_transition_probabilities(self, from_file: str,
                                     transitions: dict[str, dict[str, int]]) -> list[tuple[str, float]]:
        """
        Get probability distribution for next file given current file.

        Args:
            from_file: Current file being read
            transitions: Transition matrix from build_transition_matrix

        Returns:
            List of (to_file, probability) sorted by probability descending
        """
        if from_file not in transitions:
            return []

        to_counts = transitions[from_file]
        total = sum(to_counts.values())
        if total == 0:
            return []

        probs = [(to_file, count / total) for to_file, count in to_counts.items()]
        return sorted(probs, key=lambda x: x[1], reverse=True)

    def get_stats(self) -> dict:
        """
        Get statistics about parsed sessions.

        Returns:
            Dict with session count, total events, unique files, etc.
        """
        sessions = self.list_sessions()
        total_events = 0
        total_reads = 0
        total_writes = 0
        unique_files = set()

        for session_file in sessions:
            events = self.parse_session(session_file)
            total_events += len(events)

            reads = self.extract_file_reads(events)
            writes = self.extract_file_writes(events)

            total_reads += len(reads)
            total_writes += len(writes)
            unique_files.update(reads)
            unique_files.update(writes)

        return {
            'session_count': len(sessions),
            'total_events': total_events,
            'total_reads': total_reads,
            'total_writes': total_writes,
            'unique_files': len(unique_files),
            'base_path': str(self.base_path)
        }

    def get_token_usage(self) -> dict:
        """
        Get token usage statistics from session logs.

        Parses all session logs and aggregates token counts including:
        - Total input tokens
        - Total output tokens
        - Cache creation tokens
        - Cache read tokens (savings from caching)

        Returns:
            Dict with token usage statistics and estimated cost savings
        """
        # Claude API pricing (as of Dec 2024)
        # Opus: $15/M input, $75/M output
        # Cache write: $18.75/M, Cache read: $1.50/M
        PRICE_INPUT = 15.0 / 1_000_000
        PRICE_OUTPUT = 75.0 / 1_000_000
        PRICE_CACHE_WRITE = 18.75 / 1_000_000
        PRICE_CACHE_READ = 1.50 / 1_000_000

        sessions = self.list_all_sessions()

        stats = {
            'input_tokens': 0,
            'output_tokens': 0,
            'cache_creation_tokens': 0,
            'cache_read_tokens': 0,
            'message_count': 0,
        }

        for session_file in sessions:
            try:
                with open(session_file) as f:
                    for line in f:
                        line = line.strip()
                        if not line:
                            continue
                        try:
                            event = json.loads(line)
                            if 'message' in event and 'usage' in event['message']:
                                usage = event['message']['usage']
                                stats['input_tokens'] += usage.get('input_tokens', 0)
                                stats['output_tokens'] += usage.get('output_tokens', 0)
                                stats['cache_creation_tokens'] += usage.get('cache_creation_input_tokens', 0)
                                stats['cache_read_tokens'] += usage.get('cache_read_input_tokens', 0)
                                stats['message_count'] += 1
                        except json.JSONDecodeError:
                            continue
            except Exception:
                continue

        # Calculate costs
        input_cost = stats['input_tokens'] * PRICE_INPUT
        output_cost = stats['output_tokens'] * PRICE_OUTPUT
        cache_write_cost = stats['cache_creation_tokens'] * PRICE_CACHE_WRITE
        cache_read_cost = stats['cache_read_tokens'] * PRICE_CACHE_READ

        # Cache savings = what we would have paid if cache reads were full price inputs
        cache_savings = stats['cache_read_tokens'] * (PRICE_INPUT - PRICE_CACHE_READ)

        total_cost = input_cost + output_cost + cache_write_cost + cache_read_cost
        total_tokens = stats['input_tokens'] + stats['output_tokens']

        return {
            'input_tokens': stats['input_tokens'],
            'output_tokens': stats['output_tokens'],
            'cache_creation_tokens': stats['cache_creation_tokens'],
            'cache_read_tokens': stats['cache_read_tokens'],
            'total_tokens': total_tokens,
            'message_count': stats['message_count'],

            # Costs in dollars
            'cost': {
                'input': round(input_cost, 4),
                'output': round(output_cost, 4),
                'cache_write': round(cache_write_cost, 4),
                'cache_read': round(cache_read_cost, 4),
                'total': round(total_cost, 4),
            },

            # Savings from caching
            'savings': {
                'from_cache': round(cache_savings, 4),
                'cache_hit_rate': round(
                    stats['cache_read_tokens'] / (stats['input_tokens'] + stats['cache_read_tokens'])
                    if (stats['input_tokens'] + stats['cache_read_tokens']) > 0 else 0, 4
                )
            }
        }

    def calculate_token_rate(self, sample_size: int = 100) -> dict:
        """
        Calculate actual ms_per_token rate from session history.

        Analyzes timestamps and token counts from recent messages to derive
        the real processing rate on this system. This rate can be used to
        estimate time savings from token savings.

        Args:
            sample_size: Number of recent messages to analyze

        Returns:
            Dict with calculated rate and confidence metrics
        """

        sessions = self.list_all_sessions()
        if not sessions:
            return {'ms_per_token': 0, 'samples': 0, 'confidence': 'none'}

        # Collect (duration_ms, tokens) pairs
        measurements = []

        for session_file in sessions[-10:]:  # Last 10 sessions
            try:
                messages = []
                with open(session_file) as f:
                    for line in f:
                        line = line.strip()
                        if not line:
                            continue
                        try:
                            event = json.loads(line)
                            if event.get('type') == 'assistant' and 'message' in event:
                                msg = event['message']
                                if 'usage' in msg and 'timestamp' in event:
                                    messages.append({
                                        'timestamp': event['timestamp'],
                                        'tokens': msg['usage'].get('input_tokens', 0) + msg['usage'].get('output_tokens', 0)
                                    })
                        except json.JSONDecodeError:
                            continue

                # Calculate duration between consecutive messages
                for i in range(1, len(messages)):
                    try:
                        # Parse ISO timestamps
                        t1 = datetime.fromisoformat(messages[i-1]['timestamp'].replace('Z', '+00:00'))
                        t2 = datetime.fromisoformat(messages[i]['timestamp'].replace('Z', '+00:00'))
                        duration_ms = (t2 - t1).total_seconds() * 1000
                        tokens = messages[i]['tokens']

                        # Filter outliers (>60s or <10ms probably not representative)
                        if 10 < duration_ms < 60000 and tokens > 0:
                            measurements.append({
                                'duration_ms': duration_ms,
                                'tokens': tokens,
                                'rate': duration_ms / tokens
                            })

                        if len(measurements) >= sample_size:
                            break
                    except (ValueError, TypeError):
                        continue

                if len(measurements) >= sample_size:
                    break

            except Exception:
                continue

        if not measurements:
            return {'ms_per_token': 0, 'samples': 0, 'confidence': 'none'}

        # Calculate statistics
        rates = [m['rate'] for m in measurements]
        avg_rate = sum(rates) / len(rates)

        # Use median for robustness against outliers
        sorted_rates = sorted(rates)
        median_rate = sorted_rates[len(sorted_rates) // 2]

        # Calculate range for display
        min_rate = min(rates)
        max_rate = max(rates)
        p25 = sorted_rates[len(sorted_rates) // 4] if len(sorted_rates) >= 4 else min_rate
        p75 = sorted_rates[3 * len(sorted_rates) // 4] if len(sorted_rates) >= 4 else max_rate

        # Confidence based on sample size
        if len(measurements) >= 50:
            confidence = 'high'
        elif len(measurements) >= 20:
            confidence = 'medium'
        else:
            confidence = 'low'

        return {
            'ms_per_token': round(median_rate, 3),
            'avg_ms_per_token': round(avg_rate, 3),
            'range': {
                'min': round(min_rate, 3),
                'max': round(max_rate, 3),
                'p25': round(p25, 3),
                'p75': round(p75, 3)
            },
            'samples': len(measurements),
            'confidence': confidence,
            'methodology': 'Calculated from session message timestamps and token counts'
        }

    def list_all_sessions(self) -> list[Path]:
        """List all session files (both agent-*.jsonl and regular *.jsonl)."""
        if not self.base_path.exists():
            return []
        return sorted(self.base_path.glob('*.jsonl'))

    def sync_to_redis(self, redis_client: 'RedisClient') -> dict:
        """
        Sync transition matrix to Redis sorted sets.

        For each from_file, creates a sorted set at aoa:transition:{from_file}
        with to_file as member and count as score.

        Args:
            redis_client: RedisClient instance

        Returns:
            Dict with sync statistics
        """
        transitions = self.build_transition_matrix()
        keys_written = 0
        total_transitions = 0

        for from_file, to_files in transitions.items():
            key = f"{PREFIX_TRANSITION}:{from_file}"
            for to_file, count in to_files.items():
                redis_client.zadd(key, count, to_file)
                total_transitions += 1
            keys_written += 1

        return {
            'keys_written': keys_written,
            'total_transitions': total_transitions
        }

    @staticmethod
    def predict_next(redis_client: 'RedisClient', current_file: str,
                     limit: int = 5) -> list[tuple[str, float]]:
        """
        Predict next files based on transition probabilities.

        Args:
            redis_client: RedisClient instance
            current_file: Current file being accessed
            limit: Max predictions to return

        Returns:
            List of (file_path, probability) tuples
        """
        key = f"{PREFIX_TRANSITION}:{current_file}"

        # Get top transitions by count
        results = redis_client.zrange(key, 0, limit - 1, desc=True, withscores=True)
        if not results:
            return []

        # Calculate total for probability
        total = sum(score for _, score in results)
        if total == 0:
            return []

        return [(file_path, score / total) for file_path, score in results]

    @staticmethod
    def get_all_predictions(redis_client: 'RedisClient', current_files: list[str],
                            limit: int = 5) -> list[tuple[str, float]]:
        """
        Get predictions based on multiple current files.

        Combines transition probabilities from all current files.

        Args:
            redis_client: RedisClient instance
            current_files: List of files currently being accessed
            limit: Max predictions to return

        Returns:
            List of (file_path, combined_score) tuples
        """
        combined: dict[str, float] = defaultdict(float)

        for current_file in current_files:
            predictions = SessionLogParser.predict_next(redis_client, current_file, limit=20)
            for file_path, prob in predictions:
                # Avoid predicting files already being accessed
                if file_path not in current_files:
                    combined[file_path] += prob

        # Sort by combined score
        sorted_predictions = sorted(combined.items(), key=lambda x: x[1], reverse=True)
        return sorted_predictions[:limit]


def main():
    """CLI for testing the parser."""
    import argparse
    import os

    parser = argparse.ArgumentParser(description='Parse Claude session logs')
    parser.add_argument('--project', default=None,
                        help='Project path to analyze')
    parser.add_argument('--stats', action='store_true',
                        help='Show statistics')
    parser.add_argument('--transitions', action='store_true',
                        help='Build and show transition matrix')
    parser.add_argument('--sync', action='store_true',
                        help='Sync transitions to Redis')
    parser.add_argument('--predict', type=str, metavar='FILE',
                        help='Predict next files for given file')
    parser.add_argument('--top', type=int, default=10,
                        help='Number of top transitions to show')
    parser.add_argument('--redis-url', default=os.environ.get('REDIS_URL', 'redis://localhost:6379/0'),
                        help='Redis URL')

    args = parser.parse_args()

    sp = SessionLogParser(args.project)

    if args.stats:
        stats = sp.get_stats()
        print(f"Sessions: {stats['session_count']}")
        print(f"Total events: {stats['total_events']}")
        print(f"Total reads: {stats['total_reads']}")
        print(f"Total writes: {stats['total_writes']}")
        print(f"Unique files: {stats['unique_files']}")
        print(f"Base path: {stats['base_path']}")

    if args.transitions:
        transitions = sp.build_transition_matrix()
        print(f"\nTransition matrix ({len(transitions)} source files):")

        # Flatten and sort by count
        all_transitions = []
        for from_file, to_files in transitions.items():
            for to_file, count in to_files.items():
                all_transitions.append((from_file, to_file, count))

        all_transitions.sort(key=lambda x: x[2], reverse=True)

        print(f"\nTop {args.top} transitions:")
        for from_file, to_file, count in all_transitions[:args.top]:
            print(f"  {from_file} -> {to_file}: {count}")

    if args.sync:
        if not REDIS_AVAILABLE:
            print("Error: Redis client not available. Run from package context.")
            return

        redis_client = RedisClient(url=args.redis_url)
        if not redis_client.ping():
            print(f"Error: Cannot connect to Redis at {args.redis_url}")
            return

        result = sp.sync_to_redis(redis_client)
        print("Synced to Redis:")
        print(f"  Keys written: {result['keys_written']}")
        print(f"  Total transitions: {result['total_transitions']}")

    if args.predict:
        if not REDIS_AVAILABLE:
            print("Error: Redis client not available. Run from package context.")
            return

        redis_client = RedisClient(url=args.redis_url)
        if not redis_client.ping():
            print(f"Error: Cannot connect to Redis at {args.redis_url}")
            return

        predictions = SessionLogParser.predict_next(redis_client, args.predict, limit=args.top)
        if predictions:
            print(f"Predictions for {args.predict}:")
            for file_path, prob in predictions:
                print(f"  {file_path}: {prob:.1%}")
        else:
            print(f"No predictions for {args.predict}")


if __name__ == '__main__':
    main()
