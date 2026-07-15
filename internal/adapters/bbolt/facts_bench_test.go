package bbolt

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/ports"
)

// Spec §5 budget row (01-facts-substrate.md): watch fact-swap ≤2ms.
// Gate evidence for FDN-3 — one changed file's raw facts atomically replaced
// (the watcher-path write), against a store pre-seeded with 200 other files.
func BenchmarkReplaceFactsForFile(b *testing.B) {
	store, err := NewStore(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { store.Close() })

	mkFacts := func(path string, n int) []ports.Fact {
		fs := make([]ports.Fact, n)
		for i := range fs {
			fs[i] = ports.Fact{
				Kind: ports.FactDep, Subject: "go:" + path,
				Attrs:  map[string]string{"spec": fmt.Sprintf("example.com/lib%02d", i)},
				Source: ports.FactSource{File: path + "/f.go", Line: uint32(i + 1)},
				Prov:   ports.ProvDerived, TS: 1,
			}
		}
		return fs
	}
	for f := 0; f < 200; f++ {
		p := fmt.Sprintf("internal/pkg%03d", f)
		if err := store.ReplaceFactsForFile("proj", p+"/f.go", mkFacts(p, 15)); err != nil {
			b.Fatal(err)
		}
	}
	hot := mkFacts("internal/hot", 15)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.ReplaceFactsForFile("proj", "internal/hot/f.go", hot); err != nil {
			b.Fatal(err)
		}
	}
}

// Operative watcher-path shape after the D11 correction: a burst of changed
// files lands in ONE debounced batch tx. Amortized per-file cost is the
// number the watch budget actually governs.
func BenchmarkReplaceAllFacts200FileBurst(b *testing.B) {
	store, err := NewStore(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { store.Close() })

	burst := make(map[string][]ports.Fact, 200)
	for f := 0; f < 200; f++ {
		p := fmt.Sprintf("internal/pkg%03d", f)
		fs := make([]ports.Fact, 15)
		for i := range fs {
			fs[i] = ports.Fact{
				Kind: ports.FactDep, Subject: "go:" + p,
				Attrs:  map[string]string{"spec": fmt.Sprintf("example.com/lib%02d", i)},
				Source: ports.FactSource{File: p + "/f.go", Line: uint32(i + 1)},
				Prov:   ports.ProvDerived, TS: 1,
			}
		}
		burst[p+"/f.go"] = fs
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.ReplaceAllFacts("proj", burst); err != nil {
			b.Fatal(err)
		}
	}
}
