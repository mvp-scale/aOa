package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupContentTest creates a temp directory with files and returns an engine pointing at it.
func setupContentTest(t *testing.T, files map[string]string) (*SearchEngine, string) {
	t.Helper()
	dir := t.TempDir()

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	var fileID uint32
	for name, content := range files {
		fileID++
		absPath := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o755))
		require.NoError(t, os.WriteFile(absPath, []byte(content), 0o644))

		info, _ := os.Stat(absPath)
		idx.Files[fileID] = &ports.FileMeta{
			Path:     name,
			Language: "go",
			Size:     info.Size(),
		}
	}

	engine := NewSearchEngine(idx, nil, dir)
	return engine, dir
}

func TestContentSearch_FindsBodyMatch(t *testing.T) {
	engine, _ := setupContentTest(t, map[string]string{
		"main.go": "package main\n\nfunc main() {\n\tprojectID := getProjectID()\n\tfmt.Println(projectID)\n}\n",
	})

	// Add a symbol hit on line 3 for "main"
	ref := ports.TokenRef{FileID: 1, Line: 3}
	engine.idx.Tokens["main"] = []ports.TokenRef{ref}
	engine.idx.Metadata[ref] = &ports.SymbolMeta{
		Name: "main", Kind: "function", StartLine: 3, EndLine: 6,
	}

	result := engine.Search("projectID", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	// Should find content hits in the body (lines 4 and 5 contain "projectID")
	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	assert.GreaterOrEqual(t, len(contentHits), 1, "expected at least one content hit")
	for _, h := range contentHits {
		assert.Contains(t, strings.ToLower(h.Content), "projectid")
	}
}

func TestContentSearch_DedupWithSymbol(t *testing.T) {
	engine, _ := setupContentTest(t, map[string]string{
		"auth.go": "package auth\n\nfunc Login() {\n\t// login logic\n}\n",
	})

	// Add a symbol hit for "login" on line 3
	ref := ports.TokenRef{FileID: 1, Line: 3}
	engine.idx.Tokens["login"] = []ports.TokenRef{ref}
	engine.idx.Metadata[ref] = &ports.SymbolMeta{
		Name: "Login", Kind: "function", StartLine: 3, EndLine: 5,
	}

	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	// Line 3 should appear only once (as symbol hit), not duplicated as content
	line3Hits := 0
	for _, h := range result.Hits {
		if h.Line == 3 && h.File == "auth.go" {
			line3Hits++
			assert.Equal(t, "symbol", h.Kind, "line 3 should be a symbol hit")
		}
	}
	assert.Equal(t, 1, line3Hits, "line 3 should appear exactly once")

	// Line 4 has "login" in a comment — should appear as content hit
	var contentLine4 bool
	for _, h := range result.Hits {
		if h.Line == 4 && h.Kind == "content" {
			contentLine4 = true
		}
	}
	assert.True(t, contentLine4, "line 4 (comment with 'login') should be a content hit")
}

func TestContentSearch_RegexMode(t *testing.T) {
	engine, _ := setupContentTest(t, map[string]string{
		"handler.go": "package handler\n\nfunc HandleRequest(w http.ResponseWriter, r *http.Request) {\n}\n",
	})

	result := engine.Search("Handle.*Request", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 50,
	})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	assert.GreaterOrEqual(t, len(contentHits), 1, "expected regex content match")
	assert.Contains(t, contentHits[0].Content, "HandleRequest")
}

func TestContentSearch_NoDomainButHasTerms(t *testing.T) {
	dir := t.TempDir()
	content := "package app\n\nfunc Start() {\n\tlogin(session)\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.go"), []byte(content), 0o644))

	ref := ports.TokenRef{FileID: 1, Line: 3}
	idx := &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"start":   {ref},
			"login":   {ref},
			"session": {ref},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			ref: {Name: "Start", Kind: "function", Signature: "Start()", StartLine: 3, EndLine: 5},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "app.go", Language: "go", Size: int64(len(content))},
		},
	}

	// Provide atlas domains so keyword->term resolution works.
	// "login" keyword -> "auth_login" term, "session" keyword -> "sessions" term.
	domains := map[string]Domain{
		"@authentication": {Terms: map[string][]string{
			"auth_login": {"login", "signin"},
			"sessions":   {"session", "cookie"},
		}},
	}

	engine := NewSearchEngine(idx, domains, dir)

	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	var foundContent bool
	for _, h := range result.Hits {
		if h.Kind == "content" {
			foundContent = true
			// Content hits must NOT have a domain
			assert.Empty(t, h.Domain, "content hit should not have a domain")
			// Content hits SHOULD have terms (resolved from keywords via atlas)
			assert.NotEmpty(t, h.Tags, "content hit should have terms")
			// Tags should be atlas terms, not raw keywords
			for _, tag := range h.Tags {
				assert.NotEqual(t, "login", tag, "tags should be terms, not raw keywords")
				assert.NotEqual(t, "session", tag, "tags should be terms, not raw keywords")
			}
			// Content hits SHOULD have enclosing symbol context
			assert.Equal(t, "Start()", h.Symbol, "content hit should show enclosing symbol")
			assert.Equal(t, [2]int{3, 5}, h.Range, "content hit should show enclosing symbol range")
		}
	}
	assert.True(t, foundContent, "expected at least one content hit")
}

func TestContentSearch_NestedEnclosingSymbol(t *testing.T) {
	dir := t.TempDir()
	content := "package app\n\ntype Server struct {\n}\n\nfunc (s *Server) Handle() {\n\tprojectID := getID()\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server.go"), []byte(content), 0o644))

	classRef := ports.TokenRef{FileID: 1, Line: 3}
	methodRef := ports.TokenRef{FileID: 1, Line: 6}
	idx := &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"server": {classRef},
			"handle": {methodRef},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			classRef:  {Name: "Server", Kind: "class", Signature: "class Server", StartLine: 3, EndLine: 8},
			methodRef: {Name: "Handle", Kind: "method", Signature: "Handle()", Parent: "Server", StartLine: 6, EndLine: 8},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "server.go", Language: "go", Size: int64(len(content))},
		},
	}

	engine := NewSearchEngine(idx, nil, dir)
	result := engine.Search("projectID", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	var contentHit *Hit
	for i, h := range result.Hits {
		if h.Kind == "content" && h.Line == 7 {
			contentHit = &result.Hits[i]
			break
		}
	}
	require.NotNil(t, contentHit, "expected content hit on line 7")

	// Should get the innermost enclosing symbol (Handle method, not Server class)
	assert.Equal(t, "Server.Handle()", contentHit.Symbol, "should show innermost enclosing symbol")
	assert.Equal(t, [2]int{6, 8}, contentHit.Range, "should show innermost symbol range")
	assert.Empty(t, contentHit.Domain, "content hit should not have a domain")
}

func TestContentSearch_SkipsLargeFiles(t *testing.T) {
	// Create a file just over 1MB
	bigContent := strings.Repeat("x", maxContentFileSize+1) + "\nsearchterm\n"

	engine, _ := setupContentTest(t, map[string]string{
		"big.go": bigContent,
	})

	result := engine.Search("searchterm", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	// Should have no hits — file is too large
	assert.Empty(t, result.Hits, "large file should be skipped")
}

func TestContentSearch_MissingFile(t *testing.T) {
	dir := t.TempDir()
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "deleted.go", Language: "go", Size: 100},
		},
	}

	engine := NewSearchEngine(idx, nil, dir)

	// File doesn't exist on disk — should silently skip
	result := engine.Search("anything", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)
	assert.Empty(t, result.Hits, "missing file should be silently skipped")
}

// setupContentTestWithCache creates an engine with a FileCache attached.
func setupContentTestWithCache(t *testing.T, files map[string]string) (*SearchEngine, string) {
	t.Helper()
	dir := t.TempDir()

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	var fileID uint32
	for name, content := range files {
		fileID++
		absPath := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o755))
		require.NoError(t, os.WriteFile(absPath, []byte(content), 0o644))

		info, _ := os.Stat(absPath)
		idx.Files[fileID] = &ports.FileMeta{
			Path:     name,
			Language: "go",
			Size:     info.Size(),
		}
	}

	engine := NewSearchEngine(idx, nil, dir)
	cache := NewFileCache(0)
	engine.SetCache(cache)
	return engine, dir
}

func TestContentSearch_Cached_SingleToken(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"main.go": "package main\n\nfunc main() {\n\tprojectID := getProjectID()\n\tfmt.Println(projectID)\n}\n",
	})

	ref := ports.TokenRef{FileID: 1, Line: 3}
	engine.idx.Tokens["main"] = []ports.TokenRef{ref}
	engine.idx.Metadata[ref] = &ports.SymbolMeta{
		Name: "main", Kind: "function", StartLine: 3, EndLine: 6,
	}

	result := engine.Search("projectID", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	assert.GreaterOrEqual(t, len(contentHits), 1, "expected at least one content hit")
	for _, h := range contentHits {
		assert.Equal(t, "content", h.Kind)
	}
}

func TestContentSearch_Cached_SubstringSemantics(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"auth.go":    "package auth\n\nfunc LoginSession() {\n}\n",
		"session.go": "package session\n\nfunc StartSession() {\n}\n",
	})

	// "login session" as literal substring — grep semantics, not token OR.
	// Only auth.go line 3 "LoginSession" contains "loginsession" (case-insensitive).
	// session.go has "StartSession" which does NOT contain "login session".
	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	// "login" appears as substring in auth.go line 3 ("LoginSession")
	assert.GreaterOrEqual(t, len(contentHits), 1, "should find substring match")

	found := false
	for _, h := range contentHits {
		if h.File == "auth.go" && h.Line == 3 {
			found = true
		}
	}
	assert.True(t, found, "should find 'login' in LoginSession on auth.go:3")
}

func TestContentSearch_Cached_AND(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"both.go":    "package both\n\nfunc LoginSession() {\n}\n",
		"partial.go": "package partial\n\nfunc Login() {\n}\n",
	})

	// AND search: "login,session" — only lines with both tokens match
	result := engine.Search("login,session", ports.SearchOptions{
		AndMode:  true,
		MaxCount: 50,
	})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	// "LoginSession" on line 3 of both.go has both "login" and "session" tokens
	// partial.go line 3 only has "login", not "session"
	require.GreaterOrEqual(t, len(contentHits), 1, "AND should find line with both tokens")
	for _, h := range contentHits {
		assert.Equal(t, "both.go", h.File, "AND intersection should only match both.go")
	}
}

func TestContentSearch_Cached_Regex(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"handler.go": "package handler\n\nfunc HandleRequest(w http.ResponseWriter, r *http.Request) {\n}\n",
	})

	result := engine.Search("Handle.*Request", ports.SearchOptions{
		Mode:     "regex",
		MaxCount: 50,
	})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	assert.GreaterOrEqual(t, len(contentHits), 1, "regex should match content")
	assert.Contains(t, contentHits[0].Content, "HandleRequest")
}

func TestContentSearch_Cached_InvertMatch(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"a.go": "package a\nfunc Alpha() {}\nfunc Beta() {}\n",
	})

	result := engine.Search("alpha", ports.SearchOptions{
		InvertMatch: true,
		MaxCount:    50,
	})
	require.NotNil(t, result)

	// InvertMatch should fall back to brute-force; lines NOT containing "alpha" should match
	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	assert.GreaterOrEqual(t, len(contentHits), 1, "invert match should produce hits")
	for _, h := range contentHits {
		assert.NotContains(t, strings.ToLower(h.Content), "alpha",
			"invert match content should not contain 'alpha'")
	}
}

func TestContentSearch_SubstringMatch(t *testing.T) {
	// Verify grep-compatible substring matching: "tree" must find "btree", "subtree"
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"data.go": "package data\n\nvar store = NewBtree()\nvar sub = subtreeOf(root)\nvar t = tree\n",
	})

	result := engine.Search("tree", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	// Must find all three: "Btree" (line 3), "subtreeOf" (line 4), "tree" (line 5)
	assert.Equal(t, 3, len(contentHits), "substring match should find btree, subtree, and tree")

	lines := make(map[int]bool)
	for _, h := range contentHits {
		lines[h.Line] = true
	}
	assert.True(t, lines[3], "should match 'Btree' on line 3")
	assert.True(t, lines[4], "should match 'subtreeOf' on line 4")
	assert.True(t, lines[5], "should match 'tree' on line 5")
}
