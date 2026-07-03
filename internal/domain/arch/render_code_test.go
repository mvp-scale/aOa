package arch

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// makeCodeFixture — synthetic fixture for render_code tests
//
// Extends makeFixture() with a CodeSymbolIndex whose entries cover the
// defining files of each unit. Symbols are synthetic but carry realistic
// file:line data so golden tests remain byte-stable.
// ---------------------------------------------------------------------------

func makeCodeFixture() RenderInput {
	base := makeFixture()
	base.CodeSymbols = &CodeSymbolIndex{
		ByFile: map[string][]CodeSymbol{
			"cmd/main.go": {
				{Name: "main", Kind: "func", File: "cmd/main.go", StartLine: 1, EndLine: 20},
			},
			"cmd/grep.go": {
				{Name: "RunGrep", Kind: "func", Signature: "func RunGrep()", File: "cmd/grep.go", StartLine: 1, EndLine: 40},
			},
			"cmd/init.go": {
				{Name: "RunInit", Kind: "func", Signature: "func RunInit()", File: "cmd/init.go", StartLine: 1, EndLine: 30},
			},
			"internal/app/app.go": {
				{Name: "App", Kind: "struct", File: "internal/app/app.go", StartLine: 1, EndLine: 10},
				{Name: "Run", Kind: "func", Signature: "func (a *App) Run()", File: "internal/app/app.go", StartLine: 25, EndLine: 50},
			},
			"internal/adapters/bbolt/store.go": {
				{Name: "Store", Kind: "struct", File: "internal/adapters/bbolt/store.go", StartLine: 10, EndLine: 20},
			},
			"internal/adapters/socket/server.go": {
				{Name: "Server", Kind: "struct", File: "internal/adapters/socket/server.go", StartLine: 5, EndLine: 15},
			},
			"internal/ports/storage.go": {
				{Name: "IndexStore", Kind: "interface", File: "internal/ports/storage.go", StartLine: 10, EndLine: 30},
			},
			"internal/domain/enricher/enrich.go": {
				{Name: "Enrich", Kind: "func", File: "internal/domain/enricher/enrich.go", StartLine: 1, EndLine: 25},
			},
			"internal/domain/learner/learn.go": {
				{Name: "Learner", Kind: "struct", File: "internal/domain/learner/learn.go", StartLine: 1, EndLine: 15},
			},
			"internal/domain/searcher/search.go": {
				{Name: "Search", Kind: "func", File: "internal/domain/searcher/search.go", StartLine: 1, EndLine: 35},
			},
		},
	}
	return base
}

// ---------------------------------------------------------------------------
// T4 — Golden determinism for the code shard
// ---------------------------------------------------------------------------

func TestDeterminism_Code(t *testing.T) {
	in := makeCodeFixture()

	s1, err := RenderCode(in)
	require.NoError(t, err)
	b1, err := MarshalShard(s1)
	require.NoError(t, err)

	s2, err := RenderCode(in)
	require.NoError(t, err)
	b2, err := MarshalShard(s2)
	require.NoError(t, err)

	require.Equal(t, string(b1), string(b2),
		"code shard must be byte-identical across two independent renders (T4)")

	checkAndUpdateGolden(t, "testdata/golden_code.json", b1)
}

// ---------------------------------------------------------------------------
// Provenance
// ---------------------------------------------------------------------------

func TestRenderCode_Provenance(t *testing.T) {
	in := makeCodeFixture()
	s, err := RenderCode(in)
	require.NoError(t, err)

	// Kind must be "simple" (T-2 team decision: code shard kind = simple).
	assert.Equal(t, "simple", s.Kind, "code shard kind must be 'simple' (T-2)")

	// Prov must be MIXED — subset choice is heuristic.
	assert.Equal(t, "mixed", s.Prov.Kind,
		"code shard prov must be MIXED (symbols REAL, subset MIXED)")

	// Label must surface the split honestly.
	assert.Contains(t, s.Prov.Label, "MIXED",
		"prov label must say MIXED")
	assert.Contains(t, s.Prov.Label, "real",
		"prov label must surface that symbols are real")
	assert.Contains(t, s.Prov.Label, "subset",
		"prov label must acknowledge subset heuristic")
}

// ---------------------------------------------------------------------------
// REAL nodes — all nodes carry file:line and real=true
// ---------------------------------------------------------------------------

func TestRenderCode_RealNodes(t *testing.T) {
	in := makeCodeFixture()
	s, err := RenderCode(in)
	require.NoError(t, err)

	require.NotEmpty(t, s.Nodes, "code shard must have at least one node")

	for i, n := range s.Nodes {
		assert.True(t, n.Real, "node[%d] %q must have real=true (G7)", i, n.ID)
		assert.NotEmpty(t, n.Sources, "node[%d] %q must carry at least one source ref (G7 file:line)", i, n.ID)
		if len(n.Sources) > 0 {
			assert.NotEmpty(t, n.Sources[0].File,
				"node[%d] %q source must have non-empty file", i, n.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Caption — custom format per WP step 2
// ---------------------------------------------------------------------------

func TestRenderCode_Caption(t *testing.T) {
	in := makeCodeFixture()
	s, err := RenderCode(in)
	require.NoError(t, err)

	assert.Contains(t, s.Count, "critical path",
		"code shard caption must mention 'critical path'")
	assert.Contains(t, s.Count, "entrypoint",
		"code shard caption must name the entrypoint")
}

// ---------------------------------------------------------------------------
// Absent when CodeSymbols is nil — no phantom shard
// ---------------------------------------------------------------------------

func TestRenderCode_AbsentWhenNoSymbols(t *testing.T) {
	// RenderAll with nil symbolIndex must NOT produce a "code" shard.
	svc := &Service{}
	in := makeFixture()

	shards, manifest, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, nil)
	require.NoError(t, err)

	_, hasCode := shards["code"]
	assert.False(t, hasCode, "code shard must be absent when symbolIndex is nil (no phantom)")

	for _, v := range manifest.Views {
		assert.NotEqual(t, "code", v.ID,
			"manifest must not list code view when symbolIndex is nil")
	}
}

// ---------------------------------------------------------------------------
// Present when CodeSymbols is non-nil — code view appears in manifest
// ---------------------------------------------------------------------------

func TestRenderCode_PresentWithSymbols(t *testing.T) {
	svc := &Service{}
	in := makeCodeFixture()

	shards, manifest, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, in.CodeSymbols)
	require.NoError(t, err)

	_, hasCode := shards["code"]
	assert.True(t, hasCode, "code shard must be present when symbolIndex is non-nil")

	found := false
	for _, v := range manifest.Views {
		if v.ID == "code" {
			found = true
			assert.Equal(t, "mixed", v.Prov, "manifest code entry must carry prov=mixed")
			assert.Len(t, v.Hash, 12, "code manifest hash must be 12 chars")
			break
		}
	}
	assert.True(t, found, "manifest must include 'code' view entry when symbolIndex is non-nil")
}

// ---------------------------------------------------------------------------
// T22 extension — byte-stability under input permutation with code view
// ---------------------------------------------------------------------------

func TestT22_Code_ByteStability_UnderPermutation(t *testing.T) {
	svc := &Service{}
	in := makeCodeFixture()

	// Render with canonical order.
	shards1, _, _, err := svc.RenderAll(in.Scope, in.Units, in.Deps, nil, nil, in.CodeSymbols)
	require.NoError(t, err)

	// Shuffle units and deps.
	shuffledUnits := make([]UnitFact, len(in.Units))
	copy(shuffledUnits, in.Units)
	shuffleDeterministic(shuffledUnits)

	shuffledDeps := make([]DepFact, len(in.Deps))
	copy(shuffledDeps, in.Deps)
	shuffleDepsDeterministic(shuffledDeps)

	// Render with shuffled order.
	shards2, _, _, err := svc.RenderAll(in.Scope, shuffledUnits, shuffledDeps, nil, nil, in.CodeSymbols)
	require.NoError(t, err)

	code1, ok1 := shards1["code"]
	code2, ok2 := shards2["code"]
	require.True(t, ok1, "code shard must be present in first render")
	require.True(t, ok2, "code shard must be present in second render")

	assert.Equal(t, string(code1), string(code2),
		"code shard must be byte-identical under input permutation (T22)")
}

// ---------------------------------------------------------------------------
// Entrypoint heuristic — cmd/main wins over other units
// ---------------------------------------------------------------------------

func TestSelectCodeEntrypoint_CmdMain(t *testing.T) {
	units := []UnitFact{
		{ID: "m_cmd_main", Label: "cmd/main", Path: "cmd/main.go"},
		{ID: "m_cmd_grep", Label: "cmd/grep", Path: "cmd/grep.go"},
		{ID: "m_app", Label: "app", Path: "internal/app/app.go"},
	}
	deps := []DepFact{
		// cmd/main imports app (fan-in for app = 1).
		{FromUnit: "m_cmd_main", ToUnit: "m_app"},
	}
	ep := selectCodeEntrypoint(units, deps)
	assert.Equal(t, "m_cmd_main", ep,
		"cmd/main must win entrypoint selection (cmd+main score = highest)")
}

func TestSelectCodeEntrypoint_FallbackNoCmdUnit(t *testing.T) {
	// No "cmd" unit — fallback should return lowest-fan-in or alphabetical.
	units := []UnitFact{
		{ID: "m_a", Label: "a", Path: "internal/a/a.go"},
		{ID: "m_b", Label: "b", Path: "internal/b/b.go"},
	}
	deps := []DepFact{
		{FromUnit: "m_a", ToUnit: "m_b"},
	}
	ep := selectCodeEntrypoint(units, deps)
	// m_a has fan-in 0, m_b has fan-in 1 → m_a wins.
	assert.Equal(t, "m_a", ep,
		"unit with lower fan-in must win when no cmd unit present")
}

// ---------------------------------------------------------------------------
// pickSymbol — priority: exported func > exported type > first by line
// ---------------------------------------------------------------------------

func TestPickSymbol_ExportedFuncWins(t *testing.T) {
	idx := &CodeSymbolIndex{
		ByFile: map[string][]CodeSymbol{
			"pkg/a.go": {
				{Name: "init", Kind: "func", File: "pkg/a.go", StartLine: 1},
				{Name: "Run", Kind: "func", File: "pkg/a.go", StartLine: 5},   // exported func
				{Name: "Config", Kind: "struct", File: "pkg/a.go", StartLine: 3},
			},
		},
	}
	u := UnitFact{ID: "m_a", File: "pkg/a.go", Path: "pkg/a.go"}
	sym := pickSymbol(idx, u)
	require.NotNil(t, sym)
	assert.Equal(t, "Run", sym.Name, "exported func must beat exported struct and unexported func")
}

func TestPickSymbol_ExportedTypeWhenNoExportedFunc(t *testing.T) {
	idx := &CodeSymbolIndex{
		ByFile: map[string][]CodeSymbol{
			"pkg/a.go": {
				{Name: "unexportedFn", Kind: "func", File: "pkg/a.go", StartLine: 1},
				{Name: "Config", Kind: "struct", File: "pkg/a.go", StartLine: 3},
			},
		},
	}
	u := UnitFact{ID: "m_a", File: "pkg/a.go", Path: "pkg/a.go"}
	sym := pickSymbol(idx, u)
	require.NotNil(t, sym)
	assert.Equal(t, "Config", sym.Name, "exported struct beats unexported func")
}

func TestPickSymbol_NilWhenNoMatch(t *testing.T) {
	idx := &CodeSymbolIndex{ByFile: map[string][]CodeSymbol{}}
	u := UnitFact{ID: "m_a", File: "pkg/a.go", Path: "pkg"}
	sym := pickSymbol(idx, u)
	assert.Nil(t, sym, "nil expected when no symbols for unit")
}

// ---------------------------------------------------------------------------
// JSON shape — code shard must be valid JSON parseable as a Shard
// ---------------------------------------------------------------------------

func TestRenderCode_JSONShape(t *testing.T) {
	in := makeCodeFixture()
	s, err := RenderCode(in)
	require.NoError(t, err)
	b, err := MarshalShard(s)
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &out), "code shard must be valid JSON")

	assert.Equal(t, "simple", out["kind"], "JSON kind must be 'simple'")
	assert.NotNil(t, out["nodes"], "JSON must contain nodes")
	assert.NotNil(t, out["prov"], "JSON must contain prov")

	// Edges are expected (chain between nodes) but only when >1 node.
	nodes, _ := out["nodes"].([]interface{})
	if len(nodes) > 1 {
		assert.NotNil(t, out["edges"], "JSON must contain edges when >1 node")
	}
}
