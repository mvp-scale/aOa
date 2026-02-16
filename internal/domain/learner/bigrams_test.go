package learner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// T-04: Bigram Extraction — From conversation text, >=6 threshold
// Goals: G3 (domain learning)
// =============================================================================

func TestBigram_ExtractFromText(t *testing.T) {
	bg := ExtractBigrams("authentication handler login session")
	require.NotNil(t, bg)

	assert.Equal(t, uint32(1), bg["authentication:handler"])
	assert.Equal(t, uint32(1), bg["handler:login"])
	assert.Equal(t, uint32(1), bg["login:session"])
	assert.Len(t, bg, 3)
}

func TestBigram_ExtractFromText_Repeated(t *testing.T) {
	// Same pair appearing multiple times in one text
	bg := ExtractBigrams("auth handler auth handler auth handler")
	require.NotNil(t, bg)

	assert.Equal(t, uint32(3), bg["auth:handler"])
	assert.Equal(t, uint32(2), bg["handler:auth"])
}

func TestBigram_ThresholdGate(t *testing.T) {
	l := New()

	// "auth handler stuff" produces bigrams: auth:handler(1), handler:stuff(1)
	// Each call adds 1 count. Need 6 calls to cross threshold.
	for i := 0; i < 5; i++ {
		l.ProcessBigrams("auth handler stuff")
	}

	// Below threshold (5 < 6) — not in state.Bigrams
	assert.NotContains(t, l.state.Bigrams, "auth:handler")

	// One more (6 total) — crosses threshold
	l.ProcessBigrams("auth handler stuff")

	assert.Contains(t, l.state.Bigrams, "auth:handler")
	assert.Equal(t, uint32(6), l.state.Bigrams["auth:handler"])
}

func TestBigram_ThresholdGate_AlreadyPromoted(t *testing.T) {
	l := New()

	// Force promotion by feeding 6 times
	for i := 0; i < 6; i++ {
		l.ProcessBigrams("auth handler")
	}
	require.Contains(t, l.state.Bigrams, "auth:handler")
	countAtPromotion := l.state.Bigrams["auth:handler"]

	// After promotion, direct increment
	l.ProcessBigrams("auth handler")
	assert.Equal(t, countAtPromotion+1, l.state.Bigrams["auth:handler"])

	// Staging should be clean for this bigram
	assert.Zero(t, l.staging["auth:handler"])
}

func TestBigram_ThresholdGate_SingleTextBurst(t *testing.T) {
	l := New()

	// A single text with 6+ repetitions should promote immediately
	text := strings.Repeat("auth handler ", 7) // 7 occurrences
	l.ProcessBigrams(text)

	assert.Contains(t, l.state.Bigrams, "auth:handler")
	assert.GreaterOrEqual(t, l.state.Bigrams["auth:handler"], BigramThreshold)
}

func TestBigram_MatchesPythonTokenizer(t *testing.T) {
	// Python: re.findall(r'\b[a-z][a-z0-9_]+\b', text.lower())
	cases := []struct {
		text   string
		tokens []string
	}{
		// Basic words
		{"fix the auth bug", []string{"fix", "auth", "bug"}}, // "the" is stop word
		// Underscore preserved
		{"check file_path now", []string{"check", "file_path", "now"}},
		// Numbers after first char
		{"use sha256 hash", []string{"use", "sha256", "hash"}},
		// Leading uppercase → lowercased
		{"Fix Auth Handler", []string{"fix", "auth", "handler"}},
		// Digits at start excluded (first char must be [a-z])
		{"use 3des cipher", []string{"use", "cipher"}}, // "3des" starts with digit
		// Single char excluded (regex requires 2+ chars)
		{"a b c def", []string{"def"}},
		// Mixed with punctuation
		{"error: null_ptr in main()", []string{"error", "null_ptr", "main"}},
	}

	for _, tc := range cases {
		got := bigramTokenize(tc.text)
		// Filter expected through stop words for fair comparison
		var want []string
		for _, tok := range tc.tokens {
			if !stopWords[tok] {
				want = append(want, tok)
			}
		}
		assert.Equal(t, want, got, "text: %q", tc.text)
	}
}

func TestBigram_CaseNormalized(t *testing.T) {
	bg1 := ExtractBigrams("Auth Handler")
	bg2 := ExtractBigrams("auth handler")
	bg3 := ExtractBigrams("AUTH HANDLER")

	// All produce the same bigram
	assert.Equal(t, bg1["auth:handler"], bg2["auth:handler"])
	assert.Equal(t, bg2["auth:handler"], bg3["auth:handler"])
	assert.Equal(t, uint32(1), bg1["auth:handler"])
}

func TestBigram_IgnoresStopWords(t *testing.T) {
	// "the" and "for" are stop words — shouldn't form bigrams
	bg := ExtractBigrams("check the auth handler for errors")

	// Without stop words: "check", "auth", "handler", "errors"
	// Bigrams: check:auth, auth:handler, handler:errors
	assert.Contains(t, bg, "check:auth")
	assert.Contains(t, bg, "auth:handler")
	assert.Contains(t, bg, "handler:errors")

	// These should NOT exist (would require stop words as tokens)
	assert.NotContains(t, bg, "check:the")
	assert.NotContains(t, bg, "the:auth")
	assert.NotContains(t, bg, "handler:for")
	assert.NotContains(t, bg, "for:errors")
}

func TestBigram_StopWordList(t *testing.T) {
	// Verify specific stop words are filtered
	words := []string{"the", "and", "for", "are", "but", "not",
		"with", "from", "this", "that", "will", "would",
		"could", "should", "have", "has", "been"}

	for _, w := range words {
		assert.True(t, stopWords[w], "expected stop word: %q", w)
	}

	// These should NOT be stop words (could be code-relevant)
	codeWords := []string{"error", "auth", "handler", "function",
		"return", "import", "class", "type", "test"}
	for _, w := range codeWords {
		assert.False(t, stopWords[w], "should not be stop word: %q", w)
	}
}

func TestBigram_DecayInAutotune(t *testing.T) {
	l := New()

	// Seed bigrams directly (simulating post-threshold)
	l.state.Bigrams["auth:handler"] = 10
	l.state.Bigrams["login:session"] = 5
	l.state.Bigrams["rare:pair"] = 1

	l.RunMathTune()

	// Decayed: int(10 * 0.90) = 9, int(5 * 0.90) = 4, int(1 * 0.90) = 0
	assert.Equal(t, uint32(9), l.state.Bigrams["auth:handler"])
	assert.Equal(t, uint32(4), l.state.Bigrams["login:session"])

	// Pruned (decayed to 0)
	assert.NotContains(t, l.state.Bigrams, "rare:pair")
}

func TestBigram_EmptyText(t *testing.T) {
	bg := ExtractBigrams("")
	assert.Nil(t, bg)
}

func TestBigram_SingleToken(t *testing.T) {
	// Only one token after stop word filtering — no bigrams possible
	bg := ExtractBigrams("the authentication")
	assert.Nil(t, bg) // "the" is stop word, only "authentication" remains
}

func TestBigram_ColonSeparator(t *testing.T) {
	bg := ExtractBigrams("database connection pool")
	require.NotNil(t, bg)

	// Verify colon separator format (matches Python spec)
	for key := range bg {
		parts := strings.SplitN(key, ":", 2)
		assert.Len(t, parts, 2, "bigram should have exactly one colon: %q", key)
		assert.NotEmpty(t, parts[0])
		assert.NotEmpty(t, parts[1])
	}
}

func TestBigram_UnderscoreTokens(t *testing.T) {
	// Underscored identifiers should stay as single tokens
	bg := ExtractBigrams("check file_path against base_dir")
	require.NotNil(t, bg)

	assert.Contains(t, bg, "check:file_path")
	assert.Contains(t, bg, "file_path:against")
	assert.Contains(t, bg, "against:base_dir")

	// Should NOT split on underscore
	assert.NotContains(t, bg, "file:path")
	assert.NotContains(t, bg, "base:dir")
}
