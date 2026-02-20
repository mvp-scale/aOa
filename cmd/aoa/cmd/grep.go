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
	grepExcludeDir   string
	grepOnlyMatch    bool
	grepFilesNoMatch bool
	grepNoFilename   bool
	grepNoColor      bool
	grepAfterCtx     int
	grepBeforeCtx    int
	grepContext      int
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
	f.StringVar(&grepExcludeDir, "exclude-dir", "", "Directory glob filter (exclude)")
	f.BoolVarP(&grepOnlyMatch, "only-matching", "o", false, "Print only the matching part")
	f.BoolVarP(&grepFilesNoMatch, "files-without-match", "L", false, "Print files without matches")
	f.BoolVar(&grepNoFilename, "no-filename", false, "Suppress filename prefix")
	f.BoolVar(&grepNoColor, "no-color", false, "Suppress color output")
	f.IntVarP(&grepAfterCtx, "after-context", "A", 0, "Lines of context after match")
	f.IntVarP(&grepBeforeCtx, "before-context", "B", 0, "Lines of context before match")
	f.IntVarP(&grepContext, "context", "C", 0, "Lines of context (overrides -A/-B)")

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
		AndMode:           grepAndMode,
		CountOnly:         grepCountOnly,
		WordBoundary:      grepWordBound,
		Quiet:             grepQuiet,
		InvertMatch:       grepInvertMatch,
		MaxCount:          grepMaxCount,
		IncludeGlob:       grepIncludeGlob,
		ExcludeGlob:       grepExcludeGlob,
		ExcludeDirGlob:    grepExcludeDir,
		OnlyMatching:      grepOnlyMatch,
		FilesWithoutMatch: grepFilesNoMatch,
		AfterContext:       grepAfterCtx,
		BeforeContext:      grepBeforeCtx,
		Context:            grepContext,
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

	result, err := client.Search(query, opts)
	if err != nil {
		if isConnectError(err) {
			return fmt.Errorf("daemon not running. Start with: aoa daemon start")
		}
		return err
	}
	fmt.Print(formatSearchResult(result, opts.CountOnly, opts.Quiet, grepNoFilename, grepNoColor))
	if opts.Quiet {
		os.Exit(result.ExitCode)
	}
	return nil
}

func isConnectError(err error) bool {
	return strings.Contains(err.Error(), "connect:")
}
