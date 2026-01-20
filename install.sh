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

    # 5. Check for shell integration
    for shell_rc in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.profile"; do
        if [ -f "$shell_rc" ]; then
            if grep -q '# BEGIN aOa' "$shell_rc" 2>/dev/null || grep -q 'eval "\$(aoa env)"' "$shell_rc" 2>/dev/null; then
                echo -e "  ${DIM}•${NC} Shell integration: ${BOLD}${shell_rc##*/}${NC}"
                FOUND_ITEMS=$((FOUND_ITEMS + 1))
                break  # Only report once
            fi
        fi
    done

    # 6. Check for registered projects (will clean them up)
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

    # 6. Remove shell integration (between markers)
    for shell_rc in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.profile"; do
        if [ -f "$shell_rc" ] && grep -q '# BEGIN aOa' "$shell_rc" 2>/dev/null; then
            echo -n "  Removing from ${shell_rc##*/}........... "
            sed -i '/# BEGIN aOa/,/# END aOa/d' "$shell_rc"
            echo -e "${GREEN}✓${NC}"
        fi
    done

    # Also remove old eval-style integration if present
    for shell_rc in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.bash_profile" "$HOME/.profile"; do
        if [ -f "$shell_rc" ] && grep -q 'eval "\$(aoa env)"' "$shell_rc" 2>/dev/null; then
            echo -n "  Removing old integration from ${shell_rc##*/}... "
            sed -i '/# aOa - O(1) environment/d' "$shell_rc"
            sed -i '/eval "\$(aoa env)"/d' "$shell_rc"
            echo -e "${GREEN}✓${NC}"
        fi
    done

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
    echo -e "      ${DIM}• Change port in .env${NC}"
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

# Check Docker installation
echo -n "  Docker installed.............. "
if ! command -v docker &> /dev/null; then
    echo -e "${RED}✗ Not found${NC}"
    echo
    echo -e "  ${YELLOW}Docker is required to run aOa services.${NC}"
    echo -e "  Install from: ${DIM}https://docs.docker.com/get-docker/${NC}"
    echo
    exit 1
fi
echo -e "${GREEN}✓${NC}"

# Check if user can actually RUN Docker (permissions + daemon)
echo -n "  Docker accessible............. "
# Note: Using if-then form to prevent set -e from triggering on failure
if DOCKER_ERR=$(docker info 2>&1); then
    echo -e "${GREEN}✓${NC}"
else
    # Diagnose the failure
    if echo "$DOCKER_ERR" | grep -qiE "permission denied|connect.*denied"; then
        echo -e "${RED}✗ Permission denied${NC}"
        echo

        # Is user already in docker group but session doesn't have it?
        if getent group docker 2>/dev/null | grep -qw "$USER"; then
            echo -e "  ${YELLOW}You're in the docker group, but this session doesn't have it yet.${NC}"
            echo
            echo -n -e "  ${CYAN}Continue installation with docker group? [Y/n] ${NC}"
            read -r continue_choice

            if [[ ! "$continue_choice" =~ ^[Nn]$ ]]; then
                echo
                echo -e "  ${DIM}Restarting installer with docker group...${NC}"
                echo
                # Use sg to run installer with the docker group
                exec sg docker -c "$0 $*"
            else
                echo
                echo -e "  ${BOLD}To continue later:${NC}"
                echo -e "    ${DIM}\$${NC} newgrp docker    ${DIM}# activate group (this session)${NC}"
                echo -e "    ${DIM}\$${NC} ./install.sh"
                echo
                echo -e "  ${DIM}Or log out and back in for permanent access.${NC}"
            fi
        else
            # User not in docker group - check if they can fix it themselves
            echo -e "  ${YELLOW}Your user ($USER) is not in the 'docker' group.${NC}"
            echo

            # Check for sudo access (non-interactive check)
            if sudo -n true 2>/dev/null; then
                # User has passwordless sudo
                HAS_SUDO=1
            elif groups 2>/dev/null | grep -qwE "sudo|wheel|admin"; then
                # User is in a sudo-capable group
                HAS_SUDO=1
            else
                HAS_SUDO=0
            fi

            if [ "$HAS_SUDO" -eq 1 ]; then
                echo -n -e "  ${CYAN}Add $USER to docker group? [Y/n] ${NC}"
                read -r add_choice

                if [[ ! "$add_choice" =~ ^[Nn]$ ]]; then
                    echo
                    echo -n "  Adding to docker group........ "
                    if sudo usermod -aG docker "$USER" 2>/dev/null; then
                        echo -e "${GREEN}✓${NC}"
                        echo
                        echo -e "  ${GREEN}✓ Added $USER to docker group${NC}"
                        echo
                        echo -n -e "  ${CYAN}Continue installation now? [Y/n] ${NC}"
                        read -r continue_choice

                        if [[ ! "$continue_choice" =~ ^[Nn]$ ]]; then
                            echo
                            echo -e "  ${DIM}Restarting installer with docker group...${NC}"
                            echo
                            # Use sg to run installer with the new group membership
                            exec sg docker -c "$0 $*"
                        else
                            echo
                            echo -e "  ${BOLD}To continue later:${NC}"
                            echo -e "    ${DIM}\$${NC} newgrp docker    ${DIM}# activate group (this session)${NC}"
                            echo -e "    ${DIM}\$${NC} ./install.sh"
                            echo
                            echo -e "  ${DIM}Or log out and back in for permanent access.${NC}"
                        fi
                    else
                        echo -e "${RED}✗ Failed${NC}"
                        echo
                        echo -e "  ${DIM}Try manually: sudo usermod -aG docker $USER${NC}"
                    fi
                fi
            else
                # No sudo - need sysadmin help
                echo -e "  ${BOLD}Ask your system administrator to run:${NC}"
                echo
                echo -e "    ${CYAN}sudo usermod -aG docker $USER${NC}"
                echo
                echo -e "  ${DIM}Then log out and back in (or run 'newgrp docker').${NC}"
            fi
        fi
        echo
        exit 1

    elif echo "$DOCKER_ERR" | grep -qiE "cannot connect|connection refused|daemon running"; then
        echo -e "${RED}✗ Docker daemon not running${NC}"
        echo

        # Check for sudo access
        if sudo -n true 2>/dev/null || groups 2>/dev/null | grep -qwE "sudo|wheel|admin"; then
            echo -n -e "  ${CYAN}Start Docker daemon? [Y/n] ${NC}"
            read -r start_choice

            if [[ ! "$start_choice" =~ ^[Nn]$ ]]; then
                echo
                echo -n "  Starting Docker............... "
                if sudo systemctl start docker 2>/dev/null; then
                    echo -e "${GREEN}✓${NC}"
                    sleep 2
                    # Re-check
                    if docker info &>/dev/null; then
                        echo -e "  ${GREEN}Docker is now running.${NC}"
                        echo
                    else
                        echo -e "  ${YELLOW}Docker started but still not accessible.${NC}"
                        echo -e "  ${DIM}You may need to be in the docker group.${NC}"
                        echo
                        exit 1
                    fi
                else
                    echo -e "${RED}✗ Failed${NC}"
                    echo
                    echo -e "  ${DIM}Try: sudo systemctl start docker${NC}"
                    exit 1
                fi
            else
                exit 1
            fi
        else
            echo -e "  ${BOLD}Ask your system administrator to run:${NC}"
            echo
            echo -e "    ${CYAN}sudo systemctl start docker${NC}"
            echo -e "    ${CYAN}sudo systemctl enable docker${NC}  ${DIM}(to start on boot)${NC}"
            echo
            exit 1
        fi
    else
        # Unknown error
        echo -e "${RED}✗ Error${NC}"
        echo
        echo -e "  ${DIM}${DOCKER_ERR}${NC}" | head -3
        echo
        exit 1
    fi
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

# Set default gateway configuration
AOA_GATEWAY_HOST="${AOA_GATEWAY_HOST:-localhost}"
AOA_GATEWAY_PORT="${AOA_GATEWAY_PORT:-8080}"

# Function to check if port is available
check_port() {
    local port=$1
    # Check if something is listening on this port
    if command -v ss &>/dev/null; then
        ss -tln 2>/dev/null | grep -q ":${port} " && return 1
    elif command -v netstat &>/dev/null; then
        netstat -tln 2>/dev/null | grep -q ":${port} " && return 1
    else
        # Fallback: try to connect
        (echo >/dev/tcp/localhost/${port}) 2>/dev/null && return 1
    fi
    return 0
}

# Check if chosen port is available
echo -n "  Port ${AOA_GATEWAY_PORT}.................... "
if check_port "$AOA_GATEWAY_PORT"; then
    echo -e "${GREEN}✓ Available${NC}"
else
    echo -e "${YELLOW}! In use${NC}"
    echo

    # Check if it's our own aOa container
    OUR_CONTAINER=$(docker ps --format '{{.Names}}' 2>/dev/null | grep -E "^aoa-${USER}$|^aoa$" || true)
    if [ -n "$OUR_CONTAINER" ]; then
        echo -e "  ${DIM}Found existing aOa container: ${OUR_CONTAINER}${NC}"
    else
        echo -e "  ${DIM}Port ${AOA_GATEWAY_PORT} is in use by another service.${NC}"
    fi
    echo

    # Find next available port
    NEW_PORT=$((AOA_GATEWAY_PORT + 1))
    while ! check_port "$NEW_PORT" && [ "$NEW_PORT" -lt 9000 ]; do
        NEW_PORT=$((NEW_PORT + 1))
    done

    # Offer options: keep current, use suggested, or enter custom
    echo -e "  ${BOLD}Options:${NC}"
    echo -e "    ${BOLD}1${NC}) Keep port ${AOA_GATEWAY_PORT} ${DIM}(will replace existing container)${NC}"
    if [ "$NEW_PORT" -lt 9000 ]; then
        echo -e "    ${BOLD}2${NC}) Use port ${NEW_PORT} ${DIM}(next available)${NC}"
    fi
    echo -e "    ${BOLD}3${NC}) Enter custom port"
    echo
    echo -n -e "  ${CYAN}Choice [1]: ${NC}"
    read -r port_choice

    case "$port_choice" in
        2)
            if [ "$NEW_PORT" -lt 9000 ]; then
                AOA_GATEWAY_PORT="$NEW_PORT"
                echo -e "  ${GREEN}✓ Using port ${AOA_GATEWAY_PORT}${NC}"
            else
                echo -e "  ${RED}✗ No available ports found${NC}"
                exit 1
            fi
            ;;
        3)
            echo -n -e "  ${CYAN}Enter port: ${NC}"
            read -r custom_port
            if [[ "$custom_port" =~ ^[0-9]+$ ]] && [ "$custom_port" -ge 1024 ] && [ "$custom_port" -le 65535 ]; then
                AOA_GATEWAY_PORT="$custom_port"
                echo -e "  ${GREEN}✓ Using port ${AOA_GATEWAY_PORT}${NC}"
            else
                echo -e "  ${RED}✗ Invalid port (must be 1024-65535)${NC}"
                exit 1
            fi
            ;;
        *)
            # Default: keep current port (will replace container)
            echo -e "  ${GREEN}✓ Keeping port ${AOA_GATEWAY_PORT}${NC}"
            ;;
    esac
    echo
fi

# Create .env file
echo -e "  ${BOLD}Creating configuration:${NC}"
echo
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

# Gateway configuration
AOA_GATEWAY_HOST=${AOA_GATEWAY_HOST}
AOA_GATEWAY_PORT=${AOA_GATEWAY_PORT}
EOF
echo -e "  .env........................ ${GREEN}✓${NC} ${DIM}${AOA_HOME}/.env${NC}"
echo -e "    PROJECTS_ROOT           = ${BOLD}${PROJECTS_ROOT}${NC}"
echo -e "    AOA_GATEWAY_HOST        = ${BOLD}${AOA_GATEWAY_HOST}${NC}"
echo -e "    AOA_GATEWAY_PORT        = ${BOLD}${AOA_GATEWAY_PORT}${NC}"
echo

# Auto-detect Claude sessions
CLAUDE_SESSIONS=""
if [ -d "${PROJECTS_ROOT}/.claude" ]; then
    CLAUDE_SESSIONS="${PROJECTS_ROOT}/.claude"
elif [ -d "${HOME}/.claude" ]; then
    CLAUDE_SESSIONS="${HOME}/.claude"
fi

if [ -n "$CLAUDE_SESSIONS" ]; then
    echo -e "  Claude sessions............. ${GREEN}✓${NC} ${DIM}${CLAUDE_SESSIONS}${NC}"
else
    echo -e "  Claude sessions............. ${DIM}not found (will be created on first use)${NC}"
    CLAUDE_SESSIONS="${HOME}/.claude"
fi

# Shell integration - add env vars to user's shell config
echo
SHELL_CONFIG=""
if [ -n "$ZSH_VERSION" ] || [ "$SHELL" = "$(command -v zsh)" ] || [ -f "$HOME/.zshrc" ]; then
    SHELL_CONFIG="$HOME/.zshrc"
elif [ -f "$HOME/.bashrc" ]; then
    SHELL_CONFIG="$HOME/.bashrc"
elif [ -f "$HOME/.bash_profile" ]; then
    SHELL_CONFIG="$HOME/.bash_profile"
elif [ -f "$HOME/.profile" ]; then
    SHELL_CONFIG="$HOME/.profile"
fi

if [ -n "$SHELL_CONFIG" ]; then
    # Check if already integrated (look for our marker)
    if grep -q '# BEGIN aOa' "$SHELL_CONFIG" 2>/dev/null; then
        # Update existing integration (replace between markers)
        sed -i '/# BEGIN aOa/,/# END aOa/d' "$SHELL_CONFIG"
    fi

    # Add fresh integration with direct exports (no eval)
    cat >> "$SHELL_CONFIG" << EOFSHELL

# BEGIN aOa
export AOA_URL="http://${AOA_GATEWAY_HOST}:${AOA_GATEWAY_PORT}"
export AOA_GATEWAY_HOST="${AOA_GATEWAY_HOST}"
export AOA_GATEWAY_PORT="${AOA_GATEWAY_PORT}"
# END aOa
EOFSHELL

    echo -e "  Shell integration........... ${GREEN}✓${NC} ${DIM}${SHELL_CONFIG}${NC}"
    echo -e "    AOA_URL                 = ${BOLD}http://${AOA_GATEWAY_HOST}:${AOA_GATEWAY_PORT}${NC}"

    # Store shell config path in .env for future updates (e.g., aoa port)
    echo "" >> "$AOA_HOME/.env"
    echo "# Shell config location (for aoa port updates)" >> "$AOA_HOME/.env"
    echo "SHELL_CONFIG=${SHELL_CONFIG}" >> "$AOA_HOME/.env"
else
    echo -e "  Shell integration........... ${YELLOW}!${NC} ${DIM}No shell config found${NC}"
    echo -e "    ${DIM}Manually add to your shell config:${NC}"
    echo -e "    ${DIM}export AOA_URL=\"http://${AOA_GATEWAY_HOST}:${AOA_GATEWAY_PORT}\"${NC}"
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

# Assemble from component templates
jq -n \
  --slurpfile perms "$AOA_HOME/plugin/templates/permissions.allow.json" \
  --slurpfile hooks "$AOA_HOME/plugin/templates/hooks.json" \
  '{
    _comment: "Permissions are pre-approved (no prompts). Patterns use glob syntax: '"'"'command:*'"'"' means '"'"'command followed by anything'"'"'",
    permissions: {allow: $perms[0]},
    hooks: $hooks[0].hooks,
    statusLine: $hooks[0].statusLine
  }' > "$AOA_DATA/settings.template.json"

# Validate assembled template
if ! jq empty "$AOA_DATA/settings.template.json" 2>/dev/null; then
    echo -e "${RED}✗ Template validation failed${NC}"
    echo -e "${DIM}Check plugin/templates/*.json for syntax errors${NC}"
    exit 1
fi

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

# Rolling window build output (shows last 10 lines, updates in place)
WINDOW_SIZE=10
build_with_progress() {
    local cmd="$1"
    local lines=()
    local i

    # Print placeholder lines
    for ((i=0; i<WINDOW_SIZE; i++)); do
        echo -e "  ${DIM}...${NC}"
    done

    # Run build and capture output line by line
    eval "$cmd" 2>&1 | while IFS= read -r line; do
        # Truncate long lines
        line="${line:0:70}"
        lines+=("$line")
        # Keep only last WINDOW_SIZE lines
        if [ ${#lines[@]} -gt $WINDOW_SIZE ]; then
            lines=("${lines[@]:1}")
        fi

        # Move cursor up and clear lines
        printf '\033[%dA' $WINDOW_SIZE
        for ((i=0; i<WINDOW_SIZE; i++)); do
            printf '\033[2K'  # Clear line
            if [ $i -lt ${#lines[@]} ]; then
                echo -e "  ${DIM}${lines[$i]}${NC}"
            else
                echo -e "  ${DIM}...${NC}"
            fi
        done
    done

    # Clear the window and show completion
    printf '\033[%dA' $WINDOW_SIZE
    for ((i=0; i<WINDOW_SIZE; i++)); do
        printf '\033[2K\n'
    done
    printf '\033[%dA' $WINDOW_SIZE
}

if [ "$USE_COMPOSE" -eq 1 ]; then
    echo -e "  ${DIM}Building Docker images (compose mode)...${NC}"
    echo
    build_with_progress "docker compose build --no-cache"
else
    echo -e "  ${DIM}Building unified Docker image...${NC}"
    echo
    build_with_progress "docker build --no-cache -t aoa ."
fi

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
        -p "${AOA_GATEWAY_PORT}:8080" \
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
if curl -s "http://localhost:${AOA_GATEWAY_PORT}/health" > /dev/null 2>&1; then
    echo -e "  ${GREEN}✓ Services running on port ${AOA_GATEWAY_PORT}${NC}"
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
    echo -e "  ${DIM}•${NC} Docker Compose        ${DIM}- Project: aoa-${USER}, Port: ${AOA_GATEWAY_PORT}${NC}"
else
    echo -e "  ${DIM}•${NC} Docker container      ${DIM}- Name: aoa-${USER}, Port: ${AOA_GATEWAY_PORT}${NC}"
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
