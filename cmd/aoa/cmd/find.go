package cmd

import (
	"fmt"

	"github.com/corey/aoa-go/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var findCmd = &cobra.Command{
	Use:   "find <glob>",
	Short: "Find files by glob pattern",
	Long:  "Glob-based file search across the indexed codebase. Matches against filenames.",
	Args:  cobra.ExactArgs(1),
	RunE:  runFind,
}

func runFind(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if !client.Ping() {
		return fmt.Errorf("daemon not running. Start with: aoa-go daemon start")
	}

	result, err := client.Files(args[0], "")
	if err != nil {
		return err
	}

	fmt.Print(formatFiles(result))
	return nil
}
