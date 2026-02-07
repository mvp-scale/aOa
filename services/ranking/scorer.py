"""
aOa Scorer - File Ranking by Recency, Frequency, and Tag Affinity

Provides predictive file scoring for prefetch optimization.
Includes Thompson Sampling for weight optimization (Phase 4).
"""

import math
import random
import time

from .redis_client import RedisClient


class Scorer:
    """
    File scorer using multiple signals:
    - Recency: Unix timestamp of last access (higher = more recent)
    - Frequency: Count of accesses (higher = more frequent)
    - Tag Affinity: Score per tag association (higher = stronger affinity)
    """

    # Default weights for composite scoring
    DEFAULT_WEIGHTS = {
        'recency': 0.4,    # Recent files are important
        'frequency': 0.3,  # Frequently accessed files matter
        'tag': 0.3,        # Tag matches are relevant
    }

    # Decay settings
    RECENCY_HALF_LIFE = 3600  # 1 hour: recency score halves

    # Confidence calculation settings (P2-001)
    MIN_ACCESSES_FULL_CONFIDENCE = 20  # Accesses needed for max evidence factor
    MIN_HOURS_FULL_CONFIDENCE = 24     # Hours needed for max stability factor
    EVIDENCE_WEIGHT = 0.7              # Weight for evidence factor
    STABILITY_WEIGHT = 0.3             # Weight for stability factor

    def __init__(self, redis_client: RedisClient | None = None, db: int | None = None):
        """
        Initialize scorer.

        Args:
            redis_client: Existing RedisClient instance (optional)
            db: Database number for testing (optional)
        """
        self.redis = redis_client or RedisClient(db=db)
        self.weights = self.DEFAULT_WEIGHTS.copy()

    # =========================================================================
    # Recording Access
    # =========================================================================

    def record_access(self, file_path: str, tags: list[str] | None = None,
                      timestamp: int | None = None) -> dict[str, float]:
        """
        Record a file access, updating all scoring signals.

        Args:
            file_path: Path to the file being accessed
            tags: List of tags associated with this access
            timestamp: Unix timestamp (defaults to now)

        Returns:
            Dict with updated scores for each signal
        """
        ts = timestamp or int(time.time())
        tags = tags or []

        scores = {}

        # Update recency (set to timestamp - higher is more recent)
        recency_key = RedisClient.PREFIX_RECENCY
        self.redis.zadd(recency_key, ts, file_path)
        scores['recency'] = float(ts)

        # Update frequency (increment by 1)
        frequency_key = RedisClient.PREFIX_FREQUENCY
        scores['frequency'] = self.redis.zincrby(frequency_key, 1, file_path)

        # Track first_seen for confidence calculation (P2-001)
        # Use SETNX to only set if key doesn't exist
        first_seen_key = f"aoa:first_seen:{file_path}"
        self.redis.client.setnx(first_seen_key, ts)

        # Update tag affinity (increment each tag's score for this file)
        for tag in tags:
            tag_key = f"{RedisClient.PREFIX_TAG}:{tag}"
            self.redis.zincrby(tag_key, 1, file_path)
            scores[f'tag:{tag}'] = self.redis.zscore(tag_key, file_path)

        return scores

    # =========================================================================
    # Score Retrieval
    # =========================================================================

    def get_recency_score(self, file_path: str) -> float | None:
        """Get recency score for a file (timestamp of last access)."""
        return self.redis.zscore(RedisClient.PREFIX_RECENCY, file_path)

    def get_frequency_score(self, file_path: str) -> float | None:
        """Get frequency score for a file (access count)."""
        return self.redis.zscore(RedisClient.PREFIX_FREQUENCY, file_path)

    def get_tag_score(self, file_path: str, tag: str) -> float | None:
        """Get tag affinity score for a file and tag."""
        tag_key = f"{RedisClient.PREFIX_TAG}:{tag}"
        return self.redis.zscore(tag_key, file_path)

    def get_first_seen(self, file_path: str) -> float | None:
        """Get first_seen timestamp for a file."""
        first_seen_key = f"aoa:first_seen:{file_path}"
        val = self.redis.client.get(first_seen_key)
        if val:
            return float(val.decode() if isinstance(val, bytes) else val)
        return None

    # =========================================================================
    # Confidence Calculation (P2-001)
    # =========================================================================

    def calculate_confidence(self, composite: float, access_count: int,
                             time_span_hours: float) -> float:
        """
        Calculate confidence score from composite score and evidence.

        Confidence reflects BOTH the score AND how much evidence we have.
        High score + few accesses = low confidence (might be noise)
        Medium score + many accesses = higher confidence (reliable pattern)

        Args:
            composite: Weighted score 0-100
            access_count: Total accesses recorded for this file
            time_span_hours: Hours since first access

        Returns:
            Confidence 0.0-1.0
        """
        # Base confidence from composite (0-1)
        base = composite / 100.0

        # Evidence factor: more accesses = more confident
        # Uses log scale: 1 access = 0.3, 5 = 0.6, 20+ = 0.9+
        evidence = min(1.0, 0.3 + 0.7 * math.log1p(access_count) /
                       math.log1p(self.MIN_ACCESSES_FULL_CONFIDENCE))

        # Time stability factor: longer history = more stable
        # Ramps up over 24 hours
        stability = min(1.0, 0.5 + 0.5 * time_span_hours /
                        self.MIN_HOURS_FULL_CONFIDENCE)

        # Combined confidence
        # Weight base score by evidence (more important) and stability (less important)
        confidence = base * (self.EVIDENCE_WEIGHT * evidence +
                            self.STABILITY_WEIGHT * stability)

        return round(confidence, 4)

    # =========================================================================
    # Ranking
    # =========================================================================

    def get_ranked_files(self, tags: list[str] | None = None,
                         limit: int = 10, db: int | None = None) -> list[dict]:
        """
        Get files ranked by composite score.

        Args:
            tags: Filter/boost by these tags (optional)
            limit: Maximum number of files to return
            db: Database number (for benchmark testing)

        Returns:
            List of dicts with 'file', 'score', and individual signal scores
        """
        import math

        if db is not None:
            self.redis.client.select(db)

        now = time.time()

        # Get all files from recency and frequency sets
        recency_files = self.redis.zrange(RedisClient.PREFIX_RECENCY, 0, -1,
                                          desc=True, withscores=True)
        frequency_files = self.redis.zrange(RedisClient.PREFIX_FREQUENCY, 0, -1,
                                            desc=True, withscores=True)

        # Build file -> scores map
        file_scores = {}

        # Process recency scores (normalize to 0-100 based on age)
        for file_path, timestamp in recency_files:
            age_seconds = now - timestamp
            # Exponential decay: score = 100 * e^(-age / half_life)
            # Half life = 1 hour = 3600 seconds
            recency_score = 100 * math.exp(-age_seconds / self.RECENCY_HALF_LIFE)
            file_scores[file_path] = {'recency': recency_score, 'frequency': 0, 'tags': {}}

        # Process frequency scores (already in reasonable range)
        max_freq = max((f[1] for f in frequency_files), default=1)
        for file_path, freq in frequency_files:
            # Normalize to 0-100 range
            freq_score = (freq / max_freq) * 100 if max_freq > 0 else 0
            if file_path not in file_scores:
                file_scores[file_path] = {'recency': 0, 'frequency': 0, 'tags': {}}
            file_scores[file_path]['frequency'] = freq_score

        # Process tag scores if tags specified
        if tags:
            for tag in tags:
                tag_key = f"{RedisClient.PREFIX_TAG}:{tag}"
                tag_files = self.redis.zrange(tag_key, 0, -1, desc=True, withscores=True)
                max_tag = max((f[1] for f in tag_files), default=1)
                for file_path, tag_score in tag_files:
                    # Normalize to 0-100 range
                    normalized_tag = (tag_score / max_tag) * 100 if max_tag > 0 else 0
                    if file_path not in file_scores:
                        file_scores[file_path] = {'recency': 0, 'frequency': 0, 'tags': {}}
                    file_scores[file_path]['tags'][tag] = normalized_tag

        # If no files, return empty
        if not file_scores:
            return []

        # Calculate composite scores
        for file_path, scores in file_scores.items():
            composite = (
                scores['recency'] * self.weights['recency'] +
                scores['frequency'] * self.weights['frequency']
            )

            # Add tag contribution
            if tags and scores['tags']:
                tag_weight = self.weights['tag'] / len(tags)
                for tag in tags:
                    composite += scores['tags'].get(tag, 0) * tag_weight

            scores['composite'] = composite

        # Sort by composite score and return top N
        sorted_files = sorted(file_scores.items(),
                              key=lambda x: x[1]['composite'],
                              reverse=True)[:limit]

        # Build response with confidence calculation
        ranked_files = []
        for file_path, scores in sorted_files:
            # Get raw access count for confidence calculation
            raw_freq = self.get_frequency_score(file_path) or 1

            # Get first_seen for time span calculation
            first_seen = self.get_first_seen(file_path)
            if first_seen:
                time_span_hours = (now - first_seen) / 3600
            else:
                time_span_hours = 0

            # Calculate calibrated confidence (P2-001)
            confidence = self.calculate_confidence(
                composite=scores['composite'],
                access_count=int(raw_freq),
                time_span_hours=time_span_hours
            )

            entry = {
                'file': file_path,
                'score': round(scores['composite'], 4),
                'confidence': confidence,
                'recency': round(scores['recency'], 2),
                'frequency': round(scores['frequency'], 2),
            }
            if tags and scores['tags']:
                entry['tags'] = {k: round(v, 2) for k, v in scores['tags'].items()}
            ranked_files.append(entry)

        return ranked_files

    def get_top_files_by_recency(self, limit: int = 10) -> list[tuple[str, float]]:
        """Get files sorted by most recent access."""
        return self.redis.zrange(RedisClient.PREFIX_RECENCY, 0, limit - 1,
                                 desc=True, withscores=True)

    def get_top_files_by_frequency(self, limit: int = 10) -> list[tuple[str, float]]:
        """Get files sorted by most frequent access."""
        return self.redis.zrange(RedisClient.PREFIX_FREQUENCY, 0, limit - 1,
                                 desc=True, withscores=True)

    def get_files_for_tag(self, tag: str, limit: int = 10) -> list[tuple[str, float]]:
        """Get files with highest affinity for a tag."""
        tag_key = f"{RedisClient.PREFIX_TAG}:{tag}"
        return self.redis.zrange(tag_key, 0, limit - 1, desc=True, withscores=True)

    # =========================================================================
    # Weight Management
    # =========================================================================

    def set_weights(self, recency: float = None, frequency: float = None,
                    tag: float = None) -> dict[str, float]:
        """
        Update scoring weights.

        Args:
            recency: Weight for recency signal (0.0-1.0)
            frequency: Weight for frequency signal (0.0-1.0)
            tag: Weight for tag affinity signal (0.0-1.0)

        Returns:
            Current weights after update
        """
        if recency is not None:
            self.weights['recency'] = max(0.0, min(1.0, recency))
        if frequency is not None:
            self.weights['frequency'] = max(0.0, min(1.0, frequency))
        if tag is not None:
            self.weights['tag'] = max(0.0, min(1.0, tag))

        return self.weights.copy()

    def get_weights(self) -> dict[str, float]:
        """Get current scoring weights."""
        return self.weights.copy()

    # =========================================================================
    # Decay (P1-008)
    # =========================================================================

    # =========================================================================
    # Statistics
    # =========================================================================

    def get_stats(self) -> dict:
        """Get statistics about current scoring state."""
        recency_count = self.redis.zcard(RedisClient.PREFIX_RECENCY)
        frequency_count = self.redis.zcard(RedisClient.PREFIX_FREQUENCY)

        # Count tag keys
        tag_keys = self.redis.keys(f"{RedisClient.PREFIX_TAG}:*")
        tag_count = len(tag_keys)

        total_tag_entries = sum(self.redis.zcard(k) for k in tag_keys)

        return {
            'files_tracked': recency_count,
            'frequency_entries': frequency_count,
            'tags_tracked': tag_count,
            'tag_associations': total_tag_entries,
            'weights': self.weights.copy(),
        }

    def clear_all(self) -> int:
        """Clear all scoring data. Use with caution!"""
        keys = []
        keys.extend(self.redis.keys(f"{RedisClient.PREFIX_RECENCY}*"))
        keys.extend(self.redis.keys(f"{RedisClient.PREFIX_FREQUENCY}*"))
        keys.extend(self.redis.keys(f"{RedisClient.PREFIX_TAG}:*"))
        keys.extend(self.redis.keys(f"{RedisClient.PREFIX_COMPOSITE}:*"))

        if keys:
            return self.redis.delete(*keys)
        return 0


# =============================================================================
# Weight Tuner - Thompson Sampling for Phase 4
# =============================================================================

class WeightTuner:
    """
    Thompson Sampling weight optimizer for Hit@5 maximization.

    Maintains Beta distributions for discrete weight configurations (arms),
    updating based on hit/miss feedback from predictions.

    Usage:
        tuner = WeightTuner(redis_client)
        weights = tuner.select_weights()  # Get weights for this prediction
        # ... make prediction, observe outcome ...
        tuner.record_feedback(hit=True)   # Record outcome
    """

    # Discrete weight configurations (arms) to try
    # Each must sum to 1.0
    ARMS = [
        {'recency': 0.5, 'frequency': 0.3, 'tag': 0.2, 'name': 'recency-heavy'},
        {'recency': 0.4, 'frequency': 0.4, 'tag': 0.2, 'name': 'balanced-rf'},
        {'recency': 0.4, 'frequency': 0.3, 'tag': 0.3, 'name': 'default'},
        {'recency': 0.3, 'frequency': 0.4, 'tag': 0.3, 'name': 'frequency-heavy'},
        {'recency': 0.3, 'frequency': 0.3, 'tag': 0.4, 'name': 'tag-heavy'},
        {'recency': 0.2, 'frequency': 0.4, 'tag': 0.4, 'name': 'low-recency'},
        {'recency': 0.5, 'frequency': 0.2, 'tag': 0.3, 'name': 'high-rec-low-freq'},
        {'recency': 0.33, 'frequency': 0.33, 'tag': 0.34, 'name': 'equal'},
    ]

    # Redis key prefix for tuner data
    REDIS_PREFIX = "aoa:tuner"

    def __init__(self, redis_client: RedisClient | None = None):
        """
        Initialize with Beta(1,1) priors (uniform) for each arm.

        Args:
            redis_client: RedisClient instance for persistence
        """
        self.redis = redis_client
        self._current_arm = 0  # Track selected arm for feedback

    def _get_arm_stats(self, arm_idx: int) -> tuple[int, int]:
        """
        Get (alpha, beta) for an arm from Redis.

        Beta(alpha, beta) represents probability of success.
        Prior: Beta(1, 1) = uniform

        Redis stores the number of hits/misses (starting from 0).
        We add 1 to each to get the Beta parameters (prior + observations).
        """
        if self.redis:
            key = f"{self.REDIS_PREFIX}:arm:{arm_idx}"
            alpha_raw = self.redis.client.hget(key, "alpha")
            beta_raw = self.redis.client.hget(key, "beta")

            # Redis stores observations (0-indexed), add 1 for prior
            alpha = int(alpha_raw) + 1 if alpha_raw else 1
            beta = int(beta_raw) + 1 if beta_raw else 1
            return (alpha, beta)
        return (1, 1)  # Default prior

    def _update_arm_stats(self, arm_idx: int, hit: bool):
        """Update arm stats after feedback."""
        if self.redis:
            key = f"{self.REDIS_PREFIX}:arm:{arm_idx}"
            if hit:
                self.redis.client.hincrby(key, "alpha", 1)
            else:
                self.redis.client.hincrby(key, "beta", 1)

    def select_weights(self) -> dict[str, float]:
        """
        Select weights using Thompson Sampling.

        1. Sample from each arm's Beta distribution
        2. Select arm with highest sample
        3. Return that arm's weights

        Returns:
            Dict with 'recency', 'frequency', 'tag' weights
        """
        best_arm = 0
        best_sample = -1.0

        for idx in range(len(self.ARMS)):
            alpha, beta = self._get_arm_stats(idx)
            # Sample from Beta(alpha, beta)
            sample = random.betavariate(alpha, beta)
            if sample > best_sample:
                best_sample = sample
                best_arm = idx

        # Store selected arm for feedback
        self._current_arm = best_arm

        # Return weights (excluding 'name' key)
        arm = self.ARMS[best_arm]
        return {
            'recency': arm['recency'],
            'frequency': arm['frequency'],
            'tag': arm['tag'],
            '_arm_idx': best_arm,  # Include for tracking
        }

    def record_feedback(self, hit: bool, arm_idx: int | None = None):
        """
        Record hit/miss feedback for the selected arm.

        Args:
            hit: True if prediction was a hit
            arm_idx: Arm index (defaults to last selected)
        """
        arm = arm_idx if arm_idx is not None else self._current_arm
        self._update_arm_stats(arm, hit)

    def get_best_weights(self) -> dict[str, float]:
        """
        Get the arm with highest expected success rate.
        (For exploitation only, no exploration)
        """
        best_arm = 0
        best_mean = 0.0

        for idx in range(len(self.ARMS)):
            alpha, beta = self._get_arm_stats(idx)
            mean = alpha / (alpha + beta)
            if mean > best_mean:
                best_mean = mean
                best_arm = idx

        arm = self.ARMS[best_arm]
        return {
            'recency': arm['recency'],
            'frequency': arm['frequency'],
            'tag': arm['tag'],
            '_arm_idx': best_arm,
            '_mean': best_mean,
        }

    def get_stats(self) -> list[dict]:
        """
        Get statistics for all arms, sorted by mean success rate.

        Returns:
            List of dicts with arm info, alpha, beta, mean, samples
        """
        stats = []
        for idx, arm in enumerate(self.ARMS):
            alpha, beta = self._get_arm_stats(idx)
            samples = alpha + beta - 2  # Subtract prior

            stats.append({
                'arm_idx': idx,
                'name': arm.get('name', f'arm-{idx}'),
                'weights': {
                    'recency': arm['recency'],
                    'frequency': arm['frequency'],
                    'tag': arm['tag'],
                },
                'alpha': alpha,
                'beta': beta,
                'mean': round(alpha / (alpha + beta), 4),
                'samples': samples,
            })

        # Sort by mean descending
        return sorted(stats, key=lambda x: x['mean'], reverse=True)

    def reset(self):
        """Reset all arm statistics to priors."""
        if self.redis:
            for idx in range(len(self.ARMS)):
                key = f"{self.REDIS_PREFIX}:arm:{idx}"
                self.redis.client.delete(key)
