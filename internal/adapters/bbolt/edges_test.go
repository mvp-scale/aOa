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
	// Compile-time structural check: SaveEdgesForFile signature is correct.
	// Runtime check: it compiles and is callable.
	store, _ := newTestStore(t)

	// Just verifying the method exists and works — C1 enforcement is in T17 harness.
	err := store.SaveEdgesForFile("proj-1", 1, nil) // nil = empty edge list
	assert.NoError(t, err)
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
			assert.Equal(t, e.ImportPath, got[i].ImportPath, "fileID=%d edge[%d]", fileID, i)
		}
	}
}

func TestEdgeStore_SaveEdgesBatch_DeleteViaEmptySlice(t *testing.T) {
	// An empty-slice entry in the batch signals "delete" — the key must be absent after flush.
	store, _ := newTestStore(t)

	// Seed file 10 with real edges.
	require.NoError(t, store.SaveEdgesForFile("proj-1", 10, makeTestEdges("pkg/foo.go")))

	// Batch: file 10 → empty (delete), file 11 → new edges (save).
	batch := map[uint32][]ImportEdge{
		10: {},                                    // delete
		11: makeTestEdges("pkg/bar.go"),            // save
	}
	require.NoError(t, store.SaveEdgesBatch("proj-1", batch))

	// File 10 must be gone.
	del, err := store.LoadEdgesForFile("proj-1", 10)
	require.NoError(t, err)
	assert.Len(t, del, 0, "file 10 must have 0 edges after batch-delete")

	// File 11 must be present.
	saved, err := store.LoadEdgesForFile("proj-1", 11)
	require.NoError(t, err)
	assert.Len(t, saved, len(batch[11]), "file 11 must have edges after batch-save")
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
