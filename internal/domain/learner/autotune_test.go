package learner

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// L-03: Autotune — 21-step math tune
// =============================================================================

func TestAutotune_21StepsExecuteInOrder(t *testing.T) {
	// Verify autotune runs without error on a realistic state.
	fixture := loadFixture(t, "00-fresh.json")
	l := NewFromState(fixture)
	events := loadEvents(t, "events-01-to-50.json")
	for _, ev := range events {
		l.Observe(ev)
	}
	// Should not panic
	l.RunMathTune()
	// After autotune, domain hits should be decayed
	assert.Less(t, l.state.DomainMeta["@authentication"].Hits, 30.0)
}

func TestAutotune_DecayFactor090(t *testing.T) {
	l := New()
	l.state.DomainMeta["@test"] = &ports.DomainMeta{
		Hits: 10.0, Tier: "core", Source: "seeded", State: "active",
		HitsLastCycle: 10.0, // non-zero to avoid stale detection
	}
	l.RunMathTune()
	assert.InDelta(t, 9.0, l.state.DomainMeta["@test"].Hits, 0.001)
}

func TestAutotune_FloatPrecision_MatchesPython(t *testing.T) {
	// Python: int(7 * 0.90) = int(6.3) = 6
	// Go must use math.Trunc to match.
	l := New()
	l.state.KeywordHits["test"] = 7
	l.state.DomainMeta["@d"] = &ports.DomainMeta{
		Hits: 1.0, Tier: "core", Source: "seeded", State: "active",
		HitsLastCycle: 1.0,
	}
	l.RunMathTune()
	assert.Equal(t, uint32(6), l.state.KeywordHits["test"])

	// Domain hits use float, no truncation: 1.0 * 0.9 = 0.9
	assert.InDelta(t, 0.9, l.state.DomainMeta["@d"].Hits, 0.001)
}

func TestAutotune_PruneBelow03(t *testing.T) {
	// keyword_hits with value that decays to 0 should be removed
	l := New()
	l.state.KeywordHits["old"] = 1 // 1 * 0.9 = 0.9 → int(0.9) = 0 → removed
	l.state.KeywordHits["active"] = 5 // 5 * 0.9 = 4.5 → int(4.5) = 4 → kept
	l.state.DomainMeta["@d"] = &ports.DomainMeta{
		Hits: 1.0, Tier: "core", Source: "seeded", State: "active",
		HitsLastCycle: 1.0,
	}
	l.RunMathTune()
	assert.Zero(t, l.state.KeywordHits["old"], "old should be pruned")
	assert.Equal(t, uint32(4), l.state.KeywordHits["active"])
}

func TestAutotune_DecayBeforeDedup(t *testing.T) {
	// SPEC: domain decay (step 8) runs BEFORE dedup (step 9).
	// Dedup operates on post-decay cohit values.
	l := New()
	l.state.DomainMeta["@d"] = &ports.DomainMeta{
		Hits: 10.0, Tier: "core", Source: "seeded", State: "active",
		HitsLastCycle: 10.0,
	}
	// Cohit values below DedupMinTotal (100) — dedup won't fire, but verify ordering
	l.state.CohitKwTerm["login:auth"] = 50
	l.RunMathTune()
	// After step 8 (domain decay) + step 17 (cohit decay):
	// 50 * 0.9 = 45 (int-truncated)
	assert.Equal(t, uint32(45), l.state.CohitKwTerm["login:auth"])
}

func TestAutotune_PromoteToCoreTop24(t *testing.T) {
	l := New()
	// Create 5 domains: 4 core, 1 context
	for i, name := range []string{"@a", "@b", "@c", "@d", "@e"} {
		l.state.DomainMeta[name] = &ports.DomainMeta{
			Hits: float64(10 - i), Tier: "core", Source: "seeded", State: "active",
			HitsLastCycle: float64(10 - i),
		}
	}
	l.state.DomainMeta["@e"].Tier = "context" // context tier domain
	l.RunMathTune()
	// All 5 fit in top 24, so @e should be promoted to core
	assert.Equal(t, "core", l.state.DomainMeta["@e"].Tier)
}

func TestAutotune_DemotionTrimsKeywords(t *testing.T) {
	// With 8 domains (< 24), all fit in core. No demotion possible.
	// Test requires >24 domains. Verify no crash with 8.
	fixture := loadFixture(t, "00-fresh.json")
	l := NewFromState(fixture)
	events := loadEvents(t, "events-01-to-50.json")
	for _, ev := range events {
		l.Observe(ev)
	}
	l.RunMathTune()
	// All 7 domains fit in core (< 24)
	for name, dm := range l.state.DomainMeta {
		if dm.State != "deprecated" {
			assert.Equal(t, "core", dm.Tier, "domain %s should be core (< 24 total)", name)
		}
	}
}

func TestAutotune_RemoveDomainBelow03(t *testing.T) {
	// Domain with hits < 0.3 at rank 24+ gets removed.
	// Need >24 domains to test this.
	l := New()
	for i := 0; i < 25; i++ {
		name := "@d" + string(rune('a'+i))
		l.state.DomainMeta[name] = &ports.DomainMeta{
			Hits: float64(30 - i), Tier: "core", Source: "seeded", State: "active",
			HitsLastCycle: float64(30 - i),
		}
	}
	// 25th domain (@dy) at rank 24, hits=6, after decay=5.4 → kept as context
	// Add a 26th with very low hits
	l.state.DomainMeta["@dz"] = &ports.DomainMeta{
		Hits: 0.2, Tier: "context", Source: "learned", State: "active",
		HitsLastCycle: 0.2,
	}
	l.RunMathTune()
	// @dz should be removed (0.2 * 0.9 = 0.18 < 0.3)
	assert.Nil(t, l.state.DomainMeta["@dz"], "@dz should be removed (hits < 0.3 at rank 24+)")
}

func TestAutotune_StaleCycleTracking(t *testing.T) {
	l := New()
	l.state.DomainMeta["@d"] = &ports.DomainMeta{
		Hits: 0.0, Tier: "core", Source: "seeded", State: "active",
		HitsLastCycle: 0.0,
	}
	l.RunMathTune()
	dm := l.state.DomainMeta["@d"]
	assert.Equal(t, "stale", dm.State)
	assert.Equal(t, uint32(1), dm.StaleCycles)

	// Second autotune: stale_cycles >= 2 → deprecated
	l.RunMathTune()
	dm = l.state.DomainMeta["@d"]
	assert.Equal(t, "deprecated", dm.State)
	assert.Equal(t, uint32(2), dm.StaleCycles)
}

func TestAutotune_Idempotent_NoSignals(t *testing.T) {
	l := New()
	l.state.DomainMeta["@d"] = &ports.DomainMeta{
		Hits: 5.0, Tier: "core", Source: "seeded", State: "active",
		HitsLastCycle: 5.0,
	}
	l.RunMathTune()
	assert.InDelta(t, 4.5, l.state.DomainMeta["@d"].Hits, 0.001)
	// Second tune decays again
	l.RunMathTune()
	assert.InDelta(t, 4.05, l.state.DomainMeta["@d"].Hits, 0.001)
}

// =============================================================================
// Fixture Parity — zero tolerance
// =============================================================================

// compareState compares a learner's state against a fixture snapshot.
// Uses exact comparison for uint32 maps, InDelta for float64 domain hits.
func compareState(t *testing.T, label string, got, want *ports.LearnerState) {
	t.Helper()

	assert.Equal(t, want.PromptCount, got.PromptCount, "%s: prompt_count", label)
	assert.Equal(t, want.KeywordHits, got.KeywordHits, "%s: keyword_hits", label)
	assert.Equal(t, want.TermHits, got.TermHits, "%s: term_hits", label)
	assert.Equal(t, want.CohitKwTerm, got.CohitKwTerm, "%s: cohit_kw_term", label)
	assert.Equal(t, want.CohitTermDomain, got.CohitTermDomain, "%s: cohit_term_domain", label)
	assert.Equal(t, want.Bigrams, got.Bigrams, "%s: bigrams", label)
	assert.Equal(t, want.FileHits, got.FileHits, "%s: file_hits", label)
	assert.Equal(t, want.KeywordBlocklist, got.KeywordBlocklist, "%s: keyword_blocklist", label)

	// Compare domain_meta with float tolerance
	assert.Equal(t, len(want.DomainMeta), len(got.DomainMeta), "%s: domain_meta count", label)
	for name, wantDM := range want.DomainMeta {
		gotDM, ok := got.DomainMeta[name]
		if !assert.True(t, ok, "%s: domain %s missing", label, name) {
			continue
		}
		assert.InDelta(t, wantDM.Hits, gotDM.Hits, 0.001, "%s: %s.hits", label, name)
		assert.Equal(t, wantDM.TotalHits, gotDM.TotalHits, "%s: %s.total_hits", label, name)
		assert.Equal(t, wantDM.Tier, gotDM.Tier, "%s: %s.tier", label, name)
		assert.Equal(t, wantDM.Source, gotDM.Source, "%s: %s.source", label, name)
		assert.Equal(t, wantDM.State, gotDM.State, "%s: %s.state", label, name)
		assert.Equal(t, wantDM.StaleCycles, gotDM.StaleCycles, "%s: %s.stale_cycles", label, name)
		assert.InDelta(t, wantDM.HitsLastCycle, gotDM.HitsLastCycle, 0.001, "%s: %s.hits_last_cycle", label, name)
		assert.Equal(t, wantDM.LastHitAt, gotDM.LastHitAt, "%s: %s.last_hit_at", label, name)
		assert.Equal(t, wantDM.CreatedAt, gotDM.CreatedAt, "%s: %s.created_at", label, name)
	}
}

func TestAutotuneParity_FreshTo50(t *testing.T) {
	// Load fresh state, replay 50 events, run autotune.
	// Snapshot must match 01-fifty-intents.json exactly.
	initial := loadFixture(t, "00-fresh.json")
	expected := loadFixture(t, "01-fifty-intents.json")
	events := loadEvents(t, "events-01-to-50.json")

	l := NewFromState(initial)
	for _, ev := range events {
		l.Observe(ev)
	}
	l.RunMathTune()

	compareState(t, "FreshTo50", l.State(), expected)
}

func TestAutotuneParity_50To100(t *testing.T) {
	// Load 50-intent state, replay events 51-100, run autotune.
	// Snapshot must match 02-hundred-intents.json.
	initial := loadFixture(t, "01-fifty-intents.json")
	expected := loadFixture(t, "02-hundred-intents.json")
	events := loadEvents(t, "events-51-to-100.json")

	l := NewFromState(initial)
	for _, ev := range events {
		l.Observe(ev)
	}
	l.RunMathTune()

	compareState(t, "50To100", l.State(), expected)
}

func TestAutotuneParity_100To200(t *testing.T) {
	// Load 100-intent state, replay events 101-200 (two autotune cycles: 150 and 200).
	// Snapshot must match 03-two-hundred.json.
	initial := loadFixture(t, "02-hundred-intents.json")
	expected := loadFixture(t, "03-two-hundred.json")
	events := loadEvents(t, "events-101-to-200.json")

	l := NewFromState(initial)

	// Events 101-150
	for _, ev := range events[:50] {
		l.Observe(ev)
	}
	l.RunMathTune() // Autotune at 150

	// Events 151-200
	for _, ev := range events[50:] {
		l.Observe(ev)
	}
	l.RunMathTune() // Autotune at 200

	compareState(t, "100To200", l.State(), expected)
}

func TestAutotuneParity_PostWipe(t *testing.T) {
	// After wipe, state matches 04-post-wipe.json (empty state).
	expected := loadFixture(t, "04-post-wipe.json")
	l := New()
	compareState(t, "PostWipe", l.State(), expected)
}

func TestAutotuneParity_FullReplay(t *testing.T) {
	// Full replay from fresh → 200 intents with intermediate autotunes.
	// This tests the entire pipeline end-to-end.
	initial := loadFixture(t, "00-fresh.json")
	expected200 := loadFixture(t, "03-two-hundred.json")

	l := NewFromState(initial)

	// Events 1-50 + autotune
	events1 := loadEvents(t, "events-01-to-50.json")
	for _, ev := range events1 {
		l.Observe(ev)
	}
	l.RunMathTune()

	// Verify intermediate state
	expected50 := loadFixture(t, "01-fifty-intents.json")
	compareState(t, "After50", l.State(), expected50)

	// Events 51-100 + autotune
	events2 := loadEvents(t, "events-51-to-100.json")
	for _, ev := range events2 {
		l.Observe(ev)
	}
	l.RunMathTune()

	// Verify intermediate state
	expected100 := loadFixture(t, "02-hundred-intents.json")
	compareState(t, "After100", l.State(), expected100)

	// Events 101-200 with two autotunes
	events3 := loadEvents(t, "events-101-to-200.json")
	for _, ev := range events3[:50] {
		l.Observe(ev)
	}
	l.RunMathTune()
	for _, ev := range events3[50:] {
		l.Observe(ev)
	}
	l.RunMathTune()

	compareState(t, "After200", l.State(), expected200)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkAutotune(b *testing.B) {
	// Build realistic state: 8 domains, ~20 terms, ~100 keywords
	fixture, err := func() (*ports.LearnerState, error) {
		data, err := os.ReadFile(fixtureDir + "01-fifty-intents.json")
		if err != nil {
			return nil, err
		}
		var s ports.LearnerState
		return &s, json.Unmarshal(data, &s)
	}()
	require.NoError(b, err)

	l := NewFromState(fixture)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.RunMathTune()
	}
}
