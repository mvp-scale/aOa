// FDN-4 (board #30): the FactStore → arch bridge. Converts the compacted
// facts substrate (ports.Fact{Kind:unit} + ports.DepAdjacency, FDN-1/FDN-3)
// into the same []arch.UnitFact/[]arch.DepFact shape aggregateEdges (arch.go)
// has always produced from raw EdgeStore edges, so deriveArch can read the
// FactStore query plane instead of LoadAllEdges+aggregateEdges (D25: adapt at
// the boundary rather than reconciling UnitFact/DepFact's own field shape —
// RenderAll's pure input contract, and every existing arch/*_test.go golden,
// stay untouched).
//
// Namespace mismatch this bridge exists to close: the compactor
// (internal/domain/facts/compactor.go) and factSubjectForFile (indexer.go)
// mint subject IDs as "<lang-ns>:<path>" — "go:internal/app" (directory
// grain), "py:pkg/mod" / "ts:web/app" (FILE grain, extension stripped), or
// "ext:<spec>" passthrough. aggregateEdges' unit grain is uniformly the
// DIRECTORY of the importing file, slugged via arch.UnitSlug, regardless of
// language. factSubjectToUnitPath below collapses every namespace to that
// same directory-grain path string; unitFactsFromFactStore re-keys through
// arch.UnitSlug (via the app-package unitSlug shim) so a FactStore-derived
// unit ID always matches what aggregateEdges would have minted for the same
// underlying file.
package app

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
)

// factSubjectToUnitPath converts a FactStore subject (or a DepEdge.Unit,
// same namespace, D7) into the directory-grain path string aggregateEdges
// derives from filepath.Dir(ImportEdge.FromFile)/ImportEdge.ImportPath.
//
//   - "ext:<spec>"        → passthrough unchanged (aggregateEdges never
//     collapses external import specs to a directory — they ARE the toPath).
//   - "go:<dir>"          → already directory grain; "" (repo root) → "root".
//   - "py:<file-no-ext>"  → collapsed to filepath.Dir(file) ("" → "root"):
//     the compactor mints Python/TS subjects at FILE grain, but arch's unit
//     grain is uniformly directory — multiple files in one directory must
//     collapse onto the same unit.
//   - "ts:<file-no-ext>"  → same collapse as "py:".
//   - "file:<path>"       → same collapse (the unclassified-language fallback).
//   - anything else (no "ns:" separator) → returned as-is.
func factSubjectToUnitPath(subject string) string {
	if subject == "" {
		return "root"
	}
	if strings.HasPrefix(subject, "ext:") {
		return subject
	}
	i := strings.Index(subject, ":")
	if i < 0 {
		return subject
	}
	ns, rest := subject[:i], subject[i+1:]
	switch ns {
	case "go":
		if rest == "" {
			return "root"
		}
		return rest
	case "py", "ts", "file":
		dir := filepath.ToSlash(filepath.Dir(rest))
		if dir == "." || dir == "" {
			return "root"
		}
		return dir
	default:
		return rest
	}
}

// unitFactsFromFactStore converts the compactor's output — unit facts
// (ports.Fact{Kind:FactUnit}) plus the resolved forward adjacency
// (ports.DepAdjacency.Fwd) — into []arch.UnitFact/[]arch.DepFact, the exact
// shape aggregateEdges (arch.go) has always fed into arch.Service.RenderAll.
//
// idx may be nil (C4/headless, same contract as aggregateEdges); when
// non-nil, file Domain fields from the index populate UnitFact.Domain for
// rung-3 grouping — same fileDomains lookup aggregateEdges performs.
//
// Known divergence from aggregateEdges (recorded, not silent): the
// compactor's resolved adjacency (ports.DepEdge) carries only a unit +
// count — no per-edge file:line, unlike the raw ImportEdge stream
// aggregateEdges consumes. DepFact.File/Line here is therefore the FROM
// unit's own representative source pointer (its ports.Fact.Source — a real,
// non-fabricated G7 pointer into the correct file, just at unit grain
// instead of edge grain) rather than the specific import statement's line.
//
// Self-loops (fromID == toID) are dropped and output is sorted by ID / by
// (from, to) — both match aggregateEdges' byte-determinism contract.
func unitFactsFromFactStore(units []ports.Fact, adj *ports.DepAdjacency, idx *ports.Index) ([]arch.UnitFact, []arch.DepFact) {
	fileDomains := make(map[string]string)
	if idx != nil {
		for _, fm := range idx.Files {
			if fm.Domain != "" {
				fileDomains[fm.Path] = fm.Domain
			}
		}
	}

	// resolve memoizes subject → (unitID, path, unit source) so every unit
	// fact AND every dep-edge endpoint routes through the identical
	// subject→path→ID pipeline — IDs agree by construction, exactly the
	// property aggregateEdges gets from calling unitSlug on one shared
	// (fromDir/toPath) computation.
	pathOf := make(map[string]string, len(units))
	resolvePath := func(subject string) string {
		if p, ok := pathOf[subject]; ok {
			return p
		}
		p := factSubjectToUnitPath(subject)
		pathOf[subject] = p
		return p
	}

	unitMap := make(map[string]*arch.UnitFact, len(units))
	order := make([]string, 0, len(units))
	addUnit := func(path, file string, line uint32, domain string) string {
		id := unitSlug(path)
		if existing, ok := unitMap[id]; ok {
			if existing.Domain == "" && domain != "" {
				existing.Domain = domain
			}
			return id
		}
		unitMap[id] = &arch.UnitFact{
			ID:     id,
			Label:  unitLabel(path),
			Path:   path,
			File:   file,
			Line:   line,
			Domain: domain,
		}
		order = append(order, id)
		return id
	}

	for _, u := range units {
		if strings.HasPrefix(u.Subject, "ext:") {
			continue // ext units are minted lazily below, only when actually
			// referenced by a dep edge — matches aggregateEdges (which never
			// sees an "ext:" unit except as an import target).
		}
		addUnit(resolvePath(u.Subject), u.Source.File, u.Source.Line, fileDomains[u.Source.File])
	}

	type depKey struct{ from, to string }
	depMap := make(map[depKey]*arch.DepFact)

	var fromSubjects []string
	if adj != nil {
		for subj := range adj.Fwd {
			fromSubjects = append(fromSubjects, subj)
		}
	}
	sort.Strings(fromSubjects) // deterministic edge-visitation order (T4)

	for _, subj := range fromSubjects {
		fromPath := resolvePath(subj)
		fromID := unitSlug(fromPath)
		// The subject's own unit should already exist from the loop above
		// (every FactDep-owning subject is unit-ensured by the compactor);
		// addUnit is idempotent so this is a defensive no-op fallback for a
		// from-subject the unit scan somehow missed (never observed in
		// practice — the compactor always calls ensureUnit(subj, ...) for
		// every raw dep fact's Subject).
		if _, ok := unitMap[fromID]; !ok {
			fromID = addUnit(fromPath, "", 0, "")
		}

		for _, e := range adj.Fwd[subj] {
			toPath := resolvePath(e.Unit)
			toID := unitSlug(toPath)
			if _, ok := unitMap[toID]; !ok {
				addUnit(toPath, "", 0, "")
			}
			if fromID == toID {
				continue // self-loop (same directory imports itself) — dropped
			}
			key := depKey{fromID, toID}
			if dv, ok := depMap[key]; ok {
				dv.Count += int(e.Count)
			} else {
				fromUnit := unitMap[fromID]
				depMap[key] = &arch.DepFact{
					FromUnit: fromID,
					ToUnit:   toID,
					Count:    int(e.Count),
					File:     fromUnit.File,
					Line:     fromUnit.Line,
				}
			}
		}
	}

	sort.Strings(order)
	outUnits := make([]arch.UnitFact, 0, len(order))
	for _, id := range order {
		outUnits = append(outUnits, *unitMap[id])
	}

	rows := make([]depKey, 0, len(depMap))
	for k := range depMap {
		rows = append(rows, k)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].from != rows[j].from {
			return rows[i].from < rows[j].from
		}
		return rows[i].to < rows[j].to
	})
	outDeps := make([]arch.DepFact, len(rows))
	for i, k := range rows {
		outDeps[i] = *depMap[k]
	}

	return outUnits, outDeps
}
