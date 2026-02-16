package test

import (
	"fmt"
	"testing"

	"github.com/corey/aoa-go/internal/domain/index"
	"github.com/corey/aoa-go/internal/ports"
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

	engine := index.NewSearchEngine(idx, domains)

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

// --- Autotune Parity (F-03, L-01, L-03) ---

func TestAutotuneParity_FreshTo50(t *testing.T) {
	t.Skip("Fixtures not captured — F-03")
}

func TestAutotuneParity_50To100(t *testing.T) {
	t.Skip("Fixtures not captured — F-03")
}

func TestAutotuneParity_100To200(t *testing.T) {
	t.Skip("Fixtures not captured — F-03")
}

func TestAutotuneParity_PostWipe(t *testing.T) {
	t.Skip("Fixtures not captured — F-03")
}

// --- Observe Signal Parity (L-02, T-03) ---

func TestObserveSignalParity(t *testing.T) {
	t.Skip("Observe not implemented — L-02")
}

// --- Dedup Parity (L-04) ---

func TestDedupParity(t *testing.T) {
	t.Skip("Dedup not implemented — L-04")
}

// --- Competitive Displacement Parity (L-05) ---

func TestDisplacementParity(t *testing.T) {
	t.Skip("Displacement not implemented — L-05")
}

// --- Enrichment Parity (U-07) ---

func TestEnrichmentParity(t *testing.T) {
	t.Skip("Enricher not implemented — U-07")
}

// --- Bigram Parity (T-04) ---

func TestBigramParity(t *testing.T) {
	t.Skip("Bigrams not implemented — T-04")
}

// --- Integration: Full Session Simulation (M-01, M-02, M-03) ---

func TestFullSessionParity_200Intents(t *testing.T) {
	t.Skip("Full system not implemented — M-01")
}

func TestSearchDiff_100Queries(t *testing.T) {
	t.Skip("Full system not implemented — M-02")
}
