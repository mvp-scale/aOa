#!/usr/bin/env python3
"""
Session Metrics - Extract and store Claude Code session metrics.

Defensive design:
- All parsing wrapped in try/catch - Claude's format may change
- Errors are logged but never thrown to caller
- Missing/malformed data returns safe defaults
- Never blocks user workflow

Redis Schema:
  cc:prompts:{project_id}        -> List of recent prompts (LPUSH, LTRIM)
  cc:session:{session_id}        -> Hash with session metrics
  cc:sessions:{project_id}       -> Sorted set of session_ids by timestamp
  cc:stats:{project_id}:daily    -> Hash with daily aggregates
"""

import json
import logging
import os
import re
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional

logger = logging.getLogger(__name__)


def safe_get(data: dict, *keys, default=None) -> Any:
    """Safely traverse nested dict. Returns default if any key missing."""
    try:
        result = data
        for key in keys:
            if isinstance(result, dict):
                result = result.get(key, default)
            else:
                return default
        return result if result is not None else default
    except Exception:
        return default


def parse_model_string(model: str) -> dict:
    """Parse model string like 'claude-opus-4-5-20251101' into components.

    Returns: {name: 'Opus', version: '4.5', build: '20251101', raw: original}
    Falls back to raw string if parse fails.
    """
    try:
        if not model or model == "unknown":
            return {"name": "unknown", "version": "", "build": "", "raw": model or "unknown"}

        # Handle synthetic/internal models
        if model.startswith("<"):
            return {"name": "synthetic", "version": "", "build": "", "raw": model}

        # Parse: claude-{name}-{major}-{minor}-{build}
        # Example: claude-opus-4-5-20251101
        parts = model.split("-")
        if len(parts) >= 5 and parts[0] == "claude":
            name = parts[1].capitalize()  # opus -> Opus
            version = f"{parts[2]}.{parts[3]}"  # 4-5 -> 4.5
            build = parts[4] if len(parts) > 4 else ""
            return {"name": name, "version": version, "build": build, "raw": model}

        # Fallback: just use the string
        return {"name": model, "version": "", "build": "", "raw": model}
    except Exception:
        return {"name": str(model), "version": "", "build": "", "raw": str(model)}


def extract_tool_counts(content: list) -> dict:
    """Extract tool usage counts from assistant message content.

    Returns: {Bash: 5, Read: 3, Edit: 2, ...}
    """
    counts = {}
    try:
        if not isinstance(content, list):
            return counts

        for item in content:
            if isinstance(item, dict) and item.get("type") == "tool_use":
                tool = item.get("name", "unknown")
                counts[tool] = counts.get(tool, 0) + 1
    except Exception as e:
        logger.debug(f"Error extracting tool counts: {e}")

    return counts


def categorize_tools(tool_counts: dict) -> dict:
    """Categorize tools into our display groups.

    Returns: {B: bash_count, R: read_count, W: write_count, T: task_count, M: mcp_count, Web: web_count}
    """
    categories = {"B": 0, "R": 0, "W": 0, "T": 0, "M": 0, "Web": 0}

    try:
        for tool, count in tool_counts.items():
            if tool == "Bash":
                categories["B"] += count
            elif tool in ("Read", "Grep", "Glob"):
                categories["R"] += count
            elif tool in ("Edit", "Write"):
                categories["W"] += count
            elif tool in ("Task", "TaskOutput", "TaskCreate", "TaskUpdate", "TaskList"):
                categories["T"] += count
            elif tool.startswith("mcp__"):
                categories["M"] += count
            elif tool in ("WebSearch", "WebFetch"):
                categories["Web"] += count
    except Exception as e:
        logger.debug(f"Error categorizing tools: {e}")

    return categories


def clean_user_prompt(content: str) -> Optional[str]:
    """Extract clean user prompt from message content.

    Removes system-generated blocks, XML tags, and normalizes whitespace.
    Returns None if result is too short or empty.
    """
    try:
        if not isinstance(content, str) or len(content) < 5:
            return None

        clean = content

        # Remove system-generated blocks (content AND tags)
        patterns = [
            r"<system-reminder>.*?</system-reminder>",
            r"<local-command-[^>]*>.*?</local-command-[^>]*>",
            r"<command-[^>]+>.*?</command-[^>]+>",
            r"<user-prompt-submit-hook>.*?</user-prompt-submit-hook>",
            r"<hook-[^>]+>.*?</hook-[^>]+>",
        ]

        for pattern in patterns:
            clean = re.sub(pattern, "", clean, flags=re.DOTALL)

        # Remove any remaining XML-style tags
        clean = re.sub(r"<[^>]+>", "", clean)

        # Collapse whitespace and trim
        clean = re.sub(r"\s+", " ", clean).strip()

        # Return None if too short
        if len(clean) < 3:
            return None

        return clean
    except Exception as e:
        logger.debug(f"Error cleaning prompt: {e}")
        return None


class SessionMetrics:
    """Extract metrics from Claude Code session files with defensive error handling."""

    def __init__(self, project_path: Optional[str] = None):
        """Initialize with project path. Auto-detects if not provided."""
        self.project_path = project_path or os.getcwd()
        self.project_id = self._encode_path(self.project_path)

        # Session directory
        claude_base = os.environ.get('CLAUDE_SESSIONS', str(Path.home() / ".claude"))
        self.sessions_dir = Path(claude_base) / "projects" / self.project_id

    def _encode_path(self, path: str) -> str:
        """Encode path: /home/corey/aOa -> -home-corey-aOa"""
        return path.replace("/", "-")

    def _get_session_files(self, limit: int = 20) -> list[Path]:
        """Get most recent session files, sorted by modification time."""
        try:
            if not self.sessions_dir.exists():
                return []

            sessions = []
            for f in self.sessions_dir.iterdir():
                if f.suffix == ".jsonl" and f.is_file():
                    try:
                        sessions.append((f.stat().st_mtime, f))
                    except OSError:
                        continue

            sessions.sort(reverse=True)
            return [s[1] for s in sessions[:limit]]
        except Exception as e:
            logger.debug(f"Error listing sessions: {e}")
            return []

    def parse_session(self, session_path: Path) -> dict:
        """Parse a single session file and extract all metrics.

        Returns comprehensive session data or empty dict on error.
        """
        result = {
            "session_id": session_path.stem,
            "file_path": str(session_path),
            "prompts": [],
            "turns": [],
            "tool_counts": {},
            "model_counts": {},
            "start_time": None,
            "end_time": None,
            "total_input": 0,
            "total_output": 0,
            "total_cache_read": 0,
            "total_cache_write": 0,
            "total_duration_ms": 0,
            "error": None,
        }

        turn_durations = {}  # uuid -> duration_ms
        all_lines = []  # Store all lines for two-pass parsing

        try:
            # First pass: collect all lines and turn durations
            with open(session_path) as f:
                for line in f:
                    all_lines.append(line)
                    try:
                        data = json.loads(line)
                        # Collect turn durations first
                        if data.get("type") == "system" and data.get("subtype") == "turn_duration":
                            parent = data.get("parentUuid")
                            duration = data.get("durationMs", 0)
                            if parent and duration:
                                turn_durations[parent] = duration
                                result["total_duration_ms"] += duration
                    except json.JSONDecodeError:
                        continue

            # Second pass: process all messages with durations available
            for line in all_lines:
                try:
                    data = json.loads(line)
                except json.JSONDecodeError:
                    continue

                msg_type = data.get("type")
                timestamp = data.get("timestamp")

                # Track time bounds
                if timestamp:
                    if result["start_time"] is None or timestamp < result["start_time"]:
                        result["start_time"] = timestamp
                    if result["end_time"] is None or timestamp > result["end_time"]:
                        result["end_time"] = timestamp

                # User prompts
                if msg_type == "user" and not data.get("isMeta"):
                    content = safe_get(data, "message", "content", default="")
                    clean = clean_user_prompt(content)
                    if clean:
                        result["prompts"].append({
                            "text": clean,
                            "timestamp": timestamp,
                        })

                # Assistant turns with usage
                if msg_type == "assistant":
                        msg = safe_get(data, "message", default={})
                        usage = safe_get(msg, "usage", default={})
                        content = safe_get(msg, "content", default=[])
                        model = safe_get(msg, "model", default="unknown")
                        uuid = data.get("uuid", "")

                        # Token counts
                        input_tokens = safe_get(usage, "input_tokens", default=0)
                        output_tokens = safe_get(usage, "output_tokens", default=0)
                        cache_read = safe_get(usage, "cache_read_input_tokens", default=0)
                        cache_write = safe_get(usage, "cache_creation_input_tokens", default=0)

                        result["total_input"] += input_tokens
                        result["total_output"] += output_tokens
                        result["total_cache_read"] += cache_read
                        result["total_cache_write"] += cache_write

                        # Model counts
                        if model and model != "unknown":
                            result["model_counts"][model] = result["model_counts"].get(model, 0) + 1

                        # Tool counts
                        turn_tools = extract_tool_counts(content)
                        for tool, count in turn_tools.items():
                            result["tool_counts"][tool] = result["tool_counts"].get(tool, 0) + count

                        # Only include completed turns (those with turn_duration events)
                        duration_ms = turn_durations.get(uuid, 0)
                        if duration_ms > 0:
                            # Build turn record
                            turn = {
                                "timestamp": timestamp,
                                "uuid": uuid,
                                "model": model,
                                "input_tokens": input_tokens,
                                "output_tokens": output_tokens,
                                "cache_read": cache_read,
                                "cache_write": cache_write,
                                "tools": turn_tools,
                                "duration_ms": duration_ms,
                            }

                            # Calculate throughput (tokens per second)
                            duration_sec = duration_ms / 1000
                            turn["gen_tps"] = round(output_tokens / duration_sec, 1)
                            turn["read_tps"] = round(input_tokens / duration_sec, 1)
                            turn["cache_tps"] = round(cache_read / duration_sec, 1)

                            result["turns"].append(turn)

        except Exception as e:
            result["error"] = str(e)
            logger.warning(f"Error parsing session {session_path}: {e}")

        return result

    def get_sessions_summary(self, limit: int = 10) -> list[dict]:
        """Get summary of recent sessions for 'aoa cc sessions' view.

        Returns list of session summaries with metrics.
        """
        sessions = []

        try:
            for session_path in self._get_session_files(limit=limit):
                session = self.parse_session(session_path)

                if session.get("error"):
                    continue

                # Calculate derived metrics
                total_context = (
                    session["total_input"] +
                    session["total_cache_read"] +
                    session["total_cache_write"]
                )
                cache_hit = round(
                    session["total_cache_read"] / total_context * 100, 1
                ) if total_context > 0 else 0

                # Calculate session elapsed time from timestamps
                elapsed_seconds = 0
                if session["start_time"] and session["end_time"]:
                    try:
                        start_dt = datetime.fromisoformat(session["start_time"].replace("Z", "+00:00"))
                        end_dt = datetime.fromisoformat(session["end_time"].replace("Z", "+00:00"))
                        elapsed_seconds = (end_dt - start_dt).total_seconds()
                    except Exception:
                        pass

                # Duration in minutes
                duration_min = round(elapsed_seconds / 60, 1) if elapsed_seconds > 0 else 0

                # Velocity: total tokens / elapsed time
                total_tokens = session["total_input"] + session["total_cache_read"] + session["total_output"]
                if elapsed_seconds > 0:
                    avg_velocity = round(total_tokens / elapsed_seconds, 1)
                else:
                    avg_velocity = 0

                # Model counts (O, S, H)
                model_summary = {"O": 0, "S": 0, "H": 0}
                for model, count in session["model_counts"].items():
                    parsed = parse_model_string(model)
                    name = parsed["name"].lower()
                    if "opus" in name:
                        model_summary["O"] += count
                    elif "sonnet" in name:
                        model_summary["S"] += count
                    elif "haiku" in name:
                        model_summary["H"] += count

                # Tool categories
                tool_cats = categorize_tools(session["tool_counts"])

                # Parse start time for date display
                start_date = ""
                start_time = ""
                if session["start_time"]:
                    try:
                        dt = datetime.fromisoformat(session["start_time"].replace("Z", "+00:00"))
                        start_date = dt.strftime("%b %d")
                        start_time = dt.strftime("%H:%M")
                    except Exception:
                        pass

                sessions.append({
                    "session_id": session["session_id"],
                    "date": start_date,
                    "start_time": start_time,
                    "duration_min": duration_min,
                    "input_tokens": session["total_input"],
                    "output_tokens": session["total_output"],
                    "cache_hit": cache_hit,
                    "velocity": avg_velocity,
                    "models": model_summary,
                    "tools": tool_cats,
                    "prompt_count": len(session["prompts"]),
                    "turn_count": len(session["turns"]),
                })

        except Exception as e:
            logger.warning(f"Error getting sessions summary: {e}")

        return sessions

    def get_prompts(self, limit: int = 25) -> list[dict]:
        """Get recent user prompts for 'aoa cc prompts' view."""
        prompts = []

        try:
            for session_path in self._get_session_files(limit=5):
                session = self.parse_session(session_path)
                prompts.extend(session.get("prompts", []))

                if len(prompts) >= limit:
                    break

            # Sort by timestamp descending
            prompts.sort(key=lambda x: x.get("timestamp", ""), reverse=True)
            return prompts[:limit]

        except Exception as e:
            logger.warning(f"Error getting prompts: {e}")
            return []

    def get_turns(self, session_id: Optional[str] = None, limit: int = 100) -> dict:
        """Get per-turn metrics for 'aoa cc turns' view.

        Args:
            session_id: Specific session to get turns from. If None, uses most recent.
            limit: Maximum turns to return.

        Returns:
            Dict with session info and turns array with throughput metrics.
        """
        result = {
            "session_id": None,
            "session_date": None,
            "session_time": None,
            "turn_count": 0,
            "turns": [],
        }

        try:
            # Find the session
            session_path = None
            if session_id:
                # Look for specific session
                for f in self._get_session_files(limit=100):
                    if f.stem == session_id:
                        session_path = f
                        break
            else:
                # Use most recent
                files = self._get_session_files(limit=1)
                if files:
                    session_path = files[0]

            if not session_path:
                return result

            # Parse the session
            session = self.parse_session(session_path)
            result["session_id"] = session["session_id"]

            # Parse start time for display
            if session["start_time"]:
                try:
                    dt = datetime.fromisoformat(session["start_time"].replace("Z", "+00:00"))
                    result["session_date"] = dt.strftime("%b %d")
                    result["session_time"] = dt.strftime("%H:%M")
                except Exception:
                    pass

            # Process turns with throughput calculations
            turns = session.get("turns", [])
            result["turn_count"] = len(turns)

            # Most recent first - take last N, then reverse
            for turn in reversed(turns[-limit:]):
                duration_sec = turn.get("duration_ms", 0) / 1000 if turn.get("duration_ms", 0) > 0 else 0

                # Calculate throughput rates
                gen_rate = round(turn["output_tokens"] / duration_sec, 1) if duration_sec > 0 else 0
                read_rate = round(turn["input_tokens"] / duration_sec, 1) if duration_sec > 0 else 0
                cache_rate = round(turn["cache_read"] / duration_sec, 1) if duration_sec > 0 else 0

                # Format timestamp for display (just time portion)
                turn_time = ""
                if turn.get("timestamp"):
                    try:
                        dt = datetime.fromisoformat(turn["timestamp"].replace("Z", "+00:00"))
                        turn_time = dt.strftime("%H:%M:%S")
                    except Exception:
                        pass

                result["turns"].append({
                    "time": turn_time,
                    "model": turn.get("model", "unknown"),
                    "input_tokens": turn.get("input_tokens", 0),
                    "output_tokens": turn.get("output_tokens", 0),
                    "cache_read": turn.get("cache_read", 0),
                    "cache_write": turn.get("cache_write", 0),
                    "duration_sec": round(duration_sec, 1),
                    "gen_rate": gen_rate,      # OUT / DUR
                    "read_rate": read_rate,    # IN / DUR
                    "cache_rate": cache_rate,  # C_RD / DUR
                })

        except Exception as e:
            logger.warning(f"Error getting turns: {e}")

        return result

    def get_stats(self, days: int = 30) -> dict:
        """Get aggregated stats for 'aoa cc stats' view.

        Returns stats broken down by today, 7 days, 30 days.
        """
        stats = {
            "periods": {
                "today": self._empty_period_stats(),
                "7d": self._empty_period_stats(),
                "30d": self._empty_period_stats(),
            },
            "model_distribution": {},
            "has_data": {"today": False, "7d": False, "30d": False},
        }

        try:
            now = datetime.now(timezone.utc)

            for session_path in self._get_session_files(limit=100):
                session = self.parse_session(session_path)

                if not session["start_time"]:
                    continue

                try:
                    session_dt = datetime.fromisoformat(
                        session["start_time"].replace("Z", "+00:00")
                    )
                    days_ago = (now - session_dt).days
                except Exception:
                    continue

                # Determine which periods this session belongs to
                periods = []
                if days_ago == 0:
                    periods.append("today")
                if days_ago < 7:
                    periods.append("7d")
                if days_ago < 30:
                    periods.append("30d")

                # Aggregate into each applicable period
                for period in periods:
                    stats["has_data"][period] = True
                    p = stats["periods"][period]

                    p["sessions"] += 1
                    p["turns"] += len(session["turns"])
                    p["input_tokens"] += session["total_input"]
                    p["output_tokens"] += session["total_output"]
                    p["cache_read"] += session["total_cache_read"]
                    p["cache_write"] += session["total_cache_write"]
                    p["duration_ms"] += session["total_duration_ms"]

                    # Model counts
                    for model, count in session["model_counts"].items():
                        p["model_counts"][model] = p["model_counts"].get(model, 0) + count

                    # Velocities for averaging
                    for turn in session["turns"]:
                        if turn["velocity"] > 0:
                            p["velocities"].append(turn["velocity"])

            # Calculate derived stats for each period
            for period, p in stats["periods"].items():
                if not stats["has_data"][period]:
                    continue

                # Cache hit rate
                total_context = p["input_tokens"] + p["cache_read"] + p["cache_write"]
                p["cache_hit"] = round(
                    p["cache_read"] / total_context * 100, 1
                ) if total_context > 0 else 0

                # Average velocity
                p["velocity"] = round(
                    sum(p["velocities"]) / len(p["velocities"]), 1
                ) if p["velocities"] else 0

                # Clean up
                del p["velocities"]

                # Build model distribution
                for model, count in p["model_counts"].items():
                    if model not in stats["model_distribution"]:
                        stats["model_distribution"][model] = {
                            "today": 0, "7d": 0, "30d": 0
                        }
                    stats["model_distribution"][model][period] = count

        except Exception as e:
            logger.warning(f"Error getting stats: {e}")

        return stats

    def _empty_period_stats(self) -> dict:
        """Return empty stats structure for a time period."""
        return {
            "sessions": 0,
            "turns": 0,
            "input_tokens": 0,
            "output_tokens": 0,
            "cache_read": 0,
            "cache_write": 0,
            "cache_hit": 0,
            "velocity": 0,
            "duration_ms": 0,
            "model_counts": {},
            "velocities": [],  # Temp for calculating average
        }


if __name__ == "__main__":
    # Quick test
    logging.basicConfig(level=logging.DEBUG)

    metrics = SessionMetrics("/home/corey/aOa")

    print("=== Sessions Summary ===")
    for s in metrics.get_sessions_summary(limit=3):
        print(f"  {s['date']} {s['start_time']} | {s['duration_min']}m | "
              f"O:{s['models']['O']} S:{s['models']['S']} H:{s['models']['H']} | "
              f"{s['velocity']} t/s | {s['cache_hit']}% cache")

    print("\n=== Recent Prompts ===")
    for p in metrics.get_prompts(limit=3):
        text = p["text"][:60] + "..." if len(p["text"]) > 60 else p["text"]
        print(f"  {text}")

    print("\n=== Stats ===")
    stats = metrics.get_stats()
    for period in ["today", "7d", "30d"]:
        if stats["has_data"][period]:
            p = stats["periods"][period]
            print(f"  {period}: {p['turns']} turns | {p['velocity']} t/s | {p['cache_hit']}% cache")
