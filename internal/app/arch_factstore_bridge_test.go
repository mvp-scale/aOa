//go:build !lean

package app

import (
	"testing"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/domain/facts"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// factSubjectToUnitPath — namespace collapse (FDN-4)
// =============================================================================

func TestFactSubjectToUnitPath(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		want    string
	}{
		{"go directory grain", "go:internal/app", "internal/app"},
		{"go root package", "go:", "root"},
		{"python file grain collapses to dir", "py:pkg/mod", "pkg"},
		{"python root-level file collapses to root", "py:mod", "root"},
		{"ts file grain collapses to dir", "ts:web/app/index", "web/app"},
		{"file fallback collapses to dir", "file:foo/bar.xyz", "foo"},
		{"ext passthrough unchanged", "ext:std/fmt", "ext:std/fmt"},
		{"ext bare passthrough", "ext:react", "ext:react"},
		{"empty subject is root", "", "root"},
		{"no namespace separator returned as-is", "weird", "weird"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, factSubjectToUnitPath(tc.subject))
		})
	}
}

// =============================================================================
// unitFactsFromFactStore — the FactStore → arch bridge (FDN-4)
// =============================================================================

func TestUnitFactsFromFactStore_BasicReKeying(t *testing.T) {
	units := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:internal/app", Source: ports.FactSource{File: "internal/app/arch.go", Line: 5}},
		{Kind: ports.FactUnit, Subject: "go:internal/domain/arch", Source: ports.FactSource{File: "internal/domain/arch/model.go", Line: 1}},
	}
	adj := &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{
			"go:internal/app": {{Unit: "go:internal/domain/arch", Count: 3}},
		},
	}

	outUnits, outDeps := unitFactsFromFactStore(units, adj, nil)

	require.Len(t, outUnits, 2)
	assert.Equal(t, arch.UnitSlug("internal/app"), outUnits[0].ID)
	assert.Equal(t, "internal/app", outUnits[0].Path)
	assert.Equal(t, arch.UnitSlug("internal/domain/arch"), outUnits[1].ID)

	require.Len(t, outDeps, 1)
	assert.Equal(t, arch.UnitSlug("internal/app"), outDeps[0].FromUnit)
	assert.Equal(t, arch.UnitSlug("internal/domain/arch"), outDeps[0].ToUnit)
	assert.Equal(t, 3, outDeps[0].Count)
	// DepFact.File/Line is the FROM unit's own representative source pointer
	// (documented divergence — the compacted adjacency has no per-edge line).
	assert.Equal(t, "internal/app/arch.go", outDeps[0].File)
	assert.Equal(t, uint32(5), outDeps[0].Line)
}

func TestUnitFactsFromFactStore_ExternalUnitsMintedFromEdges(t *testing.T) {
	// The compactor never stores an "ext:" unit fact (compactor.go ensureUnit
	// returns early for "ext:" IDs) — the bridge must mint the ext node
	// lazily from the edge target, matching aggregateEdges' behavior.
	units := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:internal/app", Source: ports.FactSource{File: "internal/app/arch.go", Line: 2}},
	}
	adj := &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{
			"go:internal/app": {{Unit: "ext:std/fmt", Count: 1}},
		},
	}

	outUnits, outDeps := unitFactsFromFactStore(units, adj, nil)

	require.Len(t, outUnits, 2, "ext:std/fmt must be minted as a unit even though no FactUnit fact carries it")
	var extUnit *arch.UnitFact
	for i := range outUnits {
		if outUnits[i].Path == "ext:std/fmt" {
			extUnit = &outUnits[i]
		}
	}
	require.NotNil(t, extUnit, "ext unit must be present")
	assert.Equal(t, arch.UnitSlug("ext:std/fmt"), extUnit.ID)
	assert.Empty(t, extUnit.Domain, "ext unit must not carry a domain")

	require.Len(t, outDeps, 1)
	assert.Equal(t, arch.UnitSlug("ext:std/fmt"), outDeps[0].ToUnit)
}

func TestUnitFactsFromFactStore_MultiFileUnitCollapse(t *testing.T) {
	// Two Python files in the same directory ("pkg/a.py", "pkg/b.py") mint
	// distinct FILE-grain subjects under the compactor's namespace
	// (py:pkg/a, py:pkg/b), but must collapse onto ONE arch unit (directory
	// grain) — and their outbound edge counts to the same target must sum.
	units := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "py:pkg/a", Source: ports.FactSource{File: "pkg/a.py", Line: 1}},
		{Kind: ports.FactUnit, Subject: "py:pkg/b", Source: ports.FactSource{File: "pkg/b.py", Line: 1}},
		{Kind: ports.FactUnit, Subject: "py:other/c", Source: ports.FactSource{File: "other/c.py", Line: 1}},
	}
	adj := &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{
			"py:pkg/a": {{Unit: "py:other/c", Count: 2}},
			"py:pkg/b": {{Unit: "py:other/c", Count: 5}},
		},
	}

	outUnits, outDeps := unitFactsFromFactStore(units, adj, nil)

	require.Len(t, outUnits, 2, "pkg/a.py and pkg/b.py must collapse onto one 'pkg' unit")
	require.Len(t, outDeps, 1, "both edges target the same collapsed unit — must merge into one DepFact")
	assert.Equal(t, arch.UnitSlug("pkg"), outDeps[0].FromUnit)
	assert.Equal(t, arch.UnitSlug("other"), outDeps[0].ToUnit)
	assert.Equal(t, 7, outDeps[0].Count, "counts from both collapsed source files must sum")
}

func TestUnitFactsFromFactStore_SelfLoopDropped(t *testing.T) {
	units := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:internal/app", Source: ports.FactSource{File: "internal/app/a.go", Line: 1}},
		{Kind: ports.FactUnit, Subject: "go:internal/app", Source: ports.FactSource{File: "internal/app/b.go", Line: 1}},
	}
	adj := &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{
			"go:internal/app": {{Unit: "go:internal/app", Count: 4}},
		},
	}
	outUnits, outDeps := unitFactsFromFactStore(units, adj, nil)
	require.Len(t, outUnits, 1)
	assert.Empty(t, outDeps, "same-directory self-loop must be dropped")
}

func TestUnitFactsFromFactStore_DomainFromIndex(t *testing.T) {
	units := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:internal/auth", Source: ports.FactSource{File: "internal/auth/login.go", Line: 3}},
	}
	idx := &ports.Index{
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "internal/auth/login.go", Domain: "authentication"},
		},
	}
	outUnits, _ := unitFactsFromFactStore(units, nil, idx)
	require.Len(t, outUnits, 1)
	assert.Equal(t, "authentication", outUnits[0].Domain)
}

func TestUnitFactsFromFactStore_Deterministic(t *testing.T) {
	units := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:pkg/c", Source: ports.FactSource{File: "pkg/c/c.go", Line: 1}},
		{Kind: ports.FactUnit, Subject: "go:pkg/a", Source: ports.FactSource{File: "pkg/a/a.go", Line: 1}},
		{Kind: ports.FactUnit, Subject: "go:pkg/b", Source: ports.FactSource{File: "pkg/b/b.go", Line: 1}},
	}
	adj := &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{
			"go:pkg/a": {{Unit: "go:pkg/b", Count: 1}},
			"go:pkg/c": {{Unit: "go:pkg/a", Count: 1}},
		},
	}
	u1, d1 := unitFactsFromFactStore(units, adj, nil)
	u2, d2 := unitFactsFromFactStore(units, adj, nil)
	require.Equal(t, u1, u2)
	require.Equal(t, d1, d2)
	// Units sorted by ID.
	for i := 1; i < len(u1); i++ {
		assert.Less(t, u1[i-1].ID, u1[i].ID)
	}
	for i := 1; i < len(d1); i++ {
		prevKey := d1[i-1].FromUnit + "\x00" + d1[i-1].ToUnit
		curKey := d1[i].FromUnit + "\x00" + d1[i].ToUnit
		assert.Less(t, prevKey, curKey)
	}
}

// =============================================================================
// Parity with aggregateEdges (the legacy EdgeStore path) on Go-only input.
// =============================================================================

// TestUnitFactsFromFactStore_ParityWithAggregateEdges_GoOnly drives the SAME
// raw (pre-resolution) import edges through both pipelines:
//
//  1. legacy: facts.Resolve → aggregateEdges (what deriveArch did before FDN-4)
//  2. new:    raw Facts → facts.CompactWithManifests → unitFactsFromFactStore
//     (what deriveArch does after FDN-4)
//
// and asserts they produce the same unit ID/Label/Domain set and the same
// (from, to, count) edge set. File/Line is intentionally NOT compared: the
// compacted adjacency (ports.DepEdge) has no per-edge source pointer (see
// unitFactsFromFactStore's doc comment) — this is a recorded, deliberate
// divergence, not a bug.
func TestUnitFactsFromFactStore_ParityWithAggregateEdges_GoOnly(t *testing.T) {
	rawEdges := []ports.ImportEdge{
		{FromFile: "internal/app/arch.go", ImportPath: "github.com/corey/aoa/internal/domain/arch", StartLine: 5},
		{FromFile: "internal/app/arch.go", ImportPath: "fmt", StartLine: 2},
		{FromFile: "internal/adapters/bbolt/store.go", ImportPath: "github.com/corey/aoa/internal/ports", StartLine: 1},
		{FromFile: "internal/adapters/bbolt/other.go", ImportPath: "github.com/corey/aoa/internal/ports", StartLine: 9},
	}
	fileSet := map[string]bool{
		"internal/app/arch.go":              true,
		"internal/domain/arch/model.go":     true,
		"internal/adapters/bbolt/store.go":  true,
		"internal/adapters/bbolt/other.go":  true,
		"internal/ports/storage.go":         true,
	}
	manifests := facts.Manifests{GoModules: map[string]string{"github.com/corey/aoa": ""}}

	// --- legacy path ---
	resolved := facts.Resolve(rawEdges, fileSet, manifests).Resolved
	require.Len(t, resolved, len(rawEdges), "all Go imports resolve (std or module-prefixed)")
	legacyUnits, legacyDeps := aggregateEdges(resolved, nil)

	// --- new path ---
	var rawFacts []ports.Fact
	for _, e := range rawEdges {
		subject := factSubjectForFile(e.FromFile, "go")
		rawFacts = append(rawFacts, importEdgeToFact(e, subject))
	}
	factUnits, adj, _ := facts.CompactWithManifests(rawFacts, fileSet, manifests)
	newUnits, newDeps := unitFactsFromFactStore(factUnits, adj, nil)

	// Compare unit ID sets (Label/Domain/Path — not File/Line, which is a
	// "first seen" heuristic on both sides but not guaranteed to pick the
	// same representative file when a unit has multiple owning facts).
	legacyByID := make(map[string]arch.UnitFact, len(legacyUnits))
	for _, u := range legacyUnits {
		legacyByID[u.ID] = u
	}
	newByID := make(map[string]arch.UnitFact, len(newUnits))
	for _, u := range newUnits {
		newByID[u.ID] = u
	}
	require.Equal(t, len(legacyByID), len(newByID), "same unit count")
	for id, lu := range legacyByID {
		nu, ok := newByID[id]
		require.True(t, ok, "unit %q present in legacy but missing from FactStore path", id)
		assert.Equal(t, lu.Label, nu.Label, "unit %q label", id)
		assert.Equal(t, lu.Path, nu.Path, "unit %q path", id)
		assert.Equal(t, lu.Domain, nu.Domain, "unit %q domain", id)
	}

	// Compare (from, to, count) edge sets.
	type edgeKey struct {
		from, to string
	}
	legacyEdges := make(map[edgeKey]int, len(legacyDeps))
	for _, d := range legacyDeps {
		legacyEdges[edgeKey{d.FromUnit, d.ToUnit}] = d.Count
	}
	newEdges := make(map[edgeKey]int, len(newDeps))
	for _, d := range newDeps {
		newEdges[edgeKey{d.FromUnit, d.ToUnit}] = d.Count
	}
	assert.Equal(t, legacyEdges, newEdges, "(from, to) -> count must match between the legacy and FactStore paths")
}
