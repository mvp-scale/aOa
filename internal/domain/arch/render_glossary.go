package arch

import (
	"fmt"
	"sort"
)

// glossaryRowsMax caps the Glossary table (same shared budget rationale as
// sbomRowsMax — see its comment).
const glossaryRowsMax = 30

// RenderGlossary produces a "table" shard for the Glossary view
// (view-standards.json id "glossary":217-227): "What does each term of art
// mean here, precisely?" One row per candidate term (in.GlossaryTerms,
// populated by the app layer from internal/domain/glossary.Harvest over the
// atlas/enricher).
//
// Columns: term, definition, owning domain. Rows sort alphabetically by term
// (a glossary is a lookup, not a ranking — matches the reader's mental
// model of a dictionary, unlike SBOM/TechStack's flagged-first ordering).
//
// Provenance: always MIXED (D2) — the atlas groupings a term's definition is
// synthesized from are real, but the definition text itself is a candidate,
// not ratified human prose (VL-1c scope: "candidates, not ratified").
func RenderGlossary(in RenderInput) (*Shard, error) {
	columns := []string{"term", "definition", "owning domain"}

	sorted := make([]GlossaryEntry, len(in.GlossaryTerms))
	copy(sorted, in.GlossaryTerms)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Term != sorted[j].Term {
			return sorted[i].Term < sorted[j].Term
		}
		return sorted[i].Domain < sorted[j].Domain
	})

	trueTotal := len(sorted)
	shown := sorted
	if len(shown) > glossaryRowsMax {
		shown = shown[:glossaryRowsMax]
	}

	rows := make([][]string, 0, len(shown))
	for _, e := range shown {
		rows = append(rows, []string{e.Term, e.Definition, e.Domain})
	}
	if rows == nil {
		rows = [][]string{}
	}

	prov := Prov{Kind: "mixed", Label: "MIXED · atlas term candidates · not ratified"}
	shard := &Shard{
		Kind:    "table",
		Title:   "Glossary",
		Prov:    prov,
		Columns: columns,
		Rows:    rows,
	}
	_, shard.FindingsClause = DeriveCaption(shard, in.Findings)

	switch {
	case trueTotal == 0:
		shard.Count = "0 terms"
	case trueTotal > len(shown):
		shard.Count = fmt.Sprintf("%d terms — showing %d", trueTotal, len(shown))
	default:
		shard.Count = fmt.Sprintf("%d terms", trueTotal)
	}
	return shard, nil
}
