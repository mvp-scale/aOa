package index

import (
	"regexp"
	"sort"
	"strings"
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
	observer SearchObserver
}

// SearchResult holds the output of a search operation.
type SearchResult struct {
	Hits     []Hit
	Count    int // for -c mode
	ExitCode int // for -q mode (0=found, 1=not)
}

// Hit represents a single search result entry.
type Hit struct {
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Symbol  string   `json:"symbol"`
	Range   [2]int   `json:"range"`
	Domain  string   `json:"domain"`
	Tags    []string `json:"tags"`
	Kind    string   `json:"kind,omitempty"`    // "symbol" or "content"
	Content string   `json:"content,omitempty"` // matching line text (content hits only)

	fileID uint32 // internal, for deterministic sorting
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

// SetCache attaches a FileCache for content scanning and performs
// the initial cache warm from the current index.
func (e *SearchEngine) SetCache(c *FileCache) {
	e.cache = c
	if c != nil && e.projectRoot != "" {
		c.WarmFromIndex(e.idx.Files, e.projectRoot)
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

	// Re-warm file cache if attached
	if e.cache != nil {
		e.cache.WarmFromIndex(e.idx.Files, e.projectRoot)
	}
}

// SetObserver registers a callback invoked after every search.
func (e *SearchEngine) SetObserver(obs SearchObserver) {
	e.observer = obs
}

// Search executes a query with the given options.
func (e *SearchEngine) Search(query string, opts ports.SearchOptions) *SearchResult {
	start := time.Now()

	maxCount := opts.MaxCount
	if maxCount <= 0 {
		maxCount = 20
	}

	var hits []Hit

	switch {
	case opts.Mode == "regex":
		hits = e.searchRegex(query, opts)
	case opts.AndMode:
		hits = e.searchAND(query, opts)
	default:
		// Case insensitive: lowercase the query before tokenizing
		q := query
		if opts.Mode == "case_insensitive" {
			q = strings.ToLower(q)
		}
		tokens := Tokenize(q)
		if len(tokens) == 0 {
			return e.buildResult(nil, opts, maxCount)
		}
		if len(tokens) == 1 {
			hits = e.searchLiteral(tokens[0], opts)
		} else {
			hits = e.searchOR(tokens, opts)
		}
	}

	// Invert symbol hits: replace with all symbols NOT in the matched set
	if opts.InvertMatch {
		hits = e.invertSymbolHits(hits, opts)
	}

	// Append content hits from file body scanning (grep-style)
	if e.projectRoot != "" {
		contentHits := e.scanFileContents(query, opts, hits)
		hits = append(hits, contentHits...)
	}

	result := e.buildResult(hits, opts, maxCount)

	// Generate tags for content hits that deferred tag generation (indexed paths)
	e.fillContentTags(result.Hits)

	elapsed := time.Since(start)

	if e.observer != nil {
		e.observer(query, opts, result, elapsed)
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

		if !matchesGlobs(file.Path, opts.IncludeGlob, opts.ExcludeGlob) {
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

		if !matchesGlobs(file.Path, opts.IncludeGlob, opts.ExcludeGlob) {
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

		if !matchesGlobs(file.Path, opts.IncludeGlob, opts.ExcludeGlob) {
			continue
		}

		hits = append(hits, e.buildHit(ref, sym, file))
	}

	sortByFileIDLine(hits)
	return hits
}

// searchRegex compiles a regex and scans all symbols.
func (e *SearchEngine) searchRegex(pattern string, opts ports.SearchOptions) []Hit {
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

		if !matchesGlobs(file.Path, opts.IncludeGlob, opts.ExcludeGlob) {
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
		if !matchesGlobs(file.Path, opts.IncludeGlob, opts.ExcludeGlob) {
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
		Domain: e.assignDomain(ref),
		Tags:   e.generateTags(ref),
		Kind:   "symbol",
		fileID: ref.FileID,
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

	if len(hits) > maxCount {
		hits = hits[:maxCount]
	}

	return &SearchResult{Hits: hits}
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

// sortByFileIDLine sorts hits by file_id ascending, then line ascending.
func sortByFileIDLine(hits []Hit) {
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].fileID != hits[j].fileID {
			return hits[i].fileID < hits[j].fileID
		}
		return hits[i].Line < hits[j].Line
	})
}
