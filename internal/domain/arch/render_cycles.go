package arch

import (
	"fmt"
	"sort"
	"strings"
)

// RenderCycles produces a "table" shard from the pre-computed SCCs.
// The SCC list comes from TarjanSCC (shared with DetectCycles — built once).
//
// Columns: "cycle", "members", "size", "cheapest edge to cut"
// Each SCC with len > 1 becomes one row; ⚠-prefixed (flagged).
//
// Determinism: SCCs are pre-sorted by TarjanSCC; rows order mirrors SCC order.
func RenderCycles(in RenderInput) (*Shard, error) {
	// Build unit label lookup.
	labelOf := make(map[string]string, len(in.Units))
	for _, u := range in.Units {
		labelOf[u.ID] = u.Label
	}

	columns := []string{"cycle", "members", "size", "cheapest edge to cut"}
	var rows [][]string

	for _, scc := range in.SCCs {
		if len(scc) < 2 {
			continue
		}

		// Build readable cycle path: A → B → C → A.
		cycleStr := scc[0]
		for _, m := range scc[1:] {
			cycleStr += " → " + m
		}
		cycleStr += " → " + scc[0]

		// Members as comma-separated labels.
		memberLabels := make([]string, len(scc))
		for i, m := range scc {
			if lbl := labelOf[m]; lbl != "" {
				memberLabels[i] = lbl
			} else {
				memberLabels[i] = m
			}
		}
		sort.Strings(memberLabels)
		membersStr := strings.Join(memberLabels, ", ")

		// Reuse cheapest edge from the pre-computed cycle finding (set by DetectCycles).
		cheapestEdge := "unknown"
		for fi := range in.Findings {
			finding := &in.Findings[fi]
			if finding.Rule != "cycle" || len(finding.Subjects) != len(scc) || finding.CheapestCut == "" {
				continue
			}
			matched := true
			for i, s := range finding.Subjects {
				if s != scc[i] {
					matched = false
					break
				}
			}
			if matched {
				cheapestEdge = finding.CheapestCut
				break
			}
		}

		rows = append(rows, []string{
			"⚠ " + cycleStr,    // ⚠-prefixed: flagged row
			membersStr,
			fmt.Sprintf("%d", len(scc)),
			cheapestEdge,
		})
	}

	if rows == nil {
		rows = [][]string{}
	}

	shard := &Shard{
		Kind:    "table",
		Title:   "Dependency cycles",
		Prov:    Prov{Kind: "derived", Label: "REAL · imports + deterministic grouping"},
		Columns: columns,
		Rows:    rows,
	}
	shard.Count = DeriveCaption(shard, in.Findings)
	return shard, nil
}
