package index

import (
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

// contentFileLine is used for deduplicating content hits by file+line.
type contentFileLine struct {
	fileID uint32
	line   int
}

// scanFileContents scans all cached/on-disk files for grep-style substring matches.
// Always uses brute-force substring matching to preserve full grep semantics —
// "tree" matches "btree", "subtree", etc. Tags are deferred (nil) and filled
// after MaxCount truncation by fillContentTags to avoid wasted work.
func (e *SearchEngine) scanFileContents(query string, opts ports.SearchOptions, symbolHits []Hit) []Hit {
	return e.scanContentBruteForce(query, opts, symbolHits)
}

// scanContentBruteForce is the original full-scan path: iterates every line of
// every cached/on-disk file running a matcher function per line.
func (e *SearchEngine) scanContentBruteForce(query string, opts ports.SearchOptions, symbolHits []Hit) []Hit {
	dedup := make(map[contentFileLine]bool, len(symbolHits))
	for _, h := range symbolHits {
		dedup[contentFileLine{h.fileID, h.Line}] = true
	}

	matcher := buildContentMatcher(query, opts)
	if matcher == nil {
		return nil
	}

	var hits []Hit

	for fileID, file := range e.idx.Files {
		if file.Size > maxContentFileSize {
			continue
		}
		if !matchesGlobs(file.Path, opts.IncludeGlob, opts.ExcludeGlob) {
			continue
		}

		var lines []string
		if e.cache != nil {
			lines = e.cache.GetLines(fileID)
		}
		if lines == nil {
			lines = readFileLines(filepath.Join(e.projectRoot, file.Path))
			if lines == nil {
				continue
			}
		}

		spans := e.fileSpans[fileID]

		for lineIdx, line := range lines {
			lineNum := lineIdx + 1
			matched := matcher(line)
			if opts.InvertMatch {
				matched = !matched
			}
			if !matched {
				continue
			}
			fl := contentFileLine{fileID, lineNum}
			if dedup[fl] {
				continue
			}
			dedup[fl] = true

			hit := e.buildContentHit(fileID, file.Path, lineNum, line, spans)
			hits = append(hits, hit)
		}
	}

	sortByFileIDLine(hits)
	return hits
}

// buildContentHit constructs a content Hit with enclosing symbol context.
// Tags are deferred (nil) and filled after MaxCount truncation by fillContentTags.
func (e *SearchEngine) buildContentHit(fileID uint32, path string, lineNum int, line string, spans []symbolSpan) Hit {
	hit := Hit{
		File:    path,
		Line:    lineNum,
		Kind:    "content",
		Content: strings.TrimSpace(line),
		fileID:  fileID,
	}
	if enclosing := findEnclosingSymbol(spans, lineNum); enclosing != nil {
		hit.Symbol = FormatSymbol(enclosing.sym)
		hit.Range = [2]int{enclosing.startLine, enclosing.endLine}
	}
	return hit
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
	tokens := TokenizeContentLine(line)
	return e.resolveTerms(tokens)
}

// fillContentTags populates Tags for any content hits that have nil Tags.
// Called after truncation so tags are only generated for the final result set.
func (e *SearchEngine) fillContentTags(hits []Hit) {
	for i := range hits {
		if hits[i].Kind == "content" && hits[i].Tags == nil {
			hits[i].Tags = e.generateContentTags(hits[i].Content)
		}
	}
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
			for _, t := range terms {
				if !containsFold(line, t) {
					return false
				}
			}
			return true
		}

	default:
		// Case-insensitive substring match (allocation-free)
		lowerQuery := strings.ToLower(query)
		return func(line string) bool {
			return containsFold(line, lowerQuery)
		}
	}
}

// containsFold reports whether s contains substr (which must be lowercase).
// Equivalent to strings.Contains(strings.ToLower(s), substr) but avoids
// allocating a lowercased copy of s on each call. Critical for the content
// scan hot path (~74K lines per search).
func containsFold(s, lowerSubstr string) bool {
	n := len(lowerSubstr)
	if n == 0 {
		return true
	}
	if n > len(s) {
		return false
	}
	// Byte-at-a-time scan for ASCII case-insensitive match.
	// Works because query tokens in this codebase are ASCII identifiers.
	first := lowerSubstr[0]
	for i := 0; i <= len(s)-n; i++ {
		c := s[i]
		// Fast lowercase for ASCII letters
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != first {
			continue
		}
		// Check rest of substr
		match := true
		for j := 1; j < n; j++ {
			c2 := s[i+j]
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c2 != lowerSubstr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
