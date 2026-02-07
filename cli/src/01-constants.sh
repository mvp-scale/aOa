# =============================================================================
# SECTION 01: Constants & Configuration
# =============================================================================
#
# PURPOSE
#   Global constants used throughout the CLI. ANSI color codes, service URLs,
#   and path configuration. These are set once at startup.
#
# DEPENDENCIES
#   - 00-header.sh: set -e must be active
#
# PROVIDES
#   AOA_URL                      Main API endpoint (external access)
#   AOA_DOCKER_HOST/PORT         Docker networking (install.sh only)
#   AOA_HOME, AOA_DATA           Installation paths
#   BOLD, DIM, GREEN, etc.       ANSI color codes for output formatting
#
# =============================================================================

# Find AOA_HOME by locating the CLI script itself
CLI_PATH="$(readlink -f "$0")"
AOA_HOME="$(dirname "$(dirname "$CLI_PATH")")"
AOA_DATA="${AOA_DATA:-$AOA_HOME/data}"

# AOA_URL: Main API endpoint - single source of truth
# Priority: 1) Environment variable, 2) Current project's home.json, 3) AOA_HOME's home.json, 4) Default
if [ -z "$AOA_URL" ]; then
    # Try current project's .aoa/home.json first
    _project_root=$(git rev-parse --show-toplevel 2>/dev/null)
    if [ -n "$_project_root" ] && [ -f "$_project_root/.aoa/home.json" ]; then
        AOA_URL=$(jq -r '.aoa_url // empty' "$_project_root/.aoa/home.json" 2>/dev/null)
    fi
    # Fall back to AOA_HOME's home.json
    if [ -z "$AOA_URL" ] && [ -f "$AOA_HOME/.aoa/home.json" ]; then
        AOA_URL=$(jq -r '.aoa_url // empty' "$AOA_HOME/.aoa/home.json" 2>/dev/null)
    fi
    # Fall back to default if still not set
    AOA_URL="${AOA_URL:-http://localhost:8080}"
    unset _project_root
fi
export AOA_URL

# Legacy aliases for backwards compatibility (deprecated - use AOA_URL)
INDEX_URL="${AOA_URL}"
STATUS_URL="${AOA_URL}"

# ANSI Colors
BOLD='\033[1m'
DIM='\033[2m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
RED='\033[0;31m'
MAGENTA='\033[0;35m'     # Domain names
BRIGHT_RED='\033[1;91m'  # Search term highlighting
NC='\033[0m'             # No color (reset)
