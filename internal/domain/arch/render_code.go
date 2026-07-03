package arch

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// RenderCode produces a "simple" shard from the symbol index, walking the dep
// graph outward from a deterministically-selected entrypoint.
//
// # Entrypoint heuristic (DOCUMENTED — contributes to MIXED provenance)
//
// Among all units, score each one:
//   - +100 if the unit path starts with "cmd/" or contains "/cmd/" (command packages)
//   - +50  if the path/label contains "main" (top-level entry)
//   - −10 × fan-in (higher fan-in → less likely to be an entry point)
//
// Ties broken alphabetically by unit ID (stable, deterministic).
// Fallback when no units exist: empty shard.
//
// # Walk
//
// BFS from the entrypoint through outbound dep edges, limited to depth k=3
// (default). Neighbors at each level are visited in alphabetical order by
// unit ID for determinism. Results are capped at simple_view_nodes_max (30).
//
// # Symbol selection per unit (DOCUMENTED — contributes to MIXED provenance)
//
// For each visited unit, `pickSymbol` looks up the unit's defining file in
// CodeSymbols.ByFile, then falls back to directory-prefix matching. Within
// the matching symbols, preference order is:
//  1. Exported functions (name[0] uppercase, kind contains "func")
//  2. Exported types (name[0] uppercase, kind contains "type"/"struct"/"interface")
//  3. First symbol by StartLine (ascending)
//
// # Provenance
//
// - Node Sources (file:line) are REAL: taken from SymbolMeta / UnitFact (actual code).
// - Subset choice is MIXED: which units are "critical" and which symbol represents
//   each unit are heuristic determinations, not derivable from import facts alone.
// - Shard stamps the split: kind="mixed", label="MIXED · symbols real · subset selection inferred".
func RenderCode(in RenderInput) (*Shard, error) {
	if in.CodeSymbols == nil {
		// Defensive: conditional registration in service.go prevents this.
		return nil, fmt.Errorf("arch: RenderCode called with nil CodeSymbols")
	}

	// Build dep adjacency: unit ID → sorted outbound unit IDs.
	adj := buildCodeAdj(in.Deps)

	// Select deterministic entrypoint.
	ep := selectCodeEntrypoint(in.Units, in.Deps)

	// BFS walk from entrypoint to depth k=3.
	const maxDepth = 3
	visited := codeWalkBFS(ep, adj, maxDepth)

	// Cap at simple_view_nodes_max (view-standards.json global.budgets).
	const maxNodes = 30
	if len(visited) > maxNodes {
		visited = visited[:maxNodes]
	}

	// Build unit lookup.
	unitByID := make(map[string]UnitFact, len(in.Units))
	for _, u := range in.Units {
		unitByID[u.ID] = u
	}

	// Build a node per visited unit. Symbol file:line is REAL (SymbolMeta);
	// falling back to UnitFact.File/Line (also a real code location).
	nodes := make([]Node, 0, len(visited))
	for _, vu := range visited {
		u, ok := unitByID[vu.id]
		if !ok {
			continue
		}

		sym := pickSymbol(in.CodeSymbols, u)

		var sources []SourceRef
		sub := ""
		if sym != nil {
			sources = []SourceRef{{File: sym.File, Line: uint32(sym.StartLine)}}
			if sym.Kind != "" {
				sub = sym.Kind
			}
			if sym.Signature != "" && sub == "" {
				sub = sym.Signature
			}
		} else if u.File != "" {
			// Unit has a known defining file even without indexed symbol: still REAL.
			sources = []SourceRef{{File: u.File, Line: u.Line}}
		}

		nodes = append(nodes, Node{
			ID:      u.ID,
			Label:   truncate(u.Label, 30),
			Sub:     sub,
			Real:    true,
			Sources: sources,
		})
	}

	// Build linear chain edges connecting nodes in walk order.
	edges := make([]ShardEdge, 0, max0(len(nodes)-1, 0))
	for i := 1; i < len(nodes); i++ {
		edges = append(edges, ShardEdge{
			ID:     fmt.Sprintf("e%d", i),
			Source: nodes[i-1].ID,
			Target: nodes[i].ID,
		})
	}

	// Provenance: MIXED because subset choice is heuristic.
	// Nodes carry REAL file:line (stamped via sources field above).
	prov := Prov{
		Kind:  "mixed",
		Label: "MIXED · symbols real · subset selection inferred",
	}

	// Caption: "N symbols along critical path — entrypoint: X" (WP step 2).
	epLabel := ""
	if len(visited) > 0 {
		if u, ok := unitByID[visited[0].id]; ok {
			epLabel = u.Label
		}
	}
	n := len(nodes)
	count := fmt.Sprintf("%d symbols along critical path — entrypoint: %s", n, epLabel)

	shard := &Shard{
		Kind:  "simple",
		Title: "Critical path symbols",
		Count: count,
		Prov:  prov,
		Nodes: nodes,
		Edges: edges,
	}
	return shard, nil
}

// max0 returns the larger of a and 0. Avoids negative capacity in make().
func max0(a, _ int) int {
	if a < 0 {
		return 0
	}
	return a
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// visitedUnit carries a unit ID and its BFS discovery depth.
type visitedUnit struct {
	id    string
	depth int
}

// buildCodeAdj builds a unit → sorted outbound-neighbor-IDs adjacency map.
// Sorted for determinism (same input → same BFS order, T4 requirement).
//
// External package units (unit IDs starting with "u_ext_") are excluded from the
// adjacency: the code view surfaces intra-project symbols, not stdlib/vendor chains.
func buildCodeAdj(deps []DepFact) map[string][]string {
	raw := make(map[string]map[string]bool)
	for _, d := range deps {
		// Skip edges to external packages — their unit IDs start with "u_ext_".
		if strings.HasPrefix(d.ToUnit, "u_ext_") {
			continue
		}
		if raw[d.FromUnit] == nil {
			raw[d.FromUnit] = make(map[string]bool)
		}
		raw[d.FromUnit][d.ToUnit] = true
	}
	adj := make(map[string][]string, len(raw))
	for from, toSet := range raw {
		toList := make([]string, 0, len(toSet))
		for to := range toSet {
			toList = append(toList, to)
		}
		sort.Strings(toList)
		adj[from] = toList
	}
	return adj
}

// selectCodeEntrypoint picks the "command entrypoint" unit from units.
// Scoring heuristic (documented in RenderCode's block comment):
//   +100  path starts with "cmd/" or contains "/cmd/"
//   +50   path or label contains "main"
//   −10×  fan-in count (fewer importers → more likely entry)
//
// Ties broken alphabetically by unit ID.
// Returns "" when units is empty.
func selectCodeEntrypoint(units []UnitFact, deps []DepFact) string {
	if len(units) == 0 {
		return ""
	}

	fanIn := make(map[string]int, len(units))
	for _, d := range deps {
		fanIn[d.ToUnit]++
	}

	// Sort units by ID so tie-breaking is alphabetical and deterministic.
	sorted := make([]UnitFact, len(units))
	copy(sorted, units)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	best := ""
	bestScore := -1 << 30 // very negative sentinel

	for _, u := range sorted {
		// Skip external package units — they have no local file:line.
		if strings.HasPrefix(u.ID, "u_ext_") {
			continue
		}

		score := 0

		p := strings.ToLower(u.Path)
		lbl := strings.ToLower(u.Label)

		// Prefer command packages.
		if strings.HasPrefix(p, "cmd/") || strings.Contains(p, "/cmd/") || p == "cmd" {
			score += 100
		}
		// Prefer "main" in path/label (CLI top-level).
		if strings.Contains(p, "main") || strings.Contains(lbl, "main") {
			score += 50
		}
		// Lower fan-in is better (entry point typically has zero or few importers).
		score -= fanIn[u.ID] * 10

		if score > bestScore {
			bestScore = score
			best = u.ID
		}
	}
	return best
}

// codeWalkBFS performs a BFS from start through adj, limited to maxDepth hops.
// Returns visited units in BFS order (breadth-first, with stable alphabetical
// ordering within each depth level for T4 determinism).
func codeWalkBFS(start string, adj map[string][]string, maxDepth int) []visitedUnit {
	if start == "" {
		return nil
	}
	visited := map[string]bool{start: true}
	result := []visitedUnit{{id: start, depth: 0}}
	queue := []visitedUnit{{id: start, depth: 0}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr.depth >= maxDepth {
			continue
		}

		// Neighbors are pre-sorted in buildCodeAdj — iterate in that stable order.
		for _, next := range adj[curr.id] {
			if !visited[next] {
				visited[next] = true
				vu := visitedUnit{id: next, depth: curr.depth + 1}
				result = append(result, vu)
				queue = append(queue, vu)
			}
		}
	}
	return result
}

// pickSymbol selects the best representative symbol for a unit from the index.
//
// Lookup strategy (two-tier):
//  1. Direct: CodeSymbols.ByFile[unit.File] — the unit's known defining file.
//  2. Directory fallback: any ByFile entry whose filepath.Dir matches the unit path.
//
// Within the candidates, preference order (MIXED — documented heuristic):
//  1. Exported functions (Name[0] uppercase, Kind contains "func").
//  2. Exported types/structs/interfaces (Name[0] uppercase, non-func).
//  3. First symbol by StartLine ascending.
//
// Returns nil when no symbol data is available for the unit.
func pickSymbol(idx *CodeSymbolIndex, u UnitFact) *CodeSymbol {
	if idx == nil {
		return nil
	}

	// Tier 1: defining file direct lookup.
	if u.File != "" {
		if syms, ok := idx.ByFile[u.File]; ok && len(syms) > 0 {
			return bestSymbol(syms)
		}
	}

	// Tier 2: directory-prefix fallback — scan all files in the unit's directory.
	unitDir := u.Path
	if filepath.Ext(unitDir) != "" {
		// Path looks like a file; use its directory.
		unitDir = filepath.Dir(unitDir)
	}
	if unitDir == "." || unitDir == "" {
		return nil
	}

	var candidates []CodeSymbol
	for filePath, syms := range idx.ByFile {
		if filepath.Dir(filePath) == unitDir {
			candidates = append(candidates, syms...)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	// Sort candidates by file then StartLine for determinism before picking.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].File != candidates[j].File {
			return candidates[i].File < candidates[j].File
		}
		return candidates[i].StartLine < candidates[j].StartLine
	})
	return bestSymbol(candidates)
}

// bestSymbol picks the highest-priority symbol from a non-empty slice.
// Preference: exported func > exported type/struct/interface > first by StartLine.
func bestSymbol(syms []CodeSymbol) *CodeSymbol {
	if len(syms) == 0 {
		return nil
	}

	// Score: higher = better.
	score := func(s *CodeSymbol) int {
		if s.Name == "" {
			return 0
		}
		exported := unicode.IsUpper(rune(s.Name[0]))
		k := strings.ToLower(s.Kind)
		switch {
		case exported && strings.Contains(k, "func"):
			return 3
		case exported && (strings.Contains(k, "type") || strings.Contains(k, "struct") || strings.Contains(k, "interface")):
			return 2
		case exported:
			return 1
		default:
			return 0
		}
	}

	best := &syms[0]
	bestScore := score(best)
	for i := range syms[1:] {
		s := &syms[i+1]
		if sc := score(s); sc > bestScore {
			bestScore = sc
			best = s
		} else if sc == bestScore && s.StartLine < best.StartLine {
			best = s
		}
	}
	return best
}
