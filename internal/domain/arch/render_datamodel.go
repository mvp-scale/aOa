package arch

import (
	"fmt"
	"sort"
)

// dataModelNodesMax caps the Data Model / ER view at the shared
// simple/entity-view node budget (view-standards.json
// global.budgets.simple_view_nodes_max — same shared-constant rationale as
// sbomRowsMax/apiContractRowsMax).
const dataModelNodesMax = 30

// RenderDataModel produces an "entity" shard answering the Data Model / ER
// (view id "datamodel") question (view-standards.json:216-230): "What are
// the core entities, their key fields, and how do they relate?" One node
// per struct entity found by the treesitter schema extractor (in.Entities,
// COL-1 — the first `entity`-kind fact, D1).
//
// No relationship edges are emitted (D29 ruling: FK/relationship-verb
// detection is MIXED/overlay-only, a separate later slice) — the shard's
// Edges field is left empty, and the view honestly shows 0 relationships
// rather than guessing cardinality from field names.
//
// Zero-field structs (e.g. `type Empty struct{}`) are skipped: Node.Fields
// is additive-omitempty (D40), so an empty field list would drop the JSON
// key entirely, and the viewer's entity layout unconditionally reads
// n.fields.length (viewer.js:799) — a marker struct with no fields also
// isn't a meaningful row for this view's question.
//
// Provenance: always "derived"/REAL (D2/D15) — struct name and field names
// are read straight off the AST, same honesty tier as route extraction (a
// syntactic match, no type resolution).
func RenderDataModel(in RenderInput) (*Shard, error) {
	var withFields []EntityEntry
	for _, e := range in.Entities {
		if len(e.Fields) == 0 {
			continue
		}
		withFields = append(withFields, e)
	}

	sort.Slice(withFields, func(i, j int) bool {
		return withFields[i].Name < withFields[j].Name
	})

	trueTotal := len(withFields)

	shown := withFields
	if len(shown) > dataModelNodesMax {
		shown = shown[:dataModelNodesMax]
	}

	nodes := make([]Node, 0, len(shown))
	for _, e := range shown {
		fields := make([]string, len(e.Fields))
		copy(fields, e.Fields)
		nodes = append(nodes, Node{
			ID:     e.Name,
			Type:   "entity",
			Label:  e.Name,
			Real:   true,
			Tech:   e.Tech,
			Fields: fields,
			Sources: []SourceRef{
				{File: e.File, Line: e.Line},
			},
		})
	}

	prov := Prov{Kind: "derived", Label: "REAL · tree-sitter struct-field extraction (Go)"}
	shard := &Shard{
		Kind:  "entity",
		Title: "Data Model / ER",
		Prov:  prov,
		Nodes: nodes,
	}
	_, shard.FindingsClause = DeriveCaption(shard, in.Findings)

	switch {
	case trueTotal == 0:
		shard.Count = "0 entities"
	case trueTotal > len(shown):
		shard.Count = fmt.Sprintf("%d entities — showing %d", trueTotal, len(shown))
	default:
		shard.Count = fmt.Sprintf("%d entities", trueTotal)
	}
	return shard, nil
}
