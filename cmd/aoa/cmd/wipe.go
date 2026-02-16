package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var wipeForce bool

var wipeCmd = &cobra.Command{
	Use:   "wipe",
	Short: "Clear all aOa data for project",
	Long:  "Deletes all persisted index and learner state. Works with or without daemon.",
	RunE:  runWipe,
}

func init() {
	wipeCmd.Flags().BoolVar(&wipeForce, "force", false, "Skip confirmation prompt")
}

func runWipe(cmd *cobra.Command, args []string) error {
	root := projectRoot()

	if !wipeForce {
		fmt.Printf("⚠ This will delete all aOa data for %s. Continue? [y/N] ", filepath.Base(root))
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

	// If daemon is running, wipe via socket
	if client.Ping() {
		if err := client.Wipe(); err != nil {
			return err
		}
		fmt.Println("⚡ project data wiped (daemon)")
		return nil
	}

	// Daemon not running — wipe bbolt directly
	dbPath := filepath.Join(root, ".aoa", "aoa.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("⚡ no data to wipe")
		return nil
	}

	store, err := bbolt.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}

	projectID := filepath.Base(root)
	if err := store.DeleteProject(projectID); err != nil {
		store.Close()
		return err
	}
	store.Close()

	fmt.Println("⚡ project data wiped")
	return nil
}
