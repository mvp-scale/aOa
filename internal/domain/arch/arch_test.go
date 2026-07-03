package arch

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// update regenerates golden files when -update is passed.
var update = flag.Bool("update", false, "update golden fixture files")

// ---------------------------------------------------------------------------
// Synthetic fixture
//
// 12 units across 6 path-prefix groups. Deliberately planted:
//   - One 3-unit dependency cycle: domain_searcher → domain_learner → domain_enricher → domain_searcher
//   - One god node (unit grain): m_app  (in=3 from cmd group; out=5 to domain/adapters/ports)
//   - One orphan unit: m_util_orphan  (in=0, out=0)
// ---------------------------------------------------------------------------

func makeFixture() RenderInput {
	units := []UnitFact{
		// cmd group (part 0 after sort)
		{ID: "m_cmd_grep", Label: "cmd/grep", Path: "cmd/grep.go", File: "cmd/grep.go", Line: 1},
		{ID: "m_cmd_init", Label: "cmd/init", Path: "cmd/init.go", File: "cmd/init.go", Line: 1},
		{ID: "m_cmd_main", Label: "cmd/main", Path: "cmd/main.go", File: "cmd/main.go", Line: 1},
		// app group
		{ID: "m_app", Label: "app", Path: "internal/app/app.go", File: "internal/app/app.go", Line: 1},
		// domain group
		{ID: "m_domain_enricher", Label: "domain/enricher", Path: "internal/domain/enricher/enrich.go", File: "internal/domain/enricher/enrich.go", Line: 1},
		{ID: "m_domain_indexer", Label: "domain/indexer", Path: "internal/domain/indexer/index.go", File: "internal/domain/indexer/index.go", Line: 1},
		{ID: "m_domain_learner", Label: "domain/learner", Path: "internal/domain/learner/learn.go", File: "internal/domain/learner/learn.go", Line: 1},
		{ID: "m_domain_searcher", Label: "domain/searcher", Path: "internal/domain/searcher/search.go", File: "internal/domain/searcher/search.go", Line: 1},
		// adapters group
		{ID: "m_adapters_bbolt", Label: "adapters/bbolt", Path: "internal/adapters/bbolt/store.go", File: "internal/adapters/bbolt/store.go", Line: 1},
		{ID: "m_adapters_socket", Label: "adapters/socket", Path: "internal/adapters/socket/server.go", File: "internal/adapters/socket/server.go", Line: 1},
		// ports group
		{ID: "m_ports_storage", Label: "ports/storage", Path: "internal/ports/storage.go", File: "internal/ports/storage.go", Line: 1},
		// util group — ORPHAN (no deps, nobody imports it)
		{ID: "m_util_orphan", Label: "util/orphan", Path: "util/orphan.go", File: "util/orphan.go", Line: 1},
	}

	deps := []DepFact{
		// cmd → app (3 units in cmd all import app → god node in=3)
		{FromUnit: "m_cmd_main", ToUnit: "m_app", Count: 1, File: "cmd/main.go", Line: 5},
		{FromUnit: "m_cmd_grep", ToUnit: "m_app", Count: 1, File: "cmd/grep.go", Line: 5},
		{FromUnit: "m_cmd_init", ToUnit: "m_app", Count: 1, File: "cmd/init.go", Line: 5},
		// cmd → ports
		{FromUnit: "m_cmd_main", ToUnit: "m_ports_storage", Count: 1, File: "cmd/main.go", Line: 6},
		// app → domain  (god out: searcher+learner = 2 of the 5)
		{FromUnit: "m_app", ToUnit: "m_domain_searcher", Count: 2, File: "internal/app/app.go", Line: 10},
		{FromUnit: "m_app", ToUnit: "m_domain_learner", Count: 1, File: "internal/app/app.go", Line: 11},
		// app → adapters  (god out: bbolt+socket = 2 of the 5)
		{FromUnit: "m_app", ToUnit: "m_adapters_bbolt", Count: 3, File: "internal/app/app.go", Line: 15},
		{FromUnit: "m_app", ToUnit: "m_adapters_socket", Count: 1, File: "internal/app/app.go", Line: 16},
		// app → ports  (god out: 1 of the 5)
		{FromUnit: "m_app", ToUnit: "m_ports_storage", Count: 1, File: "internal/app/app.go", Line: 20},
		// CYCLE: searcher → learner → enricher → searcher
		{FromUnit: "m_domain_searcher", ToUnit: "m_domain_learner", Count: 1, File: "internal/domain/searcher/search.go", Line: 8},
		{FromUnit: "m_domain_learner", ToUnit: "m_domain_enricher", Count: 1, File: "internal/domain/learner/learn.go", Line: 7},
		{FromUnit: "m_domain_enricher", ToUnit: "m_domain_searcher", Count: 1, File: "internal/domain/enricher/enrich.go", Line: 6},
		// domain → ports
		{FromUnit: "m_domain_searcher", ToUnit: "m_ports_storage", Count: 1, File: "internal/domain/searcher/search.go", Line: 9},
		{FromUnit: "m_domain_learner", ToUnit: "m_ports_storage", Count: 1, File: "internal/domain/learner/learn.go", Line: 8},
		{FromUnit: "m_domain_enricher", ToUnit: "m_ports_storage", Count: 1, File: "internal/domain/enricher/enrich.go", Line: 7},
		{FromUnit: "m_domain_indexer", ToUnit: "m_ports_storage", Count: 1, File: "internal/domain/indexer/index.go", Line: 5},
		// adapters → ports
		{FromUnit: "m_adapters_bbolt", ToUnit: "m_ports_storage", Count: 2, File: "internal/adapters/bbolt/store.go", Line: 5},
		{FromUnit: "m_adapters_socket", ToUnit: "m_ports_storage", Count: 1, File: "internal/adapters/socket/server.go", Line: 5},
	}

	grouping := Group(units)

	findings, sccs := Detect("test", units, deps, DefaultThresholds(), grouping, nil)

	return RenderInput{
		Scope:    "test",
		Units:    units,
		Deps:     deps,
		Grouping: grouping,
		SCCs:     sccs,
		Findings: findings,
	}
}

// ---------------------------------------------------------------------------
// T4 — Golden determinism: same input → byte-identical shard across runs
// ---------------------------------------------------------------------------

func TestDeterminism_Component(t *testing.T) {
	in := makeFixture()

	s1, err := RenderComponent(in)
	require.NoError(t, err)
	b1, err := MarshalShard(s1)
	require.NoError(t, err)

	s2, err := RenderComponent(in)
	require.NoError(t, err)
	b2, err := MarshalShard(s2)
	require.NoError(t, err)

	require.Equal(t, string(b1), string(b2),
		"component shard must be byte-identical across two independent renders (T4)")

	checkAndUpdateGolden(t, "testdata/golden_component.json", b1)
}

func TestDeterminism_DSM(t *testing.T) {
	in := makeFixture()

	s1, err := RenderDSM(in)
	require.NoError(t, err)
	b1, err := MarshalShard(s1)
	require.NoError(t, err)

	s2, err := RenderDSM(in)
	require.NoError(t, err)
	b2, err := MarshalShard(s2)
	require.NoError(t, err)

	require.Equal(t, string(b1), string(b2),
		"dsm shard must be byte-identical across two independent renders (T4)")

	checkAndUpdateGolden(t, "testdata/golden_dsm.json", b1)
}

func TestDeterminism_Cycles(t *testing.T) {
	in := makeFixture()

	s1, err := RenderCycles(in)
	require.NoError(t, err)
	b1, err := MarshalShard(s1)
	require.NoError(t, err)

	s2, err := RenderCycles(in)
	require.NoError(t, err)
	b2, err := MarshalShard(s2)
	require.NoError(t, err)

	require.Equal(t, string(b1), string(b2),
		"cycles shard must be byte-identical across two independent renders (T4)")

	checkAndUpdateGolden(t, "testdata/golden_cycles.json", b1)
}

// checkAndUpdateGolden either writes or verifies a golden file.
func checkAndUpdateGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, got, 0644))
		t.Logf("updated golden: %s (hash %s)", path, ContentHash(got))
		return
	}
	want, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf("golden file missing: %s — run: go test ./internal/domain/arch/ -update", path)
	}
	require.NoError(t, err)
	require.Equal(t, string(want), string(got),
		"golden mismatch in %s — run with -update to regenerate", path)
}

// ---------------------------------------------------------------------------
// T5 — Detector correctness: planted cycle/god/orphan all fire
// ---------------------------------------------------------------------------

func TestDetectors_Cycle(t *testing.T) {
	units := []UnitFact{
		{ID: "m_a", Label: "A", Path: "pkg/a/a.go", File: "pkg/a/a.go", Line: 1},
		{ID: "m_b", Label: "B", Path: "pkg/b/b.go", File: "pkg/b/b.go", Line: 1},
		{ID: "m_c", Label: "C", Path: "pkg/c/c.go", File: "pkg/c/c.go", Line: 1},
	}
	deps := []DepFact{
		{FromUnit: "m_a", ToUnit: "m_b", Count: 1, File: "pkg/a/a.go", Line: 5},
		{FromUnit: "m_b", ToUnit: "m_c", Count: 1, File: "pkg/b/b.go", Line: 5},
		{FromUnit: "m_c", ToUnit: "m_a", Count: 1, File: "pkg/c/c.go", Line: 5}, // closes cycle
	}

	findings, sccs := DetectCycles("test", units, deps)

	require.Len(t, sccs, 1, "exactly one SCC expected")
	assert.Len(t, sccs[0], 3, "SCC must contain all three units")

	// All three units must appear in the SCC.
	sccSet := make(map[string]bool)
	for _, m := range sccs[0] {
		sccSet[m] = true
	}
	assert.True(t, sccSet["m_a"], "m_a must be in cycle SCC")
	assert.True(t, sccSet["m_b"], "m_b must be in cycle SCC")
	assert.True(t, sccSet["m_c"], "m_c must be in cycle SCC")

	require.Len(t, findings, 1, "exactly one cycle finding expected")
	f := findings[0]
	assert.Equal(t, "cycle", f.Rule)
	assert.Equal(t, "error", f.Severity)
	assert.NotEmpty(t, f.ID, "finding must have stable fingerprint ID")
	assert.True(t, strings.HasPrefix(f.Message, "dependency cycle:"),
		"message must start with 'dependency cycle:' — got: %s", f.Message)
	assert.Len(t, f.Subjects, 3, "all cycle members must be subjects")
	assert.NotEmpty(t, f.Sources, "cycle finding must carry file:line sources (G7)")
	for _, src := range f.Sources {
		assert.NotEmpty(t, src.File, "source ref must have non-empty file (G7)")
	}
}

func TestDetectors_GodNode(t *testing.T) {
	// God node: m_hub has in=3 and out=3, both ≥ threshold(3).
	units := []UnitFact{
		{ID: "m_hub", Label: "Hub", Path: "internal/hub/hub.go", File: "internal/hub/hub.go", Line: 1},
		{ID: "m_caller1", Label: "Caller1", Path: "cmd/caller1.go", File: "cmd/caller1.go", Line: 1},
		{ID: "m_caller2", Label: "Caller2", Path: "cmd/caller2.go", File: "cmd/caller2.go", Line: 1},
		{ID: "m_caller3", Label: "Caller3", Path: "cmd/caller3.go", File: "cmd/caller3.go", Line: 1},
		{ID: "m_dep1", Label: "Dep1", Path: "internal/dep1/dep1.go", File: "internal/dep1/dep1.go", Line: 1},
		{ID: "m_dep2", Label: "Dep2", Path: "internal/dep2/dep2.go", File: "internal/dep2/dep2.go", Line: 1},
		{ID: "m_dep3", Label: "Dep3", Path: "internal/dep3/dep3.go", File: "internal/dep3/dep3.go", Line: 1},
	}
	deps := []DepFact{
		// 3 callers → hub
		{FromUnit: "m_caller1", ToUnit: "m_hub", Count: 1, File: "cmd/caller1.go", Line: 3},
		{FromUnit: "m_caller2", ToUnit: "m_hub", Count: 1, File: "cmd/caller2.go", Line: 3},
		{FromUnit: "m_caller3", ToUnit: "m_hub", Count: 1, File: "cmd/caller3.go", Line: 3},
		// hub → 3 deps
		{FromUnit: "m_hub", ToUnit: "m_dep1", Count: 1, File: "internal/hub/hub.go", Line: 5},
		{FromUnit: "m_hub", ToUnit: "m_dep2", Count: 1, File: "internal/hub/hub.go", Line: 6},
		{FromUnit: "m_hub", ToUnit: "m_dep3", Count: 1, File: "internal/hub/hub.go", Line: 7},
	}

	findings := DetectGods("test", units, deps, DefaultThresholds())

	require.Len(t, findings, 1, "exactly one god finding expected")
	f := findings[0]
	assert.Equal(t, "god", f.Rule)
	assert.Equal(t, "warn", f.Severity)
	assert.Equal(t, []string{"m_hub"}, f.Subjects)
	assert.True(t, strings.Contains(f.Message, "in 3"),
		"message must contain 'in 3' — got: %s", f.Message)
	assert.True(t, strings.Contains(f.Message, "out 3"),
		"message must contain 'out 3' — got: %s", f.Message)
	assert.NotEmpty(t, f.Sources, "god finding must carry file:line sources (G7)")
	for _, src := range f.Sources {
		assert.NotEmpty(t, src.File, "source ref must have non-empty file (G7)")
	}
}

func TestDetectors_Orphan(t *testing.T) {
	units := []UnitFact{
		{ID: "m_connected", Label: "Connected", Path: "internal/connected/c.go", File: "internal/connected/c.go", Line: 1},
		{ID: "m_partner", Label: "Partner", Path: "internal/partner/p.go", File: "internal/partner/p.go", Line: 1},
		{ID: "m_orphan", Label: "Orphan", Path: "util/orphan.go", File: "util/orphan.go", Line: 1},
	}
	deps := []DepFact{
		{FromUnit: "m_connected", ToUnit: "m_partner", Count: 1, File: "internal/connected/c.go", Line: 5},
	}

	findings := DetectOrphans("test", units, deps)

	require.Len(t, findings, 1, "exactly one orphan finding expected")
	f := findings[0]
	assert.Equal(t, "orphan", f.Rule)
	assert.Equal(t, "info", f.Severity)
	assert.Equal(t, []string{"m_orphan"}, f.Subjects)
	assert.True(t, strings.Contains(f.Message, "Orphan"),
		"message must mention unit label — got: %s", f.Message)
	assert.True(t, strings.Contains(f.Message, "no connections"),
		"message must say 'no connections' — got: %s", f.Message)
	assert.NotEmpty(t, f.Sources, "orphan finding must carry file:line sources (G7)")
	assert.Equal(t, "util/orphan.go", f.Sources[0].File)
}

func TestDetectors_AllFire_Fixture(t *testing.T) {
	// Verify that the full fixture (makeFixture) triggers all three detector types.
	in := makeFixture()

	var cycleFindings, godFindings, orphanFindings []Finding
	for _, f := range in.Findings {
		switch f.Rule {
		case "cycle":
			cycleFindings = append(cycleFindings, f)
		case "god":
			godFindings = append(godFindings, f)
		case "orphan":
			orphanFindings = append(orphanFindings, f)
		}
	}

	// Cycle: expect exactly one SCC (searcher/learner/enricher).
	require.Len(t, cycleFindings, 1, "fixture must produce exactly one cycle finding")
	cycleSubs := cycleFindings[0].Subjects
	sort.Strings(cycleSubs)
	assert.Equal(t,
		[]string{"m_domain_enricher", "m_domain_learner", "m_domain_searcher"},
		cycleSubs,
		"cycle SCC must contain exactly the three planted members")

	// God: expect exactly one (m_app).
	require.Len(t, godFindings, 1, "fixture must produce exactly one god finding")
	assert.Equal(t, []string{"m_app"}, godFindings[0].Subjects)
	assert.Contains(t, godFindings[0].Message, "in 3")
	assert.Contains(t, godFindings[0].Message, "out 5")

	// Orphan: expect exactly one (m_util_orphan).
	require.Len(t, orphanFindings, 1, "fixture must produce exactly one orphan finding")
	assert.Equal(t, []string{"m_util_orphan"}, orphanFindings[0].Subjects)
}

// ---------------------------------------------------------------------------
// T4 extension — Content hash stability: hash is deterministic across calls
// ---------------------------------------------------------------------------

func TestContentHashStability(t *testing.T) {
	in := makeFixture()

	s, err := RenderComponent(in)
	require.NoError(t, err)
	b1, err := MarshalShard(s)
	require.NoError(t, err)
	h1 := ContentHash(b1)

	// Second independent render and hash.
	s2, err := RenderComponent(in)
	require.NoError(t, err)
	b2, err := MarshalShard(s2)
	require.NoError(t, err)
	h2 := ContentHash(b2)

	assert.Equal(t, h1, h2, "content hash must be identical across renders")
	assert.Len(t, h1, 12, "content hash must be 12 hex chars")
}

// ---------------------------------------------------------------------------
// Grouping tests — path-prefix rung-2 heuristic
// ---------------------------------------------------------------------------

func TestGrouping_PathPrefix(t *testing.T) {
	cases := []struct {
		path  string
		label string
	}{
		{"internal/domain/arch/model.go", "domain"},
		{"internal/adapters/bbolt/store.go", "adapters"},
		{"internal/app/app.go", "app"},
		{"internal/ports/storage.go", "ports"},
		{"cmd/aoa/main.go", "cmd"},
		{"cmd/grep.go", "cmd"},
		{"util/orphan.go", "util"},
		{"src/lib/util.go", "lib"},      // src skipped as language root
		{"pkg/controller/ctrl.go", "controller"}, // pkg skipped
	}

	for _, tc := range cases {
		got := pathPrefixGroup(tc.path)
		assert.Equal(t, tc.label, got, "pathPrefixGroup(%q)", tc.path)
	}
}

func TestGrouping_Deterministic(t *testing.T) {
	units := makeFixture().Units

	g1 := Group(units)
	g2 := Group(units)

	// Group IDs and labels must match.
	require.Equal(t, len(g1.Groups), len(g2.Groups))
	for i := range g1.Groups {
		assert.Equal(t, g1.Groups[i], g2.Groups[i])
	}
	// UnitGroup mapping must match.
	for id := range g1.UnitGroup {
		assert.Equal(t, g1.UnitGroup[id], g2.UnitGroup[id], "group for unit %s", id)
	}
}

// ---------------------------------------------------------------------------
// Schema sanity — rendered shards decode into expected shapes
// ---------------------------------------------------------------------------

func TestShardSchema_Component(t *testing.T) {
	in := makeFixture()
	s, err := RenderComponent(in)
	require.NoError(t, err)
	b, err := MarshalShard(s)
	require.NoError(t, err)

	// Must decode into a map with expected fields.
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))

	assert.Equal(t, "buckets", m["kind"])
	assert.Equal(t, "DOWN", m["dir"])
	assert.NotNil(t, m["prov"])
	assert.NotNil(t, m["buckets"])
	assert.NotNil(t, m["count"])

	// All buckets must have id, label, part, members.
	buckets, ok := m["buckets"].([]interface{})
	require.True(t, ok)
	for i, bRaw := range buckets {
		bMap, ok2 := bRaw.(map[string]interface{})
		require.True(t, ok2, "bucket[%d] must be an object", i)
		assert.NotEmpty(t, bMap["id"], "bucket[%d].id", i)
		assert.NotEmpty(t, bMap["label"], "bucket[%d].label", i)
		// members must be a slice (possibly empty).
		_, hasMem := bMap["members"]
		assert.True(t, hasMem, "bucket[%d] must have members field", i)
	}
}

func TestShardSchema_DSM(t *testing.T) {
	in := makeFixture()
	s, err := RenderDSM(in)
	require.NoError(t, err)
	b, err := MarshalShard(s)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))

	assert.Equal(t, "matrix", m["kind"])
	items, ok := m["items"].([]interface{})
	require.True(t, ok)
	matrix, ok2 := m["matrix"].([]interface{})
	require.True(t, ok2)

	assert.Equal(t, len(items), len(matrix), "matrix must be square (n×n)")
	for i, row := range matrix {
		rowSlice, ok := row.([]interface{})
		require.True(t, ok, "matrix row %d must be a list", i)
		assert.Equal(t, len(items), len(rowSlice), "matrix row %d must have n columns", i)
	}
}

func TestShardSchema_Cycles(t *testing.T) {
	in := makeFixture()
	s, err := RenderCycles(in)
	require.NoError(t, err)
	b, err := MarshalShard(s)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))

	assert.Equal(t, "table", m["kind"])
	cols, ok := m["columns"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 4, len(cols), "cycles table must have 4 columns")

	rows, ok2 := m["rows"].([]interface{})
	require.True(t, ok2)
	// Fixture has exactly one cycle → one row.
	assert.Len(t, rows, 1, "fixture has exactly one planted cycle")

	// Row must start with ⚠
	firstRow, ok3 := rows[0].([]interface{})
	require.True(t, ok3)
	firstCell, ok4 := firstRow[0].(string)
	require.True(t, ok4)
	assert.True(t, strings.HasPrefix(firstCell, "⚠ "),
		"cycle row first cell must be ⚠-prefixed, got: %s", firstCell)
}

// ---------------------------------------------------------------------------
// G7 — every member/node carries file:line sources
// ---------------------------------------------------------------------------

func TestG7_SourcesPresent(t *testing.T) {
	in := makeFixture()
	s, err := RenderComponent(in)
	require.NoError(t, err)

	for _, bkt := range s.Buckets {
		for _, m := range bkt.Members {
			assert.NotEmpty(t, m.Sources,
				"member %s in bucket %s must have sources (G7)", m.ID, bkt.ID)
			assert.NotEmpty(t, m.Sources[0].File,
				"member %s sources[0].file must be non-empty (G7)", m.ID)
		}
	}
}

// ---------------------------------------------------------------------------
// Tarjan SCC unit tests
// ---------------------------------------------------------------------------

func TestTarjanSCC_NoSCC(t *testing.T) {
	adj := map[string][]string{
		"a": {"b"},
		"b": {"c"},
		"c": {},
	}
	sccs := TarjanSCC(adj)
	assert.Empty(t, sccs, "DAG should have no SCCs with len > 1")
}

func TestTarjanSCC_SelfLoop(t *testing.T) {
	// A self-loop is a trivial SCC of size 1 — our impl skips size-1 SCCs.
	adj := map[string][]string{
		"a": {"a"},
	}
	sccs := TarjanSCC(adj)
	assert.Empty(t, sccs, "self-loop is a trivial SCC; skip len==1")
}

func TestTarjanSCC_ThreeCycle(t *testing.T) {
	adj := map[string][]string{
		"x": {"y"},
		"y": {"z"},
		"z": {"x"},
	}
	sccs := TarjanSCC(adj)
	require.Len(t, sccs, 1)
	assert.Len(t, sccs[0], 3)
}

func TestTarjanSCC_TwoCycles(t *testing.T) {
	adj := map[string][]string{
		"a": {"b"},
		"b": {"a"}, // cycle 1
		"c": {"d"},
		"d": {"c"}, // cycle 2
		"e": {"a"}, // reaches cycle 1 but not part of it
	}
	sccs := TarjanSCC(adj)
	assert.Len(t, sccs, 2)
}

func TestTarjanSCC_Deterministic(t *testing.T) {
	adj := map[string][]string{
		"p": {"q"},
		"q": {"r"},
		"r": {"p"},
		"s": {"t"},
		"t": {"s"},
	}
	r1 := TarjanSCC(adj)
	r2 := TarjanSCC(adj)
	require.Equal(t, len(r1), len(r2), "same adj must produce same SCC count")
	for i := range r1 {
		assert.Equal(t, r1[i], r2[i], "SCC[%d] must be identical", i)
	}
}

// ---------------------------------------------------------------------------
// T5 extension — budget overflow, dead-code candidate, DSM mutual pair
// ---------------------------------------------------------------------------

// makeBudgetFixture returns a fixture with one group containing 42 members (>40).
func makeBudgetFixture() ([]UnitFact, GroupingResult) {
	// 42 units all in the "bigpkg" path-prefix group.
	units := make([]UnitFact, 42)
	for i := range units {
		id := fmt.Sprintf("m_big_%02d", i)
		units[i] = UnitFact{
			ID:    id,
			Label: fmt.Sprintf("big/%02d", i),
			Path:  fmt.Sprintf("bigpkg/file%02d.go", i),
			File:  fmt.Sprintf("bigpkg/file%02d.go", i),
			Line:  1,
		}
	}
	grouping := Group(units)
	return units, grouping
}

func TestDetectors_BudgetOverflow(t *testing.T) {
	units, grouping := makeBudgetFixture()
	// No dep edges needed — budget is purely membership count.
	findings := DetectBudgetOverflow("test", units, grouping)

	require.Len(t, findings, 1, "exactly one budget finding expected for the big group")
	f := findings[0]
	assert.Equal(t, "budget", f.Rule)
	assert.Equal(t, "warn", f.Severity)
	assert.NotEmpty(t, f.ID, "finding must have stable fingerprint ID")
	assert.Contains(t, f.Message, "42", "message must mention the member count")
	assert.Contains(t, f.Message, "budget overflow", "message prefix must be 'budget overflow'")
	assert.NotEmpty(t, f.Subjects, "budget finding must identify the overflowing group")
	assert.NotEmpty(t, f.Sources, "budget finding must carry file:line sources (G7)")
	for _, src := range f.Sources {
		assert.NotEmpty(t, src.File, "source ref must have non-empty file (G7)")
	}
}

func TestDetectors_BudgetOverflow_NoneBelow40(t *testing.T) {
	// Groups with exactly 40 members must NOT produce a finding.
	units := make([]UnitFact, 40)
	for i := range units {
		units[i] = UnitFact{
			ID:   fmt.Sprintf("m_ok_%02d", i),
			Label: fmt.Sprintf("ok/%02d", i),
			Path: fmt.Sprintf("okpkg/file%02d.go", i),
			File: fmt.Sprintf("okpkg/file%02d.go", i),
			Line: 1,
		}
	}
	grouping := Group(units)
	findings := DetectBudgetOverflow("test", units, grouping)
	assert.Empty(t, findings, "exactly 40 members must not trigger budget overflow (limit is >40)")
}

// makeDeadCodeFixture: one unit with outbound deps but zero inbound and zero ref hits.
func makeDeadCodeFixture() ([]UnitFact, []DepFact, map[string]int) {
	units := []UnitFact{
		{ID: "m_entry",   Label: "entry",   Path: "cmd/entry.go",     File: "cmd/entry.go",     Line: 1},
		{ID: "m_library", Label: "library", Path: "internal/lib/l.go", File: "internal/lib/l.go", Line: 1},
		// dead candidate: has outbound dep to library, but nothing imports it.
		{ID: "m_dead",    Label: "dead",    Path: "internal/dead/d.go", File: "internal/dead/d.go", Line: 1},
		{ID: "m_util",    Label: "util",    Path: "internal/util/u.go", File: "internal/util/u.go", Line: 1},
	}
	deps := []DepFact{
		{FromUnit: "m_entry",   ToUnit: "m_library", Count: 1, File: "cmd/entry.go",     Line: 5},
		{FromUnit: "m_dead",    ToUnit: "m_util",    Count: 1, File: "internal/dead/d.go", Line: 3},
		// m_util is imported by m_dead → has inbound dep → NOT a dead candidate.
		// m_library is imported by m_entry → has inbound dep → NOT a dead candidate.
		// m_dead: has outbound but zero inbound, and zero ref hits → dead candidate.
		// m_entry: zero inbound. We give it a ref hit so it's NOT a dead candidate.
	}
	// m_entry has ref hits (e.g. it's the main package, known by name).
	refHits := map[string]int{"m_entry": 5}
	return units, deps, refHits
}

func TestDetectors_DeadCodeCandidate(t *testing.T) {
	units, deps, refHits := makeDeadCodeFixture()

	findings := DetectDeadCandidates("test", units, deps, refHits)

	require.Len(t, findings, 1, "exactly one dead-code candidate expected")
	f := findings[0]
	assert.Equal(t, "dead-candidate", f.Rule)
	assert.Equal(t, "info", f.Severity)
	assert.Equal(t, []string{"m_dead"}, f.Subjects)
	assert.Contains(t, f.Message, "dead", "message must mention the dead label")
	assert.Contains(t, f.Message, "candidate", "message must say candidate (T5)")
	// PC3/T46: with measured (non-nil) fuel the message states the real count.
	assert.Contains(t, f.Message, "0 index references", "measured fuel must state 0 index references")
	assert.NotContains(t, f.Message, "not measured", "measured fuel must not carry the unmeasured caveat")
	assert.NotEmpty(t, f.Sources, "dead-candidate finding must carry file:line sources (G7)")
	assert.Equal(t, "internal/dead/d.go", f.Sources[0].File)
	// reflection caveat in attrs (WP step 5).
	require.NotNil(t, f.Attrs, "dead-candidate finding must carry attrs (reflection caveat)")
	assert.Contains(t, f.Attrs["caveat"], "reflection", "attrs.caveat must mention reflection")
}

func TestDetectors_DeadCodeCandidate_NilRefHits(t *testing.T) {
	// nil refHits means all units have 0 ref hits — units with 0 inbound fire.
	units := []UnitFact{
		{ID: "m_root",  Label: "root",  Path: "cmd/main.go",        File: "cmd/main.go",        Line: 1},
		{ID: "m_child", Label: "child", Path: "internal/child/c.go", File: "internal/child/c.go", Line: 1},
	}
	deps := []DepFact{
		{FromUnit: "m_root", ToUnit: "m_child", Count: 1, File: "cmd/main.go", Line: 2},
	}
	// With nil refHits: m_root has 0 inbound + 0 ref hits → candidate.
	// m_child has 1 inbound → not a candidate.
	findings := DetectDeadCandidates("test", units, deps, nil)
	require.Len(t, findings, 1, "m_root must be detected as dead-candidate with nil refHits")
	assert.Equal(t, []string{"m_root"}, findings[0].Subjects)
	// PC3/T46: nil fuel means references were NOT measured — the message must say
	// so and must NOT claim "no index references" (an unverified assertion, G7).
	assert.Contains(t, findings[0].Message, "not measured")
	assert.NotContains(t, findings[0].Message, "index references,")
}

// TestPC3_30kDeadCandidateCollapse gives the concrete before→after delta at 30k
// scale (ledger T46). NOTE: the T15 gen30kFixture has NO zero-inbound units —
// its ~30k findings are god-nodes, not dead-candidates — so this uses a purpose
// -built fixture of 30,000 leaf units that each import a shared core (zero
// inbound each). With nil fuel every leaf fires (the shipped dead-candidate
// noise); with real per-unit reference fuel only the genuinely-unreferenced
// leaves survive.
func TestPC3_30kDeadCandidateCollapse(t *testing.T) {
	const n = 30_000
	units := make([]UnitFact, 0, n+1)
	deps := make([]DepFact, 0, n)
	units = append(units, UnitFact{ID: "u_core", Label: "core", Path: "internal/core/c.go", File: "internal/core/c.go", Line: 1})
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("u_leaf%06d", i)
		f := fmt.Sprintf("internal/leaf%06d/l.go", i)
		units = append(units, UnitFact{ID: id, Label: fmt.Sprintf("leaf%d", i), Path: f, File: f, Line: 1})
		deps = append(deps, DepFact{FromUnit: id, ToUnit: "u_core", Count: 1, File: f, Line: 1})
	}

	// nil fuel (the defect): every leaf has zero inbound → all n fire as noise.
	before := DetectDeadCandidates("local", units, deps, nil)
	t.Logf("PC3 30k: nil fuel → %d dead-candidate findings (the noise)", len(before))
	require.Len(t, before, n, "nil fuel fires on every zero-inbound leaf (the 30k-noise defect)")

	// Real fuel: all but a handful of leaves carry index references.
	const genuinelyDead = 2
	refHits := map[string]int{"u_core": n}
	for i := genuinelyDead; i < n; i++ {
		refHits[fmt.Sprintf("u_leaf%06d", i)] = 1
	}
	after := DetectDeadCandidates("local", units, deps, refHits)
	t.Logf("PC3 30k: real fuel → %d dead-candidate findings (collapsed)", len(after))
	require.Len(t, after, genuinelyDead, "real fuel collapses 30k noise to only genuinely-unreferenced units")
}

// makeMutualPairFixture: two groups with deps in both directions (no unit-level cycle).
func makeMutualPairFixture() ([]UnitFact, []DepFact, GroupingResult) {
	units := []UnitFact{
		// "frontend" group
		{ID: "m_fe_view",       Label: "fe/view",       Path: "frontend/view.go",       File: "frontend/view.go",       Line: 1},
		{ID: "m_fe_controller", Label: "fe/controller", Path: "frontend/controller.go", File: "frontend/controller.go", Line: 1},
		// "backend" group
		{ID: "m_be_api",     Label: "be/api",     Path: "backend/api.go",     File: "backend/api.go",     Line: 1},
		{ID: "m_be_service", Label: "be/service", Path: "backend/service.go", File: "backend/service.go", Line: 1},
	}
	deps := []DepFact{
		// frontend → backend (controller uses API)
		{FromUnit: "m_fe_controller", ToUnit: "m_be_api", Count: 2, File: "frontend/controller.go", Line: 5},
		// backend → frontend (service uses view — creates group-level mutual dep)
		{FromUnit: "m_be_service", ToUnit: "m_fe_view", Count: 1, File: "backend/service.go", Line: 3},
		// NO unit-level cycle: controller→api, service→view — no path back.
	}
	grouping := Group(units)
	return units, deps, grouping
}

func TestDetectors_MutualPair(t *testing.T) {
	units, deps, grouping := makeMutualPairFixture()

	findings := DetectMutualPairs("test", units, deps, grouping)

	require.Len(t, findings, 1, "exactly one mutual-pair finding expected")
	f := findings[0]
	assert.Equal(t, "mutual", f.Rule)
	assert.Equal(t, "warn", f.Severity)
	assert.Len(t, f.Subjects, 2, "mutual finding must identify both groups")
	assert.Contains(t, f.Message, "mutual dependency", "message prefix must say 'mutual dependency'")
	assert.NotEmpty(t, f.ID, "finding must have stable fingerprint ID")
	assert.NotEmpty(t, f.Sources, "mutual finding must carry file:line sources (G7)")
	for _, src := range f.Sources {
		assert.NotEmpty(t, src.File, "source ref must have non-empty file (G7)")
	}
}

func TestDetectors_MutualPair_NoCycleAtUnitGrain(t *testing.T) {
	// The mutual-pair fixture must NOT produce unit-level cycle findings
	// (the two groups mutually depend at group grain, but there is no SCC at unit grain).
	units, deps, _ := makeMutualPairFixture()
	cycleFindings, sccs := DetectCycles("test", units, deps)
	assert.Empty(t, sccs, "mutual-pair fixture must have no unit-level SCCs")
	assert.Empty(t, cycleFindings, "mutual-pair fixture must have no cycle findings")
}

func TestDetectors_MutualPair_Deterministic(t *testing.T) {
	units, deps, grouping := makeMutualPairFixture()
	f1 := DetectMutualPairs("test", units, deps, grouping)
	f2 := DetectMutualPairs("test", units, deps, grouping)
	require.Equal(t, len(f1), len(f2), "mutual pair detection must be deterministic")
	if len(f1) > 0 {
		assert.Equal(t, f1[0].ID, f2[0].ID, "finding ID must be stable")
		assert.Equal(t, f1[0].Subjects, f2[0].Subjects, "subjects must be stable")
	}
}
