package arch

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// RenderCapability projects unit/dep facts into the "capability map" — the
// C4 Container-equivalent view that shows the capabilities themselves
// (ruling C3: "Capabilities: ADD ... the view that shows the capabilities
// themselves"). A capability is the CONTAINER other views render inside, not
// a competing view: repo -> capabilities (each app-base in a monorepo) ->
// each capability gets its own full blueprint set (N×M, future work).
//
// v1 shape: footprint.go's Footprint always carries exactly ONE anchor
// (ruling B — "v1 always has exactly ONE anchor"), so every unit in the
// current scope belongs to a single capability. Capability = the scope grain
// (owner ruling: "the CAPABILITIES nav panel exists, currently always a
// single 'local'") — this render emits exactly one Bucket per call, seeded
// from the scope name, containing every local (non-external) unit. The
// cross-capability edge aggregation below is written generically so a
// future multi-anchor unitCapability assignment (V1-D large-repo proof)
// slots in without reshaping this function.
//
// Provenance: ALWAYS "mixed" — capability boundaries are footprint-detected
// heuristics (INFERRED), never a raw derived fact (D2 honesty). Every Bucket
// is stamped Inferred=true; the owner relabels DECLARED later (field only,
// no relabel flow in v1 — D18/D24).
func RenderCapability(in RenderInput) (*Shard, error) {
	unitByID := make(map[string]UnitFact, len(in.Units))
	for _, u := range in.Units {
		unitByID[u.ID] = u
	}

	// Fan-in per unit (for member sub: "in N", matches RenderComponent).
	fanIn := make(map[string]int, len(in.Units))
	for _, d := range in.Deps {
		fanIn[d.ToUnit]++
	}

	// v1: one implicit capability spans the whole scope (ruling B — exactly
	// one footprint anchor). External ("ext:") units are not part of this
	// system's own deployable pieces (they surface in the context view).
	capID := "cap_" + scopeSlug(in.Scope)
	unitCapability := make(map[string]string, len(in.Units))
	for _, u := range in.Units {
		if isExternalUnit(u.ID, unitByID) {
			continue
		}
		unitCapability[u.ID] = capID
	}

	members := make([]Member, 0, len(unitCapability))
	for id := range unitCapability {
		u := unitByID[id]
		sub := ""
		if fi := fanIn[u.ID]; fi > 0 {
			sub = fmt.Sprintf("in %d", fi)
		}
		members = append(members, Member{
			ID:      u.ID,
			Label:   truncate(u.Label, 26), // member budget: ≤26 chars
			Sub:     sub,
			Sources: []SourceRef{{File: u.File, Line: u.Line}},
		})
	}
	sort.Slice(members, func(i, j int) bool {
		fi, fj := fanIn[members[i].ID], fanIn[members[j].ID]
		if fi != fj {
			return fi > fj
		}
		return members[i].Label < members[j].Label
	})

	buckets := []Bucket{{
		ID:       capID,
		Label:    truncate(capabilityLabel(in.Scope), 30), // node budget: ≤30 chars
		Part:     0,
		Inferred: true, // footprint-seeded (INFERRED) — owner relabels DECLARED later
		Members:  members,
	}}

	// Cross-capability edges: aggregate dep-edge counts where both endpoints
	// resolve to a (local) capability and those capabilities differ. Always
	// empty in v1 (single capability), generalizes once unitCapability
	// carries more than one distinct value.
	type edgeKey struct{ src, dst string }
	edgeCounts := make(map[edgeKey]int)
	for _, d := range in.Deps {
		srcCap, srcOK := unitCapability[d.FromUnit]
		dstCap, dstOK := unitCapability[d.ToUnit]
		if !srcOK || !dstOK || srcCap == dstCap {
			continue
		}
		edgeCounts[edgeKey{srcCap, dstCap}] += d.Count
	}
	var edges []ShardEdge
	for k, cnt := range edgeCounts {
		edges = append(edges, ShardEdge{
			ID:     fmt.Sprintf("e_%s_%s", k.src, k.dst),
			Source: k.src,
			Target: k.dst,
			Count:  cnt,
		})
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })

	prov := Prov{Kind: "mixed", Label: "MIXED · capabilities seeded from footprint app-base detection"}
	shard := &Shard{
		Kind:    "buckets",
		Title:   "Capabilities",
		Dir:     "DOWN",
		Prov:    prov,
		Buckets: buckets,
		Edges:   edges,
	}
	shard.Count, shard.FindingsClause = DeriveCaption(shard, in.Findings)
	return shard, nil
}

// scopeSlug turns a scope name into a stable bucket-ID suffix. Non
// [a-z0-9_] runs collapse to a single underscore (mirrors unit-slug style
// elsewhere in this package) so the ID stays byte-stable and URL/JSON-safe.
func scopeSlug(scope string) string {
	if scope == "" {
		return "local"
	}
	out := make([]rune, 0, len(scope))
	prevUnderscore := false
	for _, r := range scope {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
			prevUnderscore = false
		case r >= 'A' && r <= 'Z':
			out = append(out, r-'A'+'a')
			prevUnderscore = false
		default:
			if !prevUnderscore {
				out = append(out, '_')
				prevUnderscore = true
			}
		}
	}
	return strings.Trim(string(out), "_")
}

// capabilityLabel produces a display label for the single v1 capability
// bucket from the scope name (e.g. "local" -> "Local").
func capabilityLabel(scope string) string {
	if scope == "" {
		return "Local"
	}
	r := []rune(scope)
	upper := strings.ToUpper(string(r[0]))
	r[0], _ = utf8.DecodeRuneInString(upper)
	return string(r)
}
