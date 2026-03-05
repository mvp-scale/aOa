package bbolt

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// L17.1 + L17.2: ProjectTelemetry — lifetime counters, atomic save, backfill
// Goals: G6 (Value Proof — lifetime metrics survive session pruning)
// =============================================================================

// makeTestTelemetryDelta creates a telemetry delta for testing.
func makeTestTelemetryDelta(tokensSaved int64, reads, guided int) *ports.ProjectTelemetry {
	return &ports.ProjectTelemetry{
		TokensSaved:     tokensSaved,
		TimeSavedMs:     tokensSaved * 8, // ~8ms/token approximation
		Reads:           reads,
		GuidedReads:     guided,
		Sessions:        1,
		Prompts:         10,
		InputTokens:     5000,
		OutputTokens:    2000,
		CacheReadTokens: 3000,
		ShadowSaved:     500,
	}
}

func TestTelemetry_LoadEmpty(t *testing.T) {
	// Fresh project with no telemetry and no sessions → zero-value telemetry, no error.
	store, _ := newTestStore(t)

	telem, err := store.LoadTelemetry("proj-new")
	require.NoError(t, err)
	require.NotNil(t, telem, "LoadTelemetry should return non-nil zero struct, not nil")

	assert.Equal(t, int64(0), telem.TokensSaved)
	assert.Equal(t, int64(0), telem.TimeSavedMs)
	assert.Equal(t, 0, telem.Reads)
	assert.Equal(t, 0, telem.GuidedReads)
	assert.Equal(t, 0, telem.Sessions)
	assert.Equal(t, 0, telem.Prompts)
	assert.Equal(t, int64(0), telem.InputTokens)
	assert.Equal(t, int64(0), telem.OutputTokens)
	assert.Equal(t, int64(0), telem.CacheReadTokens)
	assert.Equal(t, int64(0), telem.ShadowSaved)
	assert.Equal(t, int64(0), telem.FirstSessionAt)
}

func TestTelemetry_SaveAndLoad_SingleSession(t *testing.T) {
	// Save one session with telemetry delta, then load. Lifetime matches delta.
	store, _ := newTestStore(t)

	summary := &ports.SessionSummary{
		SessionID:       "sess-1",
		StartTime:       1700000000,
		EndTime:         1700003600,
		PromptCount:     10,
		ReadCount:       5,
		GuidedReadCount: 3,
		TokensSaved:     1200,
		TimeSavedMs:     9600,
		InputTokens:     5000,
		OutputTokens:    2000,
		CacheReadTokens: 3000,
	}

	delta := &ports.ProjectTelemetry{
		TokensSaved:     1200,
		TimeSavedMs:     9600,
		Reads:           5,
		GuidedReads:     3,
		Sessions:        1,
		Prompts:         10,
		InputTokens:     5000,
		OutputTokens:    2000,
		CacheReadTokens: 3000,
		ShadowSaved:     400,
		FirstSessionAt:  1700000000,
	}

	err := store.SaveSessionWithTelemetry("proj-1", summary, delta)
	require.NoError(t, err)

	// Session should be persisted
	loaded, err := store.LoadSessionSummary("proj-1", "sess-1")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "sess-1", loaded.SessionID)
	assert.Equal(t, int64(1200), loaded.TokensSaved)

	// Telemetry should match delta
	telem, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	require.NotNil(t, telem)
	assert.Equal(t, int64(1200), telem.TokensSaved)
	assert.Equal(t, int64(9600), telem.TimeSavedMs)
	assert.Equal(t, 5, telem.Reads)
	assert.Equal(t, 3, telem.GuidedReads)
	assert.Equal(t, 1, telem.Sessions)
	assert.Equal(t, 10, telem.Prompts)
	assert.Equal(t, int64(5000), telem.InputTokens)
	assert.Equal(t, int64(2000), telem.OutputTokens)
	assert.Equal(t, int64(3000), telem.CacheReadTokens)
	assert.Equal(t, int64(400), telem.ShadowSaved)
	assert.Equal(t, int64(1700000000), telem.FirstSessionAt)
}

func TestTelemetry_AccumulatesAcrossSessions(t *testing.T) {
	// Three sessions with deltas → telemetry is the sum.
	store, _ := newTestStore(t)

	sessions := []struct {
		id          string
		tokensSaved int64
		reads       int
		guided      int
	}{
		{"sess-1", 1000, 5, 3},
		{"sess-2", 2000, 8, 5},
		{"sess-3", 500, 2, 1},
	}

	for _, s := range sessions {
		summary := &ports.SessionSummary{
			SessionID:       s.id,
			StartTime:       1700000000,
			ReadCount:       s.reads,
			GuidedReadCount: s.guided,
			TokensSaved:     s.tokensSaved,
		}
		delta := &ports.ProjectTelemetry{
			TokensSaved: s.tokensSaved,
			Reads:       s.reads,
			GuidedReads: s.guided,
			Sessions:    1,
		}
		err := store.SaveSessionWithTelemetry("proj-1", summary, delta)
		require.NoError(t, err)
	}

	telem, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)

	assert.Equal(t, int64(3500), telem.TokensSaved, "1000+2000+500")
	assert.Equal(t, 15, telem.Reads, "5+8+2")
	assert.Equal(t, 9, telem.GuidedReads, "3+5+1")
	assert.Equal(t, 3, telem.Sessions, "3 sessions")
}

func TestTelemetry_MultiFlushSameSession_DeltaOnly(t *testing.T) {
	// Flush the same session 3 times with increasing absolute counters.
	// Each flush sends only the DELTA (diff since last flush).
	// This simulates periodic flushes during a long session.
	store, _ := newTestStore(t)

	// First flush: absolute 100 tokens saved, delta = 100
	s := &ports.SessionSummary{SessionID: "sess-1", TokensSaved: 100, ReadCount: 3}
	d := &ports.ProjectTelemetry{TokensSaved: 100, Reads: 3, Sessions: 1}
	require.NoError(t, store.SaveSessionWithTelemetry("proj-1", s, d))

	// Second flush: absolute 250 tokens saved, delta = 150
	s = &ports.SessionSummary{SessionID: "sess-1", TokensSaved: 250, ReadCount: 7}
	d = &ports.ProjectTelemetry{TokensSaved: 150, Reads: 4}
	require.NoError(t, store.SaveSessionWithTelemetry("proj-1", s, d))

	// Third flush: absolute 400 tokens saved, delta = 150
	s = &ports.SessionSummary{SessionID: "sess-1", TokensSaved: 400, ReadCount: 10}
	d = &ports.ProjectTelemetry{TokensSaved: 150, Reads: 3}
	require.NoError(t, store.SaveSessionWithTelemetry("proj-1", s, d))

	// Telemetry should be sum of deltas: 100+150+150 = 400
	telem, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	assert.Equal(t, int64(400), telem.TokensSaved, "sum of deltas, not sum of absolutes")
	assert.Equal(t, 10, telem.Reads, "sum of deltas: 3+4+3")
	assert.Equal(t, 1, telem.Sessions, "still 1 session")

	// Session should have latest absolute values
	loaded, err := store.LoadSessionSummary("proj-1", "sess-1")
	require.NoError(t, err)
	assert.Equal(t, int64(400), loaded.TokensSaved, "session has final absolute value")
	assert.Equal(t, 10, loaded.ReadCount, "session has final absolute value")
}

func TestTelemetry_Backfill_FromExistingSessions(t *testing.T) {
	// Sessions saved via old SaveSessionSummary (no telemetry bucket).
	// First LoadTelemetry should backfill from session data.
	store, _ := newTestStore(t)

	// Save sessions the old way (no telemetry)
	s1 := &ports.SessionSummary{
		SessionID:       "sess-old-1",
		StartTime:       1700000000,
		PromptCount:     20,
		ReadCount:       10,
		GuidedReadCount: 6,
		TokensSaved:     3000,
		TimeSavedMs:     24000,
		InputTokens:     50000,
		OutputTokens:    20000,
		CacheReadTokens: 30000,
	}
	s2 := &ports.SessionSummary{
		SessionID:       "sess-old-2",
		StartTime:       1700100000,
		PromptCount:     15,
		ReadCount:       8,
		GuidedReadCount: 4,
		TokensSaved:     2000,
		TimeSavedMs:     16000,
		InputTokens:     40000,
		OutputTokens:    15000,
		CacheReadTokens: 25000,
	}

	require.NoError(t, store.SaveSessionSummary("proj-1", s1))
	require.NoError(t, store.SaveSessionSummary("proj-1", s2))

	// Now LoadTelemetry triggers backfill
	telem, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	require.NotNil(t, telem)

	assert.Equal(t, int64(5000), telem.TokensSaved, "3000+2000")
	assert.Equal(t, int64(40000), telem.TimeSavedMs, "24000+16000")
	assert.Equal(t, 18, telem.Reads, "10+8")
	assert.Equal(t, 10, telem.GuidedReads, "6+4")
	assert.Equal(t, 2, telem.Sessions, "2 sessions")
	assert.Equal(t, 35, telem.Prompts, "20+15")
	assert.Equal(t, int64(90000), telem.InputTokens, "50000+40000")
	assert.Equal(t, int64(35000), telem.OutputTokens, "20000+15000")
	assert.Equal(t, int64(55000), telem.CacheReadTokens, "30000+25000")
	assert.Equal(t, int64(1700000000), telem.FirstSessionAt, "earliest start_time")
}

func TestTelemetry_Backfill_Idempotent(t *testing.T) {
	// Backfill happens once. Second LoadTelemetry returns same values.
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveSessionSummary("proj-1", &ports.SessionSummary{
		SessionID:   "sess-1",
		StartTime:   1700000000,
		TokensSaved: 1000,
		ReadCount:   5,
	}))

	telem1, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1000), telem1.TokensSaved)

	// Second load — no re-backfill, same values
	telem2, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1000), telem2.TokensSaved)
	assert.Equal(t, 5, telem2.Reads)
}

func TestTelemetry_Backfill_ThenNewSession(t *testing.T) {
	// Backfill from old sessions, then add a new session via SaveSessionWithTelemetry.
	// Lifetime = backfilled + new delta.
	store, _ := newTestStore(t)

	// Old session
	require.NoError(t, store.SaveSessionSummary("proj-1", &ports.SessionSummary{
		SessionID:   "sess-old",
		StartTime:   1700000000,
		TokensSaved: 1000,
		ReadCount:   5,
		PromptCount: 10,
	}))

	// Trigger backfill
	telem, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1000), telem.TokensSaved)

	// New session via atomic save
	newSummary := &ports.SessionSummary{
		SessionID:   "sess-new",
		StartTime:   1700100000,
		TokensSaved: 500,
		ReadCount:   3,
		PromptCount: 8,
	}
	newDelta := &ports.ProjectTelemetry{
		TokensSaved: 500,
		Reads:       3,
		Sessions:    1,
		Prompts:     8,
	}
	require.NoError(t, store.SaveSessionWithTelemetry("proj-1", newSummary, newDelta))

	// Lifetime should be backfilled + new
	telem, err = store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1500), telem.TokensSaved, "1000 backfilled + 500 new")
	assert.Equal(t, 8, telem.Reads, "5 backfilled + 3 new")
	assert.Equal(t, 2, telem.Sessions, "1 backfilled + 1 new")
	assert.Equal(t, 18, telem.Prompts, "10 backfilled + 8 new")
}

func TestTelemetry_SessionPruning_PreservesLifetime(t *testing.T) {
	// This is the whole point of L17: after session pruning, lifetime totals survive.
	// Save 205 sessions (exceeds maxSessionRetention=200), verify telemetry keeps all counts.
	store, _ := newTestStore(t)

	var expectedTokensSaved int64
	for i := 0; i < 205; i++ {
		saved := int64((i + 1) * 100)
		expectedTokensSaved += saved

		summary := &ports.SessionSummary{
			SessionID:   fmt.Sprintf("sess-%04d", i),
			StartTime:   int64(1700000000 + i*3600),
			TokensSaved: saved,
			ReadCount:   1,
		}
		delta := &ports.ProjectTelemetry{
			TokensSaved: saved,
			Reads:       1,
			Sessions:    1,
		}
		require.NoError(t, store.SaveSessionWithTelemetry("proj-1", summary, delta))
	}

	// Only 200 sessions should remain (5 pruned)
	sessions, err := store.ListSessionSummaries("proj-1")
	require.NoError(t, err)
	assert.Len(t, sessions, maxSessionRetention, "pruned to 200")

	// But telemetry retains all 205 sessions' worth of data
	telem, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	assert.Equal(t, expectedTokensSaved, telem.TokensSaved, "lifetime survives pruning")
	assert.Equal(t, 205, telem.Reads, "lifetime reads survive pruning")
	assert.Equal(t, 205, telem.Sessions, "lifetime session count survives pruning")
}

func TestTelemetry_FirstSessionAt(t *testing.T) {
	// FirstSessionAt should be set on the first delta and never overwritten.
	store, _ := newTestStore(t)

	d1 := &ports.ProjectTelemetry{TokensSaved: 100, Sessions: 1, FirstSessionAt: 1700000000}
	require.NoError(t, store.SaveSessionWithTelemetry("proj-1",
		&ports.SessionSummary{SessionID: "s1", StartTime: 1700000000}, d1))

	d2 := &ports.ProjectTelemetry{TokensSaved: 200, Sessions: 1, FirstSessionAt: 1700100000}
	require.NoError(t, store.SaveSessionWithTelemetry("proj-1",
		&ports.SessionSummary{SessionID: "s2", StartTime: 1700100000}, d2))

	telem, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1700000000), telem.FirstSessionAt,
		"FirstSessionAt should be earliest, not overwritten by later sessions")
}

func TestTelemetry_DeleteProject_ClearsTelemetry(t *testing.T) {
	// DeleteProject removes telemetry along with everything else.
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveSessionWithTelemetry("proj-1",
		&ports.SessionSummary{SessionID: "s1", TokensSaved: 1000},
		&ports.ProjectTelemetry{TokensSaved: 1000, Sessions: 1}))

	telem, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1000), telem.TokensSaved)

	// Delete project
	require.NoError(t, store.DeleteProject("proj-1"))

	// Telemetry is gone
	telem, err = store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), telem.TokensSaved)
	assert.Equal(t, 0, telem.Sessions)
}

func TestTelemetry_ProjectScoped(t *testing.T) {
	// Two projects have independent telemetry.
	store, _ := newTestStore(t)

	require.NoError(t, store.SaveSessionWithTelemetry("proj-A",
		&ports.SessionSummary{SessionID: "sA", TokensSaved: 1000},
		&ports.ProjectTelemetry{TokensSaved: 1000, Sessions: 1}))

	require.NoError(t, store.SaveSessionWithTelemetry("proj-B",
		&ports.SessionSummary{SessionID: "sB", TokensSaved: 2000},
		&ports.ProjectTelemetry{TokensSaved: 2000, Sessions: 1}))

	telemA, err := store.LoadTelemetry("proj-A")
	require.NoError(t, err)
	assert.Equal(t, int64(1000), telemA.TokensSaved)

	telemB, err := store.LoadTelemetry("proj-B")
	require.NoError(t, err)
	assert.Equal(t, int64(2000), telemB.TokensSaved)
}

func TestTelemetry_NilDelta(t *testing.T) {
	// Nil delta should save session but not modify telemetry.
	store, _ := newTestStore(t)

	err := store.SaveSessionWithTelemetry("proj-1",
		&ports.SessionSummary{SessionID: "s1", TokensSaved: 500}, nil)
	require.NoError(t, err)

	// Session saved
	loaded, err := store.LoadSessionSummary("proj-1", "s1")
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// Telemetry unchanged (zero)
	telem, err := store.LoadTelemetry("proj-1")
	require.NoError(t, err)
	assert.Equal(t, int64(0), telem.TokensSaved)
}

func TestTelemetry_NilSummary_Errors(t *testing.T) {
	store, _ := newTestStore(t)

	err := store.SaveSessionWithTelemetry("proj-1", nil, &ports.ProjectTelemetry{})
	assert.Error(t, err, "nil summary should return error")
}

func TestTelemetry_JSON_Roundtrip(t *testing.T) {
	// Verify ProjectTelemetry serializes cleanly to JSON and back.
	original := &ports.ProjectTelemetry{
		TokensSaved:     12345,
		TimeSavedMs:     98760,
		Reads:           42,
		GuidedReads:     28,
		Sessions:        7,
		Prompts:         150,
		InputTokens:     500000,
		OutputTokens:    200000,
		CacheReadTokens: 300000,
		ShadowSaved:     8000,
		FirstSessionAt:  1700000000,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var loaded ports.ProjectTelemetry
	require.NoError(t, json.Unmarshal(data, &loaded))

	assert.Equal(t, original.TokensSaved, loaded.TokensSaved)
	assert.Equal(t, original.TimeSavedMs, loaded.TimeSavedMs)
	assert.Equal(t, original.Reads, loaded.Reads)
	assert.Equal(t, original.GuidedReads, loaded.GuidedReads)
	assert.Equal(t, original.Sessions, loaded.Sessions)
	assert.Equal(t, original.Prompts, loaded.Prompts)
	assert.Equal(t, original.InputTokens, loaded.InputTokens)
	assert.Equal(t, original.OutputTokens, loaded.OutputTokens)
	assert.Equal(t, original.CacheReadTokens, loaded.CacheReadTokens)
	assert.Equal(t, original.ShadowSaved, loaded.ShadowSaved)
	assert.Equal(t, original.FirstSessionAt, loaded.FirstSessionAt)
}
