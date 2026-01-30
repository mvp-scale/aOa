"""
Redis Client for aOa Ranking

Wraps redis-py with aOa-specific sorted set operations for file scoring.
"""

import os

import redis


class RedisClient:
    """Redis client wrapper for scoring operations."""

    # Key prefixes for different score types
    PREFIX_RECENCY = "aoa:recency"
    PREFIX_FREQUENCY = "aoa:frequency"
    PREFIX_TAG = "aoa:tag"
    PREFIX_COMPOSITE = "aoa:composite"

    def __init__(self, url: str | None = None, db: int | None = None):
        """
        Initialize Redis connection.

        Args:
            url: Redis URL (defaults to REDIS_URL env var or localhost)
            db: Database number (overrides URL's db if provided)
        """
        self.url = url or os.environ.get('REDIS_URL', 'redis://localhost:6379/0')
        self._client: redis.Redis | None = None
        self._db_override = db

    @property
    def client(self) -> redis.Redis:
        """Lazy-initialize Redis connection."""
        if self._client is None:
            # R-018: Explicit connection pool with max_connections
            pool = redis.ConnectionPool.from_url(
                self.url, decode_responses=True, max_connections=10
            )
            self._client = redis.Redis(connection_pool=pool)
            if self._db_override is not None:
                self._client.select(self._db_override)
        return self._client

    def ping(self) -> bool:
        """Check if Redis is available."""
        try:
            return self.client.ping()
        except redis.ConnectionError:
            return False

    # =========================================================================
    # Sorted Set Operations
    # =========================================================================

    def zadd(self, key: str, score: float, member: str) -> int:
        """
        Add member to sorted set with score.

        Args:
            key: Redis key for the sorted set
            score: Score value (higher = more relevant)
            member: The file path or identifier

        Returns:
            Number of elements added (0 if updated, 1 if new)
        """
        return self.client.zadd(key, {member: score})

    def zincrby(self, key: str, increment: float, member: str) -> float:
        """
        Increment member's score in sorted set.

        Args:
            key: Redis key for the sorted set
            increment: Amount to add to score
            member: The file path or identifier

        Returns:
            New score after increment
        """
        return self.client.zincrby(key, increment, member)

    def zrange(self, key: str, start: int = 0, end: int = -1,
               desc: bool = True, withscores: bool = False) -> list:
        """
        Get range of members from sorted set.

        Args:
            key: Redis key for the sorted set
            start: Start index (0-based)
            end: End index (-1 for all)
            desc: If True, return highest scores first
            withscores: If True, return (member, score) tuples

        Returns:
            List of members or (member, score) tuples
        """
        if desc:
            return self.client.zrevrange(key, start, end, withscores=withscores)
        return self.client.zrange(key, start, end, withscores=withscores)

    def zscore(self, key: str, member: str) -> float | None:
        """Get score of member in sorted set."""
        return self.client.zscore(key, member)

    def zcard(self, key: str) -> int:
        """Get number of members in sorted set."""
        return self.client.zcard(key)

    def zrem(self, key: str, *members: str) -> int:
        """Remove members from sorted set."""
        return self.client.zrem(key, *members)

    # =========================================================================
    # Compound Operations
    # =========================================================================

    def zunionstore(self, dest: str, keys: list[str],
                    weights: list[float] | None = None,
                    aggregate: str = 'SUM') -> int:
        """
        Union multiple sorted sets with optional weights.

        Args:
            dest: Destination key for result
            keys: Source sorted set keys
            weights: Weight multipliers for each key (optional)
            aggregate: How to combine scores (SUM, MIN, MAX)

        Returns:
            Number of elements in resulting set
        """
        # redis-py expects a dict {key: weight} or list of keys
        if weights:
            key_weights = dict(zip(keys, weights))
            return self.client.zunionstore(dest, key_weights, aggregate=aggregate)
        return self.client.zunionstore(dest, keys, aggregate=aggregate)

    def zinterstore(self, dest: str, keys: list[str],
                    weights: list[float] | None = None,
                    aggregate: str = 'SUM') -> int:
        """
        Intersect multiple sorted sets with optional weights.

        Args:
            dest: Destination key for result
            keys: Source sorted set keys
            weights: Weight multipliers for each key (optional)
            aggregate: How to combine scores (SUM, MIN, MAX)

        Returns:
            Number of elements in resulting set
        """
        # redis-py expects a dict {key: weight} or list of keys
        if weights:
            key_weights = dict(zip(keys, weights))
            return self.client.zinterstore(dest, key_weights, aggregate=aggregate)
        return self.client.zinterstore(dest, keys, aggregate=aggregate)

    # =========================================================================
    # Utility Operations
    # =========================================================================

    def keys(self, pattern: str) -> list[str]:
        """Get all keys matching pattern."""
        return self.client.keys(pattern)

    def delete(self, *keys: str) -> int:
        """Delete one or more keys."""
        return self.client.delete(*keys)

    def flushdb(self) -> bool:
        """Clear current database. Use with caution!"""
        return self.client.flushdb()

    def expire(self, key: str, seconds: int) -> bool:
        """Set key expiration."""
        return self.client.expire(key, seconds)

    def ttl(self, key: str) -> int:
        """Get remaining TTL for key."""
        return self.client.ttl(key)

    # =========================================================================
    # Lua Script Support (for atomic operations)
    # =========================================================================

    def eval(self, script: str, keys: list[str], args: list) -> any:
        """
        Execute Lua script.

        Args:
            script: Lua script text
            keys: KEYS array for script
            args: ARGV array for script

        Returns:
            Script result
        """
        return self.client.eval(script, len(keys), *keys, *args)

    def register_script(self, script: str):
        """
        Register Lua script for efficient repeated execution.

        Args:
            script: Lua script text

        Returns:
            Script object that can be called with (keys, args)
        """
        return self.client.register_script(script)
