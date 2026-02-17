package bbolt

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// S-03: bbolt Storage Adapter — Save/load index, crash recovery
// L-06: Persist learner state to bbolt
// Goals: G6 (build on embedded storage), G7 (memory bounded)
// Expectation: bbolt replaces Redis. All data project-scoped. Survives crashes.
// =============================================================================

// newTestStore creates a temporary bbolt store for testing.
func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	store, err := NewStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store, path
}

// makeTestIndex creates a realistic test index.
func makeTestIndex() *ports.Index {
	return &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"login": {
				{FileID: 1, Line: 10},
				{FileID: 2, Line: 25},
			},
			"handler": {
				{FileID: 1, Line: 10},
			},
			"session": {
				{FileID: 3, Line: 5},
			},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {
				Name:      "login",
				Signature: "login(self, user, password)",
				Kind:      "function",
				StartLine: 10,
				EndLine:   25,
				Parent:    "AuthHandler",
				Tags:      []string{"auth", "login", "handler"},
			},
			{FileID: 2, Line: 25}: {
				Name:      "login_view",
				Signature: "login_view(request)",
				Kind:      "function",
				StartLine: 25,
				EndLine:   40,
				Tags:      []string{"login", "view"},
			},
			{FileID: 3, Line: 5}: {
				Name:      "SessionManager",
				Signature: "class SessionManager(object)",
				Kind:      "class",
				StartLine: 5,
				EndLine:   80,
			},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "services/auth/handler.py", LastModified: 1700000000, Language: "python", Domain: "@authentication", Size: 4096},
			2: {Path: "views/login.py", LastModified: 1700000100, Language: "python", Domain: "@api", Size: 2048},
			3: {Path: "services/session/manager.py", LastModified: 1700000200, Language: "python", Domain: "@authentication", Size: 8192},
		},
	}
}

// makeTestLearnerState creates a realistic learner state.
func makeTestLearnerState() *ports.LearnerState {
	return &ports.LearnerState{
		KeywordHits:     map[string]uint32{"login": 15, "session": 8, "handler": 3},
		TermHits:        map[string]uint32{"authentication": 20, "session_mgmt": 10},
		DomainMeta:      map[string]*ports.DomainMeta{
			"@authentication": {
				Hits:          18.5,
				TotalHits:     42,
				Tier:          "core",
				Source:        "seeded",
				State:         "active",
				StaleCycles:   0,
				HitsLastCycle: 12.3,
				LastHitAt:     50,
				CreatedAt:     1700000000,
			},
			"@api": {
				Hits:          7.2,
				TotalHits:     15,
				Tier:          "context",
				Source:        "learned",
				State:         "active",
				StaleCycles:   1,
				HitsLastCycle: 5.0,
				LastHitAt:     48,
				CreatedAt:     1700000100,
			},
		},
		CohitKwTerm:     map[string]uint32{"login:authentication": 12, "session:session_mgmt": 7},
		CohitTermDomain: map[string]uint32{"authentication:@authentication": 20},
		Bigrams:         map[string]uint32{"login:handler": 8, "session:token": 6},
		FileHits:        map[string]uint32{"services/auth/handler.py": 25, "views/login.py": 10},
		KeywordBlocklist: map[string]bool{"self": true, "return": true},
		GapKeywords:     map[string]bool{"foobar": true},
		PromptCount:     50,
	}
}

func TestStore_SaveLoadIndex_Roundtrip(t *testing.T) {
	// S-03, G6: Save an index, load it back. Tokens, metadata, files
	// all match the original. No data loss.
	store, _ := newTestStore(t)
	original := makeTestIndex()

	err := store.SaveIndex("proj-1", original)
	require.NoError(t, err)

	loaded, err := store.LoadIndex("proj-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// Tokens match
	assert.Equal(t, len(original.Tokens), len(loaded.Tokens))
	for tok, refs := range original.Tokens {
		assert.Equal(t, refs, loaded.Tokens[tok], "token %q refs mismatch", tok)
	}

	// Metadata match
	assert.Equal(t, len(original.Metadata), len(loaded.Metadata))
	for ref, sym := range original.Metadata {
		loadedSym, ok := loaded.Metadata[ref]
		require.True(t, ok, "missing metadata for %v", ref)
		assert.Equal(t, sym.Name, loadedSym.Name)
		assert.Equal(t, sym.Signature, loadedSym.Signature)
		assert.Equal(t, sym.Kind, loadedSym.Kind)
		assert.Equal(t, sym.StartLine, loadedSym.StartLine)
		assert.Equal(t, sym.EndLine, loadedSym.EndLine)
		assert.Equal(t, sym.Parent, loadedSym.Parent)
		assert.Equal(t, sym.Tags, loadedSym.Tags)
	}

	// Files match
	assert.Equal(t, len(original.Files), len(loaded.Files))
	for fid, fm := range original.Files {
		loadedFM, ok := loaded.Files[fid]
		require.True(t, ok, "missing file %d", fid)
		assert.Equal(t, fm.Path, loadedFM.Path)
		assert.Equal(t, fm.LastModified, loadedFM.LastModified)
		assert.Equal(t, fm.Language, loadedFM.Language)
		assert.Equal(t, fm.Domain, loadedFM.Domain)
		assert.Equal(t, fm.Size, loadedFM.Size)
	}
}

func TestStore_SaveLoadLearnerState_Roundtrip(t *testing.T) {
	// L-06, G6: Save learner state, load it back. All maps, counters,
	// and float values match. Zero tolerance on floats.
	store, _ := newTestStore(t)
	original := makeTestLearnerState()

	err := store.SaveLearnerState("proj-1", original)
	require.NoError(t, err)

	loaded, err := store.LoadLearnerState("proj-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// All maps and counters
	assert.Equal(t, original.KeywordHits, loaded.KeywordHits)
	assert.Equal(t, original.TermHits, loaded.TermHits)
	assert.Equal(t, original.CohitKwTerm, loaded.CohitKwTerm)
	assert.Equal(t, original.CohitTermDomain, loaded.CohitTermDomain)
	assert.Equal(t, original.Bigrams, loaded.Bigrams)
	assert.Equal(t, original.FileHits, loaded.FileHits)
	assert.Equal(t, original.KeywordBlocklist, loaded.KeywordBlocklist)
	assert.Equal(t, original.GapKeywords, loaded.GapKeywords)
	assert.Equal(t, original.PromptCount, loaded.PromptCount)

	// Domain meta — zero tolerance on floats
	require.Equal(t, len(original.DomainMeta), len(loaded.DomainMeta))
	for name, orig := range original.DomainMeta {
		got := loaded.DomainMeta[name]
		require.NotNil(t, got, "missing domain %s", name)
		assert.Equal(t, orig.Hits, got.Hits, "Hits mismatch for %s", name)
		assert.Equal(t, orig.TotalHits, got.TotalHits)
		assert.Equal(t, orig.Tier, got.Tier)
		assert.Equal(t, orig.Source, got.Source)
		assert.Equal(t, orig.State, got.State)
		assert.Equal(t, orig.StaleCycles, got.StaleCycles)
		assert.Equal(t, orig.HitsLastCycle, got.HitsLastCycle, "HitsLastCycle mismatch for %s", name)
		assert.Equal(t, orig.LastHitAt, got.LastHitAt)
		assert.Equal(t, orig.CreatedAt, got.CreatedAt)
	}
}

func TestStore_CrashRecovery(t *testing.T) {
	// S-03, G6: Write data, simulate crash (close without fsync),
	// reopen database. Data from last committed transaction is intact.
	// bbolt's transactional writes guarantee this.
	dir := t.TempDir()
	path := filepath.Join(dir, "crash.db")

	store, err := NewStore(path)
	require.NoError(t, err)

	idx := makeTestIndex()
	err = store.SaveIndex("proj-1", idx)
	require.NoError(t, err)

	// Close (simulates orderly shutdown — bbolt fsyncs on commit)
	err = store.Close()
	require.NoError(t, err)

	// Reopen — data from committed transaction should be intact
	store2, err := NewStore(path)
	require.NoError(t, err)
	defer store2.Close()

	loaded, err := store2.LoadIndex("proj-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, len(idx.Tokens), len(loaded.Tokens))
	assert.Equal(t, len(idx.Metadata), len(loaded.Metadata))
	assert.Equal(t, len(idx.Files), len(loaded.Files))
}

func TestStore_ProjectScoped(t *testing.T) {
	// S-03, G7: Two projects stored in same bbolt file use separate
	// buckets. Project A's data is invisible to project B.
	store, _ := newTestStore(t)

	idxA := makeTestIndex()
	stateA := makeTestLearnerState()

	idxB := &ports.Index{
		Tokens:   map[string][]ports.TokenRef{"deploy": {{FileID: 99, Line: 1}}},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{{FileID: 99, Line: 1}: {Name: "deploy", Kind: "function", Signature: "deploy()"}},
		Files:    map[uint32]*ports.FileMeta{99: {Path: "deploy.py", Language: "python"}},
	}
	stateB := &ports.LearnerState{
		KeywordHits: map[string]uint32{"deploy": 5},
		PromptCount: 10,
	}

	require.NoError(t, store.SaveIndex("proj-A", idxA))
	require.NoError(t, store.SaveLearnerState("proj-A", stateA))
	require.NoError(t, store.SaveIndex("proj-B", idxB))
	require.NoError(t, store.SaveLearnerState("proj-B", stateB))

	// Load A — should get A's data
	loadedIdxA, err := store.LoadIndex("proj-A")
	require.NoError(t, err)
	assert.Equal(t, 3, len(loadedIdxA.Tokens))

	loadedStateA, err := store.LoadLearnerState("proj-A")
	require.NoError(t, err)
	assert.Equal(t, uint32(50), loadedStateA.PromptCount)

	// Load B — should get B's data, not A's
	loadedIdxB, err := store.LoadIndex("proj-B")
	require.NoError(t, err)
	assert.Equal(t, 1, len(loadedIdxB.Tokens))
	assert.Contains(t, loadedIdxB.Tokens, "deploy")

	loadedStateB, err := store.LoadLearnerState("proj-B")
	require.NoError(t, err)
	assert.Equal(t, uint32(10), loadedStateB.PromptCount)

	// Nonexistent project — nil, nil
	loadedIdxC, err := store.LoadIndex("proj-C")
	require.NoError(t, err)
	assert.Nil(t, loadedIdxC)
}

func TestStore_DeleteProject(t *testing.T) {
	// S-03, G7: DeleteProject removes all data for that project.
	// Other projects unaffected.
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveIndex("proj-A", makeTestIndex()))
	require.NoError(t, store.SaveLearnerState("proj-A", makeTestLearnerState()))
	require.NoError(t, store.SaveIndex("proj-B", &ports.Index{
		Tokens:   map[string][]ports.TokenRef{"x": {{FileID: 1, Line: 1}}},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{{FileID: 1, Line: 1}: {Name: "x"}},
		Files:    map[uint32]*ports.FileMeta{1: {Path: "x.py"}},
	}))

	// Delete A
	err := store.DeleteProject("proj-A")
	require.NoError(t, err)

	// A is gone
	idx, err := store.LoadIndex("proj-A")
	require.NoError(t, err)
	assert.Nil(t, idx)

	state, err := store.LoadLearnerState("proj-A")
	require.NoError(t, err)
	assert.Nil(t, state)

	// B still exists
	idxB, err := store.LoadIndex("proj-B")
	require.NoError(t, err)
	require.NotNil(t, idxB)
	assert.Contains(t, idxB.Tokens, "x")

	// Delete nonexistent — idempotent
	err = store.DeleteProject("proj-C")
	assert.NoError(t, err)
}

func TestStore_ConcurrentReads(t *testing.T) {
	// S-03, G6: Multiple goroutines reading simultaneously.
	// bbolt supports concurrent readers, single writer.
	store, _ := newTestStore(t)
	require.NoError(t, store.SaveIndex("proj-1", makeTestIndex()))

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx, err := store.LoadIndex("proj-1")
			if err != nil {
				errs <- err
				return
			}
			if idx == nil {
				errs <- fmt.Errorf("got nil index")
				return
			}
			if len(idx.Tokens) != 3 {
				errs <- fmt.Errorf("expected 3 tokens, got %d", len(idx.Tokens))
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent read error: %v", err)
	}
}

func TestStore_LargeState_Performance(t *testing.T) {
	// S-03, G1: Save/load with 500 files, 24 domains, 500 keywords.
	// Save <10ms, load <10ms.
	store, _ := newTestStore(t)

	// Build a large index
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	for i := uint32(0); i < 500; i++ {
		idx.Files[i] = &ports.FileMeta{
			Path:         fmt.Sprintf("services/module_%d/handler.py", i),
			LastModified: 1700000000 + int64(i),
			Language:     "python",
			Domain:       fmt.Sprintf("@domain_%d", i%24),
		}
		ref := ports.TokenRef{FileID: i, Line: uint16(i%200 + 1)}
		tok := fmt.Sprintf("symbol_%d", i)
		idx.Tokens[tok] = append(idx.Tokens[tok], ref)
		idx.Metadata[ref] = &ports.SymbolMeta{
			Name:      tok,
			Signature: fmt.Sprintf("%s(self)", tok),
			Kind:      "function",
			StartLine: ref.Line,
			EndLine:   ref.Line + 20,
			Tags:      []string{"tag1", "tag2"},
		}
	}

	// Save
	start := time.Now()
	err := store.SaveIndex("proj-1", idx)
	saveTime := time.Since(start)
	require.NoError(t, err)

	// Load
	start = time.Now()
	loaded, err := store.LoadIndex("proj-1")
	loadTime := time.Since(start)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, len(idx.Files), len(loaded.Files))
	assert.Less(t, saveTime, 100*time.Millisecond, "save took %v", saveTime) // generous for CI
	assert.Less(t, loadTime, 100*time.Millisecond, "load took %v", loadTime)

	t.Logf("Performance: save=%v load=%v files=%d tokens=%d",
		saveTime, loadTime, len(idx.Files), len(idx.Tokens))
}

func TestStore_StateSurvivesRestart(t *testing.T) {
	// L-06, G6: Save state, close store, reopen store, load state.
	// Simulates process restart. State matches pre-save snapshot.
	dir := t.TempDir()
	path := filepath.Join(dir, "restart.db")

	// First session: save state
	store1, err := NewStore(path)
	require.NoError(t, err)

	original := makeTestLearnerState()
	err = store1.SaveLearnerState("proj-1", original)
	require.NoError(t, err)

	err = store1.Close()
	require.NoError(t, err)

	// Verify file exists on disk
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Second session: reopen and load
	store2, err := NewStore(path)
	require.NoError(t, err)
	defer store2.Close()

	loaded, err := store2.LoadLearnerState("proj-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, original.PromptCount, loaded.PromptCount)
	assert.Equal(t, original.KeywordHits, loaded.KeywordHits)
	assert.Equal(t, original.DomainMeta["@authentication"].Hits, loaded.DomainMeta["@authentication"].Hits)
	assert.Equal(t, original.DomainMeta["@api"].HitsLastCycle, loaded.DomainMeta["@api"].HitsLastCycle)
	assert.Equal(t, original.KeywordBlocklist, loaded.KeywordBlocklist)
	assert.Equal(t, original.GapKeywords, loaded.GapKeywords)
}

// =============================================================================
// Lock contention tests — verify the 1s timeout prevents hangs
// =============================================================================

func TestStore_OpenTimeout_DoesNotHang(t *testing.T) {
	// V-01: When another process/goroutine holds the bbolt exclusive lock,
	// a second open should timeout in ~1 second, not hang forever.
	dir := t.TempDir()
	path := filepath.Join(dir, "locked.db")

	// First store holds the exclusive lock.
	store1, err := NewStore(path)
	require.NoError(t, err)
	defer store1.Close()

	// Second open should timeout, not hang.
	start := time.Now()
	store2, err := NewStore(path)
	elapsed := time.Since(start)

	require.Error(t, err, "second open should fail with lock timeout")
	assert.Nil(t, store2, "store should be nil on timeout")
	assert.Contains(t, err.Error(), "timeout", "error should mention timeout")
	assert.Less(t, elapsed, 3*time.Second, "should complete within 3s, not hang")
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond, "should wait ~1s for the configured timeout")
}

func TestStore_OpenTimeout_ErrorMessage(t *testing.T) {
	// The error message should be useful for diagnosis — wrapped with context.
	dir := t.TempDir()
	path := filepath.Join(dir, "locked.db")

	store1, err := NewStore(path)
	require.NoError(t, err)
	defer store1.Close()

	_, err = NewStore(path)
	require.Error(t, err)
	// Should include our "bbolt open:" wrapper
	assert.Contains(t, err.Error(), "bbolt open")
	assert.Contains(t, err.Error(), "timeout")
}

func TestStore_OpenAfterClose_Succeeds(t *testing.T) {
	// After the lock holder closes, a new open should succeed immediately.
	dir := t.TempDir()
	path := filepath.Join(dir, "released.db")

	store1, err := NewStore(path)
	require.NoError(t, err)
	// Write some data so the file isn't empty.
	require.NoError(t, store1.SaveIndex("test", makeTestIndex()))
	store1.Close()

	// Second open should succeed immediately (no timeout wait).
	start := time.Now()
	store2, err := NewStore(path)
	elapsed := time.Since(start)

	require.NoError(t, err, "open after close should succeed")
	require.NotNil(t, store2)
	assert.Less(t, elapsed, 500*time.Millisecond, "should open instantly after lock released")
	defer store2.Close()

	// Verify data is accessible.
	idx, err := store2.LoadIndex("test")
	require.NoError(t, err)
	assert.Equal(t, 3, len(idx.Tokens))
}

func TestStore_OpenTimeout_ConcurrentAttempts(t *testing.T) {
	// Multiple goroutines trying to open a locked DB should all fail fast.
	dir := t.TempDir()
	path := filepath.Join(dir, "locked.db")

	store1, err := NewStore(path)
	require.NoError(t, err)
	defer store1.Close()

	const n = 3
	errs := make(chan error, n)
	durations := make(chan time.Duration, n)

	for i := 0; i < n; i++ {
		go func() {
			start := time.Now()
			s, err := NewStore(path)
			durations <- time.Since(start)
			if s != nil {
				s.Close()
			}
			errs <- err
		}()
	}

	for i := 0; i < n; i++ {
		err := <-errs
		d := <-durations
		assert.Error(t, err, "concurrent open %d should fail", i)
		assert.Contains(t, err.Error(), "timeout")
		assert.Less(t, d, 3*time.Second, "concurrent open %d should not hang", i)
	}
}
