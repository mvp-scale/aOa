package bbolt

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// FDN-1 (board #27): FactStore — bbolt adapter for the facts substrate
// (playbook/integration/01-facts-substrate.md §3). Additive: does not touch
// EdgeStore (edges.go) or the scope-keyed Finding store (findings.go).
// =============================================================================

func TestFactStore_InterfaceSatisfied(t *testing.T) {
	var _ ports.FactStore = (*Store)(nil)
}

func TestReplaceFactsForFile_WriteThenRead(t *testing.T) {
	store, _ := newTestStore(t)

	facts := []ports.Fact{
		{
			Kind: ports.FactDep, Subject: "go:internal/app", Object: "",
			Attrs:  map[string]string{"spec": "github.com/corey/aoa/internal/ports"},
			Source: ports.FactSource{File: "internal/app/app.go", Line: 11, Commit: "abc123"},
			Prov:   ports.ProvDerived, TS: 1000,
		},
		{
			Kind: ports.FactDep, Subject: "go:internal/app", Object: "",
			Attrs:  map[string]string{"spec": "fmt"},
			Source: ports.FactSource{File: "internal/app/app.go", Line: 12, Commit: "abc123"},
			Prov:   ports.ProvDerived, TS: 1000,
		},
	}

	require.NoError(t, store.ReplaceFactsForFile("proj-1", "internal/app/app.go", facts))

	got, err := store.FactsForSubject("proj-1", "go:internal/app")
	require.NoError(t, err)
	assert.Len(t, got, 2)

	kinds, err := store.FactsByKind("proj-1", ports.FactDep)
	require.NoError(t, err)
	assert.Len(t, kinds, 2)
}

func TestReplaceFactsForFile_ReplacesPriorFacts(t *testing.T) {
	store, _ := newTestStore(t)

	first := []ports.Fact{
		{Kind: ports.FactDep, Subject: "go:internal/app", Source: ports.FactSource{File: "a.go", Line: 1}, Prov: ports.ProvDerived},
		{Kind: ports.FactDep, Subject: "go:internal/app", Source: ports.FactSource{File: "a.go", Line: 2}, Prov: ports.ProvDerived},
	}
	require.NoError(t, store.ReplaceFactsForFile("proj-1", "a.go", first))

	second := []ports.Fact{
		{Kind: ports.FactDep, Subject: "go:internal/app", Source: ports.FactSource{File: "a.go", Line: 5}, Prov: ports.ProvDerived},
	}
	require.NoError(t, store.ReplaceFactsForFile("proj-1", "a.go", second))

	got, err := store.FactsForSubject("proj-1", "go:internal/app")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, uint32(5), got[0].Source.Line)
}

func TestReplaceFactsForFile_EmptyIsPureDelete(t *testing.T) {
	store, _ := newTestStore(t)

	facts := []ports.Fact{
		{Kind: ports.FactDep, Subject: "go:internal/app", Source: ports.FactSource{File: "a.go", Line: 1}, Prov: ports.ProvDerived},
	}
	require.NoError(t, store.ReplaceFactsForFile("proj-1", "a.go", facts))
	require.NoError(t, store.ReplaceFactsForFile("proj-1", "a.go", nil))

	got, err := store.FactsForSubject("proj-1", "go:internal/app")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPutResolved_UnitsAndAdjacency(t *testing.T) {
	store, _ := newTestStore(t)

	units := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:internal/app", Prov: ports.ProvDerived},
		{Kind: ports.FactUnit, Subject: "go:internal/ports", Prov: ports.ProvDerived},
	}
	adj := &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{
			"go:internal/app": {{Unit: "go:internal/ports", Count: 3}},
		},
		Rev: map[string][]ports.DepEdge{
			"go:internal/ports": {{Unit: "go:internal/app", Count: 3}},
		},
	}
	require.NoError(t, store.PutResolved("proj-1", units, adj))

	got, err := store.FactsByKind("proj-1", ports.FactUnit)
	require.NoError(t, err)
	assert.Len(t, got, 2)

	deps, err := store.Dependencies("proj-1", "go:internal/app")
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "go:internal/ports", deps[0].Unit)
	assert.Equal(t, uint16(3), deps[0].Count)

	rev, err := store.Dependents("proj-1", "go:internal/ports")
	require.NoError(t, err)
	require.Len(t, rev, 1)
	assert.Equal(t, "go:internal/app", rev[0].Unit)
}

func TestPutResolved_OverwritesPriorResolved(t *testing.T) {
	store, _ := newTestStore(t)

	require.NoError(t, store.PutResolved("proj-1",
		[]ports.Fact{{Kind: ports.FactUnit, Subject: "go:a", Prov: ports.ProvDerived}},
		&ports.DepAdjacency{Fwd: map[string][]ports.DepEdge{"go:a": {{Unit: "go:b", Count: 1}}}},
	))
	require.NoError(t, store.PutResolved("proj-1",
		[]ports.Fact{{Kind: ports.FactUnit, Subject: "go:c", Prov: ports.ProvDerived}},
		&ports.DepAdjacency{Fwd: map[string][]ports.DepEdge{}},
	))

	units, err := store.FactsByKind("proj-1", ports.FactUnit)
	require.NoError(t, err)
	require.Len(t, units, 1)
	assert.Equal(t, "go:c", units[0].Subject)

	deps, err := store.Dependencies("proj-1", "go:a")
	require.NoError(t, err)
	assert.Empty(t, deps) // stale adjacency from the previous PutResolved is gone
}

func TestDependencies_MissingUnitReturnsNilNoError(t *testing.T) {
	store, _ := newTestStore(t)
	got, err := store.Dependencies("proj-1", "go:nowhere")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestBaseline_SaveLoadRoundTrip(t *testing.T) {
	store, _ := newTestStore(t)

	b := &ports.FactBaseline{
		Ref: "main", Commit: "deadbee", CreatedAt: 1234,
		Units:    []string{"go:a", "go:b"},
		Edges:    []ports.BaselineEdge{{S: "go:a", O: "go:b"}},
		Findings: []string{"cycle|go:a"},
	}
	require.NoError(t, store.SaveBaseline("proj-1", "release", b))

	got, err := store.LoadBaseline("proj-1", "release")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, b, got)
}

func TestBaseline_LoadMissingReturnsNilNoError(t *testing.T) {
	store, _ := newTestStore(t)
	got, err := store.LoadBaseline("proj-1", "nope")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestDeleteProjectFacts_RemovesAllFactBuckets(t *testing.T) {
	store, _ := newTestStore(t)

	require.NoError(t, store.ReplaceFactsForFile("proj-1", "a.go",
		[]ports.Fact{{Kind: ports.FactDep, Subject: "go:a", Source: ports.FactSource{File: "a.go", Line: 1}, Prov: ports.ProvDerived}}))
	require.NoError(t, store.PutResolved("proj-1",
		[]ports.Fact{{Kind: ports.FactUnit, Subject: "go:a", Prov: ports.ProvDerived}},
		&ports.DepAdjacency{Fwd: map[string][]ports.DepEdge{"go:a": {{Unit: "go:b", Count: 1}}}},
	))
	require.NoError(t, store.SaveBaseline("proj-1", "rel", &ports.FactBaseline{Ref: "main"}))

	require.NoError(t, store.DeleteProjectFacts("proj-1"))

	subj, err := store.FactsForSubject("proj-1", "go:a")
	require.NoError(t, err)
	assert.Empty(t, subj)

	units, err := store.FactsByKind("proj-1", ports.FactUnit)
	require.NoError(t, err)
	assert.Empty(t, units)

	deps, err := store.Dependencies("proj-1", "go:a")
	require.NoError(t, err)
	assert.Empty(t, deps)

	base, err := store.LoadBaseline("proj-1", "rel")
	require.NoError(t, err)
	assert.Nil(t, base)
}

func TestDeleteProjectFacts_IdempotentOnMissingProject(t *testing.T) {
	store, _ := newTestStore(t)
	require.NoError(t, store.DeleteProjectFacts("no-such-project"))
}

func TestDeleteProject_AlsoWipesFacts(t *testing.T) {
	store, _ := newTestStore(t)

	require.NoError(t, store.PutResolved("proj-1",
		[]ports.Fact{{Kind: ports.FactUnit, Subject: "go:a", Prov: ports.ProvDerived}}, nil))

	require.NoError(t, store.DeleteProject("proj-1"))

	units, err := store.FactsByKind("proj-1", ports.FactUnit)
	require.NoError(t, err)
	assert.Empty(t, units)
}

func TestFactStore_DoesNotCollideWithEdgeStoreOrFindings(t *testing.T) {
	// L19.9's facts_unresolved bucket ([]ports.ImportEdge) and L19.15's
	// facts_findings bucket ([]ports.Finding, scope-keyed) already existed
	// before FDN-1. The new fact-substrate unresolved/findings buckets must
	// use distinct names or these two features would silently corrupt each
	// other's data (see facts.go bucket-name comment).
	store, _ := newTestStore(t)

	edges := []ImportEdge{{FromFile: "a.go", ImportPath: "fmt", StartLine: 1}}
	require.NoError(t, store.SaveUnresolved("proj-1", edges))
	require.NoError(t, store.SaveFindings("proj-1", "scope-a", []ports.Finding{{Rule: "cycle"}}))

	// Old readers still see their own data untouched by the new FactStore.
	loadedEdges, err := store.LoadEdgesForFile("proj-1", 0)
	require.NoError(t, err)
	assert.Nil(t, loadedEdges) // no edges saved for fileID 0, just proving no panic/corruption

	findings, err := store.LoadFindings("proj-1", "scope-a")
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "cycle", findings[0].Rule)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkDependencies(b *testing.B) {
	store, _ := newTestStore(&testing.T{})

	// Set up a unit with 50 dependencies (warm case for real workload)
	units := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:main", Prov: ports.ProvDerived},
	}
	fwd := make(map[string][]ports.DepEdge)
	deps := make([]ports.DepEdge, 50)
	for i := 0; i < 50; i++ {
		deps[i] = ports.DepEdge{
			Unit:  "go:lib" + string(rune(65+i%26)) + string(rune(48+i/26)),
			Count: uint16(i + 1),
		}
	}
	fwd["go:main"] = deps

	require.NoError(&testing.T{}, store.PutResolved("proj-1", units, &ports.DepAdjacency{Fwd: fwd}))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := store.Dependencies("proj-1", "go:main")
		if err != nil {
			b.Fatal(err)
		}
	}
}
