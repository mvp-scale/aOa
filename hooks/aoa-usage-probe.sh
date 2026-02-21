#!/bin/bash
# =============================================================================
# aOa Usage Probe — Hit Claude OAuth usage endpoint, save to .aoa/usage.json
# =============================================================================
#
# Run manually:   ./hooks/aoa-usage-probe.sh
# Or from hook:   called by aoa-status-line.sh when usage.json is stale
#
# Reads OAuth token from ~/.claude/.credentials.json
# Writes result to .aoa/usage.json
# =============================================================================

set -uo pipefail

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$(pwd)}"
USAGE_FILE="$PROJECT_DIR/.aoa/usage.json"
CREDS_FILE="$HOME/.claude/.credentials.json"

# Sanity checks
if [ ! -d "$PROJECT_DIR/.aoa" ]; then
    echo "ERROR: $PROJECT_DIR/.aoa does not exist" >&2
    exit 1
fi

if [ ! -f "$CREDS_FILE" ]; then
    echo "ERROR: $CREDS_FILE not found" >&2
    exit 1
fi

# Extract OAuth access token
TOKEN=$(jq -r '.claudeAiOauth.accessToken // empty' "$CREDS_FILE" 2>/dev/null)
if [ -z "$TOKEN" ]; then
    echo "ERROR: Could not read accessToken from $CREDS_FILE" >&2
    exit 1
fi

# Hit the usage endpoint
echo "Fetching usage from api.anthropic.com..."
HTTP_CODE=$(curl -s -o "$USAGE_FILE.tmp" -w "%{http_code}" \
    -H "Authorization: Bearer $TOKEN" \
    "https://api.anthropic.com/api/oauth/usage")

if [ "$HTTP_CODE" = "200" ]; then
    # Add our own metadata
    TS=$(date +%s)
    # Merge timestamp into the response
    jq --argjson ts "$TS" '. + {"captured_at": $ts}' "$USAGE_FILE.tmp" > "$USAGE_FILE.tmp2" 2>/dev/null
    if [ $? -eq 0 ]; then
        mv "$USAGE_FILE.tmp2" "$USAGE_FILE"
        rm -f "$USAGE_FILE.tmp"
    else
        # jq failed — raw response might not be JSON, save as-is
        mv "$USAGE_FILE.tmp" "$USAGE_FILE"
    fi
    echo "OK — saved to $USAGE_FILE"
    echo ""
    echo "Response:"
    jq . "$USAGE_FILE" 2>/dev/null || cat "$USAGE_FILE"
elif [ "$HTTP_CODE" = "401" ]; then
    echo "ERROR: 401 Unauthorized — token may be expired" >&2
    echo "Try running Claude Code to refresh the token, then retry" >&2
    rm -f "$USAGE_FILE.tmp"
    exit 1
else
    echo "ERROR: HTTP $HTTP_CODE" >&2
    echo "Response body:"
    cat "$USAGE_FILE.tmp" >&2
    rm -f "$USAGE_FILE.tmp"
    exit 1
fi
