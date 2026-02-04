#!/usr/bin/env python3
"""
GL-089: Redis Work Queue for aOa

Universal job queue that handles all aOa background work:
- Intelligence phase: domain enrichment, keyword mapping
- Intent phase: pattern discovery, file analysis
- Maintenance: cleanup, tuning, rebalancing

Queue is the single source of truth. If it's in pending, it needs work.
If it's in complete, it's done. Simple.
"""

import json
import os
import time
import uuid
from dataclasses import dataclass, asdict
from enum import Enum
from typing import Optional

# Redis client
import sys
_services_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, _services_dir)
sys.path.insert(0, '/app/services')

try:
    from ranking.redis_client import RedisClient
except ImportError:
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


class JobType(str, Enum):
    """All supported job types."""
    # Intelligence phase
    ENRICH = "enrich"              # Generate terms + keywords for domain
    MAP_KEYWORDS = "map_keywords"  # Map keywords to codebase files

    # Intent phase
    ANALYZE_INTENT = "analyze_intent"    # Analyze recent file patterns
    DISCOVER_DOMAIN = "discover_domain"  # Create new domain from patterns

    # Maintenance
    CLEANUP = "cleanup"    # Deprecate stale domains
    TUNE = "tune"          # Rebalance/optimize domains
    REINDEX = "reindex"    # Rebuild keyword mappings

    # Stop hook triggered (SH-02c)
    SCRAPE = "scrape"      # Session scrape: bigrams + file hits (every stop)
    AUTOTUNE = "autotune"  # Decay, prune, promote (every 100 stops)


class JobStatus(str, Enum):
    """Job lifecycle states."""
    PENDING = "pending"
    ACTIVE = "active"
    COMPLETE = "complete"
    FAILED = "failed"


@dataclass
class Job:
    """A unit of work."""
    id: str
    type: JobType
    project_id: str
    payload: dict  # Type-specific data
    phase: str = "intelligence"  # "intelligence" or "intent"
    priority: int = 5  # 1=highest, 10=lowest
    created_at: float = 0
    started_at: float = 0
    completed_at: float = 0
    error: str = ""

    def __post_init__(self):
        if not self.created_at:
            self.created_at = time.time()
        if not self.id:
            self.id = str(uuid.uuid4())[:8]

    def to_json(self) -> str:
        d = asdict(self)
        d['type'] = self.type.value if isinstance(self.type, JobType) else self.type
        return json.dumps(d)

    @classmethod
    def from_json(cls, data: str) -> 'Job':
        d = json.loads(data)
        d['type'] = JobType(d['type'])
        return cls(**d)


class JobQueue:
    """
    Redis-backed job queue for aOa.

    Keys:
        aoa:{project}:jobs:pending  - LIST of jobs waiting to be processed
        aoa:{project}:jobs:active   - HASH of jobs currently being processed
        aoa:{project}:jobs:complete - LIST of completed jobs (capped for audit)
        aoa:{project}:jobs:failed   - LIST of failed jobs (for retry/debug)
        aoa:{project}:jobs:stats    - HASH of counters
    """

    COMPLETE_CAP = 100  # Keep last N completed jobs for audit

    def __init__(self, project_id: str, redis_url: Optional[str] = None):
        self.project_id = project_id
        self.redis = RedisClient(url=redis_url)

    def _key(self, suffix: str) -> str:
        return f"aoa:{self.project_id}:jobs:{suffix}"

    # =========================================================================
    # Core Queue Operations
    # =========================================================================

    def push(self, job: Job) -> str:
        """Add job to pending queue. Returns job ID."""
        self.redis.client.rpush(self._key("pending"), job.to_json())
        self.redis.client.hincrby(self._key("stats"), "total_pushed", 1)
        return job.id

    def push_many(self, jobs: list[Job]) -> int:
        """Add multiple jobs to pending queue. Returns count. Skips duplicates."""
        if not jobs:
            return 0

        # Skip jobs already in pending (idempotent push)
        existing = {j.id for j in self.pending_jobs(100)}
        new_jobs = [j for j in jobs if j.id not in existing]

        if not new_jobs:
            return 0

        pipe = self.redis.client.pipeline()
        for job in new_jobs:
            pipe.rpush(self._key("pending"), job.to_json())
        pipe.hincrby(self._key("stats"), "total_pushed", len(new_jobs))
        pipe.execute()
        return len(new_jobs)

    def pop(self, timeout: int = 0) -> Optional[Job]:
        """
        Get next job from pending queue and move to active.

        Args:
            timeout: Seconds to wait for job (0 = no wait, None = block forever)

        Returns:
            Job if available, None if queue empty
        """
        if timeout:
            result = self.redis.client.blpop(self._key("pending"), timeout=timeout)
            if not result:
                return None
            job_json = result[1]
        else:
            job_json = self.redis.client.lpop(self._key("pending"))
            if not job_json:
                return None

        job = Job.from_json(job_json)
        job.started_at = time.time()

        # Move to active
        self.redis.client.hset(self._key("active"), job.id, job.to_json())
        return job

    def complete(self, job: Job) -> None:
        """Mark job as complete."""
        job.completed_at = time.time()

        pipe = self.redis.client.pipeline()
        # Remove from active
        pipe.hdel(self._key("active"), job.id)
        # Add to complete (capped list)
        pipe.lpush(self._key("complete"), job.to_json())
        pipe.ltrim(self._key("complete"), 0, self.COMPLETE_CAP - 1)
        # Update stats
        pipe.hincrby(self._key("stats"), "total_complete", 1)
        pipe.execute()

    def fail(self, job: Job, error: str) -> None:
        """Mark job as failed."""
        job.completed_at = time.time()
        job.error = error

        pipe = self.redis.client.pipeline()
        # Remove from active
        pipe.hdel(self._key("active"), job.id)
        # Add to failed
        # R-004: Cap failed list to 100 items + 7-day TTL to prevent unbounded growth
        failed_key = self._key("failed")
        pipe.lpush(failed_key, job.to_json())
        pipe.ltrim(failed_key, 0, 99)
        pipe.expire(failed_key, 604800)  # 7 days
        # Update stats
        pipe.hincrby(self._key("stats"), "total_failed", 1)
        pipe.execute()

    def retry_failed(self) -> int:
        """Move all failed jobs back to pending. Returns count."""
        count = 0
        while True:
            job_json = self.redis.client.rpop(self._key("failed"))
            if not job_json:
                break
            job = Job.from_json(job_json)
            job.error = ""  # Clear error
            job.started_at = 0
            job.completed_at = 0
            self.redis.client.rpush(self._key("pending"), job.to_json())
            count += 1
        return count

    # =========================================================================
    # Query Operations
    # =========================================================================

    def status(self) -> dict:
        """Get queue status summary."""
        pipe = self.redis.client.pipeline()
        pipe.llen(self._key("pending"))
        pipe.hlen(self._key("active"))
        pipe.llen(self._key("complete"))
        pipe.llen(self._key("failed"))
        pipe.hgetall(self._key("stats"))
        pending, active, complete, failed, stats = pipe.execute()

        return {
            "pending": pending,
            "active": active,
            "complete": complete,
            "failed": failed,
            "stats": stats or {}
        }

    def pending_jobs(self, limit: int = 10) -> list[Job]:
        """Get pending jobs without removing them."""
        items = self.redis.client.lrange(self._key("pending"), 0, limit - 1)
        return [Job.from_json(item) for item in items]

    def active_jobs(self) -> list[Job]:
        """Get all active jobs."""
        items = self.redis.client.hvals(self._key("active"))
        return [Job.from_json(item) for item in items]

    def failed_jobs(self, limit: int = 10) -> list[Job]:
        """Get failed jobs."""
        items = self.redis.client.lrange(self._key("failed"), 0, limit - 1)
        return [Job.from_json(item) for item in items]

    def has_work(self) -> bool:
        """Check if there's pending work."""
        return self.redis.client.llen(self._key("pending")) > 0

    def is_processing(self) -> bool:
        """Check if a worker is currently processing."""
        return self.redis.client.hlen(self._key("active")) > 0

    # =========================================================================
    # Cleanup Operations
    # =========================================================================

    def clear_complete(self) -> int:
        """Clear completed jobs list. Returns count cleared."""
        count = self.redis.client.llen(self._key("complete"))
        self.redis.client.delete(self._key("complete"))
        return count

    def clear_all(self) -> dict:
        """Clear all queues. Returns counts."""
        pipe = self.redis.client.pipeline()
        pipe.llen(self._key("pending"))
        pipe.hlen(self._key("active"))
        pipe.llen(self._key("complete"))
        pipe.llen(self._key("failed"))
        pipe.delete(self._key("pending"))
        pipe.delete(self._key("active"))
        pipe.delete(self._key("complete"))
        pipe.delete(self._key("failed"))
        pipe.delete(self._key("stats"))
        results = pipe.execute()

        return {
            "pending_cleared": results[0],
            "active_cleared": results[1],
            "complete_cleared": results[2],
            "failed_cleared": results[3]
        }


# =============================================================================
# Job Factory Functions
# =============================================================================

def create_enrich_job(project_id: str, domain: str, description: str = "") -> Job:
    """Create a domain enrichment job."""
    return Job(
        id=domain,  # Domain name IS the job ID - enables idempotent push
        type=JobType.ENRICH,
        project_id=project_id,
        phase="intelligence",
        payload={
            "domain": domain,
            "description": description
        }
    )


def create_map_keywords_job(project_id: str, domain: str, scope: str = "all") -> Job:
    """Create a keyword mapping job."""
    return Job(
        id="",
        type=JobType.MAP_KEYWORDS,
        project_id=project_id,
        phase="intelligence",
        payload={
            "domain": domain,
            "scope": scope  # "all" or "recent"
        }
    )


def create_intent_job(project_id: str, files: list[str]) -> Job:
    """Create an intent analysis job."""
    return Job(
        id="",
        type=JobType.ANALYZE_INTENT,
        project_id=project_id,
        phase="intent",
        payload={
            "files": files
        }
    )


def create_discover_job(project_id: str, patterns: list[str]) -> Job:
    """Create a domain discovery job."""
    return Job(
        id="",
        type=JobType.DISCOVER_DOMAIN,
        project_id=project_id,
        phase="intent",
        payload={
            "patterns": patterns
        }
    )


def create_scrape_job(project_id: str, session_id: str, stop_count: int) -> Job:
    """Create a session scrape job (bigrams + file hits)."""
    return Job(
        id=f"scrape-{stop_count}",  # Idempotent: one scrape per stop_count
        type=JobType.SCRAPE,
        project_id=project_id,
        phase="intent",
        payload={
            "session_id": session_id,
            "stop_count": stop_count
        }
    )


def create_autotune_job(project_id: str, stop_count: int) -> Job:
    """Create an autotune job (decay, prune, promote)."""
    return Job(
        id=f"autotune-{stop_count}",  # Idempotent: one autotune per stop_count
        type=JobType.AUTOTUNE,
        project_id=project_id,
        phase="intent",
        payload={
            "stop_count": stop_count
        }
    )


# =============================================================================
# Test
# =============================================================================

if __name__ == "__main__":
    # Quick test
    q = JobQueue("test-project")

    # Clear any existing test data
    q.clear_all()

    # Push some jobs
    jobs = [
        create_enrich_job("test-project", "@search", "Code search functionality"),
        create_enrich_job("test-project", "@api", "REST API endpoints"),
        create_enrich_job("test-project", "@auth", "Authentication system"),
    ]

    print(f"Pushed {q.push_many(jobs)} jobs")
    print(f"Status: {q.status()}")

    # Process one
    job = q.pop()
    if job:
        print(f"Processing: {job.type.value} - {job.payload}")
        q.complete(job)

    print(f"Status after processing: {q.status()}")

    # Cleanup
    q.clear_all()
    print("Test complete")
