package index

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTrigramTestEngine creates an engine with symbols for trigram fallback testing.
// None of the query tokens will exist as exact tokens in the index, forcing the
// trigram path.
func buildTrigramTestEngine(t *testing.T) *SearchEngine {
	t.Helper()

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	idx.Files[1] = &ports.FileMeta{Path: "auth.go", Language: "go", Size: 200}
	idx.Files[2] = &ports.FileMeta{Path: "handler.go", Language: "go", Size: 300}
	idx.Files[3] = &ports.FileMeta{Path: "util.go", Language: "go", Size: 100}

	// Symbol: authenticateRequest
	ref1 := ports.TokenRef{FileID: 1, Line: 10}
	idx.Metadata[ref1] = &ports.SymbolMeta{
		Name: "authenticateRequest", Signature: "authenticateRequest(r *http.Request)",
		Kind: "function", StartLine: 10, EndLine: 25,
	}
	idx.Tokens["authenticate"] = []ports.TokenRef{ref1}
	idx.Tokens["request"] = []ports.TokenRef{ref1}

	// Symbol: authorizeUser
	ref2 := ports.TokenRef{FileID: 1, Line: 30}
	idx.Metadata[ref2] = &ports.SymbolMeta{
		Name: "authorizeUser", Signature: "authorizeUser(u User)",
		Kind: "function", StartLine: 30, EndLine: 45,
	}
	idx.Tokens["authorize"] = []ports.TokenRef{ref2}
	idx.Tokens["user"] = []ports.TokenRef{ref2}

	// Symbol: handlePodUpdate
	ref3 := ports.TokenRef{FileID: 2, Line: 5}
	idx.Metadata[ref3] = &ports.SymbolMeta{
		Name: "handlePodUpdate", Signature: "handlePodUpdate(pod *v1.Pod)",
		Kind: "function", StartLine: 5, EndLine: 20,
	}
	idx.Tokens["handle"] = []ports.TokenRef{ref3}
	idx.Tokens["pod"] = []ports.TokenRef{ref3}
	idx.Tokens["update"] = []ports.TokenRef{ref3}

	// Symbol: formatOutput (unrelated — control for scoring)
	ref4 := ports.TokenRef{FileID: 3, Line: 1}
	idx.Metadata[ref4] = &ports.SymbolMeta{
		Name: "formatOutput", Signature: "formatOutput(s string)",
		Kind: "function", StartLine: 1, EndLine: 5,
	}
	idx.Tokens["format"] = []ports.TokenRef{ref4}
	idx.Tokens["output"] = []ports.TokenRef{ref4}

	return NewSearchEngine(idx, make(map[string]Domain), "")
}

func TestTrigramFallback_TypoFindsApproximateMatch(t *testing.T) {
	engine := buildTrigramTestEngine(t)

	// "authenticateHandler" is a typo — no symbol has that exact name.
	// But "authenticateRequest" shares most of the "authenticate" trigrams.
	// Exact token lookup returns 0 (no token "handler" → all hits from "authenticate").
	// Wait — "authenticate" IS a token. Let me use a query with NO matching tokens.
	// Query: "authencateRequest" (typo in authenticate)
	result := engine.Search("authencateRequest", ports.SearchOptions{})
	require.NotNil(t, result)
	require.Greater(t, len(result.Hits), 0, "trigram fallback should return approximate matches")

	// authenticateRequest should rank highest (most trigram overlap with "authencateRequest")
	assert.Equal(t, "authenticateRequest(r *http.Request)", result.Hits[0].Symbol)
}

func TestTrigramFallback_ShortQueryFindsMatches(t *testing.T) {
	engine := buildTrigramTestEngine(t)

	// "authz" is not a token (tokenizer would keep it as-is since no camelCase split,
	// but it doesn't exist in the index). Trigrams: "aut", "uth", "thz"
	// "authenticateRequest" has "aut", "uth" — 2/3 trigrams (67%)
	// "authorizeUser" has "aut", "uth" — 2/3 trigrams (67%)
	result := engine.Search("authz", ports.SearchOptions{})
	require.NotNil(t, result)
	require.Greater(t, len(result.Hits), 0)

	// Both auth* symbols should appear
	names := make(map[string]bool)
	for _, h := range result.Hits {
		names[h.Symbol] = true
	}
	assert.True(t, names["authenticateRequest(r *http.Request)"] || names["authorizeUser(u User)"],
		"should find auth-related symbols via trigram overlap")
}

func TestTrigramFallback_NoMatchBelowThreshold(t *testing.T) {
	engine := buildTrigramTestEngine(t)

	// "zzzzzzzzzzzzz" shares no trigrams with any symbol
	result := engine.Search("zzzzzzzzzzzzz", ports.SearchOptions{})
	require.NotNil(t, result)
	assert.Equal(t, 0, len(result.Hits))
}

func TestTrigramFallback_NotTriggeredWhenTokenHitsExist(t *testing.T) {
	engine := buildTrigramTestEngine(t)

	// "authenticate" is an exact token in the index — should use normal path
	result := engine.Search("authenticate", ports.SearchOptions{})
	require.NotNil(t, result)
	require.Equal(t, 1, len(result.Hits))
	assert.Equal(t, "authenticateRequest(r *http.Request)", result.Hits[0].Symbol)
}

func TestTrigramFallback_SkippedForANDMode(t *testing.T) {
	engine := buildTrigramTestEngine(t)

	// AND mode is explicit intersection — if no symbol has all terms,
	// zero results is the correct answer. Trigram fallback should NOT fire.
	result := engine.Search("authx,reqz", ports.SearchOptions{AndMode: true})
	require.NotNil(t, result)
	assert.Equal(t, 0, len(result.Hits), "AND mode should not trigger trigram fallback")
}

func TestTrigramFallback_RespectsAllowedFiles(t *testing.T) {
	engine := buildTrigramTestEngine(t)

	// Use scope to restrict to handler.go (file 2 only)
	// "authenticx" should only match symbols in allowed files
	result := engine.Search("authenticx", ports.SearchOptions{
		Scope: "handler.go",
	})
	require.NotNil(t, result)
	// After scope filter, only handler.go symbols should survive
	for _, h := range result.Hits {
		assert.Contains(t, h.File, "handler.go")
	}
}

func TestTrigramFallback_SkippedForRegex(t *testing.T) {
	engine := buildTrigramTestEngine(t)

	// Regex mode with a pattern that matches nothing — should NOT trigger fallback
	result := engine.Search("zzzzNoMatch", ports.SearchOptions{Mode: "regex"})
	require.NotNil(t, result)
	// Regex search returns 0 and fallback is skipped
	assert.Equal(t, 0, len(result.Hits))
}

func TestBuildSymTrigrams_PopulatesIndex(t *testing.T) {
	engine := buildTrigramTestEngine(t)

	// "aut" trigram should exist (from "authenticateRequest" and "authorizeUser")
	tri := [3]byte{'a', 'u', 't'}
	refs := engine.symTrigramPosting[tri]
	assert.GreaterOrEqual(t, len(refs), 2, "trigram 'aut' should map to at least 2 symbols")

	// "zzz" trigram should not exist
	tri2 := [3]byte{'z', 'z', 'z'}
	assert.Empty(t, engine.symTrigramPosting[tri2])
}

func TestTrigramFallback_RanksHighOverlapFirst(t *testing.T) {
	engine := buildTrigramTestEngine(t)

	// "handlePodUpdat" (missing final 'e') — should find handlePodUpdate
	// as the top hit since it shares almost all trigrams
	result := engine.Search("handlePodUpdat", ports.SearchOptions{})
	require.NotNil(t, result)
	require.Greater(t, len(result.Hits), 0)
	assert.Equal(t, "handlePodUpdate(pod *v1.Pod)", result.Hits[0].Symbol,
		"highest trigram overlap should rank first")
}
