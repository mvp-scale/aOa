package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/corey/aoa-go/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var domainsJSON bool

var domainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "Show semantic domain stats",
	Long:  "Displays active domains sorted by hits. Shows tier, state, and source.",
	RunE:  runDomains,
}

func init() {
	domainsCmd.Flags().BoolVar(&domainsJSON, "json", false, "Output as JSON")
}

func runDomains(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if !client.Ping() {
		return fmt.Errorf("daemon not running. Start with: aoa-go daemon start")
	}

	result, err := client.Domains()
	if err != nil {
		return err
	}

	if domainsJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Print(formatDomains(result))
	return nil
}
