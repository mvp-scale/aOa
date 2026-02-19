package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildInvertTestIndex creates a small index with 3 symbols for invert-match tests.
// Symbols: "login" (file1.go:10), "logout" (file1.go:20), "dashboard" (file2.py:5)
func buildInvertTestIndex() (*ports.Index, map[string]Domain) {
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	idx.Files[1] = &ports.FileMeta{Path: "auth/file1.go", Language: "go", Size: 500}
	idx.Files[2] = &ports.FileMeta{Path: "ui/file2.py", Language: "python", Size: 300}

	refLogin := ports.TokenRef{FileID: 1, Line: 10}
	refLogout := ports.TokenRef{FileID: 1, Line: 20}
	refDashboard := ports.TokenRef{FileID: 2, Line: 5}

	idx.Metadata[refLogin] = &ports.SymbolMeta{Name: "login", Signature: "login()", Kind: "function", StartLine: 10, EndLine: 15}
	idx.Metadata[refLogout] = &ports.SymbolMeta{Name: "logout", Signature: "logout()", Kind: "function", StartLine: 20, EndLine: 25}
	idx.Metadata[refDashboard] = &ports.SymbolMeta{Name: "dashboard", Signature: "dashboard()", Kind: "function", StartLine: 5, EndLine: 12}

	idx.Tokens["login"] = []ports.TokenRef{refLogin}
	idx.Tokens["logout"] = []ports.TokenRef{refLogout}
	idx.Tokens["log"] = []ports.TokenRef{refLogin, refLogout}
	idx.Tokens["dashboard"] = []ports.TokenRef{refDashboard}

	return idx, make(map[string]Domain)
}

func TestInvertMatch_Literal(t *testing.T) {
	idx, domains := buildInvertTestIndex()
	engine := NewSearchEngine(idx, domains, "")

	// Normal: "login" matches 1 symbol
	normal := engine.Search("login", ports.SearchOptions{})
	assert.Equal(t, 1, len(normal.Hits))
	assert.Equal(t, "login()", normal.Hits[0].Symbol)

	// Inverted: should return the other 2 symbols
	inverted := engine.Search("login", ports.SearchOptions{InvertMatch: true})
	assert.Equal(t, 2, len(inverted.Hits))
	names := []string{inverted.Hits[0].Symbol, inverted.Hits[1].Symbol}
	assert.Contains(t, names, "logout()")
	assert.Contains(t, names, "dashboard()")
}

func TestInvertMatch_Regex(t *testing.T) {
	idx, domains := buildInvertTestIndex()
	engine := NewSearchEngine(idx, domains, "")

	// Regex "log.*" matches login and logout
	normal := engine.Search("log.*", ports.SearchOptions{Mode: "regex"})
	assert.Equal(t, 2, len(normal.Hits))

	// Inverted: should return only dashboard
	inverted := engine.Search("log.*", ports.SearchOptions{Mode: "regex", InvertMatch: true})
	assert.Equal(t, 1, len(inverted.Hits))
	assert.Equal(t, "dashboard()", inverted.Hits[0].Symbol)
}

func TestInvertMatch_OR(t *testing.T) {
	idx, domains := buildInvertTestIndex()
	engine := NewSearchEngine(idx, domains, "")

	// OR: "login dashboard" matches 2 symbols (login + dashboard via separate tokens)
	normal := engine.Search("login dashboard", ports.SearchOptions{})
	assert.Equal(t, 2, len(normal.Hits))

	// Inverted: should return only logout
	inverted := engine.Search("login dashboard", ports.SearchOptions{InvertMatch: true})
	assert.Equal(t, 1, len(inverted.Hits))
	assert.Equal(t, "logout()", inverted.Hits[0].Symbol)
}

func TestInvertMatch_AND(t *testing.T) {
	idx, domains := buildInvertTestIndex()
	// Add a symbol that has both "log" and "auth" tokens
	refBoth := ports.TokenRef{FileID: 1, Line: 30}
	idx.Metadata[refBoth] = &ports.SymbolMeta{Name: "authLog", Signature: "authLog()", Kind: "function", StartLine: 30, EndLine: 35}
	idx.Tokens["auth"] = []ports.TokenRef{refBoth}
	idx.Tokens["log"] = append(idx.Tokens["log"], refBoth)

	engine := NewSearchEngine(idx, domains, "")

	// AND: "log,auth" matches only authLog (has both tokens)
	normal := engine.Search("log,auth", ports.SearchOptions{AndMode: true})
	assert.Equal(t, 1, len(normal.Hits))
	assert.Equal(t, "authLog()", normal.Hits[0].Symbol)

	// Inverted: all except authLog = login, logout, dashboard
	inverted := engine.Search("log,auth", ports.SearchOptions{AndMode: true, InvertMatch: true})
	assert.Equal(t, 3, len(inverted.Hits))
}

func TestInvertMatch_Content(t *testing.T) {
	// Create temp dir with a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	content := "package main\n\nfunc login() {\n\treturn true\n}\n\nfunc dashboard() {\n\treturn false\n}\n"
	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}
	idx.Files[1] = &ports.FileMeta{Path: "test.go", Language: "go", Size: int64(len(content))}

	engine := NewSearchEngine(idx, make(map[string]Domain), tmpDir)

	// Normal content scan: "login" appears in file
	normal := engine.Search("login", ports.SearchOptions{})
	loginFound := false
	for _, h := range normal.Hits {
		if h.Kind == "content" {
			loginFound = true
		}
	}
	assert.True(t, loginFound, "should find login as content hit")

	// Inverted: lines NOT matching "login" should be returned
	inverted := engine.Search("login", ports.SearchOptions{InvertMatch: true})
	for _, h := range inverted.Hits {
		if h.Kind == "content" {
			assert.NotContains(t, h.Content, "login", "inverted content should not contain 'login'")
		}
	}
}

func TestInvertMatch_CountOnly(t *testing.T) {
	idx, domains := buildInvertTestIndex()
	engine := NewSearchEngine(idx, domains, "")

	// -v -c: count non-matches
	result := engine.Search("login", ports.SearchOptions{InvertMatch: true, CountOnly: true})
	assert.Equal(t, 2, result.Count)
}

func TestInvertMatch_Quiet(t *testing.T) {
	idx, domains := buildInvertTestIndex()
	engine := NewSearchEngine(idx, domains, "")

	// -v -q: exit code 0 when non-matches exist
	result := engine.Search("login", ports.SearchOptions{InvertMatch: true, Quiet: true})
	assert.Equal(t, 0, result.ExitCode)

	// -v -q with query matching ALL symbols: exit code 1 (no non-matches)
	// "log" matches login and logout via the "log" token
	// Add dashboard to log token so everything matches
	idx.Tokens["login"] = append(idx.Tokens["login"], ports.TokenRef{FileID: 2, Line: 5})
	idx.Tokens["logout"] = append(idx.Tokens["logout"], ports.TokenRef{FileID: 2, Line: 5})
	engine2 := NewSearchEngine(idx, domains, "")
	result2 := engine2.Search("login logout dashboard", ports.SearchOptions{InvertMatch: true, Quiet: true})
	assert.Equal(t, 1, result2.ExitCode)
}

func TestInvertMatch_WithGlob(t *testing.T) {
	idx, domains := buildInvertTestIndex()
	engine := NewSearchEngine(idx, domains, "")

	// -v --include *.go: inverts only within Go files
	inverted := engine.Search("login", ports.SearchOptions{InvertMatch: true, IncludeGlob: "*.go"})
	// login is in file1.go, so inverted within *.go gives logout only
	assert.Equal(t, 1, len(inverted.Hits))
	assert.Equal(t, "logout()", inverted.Hits[0].Symbol)
}
