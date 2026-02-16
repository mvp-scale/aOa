package cmd

import (
	"fmt"
	"sort"

	"github.com/corey/aoa-go/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var intentCmd = &cobra.Command{
	Use:   "intent [recent]",
	Short: "Show intent tracking summary",
	Long:  "Displays prompt count, top active domains, and top keywords.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runIntent,
}

func runIntent(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if !client.Ping() {
		return fmt.Errorf("daemon not running. Start with: aoa-go daemon start")
	}

	stats, err := client.Stats()
	if err != nil {
		return err
	}

	domains, err := client.Domains()
	if err != nil {
		return err
	}

	fmt.Printf("%s⚡ Intent tracking%s │ %d intents\n", colorBold, colorReset, stats.PromptCount)

	if len(domains.Domains) > 0 {
		fmt.Printf("\n  %sActive domains:%s\n", colorBold, colorReset)
		limit := 10
		if limit > len(domains.Domains) {
			limit = len(domains.Domains)
		}
		for _, d := range domains.Domains[:limit] {
			tierColor := colorGreen
			if d.Tier == "context" {
				tierColor = colorGray
			}
			fmt.Printf("    %s@%-20s%s %6.1f hits  %s(%s)%s\n",
				colorMagenta, d.Name, colorReset,
				d.Hits,
				tierColor, d.Tier, colorReset)
		}
	}

	// Show top bigrams if available
	bigrams, err := client.Bigrams()
	if err == nil && bigrams.Count > 0 {
		type bg struct {
			name  string
			count uint32
		}
		var sorted []bg
		for name, count := range bigrams.Bigrams {
			sorted = append(sorted, bg{name, count})
		}
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].count > sorted[j].count
		})
		limit := 10
		if limit > len(sorted) {
			limit = len(sorted)
		}
		if limit > 0 {
			fmt.Printf("\n  %sTop bigrams:%s\n", colorBold, colorReset)
			for _, b := range sorted[:limit] {
				fmt.Printf("    %-30s %s%d%s\n", b.name, colorCyan, b.count, colorReset)
			}
		}
	}

	return nil
}
