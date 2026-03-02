package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/spf13/cobra"
)

var peekCmd = &cobra.Command{
	Use:           "peek <code> [code ...]",
	Short:         "Resolve peek codes to method source",
	Long:          "Peek codes come from grep output. Pass one or more codes to see method bodies inline.",
	Args:          cobra.MinimumNArgs(1),
	RunE:          runPeek,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func runPeek(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	result, err := client.Peek(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "peek: %v\n", err)
		return err
	}

	fmt.Print(formatPeekResult(result))
	return nil
}

// formatPeekResult renders resolved peek symbols for terminal or shim output.
func formatPeekResult(result *socket.PeekResult) string {
	var sb strings.Builder
	for i, sym := range result.Symbols {
		if i > 0 {
			sb.WriteByte('\n')
		}

		if sym.Error != "" {
			sb.WriteString(fmt.Sprintf("── %s: %s ──\n", sym.Code, sym.Error))
			continue
		}

		// Header: ── symbol [range] file  @domain ──
		sb.WriteString(fmt.Sprintf("── %s [%d-%d] %s",
			sym.Symbol, sym.Range[0], sym.Range[1], sym.File))
		if sym.Domain != "" {
			sb.WriteString(fmt.Sprintf("  %s", sym.Domain))
		}
		sb.WriteString(" ──\n")

		// Source lines (no line numbers)
		for _, line := range sym.Lines {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
