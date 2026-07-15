package arch

import (
	"fmt"
	"sort"
)

// changeRowsMax caps the Change Map table (same shared budget rationale as
// sbomRowsMax/techStackRowsMax — see sbomRowsMax's comment).
const changeRowsMax = 30

// RenderChangeMap produces a "table" shard for the Change Map (view id
// "change") view (VL-2, board #36): "Where is churn combining with
// complexity to create risk?" One row per unit that changed within the
// bounded git-log window (in.ChurnEntries, populated by the app layer from a
// bounded git-history read joined with indexed symbol counts).
//
// Columns: unit, changed files, commits, complexity, risk.
// Risk = changed files * complexity (ChurnEntry.Risk) — the naive product is
// the view's whole premise: frequent change AND structural complexity
// together are the highest-risk combination, not either alone. Rows are
// sorted by risk descending so the riskiest unit is always row one (view-
// standards pass criterion: "name the single riskiest unit and why").
//
// Provenance: always MIXED (D2/D15) — commit/file counts are read straight
// from git history (real), but the commit-depth/time-window bound and the
// unit-path join are reader heuristics, same honesty class as RenderSBOM.
func RenderChangeMap(in RenderInput) (*Shard, error) {
	columns := []string{"unit", "changed files", "commits", "complexity", "risk"}

	sorted := make([]ChurnEntry, len(in.ChurnEntries))
	copy(sorted, in.ChurnEntries)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Risk != sorted[j].Risk {
			return sorted[i].Risk > sorted[j].Risk
		}
		return sorted[i].Path < sorted[j].Path
	})

	trueTotal := len(sorted)

	shown := sorted
	if len(shown) > changeRowsMax {
		shown = shown[:changeRowsMax]
	}

	rows := make([][]string, 0, len(shown))
	for _, c := range shown {
		rows = append(rows, []string{
			c.Path,
			fmt.Sprintf("%d", c.ChangedFiles),
			fmt.Sprintf("%d", c.Commits),
			fmt.Sprintf("%d", c.Complexity),
			fmt.Sprintf("%d", c.Risk),
		})
	}
	if rows == nil {
		rows = [][]string{}
	}

	prov := Prov{Kind: "mixed", Label: "MIXED · bounded git history · indexed complexity proxy"}
	shard := &Shard{
		Kind:    "table",
		Title:   "Change Map",
		Prov:    prov,
		Columns: columns,
		Rows:    rows,
	}
	_, shard.FindingsClause = DeriveCaption(shard, in.Findings)

	switch {
	case trueTotal == 0:
		shard.Count = "0 units changed"
	case trueTotal > len(shown):
		shard.Count = fmt.Sprintf("%d units changed — showing %d highest-risk", trueTotal, len(shown))
	default:
		shard.Count = fmt.Sprintf("%d units changed", trueTotal)
	}
	return shard, nil
}
