package kglab

import (
	"strings"
	"testing"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- the three blueprint types, all from one ViewQuery IR -------------------

func TestCompile_ComponentBlueprint_ReturnsBucketsShard(t *testing.T) {
	units, deps := SampleGraph()
	shard, err := Compile(ViewQuery{Scope: "sample", Render: RenderSpec{Kind: "component"}}, units, deps)
	require.NoError(t, err)
	require.NotNil(t, shard)
	assert.Equal(t, "buckets", shard.Kind)
	assert.Len(t, shard.Buckets, 5, "cmd/app/domain/adapters/ports")
}

func TestCompile_CyclesBlueprint_ReturnsTableShard(t *testing.T) {
	units, deps := SampleGraph()
	shard, err := Compile(ViewQuery{Scope: "sample", Render: RenderSpec{Kind: "cycles"}}, units, deps)
	require.NoError(t, err)
	assert.Equal(t, "table", shard.Kind)
	require.Len(t, shard.Rows, 1, "one planted domain cycle")
	assert.Contains(t, strings.Join(shard.Rows[0], " "), "domain/searcher")
}

func TestCompile_DSMBlueprint_ReturnsMatrixShard(t *testing.T) {
	units, deps := SampleGraph()
	shard, err := Compile(ViewQuery{Scope: "sample", Render: RenderSpec{Kind: "dsm"}}, units, deps)
	require.NoError(t, err)
	assert.Equal(t, "matrix", shard.Kind)
	assert.Len(t, shard.Items, 5)
	assert.Len(t, shard.Matrix, 5, "square 5x5")
}

// --- honesty gates: refuse, never fabricate --------------------------------

func TestCompile_HonestyGate_CallEdgeKindFails(t *testing.T) {
	units, deps := SampleGraph()
	shard, err := Compile(ViewQuery{
		Scope:    "sample",
		Traverse: &TraverseSpec{Seed: "m_app", Hops: 1, EdgeKind: "calls"},
		Render:   RenderSpec{Kind: "component"},
	}, units, deps)
	require.Error(t, err)
	assert.Nil(t, shard)
	assert.Contains(t, err.Error(), "no substrate")
}

func TestCompile_HonestyGate_SequenceEdgeKindFails(t *testing.T) {
	units, deps := SampleGraph()
	_, err := Compile(ViewQuery{
		Scope:    "sample",
		Traverse: &TraverseSpec{Seed: "m_app", Hops: 1, EdgeKind: "sequence"},
		Render:   RenderSpec{Kind: "component"},
	}, units, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no substrate")
}

func TestCompile_HonestyGate_UnresolvedSeed(t *testing.T) {
	units, deps := SampleGraph()
	_, err := Compile(ViewQuery{
		Scope:    "sample",
		Traverse: &TraverseSpec{Seed: "m_nope", Hops: 2, EdgeKind: "imports"},
		Render:   RenderSpec{Kind: "component"},
	}, units, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCompile_HonestyGate_BudgetExceeded(t *testing.T) {
	units, deps := SampleGraph()
	_, err := Compile(ViewQuery{
		Scope:  "sample",
		Budget: BudgetSpec{MaxNodes: 3},
		Render: RenderSpec{Kind: "component"},
	}, units, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds budget")
}

func TestCompile_HonestyGate_RenderCodeRejected(t *testing.T) {
	units, deps := SampleGraph()
	_, err := Compile(ViewQuery{Scope: "sample", Render: RenderSpec{Kind: "code"}}, units, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of scope")
}

func TestCompile_HonestyGate_UnknownRenderKind(t *testing.T) {
	units, deps := SampleGraph()
	// "buckets" is a Shard.Kind value, NOT a query render kind — must be rejected.
	_, err := Compile(ViewQuery{Scope: "sample", Render: RenderSpec{Kind: "buckets"}}, units, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown render kind")
}

// --- select + traverse: the query actually narrows the graph ---------------

func TestCompile_SelectByPathPrefix(t *testing.T) {
	units, deps := SampleGraph()
	shard, err := Compile(ViewQuery{
		Scope:  "domain-only",
		Select: &SelectSpec{PathPrefix: "internal/domain"},
		Render: RenderSpec{Kind: "component"},
	}, units, deps)
	require.NoError(t, err)
	require.Len(t, shard.Buckets, 1, "only the domain group survives")
	assert.Equal(t, "domain", shard.Buckets[0].Label)
}

func TestCompile_TraversalFiltersToReachable(t *testing.T) {
	units, deps := SampleGraph()
	// Forward from app, 1 hop: app + {domain, adapters, ports} deps — cmd is NOT
	// reachable forward, so 4 groups, not 5.
	shard, err := Compile(ViewQuery{
		Scope:    "from-app",
		Traverse: &TraverseSpec{Seed: "m_app", Dir: "forward", Hops: 1, EdgeKind: "imports"},
		Render:   RenderSpec{Kind: "component"},
	}, units, deps)
	require.NoError(t, err)
	assert.Len(t, shard.Buckets, 4, "app/domain/adapters/ports; cmd excluded")
	for _, b := range shard.Buckets {
		assert.NotEqual(t, "cmd", b.Label, "cmd is upstream of app, not reachable forward")
	}
}

func TestCompile_ReverseTraversalIsBlastRadius(t *testing.T) {
	units, deps := SampleGraph()
	// Reverse from ports/storage: everyone who (transitively) imports it.
	shard, err := Compile(ViewQuery{
		Scope:    "blast-ports",
		Traverse: &TraverseSpec{Seed: "m_ports_storage", Dir: "reverse", Hops: 0, EdgeKind: "imports"},
		Render:   RenderSpec{Kind: "component"},
	}, units, deps)
	require.NoError(t, err)
	// ports is imported by app, domain, adapters, and transitively cmd -> all 5.
	assert.Len(t, shard.Buckets, 5)
}

// --- determinism + provenance ----------------------------------------------

func TestCompile_Determinism(t *testing.T) {
	units, deps := SampleGraph()
	q := ViewQuery{Scope: "sample", Render: RenderSpec{Kind: "dsm"}}
	s1, err := Compile(q, units, deps)
	require.NoError(t, err)
	s2, err := Compile(q, units, deps)
	require.NoError(t, err)
	b1, err := arch.MarshalShard(s1)
	require.NoError(t, err)
	b2, err := arch.MarshalShard(s2)
	require.NoError(t, err)
	assert.Equal(t, string(b1), string(b2), "same query -> byte-identical shard")
}

func TestCompile_Component_ProvIsReal(t *testing.T) {
	units, deps := SampleGraph()
	shard, err := Compile(ViewQuery{Scope: "sample", Render: RenderSpec{Kind: "component"}}, units, deps)
	require.NoError(t, err)
	// No overlay -> every edge is a REAL import edge -> provenance stays derived/REAL.
	assert.Equal(t, "derived", shard.Prov.Kind)
}
