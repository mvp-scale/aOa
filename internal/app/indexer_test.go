package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildIndex_Counts(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	// Create two Go files with functions
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "a.go"),
		[]byte("package main\n\nfunc Alpha() {}\nfunc Beta() {}\n"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "b.go"),
		[]byte("package main\n\nfunc Gamma() {}\n"),
		0644,
	))

	idx, result, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	assert.Equal(t, 2, result.FileCount, "should index 2 files")
	assert.Equal(t, 3, result.SymbolCount, "should find 3 symbols")
	assert.Greater(t, result.TokenCount, 0, "should have tokens")
	assert.Equal(t, 2, len(idx.Files))
	assert.Equal(t, 3, len(idx.Metadata))
}

func TestBuildIndex_SkipsLargeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	// Create a >1MB Go file
	bigContent := make([]byte, 1<<20+100)
	copy(bigContent, []byte("package main\n\nfunc BigFunc() {}\n"))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "big.go"), bigContent, 0644))

	// Create a normal file
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "small.go"),
		[]byte("package main\n\nfunc SmallFunc() {}\n"),
		0644,
	))

	_, result, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	// Only the small file should be indexed
	assert.Equal(t, 1, result.FileCount, "big file should be skipped")
}

func TestBuildIndex_SkipsIgnoredDirs(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	// Create a file in node_modules
	nmDir := filepath.Join(tmpDir, "node_modules")
	require.NoError(t, os.MkdirAll(nmDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(nmDir, "dep.go"),
		[]byte("package dep\n\nfunc DepFunc() {}\n"),
		0644,
	))

	// Create a normal file at root
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "main.go"),
		[]byte("package main\n\nfunc Main() {}\n"),
		0644,
	))

	_, result, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	assert.Equal(t, 1, result.FileCount, "node_modules should be skipped")
	assert.Equal(t, 1, result.SymbolCount)
}
