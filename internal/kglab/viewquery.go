package kglab

import "github.com/corey/aoa/internal/domain/arch"

// ViewQuery is the single IR that represents a saved blueprint query.
//
// It is the primitive the whole reframe rests on: ~all blueprint views reduce
// to one struct with a different Render kind + optional Select/Traverse/Group.
// A future text grammar (Cypher/GQL-family) would parse TO this struct; for the
// lab we author it as a struct literal.
type ViewQuery struct {
	// Scope is a human label for the view (used in Shard.Title / Finding scope).
	Scope string

	// Select filters which units are in scope. nil = all units.
	Select *SelectSpec

	// Traverse walks the DepFact adjacency from a seed. nil = no traversal.
	Traverse *TraverseSpec

	// Group overrides grouping. nil = arch.Group (rung-2 path-prefix default).
	Group *arch.GroupOptions

	// Budget caps node count after traversal. Zero value = no cap.
	Budget BudgetSpec

	// Render selects one of the 4 real renderers.
	Render RenderSpec
}

// SelectSpec filters units. Semantics are OR: a unit is kept if its Path has
// PathPrefix OR its ID is in IDs. An empty spec keeps everything.
type SelectSpec struct {
	PathPrefix string   // keep units whose Path starts with this (empty = ignore)
	IDs        []string // keep units whose ID is in this list (nil = ignore)
}

// TraverseSpec drives the BFS over the :IMPORTS adjacency.
type TraverseSpec struct {
	Seed     string // starting unit ID (must exist in the selected set)
	Dir      string // "forward" (seed->deps) or "reverse" (deps->seed, i.e. blast radius)
	Hops     int    // hop budget k; <=0 = unbounded
	EdgeKind string // must be "imports" or ""; anything else is an honesty-gate error
}

// BudgetSpec caps the node count. Refusing (not truncating) keeps the view honest:
// a truncated graph silently fabricates a partial picture.
type BudgetSpec struct {
	MaxNodes int // >0 = error if the in-scope node count exceeds this
}

// RenderSpec selects the renderer. Kind is one of:
//
//	"component" -> arch.RenderComponent (Shard.Kind "buckets")
//	"cycles"    -> arch.RenderCycles    (Shard.Kind "table")
//	"dsm"       -> arch.RenderDSM       (Shard.Kind "matrix")
//	"code"      -> rejected: needs CodeSymbolIndex (out of scope for the lab)
//
// Any other value is an error.
type RenderSpec struct {
	Kind string
}
