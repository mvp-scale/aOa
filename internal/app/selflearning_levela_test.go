package app

// Level A — the mechanism oracle for self-learning validation.
//
// Brief: details/2026-07-10-test-plan-self-learning-validation.md (Level A).
// Fidelity ruling (owner): raw-text simulation — synthetic conversational text is
// fed through the REAL pipeline (enricher.Lookup → signalCollector → Observe →
// RunMathTune → runDedup), never pre-resolved events. No daemon, no live Claude.
//
// Known-answer fixture (verified against atlas/v1):
//   - the term "execution" belongs to EXACTLY two domains: state_machine, graphql
//     (atlas/v1/09-architecture-patterns.json + 12-ml-datascience graphql).
//   - keyword "interpret" resolves execution→state_machine (+ effects→functional,
//     a different entity, irrelevant to the execution race).
//   - keyword "cost"      resolves execution→graphql (clean: single pair).
// So feeding "interpret" N times and "cost" M times drives the cohit race for the
// entity "execution" to N:M with a KNOWN winner. dedup requires the entity's total
// cohits ≥ DedupMinTotal (100) and ≥2 containers before it elects a winner.

import (
	"encoding/json"
	"testing"

	"github.com/corey/aoa/internal/domain/learner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	smWord = "interpret" // execution → state_machine
	gqWord = "cost"      // execution → graphql
)

// feedConversation feeds the same synthetic turn n times through the real
// conversation-capture path with observe=false (accumulates cohits, no autotune).
func feedConversation(a *App, text string, n int) {
	a.promptN = 1
	for i := 0; i < n; i++ {
		a.processConversationSignal(text, false)
	}
}

// triggerAutotune fires ONE real autotune cycle through the pipeline by landing an
// observe=true turn on the autotune-interval boundary (promptN % 50 == 0). The
// winning side is fed so the extra turn never changes the elected winner.
func triggerAutotune(a *App, winnerText string) {
	a.promptN = AutotuneIntervalForTest
	a.processConversationSignal(winnerText, true)
}

// AutotuneIntervalForTest mirrors learner.AutotuneInterval (50). Kept local so the
// test reads as a known-answer fixture rather than importing the constant it asserts.
const AutotuneIntervalForTest = 50

// TestLevelA1_DedupElectsProjectSpecificWinner is the core self-improvement oracle:
// when THIS project uses "execution" as a state_machine concept ~3× more than as a
// graphql concept, dedup must elect state_machine and DELETE the graphql edge — the
// term→domain affinity flips to the project-specific winner.
func TestLevelA1_DedupElectsProjectSpecificWinner(t *testing.T) {
	a := newTestAppWithStore(t)

	// Simulate the project: execution-as-state_machine 3× execution-as-graphql.
	feedConversation(a, smWord, 90)
	feedConversation(a, gqWord, 30)

	// Pre-condition: both affinities present, state_machine ahead 90:30. If this
	// fails, the raw-text→enricher→observe path itself is broken (not dedup).
	pre := a.Learner.State()
	require.Equal(t, uint32(90), pre.CohitTermDomain["execution:state_machine"],
		"raw-text capture should have built execution:state_machine to 90")
	require.Equal(t, uint32(30), pre.CohitTermDomain["execution:graphql"],
		"raw-text capture should have built execution:graphql to 30")

	// One real autotune cycle → dedup runs.
	triggerAutotune(a, smWord)

	post := a.Learner.State()

	// THE FLIP: the project-specific winner survives, the loser edge is deleted.
	_, graphqlKept := post.CohitTermDomain["execution:graphql"]
	assert.False(t, graphqlKept,
		"dedup must DELETE the losing execution:graphql edge (the affinity flips to the project winner)")
	assert.Greater(t, post.CohitTermDomain["execution:state_machine"], uint32(0),
		"dedup must KEEP the winning execution:state_machine edge")
}

// TestLevelA1_WinnerFollowsTheData is the control: reverse the usage ratio and the
// elected winner reverses too. Proves the winner is driven by project co-occurrence,
// not by atlas ordering or a hardcoded default.
func TestLevelA1_WinnerFollowsTheData(t *testing.T) {
	a := newTestAppWithStore(t)

	// Reversed project: execution-as-graphql 3× execution-as-state_machine.
	feedConversation(a, gqWord, 90)
	feedConversation(a, smWord, 30)

	triggerAutotune(a, gqWord)

	post := a.Learner.State()

	_, smKept := post.CohitTermDomain["execution:state_machine"]
	assert.False(t, smKept,
		"reversed usage must DELETE execution:state_machine (winner follows the data)")
	assert.Greater(t, post.CohitTermDomain["execution:graphql"], uint32(0),
		"reversed usage must KEEP execution:graphql as the elected winner")
}

// emptyAutotuneCycle fires ONE real autotune (RunMathTune) at the given prompt
// boundary with no new observe signal — the same RunMathTune the pipeline invokes,
// used to age domains across cycles without feeding them.
func emptyAutotuneCycle(a *App, prompt uint32) {
	a.Learner.ObserveAndMaybeTune(learner.ObserveEvent{PromptNumber: prompt})
}

// TestLevelA2_Promotion: a domain used repeatedly by THIS project is promoted
// context→core and stays active. This is the "self-improvement lifts what matters"
// half of the lifecycle, and it holds.
func TestLevelA2_Promotion(t *testing.T) {
	a := newTestAppWithStore(t)

	// Use state_machine heavily across two autotune cycles. (Two cycles matter:
	// the first autotune snapshots HitsLastCycle from 0 and transiently flags any
	// domain stale; a domain fed again in the next cycle resolves to active.)
	feedConversation(a, smWord, 60)
	triggerAutotune(a, smWord) // prompt 50
	feedConversation(a, smWord, 60)
	a.promptN = 100
	a.processConversationSignal(smWord, true) // prompt 100 → second autotune

	dm := a.Learner.State().DomainMeta["state_machine"]
	require.NotNil(t, dm, "heavily-used domain must exist")
	assert.Equal(t, "core", dm.Tier, "a repeatedly-used domain is promoted to core")
	assert.Equal(t, "active", dm.State, "a domain used again next cycle resolves to active")
}

// TestLevelA2_AgingHorizon_Characterization pins the ACTUAL aging behavior of the
// Go learner, which DIVERGES from the horizon described in the PRD/test-plan
// ("active→stale(50)→deprecated(100)→pruned(~580)").
//
// Real behavior (verified): a domain observed once is flagged `stale` on the FIRST
// autotune, then step-4 reactivates it (because step-7 snapshots HitsLastCycle from
// the cumulative, float64-decaying Hits, which never returns to exactly 0). It
// therefore stays `active`/`core` across many cycles, decaying 0.9^n asymptotically,
// and is NEVER `deprecated` and NEVER pruned while it holds a core rank — even once
// its hits fall below PruneFloor (0.3). Aging out requires DISPLACEMENT (≥24 domains
// outranking it), not the documented state-machine progression.
//
// This test is a REGRESSION GUARD on that real behavior and a FLAG on the divergence
// (see session report / finding). If the lifecycle is ever changed to actually age
// observed domains to deprecated, this test should be revisited deliberately.
func TestLevelA2_AgingHorizon_Characterization(t *testing.T) {
	a := newTestAppWithStore(t)

	// Seed one domain a single time, never feed it again.
	a.promptN = 1
	a.processConversationSignal(smWord, false)

	// Cycle 1 (prompt 50): flagged stale exactly once.
	emptyAutotuneCycle(a, 50)
	dm := a.Learner.State().DomainMeta["state_machine"]
	require.NotNil(t, dm)
	assert.Equal(t, "stale", dm.State, "first autotune flags an unfed domain stale")
	assert.Equal(t, uint32(1), dm.StaleCycles, "stale_cycles increments to 1")

	// Cycles 2..12 (prompt 100..600): reactivates and stays active — NEVER deprecated,
	// NEVER pruned, even past the documented deprecated(100)/pruned(~580) horizon.
	for cyc := 2; cyc <= 12; cyc++ {
		emptyAutotuneCycle(a, uint32(50*cyc))
		dm = a.Learner.State().DomainMeta["state_machine"]
		require.NotNil(t, dm, "core domain is NOT pruned by the aging path (cyc %d)", cyc)
		assert.Equal(t, "active", dm.State,
			"observed domain reactivates and stays active — never reaches deprecated (cyc %d)", cyc)
	}

	// By prompt 600 its hits have decayed below PruneFloor yet it survives as core.
	assert.Less(t, dm.Hits, PruneFloorForTest,
		"hits have decayed below the prune floor")
	assert.Equal(t, "core", dm.Tier,
		"yet it remains core (a lone/top domain is never displaced, so never pruned)")
}

// PruneFloorForTest mirrors learner.PruneFloor (0.3) as a known-answer constant.
const PruneFloorForTest = 0.3

// TestLevelA3_Determinism asserts the learner is a pure function of its feed:
// the same simulated conversation fed twice yields byte-identical persisted state.
// This is the regression guarantee that drift can no longer silently perturb learning.
func TestLevelA3_Determinism(t *testing.T) {
	run := func() []byte {
		a := newTestAppWithStore(t)
		feedConversation(a, smWord, 90)
		feedConversation(a, gqWord, 30)
		triggerAutotune(a, smWord)
		b, err := json.Marshal(a.Learner.State())
		require.NoError(t, err)
		return b
	}

	first := run()
	second := run()

	assert.Equal(t, string(first), string(second),
		"identical simulated feed must produce byte-identical learner state (determinism)")
}
