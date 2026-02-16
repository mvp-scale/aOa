#!/bin/bash
# aOa status line hook — reads pre-computed status from project-local file.
# No computation at hook time. The daemon writes this file on every state change.
cat "$CLAUDE_PROJECT_DIR/.aoa/status-line.txt" 2>/dev/null
