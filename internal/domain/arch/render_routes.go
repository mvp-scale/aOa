package arch

import (
	"fmt"
	"sort"
)

// apiContractRowsMax caps the API Contract table at the shared simple-view
// node budget (same shared-constant rationale as sbomRowsMax/changeRowsMax).
const apiContractRowsMax = 30

// RenderAPIContract produces a "table" shard answering the API Contract
// question (view-standards.json): "What HTTP endpoints does this system
// expose, and what handles them?" One row per route-registration call found
// by the treesitter route extractor (in.Routes, VL-3, board #37 — the first
// `route`-kind fact, D1).
//
// Columns: method, path, handler, framework.
//
// Provenance: always "derived"/REAL (D2/D15) — method/path/handler are read
// straight off the AST call site, same honesty tier as import extraction
// (a literal syntactic match, no type resolution).
func RenderAPIContract(in RenderInput) (*Shard, error) {
	columns := []string{"method", "path", "handler", "framework"}

	sorted := make([]RouteEntry, len(in.Routes))
	copy(sorted, in.Routes)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Path != sorted[j].Path {
			return sorted[i].Path < sorted[j].Path
		}
		return sorted[i].Method < sorted[j].Method
	})

	trueTotal := len(sorted)

	shown := sorted
	if len(shown) > apiContractRowsMax {
		shown = shown[:apiContractRowsMax]
	}

	rows := make([][]string, 0, len(shown))
	for _, r := range shown {
		method := r.Method
		if method == "" {
			method = "—"
		}
		rows = append(rows, []string{method, r.Path, r.Handler, r.Framework})
	}
	if rows == nil {
		rows = [][]string{}
	}

	prov := Prov{Kind: "derived", Label: "REAL · tree-sitter call-site extraction (net/http + gin)"}
	shard := &Shard{
		Kind:    "table",
		Title:   "API Contract",
		Prov:    prov,
		Columns: columns,
		Rows:    rows,
	}
	_, shard.FindingsClause = DeriveCaption(shard, in.Findings)

	switch {
	case trueTotal == 0:
		shard.Count = "0 routes"
	case trueTotal > len(shown):
		shard.Count = fmt.Sprintf("%d routes — showing %d", trueTotal, len(shown))
	default:
		shard.Count = fmt.Sprintf("%d routes", trueTotal)
	}
	return shard, nil
}
