package bbolt

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

// =============================================================================
// L19.15 facts_findings — C3 schema version + T19 extension
// =============================================================================

// makeTestFindings returns a stable set of findings for testing.
func makeTestFindings() []arch.Finding {
	return []arch.Finding{
		{
			ID:       "abc123def456789a",
			Rule:     "cycle",
			Severity: "error",
			Scope:    "internal",
			Message:  "dependency cycle: pkg/a → pkg/b → pkg/a",
			Subjects: []string{"m_pkg_a", "m_pkg_b"},
			Sources:  []arch.SourceRef{{File: "pkg/a/a.go", Line: 5}},
		},
		{
			ID:       "bbb222ccc333444d",
			Rule:     "god",
			Severity: "warn",
			Scope:    "internal",
			Message:  "god component: Hub (in 5 · out 4)",
			Subjects: []string{"m_hub"},
			Sources:  []arch.SourceRef{{File: "internal/hub/hub.go", Line: 1}},
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Round-trip
// ─────────────────────────────────────────────────────────────────────────────

func TestFindingsStore_SaveLoadRoundtrip(t *testing.T) {
	store, _ := newTestStore(t)
	want := makeTestFindings()

	require.NoError(t, store.SaveFindings("proj-1", "internal", want))

	got, err := store.LoadFindings("proj-1", "internal")
	require.NoError(t, err)
	require.Len(t, got, len(want))
	assert.Equal(t, want[0].ID, got[0].ID)
	assert.Equal(t, want[0].Rule, got[0].Rule)
	assert.Equal(t, want[0].Message, got[0].Message)
	assert.Equal(t, want[1].ID, got[1].ID)
}

func TestFindingsStore_LoadMissing_Project(t *testing.T) {
	// No project at all → empty result, no error.
	store, _ := newTestStore(t)
	got, err := store.LoadFindings("no-such-proj", "internal")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestFindingsStore_LoadMissing_Scope(t *testing.T) {
	// Project exists, scope absent → empty result, no error.
	store, _ := newTestStore(t)
	require.NoError(t, store.SaveFindings("proj-1", "internal", makeTestFindings()))

	got, err := store.LoadFindings("proj-1", "other-scope")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestFindingsStore_Overwrite(t *testing.T) {
	// Saving findings for the same scope twice overwrites the prior entry.
	store, _ := newTestStore(t)

	original := makeTestFindings()
	require.NoError(t, store.SaveFindings("proj-1", "internal", original))

	updated := []arch.Finding{{
		ID: "new111", Rule: "orphan", Severity: "info", Scope: "internal",
		Message: "orphan: X — no connections", Subjects: []string{"m_x"},
		Sources: []arch.SourceRef{{File: "x.go", Line: 1}},
	}}
	require.NoError(t, store.SaveFindings("proj-1", "internal", updated))

	got, err := store.LoadFindings("proj-1", "internal")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "orphan", got[0].Rule)
}

func TestFindingsStore_ProjectScoped(t *testing.T) {
	// Findings are per-project: proj-A's findings are invisible to proj-B.
	store, _ := newTestStore(t)

	fa := []arch.Finding{{ID: "a1", Rule: "cycle", Severity: "error", Scope: "s", Message: "cycle A", Sources: []arch.SourceRef{}}}
	fb := []arch.Finding{{ID: "b1", Rule: "god", Severity: "warn", Scope: "s", Message: "god B", Sources: []arch.SourceRef{}}}

	require.NoError(t, store.SaveFindings("proj-A", "s", fa))
	require.NoError(t, store.SaveFindings("proj-B", "s", fb))

	a, err := store.LoadFindings("proj-A", "s")
	require.NoError(t, err)
	assert.Len(t, a, 1)
	assert.Equal(t, "cycle", a[0].Rule)

	b, err := store.LoadFindings("proj-B", "s")
	require.NoError(t, err)
	assert.Len(t, b, 1)
	assert.Equal(t, "god", b[0].Rule)
}

func TestFindingsStore_MultipleScopesInProject(t *testing.T) {
	// Different scopes within the same project are stored independently.
	store, _ := newTestStore(t)

	f1 := []arch.Finding{{ID: "f1", Rule: "cycle", Severity: "error", Scope: "scope1", Message: "m1", Sources: []arch.SourceRef{{File: "a.go", Line: 1}}}}
	f2 := []arch.Finding{{ID: "f2", Rule: "orphan", Severity: "info", Scope: "scope2", Message: "m2", Sources: []arch.SourceRef{{File: "b.go", Line: 1}}}}

	require.NoError(t, store.SaveFindings("proj-1", "scope1", f1))
	require.NoError(t, store.SaveFindings("proj-1", "scope2", f2))

	got1, err := store.LoadFindings("proj-1", "scope1")
	require.NoError(t, err)
	require.Len(t, got1, 1)
	assert.Equal(t, "cycle", got1[0].Rule)

	got2, err := store.LoadFindings("proj-1", "scope2")
	require.NoError(t, err)
	require.Len(t, got2, 1)
	assert.Equal(t, "orphan", got2[0].Rule)
}

func TestFindingsStore_SaveEmpty(t *testing.T) {
	// Saving an empty slice is valid — clears findings for the scope.
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveFindings("proj-1", "s", makeTestFindings()))
	require.NoError(t, store.SaveFindings("proj-1", "s", []arch.Finding{}))

	got, err := store.LoadFindings("proj-1", "s")
	require.NoError(t, err)
	assert.True(t, len(got) == 0, "empty findings slice must round-trip as empty")
}

// ─────────────────────────────────────────────────────────────────────────────
// T19 extension: C3 schema migration — both-direction tolerance
// ─────────────────────────────────────────────────────────────────────────────

func TestFindingsStore_T19_NewBinaryOldDB_NoBucket(t *testing.T) {
	// T19 new-binary/old-DB: DB without a facts_findings bucket.
	// LoadFindings → empty result, no error, no panic.
	// SaveFindings → bucket created silently.
	// After save, LoadFindings returns the saved data.
	store, _ := newTestStore(t)

	// No facts_findings bucket yet — Load must return nil.
	got, err := store.LoadFindings("proj-1", "internal")
	require.NoError(t, err, "load on fresh DB must not error")
	assert.Nil(t, got, "load on fresh DB must return nil")

	// Save creates the bucket silently.
	findings := makeTestFindings()
	require.NoError(t, store.SaveFindings("proj-1", "internal", findings))

	// Now load should return the saved data.
	loaded, err := store.LoadFindings("proj-1", "internal")
	require.NoError(t, err)
	require.Len(t, loaded, len(findings))
}

func TestFindingsStore_T19_VersionByteWrittenOnCreate(t *testing.T) {
	// T19: _version byte is written on facts_findings bucket creation.
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveFindings("proj-1", "internal", makeTestFindings()))

	err := store.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte("proj-1"))
		require.NotNil(t, proj)
		fb := proj.Bucket(bucketFactsFindings)
		require.NotNil(t, fb, "facts_findings bucket must exist after save")
		v := fb.Get(keyVersion)
		require.NotNil(t, v, "_version must be set")
		assert.Equal(t, []byte{findingsVersion}, v, "_version must equal findingsVersion")
		return nil
	})
	require.NoError(t, err)
}

func TestFindingsStore_T19_VersionMismatch_DropAndRecreate(t *testing.T) {
	// T19 version mismatch: write a facts_findings bucket with a wrong version byte.
	// The next SaveFindings must drop and re-create the bucket.
	// Old data (from wrong-version bucket) must not appear after recreate.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	require.NoError(t, err)

	// Manually write a facts_findings bucket with version byte = 0xFF (unknown).
	oldFinding := arch.Finding{ID: "old1", Rule: "old-rule", Severity: "info", Scope: "s", Message: "old", Sources: []arch.SourceRef{}}
	oldData, _ := json.Marshal([]arch.Finding{oldFinding})
	err = db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte("proj-1"))
		if err != nil {
			return err
		}
		fb, err := proj.CreateBucketIfNotExists(bucketFactsFindings)
		if err != nil {
			return err
		}
		// Write wrong version.
		if err := fb.Put(keyVersion, []byte{0xFF}); err != nil {
			return err
		}
		return fb.Put([]byte("s"), oldData)
	})
	require.NoError(t, err)
	db.Close()

	// Open via our Store.
	store, err := NewStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	// Save new findings — must detect version mismatch and drop+recreate.
	newFindings := makeTestFindings()
	require.NoError(t, store.SaveFindings("proj-1", "s", newFindings))

	// The old wrong-version data must NOT appear.
	all, err := store.LoadFindings("proj-1", "s")
	require.NoError(t, err)
	require.Len(t, all, len(newFindings), "only new findings must be present after drop+recreate")
	assert.NotEqual(t, "old-rule", all[0].Rule, "old wrong-version finding must be gone after bucket drop")

	// Version byte must now be correct.
	err = store.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte("proj-1"))
		require.NotNil(t, proj)
		fb := proj.Bucket(bucketFactsFindings)
		require.NotNil(t, fb)
		v := fb.Get(keyVersion)
		assert.Equal(t, []byte{findingsVersion}, v)
		return nil
	})
	require.NoError(t, err)
}

func TestFindingsStore_T19_OldBinaryNewDB_IndexNotCorrupted(t *testing.T) {
	// T19 old-binary/new-DB: a DB with a facts_findings bucket must not corrupt
	// the index bucket. An "old binary" (no FindingsStore code) would only call
	// LoadIndex; here we verify that works after findings are written.
	//
	// Proves: presence of a facts_findings bucket NEVER triggers index self-recovery
	// and NEVER corrupts existing index data.
	store, _ := newTestStore(t)

	// Write an index.
	idx := makeTestIndex()
	require.NoError(t, store.SaveIndex("proj-1", idx))

	// Write findings (simulates a new-binary write that an old binary might see).
	require.NoError(t, store.SaveFindings("proj-1", "internal", makeTestFindings()))

	// An "old binary" would call LoadIndex — it must still work.
	loaded, err := store.LoadIndex("proj-1")
	require.NoError(t, err)
	require.NotNil(t, loaded, "index must not be nil — facts_findings bucket must not corrupt index")
	assert.Equal(t, len(idx.Files), len(loaded.Files), "index data must be intact")
	assert.False(t, store.Recovered(), "facts_findings bucket must NOT trigger index self-recovery")
}

func TestFindingsStore_T19_ReadPath_WrongVersion_ReturnsEmpty(t *testing.T) {
	// T19 read path: a facts_findings bucket with wrong version byte must return
	// empty findings (not an error, not corrupted data). Mirrors T38 for edges.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	require.NoError(t, err)

	// Write findings under a wrong version byte.
	goodFinding := arch.Finding{ID: "g1", Rule: "cycle", Severity: "error", Scope: "s", Message: "c", Sources: []arch.SourceRef{}}
	data, _ := json.Marshal([]arch.Finding{goodFinding})
	err = db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte("proj-1"))
		if err != nil {
			return err
		}
		fb, err := proj.CreateBucketIfNotExists(bucketFactsFindings)
		if err != nil {
			return err
		}
		if err := fb.Put(keyVersion, []byte{0xFF}); err != nil {
			return err
		}
		return fb.Put([]byte("s"), data)
	})
	require.NoError(t, err)
	db.Close()

	store, err := NewStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	// LoadFindings must return nil, nil (wrong version → treat as empty).
	got, err := store.LoadFindings("proj-1", "s")
	require.NoError(t, err, "wrong-version bucket must not error on load")
	assert.Nil(t, got, "wrong-version facts_findings bucket: LoadFindings must return nil")
}
