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
#   GATEWAY_HOST, GATEWAY_PORT   Service connection settings
#   INDEX_URL, STATUS_URL        API endpoint URLs
#   AOA_HOME, AOA_DATA           Installation paths
#   BOLD, DIM, GREEN, etc.       ANSI color codes for output formatting
#
# =============================================================================

# Gateway configuration (connects to aOa index service)
GATEWAY_HOST="${AOA_GATEWAY_HOST:-localhost}"
GATEWAY_PORT="${AOA_GATEWAY_PORT:-8080}"

# Find AOA_HOME by locating the CLI script itself
CLI_PATH="$(readlink -f "$0")"
AOA_HOME="$(dirname "$(dirname "$CLI_PATH")")"
AOA_DATA="${AOA_DATA:-$AOA_HOME/data}"

# Service URLs
INDEX_URL="http://${GATEWAY_HOST}:${GATEWAY_PORT}"
STATUS_URL="http://${GATEWAY_HOST}:${GATEWAY_PORT}"

# ANSI Colors
BOLD='\033[1m'
DIM='\033[2m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
RED='\033[0;31m'
BRIGHT_RED='\033[1;91m'  # Search term highlighting
NC='\033[0m'             # No color (reset)
