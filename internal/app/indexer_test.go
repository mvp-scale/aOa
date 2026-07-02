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

// TestBuildIndexWithFacts_VarOnlyFileHasEdges is a regression test for the bug
// where `if parseErr == nil && len(metas) > 0` gated edge collection on non-empty
// metas. Go files with only var/const declarations produce 0 metas (extractGo
// handles only func/method/type), but they may still have import edges.
// Correct behaviour: edges are emitted regardless of metas length.
func TestBuildIndexWithFacts_VarOnlyFileHasEdges(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	// var-only Go file — no functions or types, so metas will be empty.
	// Two imports → expects 2 import edges.
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "consts.go"),
		[]byte("package main\n\nimport (\n\t\"os\"\n\t\"fmt\"\n)\n\nvar Debug = os.Getenv(\"DEBUG\") != \"\"\nvar Prefix = fmt.Sprintf(\"[app]\")\n"),
		0644,
	))

	_, result, edges, err := BuildIndexWithFacts(tmpDir, parser, true)
	require.NoError(t, err)

	assert.Equal(t, 1, result.FileCount)
	assert.Equal(t, 0, result.SymbolCount, "var-only file produces no symbol metas")
	assert.Equal(t, 2, result.EdgeCount, "import edges must be collected even when metas is empty")
	require.Len(t, edges, 2, "edges slice must match EdgeCount")

	paths := make([]string, len(edges))
	for i, e := range edges {
		paths[i] = e.ImportPath
	}
	assert.Contains(t, paths, "os")
	assert.Contains(t, paths, "fmt")
}

// TestBuildIndex_NilParser_TokenizationOnly verifies that BuildIndex works
// without a parser (tokenization-only mode). Files are discovered via
// defaultCodeExtensions, and content is tokenized for file-level search.
func TestBuildIndex_NilParser_TokenizationOnly(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Go files — should be discovered by defaultCodeExtensions
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "main.go"),
		[]byte("package main\n\nfunc SearchEngine() {\n\treturn\n}\n"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "util.py"),
		[]byte("def helper_function():\n    pass\n"),
		0644,
	))

	// nil parser = tokenization-only mode
	idx, result, err := BuildIndex(tmpDir, nil)
	require.NoError(t, err)

	// Files should be indexed
	assert.Equal(t, 2, result.FileCount, "should discover files via defaultCodeExtensions")

	// No symbols (no parser)
	assert.Equal(t, 0, result.SymbolCount, "no symbols without parser")
	assert.Equal(t, 0, len(idx.Metadata), "no metadata without parser")

	// But tokens should exist from content tokenization
	assert.Greater(t, result.TokenCount, 0, "should have tokens from content")

	// Check specific tokens exist — Tokenize("SearchEngine") → ["search", "engine"]
	assert.Greater(t, len(idx.Tokens["search"]), 0, "should tokenize 'search' from content")
	assert.Greater(t, len(idx.Tokens["engine"]), 0, "should tokenize 'engine' from content")
}
