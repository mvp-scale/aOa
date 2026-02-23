package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// matcherOpts controls how patterns are compiled into matchers.
type matcherOpts struct {
	useRegex     bool
	caseInsens   bool
	wordBound    bool
	fixedStrings bool
	onlyMatch    bool
}

// grepOutputOpts controls output formatting for native grep.
type grepOutputOpts struct {
	lineNumber   bool
	withFilename bool
	noFilename   bool
	countOnly    bool
	quiet        bool
	invertMatch  bool
	maxCount     int
	onlyMatch    bool
	filesMatch   bool
	filesNoMatch bool
	afterCtx     int
	beforeCtx    int
	context      int
	useColor     bool
	recursive    bool
	includeGlob  string
	excludeGlob  string
	excludeDir   string
}

// lineMatcher is a compiled pattern matcher.
type lineMatcher struct {
	match     func(string) bool
	re        *regexp.Regexp // non-nil when regex is used (for -o extraction)
	isLiteral bool
	pattern   string
}

// contextLine stores a line with its line number for context output.
type contextLine struct {
	lineNum int
	text    string
}

// ringBuffer is a fixed-size circular buffer for -B (before context) lines.
type ringBuffer struct {
	buf  []contextLine
	size int
	pos  int
	n    int // total items ever added
}

func newRingBuffer(size int) *ringBuffer {
	if size <= 0 {
		return &ringBuffer{}
	}
	return &ringBuffer{buf: make([]contextLine, size), size: size}
}

func (r *ringBuffer) add(cl contextLine) {
	if r.size == 0 {
		return
	}
	r.buf[r.pos%r.size] = cl
	r.pos++
	r.n++
}

func (r *ringBuffer) items() []contextLine {
	if r.size == 0 {
		return nil
	}
	count := r.size
	if r.n < r.size {
		count = r.n
	}
	result := make([]contextLine, 0, count)
	start := r.pos - count
	if start < 0 {
		start = 0
	}
	for i := start; i < r.pos; i++ {
		result = append(result, r.buf[i%r.size])
	}
	return result
}

func (r *ringBuffer) clear() {
	r.pos = 0
	r.n = 0
}

// buildLineMatcher compiles a pattern string into a lineMatcher.
func buildLineMatcher(pattern string, opts matcherOpts) (*lineMatcher, error) {
	lm := &lineMatcher{pattern: pattern}

	if opts.fixedStrings && !opts.useRegex {
		// Fixed string mode: literal match, no regex
		pat := pattern
		if opts.caseInsens {
			pat = strings.ToLower(pat)
		}
		if opts.wordBound {
			re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(pattern) + `\b`)
			if err != nil {
				return nil, err
			}
			lm.re = re
			lm.match = func(line string) bool { return re.MatchString(line) }
		} else {
			lm.isLiteral = true
			lm.match = func(line string) bool {
				if opts.caseInsens {
					return strings.Contains(strings.ToLower(line), pat)
				}
				return strings.Contains(line, pat)
			}
		}
		return lm, nil
	}

	if opts.useRegex {
		// Regex mode
		rePattern := pattern
		flags := ""
		if opts.caseInsens {
			flags = "(?i)"
		}
		if opts.wordBound {
			rePattern = `\b(?:` + rePattern + `)\b`
		}
		re, err := regexp.Compile(flags + rePattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		lm.re = re
		lm.match = func(line string) bool { return re.MatchString(line) }
		return lm, nil
	}

	// Default: literal substring match
	pat := pattern
	if opts.caseInsens {
		pat = strings.ToLower(pat)
	}
	if opts.wordBound {
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(pattern) + `\b`)
		if err != nil {
			return nil, err
		}
		lm.re = re
		lm.match = func(line string) bool { return re.MatchString(line) }
	} else {
		lm.isLiteral = true
		lm.match = func(line string) bool {
			if opts.caseInsens {
				return strings.Contains(strings.ToLower(line), pat)
			}
			return strings.Contains(line, pat)
		}
	}
	return lm, nil
}

// findMatch returns the matched substring for -o mode.
func (lm *lineMatcher) findMatch(line string) string {
	if lm.re != nil {
		return lm.re.FindString(line)
	}
	// Literal: return the pattern itself if it matches
	if lm.isLiteral {
		idx := strings.Index(line, lm.pattern)
		if idx >= 0 {
			return line[idx : idx+len(lm.pattern)]
		}
		// Case insensitive
		idx = strings.Index(strings.ToLower(line), strings.ToLower(lm.pattern))
		if idx >= 0 {
			return line[idx : idx+len(lm.pattern)]
		}
	}
	return ""
}

// grepStdin reads from stdin and applies the matcher, writing matches to stdout.
func grepStdin(pattern string, mOpts matcherOpts, oOpts grepOutputOpts) error {
	matcher, err := buildLineMatcher(pattern, mOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grep: %v\n", err)
		return grepExit{2}
	}

	beforeN, afterN := resolveContext(oOpts)
	beforeBuf := newRingBuffer(beforeN)
	matchCount := 0
	lineNum := 0
	afterRemaining := 0
	lastMatchLine := -1
	printedSep := true // suppress leading separator

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		matched := matcher.match(line)
		if oOpts.invertMatch {
			matched = !matched
		}

		if matched {
			// Print group separator if there's a gap
			if beforeN > 0 || afterN > 0 {
				if lastMatchLine > 0 && lineNum-lastMatchLine > afterN+1 && !printedSep {
					fmt.Println("--")
				}
			}

			// Print buffered before-context
			for _, cl := range beforeBuf.items() {
				if cl.lineNum > lastMatchLine+afterN || lastMatchLine < 0 {
					printStdinLine(cl.lineNum, cl.text, oOpts, false)
				}
			}
			beforeBuf.clear()

			matchCount++
			if !oOpts.countOnly && !oOpts.quiet {
				if oOpts.onlyMatch && !oOpts.invertMatch {
					m := matcher.findMatch(line)
					if m != "" {
						printStdinLine(lineNum, m, oOpts, true)
					}
				} else {
					printStdinLine(lineNum, line, oOpts, true)
				}
			}
			afterRemaining = afterN
			lastMatchLine = lineNum
			printedSep = false

			if oOpts.maxCount > 0 && matchCount >= oOpts.maxCount {
				break
			}
		} else {
			if afterRemaining > 0 {
				printStdinLine(lineNum, line, oOpts, false)
				afterRemaining--
			} else {
				beforeBuf.add(contextLine{lineNum: lineNum, text: line})
			}
		}
	}

	if oOpts.countOnly {
		fmt.Println(matchCount)
	}
	if oOpts.quiet {
		if matchCount > 0 {
			return grepExit{0}
		}
		return grepExit{1}
	}
	if matchCount == 0 {
		return grepExit{1}
	}
	return nil
}

// printStdinLine prints a single stdin grep result line.
func printStdinLine(lineNum int, text string, oOpts grepOutputOpts, isMatch bool) {
	if oOpts.countOnly || oOpts.quiet {
		return
	}
	sep := ":"
	if !isMatch {
		sep = "-"
	}
	if oOpts.lineNumber {
		fmt.Printf("%d%s%s\n", lineNum, sep, text)
	} else {
		fmt.Println(text)
	}
}

// grepFiles searches one or more files/directories for the pattern.
func grepFiles(pattern string, fileArgs []string, mOpts matcherOpts, oOpts grepOutputOpts) error {
	matcher, err := buildLineMatcher(pattern, mOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grep: %v\n", err)
		return grepExit{2}
	}

	files, errOccurred := expandFileArgs(fileArgs, oOpts)
	if len(files) == 0 && errOccurred {
		return grepExit{2}
	}

	// Determine whether to show filenames
	showFilename := len(files) > 1
	if oOpts.withFilename {
		showFilename = true
	}
	if oOpts.noFilename {
		showFilename = false
	}

	totalMatches := 0
	for _, fpath := range files {
		n, err := grepFile(fpath, matcher, showFilename, oOpts)
		totalMatches += n
		if err != nil {
			// grepFile already printed to stderr
			errOccurred = true
		}
	}

	if oOpts.quiet {
		if totalMatches > 0 {
			return grepExit{0}
		}
		return grepExit{1}
	}
	if errOccurred && totalMatches == 0 {
		return grepExit{2}
	}
	if totalMatches == 0 {
		return grepExit{1}
	}
	return nil
}

// expandFileArgs expands file arguments, handling directories and globs.
// Returns the list of files and whether any error was printed to stderr.
func expandFileArgs(fileArgs []string, oOpts grepOutputOpts) ([]string, bool) {
	var files []string
	errOccurred := false

	for _, arg := range fileArgs {
		info, err := os.Stat(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "grep: %s: No such file or directory\n", arg)
			errOccurred = true
			continue
		}
		if info.IsDir() {
			if !oOpts.recursive {
				fmt.Fprintf(os.Stderr, "grep: %s: Is a directory\n", arg)
				errOccurred = true
				continue
			}
			dirFiles := walkDir(arg, oOpts)
			files = append(files, dirFiles...)
		} else {
			files = append(files, arg)
		}
	}
	return files, errOccurred
}

// skipDirs are directories never recursed into.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"__pycache__":  true,
	".aoa":         true,
	"vendor":       true,
	".venv":        true,
}

// walkDir recursively collects files in a directory, respecting filters.
func walkDir(root string, oOpts grepOutputOpts) []string {
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if skipDirs[base] {
				return filepath.SkipDir
			}
			if oOpts.excludeDir != "" {
				matched, _ := filepath.Match(oOpts.excludeDir, base)
				if matched {
					return filepath.SkipDir
				}
			}
			return nil
		}
		// Apply include/exclude globs
		if oOpts.includeGlob != "" {
			matched, _ := filepath.Match(oOpts.includeGlob, filepath.Base(path))
			if !matched {
				return nil
			}
		}
		if oOpts.excludeGlob != "" {
			matched, _ := filepath.Match(oOpts.excludeGlob, filepath.Base(path))
			if matched {
				return nil
			}
		}
		files = append(files, path)
		return nil
	})
	return files
}

// isBinaryFile checks if the first 512 bytes contain a NUL byte.
func isBinaryFile(f *os.File) bool {
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	// Seek back to start
	f.Seek(0, io.SeekStart)
	return false
}

// grepFile searches a single file and prints results.
// Returns the number of matches found and any error.
func grepFile(fpath string, matcher *lineMatcher, showFilename bool, oOpts grepOutputOpts) (int, error) {
	f, err := os.Open(fpath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grep: %s: %v\n", fpath, err)
		return 0, err
	}
	defer f.Close()

	if isBinaryFile(f) {
		// For binary files, just check if any line matches
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			matched := matcher.match(line)
			if oOpts.invertMatch {
				matched = !matched
			}
			if matched {
				if !oOpts.quiet && !oOpts.countOnly && !oOpts.filesMatch {
					fmt.Fprintf(os.Stderr, "Binary file %s matches\n", fpath)
				}
				if oOpts.filesMatch && !oOpts.quiet {
					fmt.Println(fpath)
				}
				return 1, nil
			}
		}
		return 0, nil
	}

	beforeN, afterN := resolveContext(oOpts)
	beforeBuf := newRingBuffer(beforeN)
	matchCount := 0
	lineNum := 0
	afterRemaining := 0
	lastMatchLine := -1
	printedSep := true

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		matched := matcher.match(line)
		if oOpts.invertMatch {
			matched = !matched
		}

		if matched {
			matchCount++

			if oOpts.filesMatch {
				if !oOpts.quiet {
					fmt.Println(fpath)
				}
				return matchCount, nil
			}

			// Group separator
			if beforeN > 0 || afterN > 0 {
				if lastMatchLine > 0 && lineNum-lastMatchLine > afterN+1 && !printedSep {
					fmt.Println("--")
				}
			}

			// Print before-context
			if !oOpts.countOnly && !oOpts.quiet {
				for _, cl := range beforeBuf.items() {
					if cl.lineNum > lastMatchLine+afterN || lastMatchLine < 0 {
						printFileLine(fpath, cl.lineNum, cl.text, showFilename, oOpts, false)
					}
				}
			}
			beforeBuf.clear()

			if !oOpts.countOnly && !oOpts.quiet {
				if oOpts.onlyMatch && !oOpts.invertMatch {
					m := matcher.findMatch(line)
					if m != "" {
						printFileLine(fpath, lineNum, m, showFilename, oOpts, true)
					}
				} else {
					printFileLine(fpath, lineNum, line, showFilename, oOpts, true)
				}
			}

			afterRemaining = afterN
			lastMatchLine = lineNum
			printedSep = false

			if oOpts.maxCount > 0 && matchCount >= oOpts.maxCount {
				break
			}
		} else {
			if afterRemaining > 0 && !oOpts.countOnly && !oOpts.quiet {
				printFileLine(fpath, lineNum, line, showFilename, oOpts, false)
				afterRemaining--
			} else {
				beforeBuf.add(contextLine{lineNum: lineNum, text: line})
			}
		}
	}

	if oOpts.filesNoMatch && matchCount == 0 && !oOpts.quiet {
		fmt.Println(fpath)
	}

	if oOpts.countOnly && !oOpts.quiet {
		if showFilename {
			fmt.Printf("%s:%d\n", fpath, matchCount)
		} else {
			fmt.Println(matchCount)
		}
	}

	return matchCount, nil
}

// printFileLine prints a single grep result line for a file.
func printFileLine(fpath string, lineNum int, text string, showFilename bool, oOpts grepOutputOpts, isMatch bool) {
	sep := ":"
	if !isMatch {
		sep = "-"
	}
	var sb strings.Builder
	if showFilename {
		if oOpts.useColor {
			sb.WriteString(colorMagenta)
			sb.WriteString(fpath)
			sb.WriteString(colorReset)
		} else {
			sb.WriteString(fpath)
		}
		sb.WriteString(sep)
	}
	if oOpts.lineNumber {
		if oOpts.useColor {
			sb.WriteString(colorGreen)
			fmt.Fprintf(&sb, "%d", lineNum)
			sb.WriteString(colorReset)
		} else {
			fmt.Fprintf(&sb, "%d", lineNum)
		}
		sb.WriteString(sep)
	}
	sb.WriteString(text)
	fmt.Println(sb.String())
}

// resolveContext returns the effective before and after context line counts.
func resolveContext(oOpts grepOutputOpts) (before, after int) {
	if oOpts.context > 0 {
		return oOpts.context, oOpts.context
	}
	return oOpts.beforeCtx, oOpts.afterCtx
}
