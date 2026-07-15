//go:build !lean

// Package app — FDN-3 boot-trigger mutual exclusion (regression-and-blast-radius
// punch).
//
// WarmCaches has two boot-time derive triggers gated on HasEdgesBucket: PC1
// ("boot-arch-derive", fires when the local arch manifest is absent) and
// FDN-3's "boot-facts-recompact" (fires when the facts substrate is stale).
// Before this fix, the two conditions were independent, so a project with an
// edges bucket predating BOTH the arch feature and the facts substrate (the
// exact state any pre-existing DB is in on first boot with this binary) fired
// BOTH goroutines: PC1's direct a.deriveArch() AND boot-facts-recompact's
// a.Reindex(), which itself re-derives arch internally (Reindex's own
// "arch-derive" safeGo). That is a full duplicate derive plus an extra full
// Reindex parse pass on the very boot this feature ships to.
//
// This test proves the two triggers are now mutually exclusive by counting
// SaveManifest calls (every completed deriveArch — direct or via Reindex —
// writes exactly one manifest per scope, unconditionally, per deriveArch's
// own doc comment: "last completed run always reflects the newest data").
// Exactly one derive must reach SaveManifest, not two.
package app

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// manifestCountingStore wraps a real *bbolt.Store and counts SaveManifest
// calls, so the test can observe how many independent derive passes actually
// completed without reaching into WarmCaches' private goroutine names.
type manifestCountingStore struct {
	storeBackend
	saveManifestCalls atomic.Int64
}

func (m *manifestCountingStore) SaveManifest(projectID, scope string, data []byte) error {
	m.saveManifestCalls.Add(1)
	return m.storeBackend.SaveManifest(projectID, scope, data)
}

// TestFDN3_BootTriggers_MutuallyExclusive_UpgradeState drives the exact
// "pre-facts, pre-arch-manifest" upgrade-boot state (edges bucket present,
// local arch manifest absent, facts substrate stale/never populated) and
// asserts WarmCaches fires only ONE derive pass to completion, not two.
func TestFDN3_BootTriggers_MutuallyExclusive_UpgradeState(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(goFile, []byte(`package main

import "fmt"

func Hello() { fmt.Println("hello") }
`), 0644))

	dbPath := filepath.Join(t.TempDir(), "fdn3_upgrade.db")
	rawStore, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rawStore.Close() })

	// Build an index the same way the other boot tests do (T45 pattern: run
	// WarmCaches once so seed.Index is actually populated — cloning an
	// unwarmed App's Index would be empty, which would make the SECOND
	// App's r.FileCount==0 and take the from-scratch build branch instead
	// of the boot-trigger branches under test), then seed the upgrade-state
	// store: edges present (via SaveEdgesForFile, mimicking a
	// pre-facts-substrate `aoa init`), but NEITHER an arch manifest NOR a
	// facts compaction — the state any existing project's DB is in on first
	// boot with a binary that has both features.
	seed := newBurstTestApp(t, tmpDir, rawStore)
	seed.ArchEnabled = true
	seed.WarmCaches(func(string) {})
	seed.bgWg.Wait()

	dbPath2 := filepath.Join(t.TempDir(), "fdn3_upgrade2.db")
	plainStore, err := bbolt.NewStore(dbPath2)
	require.NoError(t, err)
	t.Cleanup(func() { _ = plainStore.Close() })
	require.NoError(t, plainStore.SaveIndex("test", seed.Index.Clone()))
	require.NoError(t, plainStore.SaveEdgesForFile("test", 1, edgesForDeriveTest()))

	counting := &manifestCountingStore{storeBackend: plainStore}
	a := newBurstTestApp(t, tmpDir, counting)
	a.ArchEnabled = true

	// Pre-conditions matching the punch's repro exactly.
	require.True(t, counting.HasEdgesBucket("test"), "pre-condition: edges bucket present")
	require.False(t, a.hasLocalArchManifest(), "pre-condition: arch manifest absent")
	require.False(t, a.hasFreshFacts(), "pre-condition: facts substrate stale/never compacted")

	a.WarmCaches(func(string) {})
	a.bgWg.Wait()

	assert.Equal(t, int64(1), counting.saveManifestCalls.Load(),
		"exactly one derive pass (boot-facts-recompact's Reindex, which re-derives arch internally) "+
			"must reach SaveManifest — PC1's boot-arch-derive must not ALSO fire a redundant direct derive "+
			"when facts are stale")

	// Functional correctness is preserved: the single derive pass still
	// leaves the project fully warmed (manifest + fresh facts present).
	assert.True(t, a.hasLocalArchManifest(), "manifest must exist after boot (via boot-facts-recompact's Reindex)")
	assert.True(t, a.hasFreshFacts(), "facts substrate must be fresh after boot")
}

// TestFDN3_BootTriggers_PC1AloneCoversFreshFactsCase verifies PC1 still fires
// on its own when facts are ALREADY fresh but the arch manifest is missing
// (e.g. after `aoa arch recon`) — the added hasFreshFacts() gate must not
// suppress the legitimate PC1-only case.
func TestFDN3_BootTriggers_PC1AloneCoversFreshFactsCase(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(goFile, []byte(`package main

import "fmt"

func Hello() { fmt.Println("hello") }
`), 0644))

	dbPath := filepath.Join(t.TempDir(), "fdn3_fresh.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	// First boot: full Reindex path populates edges + facts + arch manifest.
	a := newBurstTestApp(t, tmpDir, store)
	a.ArchEnabled = true
	a.WarmCaches(func(string) {})
	a.bgWg.Wait()
	require.True(t, a.hasFreshFacts(), "pre-condition: facts fresh after first boot")
	require.True(t, a.hasLocalArchManifest(), "pre-condition: manifest present after first boot")

	// Simulate `aoa arch recon` invalidation: manifest/shards gone, facts
	// substrate untouched (recon only invalidates the derived view cache).
	require.NoError(t, store.DeleteShardsForScope("test", "local"))
	m, err := store.LoadManifest("test", "local")
	require.NoError(t, err)
	require.Empty(t, m, "pre-condition: manifest absent after invalidation")

	counting := &manifestCountingStore{storeBackend: store}
	a2 := newBurstTestApp(t, tmpDir, counting)
	a2.ArchEnabled = true
	a2.WarmCaches(func(string) {})
	a2.bgWg.Wait()

	assert.Equal(t, int64(1), counting.saveManifestCalls.Load(),
		"PC1 alone must still fire and re-derive when facts are fresh but the manifest is absent")
	manifestData, err := store.LoadManifest("test", "local")
	require.NoError(t, err)
	assert.NotEmpty(t, manifestData, "manifest must be re-created")
}
