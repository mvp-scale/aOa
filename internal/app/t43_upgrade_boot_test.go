//go:build !lean

// Package app — T43 upgrade-boot edge backfill.
//
// Acceptance gate (verbatim from F2 kickoff):
//
//	"populated index + empty edges bucket + flag on → WarmCaches backfills a non-empty bucket."
//
// Scenario A (arch_on): real bbolt DB with a populated index but NO edges bucket
// (simulating an old-binary project) + ArchEnabled=true → WarmCaches fires
// safeGo("upgrade-arch-derive") → Reindex → edges bucket exists and is non-empty.
//
// Scenario B (arch_off): same old-binary DB + ArchEnabled=false → no backfill
// (C4 law: arch extraction is gated by the flag).
package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestT43_UpgradeBootEdgeBackfill verifies that WarmCaches backfills the edges
// bucket on first boot against an old-binary DB (populated index, no edges bucket).
func TestT43_UpgradeBootEdgeBackfill(t *testing.T) {
	// Shared fixture: tmpDir with a Go file that has import edges so Reindex
	// has something to derive.
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(goFile, []byte(`package main

import "fmt"

func Hello() { fmt.Println("hello") }
`), 0644))

	parser := treesitter.NewParser()

	// Build index without arch extraction (arch=false) — simulates an old binary
	// that produced a populated index but no edges bucket.
	idx, _, _, err := BuildIndexWithFacts(tmpDir, parser, false)
	require.NoError(t, err)
	require.NotEmpty(t, idx.Files, "T43 pre-condition: index must have at least one file")

	// ── Scenario A: arch ON → backfill ────────────────────────────────────────

	t.Run("arch_on", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "t43_on.db")
		store, err := bbolt.NewStore(dbPath)
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })

		// Seed the index (no edges) — this is the old-binary DB state.
		require.NoError(t, store.SaveIndex("test", idx),
			"T43: SaveIndex must succeed when seeding old-binary DB fixture")
		assert.False(t, store.HasEdgesBucket("test"),
			"T43 pre-condition: edges bucket must NOT exist before WarmCaches")

		// Wire App: real store, real parser, arch ON.
		a := newBurstTestApp(t, tmpDir, store)
		a.ArchEnabled = true
		a.Parser = parser

		// Boot: WarmCaches detects populated index + no edges bucket + arch ON →
		// dispatches safeGo("upgrade-arch-derive") → Reindex().
		a.WarmCaches(func(string) {})

		// Wait for the background Reindex goroutine to complete.
		a.bgWg.Wait()

		// Gate: edges bucket must exist and be non-empty.
		assert.True(t, store.HasEdgesBucket("test"),
			"T43: edges bucket must exist after upgrade-boot backfill")
		edges, err := store.LoadAllEdges("test")
		require.NoError(t, err)
		assert.NotEmpty(t, edges,
			"T43: edges must be non-empty after upgrade-boot backfill")
	})

	// ── Scenario B: arch OFF → no backfill (C4 law) ───────────────────────────

	t.Run("arch_off", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "t43_off.db")
		store, err := bbolt.NewStore(dbPath)
		require.NoError(t, err)
		t.Cleanup(func() { _ = store.Close() })

		// Same old-binary DB state.
		require.NoError(t, store.SaveIndex("test", idx),
			"T43: SaveIndex must succeed when seeding old-binary DB fixture")

		// Wire App: arch OFF.
		a := newBurstTestApp(t, tmpDir, store)
		a.ArchEnabled = false
		a.Parser = parser

		a.WarmCaches(func(string) {})
		a.bgWg.Wait()

		// C4: no backfill when arch flag is OFF.
		assert.False(t, store.HasEdgesBucket("test"),
			"T43: edges bucket must NOT be created when arch flag is OFF (C4 law)")
	})
}
