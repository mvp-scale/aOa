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
	egrepCountOnly    bool
	egrepQuiet        bool
	egrepInvertMatch  bool
	egrepMaxCount     int
	egrepPatterns     []string
	egrepIncludeGlob  string
	egrepExcludeGlob  string
	egrepCaseInsens   bool
	egrepWordBound    bool
	egrepAndMode      bool
	egrepExcludeDir   string
	egrepOnlyMatch    bool
	egrepFilesNoMatch bool
	egrepNoFilename   bool
	egrepNoColor      bool
	egrepAfterCtx     int
	egrepBeforeCtx    int
	egrepContext       int
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
	f.BoolVarP(&egrepCaseInsens, "ignore-case", "i", false, "Case insensitive")
	f.BoolVarP(&egrepWordBound, "word-regexp", "w", false, "Word boundary")
	f.BoolVarP(&egrepAndMode, "and", "a", false, "AND mode (comma-separated terms)")
	f.BoolVarP(&egrepCountOnly, "count", "c", false, "Count only")
	f.BoolVarP(&egrepQuiet, "quiet", "q", false, "Quiet mode (exit code only)")
	f.BoolVarP(&egrepInvertMatch, "invert-match", "v", false, "Select non-matching")
	f.IntVarP(&egrepMaxCount, "max-count", "m", 20, "Max results")
	f.StringArrayVarP(&egrepPatterns, "regexp", "e", nil, "Multiple patterns (combined with |)")
	f.StringVar(&egrepIncludeGlob, "include", "", "File glob filter (include)")
	f.StringVar(&egrepExcludeGlob, "exclude", "", "File glob filter (exclude)")
	f.StringVar(&egrepExcludeDir, "exclude-dir", "", "Directory glob filter (exclude)")
	f.BoolVarP(&egrepOnlyMatch, "only-matching", "o", false, "Print only the matching part")
	f.BoolVarP(&egrepFilesNoMatch, "files-without-match", "L", false, "Print files without matches")
	f.BoolVar(&egrepNoFilename, "no-filename", false, "Suppress filename prefix")
	f.BoolVar(&egrepNoColor, "no-color", false, "Suppress color output")
	f.IntVarP(&egrepAfterCtx, "after-context", "A", 0, "Lines of context after match")
	f.IntVarP(&egrepBeforeCtx, "before-context", "B", 0, "Lines of context before match")
	f.IntVarP(&egrepContext, "context", "C", 0, "Lines of context (overrides -A/-B)")

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
		Mode:              "regex",
		CountOnly:         egrepCountOnly,
		Quiet:             egrepQuiet,
		InvertMatch:       egrepInvertMatch,
		MaxCount:          egrepMaxCount,
		IncludeGlob:       egrepIncludeGlob,
		ExcludeGlob:       egrepExcludeGlob,
		WordBoundary:      egrepWordBound,
		AndMode:           egrepAndMode,
		ExcludeDirGlob:    egrepExcludeDir,
		OnlyMatching:      egrepOnlyMatch,
		FilesWithoutMatch: egrepFilesNoMatch,
		AfterContext:       egrepAfterCtx,
		BeforeContext:      egrepBeforeCtx,
		Context:            egrepContext,
	}
	if egrepCaseInsens {
		opts.Mode = "case_insensitive"
	}

	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	result, err := client.Search(pattern, opts)
	if err != nil {
		if isConnectError(err) {
			return fmt.Errorf("daemon not running. Start with: aoa daemon start")
		}
		return err
	}
	fmt.Print(formatSearchResult(result, opts.CountOnly, opts.Quiet, egrepNoFilename, egrepNoColor))
	if opts.Quiet {
		os.Exit(result.ExitCode)
	}
	return nil
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
