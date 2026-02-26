package index

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/corey/aoa/internal/ports"
)

// Domain holds the term->keyword mapping for enrichment.
type Domain struct {
	Terms map[string][]string // term -> keywords
}

// SearchObserver is called after every search with the query, options, results, and elapsed time.
// Used by the app layer to feed search signals into the learner.
type SearchObserver func(query string, opts ports.SearchOptions, result *SearchResult, elapsed time.Duration)

// SearchEngine is the in-memory search index with domain enrichment.
type SearchEngine struct {
	idx         *ports.Index
	domains     map[string]Domain
	projectRoot string // project root for content scanning; empty disables it

	// refToTokens maps each TokenRef back to its token list.
	// Built from the inverted index during construction.
	refToTokens map[ports.TokenRef][]string

	// tokenDocFreq counts how many symbols contain each token.
	tokenDocFreq map[string]int

	// keywordToTerms maps keyword -> unique term names (reverse of Domain.Terms).
	// Used to resolve raw tokens to atlas terms for tag generation.
	keywordToTerms map[string][]string

	// fileSpans caches per-file symbol spans for content scanning.
	// Rebuilt in NewSearchEngine() and Rebuild() instead of per-search.
	fileSpans map[uint32][]symbolSpan

	// cache holds pre-read file contents to avoid disk I/O during search.
	cache *FileCache

	// observer is called after each search to emit learning signals.
	observer   SearchObserver
	observerWg sync.WaitGroup

	// debug enables phase timing output (AOA_DEBUG=1).
	debug bool
}

// SearchResult holds the output of a search operation.
type SearchResult struct {
	Hits            []Hit
	Count           int // for -c mode
	ExitCode        int // for -q mode (0=found, 1=not)
	TotalMatchChars int // L9.6: total chars of all matches before truncation
}

// Hit represents a single search result entry.
type Hit struct {
	File         string         `json:"file"`
	Line         int            `json:"line"`
	Symbol       string         `json:"symbol"`
	Range        [2]int         `json:"range"`
	Domain       string         `json:"domain"`
	Tags         []string       `json:"tags"`
	Kind         string         `json:"kind,omitempty"`          // "symbol" or "content"
	Content      string         `json:"content,omitempty"`       // matching line text (content hits only)
	ContextLines map[int]string `json:"context_lines,omitempty"` // lineNum → text (surrounding context)

	fileID uint32         // internal, for deterministic sorting
	ref    ports.TokenRef // internal, for deferred enrichment
}

// NewSearchEngine creates an engine from an existing index + domain map.
// If projectRoot is non-empty, content scanning (grep-style body matches) is enabled.
func NewSearchEngine(idx *ports.Index, domains map[string]Domain, projectRoot string) *SearchEngine {
	e := &SearchEngine{
		idx:            idx,
		domains:        domains,
		projectRoot:    projectRoot,
		refToTokens:    make(map[ports.TokenRef][]string),
		tokenDocFreq:   make(map[string]int),
		keywordToTerms: make(map[string][]string),
		debug:          os.Getenv("AOA_DEBUG") == "1",
	}

	// Build reverse map: ref -> tokens, and doc frequency
	for tok, refs := range idx.Tokens {
		e.tokenDocFreq[tok] = len(refs)
		for _, ref := range refs {
			e.refToTokens[ref] = append(e.refToTokens[ref], tok)
		}
	}

	// Build keyword -> terms reverse lookup from atlas domains.
	// Domain.Terms maps term -> []keyword; invert to keyword -> []term.
	kwTermSeen := make(map[string]map[string]bool) // keyword -> set of terms
	for _, domain := range domains {
		for term, keywords := range domain.Terms {
			for _, kw := range keywords {
				kwLower := strings.ToLower(kw)
				if kwTermSeen[kwLower] == nil {
					kwTermSeen[kwLower] = make(map[string]bool)
				}
				if !kwTermSeen[kwLower][term] {
					kwTermSeen[kwLower][term] = true
					e.keywordToTerms[kwLower] = append(e.keywordToTerms[kwLower], term)
				}
			}
		}
	}

	// Pre-compute file spans for content scanning
	e.fileSpans = e.buildFileSpans()

	return e
}

// Cache returns the attached FileCache (may be nil).
func (e *SearchEngine) Cache() *FileCache { return e.cache }

// SetCache attaches a FileCache for content scanning.
// Call WarmCache() separately to populate it from disk.
func (e *SearchEngine) SetCache(c *FileCache) {
	e.cache = c
}

// WarmCache populates the file cache from the current index.
// This is IO-heavy and may take seconds on large repos.
func (e *SearchEngine) WarmCache() {
	if e.cache != nil && e.projectRoot != "" {
		e.cache.WarmFromIndex(e.idx.Files, e.projectRoot)
	}
}

// UpdateCacheFile updates a single file in the cache and rebuilds content indices.
// O(1) disk I/O + O(cached-lines) index rebuild. Use after file changes instead of WarmCache.
func (e *SearchEngine) UpdateCacheFile(fileID uint32, absPath string, fileSize int64) {
	if e.cache != nil {
		e.cache.UpdateFile(fileID, absPath, fileSize)
	}
}

// RemoveCacheFile removes a file from the cache and rebuilds content indices.
func (e *SearchEngine) RemoveCacheFile(fileID uint32) {
	if e.cache != nil {
		e.cache.RemoveFile(fileID)
	}
}

// Rebuild reconstructs derived maps (refToTokens, tokenDocFreq, keywordToTerms)
// from current index state. Must be called after any index mutation (add/remove symbols).
func (e *SearchEngine) Rebuild() {
	e.refToTokens = make(map[ports.TokenRef][]string)
	e.tokenDocFreq = make(map[string]int)

	for tok, refs := range e.idx.Tokens {
		e.tokenDocFreq[tok] = len(refs)
		for _, ref := range refs {
			e.refToTokens[ref] = append(e.refToTokens[ref], tok)
		}
	}

	kwTermSeen := make(map[string]map[string]bool)
	e.keywordToTerms = make(map[string][]string)
	for _, domain := range e.domains {
		for term, keywords := range domain.Terms {
			for _, kw := range keywords {
				kwLower := strings.ToLower(kw)
				if kwTermSeen[kwLower] == nil {
					kwTermSeen[kwLower] = make(map[string]bool)
				}
				if !kwTermSeen[kwLower][term] {
					kwTermSeen[kwLower][term] = true
					e.keywordToTerms[kwLower] = append(e.keywordToTerms[kwLower], term)
				}
			}
		}
	}

	// Rebuild file spans for content scanning
	e.fileSpans = e.buildFileSpans()
}

// SetObserver registers a callback invoked after every search.
func (e *SearchEngine) SetObserver(obs SearchObserver) {
	e.observer = obs
}

// WaitObservers blocks until all in-flight observer goroutines complete.
// Used by tests to avoid races between async observers and assertions.
func (e *SearchEngine) WaitObservers() {
	e.observerWg.Wait()
}

// Search executes a query with the given options.
func (e *SearchEngine) Search(query string, opts ports.SearchOptions) *SearchResult {
	start := time.Now()

	maxCount := opts.MaxCount
	if maxCount <= 0 {
		maxCount = 20
	}

	var hits []Hit

	// Symbol search: skip when no symbols are indexed (lean/tokenization-only mode).
	// This avoids O(n) iteration over content-only token posting lists.
	tSym := time.Now()
	if len(e.idx.Metadata) > 0 {
		switch {
		case opts.Mode == "regex":
			hits = e.searchRegex(query, opts)
		case opts.AndMode:
			hits = e.searchAND(query, opts)
		default:
			tokens := Tokenize(query)
			if opts.Mode == "case_insensitive" {
				for i, t := range tokens {
					tokens[i] = strings.ToLower(t)
				}
			}
			if len(tokens) == 0 {
				return e.buildResult(nil, opts, maxCount)
			}
			if len(tokens) == 1 {
				hits = e.searchLiteral(tokens[0], opts)
			} else {
				hits = e.searchOR(tokens, opts)
			}
		}
	}
	if e.debug {
		fmt.Printf("[%s] [debug] search phase=symbols query=%q hits=%d elapsed=%v\n",
			time.Now().Format("15:04:05.000"), query, len(hits), time.Since(tSym))
	}

	// Invert symbol hits: replace with all symbols NOT in the matched set
	if opts.InvertMatch {
		hits = e.invertSymbolHits(hits, opts)
	}

	// Append content hits from file body scanning (grep-style)
	if e.projectRoot != "" {
		t0 := time.Now()
		contentHits := e.scanFileContents(query, opts, hits)
		hits = append(hits, contentHits...)
		if e.debug {
			fmt.Printf("[%s] [debug] search phase=content query=%q candidates=%d elapsed=%v\n",
				time.Now().Format("15:04:05.000"), query, len(contentHits), time.Since(t0))
		}
	}

	// -L: files-without-match — return files NOT in the matched set
	if opts.FilesWithoutMatch {
		hits = e.filesWithoutMatch(hits, opts)
	}

	// -o: only-matching — replace content with just the matching substring
	if opts.OnlyMatching {
		e.applyOnlyMatching(hits, query, opts)
	}

	result := e.buildResult(hits, opts, maxCount)

	// Enrich symbol hits (domain + tags) only for survivors after truncation
	t1 := time.Now()
	e.enrichHits(result.Hits)
	if e.debug {
		fmt.Printf("[%s] [debug] search phase=enrich query=%q elapsed=%v\n",
			time.Now().Format("15:04:05.000"), query, time.Since(t1))
	}

	// Generate tags for content hits that deferred tag generation (indexed paths)
	t2 := time.Now()
	e.fillContentTags(result.Hits)
	if e.debug {
		fmt.Printf("[%s] [debug] search phase=tags query=%q elapsed=%v\n",
			time.Now().Format("15:04:05.000"), query, time.Since(t2))
	}

	// Attach context lines for -A/-B/-C (after enrichment, on final result set only)
	e.attachContextLines(result.Hits, opts)

	elapsed := time.Since(start)

	if e.observer != nil {
		e.observerWg.Add(1)
		go func() {
			defer e.observerWg.Done()
			e.observer(query, opts, result, elapsed)
		}()
	}

	return result
}

// searchLiteral performs a single-term O(1) lookup.
// Results are in insertion order (file_id ascending, line ascending).
func (e *SearchEngine) searchLiteral(token string, opts ports.SearchOptions) []Hit {
	refs, ok := e.idx.Tokens[token]
	if !ok {
		return nil
	}

	var hits []Hit
	for _, ref := range refs {
		sym := e.idx.Metadata[ref]
		if sym == nil {
			continue
		}
		file := e.idx.Files[ref.FileID]
		if file == nil {
			continue
		}

		if opts.WordBoundary {
			if !e.refHasToken(ref, token) {
				continue
			}
		}

		if !matchesAllGlobs(file.Path, opts) {
			continue
		}

		hits = append(hits, e.buildHit(ref, sym, file))
	}

	// Refs are already in insertion order (file_id, line) from index construction.
	// No additional sort needed.
	return hits
}

// searchOR performs multi-term union.
// Sort: symbol density descending (symbols matching more query terms rank higher),
// then file ID ascending, then line ascending.
func (e *SearchEngine) searchOR(tokens []string, opts ports.SearchOptions) []Hit {
	// Build token set for quick lookup
	tokenSet := make(map[string]bool, len(tokens))
	for _, tok := range tokens {
		tokenSet[tok] = true
	}

	// Collect unique refs from all terms
	seen := make(map[ports.TokenRef]bool)
	var allRefs []ports.TokenRef
	for _, tok := range tokens {
		refs, ok := e.idx.Tokens[tok]
		if !ok {
			continue
		}
		for _, ref := range refs {
			if !seen[ref] {
				seen[ref] = true
				allRefs = append(allRefs, ref)
			}
		}
	}

	// Compute per-symbol density: how many query terms match this symbol's tokens
	refDensity := make(map[ports.TokenRef]int, len(allRefs))
	for _, ref := range allRefs {
		count := 0
		for _, tok := range e.refToTokens[ref] {
			if tokenSet[tok] {
				count++
			}
		}
		refDensity[ref] = count
	}

	// Sort by: (1) density descending, (2) file ID ascending, (3) line ascending
	sort.SliceStable(allRefs, func(i, j int) bool {
		di, dj := refDensity[allRefs[i]], refDensity[allRefs[j]]
		if di != dj {
			return di > dj
		}
		if allRefs[i].FileID != allRefs[j].FileID {
			return allRefs[i].FileID < allRefs[j].FileID
		}
		return allRefs[i].Line < allRefs[j].Line
	})

	var hits []Hit
	for _, ref := range allRefs {
		sym := e.idx.Metadata[ref]
		if sym == nil {
			continue
		}
		file := e.idx.Files[ref.FileID]
		if file == nil {
			continue
		}

		if opts.WordBoundary {
			hasAny := false
			for _, tok := range tokens {
				if e.refHasToken(ref, tok) {
					hasAny = true
					break
				}
			}
			if !hasAny {
				continue
			}
		}

		if !matchesAllGlobs(file.Path, opts) {
			continue
		}

		hits = append(hits, e.buildHit(ref, sym, file))
	}

	return hits
}

// searchAND performs multi-term intersection.
func (e *SearchEngine) searchAND(query string, opts ports.SearchOptions) []Hit {
	termStrs := strings.Split(query, ",")
	var tokens []string
	for _, t := range termStrs {
		t = strings.TrimSpace(t)
		tokenized := Tokenize(t)
		tokens = append(tokens, tokenized...)
	}

	if len(tokens) == 0 {
		return nil
	}

	// For each token, collect symbol refs into a set.
	// Intersect: only symbols present in ALL token sets.
	var sets []map[ports.TokenRef]bool
	for _, tok := range tokens {
		refs, ok := e.idx.Tokens[tok]
		if !ok {
			return nil // one term missing -> empty intersection
		}
		s := make(map[ports.TokenRef]bool, len(refs))
		for _, ref := range refs {
			s[ref] = true
		}
		sets = append(sets, s)
	}

	// Start with smallest set for efficiency
	sort.Slice(sets, func(i, j int) bool {
		return len(sets[i]) < len(sets[j])
	})

	result := sets[0]
	for i := 1; i < len(sets); i++ {
		intersected := make(map[ports.TokenRef]bool)
		for ref := range result {
			if sets[i][ref] {
				intersected[ref] = true
			}
		}
		result = intersected
		if len(result) == 0 {
			return nil
		}
	}

	var hits []Hit
	for ref := range result {
		sym := e.idx.Metadata[ref]
		if sym == nil {
			continue
		}
		file := e.idx.Files[ref.FileID]
		if file == nil {
			continue
		}

		if !matchesAllGlobs(file.Path, opts) {
			continue
		}

		hits = append(hits, e.buildHit(ref, sym, file))
	}

	sortByFileIDLine(hits)
	return hits
}

// searchRegex compiles a regex and scans all symbols.
func (e *SearchEngine) searchRegex(pattern string, opts ports.SearchOptions) []Hit {
	if opts.WordBoundary {
		pattern = `\b` + pattern + `\b`
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	var hits []Hit
	for ref, sym := range e.idx.Metadata {
		if sym == nil {
			continue
		}
		file := e.idx.Files[ref.FileID]
		if file == nil {
			continue
		}

		if !re.MatchString(sym.Name) {
			continue
		}

		if !matchesAllGlobs(file.Path, opts) {
			continue
		}

		hits = append(hits, e.buildHit(ref, sym, file))
	}

	sortByFileIDLine(hits)
	return hits
}

// invertSymbolHits returns all symbols NOT in the matched set, respecting glob filters.
func (e *SearchEngine) invertSymbolHits(matched []Hit, opts ports.SearchOptions) []Hit {
	// Build set of matched (fileID, line) pairs
	type fileLine struct {
		fileID uint32
		line   uint16
	}
	matchSet := make(map[fileLine]bool, len(matched))
	for _, h := range matched {
		matchSet[fileLine{h.fileID, uint16(h.Line)}] = true
	}

	// Collect all symbols NOT in the matched set
	var inverted []Hit
	for ref, sym := range e.idx.Metadata {
		if sym == nil {
			continue
		}
		if matchSet[fileLine{ref.FileID, ref.Line}] {
			continue
		}
		file := e.idx.Files[ref.FileID]
		if file == nil {
			continue
		}
		if !matchesAllGlobs(file.Path, opts) {
			continue
		}
		inverted = append(inverted, e.buildHit(ref, sym, file))
	}

	sortByFileIDLine(inverted)
	return inverted
}

// buildHit constructs a Hit from a ref, symbol, and file.
func (e *SearchEngine) buildHit(ref ports.TokenRef, sym *ports.SymbolMeta, file *ports.FileMeta) Hit {
	return Hit{
		File:   file.Path,
		Line:   int(ref.Line),
		Symbol: FormatSymbol(sym),
		Range:  [2]int{int(sym.StartLine), int(sym.EndLine)},
		Kind:   "symbol",
		fileID: ref.FileID,
		ref:    ref,
	}
}

// enrichHits fills in Domain and Tags for symbol hits that survived truncation.
// Called after buildResult to avoid enriching hundreds of hits that get discarded.
func (e *SearchEngine) enrichHits(hits []Hit) {
	for i := range hits {
		if hits[i].Kind != "symbol" {
			continue
		}
		hits[i].Domain = e.assignDomain(hits[i].ref)
		hits[i].Tags = e.generateTags(hits[i].ref)
	}
}

// buildResult applies count/quiet modes and max_count truncation.
func (e *SearchEngine) buildResult(hits []Hit, opts ports.SearchOptions, maxCount int) *SearchResult {
	if opts.Quiet {
		exitCode := 1
		if len(hits) > 0 {
			exitCode = 0
		}
		return &SearchResult{ExitCode: exitCode}
	}

	if opts.CountOnly {
		return &SearchResult{Count: len(hits)}
	}

	// L9.6: Compute TotalMatchChars before truncation for counterfactual comparison.
	// This is the total chars that all hits would produce if returned untruncated.
	var totalMatchChars int
	for _, h := range hits {
		totalMatchChars += len(h.File) + len(h.Content) + 10 // file:line:content\n
	}

	if len(hits) > maxCount {
		hits = hits[:maxCount]
	}

	return &SearchResult{Hits: hits, TotalMatchChars: totalMatchChars}
}

// refHasToken checks if a ref's token list contains the exact token.
func (e *SearchEngine) refHasToken(ref ports.TokenRef, token string) bool {
	for _, t := range e.refToTokens[ref] {
		if t == token {
			return true
		}
	}
	return false
}

// matchesAllGlobs applies include/exclude glob filters and directory exclusion on the file path.
func matchesAllGlobs(path string, opts ports.SearchOptions) bool {
	if !matchesGlobs(path, opts.IncludeGlob, opts.ExcludeGlob) {
		return false
	}
	if opts.ExcludeDirGlob != "" {
		dir := filepath.Dir(path)
		if fnmatchGlob(opts.ExcludeDirGlob, dir) {
			return false
		}
	}
	return true
}

// matchesGlobs applies include/exclude glob filters on the file path.
// Uses fnmatch-like semantics where * matches path separators (unlike filepath.Match).
func matchesGlobs(path, include, exclude string) bool {
	if include != "" {
		if !fnmatchGlob(include, path) {
			return false
		}
	}
	if exclude != "" {
		if fnmatchGlob(exclude, path) {
			return false
		}
	}
	return true
}

// fnmatchGlob matches a glob pattern against a path, allowing * to match /.
// Converts the glob to a regex: * → .*, ? → ., rest escaped.
func fnmatchGlob(pattern, path string) bool {
	var regexBuf strings.Builder
	regexBuf.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			regexBuf.WriteString(".*")
		case '?':
			regexBuf.WriteByte('.')
		case '.', '+', '(', ')', '{', '}', '[', ']', '^', '$', '|', '\\':
			regexBuf.WriteByte('\\')
			regexBuf.WriteByte(pattern[i])
		default:
			regexBuf.WriteByte(pattern[i])
		}
	}
	regexBuf.WriteString("$")
	re, err := regexp.Compile(regexBuf.String())
	if err != nil {
		return false
	}
	return re.MatchString(path)
}

// filesWithoutMatch returns one hit per indexed file that is NOT in the matched set.
// Respects include/exclude/exclude-dir glob filters from opts.
func (e *SearchEngine) filesWithoutMatch(matched []Hit, opts ports.SearchOptions) []Hit {
	matchedFiles := make(map[uint32]bool, len(matched))
	for _, h := range matched {
		matchedFiles[h.fileID] = true
	}

	var hits []Hit
	for fileID, file := range e.idx.Files {
		if matchedFiles[fileID] {
			continue
		}
		if !matchesAllGlobs(file.Path, opts) {
			continue
		}
		hits = append(hits, Hit{
			File:   file.Path,
			Kind:   "file",
			fileID: fileID,
		})
	}
	sortByFileIDLine(hits)
	return hits
}

// applyOnlyMatching replaces hit content with just the matching substring.
func (e *SearchEngine) applyOnlyMatching(hits []Hit, query string, opts ports.SearchOptions) {
	for i := range hits {
		h := &hits[i]
		if h.Kind == "content" && h.Content != "" {
			match := extractMatch(h.Content, query, opts)
			if match != "" {
				h.Content = match
			}
		} else if h.Kind == "symbol" && h.Symbol != "" {
			match := extractMatch(h.Symbol, query, opts)
			if match != "" {
				h.Symbol = match
			}
		}
	}
}

// extractMatch finds the matching substring within text for the given query/opts.
func extractMatch(text, query string, opts ports.SearchOptions) string {
	if opts.Mode == "regex" {
		re, err := regexp.Compile(query)
		if err != nil {
			return ""
		}
		return re.FindString(text)
	}
	if opts.Mode == "case_insensitive" {
		lowerText := strings.ToLower(text)
		lowerQuery := strings.ToLower(query)
		idx := strings.Index(lowerText, lowerQuery)
		if idx >= 0 {
			return text[idx : idx+len(query)]
		}
		return ""
	}
	idx := strings.Index(text, query)
	if idx >= 0 {
		return text[idx : idx+len(query)]
	}
	return ""
}

// attachContextLines populates ContextLines for content hits that have FileCache data.
func (e *SearchEngine) attachContextLines(hits []Hit, opts ports.SearchOptions) {
	before := opts.BeforeContext
	after := opts.AfterContext
	if opts.Context > 0 {
		before = opts.Context
		after = opts.Context
	}
	if before == 0 && after == 0 {
		return
	}
	if e.cache == nil {
		return
	}

	for i := range hits {
		h := &hits[i]
		if h.Kind != "content" || h.fileID == 0 {
			continue
		}

		lines := e.cache.GetLines(h.fileID)
		if lines == nil {
			continue
		}

		ctx := make(map[int]string)
		totalLines := len(lines)

		// Before context
		for j := h.Line - before; j < h.Line; j++ {
			if j >= 1 && j <= totalLines {
				ctx[j] = lines[j-1] // lines is 0-based, j is 1-based
			}
		}

		// After context
		for j := h.Line + 1; j <= h.Line+after; j++ {
			if j >= 1 && j <= totalLines {
				ctx[j] = lines[j-1]
			}
		}

		if len(ctx) > 0 {
			h.ContextLines = ctx
		}
	}
}

// sortByFileIDLine sorts hits by file_id ascending, then line ascending.
func sortByFileIDLine(hits []Hit) {
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].fileID != hits[j].fileID {
			return hits[i].fileID < hits[j].fileID
		}
		return hits[i].Line < hits[j].Line
	})
}
