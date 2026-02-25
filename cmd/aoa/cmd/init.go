package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Index the current project",
	Long:  "Scans all code files, extracts symbols with tree-sitter, and builds the search index.",
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	dbPath := filepath.Join(root, ".aoa", "aoa.db")
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
		runReconIfEnabled(root, dbPath)
		createShims()
		return nil
	}

	// No daemon — do it directly.

	// Ensure .aoa directory exists
	if err := os.MkdirAll(filepath.Join(root, ".aoa"), 0755); err != nil {
		return fmt.Errorf("create .aoa dir: %w", err)
	}

	store, err := bbolt.NewStore(dbPath)
	if err != nil {
		if isDBLockError(err) {
			return fmt.Errorf("cannot init: %s", diagnoseDBLock(root))
		}
		return fmt.Errorf("open database: %w", err)
	}

	parser := newParser()

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

	// Close store before recon to release bbolt single-writer lock.
	store.Close()

	fmt.Printf("⚡ aOa indexed %d files, %d symbols, %d tokens\n",
		stats.FileCount, stats.SymbolCount, stats.TokenCount)
	runReconIfEnabled(root, dbPath)
	createShims()
	return nil
}

// runReconIfEnabled invokes aoa-recon enhance only if recon has been explicitly
// enabled via `aoa recon init` (which writes .aoa/recon.enabled).
func runReconIfEnabled(root, dbPath string) {
	enabledPath := filepath.Join(root, ".aoa", "recon.enabled")
	if _, err := os.Stat(enabledPath); err != nil {
		fmt.Println("  Tip: run 'aoa recon init' to enable structural analysis")
		return
	}

	reconPath, err := exec.LookPath("aoa-recon")
	if err != nil {
		fmt.Println("  warning: recon enabled but aoa-recon not found on PATH")
		return
	}

	fmt.Println("⚡ Enhancing with aoa-recon...")
	cmd := exec.Command(reconPath, "enhance", "--db", dbPath, "--root", root)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: aoa-recon enhance failed: %v\n", err)
	}
}

// createShims writes grep and egrep shim scripts to ~/.aoa/shims/.
// Each shim execs the corresponding aoa subcommand, transparently replacing
// system binaries when ~/.aoa/shims is prepended to PATH.
func createShims() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not determine home directory: %v\n", err)
		return
	}

	shimDir := filepath.Join(home, ".aoa", "shims")
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
		fmt.Printf("\n⚡ aOa shims created in ~/.aoa/shims/\n\n")
		fmt.Printf("  To activate for AI tools, add to ~/.bashrc or ~/.zshrc:\n\n")
		fmt.Printf("    alias claude='PATH=\"$HOME/.aoa/shims:$PATH\" claude'\n")
		fmt.Printf("    alias gemini='PATH=\"$HOME/.aoa/shims:$PATH\" gemini'\n")
	}
}
