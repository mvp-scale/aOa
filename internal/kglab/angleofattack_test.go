package kglab

import (
	"testing"

	"github.com/corey/aoa/internal/domain/analyzer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ontology: the 16-concept registry is honest -----------------------------

func TestOntology_SixteenConcepts(t *testing.T) {
	assert.Len(t, Ontology, 16, "15 canonical + catch_clause")
}

func TestOntology_NamesMatchLangMap(t *testing.T) {
	// Every ontology name must be a real analyzer concept constant.
	valid := map[string]bool{
		analyzer.ConceptCall: true, analyzer.ConceptStringLiteral: true, analyzer.ConceptStringConcat: true,
		analyzer.ConceptAssignment: true, analyzer.ConceptForLoop: true, analyzer.ConceptDefer: true,
		analyzer.ConceptReturn: true, analyzer.ConceptImport: true, analyzer.ConceptFunction: true,
		analyzer.ConceptClass: true, analyzer.ConceptBlock: true, analyzer.ConceptSwitch: true,
		analyzer.ConceptFormatCall: true, analyzer.ConceptTypeAssertion: true, analyzer.ConceptInterface: true,
		analyzer.ConceptCatchClause: true,
	}
	for _, c := range Ontology {
		assert.True(t, valid[c.Name], "ontology concept %q must be a real analyzer constant", c.Name)
	}
}

func TestOntology_ImportIsOnlyWiredEdge(t *testing.T) {
	wired := 0
	for _, c := range Ontology {
		if c.Role == RoleEdge && c.Status == StatusWired {
			wired++
			assert.Equal(t, "import", c.Name, "the only wired edge is import")
		}
	}
	assert.Equal(t, 1, wired, "exactly one wired edge today (1/16)")
}

func TestEdgeKindHasSubstrate(t *testing.T) {
	assert.True(t, EdgeKindHasSubstrate("imports"), "wire token")
	assert.True(t, EdgeKindHasSubstrate("import"), "concept name (singular/plural collision resolved)")
	assert.False(t, EdgeKindHasSubstrate("calls"), "call is recon, not wired")
	assert.False(t, EdgeKindHasSubstrate("for_loop"), "property is never an edge")
	assert.False(t, EdgeKindHasSubstrate("frobnicator"), "unknown")
}

// --- concept-driven honesty gate (registry, not hardcoded) -------------------

func TestCompile_Gate_CallsRefused_Registry(t *testing.T) {
	units, deps := SampleGraphWithCalls()
	_, err := Compile(ViewQuery{
		Scope:    "s",
		Traverse: &TraverseSpec{Seed: "m_app", EdgeKind: "calls"},
		Render:   RenderSpec{Kind: "component"},
	}, units, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no substrate")
	assert.Contains(t, err.Error(), "recon-matched")
}

func TestCompile_Gate_PropertyRefused(t *testing.T) {
	units, deps := SampleGraph()
	_, err := Compile(ViewQuery{
		Scope:    "s",
		Traverse: &TraverseSpec{Seed: "m_app", EdgeKind: "for_loop"},
		Render:   RenderSpec{Kind: "component"},
	}, units, deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "property")
}

func TestBuildAdj_FiltersToKind(t *testing.T) {
	units, deps := SampleGraphWithCalls()
	imp := buildAdj(units, deps, "imports")
	cal := buildAdj(units, deps, "calls")
	assert.NotEqual(t, imp, cal, "different concepts yield different adjacency")
	assert.Len(t, cal["m_cmd_aoa"], 1, "one :calls edge from cmd")
}

// --- fact sets + drift: the Angle of Attack ----------------------------------

func TestFactSetFromDeps_MapsImportConcept(t *testing.T) {
	_, deps := SampleGraph()
	fs := FactSetFromDeps("real", deps)
	require.Equal(t, len(deps), fs.Len())
	for _, f := range fs.Facts {
		assert.Equal(t, "import", f.Concept)
		assert.Equal(t, ProvREAL, f.Prov)
		assert.NotEmpty(t, f.File, "REAL facts carry file:line")
	}
}

func TestLoadTarget_RejectsUnknownConcept(t *testing.T) {
	_, err := LoadTarget("t", []TargetFact{{Concept: "frobnicator", FromUnit: "a", ToUnit: "b"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown concept")
}

func TestLoadTarget_ProvDeclaredNoFileLine(t *testing.T) {
	fs, err := LoadTarget("t", SampleTarget())
	require.NoError(t, err)
	for _, f := range fs.Facts {
		assert.Equal(t, ProvDECLARED, f.Prov)
		assert.Empty(t, f.File, "DECLARED intent has no backing file:line")
	}
}

func TestDriftDiff_SampleGraphVsTarget(t *testing.T) {
	_, deps := SampleGraph()
	real := FactSetFromDeps("real", deps)
	target, err := LoadTarget("target", SampleTarget())
	require.NoError(t, err)
	res := DriftDiff(real, target)
	assert.Equal(t, 3, res.Violations, "the 3 forbidden cycle edges")
	assert.Equal(t, 1, res.Missing, "the declared-not-built edge")
	assert.Equal(t, res.Conformant+res.Violations+res.Missing, len(res.Items))
}

func TestDriftDiff_ViolationCarriesFileLine(t *testing.T) {
	_, deps := SampleGraph()
	real := FactSetFromDeps("real", deps)
	target, _ := LoadTarget("target", SampleTarget())
	res := DriftDiff(real, target)
	for _, it := range res.Items {
		if it.Alignment == AlignViolation {
			assert.NotEmpty(t, it.Fact.File, "a VIOLATION is actionable: real file:line")
		}
		if it.Alignment == AlignMissing {
			assert.Empty(t, it.Fact.File, "a MISSING has no real backing")
		}
	}
}

func TestDriftDiff_SortOrder(t *testing.T) {
	_, deps := SampleGraph()
	real := FactSetFromDeps("real", deps)
	target, _ := LoadTarget("target", SampleTarget())
	res := DriftDiff(real, target)
	// First items are VIOLATIONs, last are CONFORMANT.
	assert.Equal(t, AlignViolation, res.Items[0].Alignment)
	assert.Equal(t, AlignConformant, res.Items[len(res.Items)-1].Alignment)
}

// --- completeness ledger -----------------------------------------------------

func TestLedger_ExactlyOneWired(t *testing.T) {
	l := BuildLedger(DefaultEntries(), "")
	assert.Equal(t, 1, wiredEdgeCount(l.Entries), "1/16 baseline — CI trip-wire if a status flips without a pipeline")
	assert.Len(t, l.Entries, 16)
}

func TestLedger_RevDeterministic(t *testing.T) {
	a := BuildLedger(DefaultEntries(), "")
	b := BuildLedger(DefaultEntries(), "")
	assert.Equal(t, a.Rev, b.Rev)
	assert.Len(t, a.Rev, 12)
}

func TestDiffLedger_DetectsStatusFlip(t *testing.T) {
	prev := BuildLedger(DefaultEntries(), "")
	next := DefaultEntries()
	next[1].Status = StatusWired // flip "call" to wired
	nl := BuildLedger(next, prev.Rev)
	assert.NotEqual(t, prev.Rev, nl.Rev, "rev changes on status flip")
	assert.Contains(t, DiffLedger(prev, nl), "call")
}

// --- agent-agnostic contract -------------------------------------------------

func TestComputeDrift_HasViolationsAndMissing(t *testing.T) {
	r := ComputeDrift()
	assert.True(t, r.OK)
	assert.Positive(t, r.Result.Violations)
	assert.Positive(t, r.Result.Missing)
	assert.Contains(t, r.Caption, "import-only", "honest: drift is import-only today")
}

func TestRenderResponseJSON_DeterministicNoEscape(t *testing.T) {
	r := ComputeDrift()
	b1, err := RenderResponseJSON(r)
	require.NoError(t, err)
	b2, _ := RenderResponseJSON(r)
	assert.Equal(t, string(b1), string(b2), "deterministic")
	assert.NotContains(t, string(b1), `<`, "no HTML escaping (mirrors MarshalShard)")
}
