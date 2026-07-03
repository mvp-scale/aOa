//go:build !lean

// Package app — T18 burst coalescing test (L19.12).
//
// Verifies C2: per-file edge writes within one debounce window are batched into
// a single write transaction (SaveEdgesBatch), not one tx per file.
//
// Acceptance criteria (board L19.12):
//   - 200 file events within 1 s → write-tx count bounded (single digits, not 200).
//   - Read latency sub-ms during burst (P99 < 1 ms).
//   - No ErrTimeout or deadlock.
//   - SaveEdgesBatch is atomic: all-or-nothing per window.
//   - -race clean.
package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── countingEdgeStore ─────────────────────────────────────────────────────────
//
// countingEdgeStore embeds noopStore and counts SaveEdgesBatch calls so T18 can
// assert that N file events coalesce into a bounded number of write transactions.
// Thread-safe via atomic counters.

type countingEdgeStore struct {
	noopStore
	batchCalls atomic.Int64 // number of SaveEdgesBatch calls
	fileSaves  atomic.Int64 // total file entries across all batch calls
}

func (s *countingEdgeStore) SaveEdgesBatch(_ string, batch map[uint32][]ports.ImportEdge) error {
	s.batchCalls.Add(1)
	s.fileSaves.Add(int64(len(batch)))
	return nil
}

// ── blockingReadStore ─────────────────────────────────────────────────────────
//
// blockingReadStore counts LoadEdgesForFile latencies so T18 can assert P99 < 1ms.
// It uses a small artificial sleep on SaveEdgesBatch to simulate a write lock.
// This proves that reads are NOT blocked by writes during the burst window (C2).

type blockingReadStore struct {
	noopStore
	mu           sync.Mutex    // held during SaveEdgesBatch to simulate write contention
	batchCalls   atomic.Int64
	readLatencies []time.Duration
	latMu        sync.Mutex
}

func (s *blockingReadStore) SaveEdgesBatch(_ string, _ map[uint32][]ports.ImportEdge) error {
	s.batchCalls.Add(1)
	s.mu.Lock()
	time.Sleep(5 * time.Millisecond) // simulate realistic bbolt write duration
	s.mu.Unlock()
	return nil
}

func (s *blockingReadStore) LoadEdgesForFile(_ string, _ uint32) ([]ports.ImportEdge, error) {
	start := time.Now()
	// Block until the write lock is available — measures wait-for-lock latency.
	// Use a closure so defer is used, satisfying the lock-discipline linter.
	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
	}()
	lat := time.Since(start)
	s.latMu.Lock()
	s.readLatencies = append(s.readLatencies, lat)
	s.latMu.Unlock()
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// newBurstTestApp returns a watcher-style test App with the provided store
// injected. ArchEnabled is false by default; set it explicitly in each test.
func newBurstTestApp(t *testing.T, root string, store storeBackend) *App {
	t.Helper()
	a := newWatcherTestApp(t, root)
	a.Store = store
	a.stopCh = make(chan struct{})
	return a
}

// writeGoFile writes a synthetic Go source file with one import.
func writeGoFile(t *testing.T, path string, i int) {
	t.Helper()
	src := fmt.Sprintf("package main\nimport \"fmt\"\nfunc F%d() { fmt.Println(%d) }\n", i, i)
	require.NoError(t, os.WriteFile(path, []byte(src), 0644))
}

// ── T18 tests ─────────────────────────────────────────────────────────────────

// TestT18_BurstCoalescing is the T18 burst write-pressure test (L19.12).
func TestT18_BurstCoalescing(t *testing.T) {
	const N = 200 // file-event count per burst

	// T18-a: tx count is bounded.
	// 200 file events within 1 s → SaveEdgesBatch called ≤ a small constant
	// (1 in this controlled test) instead of 200 individual write txs.
	t.Run("tx_count_bounded", func(t *testing.T) {
		tmpDir := t.TempDir()
		counter := &countingEdgeStore{}
		a := newBurstTestApp(t, tmpDir, counter)
		a.ArchEnabled = true

		// Create N Go files.
		files := make([]string, N)
		for i := 0; i < N; i++ {
			f := filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i))
			writeGoFile(t, f, i)
			files[i] = f
		}

		// T39: wall-clock bound — burst + flush must complete in ≤10 s.
		// (tree-sitter parses are slower under -race, so we use a generous bound
		// that still catches hung flushes or lock contention.)
		burstStart := time.Now()

		// Burst: fire N onFileChanged calls as fast as possible.
		for _, f := range files {
			a.onFileChanged(f)
		}

		// Before the timer fires, no SaveEdgesBatch calls should have occurred.
		assert.Equal(t, int64(0), counter.batchCalls.Load(),
			"no batch tx should fire while burst is accumulating")

		// Drain: call doFlushEdgeBatch directly (bypasses 200ms timer in tests).
		a.doFlushEdgeBatch()

		// T39: assert wall-clock bound.
		assert.Less(t, time.Since(burstStart), 10*time.Second,
			"T39: 200-file burst + flush must complete in <10s")

		// Assert: exactly 1 batch tx — all N file edges in one write transaction.
		batchCalls := counter.batchCalls.Load()
		assert.Equal(t, int64(1), batchCalls,
			"expected 1 SaveEdgesBatch call for %d events, got %d", N, batchCalls)

		// Assert: all N files contributed edges to the batch.
		fileSaves := counter.fileSaves.Load()
		assert.Equal(t, int64(N), fileSaves,
			"expected %d file-edge entries in batch, got %d", N, fileSaves)
	})

	// T18-b: reads stay sub-ms during the burst window.
	// Since no write tx fires during the burst (only at flush), concurrent reads
	// are unblocked and observe negligible latency. P99 < 1 ms.
	t.Run("reads_sub_ms_during_burst", func(t *testing.T) {
		tmpDir := t.TempDir()
		bstore := &blockingReadStore{}
		a := newBurstTestApp(t, tmpDir, bstore)
		a.ArchEnabled = true

		// Create N files.
		files := make([]string, N)
		for i := 0; i < N; i++ {
			f := filepath.Join(tmpDir, fmt.Sprintf("burst%d.go", i))
			writeGoFile(t, f, i)
			files[i] = f
		}

		// Concurrent reader: samples read latency while the burst runs.
		var readerWg sync.WaitGroup
		stopReader := make(chan struct{})
		readerWg.Add(1)
		go func() {
			defer readerWg.Done()
			for {
				select {
				case <-stopReader:
					return
				default:
					bstore.LoadEdgesForFile(a.ProjectID, 1)
				}
			}
		}()

		// Burst.
		for _, f := range files {
			a.onFileChanged(f)
		}

		// Stop reader before flush so latencies reflect burst-only window.
		close(stopReader)
		readerWg.Wait()

		// Now flush (simulates write tx running AFTER burst window).
		a.doFlushEdgeBatch()

		// Assert reads during burst were sub-ms.
		bstore.latMu.Lock()
		lats := make([]time.Duration, len(bstore.readLatencies))
		copy(lats, bstore.readLatencies)
		bstore.latMu.Unlock()

		// T39: no zero-sample skip — the reader goroutine runs concurrently with the
		// burst; at least one sample is expected.  A skip here means the goroutine
		// was scheduled out before firing even once, which indicates a real problem.
		require.NotEmpty(t, lats,
			"T39: reads_sub_ms_during_burst: no read samples captured during burst window")
		sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
		p99idx := int(float64(len(lats)) * 0.99)
		if p99idx >= len(lats) {
			p99idx = len(lats) - 1
		}
		p99 := lats[p99idx]
		assert.Less(t, p99, time.Millisecond,
			"P99 read latency during burst should be <1ms, got %v (n=%d)", p99, len(lats))
	})

	// T18-c: second burst within same window coalesces (idempotent accumulator).
	// Two back-to-back bursts each produce exactly 1 batch tx when flushed.
	t.Run("two_bursts_two_batches", func(t *testing.T) {
		tmpDir := t.TempDir()
		counter := &countingEdgeStore{}
		a := newBurstTestApp(t, tmpDir, counter)
		a.ArchEnabled = true

		for i := 0; i < 10; i++ {
			f := filepath.Join(tmpDir, fmt.Sprintf("a%d.go", i))
			writeGoFile(t, f, i)
			a.onFileChanged(f)
		}
		a.doFlushEdgeBatch() // window 1

		for i := 0; i < 10; i++ {
			f := filepath.Join(tmpDir, fmt.Sprintf("a%d.go", i)) // same files, modified
			writeGoFile(t, f, i+100)
			a.onFileChanged(f)
		}
		a.doFlushEdgeBatch() // window 2

		assert.Equal(t, int64(2), counter.batchCalls.Load(),
			"two separate flush windows should produce exactly 2 batch calls")
	})

	// T18-d: -race clean (concurrent onFileChanged + doFlushEdgeBatch).
	t.Run("race_clean", func(t *testing.T) {
		tmpDir := t.TempDir()
		counter := &countingEdgeStore{}
		a := newBurstTestApp(t, tmpDir, counter)
		a.ArchEnabled = true

		var wg sync.WaitGroup

		// Writer goroutine: fires file-change events.
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				f := filepath.Join(tmpDir, fmt.Sprintf("race%d.go", i))
				writeGoFile(t, f, i)
				a.onFileChanged(f)
			}
		}()

		// Flush goroutine: triggers batch writes concurrently.
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				a.doFlushEdgeBatch()
				time.Sleep(time.Millisecond)
			}
		}()

		wg.Wait()
		a.doFlushEdgeBatch() // drain any remaining
	})

	// T39-e: real bbolt reads are not blocked during a write tx (C2 / L19.12).
	// Runs concurrent LoadEdgesForFile calls against a real bbolt store while
	// doFlushEdgeBatch holds a write transaction.  bbolt allows concurrent
	// readers during a writer — all reads must complete quickly.
	t.Run("real_bbolt_reads_during_flush", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "edges.db")
		realStore, err := bbolt.NewStore(dbPath)
		require.NoError(t, err)
		t.Cleanup(func() { realStore.Close() })

		// Use the real bbolt store directly as the app store.
		// *bbolt.Store satisfies storeBackend (ports.Storage + ports.EdgeStore +
		// Healthy/Recovered/Close/SaveAllDimensions/LoadAllDimensions).
		a := newBurstTestApp(t, tmpDir, realStore)
		a.ArchEnabled = true

		// Seed initial edges in bbolt so LoadEdgesForFile has data to read.
		for i := 0; i < 5; i++ {
			require.NoError(t, realStore.SaveEdgesForFile("test",
				uint32(i+1),
				[]ports.ImportEdge{{FromFile: fmt.Sprintf("rbolt%d.go", i), ImportPath: "fmt", StartLine: 1}},
			))
		}

		// Create Go files and fire onFileChanged to accumulate a write batch.
		for i := 0; i < 5; i++ {
			f := filepath.Join(tmpDir, fmt.Sprintf("rbolt%d.go", i))
			writeGoFile(t, f, i)
			a.onFileChanged(f)
		}

		// Concurrent reader: fires LoadEdgesForFile calls during the flush.
		var readWg sync.WaitGroup
		latencies := make([]time.Duration, 0, 20)
		var latMu sync.Mutex

		readWg.Add(1)
		go func() {
			defer readWg.Done()
			for i := 0; i < 20; i++ {
				fileID := uint32(i%5 + 1)
				start := time.Now()
				_, _ = realStore.LoadEdgesForFile("test", fileID)
				latMu.Lock()
				latencies = append(latencies, time.Since(start))
				latMu.Unlock()
			}
		}()

		// Flush runs the write tx concurrently with the readers.
		a.doFlushEdgeBatch()
		readWg.Wait()

		// Assert: reads completed — bbolt readers are not blocked by the write tx.
		latMu.Lock()
		lats := make([]time.Duration, len(latencies))
		copy(lats, latencies)
		latMu.Unlock()

		require.NotEmpty(t, lats, "T39: expected read samples during real-bbolt flush")
		sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
		// P99 bound — real bbolt reads should not be blocked by the write tx.
		p99idx := int(float64(len(lats))*0.99)
		if p99idx >= len(lats) {
			p99idx = len(lats) - 1
		}
		assert.Less(t, lats[p99idx], 50*time.Millisecond,
			"T39: real bbolt reads P99 must not be blocked during write tx (got %v)", lats[p99idx])
	})

	// T39-f: AfterFunc timer path — doFlushEdgeBatch fires via the real 200ms
	// time.AfterFunc timer, not called directly from the test.
	// This exercises the timer arm → fire → drain path that the other subtests
	// bypass by calling doFlushEdgeBatch() directly.
	t.Run("timer_path", func(t *testing.T) {
		tmpDir := t.TempDir()
		counter := &countingEdgeStore{}
		a := newBurstTestApp(t, tmpDir, counter)
		a.ArchEnabled = true

		// Create a Go file and fire onFileChanged — this arms the 200ms AfterFunc
		// timer inside markEdgeBatchDirty; doFlushEdgeBatch must NOT be called
		// directly by the test.
		goFile := filepath.Join(tmpDir, "timer.go")
		writeGoFile(t, goFile, 1)
		a.onFileChanged(goFile)

		// Timer is now armed.  Confirm no flush has fired yet.
		assert.Equal(t, int64(0), counter.batchCalls.Load(),
			"T39/timer_path: no SaveEdgesBatch should fire before the 200ms window expires")

		// Wait for the AfterFunc to fire (400ms > 200ms debounce).
		// timerWg.Wait() would deadlock here (the timer goroutine is still live),
		// so we poll with a generous timeout.
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if counter.batchCalls.Load() >= 1 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		assert.Equal(t, int64(1), counter.batchCalls.Load(),
			"T39/timer_path: AfterFunc timer must fire exactly one SaveEdgesBatch call within 2s")
	})
}
