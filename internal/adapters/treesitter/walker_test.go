//go:build !lean

package treesitter

import (
	"testing"

	"github.com/corey/aoa/internal/domain/analyzer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseAndWalk(t *testing.T, filename string, source string, rules []analyzer.Rule, isMain bool) WalkResult {
	t.Helper()
	p := NewParser()
	tree, lang, err := p.ParseToTree(filename, []byte(source))
	require.NoError(t, err)
	require.NotNil(t, tree, "tree should not be nil")
	defer tree.Close()

	return WalkForDimensions(tree.RootNode(), []byte(source), lang, rules, isMain)
}

func TestWalker_DeferInLoop(t *testing.T) {
	source := `package main

func handler() {
	for i := 0; i < 10; i++ {
		f, _ := os.Open("file")
		defer f.Close()
	}
}
`
	rules := analyzer.AllRules()
	result := parseAndWalk(t, "main.go", source, rules, false)

	var found bool
	for _, f := range result.Findings {
		if f.RuleID == "defer_in_loop" {
			found = true
			assert.Equal(t, 6, f.Line)
		}
	}
	assert.True(t, found, "should find defer_in_loop")
}

func TestWalker_IgnoredError(t *testing.T) {
	source := `package main

func handler() {
	_ = doSomething()
}

func doSomething() error { return nil }
`
	rules := analyzer.AllRules()
	result := parseAndWalk(t, "main.go", source, rules, false)

	var found bool
	for _, f := range result.Findings {
		if f.RuleID == "ignored_error" {
			found = true
		}
	}
	assert.True(t, found, "should find ignored_error")
}

func TestWalker_PanicInLib(t *testing.T) {
	source := `package mylib

func Initialize() {
	panic("bad state")
}
`
	rules := analyzer.AllRules()
	result := parseAndWalk(t, "mylib.go", source, rules, false)

	var found bool
	for _, f := range result.Findings {
		if f.RuleID == "panic_in_lib" {
			found = true
		}
	}
	assert.True(t, found, "should find panic_in_lib in library code")
}

func TestWalker_PanicInMain_Skipped(t *testing.T) {
	source := `package main

func main() {
	panic("expected")
}
`
	rules := analyzer.AllRules()
	result := parseAndWalk(t, "main.go", source, rules, true)

	for _, f := range result.Findings {
		assert.NotEqual(t, "panic_in_lib", f.RuleID, "panic_in_lib should be skipped in main")
	}
}

func TestWalker_CleanCode(t *testing.T) {
	source := `package handler

// Add returns the sum of two integers.
func Add(a, b int) int {
	return a + b
}
`
	rules := analyzer.AllRules()
	result := parseAndWalk(t, "handler.go", source, rules, false)

	// No structural findings expected for clean code
	structFindings := 0
	for _, f := range result.Findings {
		for _, r := range rules {
			if r.ID == f.RuleID && (r.Kind == analyzer.RuleStructural || r.Kind == analyzer.RuleComposite) {
				structFindings++
			}
		}
	}
	assert.Equal(t, 0, structFindings, "clean code should have no structural findings")
}

func TestWalker_SymbolSpans(t *testing.T) {
	source := `package main

func handler() {
	fmt.Println("hi")
}

func helper() int {
	return 42
}
`
	rules := analyzer.AllRules()
	result := parseAndWalk(t, "main.go", source, rules, false)

	assert.GreaterOrEqual(t, len(result.Symbols), 2)
	names := make([]string, len(result.Symbols))
	for i, s := range result.Symbols {
		names[i] = s.Name
	}
	assert.Contains(t, names, "handler")
	assert.Contains(t, names, "helper")
}

func TestWalker_SQLStringConcat(t *testing.T) {
	// ADR-correct: sql_string_concat matches call to query/execute/exec
	// with a string_concat argument containing SQL keywords
	source := `package db

import "database/sql"

func queryUser(db *sql.DB, name string) {
	db.Query("SELECT * FROM users WHERE name = " + name)
}
`
	rules := analyzer.AllRules()
	result := parseAndWalk(t, "db.go", source, rules, false)

	var found bool
	for _, f := range result.Findings {
		if f.RuleID == "sql_string_concat" {
			found = true
		}
	}
	assert.True(t, found, "should find sql_string_concat")
}

func TestParseToTree_Basic(t *testing.T) {
	p := NewParser()
	tree, lang, err := p.ParseToTree("test.go", []byte("package main\nfunc main() {}\n"))
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()
	assert.Equal(t, "go", lang)
	assert.NotNil(t, tree.RootNode())
}

func TestParseToTree_UnknownLanguage(t *testing.T) {
	p := NewParser()
	tree, lang, err := p.ParseToTree("test.xyz", []byte("content"))
	assert.NoError(t, err)
	assert.Nil(t, tree)
	assert.Empty(t, lang)
}
