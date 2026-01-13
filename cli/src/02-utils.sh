# =============================================================================
# SECTION 02: Utility Functions
# =============================================================================
#
# PURPOSE
#   Common helper functions used across multiple commands. Project identification,
#   path resolution, and other shared utilities.
#
# DEPENDENCIES
#   - 01-constants.sh: None directly, but callers need colors
#
# PROVIDES
#   get_project_id()       Get UUID from .aoa/home.json
#   generate_project_id()  Create new UUID for project
#   get_project_root()     Find git repository root
#   get_project_name()     Extract project name from path
#
# =============================================================================

# Get project ID from .aoa/home.json (UUID generated at init)
get_project_id() {
    local project_root=$(get_project_root)
    if [ -z "$project_root" ]; then
        echo ""
        return
    fi

    local home_file="$project_root/.aoa/home.json"
    if [ -f "$home_file" ]; then
        jq -r '.project_id // empty' "$home_file" 2>/dev/null
    fi
}

# Generate a new UUID for project identification
generate_project_id() {
    # Try uuidgen first (Linux/macOS), fall back to Python
    if command -v uuidgen > /dev/null 2>&1; then
        uuidgen | tr '[:upper:]' '[:lower:]'
    else
        python3 -c "import uuid; print(uuid.uuid4())"
    fi
}

# Get project root (git repository root)
get_project_root() {
    git rev-parse --show-toplevel 2>/dev/null
}

# Get project name from root directory
get_project_name() {
    local root=$(get_project_root)
    if [ -n "$root" ]; then
        basename "$root"
    fi
}
