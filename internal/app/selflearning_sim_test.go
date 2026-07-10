package app

// End-to-end SIMULATION of the whole self-learning loop, off to the side.
//
// This chains Level A (write side) and Level D (read side / linkage) through the
// REAL pipeline — synthetic conversation → real enricher → real observe/autotune/
// dedup → real arch join — entirely in memory. No daemon, no real db, no real
// metadata touched. You dial two knobs (the synthetic project `idx` and the
// conversational `feed`) and watch the substrate (learned graph edges) change.
// Because it's the same code production runs, a green here is confidence it works
// in the real system without changing any real metadata.

import (
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type feedTurn struct {
	text  string // a synthetic conversational turn (real atlas keyword(s))
	times int    // how many turns of it — this is the "usage ratio" knob
}

// unitNodesFromIdx builds one graph node per distinct unit in the synthetic index,
// so mergeLearnedEdges (which only connects real nodes) can attach learned edges.
func unitNodesFromIdx(idx *ports.Index) []ports.GraphNode {
	seen := map[string]bool{}
	var nodes []ports.GraphNode
	for _, fm := range idx.Files {
		u := unitSlug(filepath.Dir(fm.Path))
		if !seen[u] {
			seen[u] = true
			nodes = append(nodes, ports.GraphNode{ID: u})
		}
	}
	return nodes
}

// simulateLearningLoop runs the FULL loop and returns the substrate's learned edges.
func simulateLearningLoop(t *testing.T, idx *ports.Index, feed []feedTurn) []ports.GraphEdge {
	t.Helper()
	a := newTestAppWithStore(t)

	// 1. Feed synthetic conversation through the REAL enricher→observe path.
	a.promptN = 1
	for _, ft := range feed {
		for i := 0; i < ft.times; i++ {
			a.processConversationSignal(ft.text, false)
		}
	}
	// 2. Fire one real autotune cycle → decay + dedup elects project winners.
	a.promptN = 50
	a.processConversationSignal(feed[0].text, true)

	// 3. Read the learned state and run the REAL arch join into a substrate graph.
	cohit := a.Learner.State().CohitTermDomain
	payload := ports.GraphPayload{Grain: "unit", Nodes: unitNodesFromIdx(idx)}
	return mergeLearnedEdges(payload, idx, cohit).Edges
}

func hasEdge(edges []ports.GraphEdge, from, to, prov string) bool {
	for _, e := range edges {
		if e.From == from && e.To == to && e.Prov == prov {
			return true
		}
	}
	return false
}

// synthProject builds a side-sandbox codebase: one unit that USES a shared term
// ("execution"), and two candidate domain-carrier units (@state_machine, @graphql).
// The conversation decides which one the learned edge binds to.
func synthProject() *ports.Index {
	return &ports.Index{
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "Run", Kind: "function", Tags: []string{"execution"}},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "engine/run.go", Language: "go", Domain: "@scheduling"},     // uses term "execution"
			2: {Path: "sm/state.go", Language: "go", Domain: "@state_machine"},    // carries @state_machine
			3: {Path: "gql/exec.go", Language: "go", Domain: "@graphql"},          // carries @graphql
		},
	}
}

// TestSim_ConversationShapesSubstrate: the state_machine-heavy conversation makes the
// substrate bind engine→sm (mixed) and NOT engine→gql (dedup deleted the loser).
func TestSim_ConversationShapesSubstrate(t *testing.T) {
	idx := synthProject()
	engine, sm, gql := unitSlug("engine"), unitSlug("sm"), unitSlug("gql")

	// "interpret" → execution:state_machine ; "cost" → execution:graphql (real atlas).
	edges := simulateLearningLoop(t, idx, []feedTurn{
		{"interpret", 200}, // state_machine usage dominates
		{"cost", 60},
	})

	assert.True(t, hasEdge(edges, engine, sm, "mixed"),
		"state_machine-heavy conversation must bind engine→sm as a learned MIXED edge")
	assert.False(t, hasEdge(edges, engine, gql, "mixed"),
		"the losing graphql affinity was deleted by dedup — no engine→gql edge")
}

// TestSim_ManipulateAndFlip: change ONLY the conversation ratio and the substrate
// repoints. This is the "manipulate domain terms and iterate quickly" proof — same
// synthetic metadata, opposite usage, opposite learned edge.
func TestSim_ManipulateAndFlip(t *testing.T) {
	idx := synthProject()
	engine, sm, gql := unitSlug("engine"), unitSlug("sm"), unitSlug("gql")

	// Flip the ratio: graphql now dominates.
	edges := simulateLearningLoop(t, idx, []feedTurn{
		{"cost", 200}, // graphql usage dominates
		{"interpret", 60},
	})

	assert.True(t, hasEdge(edges, engine, gql, "mixed"),
		"flipping the conversation to graphql-heavy must repoint the substrate to engine→gql")
	assert.False(t, hasEdge(edges, engine, sm, "mixed"),
		"state_machine is now the loser — no engine→sm edge")
}

// TestSim_Deterministic: the whole loop is a pure function of (idx, feed).
func TestSim_Deterministic(t *testing.T) {
	idx := synthProject()
	feed := []feedTurn{{"interpret", 200}, {"cost", 60}}
	e1 := simulateLearningLoop(t, idx, feed)
	e2 := simulateLearningLoop(t, idx, feed)
	require.Equal(t, len(e1), len(e2), "edge count stable across runs")
	for i := range e1 {
		assert.Equal(t, e1[i], e2[i], "edge[%d] identical across runs", i)
	}
}
