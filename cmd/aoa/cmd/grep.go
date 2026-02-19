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
	grepAndMode      bool
	grepCountOnly    bool
	grepCaseInsens   bool
	grepWordBound    bool
	grepQuiet        bool
	grepInvertMatch  bool
	grepMaxCount     int
	grepUseRegex     bool
	grepPatterns     []string
	grepIncludeGlob  string
	grepExcludeGlob  string
)

var grepCmd = &cobra.Command{
	Use:   "grep [flags] <query>",
	Short: "Search indexed symbols",
	Long:  "Fast O(1) symbol lookup. Space-separated terms are OR search, ranked by density.",
	Args:  cobra.ArbitraryArgs,
	RunE:  runGrep,
}

func init() {
	f := grepCmd.Flags()
	f.BoolVarP(&grepAndMode, "and", "a", false, "AND mode (comma-separated terms)")
	f.BoolVarP(&grepCountOnly, "count", "c", false, "Count only")
	f.BoolVarP(&grepCaseInsens, "ignore-case", "i", false, "Case insensitive")
	f.BoolVarP(&grepWordBound, "word-regexp", "w", false, "Word boundary")
	f.BoolVarP(&grepQuiet, "quiet", "q", false, "Quiet mode (exit code only)")
	f.BoolVarP(&grepInvertMatch, "invert-match", "v", false, "Select non-matching")
	f.IntVarP(&grepMaxCount, "max-count", "m", 20, "Max results")
	f.BoolVarP(&grepUseRegex, "extended-regexp", "E", false, "Use regex (routes to egrep)")
	f.StringArrayVarP(&grepPatterns, "regexp", "e", nil, "Multiple patterns (OR)")
	f.StringVar(&grepIncludeGlob, "include", "", "File glob filter (include)")
	f.StringVar(&grepExcludeGlob, "exclude", "", "File glob filter (exclude)")

	// No-op flags for grep compatibility
	f.BoolP("recursive", "r", false, "Always recursive (no-op)")
	f.BoolP("line-number", "n", false, "Always shows line numbers (no-op)")
	f.BoolP("with-filename", "H", false, "Always shows filenames (no-op)")
	f.BoolP("fixed-strings", "F", false, "Already literal (no-op)")
	f.BoolP("files-with-matches", "l", false, "Default behavior (no-op)")
	f.MarkHidden("recursive")
	f.MarkHidden("line-number")
	f.MarkHidden("with-filename")
	f.MarkHidden("fixed-strings")
	f.MarkHidden("files-with-matches")
}

func runGrep(cmd *cobra.Command, args []string) error {
	// Build query from args and -e patterns
	query := buildQuery(args, grepPatterns)
	if query == "" {
		return fmt.Errorf("no search query provided")
	}

	// If -E flag, route to egrep behavior
	if grepUseRegex {
		return runEgrepSearch(query)
	}

	opts := ports.SearchOptions{
		AndMode:      grepAndMode,
		CountOnly:    grepCountOnly,
		WordBoundary: grepWordBound,
		Quiet:        grepQuiet,
		InvertMatch:  grepInvertMatch,
		MaxCount:     grepMaxCount,
		IncludeGlob:  grepIncludeGlob,
		ExcludeGlob:  grepExcludeGlob,
	}
	if grepCaseInsens {
		opts.Mode = "case_insensitive"
	}

	return executeSearch(query, opts)
}

// buildQuery combines positional args and -e patterns into a single query string.
func buildQuery(args, patterns []string) string {
	parts := make([]string, 0, len(args)+len(patterns))
	for _, a := range args {
		// Convert pipe-separated patterns to space-separated (grep parity)
		a = strings.ReplaceAll(a, `\|`, " ")
		parts = append(parts, a)
	}
	parts = append(parts, patterns...)
	return strings.Join(parts, " ")
}

func executeSearch(query string, opts ports.SearchOptions) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	// Try daemon mode first
	if client.Ping() {
		result, err := client.Search(query, opts)
		if err != nil {
			return err
		}
		fmt.Print(formatSearchResult(result, opts.CountOnly, opts.Quiet))
		if opts.Quiet {
			os.Exit(result.ExitCode)
		}
		return nil
	}

	// No daemon running — direct mode would go here once app.go wiring is complete.
	// For now, report that daemon isn't running.
	return fmt.Errorf("daemon not running. Start with: aoa daemon start")
}
