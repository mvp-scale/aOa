//go:build !lean

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApp_Reindex(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a Go file before creating the app
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "hello.go"),
		[]byte("package main\n\nfunc Hello() {}\n"),
		0644,
	))

	// Create a minimal App (no store for simplicity)
	a := newWatcherTestApp(t, tmpDir)

	// Initially empty index
	assert.Equal(t, 0, len(a.Index.Files))

	// Reindex should populate the index
	result, err := a.Reindex()
	require.NoError(t, err)

	assert.Equal(t, 1, result.FileCount)
	assert.Equal(t, 1, result.SymbolCount)
	assert.Greater(t, result.TokenCount, 0)
	assert.Greater(t, result.ElapsedMs, int64(-1)) // non-negative

	// Index should now have data
	assert.Equal(t, 1, len(a.Index.Files))
	assert.Greater(t, len(a.Index.Metadata), 0)

	// Search should find the symbol
	searchResult := a.Engine.Search("hello", ports.SearchOptions{})
	assert.GreaterOrEqual(t, len(searchResult.Hits), 1)
}

// TestReindex_BumpsRevision verifies L19.11: Reindex increments the global revision
// counter so ETag caches are invalidated on the same tick the index swaps.
func TestReindex_BumpsRevision(t *testing.T) {
	tmpDir := t.TempDir()

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "hello.go"),
		[]byte("package main\n\nfunc Hello() {}\n"),
		0644,
	))

	a := newWatcherTestApp(t, tmpDir)

	before := a.Revision()
	_, err := a.Reindex()
	require.NoError(t, err)

	assert.Greater(t, a.Revision(), before, "revision must increase after Reindex")
}
