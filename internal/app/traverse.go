package app

import "sort"

// bfsReachable returns every node reachable from seed over adj within k hops.
// It is the shared traversal primitive behind `arch blast` (reverse edges =
// dependents) and the forthcoming `aoa map` verbs. Pure and deterministic:
// neighbors are visited in sorted order and the result is sorted; seed is
// excluded. k <= 0 means unbounded (bounded only by the graph).
//
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
