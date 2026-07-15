package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corey/aoa/atlas"
	"github.com/corey/aoa/internal/domain/enricher"
	"github.com/corey/aoa/internal/domain/index"
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
	assert.Nil(t, harvestGlossary(nil, nil))
}

func TestHarvestGlossary_RealEnricher_ProducesEntries(t *testing.T) {
	enr := newTestEnricherForVL1(t)
	// idx with no Tokens (fresh/unindexed project) falls back to the full,
	// unfiltered harvest rather than silently emptying the view.
	entries := harvestGlossary(enr, &ports.Index{})
	assert.NotEmpty(t, entries, "the embedded atlas has real domains/terms")
	for _, e := range entries {
		assert.NotEmpty(t, e.Term)
		assert.NotEmpty(t, e.Domain)
	}
}

func TestHarvestGlossary_FiltersByProjectTokens(t *testing.T) {
	enr := newTestEnricherForVL1(t)
	all := harvestGlossary(enr, &ports.Index{})
	require.NotEmpty(t, all, "precondition: the embedded atlas has real domains/terms")

	// "jwt" is a single, uniquely-owned keyword of authentication/token
	// (atlas/v1/01-auth-identity.json). One keyword alone is no longer
	// sufficient (VL-1.p1 punch: majority co-occurrence in the same code
	// file is required), so this must NOT survive on its own.
	idx := idxWithGoFileVL1(1, "jwt")
	filtered := harvestGlossary(enr, idx)
	assert.Empty(t, filtered, "a single, non-co-occurring keyword must not be enough to surface a term")
}

// TestHarvestGlossary_FiltersByProjectTokens_RealisticScale exercises
// HarvestFiltered against a project-sized token map (VL-1.p1 correctness
// punch: the prior test only proved the filter *could* shrink the set using a
// synthetic single-token index, never against real-world scale, and at real
// scale the "any one keyword present anywhere" gate was a near no-op).
//
// The fixture tokenizes real identifiers from this repo's own source files
// (internal/domain/index.Tokenize — the production tokenizer) across many
// distinct files, none of which co-occur multiple keywords of any single
// atlas term. One additional file is seeded with a real atlas term's
// keywords genuinely co-occurring together, as the one term that must
// survive. This proves the filter isn't defeated by scale (thousands of
// real tokens) and that co-occurrence — not raw presence — gates survival.
func TestHarvestGlossary_FiltersByProjectTokens_RealisticScale(t *testing.T) {
	enr := newTestEnricherForVL1(t)
	all := harvestGlossary(enr, &ports.Index{})
	require.NotEmpty(t, all, "precondition: the embedded atlas has real domains/terms")

	idx := realisticProjectIndexForVL1(t)
	require.Greater(t, len(idx.Tokens), 1000, "precondition: fixture must be project-sized, not a toy index")

	filtered := harvestGlossary(enr, idx)
	t.Logf("realistic-scale harvest: %d real tokens, %d files -> %d/%d terms survive (unfiltered baseline %d)",
		len(idx.Tokens), len(idx.Files), len(filtered), len(all), len(all))
	assert.Less(t, len(filtered), len(all)/5,
		"a realistic project-sized token map (%d real tokens across %d files) must shrink the ~%d-term atlas dump by an order of magnitude — got %d survivors, not the near no-op the punch described",
		len(idx.Tokens), len(idx.Files), len(all), len(filtered))

	found := false
	for _, e := range filtered {
		if e.Term == "token" && e.Domain == "authentication" {
			found = true
		}
	}
	assert.True(t, found, "the one term whose keywords genuinely co-occur in a single code file must survive")
}

// idxWithGoFileVL1 builds a minimal *ports.Index with one "go" file carrying
// the given tokens.
func idxWithGoFileVL1(fileID uint32, tokens ...string) *ports.Index {
	idx := &ports.Index{
		Files:  map[uint32]*ports.FileMeta{fileID: {Path: "f.go", Language: "go"}},
		Tokens: map[string][]ports.TokenRef{},
	}
	for _, tok := range tokens {
		idx.Tokens[tok] = append(idx.Tokens[tok], ports.TokenRef{FileID: fileID, Line: 1})
	}
	return idx
}

// realisticProjectIndexForVL1 tokenizes a sample of this repo's own real .go
// source files with the production tokenizer, assigning each file its own
// FileID, then adds one deliberately-seeded file whose tokens are a real
// atlas term's keywords ("token"/authentication: jwt, bearer, refresh,
// expiry, claims, decode, verify — atlas/v1/01-auth-identity.json) so
// exactly one term has genuine same-file co-occurrence.
func realisticProjectIndexForVL1(t *testing.T) *ports.Index {
	t.Helper()
	root, err := filepath.Abs("../..")
	require.NoError(t, err)

	var sourceFiles []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			sourceFiles = append(sourceFiles, path)
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, sourceFiles, "precondition: repo must contain .go source to tokenize")

	idx := &ports.Index{
		Files:  map[uint32]*ports.FileMeta{},
		Tokens: map[string][]ports.TokenRef{},
	}
	var fileID uint32
	for _, path := range sourceFiles {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		fileID++
		idx.Files[fileID] = &ports.FileMeta{Path: path, Language: "go"}
		for lineNo, line := range strings.Split(string(content), "\n") {
			for _, tok := range index.TokenizeContentLine(line) {
				idx.Tokens[tok] = append(idx.Tokens[tok], ports.TokenRef{FileID: fileID, Line: uint16(lineNo + 1)})
			}
		}
		if len(idx.Tokens) > 4000 {
			break // plenty for "project-sized"; keep the test fast
		}
	}

	fileID++
	authFile := fileID
	idx.Files[authFile] = &ports.FileMeta{Path: "auth/jwt.go", Language: "go"}
	for _, kw := range []string{"jwt", "bearer", "refresh", "expiry", "claims", "decode", "verify"} {
		idx.Tokens[kw] = append(idx.Tokens[kw], ports.TokenRef{FileID: authFile, Line: 1})
	}

	return idx
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
