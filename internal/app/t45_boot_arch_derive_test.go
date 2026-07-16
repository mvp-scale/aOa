//go:build !lean

// Package app — T45 boot-time arch derive (PC1 / L19.18 punch list).
//
// Acceptance gate (verbatim from checkpoint-F2.md PC1):
//
//	"populated index + edges bucket present + NO arch_shards + flag on
//	→ boot/WarmCaches → poll → shards present, `arch views` answers —
//	with NO file edit and NO second init."
//
// Also covers the app-level overlay e2e (checkpoint finding 12):
//
//	"a real .aoa/arch/overlays/local.json with one invented ID →
//	deriveArch → findings contain overlay-leash warning + prov drops to mixed."
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

// TestT45_BootArchDerive verifies PC1: WarmCaches fires boot-arch-derive when
// the edges bucket is present but arch_shards are absent (canonical
// `init → daemon boot → arch views` scenario).
func TestT45_BootArchDerive(t *testing.T) {
	tmpDir := t.TempDir()

	// Seed a real bbolt store: index present + edges present + NO arch_shards.
	dbPath := filepath.Join(t.TempDir(), "t45.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	// Wire an App with the real store so WarmCaches triggers the PC1 path.
	a := newBurstTestApp(t, tmpDir, store)
	a.ArchEnabled = true

	// Seed the index so FileCount > 0 (WarmCaches checks this before triggering).
	// The simplest way: save an index that already has one file entry.
	// We do this by writing a Go file and calling WarmCaches itself (which will
	// build from scratch on first call, setting FileCount).
	goFile := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(goFile, []byte(`package main

import "fmt"

func Hello() { fmt.Println("hello") }
`), 0644))

	// First WarmCaches: builds the index from files (FileCount = 0 path → full
	// BuildIndexWithFacts). arch ON but no edges yet → T43 fires Reindex,
	// which also saves edges.  Wait for background goroutines.
	a.WarmCaches(func(string) {})
	a.bgWg.Wait()

	// Verify T43 did its job: edges bucket must exist now.
	assert.True(t, store.HasEdgesBucket("test"),
		"T45 pre-condition: T43 must have populated the edges bucket")

	// Drop arch_shards so we can simulate the canonical "init populated edges,
	// but daemon has never run a derive" state. We achieve this by saving fresh
	// edges and NOT calling deriveArch — the arch_shards bucket only exists
	// after deriveArch writes it. Verify the pre-condition holds.
	//
	// If T43's Reindex already derived shards (possible in a single run), we
	// still need the pre-condition to be false for the PC1 test to be meaningful.
	// In practice Reindex derives shards too; to isolate the PC1 trigger we
	// verify HasEdgesBucket is true and HasArchBucket may or may not be true at
	// this point. The important invariant is: WHEN arch_shards are absent, PC1
	// must derive them. We test that by constructing the exact DB state.
	dbPath2 := filepath.Join(t.TempDir(), "t45_pc1.db")
	store2, err := bbolt.NewStore(dbPath2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store2.Close() })

	// Build an index manually (identical to T43 pattern).
	require.NoError(t, store2.SaveIndex("test", a.Index.Clone()))

	// Populate edges via SaveEdgesForFile (mimics `aoa init` saving resolved edges).
	// Use fileID=1 — LoadAllEdges iterates all 4-byte keys, so this is found.
	require.NoError(t, store2.SaveEdgesForFile("test", 1, edgesForDeriveTest()))

	// Pre-conditions: edges present, shards absent.
	assert.True(t, store2.HasEdgesBucket("test"),
		"T45 pre-condition: edges bucket must exist before boot")
	assert.False(t, store2.HasArchBucket("test"),
		"T45 pre-condition: arch_shards must NOT exist before boot (PC1 scenario)")

	// Boot: WarmCaches detects populated index + edges + no arch_shards + arch ON
	// → fires safeGo("boot-arch-derive") → deriveArch() → SaveShards + SaveManifest.
	a2 := newBurstTestApp(t, tmpDir, store2)
	a2.ArchEnabled = true
	a2.WarmCaches(func(string) {})
	a2.bgWg.Wait()

	// Gate: arch_shards bucket must now exist (PC1 trigger fired and derived).
	assert.True(t, store2.HasArchBucket("test"),
		"T45: arch_shards must exist after boot-arch-derive (PC1)")

	// Bonus: manifest must exist for scope "local" so `arch views` can serve it.
	manifestData, err := store2.LoadManifest("test", "local")
	require.NoError(t, err)
	assert.NotEmpty(t, manifestData,
		"T45: manifest must be persisted after boot-arch-derive")
}

// TestT45_BootArchDerive_ArchOff verifies the C4 gate: when ArchEnabled is false,
// boot-arch-derive does NOT fire even when edges exist but shards are absent.
func TestT45_BootArchDerive_ArchOff(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(t.TempDir(), "t45_off.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	// Seed index + edges (same pre-condition as the arch-on test).
	a := newBurstTestApp(t, tmpDir, store)
	a.ArchEnabled = true // temporarily ON to build index with edges
	goFile := filepath.Join(tmpDir, "off_main.go")
	require.NoError(t, os.WriteFile(goFile, []byte("package main\nimport \"fmt\"\nfunc Hi() { fmt.Println(\"hi\") }\n"), 0644))
	a.WarmCaches(func(string) {})
	a.bgWg.Wait()

	// Drain derived shards so we can test the PC1 gate specifically.
	// Use a second fresh store with just the index + edges.
	dbPath2 := filepath.Join(t.TempDir(), "t45_off2.db")
	store2, err := bbolt.NewStore(dbPath2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store2.Close() })

	require.NoError(t, store2.SaveIndex("test", a.Index.Clone()))
	require.NoError(t, store2.SaveEdgesForFile("test", 1, edgesForDeriveTest()))
	assert.True(t, store2.HasEdgesBucket("test"),
		"T45/off pre-condition: edges must be present")
	assert.False(t, store2.HasArchBucket("test"),
		"T45/off pre-condition: shards must be absent")

	// Boot with arch OFF: PC1 trigger must NOT fire (C4 law).
	a2 := newBurstTestApp(t, tmpDir, store2)
	a2.ArchEnabled = false
	a2.WarmCaches(func(string) {})
	a2.bgWg.Wait()

	assert.False(t, store2.HasArchBucket("test"),
		"T45/off: arch_shards must NOT be created when ArchEnabled=false (C4)")
}

// TestT45_OverlayLeash_AppLevel is the app-level overlay e2e (checkpoint finding 12).
// Drives an overlay with an invented unit ID through deriveArch (not domain-level)
// and asserts that the findings contain an overlay-leash warning and the manifest
// view provenance drops to "mixed".
func TestT45_OverlayLeash_AppLevel(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(t.TempDir(), "t45_overlay.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	// Populate edges so deriveArch proceeds past the early-return guard.
	require.NoError(t, store.SaveEdgesForFile("test", 1, edgesForDeriveTest()))
	// FDN-4: deriveArch (below) is called directly, bypassing WarmCaches' own
	// compaction step — seed the equivalent compacted facts directly so the
	// FactStore query plane has something to derive from.
	seedUnits, seedAdj := factsFromResolvedEdges(edgesForDeriveTest())
	require.NoError(t, store.PutResolved("test", seedUnits, seedAdj))

	// Write a real .aoa/arch/overlays/local.json with one invented unit ID.
	// "u_invented_nonexistent" cannot map to any unit derived from edgesForDeriveTest.
	overlayDir := filepath.Join(tmpDir, ".aoa", "arch", "overlays")
	require.NoError(t, os.MkdirAll(overlayDir, 0755))
	overlayJSON := []byte(`{
	"$schema": "aoa.arch-overlay/v1",
	"groups": [
		{
			"id": "phantom",
			"label": "Phantom Group",
			"unitIds": ["u_invented_nonexistent"]
		}
	]
}`)
	require.NoError(t, os.WriteFile(filepath.Join(overlayDir, "local.json"), overlayJSON, 0644))

	// Wire App: ProjectRoot points to tmpDir so loadOverlaySpec finds the file.
	a := newBurstTestApp(t, tmpDir, store)
	a.ArchEnabled = true
	a.stopCh = make(chan struct{})

	// Call deriveArch directly (outside mu, per C1 — same as safeGo path).
	a.deriveArch()

	// Assert 1: findings must contain an overlay-leash warning.
	findings, err := store.LoadFindings("test", "local")
	require.NoError(t, err)
	var leashFound bool
	for _, f := range findings {
		if f.Rule == "overlay-leash" {
			leashFound = true
			break
		}
	}
	assert.True(t, leashFound,
		"T45/overlay: findings must contain an overlay-leash warning for invented unit ID")

	// Assert 2: manifest views must have prov = "mixed" (overlay attempted).
	manifestData, err := store.LoadManifest("test", "local")
	require.NoError(t, err)
	require.NotEmpty(t, manifestData,
		"T45/overlay: manifest must be persisted after deriveArch with overlay")

	var manifest arch.Manifest
	require.NoError(t, json.Unmarshal(manifestData, &manifest),
		"T45/overlay: manifest must be valid JSON")
	require.NotEmpty(t, manifest.Views,
		"T45/overlay: manifest must have at least one view")
	for _, ve := range manifest.Views {
		// api-contract (VL-3, board #37) is unrelated to unit grouping/
		// overlays entirely — its facts come straight from an AST call-site
		// scan of route registrations, never from GroupWithOptions. Forcing
		// it to "mixed" here would be a dishonest signal (D2: provenance
		// reflects THAT view's own derivation confidence, not an unrelated
		// subsystem's). Every other mandatory view either derives its Prov
		// from in.GroupProv (component/dsm/cycles) or is unconditionally
		// "mixed" already (sbom/glossary/change/techportfolio/context/
		// capability) — api-contract is the first unconditionally
		// "derived" view, which is what this exception documents.
		//
		// datamodel (COL-1) is the same class as api-contract: its facts
		// come straight from an AST struct-field scan, never from
		// GroupWithOptions, so it is unconditionally "derived" too.
		//
		// deployment (COL-2, board M6) is the same class again: its facts
		// come straight from Dockerfile/compose.yaml/Kubernetes-manifest
		// reads, never from GroupWithOptions, so it is unconditionally
		// "derived" too.
		if ve.ID == "api-contract" || ve.ID == "datamodel" || ve.ID == "deployment" {
			assert.Equal(t, "derived", ve.Prov,
				"T45/overlay: "+ve.ID+" prov is independent of the overlay leash")
			continue
		}
		assert.Equal(t, "mixed", ve.Prov,
			"T45/overlay: view %q prov must be 'mixed' when overlay had invalid IDs", ve.ID)
	}
}
