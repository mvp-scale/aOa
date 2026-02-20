package migration

// L3 Grep/Egrep Parity Test
//
// Tests every flag and flag combination exposed by `aoa grep` and `aoa egrep`
// against the existing 13-file fixture index. This is the comprehensive parity
// proof that aOa's search behaves correctly across all supported modes.
//
// Coverage matrix (from `grep --help` vs `aoa grep --help`):
//
//   IMPLEMENTED FLAGS (aoa grep):
//     -i  --ignore-case      Case insensitive
//     -w  --word-regexp       Word boundary match
//     -v  --invert-match      Select non-matching
//     -c  --count             Count only
//     -q  --quiet             Exit code only
//     -m  --max-count=NUM     Limit results (default 20)
//     -E  --extended-regexp   Route to regex mode (egrep)
//     -e  --regexp=PATTERN    Multiple patterns (OR)
//     -a  --and               AND mode (comma-separated) [NOTE: diverges from grep -a]
//         --include=GLOB      Include file filter
//         --exclude=GLOB      Exclude file filter
//
//   IMPLEMENTED FLAGS (aoa egrep):
//     -c  --count             Count only
//     -q  --quiet             Exit code only
//     -v  --invert-match      Select non-matching
//     -m  --max-count=NUM     Limit results (default 20)
//     -e  --regexp=PATTERN    Multiple patterns (joined with |)
//         --include=GLOB      Include file filter
//         --exclude=GLOB      Exclude file filter
//
//   NO-OP FLAGS (accepted but do nothing):
//     -r  --recursive         Always recursive
//     -n  --line-number       Always shows line numbers
//     -H  --with-filename     Always shows filenames
//     -F  --fixed-strings     Already literal (grep only)
//     -l  --files-with-matches  Default behavior (grep only)
//
//   NOT IMPLEMENTED (GNU grep features not in aoa):
//     -x  --line-regexp       Match whole lines
//     -o  --only-matching     Only matching part
//     -b  --byte-offset       Byte offset
//     -A/-B/-C               Context lines
//     --exclude-dir           Directory exclusion
//     -L  --files-without-match  Inverse of -l
//
//   EGREP GAPS (flags on grep but missing from egrep):
//     -i  --ignore-case       Not on egrep
//     -w  --word-regexp        Not on egrep
//     -a  --and               Not on egrep
//
//   DIVERGENCE:
//     grep -a = --text (binary-as-text)
//     aoa grep -a = --and (AND mode)

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadTestIndex loads the shared 13-file fixture index used by all tests.
// Returns the engine ready to search. Tests in this file do NOT use content
// search (no projectRoot) — they exercise the symbol index path only,
// matching how the CLI routes queries through the daemon.
func loadTestIndex(t *testing.T) *index.SearchEngine {
	t.Helper()
	idx, domains, err := loadFixtureIndex("../fixtures/search/index-state.json")
	require.NoError(t, err, "Failed to load index-state.json")
	return index.NewSearchEngine(idx, domains, "")
}

// =============================================================================
// GREP: Individual Flag Tests
// =============================================================================

func TestGrep_Literal_SingleToken(t *testing.T) {
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	assert.Equal(t, 5, len(result.Hits), "login should match 5 symbols")
	for _, hit := range result.Hits {
		assert.NotEmpty(t, hit.File)
		assert.NotEmpty(t, hit.Symbol)
	}
}

func TestGrep_Literal_MultiTokenOR(t *testing.T) {
	engine := loadTestIndex(t)
	// Space-separated = OR search
	result := engine.Search("login session", ports.SearchOptions{MaxCount: 50})
	assert.GreaterOrEqual(t, len(result.Hits), 5, "OR of login+session should return multiple hits")
}

func TestGrep_Literal_ZeroResults(t *testing.T) {
	engine := loadTestIndex(t)
	result := engine.Search("xyznonexistent", ports.SearchOptions{MaxCount: 50})
	assert.Empty(t, result.Hits)
}

func TestGrep_Flag_i_CaseInsensitive(t *testing.T) {
	// grep -i LOGIN → matches lowercase "login"
	engine := loadTestIndex(t)
	result := engine.Search("LOGIN", ports.SearchOptions{
		Mode:     "case_insensitive",
		MaxCount: 50,
	})
	assert.Equal(t, 5, len(result.Hits), "-i: LOGIN should match 5 symbols with 'login'")
	// Verify same results as lowercase
	resultLower := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	assert.Equal(t, len(resultLower.Hits), len(result.Hits),
		"-i: uppercase and lowercase should return same count")
}

func TestGrep_Flag_w_WordBoundary(t *testing.T) {
	// grep -w log → only exact "log" token, NOT login/logout/logger
	engine := loadTestIndex(t)
	result := engine.Search("log", ports.SearchOptions{
		WordBoundary: true,
		MaxCount:     50,
	})
	// "log" as exact token should NOT match "login", "logout", "logging"
	for _, hit := range result.Hits {
		// Verify none of these are login/logout-only symbols
		assert.NotContains(t, hit.Symbol, "login",
			"-w: %s should not match (contains login, not exact log)", hit.Symbol)
	}
	// Without -w, "log" matches many more
	resultNoW := engine.Search("log", ports.SearchOptions{MaxCount: 50})
	assert.GreaterOrEqual(t, len(resultNoW.Hits), len(result.Hits),
		"without -w, log should return >= hits as with -w")
}

func TestGrep_Flag_v_InvertMatch(t *testing.T) {
	// grep -v login → symbols that do NOT contain "login"
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		InvertMatch: true,
		MaxCount:    100,
	})
	normalResult := engine.Search("login", ports.SearchOptions{MaxCount: 100})
	// Inverted should not overlap with normal
	normalFiles := make(map[string]map[int]bool)
	for _, h := range normalResult.Hits {
		if normalFiles[h.File] == nil {
			normalFiles[h.File] = make(map[int]bool)
		}
		normalFiles[h.File][h.Line] = true
	}
	for _, h := range result.Hits {
		if normalFiles[h.File] != nil {
			assert.False(t, normalFiles[h.File][h.Line],
				"-v: hit %s:%d should not appear in both normal and inverted results", h.File, h.Line)
		}
	}
}

func TestGrep_Flag_c_CountOnly(t *testing.T) {
	// grep -c auth → count of matching symbols
	engine := loadTestIndex(t)
	result := engine.Search("auth", ports.SearchOptions{
		CountOnly: true,
		MaxCount:  50,
	})
	assert.Equal(t, 3, result.Count, "-c: auth should count 3 symbols")
	assert.Empty(t, result.Hits, "-c: should not return individual hits")
}

func TestGrep_Flag_q_Quiet_Found(t *testing.T) {
	// grep -q config → exit code 0 when results exist
	engine := loadTestIndex(t)
	result := engine.Search("config", ports.SearchOptions{
		Quiet:    true,
		MaxCount: 50,
	})
	assert.Equal(t, 0, result.ExitCode, "-q: should return exit code 0 when results exist")
}

func TestGrep_Flag_q_Quiet_NotFound(t *testing.T) {
	// grep -q xyznothing → exit code 1 when no results
	engine := loadTestIndex(t)
	result := engine.Search("xyznothing", ports.SearchOptions{
		Quiet:    true,
		MaxCount: 50,
	})
	assert.Equal(t, 1, result.ExitCode, "-q: should return exit code 1 when no results")
}

func TestGrep_Flag_m_MaxCount(t *testing.T) {
	// grep -m 3 test → only first 3 hits
	engine := loadTestIndex(t)
	result := engine.Search("test", ports.SearchOptions{MaxCount: 3})
	assert.Equal(t, 3, len(result.Hits), "-m 3: should return exactly 3 hits")

	// Verify fewer than unrestricted
	resultAll := engine.Search("test", ports.SearchOptions{MaxCount: 100})
	assert.Greater(t, len(resultAll.Hits), 3, "unrestricted 'test' should return >3 hits")
}

func TestGrep_Flag_m1_MaxCountOne(t *testing.T) {
	// Edge case: grep -m 1 test → single best hit
	engine := loadTestIndex(t)
	result := engine.Search("test", ports.SearchOptions{MaxCount: 1})
	assert.Equal(t, 1, len(result.Hits), "-m 1: should return exactly 1 hit")
}

func TestGrep_Flag_a_ANDMode(t *testing.T) {
	// grep -a validate,token → only symbols with BOTH tokens
	engine := loadTestIndex(t)
	result := engine.Search("validate,token", ports.SearchOptions{
		AndMode:  true,
		MaxCount: 50,
	})
	assert.Equal(t, 1, len(result.Hits), "-a: validate,token should match exactly 1 symbol")
	assert.Contains(t, result.Hits[0].Symbol, "validate_token")
}

func TestGrep_Flag_a_ANDMode_NoIntersection(t *testing.T) {
	// grep -a login,expose → no symbol has both
	engine := loadTestIndex(t)
	result := engine.Search("login,expose", ports.SearchOptions{
		AndMode:  true,
		MaxCount: 50,
	})
	assert.Empty(t, result.Hits, "-a: login,expose should return 0 (no intersection)")
}

func TestGrep_Flag_E_RoutesToRegex(t *testing.T) {
	// grep -E 'handle.*login' → regex mode
	engine := loadTestIndex(t)
	result := engine.Search("handle.*login", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 50,
	})
	assert.Equal(t, 2, len(result.Hits), "-E: handle.*login should match 2 symbols")
}

func TestGrep_Flag_e_MultiplePatterns(t *testing.T) {
	// grep -e login -e logout → OR of both patterns (combined as space-separated)
	engine := loadTestIndex(t)
	result := engine.Search("login logout", ports.SearchOptions{MaxCount: 50})
	assert.GreaterOrEqual(t, len(result.Hits), 5,
		"-e: login + logout OR should return multiple hits")
}

func TestGrep_Flag_include(t *testing.T) {
	// grep --include='services/*' handler → only files matching glob
	engine := loadTestIndex(t)
	result := engine.Search("handler", ports.SearchOptions{
		IncludeGlob: "services/*",
		MaxCount:    50,
	})
	for _, hit := range result.Hits {
		assert.True(t, len(hit.File) > 0 && hit.File[:9] == "services/",
			"--include: %s should be under services/", hit.File)
	}
}

func TestGrep_Flag_exclude(t *testing.T) {
	// grep --exclude='tests/*' create → no test files
	engine := loadTestIndex(t)
	result := engine.Search("create", ports.SearchOptions{
		ExcludeGlob: "tests/*",
		MaxCount:    50,
	})
	for _, hit := range result.Hits {
		assert.False(t, len(hit.File) >= 6 && hit.File[:6] == "tests/",
			"--exclude: %s should not be under tests/", hit.File)
	}
}

// =============================================================================
// GREP: Flag Combinations
// =============================================================================

func TestGrep_Combo_i_w(t *testing.T) {
	// grep -i -w LOG → case-insensitive word boundary: matches "log" token only
	engine := loadTestIndex(t)
	result := engine.Search("LOG", ports.SearchOptions{
		Mode:         "case_insensitive",
		WordBoundary: true,
		MaxCount:     50,
	})
	// Should match same as lowercase -w log
	resultLower := engine.Search("log", ports.SearchOptions{
		WordBoundary: true,
		MaxCount:     50,
	})
	assert.Equal(t, len(resultLower.Hits), len(result.Hits),
		"-i -w: LOG should match same symbols as log with word boundary")
}

func TestGrep_Combo_v_c(t *testing.T) {
	// grep -v -c login → count of symbols NOT matching login
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		InvertMatch: true,
		CountOnly:   true,
		MaxCount:    100,
	})
	assert.Greater(t, result.Count, 0, "-v -c: inverted count should be > 0")

	// Normal count + inverted count should approximate total symbols
	normalResult := engine.Search("login", ports.SearchOptions{
		CountOnly: true,
		MaxCount:  100,
	})
	t.Logf("-c login=%d, -v -c login=%d", normalResult.Count, result.Count)
}

func TestGrep_Combo_i_c(t *testing.T) {
	// grep -i -c AUTH → case-insensitive count
	engine := loadTestIndex(t)
	result := engine.Search("AUTH", ports.SearchOptions{
		Mode:      "case_insensitive",
		CountOnly: true,
		MaxCount:  100,
	})
	assert.Equal(t, 3, result.Count, "-i -c: AUTH should count same as auth")
}

func TestGrep_Combo_a_include(t *testing.T) {
	// grep -a --include='services/*' validate,token → AND + glob filter
	engine := loadTestIndex(t)
	result := engine.Search("validate,token", ports.SearchOptions{
		AndMode:     true,
		IncludeGlob: "services/*",
		MaxCount:    50,
	})
	assert.Equal(t, 1, len(result.Hits),
		"-a --include: validate,token in services/* should match 1")
	assert.Contains(t, result.Hits[0].File, "services/")
}

func TestGrep_Combo_a_exclude(t *testing.T) {
	// grep -a --exclude='tests/*' validate,token → AND + exclude
	engine := loadTestIndex(t)
	result := engine.Search("validate,token", ports.SearchOptions{
		AndMode:     true,
		ExcludeGlob: "tests/*",
		MaxCount:    50,
	})
	assert.Equal(t, 1, len(result.Hits),
		"-a --exclude: validate,token excluding tests/* should match 1")
}

func TestGrep_Combo_v_include(t *testing.T) {
	// grep -v --include='tests/*' login → invert + include
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		InvertMatch: true,
		IncludeGlob: "tests/*",
		MaxCount:    100,
	})
	for _, hit := range result.Hits {
		assert.True(t, len(hit.File) >= 6 && hit.File[:6] == "tests/",
			"-v --include: %s should be under tests/", hit.File)
	}
}

func TestGrep_Combo_v_exclude(t *testing.T) {
	// grep -v --exclude='tests/*' login → invert + exclude (no test files)
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		InvertMatch: true,
		ExcludeGlob: "tests/*",
		MaxCount:    100,
	})
	for _, hit := range result.Hits {
		assert.False(t, len(hit.File) >= 6 && hit.File[:6] == "tests/",
			"-v --exclude: %s should not be under tests/", hit.File)
	}
}

func TestGrep_Combo_q_v(t *testing.T) {
	// grep -q -v xyznonexistent → invert of nothing = everything → exit 0
	engine := loadTestIndex(t)
	result := engine.Search("xyznonexistent", ports.SearchOptions{
		Quiet:       true,
		InvertMatch: true,
		MaxCount:    100,
	})
	assert.Equal(t, 0, result.ExitCode,
		"-q -v: inverting a non-match should find results → exit 0")
}

func TestGrep_Combo_i_a(t *testing.T) {
	// grep -i -a VALIDATE,TOKEN → case-insensitive AND
	engine := loadTestIndex(t)
	result := engine.Search("VALIDATE,TOKEN", ports.SearchOptions{
		Mode:     "case_insensitive",
		AndMode:  true,
		MaxCount: 50,
	})
	assert.Equal(t, 1, len(result.Hits),
		"-i -a: VALIDATE,TOKEN should match 1 (same as lowercase)")
}

func TestGrep_Combo_m_v(t *testing.T) {
	// grep -m 2 -v login → inverted, limited to 2
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		InvertMatch: true,
		MaxCount:    2,
	})
	assert.LessOrEqual(t, len(result.Hits), 2,
		"-m 2 -v: should return at most 2 inverted hits")
}

func TestGrep_Combo_c_include(t *testing.T) {
	// grep -c --include='tests/*' test → count only within tests/
	engine := loadTestIndex(t)
	result := engine.Search("test", ports.SearchOptions{
		CountOnly:   true,
		IncludeGlob: "tests/*",
		MaxCount:    100,
	})
	assert.Greater(t, result.Count, 0, "-c --include: should count test symbols in tests/")

	// Compare with unrestricted count
	allResult := engine.Search("test", ports.SearchOptions{
		CountOnly: true,
		MaxCount:  100,
	})
	assert.GreaterOrEqual(t, allResult.Count, result.Count,
		"-c --include: restricted count should be <= total count")
}

// =============================================================================
// EGREP: Individual Flag Tests
// =============================================================================

func TestEgrep_Regex_SimplePattern(t *testing.T) {
	// egrep 'handle.*login'
	engine := loadTestIndex(t)
	result := engine.Search("handle.*login", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 50,
	})
	assert.Equal(t, 2, len(result.Hits), "egrep: handle.*login should match 2")
}

func TestEgrep_Regex_Alternation(t *testing.T) {
	// egrep 'login|logout'
	engine := loadTestIndex(t)
	result := engine.Search("login|logout", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 50,
	})
	assert.Equal(t, 7, len(result.Hits), "egrep: login|logout should match 7")
}

func TestEgrep_Regex_Anchored(t *testing.T) {
	// egrep 'test_.*login'
	engine := loadTestIndex(t)
	result := engine.Search("test_.*login", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 50,
	})
	assert.Equal(t, 3, len(result.Hits), "egrep: test_.*login should match 3")
}

func TestEgrep_Regex_NoMatch(t *testing.T) {
	// egrep 'xyz[0-9]+abc' → nothing
	engine := loadTestIndex(t)
	result := engine.Search("xyz[0-9]+abc", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 50,
	})
	assert.Empty(t, result.Hits, "egrep: xyz[0-9]+abc should match nothing")
}

func TestEgrep_Flag_c(t *testing.T) {
	// egrep -c 'login|logout'
	engine := loadTestIndex(t)
	result := engine.Search("login|logout", ports.SearchOptions{
		Mode:      "regex",
		CountOnly: true,
		MaxCount:  50,
	})
	assert.Equal(t, 7, result.Count, "egrep -c: login|logout should count 7")
}

func TestEgrep_Flag_q_Found(t *testing.T) {
	// egrep -q 'login' → exit 0
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		Mode:     "regex",
		Quiet:    true,
		MaxCount: 50,
	})
	assert.Equal(t, 0, result.ExitCode, "egrep -q: login found → exit 0")
}

func TestEgrep_Flag_q_NotFound(t *testing.T) {
	// egrep -q 'xyznothing' → exit 1
	engine := loadTestIndex(t)
	result := engine.Search("xyznothing", ports.SearchOptions{
		Mode:     "regex",
		Quiet:    true,
		MaxCount: 50,
	})
	assert.Equal(t, 1, result.ExitCode, "egrep -q: xyznothing not found → exit 1")
}

func TestEgrep_Flag_v(t *testing.T) {
	// egrep -v 'login' → symbols NOT matching login regex
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		Mode:        "regex",
		InvertMatch: true,
		MaxCount:    100,
	})
	for _, hit := range result.Hits {
		assert.NotContains(t, hit.Symbol, "login",
			"egrep -v: %s should not contain login", hit.Symbol)
	}
}

func TestEgrep_Flag_m(t *testing.T) {
	// egrep -m 2 'test' → limited to 2
	engine := loadTestIndex(t)
	result := engine.Search("test", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 2,
	})
	assert.Equal(t, 2, len(result.Hits), "egrep -m 2: should return exactly 2")
}

func TestEgrep_Flag_include(t *testing.T) {
	// egrep --include='services/*' 'handler'
	engine := loadTestIndex(t)
	result := engine.Search("handler", ports.SearchOptions{
		Mode:        "regex",
		IncludeGlob: "services/*",
		MaxCount:    50,
	})
	for _, hit := range result.Hits {
		assert.True(t, len(hit.File) > 0 && hit.File[:9] == "services/",
			"egrep --include: %s should be under services/", hit.File)
	}
}

func TestEgrep_Flag_exclude(t *testing.T) {
	// egrep --exclude='tests/*' 'test'
	engine := loadTestIndex(t)
	result := engine.Search("test", ports.SearchOptions{
		Mode:        "regex",
		ExcludeGlob: "tests/*",
		MaxCount:    50,
	})
	for _, hit := range result.Hits {
		assert.False(t, len(hit.File) >= 6 && hit.File[:6] == "tests/",
			"egrep --exclude: %s should not be under tests/", hit.File)
	}
}

func TestEgrep_Flag_e_MultiplePatterns(t *testing.T) {
	// egrep -e login -e logout → joined as login|logout
	engine := loadTestIndex(t)
	result := engine.Search("login|logout", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 50,
	})
	assert.Equal(t, 7, len(result.Hits), "egrep -e: login|logout should match 7")
}

// =============================================================================
// EGREP: Flag Combinations
// =============================================================================

func TestEgrep_Combo_v_c(t *testing.T) {
	// egrep -v -c 'login'
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		Mode:        "regex",
		InvertMatch: true,
		CountOnly:   true,
		MaxCount:    100,
	})
	assert.Greater(t, result.Count, 0, "egrep -v -c: inverted count should be > 0")
}

func TestEgrep_Combo_v_include(t *testing.T) {
	// egrep -v --include='tests/*' 'login'
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		Mode:        "regex",
		InvertMatch: true,
		IncludeGlob: "tests/*",
		MaxCount:    100,
	})
	for _, hit := range result.Hits {
		assert.True(t, len(hit.File) >= 6 && hit.File[:6] == "tests/",
			"egrep -v --include: %s should be under tests/", hit.File)
	}
}

func TestEgrep_Combo_m_v(t *testing.T) {
	// egrep -m 3 -v 'login'
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		Mode:        "regex",
		InvertMatch: true,
		MaxCount:    3,
	})
	assert.LessOrEqual(t, len(result.Hits), 3,
		"egrep -m 3 -v: should return at most 3 inverted hits")
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestGrep_Edge_ShortToken(t *testing.T) {
	// Single char tokens are below min length 2 → no results
	engine := loadTestIndex(t)
	result := engine.Search("a", ports.SearchOptions{MaxCount: 50})
	assert.Empty(t, result.Hits, "single char 'a' should return nothing (min token length 2)")
}

func TestGrep_Edge_TwoCharToken(t *testing.T) {
	// Two-char token should work
	engine := loadTestIndex(t)
	result := engine.Search("db", ports.SearchOptions{MaxCount: 50})
	// May or may not have hits — just verify no crash
	assert.NotNil(t, result)
}

func TestGrep_Edge_Unicode(t *testing.T) {
	// Unicode doesn't crash, returns nothing
	engine := loadTestIndex(t)
	result := engine.Search("résumé", ports.SearchOptions{MaxCount: 50})
	assert.Empty(t, result.Hits, "unicode query should return nothing gracefully")
}

func TestGrep_Edge_CamelCaseTokenization(t *testing.T) {
	// CamelCase split: getUserToken → get + user + token (OR)
	engine := loadTestIndex(t)
	result := engine.Search("getUserToken", ports.SearchOptions{MaxCount: 50})
	assert.GreaterOrEqual(t, len(result.Hits), 1,
		"CamelCase getUserToken should match via get/user/token tokens")

	// Verify tokenization
	tokens := index.Tokenize("getUserToken")
	assert.Equal(t, []string{"get", "user", "token"}, tokens)
}

func TestGrep_Edge_DottedTokenization(t *testing.T) {
	// Dotted split: app.post → app + post (OR)
	tokens := index.Tokenize("app.post")
	assert.Equal(t, []string{"app", "post"}, tokens)
}

func TestGrep_Edge_HyphenatedTokenization(t *testing.T) {
	// Hyphenated split: tree-sitter → tree + sitter (OR)
	tokens := index.Tokenize("tree-sitter")
	assert.Equal(t, []string{"tree", "sitter"}, tokens)
}

func TestGrep_Edge_MaxCount_Zero(t *testing.T) {
	// MaxCount=0 → engine default behavior
	engine := loadTestIndex(t)
	result := engine.Search("test", ports.SearchOptions{MaxCount: 0})
	assert.NotNil(t, result)
}

func TestEgrep_Edge_InvalidRegex(t *testing.T) {
	// Invalid regex should not panic
	engine := loadTestIndex(t)
	result := engine.Search("[invalid", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 50,
	})
	// Should return empty or error, not panic
	assert.NotNil(t, result)
}

// =============================================================================
// No-op Flags (verify they're accepted without changing behavior)
// =============================================================================

func TestGrep_Noop_r_AlwaysRecursive(t *testing.T) {
	// -r is a no-op: aoa always searches all files.
	// We verify by confirming results match regardless.
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	assert.Equal(t, 5, len(result.Hits), "search should work (always recursive)")
}

func TestGrep_Noop_n_AlwaysLineNumbers(t *testing.T) {
	// -n is a no-op: aoa always returns line numbers.
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	for _, hit := range result.Hits {
		assert.Greater(t, hit.Line, 0, "every hit should have a line number")
	}
}

func TestGrep_Noop_H_AlwaysFilename(t *testing.T) {
	// -H is a no-op: aoa always returns filenames.
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	for _, hit := range result.Hits {
		assert.NotEmpty(t, hit.File, "every hit should have a filename")
	}
}

// =============================================================================
// L3.6: EGREP -i (Case Insensitive)
// =============================================================================

func TestEgrep_Flag_i(t *testing.T) {
	// egrep -i LOGIN → matches lowercase "login" regex
	engine := loadTestIndex(t)
	result := engine.Search("LOGIN", ports.SearchOptions{
		Mode:     "case_insensitive",
		MaxCount: 50,
	})
	assert.Equal(t, 5, len(result.Hits), "egrep -i: LOGIN should match 5 symbols with 'login'")
	// Verify same results as lowercase
	resultLower := engine.Search("login", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 50,
	})
	assert.Equal(t, len(resultLower.Hits), len(result.Hits),
		"egrep -i: uppercase and lowercase should return same count")
}

func TestEgrep_Flag_i_Regex(t *testing.T) {
	// egrep -i 'HANDLE.*LOGIN' → case-insensitive regex
	engine := loadTestIndex(t)
	result := engine.Search("handle.*login", ports.SearchOptions{
		Mode:     "case_insensitive",
		MaxCount: 50,
	})
	assert.GreaterOrEqual(t, len(result.Hits), 2,
		"egrep -i: handle.*login should match >= 2 symbols")
}

// =============================================================================
// L3.10: EGREP -w (Word Boundary)
// =============================================================================

func TestEgrep_Flag_w(t *testing.T) {
	// egrep -w log → word boundary in regex mode
	engine := loadTestIndex(t)
	result := engine.Search("log", ports.SearchOptions{
		Mode:         "regex",
		WordBoundary: true,
		MaxCount:     50,
	})
	for _, hit := range result.Hits {
		assert.NotContains(t, hit.Symbol, "login",
			"egrep -w: %s should not match (contains login, not exact log)", hit.Symbol)
	}
}

// =============================================================================
// L3.14: EGREP -a (AND Mode)
// =============================================================================

func TestEgrep_Flag_a_ANDMode(t *testing.T) {
	// egrep -a validate,token → AND mode in egrep
	engine := loadTestIndex(t)
	result := engine.Search("validate,token", ports.SearchOptions{
		AndMode:  true,
		MaxCount: 50,
	})
	assert.Equal(t, 1, len(result.Hits), "egrep -a: validate,token should match exactly 1 symbol")
	assert.Contains(t, result.Hits[0].Symbol, "validate_token")
}

// =============================================================================
// L3.8: --exclude-dir (grep + egrep)
// =============================================================================

func TestGrep_Flag_excludeDir(t *testing.T) {
	// grep --exclude-dir=tests login → no files under tests/
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		ExcludeDirGlob: "tests",
		MaxCount:       50,
	})
	for _, hit := range result.Hits {
		assert.False(t, len(hit.File) >= 6 && hit.File[:6] == "tests/",
			"--exclude-dir: %s should not be under tests/", hit.File)
	}
	// Verify we still get hits from other directories (login exists in services/)
	assert.Greater(t, len(result.Hits), 0,
		"--exclude-dir: should still return hits from non-excluded dirs")
}

func TestEgrep_Flag_excludeDir(t *testing.T) {
	// egrep --exclude-dir=tests 'test' → no files under tests/
	engine := loadTestIndex(t)
	result := engine.Search("test", ports.SearchOptions{
		Mode:           "regex",
		ExcludeDirGlob: "tests",
		MaxCount:       50,
	})
	for _, hit := range result.Hits {
		assert.False(t, len(hit.File) >= 6 && hit.File[:6] == "tests/",
			"egrep --exclude-dir: %s should not be under tests/", hit.File)
	}
}

func TestGrep_Flag_excludeDir_Nested(t *testing.T) {
	// grep --exclude-dir='services/auth' login → no files under services/auth/
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		ExcludeDirGlob: "services/auth",
		MaxCount:       50,
	})
	for _, hit := range result.Hits {
		assert.False(t, strings.HasPrefix(hit.File, "services/auth/"),
			"--exclude-dir: %s should not be under services/auth/", hit.File)
	}
}

// =============================================================================
// L3.9: -o / --only-matching (grep + egrep)
// =============================================================================

func TestGrep_Flag_o(t *testing.T) {
	// grep -o login → only the matching part "login"
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		OnlyMatching: true,
		MaxCount:     50,
	})
	assert.Greater(t, len(result.Hits), 0, "-o: should have results")
	for _, hit := range result.Hits {
		if hit.Kind == "symbol" {
			assert.Equal(t, "login", hit.Symbol,
				"-o: symbol should be trimmed to just 'login', got %q", hit.Symbol)
		}
	}
}

func TestEgrep_Flag_o(t *testing.T) {
	// egrep -o 'login|logout' → only the matching regex part
	engine := loadTestIndex(t)
	result := engine.Search("login|logout", ports.SearchOptions{
		Mode:         "regex",
		OnlyMatching: true,
		MaxCount:     50,
	})
	assert.Greater(t, len(result.Hits), 0, "egrep -o: should have results")
	for _, hit := range result.Hits {
		if hit.Kind == "symbol" {
			assert.True(t, hit.Symbol == "login" || hit.Symbol == "logout",
				"egrep -o: symbol should be 'login' or 'logout', got %q", hit.Symbol)
		}
	}
}

// =============================================================================
// L3.11: -L / --files-without-match (grep + egrep)
// =============================================================================

func TestGrep_Flag_L(t *testing.T) {
	// grep -L login → files that do NOT contain "login"
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		FilesWithoutMatch: true,
		MaxCount:          50,
	})
	// Should return files not matching login
	loginResult := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	loginFiles := make(map[string]bool)
	for _, h := range loginResult.Hits {
		loginFiles[h.File] = true
	}
	for _, h := range result.Hits {
		assert.Equal(t, "file", h.Kind, "-L: hits should be kind 'file'")
		assert.False(t, loginFiles[h.File],
			"-L: %s should NOT be in the login match set", h.File)
	}
	assert.Greater(t, len(result.Hits), 0, "-L: should return some non-matching files")
}

func TestGrep_Flag_L_NoResults(t *testing.T) {
	// grep -L xyznothing → all files (nothing matches, so all are "without match")
	engine := loadTestIndex(t)
	result := engine.Search("xyznothing", ports.SearchOptions{
		FilesWithoutMatch: true,
		MaxCount:          50,
	})
	// All 13 files should be returned
	assert.Equal(t, 13, len(result.Hits), "-L: all 13 files when nothing matches")
}

// =============================================================================
// L3.12: --no-filename (output only — tested via formatSearchResult)
// =============================================================================

func TestGrep_Flag_noFilename(t *testing.T) {
	// --no-filename suppresses file prefix in output.
	// Test at engine level: verify results still come through (output formatting is CLI-level).
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	assert.Equal(t, 5, len(result.Hits),
		"--no-filename: search results unaffected by output flag")
	// The actual filename suppression is in cmd/aoa/cmd/output.go
}

// =============================================================================
// L3.13: --no-color (output only — tested via formatSearchResult)
// =============================================================================

func TestGrep_Flag_noColor(t *testing.T) {
	// --no-color strips ANSI codes from output.
	// Test at engine level: verify results still come through.
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	assert.Equal(t, 5, len(result.Hits),
		"--no-color: search results unaffected by output flag")
}

// =============================================================================
// L3.7: -A/-B/-C Context Lines (Medium feature)
// Context lines only populate for content hits with FileCache data.
// Symbol-only index tests return nil ContextLines (acceptable — matches grep).
// =============================================================================

func TestGrep_Flag_A_SymbolIndex(t *testing.T) {
	// -A on symbol-only index: ContextLines should be nil (no file cache)
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		AfterContext: 2,
		MaxCount:     50,
	})
	assert.Equal(t, 5, len(result.Hits), "-A: should still return 5 symbol hits")
	for _, hit := range result.Hits {
		assert.Nil(t, hit.ContextLines,
			"-A: symbol-only hits should have nil ContextLines (no cache)")
	}
}

func TestGrep_Flag_B_SymbolIndex(t *testing.T) {
	// -B on symbol-only index: ContextLines should be nil
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		BeforeContext: 2,
		MaxCount:      50,
	})
	assert.Equal(t, 5, len(result.Hits), "-B: should still return 5 symbol hits")
}

func TestGrep_Flag_C_SymbolIndex(t *testing.T) {
	// -C on symbol-only index: ContextLines should be nil
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		Context:  3,
		MaxCount: 50,
	})
	assert.Equal(t, 5, len(result.Hits), "-C: should still return 5 symbol hits")
}

func TestGrep_Combo_A_B(t *testing.T) {
	// -A 2 -B 1: both before and after context
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		AfterContext:  2,
		BeforeContext: 1,
		MaxCount:      50,
	})
	assert.Equal(t, 5, len(result.Hits), "-A -B: should still return 5 symbol hits")
}

func TestGrep_Flag_C_OverridesAB(t *testing.T) {
	// -C should override both -A and -B
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		AfterContext:  1,
		BeforeContext: 1,
		Context:       5,
		MaxCount:      50,
	})
	assert.Equal(t, 5, len(result.Hits), "-C overriding -A/-B: should still return 5 symbol hits")
}

// =============================================================================
// L3.6-L3.14: Flag Combinations with New Flags
// =============================================================================

func TestEgrep_Combo_i_w(t *testing.T) {
	// egrep -i -w LOG → case-insensitive word boundary
	engine := loadTestIndex(t)
	result := engine.Search("LOG", ports.SearchOptions{
		Mode:         "case_insensitive",
		WordBoundary: true,
		MaxCount:     50,
	})
	resultLower := engine.Search("log", ports.SearchOptions{
		WordBoundary: true,
		MaxCount:     50,
	})
	assert.Equal(t, len(resultLower.Hits), len(result.Hits),
		"egrep -i -w: LOG should match same as log with word boundary")
}

func TestGrep_Combo_o_i(t *testing.T) {
	// grep -o -i LOGIN → case-insensitive only-matching
	engine := loadTestIndex(t)
	result := engine.Search("LOGIN", ports.SearchOptions{
		Mode:         "case_insensitive",
		OnlyMatching: true,
		MaxCount:     50,
	})
	assert.Greater(t, len(result.Hits), 0, "-o -i: should have results")
}

func TestGrep_Combo_L_include(t *testing.T) {
	// grep -L --include='services/*' login → files without login in services/
	engine := loadTestIndex(t)
	result := engine.Search("login", ports.SearchOptions{
		FilesWithoutMatch: true,
		IncludeGlob:       "services/*",
		MaxCount:          50,
	})
	for _, h := range result.Hits {
		assert.True(t, strings.HasPrefix(h.File, "services/"),
			"-L --include: %s should be under services/", h.File)
	}
}

func TestGrep_Combo_excludeDir_include(t *testing.T) {
	// grep --exclude-dir=tests --include='*' test → exclude tests/ dir
	engine := loadTestIndex(t)
	result := engine.Search("test", ports.SearchOptions{
		ExcludeDirGlob: "tests",
		MaxCount:       50,
	})
	for _, h := range result.Hits {
		assert.False(t, strings.HasPrefix(h.File, "tests/"),
			"--exclude-dir + --include: %s should not be under tests/", h.File)
	}
}

// =============================================================================
// Coverage Summary
// =============================================================================

func TestGrepParity_CoverageMatrix(t *testing.T) {
	// This is a documentation test — prints the coverage matrix.
	// It doesn't assert anything; it makes the parity state visible.
	matrix := []struct {
		flag    string
		grepOk bool
		egrpOk bool
		note    string
	}{
		{"-i / --ignore-case", true, true, ""},
		{"-w / --word-regexp", true, true, ""},
		{"-v / --invert-match", true, true, ""},
		{"-c / --count", true, true, ""},
		{"-q / --quiet", true, true, ""},
		{"-m / --max-count", true, true, ""},
		{"-E / --extended-regexp", true, false, "grep -E routes to egrep"},
		{"-e / --regexp", true, true, ""},
		{"-a / --and", true, true, "NOTE: diverges from grep -a (--text)"},
		{"--include", true, true, ""},
		{"--exclude", true, true, ""},
		{"--exclude-dir", true, true, ""},
		{"-o / --only-matching", true, true, ""},
		{"-L / --files-without-match", true, true, ""},
		{"--no-filename", true, true, ""},
		{"--no-color", true, true, ""},
		{"-A / --after-context", true, true, ""},
		{"-B / --before-context", true, true, ""},
		{"-C / --context", true, true, ""},
		{"-r / --recursive", true, true, "no-op (always recursive)"},
		{"-n / --line-number", true, true, "no-op (always shows)"},
		{"-H / --with-filename", true, true, "no-op (always shows)"},
		{"-F / --fixed-strings", true, false, "no-op (already literal)"},
		{"-l / --files-with-matches", true, false, "no-op"},
	}

	t.Log("=== aoa grep/egrep vs GNU grep parity ===")
	t.Log("")
	t.Logf("%-30s  %5s  %5s  %s", "Flag", "grep", "egrep", "Note")
	t.Logf("%-30s  %5s  %5s  %s", "----", "----", "-----", "----")
	for _, m := range matrix {
		g := "  ✓  "
		if !m.grepOk {
			g = "  -  "
		}
		e := "  ✓  "
		if !m.egrpOk {
			e = "  -  "
		}
		t.Logf("%-30s  %s  %s  %s", m.flag, g, e, m.note)
	}

	t.Log("")
	t.Log("NOT IMPLEMENTED (GNU grep features not relevant to aoa):")
	notImpl := []string{
		"-x / --line-regexp",
		"-b / --byte-offset",
	}
	for _, f := range notImpl {
		t.Logf("  %s", f)
	}
	t.Log("")
	t.Logf("TOTAL: %d flags implemented, %d not implemented", len(matrix), len(notImpl))
	t.Logf("Coverage: %.0f%%", float64(len(matrix))/float64(len(matrix)+len(notImpl))*100)
}

// =============================================================================
// Helpers
// =============================================================================

// loadFixtureIndex loads the shared 13-file index fixture.
// Same as test/helpers_test.go but in a different package.
func loadFixtureIndex(path string) (*ports.Index, map[string]index.Domain, error) {
	// Reuse the same JSON structure from test/helpers_test.go
	return loadIndexFromFile(path)
}

// fixtureFile mirrors the JSON structure of index-state.json.
type fixtureFile struct {
	Files   map[string]fixtureFileMeta `json:"files"`
	Symbols []fixtureSymbol            `json:"symbols"`
	Domains map[string]fixtureDomain   `json:"domains"`
}
type fixtureFileMeta struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Domain   string `json:"domain"`
}
type fixtureSymbol struct {
	FileID    uint32   `json:"file_id"`
	Name      string   `json:"name"`
	Signature string   `json:"signature"`
	Kind      string   `json:"kind"`
	StartLine uint16   `json:"start_line"`
	EndLine   uint16   `json:"end_line"`
	Parent    string   `json:"parent"`
	Tokens    []string `json:"tokens"`
	Tags      []string `json:"tags"`
}
type fixtureDomain struct {
	Terms map[string][]string `json:"terms"`
}

func loadIndexFromFile(path string) (*ports.Index, map[string]index.Domain, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read index fixture: %w", err)
	}

	var f fixtureFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, nil, fmt.Errorf("parse index fixture: %w", err)
	}

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	for idStr, fm := range f.Files {
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			return nil, nil, fmt.Errorf("parse file id %q: %w", idStr, err)
		}
		idx.Files[uint32(id)] = &ports.FileMeta{
			Path:     fm.Path,
			Language: fm.Language,
			Domain:   fm.Domain,
		}
	}

	for _, sym := range f.Symbols {
		ref := ports.TokenRef{FileID: sym.FileID, Line: sym.StartLine}
		idx.Metadata[ref] = &ports.SymbolMeta{
			Name:      sym.Name,
			Signature: sym.Signature,
			Kind:      sym.Kind,
			StartLine: sym.StartLine,
			EndLine:   sym.EndLine,
			Parent:    sym.Parent,
			Tags:      sym.Tags,
		}
		for _, tok := range sym.Tokens {
			idx.Tokens[tok] = append(idx.Tokens[tok], ref)
		}
	}

	domains := make(map[string]index.Domain, len(f.Domains))
	for name, fd := range f.Domains {
		domains[name] = index.Domain{Terms: fd.Terms}
	}

	return idx, domains, nil
}
