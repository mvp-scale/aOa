package arch

import (
	"sort"
)

// RenderDSM produces a "matrix" shard (Dependency Structure Matrix) from units
// and dep edges, using the same grouping as the component renderer.
//
// The rows/columns are groups ordered by Part (ascending) then fan-in (desc).
// matrix[i][j] = number of dependencies from group i to group j.
// null (nil) when the count is 0 — not emitted as the integer 0.
//
// Determinism guarantee (T4): groups are sorted by Part then ID; intra-group
// edges are excluded; nil cells are always nil (never 0 as int).
func RenderDSM(in RenderInput) (*Shard, error) {
	g := in.Grouping

	// Compute group-level fan-in (number of inbound cross-group edges).
	groupFanIn := make(map[string]int, len(g.Groups))
	for _, d := range in.Deps {
		src := g.UnitGroup[d.FromUnit]
		dst := g.UnitGroup[d.ToUnit]
		if src == "" || dst == "" || src == dst {
			continue
		}
		groupFanIn[dst]++
	}

	// Order groups: by Part (asc) then fan-in (desc) then ID (asc).
	groups := make([]GroupMeta, len(g.Groups))
	copy(groups, g.Groups)
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Part != groups[j].Part {
			return groups[i].Part < groups[j].Part
		}
		fi := groupFanIn[groups[i].ID]
		fj := groupFanIn[groups[j].ID]
		if fi != fj {
			return fi > fj
		}
		return groups[i].ID < groups[j].ID
	})

	// Build group index for matrix lookup.
	groupIdx := make(map[string]int, len(groups))
	items := make([]string, len(groups))
	for i, gm := range groups {
		groupIdx[gm.ID] = i
		items[i] = gm.Label
	}

	// Aggregate dep counts at group grain.
	n := len(groups)
	// Use int matrix internally; nil = no edge.
	type cell = *int
	raw := make([][]cell, n)
	for i := range raw {
		raw[i] = make([]cell, n)
	}

	for _, d := range in.Deps {
		si := groupIdx[g.UnitGroup[d.FromUnit]]
		di := groupIdx[g.UnitGroup[d.ToUnit]]
		if g.UnitGroup[d.FromUnit] == "" || g.UnitGroup[d.ToUnit] == "" {
			continue
		}
		if g.UnitGroup[d.FromUnit] == g.UnitGroup[d.ToUnit] {
			continue // intra-group: skip
		}
		if raw[si][di] == nil {
			v := 0
			raw[si][di] = &v
		}
		*raw[si][di] += d.Count
	}

	// Convert to [][]interface{} (nil → JSON null; *int → JSON number).
	matrix := make([][]interface{}, n)
	for i := range matrix {
		row := make([]interface{}, n)
		for j := range row {
			if raw[i][j] != nil {
				row[j] = *raw[i][j]
			}
			// else: row[j] remains nil → JSON null
		}
		matrix[i] = row
	}

	prov := provFromKind(in.GroupProv)
	shard := &Shard{
		Kind:   "matrix",
		Title:  "Dependency Structure Matrix",
		Dir:    "DOWN",
		Prov:   prov,
		Items:  items,
		Matrix: matrix,
	}
	shard.Count, shard.FindingsClause = DeriveCaption(shard, in.Findings)
	return shard, nil
}
