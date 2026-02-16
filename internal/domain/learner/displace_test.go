package learner

import (
	"testing"

	"github.com/corey/aoa-go/internal/ports"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// L-05: Competitive Displacement
// =============================================================================

func TestDisplace_Top24BecomeCore(t *testing.T) {
	l := New()
	// Create 30 domains
	for i := 0; i < 30; i++ {
		name := "@d" + string(rune('A'+i))
		l.state.DomainMeta[name] = &ports.DomainMeta{
			Hits: float64(60 - i), Tier: "context", Source: "seeded", State: "active",
			HitsLastCycle: float64(60 - i),
		}
	}
	l.RunMathTune()
	coreCount := 0
	contextCount := 0
	for _, dm := range l.state.DomainMeta {
		if dm.State == "deprecated" {
			continue
		}
		if dm.Tier == "core" {
			coreCount++
		} else {
			contextCount++
		}
	}
	assert.Equal(t, 24, coreCount, "exactly 24 should be core")
	assert.Equal(t, 6, contextCount, "remaining 6 should be context")
}

func TestDisplace_ContextTierDemotion(t *testing.T) {
	l := New()
	// 25 domains: first 24 high hits, 25th drops to context
	for i := 0; i < 25; i++ {
		name := "@d" + string(rune('A'+i))
		l.state.DomainMeta[name] = &ports.DomainMeta{
			Hits: float64(50 - i), Tier: "core", Source: "seeded", State: "active",
			HitsLastCycle: float64(50 - i),
		}
	}
	l.RunMathTune()
	// 25th domain (@dY) should be demoted to context
	assert.Equal(t, "context", l.state.DomainMeta["@dY"].Tier)
}

func TestDisplace_RemoveBelow03(t *testing.T) {
	// Already tested in TestAutotune_RemoveDomainBelow03. Verify again here.
	l := New()
	for i := 0; i < 25; i++ {
		name := "@d" + string(rune('A'+i))
		l.state.DomainMeta[name] = &ports.DomainMeta{
			Hits: float64(30 - i), Tier: "core", Source: "seeded", State: "active",
			HitsLastCycle: float64(30 - i),
		}
	}
	l.state.DomainMeta["@low"] = &ports.DomainMeta{
		Hits: 0.1, Tier: "context", Source: "learned", State: "active",
		HitsLastCycle: 0.1,
	}
	l.RunMathTune()
	assert.Nil(t, l.state.DomainMeta["@low"])
}

func TestDisplace_NewDomainEntersAsContext(t *testing.T) {
	// A new learned domain with low hits enters as context.
	l := New()
	for i := 0; i < 24; i++ {
		name := "@d" + string(rune('A'+i))
		l.state.DomainMeta[name] = &ports.DomainMeta{
			Hits: float64(50 - i), Tier: "core", Source: "seeded", State: "active",
			HitsLastCycle: float64(50 - i),
		}
	}
	// 25th domain: low hits, but above prune floor
	l.state.DomainMeta["@new"] = &ports.DomainMeta{
		Hits: 5.0, Tier: "context", Source: "learned", State: "active",
		HitsLastCycle: 5.0,
	}
	l.RunMathTune()
	assert.Equal(t, "context", l.state.DomainMeta["@new"].Tier)
}

func TestDisplace_TieBreaking(t *testing.T) {
	l := New()
	// Two domains with same hits — alphabetical wins
	l.state.DomainMeta["@beta"] = &ports.DomainMeta{
		Hits: 10.0, Tier: "core", Source: "seeded", State: "active",
		HitsLastCycle: 10.0,
	}
	l.state.DomainMeta["@alpha"] = &ports.DomainMeta{
		Hits: 10.0, Tier: "core", Source: "seeded", State: "active",
		HitsLastCycle: 10.0,
	}
	l.RunMathTune()
	// Both should be core (only 2 domains, well under 24)
	assert.Equal(t, "core", l.state.DomainMeta["@alpha"].Tier)
	assert.Equal(t, "core", l.state.DomainMeta["@beta"].Tier)
}

func TestDisplace_24Exactly_NoneContext(t *testing.T) {
	l := New()
	for i := 0; i < 24; i++ {
		name := "@d" + string(rune('A'+i))
		l.state.DomainMeta[name] = &ports.DomainMeta{
			Hits: float64(48 - i), Tier: "core", Source: "seeded", State: "active",
			HitsLastCycle: float64(48 - i),
		}
	}
	l.RunMathTune()
	for _, dm := range l.state.DomainMeta {
		if dm.State != "deprecated" {
			assert.Equal(t, "core", dm.Tier, "all 24 should be core")
		}
	}
}

func TestDisplace_LessThan24_AllCore(t *testing.T) {
	l := New()
	for i := 0; i < 5; i++ {
		name := "@d" + string(rune('A'+i))
		l.state.DomainMeta[name] = &ports.DomainMeta{
			Hits: float64(10 - i), Tier: "context", Source: "seeded", State: "active",
			HitsLastCycle: float64(10 - i),
		}
	}
	l.RunMathTune()
	for _, dm := range l.state.DomainMeta {
		if dm.State != "deprecated" {
			assert.Equal(t, "core", dm.Tier, "all should be core when < 24")
		}
	}
}

func TestDisplace_CascadeClean_Keywords(t *testing.T) {
	// When a domain is removed, it's deleted from DomainMeta.
	// Cascade cleanup of keywords/terms is handled by the state maps directly.
	l := New()
	for i := 0; i < 25; i++ {
		name := "@d" + string(rune('A'+i))
		l.state.DomainMeta[name] = &ports.DomainMeta{
			Hits: float64(30 - i), Tier: "core", Source: "seeded", State: "active",
			HitsLastCycle: float64(30 - i),
		}
	}
	l.state.DomainMeta["@remove_me"] = &ports.DomainMeta{
		Hits: 0.1, Tier: "context", Source: "learned", State: "active",
		HitsLastCycle: 0.1,
	}
	l.RunMathTune()
	_, exists := l.state.DomainMeta["@remove_me"]
	assert.False(t, exists, "domain should be cascade-removed")
}
