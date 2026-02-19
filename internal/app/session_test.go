package app

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestAppWithStore creates a test app with a real bbolt store for session tests.
func newTestAppWithStore(t *testing.T) *App {
	t.Helper()
	a := newTestApp(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	store, err := bbolt.NewStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	a.Store = store
	return a
}

func TestSessionBoundary_NewSession(t *testing.T) {
	a := newTestAppWithStore(t)

	ev := ports.SessionEvent{
		Kind:      ports.EventUserInput,
		SessionID: "sess-1",
		TurnID:    "turn-1",
		Timestamp: time.Now(),
		Text:      "hello",
	}
	a.onSessionEvent(ev)

	assert.Equal(t, "sess-1", a.currentSessionID)
	assert.Equal(t, 1, a.sessionPrompts)
}

func TestSessionBoundary_SameSession_NoOp(t *testing.T) {
	a := newTestAppWithStore(t)

	// First event sets session
	ev1 := ports.SessionEvent{
		Kind:      ports.EventUserInput,
		SessionID: "sess-1",
		TurnID:    "turn-1",
		Timestamp: time.Now(),
		Text:      "hello",
	}
	a.onSessionEvent(ev1)

	// Second event same session — no reset
	ev2 := ports.SessionEvent{
		Kind:      ports.EventUserInput,
		SessionID: "sess-1",
		TurnID:    "turn-2",
		Timestamp: time.Now(),
		Text:      "world",
	}
	a.onSessionEvent(ev2)

	assert.Equal(t, "sess-1", a.currentSessionID)
	assert.Equal(t, 2, a.sessionPrompts)
}

func TestSessionBoundary_DifferentSession_Flushes(t *testing.T) {
	a := newTestAppWithStore(t)

	// First session
	ev1 := ports.SessionEvent{
		Kind:      ports.EventUserInput,
		SessionID: "sess-1",
		TurnID:    "turn-1",
		Timestamp: time.Now(),
		Text:      "hello",
	}
	a.onSessionEvent(ev1)
	a.onSessionEvent(ports.SessionEvent{
		Kind:      ports.EventUserInput,
		SessionID: "sess-1",
		TurnID:    "turn-2",
		Timestamp: time.Now(),
		Text:      "more work",
	})
	assert.Equal(t, 2, a.sessionPrompts)

	// Switch to new session — should flush sess-1 and reset
	ev2 := ports.SessionEvent{
		Kind:      ports.EventUserInput,
		SessionID: "sess-2",
		TurnID:    "turn-3",
		Timestamp: time.Now(),
		Text:      "new session",
	}
	a.onSessionEvent(ev2)

	assert.Equal(t, "sess-2", a.currentSessionID)
	assert.Equal(t, 1, a.sessionPrompts) // reset for new session

	// Verify sess-1 was persisted
	summary, err := a.Store.LoadSessionSummary(a.ProjectID, "sess-1")
	require.NoError(t, err)
	require.NotNil(t, summary, "sess-1 should be persisted after boundary change")
	assert.Equal(t, 2, summary.PromptCount)
}

func TestSessionBoundary_Revisit_RestoresCounters(t *testing.T) {
	a := newTestAppWithStore(t)

	// Build up sess-1
	a.onSessionEvent(ports.SessionEvent{
		Kind:      ports.EventUserInput,
		SessionID: "sess-1",
		TurnID:    "turn-1",
		Timestamp: time.Now(),
		Text:      "hello",
	})
	a.onSessionEvent(ports.SessionEvent{
		Kind:      ports.EventUserInput,
		SessionID: "sess-1",
		TurnID:    "turn-2",
		Timestamp: time.Now(),
		Text:      "more",
	})
	assert.Equal(t, 2, a.sessionPrompts)

	// Switch to sess-2
	a.onSessionEvent(ports.SessionEvent{
		Kind:      ports.EventUserInput,
		SessionID: "sess-2",
		TurnID:    "turn-3",
		Timestamp: time.Now(),
		Text:      "new",
	})

	// Switch back to sess-1 — should restore counters
	a.onSessionEvent(ports.SessionEvent{
		Kind:      ports.EventUserInput,
		SessionID: "sess-1",
		TurnID:    "turn-4",
		Timestamp: time.Now(),
		Text:      "back",
	})

	assert.Equal(t, "sess-1", a.currentSessionID)
	// Restored 2 from persisted + 1 new = 3
	assert.Equal(t, 3, a.sessionPrompts)
}

func TestSessionBoundary_EmptySessionID_Ignored(t *testing.T) {
	a := newTestAppWithStore(t)

	// Event with no session ID — should not trigger boundary logic
	ev := ports.SessionEvent{
		Kind:      ports.EventUserInput,
		TurnID:    "turn-1",
		Timestamp: time.Now(),
		Text:      "hello",
	}
	a.onSessionEvent(ev)

	assert.Equal(t, "", a.currentSessionID)
	assert.Equal(t, 1, a.sessionPrompts) // promptN++ still happens
}
