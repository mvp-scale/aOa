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

	// sortedDomainNames is the sorted list of domain names, cached to avoid
	// allocating and sorting on every assignDomainByKeywords call.
	sortedDomainNames []string

	// domainKeywordsLower maps raw keyword -> lowercased form, cached to avoid
	// per-call strings.ToLower in assignDomainByKeywords.
	domainKeywordsLower map[string]string

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

	// Pre-compute sorted domain names and lowercased keywords for enrichment.
	e.sortedDomainNames = make([]string, 0, len(domains))
	for name := range domains {
		e.sortedDomainNames = append(e.sortedDomainNames, name)
	}
	sort.Strings(e.sortedDomainNames)

	e.domainKeywordsLower = make(map[string]string)
	for _, domain := range domains {
		for _, keywords := range domain.Terms {
			for _, kw := range keywords {
				e.domainKeywordsLower[kw] = strings.ToLower(kw)
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

	// Pre-compute matching file IDs when glob filters are set.
	// This turns O(n) per-ref glob regex matches into O(1) map lookups,
	// and lets content scanning iterate only matching files instead of all files.
	var allowedFiles map[uint32]bool // nil = all files match
	hasGlobs := opts.IncludeGlob != "" || opts.ExcludeGlob != "" || opts.ExcludeDirGlob != ""
	if hasGlobs {
		tGlob := time.Now()
		allowedFiles = e.fileIDsMatchingGlob(opts)
		if e.debug {
			fmt.Printf("[%s] [debug] search phase=glob-prefilter query=%q matched=%d/%d elapsed=%v\n",
				time.Now().Format("15:04:05.000"), query, len(allowedFiles), len(e.idx.Files), time.Since(tGlob))
		}
		// Fast exit: if no files match the glob, no results are possible
		// (unless InvertMatch or FilesWithoutMatch needs the full set).
		if len(allowedFiles) == 0 && !opts.InvertMatch && !opts.FilesWithoutMatch {
			result := e.buildResult(nil, opts, maxCount)
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
	}

	var hits []Hit

	// Symbol search: skip when no symbols are indexed (lean/tokenization-only mode).
	// This avoids O(n) iteration over content-only token posting lists.
	tSym := time.Now()
	if len(e.idx.Metadata) > 0 {
		switch {
		case opts.Mode == "regex":
			hits = e.searchRegex(query, opts, allowedFiles)
		case opts.AndMode:
			hits = e.searchAND(query, opts, allowedFiles)
		case opts.WordBoundary:
			// Word boundary mode: match the full query as a complete word in
			// symbol names. The query is tokenized to find candidate refs via
			// the inverted index, but the final filter uses a case-insensitive
			// word-boundary regex against the symbol name. This prevents
			// camelCase-split fragments (e.g. "TTailerToCanonical" ->
			// [tailer, to, canonical]) from matching unrelated symbols that
			// happen to contain one fragment like "to".
			hits = e.searchWordBoundary(query, opts, maxCount, allowedFiles)
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
			// Early termination: cap collection when we don't need the
			// full set. CountOnly needs accurate total; InvertMatch needs
			// all matches to compute the inverse.
			symLimit := maxCount
			if opts.CountOnly || opts.InvertMatch {
				symLimit = 0 // 0 = no limit
			}
			if len(tokens) == 1 {
				hits = e.searchLiteral(tokens[0], opts, symLimit, allowedFiles)
			} else {
				hits = e.searchOR(tokens, opts, symLimit, allowedFiles)
			}
		}
	}
	if e.debug {
		fmt.Printf("[%s] [debug] search phase=symbols query=%q hits=%d elapsed=%v\n",
			time.Now().Format("15:04:05.000"), query, len(hits), time.Since(tSym))
	}

	// Invert symbol hits: replace with all symbols NOT in the matched set
	if opts.InvertMatch {
		hits = e.invertSymbolHits(hits, opts, allowedFiles)
	}

	// Append content hits from file body scanning (grep-style).
	// Skip when symbol search already found enough — content scan is the
	// fallback for when symbols are insufficient, not a supplement.
	if e.projectRoot != "" && (len(hits) < maxCount || opts.CountOnly) {
		t0 := time.Now()
		contentHits := e.scanFileContents(query, opts, hits, allowedFiles)
		hits = append(hits, contentHits...)
		if e.debug {
			fmt.Printf("[%s] [debug] search phase=content query=%q candidates=%d elapsed=%v\n",
				time.Now().Format("15:04:05.000"), query, len(contentHits), time.Since(t0))
		}
	}

	// -L: files-without-match — return files NOT in the matched set
	if opts.FilesWithoutMatch {
		hits = e.filesWithoutMatch(hits, opts, allowedFiles)
	}

	// -o: only-matching — replace content with just the matching substring
	if opts.OnlyMatching {
		e.applyOnlyMatching(hits, query, opts)
	}

	// L16.4: Scope filter — restrict results to files matching path substring.
	if opts.Scope != "" && len(hits) > 0 {
		hits = filterByScope(hits, opts.Scope)
	}

	// L16.2 + L16.3: Rank hits by relevance before truncation.
	// Exact symbol name matches surface first; generated files sink to bottom.
	// Skip for modes that already have their own ranking (OR uses density sort,
	// regex uses sortByFileIDLine after candidate filtering).
	if len(hits) > 1 && !opts.CountOnly && !opts.Quiet && !opts.InvertMatch &&
		opts.Mode != "regex" && !opts.AndMode {
		rankHits(hits, query)
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
// When limit > 0, stops after collecting that many hits (early termination).
// allowedFiles, when non-nil, restricts results to pre-computed matching file IDs.
func (e *SearchEngine) searchLiteral(token string, opts ports.SearchOptions, limit int, allowedFiles map[uint32]bool) []Hit {
	refs, ok := e.idx.Tokens[token]
	if !ok {
		return nil
	}

	var hits []Hit
	for _, ref := range refs {
		if allowedFiles != nil && !allowedFiles[ref.FileID] {
			continue
		}
		sym := e.idx.Metadata[ref]
		if sym == nil {
			continue
		}
		file := e.idx.Files[ref.FileID]
		if file == nil {
			continue
		}

		if allowedFiles == nil && !matchesAllGlobs(file.Path, opts) {
			continue
		}

		hits = append(hits, e.buildHit(ref, sym, file))
		if limit > 0 && len(hits) >= limit {
			break
		}
	}

	// Refs are already in insertion order (file_id, line) from index construction.
	// No additional sort needed.
	return hits
}

// searchWordBoundary implements -w (word boundary) search for symbol names.
// The query is tokenized and ALL query tokens must appear as exact tokens in
// the symbol's token list. This prevents camelCase-split fragments (e.g.
// "TTailerToCanonical" -> [tailer, to, canonical]) from matching unrelated
// symbols that happen to contain one common fragment like "to".
//
// For single-token queries this is equivalent to the old refHasToken check.
// For multi-token queries this is an AND-match on token lists, which is the
// correct word-boundary semantic: every word in the query must exist as a
// discrete word in the symbol name.
func (e *SearchEngine) searchWordBoundary(query string, opts ports.SearchOptions, maxCount int, allowedFiles map[uint32]bool) []Hit {
	// Tokenize query to find candidate refs via the inverted index.
	tokens := Tokenize(query)
	if opts.Mode == "case_insensitive" {
		for i, t := range tokens {
			tokens[i] = strings.ToLower(t)
		}
	}
	if len(tokens) == 0 {
		return nil
	}

	// Early termination limits
	symLimit := maxCount
	if opts.CountOnly || opts.InvertMatch {
		symLimit = 0
	}

	// Fast path for single-token queries: O(1) index lookup + refHasToken filter.
	if len(tokens) == 1 {
		return e.searchLiteral(tokens[0], opts, symLimit, allowedFiles)
	}

	// Multi-token path: collect candidate refs from ALL token posting lists,
	// then filter to only those refs that contain every query token.

	// Build token set for lookup
	tokenSet := make(map[string]bool, len(tokens))
	for _, tok := range tokens {
		tokenSet[tok] = true
	}

	// Start with the shortest posting list for efficiency (smallest first
	// strategy reduces the candidate set early).
	type postingEntry struct {
		token string
		refs  []ports.TokenRef
	}
	var postings []postingEntry
	for _, tok := range tokens {
		refs, ok := e.idx.Tokens[tok]
		if !ok {
			// If any query token has zero matches, no symbol can contain
			// ALL tokens. Return empty.
			return nil
		}
		postings = append(postings, postingEntry{tok, refs})
	}
	sort.Slice(postings, func(i, j int) bool {
		return len(postings[i].refs) < len(postings[j].refs)
	})

	// Build candidate set from the smallest posting list
	candidates := make(map[ports.TokenRef]bool, len(postings[0].refs))
	for _, ref := range postings[0].refs {
		candidates[ref] = true
	}

	// Intersect with remaining posting lists
	for i := 1; i < len(postings); i++ {
		next := make(map[ports.TokenRef]bool)
		for _, ref := range postings[i].refs {
			if candidates[ref] {
				next[ref] = true
			}
		}
		candidates = next
		if len(candidates) == 0 {
			return nil
		}
	}

	// Verify each candidate: all query tokens must appear in the ref's token list.
	// The intersection above used posting lists, but a ref can appear in a
	// posting list for token X even if it doesn't have token X as an exact match
	// (e.g., case-insensitive mode). Double-check with refHasToken.
	var hits []Hit
	for ref := range candidates {
		if allowedFiles != nil && !allowedFiles[ref.FileID] {
			continue
		}
		sym := e.idx.Metadata[ref]
		if sym == nil {
			continue
		}
		file := e.idx.Files[ref.FileID]
		if file == nil {
			continue
		}

		// Verify all query tokens are present as exact tokens in this ref
		allPresent := true
		for _, tok := range tokens {
			if !e.refHasToken(ref, tok) {
				allPresent = false
				break
			}
		}
		if !allPresent {
			continue
		}

		if allowedFiles == nil && !matchesAllGlobs(file.Path, opts) {
			continue
		}

		hits = append(hits, e.buildHit(ref, sym, file))
	}

	sortByFileIDLine(hits)

	// Apply limit after sorting (intersection candidates aren't insertion-ordered)
	if symLimit > 0 && len(hits) > symLimit {
		hits = hits[:symLimit]
	}

	return hits
}

// searchOR performs multi-term union.
// Sort: symbol density descending (symbols matching more query terms rank higher),
// then file ID ascending, then line ascending.
// When limit > 0, caps per-list collection at 5×limit to bound work while
// retaining enough candidates for accurate density ranking.
func (e *SearchEngine) searchOR(tokens []string, opts ports.SearchOptions, limit int, allowedFiles map[uint32]bool) []Hit {
	// Build token set for quick lookup
	tokenSet := make(map[string]bool, len(tokens))
	for _, tok := range tokens {
		tokenSet[tok] = true
	}

	// Per-list cap: collect at most this many refs from each posting list.
	// 5× limit gives enough candidates for density ranking after dedup.
	perListCap := 0
	if limit > 0 {
		perListCap = limit * 5
	}

	// Collect unique refs from all terms
	seen := make(map[ports.TokenRef]bool)
	var allRefs []ports.TokenRef
	for _, tok := range tokens {
		refs, ok := e.idx.Tokens[tok]
		if !ok {
			continue
		}
		collected := 0
		for _, ref := range refs {
			if !seen[ref] {
				seen[ref] = true
				allRefs = append(allRefs, ref)
				collected++
				if perListCap > 0 && collected >= perListCap {
					break
				}
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
		if allowedFiles != nil && !allowedFiles[ref.FileID] {
			continue
		}
		sym := e.idx.Metadata[ref]
		if sym == nil {
			continue
		}
		file := e.idx.Files[ref.FileID]
		if file == nil {
			continue
		}

		if allowedFiles == nil && !matchesAllGlobs(file.Path, opts) {
			continue
		}

		hits = append(hits, e.buildHit(ref, sym, file))
		if limit > 0 && len(hits) >= limit {
			break
		}
	}

	return hits
}

// searchAND performs multi-term intersection via sorted merge-join.
//
// Algorithm:
//  1. Tokenize each comma-separated term; deduplicate.
//  2. Look up posting lists; if any token has zero refs, return nil.
//  3. Sort posting lists by length (smallest first).
//  4. Intersect pairwise using sorted merge-join: walk two sorted posting
//     lists with two cursors, advancing the smaller one, emitting on match.
//     Each merge-join is O(|A|+|B|) with zero map allocation.
//  5. Chain intersections: result of each join feeds into the next.
//
// Posting lists are naturally sorted by (FileID, Line) from index construction,
// so merge-join works without any pre-sorting. This avoids the map[TokenRef]bool
// allocations that made the old approach O(sum_of_all_lists) in memory.
func (e *SearchEngine) searchAND(query string, opts ports.SearchOptions, allowedFiles map[uint32]bool) []Hit {
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

	// Deduplicate tokens to avoid redundant intersection passes.
	seenTok := make(map[string]bool, len(tokens))
	unique := tokens[:0]
	for _, tok := range tokens {
		if !seenTok[tok] {
			seenTok[tok] = true
			unique = append(unique, tok)
		}
	}
	tokens = unique

	// Single-token AND is just a literal lookup.
	if len(tokens) == 1 {
		return e.searchLiteral(tokens[0], opts, 0, allowedFiles)
	}

	// Collect posting lists and verify all tokens exist in the index.
	lists := make([][]ports.TokenRef, 0, len(tokens))
	for _, tok := range tokens {
		refs, ok := e.idx.Tokens[tok]
		if !ok {
			return nil // one term missing -> empty intersection
		}
		lists = append(lists, refs)
	}

	// Sort by posting list length (smallest first) for early reduction.
	sort.Slice(lists, func(i, j int) bool {
		return len(lists[i]) < len(lists[j])
	})

	// Chain pairwise merge-join intersections starting from the two smallest.
	// Each intersection produces a sorted result that feeds into the next.
	result := lists[0]
	for i := 1; i < len(lists); i++ {
		result = intersectSortedRefs(result, lists[i])
		if len(result) == 0 {
			return nil
		}
	}

	var hits []Hit
	for _, ref := range result {
		if allowedFiles != nil && !allowedFiles[ref.FileID] {
			continue
		}
		sym := e.idx.Metadata[ref]
		if sym == nil {
			continue
		}
		file := e.idx.Files[ref.FileID]
		if file == nil {
			continue
		}

		if allowedFiles == nil && !matchesAllGlobs(file.Path, opts) {
			continue
		}

		hits = append(hits, e.buildHit(ref, sym, file))
	}

	// Result from merge-join is already sorted by (FileID, Line).
	return hits
}

// intersectSortedRefs returns the intersection of two posting lists that are
// sorted by (FileID, Line). Uses a merge-join walk with two cursors -- O(|a|+|b|)
// time, O(|intersection|) space. Zero map allocation.
func intersectSortedRefs(a, b []ports.TokenRef) []ports.TokenRef {
	var out []ports.TokenRef
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ra, rb := a[i], b[j]
		if ra.FileID < rb.FileID || (ra.FileID == rb.FileID && ra.Line < rb.Line) {
			i++
		} else if ra.FileID > rb.FileID || (ra.FileID == rb.FileID && ra.Line > rb.Line) {
			j++
		} else {
			// Match: same (FileID, Line)
			out = append(out, ra)
			i++
			j++
		}
	}
	return out
}

// searchRegex compiles a regex and scans symbols.
// Optimization: extracts literal substrings from the pattern and uses the token
// index to narrow candidates before applying the regex. Falls back to full scan
// only when no literals can be extracted.
func (e *SearchEngine) searchRegex(pattern string, opts ports.SearchOptions, allowedFiles map[uint32]bool) []Hit {
	if opts.WordBoundary {
		pattern = `\b` + pattern + `\b`
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	// Try to extract literal substrings for pre-filtering via token index.
	// This narrows the candidate set from O(all symbols) to O(matching refs).
	candidates, hadLiterals := e.regexCandidateRefs(pattern)

	var hits []Hit
	if len(candidates) > 0 {
		// Fast path: check only candidate refs from token index
		for _, ref := range candidates {
			if allowedFiles != nil && !allowedFiles[ref.FileID] {
				continue
			}
			sym := e.idx.Metadata[ref]
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
			if allowedFiles == nil && !matchesAllGlobs(file.Path, opts) {
				continue
			}
			hits = append(hits, e.buildHit(ref, sym, file))
		}
	}
	if len(hits) == 0 && !(hadLiterals && len(candidates) == 0) {
		// Slow path: full-scan all symbols.
		// Skip when we extracted regex literals but found zero posting list refs —
		// if the literal tokens don't exist in the index, no symbol can match.
		hits = hits[:0]
		for ref, sym := range e.idx.Metadata {
			if allowedFiles != nil && !allowedFiles[ref.FileID] {
				continue
			}
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
			if allowedFiles == nil && !matchesAllGlobs(file.Path, opts) {
				continue
			}
			hits = append(hits, e.buildHit(ref, sym, file))
		}
	}

	sortByFileIDLine(hits)
	return hits
}

// regexCandidateRefs extracts literal substrings from a regex pattern and
// returns the union of their posting lists.
// Returns (nil, false) if no literals could be extracted (must full-scan).
// Returns (nil, true) if literals were found but no posting list matches exist (can skip scan).
// Returns (refs, true) if candidates were found.
func (e *SearchEngine) regexCandidateRefs(pattern string) (refs []ports.TokenRef, hadLiterals bool) {
	literals := extractRegexLiterals(pattern)
	if len(literals) == 0 {
		return nil, false
	}

	// Tokenize each literal and look up in the index
	seen := make(map[ports.TokenRef]bool)
	for _, lit := range literals {
		tokens := Tokenize(lit)
		for _, tok := range tokens {
			for _, ref := range e.idx.Tokens[tok] {
				if !seen[ref] {
					seen[ref] = true
					refs = append(refs, ref)
				}
			}
			// Also check lowercased version
			lower := strings.ToLower(tok)
			if lower != tok {
				for _, ref := range e.idx.Tokens[lower] {
					if !seen[ref] {
						seen[ref] = true
						refs = append(refs, ref)
					}
				}
			}
		}
	}

	return refs, true
}

// extractRegexLiterals is defined in content.go — uses regexp/syntax for
// accurate literal extraction. Shared by both symbol and content regex search.

// invertSymbolHits returns all symbols NOT in the matched set, respecting glob filters.
func (e *SearchEngine) invertSymbolHits(matched []Hit, opts ports.SearchOptions, allowedFiles map[uint32]bool) []Hit {
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
		if allowedFiles != nil && !allowedFiles[ref.FileID] {
			continue
		}
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
		if allowedFiles == nil && !matchesAllGlobs(file.Path, opts) {
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

	totalHits := len(hits)
	if len(hits) > maxCount {
		hits = hits[:maxCount]
	}

	// L9.6: Estimate TotalMatchChars for counterfactual comparison.
	// Compute on truncated set and scale up to avoid O(all-hits) loop.
	var sampleChars int
	for _, h := range hits {
		sampleChars += len(h.File) + len(h.Content) + 10 // file:line:content\n
	}
	totalMatchChars := sampleChars
	if len(hits) > 0 && totalHits > len(hits) {
		avgPerHit := sampleChars / len(hits)
		totalMatchChars = avgPerHit * totalHits
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

// fileIDsMatchingGlob pre-computes the set of file IDs whose paths match the
// include/exclude/exclude-dir globs. Returns nil when no globs are set (meaning
// all files match). This turns O(n) per-ref glob checks into O(1) map lookups.
func (e *SearchEngine) fileIDsMatchingGlob(opts ports.SearchOptions) map[uint32]bool {
	if opts.IncludeGlob == "" && opts.ExcludeGlob == "" && opts.ExcludeDirGlob == "" {
		return nil // no filtering needed
	}
	matched := make(map[uint32]bool)
	for fileID, file := range e.idx.Files {
		if matchesAllGlobs(file.Path, opts) {
			matched[fileID] = true
		}
	}
	return matched
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

// globCache caches compiled regexes for glob patterns.
// A sync.Map is ideal here: patterns are written once, read many times, and
// the set of unique patterns per search is tiny (1-3).
var globCache sync.Map // pattern string → *regexp.Regexp

// compileGlob converts a glob pattern to a compiled regex, caching the result.
func compileGlob(pattern string) *regexp.Regexp {
	if v, ok := globCache.Load(pattern); ok {
		return v.(*regexp.Regexp)
	}
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
		return nil
	}
	globCache.Store(pattern, re)
	return re
}

// fnmatchGlob matches a glob pattern against a path, allowing * to match /.
// Converts the glob to a regex: * → .*, ? → ., rest escaped.
// Compiled regexes are cached — first call compiles, subsequent calls are O(1) lookup.
func fnmatchGlob(pattern, path string) bool {
	re := compileGlob(pattern)
	if re == nil {
		return false
	}
	return re.MatchString(path)
}

// filesWithoutMatch returns one hit per indexed file that is NOT in the matched set.
// Respects include/exclude/exclude-dir glob filters from opts.
func (e *SearchEngine) filesWithoutMatch(matched []Hit, opts ports.SearchOptions, allowedFiles map[uint32]bool) []Hit {
	matchedFiles := make(map[uint32]bool, len(matched))
	for _, h := range matched {
		matchedFiles[h.fileID] = true
	}

	var hits []Hit
	for fileID, file := range e.idx.Files {
		if matchedFiles[fileID] {
			continue
		}
		if allowedFiles != nil && !allowedFiles[fileID] {
			continue
		}
		if allowedFiles == nil && !matchesAllGlobs(file.Path, opts) {
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

// PeekRef returns the fileID and startLine for this hit, suitable for peek.Encode.
// Uses Range[0] (the symbol start line) so that content hits within the same
// method naturally dedup to the same peek code.
func (h *Hit) PeekRef() (fileID uint32, startLine uint16) {
	return h.fileID, uint16(h.Range[0])
}

// ProjectRoot returns the project root path for this engine.
func (e *SearchEngine) ProjectRoot() string {
	return e.projectRoot
}

// EnrichRef returns the domain and tags for a given TokenRef.
// This wraps the private enrichment methods for use by the peek handler.
func (e *SearchEngine) EnrichRef(ref ports.TokenRef) (domain string, tags []string) {
	return e.assignDomain(ref), e.generateTags(ref)
}

// filterByScope keeps only hits whose file path contains the scope substring.
// Case-insensitive match so --scope tainteviction matches TaintEviction/.
func filterByScope(hits []Hit, scope string) []Hit {
	scopeLower := strings.ToLower(scope)
	filtered := hits[:0]
	for _, h := range hits {
		if strings.Contains(strings.ToLower(h.File), scopeLower) {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

// rankHits reorders hits by relevance:
//  1. Exact symbol name match (case-insensitive) → top
//  2. Symbol name contains query as substring → next
//  3. Normal results → middle
//  4. Generated/deepcopy files → bottom
//
// Within each tier, original order is preserved (stable sort).
func rankHits(hits []Hit, query string) {
	queryLower := strings.ToLower(query)
	sort.SliceStable(hits, func(i, j int) bool {
		return hitRank(hits[i], queryLower) < hitRank(hits[j], queryLower)
	})
}

// hitRank returns a sort key: lower = better rank.
//
//	0 = exact symbol name match
//	1 = symbol name contains query as substring
//	2 = normal result
//	3 = generated/deepcopy file (deprioritized)
func hitRank(h Hit, queryLower string) int {
	if isGeneratedFile(h.File) {
		return 3
	}
	if h.Kind == "symbol" {
		nameLower := strings.ToLower(symbolName(h.Symbol))
		if nameLower == queryLower {
			return 0
		}
		if strings.Contains(nameLower, queryLower) {
			return 1
		}
	}
	return 2
}

// symbolName extracts the bare function/method name from a formatted symbol.
// FormatSymbol produces: "Parent.signature(args)" or "signature(args)".
// We want just the function name part before the "(" and after the last ".".
func symbolName(formatted string) string {
	// Strip args: "handlePodUpdate(ctx)" → "handlePodUpdate"
	if idx := strings.IndexByte(formatted, '('); idx >= 0 {
		formatted = formatted[:idx]
	}
	// Strip parent: "Controller.handlePodUpdate" → "handlePodUpdate"
	if idx := strings.LastIndexByte(formatted, '.'); idx >= 0 {
		formatted = formatted[idx+1:]
	}
	return formatted
}

// isGeneratedFile returns true for auto-generated files that should be
// deprioritized in search results. Matches common Go codegen patterns.
func isGeneratedFile(path string) bool {
	base := filepath.Base(path)
	baseLower := strings.ToLower(base)
	switch {
	case strings.HasPrefix(baseLower, "zz_generated"):
		return true
	case strings.HasSuffix(baseLower, "_generated.go"):
		return true
	case strings.Contains(baseLower, "deepcopy"):
		return true
	case strings.Contains(baseLower, "conversion") && strings.HasPrefix(baseLower, "zz_"):
		return true
	}
	return false
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
