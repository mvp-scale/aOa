package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/domain/learner"
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

func BenchmarkSearch_E2E_CaseInsensitive(b *testing.B) {
	// Target: <500µs per query with -i flag (trigram path)
	dir := b.TempDir()

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

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
	}

	engine := index.NewSearchEngine(idx, nil, dir)
	cache := index.NewFileCache(0)
	engine.SetCache(cache)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		engine.Search("dashboard", ports.SearchOptions{
			Mode:     "case_insensitive",
			MaxCount: 20,
		})
	}
}

func BenchmarkObserve_E2E(b *testing.B) {
	// Target: <10µs per observe() call (Python: 3-5ms → 300-500x faster)
	// Measures the full Observe() path: keyword/term/domain/cohit increments.
	// This is the hot path — every tool call triggers observe().
	data, err := os.ReadFile("fixtures/learner/events-01-to-50.json")
	if err != nil {
		b.Fatalf("load events: %v", err)
	}
	var events []learner.ObserveEvent
	if err := json.Unmarshal(data, &events); err != nil {
		b.Fatalf("parse events: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l := learner.New()
		for _, ev := range events {
			l.Observe(ev)
		}
	}
}

func BenchmarkAutotune_E2E(b *testing.B) {
	// Target: <5ms per autotune (Python: 250-600ms → 50-120x faster)
	// State: post-100 intents (8 domains, ~20 terms, ~100 keywords).
	// All 21 steps: dedup, decay, prune, rank, promote/demote.
	data, err := os.ReadFile("fixtures/learner/02-hundred-intents.json")
	if err != nil {
		b.Fatalf("load fixture: %v", err)
	}
	var state ports.LearnerState
	if err := json.Unmarshal(data, &state); err != nil {
		b.Fatalf("parse fixture: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Clone state each iteration so decay doesn't compound
		clone := cloneLearnerState(&state)
		l := learner.NewFromState(clone)
		l.RunMathTune()
	}
}

func BenchmarkIndexFile_E2E(b *testing.B) {
	// Target: <20ms per file (Python: 50-200ms → 2.5-10x faster)
	// Parse a real .go source file with tree-sitter, extract symbols.
	parser := treesitter.NewParser()

	// Use this benchmark file itself as the test subject
	source, err := os.ReadFile("benchmark_test.go")
	if err != nil {
		b.Fatalf("read source: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		parser.ParseFileToMeta("benchmark_test.go", source)
	}
}

func BenchmarkStartup_E2E(b *testing.B) {
	// Target: <200ms cold start (Python: 3-8s → 15-40x faster)
	// Save a 500-file index + learner state to bbolt, then load it back.
	// Measures the cold-start path: open db → deserialize → ready to serve.
	dir := b.TempDir()
	dbPath := filepath.Join(dir, "aoa.db")

	// Build a 500-file index
	idx := build500FileIndex(dir)

	// Build learner state from fixture
	ldata, err := os.ReadFile("fixtures/learner/02-hundred-intents.json")
	if err != nil {
		b.Fatalf("load learner fixture: %v", err)
	}
	var lstate ports.LearnerState
	if err := json.Unmarshal(ldata, &lstate); err != nil {
		b.Fatalf("parse learner fixture: %v", err)
	}

	// Persist to bbolt
	store, err := bbolt.NewStore(dbPath)
	if err != nil {
		b.Fatalf("create store: %v", err)
	}
	if err := store.SaveIndex("bench", idx); err != nil {
		b.Fatalf("save index: %v", err)
	}
	if err := store.SaveLearnerState("bench", &lstate); err != nil {
		b.Fatalf("save learner: %v", err)
	}
	store.Close()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		s, err := bbolt.NewStore(dbPath)
		if err != nil {
			b.Fatalf("open store: %v", err)
		}
		_, err = s.LoadIndex("bench")
		if err != nil {
			b.Fatalf("load index: %v", err)
		}
		_, err = s.LoadLearnerState("bench")
		if err != nil {
			b.Fatalf("load learner: %v", err)
		}
		s.Close()
	}
}

func BenchmarkMemory_500Files(b *testing.B) {
	// Target: <50MB heap for 500-file index
	// Measures heap allocation for index + search engine + file cache.
	dir := b.TempDir()
	idx := build500FileIndex(dir)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		engine := index.NewSearchEngine(idx, nil, dir)
		cache := index.NewFileCache(0)
		engine.SetCache(cache)
		// Perform a search to force any lazy init
		engine.Search("handler", ports.SearchOptions{MaxCount: 20})
	}

	// Report heap size after the last iteration
	b.StopTimer()
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	b.ReportMetric(float64(m.HeapAlloc)/1024/1024, "MB_heap")
}

// =============================================================================
// Helpers
// =============================================================================

// build500FileIndex creates a synthetic 500-file index with 1000 symbols.
// Reuses the same pattern as BenchmarkSearch_E2E but returns the index.
func build500FileIndex(dir string) *ports.Index {
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

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

	return idx
}

// cloneLearnerState deep-copies a LearnerState so benchmarks don't mutate shared state.
func cloneLearnerState(s *ports.LearnerState) *ports.LearnerState {
	clone := &ports.LearnerState{
		PromptCount:      s.PromptCount,
		KeywordHits:      make(map[string]uint32, len(s.KeywordHits)),
		TermHits:         make(map[string]uint32, len(s.TermHits)),
		DomainMeta:       make(map[string]*ports.DomainMeta, len(s.DomainMeta)),
		CohitKwTerm:      make(map[string]uint32, len(s.CohitKwTerm)),
		CohitTermDomain:  make(map[string]uint32, len(s.CohitTermDomain)),
		Bigrams:          make(map[string]uint32, len(s.Bigrams)),
		FileHits:         make(map[string]uint32, len(s.FileHits)),
		KeywordBlocklist: make(map[string]bool, len(s.KeywordBlocklist)),
		GapKeywords:      make(map[string]bool, len(s.GapKeywords)),
	}
	for k, v := range s.KeywordHits {
		clone.KeywordHits[k] = v
	}
	for k, v := range s.TermHits {
		clone.TermHits[k] = v
	}
	for k, v := range s.DomainMeta {
		dm := *v
		clone.DomainMeta[k] = &dm
	}
	for k, v := range s.CohitKwTerm {
		clone.CohitKwTerm[k] = v
	}
	for k, v := range s.CohitTermDomain {
		clone.CohitTermDomain[k] = v
	}
	for k, v := range s.Bigrams {
		clone.Bigrams[k] = v
	}
	for k, v := range s.FileHits {
		clone.FileHits[k] = v
	}
	for k, v := range s.KeywordBlocklist {
		clone.KeywordBlocklist[k] = v
	}
	for k, v := range s.GapKeywords {
		clone.GapKeywords[k] = v
	}
	return clone
}
