package kglab

// ontology.go — the Angle of Attack ontology: the 16 language-agnostic
// development concepts (verified against internal/domain/analyzer/lang_map.go)
// as DATA, so the graph is concept-driven, not hardcoded to imports.
//
// aOa's recon layer already collapses 509 tree-sitter languages into these 16
// concepts. Here we record each concept's GRAPH ROLE and wiring STATUS, so the
// completeness ledger can show which concepts are real graph substrate today
// (only :import) vs recon-matched vs stub — and adding a concept edge becomes a
// data edit, not an engine change.

// ConceptRole is a concept's role in the knowledge graph.
type ConceptRole string

const (
	RoleNode     ConceptRole = "node"     // the concept IS a node (function/class/interface)
	RoleEdge     ConceptRole = "edge"     // a directed relationship (import/call)
	RoleProperty ConceptRole = "property" // a node attribute / recon finding signal, NOT topology
)

// ConceptStatus is how far a concept is wired into the graph substrate.
type ConceptStatus string

const (
	StatusWired ConceptStatus = "wired" // real graph substrate today (DepFact/UnitFact)
	StatusRecon ConceptStatus = "recon" // recon matches it, but it emits findings, not graph facts
	StatusStub  ConceptStatus = "stub"  // recognized concept, no pipeline at all
)

// ConceptDef is one row of the ontology.
type ConceptDef struct {
	Name      string        `json:"name"`      // matches analyzer.Concept* exactly
	Role      ConceptRole   `json:"role"`      // node | edge | property
	Status    ConceptStatus `json:"status"`    // wired | recon | stub
	EdgeToken string        `json:"edge_token"` // DepFact.Kind wire value for edges ("imports"); "" otherwise
	Evidence  string        `json:"evidence"`  // file:line or source, or "none"
}

// Ontology is the 16-concept registry. Exactly one edge is WIRED today (import).
// Role split is honest: node=3, edge=2, property=11. "contains" is deliberately
// NOT here — it is not a lang_map constant, so inventing it would fabricate a concept.
var Ontology = []ConceptDef{
	// EDGE concepts
	{"import", RoleEdge, StatusWired, "imports", "internal/domain/arch/model.go:34 (DepFact is the import edge)"},
	{"call", RoleEdge, StatusRecon, "calls", "recon/rules/README.md (recon matches call; no DepFact{Kind:calls} pipeline)"},
	// NODE concepts (recognized, no NodeFact/Compile path yet)
	{"function", RoleNode, StatusStub, "", "recon/rules/README.md (matched; no FunctionFact)"},
	{"class", RoleNode, StatusStub, "", "recon/rules/README.md (matched; no ClassFact)"},
	{"interface", RoleNode, StatusStub, "", "recon/rules/README.md (matched; no InterfaceFact)"},
	// PROPERTY concepts (node attributes / recon finding signals, not topology)
	{"assignment", RoleProperty, StatusRecon, "", "recon/rules/README.md"},
	{"for_loop", RoleProperty, StatusRecon, "", "recon/rules/README.md"},
	{"return", RoleProperty, StatusRecon, "", "recon/rules/README.md"},
	{"block", RoleProperty, StatusRecon, "", "recon/rules/README.md"},
	{"switch", RoleProperty, StatusRecon, "", "recon/rules/README.md"},
	{"string_literal", RoleProperty, StatusRecon, "", "recon/rules/README.md"},
	{"string_concat", RoleProperty, StatusRecon, "", "recon/rules/README.md"},
	{"format_call", RoleProperty, StatusRecon, "", "recon/rules/README.md"},
	{"defer", RoleProperty, StatusRecon, "", "recon/rules/README.md"},
	{"type_assertion", RoleProperty, StatusRecon, "", "recon/rules/README.md"},
	{"catch_clause", RoleProperty, StatusRecon, "", "recon/rules/README.md (the 16th lang_map constant)"},
}

// OntologyByName looks up a concept by its canonical name.
func OntologyByName(name string) (ConceptDef, bool) {
	for _, c := range Ontology {
		if c.Name == name {
			return c, true
		}
	}
	return ConceptDef{}, false
}

// OntologyByEdgeToken looks up an edge concept by its wire token (e.g. "imports").
func OntologyByEdgeToken(token string) (ConceptDef, bool) {
	for _, c := range Ontology {
		if c.Role == RoleEdge && c.EdgeToken == token {
			return c, true
		}
	}
	return ConceptDef{}, false
}

// EdgeKindHasSubstrate reports whether an edge token/name is a WIRED edge concept
// that can actually be traversed today. Accepts either the wire token ("imports")
// or the concept name ("import") — resolving the plural/singular collision.
func EdgeKindHasSubstrate(token string) bool {
	for _, c := range Ontology {
		if c.Role == RoleEdge && c.Status == StatusWired && (c.Name == token || c.EdgeToken == token) {
			return true
		}
	}
	return false
}

// edgeReason explains why an edge token is not traversable (for honest errors).
func edgeReason(token string) string {
	c, ok := OntologyByName(token)
	if !ok {
		c, ok = OntologyByEdgeToken(token)
	}
	if !ok {
		return "unknown concept — not in the 16-concept ontology"
	}
	switch {
	case c.Role == RoleProperty:
		return "concept is a property (recon signal), not a graph edge"
	case c.Role == RoleNode:
		return "concept is a node type, not an edge"
	case c.Role == RoleEdge && c.Status == StatusRecon:
		return "concept is recon-matched but not yet wired as a graph edge"
	default:
		return "concept has no graph substrate"
	}
}
