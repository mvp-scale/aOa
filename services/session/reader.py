#!/usr/bin/env python3
"""
Session Reader - Extracts prompts and file access from Claude session files.

Claude stores sessions in ~/.claude/projects/{encoded-path}/*.jsonl
Each session file contains JSONL records with types:
  - "user": User messages with prompts
  - "assistant": Claude responses with tool uses
  - "file-history-snapshot": File backup records
  - "summary": Session summaries
"""

import json
import os
import re
from pathlib import Path
from typing import Optional


class SessionReader:
    """Reads Claude session files to extract prompts and file access."""

    CLAUDE_PROJECTS_DIR = Path.home() / ".claude" / "projects"

    def __init__(self, project_path: Optional[str] = None):
        """Initialize with project path. Auto-detects if not provided."""
        self.project_path = project_path or os.getcwd()
        self._session_dir = self._find_session_dir()

    def _encode_path(self, path: str) -> str:
        """Encode path the way Claude does: /home/corey/aOa -> -home-corey-aOa"""
        return path.replace("/", "-")

    def _find_session_dir(self) -> Optional[Path]:
        """Find the Claude session directory for this project."""
        encoded = self._encode_path(self.project_path)
        session_dir = self.CLAUDE_PROJECTS_DIR / encoded
        return session_dir if session_dir.exists() else None

    def _get_session_files(self, limit: int = 5) -> list[Path]:
        """Get most recent session files, sorted by modification time."""
        if not self._session_dir:
            return []

        sessions = []
        for f in self._session_dir.iterdir():
            if f.suffix == ".jsonl" and f.is_file():
                sessions.append((f.stat().st_mtime, f))

        sessions.sort(reverse=True)
        return [s[1] for s in sessions[:limit]]

    def _parse_session_file(self, path: Path) -> tuple[list[str], set[str]]:
        """Parse a session file, returning (prompts, files_touched)."""
        prompts = []
        files_touched = set()

        try:
            with open(path) as f:
                for line in f:
                    try:
                        data = json.loads(line)
                    except json.JSONDecodeError:
                        continue

                    msg_type = data.get("type")

                    # Extract user prompts (skip meta messages like /clear)
                    if msg_type == "user" and not data.get("isMeta"):
                        msg = data.get("message", {})
                        content = msg.get("content", "")
                        if isinstance(content, str) and len(content) > 10:
                            # Strip XML tags from content
                            clean = re.sub(r"<[^>]+>", "", content).strip()
                            if clean:
                                prompts.append(clean)

                    # Extract files from tool uses
                    if msg_type == "assistant":
                        msg = data.get("message", {})
                        content = msg.get("content", [])
                        if isinstance(content, list):
                            for item in content:
                                if isinstance(item, dict) and item.get("type") == "tool_use":
                                    tool = item.get("name", "")
                                    inp = item.get("input", {})
                                    # File operations
                                    if tool in ("Read", "Edit", "Write", "Glob", "Grep"):
                                        file_path = inp.get("file_path") or inp.get("path")
                                        if file_path:
                                            files_touched.add(file_path)
        except (IOError, OSError):
            pass

        return prompts, files_touched

    def get_recent_prompts(self, limit: int = 10) -> list[str]:
        """Extract last N user prompts from recent sessions."""
        prompts = []
        for session_file in self._get_session_files():
            session_prompts, _ = self._parse_session_file(session_file)
            prompts.extend(session_prompts)
            if len(prompts) >= limit:
                break
        return prompts[:limit]

    def get_files_touched(self, limit: int = 50) -> set[str]:
        """Extract files Read/Edited in recent sessions."""
        files = set()
        for session_file in self._get_session_files():
            _, session_files = self._parse_session_file(session_file)
            files.update(session_files)
            if len(files) >= limit:
                break
        return files

    def get_session_stats(self) -> dict:
        """Get statistics about the current project's sessions."""
        if not self._session_dir:
            return {"error": "No session directory found", "project": self.project_path}

        session_files = self._get_session_files(limit=100)
        total_prompts = 0
        all_files = set()

        for sf in session_files:
            prompts, files = self._parse_session_file(sf)
            total_prompts += len(prompts)
            all_files.update(files)

        return {
            "project": self.project_path,
            "session_dir": str(self._session_dir),
            "session_count": len(session_files),
            "total_prompts": total_prompts,
            "unique_files_touched": len(all_files),
        }


if __name__ == "__main__":
    # Quick test
    reader = SessionReader("/home/corey/aOa")
    print("Session stats:", reader.get_session_stats())
    print("\nRecent prompts:")
    for p in reader.get_recent_prompts(3):
        print(f"  - {p[:80]}...")
    print(f"\nFiles touched: {len(reader.get_files_touched())} unique files")
