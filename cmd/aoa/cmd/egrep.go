package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/ports"
	"github.com/spf13/cobra"
)

var (
	egrepCountOnly   bool
	egrepQuiet       bool
	egrepMaxCount    int
	egrepPatterns    []string
	egrepIncludeGlob string
	egrepExcludeGlob string
)

var egrepCmd = &cobra.Command{
	Use:   "egrep [flags] <pattern>",
	Short: "Regex search across indexed symbols",
	Long:  "Extended regular expression search. Scans all symbols with full regex support.",
	Args:  cobra.ArbitraryArgs,
	RunE:  runEgrep,
}

func init() {
	f := egrepCmd.Flags()
	f.BoolVarP(&egrepCountOnly, "count", "c", false, "Count only")
	f.BoolVarP(&egrepQuiet, "quiet", "q", false, "Quiet mode (exit code only)")
	f.IntVarP(&egrepMaxCount, "max-count", "m", 20, "Max results")
	f.StringArrayVarP(&egrepPatterns, "regexp", "e", nil, "Multiple patterns (combined with |)")
	f.StringVar(&egrepIncludeGlob, "include", "", "File glob filter (include)")
	f.StringVar(&egrepExcludeGlob, "exclude", "", "File glob filter (exclude)")

	// No-op flags for egrep compatibility
	f.BoolP("recursive", "r", false, "Always recursive (no-op)")
	f.BoolP("line-number", "n", false, "Always shows line numbers (no-op)")
	f.BoolP("with-filename", "H", false, "Always shows filenames (no-op)")
	f.MarkHidden("recursive")
	f.MarkHidden("line-number")
	f.MarkHidden("with-filename")
}

func runEgrep(cmd *cobra.Command, args []string) error {
	pattern := buildEgrepPattern(args, egrepPatterns)
	if pattern == "" {
		return fmt.Errorf("no search pattern provided")
	}
	return runEgrepSearch(pattern)
}

func runEgrepSearch(pattern string) error {
	opts := ports.SearchOptions{
		Mode:        "regex",
		CountOnly:   egrepCountOnly,
		Quiet:       egrepQuiet,
		MaxCount:    egrepMaxCount,
		IncludeGlob: egrepIncludeGlob,
		ExcludeGlob: egrepExcludeGlob,
	}

	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if client.Ping() {
		result, err := client.Search(pattern, opts)
		if err != nil {
			return err
		}
		fmt.Print(formatSearchResult(result, opts.CountOnly, opts.Quiet))
		if opts.Quiet {
			os.Exit(result.ExitCode)
		}
		return nil
	}

	return fmt.Errorf("daemon not running. Start with: aoa daemon start")
}

// buildEgrepPattern combines args and -e patterns into a single regex.
// Multiple -e patterns are joined with | (regex OR).
func buildEgrepPattern(args, patterns []string) string {
	all := make([]string, 0, len(args)+len(patterns))
	all = append(all, args...)
	all = append(all, patterns...)
	if len(all) == 0 {
		return ""
	}
	if len(all) == 1 {
		return all[0]
	}
	return strings.Join(all, "|")
}
