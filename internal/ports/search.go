package ports

// Searcher provides search and enrichment capabilities.
// Adapters use this interface instead of importing domain/index directly.
type Searcher interface {
	// Search executes a query and returns results.
	Search(query string, opts SearchOptions) *SearchResult

	// EnrichRef returns the domain and tags for a given token reference.
	EnrichRef(ref TokenRef) (domain string, tags []string)

	// ProjectRoot returns the project root path.
	ProjectRoot() string

	// FormatSymbol formats a SymbolMeta for display.
	FormatSymbol(sym *SymbolMeta) string
}

// SearchResult holds the output of a search operation.
// Adapters receive this from the Searcher interface.
type SearchResult struct {
	Hits            []SearchHit
	Count           int
	ExitCode        int
	TotalMatchChars int
}

// LineCache provides access to cached file contents by file ID.
type LineCache interface {
	GetLines(fileID uint32) []string
}

// SearchHit is a single result entry with all public fields.
type SearchHit struct {
	File         string
	Line         int
	Symbol       string
	Range        [2]int
	Domain       string
	Tags         []string
	Kind         string
	Content      string
	ContextLines map[int]string
	PeekCode     string // pre-computed by the Searcher implementation
}
