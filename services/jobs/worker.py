#!/usr/bin/env python3
"""
GL-089: Job Worker for aOa

Processes jobs from the queue. Each job type has a handler.
Worker is stateless - all state lives in Redis queue.

Run modes:
1. Single job: process one job and exit (for hooks)
2. Batch: process N jobs and exit
3. Drain: process until queue empty
"""

import json
import os
import re
import sys
import time
import urllib.parse
import urllib.request
from typing import Callable, Optional

# Add paths for imports
_services_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, _services_dir)
sys.path.insert(0, '/app/services')

from jobs.queue import Job, JobQueue, JobType

# Domain learner imported lazily to avoid circular imports
DomainLearner = None
Domain = None

def _get_domain_learner():
    """Lazy import of DomainLearner to avoid circular import."""
    global DomainLearner, Domain
    if DomainLearner is None:
        from domains.learner import DomainLearner as DL, Domain as D
        DomainLearner = DL
        Domain = D
    return DomainLearner


class JobWorker:
    """
    Processes jobs from the queue.

    Each job type maps to a handler function.
    Handlers receive (job, context) and return True on success.
    """

    def __init__(self, project_id: str, redis_url: Optional[str] = None):
        self.project_id = project_id
        self.queue = JobQueue(project_id, redis_url)
        self.redis_url = redis_url
        self.handlers: dict[JobType, Callable] = {
            JobType.ENRICH: self._handle_enrich,
            JobType.MAP_KEYWORDS: self._handle_map_keywords,
            JobType.ANALYZE_INTENT: self._handle_analyze_intent,
            JobType.DISCOVER_DOMAIN: self._handle_discover_domain,
            JobType.CLEANUP: self._handle_cleanup,
            JobType.TUNE: self._handle_tune,
            JobType.REINDEX: self._handle_reindex,
            JobType.SCRAPE: self._handle_scrape,
            JobType.AUTOTUNE: self._handle_autotune,
        }

    def process_one(self, timeout: int = 0) -> Optional[Job]:
        """
        Process a single job.

        Args:
            timeout: Seconds to wait for job (0 = no wait)

        Returns:
            Processed job or None if queue empty
        """
        job = self.queue.pop(timeout=timeout)
        if not job:
            return None

        handler = self.handlers.get(job.type)
        if not handler:
            self.queue.fail(job, f"Unknown job type: {job.type}")
            return job

        try:
            handler(job)
            self.queue.complete(job)
        except Exception as e:
            self.queue.fail(job, str(e))

        return job

    def process_batch(self, count: int = 3) -> list[Job]:
        """Process up to N jobs. Returns list of processed jobs."""
        processed = []
        for _ in range(count):
            job = self.process_one(timeout=0)
            if not job:
                break
            processed.append(job)
        return processed

    def drain(self, max_jobs: int = 100) -> int:
        """Process all pending jobs. Returns count processed."""
        count = 0
        while count < max_jobs:
            job = self.process_one(timeout=0)
            if not job:
                break
            count += 1
        return count

    # =========================================================================
    # Job Handlers
    # =========================================================================

    def _handle_enrich(self, job: Job) -> None:
        """
        Build a domain from its @domain.json file.

        Expects the domain file to exist at .aoa/domains/@{name}.json
        with terms and keywords already generated (by parallel workers).

        Calls learner.enrich_domain() directly (no subprocess).
        """
        import json

        domain_name = job.payload.get("domain")
        if not domain_name:
            raise ValueError("Missing domain name in job payload")

        # Find project root (where .aoa/domains lives)
        project_root = os.environ.get("AOA_PROJECT_ROOT", "/codebase")
        domain_file = os.path.join(project_root, ".aoa", "domains", f"{domain_name}.json")

        # 1. Check if domain file exists
        if not os.path.exists(domain_file):
            print(f"[Worker] ENRICH: {domain_name} - waiting for domain file", flush=True)
            raise RuntimeError(f"Domain file not found: {domain_file}")

        # 2. Read and parse the domain file
        print(f"[Worker] ENRICH: {domain_name} - reading {domain_file}", flush=True)
        try:
            with open(domain_file, 'r') as f:
                data = json.load(f)
        except Exception as e:
            raise RuntimeError(f"Failed to read domain file: {e}")

        term_keywords = data.get("terms", {})
        if not term_keywords:
            raise RuntimeError(f"Domain file has no terms: {domain_file}")

        # 3. Call learner directly to enrich the domain
        print(f"[Worker] ENRICH: {domain_name} - enriching via learner", flush=True)
        DL = _get_domain_learner()
        learner = DL(self.project_id, self.redis_url)
        result = learner.enrich_domain(domain_name, term_keywords)
        keywords_added = result.get("keywords_added", 0)
        print(f"[Worker] ENRICH: {domain_name} - added {keywords_added} keywords", flush=True)

        # 4. Verify domain is enriched
        if not learner.is_domain_enriched(domain_name):
            raise RuntimeError(f"Enrichment failed: {domain_name} not marked as enriched")

        print(f"[Worker] ENRICH: {domain_name} - VERIFIED enriched ✓", flush=True)
        # Note: File cleanup happens in CLI (aoa domains link) after successful link
        # Worker runs in Docker and can't reliably delete host files

    def _handle_map_keywords(self, job: Job) -> None:
        """Map domain keywords to codebase files."""
        domain_name = job.payload.get("domain")
        scope = job.payload.get("scope", "all")

        print(f"[Worker] MAP_KEYWORDS: {domain_name} scope={scope}", flush=True)
        # Implementation would scan files and create keyword->file mappings

    def _handle_analyze_intent(self, job: Job) -> None:
        """Analyze file access patterns for intent phase."""
        files = job.payload.get("files", [])

        print(f"[Worker] ANALYZE_INTENT: {len(files)} files", flush=True)
        # Implementation would analyze file patterns to discover domains

    def _handle_discover_domain(self, job: Job) -> None:
        """Create new domain from discovered patterns."""
        patterns = job.payload.get("patterns", [])

        print(f"[Worker] DISCOVER_DOMAIN: patterns={patterns}", flush=True)
        # Implementation would create new domain skeleton

    def _handle_cleanup(self, job: Job) -> None:
        """Clean up stale or deprecated domains."""
        domain_name = job.payload.get("domain")
        action = job.payload.get("action", "deprecate")

        print(f"[Worker] CLEANUP: {domain_name} action={action}", flush=True)
        # Implementation would mark domain as deprecated or remove

    def _handle_tune(self, job: Job) -> None:
        """Run domain tuning/rebalancing."""
        trigger = job.payload.get("trigger", "manual")

        print(f"[Worker] TUNE: trigger={trigger}", flush=True)
        # Implementation would rebalance domain terms

    def _handle_reindex(self, job: Job) -> None:
        """Rebuild keyword mappings."""
        domain_name = job.payload.get("domain")

        print(f"[Worker] REINDEX: {domain_name or 'all'}", flush=True)
        # Implementation would rebuild keyword->file mappings

    def _handle_scrape(self, job: Job) -> None:
        """
        SH-03: Session scrape - extract bigrams + track file hits.

        Runs every 5 stops. Extracts:
        1. Bigrams from user prompts (word pairs → Redis HINCRBY)
        2. File hits from intent records (file:range → keyword lookups)
        """
        import re
        import urllib.request
        import urllib.error

        session_id = job.payload.get("session_id", "")
        stop_count = job.payload.get("stop_count", 0)

        print(f"[Worker] SCRAPE: session={session_id[:8]}... stop={stop_count}", flush=True)

        # Get Redis client
        from ranking.redis_client import RedisClient
        redis = RedisClient(self.redis_url)
        r = redis.client

        # SH-07: Extract bigrams from conversation via /cc/conversation API
        # Uses cursor to only get NEW content since last scrape
        try:
            aoa_url = os.environ.get("AOA_URL", "http://localhost:8080")

            # Get cursor: last scrape timestamp
            cursor_key = f"aoa:{self.project_id}:scrape_cursor"
            since = r.get(cursor_key) or ""

            # Fetch conversation since last scrape
            url = f"{aoa_url}/cc/conversation?limit=100"
            if since:
                url += f"&since={urllib.parse.quote(since)}"
            req = urllib.request.Request(url)
            with urllib.request.urlopen(req, timeout=10) as resp:
                data = json.loads(resp.read().decode())

            texts = data.get("texts", [])
            latest_ts = data.get("latest_timestamp", "")

            bigram_count = 0
            for item in texts:
                text = item.get("text", "")
                if not isinstance(text, str) or len(text) < 10:
                    continue
                # Tokenize: lowercase, split on non-word chars
                words = re.findall(r'\b[a-z][a-z0-9_]+\b', text.lower())
                # Extract bigrams (consecutive word pairs)
                for i in range(len(words) - 1):
                    bigram = f"{words[i]}:{words[i+1]}"
                    r.hincrby(f"aoa:{self.project_id}:bigrams", bigram, 1)
                    bigram_count += 1

            # Update cursor for next scrape
            if latest_ts:
                r.set(cursor_key, latest_ts)

            print(f"[Worker] SCRAPE: extracted {bigram_count} bigrams from {len(texts)} texts (since={since[:20] if since else 'start'})", flush=True)

        except Exception as e:
            print(f"[Worker] SCRAPE: bigram extraction failed: {e}", flush=True)

        # SH-09: Track file hits from intent records
        try:
            aoa_url = os.environ.get("AOA_URL", "http://localhost:8080")
            url = f"{aoa_url}/intent/recent?limit=20&project_id={self.project_id}"
            req = urllib.request.Request(url)
            with urllib.request.urlopen(req, timeout=5) as resp:
                data = json.loads(resp.read().decode())

            hit_count = 0
            for record in data.get("records", []):
                files = record.get("files", [])
                for file_entry in files:
                    # Skip command entries
                    if file_entry.startswith("cmd:") or file_entry.startswith("pattern:"):
                        continue
                    # Extract file path (remove :line-range if present)
                    file_path = file_entry.split(":")[0] if ":" in file_entry else file_entry
                    if file_path:
                        r.hincrby(f"aoa:{self.project_id}:file_hits", file_path, 1)
                        hit_count += 1

            print(f"[Worker] SCRAPE: tracked {hit_count} file hits", flush=True)

        except Exception as e:
            print(f"[Worker] SCRAPE: file hit tracking failed: {e}", flush=True)

    def _handle_autotune(self, job: Job) -> None:
        """
        SH-12: Autotune - decay, prune, promote.

        Runs every 100 stops. Operations:
        1. Decay: Reduce hit counts by factor (recency weighting)
        2. Prune: Remove keywords below threshold
        3. Promote: Move high-hit context keywords to core
        """
        stop_count = job.payload.get("stop_count", 0)

        print(f"[Worker] AUTOTUNE: stop={stop_count}", flush=True)

        # Get Redis client
        from ranking.redis_client import RedisClient
        redis = RedisClient(self.redis_url)
        r = redis.client

        # Decay factor (0.9 = lose 10% per autotune cycle)
        DECAY_FACTOR = 0.9

        # Decay bigram counts
        bigrams = r.hgetall(f"aoa:{self.project_id}:bigrams")
        decayed = 0
        pruned = 0
        for bigram, count in bigrams.items():
            new_count = int(float(count) * DECAY_FACTOR)
            if new_count <= 0:
                r.hdel(f"aoa:{self.project_id}:bigrams", bigram)
                pruned += 1
            else:
                r.hset(f"aoa:{self.project_id}:bigrams", bigram, new_count)
                decayed += 1

        # Decay file hits
        file_hits = r.hgetall(f"aoa:{self.project_id}:file_hits")
        file_decayed = 0
        file_pruned = 0
        for file_path, count in file_hits.items():
            new_count = int(float(count) * DECAY_FACTOR)
            if new_count <= 0:
                r.hdel(f"aoa:{self.project_id}:file_hits", file_path)
                file_pruned += 1
            else:
                r.hset(f"aoa:{self.project_id}:file_hits", file_path, new_count)
                file_decayed += 1

        print(f"[Worker] AUTOTUNE: bigrams decayed={decayed} pruned={pruned}, files decayed={file_decayed} pruned={file_pruned}", flush=True)


def get_queue_status(project_id: str, redis_url: Optional[str] = None) -> dict:
    """Get queue status for a project."""
    q = JobQueue(project_id, redis_url)
    return q.status()


def push_enrich_jobs(project_id: str, domains: list[dict], redis_url: Optional[str] = None) -> int:
    """
    Push enrichment jobs for domains.

    Args:
        project_id: Project identifier
        domains: List of {"name": "@x", "description": "..."}

    Returns:
        Number of jobs pushed
    """
    from jobs.queue import create_enrich_job

    q = JobQueue(project_id, redis_url)
    jobs = [
        create_enrich_job(project_id, d["name"], d.get("description", ""))
        for d in domains
    ]
    return q.push_many(jobs)


# =============================================================================
# CLI Entry Point
# =============================================================================

if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="aOa Job Worker")
    parser.add_argument("project_id", help="Project ID to process")
    parser.add_argument("--mode", choices=["one", "batch", "drain"], default="one",
                        help="Processing mode")
    parser.add_argument("--count", type=int, default=3,
                        help="Jobs to process in batch mode")
    parser.add_argument("--redis", help="Redis URL")

    args = parser.parse_args()

    worker = JobWorker(args.project_id, args.redis)

    if args.mode == "one":
        job = worker.process_one()
        if job:
            print(f"Processed: {job.type.value} - {job.id}")
        else:
            print("No jobs pending")

    elif args.mode == "batch":
        jobs = worker.process_batch(args.count)
        print(f"Processed {len(jobs)} jobs")

    elif args.mode == "drain":
        count = worker.drain()
        print(f"Drained {count} jobs")

    # Print final status
    status = worker.queue.status()
    print(f"Queue: {status['pending']} pending, {status['active']} active, "
          f"{status['complete']} complete, {status['failed']} failed")
