package cmd

import (
	"fmt"
	"sort"

	"github.com/corey/aoa-go/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var bigramsCmd = &cobra.Command{
	Use:   "bigrams",
	Short: "Show usage signal bigrams",
	Long:  "Displays top bigrams sorted by count. Bigrams are adjacent word pairs from conversation text.",
	RunE:  runBigrams,
}

func runBigrams(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if !client.Ping() {
		return fmt.Errorf("daemon not running. Start with: aoa-go daemon start")
	}

	result, err := client.Bigrams()
	if err != nil {
		return err
	}

	fmt.Print(formatBigrams(result))
	return nil
}

func formatBigrams(result *socket.BigramsResult) string {
	if result.Count == 0 {
		return fmt.Sprintf("%s⚡ 0 bigrams%s\n", colorBold, colorReset)
	}

	type bg struct {
		name  string
		count uint32
	}
	var sorted []bg
	for name, count := range result.Bigrams {
		sorted = append(sorted, bg{name, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	limit := 20
	if limit > len(sorted) {
		limit = len(sorted)
	}

	out := fmt.Sprintf("%s⚡ %d bigrams%s\n", colorBold, result.Count, colorReset)
	for _, b := range sorted[:limit] {
		out += fmt.Sprintf("  %-30s %s%d%s\n", b.name, colorCyan, b.count, colorReset)
	}
	if result.Count > limit {
		out += fmt.Sprintf("  %s... and %d more%s\n", colorGray, result.Count-limit, colorReset)
	}
	return out
}
