// Package arch — pure graph/string utilities (no I/O, stdlib-only).
//
// This file holds canonical implementations that were previously duplicated
// between internal/app/arch.go and cmd/aoa/cmd/arch.go (PC8 Finding 14).
// Both call sites now import these; copy discipline is eliminated.
package arch

import "strings"

// UnitSlug converts a path (directory or import path) to a stable unit ID.
// Deterministic: same path → same slug on every machine and run.
// Format: "u_" + lowercase-alphanum-with-underscores.
//
// Examples:
//
//	"internal/app"         → "u_internal_app"
//	"ext:std/fmt"          → "u_ext_std_fmt"
//	"ext:go.etcd.io/bbolt" → "u_ext_go_etcd_io_bbolt"
//	"" or "."              → "u_root"
func UnitSlug(path string) string {
	if path == "" || path == "." {
		return "u_root"
	}
	path = strings.ToLower(path)
	var b strings.Builder
	prevUnderscore := false
	for _, r := range path {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			prevUnderscore = false
		} else {
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
			}
			prevUnderscore = true
		}
	}
	s := strings.TrimRight(b.String(), "_")
	if s == "" {
		return "u_root"
	}
	return "u_" + s
}

// BFSShortestPath returns the shortest unit-ID path from→to in adj,
// limited to k hops. Returns nil when no path exists within the budget.
// adj maps each unit ID to its de-duplicated outbound neighbours.
//
// The from==to case is handled: returns []string{from} immediately.
func BFSShortestPath(adj map[string][]string, from, to string, k int) []string {
	if from == to {
		return []string{from}
	}
	type bfsState struct {
		id   string
		path []string
	}
	visited := map[string]bool{from: true}
	queue := []bfsState{{id: from, path: []string{from}}}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if len(curr.path) > k {
			break // exceeded hop budget
		}
		for _, next := range adj[curr.id] {
			newPath := make([]string, len(curr.path)+1)
			copy(newPath, curr.path)
			newPath[len(curr.path)] = next
			if next == to {
				return newPath // found shortest path
			}
			if !visited[next] && len(newPath) <= k {
				visited[next] = true
				queue = append(queue, bfsState{id: next, path: newPath})
			}
		}
	}
	return nil // no path within k hops
}
