// GOV-1 (board #40): the app-boundary bridge between the FactStore substrate
// and kglab's Angle-of-Attack drift engine (internal/kglab/drift.go). Lives
// here, not in kglab, because D19 keeps kglab self-contained (no bbolt, no
// daemon, no internal/app import) — bridges belonging to a daemon/store
// boundary are internal/app's job, mirroring the FDN-4 precedent
// (arch_factstore_bridge.go's unitFactsFromFactStore).
package app

import (
	"crypto/sha256"
	"fmt"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/kglab"
	"github.com/corey/aoa/internal/ports"
)

// RealDepFactsFromStore replays deriveArch's steps 1/1b/3 — FactsByKind(unit)
// + per-unit Dependencies + unitFactsFromFactStore — against ANY
// ports.FactStore (the live daemon's a.Store, or a read-only bbolt.Store
// opened by the CLI's direct-RO fallback, cmd/aoa/cmd/arch_drift.go), and
// returns the same []arch.DepFact deriveArch computes. One derivation path
// for both callers — avoids double-deriving the real substrate (a risk
// flagged in the GOV-1 scout brief).
//
// idx may be nil (no UnitFact.Domain enrichment; same nil-idx contract
// unitFactsFromFactStore already documents) — the CLI direct-RO path never
// has a live *ports.Index, matching Derive/Facts/Graph's existing convention
// (cmd/aoa/cmd/arch.go's cliArchQuerier methods).
func RealDepFactsFromStore(store ports.FactStore, projectID string, idx *ports.Index) ([]arch.DepFact, error) {
	unitFacts, err := store.FactsByKind(projectID, ports.FactUnit)
	if err != nil {
		return nil, err
	}
	if len(unitFacts) == 0 {
		return nil, nil
	}

	fwd := make(map[string][]ports.DepEdge, len(unitFacts))
	for _, u := range unitFacts {
		depEdges, derr := store.Dependencies(projectID, u.Subject)
		if derr != nil {
			continue // best-effort — mirrors deriveArch's per-unit skip-on-error
		}
		if len(depEdges) > 0 {
			fwd[u.Subject] = depEdges
		}
	}
	adj := &ports.DepAdjacency{Fwd: fwd}

	_, deps := unitFactsFromFactStore(unitFacts, adj, idx)
	return deps, nil
}

// DriftViolationFindings converts a kglab.DriftResult's VIOLATION items into
// []ports.Finding — the same DTO archFindingsToPortsFindings produces for
// detector output — so a drift check rides the existing findings shape and
// the `arch findings --new` baseline convention (GOV-1 bullet b) instead of
// inventing a parallel finding type.
//
// Only VIOLATIONs convert: they carry a real file:line (actionable, D2
// provenance-honest). MISSING has no backing fact yet — nothing real to point
// an agent at — and CONFORMANT is not a finding at all.
func DriftViolationFindings(scope string, result kglab.DriftResult) []ports.Finding {
	var out []ports.Finding
	for _, it := range result.Items {
		if it.Alignment != kglab.AlignViolation {
			continue
		}
		f := it.Fact
		out = append(out, ports.Finding{
			ID:       driftFindingID(scope, f.Concept, f.FromUnit, f.ToUnit),
			Rule:     "drift-violation",
			Severity: "error",
			Scope:    scope,
			Message:  fmt.Sprintf("%s: real %s -> %s is not declared in target %q", f.Concept, f.FromUnit, f.ToUnit, result.TargetName),
			Subjects: []string{f.FromUnit, f.ToUnit},
			Sources:  []ports.SourceRef{{File: f.File, Line: f.Line}},
		})
	}
	return out
}

// driftFindingID mints a stable, content-addressed ID (same sha256-prefix
// recipe as internal/domain/arch/detect.go's findingID; re-implemented
// locally since that helper is unexported in another package).
func driftFindingID(scope, concept, from, to string) string {
	h := sha256.New()
	h.Write([]byte("drift-violation|"))
	h.Write([]byte(scope))
	h.Write([]byte("|"))
	h.Write([]byte(concept))
	h.Write([]byte("|"))
	h.Write([]byte(from))
	h.Write([]byte("|"))
	h.Write([]byte(to))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
