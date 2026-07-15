package facts

import (
	"fmt"
	"testing"

	"github.com/corey/aoa/internal/ports"
)

// Spec §5 budget row (01-facts-substrate.md): compact ≤50ms @ 5k deps.
// Gate evidence for FDN-3 — synthetic corpus shaped like the real emission
// path (file-grain raw FactDep, Attrs["spec"], Object empty): 250 units ×
// 20 deps = 5,000 raw facts, ~40% intra-repo resolvable, rest external.
func BenchmarkCompact5kDeps(b *testing.B) {
	const units = 250
	const depsPerUnit = 20

	fileSet := make(map[string]bool, units)
	raw := make([]ports.Fact, 0, units*depsPerUnit)
	for u := 0; u < units; u++ {
		dir := fmt.Sprintf("internal/pkg%03d", u)
		file := dir + "/mod.go"
		fileSet[file] = true
		for d := 0; d < depsPerUnit; d++ {
			var spec string
			if d%5 < 2 { // intra-repo target
				spec = fmt.Sprintf("github.com/corey/aoa/internal/pkg%03d", (u+d+1)%units)
			} else { // external
				spec = fmt.Sprintf("example.com/ext/lib%02d", d)
			}
			raw = append(raw, ports.Fact{
				Kind: ports.FactDep, Subject: "go:" + dir,
				Attrs:  map[string]string{"spec": spec},
				Source: ports.FactSource{File: file, Line: uint32(d + 1)},
				Prov:   ports.ProvDerived, TS: 1,
			})
		}
	}
	manifests := Manifests{GoModules: map[string]string{"github.com/corey/aoa": ""}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CompactWithManifests(raw, fileSet, manifests)
	}
}
