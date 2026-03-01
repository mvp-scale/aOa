package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corey/aoa/hooks"
	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/spf13/cobra"
)

// noGrammarsFlag disables automatic grammar download during init.
var noGrammarsFlag bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Index the current project",
	Long:  "Scans all code files, extracts symbols with tree-sitter, and builds the search index.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&noGrammarsFlag, "no-grammars", false, "Skip automatic grammar download")
	initCmd.Flags().BoolVar(&updateFlag, "update", false, "Update parsers.json and regenerate grammar config")
}

func runInit(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	paths := app.NewPaths(root)
	dbPath := paths.DB
	projectID := filepath.Base(root)

	// If daemon is running, delegate reindex via socket (avoids bbolt lock contention).
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)
	if client.Ping() {
		fmt.Println("⚡ Daemon running — delegating reindex...")
		result, err := client.Reindex()
		if err != nil {
			return fmt.Errorf("reindex via daemon: %w", err)
		}
		fmt.Printf("⚡ aOa indexed %d files, %d symbols, %d tokens (%dms)\n",
			result.FileCount, result.SymbolCount, result.TokenCount, result.ElapsedMs)
		createShims(root)
		configureStatusLine(root)
		return nil
	}

	// No daemon — do it directly.

	// Ensure .aoa directory structure exists
	if err := paths.EnsureDirs(); err != nil {
		return fmt.Errorf("create .aoa dirs: %w", err)
	}

	store, err := bbolt.NewStore(dbPath)
	if err != nil {
		if isDBLockError(err) {
			return fmt.Errorf("cannot init: %s", diagnoseDBLock(root))
		}
		return fmt.Errorf("open database: %w", err)
	}

	parser := newParser(root)

	// Auto-detect and set up missing grammars before indexing.
	// Fetches parsers.json, downloads pre-built .so files, verifies SHA-256.
	// Returns true when download was declined or failed — halt init.
	if scanAndDownloadGrammars(root) {
		store.Close()
		return nil
	}

	fmt.Println("⚡ Scanning project...")

	idx, stats, err := app.BuildIndex(root, parser)
	if err != nil {
		store.Close()
		return fmt.Errorf("build index: %w", err)
	}

	if err := store.SaveIndex(projectID, idx); err != nil {
		store.Close()
		return fmt.Errorf("save index: %w", err)
	}

	store.Close()

	fmt.Printf("⚡ aOa indexed %d files, %d symbols, %d tokens\n",
		stats.FileCount, stats.SymbolCount, stats.TokenCount)
	createShims(root)
	configureStatusLine(root)

	// Auto-start daemon so grep, dashboard, and tailing work immediately.
	if !client.Ping() {
		fmt.Println()
		if err := spawnDaemon(root, sockPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not auto-start daemon: %v\n", err)
			fmt.Fprintf(os.Stderr, "  → start manually: aoa daemon start\n")
		}
	}
	return nil
}

// createShims writes grep and egrep shim scripts to {root}/.aoa/shims/.
// Each shim execs the corresponding aoa subcommand, transparently replacing
// system binaries when .aoa/shims is prepended to PATH.
func createShims(root string) {
	shimDir := filepath.Join(root, ".aoa", "shims")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create shims directory: %v\n", err)
		return
	}

	shims := map[string]string{
		"grep":  "#!/usr/bin/env bash\nexport AOA_SHIM=1\nexec aoa grep \"$@\"\n",
		"egrep": "#!/usr/bin/env bash\nexport AOA_SHIM=1\nexec aoa egrep \"$@\"\n",
	}

	wrote := false
	for name, content := range shims {
		path := filepath.Join(shimDir, name)

		// Skip if shim already exists with identical content.
		existing, err := os.ReadFile(path)
		if err == nil && string(existing) == content {
			continue
		}

		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write shim %s: %v\n", name, err)
			continue
		}
		wrote = true
	}

	if wrote {
		fmt.Printf("\n⚡ aOa shims created in .aoa/shims/\n\n")
		fmt.Printf("  To activate for AI tools, add to ~/.bashrc or ~/.zshrc:\n\n")
		fmt.Printf("    alias claude='PATH=\"%s:$PATH\" claude'\n", shimDir)
		fmt.Printf("    alias gemini='PATH=\"%s:$PATH\" gemini'\n", shimDir)
	}
}

// configureStatusLine deploys the embedded status line hook to .aoa/hooks/
// and auto-configures it in .claude/settings.local.json. Non-destructive:
// backs up settings before modifying, idempotent if already configured,
// and preserves all existing settings keys.
func configureStatusLine(root string) {
	// Deploy the embedded hook script to .aoa/hooks/.
	hookDir := filepath.Join(root, ".aoa", "hooks")
	hookPath := filepath.Join(hookDir, "aoa-status-line.sh")

	scriptData, err := hooks.FS.ReadFile("aoa-status-line.sh")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: embedded status line hook not found: %v\n", err)
		return
	}

	if err := os.MkdirAll(hookDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create .aoa/hooks directory: %v\n", err)
		return
	}

	// Write hook script (update if content changed, skip if identical).
	existing, readErr := os.ReadFile(hookPath)
	if readErr != nil || string(existing) != string(scriptData) {
		if err := os.WriteFile(hookPath, scriptData, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write hook script: %v\n", err)
			return
		}
	}

	// Configure .claude/settings.local.json to use the deployed hook.
	claudeDir := filepath.Join(root, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.local.json")

	// Read existing settings (or start fresh).
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not parse %s: %v\n", settingsPath, err)
			return
		}
	} else if os.IsNotExist(err) {
		settings = make(map[string]interface{})
	} else {
		fmt.Fprintf(os.Stderr, "warning: could not read %s: %v\n", settingsPath, err)
		return
	}

	// Always ensure status-line.conf exists (independent of settings.local.json).
	writeDefaultStatusLineConf(root)

	// Check if already configured.
	if sl, ok := settings["statusLine"].(map[string]interface{}); ok {
		if cmd, ok := sl["command"].(string); ok && strings.Contains(cmd, "aoa-status-line.sh") {
			fmt.Println("✓ status line already configured")
			return
		}
	}

	// Backup existing file before modifying.
	if len(data) > 0 {
		backupPath := settingsPath + ".bak"
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not backup %s: %v\n", settingsPath, err)
			return
		}
		fmt.Println("backed up .claude/settings.local.json → .claude/settings.local.json.bak")
	}

	// Inject statusLine config pointing to the deployed hook.
	settings["statusLine"] = map[string]interface{}{
		"type":    "command",
		"command": `bash "$CLAUDE_PROJECT_DIR/.aoa/hooks/aoa-status-line.sh"`,
	}

	// Ensure .claude/ directory exists.
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create .claude directory: %v\n", err)
		return
	}

	// Write back with indentation.
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not marshal settings: %v\n", err)
		return
	}
	out = append(out, '\n')

	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write %s: %v\n", settingsPath, err)
		return
	}
	fmt.Println("⚡ status line configured")
}

// defaultStatusLineConf is the template for .aoa/status-line.conf.
// Uncommented lines are the default segments shown on the status line.
// Users can comment/uncomment and reorder to customize.
const defaultStatusLineConf = `# aOa Status Line Configuration
# Uncomment segments to show them. Order = display order (left to right).
# Segments are separated by | on the status line.
# Edit and save — changes take effect on the next status line refresh.

# === Left (aOa header + traffic light) ===
intents

# === Middle (pick what matters to you) ===
tokens_saved
time_saved_range
#burn_rate
#cost
#lines_changed

# === Right ===
context
model
#dashboard

# -----------------------------------------------------------------
# All available segments (matches dashboard stat cards):
#
#   Live
#     tokens_saved        Tokens saved from guided reads         ↓93k
#     time_saved_range    Time saved estimate (low-high)         ⚡5m-52m saved
#     burn_rate           Context burn rate                      🔥1.5k/min
#     cost                Session cost                           $23.05
#     guided_ratio        Guided read percentage                 guided 70%
#     shadow_saved        Shadow engine savings                  shadow ↓5k
#     cache_hit_rate      Prompt cache hit rate                  cache 85%
#     read_count          Guided/total reads                     8/15 reads
#     autotune            Autotune progress                      23/50
#     lines_changed       Lines added/removed                    +772/-109L
#
#   Debrief
#     input_tokens        Session input tokens                   in:50k
#     output_tokens       Session output tokens                  out:12k
#     flow                Burst throughput                       45.2 tok/s
#
#   Intel
#     domains             Active domain count                    58 domains
#     mastered            Core domains (survived autotune)       4 mastered
#     observed            Files learned from                     284 observed
#     vocabulary          Keywords extracted                     1.2k keywords
#     concepts            Terms resolved                         89 concepts
#     patterns            Bigrams captured                       256 patterns
#     evidence            Cumulative domain hits                 42.5 evidence
#
#   Runway
#     runway              Context runway estimate                runway 45m
#     delta_minutes       Extra runway gained from aOa           +12m
#
#   Other
#     context             Context window usage                   ctx:60k/200k (30%)
#     model               Active model name                      Opus 4.6
#     dashboard           Clickable link to aOa dashboard        dashboard
# -----------------------------------------------------------------
`

func writeDefaultStatusLineConf(root string) {
	confPath := filepath.Join(root, ".aoa", "status-line.conf")

	// Don't overwrite user customizations.
	if _, err := os.Stat(confPath); err == nil {
		return
	}

	if err := os.WriteFile(confPath, []byte(defaultStatusLineConf), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write status-line.conf: %v\n", err)
	}
}
