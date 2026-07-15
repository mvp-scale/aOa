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
	"github.com/corey/aoa/internal/ports"
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

// nonCodeLanguage marks ports.FileMeta.Language values that are prose/data
// formats rather than executable source. Their content is free-form English
// or structured-data vocabulary, not identifier vocabulary — including them
// swamps the keyword co-occurrence signal below (e.g. a project that embeds
// atlas/v1/*.json, or documents "checkout"/"pipeline"/"queue" in unrelated
// markdown prose, would otherwise "use" nearly every atlas keyword by
// construction, not because the project's code does).
var nonCodeLanguage = map[string]bool{
	"":         true, // unknown/unset
	"md":       true,
	"markdown": true,
	"json":     true,
	"yaml":     true,
	"yml":      true,
	"txt":      true,
	"html":     true,
	"csv":      true,
}

// HarvestFiltered is Harvest, filtered to the project's actual code
// vocabulary via idx (typically the daemon's live ports.Index): a term
// survives only when a MAJORITY of its keywords co-occur together in the
// SAME code file (VL-1.p1 punch: "any one keyword present anywhere in the
// project's raw token map" is a near no-op at real-project scale — atlas
// keywords are common short English/programming words, so with thousands of
// tokens nearly every term survives by incidental overlap, not project
// relevance). Requiring several keywords to cluster in one file restores
// locality: a real domain concept concentrates its vocabulary in the file(s)
// that implement it, which incidental single-word overlap does not. Files
// with a prose/data Language (nonCodeLanguage) are excluded from the
// co-occurrence evidence entirely. Same determinism contract as Harvest
// (sorted by Domain, then Term).
//
// This is a heuristic, not a precision filter: the atlas deliberately shares
// some keywords across unrelated domains (enricher.go), so a project touching
// generic software-engineering vocabulary can still surface an occasional
// wrong-domain term. The bar this clears is "project vocabulary, not the
// atlas dump" (a large, real reduction), not zero false positives.
func HarvestFiltered(domains []enricher.DomainDef, idx *ports.Index) []Entry {
	fileKeywords := buildCodeFileKeywordSets(idx)
	entries := make([]Entry, 0, len(domains))
	for _, d := range domains {
		for term, keywords := range d.Terms {
			if !keywordsCoOccur(keywords, fileKeywords) {
				continue
			}
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

// buildCodeFileKeywordSets inverts idx.Tokens into per-file keyword
// presence sets, restricted to files whose Language is actual source code
// (not nonCodeLanguage). O(total token refs) — built once per Harvest call.
func buildCodeFileKeywordSets(idx *ports.Index) map[uint32]map[string]bool {
	fileKeywords := make(map[uint32]map[string]bool)
	if idx == nil {
		return fileKeywords
	}
	for kw, refs := range idx.Tokens {
		for _, ref := range refs {
			fm := idx.Files[ref.FileID]
			if fm == nil || nonCodeLanguage[fm.Language] {
				continue
			}
			if fileKeywords[ref.FileID] == nil {
				fileKeywords[ref.FileID] = make(map[string]bool)
			}
			fileKeywords[ref.FileID][kw] = true
		}
	}
	return fileKeywords
}

// keywordsCoOccur reports whether a majority of keywords are all present
// together in at least one code file's keyword set.
func keywordsCoOccur(keywords []string, fileKeywords map[uint32]map[string]bool) bool {
	need := len(keywords)/2 + 1
	for _, kws := range fileKeywords {
		matched := 0
		for _, kw := range keywords {
			if kws[kw] {
				matched++
			}
		}
		if matched >= need {
			return true
		}
	}
	return false
}
