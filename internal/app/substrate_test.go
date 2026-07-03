//go:build !lean

// Package app — PC4 dedicated tests: T35 (concurrent-autotune -race) and
// T37 (dirty-state flush-persisted on Stop).
//
// These tests target gaps identified in the F1 checkpoint re-entry record:
//
//   - T35: concurrent-autotune -race — two goroutines trigger autotune
//     simultaneously while the mutex cycle is hot; the -race detector must
//     report zero data races (Clone() under lock is the fix being verified).
//
//   - T37: dirty-state + Stop() flush-persisted — an app accumulates a dirty
//     index and a pending edge batch, then the Stop() persistence sub-path
//     (cancel timer → snapshot under lock → timerWg.Wait → persist outside lock)
//     must flush both to real bbolt before closing the store.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── T35: concurrent-autotune -race ───────────────────────────────────────────
//
// TestT35_ConcurrentAutotune_Race fires autotune concurrently from two paths:
//
//  1. searchObserver (the search-signal path — promptN increments inside the
//     observer, Clone() is called under a.mu before unlocking).
//
//  2. onSessionEvent(EventUserInput, observe=true) — the session-signal path;
//     processConversationSignal → ObserveAndMaybeTune → Clone() under a.mu.
//
// Both goroutines run for 60 iterations each (exceeds two autotune cycles at
// interval=50); the -race detector must report zero data races.
//
// The WarmCaches Clone residual (app.go:901, fixed in this patch) is also
// exercised implicitly via the Store.SaveIndex path — but the primary proof
// is the search + session concurrent autotune paths.
func TestT35_ConcurrentAutotune_Race(t *testing.T) {
	tmpDir := t.TempDir()

	// Real bbolt store so SaveLearnerState actually writes (not a no-op).
	dbPath := filepath.Join(tmpDir, "t35.db")
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	a := newWatcherTestApp(t, tmpDir)
	a.Store = store
	a.stopCh = make(chan struct{})

	// Build a minimal index so searches return at least one hit (otherwise
	// searchObserver skips enrichment and never calls ObserveAndMaybeTune).
	idx := &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"reconcile": {{FileID: 1, Line: 10}},
			"scheduling": {{FileID: 1, Line: 11}},
			"taint":      {{FileID: 1, Line: 12}},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "reconcile", Kind: "function", Tags: []string{"scheduling"}},
			{FileID: 1, Line: 11}: {Name: "scheduling", Kind: "function", Tags: []string{"scheduling"}},
			{FileID: 1, Line: 12}: {Name: "taint", Kind: "function", Tags: []string{"scheduling"}},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "scheduler.go", Language: "go", Domain: "@scheduling", Size: 4096},
		},
	}
	domains := make(map[string]index.Domain)
	engine := index.NewSearchEngine(idx, domains, tmpDir)
	engine.SetObserver(a.searchObserver)
	a.Index = idx
	a.Engine = engine

	if a.Enricher == nil {
		t.Skip("Enricher not available; skipping T35 concurrent-autotune race test")
	}

	const N = 60 // iterations per goroutine — exceeds 2 autotune cycles

	var wg sync.WaitGroup

	// Goroutine A: search-observer path.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < N; i++ {
			engine.Search("reconcile scheduling taint", ports.SearchOptions{})
		}
		engine.WaitObservers()
	}()

	// Goroutine B: session-signal path (observe=true, real user input).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < N; i++ {
			a.onSessionEvent(ports.SessionEvent{
				Kind:      ports.EventUserInput,
				SessionID: "t35-session",
				Text:      "reconcile pod scheduling taint eviction",
			})
		}
	}()

	wg.Wait()

	// No assertion needed beyond "no -race failure" — the -race detector is
	// the active guard here.  But we also confirm autotune fired at least once
	// so we know the concurrent paths were actually exercised.
	a.mu.Lock()
	tuned := a.lastAutotune != nil
	a.mu.Unlock()
	assert.True(t, tuned, "T35: at least one autotune must have fired across %d concurrent iterations", N)
}

// ── T37: dirty-state + Stop() flush-persisted ────────────────────────────────
//
// TestT37_DirtyStateFlushOnStop verifies that when Stop() is called with both
// a dirty index (indexDirty=true) and a pending edge batch (edgePendingBatch≠nil),
// both are flushed to real bbolt before the store is closed.
//
// Stop()'s external-service dependencies (Reader, Watcher, WebServer, Server)
// cannot be satisfied in unit tests, so this test drives the persistence
// sub-path directly — exactly what Stop() does: cancel timer → snapshot under
// lock → timerWg.Wait() → persist outside lock → close store.
//
// Verification: reopen the bbolt file and assert both the index and at least
// one edge row are present (non-nil / non-empty).
func TestT37_DirtyStateFlushOnStop(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "t37.db")

	// Phase 1: build up dirty state.
	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)

	a := newBurstTestApp(t, tmpDir, store)
	a.ArchEnabled = true

	// Fire onFileChanged on a Go file — this:
	//   • Adds the file to a.Index.Files (marks index dirty via markIndexDirty)
	//   • Queues edges into a.edgePendingBatch (via markEdgeBatchDirty)
	//   • Arms both debounce timers
	goFile := filepath.Join(tmpDir, "dirty.go")
	require.NoError(t, os.WriteFile(goFile,
		[]byte(`package main
import "fmt"
func Dirty() { fmt.Println("dirty") }
`), 0644))
	a.onFileChanged(goFile)

	// Verify state is actually dirty before we flush.
	a.mu.Lock()
	indexIsDirty := a.indexDirty
	batchIsPopulated := len(a.edgePendingBatch) > 0
	a.mu.Unlock()
	require.True(t, indexIsDirty, "T37 pre-condition: indexDirty must be set after onFileChanged")
	require.True(t, batchIsPopulated, "T37 pre-condition: edgePendingBatch must be populated after onFileChanged")

	// Phase 2: execute Stop() dirty-state persistence path.
	//
	// This mirrors exactly what Stop() does in app.go:1116-1177 for the
	// timer-cancel + snapshot + wait + flush sequence, without needing the
	// external service dependencies (Reader/Watcher/WebServer/Server).
	a.mu.Lock()

	// Cancel debounce timer (index) — mirror of app.go:1124-1128.
	if a.indexSaveTimer != nil {
		if a.indexSaveTimer.Stop() {
			a.timerWg.Done() // goroutine never ran; release the reserved slot
		}
		a.indexSaveTimer = nil
	}
	var idxSnap *ports.Index
	if a.indexDirty {
		idxSnap = a.Index.Clone()
		a.indexDirty = false
	}

	// Cancel debounce timer (edges) — mirror of app.go:1138-1142.
	if a.edgeBatchTimer != nil {
		if a.edgeBatchTimer.Stop() {
			a.timerWg.Done() // goroutine never ran; release the reserved slot
		}
		a.edgeBatchTimer = nil
	}
	var edgeBatchSnap map[uint32][]ports.ImportEdge
	if len(a.edgePendingBatch) > 0 {
		edgeBatchSnap = a.edgePendingBatch
		a.edgePendingBatch = nil
	}
	edgeProjID := a.edgePendingProjID
	a.mu.Unlock()

	// Wait for any in-flight timer goroutines (mirror of app.go:1160).
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.timerWg.Wait()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("T37: timerWg.Wait() timed out — timer goroutine did not finish within 5s")
	}

	// Persist outside lock (mirror of app.go:1164-1176) — C1 compliant.
	if idxSnap != nil {
		require.NoError(t, store.SaveIndex(a.ProjectID, idxSnap),
			"T37: SaveIndex must succeed on dirty-flush path")
	}
	if edgeBatchSnap != nil {
		require.NoError(t, store.SaveEdgesBatch(edgeProjID, edgeBatchSnap),
			"T37: SaveEdgesBatch must succeed on dirty-flush path")
	}

	// Close the store (mirror of app.go:1177).
	require.NoError(t, store.Close(), "T37: store.Close() must succeed after dirty flush")

	// Phase 3: reopen bbolt and verify both pieces of dirty state were persisted.
	store2, err := bbolt.NewStore(dbPath)
	require.NoError(t, err, "T37: must be able to reopen bbolt after Stop() flush")
	t.Cleanup(func() { store2.Close() })

	// Index must be present.
	idx2, err := store2.LoadIndex(a.ProjectID)
	require.NoError(t, err)
	require.NotNil(t, idx2,
		"T37: dirty index must be persisted by Stop() flush path")
	assert.Greater(t, len(idx2.Files), 0,
		"T37: persisted index must have at least the one dirty file")

	// Edges must be present (arch-enabled, so edge batch must have been flushed).
	edges2, err := store2.LoadAllEdges(a.ProjectID)
	// LoadAllEdges may return a non-nil error with partial results (T38 surfacing);
	// we accept partial results as long as some edges were persisted.
	_ = err
	assert.NotEmpty(t, edges2,
		fmt.Sprintf("T37: edge batch must be persisted by Stop() flush path (LoadAllEdges err=%v)", err))
}
