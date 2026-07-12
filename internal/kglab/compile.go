package kglab

import (
	"fmt"
	"strings"

	"github.com/corey/aoa/internal/domain/arch"
)

// Compile turns a ViewQuery into a rendered arch.Shard by walking the real
// pipeline: honesty gate -> select -> traverse -> budget -> group -> detect ->
// render. Every step calls a real arch function; nothing is fabricated. On any
// honesty violation it returns (nil, error) rather than a partial/fake shard.
func Compile(q ViewQuery, units []arch.UnitFact, deps []arch.DepFact) (*arch.Shard, error) {
	// 1. Honesty gate (ontology-driven): resolve the requested edge concept and
	//    refuse unless it is a WIRED edge in the ontology. Only :import is wired
	//    today; call is recon-only; node/property concepts are never edges. This
	//    replaces the old hardcoded "imports" string with a registry lookup.
	edgeKind := "imports"
	if q.Traverse != nil {
		if q.Traverse.EdgeKind != "" {
			edgeKind = q.Traverse.EdgeKind
		}
		if !EdgeKindHasSubstrate(edgeKind) {
			return nil, fmt.Errorf("kglab: edge kind %q has no substrate — %s", q.Traverse.EdgeKind, edgeReason(edgeKind))
		}
	}

	// 2. Select: filter the unit set (OR semantics), then keep only edges whose
	//    endpoints both survive.
	if q.Select != nil {
		units, deps = applySelect(*q.Select, units, deps)
	}

	// 3. Traverse: BFS from a seed over the (optionally reversed) adjacency.
	if q.Traverse != nil {
		if !containsUnit(units, q.Traverse.Seed) {
			return nil, fmt.Errorf("kglab: traversal seed %q not found in unit set (unresolved node)", q.Traverse.Seed)
		}
		adj := buildAdj(units, deps, edgeKind)
		if q.Traverse.Dir == "reverse" {
			adj = reverseAdj(adj)
		}
		reach := bfsReachable(adj, q.Traverse.Seed, q.Traverse.Hops)
		keep := map[string]bool{q.Traverse.Seed: true} // re-add seed: it is part of its own subgraph
		for _, id := range reach {
			keep[id] = true
		}
		units, deps = filterToSet(keep, units, deps)
	}

	// 4. Budget: refuse rather than silently truncate.
	if q.Budget.MaxNodes > 0 && len(units) > q.Budget.MaxNodes {
		return nil, fmt.Errorf("kglab: %d nodes exceeds budget of %d (refusing to truncate)", len(units), q.Budget.MaxNodes)
	}

	// 5. Group: default rung-2 path-prefix, or caller-supplied options.
	var grouping arch.GroupingResult
	groupProv := "derived"
	if q.Group != nil {
		grouping, groupProv, _ = arch.GroupWithOptions(units, q.Group)
	} else {
		grouping = arch.Group(units)
	}

	// 6. Detect once: yields findings + SCCs (Detect calls DetectCycles internally,
	//    so we never double-count). Both feed the RenderInput.
	findings, sccs := arch.Detect(q.Scope, units, deps, arch.DefaultThresholds(), grouping, nil)

	in := arch.RenderInput{
		Scope:     q.Scope,
		Units:     units,
		Deps:      deps,
		Grouping:  grouping,
		GroupProv: groupProv,
		SCCs:      sccs,
		Findings:  findings,
	}

	// 7. Render: dispatch to one of the 4 real renderers.
	switch q.Render.Kind {
	case "component":
		return arch.RenderComponent(in)
	case "cycles":
		return arch.RenderCycles(in)
	case "dsm":
		return arch.RenderDSM(in)
	case "code":
		return nil, fmt.Errorf("kglab: render kind \"code\" requires CodeSymbolIndex which is out of scope for the lab")
	default:
		return nil, fmt.Errorf("kglab: unknown render kind %q (want component|cycles|dsm)", q.Render.Kind)
	}
}

// applySelect keeps units matching PathPrefix OR ID list, then prunes dangling edges.
func applySelect(s SelectSpec, units []arch.UnitFact, deps []arch.DepFact) ([]arch.UnitFact, []arch.DepFact) {
	idSet := make(map[string]bool, len(s.IDs))
	for _, id := range s.IDs {
		idSet[id] = true
	}
	keep := make(map[string]bool, len(units))
	kept := make([]arch.UnitFact, 0, len(units))
	for _, u := range units {
		match := false
		if s.PathPrefix != "" && strings.HasPrefix(u.Path, s.PathPrefix) {
			match = true
		}
		if idSet[u.ID] {
			match = true
		}
		if match {
			keep[u.ID] = true
			kept = append(kept, u)
		}
	}
	return kept, pruneDeps(keep, deps)
}

// filterToSet keeps only units in the set and edges with both endpoints in it.
func filterToSet(keep map[string]bool, units []arch.UnitFact, deps []arch.DepFact) ([]arch.UnitFact, []arch.DepFact) {
	kept := make([]arch.UnitFact, 0, len(keep))
	for _, u := range units {
		if keep[u.ID] {
			kept = append(kept, u)
		}
	}
	return kept, pruneDeps(keep, deps)
}

func pruneDeps(keep map[string]bool, deps []arch.DepFact) []arch.DepFact {
	kept := make([]arch.DepFact, 0, len(deps))
	for _, d := range deps {
		if keep[d.FromUnit] && keep[d.ToUnit] {
			kept = append(kept, d)
		}
	}
	return kept
}

func containsUnit(units []arch.UnitFact, id string) bool {
	for _, u := range units {
		if u.ID == id {
			return true
		}
	}
	return false
}
