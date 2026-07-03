//go:build !lean

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		ProjectRoot:         root,
		ProjectID:           "test",
		Paths:               NewPaths(root),
		Enricher:            enr,
		Engine:              engine,
		Learner:             learner.New(),
		Parser:              parser,
		Index:               idx,
		burnRate:            NewBurnRateTracker(5 * time.Minute),
		burnRateCounterfact: NewBurnRateTracker(5 * time.Minute),
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

// capturingEdgeStore records SaveEdgesBatch and SaveUnresolved calls for T41.
// Sequential access only — no mutex needed in the single-goroutine test path.
type capturingEdgeStore struct {
	noopStore
	batches     []map[uint32][]ports.ImportEdge
	unresolvedN int
}

func (s *capturingEdgeStore) SaveEdgesBatch(_ string, batch map[uint32][]ports.ImportEdge) error {
	s.batches = append(s.batches, batch)
	return nil
}

func (s *capturingEdgeStore) SaveUnresolved(_ string, entries []ports.ImportEdge) error {
	s.unresolvedN += len(entries)
	return nil
}

// TestT41_WatcherPersistedEdgesResolved is the T41 invariant (PC1): edges flushed
// through the watcher path must carry resolved ImportPaths — the same canonical
// form that WarmCaches/Reindex produce — so the edges bucket is never a mixed
// keyspace (raw specs vs. resolved) when F2's LoadAllEdges reads it.
//
// Before the PC1 fix, doFlushEdgeBatch called SaveEdgesBatch with raw ImportPath
// specs ("fmt", "net/http") because facts.Resolve ran only on the bulk paths
// (app.go WarmCaches/Reindex). This test asserts that after the fix every
// persisted ImportPath is in resolved canonical form ("ext:std/fmt" etc.).
func TestT41_WatcherPersistedEdgesResolved(t *testing.T) {
	tmpDir := t.TempDir()

	// Go file with two known stdlib imports — no go.mod needed for stdlib
	// resolution (first path segment has no dot → "ext:std/<spec>" rule).
	goFile := filepath.Join(tmpDir, "main.go")
	require.NoError(t, os.WriteFile(goFile,
		[]byte("package main\n\nimport (\n\t\"fmt\"\n\t\"net/http\"\n)\n\nfunc main() { fmt.Println(http.StatusOK) }\n"),
		0644,
	))

	cap := &capturingEdgeStore{}
	a := newWatcherTestApp(t, tmpDir)
	a.ArchEnabled = true
	a.Store = cap
	a.stopCh = make(chan struct{})

	// Trigger the watcher path — queues raw edges into edgePendingBatch.
	a.onFileChanged(goFile)

	// Flush directly, bypassing the 200ms debounce timer (T18 convention).
	a.doFlushEdgeBatch()

	// Exactly one batch write must have occurred.
	require.Len(t, cap.batches, 1, "expected exactly one SaveEdgesBatch call after flush")
	batch := cap.batches[0]

	// Collect all persisted edges across all file IDs.
	var all []ports.ImportEdge
	for _, edges := range batch {
		all = append(all, edges...)
	}
	require.NotEmpty(t, all, "watcher flush must have persisted at least one edge")

	// T41 core assertion: every ImportPath must be in resolved canonical form.
	// Raw stdlib specs ("fmt", "net/http") must not reach the store — they must
	// become "ext:std/…" after §2.4 resolution inside doFlushEdgeBatch.
	for _, e := range all {
		assert.Truef(t, strings.HasPrefix(e.ImportPath, "ext:"),
			"T41: ImportPath %q must be resolved (ext: prefix); raw spec must not reach the store",
			e.ImportPath)
	}

	// Spot-check the two known imports resolve to their canonical form.
	foundFmt := false
	foundHTTP := false
	for _, e := range all {
		if e.ImportPath == "ext:std/fmt" {
			foundFmt = true
		}
		if e.ImportPath == "ext:std/net/http" {
			foundHTTP = true
		}
	}
	assert.True(t, foundFmt, "T41: 'fmt' must resolve to 'ext:std/fmt'")
	assert.True(t, foundHTTP, "T41: 'net/http' must resolve to 'ext:std/net/http'")
}

// TestT33_WatcherFromFileRelative is the T33 invariant: edges queued by the
// watcher must carry project-relative FromFile paths, never absolute paths.
//
// Root cause this guards against (checkpoint-F1 finding 7): the watcher called
// fp.ParseFileToMetaAndFacts(absPath, …) and passed the returned edges verbatim
// to markEdgeBatchDirty — so edges stored in bbolt had FromFile=absPath. The
// indexer.go path (indexer.go:190) correctly relativised with `e.FromFile = relPath`
// but the watcher lacked the equivalent step.
func TestT33_WatcherFromFileRelative(t *testing.T) {
	tmpDir := t.TempDir()

	// Build a subdirectory so the relative path is non-trivial.
	subDir := filepath.Join(tmpDir, "internal", "app")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// Go file with imports — the parser will produce ImportEdges whose FromFile
	// must be relative ("internal/app/server.go"), not absolute.
	goFile := filepath.Join(subDir, "server.go")
	require.NoError(t, os.WriteFile(goFile,
		[]byte("package app\n\nimport (\n\t\"fmt\"\n\t\"net/http\"\n)\n\nfunc Serve() { fmt.Println(http.StatusOK) }\n"),
		0644,
	))

	a := newWatcherTestApp(t, tmpDir)
	a.ArchEnabled = true    // enable arch extraction path
	a.Store = &noopStore{}  // markEdgeBatchDirty guards on Store != nil; must be set

	a.onFileChanged(goFile)

	// Inspect the pending edge batch directly (we're in the same package).
	a.mu.Lock()
	batch := a.edgePendingBatch
	a.mu.Unlock()

	require.NotEmpty(t, batch, "watcher must have queued edges for a Go file with imports")

	for fileID, edges := range batch {
		_ = fileID
		for _, e := range edges {
			assert.False(t, filepath.IsAbs(e.FromFile),
				"T33: edge.FromFile must be relative, got absolute path %q", e.FromFile)
			assert.Equal(t, "internal/app/server.go", e.FromFile,
				"T33: edge.FromFile must be project-relative")
		}
	}
}
