//go:build !lean

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeFactSink is a minimal ports.FactSink test double that just records
// every emitted Fact in memory (FDN-2, board #28).
type fakeFactSink struct {
	facts []ports.Fact
}

func (s *fakeFactSink) Emit(f ports.Fact) { s.facts = append(s.facts, f) }
func (s *fakeFactSink) Flush() error      { return nil }

func TestBuildIndex_Counts(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	// Create two Go files with functions
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "a.go"),
		[]byte("package main\n\nfunc Alpha() {}\nfunc Beta() {}\n"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "b.go"),
		[]byte("package main\n\nfunc Gamma() {}\n"),
		0644,
	))

	idx, result, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	assert.Equal(t, 2, result.FileCount, "should index 2 files")
	assert.Equal(t, 3, result.SymbolCount, "should find 3 symbols")
	assert.Greater(t, result.TokenCount, 0, "should have tokens")
	assert.Equal(t, 2, len(idx.Files))
	assert.Equal(t, 3, len(idx.Metadata))
}

func TestBuildIndex_SkipsLargeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	// Create a >1MB Go file
	bigContent := make([]byte, 1<<20+100)
	copy(bigContent, []byte("package main\n\nfunc BigFunc() {}\n"))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "big.go"), bigContent, 0644))

	// Create a normal file
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "small.go"),
		[]byte("package main\n\nfunc SmallFunc() {}\n"),
		0644,
	))

	_, result, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	// Only the small file should be indexed
	assert.Equal(t, 1, result.FileCount, "big file should be skipped")
}

func TestBuildIndex_SkipsIgnoredDirs(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	// Create a file in node_modules
	nmDir := filepath.Join(tmpDir, "node_modules")
	require.NoError(t, os.MkdirAll(nmDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(nmDir, "dep.go"),
		[]byte("package dep\n\nfunc DepFunc() {}\n"),
		0644,
	))

	// Create a normal file at root
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "main.go"),
		[]byte("package main\n\nfunc Main() {}\n"),
		0644,
	))

	_, result, err := BuildIndex(tmpDir, parser)
	require.NoError(t, err)

	assert.Equal(t, 1, result.FileCount, "node_modules should be skipped")
	assert.Equal(t, 1, result.SymbolCount)
}

// TestBuildIndexWithFacts_VarOnlyFileHasEdges is a regression test for the bug
// where `if parseErr == nil && len(metas) > 0` gated edge collection on non-empty
// metas. Go files with only var/const declarations produce 0 metas (extractGo
// handles only func/method/type), but they may still have import edges.
// Correct behaviour: edges are emitted regardless of metas length.
func TestBuildIndexWithFacts_VarOnlyFileHasEdges(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	// var-only Go file — no functions or types, so metas will be empty.
	// Two imports → expects 2 import edges.
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "consts.go"),
		[]byte("package main\n\nimport (\n\t\"os\"\n\t\"fmt\"\n)\n\nvar Debug = os.Getenv(\"DEBUG\") != \"\"\nvar Prefix = fmt.Sprintf(\"[app]\")\n"),
		0644,
	))

	_, result, edges, err := BuildIndexWithFacts(tmpDir, parser, true)
	require.NoError(t, err)

	assert.Equal(t, 1, result.FileCount)
	assert.Equal(t, 0, result.SymbolCount, "var-only file produces no symbol metas")
	assert.Equal(t, 2, result.EdgeCount, "import edges must be collected even when metas is empty")
	require.Len(t, edges, 2, "edges slice must match EdgeCount")

	paths := make([]string, len(edges))
	for i, e := range edges {
		paths[i] = e.ImportPath
	}
	assert.Contains(t, paths, "os")
	assert.Contains(t, paths, "fmt")
}

// TestBuildIndexWithFactsAndSink_DualRun verifies the FDN-2 dual-run
// contract: every ports.ImportEdge returned to the caller is ALSO emitted as
// a raw ports.Fact through the sink, with Object empty (unresolved) and the
// literal specifier preserved in Attrs["spec"] (§1.2 "Rules").
func TestBuildIndexWithFactsAndSink_DualRun(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "main.go"),
		[]byte("package main\n\nimport (\n\t\"os\"\n\t\"fmt\"\n)\n\nfunc Main() { _ = os.Args; _ = fmt.Sprintf }\n"),
		0644,
	))

	sink := &fakeFactSink{}
	idx, result, edges, err := BuildIndexWithFactsAndSink(tmpDir, parser, true, sink)
	require.NoError(t, err)
	require.NotNil(t, idx)

	// Legacy ImportEdge path is unaffected by the sink's presence.
	assert.Equal(t, 2, result.EdgeCount)
	require.Len(t, edges, 2)

	// Dual-run: one Fact per edge, same count.
	require.Len(t, sink.facts, 2)
	specs := make([]string, len(sink.facts))
	for i, f := range sink.facts {
		specs[i] = f.Attrs["spec"]
		assert.Equal(t, ports.FactDep, f.Kind)
		assert.Equal(t, "go:", f.Subject, "repo-root Go file's package dir is empty")
		assert.Empty(t, f.Object, "raw parse-time fact: Object unresolved")
		assert.Equal(t, ports.ProvDerived, f.Prov)
		assert.Equal(t, "main.go", f.Source.File)
		assert.NotZero(t, f.Source.Line)
	}
	assert.Contains(t, specs, "os")
	assert.Contains(t, specs, "fmt")
}

// TestBuildIndexWithFactsAndSink_NilSinkMatchesLegacy verifies that passing a
// nil sink produces byte-identical IndexResult/edges to BuildIndexWithFacts —
// existing callers are not forced into the FactSink dependency (D25: FDN-4
// switches consumers, not this task).
func TestBuildIndexWithFactsAndSink_NilSinkMatchesLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "main.go"),
		[]byte("package main\n\nimport \"os\"\n\nfunc Main() { _ = os.Args }\n"),
		0644,
	))

	_, wantResult, wantEdges, err := BuildIndexWithFacts(tmpDir, parser, true)
	require.NoError(t, err)

	_, gotResult, gotEdges, err := BuildIndexWithFactsAndSink(tmpDir, parser, true, nil)
	require.NoError(t, err)

	assert.Equal(t, *wantResult, *gotResult)
	assert.Equal(t, wantEdges, gotEdges)
}

// TestFactSubjectForFile verifies the D7 (§1.3) canonical-ID unit derivation
// for each P1 language family.
func TestFactSubjectForFile(t *testing.T) {
	assert.Equal(t, "go:internal/app", factSubjectForFile("internal/app/indexer.go", "go"))
	assert.Equal(t, "go:", factSubjectForFile("main.go", "go"))
	assert.Equal(t, "py:graphify/extract", factSubjectForFile("graphify/extract.py", "py"))
	assert.Equal(t, "ts:src/components/Button", factSubjectForFile("src/components/Button.tsx", "tsx"))
	assert.Equal(t, "file:weird/thing.zig", factSubjectForFile("weird/thing.zig", "zig"))
}

// TestBuildIndex_NilParser_TokenizationOnly verifies that BuildIndex works
// without a parser (tokenization-only mode). Files are discovered via
// defaultCodeExtensions, and content is tokenized for file-level search.
func TestBuildIndex_NilParser_TokenizationOnly(t *testing.T) {
	tmpDir := t.TempDir()

	// Create Go files — should be discovered by defaultCodeExtensions
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "main.go"),
		[]byte("package main\n\nfunc SearchEngine() {\n\treturn\n}\n"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "util.py"),
		[]byte("def helper_function():\n    pass\n"),
		0644,
	))

	// nil parser = tokenization-only mode
	idx, result, err := BuildIndex(tmpDir, nil)
	require.NoError(t, err)

	// Files should be indexed
	assert.Equal(t, 2, result.FileCount, "should discover files via defaultCodeExtensions")

	// No symbols (no parser)
	assert.Equal(t, 0, result.SymbolCount, "no symbols without parser")
	assert.Equal(t, 0, len(idx.Metadata), "no metadata without parser")

	// But tokens should exist from content tokenization
	assert.Greater(t, result.TokenCount, 0, "should have tokens from content")

	// Check specific tokens exist — Tokenize("SearchEngine") → ["search", "engine"]
	assert.Greater(t, len(idx.Tokens["search"]), 0, "should tokenize 'search' from content")
	assert.Greater(t, len(idx.Tokens["engine"]), 0, "should tokenize 'engine' from content")
}

// TestT30_FlagOn_ContentEquivalence is the T30 invariant: BuildIndexWithFacts
// with archEnabled=true must produce identical index content (Tokens, Metadata)
// as archEnabled=false for all file types — only edge emission differs between modes.
//
// Root cause this guards against (checkpoint-F1 finding 6): when parseErr==nil
// but len(metas)==0, the old code used an unconditional `continue` which skipped
// content tokenization for symbol-less parseable files. Flag-on would miss the
// content tokens that flag-off correctly populated.
func TestT30_FlagOn_ContentEquivalence(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	// File with symbols (functions): both modes produce symbol metadata.
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "symbols.go"),
		[]byte("package main\n\nimport \"fmt\"\n\nfunc SearchEngine() { fmt.Println() }\nfunc QueryParser() {}\n"),
		0644,
	))

	// Symbol-less parseable file: var/const only → metas==0; parser succeeds but
	// returns no SymbolMeta. Flag-on must NOT skip its content tokens (T30).
	// "zebrafoobar" is a distinctive token that only appears in this file.
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "consts.go"),
		[]byte("package main\n\nimport \"os\"\n\nvar Debug = os.Getenv(\"DEBUG\") != \"\"\nvar ZebraFooBar = true // zebrafoobar marker token\n"),
		0644,
	))

	// Run flag-OFF (archEnabled=false).
	idxOff, _, _, err := BuildIndexWithFacts(tmpDir, parser, false)
	require.NoError(t, err)

	// Run flag-ON (archEnabled=true).
	idxOn, _, _, err := BuildIndexWithFacts(tmpDir, parser, true)
	require.NoError(t, err)

	// Tokens must be identical: same keys, same reference counts.
	// (TokenRef slices may vary in order; compare lengths per token.)
	assert.Equal(t, len(idxOff.Tokens), len(idxOn.Tokens),
		"T30: flag-on and flag-off must produce the same number of distinct tokens")

	for tok, refsOff := range idxOff.Tokens {
		refsOn := idxOn.Tokens[tok]
		assert.Equal(t, len(refsOff), len(refsOn),
			"T30: token %q has %d refs OFF but %d refs ON — content equivalence violated",
			tok, len(refsOff), len(refsOn))
	}

	// Metadata (symbols) must be identical.
	assert.Equal(t, len(idxOff.Metadata), len(idxOn.Metadata),
		"T30: symbol count must be identical between flag-off and flag-on")

	// Specifically verify the symbol-less file's content tokens exist in flag-ON.
	// "zebrafoobar" is a distinctive content token from consts.go (appears only there).
	// TokenizeContentLine splits "ZebraFooBar" → ["zebra","foo","bar"] and tokenises
	// "zebrafoobar" from the comment. At least one of these must appear.
	// Flag-on must produce the same content tokens as flag-off (T30 invariant).
	//
	// Old bug: the unconditional `continue` after ParseFileToMetaAndFacts skipped
	// content tokenization for symbol-less files, so flag-on produced 0 refs for
	// content tokens that flag-off correctly populated.
	assert.Greater(t, len(idxOn.Tokens["zebrafoobar"]), 0,
		"T30: symbol-less file must have content tokens in flag-on mode (was skipped by old `continue` bug)")
	assert.Greater(t, len(idxOff.Tokens["zebrafoobar"]), 0,
		"T30: symbol-less file must have content tokens in flag-off mode (sanity check)")
}
