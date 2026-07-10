package learner

import "testing"

// M2.2 — the shared election. These oracles pin the behavior grep and the graph
// both depend on.

func TestBuildWinnerMap_ElectsHighestPerTerm(t *testing.T) {
	// "execution" bound to state_machine (180) over graphql (30); total 210 >= floor.
	cohit := map[string]uint32{
		"execution:state_machine": 180,
		"execution:graphql":       30,
	}
	winners := BuildWinnerMap(cohit, 100)
	if got := winners["execution"]; got != "state_machine" {
		t.Fatalf("execution winner = %q, want state_machine", got)
	}
}

// The runDedup gap: a single-domain term survives dedup at count=1 (dedup only
// contests 2+ domains). The floor must reject it — else count=1 noise looks like a winner.
func TestBuildWinnerMap_FloorsSingleDomainNoise(t *testing.T) {
	cohit := map[string]uint32{
		"foo:animation": 1, // never contested, survives dedup, but total 1 < floor
	}
	winners := BuildWinnerMap(cohit, 100)
	if _, ok := winners["foo"]; ok {
		t.Fatalf("term below the floor must NOT be elected, got winner %q", winners["foo"])
	}
}

func TestBuildWinnerMap_TieBreakLexicographic(t *testing.T) {
	cohit := map[string]uint32{
		"api:zebra": 60,
		"api:alpha": 60, // equal counts, total 120 >= floor → lexicographic winner "alpha"
	}
	winners := BuildWinnerMap(cohit, 100)
	if got := winners["api"]; got != "alpha" {
		t.Fatalf("tie winner = %q, want alpha (lexicographic)", got)
	}
}

func TestBuildWinnerMap_Deterministic(t *testing.T) {
	cohit := map[string]uint32{
		"execution:state_machine": 180, "execution:graphql": 30,
		"api:alpha": 60, "api:zebra": 60,
	}
	a := BuildWinnerMap(cohit, 100)
	b := BuildWinnerMap(cohit, 100)
	if len(a) != len(b) {
		t.Fatalf("nondeterministic size: %d vs %d", len(a), len(b))
	}
	for k, v := range a {
		if b[k] != v {
			t.Fatalf("nondeterministic winner for %q: %q vs %q", k, v, b[k])
		}
	}
}
