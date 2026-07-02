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
	// Build dep count lookup for cheapest-edge computation.
	edgeCount := make(map[string]map[string]int)
	for _, d := range in.Deps {
		if edgeCount[d.FromUnit] == nil {
			edgeCount[d.FromUnit] = make(map[string]int)
		}
		edgeCount[d.FromUnit][d.ToUnit] += d.Count
	}

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

		// Find cheapest edge (min count) within the SCC.
		sccSet := make(map[string]struct{}, len(scc))
		for _, m := range scc {
			sccSet[m] = struct{}{}
		}
		minCount := -1
		cheapestEdge := ""
		for _, from := range scc {
			for _, to := range scc {
				if from == to {
					continue
				}
				if c, ok := edgeCount[from][to]; ok {
					if minCount < 0 || c < minCount {
						minCount = c
						cheapestEdge = fmt.Sprintf("%s → %s (×%d)", from, to, c)
					}
				}
			}
		}
		if cheapestEdge == "" {
			cheapestEdge = "unknown"
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
		Prov:    Prov{Kind: "mixed", Label: "MIXED · imports real · grouping inferred"},
		Columns: columns,
		Rows:    rows,
	}
	shard.Count = DeriveCaption(shard, in.Findings)
	return shard, nil
}
