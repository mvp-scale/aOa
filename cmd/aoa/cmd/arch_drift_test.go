package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/app"
	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GOV-1 (board #40): `aoa arch drift` — pure computation tests (no os.Exit,
// no daemon), mirroring PC3's arch_findings_test.go convention.

// writeDriftFixtureDB writes a real bbolt DB at root/.aoa/aoa.db with a tiny
// FactStore substrate: a -> b (real, kept in target) and a -> c (real,
// forbidden by target -> VIOLATION). d has no real edge -> MISSING.
func writeDriftFixtureDB(t *testing.T, root, projectID string) {
	t.Helper()
	dbPath := app.NewPaths(root).DB
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	units := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:a", Source: ports.FactSource{File: "a/x.go", Line: 1}},
		{Kind: ports.FactUnit, Subject: "go:b", Source: ports.FactSource{File: "b/x.go", Line: 1}},
	}
	adj := &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{
			"go:a": {{Unit: "go:b", Count: 1}, {Unit: "go:c", Count: 1}},
		},
	}
	require.NoError(t, store.PutResolved(projectID, units, adj))
}

func writeDriftEstateFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "target.aoa")
	src := `estate gov1-fixture
view component
allow a -> b
allow a -> d
`
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))
	return path
}

func TestDriftCompute_ViolationAndMissing(t *testing.T) {
	root := t.TempDir()
	writeDriftFixtureDB(t, root, filepath.Base(root))
	aoaPath := writeDriftEstateFile(t, root)

	resp, scope, err := driftCompute(root, aoaPath)
	require.NoError(t, err)
	assert.Equal(t, "drift:gov1-fixture", scope)
	assert.Equal(t, 1, resp.Result.Violations, "a -> c is real but not declared")
	assert.Equal(t, 1, resp.Result.Missing, "a -> d is declared but not built")
	assert.Equal(t, 1, resp.Result.Conformant, "a -> b is both real and declared")

	require.Len(t, resp.Findings, 1, "only the VIOLATION becomes a finding")
	f := resp.Findings[0]
	assert.Equal(t, "drift-violation", f.Rule)
	assert.Equal(t, []string{arch.UnitSlug("a"), arch.UnitSlug("c")}, f.Subjects)
}

func TestDriftCompute_ScopeOverrideFlag(t *testing.T) {
	root := t.TempDir()
	writeDriftFixtureDB(t, root, filepath.Base(root))
	aoaPath := writeDriftEstateFile(t, root)

	archDriftScope = "custom-scope"
	t.Cleanup(func() { archDriftScope = "" })

	_, scope, err := driftCompute(root, aoaPath)
	require.NoError(t, err)
	assert.Equal(t, "custom-scope", scope)
}

func TestDriftCompute_NoDB_ReturnsOperationalError(t *testing.T) {
	root := t.TempDir()
	aoaPath := writeDriftEstateFile(t, root)

	_, _, err := driftCompute(root, aoaPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no facts substrate")
}

func TestDriftCompute_BadAoaFile_ReturnsParseError(t *testing.T) {
	root := t.TempDir()
	writeDriftFixtureDB(t, root, filepath.Base(root))
	badPath := filepath.Join(root, "bad.aoa")
	require.NoError(t, os.WriteFile(badPath, []byte("nonsense line\n"), 0o644))

	_, _, err := driftCompute(root, badPath)
	require.Error(t, err)
}

// TestDriftCompute_Baseline_NewGate proves the drift verb's --new gate reuses
// the SAME baseline machinery `aoa arch findings --new` uses (writeBaseline /
// loadBaseline / newSinceBaseline, arch_findings.go) — one convention, two verbs.
func TestDriftCompute_Baseline_NewGate(t *testing.T) {
	root := t.TempDir()
	writeDriftFixtureDB(t, root, filepath.Base(root))
	aoaPath := writeDriftEstateFile(t, root)

	resp, scope, err := driftCompute(root, aoaPath)
	require.NoError(t, err)

	ids := []string{resp.Findings[0].ID}
	require.NoError(t, writeBaseline(root, scope, ids))

	base, have, err := loadBaseline(root, scope)
	require.NoError(t, err)
	require.True(t, have)
	assert.Empty(t, newSinceBaseline(base, ids), "the same violation re-diffed is not new")
}

