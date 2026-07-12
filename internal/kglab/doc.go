// Package kglab is an isolated "knowledge-graph laboratory": it proves the
// reframe that an architectural blueprint is a QUERY over a knowledge graph,
// not a hand-drawn diagram.
//
// A single ViewQuery IR compiles to real aOa primitives —
//
//	filter (Select) -> adjacency -> bfsReachable (Traverse) ->
//	arch.Group -> arch.Detect -> one of the 4 real renderers -> arch.Shard
//
// and renders MULTIPLE blueprint types (component, cycles, DSM) from one struct.
//
// It is deliberately self-contained: it imports only internal/domain/arch (a
// pure, stdlib-only package), seeds a deterministic in-memory graph (no daemon,
// no bbolt, no parser), and clean-room copies the ~20-line bfsReachable walk so
// it never depends on internal/app. Live-daemon integration and a text grammar
// for ViewQuery are explicitly out of scope — see .context/details.
//
// Honesty is enforced by construction: any query for a call/sequence/dataflow
// edge kind, an unresolved seed, or a node count over budget returns an error
// rather than fabricating a shard. Only :IMPORTS (arch.DepFact) has substrate.
package kglab
