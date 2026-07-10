package app

// Noise eviction — the "commonplace → useless → remove" mechanism, tested at the join.
//
// autotune Step 1 ("prune terms in >30% of indexed files") is DISABLED in Go because
// the learner doesn't own the file index. The Level-D join DOES own idx, so the
// commonplace filter can live here: a term that tags a large fraction of the units
// carries no discriminating signal (TF-IDF intuition) and must not mint learned edges,
// no matter how high its cohit count. A concentrated term still does.
//
// Red-first: today learnedAffinityEdges has NO commonplace filter, so an over-common
// term wrongly produces edges. These tests pin the threshold once the filter exists.

import (
	"fmt"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
)

// projectWithTermSpread builds N units. Unit u00 is the DEDICATED domain carrier
// (@state_machine) and is never tagged, so a term→domain edge is always cross-unit
// (no within-unit self-edge confound). The term "execution" spreads across `spread`
// units (u01..u{spread}); the term "transition" tags exactly the last unit (rare).
func projectWithTermSpread(nUnits, spread int) *ports.Index {
	idx := &ports.Index{
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{},
		Files:    map[uint32]*ports.FileMeta{},
	}
	var fid uint32 = 1
	for u := 0; u < nUnits; u++ {
		path := fmt.Sprintf("u%02d/f.go", u)
		dom := "@scheduling"
		if u == 0 {
			dom = "@state_machine" // dedicated domain carrier, never tagged
		}
		idx.Files[fid] = &ports.FileMeta{Path: path, Language: "go", Domain: dom}
		tags := []string{}
		if u >= 1 && u <= spread {
			tags = append(tags, "execution") // spreads across `spread` non-carrier units
		}
		if u == nUnits-1 {
			tags = append(tags, "transition") // rare: one unit only
		}
		if len(tags) > 0 {
			idx.Metadata[ports.TokenRef{FileID: fid, Line: 10}] = &ports.SymbolMeta{Name: "S", Tags: tags}
		}
		fid++
	}
	return idx
}

// TestNoise_CommonplaceTermSuppressed: a term tagging most units is noise → no learned
// edges from it, even with a strong cohit. This FAILS today (no commonplace filter).
func TestNoise_CommonplaceTermSuppressed(t *testing.T) {
	// 10 units; "execution" tags 9 of 10 (90% — far past any sane commonplace floor).
	idx := projectWithTermSpread(10, 9)
	learned := map[string]uint32{"execution:state_machine": 500}

	edges := learnedAffinityEdges(idx, learned)

	// The commonplace term must mint NO edges.
	assert.Empty(t, edges,
		"a term tagging 90%% of units is commonplace/noise — it must mint no learned edges regardless of cohit strength")
}

// TestNoise_ConcentratedTermKept: a term in a single unit is a real, discriminating
// signal — it still mints its edge. Guards against the filter over-suppressing.
func TestNoise_ConcentratedTermKept(t *testing.T) {
	idx := projectWithTermSpread(10, 9)
	// "transition" tags only the last unit (u09); bind it to @state_machine (u00).
	learned := map[string]uint32{"transition:state_machine": 500}

	edges := learnedAffinityEdges(idx, learned)

	assert.True(t, hasEdge(edges, unitSlug("u09"), unitSlug("u00"), "mixed"),
		"a term concentrated in one unit is signal, not noise — its learned edge must survive")
}

// TestNoise_ThresholdCutoff pins the commonplace boundary: a term spread across a
// fraction ABOVE the threshold is suppressed; at or below it survives. This is the
// tunable that defines "when a term becomes noise" — swept via affinityThresholds.
func TestNoise_ThresholdCutoff(t *testing.T) {
	const nUnits = 20
	learned := map[string]uint32{"execution:state_machine": 500}
	thr := func(frac float64) affinityThresholds {
		return affinityThresholds{MinCohit: 100, MaxUnitFraction: frac, MinUnitsForCommonplace: 5}
	}
	edgeCount := func(spread int, frac float64) int {
		return len(learnedAffinityEdgesTuned(projectWithTermSpread(nUnits, spread), learned, thr(frac)))
	}

	// At a 30% threshold: 6/20 (30%) survives (not strictly greater); 7/20 (35%) is noise.
	assert.Positive(t, edgeCount(6, 0.30), "spread == threshold (30%) is kept")
	assert.Zero(t, edgeCount(7, 0.30), "spread just over threshold (35%) is evicted as noise")

	// The cutoff moves with the knob: at 50%, 10/20 survives, 11/20 is noise.
	assert.Positive(t, edgeCount(10, 0.50), "at a 50% threshold, 50% spread is kept")
	assert.Zero(t, edgeCount(11, 0.50), "at a 50% threshold, 55% spread is evicted")
}
