package learner

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// L-02: observe() — Signal intake
// =============================================================================

func TestObserve_NonBlocking(t *testing.T) {
	// observe() is a direct method call — inherently non-blocking.
	// Channel-based optimization is future work (L-02 channel variant).
	l := New()
	ev := ObserveEvent{
		PromptNumber: 1,
		Observe: ObserveData{
			Keywords: []string{"login"},
			Domains:  []string{"@auth"},
		},
	}
	l.Observe(ev) // Should not panic or block
	assert.Equal(t, uint32(1), l.state.KeywordHits["login"])
}

func TestObserve_AllSignalTypes(t *testing.T) {
	l := New()
	// Seed domain
	l.state.DomainMeta["@auth"] = testDomainMeta("core", "seeded")

	ev := ObserveEvent{
		PromptNumber: 1,
		Observe: ObserveData{
			Keywords:     []string{"login"},
			Terms:        []string{"auth"},
			Domains:      []string{"@auth"},
			KeywordTerms: [][2]string{{"login", "auth"}},
			TermDomains:  [][2]string{{"auth", "@auth"}},
		},
		FileRead: &FileRead{File: "handler.py", Offset: 1, Limit: 30},
	}
	l.Observe(ev)

	// Keywords get double-counted: once from Keywords, once from KeywordTerms
	assert.Equal(t, uint32(2), l.state.KeywordHits["login"])
	// Terms also double-counted
	assert.Equal(t, uint32(2), l.state.TermHits["auth"])
	assert.InDelta(t, 1.0, l.state.DomainMeta["@auth"].Hits, 0.001)
	assert.Equal(t, uint32(1), l.state.CohitKwTerm["login:auth"])
	assert.Equal(t, uint32(1), l.state.CohitTermDomain["auth:@auth"])
	assert.Equal(t, uint32(1), l.state.FileHits["handler.py"])
}

func TestObserve_FileHitIncrement(t *testing.T) {
	l := New()
	ev := ObserveEvent{
		PromptNumber: 1,
		FileRead:     &FileRead{File: "auth/handler.py", Offset: 1, Limit: 30},
	}
	l.Observe(ev)
	assert.Equal(t, uint32(1), l.state.FileHits["auth/handler.py"])
	l.Observe(ObserveEvent{PromptNumber: 2, FileRead: &FileRead{File: "auth/handler.py"}})
	assert.Equal(t, uint32(2), l.state.FileHits["auth/handler.py"])
}

func TestObserve_KeywordHitIncrement(t *testing.T) {
	l := New()
	ev := ObserveEvent{
		PromptNumber: 1,
		Observe:      ObserveData{Keywords: []string{"login"}},
	}
	l.Observe(ev)
	assert.Equal(t, uint32(1), l.state.KeywordHits["login"])
}

func TestObserve_CohitTracking(t *testing.T) {
	l := New()
	ev := ObserveEvent{
		PromptNumber: 1,
		Observe: ObserveData{
			KeywordTerms: [][2]string{{"login", "auth"}},
		},
	}
	l.Observe(ev)
	assert.Equal(t, uint32(1), l.state.CohitKwTerm["login:auth"])
}

func TestObserve_BatchFlush(t *testing.T) {
	// Synchronous observe — each call immediately updates state.
	// Batch optimization is future work.
	l := New()
	for i := 0; i < 50; i++ {
		l.Observe(ObserveEvent{
			PromptNumber: uint32(i + 1),
			Observe:      ObserveData{Keywords: []string{"test"}},
		})
	}
	assert.Equal(t, uint32(50), l.state.KeywordHits["test"])
}

func TestObserve_OrderPreserved(t *testing.T) {
	l := New()
	l.Observe(ObserveEvent{
		PromptNumber: 1,
		Observe:      ObserveData{Keywords: []string{"first"}},
	})
	l.Observe(ObserveEvent{
		PromptNumber: 2,
		Observe:      ObserveData{Keywords: []string{"second"}},
	})
	assert.Equal(t, uint32(2), l.PromptCount())
	assert.Equal(t, uint32(1), l.state.KeywordHits["first"])
	assert.Equal(t, uint32(1), l.state.KeywordHits["second"])
}

func TestObserve_LearnedDomainCreatedOnFirstHit(t *testing.T) {
	l := New()
	ev := ObserveEvent{
		PromptNumber: 70,
		Observe: ObserveData{
			Domains: []string{"@monitoring"},
		},
	}
	l.Observe(ev)
	dm := l.state.DomainMeta["@monitoring"]
	require.NotNil(t, dm)
	assert.Equal(t, "learned", dm.Source)
	assert.Equal(t, "context", dm.Tier)
	assert.Equal(t, "active", dm.State)
	assert.InDelta(t, 1.0, dm.Hits, 0.001)
	assert.Equal(t, int64(1739500070), dm.CreatedAt)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkObserve(b *testing.B) {
	l := New()
	ev := ObserveEvent{
		PromptNumber: 1,
		Observe: ObserveData{
			Keywords:     []string{"login", "session"},
			Terms:        []string{"login"},
			Domains:      []string{"@auth"},
			KeywordTerms: [][2]string{{"login", "login"}, {"session", "session"}},
			TermDomains:  [][2]string{{"login", "@auth"}},
		},
		FileRead: &FileRead{File: "handler.py"},
	}
	// Seed domain
	l.state.DomainMeta["@auth"] = testDomainMeta("core", "seeded")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.PromptNumber = uint32(i + 1)
		l.Observe(ev)
	}
}

// helper to create a test DomainMeta pointer
func testDomainMeta(tier, source string) *ports.DomainMeta {
	return &ports.DomainMeta{
		Tier:      tier,
		Source:    source,
		State:     "active",
		CreatedAt: 1739500000,
	}
}
