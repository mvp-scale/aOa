package test

import (
	"fmt"
	"testing"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Behavioral Parity Tests — Zero tolerance divergence from Python
// These tests are the ultimate gate: if parity breaks, the port is wrong.
// =============================================================================

// --- Search Parity (F-04, S-01, S-06, U-08) ---

func TestSearchParity(t *testing.T) {
	// Load index state
	idx, domains, err := loadIndexFixture("fixtures/search/index-state.json")
	require.NoError(t, err, "Failed to load index-state.json")

	engine := index.NewSearchEngine(idx, domains, "")

	// Load search queries
	fixtures, err := loadSearchFixtures("fixtures/search/queries.json")
	require.NoError(t, err, "Failed to load search fixtures")

	for i, fixture := range fixtures {
		name := fmt.Sprintf("Q%02d_%s", i+1, fixture.Query)
		t.Run(name, func(t *testing.T) {
			// Check expected tokenization if specified
			if fixture.ExpectedTokenization != nil {
				tokens := index.Tokenize(fixture.Query)
				assert.Equal(t, fixture.ExpectedTokenization, tokens,
					"tokenization mismatch for %q", fixture.Query)
			}

			// Build search options from fixture flags
			opts := ports.SearchOptions{
				Mode:         fixture.Mode,
				AndMode:      fixture.Flags.AndMode,
				WordBoundary: fixture.Flags.WordBoundary,
				CountOnly:    fixture.Flags.CountOnly,
				Quiet:        fixture.Flags.Quiet,
				MaxCount:     fixture.Flags.MaxCount,
				IncludeGlob:  fixture.Flags.IncludeGlob,
				ExcludeGlob:  fixture.Flags.ExcludeGlob,
			}

			// Case insensitive flag overrides mode
			if fixture.Flags.CaseInsensitive {
				opts.Mode = "case_insensitive"
			}

			result := engine.Search(fixture.Query, opts)

			// Check expected_exit_code (quiet mode)
			if fixture.ExpectedExitCode != nil {
				assert.Equal(t, *fixture.ExpectedExitCode, result.ExitCode,
					"exit code mismatch")
				return
			}

			// Check expected_count (count mode)
			if fixture.ExpectedCount != nil {
				assert.Equal(t, *fixture.ExpectedCount, result.Count,
					"count mismatch")
				return
			}

			// Check expected hits
			require.Equal(t, len(fixture.Expected), len(result.Hits),
				"hit count mismatch: expected %d, got %d",
				len(fixture.Expected), len(result.Hits))

			for j, exp := range fixture.Expected {
				got := result.Hits[j]
				assert.Equal(t, exp.File, got.File, "hit[%d] file", j)
				assert.Equal(t, exp.Line, got.Line, "hit[%d] line", j)
				assert.Equal(t, exp.Symbol, got.Symbol, "hit[%d] symbol", j)
				assert.Equal(t, exp.Range, got.Range, "hit[%d] range", j)
				assert.Equal(t, exp.Domain, got.Domain, "hit[%d] domain", j)
				assert.Equal(t, exp.Tags, got.Tags, "hit[%d] tags", j)
			}
		})
	}
}

// Autotune parity tests live in internal/domain/learner/autotune_test.go:
//   TestAutotuneParity_FreshTo50, _50To100, _100To200, _PostWipe, _FullReplay
//
// Observe, dedup, displacement are exercised end-to-end by TestAutotuneParity_FullReplay
// which replays 200 intents and validates state at every 50-intent checkpoint.
//
// Enrichment parity: internal/domain/enricher/enricher_test.go (14 tests, 134 domains)
// Bigram parity:     internal/domain/learner/bigrams_test.go (15 tests, incl. MatchesPythonTokenizer)
