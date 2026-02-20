package treesitter

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// TestDynamicLoader_LoadAndParse_EndToEnd compiles the Python grammar to a .so,
// loads it via purego, and verifies it produces the same AST as the compiled-in grammar.
func TestDynamicLoader_LoadAndParse_EndToEnd(t *testing.T) {
	// Find the Python grammar source in the Go module cache
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		goPath = filepath.Join(home, "go")
	}

	pythonSrc := filepath.Join(goPath, "pkg", "mod", "github.com", "tree-sitter",
		"tree-sitter-python@v0.25.0", "src")

	parserC := filepath.Join(pythonSrc, "parser.c")
	scannerC := filepath.Join(pythonSrc, "scanner.c")

	if _, err := os.Stat(parserC); err != nil {
		t.Skipf("Python grammar source not in module cache: %v", err)
	}

	// Check gcc is available
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}

	// Compile to .so
	dir := t.TempDir()
	soPath := filepath.Join(dir, "python.so")

	cmd := exec.Command("gcc", "-shared", "-fPIC",
		"-I"+pythonSrc,
		"-o", soPath,
		parserC, scannerC)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "gcc failed: %s", out)

	// Create a parser with only dynamic loading (no compiled-in grammars)
	p := &Parser{
		languages: make(map[string]*tree_sitter.Language),
		extToLang: make(map[string]string),
	}
	p.registerExtensions()
	p.SetGrammarPaths([]string{dir})

	// Parse Python source using dynamically loaded grammar
	source := []byte(`class Calculator:
    def add(self, a, b):
        return a + b

    def subtract(self, a, b):
        return a - b

def standalone(x):
    return x * 2
`)

	symbols, err := p.ParseFile("calc.py", source)
	require.NoError(t, err)
	require.NotNil(t, symbols, "should parse with dynamically loaded grammar")

	// Verify the symbols match what we'd expect
	require.Equal(t, 4, len(symbols), "class + 2 methods + function")

	assert.Equal(t, "Calculator", symbols[0].Name)
	assert.Equal(t, "class", symbols[0].Kind)

	assert.Equal(t, "add", symbols[1].Name)
	assert.Equal(t, "method", symbols[1].Kind)
	assert.Equal(t, "Calculator", symbols[1].Parent)

	assert.Equal(t, "subtract", symbols[2].Name)
	assert.Equal(t, "method", symbols[2].Kind)
	assert.Equal(t, "Calculator", symbols[2].Parent)

	assert.Equal(t, "standalone", symbols[3].Name)
	assert.Equal(t, "function", symbols[3].Kind)

	// Now compare with compiled-in parser
	builtinParser := NewParser()
	builtinSymbols, err := builtinParser.ParseFile("calc.py", source)
	require.NoError(t, err)

	// Both should produce identical results
	require.Equal(t, len(builtinSymbols), len(symbols), "dynamic and builtin should have same symbol count")
	for i := range symbols {
		assert.Equal(t, builtinSymbols[i].Name, symbols[i].Name, "symbol %d name", i)
		assert.Equal(t, builtinSymbols[i].Kind, symbols[i].Kind, "symbol %d kind", i)
		assert.Equal(t, builtinSymbols[i].Parent, symbols[i].Parent, "symbol %d parent", i)
		assert.Equal(t, builtinSymbols[i].StartLine, symbols[i].StartLine, "symbol %d start", i)
		assert.Equal(t, builtinSymbols[i].EndLine, symbols[i].EndLine, "symbol %d end", i)
		assert.Equal(t, builtinSymbols[i].Signature, symbols[i].Signature, "symbol %d sig", i)
	}

	t.Logf("Dynamic loader produced identical results to compiled-in grammar")
	t.Logf("Grammar .so size: %s", soPath)
}
