package facts

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unitFact(id string) ports.Fact {
	return ports.Fact{Kind: ports.FactUnit, Subject: id, Prov: ports.ProvDerived}
}

func TestDetectCycles_ThreeUnitCycle(t *testing.T) {
	units := []ports.Fact{unitFact("go:a"), unitFact("go:b"), unitFact("go:c")}
	adj := &ports.DepAdjacency{Fwd: map[string][]ports.DepEdge{
		"go:a": {{Unit: "go:b", Count: 1}},
		"go:b": {{Unit: "go:c", Count: 1}},
		"go:c": {{Unit: "go:a", Count: 1}},
	}}

	findings := detectCycles(units, adj)
	require.Len(t, findings, 1)
	f := findings[0]
	assert.Equal(t, RuleCycle, f.Attrs["rule"])
	assert.Equal(t, "go:a,go:b,go:c", f.Attrs["members"])
	assert.Equal(t, "3", f.Attrs["size"])
	assert.Equal(t, "go:a", f.Subject, "subject is the lexicographically-first member")
}

func TestDetectCycles_NoCycleInDAG(t *testing.T) {
	units := []ports.Fact{unitFact("go:a"), unitFact("go:b")}
	adj := &ports.DepAdjacency{Fwd: map[string][]ports.DepEdge{
		"go:a": {{Unit: "go:b", Count: 1}},
	}}
	assert.Empty(t, detectCycles(units, adj))
}

func TestDetectCycles_SelfLoopIsTrivialAndSkipped(t *testing.T) {
	// Matches arch.TarjanSCC's documented convention (arch_test.go
	// TestTarjanSCC_SelfLoop): a self-loop is a size-1 SCC, deliberately
	// not reported as a cycle finding.
	units := []ports.Fact{unitFact("go:a")}
	adj := &ports.DepAdjacency{Fwd: map[string][]ports.DepEdge{
		"go:a": {{Unit: "go:a", Count: 1}},
	}}
	assert.Empty(t, detectCycles(units, adj))
}

func TestDetectGodUnits_ThresholdBothDirections(t *testing.T) {
	units := []ports.Fact{unitFact("go:hub")}
	fwd := make([]ports.DepEdge, 12)
	rev := make([]ports.DepEdge, 12)
	for i := range fwd {
		fwd[i] = ports.DepEdge{Unit: "ext:x"}
		rev[i] = ports.DepEdge{Unit: "ext:y"}
	}
	adj := &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{"go:hub": fwd},
		Rev: map[string][]ports.DepEdge{"go:hub": rev},
	}

	findings := detectGodUnits(units, adj, DefaultGodThreshold)
	require.Len(t, findings, 1)
	assert.Equal(t, RuleGodUnit, findings[0].Attrs["rule"])
	assert.Equal(t, "12", findings[0].Attrs["fan_in"])
	assert.Equal(t, "12", findings[0].Attrs["fan_out"])
}

func TestDetectGodUnits_BelowThresholdOneDirection(t *testing.T) {
	units := []ports.Fact{unitFact("go:hub")}
	fwd := make([]ports.DepEdge, 12)
	rev := make([]ports.DepEdge, 3) // fan-in below threshold
	adj := &ports.DepAdjacency{
		Fwd: map[string][]ports.DepEdge{"go:hub": fwd},
		Rev: map[string][]ports.DepEdge{"go:hub": rev},
	}
	assert.Empty(t, detectGodUnits(units, adj, DefaultGodThreshold))
}

func TestDetectOrphans_ZeroInbound(t *testing.T) {
	units := []ports.Fact{unitFact("go:leaf"), unitFact("go:used")}
	adj := &ports.DepAdjacency{Rev: map[string][]ports.DepEdge{
		"go:used": {{Unit: "go:leaf", Count: 1}},
	}}
	findings := detectOrphans(units, adj)
	require.Len(t, findings, 1)
	assert.Equal(t, "go:leaf", findings[0].Subject)
	assert.Equal(t, RuleOrphan, findings[0].Attrs["rule"])
}

func TestDetectOrphans_EntrypointExcluded(t *testing.T) {
	units := []ports.Fact{unitFact("go:cmd/aoa"), unitFact("py:scripts/main")}
	assert.Empty(t, detectOrphans(units, &ports.DepAdjacency{}))
}

func TestDetectDeadCandidates_NotMeasuredEmitsNothing(t *testing.T) {
	orphans := []ports.Fact{{Subject: "go:leaf"}}
	assert.Empty(t, detectDeadCandidates(orphans, nil))
}

func TestDetectDeadCandidates_MeasuredZeroRefs(t *testing.T) {
	orphans := []ports.Fact{{Subject: "go:leaf"}}
	findings := detectDeadCandidates(orphans, map[string]int{})
	require.Len(t, findings, 1)
	assert.Equal(t, RuleDeadCandidate, findings[0].Attrs["rule"])
	assert.Equal(t, "0", findings[0].Attrs["refs"])
}

func TestDetectDeadCandidates_MeasuredWithRefsExcluded(t *testing.T) {
	orphans := []ports.Fact{{Subject: "go:leaf"}}
	findings := detectDeadCandidates(orphans, map[string]int{"go:leaf": 3})
	assert.Empty(t, findings)
}

func TestDetectBrokenImports_GroupsBySubject(t *testing.T) {
	unresolved := []ports.Fact{
		{Subject: "py:pkg/mod", Attrs: map[string]string{"spec": "..a"}, Source: ports.FactSource{File: "pkg/mod.py", Line: 3}},
		{Subject: "py:pkg/mod", Attrs: map[string]string{"spec": "..b"}, Source: ports.FactSource{File: "pkg/mod.py", Line: 9}},
	}
	findings := detectBrokenImports(unresolved)
	require.Len(t, findings, 1)
	assert.Equal(t, RuleBrokenImport, findings[0].Attrs["rule"])
	assert.Equal(t, "2", findings[0].Attrs["count"])
	assert.Equal(t, "..a,..b", findings[0].Attrs["specs"])
}

func TestDetectBrokenImports_Empty(t *testing.T) {
	assert.Empty(t, detectBrokenImports(nil))
}

func TestIsEntrypoint(t *testing.T) {
	cases := map[string]bool{
		"go:cmd/aoa":        true,
		"go:cmd/aoa/cmd":    true,
		"py:scripts/main":   true,
		"ts:src/tests":      true,
		"go:internal/app":   false,
		"go:internal/ports": false,
		"py:pkg/mod":        false,
	}
	for id, want := range cases {
		assert.Equal(t, want, isEntrypoint(id), "isEntrypoint(%q)", id)
	}
}
