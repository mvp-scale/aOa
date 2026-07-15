package arch

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeContextFixture extends makeFixture() with external ("ext:") units and
// dep edges, matching the shape unitFactsFromFactStore mints in production
// (arch_factstore_bridge.go:94-221): external units are minted lazily from
// dep-edge targets, carry Path "ext:..." verbatim, and a friendly Label
// (upstream unitLabel already stripped the namespace prefix).
func makeContextFixture() RenderInput {
	in := makeFixture()

	extUnits := []UnitFact{
		{ID: "ext:postgres", Label: "postgres", Path: "ext:postgres"},
		{ID: "ext:redis", Label: "redis", Path: "ext:redis"},
		{ID: "ext:go.etcd.io/bbolt", Label: "bbolt", Path: "ext:go.etcd.io/bbolt"},
	}
	extDeps := []DepFact{
		// app talks to postgres twice (two call sites) — heaviest external edge.
		{FromUnit: "m_app", ToUnit: "ext:postgres", Count: 2, File: "internal/app/app.go", Line: 30},
		{FromUnit: "m_domain_searcher", ToUnit: "ext:postgres", Count: 1, File: "internal/domain/searcher/search.go", Line: 12},
		{FromUnit: "m_app", ToUnit: "ext:redis", Count: 1, File: "internal/app/app.go", Line: 31},
		// adapters/bbolt imports the real bbolt package.
		{FromUnit: "m_adapters_bbolt", ToUnit: "ext:go.etcd.io/bbolt", Count: 1, File: "internal/adapters/bbolt/store.go", Line: 3},
	}

	in.Units = append(append([]UnitFact{}, in.Units...), extUnits...)
	in.Deps = append(append([]DepFact{}, in.Deps...), extDeps...)
	return in
}

// ---------------------------------------------------------------------------
// T4 — Golden determinism: same input -> byte-identical shard across runs
// ---------------------------------------------------------------------------

func TestDeterminism_Context(t *testing.T) {
	in := makeContextFixture()

	s1, err := RenderContext(in)
	require.NoError(t, err)
	b1, err := MarshalShard(s1)
	require.NoError(t, err)

	s2, err := RenderContext(in)
	require.NoError(t, err)
	b2, err := MarshalShard(s2)
	require.NoError(t, err)

	require.Equal(t, string(b1), string(b2),
		"context shard must be byte-identical across two independent renders (T4)")

	checkAndUpdateGolden(t, "testdata/golden_context.json", b1)
}

func TestRenderContext_Provenance(t *testing.T) {
	in := makeContextFixture()

	s, err := RenderContext(in)
	require.NoError(t, err)

	assert.Equal(t, "mixed", s.Prov.Kind, "context view is always MIXED — external naming is heuristic (D2)")
}

func TestRenderContext_AbsentWhenNoExternalUnits(t *testing.T) {
	in := makeFixture() // no "ext:" units/deps

	s, err := RenderContext(in)
	require.NoError(t, err)

	assert.Empty(t, s.Nodes, "no external relationships in the fact set -> no nodes")
	assert.Empty(t, s.Edges, "no external relationships in the fact set -> no edges")
	assert.Equal(t, "0 external systems", s.Count)
}

func TestRenderContext_SystemNodeAndExternalNodesPresent(t *testing.T) {
	in := makeContextFixture()

	s, err := RenderContext(in)
	require.NoError(t, err)

	require.Len(t, s.Nodes, 4, "1 system node + 3 external units")

	var sysNode *Node
	extLabels := map[string]bool{}
	for i := range s.Nodes {
		n := &s.Nodes[i]
		if n.Type == "sys" {
			sysNode = n
			continue
		}
		assert.Equal(t, "ext", n.Type, "every non-system node must be type=ext")
		assert.False(t, n.Real, "external nodes are heuristically named, never Real")
		extLabels[n.Label] = true
	}
	require.NotNil(t, sysNode, "exactly one system-identity node must be present")
	assert.True(t, sysNode.Real, "the system node grounds in real local code")

	for _, want := range []string{"postgres", "redis", "bbolt"} {
		assert.True(t, extLabels[want], "external node %q must be present", want)
	}
}

func TestRenderContext_EdgesAggregatedPerExternalUnit(t *testing.T) {
	in := makeContextFixture()

	s, err := RenderContext(in)
	require.NoError(t, err)

	require.Len(t, s.Edges, 3, "one aggregated edge per external unit (postgres, redis, bbolt)")

	byTarget := make(map[string]ShardEdge, len(s.Edges))
	for _, e := range s.Edges {
		byTarget[e.Target] = e
	}

	pg, ok := byTarget["ext:postgres"]
	require.True(t, ok, "postgres edge must be present")
	assert.Equal(t, 3, pg.Count, "app(2) + domain_searcher(1) must sum to 3")
	assert.Equal(t, "imports", pg.Label, "D28: default DepFact.Kind is imports")
	assert.Equal(t, "sys_self", pg.Source)
}

func TestRenderContext_Caption(t *testing.T) {
	in := makeContextFixture()

	s, err := RenderContext(in)
	require.NoError(t, err)

	assert.Equal(t, "3 external systems · 3 relationships", s.Count)
	assert.NotContains(t, s.Count, "⚠", "A3: calm caption must never carry a findings glyph")
}

func TestRenderContext_NodeBudgetCapped(t *testing.T) {
	in := makeContextFixture()
	// Blow past simple_view_nodes_max (30): add 40 more distinct external units.
	for i := 0; i < 40; i++ {
		id := "ext:svc" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		in.Units = append(in.Units, UnitFact{ID: id, Label: id, Path: id})
		in.Deps = append(in.Deps, DepFact{FromUnit: "m_app", ToUnit: id, Count: 1, File: "internal/app/app.go", Line: 40})
	}

	s, err := RenderContext(in)
	require.NoError(t, err)

	assert.LessOrEqual(t, len(s.Nodes), 30, "simple_view_nodes_max budget must be enforced")
}

func TestShardSchema_Context(t *testing.T) {
	in := makeContextFixture()
	s, err := RenderContext(in)
	require.NoError(t, err)
	b, err := MarshalShard(s)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))

	assert.Equal(t, "simple", m["kind"])
	assert.NotNil(t, m["prov"])
	assert.NotNil(t, m["count"])

	nodes, ok := m["nodes"].([]interface{})
	require.True(t, ok)
	for i, nRaw := range nodes {
		nMap, ok2 := nRaw.(map[string]interface{})
		require.True(t, ok2, "node[%d] must be an object", i)
		assert.NotEmpty(t, nMap["id"], "node[%d].id", i)
		assert.NotEmpty(t, nMap["label"], "node[%d].label", i)
		assert.NotEmpty(t, nMap["type"], "node[%d].type", i)
	}

	edges, ok := m["edges"].([]interface{})
	require.True(t, ok)
	for i, eRaw := range edges {
		eMap, ok2 := eRaw.(map[string]interface{})
		require.True(t, ok2, "edge[%d] must be an object", i)
		assert.NotEmpty(t, eMap["source"], "edge[%d].source", i)
		assert.NotEmpty(t, eMap["target"], "edge[%d].target", i)
		assert.NotEmpty(t, eMap["label"], "edge[%d].label", i)
	}
}
