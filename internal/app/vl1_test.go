package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/atlas"
	"github.com/corey/aoa/internal/domain/enricher"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverComponents_ReadsGoModAndPackageJSON(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(
		"module example.com/foo\n\ngo 1.22\n\nrequire github.com/spf13/cobra v1.10.2\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "package.json"), []byte(
		`{"dependencies":{"lodash":"4.17.21"}}`), 0o644))

	comps := discoverComponents(root)
	require.Len(t, comps, 2)

	byName := map[string]bool{}
	for _, c := range comps {
		byName[c.Name] = true
	}
	assert.True(t, byName["github.com/spf13/cobra"])
	assert.True(t, byName["lodash"])
}

func TestDiscoverComponents_MissingManifests_EmptyNotError(t *testing.T) {
	root := t.TempDir()
	comps := discoverComponents(root)
	assert.Empty(t, comps)
}

func TestBuildTechnologies_LanguageRowsFromIndex(t *testing.T) {
	idx := &ports.Index{Files: map[uint32]*ports.FileMeta{
		1: {Path: "a.go", Language: "go"},
		2: {Path: "b.go", Language: "go"},
		3: {Path: "c.py", Language: "python"},
	}}
	techs := buildTechnologies(idx, nil)
	require.Len(t, techs, 2)

	byName := map[string]int{}
	for _, te := range techs {
		byName[te.Name] = te.Count
		assert.Equal(t, "language", te.Kind)
	}
	assert.Equal(t, 2, byName["go"])
	assert.Equal(t, 1, byName["python"])
}

func TestBuildTechnologies_NilIndex_ComponentsOnlyStillWork(t *testing.T) {
	comps := discoverComponents(t.TempDir()) // empty
	techs := buildTechnologies(nil, comps)
	assert.Empty(t, techs)
}

func TestBuildTechnologies_DependencyRowsFromComponents(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(
		"module example.com/foo\n\ngo 1.22\n\nrequire github.com/some/pkg v1.0.0\n\nreplace github.com/some/pkg => ../local\n"), 0o644))
	comps := discoverComponents(root)
	require.Len(t, comps, 1)

	techs := buildTechnologies(nil, comps)
	require.Len(t, techs, 1)
	assert.Equal(t, "dependency", techs[0].Kind)
	assert.True(t, techs[0].Unpinned)
}

func TestHarvestGlossary_NilEnricher_ReturnsNil(t *testing.T) {
	assert.Nil(t, harvestGlossary(nil))
}

func TestHarvestGlossary_RealEnricher_ProducesEntries(t *testing.T) {
	enr := newTestEnricherForVL1(t)
	entries := harvestGlossary(enr)
	assert.NotEmpty(t, entries, "the embedded atlas has real domains/terms")
	for _, e := range entries {
		assert.NotEmpty(t, e.Term)
		assert.NotEmpty(t, e.Domain)
	}
}

func TestBuildVLInputs_Assembled(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte(
		"module example.com/foo\n\ngo 1.22\n\nrequire github.com/spf13/cobra v1.10.2\n"), 0o644))
	idx := &ports.Index{Files: map[uint32]*ports.FileMeta{1: {Path: "a.go", Language: "go"}}}
	enr := newTestEnricherForVL1(t)

	vlIn := buildVLInputs(root, idx, enr)
	require.NotNil(t, vlIn)
	assert.Len(t, vlIn.Components, 1)
	assert.NotEmpty(t, vlIn.Technologies)
	assert.NotEmpty(t, vlIn.GlossaryTerms)
}

func TestBuildVLInputs_NilIdxAndEnricher_HonestEmpty(t *testing.T) {
	vlIn := buildVLInputs(t.TempDir(), nil, nil)
	require.NotNil(t, vlIn)
	assert.Empty(t, vlIn.Components)
	assert.Empty(t, vlIn.Technologies)
	assert.Empty(t, vlIn.GlossaryTerms)
}

// newTestEnricherForVL1 loads the real embedded atlas — the same one the
// running app loads at startup (App.Enricher) — so glossary harvest tests
// exercise real data, not a synthetic stand-in.
func newTestEnricherForVL1(t *testing.T) *enricher.Enricher {
	t.Helper()
	enr, err := enricher.NewFromFS(atlas.FS, "v1")
	require.NoError(t, err)
	return enr
}
