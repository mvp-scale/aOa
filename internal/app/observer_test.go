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
	engine.WaitObservers()

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
	engine.WaitObservers()

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

// --- stripCodeBlocks tests ---

func TestStripCodeBlocks_RemovesFencedCode(t *testing.T) {
	input := "Here is the fix:\n```go\nfunc auth() {}\n```\nThis handles authentication."
	got := stripCodeBlocks(input)
	assert.Contains(t, got, "Here is the fix:")
	assert.Contains(t, got, "This handles authentication.")
	assert.NotContains(t, got, "func auth()")
}

func TestStripCodeBlocks_MultipleBlocks(t *testing.T) {
	input := "First ```code1``` middle ```code2``` end"
	got := stripCodeBlocks(input)
	assert.Equal(t, "First  middle  end", got)
}

func TestStripCodeBlocks_NoBlocks(t *testing.T) {
	input := "plain text about authentication"
	assert.Equal(t, input, stripCodeBlocks(input))
}

func TestStripCodeBlocks_UnclosedFence(t *testing.T) {
	input := "before ```unclosed code block"
	got := stripCodeBlocks(input)
	assert.Equal(t, "before ", got)
}

func TestConversationSignal_IgnoresCodeBlocks(t *testing.T) {
	a := newTestAppWithStore(t)
	a.promptN = 1

	// "auth" is in prose (intent), "oauth" is only in code block (not intent)
	text := "Fix the auth handler:\n```go\nfunc oauth() { return nil }\n```"
	a.processConversationSignal(text, false)

	state := a.Learner.State()
	assert.Greater(t, state.KeywordHits["auth"], uint32(0),
		"prose keyword should be captured")
	assert.Equal(t, uint32(0), state.KeywordHits["oauth"],
		"code block keyword should not be captured")
}

// --- processConversationSignal tests ---

func TestConversationSignal_UserInput_ProducesKeywordHits(t *testing.T) {
	a := newTestAppWithStore(t)
	a.promptN = 1

	a.processConversationSignal("the auth handler validates tokens", true)

	state := a.Learner.State()
	// "auth" is a known atlas keyword — should resolve through enricher
	assert.Greater(t, state.KeywordHits["auth"], uint32(0),
		"conversation text with atlas keyword should produce keyword_hits")
}

func TestConversationSignal_ResolvesTermsAndDomains(t *testing.T) {
	a := newTestAppWithStore(t)
	a.promptN = 1

	// "auth" resolves to terms and domains via the atlas enricher
	a.processConversationSignal("implement auth middleware", true)

	state := a.Learner.State()
	assert.Greater(t, len(state.KeywordHits), 0, "should have keyword_hits")
	assert.Greater(t, len(state.TermHits), 0, "should have term_hits")
	assert.Greater(t, len(state.DomainMeta), 0, "should have domain_meta")
	assert.Greater(t, len(state.CohitKwTerm), 0, "should have cohit_kw_term")
	assert.Greater(t, len(state.CohitTermDomain), 0, "should have cohit_term_domain")
}

func TestConversationSignal_ObserveTrue_TriggersAutotune(t *testing.T) {
	a := newTestAppWithStore(t)
	a.promptN = 50 // autotune fires at multiples of 50

	a.processConversationSignal("implement auth middleware", true)

	// After autotune, domain_meta entries get their HitsLastCycle set
	state := a.Learner.State()
	for _, dm := range state.DomainMeta {
		// Autotune step 7 snapshots hits → hits_last_cycle
		assert.Greater(t, dm.HitsLastCycle, float64(0),
			"observe=true at promptN=50 should trigger autotune (snapshot hits_last_cycle)")
		break
	}
}

func TestConversationSignal_ObserveFalse_NoAutotune(t *testing.T) {
	a := newTestAppWithStore(t)
	a.promptN = 50

	a.processConversationSignal("implement auth middleware", false)

	// observe=false means only Observe() is called, not ObserveAndMaybeTune
	state := a.Learner.State()
	for _, dm := range state.DomainMeta {
		assert.Equal(t, float64(0), dm.HitsLastCycle,
			"observe=false should not trigger autotune")
		break
	}
}

func TestConversationSignal_EmptyText_Noop(t *testing.T) {
	a := newTestAppWithStore(t)
	a.processConversationSignal("", false)
	state := a.Learner.State()
	assert.Empty(t, state.KeywordHits, "empty text should produce no signals")
}

func TestConversationSignal_NoEnricherMatches_Noop(t *testing.T) {
	a := newTestAppWithStore(t)
	// gibberish that won't match any atlas keywords
	a.processConversationSignal("xyzzy plugh foobar", false)
	state := a.Learner.State()
	assert.Empty(t, state.DomainMeta, "non-atlas words should produce no domain signals")
}

// --- Integration: conversation events in onSessionEvent produce enricher signals ---

func TestSessionEvent_UserInput_ProducesEnricherSignals(t *testing.T) {
	a := newTestAppWithStore(t)

	ev := ports.SessionEvent{
		Kind:      ports.EventUserInput,
		TurnID:    "turn-user-1",
		Timestamp: time.Now(),
		Text:      "fix the authentication handler",
	}
	a.onSessionEvent(ev)

	state := a.Learner.State()
	assert.Greater(t, len(state.KeywordHits), 0,
		"user input should produce keyword_hits via enricher")
	assert.Greater(t, len(state.DomainMeta), 0,
		"user input should produce domain signals via enricher")
}

func TestSessionEvent_AIThinking_ProducesEnricherSignals(t *testing.T) {
	a := newTestAppWithStore(t)
	a.promptN = 1

	ev := ports.SessionEvent{
		Kind:      ports.EventAIThinking,
		TurnID:    "turn-think-1",
		Timestamp: time.Now(),
		Text:      "I need to look at the authentication middleware",
	}
	a.onSessionEvent(ev)

	state := a.Learner.State()
	assert.Greater(t, len(state.KeywordHits), 0,
		"AI thinking should produce keyword_hits via enricher")
}

func TestSessionEvent_AIResponse_ProducesEnricherSignals(t *testing.T) {
	a := newTestAppWithStore(t)
	a.promptN = 1

	ev := ports.SessionEvent{
		Kind:      ports.EventAIResponse,
		TurnID:    "turn-resp-1",
		Timestamp: time.Now(),
		Text:      "The authentication handler validates the JWT token",
	}
	a.onSessionEvent(ev)

	state := a.Learner.State()
	assert.Greater(t, len(state.KeywordHits), 0,
		"AI response should produce keyword_hits via enricher")
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
}
