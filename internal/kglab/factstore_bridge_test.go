package kglab

// FDN-4 (board #30): TEST-only wiring proving a FactStore-shaped fixture
// (ports.Fact{Kind:unit} + ports.DepAdjacency, the FDN-1/FDN-3 substrate)
// converts cleanly into kglab.Compile's existing []arch.UnitFact/[]arch.DepFact
// input — WITHOUT kglab itself gaining any new dependency (D19: kglab stays
// self-contained; bridges belonging to a daemon boundary are internal/app's
// job, not kglab's).
//
// This intentionally does NOT import internal/app (that would put a daemon
// dependency between two package-graph LAYERS backwards — kglab is a lab
// package, internal/app is the app layer above it). Instead it re-implements
// the same minimal, namespace-collapsing conversion locally, at the scale
// this test needs (Go-only "go:<dir>" / "ext:<spec>" subjects) — the
// production conversion lives in internal/app/arch_factstore_bridge.go
// (unitFactsFromFactStore) and is unit-tested there against the full
// namespace set (py:/ts:/file: grains too).

import (
	"strings"
	"testing"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// factStoreFixture is a tiny hand-authored FactStore-shaped fixture: two Go
// units (app -> domain) plus one external target, mirroring the shape
// PutResolved/FactsByKind(FactUnit) would hand back for a real project.
func factStoreFixture() (units []ports.Fact, adj *ports.DepAdjacency) {
	units = []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:internal/app", Source: ports.FactSource{File: "internal/app/arch.go", Line: 4}},
		{Kind: ports.FactUnit, Subject: "go:internal/domain/arch", Source: ports.FactSource{File: "internal/domain/arch/model.go", Line: 1}},
	}
	adj = &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{
			"go:internal/app":          {{Unit: "go:internal/domain/arch", Count: 2}, {Unit: "ext:std/fmt", Count: 1}},
			"go:internal/domain/arch": {{Unit: "ext:std/sort", Count: 1}},
		},
	}
	return units, adj
}

// subjectToPath is the minimal local re-implementation of
// internal/app's factSubjectToUnitPath, scoped to what this test's fixture
// needs (go:/ext: only — see the package doc comment above).
func subjectToPath(subject string) string {
	if strings.HasPrefix(subject, "ext:") {
		return subject
	}
	return strings.TrimPrefix(subject, "go:")
}

// unitFactsFromFixture converts the FactStore-shaped fixture into
// []arch.UnitFact/[]arch.DepFact via arch.UnitSlug (the same exported ID
// mint internal/app's production bridge uses) — proving the shape is
// Compile-ready without duplicating the full production bridge.
func unitFactsFromFixture(units []ports.Fact, adj *ports.DepAdjacency) ([]arch.UnitFact, []arch.DepFact) {
	idOf := make(map[string]string)
	var outUnits []arch.UnitFact
	addUnit := func(subject, file string, line uint32) string {
		path := subjectToPath(subject)
		id := arch.UnitSlug(path)
		if _, ok := idOf[subject]; ok {
			return id
		}
		idOf[subject] = id
		outUnits = append(outUnits, arch.UnitFact{ID: id, Label: path, Path: path, File: file, Line: line})
		return id
	}
	for _, u := range units {
		addUnit(u.Subject, u.Source.File, u.Source.Line)
	}
	var outDeps []arch.DepFact
	for subj, edges := range adj.Fwd {
		fromID := addUnit(subj, "", 0)
		for _, e := range edges {
			toID := addUnit(e.Unit, "", 0)
			outDeps = append(outDeps, arch.DepFact{FromUnit: fromID, ToUnit: toID, Count: int(e.Count)})
		}
	}
	return outUnits, outDeps
}

// TestFactStoreBridge_CompileComponentView proves a FactStore-shaped fixture,
// once bridged, compiles into a valid component shard through the EXISTING,
// unmodified kglab.Compile — no daemon, no bbolt, no internal/app import.
func TestFactStoreBridge_CompileComponentView(t *testing.T) {
	units, deps := unitFactsFromFixture(factStoreFixture())
	require.Len(t, units, 4, "2 Go units + 2 lazily-minted ext units")
	require.Len(t, deps, 3)

	shard, err := Compile(ViewQuery{Scope: "factstore-bridge", Render: RenderSpec{Kind: "component"}}, units, deps)
	require.NoError(t, err)
	require.NotNil(t, shard)
	assert.Equal(t, "buckets", shard.Kind)

	var memberIDs []string
	for _, b := range shard.Buckets {
		for _, m := range b.Members {
			memberIDs = append(memberIDs, m.ID)
		}
	}
	assert.Contains(t, memberIDs, arch.UnitSlug("internal/app"))
	assert.Contains(t, memberIDs, arch.UnitSlug("internal/domain/arch"))
	assert.Contains(t, memberIDs, arch.UnitSlug("ext:std/fmt"))
}

// TestFactStoreBridge_MatchesDirectUnitFacts asserts the bridged fixture
// compiles to the SAME shard bytes as authoring the equivalent []arch.UnitFact/
// []arch.DepFact directly — the bridge introduces no distortion Compile can see.
func TestFactStoreBridge_MatchesDirectUnitFacts(t *testing.T) {
	bridgedUnits, bridgedDeps := unitFactsFromFixture(factStoreFixture())

	directUnits := []arch.UnitFact{
		{ID: arch.UnitSlug("internal/app"), Label: "internal/app", Path: "internal/app", File: "internal/app/arch.go", Line: 4},
		{ID: arch.UnitSlug("internal/domain/arch"), Label: "internal/domain/arch", Path: "internal/domain/arch", File: "internal/domain/arch/model.go", Line: 1},
		{ID: arch.UnitSlug("ext:std/fmt"), Label: "ext:std/fmt", Path: "ext:std/fmt"},
		{ID: arch.UnitSlug("ext:std/sort"), Label: "ext:std/sort", Path: "ext:std/sort"},
	}
	directDeps := []arch.DepFact{
		{FromUnit: arch.UnitSlug("internal/app"), ToUnit: arch.UnitSlug("internal/domain/arch"), Count: 2},
		{FromUnit: arch.UnitSlug("internal/app"), ToUnit: arch.UnitSlug("ext:std/fmt"), Count: 1},
		{FromUnit: arch.UnitSlug("internal/domain/arch"), ToUnit: arch.UnitSlug("ext:std/sort"), Count: 1},
	}

	bridgedShard, err := Compile(ViewQuery{Scope: "s", Render: RenderSpec{Kind: "component"}}, bridgedUnits, bridgedDeps)
	require.NoError(t, err)
	directShard, err := Compile(ViewQuery{Scope: "s", Render: RenderSpec{Kind: "component"}}, directUnits, directDeps)
	require.NoError(t, err)

	assert.Equal(t, len(directShard.Buckets), len(bridgedShard.Buckets))
	assert.Equal(t, directShard.Count, bridgedShard.Count)
}
