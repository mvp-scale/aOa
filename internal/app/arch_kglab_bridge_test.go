package app

import (
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/kglab"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GOV-1 (board #40): RealDepFactsFromStore + DriftViolationFindings — the
// bridge between the real FactStore substrate and kglab's drift engine.

func writeFactStoreFixture(t *testing.T, projectID string) *bbolt.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "drift.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	units := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:internal/app", Source: ports.FactSource{File: "internal/app/arch.go", Line: 4}},
		{Kind: ports.FactUnit, Subject: "go:internal/domain/arch", Source: ports.FactSource{File: "internal/domain/arch/model.go", Line: 1}},
	}
	adj := &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{
			"go:internal/app": {
				{Unit: "go:internal/domain/arch", Count: 2},
				{Unit: "ext:std/fmt", Count: 1},
			},
		},
	}
	require.NoError(t, store.PutResolved(projectID, units, adj))
	return store
}

func TestRealDepFactsFromStore_MatchesFactStoreSubstrate(t *testing.T) {
	projectID := "proj"
	store := writeFactStoreFixture(t, projectID)

	deps, err := RealDepFactsFromStore(store, projectID, nil)
	require.NoError(t, err)
	require.Len(t, deps, 2, "app->domain/arch and app->ext:std/fmt")

	byTo := map[string]arch.DepFact{}
	for _, d := range deps {
		byTo[d.ToUnit] = d
	}
	appID := unitSlug("internal/app")
	domainID := unitSlug("internal/domain/arch")
	extID := unitSlug("ext:std/fmt")

	require.Contains(t, byTo, domainID)
	assert.Equal(t, appID, byTo[domainID].FromUnit)
	assert.Equal(t, 2, byTo[domainID].Count)
	require.Contains(t, byTo, extID)
	assert.Equal(t, 1, byTo[extID].Count)
}

func TestRealDepFactsFromStore_EmptyWhenNoUnits(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	deps, err := RealDepFactsFromStore(store, "proj", nil)
	require.NoError(t, err)
	assert.Empty(t, deps)
}

func TestRealDepFactsFromStore_MatchesDirectDeriveArchPath(t *testing.T) {
	// The bridge must not double-derive: calling it against a store populated
	// the same way deriveArch reads it (FactsByKind + Dependencies) must yield
	// byte-identical DepFacts to calling unitFactsFromFactStore directly.
	projectID := "proj"
	store := writeFactStoreFixture(t, projectID)

	unitFacts, err := store.FactsByKind(projectID, ports.FactUnit)
	require.NoError(t, err)
	fwd := make(map[string][]ports.DepEdge)
	for _, u := range unitFacts {
		edges, derr := store.Dependencies(projectID, u.Subject)
		require.NoError(t, derr)
		if len(edges) > 0 {
			fwd[u.Subject] = edges
		}
	}
	_, wantDeps := unitFactsFromFactStore(unitFacts, &ports.DepAdjacency{Fwd: fwd}, nil)

	gotDeps, err := RealDepFactsFromStore(store, projectID, nil)
	require.NoError(t, err)
	assert.Equal(t, wantDeps, gotDeps)
}

func sampleDriftResult() kglab.DriftResult {
	return kglab.DriftResult{
		RealName:   "real",
		TargetName: "target",
		Violations: 1,
		Missing:    1,
		Conformant: 1,
		Items: []kglab.DriftItem{
			{Alignment: kglab.AlignViolation, Fact: kglab.ConceptFact{
				Concept: "import", FromUnit: "u_a", ToUnit: "u_b", File: "a/a.go", Line: 12,
			}},
			{Alignment: kglab.AlignMissing, Fact: kglab.ConceptFact{
				Concept: "import", FromUnit: "u_c", ToUnit: "u_d",
			}},
			{Alignment: kglab.AlignConformant, Fact: kglab.ConceptFact{
				Concept: "import", FromUnit: "u_e", ToUnit: "u_f", File: "e/e.go", Line: 3,
			}},
		},
	}
}

func TestDriftViolationFindings_OnlyViolationsConvert(t *testing.T) {
	findings := DriftViolationFindings("local", sampleDriftResult())
	require.Len(t, findings, 1, "MISSING and CONFORMANT are not findings")

	f := findings[0]
	assert.Equal(t, "drift-violation", f.Rule)
	assert.Equal(t, "error", f.Severity)
	assert.Equal(t, "local", f.Scope)
	assert.Equal(t, []string{"u_a", "u_b"}, f.Subjects)
	require.Len(t, f.Sources, 1)
	assert.Equal(t, "a/a.go", f.Sources[0].File)
	assert.Equal(t, uint32(12), f.Sources[0].Line)
	assert.NotEmpty(t, f.ID)
}

func TestDriftViolationFindings_IDStableAndDeterministic(t *testing.T) {
	r := sampleDriftResult()
	f1 := DriftViolationFindings("local", r)
	f2 := DriftViolationFindings("local", r)
	require.Len(t, f1, 1)
	require.Len(t, f2, 1)
	assert.Equal(t, f1[0].ID, f2[0].ID, "same input -> same content-addressed ID")

	fOtherScope := DriftViolationFindings("other", r)
	assert.NotEqual(t, f1[0].ID, fOtherScope[0].ID, "scope is part of the ID (no cross-scope collision)")
}
