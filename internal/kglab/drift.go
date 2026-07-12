package kglab

import "sort"

// drift.go — the Angle of Attack itself: the VECTOR between where-we-are (REAL)
// and where-we-need-to-be (TARGET). Drift is a pure set-diff of two fact sets,
// no new engine.
//
// Alignment semantics (honest, documented):
//   CONFORMANT — fact present in BOTH real and target.
//   VIOLATION  — fact present in REAL, absent from TARGET (real drifted from intent).
//                Carries the REAL file:line, so it is actionable.
//   MISSING    — fact present in TARGET, absent from REAL (declared, not yet built).
//                No file:line — it does not exist yet.
// NOTE: VIOLATION means "in real, not in target"; target is silent on
// forbidden-vs-unspecified. An explicit ForbidSet is deferred.

type Alignment string

const (
	AlignConformant Alignment = "CONFORMANT"
	AlignViolation  Alignment = "VIOLATION"
	AlignMissing    Alignment = "MISSING"
)

// DriftItem is one aligned fact.
type DriftItem struct {
	Alignment Alignment   `json:"alignment"`
	Fact      ConceptFact `json:"fact"`
}

// DriftResult is the full alignment between two worlds.
type DriftResult struct {
	RealName    string      `json:"real"`
	TargetName  string      `json:"target"`
	Items       []DriftItem `json:"items"`
	Conformant  int         `json:"conformant"`
	Violations  int         `json:"violations"`
	Missing     int         `json:"missing"`
}

// DriftDiff computes the alignment of real against target. O(n+m).
func DriftDiff(real, target FactSet) DriftResult {
	targetKeys := make(map[string]bool, len(target.Facts))
	for _, f := range target.Facts {
		targetKeys[f.key()] = true
	}
	realKeys := make(map[string]bool, len(real.Facts))
	for _, f := range real.Facts {
		realKeys[f.key()] = true
	}

	res := DriftResult{RealName: real.Name, TargetName: target.Name}
	for _, f := range real.Facts {
		if targetKeys[f.key()] {
			res.Items = append(res.Items, DriftItem{AlignConformant, f})
			res.Conformant++
		} else {
			res.Items = append(res.Items, DriftItem{AlignViolation, f})
			res.Violations++
		}
	}
	for _, f := range target.Facts {
		if !realKeys[f.key()] {
			res.Items = append(res.Items, DriftItem{AlignMissing, f})
			res.Missing++
		}
	}

	// Sort VIOLATION < MISSING < CONFORMANT, then by key for determinism.
	rank := map[Alignment]int{AlignViolation: 0, AlignMissing: 1, AlignConformant: 2}
	sort.SliceStable(res.Items, func(i, j int) bool {
		ri, rj := rank[res.Items[i].Alignment], rank[res.Items[j].Alignment]
		if ri != rj {
			return ri < rj
		}
		return res.Items[i].Fact.key() < res.Items[j].Fact.key()
	})
	return res
}
