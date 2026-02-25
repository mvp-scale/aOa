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
	grepAndMode        bool
	grepCountOnly      bool
	grepCaseInsens     bool
	grepWordBound      bool
	grepQuiet          bool
	grepInvertMatch    bool
	grepMaxCount       int
	grepUseRegex       bool
	grepPatterns       []string
	grepIncludeGlob    string
	grepExcludeGlob    string
	grepExcludeDir     string
	grepOnlyMatch      bool
	grepFilesNoMatch   bool
	grepNoFilename     bool
	grepNoColor        bool
	grepAfterCtx       int
	grepBeforeCtx      int
	grepContext        int
	grepRecursive      bool
	grepLineNumber     bool
	grepWithFilename   bool
	grepFixedStrings   bool
	grepFilesMatch     bool
	grepColor          string
)

var grepCmd = &cobra.Command{
	Use:           "grep [flags] <pattern> [file ...]",
	Short:         "Search indexed symbols or grep files",
	Long:          "Drop-in grep replacement. Searches files/stdin when given, falls back to aOa index search.",
	Args:          cobra.ArbitraryArgs,
	RunE:          runGrep,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	f := grepCmd.Flags()
	f.BoolVarP(&grepAndMode, "and", "a", false, "AND mode (comma-separated terms)")
	f.BoolVarP(&grepCountOnly, "count", "c", false, "Count only")
	f.BoolVarP(&grepCaseInsens, "ignore-case", "i", false, "Case insensitive")
	f.BoolVarP(&grepWordBound, "word-regexp", "w", false, "Word boundary")
	f.BoolVarP(&grepQuiet, "quiet", "q", false, "Quiet mode (exit code only)")
	f.BoolVarP(&grepInvertMatch, "invert-match", "v", false, "Select non-matching")
	f.IntVarP(&grepMaxCount, "max-count", "m", 0, "Stop after N matches")
	f.BoolVarP(&grepUseRegex, "extended-regexp", "E", false, "Use extended regex")
	f.StringArrayVarP(&grepPatterns, "regexp", "e", nil, "Multiple patterns (OR)")
	f.StringVar(&grepIncludeGlob, "include", "", "File glob filter (include)")
	f.StringVar(&grepExcludeGlob, "exclude", "", "File glob filter (exclude)")
	f.StringVar(&grepExcludeDir, "exclude-dir", "", "Directory glob filter (exclude)")
	f.BoolVarP(&grepOnlyMatch, "only-matching", "o", false, "Print only the matching part")
	f.BoolVarP(&grepFilesNoMatch, "files-without-match", "L", false, "Print files without matches")
	f.BoolVar(&grepNoColor, "no-color", false, "Suppress color output")
	f.IntVarP(&grepAfterCtx, "after-context", "A", 0, "Lines of context after match")
	f.IntVarP(&grepBeforeCtx, "before-context", "B", 0, "Lines of context before match")
	f.IntVarP(&grepContext, "context", "C", 0, "Lines of context (overrides -A/-B)")

	// Real flags (previously no-op)
	f.BoolVarP(&grepRecursive, "recursive", "r", false, "Recurse into directories")
	f.BoolVarP(&grepLineNumber, "line-number", "n", false, "Show line numbers")
	f.BoolVarP(&grepWithFilename, "with-filename", "H", false, "Force filename prefix")
	f.BoolVarP(&grepFixedStrings, "fixed-strings", "F", false, "Treat pattern as fixed string")
	f.BoolVarP(&grepFilesMatch, "files-with-matches", "l", false, "Print only filenames of matching files")
	f.BoolVar(&grepNoFilename, "no-filename", false, "Suppress filename prefix")
	f.StringVar(&grepColor, "color", "auto", "Color output: auto, always, never")
}

func runGrep(cmd *cobra.Command, args []string) error {
	// 1. Parse pattern vs file args
	pattern, fileArgs, multiPattern, err := parseGrepArgs(args, grepPatterns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grep: %v\n", err)
		return grepExit{2}
	}

	// 2. Resolve color
	useColor := resolveColor(grepColor, grepNoColor)

	// 3. Build matcher options
	// Multiple -e patterns joined with | require regex mode
	useRegex := grepUseRegex || multiPattern
	matchOpts := matcherOpts{
		useRegex:     useRegex,
		caseInsens:   grepCaseInsens,
		wordBound:    grepWordBound,
		fixedStrings: grepFixedStrings,
		onlyMatch:    grepOnlyMatch,
	}

	outOpts := grepOutputOpts{
		lineNumber:   grepLineNumber,
		withFilename: grepWithFilename,
		noFilename:   grepNoFilename,
		countOnly:    grepCountOnly,
		quiet:        grepQuiet,
		invertMatch:  grepInvertMatch,
		maxCount:     grepMaxCount,
		onlyMatch:    grepOnlyMatch,
		filesMatch:   grepFilesMatch,
		filesNoMatch: grepFilesNoMatch,
		afterCtx:     grepAfterCtx,
		beforeCtx:    grepBeforeCtx,
		context:      grepContext,
		useColor:     useColor,
		recursive:    grepRecursive,
		includeGlob:  grepIncludeGlob,
		excludeGlob:  grepExcludeGlob,
		excludeDir:   grepExcludeDir,
	}

	// 4. Route: file args → grepFiles (takes priority over stdin)
	if len(fileArgs) > 0 {
		return grepFiles(pattern, fileArgs, matchOpts, outOpts)
	}

	// 5. Route: stdin pipe → grepStdin (only when no file args)
	if isStdinPipe() {
		return grepStdin(pattern, matchOpts, outOpts)
	}

	// 6. Default: index search via daemon
	return runGrepIndex(pattern, useColor)
}

// runGrepIndex searches the aOa index via the daemon socket.
// Falls back to system grep if the daemon is not running.
func runGrepIndex(query string, useColor bool) error {
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
	if grepUseRegex {
		opts.Mode = "regex"
	}

	return executeSearch(query, opts, useColor)
}

// parseGrepArgs separates the pattern from file arguments.
// With -e: all positional args are file paths, patterns come from -e flags.
// Without -e: first positional arg is the pattern, rest are file paths.
// multiPattern is true when multiple -e patterns were joined (requires regex mode).
func parseGrepArgs(args, ePatterns []string) (pattern string, fileArgs []string, multiPattern bool, err error) {
	if len(ePatterns) > 0 {
		// -e patterns provided: join with | for OR, all positional args are files
		pattern = strings.Join(ePatterns, "|")
		fileArgs = args
		multiPattern = len(ePatterns) > 1
	} else if len(args) > 0 {
		pattern = args[0]
		fileArgs = args[1:]
	}
	if pattern == "" {
		return "", nil, false, fmt.Errorf("no search query provided")
	}
	return pattern, fileArgs, multiPattern, nil
}

func executeSearch(query string, opts ports.SearchOptions, useColor bool) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	result, err := client.Search(query, opts)
	if err != nil {
		if isConnectError(err) {
			if isShimMode() {
				// Shim mode: fall back to system grep with correct args.
				// os.Args[2:] skips ["aoa", "grep"] to get the original grep args.
				return fallbackSystemGrep(os.Args[2:])
			}
			// Interactive mode: clear error message to stderr.
			fmt.Fprintln(os.Stderr, "Error: daemon not running. Start with: aoa daemon start")
			return grepExit{2}
		}
		return err
	}

	// Choose output format based on TTY
	if isStdoutTTY() && useColor {
		fmt.Print(formatSearchResult(result, opts.CountOnly, opts.Quiet, grepNoFilename, false))
	} else {
		fmt.Print(formatGrepCompat(result, grepLineNumber, grepNoFilename, grepFilesMatch, grepCountOnly, grepQuiet))
	}

	if opts.Quiet || result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}
	return nil
}

func isConnectError(err error) bool {
	return strings.Contains(err.Error(), "connect:")
}
