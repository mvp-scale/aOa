package cmd

import (
	"fmt"

	"github.com/corey/aoa/internal/adapters/socket"
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

	result, err := client.Files("", args[0])
	if err != nil && isConnectError(err) && reviveDaemon(root, sockPath) {
		// L21.1: lazy-start — revive once, retry once (D-U2a).
		result, err = client.Files("", args[0])
	}
	if err != nil {
		if !isConnectError(err) {
			return err
		}
		// Daemon not running — fall back to filesystem substring search.
		matches := fallbackLocateName(root, args[0])
		for _, m := range matches {
			fmt.Println(m)
		}
		return nil
	}

	fmt.Print(formatFiles(result))
	return nil
}
