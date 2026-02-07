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
        self.project_root = self._resolve_project_root()
        self.handlers: dict[JobType, Callable] = {
            JobType.ENRICH: self._handle_enrich,
            JobType.SCRAPE: self._handle_scrape,
        }

    def _resolve_project_root(self) -> str:
        """Resolve the container path for this project from Redis."""
        try:
            import redis
            r = redis.from_url(self.redis_url or os.environ.get('REDIS_URL', 'redis://localhost:6379/0'))
            container_path = r.get(f"aoa:{self.project_id}:container_path")
            if container_path:
                return container_path
        except Exception as e:
            print(f"[Worker] Could not resolve project root from Redis: {e}", flush=True)
        return '/userhome'

    def process_one(self, timeout: int = 0, job_type: Optional[str] = None) -> Optional[Job]:
        """
        Process a single job.

        Args:
            timeout: Seconds to wait for job (0 = no wait)
            job_type: If set, only process jobs of this type (e.g., "scrape")

        Returns:
            Processed job or None if queue empty
        """
        job = self.queue.pop(timeout=timeout, job_type=job_type)
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

    def process_batch(self, count: int = 3, job_type: Optional[str] = None) -> list[Job]:
        """Process up to N jobs. Returns list of processed jobs."""
        processed = []
        for _ in range(count):
            job = self.process_one(timeout=0, job_type=job_type)
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
        domain_file = os.path.join(self.project_root, ".aoa", "domains", f"{domain_name}.json")

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

    def _handle_scrape(self, job: Job) -> None:
        """
        SH-03: Session scrape - extract bigrams + track file hits.

        Runs every stop. Extracts:
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
            current_bigrams = {}
            for item in texts:
                text = item.get("text", "")
                if not isinstance(text, str) or len(text) < 10:
                    continue
                # Tokenize: lowercase, split on non-word chars
                words = re.findall(r'\b[a-z][a-z0-9_]+\b', text.lower())
                # Extract bigrams (consecutive word pairs) -- accumulate locally
                for i in range(len(words) - 1):
                    bigram = f"{words[i]}:{words[i+1]}"
                    current_bigrams[bigram] = current_bigrams.get(bigram, 0) + 1
                    bigram_count += 1

            # Batch write all bigrams + recent in one pipeline
            if current_bigrams:
                bigrams_key = f"aoa:{self.project_id}:bigrams"
                recent_key = f"aoa:{self.project_id}:recent_bigrams"
                pipe = r.pipeline()
                for bigram, count in current_bigrams.items():
                    pipe.hincrby(bigrams_key, bigram, count)
                    pipe.hincrby(recent_key, bigram, count)
                pipe.execute()

            # S78-W3: Conversation path -- keyword_hits only via observe()
            # Bigram words with count >= 6 that are known keywords get incremented.
            # No term/domain/cohit -- conversation feeds domain *creation* (rebalance),
            # not domain *hits*. This is the keyword-only signal path.
            if current_bigrams:
                try:
                    _get_domain_learner()
                    learner = DomainLearner(self.project_id)
                    blocklist_key = f"aoa:{self.project_id}:keyword_blocklist"
                    keyword_hits_key = f"aoa:{self.project_id}:keyword_hits"

                    # Collect candidate words from qualifying bigrams
                    candidate_words = set()
                    for bigram, count in current_bigrams.items():
                        if count >= 6:
                            parts = bigram.split(":")
                            for word in parts:
                                if len(word) >= 2:
                                    candidate_words.add(word)

                    # Filter: not blocklisted + is a known keyword
                    keywords_to_observe = []
                    for word in candidate_words:
                        if r.sismember(blocklist_key, word):
                            continue
                        if r.hexists(keyword_hits_key, word):
                            keywords_to_observe.append(word)

                    if keywords_to_observe:
                        learner.observe(keywords=keywords_to_observe)
                        print(f"[Worker] W3: observe({len(keywords_to_observe)} keywords from conversation)", flush=True)
                except Exception as e:
                    print(f"[Worker] W3: keyword observe failed: {e}", flush=True)

            # Update cursor for next scrape
            if latest_ts:
                r.set(cursor_key, latest_ts)

            print(f"[Worker] SCRAPE: extracted {bigram_count} bigrams from {len(texts)} texts (since={since[:20] if since else 'start'})", flush=True)

        except Exception as e:
            print(f"[Worker] SCRAPE: bigram extraction failed: {e}", flush=True)
            raise  # Let process_one() mark job as failed

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
