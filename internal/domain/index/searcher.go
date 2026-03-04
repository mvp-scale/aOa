package index

import (
	"github.com/corey/aoa/internal/peek"
	"github.com/corey/aoa/internal/ports"
)

// SearchAdapter wraps a SearchEngine to satisfy ports.Searcher.
// It converts internal domain types (with private fields) to ports types (all public).
type SearchAdapter struct {
	engine *SearchEngine
}

// Ensure SearchAdapter implements ports.Searcher at compile time.
var _ ports.Searcher = (*SearchAdapter)(nil)

// NewSearchAdapter creates a Searcher adapter for the given engine.
func NewSearchAdapter(engine *SearchEngine) *SearchAdapter {
	return &SearchAdapter{engine: engine}
}

// Search implements ports.Searcher by converting internal results to port types.
func (a *SearchAdapter) Search(query string, opts ports.SearchOptions) *ports.SearchResult {
	internal := a.engine.Search(query, opts)

	hits := make([]ports.SearchHit, len(internal.Hits))
	for i, h := range internal.Hits {
		hits[i] = ports.SearchHit{
			File:         h.File,
			Line:         h.Line,
			Symbol:       h.Symbol,
			Range:        h.Range,
			Domain:       h.Domain,
			Tags:         h.Tags,
			Kind:         h.Kind,
			Content:      h.Content,
			ContextLines: h.ContextLines,
		}
		// Compute peek code for symbols within MaxRange
		rangeSize := h.Range[1] - h.Range[0]
		if rangeSize > 0 && rangeSize <= peek.MaxRange {
			fileID, startLine := h.PeekRef()
			hits[i].PeekCode = peek.Encode(fileID, startLine)
		}
	}

	return &ports.SearchResult{
		Hits:            hits,
		Count:           internal.Count,
		ExitCode:        internal.ExitCode,
		TotalMatchChars: internal.TotalMatchChars,
	}
}

// EnrichRef implements ports.Searcher.
func (a *SearchAdapter) EnrichRef(ref ports.TokenRef) (domain string, tags []string) {
	return a.engine.EnrichRef(ref)
}

// ProjectRoot implements ports.Searcher.
func (a *SearchAdapter) ProjectRoot() string {
	return a.engine.ProjectRoot()
}

// FormatSymbol implements ports.Searcher.
func (a *SearchAdapter) FormatSymbol(sym *ports.SymbolMeta) string {
	return FormatSymbol(sym)
}
