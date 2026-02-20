package app

import (
	"testing"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAutotuneFiresViaSearchObserver verifies that the autotune cycle fires
// at prompt 50 when driven through the searchObserver path. This is an
// integration test: it wires a real SearchEngine + Enricher + Learner + Store.
func TestAutotuneFiresViaSearchObserver(t *testing.T) {
	a := newTestAppWithStore(t)

	// Set up a minimal search index so searches return at least one hit
	idx := &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"auth": {{FileID: 1, Line: 10}},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "auth", Kind: "function", Signature: "auth()", Tags: []string{"auth"}},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "auth.go", Language: "go", Domain: "@authentication", Size: 4096},
		},
	}
	a.Index = idx
	domains := make(map[string]index.Domain)
	engine := index.NewSearchEngine(idx, domains, a.ProjectRoot)
	a.Engine = engine

	// Wire the search observer
	engine.SetObserver(a.searchObserver)

	// Fire 50 searches to trigger autotune at prompt 50
	for i := 0; i < 50; i++ {
		engine.Search("auth", ports.SearchOptions{})
	}
	engine.WaitObservers()

	// After 50 searches, promptN should be >= 50 and autotune should have fired
	assert.GreaterOrEqual(t, a.promptN, uint32(50))

	// Check for Autotune activity entry
	found := false
	for i := 0; i < a.activityCount; i++ {
		aidx := (a.activityHead - 1 - i + 50) % 50
		e := a.activityRing[aidx]
		if e.Action == "Autotune" && e.Source == "aOa" {
			found = true
			assert.Contains(t, e.Attrib, "cycle")
			assert.Contains(t, e.Impact, "promoted")
			assert.Contains(t, e.Impact, "demoted")
			assert.Contains(t, e.Impact, "decayed")
			break
		}
	}
	require.True(t, found, "L0.6/L0.7: expected Autotune activity entry after 50 searches")

	// L0.11: Learn signals appear as cycling learn words in the Learned field
	learnCount := 0
	for i := 0; i < a.activityCount; i++ {
		aidx := (a.activityHead - 1 - i + 50) % 50
		e := a.activityRing[aidx]
		if e.Action == "Search" && e.Learned != "" {
			learnCount++
		}
	}
	assert.Greater(t, learnCount, 0, "L0.11: expected Search rows with Learned field set")
}
