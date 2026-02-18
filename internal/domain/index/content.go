package index

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/ports"
)

// nonAlnumRe replaces non-alphanumeric characters with spaces for tokenization
// of raw source lines, which contain syntax characters the tokenizer doesn't split on.
var nonAlnumRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// maxContentFileSize is the maximum file size (in bytes) to scan for content matches.
// Files larger than this are skipped to avoid performance issues.
const maxContentFileSize = 1 << 20 // 1 MB

// symbolSpan represents an indexed symbol's range within a file.
type symbolSpan struct {
	ref       ports.TokenRef
	sym       *ports.SymbolMeta
	startLine int
	endLine   int
}

// scanFileContents scans indexed file contents for grep-style matches.
// It deduplicates against existing symbol hits (same file+line) and returns
// content-only hits sorted by fileID/line. Each content hit includes the
// enclosing symbol(s) from the tree-sitter index for structural context.
func (e *SearchEngine) scanFileContents(query string, opts ports.SearchOptions, symbolHits []Hit) []Hit {
	// Build dedup set from symbol hits: {fileID, line}
	type fileLine struct {
		fileID uint32
		line   int
	}
	dedup := make(map[fileLine]bool, len(symbolHits))
	for _, h := range symbolHits {
		dedup[fileLine{h.fileID, h.Line}] = true
	}

	matcher := buildContentMatcher(query, opts)
	if matcher == nil {
		return nil
	}

	// Build per-file symbol spans from the index metadata.
	// Used to find enclosing symbol(s) for each content hit.
	fileSpans := e.buildFileSpans()

	var hits []Hit

	for fileID, file := range e.idx.Files {
		// Skip large files
		if file.Size > maxContentFileSize {
			continue
		}

		// Apply glob filters
		if !matchesGlobs(file.Path, opts.IncludeGlob, opts.ExcludeGlob) {
			continue
		}

		absPath := filepath.Join(e.projectRoot, file.Path)
		f, err := os.Open(absPath)
		if err != nil {
			continue // silently skip missing/unreadable files
		}

		spans := fileSpans[fileID]

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if !matcher(line) {
				continue
			}
			fl := fileLine{fileID, lineNum}
			if dedup[fl] {
				continue
			}
			dedup[fl] = true

			hit := Hit{
				File:    file.Path,
				Line:    lineNum,
				Kind:    "content",
				Content: strings.TrimSpace(line),
				Tags:    e.generateContentTags(line),
				fileID:  fileID,
			}

			// Find enclosing symbol(s) — innermost wins for Symbol/Range
			if enclosing := findEnclosingSymbol(spans, lineNum); enclosing != nil {
				hit.Symbol = FormatSymbol(enclosing.sym)
				hit.Range = [2]int{enclosing.startLine, enclosing.endLine}
			}

			hits = append(hits, hit)
		}
		f.Close()
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].fileID != hits[j].fileID {
			return hits[i].fileID < hits[j].fileID
		}
		return hits[i].Line < hits[j].Line
	})

	return hits
}

// buildFileSpans groups all indexed symbols by file and sorts by range size
// (smallest/innermost first) for enclosing symbol lookup.
func (e *SearchEngine) buildFileSpans() map[uint32][]symbolSpan {
	fileSpans := make(map[uint32][]symbolSpan)
	for ref, sym := range e.idx.Metadata {
		if sym == nil {
			continue
		}
		fileSpans[ref.FileID] = append(fileSpans[ref.FileID], symbolSpan{
			ref:       ref,
			sym:       sym,
			startLine: int(sym.StartLine),
			endLine:   int(sym.EndLine),
		})
	}
	// Sort each file's spans: smallest range first (innermost symbol wins)
	for fid := range fileSpans {
		spans := fileSpans[fid]
		sort.SliceStable(spans, func(i, j int) bool {
			si := spans[i].endLine - spans[i].startLine
			sj := spans[j].endLine - spans[j].startLine
			return si < sj
		})
		fileSpans[fid] = spans
	}
	return fileSpans
}

// findEnclosingSymbol returns the innermost symbol whose range contains lineNum.
// Spans are pre-sorted smallest-first, so the first match is the innermost.
func findEnclosingSymbol(spans []symbolSpan, lineNum int) *symbolSpan {
	for i := range spans {
		if lineNum >= spans[i].startLine && lineNum <= spans[i].endLine {
			return &spans[i]
		}
	}
	return nil
}

// generateContentTags produces atlas terms for a content hit line.
// Tokenizes the line and resolves tokens through the keyword→term lookup.
// No domain is assigned — domains only apply to symbol declarations.
func (e *SearchEngine) generateContentTags(line string) []string {
	// Normalize: replace syntax characters with spaces so the tokenizer can split
	// on word boundaries. The tokenizer only splits on [/_\-.\s]+ and camelCase,
	// missing parens, braces, operators, etc. found in source code.
	normalized := nonAlnumRe.ReplaceAllString(line, " ")
	tokens := Tokenize(normalized)
	return e.resolveTerms(tokens)
}

// buildContentMatcher returns a function that tests whether a line matches the query.
// Returns nil if the query/mode combination can't produce a valid matcher.
func buildContentMatcher(query string, opts ports.SearchOptions) func(string) bool {
	switch {
	case opts.Mode == "regex":
		re, err := regexp.Compile(query)
		if err != nil {
			return nil
		}
		return re.MatchString

	case opts.WordBoundary:
		// Wrap each token with \b for word boundary matching
		tokens := Tokenize(query)
		if len(tokens) == 0 {
			return nil
		}
		var patterns []*regexp.Regexp
		for _, tok := range tokens {
			re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(tok) + `\b`)
			if err != nil {
				return nil
			}
			patterns = append(patterns, re)
		}
		return func(line string) bool {
			for _, re := range patterns {
				if re.MatchString(line) {
					return true
				}
			}
			return false
		}

	case opts.AndMode:
		// Comma-separated terms, all must appear
		termStrs := strings.Split(query, ",")
		var terms []string
		for _, t := range termStrs {
			t = strings.TrimSpace(t)
			if t != "" {
				terms = append(terms, strings.ToLower(t))
			}
		}
		if len(terms) == 0 {
			return nil
		}
		return func(line string) bool {
			lower := strings.ToLower(line)
			for _, t := range terms {
				if !strings.Contains(lower, t) {
					return false
				}
			}
			return true
		}

	default:
		// Case-insensitive substring match
		lowerQuery := strings.ToLower(query)
		return func(line string) bool {
			return strings.Contains(strings.ToLower(line), lowerQuery)
		}
	}
}
