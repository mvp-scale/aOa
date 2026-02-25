package index

import (
	"path/filepath"
	"regexp"
	"regexp/syntax"
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
// Uses trigram index when available for O(1) candidate lookup (G0: no O(n) on hot paths).
// Regex queries extract literal substrings for trigram narrowing, then verify with regex.
// Falls back to brute-force only for short queries, InvertMatch, or queries without literals.
// Tags are deferred (nil) and filled after MaxCount truncation by fillContentTags.
func (e *SearchEngine) scanFileContents(query string, opts ports.SearchOptions, symbolHits []Hit) []Hit {
	if e.cache == nil || !e.cache.HasTrigramIndex() {
		return e.scanContentBruteForce(query, opts, symbolHits)
	}

	// InvertMatch needs full scan (finding what DOESN'T match)
	if opts.InvertMatch {
		return e.scanContentBruteForce(query, opts, symbolHits)
	}

	// Regex: extract literals for trigram narrowing, verify with compiled regex
	if opts.Mode == "regex" {
		return e.scanContentRegexTrigram(query, opts, symbolHits)
	}

	// Literal/AND/word-boundary modes
	if opts.WordBoundary || opts.AndMode {
		return e.scanContentBruteForce(query, opts, symbolHits)
	}
	if len(query) >= 3 {
		return e.scanContentTrigram(query, opts, symbolHits)
	}
	return e.scanContentBruteForce(query, opts, symbolHits)
}

// scanContentRegexTrigram uses literal extraction from regex patterns to narrow
// candidates via trigram index, then verifies each candidate with the compiled regex.
// For concatenation (func.*Observer): intersects trigram results (both must appear).
// For alternation (A|B|C): unions trigram candidates across branches.
// Falls back to brute-force only when no usable literals can be extracted.
func (e *SearchEngine) scanContentRegexTrigram(pattern string, opts ports.SearchOptions, symbolHits []Hit) []Hit {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	// Extract structured literal group preserving concat vs alternation
	group := extractRegexLiteralGroup(pattern)
	if group == nil {
		return e.scanContentBruteForce(pattern, opts, symbolHits)
	}

	allCandidates := e.resolveLiteralGroup(group)
	if len(allCandidates) == 0 {
		return nil
	}

	// Verify each candidate with the compiled regex
	dedup := make(map[contentFileLine]bool, len(symbolHits))
	for _, h := range symbolHits {
		dedup[contentFileLine{h.fileID, h.Line}] = true
	}

	var hits []Hit
	for _, cand := range allCandidates {
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
		if !matchesAllGlobs(file.Path, opts) {
			continue
		}

		lines := e.cache.GetLines(cand.FileID)
		if lines == nil || lineIdx >= len(lines) {
			continue
		}

		if !re.MatchString(lines[lineIdx]) {
			continue
		}

		dedup[fl] = true
		spans := e.fileSpans[cand.FileID]
		hit := e.buildContentHit(cand.FileID, file.Path, lineNum, lines[lineIdx], spans)
		hits = append(hits, hit)
	}

	sortByFileIDLine(hits)
	return hits
}

// resolveLiteralGroup resolves a regexLiteralGroup into candidate contentRefs
// using the trigram index. Intersects within concatenations, unions across alternations.
func (e *SearchEngine) resolveLiteralGroup(g *regexLiteralGroup) []contentRef {
	// Handle alternation branches: union results from each branch
	if len(g.branches) > 0 {
		var result []contentRef
		for _, branch := range g.branches {
			branchCands := e.resolveLiteralGroup(&branch)
			if branchCands != nil {
				result = unionContentRefs(result, branchCands)
			}
		}
		return result
	}

	// Handle intersection: all literals must appear → intersect trigram results
	if len(g.intersect) == 0 {
		return nil
	}

	// Use the most selective literal (longest → fewest trigram candidates)
	// by intersecting trigram posting lists for ALL literals
	var result []contentRef
	first := true
	for _, lit := range g.intersect {
		trigrams := extractTrigrams(strings.ToLower(lit))
		if len(trigrams) == 0 {
			continue
		}
		candidates := e.cache.TrigramLookup(trigrams)
		if candidates == nil {
			return nil // one literal has zero matches → no results
		}
		if first {
			result = candidates
			first = false
		} else {
			result = intersectContentRefs(result, candidates)
			if len(result) == 0 {
				return nil
			}
		}
	}
	return result
}

// regexLiteralGroup represents extracted literals from a regex.
// For concatenation (func.*Observer): all literals must appear → intersect trigram results.
// For alternation (A|B|C): any branch can match → union trigram results across branches.
type regexLiteralGroup struct {
	// Intersect: all of these literals must appear on the same line (from concatenation).
	intersect []string
	// Branches: each branch is an independent alternative (from alternation).
	branches []regexLiteralGroup
}

// extractRegexLiterals parses a regex and extracts literal substrings (≥3 chars)
// organized by their logical relationship (intersect vs union).
func extractRegexLiterals(pattern string) []string {
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil
	}
	parsed = parsed.Simplify()

	var literals []string
	collectLiterals(parsed, &literals)

	var usable []string
	for _, lit := range literals {
		if len(lit) >= 3 {
			usable = append(usable, lit)
		}
	}
	return usable
}

// extractRegexLiteralGroup returns the structured literal group from a regex,
// preserving concat (intersect) vs alternation (union) relationships.
func extractRegexLiteralGroup(pattern string) *regexLiteralGroup {
	parsed, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil
	}
	parsed = parsed.Simplify()
	return buildLiteralGroup(parsed)
}

// buildLiteralGroup recursively builds a regexLiteralGroup from a parsed regex.
func buildLiteralGroup(re *syntax.Regexp) *regexLiteralGroup {
	switch re.Op {
	case syntax.OpLiteral:
		lit := string(re.Rune)
		if len(lit) >= 3 {
			return &regexLiteralGroup{intersect: []string{lit}}
		}
		return nil

	case syntax.OpConcat:
		// All pieces must match on the same line → intersect
		g := &regexLiteralGroup{}
		for _, sub := range re.Sub {
			sg := buildLiteralGroup(sub)
			if sg != nil {
				g.intersect = append(g.intersect, sg.intersect...)
			}
		}
		if len(g.intersect) == 0 {
			return nil
		}
		return g

	case syntax.OpAlternate:
		// Any branch can match → union across branches
		g := &regexLiteralGroup{}
		for _, sub := range re.Sub {
			sg := buildLiteralGroup(sub)
			if sg != nil {
				g.branches = append(g.branches, *sg)
			}
		}
		if len(g.branches) == 0 {
			return nil
		}
		return g

	case syntax.OpCapture:
		if len(re.Sub) > 0 {
			return buildLiteralGroup(re.Sub[0])
		}
		return nil

	default:
		return nil
	}
}

// collectLiterals recursively extracts literal strings from a parsed regex AST (flat list).
func collectLiterals(re *syntax.Regexp, out *[]string) {
	switch re.Op {
	case syntax.OpLiteral:
		*out = append(*out, string(re.Rune))
	case syntax.OpConcat, syntax.OpAlternate, syntax.OpCapture:
		for _, sub := range re.Sub {
			collectLiterals(sub, out)
		}
	}
}

// unionContentRefs merges two sorted contentRef slices, deduplicating.
func unionContentRefs(a, b []contentRef) []contentRef {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	result := make([]contentRef, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].FileID < b[j].FileID || (a[i].FileID == b[j].FileID && a[i].LineNum < b[j].LineNum) {
			result = append(result, a[i])
			i++
		} else if a[i].FileID > b[j].FileID || (a[i].FileID == b[j].FileID && a[i].LineNum > b[j].LineNum) {
			result = append(result, b[j])
			j++
		} else {
			result = append(result, a[i])
			i++
			j++
		}
	}
	result = append(result, a[i:]...)
	result = append(result, b[j:]...)
	return result
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
		if !matchesAllGlobs(file.Path, opts) {
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
		if !matchesAllGlobs(file.Path, opts) {
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
