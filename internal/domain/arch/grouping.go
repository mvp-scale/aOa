package arch

import (
	"encoding/json"
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
// Prov for path-prefix grouping is "derived" (REAL): G7 permits deterministic
// name/group/annotate of extracted facts; "mixed" is reserved for applied overlays
// and ②b subset choices (kickoff-F2 §7 ruling D1, 2026-07-03).
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

// GroupWithOptions runs the full three-rung grouping cascade with optional overlay.
//
// Rung priority (highest wins):
//  1. Overlay (opts.Overlays) → prov "mixed" if any applied
//  2. Rung-1 (opts.Declarations) → explicit arch.yaml assignments → prov "derived"
//  3. Rung-3 (unit.Domain != "") → atlas domain label → prov "derived"
//  4. Rung-2 (pathPrefixGroup) → path-prefix heuristic → prov "derived"
//
// All three non-overlay rungs stamp prov "derived" (REAL) per kickoff-F2 §7 D1.
// Overlays (or OverlayHadInvalidIDs) stamp prov "mixed".
//
// Returns: result, provKind ("derived"|"mixed"), warning findings (overlay-leash).
func GroupWithOptions(units []UnitFact, opts *GroupOptions) (GroupingResult, string, []Finding) {
	// Start with rung-2 (path-prefix) for all units.
	labelFor := make(map[string]string, len(units))
	for _, u := range units {
		labelFor[u.ID] = pathPrefixGroup(u.Path)
	}

	// Rung-3: atlas domain — overrides rung-2 where Domain is set.
	for _, u := range units {
		if u.Domain != "" {
			labelFor[u.ID] = u.Domain
		}
	}

	// Rung-1: explicit declarations — override rung-3/rung-2.
	if opts != nil {
		for uid, label := range opts.Declarations {
			if _, exists := labelFor[uid]; exists {
				labelFor[uid] = label
			}
		}
	}

	// Overlay: override everything. Prov → mixed.
	overlayApplied := false
	var warnings []Finding
	if opts != nil {
		if len(opts.Overlays) > 0 {
			for uid, label := range opts.Overlays {
				labelFor[uid] = label
				overlayApplied = true
			}
		}
		if opts.OverlayHadInvalidIDs {
			overlayApplied = true
			// Generate a warning finding for the leash violation.
			f := Finding{
				Rule:     "overlay-leash",
				Severity: "warn",
				Message:  "overlay references unit IDs absent from facts — invented IDs ignored",
				Subjects: nil,
				Sources:  nil,
			}
			f.ID = findingID(f.Rule, "overlay", []string{"leash"})
			warnings = append(warnings, f)
		}
	}

	provKind := "derived"
	if overlayApplied {
		provKind = "mixed"
	}

	result := buildGroupingFromLabels(units, labelFor)
	return result, provKind, warnings
}

// buildGroupingFromLabels constructs a GroupingResult from a unitID → label map.
func buildGroupingFromLabels(units []UnitFact, labelFor map[string]string) GroupingResult {
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
	groupIdx := make(map[string]int, len(labels))
	for i, lbl := range labels {
		id := fmt.Sprintf("g_%s", slugify(lbl))
		groups[i] = GroupMeta{
			ID:    id,
			Label: lbl,
			Part:  i,
		}
		groupIdx[lbl] = i
	}

	unitGroup := make(map[string]string, len(units))
	for _, u := range units {
		lbl := labelFor[u.ID]
		if idx, ok := groupIdx[lbl]; ok {
			unitGroup[u.ID] = groups[idx].ID
		}
	}
	return GroupingResult{UnitGroup: unitGroup, Groups: groups}
}

// ParseOverlay decodes an overlay spec from JSON.
// Returns an error if the $schema field is not "aoa.arch-overlay/v1".
func ParseOverlay(data []byte) (*OverlaySpec, error) {
	var spec OverlaySpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("overlay: parse JSON: %w", err)
	}
	if spec.Schema != "aoa.arch-overlay/v1" {
		return nil, fmt.Errorf("overlay: unsupported schema %q — expected aoa.arch-overlay/v1", spec.Schema)
	}
	return &spec, nil
}

// ApplyOverlay validates an overlay spec against the unit fact set (leash law)
// and returns:
//   - approved: unitID → group label (only valid IDs; excludes invented IDs)
//   - invalidIDs: unit IDs from the overlay absent from the fact set (leash violations)
//
// Invalid IDs are rejected silently from approved; they are reported so the
// caller can emit warning findings and stamp provenance as "mixed".
func ApplyOverlay(spec *OverlaySpec, units []UnitFact) (approved map[string]string, invalidIDs []string) {
	unitSet := make(map[string]bool, len(units))
	for _, u := range units {
		unitSet[u.ID] = true
	}

	approved = make(map[string]string)
	seen := make(map[string]bool)
	for _, g := range spec.Groups {
		for _, uid := range g.UnitIDs {
			if seen[uid] {
				continue // dedup
			}
			seen[uid] = true
			if unitSet[uid] {
				approved[uid] = g.ID
			} else {
				invalidIDs = append(invalidIDs, uid)
			}
		}
	}
	sort.Strings(invalidIDs) // deterministic order
	return approved, invalidIDs
}
