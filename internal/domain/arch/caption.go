package arch

import (
	"fmt"
	"strings"
)

// DeriveCaption computes the human-readable count string for a shard's manifest entry, split
// into a CALM caption (never mentions findings — house ruling "calm like a map") and a separate
// findingsClause the caller may append only when its Findings lens is on. This is a Go port of
// build_c4_mockup.py:901-926 for all five shard kinds, with the A3 calm-default split applied.
//
// Format per kind (caption, findingsClause):
//
//	buckets: "N groups · M members — heaviest: A → B ×k", " · ⚠ F findings"
//	matrix:  "S dependencies · N modules · P mutual pairs: first", ""
//	table:   "N rows", " · ⚠ F flagged — first: X"
//	entity:  "N entities", ""
//	simple:  "N nodes", ""
func DeriveCaption(s *Shard, findings []Finding) (caption, findingsClause string) {
	findingsSuffix := ""
	if count := len(findings); count > 0 {
		findingsSuffix = fmt.Sprintf(" · ⚠ %d findings", count)
	}

	switch s.Kind {
	case "buckets":
		return deriveBucketsCaption(s), findingsSuffix

	case "matrix":
		return deriveMatrixCaption(s), findingsSuffix

	case "table":
		return deriveTableCaption(s), findingsSuffix

	case "entity":
		return deriveEntityCaption(s), findingsSuffix

	case "simple":
		return deriveSimpleCaption(s), findingsSuffix

	default:
		return fmt.Sprintf("%s shard", s.Kind), findingsSuffix
	}
}

func deriveBucketsCaption(s *Shard) string {
	nGroups := len(s.Buckets)
	nMembers := 0
	// PA8/T-4 checkpoint F-8: ShardEdge.Source/Target hold internal bucket IDs
	// (e.g. "g_ext_std"), not display text — the header caption must show the
	// same plain-noun labels the dock uses (e.g. "ext:std"), never the slug.
	labelByID := make(map[string]string, len(s.Buckets))
	for _, b := range s.Buckets {
		nMembers += len(b.Members)
		labelByID[b.ID] = b.Label
	}

	// Find heaviest edge (highest count).
	heavy := ""
	maxCount := 0
	for _, e := range s.Edges {
		if e.Count > maxCount {
			maxCount = e.Count
			heavy = fmt.Sprintf("%s → %s ×%d", bucketLabel(labelByID, e.Source), bucketLabel(labelByID, e.Target), e.Count)
		}
	}

	base := fmt.Sprintf("%d groups · %d members", nGroups, nMembers)
	if heavy != "" {
		base += " — heaviest: " + heavy
	}
	return base
}

// bucketLabel resolves a bucket ID to its display label. Falls back to the
// raw ID only if the shard's own bucket list somehow lacks it (defensive;
// should not happen for a self-consistent shard).
func bucketLabel(labelByID map[string]string, id string) string {
	if label, ok := labelByID[id]; ok {
		return label
	}
	return id
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

// deriveEntityCaption summarises an "entity" shard (N entities).
// Entity shards use the Nodes field (same as simple; caption text differs).
func deriveEntityCaption(s *Shard) string {
	n := len(s.Nodes)
	switch n {
	case 0:
		return "0 entities"
	case 1:
		return "1 entity"
	default:
		return fmt.Sprintf("%d entities", n)
	}
}

// deriveSimpleCaption summarises a "simple" shard (N nodes).
// Simple shards use the Nodes field (e.g. symbol chain for code view).
func deriveSimpleCaption(s *Shard) string {
	n := len(s.Nodes)
	switch n {
	case 0:
		return "0 nodes"
	case 1:
		return "1 node"
	default:
		return fmt.Sprintf("%d nodes", n)
	}
}
