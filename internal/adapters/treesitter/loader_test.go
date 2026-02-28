//go:build !lean

package treesitter

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSymbolName(t *testing.T) {
	tests := []struct {
		lang     string
		expected string
	}{
		{"python", "tree_sitter_python"},
		{"javascript", "tree_sitter_javascript"},
		{"typescript", "tree_sitter_typescript"},
		{"tsx", "tree_sitter_tsx"},
		{"go", "tree_sitter_go"},
		{"rust", "tree_sitter_rust"},
		{"c", "tree_sitter_c"},
		{"cpp", "tree_sitter_cpp"},
		{"c_sharp", "tree_sitter_c_sharp"}, // default derivation now works
		{"objc", "tree_sitter_objc"},      // override
		{"bash", "tree_sitter_bash"},
		{"hcl", "tree_sitter_hcl"},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			assert.Equal(t, tt.expected, CSymbolName(tt.lang))
		})
	}
}

func TestSOBaseName(t *testing.T) {
	tests := []struct {
		lang     string
		expected string
	}{
		{"python", "python"},
		{"javascript", "javascript"},
		{"tsx", "tsx"}, // tsx has its own .so (separate grammar in go-sitter-forest)
		{"typescript", "typescript"},
		{"go", "go"},
		{"c_sharp", "c_sharp"},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			assert.Equal(t, tt.expected, SOBaseName(tt.lang))
		})
	}
}

func TestLibExtension(t *testing.T) {
	ext := LibExtension()
	switch runtime.GOOS {
	case "darwin":
		assert.Equal(t, ".dylib", ext)
	default:
		assert.Equal(t, ".so", ext)
	}
}

func TestDefaultGrammarPaths(t *testing.T) {
	paths := DefaultGrammarPaths("/project/root")
	require.GreaterOrEqual(t, len(paths), 1)
	assert.Equal(t, "/project/root/.aoa/grammars", paths[0])

	// Global path should be second
	if len(paths) > 1 {
		home, _ := os.UserHomeDir()
		assert.Equal(t, filepath.Join(home, ".aoa", "grammars"), paths[1])
	}
}

func TestDefaultGrammarPaths_EmptyRoot(t *testing.T) {
	paths := DefaultGrammarPaths("")
	// Should still have global path
	if home, err := os.UserHomeDir(); err == nil {
		require.Equal(t, 1, len(paths))
		assert.Equal(t, filepath.Join(home, ".aoa", "grammars"), paths[0])
	}
}

func TestDynamicLoader_LoadGrammar_NotFound(t *testing.T) {
	dl := NewDynamicLoader([]string{"/nonexistent/path"})
	_, err := dl.LoadGrammar("python")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in search paths")
}

func TestDynamicLoader_GrammarPath_NotFound(t *testing.T) {
	dl := NewDynamicLoader([]string{"/nonexistent/path"})
	assert.Equal(t, "", dl.GrammarPath("python"))
}

func TestDynamicLoader_InstalledGrammars_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	dl := NewDynamicLoader([]string{dir})
	grammars := dl.InstalledGrammars()
	assert.Empty(t, grammars)
}

func TestDynamicLoader_InstalledGrammars_FindsSO(t *testing.T) {
	dir := t.TempDir()
	ext := LibExtension()

	// Create fake grammar .so files
	for _, lang := range []string{"python", "javascript", "go"} {
		f, err := os.Create(filepath.Join(dir, lang+ext))
		require.NoError(t, err)
		f.Close()
	}

	dl := NewDynamicLoader([]string{dir})
	grammars := dl.InstalledGrammars()
	assert.Equal(t, 3, len(grammars))

	// Check all 3 found (order may vary)
	grammarSet := make(map[string]bool)
	for _, g := range grammars {
		grammarSet[g] = true
	}
	assert.True(t, grammarSet["python"])
	assert.True(t, grammarSet["javascript"])
	assert.True(t, grammarSet["go"])
}

func TestDynamicLoader_GrammarPath_FindsSO(t *testing.T) {
	dir := t.TempDir()
	ext := LibExtension()

	soPath := filepath.Join(dir, "python"+ext)
	f, err := os.Create(soPath)
	require.NoError(t, err)
	f.Close()

	dl := NewDynamicLoader([]string{dir})
	assert.Equal(t, soPath, dl.GrammarPath("python"))
	assert.Equal(t, "", dl.GrammarPath("javascript"))
}

func TestDynamicLoader_SearchPathPriority(t *testing.T) {
	// Create two dirs with the same grammar — first path should win
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	ext := LibExtension()

	path1 := filepath.Join(dir1, "python"+ext)
	path2 := filepath.Join(dir2, "python"+ext)
	for _, p := range []string{path1, path2} {
		f, err := os.Create(p)
		require.NoError(t, err)
		f.Close()
	}

	dl := NewDynamicLoader([]string{dir1, dir2})
	assert.Equal(t, path1, dl.GrammarPath("python"))
}

func TestDynamicLoader_TSXSeparateFromTypescript(t *testing.T) {
	dir := t.TempDir()
	ext := LibExtension()

	// TSX and TypeScript each have their own .so
	tsxPath := filepath.Join(dir, "tsx"+ext)
	tsPath := filepath.Join(dir, "typescript"+ext)
	for _, p := range []string{tsxPath, tsPath} {
		f, err := os.Create(p)
		require.NoError(t, err)
		f.Close()
	}

	dl := NewDynamicLoader([]string{dir})
	assert.Equal(t, tsxPath, dl.GrammarPath("tsx"))
	assert.Equal(t, tsPath, dl.GrammarPath("typescript"))

	// Without tsx.so, tsx is not found (no fallback to typescript)
	dir2 := t.TempDir()
	tsOnlyPath := filepath.Join(dir2, "typescript"+ext)
	f, err := os.Create(tsOnlyPath)
	require.NoError(t, err)
	f.Close()

	dl2 := NewDynamicLoader([]string{dir2})
	assert.Equal(t, "", dl2.GrammarPath("tsx"))
	assert.Equal(t, tsOnlyPath, dl2.GrammarPath("typescript"))
}

func TestDynamicLoader_Close(t *testing.T) {
	dl := NewDynamicLoader([]string{"/tmp"})
	dl.Close()
	assert.Empty(t, dl.loaded)
	assert.Nil(t, dl.handles)
}

func TestDynamicLoader_SearchPaths(t *testing.T) {
	paths := []string{"/a", "/b", "/c"}
	dl := NewDynamicLoader(paths)
	assert.Equal(t, paths, dl.SearchPaths())
}

func TestParser_SetGrammarPaths(t *testing.T) {
	p := NewParser()
	assert.Nil(t, p.Loader())

	p.SetGrammarPaths([]string{"/tmp/grammars"})
	assert.NotNil(t, p.Loader())
	assert.Equal(t, []string{"/tmp/grammars"}, p.Loader().SearchPaths())
}

func TestParser_HasLanguage(t *testing.T) {
	p := NewParser()

	// Compiled-in languages
	assert.True(t, p.HasLanguage("python"))
	assert.True(t, p.HasLanguage("go"))
	assert.True(t, p.HasLanguage("javascript"))

	// Not available
	assert.False(t, p.HasLanguage("nonexistent"))
	assert.False(t, p.HasLanguage("klingon")) // not a real parser
}

func TestParser_HasLanguage_WithLoader(t *testing.T) {
	dir := t.TempDir()
	ext := LibExtension()

	// Create fake brainfuck.so (not a compiled-in grammar)
	f, err := os.Create(filepath.Join(dir, "brainfuck"+ext))
	require.NoError(t, err)
	f.Close()

	p := NewParser()
	p.SetGrammarPaths([]string{dir})

	// brainfuck now discoverable via loader
	assert.True(t, p.HasLanguage("brainfuck"))
	// still can't find nonexistent
	assert.False(t, p.HasLanguage("nonexistent"))
}

func TestDynamicLoader_InstalledGrammars_Dedup(t *testing.T) {
	// Two search paths with same grammar — should deduplicate
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	ext := LibExtension()

	for _, dir := range []string{dir1, dir2} {
		f, err := os.Create(filepath.Join(dir, "python"+ext))
		require.NoError(t, err)
		f.Close()
	}

	dl := NewDynamicLoader([]string{dir1, dir2})
	grammars := dl.InstalledGrammars()
	assert.Equal(t, 1, len(grammars))
	assert.Equal(t, "python", grammars[0])
}
