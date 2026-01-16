#!/bin/bash
# =============================================================================
# aOa - Angle O(1)f Attack
# Global Installation Script
# =============================================================================
#
# 5 angles. 1 attack.
#
# This script installs aOa GLOBALLY to ~/.aoa/
# Then use 'aoa init' in any project to enable it.
#
# Install once, use everywhere.
#
# =============================================================================

set -e

# Colors - aOa brand
CYAN='\033[96m'
GREEN='\033[92m'
YELLOW='\033[93m'
RED='\033[91m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# Get script directory (where aOa repo is cloned)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# AOA_HOME is the repo itself
AOA_HOME="$SCRIPT_DIR"

# Runtime data directory (inside repo, gitignored)
AOA_DATA="$SCRIPT_DIR/data"

# =============================================================================
# Mode Selection (can be set via flags)
# =============================================================================
# Default: unified (single container, simpler)
# --compose: multi-container with docker-compose (full isolation)
# --global: install CLI to /usr/local/bin for all users (requires sudo)

USE_COMPOSE=0
GLOBAL_INSTALL=0

# Parse all flags
while [[ "$1" == --* ]]; do
    case "$1" in
        --compose) USE_COMPOSE=1; shift ;;
        --global)  GLOBAL_INSTALL=1; shift ;;
        *) break ;;
    esac
done

# =============================================================================
# Uninstall Mode
# =============================================================================

if [[ "$1" == "--uninstall" ]]; then
    echo -e "${CYAN}${BOLD}"
    echo "  ╔═══════════════════════════════════════════════════════════════╗"
    echo "  ║                                                               ║"
    echo "  ║     ⚡ aOa Global Uninstaller                                 ║"
    echo "  ║                                                               ║"
    echo "  ╚═══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    echo

    echo -e "${YELLOW}${BOLD}The following will be removed:${NC}"
    echo

    FOUND_ITEMS=0

    # 1. Docker containers (check both instance-scoped and legacy)
    AOA_UNIFIED=0
    AOA_COMPOSE=0

    # Check for instance-scoped container
    if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^aoa-${USER}$"; then
        AOA_UNIFIED=1
        echo -e "  ${DIM}•${NC} Docker container: ${BOLD}aoa-${USER}${NC} (unified)"
        FOUND_ITEMS=$((FOUND_ITEMS + 1))
    elif docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^aoa$"; then
        # Legacy container name
        AOA_UNIFIED=1
        echo -e "  ${DIM}•${NC} Docker container: ${BOLD}aoa${NC} (unified, legacy)"
        FOUND_ITEMS=$((FOUND_ITEMS + 1))
    fi

    # Check for compose services (instance-scoped)
    if [ -f "$AOA_HOME/docker-compose.yml" ]; then
        AOA_COMPOSE_COUNT=$(cd "$AOA_HOME" && docker compose -p "aoa-${USER}" ps -q 2>/dev/null | wc -l)
        if [ "$AOA_COMPOSE_COUNT" -eq 0 ]; then
            # Try legacy project name
            AOA_COMPOSE_COUNT=$(cd "$AOA_HOME" && docker compose ps -q 2>/dev/null | wc -l)
        fi
        if [ "$AOA_COMPOSE_COUNT" -gt 0 ]; then
            AOA_COMPOSE=1
            echo -e "  ${DIM}•${NC} Docker services: ${BOLD}${AOA_COMPOSE_COUNT} running${NC} (compose)"
            FOUND_ITEMS=$((FOUND_ITEMS + 1))
        fi
    fi

    # 2. Docker images (both unified 'aoa' and compose 'aoa-*')
    AOA_IMAGES=$(docker images --format '{{.Repository}}' 2>/dev/null | grep -cE "^aoa([-_]|$)" || true)
    if [ "$AOA_IMAGES" -gt 0 ]; then
        echo -e "  ${DIM}•${NC} Docker images: ${BOLD}${AOA_IMAGES} images${NC}"
        FOUND_ITEMS=$((FOUND_ITEMS + 1))
    fi

    # 3. Runtime data directory
    if [ -d "$AOA_DATA" ]; then
        SIZE=$(du -sh "$AOA_DATA" 2>/dev/null | cut -f1)
        echo -e "  ${DIM}•${NC} Runtime data: ${BOLD}$AOA_DATA${NC} ${DIM}(${SIZE})${NC}"
        FOUND_ITEMS=$((FOUND_ITEMS + 1))
    fi

    # 4. CLI in PATH (check both user and global locations)
    if [ -f "$HOME/bin/aoa" ]; then
        echo -e "  ${DIM}•${NC} CLI: ${BOLD}~/bin/aoa${NC}"
        FOUND_ITEMS=$((FOUND_ITEMS + 1))
    fi
    if [ -L "/usr/local/bin/aoa" ]; then
        echo -e "  ${DIM}•${NC} CLI: ${BOLD}/usr/local/bin/aoa${NC} (global)"
        FOUND_ITEMS=$((FOUND_ITEMS + 1))
    fi

    # 5. Check for registered projects (will clean them up)
    PROJECTS_TO_CLEAN=()
    if [ -f "$AOA_DATA/projects.json" ]; then
        PROJECT_COUNT=$(jq 'length' "$AOA_DATA/projects.json" 2>/dev/null || echo 0)
        if [ "$PROJECT_COUNT" -gt 0 ]; then
            echo -e "  ${DIM}•${NC} Registered projects: ${BOLD}${PROJECT_COUNT}${NC} ${DIM}(will clean aOa files)${NC}"
            # Store project paths for cleanup
            while IFS= read -r path; do
                PROJECTS_TO_CLEAN+=("$path")
            done < <(jq -r '.[].path' "$AOA_DATA/projects.json" 2>/dev/null)
            FOUND_ITEMS=$((FOUND_ITEMS + 1))
        fi
    fi

    echo

    if [ $FOUND_ITEMS -eq 0 ]; then
        echo -e "  ${DIM}Nothing to uninstall - aOa not found.${NC}"
        echo
        exit 0
    fi

    echo -n -e "${YELLOW}Proceed with uninstall? [y/N] ${NC}"
    read -r response
    echo

    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        echo -e "  ${DIM}Uninstall cancelled.${NC}"
        echo
        exit 0
    fi

    # Perform uninstall
    echo -e "${CYAN}${BOLD}Removing aOa...${NC}"
    echo

    # 1. Stop and remove Docker containers/services (instance-scoped and legacy)
    if [ "$AOA_UNIFIED" -eq 1 ]; then
        echo -n "  Stopping container............ "
        docker stop "aoa-${USER}" > /dev/null 2>&1 || true
        docker rm "aoa-${USER}" > /dev/null 2>&1 || true
        # Legacy name
        docker stop aoa > /dev/null 2>&1 || true
        docker rm aoa > /dev/null 2>&1 || true
        echo -e "${GREEN}✓${NC}"
    fi
    if [ "$AOA_COMPOSE" -eq 1 ]; then
        echo -n "  Stopping services............. "
        cd "$AOA_HOME" && docker compose -p "aoa-${USER}" down --volumes --remove-orphans > /dev/null 2>&1 || true
        # Try legacy project name too
        cd "$AOA_HOME" && docker compose down --volumes --remove-orphans > /dev/null 2>&1 || true
        echo -e "${GREEN}✓${NC}"
    fi

    # 2. Remove Docker images (both unified 'aoa' and compose 'aoa-*')
    echo -n "  Removing images............... "
    # Get all aOa images (including dangling/intermediate)
    # Pattern matches: aoa, aoa:tag, aoa-service, aoa-service:tag
    AOA_IMAGE_IDS=$(docker images --format '{{.Repository}}:{{.Tag}} {{.ID}}' 2>/dev/null | \
                    grep -E "^aoa([-_:]|$)" | awk '{print $2}' || true)

    if [ -n "$AOA_IMAGE_IDS" ]; then
        # Force remove all images (even if containers exist)
        echo "$AOA_IMAGE_IDS" | xargs -r docker rmi -f > /dev/null 2>&1 || true
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${DIM}none${NC}"
    fi

    # Also clean up any dangling aoa images
    docker image prune -f --filter "label=aoa" > /dev/null 2>&1 || true

    # 3. Clean up registered projects (BEFORE removing ~/.aoa/)
    if [ ${#PROJECTS_TO_CLEAN[@]} -gt 0 ]; then
        echo -e "  Cleaning projects:"
        for proj_path in "${PROJECTS_TO_CLEAN[@]}"; do
            if [ -d "$proj_path" ]; then
                proj_name=$(basename "$proj_path")
                echo -n "    ${proj_name}... "

                # Remove aOa hooks (aoa-* prefix)
                rm -f "$proj_path/.claude/hooks/aoa-"* 2>/dev/null

                # Remove aOa skills (folders and files starting with aoa)
                rm -rf "$proj_path/.claude/skills/aoa"* 2>/dev/null

                # Remove aOa agents (aoa-* prefix)
                rm -f "$proj_path/.claude/agents/aoa-"* 2>/dev/null

                # Remove .aoa-config
                rm -f "$proj_path/.aoa-config" 2>/dev/null

                # Check settings.local.json - only remove if unchanged from template
                if [ -f "$proj_path/.claude/settings.local.json" ] && [ -f "$AOA_DATA/settings.template.json" ]; then
                    TEMPLATE_HASH=$(md5sum "$AOA_DATA/settings.template.json" 2>/dev/null | cut -d' ' -f1)
                    SETTINGS_HASH=$(md5sum "$proj_path/.claude/settings.local.json" 2>/dev/null | cut -d' ' -f1)

                    if [ "$TEMPLATE_HASH" = "$SETTINGS_HASH" ]; then
                        rm -f "$proj_path/.claude/settings.local.json"
                        echo -e "${GREEN}✓${NC}"
                    else
                        echo -e "${GREEN}✓${NC} ${YELLOW}(settings.local.json has customizations - preserved)${NC}"
                    fi
                else
                    echo -e "${GREEN}✓${NC}"
                fi

                # Clean up empty .claude subdirs
                rmdir "$proj_path/.claude/hooks" 2>/dev/null || true
                rmdir "$proj_path/.claude/skills" 2>/dev/null || true
                rmdir "$proj_path/.claude/agents" 2>/dev/null || true
                rmdir "$proj_path/.claude" 2>/dev/null || true
            fi
        done
    fi

    # 4. Remove runtime data
    if [ -d "$AOA_DATA" ]; then
        echo -n "  Removing data/................ "
        rm -rf "$AOA_DATA"
        echo -e "${GREEN}✓${NC}"
    fi

    # 5. Remove CLI (both user and global locations)
    if [ -f "$HOME/bin/aoa" ]; then
        echo -n "  Removing ~/bin/aoa............ "
        rm -f "$HOME/bin/aoa"
        echo -e "${GREEN}✓${NC}"
    fi
    if [ -L "/usr/local/bin/aoa" ]; then
        echo -n "  Removing /usr/local/bin/aoa... "
        sudo rm -f /usr/local/bin/aoa
        echo -e "${GREEN}✓${NC}"
    fi

    echo
    echo -e "  ${GREEN}${BOLD}✓ aOa uninstalled${NC}"
    echo

    exit 0
fi

# =============================================================================
# Header
# =============================================================================

clear
echo -e "${CYAN}${BOLD}"
echo "  ╔═══════════════════════════════════════════════════════════════╗"
echo "  ║                                                               ║"
echo "  ║     ⚡ aOa - Angle O(1)f Attack                               ║"
echo "  ║                                                               ║"
echo "  ║     5 angles. 1 attack.                                       ║"
echo "  ║     Cut Claude Code costs by 2/3.                             ║"
echo "  ║                                                               ║"
echo "  ╚═══════════════════════════════════════════════════════════════╝"
echo -e "${NC}"
echo
echo -e "  ${BOLD}Installation${NC}"
echo -e "  ${DIM}Install from this directory, then 'aoa init' in any project.${NC}"
echo
echo -e "  ${DIM}This installer will:${NC}"
echo -e "  ${DIM}  1. Check prerequisites (Docker)${NC}"
echo -e "  ${DIM}  2. Configure your projects root${NC}"
echo -e "  ${DIM}  3. Create data/ directory for runtime state${NC}"
echo -e "  ${DIM}  4. Build the aOa Docker image${NC}"
echo -e "  ${DIM}  5. Start aOa services${NC}"
echo -e "  ${DIM}  6. Install the aoa CLI${NC}"
echo

# Choose deployment mode (unless already set via --compose flag)
if [ "$USE_COMPOSE" -eq 0 ]; then
    echo -e "  ${CYAN}${BOLD}Choose deployment mode:${NC}"
    echo
    echo -e "  ${BOLD}[1]${NC} Single Container ${GREEN}(Recommended)${NC}"
    echo -e "      ${DIM}• One container, one port (8080)${NC}"
    echo -e "      ${DIM}• All services via supervisord${NC}"
    echo -e "      ${DIM}• Simpler, fewer resources${NC}"
    echo -e "      ${DIM}• Change port in .aoa/config.json${NC}"
    echo
    echo -e "  ${BOLD}[2]${NC} Docker Compose"
    echo -e "      ${DIM}• 5 separate containers${NC}"
    echo -e "      ${DIM}• Network isolation between services${NC}"
    echo -e "      ${DIM}• Better for debugging/development${NC}"
    echo
    echo -n -e "  ${YELLOW}Enter choice [1/2]: ${NC}"
    read -r mode_choice
    echo
    if [ "$mode_choice" = "2" ]; then
        USE_COMPOSE=1
        echo -e "  ${DIM}Using Docker Compose mode${NC}"
    else
        USE_COMPOSE=0
        echo -e "  ${DIM}Using single container mode${NC}"
    fi
    echo
fi

echo -n -e "  ${YELLOW}Press Enter to continue...${NC}"
read -r
echo

# =============================================================================
# Step 1: Prerequisites
# =============================================================================

echo -e "${CYAN}${BOLD}[1/5] Checking Prerequisites${NC}"
echo -e "${DIM}─────────────────────────────────────────────────────────────────${NC}"
echo

# Check Docker
echo -n "  Docker........................ "
if command -v docker &> /dev/null; then
    echo -e "${GREEN}✓ Found${NC}"
else
    echo -e "${RED}✗ Not found${NC}"
    echo
    echo -e "  ${YELLOW}Docker is required to run aOa services.${NC}"
    echo "  Install from: https://docs.docker.com/get-docker/"
    echo
    exit 1
fi

# Check Python3
echo -n "  Python 3...................... "
if command -v python3 &> /dev/null; then
    echo -e "${GREEN}✓ Found${NC}"
else
    echo -e "${YELLOW}! Not found (hooks may not work)${NC}"
fi

# Check jq
echo -n "  jq............................ "
if command -v jq &> /dev/null; then
    echo -e "${GREEN}✓ Found${NC}"
else
    echo -e "${YELLOW}! Not found (CLI features limited)${NC}"
fi

echo
echo -e "  ${GREEN}Prerequisites satisfied.${NC}"
echo
sleep 1

# =============================================================================
# Step 2: Configure Projects Root
# =============================================================================

echo -e "${CYAN}${BOLD}[2/6] Configure Projects Root${NC}"
echo -e "${DIM}─────────────────────────────────────────────────────────────────${NC}"
echo
echo -e "  ${DIM}aOa needs access to your projects for indexing.${NC}"
echo -e "  ${DIM}This sets the root directory Docker can see (read-only).${NC}"
echo
echo -e "  ${BOLD}Where are your projects located?${NC}"
echo -e "  ${DIM}Default: ${HOME}${NC}"
echo
echo -n -e "  ${YELLOW}Projects root [${HOME}]: ${NC}"
read -r projects_root_input
echo

# Use default if empty
PROJECTS_ROOT="${projects_root_input:-$HOME}"

# Validate path exists
if [ ! -d "$PROJECTS_ROOT" ]; then
    echo -e "  ${RED}✗ Directory not found: ${PROJECTS_ROOT}${NC}"
    echo -e "  ${DIM}Please create it or choose a different path.${NC}"
    exit 1
fi

# Resolve to absolute path
PROJECTS_ROOT="$(cd "$PROJECTS_ROOT" && pwd)"

echo -e "  ${GREEN}✓ Projects root: ${PROJECTS_ROOT}${NC}"
echo

# Set default gateway port if not already set
GATEWAY_PORT="${GATEWAY_PORT:-8080}"

# Create .env file
echo -n "  Creating .env configuration... "
cat > "$AOA_HOME/.env" << EOF
# =============================================================================
# aOa Docker Configuration
# =============================================================================
# Generated by install.sh on $(date)
# Edit this file and restart Docker to change paths.
#
# View current config: aoa info
# Restart: aoa stop && aoa start
# =============================================================================

# Installation mode (0=unified single container, 1=docker-compose)
USE_COMPOSE=${USE_COMPOSE}

# Root directory for user projects (mounted read-only to /userhome)
# Only projects registered via 'aoa init' within this root are indexed.
PROJECTS_ROOT=${PROJECTS_ROOT}

# Gateway port (change if 8080 is in use)
GATEWAY_PORT=${GATEWAY_PORT}
EOF
echo -e "${GREEN}✓${NC}"

# Auto-detect Claude sessions
CLAUDE_SESSIONS=""
if [ -d "${PROJECTS_ROOT}/.claude" ]; then
    CLAUDE_SESSIONS="${PROJECTS_ROOT}/.claude"
elif [ -d "${HOME}/.claude" ]; then
    CLAUDE_SESSIONS="${HOME}/.claude"
fi

if [ -n "$CLAUDE_SESSIONS" ]; then
    echo -e "  ${GREEN}✓ Claude sessions found: ${CLAUDE_SESSIONS}${NC}"
else
    echo -e "  ${DIM}  Claude sessions not found (will be created on first use)${NC}"
    CLAUDE_SESSIONS="${HOME}/.claude"
fi

echo
sleep 1

# =============================================================================
# Step 3: Create Runtime Data Directory
# =============================================================================

echo -e "${CYAN}${BOLD}[3/6] Setting Up Runtime Data Directory${NC}"
echo -e "${DIM}─────────────────────────────────────────────────────────────────${NC}"
echo
echo -e "  ${DIM}Creating: ${AOA_DATA}${NC}"
echo

# Create data directory structure
mkdir -p "$AOA_DATA"/{indexes,repos}

# Create empty projects.json
echo -n "  Initializing projects......... "
echo "[]" > "$AOA_DATA/projects.json"
echo -e "${GREEN}✓${NC}"

# Create settings template (for aoa init)
echo -n "  Creating settings template.... "
cat > "$AOA_DATA/settings.template.json" << 'EOFCONFIG'
{
  "_comment": "Permissions are pre-approved (no prompts). Patterns use glob syntax: 'command:*' means 'command followed by anything'",
  "permissions": {
    "allow": [
      "Bash(aoa grep:*)",
      "Bash(aoa egrep:*)",
      "Bash(aoa find:*)",
      "Bash(aoa tree:*)",
      "Bash(aoa locate:*)",
      "Bash(aoa head:*)",
      "Bash(aoa tail:*)",
      "Bash(aoa lines:*)",
      "Bash(aoa hot:*)",
      "Bash(aoa touched:*)",
      "Bash(aoa focus:*)",
      "Bash(aoa predict:*)",
      "Bash(aoa health:*)",
      "Bash(aoa help:*)",
      "Bash(aoa metrics:*)",
      "Bash(aoa baseline:*)",
      "Bash(aoa intent:*)",
      "Bash(aoa services:*)",
      "Bash(aoa changes:*)",
      "Bash(aoa files:*)",
      "Bash(aoa outline:*)",
      "Bash(aoa projects:*)",
      "Bash(aoa search:*)",
      "Bash(aoa multi:*)",
      "Bash(aoa pattern:*)",
      "Bash(docker-compose:*)",
      "Bash(docker ps:*)",
      "Bash(docker logs:*)",
      "Bash(docker run:*)",
      "Bash(docker exec:*)",
      "Bash(docker stop:*)",
      "Bash(docker rm:*)",
      "Bash(docker build:*)",
      "Bash(curl:*)",
      "Bash(ls:*)"
    ]
  },
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "python3 \"$CLAUDE_PROJECT_DIR/.claude/hooks/aoa-intent-summary.py\"",
            "timeout": 2
          },
          {
            "type": "command",
            "command": "python3 \"$CLAUDE_PROJECT_DIR/.claude/hooks/aoa-predict-context.py\"",
            "timeout": 3
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Read|Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "python3 \"$CLAUDE_PROJECT_DIR/.claude/hooks/aoa-intent-prefetch.py\"",
            "timeout": 2
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Read|Edit|Write|Bash|Grep|Glob",
        "hooks": [
          {
            "type": "command",
            "command": "python3 \"$CLAUDE_PROJECT_DIR/.claude/hooks/aoa-intent-capture.py\"",
            "timeout": 5
          }
        ]
      }
    ]
  },
  "statusLine": {
    "type": "command",
    "command": "bash \"$CLAUDE_PROJECT_DIR/.claude/hooks/aoa-status-line.sh\""
  }
}
EOFCONFIG
echo -e "${GREEN}✓${NC}"

echo
echo -e "  ${GREEN}Runtime data directory ready.${NC}"
echo
sleep 1

# =============================================================================
# Step 4: Build Docker Image
# =============================================================================

echo -e "${CYAN}${BOLD}[4/6] Building aOa Services${NC}"
echo -e "${DIM}─────────────────────────────────────────────────────────────────${NC}"
echo

cd "$AOA_HOME"

if [ "$USE_COMPOSE" -eq 1 ]; then
    echo -e "  ${DIM}Building Docker images (compose mode)...${NC}"
    echo -e "  ${DIM}This may take a minute on first run.${NC}"
    echo
    docker compose build --no-cache --quiet
else
    echo -e "  ${DIM}Building unified Docker image...${NC}"
    echo -e "  ${DIM}This may take a minute on first run.${NC}"
    echo
    docker build --no-cache -t aoa . --quiet
fi

echo
echo -e "  ${GREEN}✓ Docker image(s) built${NC}"
echo
sleep 1

# =============================================================================
# Step 5: Start Services
# =============================================================================

echo -e "${CYAN}${BOLD}[5/6] Starting aOa Services${NC}"
echo -e "${DIM}─────────────────────────────────────────────────────────────────${NC}"
echo

# Clean up existing aOa containers for THIS user instance
docker compose -p "aoa-${USER}" -f "$AOA_HOME/docker-compose.yml" down 2>/dev/null || true
docker stop "aoa-${USER}" 2>/dev/null || true
docker rm "aoa-${USER}" 2>/dev/null || true
# Legacy cleanup (old non-scoped containers)
docker stop aoa 2>/dev/null || true
docker rm aoa 2>/dev/null || true

if [ "$USE_COMPOSE" -eq 1 ]; then
    # Start all services via docker-compose (reads from .env)
    # Use instance-scoped project name for multi-user support
    cd "$AOA_HOME"
    docker compose -p "aoa-${USER}" up -d
else
    # Start unified single container
    # Use instance-scoped name for multi-user support
    docker run -d \
        --name "aoa-${USER}" \
        -p "${GATEWAY_PORT}:8080" \
        -v "${PROJECTS_ROOT}:/userhome:ro" \
        -v "${AOA_DATA}/repos:/repos:rw" \
        -v "${AOA_DATA}/indexes:/indexes:rw" \
        -v "${AOA_DATA}:/config:rw" \
        -v "${CLAUDE_SESSIONS}:/claude-sessions:ro" \
        -e "USER_HOME=${PROJECTS_ROOT}" \
        --restart unless-stopped \
        aoa > /dev/null
fi

echo -n "  Starting services"
for i in {1..5}; do
    echo -n "."
    sleep 1
done
echo

# Verify services are running
if curl -s "http://localhost:${GATEWAY_PORT}/health" > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Services running on port ${GATEWAY_PORT}${NC}"
else
    echo -e "  ${YELLOW}! Services starting... (may take a moment)${NC}"
fi
echo
sleep 1

# =============================================================================
# Step 6: Install CLI
# =============================================================================

echo -e "${CYAN}${BOLD}[6/6] Installing aOa CLI${NC}"
echo -e "${DIM}─────────────────────────────────────────────────────────────────${NC}"
echo

# Make CLI executable
chmod +x "$AOA_HOME/cli/aoa"

# Symlink CLI to appropriate location
PATH_UPDATED=0
if [ "$GLOBAL_INSTALL" -eq 1 ]; then
    # Global install: symlink to /usr/local/bin (requires sudo)
    echo -e "  ${DIM}Installing CLI globally (requires sudo)...${NC}"
    sudo ln -sf "$AOA_HOME/cli/aoa" /usr/local/bin/aoa
    CLI_LOCATION="/usr/local/bin/aoa"
    echo -e "  ${GREEN}✓ Symlinked to /usr/local/bin/aoa${NC}"
else
    # User install: symlink to ~/bin
    mkdir -p "$HOME/bin"
    ln -sf "$AOA_HOME/cli/aoa" "$HOME/bin/aoa"
    CLI_LOCATION="~/bin/aoa"
    echo -e "  ${GREEN}✓ Symlinked to ~/bin/aoa${NC}"

    # Check if ~/bin is in PATH
    if [[ ":$PATH:" != *":$HOME/bin:"* ]]; then
        # Detect shell config file
        SHELL_CONFIG=""
        if [ -n "$ZSH_VERSION" ] || [ -f "$HOME/.zshrc" ]; then
            SHELL_CONFIG="$HOME/.zshrc"
        elif [ -f "$HOME/.bashrc" ]; then
            SHELL_CONFIG="$HOME/.bashrc"
        elif [ -f "$HOME/.bash_profile" ]; then
            SHELL_CONFIG="$HOME/.bash_profile"
        fi

        if [ -n "$SHELL_CONFIG" ]; then
            if ! grep -q 'export PATH="\$HOME/bin:\$PATH"' "$SHELL_CONFIG" 2>/dev/null; then
                echo "" >> "$SHELL_CONFIG"
                echo "# Added by aOa installer" >> "$SHELL_CONFIG"
                echo 'export PATH="$HOME/bin:$PATH"' >> "$SHELL_CONFIG"
                echo -e "  ${GREEN}✓ Added ~/bin to PATH in ${SHELL_CONFIG##*/}${NC}"
                PATH_UPDATED=1
            fi
        fi
    fi
fi

echo

# =============================================================================
# Complete
# =============================================================================

echo -e "${CYAN}${BOLD}"
echo "  ╔═══════════════════════════════════════════════════════════════╗"
echo "  ║                                                               ║"
echo "  ║     ⚡ aOa Installed Globally!                                ║"
echo "  ║                                                               ║"
echo "  ╚═══════════════════════════════════════════════════════════════╝"
echo -e "${NC}"
echo

echo -e "${GREEN}${BOLD}What was installed:${NC}"
echo -e "  ${DIM}•${NC} ${BOLD}${AOA_HOME}${NC}"
echo -e "      ${DIM}└─ data/              Runtime state (indexes, repos, config)${NC}"
if [ "$USE_COMPOSE" -eq 1 ]; then
    echo -e "  ${DIM}•${NC} Docker Compose        ${DIM}- Project: aoa-${USER}, Port: ${GATEWAY_PORT}${NC}"
else
    echo -e "  ${DIM}•${NC} Docker container      ${DIM}- Name: aoa-${USER}, Port: ${GATEWAY_PORT}${NC}"
fi
echo -e "  ${DIM}•${NC} ${CLI_LOCATION} → ${BOLD}${AOA_HOME}/cli/aoa${NC}"
echo

echo -e "${YELLOW}${BOLD}Next steps:${NC}"
if [ "$PATH_UPDATED" -eq 1 ]; then
    echo -e "  ${BOLD}1. Restart your terminal${NC} ${DIM}(or run: source ~/${SHELL_CONFIG##*/})${NC}"
    echo
fi
echo -e "  ${BOLD}Enable aOa in any project:${NC}"
echo -e "  ${DIM}\$${NC} cd ~/your-project"
echo -e "  ${DIM}\$${NC} aoa init"
echo

echo -e "${CYAN}${BOLD}Quick test:${NC}"
echo -e "  ${DIM}\$${NC} aoa health         ${DIM}# Check services${NC}"
echo -e "  ${DIM}\$${NC} aoa projects       ${DIM}# List enabled projects${NC}"
echo

echo -e "${DIM}Install location: ${AOA_HOME}${NC}"
echo -e "${DIM}Data directory:   ${AOA_DATA}${NC}"
echo
