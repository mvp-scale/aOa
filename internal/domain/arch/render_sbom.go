package arch

import (
	"fmt"
	"sort"
)

// sbomRowsMax caps the SBOM table at the shared simple-view node budget
// (view-standards.json global.budgets.simple_view_nodes_max) — tables aren't
// literally bound by that node budget, but reusing it keeps one shared
// "how much can one screen hold" number across the app rather than inventing
// a second, uncoordinated constant (risk flag: SBOM/TechStack tables can
// easily exceed 30 rows on a real monorepo — D23 requires the honest
// truncation caption below when that happens).
const sbomRowsMax = 30

// RenderSBOM produces a "table" shard answering the SBOM question
// (view-standards.json:239-249): "What components are inside, at what
// version, from where?" One row per detected dependency (in.Components,
// populated by the app layer from internal/adapters/lockfile readers).
//
// Columns: component, version, supplier, language, unpinned.
// Unpinned rows are flagged with the shared ⚠-prefix convention (column 0)
// so the reader can answer "are we exposed to X@version" — and unratified/
// unpinned exposure — from the table alone (view-standards pass criterion).
//
// Provenance: always MIXED (D2/D15) — versions/suppliers are read straight
// from the manifest (real), but component "naming" (e.g. replace-directive
// identity choices) carries reader heuristics, same honesty class as
// RenderContext's external-system naming.
func RenderSBOM(in RenderInput) (*Shard, error) {
	columns := []string{"component", "version", "supplier", "language", "unpinned"}

	sorted := make([]Component, len(in.Components))
	copy(sorted, in.Components)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Name != sorted[j].Name {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Version < sorted[j].Version
	})

	trueTotal := len(sorted)
	unpinnedTotal := 0
	for _, c := range sorted {
		if c.Unpinned {
			unpinnedTotal++
		}
	}

	shown := sorted
	if len(shown) > sbomRowsMax {
		shown = shown[:sbomRowsMax]
	}

	rows := make([][]string, 0, len(shown))
	for _, c := range shown {
		name := c.Name
		unpinned := ""
		if c.Unpinned {
			name = "⚠ " + name
			unpinned = "true"
		}
		rows = append(rows, []string{name, c.Version, c.Supplier, c.Language, unpinned})
	}
	if rows == nil {
		rows = [][]string{}
	}

	prov := Prov{Kind: "mixed", Label: "MIXED · manifest-derived specs · heuristic naming"}
	shard := &Shard{
		Kind:    "table",
		Title:   "Software Bill of Materials",
		Prov:    prov,
		Columns: columns,
		Rows:    rows,
	}
	_, shard.FindingsClause = DeriveCaption(shard, in.Findings)

	switch {
	case trueTotal == 0:
		shard.Count = "0 components"
	case trueTotal > len(shown):
		shard.Count = fmt.Sprintf("%d components (%d unpinned) — showing %d", trueTotal, unpinnedTotal, len(shown))
	case unpinnedTotal > 0:
		shard.Count = fmt.Sprintf("%d components (%d unpinned)", trueTotal, unpinnedTotal)
	default:
		shard.Count = fmt.Sprintf("%d components", trueTotal)
	}
	return shard, nil
}
