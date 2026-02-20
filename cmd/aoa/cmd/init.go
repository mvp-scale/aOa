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
	defer store.Close()

	parser := newParser()

	fmt.Println("⚡ Scanning project...")

	idx, stats, err := app.BuildIndex(root, parser)
	if err != nil {
		return fmt.Errorf("build index: %w", err)
	}

	if err := store.SaveIndex(projectID, idx); err != nil {
		return fmt.Errorf("save index: %w", err)
	}

	fmt.Printf("⚡ aOa indexed %d files, %d symbols, %d tokens\n",
		stats.FileCount, stats.SymbolCount, stats.TokenCount)
	return nil
}
