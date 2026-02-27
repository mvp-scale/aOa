package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/spf13/cobra"
)

var removeForce bool

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Completely remove aOa from this project",
	Long:  "Stops the daemon, deletes the .aoa/ directory and all data. Use 'aoa reset' to just clear data without removing.",
	RunE:  runRemove,
}

func init() {
	removeCmd.Flags().BoolVar(&removeForce, "force", false, "Skip confirmation prompt")
}

func runRemove(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	paths := app.NewPaths(root)
	aOaDir := paths.Root

	// Check if .aoa/ even exists.
	if _, err := os.Stat(aOaDir); os.IsNotExist(err) {
		fmt.Println("aOa is not initialized in this project")
		return nil
	}

	if !removeForce {
		fmt.Printf("This will stop the daemon and delete %s/ entirely. Continue? [y/N] ", aOaDir)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("cancelled")
			return nil
		}
	}

	// Stop the daemon if running.
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)
	if client.Ping() {
		fmt.Println("stopping daemon...")
		if err := client.Shutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not stop daemon gracefully: %v\n", err)
		}
		// Wait briefly for shutdown.
		for i := 0; i < 20; i++ {
			if !client.Ping() {
				break
			}
			// Small busy-wait (no time import needed, just loop).
		}
	}

	// Clean up socket file.
	os.Remove(sockPath)

	// Remove the entire .aoa/ directory.
	if err := os.RemoveAll(aOaDir); err != nil {
		return fmt.Errorf("remove %s: %w", aOaDir, err)
	}

	fmt.Printf("aOa removed from %s\n", filepath.Base(root))
	return nil
}
