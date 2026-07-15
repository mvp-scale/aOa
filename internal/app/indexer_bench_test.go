//go:build !lean

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/treesitter"
)

// repoRootForBench locates the aOa-go module root by walking up from the
// current working directory (internal/app when this test runs) until a
// go.mod is found. Used only by the FDN-2 (board #28) real-corpus
// benchmarks below — perf verification for the extractor-registry dispatch
// swap must run over live source, not a synthetic fixture.
func repoRootForBench(b *testing.B) string {
	b.Helper()
	dir, err := os.Getwd()
	if err != nil {
		b.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			b.Fatal("go.mod not found walking up from cwd")
		}
		dir = parent
	}
}

// BenchmarkBuildIndex_RealCorpus_Internal measures BuildIndex over this
// repo's own internal/ tree — the perf gate for the FDN-2 extractor-registry
// dispatch swap (extractors.go replacing the switch statements previously in
// parser.go's extractSymbols and imports.go's extractImports). This is a
// real, large, mixed corpus (hundreds of Go files across many packages),
// not the single 24-line synthetic file BenchmarkParseFile exercises.
//
// Interleave against the pre-registry baseline (git stash the registry
// commit, or checkout HEAD~1 for parser.go/imports.go) and compare with:
//
//	go test ./internal/app/ -bench=BenchmarkBuildIndex_RealCorpus -benchmem -run=^$ -count=6
//	benchstat baseline.txt current.txt
func BenchmarkBuildIndex_RealCorpus_Internal(b *testing.B) {
	root := filepath.Join(repoRootForBench(b), "internal")
	parser := treesitter.NewParser()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := BuildIndex(root, parser)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBuildIndex_RealCorpus_Cmd is the same measurement over cmd/ — a
// smaller tree with a different file/symbol mix (Cobra command wiring).
func BenchmarkBuildIndex_RealCorpus_Cmd(b *testing.B) {
	root := filepath.Join(repoRootForBench(b), "cmd")
	parser := treesitter.NewParser()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := BuildIndex(root, parser)
		if err != nil {
			b.Fatal(err)
		}
	}
}
