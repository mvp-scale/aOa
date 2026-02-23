package cmd

import (
	"fmt"
	"os"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/ports"
	"github.com/spf13/cobra"
)

var (
	egrepCountOnly     bool
	egrepQuiet         bool
	egrepInvertMatch   bool
	egrepMaxCount      int
	egrepPatterns      []string
	egrepIncludeGlob   string
	egrepExcludeGlob   string
	egrepCaseInsens    bool
	egrepWordBound     bool
	egrepAndMode       bool
	egrepExcludeDir    string
	egrepOnlyMatch     bool
	egrepFilesNoMatch  bool
	egrepNoFilename    bool
	egrepNoColor       bool
	egrepAfterCtx      int
	egrepBeforeCtx     int
	egrepContext        int
	egrepRecursive     bool
	egrepLineNumber    bool
	egrepWithFilename  bool
	egrepFilesMatch    bool
	egrepColor         string
)

var egrepCmd = &cobra.Command{
	Use:           "egrep [flags] <pattern> [file ...]",
	Short:         "Extended regex search or grep files",
	Long:          "Drop-in egrep replacement. Uses extended regex. Searches files/stdin when given, falls back to aOa index.",
	Args:          cobra.ArbitraryArgs,
	RunE:          runEgrep,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	f := egrepCmd.Flags()
	f.BoolVarP(&egrepCaseInsens, "ignore-case", "i", false, "Case insensitive")
	f.BoolVarP(&egrepWordBound, "word-regexp", "w", false, "Word boundary")
	f.BoolVarP(&egrepAndMode, "and", "a", false, "AND mode (comma-separated terms)")
	f.BoolVarP(&egrepCountOnly, "count", "c", false, "Count only")
	f.BoolVarP(&egrepQuiet, "quiet", "q", false, "Quiet mode (exit code only)")
	f.BoolVarP(&egrepInvertMatch, "invert-match", "v", false, "Select non-matching")
	f.IntVarP(&egrepMaxCount, "max-count", "m", 0, "Stop after N matches")
	f.StringArrayVarP(&egrepPatterns, "regexp", "e", nil, "Multiple patterns (combined with |)")
	f.StringVar(&egrepIncludeGlob, "include", "", "File glob filter (include)")
	f.StringVar(&egrepExcludeGlob, "exclude", "", "File glob filter (exclude)")
	f.StringVar(&egrepExcludeDir, "exclude-dir", "", "Directory glob filter (exclude)")
	f.BoolVarP(&egrepOnlyMatch, "only-matching", "o", false, "Print only the matching part")
	f.BoolVarP(&egrepFilesNoMatch, "files-without-match", "L", false, "Print files without matches")
	f.BoolVar(&egrepNoColor, "no-color", false, "Suppress color output")
	f.IntVarP(&egrepAfterCtx, "after-context", "A", 0, "Lines of context after match")
	f.IntVarP(&egrepBeforeCtx, "before-context", "B", 0, "Lines of context before match")
	f.IntVarP(&egrepContext, "context", "C", 0, "Lines of context (overrides -A/-B)")

	// Real flags (previously no-op)
	f.BoolVarP(&egrepRecursive, "recursive", "r", false, "Recurse into directories")
	f.BoolVarP(&egrepLineNumber, "line-number", "n", false, "Show line numbers")
	f.BoolVarP(&egrepWithFilename, "with-filename", "H", false, "Force filename prefix")
	f.BoolVarP(&egrepFilesMatch, "files-with-matches", "l", false, "Print only filenames of matching files")
	f.BoolVar(&egrepNoFilename, "no-filename", false, "Suppress filename prefix")
	f.StringVar(&egrepColor, "color", "auto", "Color output: auto, always, never")
}

func runEgrep(cmd *cobra.Command, args []string) error {
	// 1. Parse pattern vs file args
	pattern, fileArgs, _, err := parseGrepArgs(args, egrepPatterns)
	if err != nil {
		return err
	}

	// 2. Resolve color
	useColor := resolveColor(egrepColor, egrepNoColor)

	// 3. Build matcher options — egrep always uses regex
	matchOpts := matcherOpts{
		useRegex:   true,
		caseInsens: egrepCaseInsens,
		wordBound:  egrepWordBound,
		onlyMatch:  egrepOnlyMatch,
	}

	outOpts := grepOutputOpts{
		lineNumber:   egrepLineNumber,
		withFilename: egrepWithFilename,
		noFilename:   egrepNoFilename,
		countOnly:    egrepCountOnly,
		quiet:        egrepQuiet,
		invertMatch:  egrepInvertMatch,
		maxCount:     egrepMaxCount,
		onlyMatch:    egrepOnlyMatch,
		filesMatch:   egrepFilesMatch,
		filesNoMatch: egrepFilesNoMatch,
		afterCtx:     egrepAfterCtx,
		beforeCtx:    egrepBeforeCtx,
		context:      egrepContext,
		useColor:     useColor,
		recursive:    egrepRecursive,
		includeGlob:  egrepIncludeGlob,
		excludeGlob:  egrepExcludeGlob,
		excludeDir:   egrepExcludeDir,
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
	return runEgrepIndex(pattern, useColor)
}

// runEgrepIndex searches the aOa index via the daemon with regex mode.
func runEgrepIndex(pattern string, useColor bool) error {
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
			return fallbackSystemGrep(os.Args[1:])
		}
		return err
	}

	if isStdoutTTY() && useColor {
		fmt.Print(formatSearchResult(result, opts.CountOnly, opts.Quiet, egrepNoFilename, false))
	} else {
		fmt.Print(formatGrepCompat(result, egrepLineNumber, egrepNoFilename, egrepFilesMatch, egrepCountOnly, egrepQuiet))
	}

	if opts.Quiet || result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}
	return nil
}
