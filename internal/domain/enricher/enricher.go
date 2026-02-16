// Package enricher provides keyword->term->domain resolution from the universal atlas.
// It is the authority on domain ownership. The atlas is loaded once at startup from
// embedded JSON files and provides O(1) keyword lookup via a flat hash map.
//
// Shared keywords are allowed: a keyword like "retry" may belong to multiple domains.
// The caller (typically the search engine) uses scoring to pick the best domain.
package enricher

import (
	"io/fs"
)

// Enricher resolves keywords to their owning domains and terms.
type Enricher struct {
	atlas *Atlas
}

// New creates an Enricher from a loaded Atlas.
func New(atlas *Atlas) *Enricher {
	return &Enricher{atlas: atlas}
}

// NewFromFS loads an atlas from an fs.FS and creates an Enricher.
func NewFromFS(fsys fs.FS, dir string) (*Enricher, error) {
	atlas, err := LoadAtlas(fsys, dir)
	if err != nil {
		return nil, err
	}
	return New(atlas), nil
}

// Lookup returns all domain/term matches for a keyword. O(1).
// Shared keywords return multiple matches (one per owning domain/term pair).
// Returns nil for unknown keywords.
func (e *Enricher) Lookup(keyword string) []KeywordMatch {
	return e.atlas.Keywords[keyword]
}

// DomainDefs returns all domain definitions from the atlas.
func (e *Enricher) DomainDefs() []DomainDef {
	return e.atlas.Domains
}

// DomainTerms returns the full terms map for a domain.
// Returns nil if the domain is not found.
func (e *Enricher) DomainTerms(domain string) map[string][]string {
	for _, d := range e.atlas.Domains {
		if d.Domain == domain {
			return d.Terms
		}
	}
	return nil
}

// Stats returns atlas statistics: domain count, term count, total keyword entries, unique keywords.
func (e *Enricher) Stats() (domains, terms, keywordEntries, uniqueKeywords int) {
	return e.atlas.DomainCount, e.atlas.TermCount, e.atlas.KeywordEntries, e.atlas.UniqueKeywords
}
