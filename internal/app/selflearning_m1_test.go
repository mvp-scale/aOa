package app

// M1 — close the capture leaks. Real session signal that Claude produces must reach
// the learner map, through the SAME proven funnel (processConversationSignal, observe=false)
// that AIThinking/AIResponse already use — no new store, no new extractor, no autotune
// off-cadence (observe=false never bumps promptN).
//
// Consensus plan 2026-07-10 (self-learning-close-the-loop), milestone M1.

import (
	"testing"
	"time"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
)

// TestM1_AwaySummary_FeedsLearner: the resume-summary text (documented "useful as a
// learner intent signal") must reach the learner. Today the EventSystemMeta case reads
// only DurationMs and drops AwaySummary — so this is RED until M1.1 routes it.
func TestM1_AwaySummary_FeedsLearner(t *testing.T) {
	a := newTestAppWithStore(t)
	promptBefore := a.promptN

	ev := ports.SessionEvent{
		Kind:        ports.EventSystemMeta,
		TurnID:      "t1",
		Timestamp:   time.Now(),
		AwaySummary: "resume work on the authentication middleware and database schema",
	}
	a.onSessionEvent(ev)

	state := a.Learner.State()
	assert.Greater(t, len(state.KeywordHits), 0,
		"AwaySummary atlas keywords must reach the learner (currently dropped)")
	assert.Greater(t, len(state.CohitTermDomain), 0,
		"AwaySummary must form term→domain cohits like any other conversation text")
	assert.Equal(t, promptBefore, a.promptN,
		"observe=false: capturing AwaySummary must NOT increment promptN or fire autotune off-cadence")
}
