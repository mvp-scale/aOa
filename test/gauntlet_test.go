package test

import (
	"testing"
	"time"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
)

// =============================================================================
// Search Performance Gauntlet — G0 regression suite
//
// 22-shape query matrix covering every Search() code path.
// Catches 1000x+ regressions that single-path benchmarks miss.
//
// Session 71 found two explosive bugs:
//   - Regex search bypassing trigram index (5s→8ms, 625x)
//   - Symbol search iterating empty metadata in lean mode (186ms→4µs, 46,500x)
//
// Neither would have been caught by BenchmarkSearch_E2E (literal-only, no trigram warm).
// =============================================================================

// gauntletEngines holds pre-built engines for gauntlet benchmarks and ceiling tests.
type gauntletEngines struct {
	full *index.SearchEngine // 500 files, 1000 symbols, FileCache + trigram index
	lean *index.SearchEngine // same files, empty Metadata, FileCache + trigram index
}

// buildGauntletEngines creates two search engines from the shared 500-file index.
// Full engine: all symbols indexed. Lean engine: tokenization-only (empty Metadata).
// Both have warmed FileCaches with trigram indices — the key difference from
// existing benchmarks which never call WarmCache().
func buildGauntletEngines(tb testing.TB) gauntletEngines {
	dir := tb.TempDir()
	idx := build500FileIndex(dir)

	// Full engine: all metadata populated, cache warmed with trigram index
	fullEngine := index.NewSearchEngine(idx, nil, dir)
	fullCache := index.NewFileCache(0)
	fullEngine.SetCache(fullCache)
	fullEngine.WarmCache()

	// Lean engine: same tokens/files but empty Metadata (triggers lean code path).
	// This exercises the len(e.idx.Metadata) > 0 guard in Search().
	leanIdx := &ports.Index{
		Tokens:   idx.Tokens,
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    idx.Files,
	}
	leanEngine := index.NewSearchEngine(leanIdx, nil, dir)
	leanCache := index.NewFileCache(0)
	leanEngine.SetCache(leanCache)
	leanEngine.WarmCache()

	return gauntletEngines{full: fullEngine, lean: leanEngine}
}

type gauntletCase struct {
	name    string
	query   string
	opts    ports.SearchOptions
	useLean bool
	ceiling time.Duration
}

// gauntletCases defines the 22-shape query matrix.
// Each tests a specific code path in Search(). Ceilings are 5-50x above expected
// sub-ms times — generous enough to avoid CI flakiness, tight enough to catch
// 1000x blowups like the Session 71 regressions.
var gauntletCases = []gauntletCase{
	// --- Literal: searchLiteral + trigram content scan ---
	{"Literal/Full", "dashboard", ports.SearchOptions{}, false, 1 * time.Millisecond},
	{"Literal/Lean", "dashboard", ports.SearchOptions{}, true, 1 * time.Millisecond},

	// --- OR: searchOR union + trigram content scan ---
	{"OR/Full", "dashboard render", ports.SearchOptions{}, false, 2 * time.Millisecond},
	{"OR/Lean", "dashboard render", ports.SearchOptions{}, true, 5 * time.Millisecond},

	// --- AND: searchAND intersection + brute-force content scan ---
	{"AND/Full", "handler,response", ports.SearchOptions{AndMode: true}, false, 10 * time.Millisecond},
	{"AND/Lean", "handler,response", ports.SearchOptions{AndMode: true}, true, 10 * time.Millisecond},

	// --- Regex concatenation: scanContentRegexTrigram intersect ---
	{"Regex_Concat/Full", "Handler.*Response", ports.SearchOptions{Mode: "regex"}, false, 5 * time.Millisecond},
	{"Regex_Concat/Lean", "Handler.*Response", ports.SearchOptions{Mode: "regex"}, true, 5 * time.Millisecond},

	// --- Regex alternation: scanContentRegexTrigram union ---
	{"Regex_Alt/Full", "OpenDatabase|LoadConfig|NewHTTPClient", ports.SearchOptions{Mode: "regex"}, false, 5 * time.Millisecond},
	{"Regex_Alt/Lean", "OpenDatabase|LoadConfig|NewHTTPClient", ports.SearchOptions{Mode: "regex"}, true, 5 * time.Millisecond},

	// --- Case-insensitive: lowered tokens + trigram ---
	{"CaseInsensitive/Full", "dashboard", ports.SearchOptions{Mode: "case_insensitive"}, false, 2 * time.Millisecond},
	{"CaseInsensitive/Lean", "dashboard", ports.SearchOptions{Mode: "case_insensitive"}, true, 2 * time.Millisecond},

	// --- Word boundary: literal + brute-force word check (regex per line × 500 files) ---
	{"WordBoundary/Full", "handler", ports.SearchOptions{WordBoundary: true}, false, 30 * time.Millisecond},
	{"WordBoundary/Lean", "handler", ports.SearchOptions{WordBoundary: true}, true, 30 * time.Millisecond},

	// --- Invert match: invertSymbolHits + brute-force full scan ---
	{"InvertMatch/Full", "dashboard", ports.SearchOptions{InvertMatch: true}, false, 50 * time.Millisecond},
	{"InvertMatch/Lean", "dashboard", ports.SearchOptions{InvertMatch: true}, true, 50 * time.Millisecond},

	// --- Glob filters: literal + per-file glob regex compilation ---
	{"GlobInclude/Full", "handler", ports.SearchOptions{IncludeGlob: "pkg1/*"}, false, 40 * time.Millisecond},
	{"GlobExclude/Full", "handler", ports.SearchOptions{ExcludeGlob: "pkg0/*"}, false, 40 * time.Millisecond},

	// --- Context lines: full path + attachContextLines ---
	{"ContextLines/Full", "dashboard", ports.SearchOptions{Context: 3}, false, 2 * time.Millisecond},

	// --- Count only: literal + count branch (500 symbol refs + content scan) ---
	{"CountOnly/Full", "handler", ports.SearchOptions{CountOnly: true}, false, 5 * time.Millisecond},

	// --- Quiet: literal + quiet branch ---
	{"Quiet/Full", "handler", ports.SearchOptions{Quiet: true}, false, 2 * time.Millisecond},

	// --- Only matching: regex + applyOnlyMatching (regex scan + content brute-force) ---
	// Uses "Handler" (capitalized) so regex matches symbol names and content lines.
	{"OnlyMatching/Full", "Handler", ports.SearchOptions{OnlyMatching: true, Mode: "regex"}, false, 20 * time.Millisecond},
}

// BenchmarkSearchGauntlet runs all 22 query shapes as sub-benchmarks.
// Produces benchstat-compatible output for regression tracking.
//
// Run:     go test ./test/ -bench=BenchmarkSearchGauntlet -benchmem -run=^$ -count=6
// Compare: benchstat baseline.txt current.txt
func BenchmarkSearchGauntlet(b *testing.B) {
	engines := buildGauntletEngines(b)

	for _, tc := range gauntletCases {
		tc := tc
		engine := engines.full
		if tc.useLean {
			engine = engines.lean
		}
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				engine.Search(tc.query, tc.opts)
			}
		})
	}
}

// TestSearchGauntlet_G0Ceiling is the canary test that runs during `go test ./...`.
// Each query shape must complete under its ceiling. One warmup run, then one
// measured run per shape. Part of `make check` — catches explosive regressions in CI.
func TestSearchGauntlet_G0Ceiling(t *testing.T) {
	engines := buildGauntletEngines(t)

	// Warmup: run each query once to prime any lazy init
	for _, tc := range gauntletCases {
		engine := engines.full
		if tc.useLean {
			engine = engines.lean
		}
		engine.Search(tc.query, tc.opts)
	}

	// Measured run: assert each query completes under its ceiling
	for _, tc := range gauntletCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			engine := engines.full
			if tc.useLean {
				engine = engines.lean
			}

			start := time.Now()
			engine.Search(tc.query, tc.opts)
			elapsed := time.Since(start)

			t.Logf("%-30s %v (ceiling: %v)", tc.name, elapsed, tc.ceiling)
			if elapsed > tc.ceiling {
				t.Errorf("CEILING BREACH: %s took %v, ceiling is %v (%.1fx over)",
					tc.name, elapsed, tc.ceiling, float64(elapsed)/float64(tc.ceiling))
			}
		})
	}
}
