# =============================================================================
# SECTION 70: Setup & Configuration
# =============================================================================
#
# PURPOSE
#   Project setup commands. Run once per project to enable aOa. Creates
#   .aoa/ directory, registers with index service, installs hooks.
#
# DEPENDENCIES
#   - 01-constants.sh: INDEX_URL, AOA_HOME, colors
#   - 02-utils.sh: get_project_id(), generate_project_id(), get_project_root()
#
# COMMANDS PROVIDED
#   cmd_init       Initialize aOa for current project
#   cmd_remove     Remove aOa from current project
#   cmd_projects   List all aOa-enabled projects
#
# NOTE
#   These commands modify the filesystem and should be run interactively.
#   They create/delete .aoa/ directories and update global state.
#
# =============================================================================

cmd_init() {
    echo -e "${CYAN}${BOLD}⚡ aOa - Initializing Project${NC}"
    echo

    # Check if aOa is installed globally
    if [ ! -d "$AOA_HOME" ]; then
        echo -e "${RED}aOa not installed globally.${NC}"
        echo -e "${DIM}Run ./install.sh from the aOa repository first.${NC}"
        return 1
    fi

    # Check if we're in a git repo
    local project_root=$(get_project_root)
    if [ -z "$project_root" ]; then
        echo -e "${RED}Not in a git repository.${NC}"
        echo -e "${DIM}aOa requires a git repo to detect project boundaries.${NC}"
        return 1
    fi

    local project_name=$(get_project_name)

    # Check if already has a project_id, otherwise generate new one
    local existing_id=$(get_project_id)
    local project_id="${existing_id:-$(generate_project_id)}"

    echo -e "  Project: ${BOLD}${project_name}${NC}"
    echo -e "  Path:    ${DIM}${project_root}${NC}"
    echo -e "  ID:      ${DIM}${project_id}${NC}"
    echo

    # Check if already initialized
    if [ -f "$project_root/.claude/hooks/aoa-status-line.sh" ]; then
        echo -e "${YELLOW}Project already initialized.${NC}"
        echo -e "${DIM}Run 'aoa remove' first to reinitialize.${NC}"
        return 0
    fi

    # Create .claude directories
    mkdir -p "$project_root/.claude/hooks"
    mkdir -p "$project_root/.claude/skills"

    # Copy hooks from templates
    echo -n "  Installing hooks.............. "
    cp "$AOA_HOME/plugin/hooks/"*.py "$project_root/.claude/hooks/" 2>/dev/null || true
    cp "$AOA_HOME/plugin/hooks/"*.sh "$project_root/.claude/hooks/" 2>/dev/null || true
    chmod +x "$project_root/.claude/hooks/"*.py "$project_root/.claude/hooks/"*.sh 2>/dev/null || true
    echo -e "${GREEN}✓${NC}"

    # Copy skills (directory-based structure for Claude Code)
    echo -n "  Installing skills............. "
    for skill_dir in "$AOA_HOME/plugin/skills/"*/; do
        if [ -d "$skill_dir" ]; then
            cp -r "$skill_dir" "$project_root/.claude/skills/" 2>/dev/null || true
        fi
    done
    echo -e "${GREEN}✓${NC}"

    # Copy agents
    echo -n "  Installing agents............. "
    mkdir -p "$project_root/.claude/agents"
    cp "$AOA_HOME/plugin/agents/"*.md "$project_root/.claude/agents/" 2>/dev/null || true
    echo -e "${GREEN}✓${NC}"

    # Create .aoa/ folder with home pointer
    echo -n "  Creating .aoa/ config......... "
    mkdir -p "$project_root/.aoa"

    # home.json - project config with UUID identifier
    cat > "$project_root/.aoa/home.json" << EOFHOME
{
  "aoa_home": "$AOA_HOME",
  "aoa_url": "$AOA_URL",
  "data_dir": "$AOA_DATA",
  "project_id": "$project_id",
  "project_root": "$project_root"
}
EOFHOME

    # domains/ folder - for intelligence angle (domain definitions + enrichment)
    mkdir -p "$project_root/.aoa/domains"

    # whitelist.txt - optional repos/URLs for this project
    if [ ! -f "$project_root/.aoa/whitelist.txt" ]; then
        cat > "$project_root/.aoa/whitelist.txt" << 'EOFWHITELIST'
# aOa Whitelist - URLs allowed for this project
# Add one domain per line (HTTPS only)
#
# Examples:
# github.com/your-org/repo
# docs.your-company.com
# internal-git.example.com
EOFWHITELIST
    fi

    # README.md - explains the folder (only if not present)
    if [ ! -f "$project_root/.aoa/README.md" ]; then
        cat > "$project_root/.aoa/README.md" << 'EOFREADME'
# .aoa - aOa Project Link

This folder links this project to your global aOa installation.

## Files

| File | Purpose |
|------|---------|
| `home.json` | Points to global aOa install location |
| `whitelist.txt` | Optional: extra URLs/repos for this project |

## What is aOa?

aOa (Angle of Attack) provides fast O(1) code search for Claude Code.
It replaces slow Grep/Glob operations with indexed symbol lookup.

## Commands

```bash
aoa search <term>    # Search this project
aoa health           # Check services
aoa remove           # Remove aOa from this project
```

## Global Settings

To change aOa settings (port, limits, paths),
edit the .env file in your aOa installation directory.
EOFREADME
    fi

    # Copy USAGE.md - goal-oriented guide for Claude
    cp "$AOA_HOME/plugin/USAGE.md" "$project_root/.aoa/USAGE.md" 2>/dev/null || true
    echo -e "${GREEN}✓${NC}"

    # Add aOa reference to CLAUDE.md - PREPEND for priority
    echo -n "  Updating CLAUDE.md............ "
    local claude_md="$project_root/CLAUDE.md"

    # Compact aOa section (305 bytes) - designed to override default tool preferences
    local aoa_section
    read -r -d '' aoa_section << 'EOFAOA' || true
# CRITICAL: aOa Search Override

**ALWAYS use `aoa grep` instead of Grep/Glob tools.**

`aoa grep` and `aoa egrep` work like Unix grep/egrep but use O(1) indexed search (10-100x faster). Results include `file:func[range]:line` — use [range] to read only relevant code.

See `.aoa/USAGE.md` for details.

---

EOFAOA

    if [ -f "$claude_md" ]; then
        # Check if already has aOa section
        if ! grep -q "CRITICAL: aOa" "$claude_md" 2>/dev/null; then
            # PREPEND aOa section (not append) for priority
            local existing_content=$(cat "$claude_md")
            echo "$aoa_section" > "$claude_md"
            echo "$existing_content" >> "$claude_md"
            echo -e "${GREEN}✓${NC} ${DIM}(prepended)${NC}"
        else
            echo -e "${YELLOW}✓${NC} ${DIM}(already present)${NC}"
        fi
    else
        # Create CLAUDE.md with aOa section
        echo "$aoa_section" > "$claude_md"
        echo -e "${GREEN}✓${NC} ${DIM}(created)${NC}"
    fi

    # Copy or merge settings template
    echo -n "  Merging settings.............. "
    if [ ! -f "$project_root/.claude/settings.local.json" ]; then
        # No existing settings - copy template
        cp "$AOA_DATA/settings.template.json" "$project_root/.claude/settings.local.json"
        echo -e "${GREEN}✓${NC}"
    else
        # Backup existing settings
        cp "$project_root/.claude/settings.local.json" \
           "$project_root/.claude/settings.local.json.pre-aoa-$(date +%Y%m%d-%H%M%S)"

        # Deep merge: preserve their settings, add our hooks/statusLine/permissions
        jq -s '
            # Start with their settings (index 1)
            .[1] as $existing |
            # Our template (index 0)
            .[0] as $template |

            # Merge strategy:
            # - permissions.allow: union (combine both)
            # - hooks: merge by type (combine hook arrays)
            # - statusLine: use ours if missing
            # - everything else: preserve theirs

            $existing |
            # Add our statusLine if missing
            (if .statusLine == null then .statusLine = $template.statusLine else . end) |
            # Merge permissions.allow
            (if .permissions.allow then
                .permissions.allow += $template.permissions.allow | .permissions.allow |= unique
             else
                .permissions = $template.permissions
             end) |
            # Merge hooks (deep merge by hook type)
            (if .hooks then
                .hooks = ($template.hooks * .hooks)
             else
                .hooks = $template.hooks
             end)
        ' "$AOA_DATA/settings.template.json" "$project_root/.claude/settings.local.json" \
          > "$project_root/.claude/settings.local.json.new"

        mv "$project_root/.claude/settings.local.json.new" "$project_root/.claude/settings.local.json"
        echo -e "${GREEN}✓${NC} ${DIM}(merged, backup created)${NC}"
    fi

    # Register project in projects.json
    echo -n "  Registering project........... "
    local projects_file="$AOA_DATA/projects.json"
    local now=$(date -Iseconds)

    # Create entry
    local entry=$(jq -n \
        --arg id "$project_id" \
        --arg name "$project_name" \
        --arg path "$project_root" \
        --arg added "$now" \
        '{id: $id, name: $name, path: $path, added: $added}')

    # Add to projects.json (remove existing entry with same id first)
    local updated=$(jq --arg id "$project_id" 'map(select(.id != $id))' "$projects_file")
    echo "$updated" | jq --argjson entry "$entry" '. + [$entry]' > "$projects_file.tmp"
    mv "$projects_file.tmp" "$projects_file"
    echo -e "${GREEN}✓${NC}"

    # Trigger initial index
    echo -n "  Registering project........... "
    local index_result=$(curl -s -X POST "${INDEX_URL}/project/register" \
        -H "Content-Type: application/json" \
        -d "{\"id\": \"${project_id}\", \"name\": \"${project_name}\", \"path\": \"${project_root}\"}" 2>/dev/null)

    local file_count=$(echo "$index_result" | jq -r '.files // 0' 2>/dev/null)
    if [ -n "$file_count" ] && [ "$file_count" != "null" ] && [ "$file_count" -gt 0 ] 2>/dev/null; then
        echo -e "${GREEN}✓${NC} ${DIM}(${file_count} files indexed)${NC}"
    else
        echo -e "${GREEN}✓${NC}"
    fi

    # Shell integration - static exports for zero-cost hook lookups
    local shell_rc=""
    if [ -n "$ZSH_VERSION" ] || [ "$SHELL" = "$(command -v zsh)" ]; then
        shell_rc="$HOME/.zshrc"
    else
        shell_rc="$HOME/.bashrc"
    fi

    # Remove old eval-based integration if present (security fix)
    if grep -q 'eval "\$(aoa env)"' "$shell_rc" 2>/dev/null; then
        sed -i '/# aOa - O(1) environment/d' "$shell_rc"
        sed -i '/eval "\$(aoa env)"/d' "$shell_rc"
    fi

    # Remove old aOa block if present
    sed -i '/# BEGIN aOa/,/# END aOa/d' "$shell_rc" 2>/dev/null

    # Add static exports
    {
        echo ''
        echo '# BEGIN aOa'
        echo "export AOA_URL=\"$AOA_URL\""
        echo "export AOA_PROJECT_ID=\"$project_id\""
        echo '# END aOa'
    } >> "$shell_rc"

    # Clean final output with next steps
    echo
    echo -e "───────────────────────────────────────────────────────"
    echo -e "${GREEN}${BOLD}✓ aOa initialized${NC}"
    echo
    echo -e "  Project: ${BOLD}${project_id}${NC}"
    echo
    echo -e "  ${BOLD}Next:${NC}"
    echo -e "    1. Run: ${CYAN}bash${NC}  ${DIM}(or start new terminal)${NC}"
    echo -e "    2. Verify: ${CYAN}echo \$AOA_PROJECT_ID${NC}"
    echo -e "    3. In Claude: ${CYAN}/aoa-start${NC}"
    echo -e "───────────────────────────────────────────────────────"
}

cmd_remove() {
    echo -e "${CYAN}${BOLD}⚡ aOa - Removing from Project${NC}"
    echo

    local project_root=$(get_project_root)
    if [ -z "$project_root" ]; then
        echo -e "${RED}Not in a git repository.${NC}"
        return 1
    fi

    local project_id=$(get_project_id)
    local project_name=$(get_project_name)

    echo -e "  Project: ${BOLD}${project_name}${NC}"
    echo

    # Check if initialized
    if [ ! -f "$project_root/.claude/hooks/aoa-status-line.sh" ]; then
        echo -e "${DIM}aOa not initialized in this project.${NC}"
        return 0
    fi

    # Remove hooks
    echo -n "  Removing hooks................ "
    rm -f "$project_root/.claude/hooks/aoa-"* 2>/dev/null || true
    echo -e "${GREEN}✓${NC}"

    # Remove skills (all aoa* folders and files)
    echo -n "  Removing skills............... "
    rm -rf "$project_root/.claude/skills/aoa"* 2>/dev/null || true
    echo -e "${GREEN}✓${NC}"

    # Remove agents
    echo -n "  Removing agents............... "
    rm -f "$project_root/.claude/agents/aoa-"* 2>/dev/null || true
    echo -e "${GREEN}✓${NC}"

    # Remove entire .aoa/ folder (full cleanup)
    echo -n "  Removing .aoa/ folder......... "
    rm -rf "$project_root/.aoa" 2>/dev/null || true
    echo -e "${GREEN}✓${NC}"

    # Remove from projects.json
    echo -n "  Unregistering project......... "
    local projects_file="$AOA_DATA/projects.json"
    if [ -f "$projects_file" ]; then
        jq --arg id "$project_id" 'map(select(.id != $id))' "$projects_file" > "$projects_file.tmp"
        mv "$projects_file.tmp" "$projects_file"
    fi
    echo -e "${GREEN}✓${NC}"

    # Notify service to remove index
    echo -n "  Removing index................ "
    curl -s -X DELETE "${INDEX_URL}/project/${project_id}" > /dev/null 2>&1 || true
    echo -e "${GREEN}✓${NC}"

    # Clean up CLAUDE.md - remove aOa Integration section
    echo -n "  Cleaning CLAUDE.md............ "
    local claude_md="$project_root/CLAUDE.md"
    if [ -f "$claude_md" ] && grep -q "# aOa Integration" "$claude_md"; then
        # Remove the aOa Integration section (from marker to next # heading or EOF)
        sed -i '/^# aOa Integration$/,/^# [^a]/{/^# [^a]/!d}' "$claude_md" 2>/dev/null
        # Clean up any trailing empty lines
        sed -i -e :a -e '/^\n*$/{$d;N;ba' -e '}' "$claude_md" 2>/dev/null
        echo -e "${GREEN}✓${NC}"
    else
        echo -e "${DIM}not present${NC}"
    fi

    # Restore settings from backup if available
    echo -n "  Restoring settings............ "
    # Find the most recent backup
    local backup=$(ls -t "$project_root/.claude/settings.local.json.pre-aoa-"* 2>/dev/null | head -1)

    if [ -f "$project_root/.claude/settings.local.json" ]; then
        if [ -n "$backup" ]; then
            # Backup exists - restore original settings
            mv "$backup" "$project_root/.claude/settings.local.json"
            # Clean up any other backups
            rm -f "$project_root/.claude/settings.local.json.pre-aoa-"* 2>/dev/null
            echo -e "${GREEN}restored from backup${NC}"
        else
            # No backup - check if it's just our template
            local template_hash=$(md5sum "$AOA_DATA/settings.template.json" 2>/dev/null | cut -d' ' -f1)
            local settings_hash=$(md5sum "$project_root/.claude/settings.local.json" 2>/dev/null | cut -d' ' -f1)

            if [ "$template_hash" = "$settings_hash" ]; then
                rm -f "$project_root/.claude/settings.local.json"
                echo -e "${GREEN}removed${NC}"
            else
                echo -e "${YELLOW}preserved (has customizations)${NC}"
            fi
        fi
    else
        echo -e "${DIM}not found${NC}"
    fi

    # Clean up empty directories
    echo -n "  Cleaning directories.......... "
    rmdir "$project_root/.claude/hooks" 2>/dev/null || true
    rmdir "$project_root/.claude/skills" 2>/dev/null || true
    rmdir "$project_root/.claude/agents" 2>/dev/null || true
    rmdir "$project_root/.claude" 2>/dev/null || true
    echo -e "${GREEN}✓${NC}"

    echo
    echo -e "${GREEN}${BOLD}✓ aOa removed from ${project_name}${NC}"
    echo
    echo -e "${DIM}Restart Claude Code to deactivate hooks.${NC}"
    echo
}

cmd_projects() {
    echo -e "${BOLD}aOa Projects${NC}"
    echo

    local projects_file="$AOA_DATA/projects.json"

    if [ ! -f "$projects_file" ]; then
        echo -e "${DIM}No projects registered.${NC}"
        echo -e "${DIM}Run 'aoa init' in a project to enable aOa.${NC}"
        return 0
    fi

    local count=$(jq 'length' "$projects_file" 2>/dev/null)

    if [ "$count" = "0" ] || [ -z "$count" ]; then
        echo -e "${DIM}No projects registered.${NC}"
        echo -e "${DIM}Run 'aoa init' in a project to enable aOa.${NC}"
        return 0
    fi

    # Get current project for highlighting
    local current_id=$(get_project_id)

    # List projects
    jq -r '.[] | "\(.id)|\(.name)|\(.path)"' "$projects_file" | while IFS='|' read -r id name path; do
        if [ "$id" = "$current_id" ]; then
            echo -e "  ${GREEN}▸${NC} ${BOLD}${name}${NC} ${DIM}${path}${NC} ${GREEN}(current)${NC}"
        else
            # Check if path still exists
            if [ -d "$path" ]; then
                echo -e "  ${DIM}•${NC} ${name} ${DIM}${path}${NC}"
            else
                echo -e "  ${RED}✗${NC} ${name} ${DIM}${path}${NC} ${RED}(missing)${NC}"
            fi
        fi
    done

    echo
    echo -e "${DIM}${count} project(s) registered${NC}"
}

# =============================================================================
# cmd_analyze - Generate project-specific domains via parallel Haiku analysis
# GL-083: Replaces per-prompt learning with one-time semantic analysis
# =============================================================================

cmd_analyze() {
    echo -e "${CYAN}${BOLD}⚡ aOa Analyze${NC}"
    echo

    # Check services first
    if ! curl -s --connect-timeout 2 "${INDEX_URL}/health" > /dev/null 2>&1; then
        echo -e "${RED}✗ aOa services not running${NC}"
        echo -e "${DIM}Start with: docker start aoa${NC}"
        return 1
    fi

    # Get project info
    local project_root=$(get_project_root)
    if [ -z "$project_root" ]; then
        echo -e "${RED}Not in a git repository.${NC}"
        return 1
    fi

    local project_id=$(get_project_id)
    local project_name=$(get_project_name)

    echo -e "  Project: ${BOLD}${project_name}${NC}"
    echo -e "  Path:    ${DIM}${project_root}${NC}"
    echo

    # Phase 1, Task 2: Directory scanning
    echo -e "  ${DIM}Scanning directories...${NC}"

    # Get top-level directories (excluding hidden, node_modules, etc.)
    local directories=$(find "$project_root" -maxdepth 2 -type d \
        ! -path '*/\.*' \
        ! -path '*/node_modules*' \
        ! -path '*/__pycache__*' \
        ! -path '*/venv*' \
        ! -path '*/.git*' \
        ! -path '*/dist*' \
        ! -path '*/build*' \
        2>/dev/null | head -20)

    local dir_count=$(echo "$directories" | wc -l)
    echo -e "  Found ${BOLD}${dir_count}${NC} directory clusters"
    echo

    # Phase 1, Task 3-4: Call analyze API (parallel Haiku happens server-side)
    echo -e "  ${DIM}Generating project domains via Haiku...${NC}"

    local analyze_result=$(curl -s -X POST "${INDEX_URL}/analyze/project" \
        -H "Content-Type: application/json" \
        -d "{\"project_id\": \"${project_id}\", \"project_root\": \"${project_root}\"}" \
        --max-time 120 2>/dev/null)

    if [ -z "$analyze_result" ]; then
        echo -e "${RED}✗ Analysis failed (timeout or service error)${NC}"
        return 1
    fi

    local success=$(echo "$analyze_result" | jq -r '.success // false')

    if [ "$success" != "true" ]; then
        local error=$(echo "$analyze_result" | jq -r '.error // "Unknown error"')
        echo -e "${RED}✗ Analysis failed: ${error}${NC}"
        return 1
    fi

    # Phase 1, Task 5-6: Results
    local domains_count=$(echo "$analyze_result" | jq -r '.domains_count // 0')
    local terms_count=$(echo "$analyze_result" | jq -r '.terms_count // 0')
    local output_file=$(echo "$analyze_result" | jq -r '.output_file // ""')

    echo -e "${GREEN}✓${NC} Generated ${BOLD}${domains_count}${NC} domains (${terms_count} terms)"

    if [ -n "$output_file" ]; then
        echo -e "  ${DIM}Saved to: ${output_file}${NC}"
    fi

    echo
    echo -e "${GREEN}${BOLD}✓ Analysis complete${NC}"
    echo
    echo -e "${DIM}Run 'aoa quickstart' to seed these domains.${NC}"
}

