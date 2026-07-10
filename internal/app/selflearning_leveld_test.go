package app

// Level D — the LINKAGE oracle for self-learning validation.
//
// The gap (consensus 2026-07-10-consensus-learner-loop.md §3–§4): the learner
// increments n-grams/cohits/cohit-terms and dedup elects a project-specific
// term→domain winner — but nothing reads that winner back into the substrate.
// deriveArch() (arch.go) builds the graph purely from import edges and never reads
// CohitTermDomain, so incrementing cohits cannot increase the evidence/patterns/
// domains we have. Seam C (§5, recommended) closes it: cohits → arch-derive edges,
// stamped MIXED, off the search hot path.
//
// This test asserts the linkage: a dedup-elected term→domain affinity must surface
// as a MIXED graph edge binding the unit that CONTAINS the term to the unit that
// CARRIES the domain — an edge the import graph does not show. It FAILS today
// (learnedAffinityEdges is a stub returning nil) because the linkage is unwired.

import (
	"encoding/json"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLevelD_LearnedCohitBecomesMixedGraphEdge is the linkage oracle.
func TestLevelD_LearnedCohitBecomesMixedGraphEdge(t *testing.T) {
	// Two units the import graph does NOT connect:
	//   unit "eng" (eng/run.go) — contains a symbol tagged with term "execution"
	//   unit "sm"  (sm/state.go) — CARRIES domain @state_machine
	idx := &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"run": {{FileID: 1, Line: 10}},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "Run", Kind: "function", Tags: []string{"execution"}},
			{FileID: 2, Line: 5}:  {Name: "State", Kind: "type", Tags: []string{"transition"}},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "eng/run.go", Language: "go", Domain: "@scheduling"},
			2: {Path: "sm/state.go", Language: "go", Domain: "@state_machine"},
		},
	}

	// The learner has elected execution→state_machine as THIS project's winner
	// (dedup-eligible: single strong container, well past DedupMinTotal).
	learned := map[string]uint32{"execution:state_machine": 150}

	engID := unitSlug("eng")
	smID := unitSlug("sm")

	edges := learnedAffinityEdges(idx, learned)

	// The linkage: a MIXED edge binds the term-owning unit → the domain-carrying unit.
	var found *ports.GraphEdge
	for i := range edges {
		if edges[i].From == engID && edges[i].To == smID {
			found = &edges[i]
			break
		}
	}
	require.NotNil(t, found,
		"a dedup-elected cohit (execution:state_machine) must produce a learned edge %s→%s that the import graph does not show — the linkage between incrementing cohits and the substrate",
		engID, smID)
	assert.Equal(t, "mixed", found.Prov,
		"a learned affinity edge must be stamped MIXED (inference-grade, never REAL)")
	assert.Empty(t, found.File,
		"a learned edge has no import statement — no file:line provenance")
}

// TestLevelD_NoLearnedSignal_NoLearnedEdges: with no dedup-elected affinity, the
// derive stays pure import-derived (no spurious mixed edges).
func TestLevelD_NoLearnedSignal_NoLearnedEdges(t *testing.T) {
	idx := &ports.Index{
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "Run", Tags: []string{"execution"}},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "eng/run.go", Domain: "@scheduling"},
			2: {Path: "sm/state.go", Domain: "@state_machine"},
		},
	}
	// Empty learned state → no learned edges.
	edges := learnedAffinityEdges(idx, map[string]uint32{})
	assert.Empty(t, edges, "no learned cohits → no learned edges (substrate unchanged)")
}

// TestLevelD_NoiseFloor: below dedup scale, a term→domain pairing is noise and must
// NOT shape the substrate. Guards against the promiscuous-atlas noise polluting the graph.
func TestLevelD_NoiseFloor(t *testing.T) {
	idx := &ports.Index{
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "Run", Tags: []string{"execution"}},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "eng/run.go", Domain: "@scheduling"},
			2: {Path: "sm/state.go", Domain: "@state_machine"},
		},
	}
	// count below learnedAffinityMinCohit (100) — not a dedup-elected winner.
	edges := learnedAffinityEdges(idx, map[string]uint32{"execution:state_machine": 99})
	assert.Empty(t, edges, "sub-threshold cohit is noise, not a project winner — no edge")
}

// TestLevelD_AdditiveOnly_ImportEdgesUntouched: the linkage is purely additive.
// mergeLearnedEdges must preserve every REAL import edge byte-for-byte (G7 provenance
// intact) and only APPEND mixed edges — never mutate or drop import edges.
func TestLevelD_AdditiveOnly_ImportEdgesUntouched(t *testing.T) {
	idx := &ports.Index{
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "Run", Tags: []string{"execution"}},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "eng/run.go", Domain: "@scheduling"},
			2: {Path: "sm/state.go", Domain: "@state_machine"},
		},
	}
	base := ports.GraphPayload{
		Grain: "unit",
		Nodes: []ports.GraphNode{{ID: unitSlug("eng")}, {ID: unitSlug("sm")}},
		Edges: []ports.GraphEdge{
			{From: unitSlug("eng"), To: "ext:std/fmt", File: "eng/run.go", Line: 3},
		},
	}
	merged := mergeLearnedEdges(base, idx, map[string]uint32{"execution:state_machine": 150})

	// The original REAL edge survives unchanged.
	var real *ports.GraphEdge
	for i := range merged.Edges {
		if merged.Edges[i].From == unitSlug("eng") && merged.Edges[i].To == "ext:std/fmt" {
			real = &merged.Edges[i]
		}
	}
	require.NotNil(t, real, "the import edge must survive the merge")
	assert.Equal(t, "eng/run.go", real.File, "REAL edge keeps its file:line provenance")
	assert.Empty(t, real.Prov, "REAL import edge is never stamped mixed")
	// And a mixed edge was appended.
	assert.Contains(t, merged.Edges, ports.GraphEdge{From: unitSlug("eng"), To: unitSlug("sm"), Prov: "mixed"})
}

// TestLevelD_Determinism_SameSnapshotSameGraph: same learned snapshot → byte-identical
// graph. Cross-time drift as the store decays is knowingly accepted (MIXED); within a
// rev the answer is reproducible — the contract from consensus §5.
func TestLevelD_Determinism_SameSnapshotSameGraph(t *testing.T) {
	idx := &ports.Index{
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "Run", Tags: []string{"execution"}},
			{FileID: 3, Line: 7}:  {Name: "Sched", Tags: []string{"execution"}},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "eng/run.go", Domain: "@scheduling"},
			2: {Path: "sm/state.go", Domain: "@state_machine"},
			3: {Path: "cron/sched.go", Domain: "@scheduling"},
		},
	}
	snap := map[string]uint32{"execution:state_machine": 150}
	base := func() ports.GraphPayload {
		return ports.GraphPayload{
			Grain: "unit",
			Nodes: []ports.GraphNode{{ID: unitSlug("eng")}, {ID: unitSlug("sm")}, {ID: unitSlug("cron")}},
			Edges: []ports.GraphEdge{},
		}
	}
	m1, err1 := json.Marshal(mergeLearnedEdges(base(), idx, snap))
	m2, err2 := json.Marshal(mergeLearnedEdges(base(), idx, snap))
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, string(m1), string(m2),
		"same learned snapshot must produce byte-identical graph (rev-reproducible)")
}
