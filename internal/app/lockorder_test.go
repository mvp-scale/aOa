//go:build !lean

// Package app — T17 lock-order assertion harness (L19.8).
//
// C1 invariant: Store.SaveIndex (which calls db.Update internally) must NEVER
// be called while App.mu is held. Violating this creates a priority-inversion
// risk and mirrors the exact antipattern the arch write path (L19.10 EdgeStore)
// must never copy.
//
// T17 works by wrapping the store with a lockGuardStore that calls
// App.mu.TryLock() on every SaveIndex invocation. If TryLock fails, the
// mutex is currently held by someone else — that means db.Update would be
// entered under the lock, which is the C1 violation. TryLock success means
// the lock is free; we immediately release it and forward to the inner store.
package app

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/corey/aoa/internal/ports"
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

// ── lockGuardStore ─────────────────────────────────────────────────────────
//
// lockGuardStore wraps a storeBackend and asserts the C1 invariant on every
// SaveIndex call: App.mu must NOT be held when db.Update fires.
//
// Mechanism: mu.TryLock() succeeds iff the mutex is currently free.
//   - TryLock succeeds → mutex was NOT held → invariant upheld ✓
//   - TryLock fails    → mutex IS held      → C1 violated → t.Fatalf
//
// After a successful TryLock we immediately release (we only needed the
// liveness check) then forward to the inner store.

type lockGuardStore struct {
	storeBackend        // embed to inherit all non-overridden methods
	mu    *sync.Mutex  // pointer to App.mu — NOT a copy
	t     testing.TB
}

// SaveIndex is the only method that triggers db.Update; all other write
// methods (SaveLearnerState, etc.) are out of scope for T17.
func (s *lockGuardStore) SaveIndex(projectID string, idx *ports.Index) error {
	if !s.mu.TryLock() {
		// TryLock failed → App.mu is currently held → C1 violation.
		s.t.Fatalf("T17 C1 violation: Store.SaveIndex called while App.mu is held "+
			"(project=%q)", projectID)
		return nil
	}
	s.mu.Unlock() // release; we only needed the liveness check
	return s.storeBackend.SaveIndex(projectID, idx)
}

// ── helper ─────────────────────────────────────────────────────────────────

// newLockGuardApp returns a watcher test App with a lockGuardStore injected
// as the Store. Any SaveIndex call that violates C1 will immediately Fatalf.
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
		// lockGuardStore.SaveIndex will Fatalf if App.mu is held.
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
		// lockGuardStore.SaveIndex would have Fatalfed if App.mu was held.
	})
}

// TestT17_Race exercises the same paths concurrently under -race to detect
// any data race introduced by the snapshot-release-write refactor.
func TestT17_Race(t *testing.T) {
	tmpDir := t.TempDir()
	a := newLockGuardApp(t, tmpDir)

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
