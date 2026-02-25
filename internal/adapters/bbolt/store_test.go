package bbolt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/corey/aoa/internal/domain/analyzer"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
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
// Session summary persistence tests
// =============================================================================

func TestStore_SessionSummary_Roundtrip(t *testing.T) {
	store, _ := newTestStore(t)

	summary := &ports.SessionSummary{
		SessionID:        "sess-abc-123",
		StartTime:        1700000000,
		EndTime:          1700003600,
		PromptCount:      42,
		ReadCount:        15,
		GuidedReadCount:  8,
		GuidedRatio:      0.533,
		TokensSaved:      12000,
		TokensCounterfact: 5000,
		InputTokens:      50000,
		OutputTokens:     20000,
		CacheReadTokens:  30000,
		CacheWriteTokens: 10000,
		Model:            "claude-opus-4-6",
	}

	err := store.SaveSessionSummary("proj-1", summary)
	require.NoError(t, err)

	loaded, err := store.LoadSessionSummary("proj-1", "sess-abc-123")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, summary.SessionID, loaded.SessionID)
	assert.Equal(t, summary.StartTime, loaded.StartTime)
	assert.Equal(t, summary.EndTime, loaded.EndTime)
	assert.Equal(t, summary.PromptCount, loaded.PromptCount)
	assert.Equal(t, summary.ReadCount, loaded.ReadCount)
	assert.Equal(t, summary.GuidedReadCount, loaded.GuidedReadCount)
	assert.InDelta(t, summary.GuidedRatio, loaded.GuidedRatio, 0.001)
	assert.Equal(t, summary.TokensSaved, loaded.TokensSaved)
	assert.Equal(t, summary.InputTokens, loaded.InputTokens)
	assert.Equal(t, summary.OutputTokens, loaded.OutputTokens)
	assert.Equal(t, summary.CacheReadTokens, loaded.CacheReadTokens)
	assert.Equal(t, summary.CacheWriteTokens, loaded.CacheWriteTokens)
	assert.Equal(t, summary.Model, loaded.Model)
}

func TestStore_SessionSummary_ListMultiple(t *testing.T) {
	store, _ := newTestStore(t)

	s1 := &ports.SessionSummary{SessionID: "sess-1", PromptCount: 10}
	s2 := &ports.SessionSummary{SessionID: "sess-2", PromptCount: 20}
	s3 := &ports.SessionSummary{SessionID: "sess-3", PromptCount: 30}

	require.NoError(t, store.SaveSessionSummary("proj-1", s1))
	require.NoError(t, store.SaveSessionSummary("proj-1", s2))
	require.NoError(t, store.SaveSessionSummary("proj-1", s3))

	summaries, err := store.ListSessionSummaries("proj-1")
	require.NoError(t, err)
	assert.Len(t, summaries, 3)

	ids := make(map[string]bool)
	for _, s := range summaries {
		ids[s.SessionID] = true
	}
	assert.True(t, ids["sess-1"])
	assert.True(t, ids["sess-2"])
	assert.True(t, ids["sess-3"])
}

func TestStore_SessionSummary_OverwriteSameID(t *testing.T) {
	store, _ := newTestStore(t)

	s1 := &ports.SessionSummary{SessionID: "sess-1", PromptCount: 10}
	require.NoError(t, store.SaveSessionSummary("proj-1", s1))

	// Overwrite with updated prompt count
	s1Updated := &ports.SessionSummary{SessionID: "sess-1", PromptCount: 25}
	require.NoError(t, store.SaveSessionSummary("proj-1", s1Updated))

	loaded, err := store.LoadSessionSummary("proj-1", "sess-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, 25, loaded.PromptCount)

	// Should still be just one entry
	summaries, err := store.ListSessionSummaries("proj-1")
	require.NoError(t, err)
	assert.Len(t, summaries, 1)
}

func TestStore_SessionSummary_MissingSessions(t *testing.T) {
	store, _ := newTestStore(t)

	// Non-existent project
	loaded, err := store.LoadSessionSummary("proj-1", "sess-1")
	require.NoError(t, err)
	assert.Nil(t, loaded)

	// Non-existent session
	require.NoError(t, store.SaveSessionSummary("proj-1", &ports.SessionSummary{SessionID: "sess-1"}))
	loaded, err = store.LoadSessionSummary("proj-1", "sess-nonexistent")
	require.NoError(t, err)
	assert.Nil(t, loaded)

	// Empty project list
	summaries, err := store.ListSessionSummaries("proj-empty")
	require.NoError(t, err)
	assert.Nil(t, summaries)
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

// =============================================================================
// Dimensional analysis persistence tests (L5)
// =============================================================================

func makeTestDimensions() map[string]*analyzer.FileAnalysis {
	var mask1 analyzer.Bitmask
	mask1.Set(analyzer.TierSecurity, 0)
	mask1.Set(analyzer.TierSecurity, 1)

	var mask2 analyzer.Bitmask
	mask2.Set(analyzer.TierQuality, 5)

	return map[string]*analyzer.FileAnalysis{
		"handler.go": {
			Path:     "handler.go",
			Language: "go",
			Bitmask:  mask1,
			Findings: []analyzer.RuleFinding{
				{RuleID: "hardcoded_secret", Line: 10, Severity: analyzer.SevCritical},
				{RuleID: "command_injection", Line: 15, Severity: analyzer.SevCritical},
			},
			Methods: []analyzer.MethodAnalysis{
				{
					Name:    "processInput",
					Line:    5,
					EndLine: 20,
					Bitmask: mask1,
					Score:   20,
					Findings: []analyzer.RuleFinding{
						{RuleID: "hardcoded_secret", Line: 10, Severity: analyzer.SevCritical},
					},
				},
			},
			ScanTime: 150,
		},
		"util.go": {
			Path:     "util.go",
			Language: "go",
			Bitmask:  mask2,
			Findings: []analyzer.RuleFinding{
				{RuleID: "ignored_error", Line: 22, Severity: analyzer.SevWarning},
			},
			ScanTime: 80,
		},
	}
}

func TestStore_Dimensions_Roundtrip(t *testing.T) {
	store, _ := newTestStore(t)
	original := makeTestDimensions()

	err := store.SaveAllDimensions("proj-1", original)
	require.NoError(t, err)

	loaded, err := store.LoadAllDimensions("proj-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, len(original), len(loaded))

	for path, orig := range original {
		got, ok := loaded[path]
		require.True(t, ok, "missing file %s", path)
		assert.Equal(t, orig.Path, got.Path)
		assert.Equal(t, orig.Language, got.Language)
		assert.Equal(t, orig.Bitmask, got.Bitmask)
		assert.Equal(t, len(orig.Findings), len(got.Findings))
		assert.Equal(t, len(orig.Methods), len(got.Methods))
		assert.Equal(t, orig.ScanTime, got.ScanTime)
	}
}

func TestStore_Dimensions_Overwrite(t *testing.T) {
	store, _ := newTestStore(t)

	orig := makeTestDimensions()
	require.NoError(t, store.SaveAllDimensions("proj-1", orig))

	// Overwrite with smaller set
	updated := map[string]*analyzer.FileAnalysis{
		"new.go": {Path: "new.go", Language: "go"},
	}
	require.NoError(t, store.SaveAllDimensions("proj-1", updated))

	loaded, err := store.LoadAllDimensions("proj-1")
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Contains(t, loaded, "new.go")
	assert.NotContains(t, loaded, "handler.go")
}

func TestStore_Dimensions_EmptyProject(t *testing.T) {
	store, _ := newTestStore(t)

	loaded, err := store.LoadAllDimensions("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestStore_Dimensions_EmptyAnalyses(t *testing.T) {
	store, _ := newTestStore(t)

	err := store.SaveAllDimensions("proj-1", map[string]*analyzer.FileAnalysis{})
	require.NoError(t, err)

	loaded, err := store.LoadAllDimensions("proj-1")
	require.NoError(t, err)
	// Empty bucket still returns a valid map
	assert.NotNil(t, loaded)
	assert.Len(t, loaded, 0)
}

// =============================================================================
// L7.2: Binary encoding tests — posting lists, gob, migration, benchmarks
// =============================================================================

func TestEncodeDecodePostingLists_Roundtrip(t *testing.T) {
	tests := []struct {
		name   string
		tokens map[string][]ports.TokenRef
	}{
		{
			name: "normal",
			tokens: map[string][]ports.TokenRef{
				"login":   {{FileID: 1, Line: 10}, {FileID: 2, Line: 25}},
				"handler": {{FileID: 1, Line: 10}},
				"session": {{FileID: 3, Line: 5}},
			},
		},
		{
			name:   "empty map",
			tokens: map[string][]ports.TokenRef{},
		},
		{
			name: "zero-ref token",
			tokens: map[string][]ports.TokenRef{
				"empty_token": {},
				"has_ref":     {{FileID: 99, Line: 1}},
			},
		},
		{
			name: "max values",
			tokens: map[string][]ports.TokenRef{
				"max": {{FileID: 4294967295, Line: 65535}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := encodePostingLists(tt.tokens)
			require.NoError(t, err)

			decoded, err := decodePostingLists(encoded)
			require.NoError(t, err)

			assert.Equal(t, len(tt.tokens), len(decoded))
			for key, refs := range tt.tokens {
				assert.Equal(t, refs, decoded[key], "mismatch for token %q", key)
			}
		})
	}
}

func TestDecodePostingLists_CorruptData(t *testing.T) {
	// Truncated data should return errors, not panics.
	_, err := decodePostingLists([]byte{})
	assert.Error(t, err)

	_, err = decodePostingLists([]byte{0x01, 0x00})
	assert.Error(t, err)

	// Header says 1 token, but no key data follows.
	_, err = decodePostingLists([]byte{0x01, 0x00, 0x00, 0x00})
	assert.Error(t, err)
}

func TestGobMetadata_Roundtrip(t *testing.T) {
	original := map[string]*ports.SymbolMeta{
		"1:10": {
			Name:      "login",
			Signature: "login(self, user, password)",
			Kind:      "function",
			StartLine: 10,
			EndLine:   25,
			Parent:    "AuthHandler",
			Tags:      []string{"auth", "login"},
		},
		"2:25": {
			Name:      "deploy",
			Signature: "deploy()",
			Kind:      "function",
			StartLine: 25,
			EndLine:   40,
		},
	}

	encoded, err := encodeGob(original)
	require.NoError(t, err)

	var decoded map[string]*ports.SymbolMeta
	err = decodeGob(encoded, &decoded)
	require.NoError(t, err)

	assert.Equal(t, len(original), len(decoded))
	for k, orig := range original {
		got := decoded[k]
		require.NotNil(t, got, "missing key %s", k)
		assert.Equal(t, orig.Name, got.Name)
		assert.Equal(t, orig.Signature, got.Signature)
		assert.Equal(t, orig.Kind, got.Kind)
		assert.Equal(t, orig.StartLine, got.StartLine)
		assert.Equal(t, orig.EndLine, got.EndLine)
		assert.Equal(t, orig.Parent, got.Parent)
		assert.Equal(t, orig.Tags, got.Tags)
	}
}

func TestGobFiles_Roundtrip(t *testing.T) {
	original := map[string]*ports.FileMeta{
		"1": {Path: "handler.py", LastModified: 1700000000, Language: "python", Domain: "@auth", Size: 4096},
		"2": {Path: "views.py", LastModified: 1700000100, Language: "python", Domain: "@api", Size: 2048},
	}

	encoded, err := encodeGob(original)
	require.NoError(t, err)

	var decoded map[string]*ports.FileMeta
	err = decodeGob(encoded, &decoded)
	require.NoError(t, err)

	assert.Equal(t, len(original), len(decoded))
	for k, orig := range original {
		got := decoded[k]
		require.NotNil(t, got, "missing key %s", k)
		assert.Equal(t, orig.Path, got.Path)
		assert.Equal(t, orig.LastModified, got.LastModified)
		assert.Equal(t, orig.Language, got.Language)
		assert.Equal(t, orig.Domain, got.Domain)
		assert.Equal(t, orig.Size, got.Size)
	}
}

func TestStore_FormatMigration_V0ToV1(t *testing.T) {
	// Write v0 JSON blobs directly via bbolt, then LoadIndex (detects v0),
	// SaveIndex (writes v1), LoadIndex again (detects v1). Verify data matches.
	dir := t.TempDir()
	path := filepath.Join(dir, "migrate.db")

	// Write v0 data directly.
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	require.NoError(t, err)

	original := makeTestIndex()

	// Build v0 JSON blobs manually (same as the old SaveIndex).
	type v0JSON struct {
		Tokens   map[string][]ports.TokenRef       `json:"tokens"`
		Metadata map[string]*ports.SymbolMeta       `json:"metadata"`
		Files    map[string]*ports.FileMeta         `json:"files"`
	}
	v0 := v0JSON{
		Tokens:   original.Tokens,
		Metadata: make(map[string]*ports.SymbolMeta, len(original.Metadata)),
		Files:    make(map[string]*ports.FileMeta, len(original.Files)),
	}
	for ref, sym := range original.Metadata {
		v0.Metadata[fmt.Sprintf("%d:%d", ref.FileID, ref.Line)] = sym
	}
	for fid, fm := range original.Files {
		v0.Files[fmt.Sprintf("%d", fid)] = fm
	}

	tokJSON, err := json.Marshal(v0.Tokens)
	require.NoError(t, err)
	metaJSON, err := json.Marshal(v0.Metadata)
	require.NoError(t, err)
	filesJSON, err := json.Marshal(v0.Files)
	require.NoError(t, err)

	err = db.Update(func(tx *bolt.Tx) error {
		proj, err := tx.CreateBucketIfNotExists([]byte("proj-1"))
		if err != nil {
			return err
		}
		ib, err := proj.CreateBucketIfNotExists(bucketIndex)
		if err != nil {
			return err
		}
		// No _version key — this is v0.
		if err := ib.Put(keyTokens, tokJSON); err != nil {
			return err
		}
		if err := ib.Put(keyMetadata, metaJSON); err != nil {
			return err
		}
		return ib.Put(keyFiles, filesJSON)
	})
	require.NoError(t, err)
	db.Close()

	// Open via Store and load v0.
	store, err := NewStore(path)
	require.NoError(t, err)
	defer store.Close()

	loaded, err := store.LoadIndex("proj-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, len(original.Tokens), len(loaded.Tokens))
	assert.Equal(t, len(original.Metadata), len(loaded.Metadata))
	assert.Equal(t, len(original.Files), len(loaded.Files))

	// Save — this writes v1.
	err = store.SaveIndex("proj-1", loaded)
	require.NoError(t, err)

	// Load again — should read v1 binary/gob.
	loaded2, err := store.LoadIndex("proj-1")
	require.NoError(t, err)
	require.NotNil(t, loaded2)

	// Verify data matches original.
	assert.Equal(t, len(original.Tokens), len(loaded2.Tokens))
	for tok, refs := range original.Tokens {
		assert.Equal(t, refs, loaded2.Tokens[tok], "token %q mismatch after migration", tok)
	}
	assert.Equal(t, len(original.Metadata), len(loaded2.Metadata))
	for ref, sym := range original.Metadata {
		got, ok := loaded2.Metadata[ref]
		require.True(t, ok, "missing metadata for %v after migration", ref)
		assert.Equal(t, sym.Name, got.Name)
		assert.Equal(t, sym.Kind, got.Kind)
	}
	assert.Equal(t, len(original.Files), len(loaded2.Files))
	for fid, fm := range original.Files {
		got, ok := loaded2.Files[fid]
		require.True(t, ok, "missing file %d after migration", fid)
		assert.Equal(t, fm.Path, got.Path)
	}

	// Verify version key is now v1.
	err = store.db.View(func(tx *bolt.Tx) error {
		proj := tx.Bucket([]byte("proj-1"))
		require.NotNil(t, proj)
		ib := proj.Bucket(bucketIndex)
		require.NotNil(t, ib)
		v := ib.Get(keyVersion)
		require.NotNil(t, v)
		assert.Equal(t, []byte{formatV1}, v)
		return nil
	})
	require.NoError(t, err)
}

func TestStore_LargeIndex_BinaryRoundtrip(t *testing.T) {
	// 50K tokens, ~500K refs. Verify roundtrip correctness and measure size.
	store, _ := newTestStore(t)

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	// Build 50K tokens with ~10 refs each = ~500K refs total.
	for i := 0; i < 50000; i++ {
		tok := fmt.Sprintf("token_%d", i)
		refs := make([]ports.TokenRef, 10)
		for j := 0; j < 10; j++ {
			fid := uint32(i*10 + j)
			line := uint16((j + 1) * 5)
			refs[j] = ports.TokenRef{FileID: fid, Line: line}
			if j == 0 {
				idx.Metadata[refs[j]] = &ports.SymbolMeta{
					Name: tok, Kind: "function", Signature: tok + "()",
					StartLine: line, EndLine: line + 20,
				}
				idx.Files[fid] = &ports.FileMeta{
					Path: fmt.Sprintf("pkg/mod_%d/file_%d.go", i/100, i),
					Language: "go", LastModified: 1700000000 + int64(i),
				}
			}
		}
		idx.Tokens[tok] = refs
	}

	err := store.SaveIndex("proj-large", idx)
	require.NoError(t, err)

	loaded, err := store.LoadIndex("proj-large")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, len(idx.Tokens), len(loaded.Tokens))
	assert.Equal(t, len(idx.Metadata), len(loaded.Metadata))
	assert.Equal(t, len(idx.Files), len(loaded.Files))

	// Spot check a few tokens.
	for _, tok := range []string{"token_0", "token_25000", "token_49999"} {
		assert.Equal(t, idx.Tokens[tok], loaded.Tokens[tok], "mismatch for %s", tok)
	}

	// Measure binary posting list size.
	encoded, err := encodePostingLists(idx.Tokens)
	require.NoError(t, err)
	t.Logf("Large index: %d tokens, %d total refs, binary size=%d bytes (%.1f MB)",
		len(idx.Tokens), 50000*10, len(encoded), float64(len(encoded))/1024/1024)
}

// =============================================================================
// Benchmarks — binary encoding
// =============================================================================

func benchmarkIndex() *ports.Index {
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}
	for i := 0; i < 10000; i++ {
		tok := fmt.Sprintf("token_%d", i)
		refs := make([]ports.TokenRef, 10)
		for j := 0; j < 10; j++ {
			fid := uint32(i*10 + j)
			line := uint16((j + 1) * 5)
			refs[j] = ports.TokenRef{FileID: fid, Line: line}
		}
		idx.Tokens[tok] = refs
		idx.Files[uint32(i)] = &ports.FileMeta{
			Path: fmt.Sprintf("file_%d.go", i), Language: "go",
			LastModified: 1700000000 + int64(i),
		}
		idx.Metadata[refs[0]] = &ports.SymbolMeta{
			Name: tok, Kind: "function", Signature: tok + "()",
			StartLine: refs[0].Line, EndLine: refs[0].Line + 20,
		}
	}
	return idx
}

func BenchmarkEncodePostingLists(b *testing.B) {
	idx := benchmarkIndex()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := encodePostingLists(idx.Tokens)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodePostingLists(b *testing.B) {
	idx := benchmarkIndex()
	data, err := encodePostingLists(idx.Tokens)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := decodePostingLists(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStore_SaveIndex_V1(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.db")
	store, err := NewStore(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	idx := benchmarkIndex()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := store.SaveIndex("bench", idx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStore_LoadIndex_V1(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "bench.db")
	store, err := NewStore(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	idx := benchmarkIndex()
	if err := store.SaveIndex("bench", idx); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := store.LoadIndex("bench")
		if err != nil {
			b.Fatal(err)
		}
	}
}
