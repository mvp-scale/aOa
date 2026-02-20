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
		"auth.go":    "package auth\n\nfunc loginSession() {\n}\n",
		"session.go": "package session\n\nfunc startSession() {\n}\n",
	})

	// Case-sensitive substring: "login" matches "loginSession" on auth.go:3.
	// session.go has "startSession" which does NOT contain "login".
	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	assert.GreaterOrEqual(t, len(contentHits), 1, "should find substring match")

	found := false
	for _, h := range contentHits {
		if h.File == "auth.go" && h.Line == 3 {
			found = true
		}
	}
	assert.True(t, found, "should find 'login' in loginSession on auth.go:3")
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
		"a.go": "package a\nfunc alpha() {}\nfunc beta() {}\n",
	})

	result := engine.Search("alpha", ports.SearchOptions{
		InvertMatch: true,
		MaxCount:    50,
	})
	require.NotNil(t, result)

	// InvertMatch falls back to brute-force; lines NOT containing "alpha" should match
	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	assert.GreaterOrEqual(t, len(contentHits), 1, "invert match should produce hits")
	for _, h := range contentHits {
		assert.NotContains(t, h.Content, "alpha",
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

// --- Case-sensitivity tests (L2.5 G1 fix) ---

func TestContentSearch_CaseSensitiveDefault(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"mixed.go": "SessionID handler\nsessionid lower\nSESSIONID upper\n",
	})

	// Default mode (no -i): case-sensitive, should only match exact case
	result := engine.Search("sessionid", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	require.Equal(t, 1, len(contentHits), "case-sensitive should find only exact match")
	assert.Equal(t, 2, contentHits[0].Line, "should match line 2 (lowercase)")
}

func TestContentSearch_CaseInsensitiveFlag(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"mixed.go": "SessionID handler\nsessionid lower\nSESSIONID upper\n",
	})

	// -i mode: case-insensitive, should match all three
	result := engine.Search("sessionid", ports.SearchOptions{
		Mode:     "case_insensitive",
		MaxCount: 50,
	})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	assert.Equal(t, 3, len(contentHits), "case-insensitive should find all three case variants")
}

func TestContentSearch_CaseSensitiveRejectsWrongCase(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"auth.go": "func LoginHandler() {}\nfunc logoutHandler() {}\n",
	})

	// "login" should NOT match "LoginHandler" (capital L) but SHOULD match nothing else
	result := engine.Search("login", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	// "login" (lowercase) does not appear as a substring in either line:
	// "LoginHandler" has capital L, "logoutHandler" has "logout" not "login"
	assert.Equal(t, 0, len(contentHits), "case-sensitive should reject wrong case")
}

// --- Trigram dispatch tests ---

func TestContentSearch_TrigramPath_UsedForLongQuery(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"data.go": "authentication handler\nauthorization check\nvalidation error\n",
	})

	// "authentication" has len >= 3, should use trigram path
	result := engine.Search("authentication", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	require.Equal(t, 1, len(contentHits))
	assert.Equal(t, 1, contentHits[0].Line, "should find 'authentication' on line 1")
}

func TestContentSearch_ExtractTrigrams(t *testing.T) {
	tests := []struct {
		query    string
		expected int // number of unique trigrams
	}{
		{"ab", 0},        // too short
		{"abc", 1},       // one trigram
		{"tree", 2},      // "tre" + "ree"
		{"aaa", 1},       // dedup: "aaa" appears once
		{"abcabc", 3},    // "abc","bca","cab","abc" → 3 unique (abc,bca,cab)
		{"authentication", 12}, // 14-3+1=12 trigrams, most unique
	}

	for _, tt := range tests {
		trigrams := extractTrigrams(tt.query)
		if tt.expected == 0 {
			assert.Nil(t, trigrams, "query %q should have no trigrams", tt.query)
		} else {
			assert.Equal(t, tt.expected, len(trigrams), "query %q trigram count", tt.query)
		}
	}
}

func TestContentSearch_CanUseTrigram(t *testing.T) {
	assert.True(t, canUseTrigram("tree", ports.SearchOptions{}), "3+ chars, default mode")
	assert.True(t, canUseTrigram("tree", ports.SearchOptions{Mode: "case_insensitive"}), "3+ chars, -i mode")
	assert.False(t, canUseTrigram("ab", ports.SearchOptions{}), "<3 chars")
	assert.False(t, canUseTrigram("tree", ports.SearchOptions{InvertMatch: true}), "InvertMatch")
	assert.False(t, canUseTrigram("tree", ports.SearchOptions{Mode: "regex"}), "regex mode")
	assert.False(t, canUseTrigram("tree", ports.SearchOptions{WordBoundary: true}), "word boundary")
	assert.False(t, canUseTrigram("tree", ports.SearchOptions{AndMode: true}), "AND mode")
}

// --- Edge cases + regression (L2.7) ---

func TestContentSearch_ShortQueryFallback(t *testing.T) {
	// Query < 3 chars can't use trigrams, must fall back to brute-force
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"main.go": "package main\n\nfunc go_handler() {\n}\n",
	})

	result := engine.Search("go", ports.SearchOptions{MaxCount: 50})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	// "go" appears in "package main" (no), "go_handler" (yes)
	// Brute-force case-sensitive: strings.Contains("func go_handler() {", "go") → true
	found := false
	for _, h := range contentHits {
		if h.Line == 3 {
			found = true
		}
	}
	assert.True(t, found, "short query should find match via brute-force fallback")
}

func TestContentSearch_ShortQueryCaseInsensitive(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"main.go": "package main\nfunc Go_handler() {\n}\n",
	})

	// Short query with -i: brute-force case-insensitive
	result := engine.Search("go", ports.SearchOptions{
		Mode:     "case_insensitive",
		MaxCount: 50,
	})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	found := false
	for _, h := range contentHits {
		if h.Line == 2 {
			found = true
		}
	}
	assert.True(t, found, "short query -i should find 'Go_handler' via brute-force")
}

func TestContentSearch_RegexWithCache(t *testing.T) {
	// Regex mode always uses brute-force (canUseTrigram returns false)
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"handler.go": "func handleAuth() {}\nfunc handlePayment() {}\nfunc doOther() {}\n",
	})

	result := engine.Search("handle[A-Z]", ports.SearchOptions{
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
	assert.Equal(t, 2, len(contentHits), "regex should find both handle* lines")
}

func TestContentSearch_WordBoundaryWithCache(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"data.go": "var tree = 1\nvar subtree = 2\nvar treehouse = 3\n",
	})

	// -w mode: whole word only, falls back to brute-force
	result := engine.Search("tree", ports.SearchOptions{
		WordBoundary: true,
		MaxCount:     50,
	})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	// "tree" as whole word appears only on line 1 ("var tree = 1")
	// "subtree" and "treehouse" should NOT match with word boundary
	require.Equal(t, 1, len(contentHits), "word boundary should find only exact 'tree'")
	assert.Equal(t, 1, contentHits[0].Line)
}

func TestContentSearch_ANDWithCache(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"both.go":    "func loginSession() {}\n",
		"partial.go": "func loginHandler() {}\n",
	})

	// AND mode: both "login" and "session" must appear on the same line
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
	require.Equal(t, 1, len(contentHits), "AND should match only line with both terms")
	assert.Equal(t, "both.go", contentHits[0].File)
}

func TestContentSearch_InvertMatchWithCache_ExcludesMatches(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"data.go": "func authentication() {}\nfunc authorization() {}\nfunc validation() {}\n",
	})

	// Invert match: lines NOT containing "auth" (brute-force)
	result := engine.Search("auth", ports.SearchOptions{
		InvertMatch: true,
		MaxCount:    50,
	})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	// Only line 3 "func validation() {}" doesn't contain "auth"
	require.Equal(t, 1, len(contentHits), "invert should exclude lines with 'auth'")
	assert.Equal(t, 3, contentHits[0].Line, "should be the validation line")
}

func TestContentSearch_GlobFilterWithTrigram(t *testing.T) {
	engine, _ := setupContentTestWithCache(t, map[string]string{
		"auth.go": "func authentication() {}\n",
		"auth.py": "def authentication():\n",
	})

	// Include only .go files — trigram path should respect glob
	result := engine.Search("authentication", ports.SearchOptions{
		IncludeGlob: "*.go",
		MaxCount:    50,
	})
	require.NotNil(t, result)

	var contentHits []Hit
	for _, h := range result.Hits {
		if h.Kind == "content" {
			contentHits = append(contentHits, h)
		}
	}
	require.Equal(t, 1, len(contentHits))
	assert.Equal(t, "auth.go", contentHits[0].File, "glob should filter to .go only")
}
