package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/corey/aoa-go/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show configuration",
	Long:  "Shows project root, DB path, socket path, and daemon status. No daemon required.",
	RunE:  runConfig,
}

func runConfig(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	dbPath := filepath.Join(root, ".aoa", "aoa.db")
	projectID := filepath.Base(root)

	client := socket.NewClient(sockPath)
	daemonStatus := fmt.Sprintf("%s✗ not running%s", colorYellow, colorReset)
	if client.Ping() {
		daemonStatus = fmt.Sprintf("%s✓ running%s", colorGreen, colorReset)
	}

	fmt.Printf("%s⚡ aOa-go config%s\n", colorBold, colorReset)
	fmt.Printf("  Project:    %s\n", projectID)
	fmt.Printf("  Root:       %s\n", root)
	fmt.Printf("  DB:         %s\n", dbPath)
	fmt.Printf("  Socket:     %s\n", sockPath)
	fmt.Printf("  Daemon:     %s\n", daemonStatus)
	return nil
}
