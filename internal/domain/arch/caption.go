package arch

import (
	"fmt"
	"strings"
)

// DeriveCaption computes the human-readable count string for a shard's manifest entry.
// This is a Go port of build_c4_mockup.py:901-926 for the three kinds we render.
//
// Format per kind:
//   buckets: "N groups · M members — heaviest: A → B ×k"
//   matrix:  "S dependencies · N modules · P mutual pairs: first"
//   table:   "N rows · ⚠ F flagged — first: X"
func DeriveCaption(s *Shard, findings []Finding) string {
	findingsSuffix := ""
	if count := len(findings); count > 0 {
		findingsSuffix = fmt.Sprintf(" · ⚠ %d findings", count)
	}

	switch s.Kind {
	case "buckets":
		return deriveBucketsCaption(s) + findingsSuffix

	case "matrix":
		return deriveMatrixCaption(s) + findingsSuffix

	case "table":
		return deriveTableCaption(s) + findingsSuffix

	default:
		return fmt.Sprintf("%s shard%s", s.Kind, findingsSuffix)
	}
}

func deriveBucketsCaption(s *Shard) string {
	nGroups := len(s.Buckets)
	nMembers := 0
	for _, b := range s.Buckets {
		nMembers += len(b.Members)
	}

	// Find heaviest edge (highest count).
	heavy := ""
	maxCount := 0
	for _, e := range s.Edges {
		if e.Count > maxCount {
			maxCount = e.Count
			heavy = fmt.Sprintf("%s → %s ×%d", e.Source, e.Target, e.Count)
		}
	}

	base := fmt.Sprintf("%d groups · %d members", nGroups, nMembers)
	if heavy != "" {
		base += " — heaviest: " + heavy
	}
	return base
}

func deriveMatrixCaption(s *Shard) string {
	n := len(s.Items)
	totalDeps := 0
	mutualPairs := 0
	firstMutual := ""

	for i := range s.Matrix {
		for j := range s.Matrix[i] {
			if s.Matrix[i][j] != nil {
				if v, ok := s.Matrix[i][j].(int); ok && v > 0 {
					totalDeps += v
					// Check for mutual pair [i][j] && [j][i].
					if i < j && i < len(s.Matrix) && j < len(s.Matrix[i]) {
						jRow := s.Matrix[j]
						if len(jRow) > i && jRow[i] != nil {
							if vji, ok2 := jRow[i].(int); ok2 && vji > 0 {
								mutualPairs++
								if firstMutual == "" && i < len(s.Items) && j < len(s.Items) {
									firstMutual = fmt.Sprintf("%s ↔ %s", s.Items[i], s.Items[j])
								}
							}
						}
					}
				}
			}
		}
	}

	base := fmt.Sprintf("%d dependencies · %d modules", totalDeps, n)
	if mutualPairs > 0 {
		base += fmt.Sprintf(" · %d mutual pairs: %s", mutualPairs, firstMutual)
	}
	return base
}

func deriveTableCaption(s *Shard) string {
	nRows := len(s.Rows)
	flagged := 0
	firstFlagged := ""
	for _, row := range s.Rows {
		if len(row) > 0 && strings.HasPrefix(row[0], "⚠ ") {
			flagged++
			if firstFlagged == "" {
				firstFlagged = strings.TrimPrefix(row[0], "⚠ ")
			}
		}
	}

	base := fmt.Sprintf("%d rows", nRows)
	if flagged > 0 {
		base += fmt.Sprintf(" · ⚠ %d flagged — first: %s", flagged, firstFlagged)
	}

	return base
}
