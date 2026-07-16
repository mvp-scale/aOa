//go:build !lean

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEntityEntries_Go(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "user.go"),
		[]byte(`package model

type User struct {
	ID   int
	Name string
}
`),
		0644,
	))

	idx, _, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	entries := buildEntityEntries(tmpDir, idx, parser)
	require.Len(t, entries, 1)
	assert.Equal(t, "User", entries[0].Name)
	assert.Equal(t, []string{"ID", "Name"}, entries[0].Fields)
	assert.Equal(t, "Go struct", entries[0].Tech)
	assert.Equal(t, "user.go", entries[0].File)
	assert.Greater(t, entries[0].Line, uint32(0))
}

func TestBuildEntityEntries_NoGoFiles_ReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a.py"), []byte("x = 1\n"), 0644))

	idx, _, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	entries := buildEntityEntries(tmpDir, idx, parser)
	assert.Nil(t, entries)
}

func TestBuildEntityEntries_NilParser_ReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	idx, _, err := BuildIndex(tmpDir, nil)
	require.NoError(t, err)
	assert.Nil(t, buildEntityEntries(tmpDir, idx, nil))
}

func TestBuildEntityEntries_NilIndex_ReturnsNil(t *testing.T) {
	parser := treesitter.NewParser()
	assert.Nil(t, buildEntityEntries(t.TempDir(), nil, parser))
}

func TestBuildEntityEntries_SortedByName(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "types.go"),
		[]byte(`package model

type Zebra struct {
	A int
}

type Apple struct {
	B int
}
`),
		0644,
	))

	idx, _, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	entries := buildEntityEntries(tmpDir, idx, parser)
	require.Len(t, entries, 2)
	assert.Equal(t, "Apple", entries[0].Name)
	assert.Equal(t, "Zebra", entries[1].Name)
}
