//go:build !lean

// Package app — recon shard invalidation → boot re-derive (pre-recon v1).
//
// `aoa arch recon` invalidates derived views via DeleteShardsForScope, which
// deletes the shard KEYS and the manifest but leaves the arch_shards bucket
// (and its _version byte) in place. The T45/PC1 boot trigger keyed on
// !HasArchBucket therefore misses the post-invalidation state: edges present,
// bucket present, manifest absent. The boot trigger must key on "manifest
// absent for the local scope", which subsumes the bucket-absent case.
package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBootArchDerive_RefiresAfterShardInvalidation drives the exact state
// `aoa arch recon` leaves behind when the daemon is down (shards + manifest
// deleted, bucket surviving) and asserts the next boot re-derives.
func TestBootArchDerive_RefiresAfterShardInvalidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Build a real index from one Go file (same pattern as T45).
	goFile := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(goFile, []byte(`package main

import "fmt"

func Hello() { fmt.Println("hello") }
`), 0644))

	dbPath := filepath.Join(t.TempDir(), "recon_inval_seed.db")
	seedStore, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = seedStore.Close() })

	a := newBurstTestApp(t, tmpDir, seedStore)
	a.ArchEnabled = true
	a.WarmCaches(func(string) {})
	a.bgWg.Wait()

	// Second store: index + edges seeded, then derived once (bucket + manifest
	// present) — the state a live project is in before `aoa arch recon` runs.
	dbPath2 := filepath.Join(t.TempDir(), "recon_inval.db")
	store, err := bbolt.NewStore(dbPath2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(t, store.SaveIndex("test", a.Index.Clone()))
	require.NoError(t, store.SaveEdgesForFile("test", 1, edgesForDeriveTest()))
	// FDN-4: deriveArch (below) reads the FactStore query plane, not
	// LoadAllEdges — seed the equivalent compacted facts directly (this test
	// calls deriveArch() directly, bypassing WarmCaches' own compaction step).
	seedUnits, seedAdj := factsFromResolvedEdges(edgesForDeriveTest())
	require.NoError(t, store.PutResolved("test", seedUnits, seedAdj))

	aDerive := newBurstTestApp(t, tmpDir, store)
	aDerive.ArchEnabled = true
	aDerive.deriveArch()
	require.True(t, store.HasArchBucket("test"),
		"pre-condition: first derive must create arch_shards")

	// Recon invalidation: keys + manifest gone, bucket (and _version) survive.
	require.NoError(t, store.DeleteShardsForScope("test", "local"))
	m, err := store.LoadManifest("test", "local")
	require.NoError(t, err)
	require.Empty(t, m, "pre-condition: manifest absent after invalidation")
	require.True(t, store.HasArchBucket("test"),
		"pre-condition: arch_shards bucket survives DeleteShardsForScope")

	// Fresh boot: derive must re-fire even though the bucket exists.
	a2 := newBurstTestApp(t, tmpDir, store)
	a2.ArchEnabled = true
	a2.WarmCaches(func(string) {})
	a2.bgWg.Wait()

	manifestData, err := store.LoadManifest("test", "local")
	require.NoError(t, err)
	assert.NotEmpty(t, manifestData,
		"boot-arch-derive must re-derive when the local manifest is absent (post-recon invalidation)")
}
