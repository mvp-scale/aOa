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
	"github.com/corey/aoa/internal/domain/status"
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
	shimDir := filepath.Join(root, ".aoa", "shims")

	// If daemon is running, delegate reindex via socket (avoids bbolt lock contention).
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)
	if client.Ping() {
		result, err := client.Reindex()
		if err != nil {
			return fmt.Errorf("reindex via daemon: %w", err)
		}
		shimsOK := createShims(root)
		statusOK := configureStatusLine(root)
		seedStatusFile(paths)

		// Read dashboard URL from running daemon.
		dashURL := ""
		if portData, err := os.ReadFile(paths.PortFile); err == nil {
			dashURL = "http://localhost:" + strings.TrimSpace(string(portData))
		}

		printInitSummary(
			result.FileCount, result.SymbolCount, result.TokenCount,
			shimsOK, statusOK, true, dashURL, shimDir,
		)
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

	shimsOK := createShims(root)
	statusOK := configureStatusLine(root)
	seedStatusFile(paths)

	// Auto-start daemon so grep, dashboard, and tailing work immediately.
	daemonOK := false
	dashURL := ""
	if !client.Ping() {
		var err error
		dashURL, err = spawnDaemon(root, sockPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not auto-start daemon: %v\n", err)
			fmt.Fprintf(os.Stderr, "  → start manually: aoa daemon start\n")
		} else {
			daemonOK = true
		}
	}

	printInitSummary(
		stats.FileCount, stats.SymbolCount, stats.TokenCount,
		shimsOK, statusOK, daemonOK, dashURL, shimDir,
	)
	return nil
}

// printInitSummary prints the cohesive checklist output for aoa init.
func printInitSummary(files, symbols, tokens int, shimsOK, statusOK, daemonOK bool, dashURL, shimDir string) {
	fmt.Println("⚡ aOa initialized")
	fmt.Println()

	// Checklist
	fmt.Printf("  ✓ indexed %s files, %s symbols, %s tokens\n",
		commaInt(files), commaInt(symbols), commaInt(tokens))
	if shimsOK {
		fmt.Println("  ✓ shims installed (grep, egrep)")
	}
	if statusOK {
		fmt.Println("  ✓ status line configured")
	}
	if daemonOK {
		if dashURL != "" {
			fmt.Printf("  ✓ daemon started — %s\n", dashURL)
		} else {
			fmt.Println("  ✓ daemon started")
		}
	}

	// Activation block
	fmt.Println()
	fmt.Println("  To activate, add to ~/.bashrc or ~/.zshrc:")
	fmt.Println()
	fmt.Printf("    alias claude='PATH=\"%s:$PATH\" claude'\n", shimDir)
	fmt.Printf("    alias gemini='PATH=\"%s:$PATH\" gemini'\n", shimDir)
	fmt.Println()
	fmt.Println("  aOa intercepts grep and egrep with semantic search.")
	fmt.Println("  Results point to methods and functions — not just matching lines.")
	fmt.Println("  Claude navigates directly to what matters. No file scanning.")
	fmt.Println("  Self-learning, O(1) indexed, sub-ms — pure math, zero AI overhead.")

	// Branded sign-off (yellow)
	fmt.Println()
	fmt.Println("  \033[93maOa learns. You build faster.\033[0m")
}

// commaInt formats an integer with comma separators (e.g. 1563 -> "1,563").
func commaInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	// Insert commas from the right.
	var b strings.Builder
	start := len(s) % 3
	if start == 0 {
		start = 3
	}
	b.WriteString(s[:start])
	for i := start; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// createShims writes grep and egrep shim scripts to {root}/.aoa/shims/.
// Each shim execs the corresponding aoa subcommand, transparently replacing
// system binaries when .aoa/shims is prepended to PATH.
// Returns true if shims exist (whether freshly written or already present).
func createShims(root string) bool {
	shimDir := filepath.Join(root, ".aoa", "shims")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create shims directory: %v\n", err)
		return false
	}

	shims := map[string]string{
		"grep":  "#!/usr/bin/env bash\nexport AOA_SHIM=1\nexec aoa grep \"$@\"\n",
		"egrep": "#!/usr/bin/env bash\nexport AOA_SHIM=1\nexec aoa egrep \"$@\"\n",
	}

	ok := true
	for name, content := range shims {
		path := filepath.Join(shimDir, name)

		// Skip if shim already exists with identical content.
		existing, err := os.ReadFile(path)
		if err == nil && string(existing) == content {
			continue
		}

		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write shim %s: %v\n", name, err)
			ok = false
			continue
		}
	}

	return ok
}

// configureStatusLine deploys the embedded status line hook to .aoa/hooks/
// and auto-configures it in .claude/settings.local.json. Non-destructive:
// backs up settings before modifying, idempotent if already configured,
// and preserves all existing settings keys.
// Returns true if the status line is configured (whether freshly or already).
func configureStatusLine(root string) bool {
	// Deploy the embedded hook script to .aoa/hooks/.
	hookDir := filepath.Join(root, ".aoa", "hooks")
	hookPath := filepath.Join(hookDir, "aoa-status-line.sh")

	scriptData, err := hooks.FS.ReadFile("aoa-status-line.sh")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: embedded status line hook not found: %v\n", err)
		return false
	}

	if err := os.MkdirAll(hookDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create .aoa/hooks directory: %v\n", err)
		return false
	}

	// Write hook script (update if content changed, skip if identical).
	existing, readErr := os.ReadFile(hookPath)
	if readErr != nil || string(existing) != string(scriptData) {
		if err := os.WriteFile(hookPath, scriptData, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write hook script: %v\n", err)
			return false
		}
	}

	// Configure .claude/settings.local.json to use the deployed hook.
	// Match Claude Code's own layout: .claude/ at 0755, settings files at 0644.
	claudeDir := filepath.Join(root, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.local.json")

	// Ensure .claude/ directory exists first — before any read/write attempts.
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create .claude directory: %v\n", err)
		return false
	}

	// Read existing settings (or start fresh).
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not parse %s: %v\n", settingsPath, err)
			return false
		}
	} else if os.IsNotExist(err) {
		settings = make(map[string]interface{})
	} else {
		fmt.Fprintf(os.Stderr, "warning: could not read %s: %v\n", settingsPath, err)
		return false
	}

	// Always ensure status-line.conf exists (independent of settings.local.json).
	writeDefaultStatusLineConf(root)

	// Check if already configured.
	if sl, ok := settings["statusLine"].(map[string]interface{}); ok {
		if cmd, ok := sl["command"].(string); ok && strings.Contains(cmd, "aoa-status-line.sh") {
			return true
		}
	}

	// Backup existing file before modifying.
	if len(data) > 0 {
		backupPath := settingsPath + ".bak"
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not backup %s: %v\n", settingsPath, err)
			return false
		}
	}

	// Inject statusLine config pointing to the deployed hook.
	settings["statusLine"] = map[string]interface{}{
		"type":    "command",
		"command": `bash "$CLAUDE_PROJECT_DIR/.aoa/hooks/aoa-status-line.sh"`,
	}

	// Write back with indentation.
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not marshal settings: %v\n", err)
		return false
	}
	out = append(out, '\n')

	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write %s: %v\n", settingsPath, err)
		return false
	}
	return true
}

// defaultStatusLineConf is the template for .aoa/status-line.conf.
// Uncommented lines are the default segments shown on the status line.
// Users can comment/uncomment and reorder to customize.
const defaultStatusLineConf = `# aOa Status Line Configuration
# Edit and save — changes take effect on the next status line refresh.
#
# Layout:  ⚡ aOa <traffic light> │ <segments...>
#
# The left section is always shown:
#   ⚡ aOa    Clickable link to dashboard (when daemon is running)
#   ⚪/🟡/🟢   Learning maturity (input count: ⚪ <30, 🟡 30-99, 🟢 100+)
#
# Uncomment segments to show them. Order = display order.
# Segments are separated by │ on the status line.

# ─── active segments ─────────────────────────────────────────────
tokens_saved
time_saved_range
#burn_rate
#cost_saved
#cost
#lines_changed
context
model

# ─── all available segments ──────────────────────────────────────
#
#   Live (aOa session metrics)
#     tokens_saved        Tokens saved from guided reads         ↓93k
#     time_saved_range    Time saved estimate (low-high)         ⚡5m-52m saved
#     cost_saved          Est. dollars saved from guided reads   $0.42 saved
#     burn_rate           Context burn rate                      🔥1.5k/min
#     guided_ratio        Guided read percentage                 guided 70%
#     shadow_saved        Shadow engine savings                  shadow ↓5k
#     cache_hit_rate      Prompt cache hit rate                  cache 85%
#     cache_saved         Cache read dollar savings              cache $1.20
#     read_count          Guided/total reads                     8/15 reads
#     autotune            Autotune progress                      23/50
#
#   Intel (learning engine)
#     domains             Active domain count                    58 domains
#     mastered            Core domains (survived autotune)       4 mastered
#     observed            Files learned from                     284 observed
#     vocabulary          Keywords extracted                     1.2k keywords
#     concepts            Terms resolved                         89 concepts
#     patterns            Bigrams captured                       256 patterns
#     evidence            Cumulative domain hits                 42.5 evidence
#     learning_speed      Domains discovered per prompt          0.8 d/prompt
#     signal_clarity      Term-to-keyword resolution rate        signal 42%
#     conversion          Domain-to-keyword conversion rate      conv 12%
#
#   Debrief (session analysis)
#     input_tokens        Session input tokens                   in:50k
#     output_tokens       Session output tokens                  out:12k
#     flow                Burst throughput (all streams)         45.2 tok/s
#     pace                Visible conversation speed             pace 12.3/s
#     turn_time           Average turn duration                  turn 8s
#     turn_count          Exchange count                         42 turns
#     leverage            Tools invoked per turn                 3.2 tools/turn
#     amplification       Output/input character ratio           4.5x amp
#     cost_per_exchange   Cost per turn                          $0.55/turn
#
#   Runway (context window projections)
#     runway              Context runway estimate                runway 45m
#     delta_minutes       Extra runway gained from aOa           +12m
#
#   Claude Code (from Claude, not aOa)
#     context             Context window usage                   ctx:60k/200k (30%)
#     model               Active model name                      Opus 4.6
#     cost                Session cost                           $23.05
#     lines_changed       Lines added/removed                    +772/-109L
`

// unconfigureStatusLine removes the aOa status line entry from
// .claude/settings.local.json. If the file becomes empty, restores backup
// or deletes it. Safe to call when .claude/ doesn't exist.
func unconfigureStatusLine(root string) {
	claudeDir := filepath.Join(root, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	backupPath := settingsPath + ".bak"

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return // file missing, nothing to undo
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return // unparseable, don't touch it
	}

	// Only remove if we put it there.
	sl, ok := settings["statusLine"].(map[string]interface{})
	if !ok {
		return
	}
	cmd, ok := sl["command"].(string)
	if !ok || !strings.Contains(cmd, "aoa-status-line.sh") {
		return
	}

	delete(settings, "statusLine")

	if len(settings) == 0 {
		// Settings map is empty — restore backup or delete file.
		if _, err := os.Stat(backupPath); err == nil {
			if err := os.Rename(backupPath, settingsPath); err == nil {
				fmt.Println("restored .claude/settings.local.json from backup")
			}
		} else {
			os.Remove(settingsPath)
			// If .claude/ is now empty, remove it too.
			entries, err := os.ReadDir(claudeDir)
			if err == nil && len(entries) == 0 {
				os.Remove(claudeDir)
			}
			fmt.Println("removed .claude/settings.local.json")
		}
		return
	}

	// Other keys remain — write back without statusLine.
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return
	}
	out = append(out, '\n')
	if err := os.WriteFile(settingsPath, out, 0644); err == nil {
		fmt.Println("removed aOa status line from .claude/settings.local.json")
	}
	// Clean up backup since we've preserved their other settings.
	os.Remove(backupPath)
}

// seedStatusFile writes a minimal status.json so the hook doesn't show
// "offline" on the very first prompt before the daemon writes real data.
func seedStatusFile(paths *app.Paths) {
	if _, err := os.Stat(paths.Status); err == nil {
		return // already exists, don't overwrite
	}
	_ = status.WriteJSON(paths.Status, &status.StatusData{})
}

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
