package cmd

import (
	"fmt"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show session statistics",
	RunE:  runStats,
}

func runStats(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if !client.Ping() {
		return fmt.Errorf("daemon not running. Start with: aoa daemon start")
	}

	result, err := client.Stats()
	if err != nil {
		return err
	}

	fmt.Print(formatStats(result))
	return nil
}
