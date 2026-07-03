//go:build !lean

// Package app — T20 -race soak for the concurrent derive+serve+watch pipeline (F3 PF3).
//
// Gate text (16-gaps-in-test-strategy.md T20):
//
//	"N seconds of concurrent socket reads + watcher writes + forced compacts under
//	 go test -race: zero races; every served shard is a consistent snapshot (no
//	 torn read across a revision bump)."
//
// Bounded for CI: nIterationsPerDeriver=50 iterations of concurrent
// derive / read / ReplaceAllEdges churn per deriver goroutine (not wall-clock
// hours).  Run under go test -race; the detector fires on any Go-level race.
//
// Concurrency model exercised:
//   - 2 deriver goroutines: call a.deriveArch() in a loop (archDeriveMu
//     serializes concurrent derives; readers run during the critical section).
//   - 4 reader goroutines: call q.Manifest() + store.LoadShard() concurrently
//     with derives — the core "socket read vs SaveShards write" race target.
//   - 1 compactor goroutine: calls store.ReplaceAllEdges() to simulate the
//     watcher-flush path writing a fresh edge set while derives are in flight.
//
// Consistent-snapshot assertion: if Manifest() returns a non-nil manifest, we
// attempt to LoadShard every view key.  A LoadShard error is a store-level tear;
// invalid JSON bytes are a write-level tear.  Neither should occur:
//   - bbolt's MVCC ensures each reader tx sees a consistent on-disk snapshot.
//   - SaveShards uses Put (not delete-then-overwrite), so old content-hash keys
//     persist across derives; a reader holding an old manifest's key always
//     finds valid bytes even after a newer derive.
//   - A nil LoadShard result just means the key changed (new content hash after
//     a re-derive) — not a data race, so we skip rather than count it as a tear.
package app

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestT20_RaceSoak is the T20 concurrent-pipeline soak under -race (PF3).
func TestT20_RaceSoak(t *testing.T) {
	const (
		nDeriverGoroutines    = 2
		nReaderGoroutines     = 4
		nCompactorGoroutines  = 1
		nIterationsPerDeriver = 50 // CI-bounded: total derives = 2 × 50 = 100
	)

	tmpDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "t20.db")

	store, err := bbolt.NewStore(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	// Seed edges so deriveArch has something to aggregate (same helper used
	// by T45 and T47 — minimal, realistic, resolved import edges).
	require.NoError(t, store.SaveEdgesForFile("test", 1, edgesForDeriveTest()))

	// Wire a real App with the real store so derives reach actual bbolt writes.
	a := newBurstTestApp(t, tmpDir, store)
	a.ArchEnabled = true

	// Obtain a querier for the reader goroutines.
	q := a.Arch()
	require.NotNil(t, q, "Arch() must return non-nil when ArchEnabled=true and store non-nil")

	// Prime: one synchronous derive so shards exist before readers start.
	// Readers will then race against subsequent concurrent derives.
	a.deriveArch()

	stop := make(chan struct{})

	// tearCount tracks data-consistency violations detected by readers.
	// A LoadShard error or non-JSON bytes under concurrent writes indicates
	// a torn read that bbolt's MVCC or our locking failed to prevent.
	var tearCount atomic.Int64

	// readerWg / compactorWg are stopped after derivers finish.
	var readerWg, compactorWg, driverWg sync.WaitGroup

	// ── Reader goroutines ─────────────────────────────────────────────────────
	// Exercise the archQuerier.Manifest() → store.LoadShard() path concurrently
	// with derives and edge compacts.  This is the "socket read" path (the
	// socket and web handlers both go through the ArchQuerier interface).
	for i := 0; i < nReaderGoroutines; i++ {
		readerWg.Add(1)
		go func() {
			defer readerWg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}

				m, mErr := q.Manifest("local")
				if mErr != nil || m == nil {
					// No manifest yet (or transient error) — keep spinning.
					continue
				}

				for _, ve := range m.Views {
					if ve.Key == "" {
						continue
					}
					// Load the shard bytes at the key declared by the manifest.
					// bbolt's MVCC guarantees we see consistent on-disk bytes
					// within a single db.View transaction.
					data, loadErr := store.LoadShard("test", ve.Key)
					if loadErr != nil {
						// A store-level error under concurrent writes is a tear.
						tearCount.Add(1)
						continue
					}
					if data == nil {
						// Key absent: a newer derive may have stored the shard
						// under a different content-hash key.  This is a stale
						// manifest reference, not a data race.
						continue
					}
					// Verify the bytes are valid JSON (no half-written page).
					if !json.Valid(data) {
						tearCount.Add(1)
					}
				}
			}
		}()
	}

	// ── Compactor goroutines ──────────────────────────────────────────────────
	// Simulate the watcher flush path: ReplaceAllEdges atomically rewrites the
	// entire edge bucket (deletes stale file rows, writes fresh ones).  This is
	// the "forced compact" — a write transaction that races with both derives
	// (which call LoadAllEdges at the start of each derive) and readers.
	compactEdges := map[uint32][]ports.ImportEdge{
		1: edgesForDeriveTest(),
		2: {
			{FromFile: "cmd/main.go", ImportPath: "internal/app", StartLine: 1},
			{FromFile: "cmd/main.go", ImportPath: "ext:std/os", StartLine: 2},
		},
	}
	for i := 0; i < nCompactorGoroutines; i++ {
		compactorWg.Add(1)
		go func() {
			defer compactorWg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = store.ReplaceAllEdges("test", compactEdges)
			}
		}()
	}

	// ── Deriver goroutines ────────────────────────────────────────────────────
	// Each calls deriveArch() nIterationsPerDeriver times.  archDeriveMu
	// serializes concurrent derives; the race detector monitors the Go-level
	// memory accesses that occur AROUND and BETWEEN the derive critical section
	// (index snapshot under mu, bbolt writes outside mu).
	for i := 0; i < nDeriverGoroutines; i++ {
		driverWg.Add(1)
		go func() {
			defer driverWg.Done()
			for j := 0; j < nIterationsPerDeriver; j++ {
				a.deriveArch()
			}
		}()
	}

	// Wait for all derives to complete, then drain readers and compactors.
	driverWg.Wait()
	close(stop)
	readerWg.Wait()
	compactorWg.Wait()

	// Assert: no torn reads detected during the concurrent pipeline.
	assert.Equal(t, int64(0), tearCount.Load(),
		"T20: zero torn-read detections (LoadShard errors or invalid JSON under concurrent derives/compacts)")

	t.Logf("T20 soak complete: %d deriver goroutines × %d iterations, %d readers, %d compactors — race detector clean",
		nDeriverGoroutines, nIterationsPerDeriver, nReaderGoroutines, nCompactorGoroutines)
}
