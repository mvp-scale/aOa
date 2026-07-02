//go:build !lean

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/atlas"
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/domain/enricher"
	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/domain/learner"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newWatcherTestApp creates a minimal App with a real parser and temp dir for watcher tests.
func newWatcherTestApp(t *testing.T, root string) *App {
	t.Helper()

	enr, err := enricher.NewFromFS(atlas.FS, "v1")
	require.NoError(t, err)

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	domains := make(map[string]index.Domain, len(enr.DomainDefs()))
	for _, d := range enr.DomainDefs() {
		domains[d.Domain] = index.Domain{Terms: d.Terms}
	}

	engine := index.NewSearchEngine(idx, domains, root)
	parser := treesitter.NewParser()

	return &App{
		ProjectRoot: root,
		ProjectID:   "test",
		Paths:       NewPaths(root),
		Enricher:    enr,
		Engine:      engine,
		Learner:     learner.New(),
		Parser:      parser,
		Index:       idx,
		toolMetrics: ToolMetrics{
			FileReads:    make(map[string]int),
			BashCommands: make(map[string]int),
			GrepPatterns: make(map[string]int),
		},
		turnBuffer: make(map[string]*turnBuilder),
	}
}

func TestOnFileChanged_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	a := newWatcherTestApp(t, tmpDir)

	// Write a Go file with a function
	goFile := filepath.Join(tmpDir, "hello.go")
	err := os.WriteFile(goFile, []byte("package main\n\nfunc HelloWorld() {\n\treturn\n}\n"), 0644)
	require.NoError(t, err)

	// Trigger onFileChanged
	a.onFileChanged(goFile)

	// Verify the symbol is now in the index
	result := a.Engine.Search("helloworld", ports.SearchOptions{})
	assert.GreaterOrEqual(t, len(result.Hits), 1, "should find HelloWorld symbol")

	// Verify file is in the Files map
	assert.Equal(t, 1, len(a.Index.Files))
}

func TestOnFileChanged_ModifyFile(t *testing.T) {
	tmpDir := t.TempDir()
	a := newWatcherTestApp(t, tmpDir)

	goFile := filepath.Join(tmpDir, "funcs.go")

	// Write initial file
	err := os.WriteFile(goFile, []byte("package main\n\nfunc OldFunc() {\n\treturn\n}\n"), 0644)
	require.NoError(t, err)
	a.onFileChanged(goFile)

	// Verify initial
	result := a.Engine.Search("oldfunc", ports.SearchOptions{})
	assert.GreaterOrEqual(t, len(result.Hits), 1, "should find OldFunc")

	// Modify: rename function
	err = os.WriteFile(goFile, []byte("package main\n\nfunc NewFunc() {\n\treturn\n}\n"), 0644)
	require.NoError(t, err)
	a.onFileChanged(goFile)

	// Old name gone — trigram fallback may return approximate matches,
	// but the exact symbol "OldFunc" must not appear.
	result = a.Engine.Search("oldfunc", ports.SearchOptions{})
	for _, h := range result.Hits {
		if h.Kind == "symbol" {
			assert.NotContains(t, h.Symbol, "OldFunc", "OldFunc should be removed from index")
		}
	}

	// New name present
	result = a.Engine.Search("newfunc", ports.SearchOptions{})
	assert.GreaterOrEqual(t, len(result.Hits), 1, "should find NewFunc")
}

func TestOnFileChanged_DeleteFile(t *testing.T) {
	tmpDir := t.TempDir()
	a := newWatcherTestApp(t, tmpDir)

	goFile := filepath.Join(tmpDir, "todelete.go")
	err := os.WriteFile(goFile, []byte("package main\n\nfunc DeleteMe() {\n\treturn\n}\n"), 0644)
	require.NoError(t, err)
	a.onFileChanged(goFile)

	// Confirm it's there
	assert.Equal(t, 1, len(a.Index.Files))

	// Delete the file
	require.NoError(t, os.Remove(goFile))
	a.onFileChanged(goFile)

	// Index should be empty
	assert.Equal(t, 0, len(a.Index.Files))
	assert.Equal(t, 0, len(a.Index.Metadata))
	assert.Equal(t, 0, len(a.Index.Tokens))
}

func TestOnFileChanged_UnsupportedExt(t *testing.T) {
	tmpDir := t.TempDir()
	a := newWatcherTestApp(t, tmpDir)

	// Write a .txt file (unsupported)
	txtFile := filepath.Join(tmpDir, "readme.txt")
	err := os.WriteFile(txtFile, []byte("Hello world"), 0644)
	require.NoError(t, err)
	a.onFileChanged(txtFile)

	// Index should remain empty
	assert.Equal(t, 0, len(a.Index.Files))
	assert.Equal(t, 0, len(a.Index.Metadata))
}

// TestOnFileChanged_BumpsRevision verifies that L19.11 (E3 freshness wiring):
// every mutation path through onFileChanged increments the global revision counter
// so ETag-based caching is invalidated on the same tick the index updates.
func TestOnFileChanged_BumpsRevision(t *testing.T) {
	t.Run("symbol path bumps revision", func(t *testing.T) {
		tmpDir := t.TempDir()
		a := newWatcherTestApp(t, tmpDir)

		goFile := filepath.Join(tmpDir, "foo.go")
		require.NoError(t, os.WriteFile(goFile,
			[]byte("package main\n\nfunc Foo() {}\n"), 0644))

		before := a.Revision()
		a.onFileChanged(goFile)
		assert.Greater(t, a.Revision(), before, "revision must increase after symbol-producing file change")
	})

	t.Run("zero-symbol file bumps revision", func(t *testing.T) {
		tmpDir := t.TempDir()
		a := newWatcherTestApp(t, tmpDir)

		// A Go file with only a package declaration — parser yields 0 symbols,
		// so the tokenise-only path runs. Revision must still bump.
		goFile := filepath.Join(tmpDir, "empty.go")
		require.NoError(t, os.WriteFile(goFile,
			[]byte("package main\n"), 0644))

		before := a.Revision()
		a.onFileChanged(goFile)
		assert.Greater(t, a.Revision(), before, "revision must increase even when file yields zero symbols")
	})

	t.Run("delete path bumps revision", func(t *testing.T) {
		tmpDir := t.TempDir()
		a := newWatcherTestApp(t, tmpDir)

		// First add a file so there is an existing index entry to remove.
		goFile := filepath.Join(tmpDir, "todelete2.go")
		require.NoError(t, os.WriteFile(goFile,
			[]byte("package main\n\nfunc Bar() {}\n"), 0644))
		a.onFileChanged(goFile)

		require.NoError(t, os.Remove(goFile))

		before := a.Revision()
		a.onFileChanged(goFile) // file is gone — delete path
		assert.Greater(t, a.Revision(), before, "revision must increase after file deletion")
	})
}
