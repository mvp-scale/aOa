package facts

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// depFact builds a raw FactDep the same shape app.importEdgeToFact produces.
func depFact(subject, spec, fromFile string, line uint32) ports.Fact {
	return ports.Fact{
		Kind:    ports.FactDep,
		Subject: subject,
		Attrs:   map[string]string{"spec": spec},
		Source:  ports.FactSource{File: fromFile, Line: line},
		Prov:    ports.ProvDerived,
		TS:      1781136000,
	}
}

func TestCompact_ResolvesGoIntraRepoEdge(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{"github.com/corey/aoa": ""}}
	fileSet := map[string]bool{
		"internal/app/app.go":     true,
		"internal/ports/facts.go": true,
	}
	raw := []ports.Fact{
		depFact("go:internal/app", "github.com/corey/aoa/internal/ports", "internal/app/app.go", 11),
	}

	units, adj, findings := CompactWithManifests(raw, fileSet, manifests)

	require.Len(t, units, 2, "importing unit + resolved target unit")
	ids := []string{units[0].Subject, units[1].Subject}
	assert.ElementsMatch(t, []string{"go:internal/app", "go:internal/ports"}, ids)
	for _, u := range units {
		assert.Equal(t, ports.FactUnit, u.Kind)
		assert.Equal(t, ports.ProvDerived, u.Prov)
	}

	require.NotNil(t, adj)
	require.Contains(t, adj.Fwd, "go:internal/app")
	assert.Equal(t, []ports.DepEdge{{Unit: "go:internal/ports", Count: 1}}, adj.Fwd["go:internal/app"])
	require.Contains(t, adj.Rev, "go:internal/ports")
	assert.Equal(t, []ports.DepEdge{{Unit: "go:internal/app", Count: 1}}, adj.Rev["go:internal/ports"])

	// go:internal/app has no inbound edge in this two-node fixture (nothing
	// else imports it) and isn't an entrypoint glob, so it is a legitimate
	// orphan finding — no cycle/god/broken-import expected.
	require.Len(t, findings, 1)
	assert.Equal(t, RuleOrphan, findings[0].Attrs["rule"])
	assert.Equal(t, "go:internal/app", findings[0].Subject)
}

// TestCompact_ResolvesGoIntraRepoEdgeToModuleRoot reproduces a real,
// verified divergence found via a gin (github.com/gin-gonic/gin) real-derive
// diff (FDN-4 gate follow-up): a subpackage importing its own module's ROOT
// package by full import path (e.g. ginS/gins.go importing
// "github.com/gin-gonic/gin") resolves to ImportPath == "" (§2.4 resolveGo,
// "no unresolved case for Go" — bestDir/remainder both empty means "root",
// never "unresolved"). subjectFromResolvedPath must map that to the "go:"
// root subject (matching factSubjectForFile's own dir=="" -> "go:" + ""
// convention), NOT return "" bare — the previous code returned "" for BOTH
// "resolved to root" and would-be "unresolved" cases, so Compact's
// `obj == "" || obj == subj` phantom/self-loop guard silently discarded a
// real, distinct, non-self edge to the module root.
func TestCompact_ResolvesGoIntraRepoEdgeToModuleRoot(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{"github.com/gin-gonic/gin": ""}}
	fileSet := map[string]bool{"ginS/gins.go": true}
	raw := []ports.Fact{
		depFact("go:ginS", "github.com/gin-gonic/gin", "ginS/gins.go", 12),
	}

	units, adj, _ := CompactWithManifests(raw, fileSet, manifests)

	require.Len(t, units, 2, "importing unit (go:ginS) + resolved root unit (go:)")
	ids := []string{units[0].Subject, units[1].Subject}
	assert.ElementsMatch(t, []string{"go:ginS", "go:"}, ids)

	require.Contains(t, adj.Fwd, "go:ginS", "the edge to the module root must not be dropped as a phantom/self-loop")
	assert.Equal(t, []ports.DepEdge{{Unit: "go:", Count: 1}}, adj.Fwd["go:ginS"])
}

func TestCompact_ExternalEdgeNotSynthesizedAsUnit(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{"github.com/corey/aoa": ""}}
	fileSet := map[string]bool{"internal/app/app.go": true}
	raw := []ports.Fact{
		depFact("go:internal/app", "fmt", "internal/app/app.go", 3),
	}

	units, adj, _ := CompactWithManifests(raw, fileSet, manifests)

	require.Len(t, units, 1)
	assert.Equal(t, "go:internal/app", units[0].Subject)
	require.Contains(t, adj.Fwd, "go:internal/app")
	assert.Equal(t, "ext:std/fmt", adj.Fwd["go:internal/app"][0].Unit)
}

func TestCompact_AggregatesMultipleImportSitesIntoOneCount(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{"github.com/corey/aoa": ""}}
	fileSet := map[string]bool{
		"internal/app/app.go":     true,
		"internal/app/watcher.go": true,
		"internal/ports/facts.go": true,
	}
	raw := []ports.Fact{
		depFact("go:internal/app", "github.com/corey/aoa/internal/ports", "internal/app/app.go", 11),
		depFact("go:internal/app", "github.com/corey/aoa/internal/ports", "internal/app/watcher.go", 4),
	}

	_, adj, _ := CompactWithManifests(raw, fileSet, manifests)

	require.Contains(t, adj.Fwd, "go:internal/app")
	assert.Equal(t, uint16(2), adj.Fwd["go:internal/app"][0].Count)
}

func TestCompact_UnresolvedSpecFeedseBrokenImportFinding(t *testing.T) {
	manifests := Manifests{}
	fileSet := map[string]bool{"pkg/mod.py": true}
	raw := []ports.Fact{
		depFact("py:pkg/mod", "..missing.thing", "pkg/mod.py", 5),
	}

	units, adj, findings := CompactWithManifests(raw, fileSet, manifests)

	require.Len(t, units, 1, "the importing unit is still synthesized even though its edge is unresolved")
	assert.Empty(t, adj.Fwd["py:pkg/mod"], "unresolved edges never enter the adjacency")

	var broken *ports.Fact
	for i := range findings {
		if findings[i].Attrs["rule"] == RuleBrokenImport {
			broken = &findings[i]
		}
	}
	require.NotNil(t, broken, "expected a broken_import finding")
	assert.Equal(t, "py:pkg/mod", broken.Subject)
	assert.Contains(t, broken.Attrs["specs"], "..missing.thing")
}

func TestCompact_DeterministicUnitOrder(t *testing.T) {
	manifests := Manifests{GoModules: map[string]string{"github.com/corey/aoa": ""}}
	fileSet := map[string]bool{
		"internal/app/app.go":        true,
		"internal/domain/index.go":   true,
		"internal/adapters/bbolt.go": true,
	}
	raw := []ports.Fact{
		depFact("go:internal/app", "github.com/corey/aoa/internal/adapters", "internal/app/app.go", 1),
		depFact("go:internal/domain", "github.com/corey/aoa/internal/adapters", "internal/domain/index.go", 1),
	}

	units1, _, _ := CompactWithManifests(raw, fileSet, manifests)
	units2, _, _ := CompactWithManifests(raw, fileSet, manifests)
	require.Equal(t, units1, units2, "Compact must be a pure function — same input, byte-identical output")

	ids := make([]string, len(units1))
	for i, u := range units1 {
		ids[i] = u.Subject
	}
	assert.Equal(t, []string{"go:internal/adapters", "go:internal/app", "go:internal/domain"}, ids)
}

func TestCompact_UnitSourcePrefersSubjectOwnFactOverBestEffortTarget(t *testing.T) {
	// go:internal/ports is reached two ways in this raw set: once as the
	// mere *target* of go:internal/app's import (best-effort source =
	// the importer's own file, internal/app/app.go — the only pointer
	// available when a unit is first met purely as a dependency object),
	// and once as the *subject* of its own dep fact (its real source,
	// internal/ports/facts.go). The unit's Source must end up as the real
	// subject-owning file regardless of which raw fact the compactor
	// happens to visit first.
	manifests := Manifests{GoModules: map[string]string{"github.com/corey/aoa": ""}}
	fileSet := map[string]bool{
		"internal/app/app.go":     true,
		"internal/ports/facts.go": true,
	}
	factA := depFact("go:internal/app", "github.com/corey/aoa/internal/ports", "internal/app/app.go", 11)
	factB := depFact("go:internal/ports", "os", "internal/ports/facts.go", 3)

	unitsAB, _, _ := CompactWithManifests([]ports.Fact{factA, factB}, fileSet, manifests)
	unitsBA, _, _ := CompactWithManifests([]ports.Fact{factB, factA}, fileSet, manifests)

	sourceOf := func(units []ports.Fact, id string) ports.FactSource {
		for _, u := range units {
			if u.Subject == id {
				return u.Source
			}
		}
		t.Fatalf("unit %s not found", id)
		return ports.FactSource{}
	}

	assert.Equal(t, "internal/ports/facts.go", sourceOf(unitsAB, "go:internal/ports").File,
		"factA-then-factB order must not let the best-effort target source shadow the real subject source")
	assert.Equal(t, "internal/ports/facts.go", sourceOf(unitsBA, "go:internal/ports").File,
		"factB-then-factA order is already correct and must stay correct")
}

func TestCompact_IgnoresNonDepFacts(t *testing.T) {
	raw := []ports.Fact{
		{Kind: ports.FactUnit, Subject: "go:internal/app", Prov: ports.ProvDerived},
	}
	units, adj, findings := CompactWithManifests(raw, map[string]bool{}, Manifests{})
	assert.Empty(t, units)
	assert.Empty(t, adj.Fwd)
	assert.Empty(t, findings)
}

func TestCompact_EmptyInput(t *testing.T) {
	units, adj, findings := CompactWithManifests(nil, map[string]bool{}, Manifests{})
	assert.Empty(t, units)
	assert.NotNil(t, adj)
	assert.Empty(t, findings)
}
