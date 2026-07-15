package lockfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGoMod_SingleLineRequire(t *testing.T) {
	src := []byte(`module github.com/corey/aoa

go 1.26.4

require github.com/spf13/cobra v1.10.2
`)
	comps, err := ParseGoMod("go.mod", src)
	require.NoError(t, err)
	require.Len(t, comps, 1)
	c := comps[0]
	assert.Equal(t, "github.com/spf13/cobra", c.Name)
	assert.Equal(t, "v1.10.2", c.Version)
	assert.Equal(t, "direct", c.Supplier)
	assert.Equal(t, "go", c.Language)
	assert.False(t, c.Unpinned)
	assert.Equal(t, "go.mod", c.File)
	assert.Equal(t, uint32(5), c.Line)
}

func TestParseGoMod_BlockRequireDirectAndIndirect(t *testing.T) {
	src := []byte(`module example.com/foo

go 1.22

require (
	github.com/direct/pkg v1.2.3
	github.com/indirect/pkg v0.0.1 // indirect
)
`)
	comps, err := ParseGoMod("go.mod", src)
	require.NoError(t, err)
	require.Len(t, comps, 2)

	byName := map[string]Component{}
	for _, c := range comps {
		byName[c.Name] = c
	}
	direct := byName["github.com/direct/pkg"]
	assert.Equal(t, "direct", direct.Supplier)
	assert.Equal(t, "v1.2.3", direct.Version)

	indirect := byName["github.com/indirect/pkg"]
	assert.Equal(t, "indirect", indirect.Supplier)
	assert.Equal(t, "v0.0.1", indirect.Version)
}

func TestParseGoMod_ReplaceLocalPath_FlaggedUnpinned(t *testing.T) {
	src := []byte(`module example.com/foo

go 1.22

require github.com/some/pkg v1.0.0

replace github.com/some/pkg => ../local/pkg
`)
	comps, err := ParseGoMod("go.mod", src)
	require.NoError(t, err)
	require.Len(t, comps, 1)
	c := comps[0]
	assert.Equal(t, "github.com/some/pkg", c.Name)
	assert.Equal(t, "replace", c.Supplier)
	assert.True(t, c.Unpinned, "a local filesystem replace target has no resolvable pinned version")
}

func TestParseGoMod_ReplaceVersionedModule_UpdatesVersion(t *testing.T) {
	src := []byte(`module example.com/foo

go 1.22

require github.com/some/pkg v1.0.0

replace github.com/some/pkg => github.com/fork/pkg v1.0.1-fixed
`)
	comps, err := ParseGoMod("go.mod", src)
	require.NoError(t, err)
	require.Len(t, comps, 1)
	c := comps[0]
	assert.Equal(t, "github.com/fork/pkg", c.Name)
	assert.Equal(t, "v1.0.1-fixed", c.Version)
	assert.Equal(t, "replace", c.Supplier)
	assert.False(t, c.Unpinned)
}

func TestParseGoMod_EmptyModule_NoComponents(t *testing.T) {
	src := []byte("module example.com/foo\n\ngo 1.22\n")
	comps, err := ParseGoMod("go.mod", src)
	require.NoError(t, err)
	assert.Empty(t, comps)
}

func TestParseGoMod_Deterministic(t *testing.T) {
	src := []byte(`module example.com/foo

go 1.22

require (
	github.com/b/pkg v1.0.0
	github.com/a/pkg v2.0.0
)
`)
	c1, err := ParseGoMod("go.mod", src)
	require.NoError(t, err)
	c2, err := ParseGoMod("go.mod", src)
	require.NoError(t, err)
	assert.Equal(t, c1, c2)
}

func TestParseGoMod_RealFixture(t *testing.T) {
	// The project's own go.mod is a large, real-world fixture: many require
	// entries across two blocks (direct deps + go-sitter-forest grammars).
	comps, err := ParseGoMod("go.mod", []byte(realGoModFixture))
	require.NoError(t, err)
	assert.NotEmpty(t, comps)
	for _, c := range comps {
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.Version)
		assert.Equal(t, "go", c.Language)
	}
}

const realGoModFixture = `module github.com/corey/aoa

go 1.26.4

require (
	github.com/ebitengine/purego v0.9.1
	github.com/fsnotify/fsnotify v1.9.0
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.11.1
	github.com/tree-sitter/go-tree-sitter v0.25.0
	go.etcd.io/bbolt v1.4.3
)

require (
	github.com/alexaandru/go-sitter-forest/abap v1.9.0
	github.com/alexaandru/go-sitter-forest/abl v1.9.7 // indirect
)
`
