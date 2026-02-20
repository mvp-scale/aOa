package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
)

// =============================================================================
// Performance Benchmarks — G1 targets vs Python baseline
// These benchmarks validate the core value proposition of the Go port.
// Each has a specific target derived from the GO-BOARD success metrics.
// =============================================================================

func BenchmarkSearch_E2E(b *testing.B) {
	// Target: <500µs per query (Python: 8-15ms → 16-30x faster)
	// Setup: Index 500 files with content cache, benchmark single-term search.
	dir := b.TempDir()

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	// Generate 500 synthetic files with varied content.
	// Each file has unique function names. "dashboard" appears in ~10 files
	// (files 1-10) to simulate a realistic selective search.
	bodyVariants := []string{
		"\tconn := OpenDatabase(ctx)\n\trows := QueryTable(conn)\n",
		"\tcfg := LoadConfig(path)\n\tval := ParseYAML(cfg)\n",
		"\tclient := NewHTTPClient()\n\tresp := FetchEndpoint(client)\n",
		"\tlogger := GetLogger(ctx)\n\tlogger.Info(\"starting\")\n",
		"\tcache := GetRedisPool()\n\tval := CacheLookup(cache, key)\n",
	}
	for i := uint32(1); i <= 500; i++ {
		name := fmt.Sprintf("pkg%d/file%d.go", i%50, i)
		absPath := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(absPath), 0o755)

		body := bodyVariants[i%uint32(len(bodyVariants))]
		extra := ""
		if i <= 10 {
			extra = "\tdashboardWidget := RenderDashboard(ctx)\n"
		}

		content := fmt.Sprintf(
			"package pkg%d\n\nimport \"fmt\"\n\n"+
				"// Handler%d processes requests for module %d\n"+
				"func Handler%d(ctx Context, req Request) Response {\n"+
				"%s%s"+
				"\tfmt.Println(\"done\")\n"+
				"\treturn Response{Status: 200}\n"+
				"}\n\n"+
				"func Helper%d() string {\n"+
				"\treturn \"helper\"\n"+
				"}\n",
			i%50, i, i, i, body, extra, i)
		_ = os.WriteFile(absPath, []byte(content), 0o644)

		info, _ := os.Stat(absPath)
		idx.Files[i] = &ports.FileMeta{
			Path:         name,
			Language:     "go",
			Size:         info.Size(),
			LastModified: int64(i),
		}

		// Add symbol refs
		ref := ports.TokenRef{FileID: i, Line: 6}
		sym := &ports.SymbolMeta{
			Name:      fmt.Sprintf("Handler%d", i),
			Kind:      "function",
			Signature: fmt.Sprintf("Handler%d(ctx Context, req Request) Response", i),
			StartLine: 6,
			EndLine:   12,
		}
		tokens := index.Tokenize(sym.Name)
		for _, tok := range tokens {
			idx.Tokens[tok] = append(idx.Tokens[tok], ref)
		}
		idx.Metadata[ref] = sym

		helperRef := ports.TokenRef{FileID: i, Line: 14}
		helperSym := &ports.SymbolMeta{
			Name:      fmt.Sprintf("Helper%d", i),
			Kind:      "function",
			Signature: fmt.Sprintf("Helper%d() string", i),
			StartLine: 14,
			EndLine:   16,
		}
		helperTokens := index.Tokenize(helperSym.Name)
		for _, tok := range helperTokens {
			idx.Tokens[tok] = append(idx.Tokens[tok], helperRef)
		}
		idx.Metadata[helperRef] = helperSym
	}

	engine := index.NewSearchEngine(idx, nil, dir)
	cache := index.NewFileCache(0)
	engine.SetCache(cache)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		engine.Search("dashboard", ports.SearchOptions{MaxCount: 20})
	}
}

func BenchmarkObserve_E2E(b *testing.B) {
	// Target: <10ns per observe() call (Python: 3-5ms → 300,000-500,000x faster)
	// Measures channel send latency only (not processing).
	// This is the hot path — every tool call triggers observe().
	b.Skip("Observe not implemented — L-02")
}

func BenchmarkAutotune_E2E(b *testing.B) {
	// Target: <5ms per autotune (Python: 250-600ms → 50-120x faster)
	// State: 24 domains, 150 terms, 500 keywords (realistic production state).
	// All 16 steps: dedup, decay, prune, rank, promote/demote.
	b.Skip("Autotune not implemented — L-03")
}

func BenchmarkIndexFile_E2E(b *testing.B) {
	// Target: <20ms per file (Python: 50-200ms → 2.5-10x faster)
	// Parse source file with tree-sitter, extract symbols, add to index.
	b.Skip("Parser not implemented — S-02")
}

func BenchmarkStartup_E2E(b *testing.B) {
	// Target: <200ms cold start (Python: 3-8s → 15-40x faster)
	// Load index + learner state from bbolt, ready to serve.
	// 500-file project, realistic state.
	b.Skip("Full system not implemented — C-04")
}

func BenchmarkMemory_500Files(b *testing.B) {
	// Target: <50MB resident (Python: ~390MB → 8x reduction)
	// Index 500 files, measure heap allocation.
	// Use b.ReportAllocs() when implemented.
	b.Skip("Full system not implemented — C-04")
}
