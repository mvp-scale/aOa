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
// Uses trigram index when available for queries >= 3 chars (O(1) candidate lookup),
// falls back to brute-force for short queries, regex, InvertMatch, etc.
// Tags are deferred (nil) and filled after MaxCount truncation by fillContentTags.
func (e *SearchEngine) scanFileContents(query string, opts ports.SearchOptions, symbolHits []Hit) []Hit {
	if e.cache != nil && e.cache.HasTrigramIndex() && canUseTrigram(query, opts) {
		return e.scanContentTrigram(query, opts, symbolHits)
	}
	return e.scanContentBruteForce(query, opts, symbolHits)
}

// canUseTrigram returns true if the query/options combination can use the trigram index.
func canUseTrigram(query string, opts ports.SearchOptions) bool {
	if opts.InvertMatch || opts.WordBoundary || opts.AndMode || opts.Mode == "regex" {
		return false
	}
	return len(query) >= 3
}

// extractTrigrams extracts unique 3-byte substrings from a lowered query string.
func extractTrigrams(lowerQuery string) [][3]byte {
	n := len(lowerQuery)
	if n < 3 {
		return nil
	}
	seen := make(map[[3]byte]bool)
	var trigrams [][3]byte
	for i := 0; i <= n-3; i++ {
		key := [3]byte{lowerQuery[i], lowerQuery[i+1], lowerQuery[i+2]}
		if !seen[key] {
			seen[key] = true
			trigrams = append(trigrams, key)
		}
	}
	return trigrams
}

// scanContentTrigram uses the trigram index to find candidate lines, then verifies
// each candidate with the appropriate matcher (case-sensitive or case-insensitive).
func (e *SearchEngine) scanContentTrigram(query string, opts ports.SearchOptions, symbolHits []Hit) []Hit {
	lowerQuery := strings.ToLower(query)
	trigrams := extractTrigrams(lowerQuery)
	if len(trigrams) == 0 {
		return e.scanContentBruteForce(query, opts, symbolHits)
	}

	candidates := e.cache.TrigramLookup(trigrams)
	if len(candidates) == 0 {
		return nil
	}

	dedup := make(map[contentFileLine]bool, len(symbolHits))
	for _, h := range symbolHits {
		dedup[contentFileLine{h.fileID, h.Line}] = true
	}

	caseSensitive := opts.Mode != "case_insensitive"

	var hits []Hit
	for _, cand := range candidates {
		lineNum := int(cand.LineNum)
		lineIdx := lineNum - 1

		fl := contentFileLine{cand.FileID, lineNum}
		if dedup[fl] {
			continue
		}

		file := e.idx.Files[cand.FileID]
		if file == nil || file.Size > maxContentFileSize {
			continue
		}
		if !matchesGlobs(file.Path, opts.IncludeGlob, opts.ExcludeGlob) {
			continue
		}

		lines := e.cache.GetLines(cand.FileID)
		if lines == nil || lineIdx >= len(lines) {
			continue
		}

		// Verify candidate: trigram index is case-insensitive, so we need exact check
		if caseSensitive {
			if !strings.Contains(lines[lineIdx], query) {
				continue
			}
		} else {
			lowerLines := e.cache.GetLowerLines(cand.FileID)
			if lowerLines == nil || lineIdx >= len(lowerLines) || !strings.Contains(lowerLines[lineIdx], lowerQuery) {
				continue
			}
		}

		dedup[fl] = true

		spans := e.fileSpans[cand.FileID]
		hit := e.buildContentHit(cand.FileID, file.Path, lineNum, lines[lineIdx], spans)
		hits = append(hits, hit)
	}

	sortByFileIDLine(hits)
	return hits
}

// scanContentBruteForce is the full-scan path: iterates every line of
// every cached/on-disk file running a matcher function per line.
// When pre-lowered lines are available and mode is case-insensitive,
// uses strings.Contains on lowered lines instead of per-byte case folding.
func (e *SearchEngine) scanContentBruteForce(query string, opts ports.SearchOptions, symbolHits []Hit) []Hit {
	dedup := make(map[contentFileLine]bool, len(symbolHits))
	for _, h := range symbolHits {
		dedup[contentFileLine{h.fileID, h.Line}] = true
	}

	matcher := buildContentMatcher(query, opts)
	if matcher == nil {
		return nil
	}

	// Pre-lowered line optimization: for case-insensitive literal search,
	// use strings.Contains on pre-lowered lines (benefits from SIMD-optimized Index)
	useLowerOpt := opts.Mode == "case_insensitive" && !opts.WordBoundary && !opts.AndMode && e.cache != nil
	lowerQuery := strings.ToLower(query)

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

		// Get pre-lowered lines when available for faster case-insensitive matching
		var lowerLines []string
		if useLowerOpt {
			lowerLines = e.cache.GetLowerLines(fileID)
		}

		spans := e.fileSpans[fileID]

		for lineIdx, line := range lines {
			lineNum := lineIdx + 1
			var matched bool
			if lowerLines != nil && lineIdx < len(lowerLines) {
				matched = strings.Contains(lowerLines[lineIdx], lowerQuery)
			} else {
				matched = matcher(line)
			}
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
		// Comma-separated terms, all must appear (case-insensitive)
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

	case opts.Mode == "case_insensitive":
		// Case-insensitive substring match (allocation-free)
		lowerQuery := strings.ToLower(query)
		return func(line string) bool {
			return containsFold(line, lowerQuery)
		}

	default:
		// Case-sensitive substring match (grep default behavior)
		return func(line string) bool {
			return strings.Contains(line, query)
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
