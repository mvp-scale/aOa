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
	"strings"
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
	// FDN-3: main.go needs a real import (matching T45's fixture, not just
	// "package main\n"). WarmCaches now also fires boot-facts-recompact
	// whenever facts are stale (true in this test's fresh store2 below),
	// which does a real Reindex/re-parse of tmpDir before any direct
	// derive — an import-free fixture would make that re-parse find zero
	// edges, so deriveArch's len(edges)==0 guard would bail without
	// touching the manifest, leaving the stale sentinel in place and
	// masking what this test is actually about (schema-version handling).
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc Hello() { fmt.Println(\"hello\") }\n"), 0644))
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
	// Deliberately import-free (unlike TestT64_SchemaVersionMismatch_TriggersReDerive's
	// fixture): boot-facts-recompact still fires here too (facts are stale in
	// store2 below), but this test's whole point is that an ALREADY-current
	// manifest must survive boot untouched. A real import would let
	// boot-facts-recompact's Reindex legitimately re-derive arch from actual
	// disk content and overwrite the sentinel — correct behavior for THAT
	// scenario, but not what this test isolates. Zero on-disk edges makes
	// deriveArch's len(edges)==0 guard bail without touching the manifest,
	// keeping this test's assertion meaningful without conflating it with
	// boot-facts-recompact's separate (pre-existing, out-of-scope-here)
	// spurious-rederive-when-manifest-already-fresh gap.
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

// TestT64b_ViewSetIncomplete_TriggersReDerive: a manifest at the CURRENT
// schema version but missing one of arch.MandatoryViewIDs() (the real-world
// VP-2 gap — the caddy-lab checkpoint red-team: a daemon boots on a binary
// that adds a new mandatory view; the persisted manifest's SchemaVersion
// still matches because adding a view doesn't bump it, so the schema-version
// gate alone lets the stale, view-incomplete manifest survive every restart
// forever). Boot must detect the missing view and re-derive anyway.
func TestT64b_ViewSetIncomplete_TriggersReDerive(t *testing.T) {
	tmpDir := t.TempDir()

	seedDBPath := filepath.Join(t.TempDir(), "t64b_incomplete_seed.db")
	seedStore, err := bbolt.NewStore(seedDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = seedStore.Close() })

	seedApp := newBurstTestApp(t, tmpDir, seedStore)
	seedApp.ArchEnabled = true
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc Hello() { fmt.Println(\"hello\") }\n"), 0644))
	seedApp.WarmCaches(func(string) {})
	seedApp.bgWg.Wait()

	dbPath := filepath.Join(t.TempDir(), "t64b_incomplete.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(t, store.SaveIndex("test", seedApp.Index.Clone()))
	require.NoError(t, store.SaveEdgesForFile("test", 1, edgesForDeriveTest()))

	// Seed the FACTS substrate directly at the CURRENT FactsSchemaVersion —
	// unlike the T64 tests above, this isolates the completeness check from
	// FDN-3's boot-facts-recompact (which would otherwise fire unconditionally
	// on a fresh store with no facts_meta, forcing a full Reindex that would
	// mask what this test is actually about). With facts already fresh,
	// hasFreshFacts() is true, so FDN-3 stays silent and only the manifest's
	// view-completeness (not its schema version, already current) decides
	// whether PC1 fires.
	units, adj := factsFromResolvedEdges(edgesForDeriveTest())
	require.NoError(t, store.PutResolved("test", units, adj))

	// A manifest at the CURRENT schema version, but only carrying "component"
	// — missing every other view.MandatoryViewIDs entry (capability, context,
	// cycles, dsm as of this test). A sentinel Rev proves whether a re-derive
	// actually happened.
	require.Equal(t, 1, arch.ArchSchemaVersion, "test fixture assumes ArchSchemaVersion==1 — update the fixture if the constant changes")
	incompleteManifest := []byte(`{"scope":"local","rev":"deadbeefincomplete","schemaVersion":1,"views":[{"id":"component","key":"local/component@deadbeefincomplete","hash":"deadbeefincomplete","caption":"stale","prov":"derived"}]}`)
	require.NoError(t, store.SaveManifest("test", "local", incompleteManifest))

	// Sanity: every mandatory view except "component" is genuinely absent.
	require.Contains(t, arch.MandatoryViewIDs(), "component")
	require.Greater(t, len(arch.MandatoryViewIDs()), 1, "pre-condition: more than one mandatory view must exist for this test to mean anything")

	a2 := newBurstTestApp(t, tmpDir, store)
	a2.ArchEnabled = true
	a2.WarmCaches(func(string) {})
	a2.bgWg.Wait()

	got, err := store.LoadManifest("test", "local")
	require.NoError(t, err)
	require.NotEmpty(t, got)

	var m arch.Manifest
	require.NoError(t, json.Unmarshal(got, &m))
	assert.NotEqual(t, "deadbeefincomplete", m.Rev,
		"T64b: boot must overwrite a view-incomplete manifest with a freshly derived one, even when SchemaVersion already matches")

	gotIDs := make([]string, 0, len(m.Views))
	for _, v := range m.Views {
		gotIDs = append(gotIDs, v.ID)
	}
	for _, want := range arch.MandatoryViewIDs() {
		assert.Contains(t, gotIDs, want, "T64b: re-derived manifest must contain every mandatory view")
	}
}

// TestT64b_ViewSetComplete_NoSpuriousReDerive: a manifest already carrying
// every arch.MandatoryViewIDs() entry at the current schema version must not
// be touched by boot — the completeness check must not fire spuriously on
// the common (already-fresh) case.
func TestT64b_ViewSetComplete_NoSpuriousReDerive(t *testing.T) {
	tmpDir := t.TempDir()

	seedDBPath := filepath.Join(t.TempDir(), "t64b_complete_seed.db")
	seedStore, err := bbolt.NewStore(seedDBPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = seedStore.Close() })

	seedApp := newBurstTestApp(t, tmpDir, seedStore)
	seedApp.ArchEnabled = true
	// Deliberately import-free (mirrors TestT64_CurrentSchemaVersion_NoSpuriousReDerive):
	// zero on-disk edges makes deriveArch's len(edges)==0 guard bail without
	// touching the manifest, isolating this test's assertion from
	// boot-facts-recompact's separate Reindex path.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644))
	seedApp.WarmCaches(func(string) {})
	seedApp.bgWg.Wait()

	dbPath := filepath.Join(t.TempDir(), "t64b_complete.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(t, store.SaveIndex("test", seedApp.Index.Clone()))
	require.NoError(t, store.SaveEdgesForFile("test", 1, edgesForDeriveTest()))

	require.Equal(t, 1, arch.ArchSchemaVersion, "test fixture assumes ArchSchemaVersion==1 — update the fixture if the constant changes")
	views := make([]string, 0)
	for _, id := range arch.MandatoryViewIDs() {
		views = append(views, `{"id":"`+id+`","key":"local/`+id+`@sentinelnorederive","hash":"sentinelnorederive","caption":"current","prov":"derived"}`)
	}
	completeManifest := []byte(`{"scope":"local","rev":"sentinelnorederive","schemaVersion":1,"views":[` + strings.Join(views, ",") + `]}`)
	require.NoError(t, store.SaveManifest("test", "local", completeManifest))

	a2 := newBurstTestApp(t, tmpDir, store)
	a2.ArchEnabled = true
	a2.WarmCaches(func(string) {})
	a2.bgWg.Wait()

	got, err := store.LoadManifest("test", "local")
	require.NoError(t, err)

	var m arch.Manifest
	require.NoError(t, json.Unmarshal(got, &m))
	assert.Equal(t, "sentinelnorederive", m.Rev,
		"T64b: a manifest already carrying every mandatory view at the current schema version must not be spuriously re-derived on boot")
}
