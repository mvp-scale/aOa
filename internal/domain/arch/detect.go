package arch

import (
	"crypto/sha256"
	"fmt"
	"sort"
)

// TarjanSCC computes all strongly connected components of a directed graph
// using Tarjan's algorithm. adj maps node ID → slice of neighbor IDs.
// Returns a list of SCCs; each component with len > 1 is a dependency cycle.
// Single-node SCCs (no self-loop) are omitted.
// Output is deterministic: nodes within each SCC are sorted; SCCs are sorted
// by their first element.
func TarjanSCC(adj map[string][]string) [][]string {
	index := 0
	stack := []string{}
	onStack := map[string]bool{}
	indices := map[string]int{}
	lowlink := map[string]int{}
	sccs := [][]string{}

	// Sort node list for deterministic traversal order.
	nodes := make([]string, 0, len(adj))
	for n := range adj {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)

	var strongconnect func(v string)
	strongconnect = func(v string) {
		indices[v] = index
		lowlink[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		// Sort neighbors for determinism.
		neighbors := make([]string, len(adj[v]))
		copy(neighbors, adj[v])
		sort.Strings(neighbors)

		for _, w := range neighbors {
			if _, seen := indices[w]; !seen {
				strongconnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] {
				if indices[w] < lowlink[v] {
					lowlink[v] = indices[w]
				}
			}
		}

		if lowlink[v] == indices[v] {
			var scc []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			sort.Strings(scc)
			if len(scc) > 1 {
				sccs = append(sccs, scc)
			}
		}
	}

	for _, n := range nodes {
		if _, seen := indices[n]; !seen {
			strongconnect(n)
		}
	}

	// Sort SCCs by first member for determinism.
	sort.Slice(sccs, func(i, j int) bool {
		return sccs[i][0] < sccs[j][0]
	})
	return sccs
}

// buildAdj constructs an adjacency list from DepFacts for Tarjan's algorithm.
// Only units present in unitSet are included as nodes (avoids phantom nodes).
func buildAdj(units []UnitFact, deps []DepFact) map[string][]string {
	unitSet := make(map[string]struct{}, len(units))
	for _, u := range units {
		unitSet[u.ID] = struct{}{}
	}
	adj := make(map[string][]string, len(units))
	for _, u := range units {
		adj[u.ID] = nil // ensure every unit appears even with no outbound edges
	}
	for _, d := range deps {
		if _, ok := unitSet[d.FromUnit]; !ok {
			continue
		}
		if _, ok := unitSet[d.ToUnit]; !ok {
			continue
		}
		adj[d.FromUnit] = append(adj[d.FromUnit], d.ToUnit)
	}
	return adj
}

// findingID computes a stable fingerprint for a finding:
// sha256(rule + scope + sorted(subjects))[:16].
// Stable across re-renders and line drift — no line numbers included.
func findingID(rule, scope string, subjects []string) string {
	sorted := make([]string, len(subjects))
	copy(sorted, subjects)
	sort.Strings(sorted)
	h := sha256.New()
	h.Write([]byte(rule))
	h.Write([]byte("|"))
	h.Write([]byte(scope))
	h.Write([]byte("|"))
	for _, s := range sorted {
		h.Write([]byte(s))
		h.Write([]byte(","))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// unitSourceRef returns the SourceRef for a unit by ID, or a zero ref if not found.
func unitSourceRef(units []UnitFact, id string) SourceRef {
	for _, u := range units {
		if u.ID == id {
			return SourceRef{File: u.File, Line: u.Line}
		}
	}
	return SourceRef{}
}

// DetectCycles finds dependency cycles using Tarjan SCC over the unit graph.
// Each SCC >1 → one Finding with rule "cycle" and the min-count edge noted.
// Returns the SCCs alongside the findings (shared with cycles renderer).
func DetectCycles(scope string, units []UnitFact, deps []DepFact) ([]Finding, [][]string) {
	adj := buildAdj(units, deps)
	sccs := TarjanSCC(adj)

	// Build dep count lookup for min-edge computation.
	edgeCount := make(map[string]map[string]int)
	edgeSource := make(map[string]map[string]SourceRef)
	for _, d := range deps {
		if edgeCount[d.FromUnit] == nil {
			edgeCount[d.FromUnit] = make(map[string]int)
			edgeSource[d.FromUnit] = make(map[string]SourceRef)
		}
		edgeCount[d.FromUnit][d.ToUnit] += d.Count
		edgeSource[d.FromUnit][d.ToUnit] = SourceRef{File: d.File, Line: d.Line}
	}

	var findings []Finding
	for _, scc := range sccs {
		// Build a readable cycle string from sorted SCC members.
		msg := "dependency cycle: " + scc[0]
		for _, m := range scc[1:] {
			msg += " → " + m
		}
		msg += " → " + scc[0]

		// Find min-count internal edge (cheapest to cut).
		minCount := -1
		var minSrc, minDst string
		var sources []SourceRef
		for _, from := range scc {
			for _, to := range scc {
				if from == to {
					continue
				}
				if c, ok := edgeCount[from][to]; ok {
					if minCount < 0 || c < minCount {
						minCount = c
						minSrc = from
						minDst = to
					}
					if sr, ok2 := edgeSource[from][to]; ok2 {
						sources = append(sources, sr)
					}
				}
			}
		}
		if len(sources) == 0 {
			// fallback: collect unit sources
			for _, m := range scc {
				sources = append(sources, unitSourceRef(units, m))
			}
		}
		sort.Slice(sources, func(i, j int) bool {
			if sources[i].File != sources[j].File {
				return sources[i].File < sources[j].File
			}
			return sources[i].Line < sources[j].Line
		})
		f := Finding{
			Rule:     "cycle",
			Severity: "error",
			Scope:    scope,
			Message:  msg,
			Subjects: append([]string(nil), scc...),
			Sources:  sources,
		}
		f.ID = findingID(f.Rule, f.Scope, f.Subjects)
		if minCount >= 0 {
			f.CheapestCut = fmt.Sprintf("%s → %s (×%d)", minSrc, minDst, minCount)
		}
		findings = append(findings, f)
	}
	return findings, sccs
}

// DetectGods finds god components: units with fan-in ≥ godIn AND fan-out ≥ godOut.
// Message format: "god component: X (in N · out M)".
func DetectGods(scope string, units []UnitFact, deps []DepFact, opts ThresholdOpts) []Finding {
	fanIn := make(map[string]int, len(units))
	fanOut := make(map[string]int, len(units))
	// Track representative source refs.
	inSrc := make(map[string][]SourceRef)
	outSrc := make(map[string][]SourceRef)

	for _, d := range deps {
		fanOut[d.FromUnit]++
		fanIn[d.ToUnit]++
		outSrc[d.FromUnit] = append(outSrc[d.FromUnit], SourceRef{File: d.File, Line: d.Line})
		inSrc[d.ToUnit] = append(inSrc[d.ToUnit], SourceRef{File: d.File, Line: d.Line})
	}

	// Sort units for deterministic output order.
	sorted := make([]UnitFact, len(units))
	copy(sorted, units)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	var findings []Finding
	for _, u := range sorted {
		in := fanIn[u.ID]
		out := fanOut[u.ID]
		if in >= opts.GodIn && out >= opts.GodOut {
			msg := fmt.Sprintf("god component: %s (in %d · out %d)", u.Label, in, out)
			sources := append(inSrc[u.ID], outSrc[u.ID]...)
			sort.Slice(sources, func(i, j int) bool {
				if sources[i].File != sources[j].File {
					return sources[i].File < sources[j].File
				}
				return sources[i].Line < sources[j].Line
			})
			// Deduplicate sources.
			sources = dedupSources(sources)

			f := Finding{
				Rule:     "god",
				Severity: "warn",
				Scope:    scope,
				Message:  msg,
				Subjects: []string{u.ID},
				Sources:  sources,
			}
			f.ID = findingID(f.Rule, f.Scope, f.Subjects)
			findings = append(findings, f)
		}
	}
	return findings
}

// DetectOrphans finds units with zero inbound AND zero outbound dependencies.
// Message format: "orphan: X — no connections".
func DetectOrphans(scope string, units []UnitFact, deps []DepFact) []Finding {
	connected := make(map[string]bool, len(units))
	for _, d := range deps {
		connected[d.FromUnit] = true
		connected[d.ToUnit] = true
	}

	// Sort units for deterministic output order.
	sorted := make([]UnitFact, len(units))
	copy(sorted, units)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	var findings []Finding
	for _, u := range sorted {
		if !connected[u.ID] {
			msg := fmt.Sprintf("orphan: %s — no connections", u.Label)
			f := Finding{
				Rule:     "orphan",
				Severity: "info",
				Scope:    scope,
				Message:  msg,
				Subjects: []string{u.ID},
				Sources:  []SourceRef{{File: u.File, Line: u.Line}},
			}
			f.ID = findingID(f.Rule, f.Scope, f.Subjects)
			findings = append(findings, f)
		}
	}
	return findings
}

// Detect runs all detectors and returns combined findings + SCCs (shared with renderer).
func Detect(scope string, units []UnitFact, deps []DepFact, opts ThresholdOpts) ([]Finding, [][]string) {
	cycleFx, sccs := DetectCycles(scope, units, deps)
	godFx := DetectGods(scope, units, deps, opts)
	orphanFx := DetectOrphans(scope, units, deps)

	all := make([]Finding, 0, len(cycleFx)+len(godFx)+len(orphanFx))
	all = append(all, cycleFx...)
	all = append(all, godFx...)
	all = append(all, orphanFx...)
	return all, sccs
}

// dedupSources removes duplicate SourceRefs (same file+line).
func dedupSources(srcs []SourceRef) []SourceRef {
	seen := make(map[SourceRef]bool, len(srcs))
	out := srcs[:0:0]
	for _, s := range srcs {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
