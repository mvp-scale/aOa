//go:build !lean

// Package app — T64 shard schema-version + boot re-derive (V1-A checkpoint PA2).
//
// Root cause (checkpoint-V1-A.md F-1/F-2 decisive live probe): PC1's boot-time
// derive trigger (WarmCaches, app.go) keys ONLY on manifest presence
// (hasLocalArchManifest). A restart on a NEWER binary whose shard/manifest
// JSON shape has changed leaves the OLD manifest sitting in bbolt — presence
// is true, so PC1 never fires, and the daemon serves stale pre-phase shards
// forever (until an unrelated file edit kicks the watcher path). This test
// proves the schema-version gate: a persisted manifest missing (or with an
// old) schemaVersion must trigger a fresh boot-arch-derive exactly like an
// absent manifest does; a manifest already at the current version must not
// trigger a spurious re-derive.
package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/domain/arch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestT64_SchemaVersionMismatch_TriggersReDerive: a manifest with NO
// schemaVersion field (simulating a pre-T64 binary's output) must be treated
// like "no current manifest" by PC1 — WarmCaches must fire boot-arch-derive
// and overwrite it with a freshly derived, correctly versioned manifest.
func TestT64_SchemaVersionMismatch_TriggersReDerive(t *testing.T) {
	tmpDir := t.TempDir()

	// Build a throwaway app/store first so its Index gets POPULATED by a real
	// WarmCaches pass (T43 path). Seeding a fresh, still-EMPTY Index directly
	// (skipping this step) makes the next boot take the "no persisted index"
	// branch, which calls ReplaceAllEdges and wipes any hand-seeded edges —
	// the same two-store dance T45 uses, for the same reason.
	seedDBPath := filepath.Join(t.TempDir(), "t64_mismatch_seed.db")
	seedStore, err := bbolt.NewStore(seedDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = seedStore.Close() })

	seedApp := newBurstTestApp(t, tmpDir, seedStore)
	seedApp.ArchEnabled = true
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644))
	seedApp.WarmCaches(func(string) {})
	seedApp.bgWg.Wait()

	// Fresh store: real (non-empty) index + synthetic edges + a hand-crafted
	// pre-T64 manifest (no schemaVersion field at all).
	dbPath := filepath.Join(t.TempDir(), "t64_mismatch.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(t, store.SaveIndex("test", seedApp.Index.Clone()))
	require.NoError(t, store.SaveEdgesForFile("test", 1, edgesForDeriveTest()))

	// A sentinel Rev proves whether a re-derive actually happened (a real
	// derive recomputes Rev from the real facts, which can never
	// coincidentally equal this sentinel).
	staleManifest := []byte(`{"scope":"local","rev":"deadbeefcafe","views":[{"id":"component","key":"local/component@deadbeefcafe","hash":"deadbeefcafe","caption":"stale","prov":"derived"}]}`)
	require.NoError(t, store.SaveManifest("test", "local", staleManifest))

	// Sanity: manifest present, no schemaVersion.
	var probe struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	require.NoError(t, json.Unmarshal(staleManifest, &probe))
	require.Zero(t, probe.SchemaVersion, "pre-condition: stale manifest must have no schemaVersion")
	require.True(t, store.HasEdgesBucket("test"), "pre-condition: edges bucket must exist before boot")

	// Boot: WarmCaches must detect the version mismatch and re-derive despite
	// the manifest being present.
	a2 := newBurstTestApp(t, tmpDir, store)
	a2.ArchEnabled = true
	a2.WarmCaches(func(string) {})
	a2.bgWg.Wait()

	got, err := store.LoadManifest("test", "local")
	require.NoError(t, err)
	require.NotEmpty(t, got)

	var m arch.Manifest
	require.NoError(t, json.Unmarshal(got, &m))
	assert.Equal(t, arch.ArchSchemaVersion, m.SchemaVersion,
		"T64: boot must overwrite a schema-version-mismatched manifest with a freshly derived, current one")
	assert.NotEqual(t, "deadbeefcafe", m.Rev,
		"T64: manifest Rev must have been recomputed by a real re-derive, not left as the stale sentinel")
}

// TestT64_CurrentSchemaVersion_NoSpuriousReDerive: a manifest already
// stamped with the CURRENT schema version must NOT be touched by boot — PC1
// stays a no-op, matching pre-T64 behavior for the common case.
func TestT64_CurrentSchemaVersion_NoSpuriousReDerive(t *testing.T) {
	tmpDir := t.TempDir()

	// Same two-store dance as TestT64_SchemaVersionMismatch_TriggersReDerive
	// (see its comment): populate a real index via one WarmCaches pass first.
	seedDBPath := filepath.Join(t.TempDir(), "t64_current_seed.db")
	seedStore, err := bbolt.NewStore(seedDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = seedStore.Close() })

	seedApp := newBurstTestApp(t, tmpDir, seedStore)
	seedApp.ArchEnabled = true
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644))
	seedApp.WarmCaches(func(string) {})
	seedApp.bgWg.Wait()

	dbPath := filepath.Join(t.TempDir(), "t64_current.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(t, store.SaveIndex("test", seedApp.Index.Clone()))
	require.NoError(t, store.SaveEdgesForFile("test", 1, edgesForDeriveTest()))

	// A manifest at the CURRENT schema version but with a sentinel Rev that
	// no real derive would ever produce — if boot leaves it alone, the
	// sentinel survives; if boot spuriously re-derives, it will be gone.
	require.Equal(t, 1, arch.ArchSchemaVersion, "test fixture assumes ArchSchemaVersion==1 — update the fixture if the constant changes")
	currentManifest := []byte(`{"scope":"local","rev":"sentinelnorederive","schemaVersion":1,"views":[{"id":"component","key":"local/component@sentinelnorederive","hash":"sentinelnorederive","caption":"current","prov":"derived"}]}`)
	require.NoError(t, store.SaveManifest("test", "local", currentManifest))

	a2 := newBurstTestApp(t, tmpDir, store)
	a2.ArchEnabled = true
	a2.WarmCaches(func(string) {})
	a2.bgWg.Wait()

	got, err := store.LoadManifest("test", "local")
	require.NoError(t, err)

	var m arch.Manifest
	require.NoError(t, json.Unmarshal(got, &m))
	assert.Equal(t, "sentinelnorederive", m.Rev,
		"T64: a manifest already at the current schema version must not be spuriously re-derived on boot")
}
