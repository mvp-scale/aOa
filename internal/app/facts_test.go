//go:build !lean

// FDN-3 (board #29): app-layer wiring tests for the facts substrate
// compactor. Uses the real treesitter parser + a real bbolt store
// (newBurstTestApp/newWatcherTestApp pattern, see burst_test.go/t45) so
// these exercise the actual Reindex/WarmCaches/doFlushEdgeBatch code paths,
// not fakes.
package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReindex_PopulatesFactStore(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "server.go"), []byte(`package main

import "fmt"

func Serve() { fmt.Println("listening") }
`), 0644))

	dbPath := filepath.Join(t.TempDir(), "reindex.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	a := newBurstTestApp(t, tmpDir, store)
	a.ArchEnabled = true

	_, err = a.Reindex()
	require.NoError(t, err)
	a.bgWg.Wait() // let the background arch-derive goroutine finish before asserting/cleanup

	units, err := a.Store.FactsByKind(a.ProjectID, ports.FactUnit)
	require.NoError(t, err)
	require.Len(t, units, 1)
	assert.Equal(t, "go:", units[0].Subject, "root-package unit id for a single top-level file")

	deps, err := a.Store.Dependencies(a.ProjectID, "go:")
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "ext:std/fmt", deps[0].Unit)

	meta, err := a.Store.FactsMeta(a.ProjectID)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, "1", meta["schema_version"])
}

func TestDoFlushEdgeBatch_PopulatesFactStore(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "server.go")
	require.NoError(t, os.WriteFile(goFile, []byte(`package main

import "fmt"

func Serve() { fmt.Println("listening") }
`), 0644))

	dbPath := filepath.Join(t.TempDir(), "flush.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	a := newBurstTestApp(t, tmpDir, store)
	a.ArchEnabled = true

	// Seed the index via a full build first (no watcher event yet).
	_, err = a.Reindex()
	require.NoError(t, err)
	a.bgWg.Wait()

	// Now simulate a watcher edit: a new file importing a different stdlib
	// package. onFileChanged queues the raw edge batch; doFlushEdgeBatch
	// (called directly, bypassing the 200ms timer per its own doc comment)
	// must both keep the legacy edges bucket AND update the facts substrate.
	otherFile := filepath.Join(tmpDir, "other.go")
	require.NoError(t, os.WriteFile(otherFile, []byte(`package main

import "os"

func Other() { _ = os.Getenv("X") }
`), 0644))
	a.onFileChanged(otherFile)
	a.doFlushEdgeBatch()
	a.bgWg.Wait() // compactAndPersistFacts/deriveArch run in the "arch-derive" background goroutine

	deps, err := a.Store.Dependencies(a.ProjectID, "go:")
	require.NoError(t, err)
	units := make([]string, 0, len(deps))
	for _, d := range deps {
		units = append(units, d.Unit)
	}
	assert.Contains(t, units, "ext:std/fmt")
	assert.Contains(t, units, "ext:std/os", "the watcher-flush path must fold the new file's raw fact into the recompacted adjacency")
}

func TestHasFreshFacts(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	a := newBurstTestApp(t, tmpDir, store)
	assert.False(t, a.hasFreshFacts(), "never compacted -> not fresh")

	require.NoError(t, a.Store.PutResolved(a.ProjectID,
		[]ports.Fact{{Kind: ports.FactUnit, Subject: "go:", Prov: ports.ProvDerived}},
		&ports.DepAdjacency{}))
	assert.True(t, a.hasFreshFacts(), "PutResolved stamps the current schema_version")
}
