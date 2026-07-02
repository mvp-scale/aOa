package arch

import (
	"fmt"
	"sort"
	"strings"
)

// languageRoots are path segments to skip when computing the path-prefix group
// (rung-2 heuristic). These are common source-tree roots that don't carry
// semantic meaning at the group level.
var languageRoots = map[string]bool{
	"src": true,
	"pkg": true,
	"lib": true,
}

// pathPrefixGroup extracts a group label from a repo-relative path using
// the rung-2 heuristic (§2.3): first two meaningful segments below the
// module root, skipping one outermost language-aware root (src/, pkg/, lib/).
//
// Only the FIRST segment is eligible for language-root skipping — this avoids
// misidentifying real directory names (e.g. "lib" after "src" is skipped once).
//
// Examples:
//   "internal/domain/arch/model.go"  → "domain"
//   "internal/adapters/bbolt/store.go" → "adapters"
//   "cmd/aoa/main.go"               → "cmd"
//   "ports/storage.go"              → "ports"
//   "src/lib/util.go"               → "lib"   (src skipped; lib is the real group)
//   "pkg/controller/ctrl.go"        → "controller" (pkg skipped)
func pathPrefixGroup(path string) string {
	parts := strings.Split(path, "/")

	// Collect non-empty, non-dot, non-filename segments.
	var segs []string
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if strings.Contains(p, ".") {
			break // filename reached
		}
		segs = append(segs, p)
	}

	// Strip at most one leading language root.
	if len(segs) > 0 && languageRoots[segs[0]] {
		segs = segs[1:]
	}

	if len(segs) == 0 {
		return "other"
	}

	// If first remaining segment is "internal", descend one more level.
	if segs[0] == "internal" && len(segs) > 1 {
		return segs[1]
	}
	return segs[0]
}

// slugify converts a label to a stable lowercase slug for use as a group ID.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// Group assigns each unit to a group using the rung-2 path-prefix heuristic.
// The returned GroupingResult is deterministic: groups are ordered by Part (ascending)
// then by ID (alphabetical within same part).
//
// Part assignment: sorted alphabetically by group label, then assigned 0,1,2,...
// so that the rendering order is stable without an arch.yaml declaration.
//
// Prov for path-prefix grouping is always "mixed" (rung-2 is heuristic, not declared).
func Group(units []UnitFact) GroupingResult {
	// Assign each unit to a group label.
	labelFor := make(map[string]string, len(units)) // unitID → groupLabel
	for _, u := range units {
		labelFor[u.ID] = pathPrefixGroup(u.Path)
	}

	// Collect unique group labels and assign stable Part values.
	labelSet := make(map[string]struct{})
	for _, lbl := range labelFor {
		labelSet[lbl] = struct{}{}
	}
	labels := make([]string, 0, len(labelSet))
	for lbl := range labelSet {
		labels = append(labels, lbl)
	}
	sort.Strings(labels)

	groups := make([]GroupMeta, len(labels))
	groupIdx := make(map[string]int, len(labels)) // label → index in groups slice
	for i, lbl := range labels {
		id := fmt.Sprintf("g_%s", slugify(lbl))
		groups[i] = GroupMeta{
			ID:    id,
			Label: lbl,
			Part:  i,
		}
		groupIdx[lbl] = i
	}

	// Build unitGroup map.
	unitGroup := make(map[string]string, len(units))
	for _, u := range units {
		lbl := labelFor[u.ID]
		unitGroup[u.ID] = groups[groupIdx[lbl]].ID
	}

	return GroupingResult{
		UnitGroup: unitGroup,
		Groups:    groups,
	}
}
