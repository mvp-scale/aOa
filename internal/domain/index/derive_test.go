package index

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
)

// buildDeriveTestEngine creates a SearchEngine with controlled domain definitions
// and a small index for DeriveFileDomains unit tests.
//
// File layout:
//
//	fileID 1 → "auth/login.go":   tokens "login", "logout", "authenticate"
//	fileID 2 → "cmd/main.go":     tokens "test", "assert", "coverage"
//	fileID 3 → "internal/noop.go": no atlas-matching tokens
func buildDeriveTestEngine() *SearchEngine {
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	// File 1: auth — three symbols, all with auth-domain tokens.
	idx.Files[1] = &ports.FileMeta{Path: "auth/login.go", Language: "go"}
	ref1a := ports.TokenRef{FileID: 1, Line: 10}
	ref1b := ports.TokenRef{FileID: 1, Line: 20}
	ref1c := ports.TokenRef{FileID: 1, Line: 30}
	idx.Metadata[ref1a] = &ports.SymbolMeta{Name: "Login", Kind: "function", StartLine: 10, EndLine: 15}
	idx.Metadata[ref1b] = &ports.SymbolMeta{Name: "Logout", Kind: "function", StartLine: 20, EndLine: 25}
	idx.Metadata[ref1c] = &ports.SymbolMeta{Name: "Authenticate", Kind: "function", StartLine: 30, EndLine: 35}
	idx.Tokens["login"] = []ports.TokenRef{ref1a}
	idx.Tokens["logout"] = []ports.TokenRef{ref1b}
	idx.Tokens["authenticate"] = []ports.TokenRef{ref1c}

	// File 2: testing — two symbols with testing-domain tokens.
	idx.Files[2] = &ports.FileMeta{Path: "cmd/main.go", Language: "go"}
	ref2a := ports.TokenRef{FileID: 2, Line: 5}
	ref2b := ports.TokenRef{FileID: 2, Line: 15}
	idx.Metadata[ref2a] = &ports.SymbolMeta{Name: "TestRun", Kind: "function", StartLine: 5, EndLine: 10}
	idx.Metadata[ref2b] = &ports.SymbolMeta{Name: "AssertEqual", Kind: "function", StartLine: 15, EndLine: 20}
	idx.Tokens["test"] = []ports.TokenRef{ref2a}
	idx.Tokens["assert"] = []ports.TokenRef{ref2b}

	// File 3: no atlas tokens → should be absent from result.
	idx.Files[3] = &ports.FileMeta{Path: "internal/noop.go", Language: "go"}
	ref3a := ports.TokenRef{FileID: 3, Line: 1}
	idx.Metadata[ref3a] = &ports.SymbolMeta{Name: "xyzUnknown", Kind: "function", StartLine: 1, EndLine: 5}
	idx.Tokens["xyzunknown"] = []ports.TokenRef{ref3a}

	domains := map[string]Domain{
		"authentication": {Terms: map[string][]string{
			"login":    {"login", "logout", "authenticate", "credential"},
			"password": {"bcrypt", "argon2", "salt"},
		}},
		"unit_testing": {Terms: map[string][]string{
			"assertions": {"assert", "expect", "equal"},
			"structure":  {"test", "suite", "describe"},
		}},
	}

	return NewSearchEngine(idx, domains, "")
}

// TestDeriveFileDomains_KnownDomain verifies that a file whose symbols match
// atlas keywords for a single domain is labelled with that domain (with "@" prefix).
func TestDeriveFileDomains_KnownDomain(t *testing.T) {
	engine := buildDeriveTestEngine()
	result := engine.DeriveFileDomains()

	assert.Equal(t, "@authentication", result["auth/login.go"],
		"auth/login.go: login/logout/authenticate tokens → @authentication")
	assert.Equal(t, "@unit_testing", result["cmd/main.go"],
		"cmd/main.go: test/assert tokens → @unit_testing")
}

// TestDeriveFileDomains_EmptyEnrichment verifies that a file with no
// atlas-matching tokens is omitted from the result (omitempty — no fake domains).
func TestDeriveFileDomains_EmptyEnrichment(t *testing.T) {
	engine := buildDeriveTestEngine()
	result := engine.DeriveFileDomains()

	_, present := result["internal/noop.go"]
	assert.False(t, present, "file with no atlas-matching tokens must be absent from result")
}

// TestDeriveFileDomains_NoDomains verifies that an engine with an empty domain
// map produces an empty result (no scoring → nothing to emit).
func TestDeriveFileDomains_NoDomains(t *testing.T) {
	idx := &ports.Index{
		Tokens:   map[string][]ports.TokenRef{"login": {{FileID: 1, Line: 1}}},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{{FileID: 1, Line: 1}: {Name: "login"}},
		Files:    map[uint32]*ports.FileMeta{1: {Path: "auth/auth.go"}},
	}
	engine := NewSearchEngine(idx, map[string]Domain{}, "")
	result := engine.DeriveFileDomains()
	assert.Empty(t, result, "no domains → empty result, no fake domains")
}

// TestDeriveFileDomains_Determinism verifies that two successive calls produce
// identical output (stable map iteration workaround: modal + lexicographic tie).
func TestDeriveFileDomains_Determinism(t *testing.T) {
	engine := buildDeriveTestEngine()
	r1 := engine.DeriveFileDomains()
	r2 := engine.DeriveFileDomains()

	assert.Equal(t, len(r1), len(r2), "result length must be stable across calls")
	for path, d := range r1 {
		assert.Equal(t, d, r2[path], "domain for %s must be stable across calls", path)
	}
}

// TestDeriveFileDomains_TieBreaking verifies that when two domains score equally
// for a file, the lexicographically-first domain name wins.
func TestDeriveFileDomains_TieBreaking(t *testing.T) {
	// One symbol with one token; token appears in both "aaa" and "zzz" domains.
	idx := &ports.Index{
		Tokens:   map[string][]ports.TokenRef{"shared": {{FileID: 1, Line: 1}}},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{{FileID: 1, Line: 1}: {Name: "Shared"}},
		Files:    map[uint32]*ports.FileMeta{1: {Path: "pkg/shared.go"}},
	}
	domains := map[string]Domain{
		"aaa_domain": {Terms: map[string][]string{"t1": {"shared"}}},
		"zzz_domain": {Terms: map[string][]string{"t1": {"shared"}}},
	}
	engine := NewSearchEngine(idx, domains, "")
	result := engine.DeriveFileDomains()

	assert.Equal(t, "@aaa_domain", result["pkg/shared.go"],
		"tie in score → lexicographically-first domain (@aaa_domain) must win")
}

// TestDeriveFileDomains_EmptyIndex verifies that an empty index produces an
// empty result without panicking.
func TestDeriveFileDomains_EmptyIndex(t *testing.T) {
	idx := &ports.Index{
		Tokens:   map[string][]ports.TokenRef{},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{},
		Files:    map[uint32]*ports.FileMeta{},
	}
	engine := NewSearchEngine(idx, map[string]Domain{"auth": {Terms: map[string][]string{"login": {"login"}}}}, "")
	result := engine.DeriveFileDomains()
	assert.Empty(t, result, "empty index → empty result")
}
