package cmd

import (
	"fmt"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check daemon status",
	RunE:  runHealth,
}

func runHealth(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if !client.Ping() {
		fmt.Println("⚡ aOa daemon is not running")
		return nil
	}

	health, err := client.Health()
	if err != nil {
		return err
	}

	fmt.Print(formatHealth(health))
	return nil
}
