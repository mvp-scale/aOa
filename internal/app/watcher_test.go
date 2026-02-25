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

	// Old name gone
	result = a.Engine.Search("oldfunc", ports.SearchOptions{})
	symbolHits := 0
	for _, h := range result.Hits {
		if h.Kind == "symbol" {
			symbolHits++
		}
	}
	assert.Equal(t, 0, symbolHits, "OldFunc should be removed")

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
