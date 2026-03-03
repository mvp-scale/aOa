package index

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/ports"
)

// buildBenchIndex creates a realistic in-memory index with numFiles files,
// symsPerFile symbols per file, and optional file content on disk for content scanning.
// Returns the engine and the temp dir (empty string if no disk files).
func buildBenchIndex(b *testing.B, numFiles, symsPerFile int, withContent bool) (*SearchEngine, string) {
	b.Helper()

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}

	// Build atlas-like domain map for enrichment
	domains := map[string]Domain{
		"@auth": {Terms: map[string][]string{
			"Authentication": {"login", "auth", "token", "session", "password"},
			"Authorization":  {"role", "permission", "access", "policy"},
		}},
		"@data": {Terms: map[string][]string{
			"Database": {"query", "insert", "update", "delete", "table", "row"},
			"Cache":    {"cache", "redis", "memcache", "ttl", "evict"},
		}},
		"@web": {Terms: map[string][]string{
			"HTTP":       {"handler", "request", "response", "middleware", "route"},
			"WebSocket":  {"socket", "connect", "message", "channel"},
			"REST":       {"endpoint", "api", "resource", "crud"},
		}},
	}

	// Token pool — realistic Go symbol names
	tokenPool := []string{
		"handler", "request", "response", "config", "server", "client",
		"query", "result", "error", "context", "logger", "metric",
		"cache", "store", "service", "auth", "token", "session",
		"user", "role", "login", "validate", "parse", "format",
		"read", "write", "update", "delete", "list", "get",
		"create", "start", "stop", "init", "close", "connect",
		"send", "receive", "process", "handle", "execute", "run",
		"build", "test", "check", "verify", "assert", "expect",
		"load", "save", "fetch", "push", "pull", "sync",
	}

	var dir string
	if withContent {
		dir = b.TempDir()
	}

	for f := uint32(1); f <= uint32(numFiles); f++ {
		path := fmt.Sprintf("pkg/mod%d/file%d.go", f%10, f)
		idx.Files[f] = &ports.FileMeta{
			Path:     path,
			Language: "go",
			Size:     4096,
			Domain:   "@web", // file-level domain set (fast path)
		}

		// Write file content for content scanning benchmarks
		if withContent {
			absPath := filepath.Join(dir, path)
			os.MkdirAll(filepath.Dir(absPath), 0o755)
			var content string
			for s := 0; s < symsPerFile; s++ {
				tok := tokenPool[(int(f)*symsPerFile+s)%len(tokenPool)]
				content += fmt.Sprintf("func %s%d_%d() {\n\t// handle %s logic\n\treturn nil\n}\n\n",
					tok, f, s, tok)
			}
			os.WriteFile(absPath, []byte(content), 0o644)
		}

		for s := 0; s < symsPerFile; s++ {
			line := uint16(s*5 + 1) // 5 lines per symbol
			ref := ports.TokenRef{FileID: f, Line: line}

			tok := tokenPool[(int(f)*symsPerFile+s)%len(tokenPool)]
			name := fmt.Sprintf("%s%d_%d", tok, f, s)

			idx.Metadata[ref] = &ports.SymbolMeta{
				Name:      name,
				Kind:      "function",
				Signature: name + "(ctx context.Context)",
				StartLine: line,
				EndLine:   line + 4,
			}

			// Index under the base token name
			idx.Tokens[tok] = append(idx.Tokens[tok], ref)
			// Also index under the full name for literal match
			idx.Tokens[name] = append(idx.Tokens[name], ref)
		}
	}

	projectRoot := ""
	if withContent {
		projectRoot = dir
	}
	engine := NewSearchEngine(idx, domains, projectRoot)

	if withContent {
		cache := NewFileCache(0)
		engine.SetCache(cache)
		engine.WarmCache()
	}

	return engine, dir
}

// =============================================================================
// Symbol search benchmarks — the core O(1) hot path
// =============================================================================

// BenchmarkSearch_Literal_Small: single-token O(1) lookup, 100 files × 10 symbols
func BenchmarkSearch_Literal_Small(b *testing.B) {
	engine, _ := buildBenchIndex(b, 100, 10, false)
	opts := ports.SearchOptions{MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("handler", opts)
	}
}

// BenchmarkSearch_Literal_Medium: single-token lookup, 500 files × 20 symbols
func BenchmarkSearch_Literal_Medium(b *testing.B) {
	engine, _ := buildBenchIndex(b, 500, 20, false)
	opts := ports.SearchOptions{MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("handler", opts)
	}
}

// BenchmarkSearch_Literal_Large: single-token lookup, 2000 files × 20 symbols
func BenchmarkSearch_Literal_Large(b *testing.B) {
	engine, _ := buildBenchIndex(b, 2000, 20, false)
	opts := ports.SearchOptions{MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("handler", opts)
	}
}

// BenchmarkSearch_OR_TwoTokens: multi-term union search
func BenchmarkSearch_OR_TwoTokens(b *testing.B) {
	engine, _ := buildBenchIndex(b, 500, 20, false)
	opts := ports.SearchOptions{MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("handler request", opts)
	}
}

// BenchmarkSearch_AND: intersection search
func BenchmarkSearch_AND(b *testing.B) {
	engine, _ := buildBenchIndex(b, 500, 20, false)
	opts := ports.SearchOptions{AndMode: true, MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("handler,request", opts)
	}
}

// BenchmarkSearch_Regex: O(n) full symbol scan
func BenchmarkSearch_Regex(b *testing.B) {
	engine, _ := buildBenchIndex(b, 500, 20, false)
	opts := ports.SearchOptions{Mode: "regex", MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("handler.*\\d+", opts)
	}
}

// BenchmarkSearch_CaseInsensitive: case-insensitive symbol search
func BenchmarkSearch_CaseInsensitive(b *testing.B) {
	engine, _ := buildBenchIndex(b, 500, 20, false)
	opts := ports.SearchOptions{Mode: "case_insensitive", MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("Handler", opts)
	}
}

// =============================================================================
// Content search benchmarks — trigram vs brute force
// =============================================================================

// BenchmarkSearch_Content_Trigram: content scan with trigram index
func BenchmarkSearch_Content_Trigram(b *testing.B) {
	engine, _ := buildBenchIndex(b, 100, 10, true)
	opts := ports.SearchOptions{MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("context", opts)
	}
}

// BenchmarkSearch_Content_BruteForce: content scan with short query (no trigram)
func BenchmarkSearch_Content_BruteForce(b *testing.B) {
	engine, _ := buildBenchIndex(b, 100, 10, true)
	opts := ports.SearchOptions{MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("fn", opts) // 2 chars, too short for trigram
	}
}

// =============================================================================
// Enrichment benchmarks — domain + tag generation
// =============================================================================

// BenchmarkEnrichHits: domain assignment + tag generation for 20 hits
func BenchmarkEnrichHits(b *testing.B) {
	engine, _ := buildBenchIndex(b, 500, 20, false)

	// Build 20 hits to enrich
	var hits []Hit
	for ref, sym := range engine.idx.Metadata {
		file := engine.idx.Files[ref.FileID]
		if file == nil || sym == nil {
			continue
		}
		hits = append(hits, engine.buildHit(ref, sym, file))
		if len(hits) >= 20 {
			break
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Reset domains so enrichment has to recompute
		for j := range hits {
			hits[j].Domain = ""
			hits[j].Tags = nil
		}
		engine.enrichHits(hits)
	}
}

// BenchmarkEnrichHits_NoDomain: enrichment fallback when file has no domain
func BenchmarkEnrichHits_NoDomain(b *testing.B) {
	engine, _ := buildBenchIndex(b, 500, 20, false)

	// Clear file domains to force keyword overlap scoring
	for _, f := range engine.idx.Files {
		f.Domain = ""
	}

	var hits []Hit
	for ref, sym := range engine.idx.Metadata {
		file := engine.idx.Files[ref.FileID]
		if file == nil || sym == nil {
			continue
		}
		hits = append(hits, engine.buildHit(ref, sym, file))
		if len(hits) >= 20 {
			break
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := range hits {
			hits[j].Domain = ""
			hits[j].Tags = nil
		}
		engine.enrichHits(hits)
	}
}

// =============================================================================
// Glob matching benchmark — regex compilation per call
// =============================================================================

// BenchmarkFnmatchGlob: measure regex compilation cost per glob match
func BenchmarkFnmatchGlob(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		fnmatchGlob("*.go", "pkg/auth/handler.go")
	}
}

// BenchmarkSearch_WithGlob: full search with glob filter active
func BenchmarkSearch_WithGlob(b *testing.B) {
	engine, _ := buildBenchIndex(b, 500, 20, false)
	opts := ports.SearchOptions{MaxCount: 20, IncludeGlob: "*.go"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("handler", opts)
	}
}

// =============================================================================
// Full pipeline benchmark — end-to-end search
// =============================================================================

// BenchmarkSearch_FullPipeline: symbol + content + enrich + tags
func BenchmarkSearch_FullPipeline(b *testing.B) {
	engine, _ := buildBenchIndex(b, 200, 15, true)
	opts := ports.SearchOptions{MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("handler", opts)
	}
}

// BenchmarkSearch_FullPipeline_CaseInsensitive: full pipeline, case-insensitive
func BenchmarkSearch_FullPipeline_CaseInsensitive(b *testing.B) {
	engine, _ := buildBenchIndex(b, 200, 15, true)
	opts := ports.SearchOptions{MaxCount: 20, Mode: "case_insensitive"}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.Search("Handler", opts)
	}
}

// =============================================================================
// Component micro-benchmarks
// =============================================================================

// BenchmarkBuildResult: TotalMatchChars loop over large hit set
func BenchmarkBuildResult(b *testing.B) {
	// Simulate 1000 hits to measure TotalMatchChars overhead
	hits := make([]Hit, 1000)
	for i := range hits {
		hits[i] = Hit{
			File:    fmt.Sprintf("pkg/mod%d/file.go", i),
			Content: "func handleRequest(ctx context.Context) error {",
			Kind:    "content",
		}
	}

	engine, _ := buildBenchIndex(b, 10, 5, false)
	opts := ports.SearchOptions{MaxCount: 20}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		engine.buildResult(hits, opts, 20)
	}
}

// BenchmarkTokenize: tokenizer performance
func BenchmarkTokenize(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Tokenize("handleAuthRequest")
	}
}

// BenchmarkTokenizeContentLine: content line tokenizer
func BenchmarkTokenizeContentLine(b *testing.B) {
	line := "\tfmt.Printf(\"user %s logged in from %s\", user.Name, req.RemoteAddr)"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		TokenizeContentLine(line)
	}
}

// BenchmarkContainsFold: case-insensitive match (hot path for brute-force content scan)
func BenchmarkContainsFold(b *testing.B) {
	line := "func (s *Server) HandleAuthRequest(ctx context.Context, req *AuthRequest) (*AuthResponse, error) {"
	query := "authrequest"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		containsFold(line, query)
	}
}
