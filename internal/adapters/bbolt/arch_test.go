package bbolt

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

// =============================================================================
// L19.14 ArchStore — T19 extension: C3 schema migration for arch_shards
// =============================================================================

// makeTestShards returns a small set of shard blobs for testing.
func makeTestShards() map[string][]byte {
	return map[string][]byte{
		"local/component@abc123def456": []byte(`{"kind":"buckets","title":"Component diagram"}`),
		"local/dsm@111222333444":       []byte(`{"kind":"matrix","title":"DSM"}`),
		"local/cycles@aabbccdd1122":    []byte(`{"kind":"table","title":"Cycles"}`),
	}
}

// makeTestManifest returns a small manifest JSON for testing.
func makeTestManifest() []byte {
	return []byte(`{"scope":"local","rev":"fb92a2ed527a","views":[]}`)
}

// ─────────────────────────────────────────────────────────────────────────────
// Basic round-trip: SaveShards + LoadShard
// ─────────────────────────────────────────────────────────────────────────────

func TestArchStore_SaveLoadShard_RoundTrip(t *testing.T) {
	store, _ := newTestStore(t)
	shards := makeTestShards()

	require.NoError(t, store.SaveShards("proj-arch", shards))

	for key, wantData := range shards {
		got, err := store.LoadShard("proj-arch", key)
		require.NoError(t, err, "LoadShard(%q)", key)
		require.NotNil(t, got, "shard %q must be retrievable", key)
		assert.Equal(t, string(wantData), string(got), "shard %q round-trip", key)
	}
}

func TestArchStore_LoadShard_Missing(t *testing.T) {
	store, _ := newTestStore(t)

	got, err := store.LoadShard("proj-arch", "local/nonexistent@000000000000")
	require.NoError(t, err, "missing shard must not error")
	assert.Nil(t, got, "missing shard must return nil")
}

func TestArchStore_SaveLoad_Manifest(t *testing.T) {
	store, _ := newTestStore(t)
	manifest := makeTestManifest()

	require.NoError(t, store.SaveManifest("proj-arch", "local", manifest))

	got, err := store.LoadManifest("proj-arch", "local")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, string(manifest), string(got))
}

func TestArchStore_LoadManifest_Missing(t *testing.T) {
	store, _ := newTestStore(t)

	got, err := store.LoadManifest("proj-arch", "local")
	require.NoError(t, err, "missing manifest must not error")
	assert.Nil(t, got, "missing manifest must return nil")
}

func TestArchStore_DeleteShardsForScope(t *testing.T) {
	store, _ := newTestStore(t)
	shards := makeTestShards()
	manifest := makeTestManifest()

	require.NoError(t, store.SaveShards("proj-arch", shards))
	require.NoError(t, store.SaveManifest("proj-arch", "local", manifest))

	// Delete the scope.
	require.NoError(t, store.DeleteShardsForScope("proj-arch", "local"))

	// All shards must be gone.
	for key := range shards {
		got, err := store.LoadShard("proj-arch", key)
		require.NoError(t, err)
		assert.Nil(t, got, "shard %q must be deleted", key)
	}

	// Manifest must be gone too.
	m, err := store.LoadManifest("proj-arch", "local")
	require.NoError(t, err)
	assert.Nil(t, m, "manifest must be deleted")
}

func TestArchStore_DeleteShardsForScope_Idempotent(t *testing.T) {
	store, _ := newTestStore(t)
	// Deleting from an empty bucket must not error.
	assert.NoError(t, store.DeleteShardsForScope("proj-arch", "local"))
	assert.NoError(t, store.DeleteShardsForScope("proj-arch", "local"))
}

// ─────────────────────────────────────────────────────────────────────────────
// HasArchBucket
// ─────────────────────────────────────────────────────────────────────────────

func TestArchStore_HasArchBucket_FalseWhenAbsent(t *testing.T) {
	store, _ := newTestStore(t)
	assert.False(t, store.HasArchBucket("proj-arch"),
		"fresh DB must not report arch bucket present")
}

func TestArchStore_HasArchBucket_TrueAfterSave(t *testing.T) {
	store, _ := newTestStore(t)
	require.NoError(t, store.SaveShards("proj-arch", makeTestShards()))
	assert.True(t, store.HasArchBucket("proj-arch"),
		"arch bucket must be present after SaveShards")
}

// ─────────────────────────────────────────────────────────────────────────────
// T19 C3 migration — arch_shards bucket
// ─────────────────────────────────────────────────────────────────────────────

func TestArchStore_T19_VersionByteWrittenOnCreate(t *testing.T) {
	// T19: _version byte is written on bucket creation.
	// Uses store.db.View (same connection) to avoid the bbolt one-writer constraint.
	store, _ := newTestStore(t)
	require.NoError(t, store.SaveShards("proj-arch", makeTestShards()))

	err := store.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte("proj-arch"))
		require.NotNil(t, proj, "project bucket must exist")

		ab := proj.Bucket(bucketArchShards)
		require.NotNil(t, ab, "arch_shards bucket must exist")

		v := ab.Get(keyVersion)
		require.NotNil(t, v, "_version key must be present")
		assert.Equal(t, []byte{archShardsVersion}, v, "_version must equal archShardsVersion")
		return nil
	})
	require.NoError(t, err)
}

func TestArchStore_T19_VersionMismatch_DropAndRecreate(t *testing.T) {
	// T19 version mismatch: write a bucket with a different version byte via raw bolt,
	// then open it via Store. SaveShards must detect the mismatch and drop+recreate.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	// Pre-populate a wrong-version arch_shards bucket.
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	require.NoError(t, err)
	err = db.Update(func(tx *bolt.Tx) error {
		proj, _ := tx.CreateBucketIfNotExists([]byte("proj-arch"))
		ab, _ := proj.CreateBucketIfNotExists(bucketArchShards)
		if err := ab.Put(keyVersion, []byte{0xFF}); err != nil { // wrong version
			return err
		}
		return ab.Put([]byte("stale-key"), []byte("stale-data"))
	})
	require.NoError(t, err)
	db.Close()

	// Open via Store.
	store, err := NewStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	// SaveShards must detect version mismatch and drop+recreate.
	freshShards := map[string][]byte{"local/component@newkey": []byte(`{"kind":"buckets"}`)}
	require.NoError(t, store.SaveShards("proj-arch", freshShards))

	// Old stale data must not appear.
	got, err := store.LoadShard("proj-arch", "stale-key")
	require.NoError(t, err)
	assert.Nil(t, got, "stale data from wrong-version bucket must be gone (C3 drop+recreate)")

	// New data must be readable.
	newData, err := store.LoadShard("proj-arch", "local/component@newkey")
	require.NoError(t, err)
	assert.NotNil(t, newData, "new shard written after recreate must be readable")
}

func TestArchStore_T19_NewBinaryOldDB_NoBucket(t *testing.T) {
	// T19 new-binary/old-DB: DB without an arch_shards bucket.
	// LoadShard must return nil, nil (not error).
	store, _ := newTestStore(t)

	// LoadShard without ever calling SaveShards — bucket absent.
	got, err := store.LoadShard("proj-arch", "local/component@foo")
	require.NoError(t, err, "absent bucket must return nil not error (C3 new-binary/old-DB)")
	assert.Nil(t, got)
}

func TestArchStore_T19_OldBinaryNewDB_BucketIgnored(t *testing.T) {
	// T19 old-binary/new-DB: a DB with an arch_shards bucket must not
	// corrupt unrelated data (index, learner). Old binary has no arch code;
	// the bucket is simply ignored.
	// We verify this by checking the index bucket is absent (not polluted by arch writes).
	store, _ := newTestStore(t)

	// Pre-populate an arch_shards bucket (simulating a new-binary write).
	require.NoError(t, store.SaveShards("proj-arch", makeTestShards()))

	// Check via the same store connection that the index bucket is NOT present
	// (arch_shards must not create or modify the index bucket).
	err := store.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte("proj-arch"))
		require.NotNil(t, proj, "project bucket must exist")

		// index sub-bucket must be absent — SaveShards must not touch it.
		ib := proj.Bucket(bucketIndex)
		assert.Nil(t, ib, "index bucket must be absent after SaveShards-only write (no cross-contamination)")
		return nil
	})
	require.NoError(t, err)
}

func TestArchStore_T19_WrongVersionRead_ReturnsEmpty(t *testing.T) {
	// T19: reading a wrong-version arch_shards bucket returns nil, nil — not corrupt data.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	// Pre-populate wrong-version arch_shards bucket via raw bolt.
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	require.NoError(t, err)
	err = db.Update(func(tx *bolt.Tx) error {
		proj, _ := tx.CreateBucketIfNotExists([]byte("proj-arch"))
		ab, _ := proj.CreateBucketIfNotExists(bucketArchShards)
		if err := ab.Put(keyVersion, []byte{0xFF}); err != nil { // future version
			return err
		}
		m, _ := json.Marshal(map[string]string{"kind": "buckets"})
		return ab.Put([]byte("local/component@foo"), m)
	})
	require.NoError(t, err)
	db.Close()

	// Open via Store.
	store, err := NewStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	// LoadShard must return nil, nil — wrong version treated as empty.
	got, err := store.LoadShard("proj-arch", "local/component@foo")
	require.NoError(t, err, "wrong-version bucket must not error")
	assert.Nil(t, got, "wrong-version bucket: LoadShard must return nil (T19/T38)")

	// HasArchBucket must return false for wrong-version bucket.
	assert.False(t, store.HasArchBucket("proj-arch"),
		"wrong-version bucket: HasArchBucket must return false (C3)")
}

// ─────────────────────────────────────────────────────────────────────────────
// C1 — SaveShards must not be called under App.mu (test documents the contract)
// ─────────────────────────────────────────────────────────────────────────────

func TestArchStore_C1_Contract(t *testing.T) {
	// C1 contract test: SaveShards is called WITHOUT holding any mutex.
	// This is a documentation test — the goroutine does not hold App.mu.
	// A real C1 violation would be calling store.SaveShards inside db.Update
	// while App.mu is held. We verify it succeeds normally and returns promptly.
	store, _ := newTestStore(t)
	done := make(chan error, 1)
	go func() {
		done <- store.SaveShards("proj-arch", makeTestShards())
	}()
	err := <-done
	assert.NoError(t, err, "SaveShards must succeed when called outside any lock (C1)")
}
