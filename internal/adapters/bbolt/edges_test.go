package bbolt

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

// =============================================================================
// L19.10 EdgeStore — T2 (per-file delete round-trip) + T19 (C3 migration)
// =============================================================================

// makeTestEdges returns a stable set of edges for a single file.
func makeTestEdges(fromFile string) []ports.ImportEdge {
	return []ports.ImportEdge{
		{FromFile: fromFile, ImportPath: "fmt", StartLine: 3},
		{FromFile: fromFile, ImportPath: "os/exec", StartLine: 4},
		{FromFile: fromFile, ImportPath: "github.com/corey/aoa/internal/ports", StartLine: 5},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T2: Per-file delete round-trip
// ─────────────────────────────────────────────────────────────────────────────

func TestEdgeStore_SaveLoadRoundtrip(t *testing.T) {
	// JSON round-trip: LoadEdgesForFile(SaveEdgesForFile(edges)) == edges byte-for-byte.
	store, _ := newTestStore(t)
	edges := makeTestEdges("internal/app/app.go")

	require.NoError(t, store.SaveEdgesForFile("proj-1", 42, edges))

	got, err := store.LoadEdgesForFile("proj-1", 42)
	require.NoError(t, err)
	require.Len(t, got, len(edges))
	for i, e := range edges {
		assert.Equal(t, e.FromFile, got[i].FromFile)
		assert.Equal(t, e.ImportPath, got[i].ImportPath)
		assert.Equal(t, e.StartLine, got[i].StartLine)
	}
}

func TestEdgeStore_T2_PerFileDeleteRoundtrip(t *testing.T) {
	// T2 (16:T2): Save edges for files A(1), B(2), C(3).
	// Delete file B. LoadAllEdges returns A and C only; B absent.
	store, _ := newTestStore(t)

	edgesA := makeTestEdges("pkg/a/a.go")
	edgesB := makeTestEdges("pkg/b/b.go")
	edgesC := makeTestEdges("pkg/c/c.go")

	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, edgesA))
	require.NoError(t, store.SaveEdgesForFile("proj-1", 2, edgesB))
	require.NoError(t, store.SaveEdgesForFile("proj-1", 3, edgesC))

	// Verify all three are present before delete.
	all, err := store.LoadAllEdges("proj-1")
	require.NoError(t, err)
	assert.Len(t, all, len(edgesA)+len(edgesB)+len(edgesC))

	// Delete file B.
	require.NoError(t, store.DeleteEdgesForFile("proj-1", 2))

	// LoadAllEdges must return A and C only.
	remaining, err := store.LoadAllEdges("proj-1")
	require.NoError(t, err)
	assert.Len(t, remaining, len(edgesA)+len(edgesC), "after delete, only A and C should remain")

	// B must be absent.
	fromFiles := make(map[string]bool)
	for _, e := range remaining {
		fromFiles[e.FromFile] = true
	}
	assert.False(t, fromFiles["pkg/b/b.go"], "file B edges must be absent after delete")
	assert.True(t, fromFiles["pkg/a/a.go"], "file A edges must remain")
	assert.True(t, fromFiles["pkg/c/c.go"], "file C edges must remain")

	// Direct LoadEdgesForFile(B) must return empty.
	bEdges, err := store.LoadEdgesForFile("proj-1", 2)
	require.NoError(t, err)
	assert.Empty(t, bEdges, "LoadEdgesForFile for deleted file must be empty")
}

func TestEdgeStore_DeleteIdempotent(t *testing.T) {
	// Deleting a nonexistent file's edges is not an error.
	store, _ := newTestStore(t)

	// Delete before any save — idempotent.
	err := store.DeleteEdgesForFile("proj-1", 999)
	assert.NoError(t, err)

	// Delete nonexistent project.
	err = store.DeleteEdgesForFile("proj-empty", 1)
	assert.NoError(t, err)
}

func TestEdgeStore_LoadMissing(t *testing.T) {
	// LoadEdgesForFile for a nonexistent project or file returns nil, nil.
	store, _ := newTestStore(t)

	// No project at all.
	got, err := store.LoadEdgesForFile("proj-empty", 1)
	require.NoError(t, err)
	assert.Nil(t, got)

	// Project exists, file absent.
	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, makeTestEdges("a.go")))
	got, err = store.LoadEdgesForFile("proj-1", 999)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestEdgeStore_LoadAllEdges_Empty(t *testing.T) {
	// LoadAllEdges returns nil when no edges exist.
	store, _ := newTestStore(t)

	all, err := store.LoadAllEdges("proj-empty")
	require.NoError(t, err)
	assert.Nil(t, all)
}

func TestEdgeStore_OverwriteFile(t *testing.T) {
	// Saving edges for the same fileID twice overwrites the prior entry.
	store, _ := newTestStore(t)

	original := makeTestEdges("handler.go")
	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, original))

	updated := []ports.ImportEdge{
		{FromFile: "handler.go", ImportPath: "net/http", StartLine: 2},
	}
	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, updated))

	got, err := store.LoadEdgesForFile("proj-1", 1)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "net/http", got[0].ImportPath)
}

func TestEdgeStore_ProjectScoped(t *testing.T) {
	// Edges are per-project: proj-A's data is invisible to proj-B.
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveEdgesForFile("proj-A", 1, makeTestEdges("a.go")))
	require.NoError(t, store.SaveEdgesForFile("proj-B", 1, makeTestEdges("b.go")))

	allA, err := store.LoadAllEdges("proj-A")
	require.NoError(t, err)
	for _, e := range allA {
		assert.Equal(t, "a.go", e.FromFile)
	}

	allB, err := store.LoadAllEdges("proj-B")
	require.NoError(t, err)
	for _, e := range allB {
		assert.Equal(t, "b.go", e.FromFile)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T19: C3 schema migration — both-direction tolerance
// ─────────────────────────────────────────────────────────────────────────────

func TestEdgeStore_T19_NewBinaryOldDB_NoBucket(t *testing.T) {
	// T19 new-binary/old-DB: DB without an edges bucket.
	// LoadEdgesForFile → empty result, no error, no panic.
	// SaveEdgesForFile → bucket created silently.
	// After save, LoadEdgesForFile returns the saved data.
	store, _ := newTestStore(t)

	// No edges bucket exists yet — Load must return empty.
	got, err := store.LoadEdgesForFile("proj-1", 1)
	require.NoError(t, err, "load on fresh DB must not error")
	assert.Nil(t, got, "load on fresh DB must return nil")

	// Save creates the bucket silently.
	edges := makeTestEdges("main.go")
	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, edges))

	// Now load should return the saved data.
	loaded, err := store.LoadEdgesForFile("proj-1", 1)
	require.NoError(t, err)
	require.Len(t, loaded, len(edges))
}

func TestEdgeStore_T19_VersionByteWrittenOnCreate(t *testing.T) {
	// T19: _version byte is written on bucket creation.
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, makeTestEdges("a.go")))

	// Verify _version is present and correct.
	err := store.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte("proj-1"))
		require.NotNil(t, proj)
		eb := proj.Bucket(bucketEdges)
		require.NotNil(t, eb, "edges bucket must exist after save")
		v := eb.Get(keyVersion)
		require.NotNil(t, v, "_version must be set")
		assert.Equal(t, []byte{edgesVersion}, v, "_version must equal edgesVersion")
		return nil
	})
	require.NoError(t, err)
}

func TestEdgeStore_T19_VersionMismatch_DropAndRecreate(t *testing.T) {
	// T19 version mismatch: write a bucket with a different version byte.
	// The next SaveEdgesForFile must drop and re-create the bucket.
	// Old data (from wrong-version bucket) must not appear after recreate.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	require.NoError(t, err)

	// Manually write an edges bucket with version byte = 0xFF (unknown).
	badEdges := []ports.ImportEdge{{FromFile: "old.go", ImportPath: "old_pkg", StartLine: 1}}
	badData, _ := json.Marshal(badEdges)
	err = db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte("proj-1"))
		if err != nil {
			return err
		}
		eb, err := proj.CreateBucketIfNotExists(bucketEdges)
		if err != nil {
			return err
		}
		// Write wrong version.
		if err := eb.Put(keyVersion, []byte{0xFF}); err != nil {
			return err
		}
		// Write a file's edges under the wrong version.
		key := fileIDKey(1)
		return eb.Put(key, badData)
	})
	require.NoError(t, err)
	db.Close()

	// Open via our Store.
	store, err := NewStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	// Save new edges — this must detect the version mismatch and drop+recreate.
	newEdges := makeTestEdges("new.go")
	require.NoError(t, store.SaveEdgesForFile("proj-1", 2, newEdges))

	// The old wrong-version file (fileID=1) must NOT appear.
	old, err := store.LoadEdgesForFile("proj-1", 1)
	require.NoError(t, err)
	assert.Empty(t, old, "old wrong-version data must be gone after bucket drop")

	// The new write must be readable.
	got, err := store.LoadEdgesForFile("proj-1", 2)
	require.NoError(t, err)
	require.Len(t, got, len(newEdges))
	assert.Equal(t, newEdges[0].ImportPath, got[0].ImportPath)

	// Version byte must now be correct.
	err = store.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte("proj-1"))
		require.NotNil(t, proj)
		eb := proj.Bucket(bucketEdges)
		require.NotNil(t, eb)
		v := eb.Get(keyVersion)
		assert.Equal(t, []byte{edgesVersion}, v)
		return nil
	})
	require.NoError(t, err)
}

func TestEdgeStore_T19_OldBinaryNewDB_IndexNotCorrupted(t *testing.T) {
	// T19 old-binary/new-DB: a DB with an edges bucket must not corrupt
	// the index bucket. An "old binary" (no EdgeStore code) would simply
	// call LoadIndex; here we verify that works after edges are written.
	//
	// This proves: the presence of an `edges` bucket NEVER triggers index
	// self-recovery and NEVER corrupts existing index data.
	store, _ := newTestStore(t)

	// Write an index.
	idx := makeTestIndex()
	require.NoError(t, store.SaveIndex("proj-1", idx))

	// Write edges (simulates a new-binary write that an old binary might see).
	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, makeTestEdges("a.go")))

	// An "old binary" would call LoadIndex — it must still work.
	loaded, err := store.LoadIndex("proj-1")
	require.NoError(t, err)
	require.NotNil(t, loaded, "index must not be nil — edges bucket must not corrupt index")
	assert.Equal(t, len(idx.Files), len(loaded.Files), "index data must be intact")
	assert.Equal(t, len(idx.Tokens), len(loaded.Tokens))
	assert.False(t, store.Recovered(), "edges bucket must NOT trigger index self-recovery")
}

func TestEdgeStore_T19_SaveEdgesForFile_NeverUnderLock(t *testing.T) {
	// C1 structural test: SaveEdgesForFile must exist, accept real edges, and
	// round-trip correctly.  The lock-order invariant (must not be called while
	// App.mu is held) is enforced by the T17 lockGuardStore harness in the app
	// package; here we assert the method is correct at the storage layer.
	store, _ := newTestStore(t)

	edges := makeTestEdges("cmd/aoa/main.go")
	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, edges))

	got, err := store.LoadEdgesForFile("proj-1", 1)
	require.NoError(t, err)
	require.Len(t, got, len(edges), "saved edges must be loadable")
	assert.Equal(t, edges[0].ImportPath, got[0].ImportPath, "ImportPath must round-trip")
	assert.Equal(t, edges[0].StartLine, got[0].StartLine, "StartLine must round-trip (G7)")

	// nil save must not error (idempotent clear).
	assert.NoError(t, store.SaveEdgesForFile("proj-1", 2, nil))
}

func TestEdgeStore_EmptyEdgeSlice(t *testing.T) {
	// Saving an empty slice is valid (clears edges for a file without deleting the key).
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, []ports.ImportEdge{}))
	got, err := store.LoadEdgesForFile("proj-1", 1)
	require.NoError(t, err)
	// Empty slice round-trips — may return nil or empty slice.
	assert.True(t, len(got) == 0, "empty edge slice must round-trip as empty")
}

// ─────────────────────────────────────────────────────────────────────────────
// L19.12 SaveEdgesBatch — C2 burst coalescing
// ─────────────────────────────────────────────────────────────────────────────

func TestEdgeStore_SaveEdgesBatch_SaveAndLoad(t *testing.T) {
	// Batch saves three files in a single write tx; each must be loadable afterwards.
	store, _ := newTestStore(t)

	batch := map[uint32][]ImportEdge{
		1: makeTestEdges("internal/app/app.go"),
		2: makeTestEdges("internal/ports/facts.go"),
		3: makeTestEdges("cmd/aoa/main.go"),
	}
	require.NoError(t, store.SaveEdgesBatch("proj-1", batch))

	for fileID, want := range batch {
		got, err := store.LoadEdgesForFile("proj-1", fileID)
		require.NoError(t, err, "LoadEdgesForFile(%d)", fileID)
		require.Len(t, got, len(want), "fileID=%d edge count", fileID)
		for i, e := range want {
			assert.Equal(t, e.ImportPath, got[i].ImportPath, "fileID=%d edge[%d] ImportPath", fileID, i)
			assert.Equal(t, e.FromFile, got[i].FromFile, "fileID=%d edge[%d] FromFile (G7 provenance)", fileID, i)
			assert.Equal(t, e.StartLine, got[i].StartLine, "fileID=%d edge[%d] StartLine (G7 provenance)", fileID, i)
		}
	}
}

func TestEdgeStore_SaveEdgesBatch_DeleteViaEmptySlice(t *testing.T) {
	// Both empty-slice and nil-slice entries signal "delete" — the key must be
	// absent after flush. nil is what markEdgeBatchDirty enqueues for deleted files;
	// []ImportEdge{} is the other valid zero-length form. Both must be exercised.
	store, _ := newTestStore(t)

	// Seed files 10 and 12 with real edges.
	require.NoError(t, store.SaveEdgesForFile("proj-1", 10, makeTestEdges("pkg/foo.go")))
	require.NoError(t, store.SaveEdgesForFile("proj-1", 12, makeTestEdges("pkg/baz.go")))

	// Batch: file 10 → empty slice (delete), file 11 → new edges (save),
	//        file 12 → nil (delete, mirrors markEdgeBatchDirty delete path).
	batch := map[uint32][]ImportEdge{
		10: {},                                    // delete via empty slice
		11: makeTestEdges("pkg/bar.go"),            // save
		12: nil,                                   // delete via nil (production delete value)
	}
	require.NoError(t, store.SaveEdgesBatch("proj-1", batch))

	// File 10 must be gone (empty-slice delete).
	del, err := store.LoadEdgesForFile("proj-1", 10)
	require.NoError(t, err)
	assert.Len(t, del, 0, "file 10 must have 0 edges after batch-delete via empty slice")

	// File 11 must be present.
	saved, err := store.LoadEdgesForFile("proj-1", 11)
	require.NoError(t, err)
	assert.Len(t, saved, len(batch[11]), "file 11 must have edges after batch-save")

	// File 12 must be gone (nil-slice delete — production delete value).
	nilDel, err := store.LoadEdgesForFile("proj-1", 12)
	require.NoError(t, err)
	assert.Len(t, nilDel, 0, "file 12 must have 0 edges after batch-delete via nil")
}

func TestEdgeStore_SaveEdgesBatch_Atomic(t *testing.T) {
	// SaveEdgesBatch is all-or-nothing: an error during the tx rolls back all writes.
	// We verify the positive case (no error → all written atomically).
	store, _ := newTestStore(t)

	batch := map[uint32][]ImportEdge{
		20: makeTestEdges("pkg/a.go"),
		21: makeTestEdges("pkg/b.go"),
		22: makeTestEdges("pkg/c.go"),
	}
	require.NoError(t, store.SaveEdgesBatch("proj-1", batch))

	all, err := store.LoadAllEdges("proj-1")
	require.NoError(t, err)
	assert.Len(t, all, 3*len(makeTestEdges("")), "all 3 files × 3 edges = 9 total edges")
}

func TestEdgeStore_SaveEdgesBatch_Empty(t *testing.T) {
	// SaveEdgesBatch with an empty map is a no-op — no error, no writes.
	store, _ := newTestStore(t)
	assert.NoError(t, store.SaveEdgesBatch("proj-1", nil))
	assert.NoError(t, store.SaveEdgesBatch("proj-1", map[uint32][]ImportEdge{}))
}

// ─────────────────────────────────────────────────────────────────────────────
// T38: _version enforced on the READ path (finding 12)
// ─────────────────────────────────────────────────────────────────────────────

func TestEdgeStore_T38_VersionCheck_ReadPath(t *testing.T) {
	// T38: LoadEdgesForFile and LoadAllEdges must return empty results (not corrupt
	// data) when the edges bucket carries a wrong _version byte.  Before this fix,
	// openEdgesBucket returned the bucket regardless of version — a read-before-write
	// scenario (new binary opening a DB with a future-version edges bucket) could
	// silently return JSON-unmarshal failures or zero-valued edges.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	require.NoError(t, err)

	// Write a real edge under a wrong version byte (simulates future-format bucket).
	goodEdge := []ImportEdge{{FromFile: "real.go", ImportPath: "fmt", StartLine: 1}}
	data, _ := json.Marshal(goodEdge)
	err = db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte("proj-1"))
		if err != nil {
			return err
		}
		eb, err := proj.CreateBucketIfNotExists(bucketEdges)
		if err != nil {
			return err
		}
		if err := eb.Put(keyVersion, []byte{0xFF}); err != nil { // future version
			return err
		}
		return eb.Put(fileIDKey(1), data)
	})
	require.NoError(t, err)
	db.Close()

	store, err := NewStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	// LoadEdgesForFile must return nil, nil (wrong version → treat as empty).
	got, err := store.LoadEdgesForFile("proj-1", 1)
	require.NoError(t, err, "wrong-version bucket must not error on load")
	assert.Nil(t, got, "wrong-version bucket: LoadEdgesForFile must return nil (T38)")

	// LoadAllEdges must also return nil (wrong version → no rows visible).
	all, err := store.LoadAllEdges("proj-1")
	require.NoError(t, err, "wrong-version bucket must not error on LoadAllEdges")
	assert.Nil(t, all, "wrong-version bucket: LoadAllEdges must return nil (T38)")
}

// ─────────────────────────────────────────────────────────────────────────────
// T38 (surfacing half — PC4): corrupt JSON within a correctly-versioned bucket
// must be surfaced, not silently fed as zero-values to F2's input (G7).
// ─────────────────────────────────────────────────────────────────────────────

func TestEdgeStore_T38_CorruptEntry_Surfaced(t *testing.T) {
	// Write corrupt (non-JSON) bytes into the edges bucket under the CORRECT
	// version so openEdgesBucket returns the bucket — the corrupt bytes are then
	// hit by json.Unmarshal inside LoadEdgesForFile / LoadAllEdges.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	require.NoError(t, err)

	// Write one valid edge (fileID 1) and one corrupt entry (fileID 2).
	goodEdge := []ImportEdge{{FromFile: "good.go", ImportPath: "fmt", StartLine: 1}}
	goodData, _ := json.Marshal(goodEdge)

	err = db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte("proj-1"))
		if err != nil {
			return err
		}
		eb, err := proj.CreateBucketIfNotExists(bucketEdges)
		if err != nil {
			return err
		}
		// Correct version so openEdgesBucket lets us in.
		if err := eb.Put(keyVersion, []byte{edgesVersion}); err != nil {
			return err
		}
		// fileID 1: valid JSON.
		if err := eb.Put(fileIDKey(1), goodData); err != nil {
			return err
		}
		// fileID 2: corrupt bytes — json.Unmarshal will fail.
		return eb.Put(fileIDKey(2), []byte("NOT JSON AT ALL {{{"))
	})
	require.NoError(t, err)
	db.Close()

	store, err := NewStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	t.Run("LoadEdgesForFile_corrupt_returns_error", func(t *testing.T) {
		// A corrupt row must surface an error — not silently return nil, nil.
		got, err := store.LoadEdgesForFile("proj-1", 2)
		assert.Error(t, err, "T38: corrupt edge entry must surface an error (not silent nil)")
		assert.Nil(t, got, "T38: corrupt entry must yield nil edges")
	})

	t.Run("LoadEdgesForFile_valid_still_works", func(t *testing.T) {
		// A valid row adjacent to a corrupt one must still be readable.
		got, err := store.LoadEdgesForFile("proj-1", 1)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, goodEdge, got, "T38: valid row must be returned correctly alongside corrupt sibling")
	})

	t.Run("LoadAllEdges_corrupt_skip_with_count", func(t *testing.T) {
		// LoadAllEdges must continue past the corrupt row, return valid edges,
		// AND surface a non-nil error carrying the skip count.
		all, err := store.LoadAllEdges("proj-1")
		assert.Error(t, err, "T38: LoadAllEdges must return non-nil error when entries are skipped")
		assert.Contains(t, err.Error(), "skipped", "T38: error must mention the skip count")
		// The valid row (fileID 1) must still be included.
		require.NotNil(t, all, "T38: valid edges must be returned even when some rows are corrupt")
		assert.Len(t, all, len(goodEdge),
			"T38: exactly the valid edge entries must be present; corrupt row must not contribute")
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// ReplaceAllEdges — P3 atomic bulk write + stale-row elimination (T34)
// ─────────────────────────────────────────────────────────────────────────────

func TestEdgeStore_ReplaceAllEdges_ClearsStale(t *testing.T) {
	// Seed two files; ReplaceAllEdges with only file 2 must leave file 1 absent
	// (stale-row elimination — finding 9 / T34).
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, makeTestEdges("old.go")))
	require.NoError(t, store.SaveEdgesForFile("proj-1", 2, makeTestEdges("keep.go")))

	// Replace with only file 2 — file 1 is a stale row from the "previous build".
	fresh := map[uint32][]ImportEdge{
		2: makeTestEdges("keep.go"),
	}
	require.NoError(t, store.ReplaceAllEdges("proj-1", fresh))

	// File 1 must be gone.
	stale, err := store.LoadEdgesForFile("proj-1", 1)
	require.NoError(t, err)
	assert.Nil(t, stale, "ReplaceAllEdges must eliminate stale file-1 row (T34)")

	// File 2 must still be present.
	kept, err := store.LoadEdgesForFile("proj-1", 2)
	require.NoError(t, err)
	assert.Len(t, kept, len(makeTestEdges("keep.go")), "ReplaceAllEdges must retain file-2 row")
}

func TestEdgeStore_ReplaceAllEdges_EmptyClears(t *testing.T) {
	// ReplaceAllEdges with nil/empty fileEdges clears all existing rows.
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveEdgesForFile("proj-1", 1, makeTestEdges("a.go")))
	require.NoError(t, store.SaveEdgesForFile("proj-1", 2, makeTestEdges("b.go")))

	// Replace with nil → clear only.
	require.NoError(t, store.ReplaceAllEdges("proj-1", nil))

	all, err := store.LoadAllEdges("proj-1")
	require.NoError(t, err)
	assert.Nil(t, all, "ReplaceAllEdges(nil) must clear all rows")
}

func TestEdgeStore_ReplaceAllEdges_Atomic(t *testing.T) {
	// After ReplaceAllEdges the _version byte must be present and correct:
	// ensureEdgesBucket is called on the fresh bucket inside the same tx.
	store, _ := newTestStore(t)

	batch := map[uint32][]ImportEdge{
		10: makeTestEdges("internal/app/app.go"),
		11: makeTestEdges("internal/ports/facts.go"),
	}
	require.NoError(t, store.ReplaceAllEdges("proj-1", batch))

	// Verify _version is set correctly on the fresh bucket.
	err := store.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte("proj-1"))
		require.NotNil(t, proj)
		eb := proj.Bucket(bucketEdges)
		require.NotNil(t, eb, "edges bucket must exist after ReplaceAllEdges")
		v := eb.Get(keyVersion)
		assert.Equal(t, []byte{edgesVersion}, v, "_version must be correct after replace")
		return nil
	})
	require.NoError(t, err)

	// Both files must be loadable.
	for fileID, want := range batch {
		got, err := store.LoadEdgesForFile("proj-1", fileID)
		require.NoError(t, err, "LoadEdgesForFile(%d)", fileID)
		assert.Len(t, got, len(want), "fileID=%d edge count after ReplaceAllEdges", fileID)
	}
}

func TestEdgeStore_ReplaceAllEdges_ClearsUnresolved(t *testing.T) {
	// T42: ReplaceAllEdges must drop the facts_unresolved bucket atomically with
	// the edges bucket — stale broken-import records from deleted files must not
	// survive across Reindex cycles (finding R8 / checkpoint-F1 PC1).
	store, _ := newTestStore(t)

	// Seed some unresolved entries (simulating a previous Reindex run that found
	// broken imports for a file that has since been deleted).
	unresolved := []ImportEdge{
		{FromFile: "pkg/deleted.go", ImportPath: "./nonexistent", StartLine: 5},
		{FromFile: "pkg/deleted.go", ImportPath: "./also-gone", StartLine: 10},
	}
	require.NoError(t, store.SaveUnresolved("proj-1", unresolved))

	// Verify the entries are present before the replace.
	err := store.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte("proj-1"))
		require.NotNil(t, proj)
		ub := proj.Bucket(bucketFactsUnresolved)
		require.NotNil(t, ub, "facts_unresolved bucket must exist after SaveUnresolved")
		return nil
	})
	require.NoError(t, err)

	// ReplaceAllEdges with a fresh (non-empty) edge set.
	fresh := map[uint32][]ImportEdge{
		1: makeTestEdges("pkg/new.go"),
	}
	require.NoError(t, store.ReplaceAllEdges("proj-1", fresh))

	// T42 core assertion: facts_unresolved bucket must be absent (dropped atomically).
	err = store.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte("proj-1"))
		require.NotNil(t, proj)
		ub := proj.Bucket(bucketFactsUnresolved)
		assert.Nil(t, ub, "T42: ReplaceAllEdges must clear the facts_unresolved bucket to prevent stale accumulation")
		return nil
	})
	require.NoError(t, err)

	// The new edge data must be present.
	got, err := store.LoadEdgesForFile("proj-1", 1)
	require.NoError(t, err)
	assert.Len(t, got, len(fresh[1]), "fresh edges must be readable after ReplaceAllEdges")
}
