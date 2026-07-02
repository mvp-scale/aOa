package arch

import (
	"fmt"
	"sort"
)

// RenderComponent produces a "buckets" shard from units and dep edges.
//
// Algorithm:
//  1. Group units using the GroupingResult.
//  2. Build cross-group dep edges (aggregated count per group pair).
//  3. Sort buckets by Part (ascending) then ID; members by fan-in desc then label.
//  4. Emit ShardEdges sorted by ID.
//  5. Count caption via DeriveCaption.
//
// Provenance: "mixed" when rung-2 path-prefix grouping is used (heuristic, not declared).
// Override to "derived" when a declaration-based GroupingResult is provided.
func RenderComponent(in RenderInput) (*Shard, error) {
	g := in.Grouping

	// Count fan-in for each unit (for member sub: "in N" label).
	fanIn := make(map[string]int, len(in.Units))
	for _, d := range in.Deps {
		fanIn[d.ToUnit]++
	}

	// Build members per group.
	// membersByGroup: groupID → []Member (sorted by fan-in desc, then label).
	membersByGroup := make(map[string][]Member, len(g.Groups))
	unitByID := make(map[string]UnitFact, len(in.Units))
	for _, u := range in.Units {
		unitByID[u.ID] = u
		gid := g.UnitGroup[u.ID]
		if gid == "" {
			gid = "g_other"
		}
		sub := ""
		if fi := fanIn[u.ID]; fi > 0 {
			sub = fmt.Sprintf("in %d", fi)
		}
		membersByGroup[gid] = append(membersByGroup[gid], Member{
			ID:    u.ID,
			Label: truncate(u.Label, 26), // member budget: ≤26 chars
			Sub:   sub,
			Sources: []SourceRef{
				{File: u.File, Line: u.Line},
			},
		})
	}

	// Sort members: fan-in desc, then label asc (matches build_c4_mockup.py:65).
	for gid, members := range membersByGroup {
		sort.Slice(members, func(i, j int) bool {
			fi := fanIn[members[i].ID]
			fj := fanIn[members[j].ID]
			if fi != fj {
				return fi > fj
			}
			return members[i].Label < members[j].Label
		})
		membersByGroup[gid] = members
	}

	// Build cross-group edge counts.
	type edgeKey struct{ src, dst string }
	edgeCounts := make(map[edgeKey]int)
	edgeSrc := make(map[edgeKey][]SourceRef)
	for _, d := range in.Deps {
		srcGroup := g.UnitGroup[d.FromUnit]
		dstGroup := g.UnitGroup[d.ToUnit]
		if srcGroup == "" || dstGroup == "" || srcGroup == dstGroup {
			continue // skip intra-group edges
		}
		k := edgeKey{srcGroup, dstGroup}
		edgeCounts[k] += d.Count
		edgeSrc[k] = append(edgeSrc[k], SourceRef{File: d.File, Line: d.Line})
	}

	// Assemble buckets, sorted by Part then ID.
	groups := make([]GroupMeta, len(g.Groups))
	copy(groups, g.Groups)
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Part != groups[j].Part {
			return groups[i].Part < groups[j].Part
		}
		return groups[i].ID < groups[j].ID
	})

	buckets := make([]Bucket, 0, len(groups))
	for _, gm := range groups {
		members := membersByGroup[gm.ID]
		if members == nil {
			members = []Member{}
		}
		buckets = append(buckets, Bucket{
			ID:      gm.ID,
			Label:   truncate(gm.Label, 30), // node budget: ≤30 chars
			Part:    gm.Part,
			Members: members,
		})
	}

	// Assemble shard edges, sorted by ID for determinism.
	var shardEdges []ShardEdge
	for k, cnt := range edgeCounts {
		eid := fmt.Sprintf("e_%s_%s", k.src, k.dst)
		shardEdges = append(shardEdges, ShardEdge{
			ID:     eid,
			Source: k.src,
			Target: k.dst,
			Count:  cnt,
		})
	}
	sort.Slice(shardEdges, func(i, j int) bool {
		return shardEdges[i].ID < shardEdges[j].ID
	})

	shard := &Shard{
		Kind:    "buckets",
		Title:   "Component diagram",
		Dir:     "DOWN",
		Prov:    Prov{Kind: "mixed", Label: "MIXED · imports real · grouping inferred"},
		Buckets: buckets,
		Edges:   shardEdges,
	}
	shard.Count = DeriveCaption(shard, in.Findings)
	return shard, nil
}

// truncate truncates s to at most n runes, appending "…" if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n > 1 {
		return string(runes[:n-1]) + "…"
	}
	return string(runes[:n])
}
