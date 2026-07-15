//go:build !lean

// Package app — T17 lock-order assertion harness (L19.8).
//
// C1 invariant: Store.SaveIndex, Store.SaveLearnerState, Store.SaveAllDimensions,
// and Store.SaveSessionWithTelemetry (all db.Update callers) must NEVER be called
// while App.mu is held. Violating this creates a priority-inversion risk and
// mirrors the exact antipattern the arch write path (L19.10 EdgeStore) must
// never copy.
//
// T17 works by wrapping the store with a lockGuardStore that calls
// App.mu.TryLock() on every db.Update method. If TryLock fails, the mutex is
// currently held by someone else — C1 violation. TryLock success means the
// lock is free; we immediately release it and forward to the inner store.
//
// IMPORTANT: lockGuardStore is ONLY sound in sequential (non-concurrent) tests.
// In concurrent tests, TryLock may fail because a DIFFERENT goroutine holds the
// mutex for unrelated work — a false positive (T32). TestT17_Race uses a plain
// noopStore and relies on the -race detector for concurrent correctness; it does
// NOT use lockGuardStore.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/domain/learner"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── noopStore ──────────────────────────────────────────────────────────────
//
// noopStore is a zero-overhead storeBackend that satisfies the full interface
// with no-op implementations. Used by T17 so the test never touches a real
// bbolt database.

type noopStore struct{}

func (n *noopStore) SaveIndex(_ string, _ *ports.Index) error              { return nil }
func (n *noopStore) LoadIndex(_ string) (*ports.Index, error)             { return nil, nil }
func (n *noopStore) SaveLearnerState(_ string, _ *ports.LearnerState) error { return nil }
func (n *noopStore) LoadLearnerState(_ string) (*ports.LearnerState, error) { return nil, nil }
func (n *noopStore) DeleteProject(_ string) error                          { return nil }
func (n *noopStore) SaveSessionSummary(_ string, _ *ports.SessionSummary) error {
	return nil
}
func (n *noopStore) LoadSessionSummary(_ string, _ string) (*ports.SessionSummary, error) {
	return nil, nil
}
func (n *noopStore) ListSessionSummaries(_ string) ([]*ports.SessionSummary, error) {
	return nil, nil
}
func (n *noopStore) SaveSessionWithTelemetry(_ string, _ *ports.SessionSummary, _ *ports.ProjectTelemetry) error {
	return nil
}
func (n *noopStore) LoadTelemetry(_ string) (*ports.ProjectTelemetry, error) {
	return &ports.ProjectTelemetry{}, nil
}
func (n *noopStore) Healthy() bool   { return true }
func (n *noopStore) Recovered() bool { return false }
func (n *noopStore) Close() error    { return nil }
func (n *noopStore) SaveAllDimensions(_ string, _ map[string]*ports.FileAnalysis) error {
	return nil
}
func (n *noopStore) LoadAllDimensions(_ string) (map[string]*ports.FileAnalysis, error) {
	return nil, nil
}

// EdgeStore no-ops (L19.10/L19.12) — C1: these must never be called while App.mu is held.
func (n *noopStore) SaveEdgesForFile(_ string, _ uint32, _ []ports.ImportEdge) error {
	return nil
}
func (n *noopStore) SaveEdgesBatch(_ string, _ map[uint32][]ports.ImportEdge) error {
	return nil
}
func (n *noopStore) LoadEdgesForFile(_ string, _ uint32) ([]ports.ImportEdge, error) {
	return nil, nil
}
func (n *noopStore) DeleteEdgesForFile(_ string, _ uint32) error { return nil }
func (n *noopStore) LoadAllEdges(_ string) ([]ports.ImportEdge, error) { return nil, nil }
func (n *noopStore) SaveUnresolved(_ string, _ []ports.ImportEdge) error  { return nil }
func (n *noopStore) ReplaceAllEdges(_ string, _ map[uint32][]ports.ImportEdge) error {
	return nil
}
func (n *noopStore) HasEdgesBucket(_ string) bool { return false }

// ArchStore no-ops (L19.14) — C1: write methods must never be called while App.mu is held.
func (n *noopStore) SaveShards(_ string, _ map[string][]byte) error       { return nil }
func (n *noopStore) LoadShard(_ string, _ string) ([]byte, error)         { return nil, nil }
func (n *noopStore) SaveManifest(_ string, _ string, _ []byte) error      { return nil }
func (n *noopStore) LoadManifest(_ string, _ string) ([]byte, error)      { return nil, nil }
func (n *noopStore) DeleteShardsForScope(_ string, _ string) error        { return nil }
func (n *noopStore) HasArchBucket(_ string) bool                          { return false }

// FindingsStore no-ops (L19.15) — C1: SaveFindings must never be called while App.mu is held.
func (n *noopStore) SaveFindings(_ string, _ string, _ []ports.Finding) error {
	return nil
}
func (n *noopStore) LoadFindings(_ string, _ string) ([]ports.Finding, error) {
	return nil, nil
}

// FactStore no-ops (FDN-1/FDN-3) — C1: write methods must never be called
// while App.mu is held.
func (n *noopStore) ReplaceFactsForFile(_, _ string, _ []ports.Fact) error { return nil }
func (n *noopStore) PutResolved(_ string, _ []ports.Fact, _ *ports.DepAdjacency) error {
	return nil
}
func (n *noopStore) PutFindings(_ string, _ []ports.Fact) error { return nil }
func (n *noopStore) ReplaceAllFacts(_ string, _ map[string][]ports.Fact) error {
	return nil
}
func (n *noopStore) FactsByKind(_ string, _ ports.FactKind) ([]ports.Fact, error) {
	return nil, nil
}
func (n *noopStore) FactsForSubject(_, _ string) ([]ports.Fact, error) { return nil, nil }
func (n *noopStore) FactsMeta(_ string) (map[string]string, error)     { return nil, nil }
func (n *noopStore) Dependencies(_, _ string) ([]ports.DepEdge, error) { return nil, nil }
func (n *noopStore) Dependents(_, _ string) ([]ports.DepEdge, error)   { return nil, nil }
func (n *noopStore) SaveBaseline(_, _ string, _ *ports.FactBaseline) error {
	return nil
}
func (n *noopStore) LoadBaseline(_, _ string) (*ports.FactBaseline, error) {
	return nil, nil
}
func (n *noopStore) DeleteProjectFacts(_ string) error { return nil }

// ── lockGuardStore ─────────────────────────────────────────────────────────
//
// lockGuardStore wraps a storeBackend and asserts the C1 invariant on every
// db.Update method: App.mu must NOT be held when db.Update fires.
//
// Mechanism: mu.TryLock() succeeds iff the mutex is currently free.
//   - TryLock succeeds → mutex was NOT held → invariant upheld ✓
//   - TryLock fails    → mutex IS held      → C1 violated → t.Errorf
//
// After a successful TryLock we immediately release (we only needed the
// liveness check) then forward to the inner store.
//
// SOUND ONLY in sequential tests: TryLock detects "held by anyone", not
// "held by caller". Concurrent tests must use noopStore + -race instead
// (see TestT17_Race).

type lockGuardStore struct {
	storeBackend      // embed to inherit all non-overridden methods
	mu  *sync.Mutex  // pointer to App.mu — NOT a copy
	t   testing.TB
}

func (s *lockGuardStore) assertUnlocked(method string, projectID string) {
	if !s.mu.TryLock() {
		// t.Errorf is goroutine-safe (unlike t.Fatalf which calls runtime.Goexit).
		// For sequential tests the calling goroutine IS the test goroutine, so
		// t.Errorf marks the test failed without risk of panic (T23 fix).
		s.t.Errorf("T17 C1 violation: Store.%s called while App.mu is held (project=%q)",
			method, projectID)
		return
	}
	s.mu.Unlock() // release; we only needed the liveness check
}

// SaveIndex triggers db.Update on the index bucket.
func (s *lockGuardStore) SaveIndex(projectID string, idx *ports.Index) error {
	s.assertUnlocked("SaveIndex", projectID)
	return s.storeBackend.SaveIndex(projectID, idx)
}

// SaveLearnerState triggers db.Update on the learner bucket.
func (s *lockGuardStore) SaveLearnerState(projectID string, st *ports.LearnerState) error {
	s.assertUnlocked("SaveLearnerState", projectID)
	return s.storeBackend.SaveLearnerState(projectID, st)
}

// SaveSessionWithTelemetry triggers db.Update on the session + telemetry buckets.
func (s *lockGuardStore) SaveSessionWithTelemetry(projectID string, sum *ports.SessionSummary, delta *ports.ProjectTelemetry) error {
	s.assertUnlocked("SaveSessionWithTelemetry", projectID)
	return s.storeBackend.SaveSessionWithTelemetry(projectID, sum, delta)
}

// SaveAllDimensions triggers db.Update on the dim-analysis bucket.
func (s *lockGuardStore) SaveAllDimensions(projectID string, analyses map[string]*ports.FileAnalysis) error {
	s.assertUnlocked("SaveAllDimensions", projectID)
	return s.storeBackend.SaveAllDimensions(projectID, analyses)
}

// SaveEdgesForFile triggers db.Update on the edges bucket (L19.10 C1 check).
func (s *lockGuardStore) SaveEdgesForFile(projectID string, fileID uint32, edges []ports.ImportEdge) error {
	s.assertUnlocked("SaveEdgesForFile", projectID)
	return s.storeBackend.SaveEdgesForFile(projectID, fileID, edges)
}

// SaveEdgesBatch triggers a single db.Update for the entire batch (L19.12 C1+C2 check).
func (s *lockGuardStore) SaveEdgesBatch(projectID string, batch map[uint32][]ports.ImportEdge) error {
	s.assertUnlocked("SaveEdgesBatch", projectID)
	return s.storeBackend.SaveEdgesBatch(projectID, batch)
}

// DeleteEdgesForFile triggers db.Update on the edges bucket (L19.10 C1 check).
func (s *lockGuardStore) DeleteEdgesForFile(projectID string, fileID uint32) error {
	s.assertUnlocked("DeleteEdgesForFile", projectID)
	return s.storeBackend.DeleteEdgesForFile(projectID, fileID)
}

// SaveUnresolved triggers db.Update on the facts_unresolved bucket (§2.4 C1 check).
func (s *lockGuardStore) SaveUnresolved(projectID string, entries []ports.ImportEdge) error {
	s.assertUnlocked("SaveUnresolved", projectID)
	return s.storeBackend.SaveUnresolved(projectID, entries)
}

// ReplaceAllEdges triggers a single db.Update that drops and rewrites all edges (P3 C1 check).
func (s *lockGuardStore) ReplaceAllEdges(projectID string, fileEdges map[uint32][]ports.ImportEdge) error {
	s.assertUnlocked("ReplaceAllEdges", projectID)
	return s.storeBackend.ReplaceAllEdges(projectID, fileEdges)
}

// LoadAllEdges is a read-only method (db.View) so no C1 check is needed.
// PC4: return real edges so deriveArch proceeds past the early-return guard
// and the guarded write methods (SaveShards / SaveManifest / SaveFindings)
// actually execute under the lock-order test, de-vacuating the harness.
func (s *lockGuardStore) LoadAllEdges(projectID string) ([]ports.ImportEdge, error) {
	return edgesForDeriveTest(), nil
}

// ArchStore write methods (L19.14 C1 check) — db.Update callers that must never
// fire while App.mu is held.

// SaveShards triggers db.Update on the arch_shards bucket (L19.14 C1 check).
func (s *lockGuardStore) SaveShards(projectID string, shards map[string][]byte) error {
	s.assertUnlocked("SaveShards", projectID)
	return s.storeBackend.SaveShards(projectID, shards)
}

// SaveManifest triggers db.Update on the arch_shards bucket (L19.14 C1 check).
func (s *lockGuardStore) SaveManifest(projectID, scope string, data []byte) error {
	s.assertUnlocked("SaveManifest", projectID)
	return s.storeBackend.SaveManifest(projectID, scope, data)
}

// DeleteShardsForScope triggers db.Update on the arch_shards bucket (L19.14 C1 check).
func (s *lockGuardStore) DeleteShardsForScope(projectID, scope string) error {
	s.assertUnlocked("DeleteShardsForScope", projectID)
	return s.storeBackend.DeleteShardsForScope(projectID, scope)
}

// SaveFindings triggers db.Update on the facts_findings bucket (L19.15 C1 check).
func (s *lockGuardStore) SaveFindings(projectID, scope string, findings []ports.Finding) error {
	s.assertUnlocked("SaveFindings", projectID)
	return s.storeBackend.SaveFindings(projectID, scope, findings)
}

// ── helper ─────────────────────────────────────────────────────────────────

// newLockGuardApp returns a watcher test App with a lockGuardStore injected
// as the Store. Any db.Update call that violates C1 will mark the test failed.
// ONLY use in sequential (non-concurrent) subtests.
func newLockGuardApp(t *testing.T, root string) *App {
	t.Helper()
	a := newWatcherTestApp(t, root)
	a.Store = &lockGuardStore{
		storeBackend: &noopStore{},
		mu:           &a.mu,
		t:            t,
	}
	return a
}

// newRaceApp returns a watcher test App with a plain noopStore — suitable for
// concurrent race-detection tests where lockGuardStore would false-positive.
func newRaceApp(t *testing.T, root string) *App {
	t.Helper()
	a := newWatcherTestApp(t, root)
	a.Store = &noopStore{}
	return a
}

// ── T17 tests ──────────────────────────────────────────────────────────────

// TestT17_NoSaveIndexUnderMu verifies C1: Store.SaveIndex is never called
// while App.mu is held. Covers every code path that reaches SaveIndex.
func TestT17_NoSaveIndexUnderMu(t *testing.T) {
	t.Run("watcher_symbol_path", func(t *testing.T) {
		// onFileChanged (symbol extraction) → markIndexDirty → timer callback
		// → doSaveIndexDebounced → SaveIndex outside lock.
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)

		goFile := filepath.Join(tmpDir, "hello.go")
		require.NoError(t, os.WriteFile(goFile,
			[]byte("package main\nfunc Hello() {}"), 0644))

		// arm the watcher path (sets indexDirty, starts debounce timer)
		a.onFileChanged(goFile)

		// Drive the debounced save callback directly; avoids waiting 2 s.
		// lockGuardStore.SaveIndex will Errorf if App.mu is held.
		a.doSaveIndexDebounced()
	})

	t.Run("watcher_tokenise_path", func(t *testing.T) {
		// onFileChanged with no parser → tokenisation-only path → markIndexDirty
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)
		a.Parser = nil // force tokenisation-only mode

		txtFile := filepath.Join(tmpDir, "notes.txt")
		require.NoError(t, os.WriteFile(txtFile,
			[]byte("hello world"), 0644))

		// .txt is not in defaultCodeExtensions so write a .go file without a parser
		goFile := filepath.Join(tmpDir, "plain.go")
		require.NoError(t, os.WriteFile(goFile,
			[]byte("package main\nfunc Plain() {}"), 0644))

		a.onFileChanged(goFile)
		a.doSaveIndexDebounced()
	})

	t.Run("watcher_delete_path", func(t *testing.T) {
		// onFileChanged after file removal → markIndexDirty.
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)

		goFile := filepath.Join(tmpDir, "bye.go")
		require.NoError(t, os.WriteFile(goFile,
			[]byte("package main\nfunc Bye() {}"), 0644))
		a.onFileChanged(goFile) // seed the file into the index

		// Clear dirty so we can isolate the delete signal.
		a.mu.Lock()
		a.indexDirty = false
		a.mu.Unlock()

		require.NoError(t, os.Remove(goFile))
		a.onFileChanged(goFile) // triggers delete path → markIndexDirty
		a.doSaveIndexDebounced()
	})

	t.Run("reindex_path", func(t *testing.T) {
		// Reindex: build outside lock, swap under lock, Save outside lock.
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)

		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, "hello.go"),
			[]byte("package main\nfunc Hello() {}"),
			0644,
		))

		_, err := a.Reindex()
		require.NoError(t, err)
		// lockGuardStore.SaveIndex would have Errorfed if App.mu was held.
	})

	t.Run("autotune_path", func(t *testing.T) {
		// searchObserver after 50 prompts triggers autotune → SaveLearnerState.
		// lockGuardStore.SaveLearnerState will Errorf if App.mu is held.
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)

		// Prime the learner: set promptN = 49 so the next increment inside
		// searchObserver reaches 50 and triggers ObserveAndMaybeTune → autotune.
		a.mu.Lock()
		a.promptN = 49
		a.mu.Unlock()

		// Drive searchObserver with a non-empty query so it doesn't early-return.
		result := &index.SearchResult{}
		a.searchObserver("hello", ports.SearchOptions{}, result, time.Millisecond)
		// If SaveLearnerState was called under a.mu, lockGuardStore would have Errorfed.
	})

	t.Run("session_flush_path", func(t *testing.T) {
		// onSessionEvent with a new SessionID triggers handleSessionBoundary →
		// buildSessionFlushPayload → doSessionFlush → SaveSessionWithTelemetry.
		// lockGuardStore.SaveSessionWithTelemetry will Errorf if App.mu is held.
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)

		// First event: establish session A.
		a.onSessionEvent(ports.SessionEvent{
			Kind:      ports.EventSystemMeta,
			SessionID: "session-A",
		})

		// Second event: switch to session B — triggers flush of session A.
		// doSessionFlush must be called outside a.mu for SaveSessionWithTelemetry.
		a.onSessionEvent(ports.SessionEvent{
			Kind:      ports.EventSystemMeta,
			SessionID: "session-B",
		})
		// If SaveSessionWithTelemetry was called under a.mu, lockGuardStore would have Errorfed.
	})

	t.Run("session_user_input_autotune_path", func(t *testing.T) {
		// onSessionEvent(EventUserInput, observe=true) → processConversationSignal
		// → ObserveAndMaybeTune → autotune → SaveLearnerState.
		// Reproduces the C1 violation at observer.go:142.
		// lockGuardStore.SaveLearnerState will Errorf if App.mu is held.
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)

		// Drive 50 user-input events so the 50th triggers autotune inside
		// processConversationSignal (observe=true path). The Enricher must be
		// non-nil for processConversationSignal to do work.
		if a.Enricher == nil {
			t.Skip("Enricher not available in this build; skipping session-signal autotune path")
		}

		for i := 0; i < 50; i++ {
			a.onSessionEvent(ports.SessionEvent{
				Kind:      ports.EventUserInput,
				SessionID: "session-autotune",
				Text:      "reconcile pod scheduling taint eviction",
			})
		}
		// If SaveLearnerState was called under a.mu at the 50th event,
		// lockGuardStore would have Errorfed.

		// Assert autotune actually fired (not vacuous — T17 guards only fire
		// when the path is exercised; lastAutotune is set under lock in writeStatus).
		a.mu.Lock()
		tuned := a.lastAutotune != nil
		a.mu.Unlock()
		assert.True(t, tuned,
			"T17/session_user_input_autotune_path: autotune must fire at promptN=50")
	})

	t.Run("session_grep_autotune_path", func(t *testing.T) {
		// onSessionEvent(EventToolInvocation, tool=Grep) → processGrepSignal
		// → ObserveAndMaybeTune → autotune → SaveLearnerState.
		// Reproduces the C1 violation at observer.go:182.
		// lockGuardStore.SaveLearnerState will Errorf if App.mu is held.
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)

		if a.Enricher == nil {
			t.Skip("Enricher not available in this build; skipping grep-signal autotune path")
		}

		// Establish a session first.
		a.onSessionEvent(ports.SessionEvent{
			Kind:      ports.EventSystemMeta,
			SessionID: "grep-session",
		})

		// Prime promptN to 50 so the grep event fires autotune:
		// processGrepSignal passes PromptNumber=a.promptN to ObserveAndMaybeTune;
		// autotune fires when PromptCount % AutotuneInterval == 0 (interval=50).
		// promptN=49 is OFF BY ONE: 49 % 50 ≠ 0, autotune never fires, test is
		// vacuous. promptN=50: 50 % 50 == 0, autotune fires, test is live.
		a.mu.Lock()
		a.promptN = 50
		a.mu.Unlock()

		a.onSessionEvent(ports.SessionEvent{
			Kind:      ports.EventToolInvocation,
			SessionID: "grep-session",
			Tool: &ports.ToolEvent{
				Name:    "Grep",
				Pattern: "reconcile pod scheduling taint eviction",
			},
		})
		// If SaveLearnerState was called under a.mu, lockGuardStore would have Errorfed.

		// Assert autotune actually fired (guard liveness: promptN=50 guarantees it).
		a.mu.Lock()
		tuned := a.lastAutotune != nil
		a.mu.Unlock()
		assert.True(t, tuned,
			"T17/session_grep_autotune_path: autotune must fire when promptN=50 and grep pattern enriches")
	})
}

// TestT17_EdgeStoreNotUnderMu verifies C1 for the L19.12 EdgeStore batch write path:
// SaveEdgesBatch (called from doFlushEdgeBatch) is never called while App.mu is held.
// onFileChanged enqueues into edgePendingBatch under a.mu; the timer fires
// doFlushEdgeBatch which snapshots-under-lock, releases, then calls SaveEdgesBatch.
func TestT17_EdgeStoreNotUnderMu(t *testing.T) {
	t.Run("watcher_edge_save_path", func(t *testing.T) {
		// onFileChanged (arch-enabled) enqueues edges into edgePendingBatch under mu,
		// then doFlushEdgeBatch dispatches SaveEdgesBatch outside mu — C1 compliant.
		// lockGuardStore.SaveEdgesBatch will Errorf if App.mu is held.
		// bgWg.Wait: PC4 — lockGuardStore.LoadAllEdges returns real edges so the
		// arch-derive goroutine runs to completion; we drain it to avoid T32
		// false-positives from the goroutine calling SaveShards while a subsequent
		// test step holds a.mu.
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)
		a.ArchEnabled = true // enable C4 so the edge path fires

		goFile := filepath.Join(tmpDir, "main.go")
		require.NoError(t, os.WriteFile(goFile,
			[]byte("package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"hi\") }"), 0644))

		a.onFileChanged(goFile)  // populates edgePendingBatch
		a.doFlushEdgeBatch()     // triggers SaveEdgesBatch — C1 guard fires here
		a.bgWg.Wait()            // drain arch-derive goroutine (PC4: now does real work)
	})

	t.Run("watcher_edge_delete_path", func(t *testing.T) {
		// onFileChanged for a deleted file enqueues a nil delete entry into
		// edgePendingBatch; doFlushEdgeBatch dispatches SaveEdgesBatch outside mu.
		// lockGuardStore.SaveEdgesBatch will Errorf if App.mu is held.
		// bgWg.Wait between flushes: PC4 — with real edges, the arch-derive goroutine
		// from flush #1 must complete before flush #2 holds a.mu (otherwise the
		// goroutine's SaveShards call hits a T32 false-positive from doFlushEdgeBatch's
		// opening a.mu.Lock()).
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)
		a.ArchEnabled = true

		goFile := filepath.Join(tmpDir, "bye.go")
		require.NoError(t, os.WriteFile(goFile,
			[]byte("package main\nimport \"os\"\nfunc bye() { os.Exit(0) }"), 0644))

		// Seed the file into the index and flush its edges.
		a.onFileChanged(goFile)
		a.doFlushEdgeBatch()
		a.bgWg.Wait() // drain arch-derive goroutine from flush #1 before continuing

		// Now delete it — enqueues nil into edgePendingBatch.
		require.NoError(t, os.Remove(goFile))
		a.onFileChanged(goFile)
		a.doFlushEdgeBatch() // lockGuardStore.SaveEdgesBatch Errorfed if under mu
		a.bgWg.Wait()        // drain arch-derive goroutine from flush #2
	})

	t.Run("watcher_edge_batch_flush_path", func(t *testing.T) {
		// Focused C1 assertion for the doFlushEdgeBatch dispatch path (L19.12):
		//   1. onFileChanged populates edgePendingBatch (under a.mu)
		//   2. doFlushEdgeBatch snapshots under lock, releases, then calls SaveEdgesBatch
		//   3. lockGuardStore.assertUnlocked("SaveEdgesBatch") Errorfed if mu is held
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)
		a.ArchEnabled = true

		goFile := filepath.Join(tmpDir, "batch.go")
		require.NoError(t, os.WriteFile(goFile,
			[]byte("package main\nimport \"fmt\"\nfunc Batch() { fmt.Println() }"), 0644))

		a.onFileChanged(goFile) // step 1: accumulate
		a.doFlushEdgeBatch()    // step 2+3: flush — C1 check fires inside SaveEdgesBatch
		a.bgWg.Wait()           // drain arch-derive goroutine (PC4: now does real work)
	})
}

// capturingTB wraps a testing.TB and captures Errorf calls without failing the
// outer test.  Used by TestT17_SaveAllDimensionsNotUnderMu to prove the
// lockGuardStore CAN fire for a C1 violation before asserting the correct path
// does not fire — turning a vacuous positive-only test into a live guard check.
type capturingTB struct {
	testing.TB
	mu   sync.Mutex
	errs []string
}

func (c *capturingTB) Errorf(format string, args ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errs = append(c.errs, fmt.Sprintf(format, args...))
}

func (c *capturingTB) Helper() {}

func (c *capturingTB) errors() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]string, len(c.errs))
	copy(cp, c.errs)
	return cp
}

// TestT17_SaveAllDimensionsNotUnderMu verifies C1 for SaveAllDimensions:
// the dim-engine scan persists via SaveAllDimensions which must not be called
// while App.mu is held.
//
// De-vacuousing (PC4): this test now includes a NEGATIVE assertion — it
// temporarily replaces the lockGuardStore's testing.TB with a capturingTB,
// calls SaveAllDimensions while holding a.mu, and asserts the guard DID fire.
// Then it restores the real TB and calls SaveAllDimensions without the lock to
// confirm the production path is clean.  Both assertions are required; without
// the negative check the test cannot fail even if the guard is broken.
func TestT17_SaveAllDimensionsNotUnderMu(t *testing.T) {
	tmpDir := t.TempDir()
	a := newLockGuardApp(t, tmpDir)

	analyses := map[string]*ports.FileAnalysis{
		"main.go": {Path: "main.go", Language: "go"},
	}

	// ── Negative assertion: guard must FIRE when called under the lock ────────
	// Swap the real t for a capturingTB so Errorf is captured, not test-fatal.
	capture := &capturingTB{TB: t}
	lgStore := a.Store.(*lockGuardStore)
	lgStore.t = capture

	a.mu.Lock()
	_ = a.Store.SaveAllDimensions(a.ProjectID, analyses) // SHOULD trigger the guard
	a.mu.Unlock()

	captured := capture.errors()
	assert.NotEmpty(t, captured,
		"T17/SaveAllDimensions: lockGuardStore must fire when called under a.mu (guard liveness)")

	// ── Positive assertion: guard must NOT fire when called without the lock ──
	// Restore the real TB.
	lgStore.t = t
	err := a.Store.SaveAllDimensions(a.ProjectID, analyses)
	require.NoError(t, err,
		"T17/SaveAllDimensions: calling outside a.mu must succeed cleanly")
}

// TestT17_LearnerObserveEvent confirms the Learner's ObserveEvent mechanism
// compiles and routes correctly — ensures the test-wiring used by the session
// autotune subtests above is type-correct.
func TestT17_LearnerObserveEvent(t *testing.T) {
	l := learner.New()
	ev := learner.ObserveEvent{
		PromptNumber: 1,
		Observe: learner.ObserveData{
			Keywords: []string{"reconcile"},
			Terms:    []string{"scheduling"},
			Domains:  []string{"scheduling"},
		},
	}
	l.Observe(ev)
	// No assertion needed — this confirms the type signatures compile without drift.
}

// TestT17_Race exercises the snapshot-release-write paths concurrently under
// -race to detect data races introduced by the C1 refactor. Uses a plain
// noopStore (NOT lockGuardStore) to avoid TryLock false-positives: when goroutine
// A calls SaveIndex legitimately (not under a.mu), TryLock can still fail because
// goroutine B holds a.mu for unrelated work — the T32 false-positive (T17 flaw).
// Race detection is the job of the -race detector, not TryLock.
func TestT17_Race(t *testing.T) {
	tmpDir := t.TempDir()
	a := newRaceApp(t, tmpDir) // plain noopStore — no TryLock false-positives

	goFile := filepath.Join(tmpDir, "race.go")
	require.NoError(t, os.WriteFile(goFile,
		[]byte("package main\nfunc Race() {}"), 0644))

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 5; i++ {
			a.onFileChanged(goFile)
			a.doSaveIndexDebounced()
		}
	}()

	for i := 0; i < 5; i++ {
		_, _ = a.Reindex()
	}
	<-done
}

// TestT17_ArchWritePathNotUnderMu verifies C1 for the arch write paths (L19.14):
// SaveShards, SaveManifest, and SaveFindings are all db.Update callers that must
// NEVER be called while App.mu is held.
//
// Each sub-test exercises a discrete path through deriveArch that reaches a
// write method, then asserts the lockGuardStore guard did NOT fire (no C1 violation).
// The negative liveness check (guard CAN fire) is done in the mu-held subtest.
func TestT17_ArchWritePathNotUnderMu(t *testing.T) {
	// ── Negative assertion: guard must FIRE for SaveShards when called under mu ──
	t.Run("SaveShards_guard_fires_when_under_mu", func(t *testing.T) {
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)

		capture := &capturingTB{TB: t}
		lgStore := a.Store.(*lockGuardStore)
		lgStore.t = capture

		a.mu.Lock()
		_ = a.Store.SaveShards(a.ProjectID, map[string][]byte{"test/key@abc123": []byte("{}")})
		a.mu.Unlock()

		assert.NotEmpty(t, capture.errors(),
			"T17/arch: lockGuardStore must fire when SaveShards called under a.mu (guard liveness)")
		lgStore.t = t
	})

	// ── Positive assertion: SaveFindings is never called while App.mu is held ──
	// We call SaveFindings directly outside the lock to confirm the clean path.
	t.Run("SaveFindings_clean_when_outside_mu", func(t *testing.T) {
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)

		// Must be called WITHOUT holding a.mu — lockGuardStore must not fire.
		err := a.Store.SaveFindings(a.ProjectID, "local", []ports.Finding{})
		assert.NoError(t, err,
			"T17/arch: SaveFindings outside a.mu must succeed without C1 violation")
	})

	// ── deriveArch path: arch write path via the real derive pipeline ────────
	// deriveArch is always called via safeGo (outside mu). Verify that when we
	// call it directly (sequentially, outside mu), no C1 violations fire.
	// PC4: lockGuardStore now overrides LoadAllEdges to return edgesForDeriveTest()
	// so deriveArch proceeds past the early-return guard and actually calls
	// SaveShards / SaveManifest / SaveFindings — the guarded writes now execute.
	t.Run("deriveArch_write_path_not_under_mu", func(t *testing.T) {
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)
		a.ArchEnabled = true
		a.stopCh = make(chan struct{})

		// lockGuardStore.LoadAllEdges returns edgesForDeriveTest() so deriveArch
		// proceeds to SaveShards / SaveManifest / SaveFindings.
		// lockGuardStore must NOT fire on any of those calls (they run outside mu).
		a.deriveArch()
		// If SaveShards, SaveManifest, or SaveFindings were called under a.mu,
		// lockGuardStore would have Errorfed and marked the test failed.
	})

	// ── doFlushEdgeBatch → arch-derive trigger is outside mu ─────────────────
	// After a flush, safeGo("arch-derive") fires deriveArch outside mu.
	// This test fires doFlushEdgeBatch with a non-empty batch and confirms the
	// arch-derive goroutine (bgWg) completes without a C1 violation.
	t.Run("flush_triggers_arch_derive_outside_mu", func(t *testing.T) {
		tmpDir := t.TempDir()
		a := newLockGuardApp(t, tmpDir)
		a.ArchEnabled = true
		a.stopCh = make(chan struct{})

		goFile := filepath.Join(tmpDir, "main.go")
		require.NoError(t, os.WriteFile(goFile,
			[]byte("package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"hi\") }"), 0644))

		a.onFileChanged(goFile)     // populate edgePendingBatch
		a.doFlushEdgeBatch()        // flush + triggers safeGo("arch-derive")
		a.bgWg.Wait()               // wait for deriveArch to finish

		// If SaveShards/SaveManifest/SaveFindings were called under a.mu, the
		// lockGuardStore would have Errorfed; the test would have already failed.
	})
}
