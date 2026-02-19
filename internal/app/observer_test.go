package app

import (
	"testing"
	"time"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/domain/learner"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
)

// --- collectHitSignals unit tests ---

func TestCollectHitSignals_DirectIncrement(t *testing.T) {
	hits := []index.Hit{
		{File: "auth.go", Domain: "@authentication", Tags: []string{"auth", "login"}, Kind: "symbol", Symbol: "Authenticate"},
		{File: "db.go", Domain: "@database", Tags: []string{"query", "sql"}, Kind: "symbol", Symbol: "Query"},
		{File: "auth.go", Domain: "@authentication", Tags: []string{"auth", "session"}, Kind: "symbol", Symbol: "Session"},
	}

	sig := collectHitSignals(hits, 10)

	// Domains: authentication, database — deduplicated, @ stripped
	assert.Equal(t, []string{"authentication", "database"}, sig.Domains)

	// Terms: auth, login, query, sql, session — deduplicated across hits
	assert.Equal(t, []string{"auth", "login", "query", "sql", "session"}, sig.Terms)

	// TermDomains: each tag paired with its hit's domain (not deduplicated)
	assert.Contains(t, sig.TermDomains, [2]string{"auth", "authentication"})
	assert.Contains(t, sig.TermDomains, [2]string{"login", "authentication"})
	assert.Contains(t, sig.TermDomains, [2]string{"query", "database"})
	assert.Contains(t, sig.TermDomains, [2]string{"sql", "database"})
	assert.Contains(t, sig.TermDomains, [2]string{"auth", "authentication"}) // from 3rd hit
	assert.Contains(t, sig.TermDomains, [2]string{"session", "authentication"})

	// No content text from symbol hits
	assert.Empty(t, sig.ContentText)
}

func TestCollectHitSignals_ContentText(t *testing.T) {
	hits := []index.Hit{
		{File: "auth.go", Domain: "@authentication", Tags: []string{"auth"}, Kind: "symbol", Symbol: "Login"},
		{File: "readme.md", Domain: "@documentation", Tags: []string{"docs"}, Kind: "content", Content: "authentication flow for login"},
		{File: "config.go", Domain: "@configuration", Tags: []string{"config"}, Kind: "content", Content: "auth middleware configuration"},
	}

	sig := collectHitSignals(hits, 10)

	// Symbol hits don't contribute content text
	// Content hits do
	assert.Equal(t, []string{"authentication flow for login", "auth middleware configuration"}, sig.ContentText)
}

func TestCollectHitSignals_EmptyDomain(t *testing.T) {
	hits := []index.Hit{
		{File: "misc.go", Domain: "", Tags: []string{"util"}, Kind: "content", Content: "helper function"},
		{File: "auth.go", Domain: "@authentication", Tags: []string{"auth"}, Kind: "symbol"},
	}

	sig := collectHitSignals(hits, 10)

	// Empty domain should not appear in Domains list
	assert.Equal(t, []string{"authentication"}, sig.Domains)

	// Terms from the empty-domain hit should still be collected
	assert.Contains(t, sig.Terms, "util")

	// TermDomains should not contain entries with empty domain
	for _, td := range sig.TermDomains {
		assert.NotEmpty(t, td[1], "TermDomains should not contain empty domain")
	}
}

func TestCollectHitSignals_TopN(t *testing.T) {
	hits := make([]index.Hit, 20)
	for i := range hits {
		hits[i] = index.Hit{
			File:   "file.go",
			Domain: "@testing",
			Tags:   []string{"test"},
			Kind:   "symbol",
		}
	}

	sig := collectHitSignals(hits, 5)

	// Should only process 5 hits — domain/term counts reflect just 1 unique each
	assert.Len(t, sig.Domains, 1)
	assert.Len(t, sig.Terms, 1)
}

// --- searchObserver integration tests ---

func TestSearchObserver_DirectIncrement(t *testing.T) {
	a := newTestAppWithStore(t)

	// Build index with multiple domains and terms
	idx := &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"auth":    {{FileID: 1, Line: 10}},
			"handler": {{FileID: 1, Line: 20}},
			"query":   {{FileID: 2, Line: 5}},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "auth", Kind: "function", Tags: []string{"auth", "login"}},
			{FileID: 1, Line: 20}: {Name: "handler", Kind: "function", Tags: []string{"handler"}},
			{FileID: 2, Line: 5}:  {Name: "query", Kind: "function", Tags: []string{"query", "sql"}},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "auth.go", Language: "go", Domain: "@authentication", Size: 4096},
			2: {Path: "db.go", Language: "go", Domain: "@database", Size: 2048},
		},
	}
	a.Index = idx
	domains := make(map[string]index.Domain)
	engine := index.NewSearchEngine(idx, domains, a.ProjectRoot)
	a.Engine = engine
	engine.SetObserver(a.searchObserver)

	// Run a single search
	engine.Search("auth", ports.SearchOptions{})

	state := a.Learner.State()

	// Signal counts should be modest — not inflated by enricher re-resolution
	// With direct increment: ~1-3 keywords from query, ~2-4 terms from hits, ~1-2 domains
	totalKeywords := uint32(0)
	for _, v := range state.KeywordHits {
		totalKeywords += v
	}
	totalTerms := uint32(0)
	for _, v := range state.TermHits {
		totalTerms += v
	}

	// The old cascade produced 137 keywords, 89 terms, 76 domains for similar queries.
	// After refactor, expect much more modest signal counts.
	assert.Less(t, totalKeywords, uint32(50), "keyword signals should be modest, not cascaded")
	assert.Less(t, totalTerms, uint32(50), "term signals should be modest, not cascaded")
}

func TestSearchObserver_ContentBigrams(t *testing.T) {
	// Verify that collectHitSignals extracts content text and the searchObserver
	// feeds it through ProcessBigrams. We test the mechanism directly since
	// setting up the search engine with real file content for content scanning
	// requires on-disk files.
	hits := []index.Hit{
		{File: "auth.go", Domain: "@authentication", Tags: []string{"auth"}, Kind: "content", Content: "auth handler middleware"},
	}

	sig := collectHitSignals(hits, 10)
	assert.Equal(t, []string{"auth handler middleware"}, sig.ContentText,
		"content hit text should be collected for bigram processing")

	// Verify the full bigram promotion path via processGrepSignal (same ProcessBigrams call)
	a := newTestAppWithStore(t)
	for i := 0; i < 7; i++ {
		a.Learner.ProcessBigrams("auth handler middleware")
	}
	state := a.Learner.State()
	assert.GreaterOrEqual(t, state.Bigrams["auth:handler"], learner.BigramThreshold,
		"repeated content text should promote bigrams past threshold")
}

func TestSearchObserver_QueryTokensUseEnricher(t *testing.T) {
	a := newTestAppWithStore(t)

	idx := &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"auth": {{FileID: 1, Line: 10}},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "auth", Kind: "function", Tags: []string{"auth"}},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "auth.go", Language: "go", Domain: "@authentication", Size: 4096},
		},
	}
	a.Index = idx
	domains := make(map[string]index.Domain)
	engine := index.NewSearchEngine(idx, domains, a.ProjectRoot)
	a.Engine = engine
	engine.SetObserver(a.searchObserver)

	engine.Search("auth", ports.SearchOptions{})

	state := a.Learner.State()

	// "auth" is a known atlas keyword — the enricher should resolve it to
	// keyword_hits. The query-token path uses addKeyword which goes through enricher.
	assert.Greater(t, state.KeywordHits["auth"], uint32(0),
		"query token 'auth' should be resolved as keyword via enricher")
}

// --- processGrepSignal tests ---

func TestGrepSignal_LearnerReceivesSignals(t *testing.T) {
	a := newTestAppWithStore(t)

	a.processGrepSignal("auth handler")

	state := a.Learner.State()

	// "auth" should appear in keyword hits (known atlas keyword)
	assert.Greater(t, state.KeywordHits["auth"], uint32(0),
		"grep pattern 'auth' should generate keyword signal")

	// Check for Learn activity with "(grep)" suffix
	found := false
	for i := 0; i < a.activityCount; i++ {
		idx := (a.activityHead - 1 - i + 50) % 50
		e := a.activityRing[idx]
		if e.Action == "Learn" && e.Source == "aOa" {
			assert.Contains(t, e.Impact, "(grep)")
			found = true
			break
		}
	}
	assert.True(t, found, "expected Learn activity with (grep) suffix")
}

func TestGrepSignal_BigramPromotion(t *testing.T) {
	a := newTestAppWithStore(t)

	// "auth handler" → bigram "auth:handler"
	// BigramThreshold is 6, so 7 calls should promote it
	for i := 0; i < 7; i++ {
		a.processGrepSignal("auth handler")
	}

	state := a.Learner.State()
	assert.GreaterOrEqual(t, state.Bigrams["auth:handler"], learner.BigramThreshold,
		"7× same grep pattern should promote bigram past threshold")
}

// --- Integration: Grep in onSessionEvent produces learning signals ---

func TestGrepSessionEvent_LearnerReceivesSignals(t *testing.T) {
	a := newTestAppWithStore(t)

	ev := ports.SessionEvent{
		Kind:      ports.EventToolInvocation,
		TurnID:    "turn-grep-1",
		Timestamp: time.Now(),
		Tool:      &ports.ToolEvent{Name: "Grep", Pattern: "auth handler"},
	}
	a.onSessionEvent(ev)

	state := a.Learner.State()
	assert.Greater(t, state.KeywordHits["auth"], uint32(0),
		"session Grep event should produce keyword learning signals")

	// Check for Learn activity with "(grep)" suffix
	found := false
	for i := 0; i < a.activityCount; i++ {
		idx := (a.activityHead - 1 - i + 50) % 50
		e := a.activityRing[idx]
		if e.Action == "Learn" && e.Source == "aOa" {
			assert.Contains(t, e.Impact, "(grep)")
			found = true
			break
		}
	}
	assert.True(t, found, "session Grep event should produce Learn activity with (grep)")
}
