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
var removeGlobal bool

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Completely remove aOa from this project",
	Long:  "Stops the daemon, deletes the .aoa/ directory and all data. Use 'aoa reset' to just clear data without removing.\nWith --global, also removes the global binary, shims, and shell configuration.",
	RunE:  runRemove,
}

func init() {
	removeCmd.Flags().BoolVar(&removeForce, "force", false, "Skip confirmation prompt")
	removeCmd.Flags().BoolVar(&removeGlobal, "global", false, "Also remove global binary, shims, and shell configuration")
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

	// Clean up status line config from .claude/settings.local.json.
	statusRemoved := unconfigureStatusLine(root)

	// Clean up guidance block from CLAUDE.md.
	guidanceRemoved := removeClaudeMDGuidance(root)

	// Remove the entire .aoa/ directory.
	if err := os.RemoveAll(aOaDir); err != nil {
		return fmt.Errorf("remove %s: %w", aOaDir, err)
	}

	fmt.Println("⚡ aOa removed")
	fmt.Println()
	fmt.Println("  ✓ .aoa/ directory deleted")
	if statusRemoved {
		fmt.Println("  ✓ status line hook removed from .claude/settings.local.json")
	}
	if guidanceRemoved {
		fmt.Println("  ✓ aOa guidance removed from CLAUDE.md")
	} else {
		claudeMD := filepath.Join(root, "CLAUDE.md")
		if data, err := os.ReadFile(claudeMD); err == nil && strings.Contains(string(data), "aOa") {
			fmt.Println("  ⚠ CLAUDE.md contains aOa references — review manually")
		}
	}

	// Global removal
	if removeGlobal {
		home, err := os.UserHomeDir()
		if err == nil {
			// Remove shell RC block
			if unconfigureShellRC() {
				fmt.Println("  ✓ shell configuration removed")
			}

			// Remove entire ~/.local/share/aoa/ (shims + versioned binaries)
			globalDataPath := filepath.Join(home, globalDataDir)
			if err := os.RemoveAll(globalDataPath); err == nil {
				fmt.Println("  ✓ global data removed (~/.local/share/aoa/)")
			}

			// Remove symlink at ~/.local/bin/aoa
			globalBin := filepath.Join(home, globalBinDir, "aoa")
			if _, err := os.Lstat(globalBin); err == nil {
				if err := os.Remove(globalBin); err == nil {
					fmt.Println("  ✓ global binary removed (~/.local/bin/aoa)")
				} else {
					fmt.Fprintf(os.Stderr, "  warning: could not remove %s: %v\n", globalBin, err)
				}
			}
		}
	}

	return nil
}
