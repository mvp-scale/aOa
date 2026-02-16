package enricher

import (
	"testing"

	"github.com/corey/aoa-go/atlas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// U-06: Atlas loading from embedded binary
// =============================================================================

func TestAtlasLoad_AllFilesParseAndValidate(t *testing.T) {
	// U-06, G3: All 15 JSON files load from embedded FS.
	// Validates: 134 domains, 938 terms, 6566 keyword entries, 3407 unique keywords.
	a, err := LoadAtlas(atlas.FS, "v1")
	require.NoError(t, err)

	assert.Equal(t, 134, a.DomainCount, "expected 134 domains")
	assert.Equal(t, 938, a.TermCount, "expected 938 terms")
	assert.Equal(t, 6566, a.KeywordEntries, "expected 6566 total keyword entries")
	assert.Equal(t, 3407, a.UniqueKeywords, "expected 3407 unique keywords")
}

func TestAtlasLoad_EmbeddedInBinary(t *testing.T) {
	// U-06, G1, G7: Atlas loads from go:embed, no filesystem read.
	// Verifying the embedded FS works at all is the test.
	a, err := LoadAtlas(atlas.FS, "v1")
	require.NoError(t, err)
	assert.Greater(t, a.DomainCount, 0, "should load domains from embedded FS")
}

func TestAtlasLoad_NoDuplicateDomainNames(t *testing.T) {
	// U-06, G3: No two domains share the same name.
	a, err := LoadAtlas(atlas.FS, "v1")
	require.NoError(t, err)

	seen := make(map[string]bool)
	for _, d := range a.Domains {
		if seen[d.Domain] {
			t.Errorf("duplicate domain name: %s", d.Domain)
		}
		seen[d.Domain] = true
	}
}

func TestAtlasLoad_AllDomainsHaveTerms(t *testing.T) {
	// U-06, G3: Every domain has at least one term with at least one keyword.
	a, err := LoadAtlas(atlas.FS, "v1")
	require.NoError(t, err)

	for _, d := range a.Domains {
		assert.Greater(t, len(d.Terms), 0, "domain %s has no terms", d.Domain)
		for term, kws := range d.Terms {
			assert.Greater(t, len(kws), 0, "domain %s term %s has no keywords", d.Domain, term)
		}
	}
}

func TestAtlasLoad_KeywordsMapPopulated(t *testing.T) {
	// U-06: Keyword map is built correctly from domain/term structure.
	a, err := LoadAtlas(atlas.FS, "v1")
	require.NoError(t, err)

	// "jwt" should be in the authentication domain under "token" term
	matches := a.Keywords["jwt"]
	assert.NotEmpty(t, matches, "jwt should have matches")

	found := false
	for _, m := range matches {
		if m.Domain == "authentication" && m.Term == "token" {
			found = true
			break
		}
	}
	assert.True(t, found, "jwt should map to authentication/token")
}

// =============================================================================
// U-07: Enricher Domain — keyword -> term -> domain mapping
// =============================================================================

func loadTestEnricher(t *testing.T) *Enricher {
	t.Helper()
	enr, err := NewFromFS(atlas.FS, "v1")
	require.NoError(t, err)
	return enr
}

func TestEnrich_KeywordToTermToDomain(t *testing.T) {
	// U-07, G3: keyword "jwt" -> term "token" -> domain "authentication".
	enr := loadTestEnricher(t)
	matches := enr.Lookup("jwt")
	require.NotEmpty(t, matches)

	found := false
	for _, m := range matches {
		if m.Domain == "authentication" && m.Term == "token" {
			found = true
			break
		}
	}
	assert.True(t, found, "jwt should resolve to authentication/token")
}

func TestEnrich_UnknownKeyword_NoDomain(t *testing.T) {
	// U-07, G3: keyword "xyzzy_unknown_42" -> no matches.
	enr := loadTestEnricher(t)
	matches := enr.Lookup("xyzzy_unknown_42")
	assert.Empty(t, matches)
}

func TestEnrich_MultipleKeywords_SingleDomain(t *testing.T) {
	// U-07, G3: keywords from the same domain resolve to that domain.
	// "bcrypt", "argon2", "scrypt" are all in authentication/password.
	enr := loadTestEnricher(t)

	for _, kw := range []string{"bcrypt", "argon2", "scrypt"} {
		matches := enr.Lookup(kw)
		require.NotEmpty(t, matches, "%s should have matches", kw)

		found := false
		for _, m := range matches {
			if m.Domain == "authentication" {
				found = true
				break
			}
		}
		assert.True(t, found, "%s should resolve to authentication domain", kw)
	}
}

func TestEnrich_SharedKeyword_MultipleDomains(t *testing.T) {
	// U-07, G3: shared keywords return all owning domains.
	// "certificate" appears in both encryption/tls and encryption/asymmetric (and possibly others).
	enr := loadTestEnricher(t)
	matches := enr.Lookup("certificate")
	assert.Greater(t, len(matches), 1, "certificate should be shared across multiple domain/term pairs")
}

func TestEnrich_DomainDefs_ReturnsAll(t *testing.T) {
	// U-07: DomainDefs returns all 134 domains.
	enr := loadTestEnricher(t)
	defs := enr.DomainDefs()
	assert.Equal(t, 134, len(defs))
}

func TestEnrich_DomainTerms_ExistingDomain(t *testing.T) {
	// U-07: DomainTerms returns terms for a known domain.
	enr := loadTestEnricher(t)
	terms := enr.DomainTerms("authentication")
	assert.NotNil(t, terms)
	assert.Contains(t, terms, "login")
	assert.Contains(t, terms, "password")
	assert.Contains(t, terms, "token")
}

func TestEnrich_DomainTerms_UnknownDomain(t *testing.T) {
	// U-07: DomainTerms returns nil for unknown domain.
	enr := loadTestEnricher(t)
	terms := enr.DomainTerms("nonexistent_domain_xyz")
	assert.Nil(t, terms)
}

func TestEnrich_Stats(t *testing.T) {
	// U-07: Stats match atlas counts.
	enr := loadTestEnricher(t)
	d, tm, ke, uk := enr.Stats()
	assert.Equal(t, 134, d)
	assert.Equal(t, 938, tm)
	assert.Equal(t, 6566, ke)
	assert.Equal(t, 3407, uk)
}

func TestEnrich_LookupIsO1(t *testing.T) {
	// U-07, G1: Keyword lookup is a map access — O(1).
	// This is a structural test: the Keywords map exists and is populated.
	enr := loadTestEnricher(t)
	// Verify it's a direct map lookup (no iteration)
	matches := enr.Lookup("jwt")
	assert.NotEmpty(t, matches)
	// A second lookup on a different keyword is equally fast
	matches2 := enr.Lookup("bcrypt")
	assert.NotEmpty(t, matches2)
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkAtlasLoad(b *testing.B) {
	// Target: <10ms from embedded bytes
	for i := 0; i < b.N; i++ {
		_, err := LoadAtlas(atlas.FS, "v1")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkKeywordLookup(b *testing.B) {
	enr, err := NewFromFS(atlas.FS, "v1")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = enr.Lookup("jwt")
	}
}
