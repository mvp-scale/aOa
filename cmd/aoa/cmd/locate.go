package cmd

import (
	"fmt"

	"github.com/corey/aoa-go/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var locateCmd = &cobra.Command{
	Use:   "locate <name>",
	Short: "Find files by name substring",
	Long:  "Substring filename search across the indexed codebase.",
	Args:  cobra.ExactArgs(1),
	RunE:  runLocate,
}

func runLocate(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if !client.Ping() {
		return fmt.Errorf("daemon not running. Start with: aoa-go daemon start")
	}

	result, err := client.Files("", args[0])
	if err != nil {
		return err
	}

	fmt.Print(formatFiles(result))
	return nil
}
