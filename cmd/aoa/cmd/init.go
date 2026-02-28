package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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

	// Auto-detect and download missing grammars before indexing.
	scanAndDownloadGrammars(root)

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
