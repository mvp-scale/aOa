package arch

import (
	"fmt"
	"sort"
)

// techStackRowsMax caps the Tech Stack table (same shared budget rationale
// as sbomRowsMax — see its comment).
const techStackRowsMax = 30

// RenderTechStack produces a "table" shard for the Tech Stack view
// (view-standards.json id "techportfolio":228-238): "What technologies are
// we standing on, and where is the risk?" One row per detected technology —
// a source language (from FileMeta.Language, aggregated by the app layer) or
// a lockfile dependency (VL-1b: "language/framework scan + lockfile joins").
//
// Columns: technology, kind, count, where used.
//
// Honesty note (D17): the view-standard's "lifecycle/risk status" vital
// field has no real backing signal here (no EOL/CVE feed wired) — inventing
// one would be a worse failure than an incomplete-but-honest table (house
// ruling: no fabricated data). Unpinned is the one real risk proxy available
// today (an unpinned dependency is a genuine, derivable exposure) and is
// surfaced with the shared ⚠-prefix flagged-row convention.
//
// Provenance: always MIXED (D2) — language detection and dependency-name
// heuristics, same honesty class as RenderSBOM.
func RenderTechStack(in RenderInput) (*Shard, error) {
	columns := []string{"technology", "kind", "count", "where used"}

	sorted := make([]TechEntry, len(in.Technologies))
	copy(sorted, in.Technologies)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].Name < sorted[j].Name
	})

	trueTotal := len(sorted)
	unpinnedTotal := 0
	for _, te := range sorted {
		if te.Unpinned {
			unpinnedTotal++
		}
	}

	shown := sorted
	if len(shown) > techStackRowsMax {
		shown = shown[:techStackRowsMax]
	}

	rows := make([][]string, 0, len(shown))
	for _, te := range shown {
		name := te.Name
		if te.Unpinned {
			name = "⚠ " + name
		}
		rows = append(rows, []string{name, te.Kind, fmt.Sprintf("%d", te.Count), te.File})
	}
	if rows == nil {
		rows = [][]string{}
	}

	prov := Prov{Kind: "mixed", Label: "MIXED · language detection heuristic · lockfile joins"}
	shard := &Shard{
		Kind:    "table",
		Title:   "Tech Stack",
		Prov:    prov,
		Columns: columns,
		Rows:    rows,
	}
	_, shard.FindingsClause = DeriveCaption(shard, in.Findings)

	switch {
	case trueTotal == 0:
		shard.Count = "0 technologies"
	case trueTotal > len(shown):
		shard.Count = fmt.Sprintf("%d technologies (%d unpinned) — showing %d", trueTotal, unpinnedTotal, len(shown))
	case unpinnedTotal > 0:
		shard.Count = fmt.Sprintf("%d technologies (%d unpinned)", trueTotal, unpinnedTotal)
	default:
		shard.Count = fmt.Sprintf("%d technologies", trueTotal)
	}
	return shard, nil
}
