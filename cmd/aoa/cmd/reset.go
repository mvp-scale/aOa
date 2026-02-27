package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/spf13/cobra"
)

var resetForce bool

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Clear index and learner data for this project",
	Long:  "Resets the search index and learned state to blank. The .aoa/ directory and daemon remain intact. Use 'aoa remove' to fully uninstall from a project.",
	RunE:  runReset,
}

func init() {
	resetCmd.Flags().BoolVar(&resetForce, "force", false, "Skip confirmation prompt")
}

func runReset(cmd *cobra.Command, args []string) error {
	root := projectRoot()

	if !resetForce {
		fmt.Printf("This will clear all index and learner data for %s. Continue? [y/N] ", filepath.Base(root))
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("cancelled")
			return nil
		}
	}

	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	// If daemon is running, reset via socket — the daemon holds the lock.
	if client.Ping() {
		if err := client.Wipe(); err != nil {
			return fmt.Errorf("reset via daemon failed: %w", err)
		}
		fmt.Println("project data reset (daemon)")
		return nil
	}

	// Daemon not running — reset bbolt directly.
	dbPath := app.NewPaths(root).DB
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("no data to reset")
		return nil
	}

	store, err := bbolt.NewStore(dbPath)
	if err != nil {
		if isDBLockError(err) {
			return fmt.Errorf("cannot reset: %s", diagnoseDBLock(root))
		}
		return fmt.Errorf("open database: %w", err)
	}

	projectID := filepath.Base(root)
	if err := store.DeleteProject(projectID); err != nil {
		store.Close()
		return err
	}
	store.Close()

	fmt.Println("project data reset")
	return nil
}
