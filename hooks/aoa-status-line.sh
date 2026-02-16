#!/bin/bash
# aOa-go status line hook — reads pre-computed status from file.
# No computation at hook time. The daemon writes this file on every state change.
cat /tmp/aoa-status-line.txt 2>/dev/null
