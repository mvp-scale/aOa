package kglab

import (
	"sort"

	"github.com/corey/aoa/internal/domain/arch"
)

// buildAdj builds a forward adjacency map (FromUnit -> []ToUnit) over the given
// units and deps. Only edges whose BOTH endpoints are in the unit set survive —
// this keeps a filtered subgraph internally consistent. Neighbour lists are
// deterministic (sorted, deduped).
func buildAdj(units []arch.UnitFact, deps []arch.DepFact, kind string) map[string][]string {
	inSet := make(map[string]bool, len(units))
	for _, u := range units {
		inSet[u.ID] = true
	}
	seen := make(map[string]map[string]bool) // from -> set(to)
	adj := make(map[string][]string, len(units))
	for _, d := range deps {
		if depKind(d) != kind {
			continue // only traverse edges of the requested concept
		}
		if !inSet[d.FromUnit] || !inSet[d.ToUnit] || d.FromUnit == d.ToUnit {
			continue
		}
		if seen[d.FromUnit] == nil {
			seen[d.FromUnit] = make(map[string]bool)
		}
		if seen[d.FromUnit][d.ToUnit] {
			continue
		}
		seen[d.FromUnit][d.ToUnit] = true
		adj[d.FromUnit] = append(adj[d.FromUnit], d.ToUnit)
	}
	for k := range adj {
		sort.Strings(adj[k])
	}
	return adj
}

// reverseAdj flips edge direction, so a forward BFS over it yields DEPENDENTS
// (who imports the seed) — the "blast radius" traversal.
func reverseAdj(adj map[string][]string) map[string][]string {
	rev := make(map[string][]string, len(adj))
	for from, tos := range adj {
		for _, to := range tos {
			rev[to] = append(rev[to], from)
		}
	}
	for k := range rev {
		sort.Strings(rev[k])
	}
	return rev
}

// bfsReachable is a clean-room copy of internal/app/traverse.go:11 (the lab must
// not import internal/app, which drags daemon deps). It returns the set of nodes
// reachable from seed within k hops, EXCLUDING the seed, sorted for determinism.
// k<=0 means unbounded.
func bfsReachable(adj map[string][]string, seed string, k int) []string {
	visited := map[string]bool{seed: true}
	frontier := []string{seed}
	var result []string
	for hop := 0; (k <= 0 || hop < k) && len(frontier) > 0; hop++ {
		var next []string
		for _, node := range frontier {
			neighbors := append([]string(nil), adj[node]...)
			sort.Strings(neighbors)
			for _, nb := range neighbors {
				if !visited[nb] {
					visited[nb] = true
					result = append(result, nb)
					next = append(next, nb)
				}
			}
		}
		frontier = next
	}
	sort.Strings(result)
	return result
}
