// Package glossary harvests candidate glossary entries from the universal
// atlas (internal/domain/enricher). VL-1c (board #35): "atlas-term harvest
// (enricher data exists) -> Glossary table, provenance DECLARED/MIXED
// (candidates, not ratified — D2)".
//
// The atlas has no human-authored prose definition per term — DomainDef.Terms
// maps a term name to its owning keyword set (enricher/atlas.go). Harvest
// synthesizes a candidate definition from that keyword set rather than
// fabricating prose the atlas doesn't contain (D17 honesty: an atlas-derived
// keyword list is a real fact; an invented sentence would not be). Callers
// must present these as candidates, never as ratified definitions.
package glossary

import (
	"sort"
	"strings"

	"github.com/corey/aoa/internal/domain/enricher"
)

// Entry is one candidate glossary term, scoped to the domain that owns it.
// A term shared by multiple domains yields one Entry per owning domain,
// since its keyword set (and therefore its candidate definition) can differ
// per domain.
type Entry struct {
	Term       string
	Domain     string
	Definition string // candidate only — synthesized from the term's keyword set
}

// Harvest converts every atlas domain's term->keyword map into candidate
// glossary entries. Deterministic: sorted by (Domain, Term) ascending; each
// entry's Definition is a comma-joined, sorted keyword list.
func Harvest(domains []enricher.DomainDef) []Entry {
	entries := make([]Entry, 0, len(domains))
	for _, d := range domains {
		for term, keywords := range d.Terms {
			kw := append([]string(nil), keywords...)
			sort.Strings(kw)
			entries = append(entries, Entry{
				Term:       term,
				Domain:     d.Domain,
				Definition: strings.Join(kw, ", "),
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Domain != entries[j].Domain {
			return entries[i].Domain < entries[j].Domain
		}
		return entries[i].Term < entries[j].Term
	})
	return entries
}
