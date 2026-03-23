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

	summary := initSummary{}

	// Step 1-3: Global install (non-fatal — falls back to per-project behavior).
	binPath, installed, err := selfInstall()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not self-install binary: %v\n", err)
	} else if binPath != "" {
		summary.binaryPath = binPath
		summary.binaryInstalled = installed
		summary.globalShimsOK = createGlobalShims(binPath)
		rcFile, rcModified, rcErr := configureShellRC()
		if rcErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not configure shell: %v\n", rcErr)
		} else {
			summary.rcFile = rcFile
			summary.rcModified = rcModified
		}
	}

	// If daemon is running, delegate reindex via socket (avoids bbolt lock contention).
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)
	if client.Ping() {
		result, err := client.Reindex()
		if err != nil {
			return fmt.Errorf("reindex via daemon: %w", err)
		}
		summary.files = result.FileCount
		summary.symbols = result.SymbolCount
		summary.tokens = result.TokenCount
		summary.shimsOK = createShims(root)
		summary.statusOK = configureStatusLine(root)
		seedStatusFile(paths)
		appendClaudeMDGuidance(root)

		// Read dashboard URL from running daemon.
		if portData, err := os.ReadFile(paths.PortFile); err == nil {
			summary.dashURL = "http://localhost:" + strings.TrimSpace(string(portData))
		}
		summary.daemonOK = true

		printInitSummary(summary)
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

	fmt.Println("  Indexing project (typically under a minute)...")
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

	summary.files = stats.FileCount
	summary.symbols = stats.SymbolCount
	summary.tokens = stats.TokenCount
	summary.shimsOK = createShims(root)
	summary.statusOK = configureStatusLine(root)
	seedStatusFile(paths)
	appendClaudeMDGuidance(root)

	// Auto-start daemon so grep, dashboard, and tailing work immediately.
	if !client.Ping() {
		dashURL, err := spawnDaemon(root, sockPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not auto-start daemon: %v\n", err)
			fmt.Fprintf(os.Stderr, "  → start manually: aoa daemon start\n")
		} else {
			summary.daemonOK = true
			summary.dashURL = dashURL
		}
	}

	printInitSummary(summary)
	return nil
}

// initSummary holds all results from init for the summary output.
type initSummary struct {
	files, symbols, tokens int

	// Global install results
	binaryPath      string
	binaryInstalled bool
	globalShimsOK   bool
	rcFile          string
	rcModified      bool

	// Per-project results
	shimsOK  bool
	statusOK bool
	daemonOK bool
	dashURL  string
}

// printInitSummary prints the cohesive checklist output for aoa init.
func printInitSummary(s initSummary) {
	fmt.Println("⚡ aOa initialized")
	fmt.Println()

	// Index stats
	fmt.Printf("  ✓ indexed %s files, %s symbols, %s tokens\n",
		commaInt(s.files), commaInt(s.symbols), commaInt(s.tokens))

	// Global install results
	if s.binaryInstalled {
		fmt.Printf("  ✓ installed to %s\n", abbreviateHome(s.binaryPath))
	} else if s.binaryPath != "" {
		fmt.Printf("  ✓ binary up to date (%s)\n", abbreviateHome(s.binaryPath))
	}
	if s.globalShimsOK {
		fmt.Println("  ✓ global shims installed (grep, egrep)")
	}
	if s.rcFile != "" {
		if s.rcModified {
			fmt.Printf("  ✓ shell configured (%s)\n", abbreviateHome(s.rcFile))
		} else {
			fmt.Printf("  ✓ shell already configured (%s)\n", abbreviateHome(s.rcFile))
		}
	}

	// Per-project results
	if s.statusOK {
		fmt.Println("  ✓ status line configured")
	}
	if s.daemonOK {
		if s.dashURL != "" {
			fmt.Printf("  ✓ daemon started — %s\n", s.dashURL)
		} else {
			fmt.Println("  ✓ daemon started")
		}
	}

	// Shell activation hint
	if s.rcModified && s.rcFile != "" {
		fmt.Println()
		fmt.Printf("  Restart your shell or run: source %s\n", abbreviateHome(s.rcFile))
	} else if s.rcFile == "" && s.binaryPath == "" {
		// Fallback: no global install succeeded — show manual instructions
		home, _ := os.UserHomeDir()
		shimDir := filepath.Join(home, ".local", "share", "aoa", "shims")
		fmt.Println()
		fmt.Println("  To activate, add to ~/.bashrc or ~/.zshrc:")
		fmt.Println()
		fmt.Printf("    export PATH=\"$HOME/.local/bin:$PATH\"\n")
		fmt.Printf("    alias claude='PATH=\"%s:$PATH\" claude'\n", shimDir)
		fmt.Printf("    alias gemini='PATH=\"%s:$PATH\" gemini'\n", shimDir)
	}

	// Branded sign-off (yellow)
	fmt.Println()
	fmt.Println("  \033[93maOa learns. You build faster.\033[0m")
}

// abbreviateHome replaces the user's home directory prefix with ~.
func abbreviateHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
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

	// Prefer the stable self-installed binary path (~/.local/bin/aoa) over
	// the running binary (which may be in an ephemeral npx cache).
	aoaBin := selfInstalledBinaryPath()
	if aoaBin == "" {
		var err error
		aoaBin, err = os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not resolve aoa binary path: %v\n", err)
			return false
		}
		aoaBin, err = filepath.EvalSymlinks(aoaBin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not resolve aoa binary symlink: %v\n", err)
			return false
		}
	}

	shims := map[string]string{
		"grep":  fmt.Sprintf("#!/usr/bin/env bash\nexport AOA_SHIM=1\nexec %q grep \"$@\"\n", aoaBin),
		"egrep": fmt.Sprintf("#!/usr/bin/env bash\nexport AOA_SHIM=1\nexec %q egrep \"$@\"\n", aoaBin),
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
const defaultStatusLineConf = `# aOa Status Line — uncomment to show, order = display order.
# ⚡ aOa <traffic light> is always shown. Segments separated by │.

# ─── active ─────────────────────────────────────────────────────
lifetime_saved
lifetime_time_saved
rate_5h_combo
rate_7d_combo
context
model

# ─── quick add (uncomment to enable) ───────────────────────────
#cost
#burn_rate
#session_time
#autotune
#tokens_saved
#read_count
#lines_changed
#turn_count

# ─── available ──────────────────────────────────────────────────
#
#   Lifetime (across all sessions)
#     lifetime_saved       ↓15.0M           Total tokens saved
#     lifetime_time_saved  5h20m            Total time saved
#     lifetime_sessions    11               Sessions tracked
#
#   Session (current session)
#     tokens_saved         ↓93k             Tokens saved this session
#     time_saved_range     2m               Time saved this session
#     cost_saved           $0.42            Est. dollars saved
#     burn_rate            🔥1.5k/min       Context burn rate
#     guided_ratio         70%              Guided read percentage
#     shadow_saved         ↓5k              Shadow engine savings
#     cache_hit_rate       85%              Prompt cache hit rate
#     cache_saved          $1.20            Cache read dollar savings
#     read_count           8/15             Guided/total reads
#     autotune             23/50            Autotune progress
#
#   Intel (learning engine)
#     domains              58               Active domain count
#     mastered             4                Core domains
#     observed             284              Files learned from
#     vocabulary           1.2k             Keywords extracted
#     concepts             89               Terms resolved
#     patterns             256              Bigrams captured
#     evidence             42.5             Cumulative domain hits
#     learning_speed       0.8 d/prompt     Domains per prompt
#     signal_clarity       42%              Term-to-keyword resolution
#     conversion           12%              Domain-to-keyword conversion
#
#   Debrief (session analysis)
#     input_tokens         50k              Session input tokens
#     output_tokens        12k              Session output tokens
#     flow                 45.2 tok/s       Burst throughput
#     pace                 12.3/s           Conversation speed
#     turn_time            8s/turn          Average turn duration
#     turn_count           42               Exchange count
#     leverage             3.2 tools/turn   Tools per turn
#     amplification        4.5x             Output/input ratio
#     cost_per_exchange    $0.55/turn       Cost per turn
#
#   Runway (context projections)
#     runway               45m              Context runway estimate
#     delta_minutes        +12m             Extra runway from aOa
#
#   Session (from Claude Code)
#     session_time         12m35s           Wall-clock session duration
#     api_time             2m10s            Time waiting for API
#     session_tokens       50kin 12kout     Cumulative in/out tokens
#
#   Rate Limits (from Claude Code — Pro/Max only)
#     rate_5h              2h15m            5-hour countdown to reset
#     rate_5h_pct          42%              5-hour usage percentage
#     rate_5h_combo        2h15m 42%        Countdown + pct (pct grays out <80%)
#     rate_7d              3d12h            7-day countdown to reset
#     rate_7d_pct          23%              7-day usage percentage
#     rate_7d_combo        3d12h 23%        Countdown + pct (pct grays out <80%)
#
#   Claude Code
#     context              ctx:60k/200k     Context window usage
#     model                Opus 4.6         Active model
#     cost                 $23.05           Session cost
#     lines_changed        +772/-109L       Lines added/removed
`

// unconfigureStatusLine removes the aOa status line entry from
// .claude/settings.local.json. If the file becomes empty, restores backup
// or deletes it. Safe to call when .claude/ doesn't exist.
// Returns true if the status line was found and removed.
func unconfigureStatusLine(root string) bool {
	claudeDir := filepath.Join(root, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.local.json")
	backupPath := settingsPath + ".bak"

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false // file missing, nothing to undo
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return false // unparseable, don't touch it
	}

	// Only remove if we put it there.
	sl, ok := settings["statusLine"].(map[string]interface{})
	if !ok {
		return false
	}
	cmd, ok := sl["command"].(string)
	if !ok || !strings.Contains(cmd, "aoa-status-line.sh") {
		return false
	}

	delete(settings, "statusLine")

	if len(settings) == 0 {
		// Settings map is empty — restore backup or delete file.
		if _, err := os.Stat(backupPath); err == nil {
			os.Rename(backupPath, settingsPath)
		} else {
			os.Remove(settingsPath)
			// If .claude/ is now empty, remove it too.
			entries, err := os.ReadDir(claudeDir)
			if err == nil && len(entries) == 0 {
				os.Remove(claudeDir)
			}
		}
		return true
	}

	// Other keys remain — write back without statusLine.
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false
	}
	out = append(out, '\n')
	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		return false
	}
	// Clean up backup since we've preserved their other settings.
	os.Remove(backupPath)
	return true
}

// seedStatusFile writes a minimal status.json so the hook doesn't show
// "offline" on the very first prompt before the daemon writes real data.
func seedStatusFile(paths *app.Paths) {
	if _, err := os.Stat(paths.Status); err == nil {
		return // already exists, don't overwrite
	}
	_ = status.WriteJSON(paths.Status, &status.StatusData{})
}

// aOaGuidanceSentinel is the marker checked before appending guidance to CLAUDE.md.
// If this string exists in the file, guidance has already been added.
const aOaGuidanceSentinel = "<!-- aOa-guidance -->"

// aOaGuidance is the block appended to CLAUDE.md during init.
// Compact, one-time guidance that persists in the agent's context window.
// aOaGuidanceEnd is the closing sentinel for the guidance block.
const aOaGuidanceEnd = "<!-- /aOa-guidance -->"

const aOaGuidance = `
<!-- aOa-guidance -->
## Improved grep — use Bash grep/egrep for all code search

All code search MUST go through Bash ` + "`grep`" + `/` + "`egrep`" + `. This applies to you AND any subagents
you spawn. The built-in Grep/Glob tools bypass the semantic index and require 5-10x more calls.

### What grep returns

` + "```" + `
$ grep processTaintBaseEviction
  2dkfzw  pkg/controller/nodelifecycle:nc.processTaintBaseEviction(ctx)[979-1068]:979  @scheduling  #taint #eviction
  2dkg19  pkg/controller/nodelifecycle:nc.doEviction(ctx, taint)[1070-1142]:1070  @scheduling  #eviction #node
  --      pkg/controller/taint:tc.handlePodUpdate(old, cur)[245-312]:245  @scheduling  #taint #pod
` + "```" + `

Each line gives you: peek code, full signature, method boundaries ` + "`[start-end]`" + `, @domain, #terms.

### Workflow: grep → peek

` + "```" + `
$ grep processTaintBaseEviction          # find symbols
$ aoa peek 2dkfzw 2dkg19                 # read multiple method bodies in one call
` + "```" + `

- If peek returns "symbol not found", fall back to Read at the ` + "`[start-end]`" + ` lines from grep.
- ` + "`--`" + ` instead of a peek code means too large for peek — use Read at those lines.

### Commands

| Task | Command | Example |
|------|---------|---------|
| Find symbol | ` + "`grep name`" + ` | ` + "`grep reconcilePod`" + ` |
| Multi-symbol | ` + "`egrep 'A\\|B\\|C'`" + ` | ` + "`egrep 'health\\|ready\\|alive'`" + ` |
| Scoped search | ` + "`grep --scope path pat`" + ` | ` + "`grep --scope controller reconcile`" + ` |
| Find files | ` + "`aoa locate name`" + ` | ` + "`aoa locate webhook`" + ` |
| Glob files | ` + "`aoa find glob`" + ` | ` + "`aoa find *_types.go`" + ` |
| Read methods | ` + "`aoa peek code`" + ` | ` + "`aoa peek a1 b2 c3`" + ` |
| Directory tree | ` + "`aoa tree dir -d N`" + ` | ` + "`aoa tree pkg/api -d 2`" + ` |

**Use Read for**: non-code files (YAML, configs, go.mod), surrounding context (imports, constants),
or when peek shows ` + "`--`" + `.
<!-- /aOa-guidance -->
`

// appendClaudeMDGuidance writes aOa guidance to the top of CLAUDE.md.
// On upgrade: strips old guidance block and writes the new one.
// Creates the file if missing.
func appendClaudeMDGuidance(root string) {
	claudeMD := filepath.Join(root, "CLAUDE.md")

	existing, err := os.ReadFile(claudeMD)

	// Strip old guidance block if present (upgrade path).
	if err == nil && strings.Contains(string(existing), aOaGuidanceSentinel) {
		removeClaudeMDGuidance(root)
		existing, err = os.ReadFile(claudeMD)
	}

	// Prepend guidance to the top so the agent sees it first.
	var content string
	if err == nil {
		content = aOaGuidance + "\n" + string(existing)
	} else {
		content = aOaGuidance
	}

	if err := os.WriteFile(claudeMD, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write aOa guidance to CLAUDE.md: %v\n", err)
	}
}

// removeClaudeMDGuidance removes the aOa guidance block from CLAUDE.md.
// Only removes if the block matches exactly (sentinel-delimited).
// Returns true if guidance was found and removed, false otherwise.
func removeClaudeMDGuidance(root string) bool {
	claudeMD := filepath.Join(root, "CLAUDE.md")

	data, err := os.ReadFile(claudeMD)
	if err != nil {
		return false // no file, nothing to remove
	}

	content := string(data)
	startIdx := strings.Index(content, aOaGuidanceSentinel)
	if startIdx < 0 {
		return false // sentinel not found
	}

	endIdx := strings.Index(content, aOaGuidanceEnd)
	if endIdx < 0 {
		return false // no closing sentinel — modified by user, leave it
	}

	// Include the closing sentinel and any trailing newline
	endIdx += len(aOaGuidanceEnd)
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	// Trim leading newline before opening sentinel if present
	if startIdx > 0 && content[startIdx-1] == '\n' {
		startIdx--
	}

	cleaned := content[:startIdx] + content[endIdx:]

	// If CLAUDE.md is now empty (we created it), remove the file entirely
	if strings.TrimSpace(cleaned) == "" {
		os.Remove(claudeMD)
		return true
	}

	if err := os.WriteFile(claudeMD, []byte(cleaned), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update CLAUDE.md: %v\n", err)
		return false
	}
	return true
}

func writeDefaultStatusLineConf(root string) {
	confPath := filepath.Join(root, ".aoa", "status-line.conf")

	// Always overwrite — the conf is refreshed on upgrade so new segments
	// are available. Users can re-customize after init.
	if err := os.WriteFile(confPath, []byte(defaultStatusLineConf), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write status-line.conf: %v\n", err)
	}
}
