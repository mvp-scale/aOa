package learner

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureDir is the path to learner fixture files.
const fixtureDir = "../../../test/fixtures/learner/"

func loadFixture(t *testing.T, name string) *ports.LearnerState {
	t.Helper()
	data, err := os.ReadFile(fixtureDir + name)
	require.NoError(t, err, "read fixture %s", name)
	var state ports.LearnerState
	require.NoError(t, json.Unmarshal(data, &state), "parse fixture %s", name)
	return &state
}

func loadEvents(t *testing.T, name string) []ObserveEvent {
	t.Helper()
	data, err := os.ReadFile(fixtureDir + name)
	require.NoError(t, err, "read events %s", name)
	var events []ObserveEvent
	require.NoError(t, json.Unmarshal(data, &events), "parse events %s", name)
	return events
}

// =============================================================================
// L-01: Learner Domain — Core state machine
// =============================================================================

func TestNewLearner_FreshState(t *testing.T) {
	l := New()
	assert.Equal(t, uint32(0), l.PromptCount())
	assert.Empty(t, l.state.KeywordHits)
	assert.Empty(t, l.state.TermHits)
	assert.Empty(t, l.state.DomainMeta)
	assert.Empty(t, l.state.CohitKwTerm)
	assert.Empty(t, l.state.CohitTermDomain)
	assert.Empty(t, l.state.Bigrams)
	assert.Empty(t, l.state.FileHits)
	assert.Empty(t, l.state.KeywordBlocklist)
	assert.Empty(t, l.state.GapKeywords)
}

func TestNewLearner_FromSnapshot(t *testing.T) {
	fixture := loadFixture(t, "01-fifty-intents.json")
	l := NewFromState(fixture)
	assert.Equal(t, uint32(50), l.PromptCount())
	assert.Equal(t, uint32(39), l.state.KeywordHits["login"])
	assert.InDelta(t, 27.0, l.state.DomainMeta["@authentication"].Hits, 0.001)
}

func TestLearnerSnapshot_Deterministic(t *testing.T) {
	fixture := loadFixture(t, "01-fifty-intents.json")
	l := NewFromState(fixture)
	snap1, err1 := l.Snapshot()
	require.NoError(t, err1)
	snap2, err2 := l.Snapshot()
	require.NoError(t, err2)
	assert.Equal(t, snap1, snap2, "two snapshots of same state should be identical")
}

func TestLearnerSnapshot_Roundtrip(t *testing.T) {
	fixture := loadFixture(t, "01-fifty-intents.json")
	l := NewFromState(fixture)
	snap, err := l.Snapshot()
	require.NoError(t, err)

	l2, err := NewFromJSON(snap)
	require.NoError(t, err)
	assert.Equal(t, l.state.PromptCount, l2.state.PromptCount)
	assert.Equal(t, l.state.KeywordHits, l2.state.KeywordHits)
	assert.InDelta(t, l.state.DomainMeta["@authentication"].Hits,
		l2.state.DomainMeta["@authentication"].Hits, 0.001)
}

func TestLearnerState_AllMapsInitialized(t *testing.T) {
	l := New()
	s := l.State()
	assert.NotNil(t, s.KeywordHits)
	assert.NotNil(t, s.TermHits)
	assert.NotNil(t, s.DomainMeta)
	assert.NotNil(t, s.CohitKwTerm)
	assert.NotNil(t, s.CohitTermDomain)
	assert.NotNil(t, s.Bigrams)
	assert.NotNil(t, s.FileHits)
	assert.NotNil(t, s.KeywordBlocklist)
	assert.NotNil(t, s.GapKeywords)
}

func TestLearnerPromptCount_Increments(t *testing.T) {
	fixture := loadFixture(t, "00-fresh.json")
	l := NewFromState(fixture)
	events := loadEvents(t, "events-01-to-50.json")
	for _, ev := range events[:10] {
		l.Observe(ev)
	}
	assert.Equal(t, uint32(10), l.PromptCount())
}

func TestLearnerAutotune_TriggersAt50(t *testing.T) {
	fixture := loadFixture(t, "00-fresh.json")
	l := NewFromState(fixture)
	events := loadEvents(t, "events-01-to-50.json")

	// At 49: no autotune effect (domain hits still raw)
	for _, ev := range events[:49] {
		l.Observe(ev)
	}
	// @authentication should have raw hits (not decayed)
	authHits49 := l.state.DomainMeta["@authentication"].Hits
	assert.Greater(t, authHits49, 20.0, "should have accumulated raw hits")

	// At 50: ObserveAndMaybeTune triggers autotune
	l.ObserveAndMaybeTune(events[49])
	authHits50 := l.state.DomainMeta["@authentication"].Hits
	// After decay: hits should be less than raw (multiplied by 0.90)
	assert.Less(t, authHits50, authHits49, "autotune should have decayed hits")
}
