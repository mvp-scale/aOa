// FDN-3 (board #29): the compactor. Turns raw dep Facts (file grain, Object
// empty, Attrs["spec"] the literal specifier — emitted by
// app.BuildIndexWithFactsAndSink/importEdgeToFact) into the resolved facts
// substrate: unit records, a DepAdjacency posting-list graph, and
// compact-time finding facts (detect.go).
//
// Compact reuses resolve.go's per-edge resolution machinery UNCHANGED (it is
// STABLE law — see resolve.go's own doc comment): resolveOne/buildDirSet/
// langFromPath are unexported but in this same package, so Compact calls
// them directly instead of round-tripping through the batch-oriented
// Resolve() entry point. That round trip would lose the correlation between
// an edge and the raw Fact's Subject (ImportEdge carries no Subject field),
// so Compact drives resolution per-Fact instead, preserving Subject
// end-to-end (§2.4, §3).
package facts

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/ports"
)

// FactsSchemaVersion stamps the compactor's output shape (units + adjacency
// + findings) so App-boot can detect a stale/incompatible compaction and
// force a full recompact — mirrors the D14/t64 arch-shard freshness pattern
// (internal/app/arch.go hasLocalArchManifest, arch.ArchSchemaVersion).
// Deliberately a SEPARATE constant from arch.ArchSchemaVersion: the facts
// substrate and the arch shard cache are versioned independently (facts
// feeds arch, not the reverse; bumping one must never force a spurious
// re-derive of the other).
const FactsSchemaVersion = 1

// Compact is the compactor entry point (§2.4/§3). raw is every currently
// known FactDep fact for the project (facts_raw, file grain); root is the
// project root (used only to read build manifests — go.mod, etc. — via
// ReadManifests); fileSet is the current Index.Files path set (needed by
// resolve.go's resolution rules, identical to the legacy resolveEdgeBatch
// input).
//
// Returns:
//   - units:    one FactUnit fact per distinct unit seen, subject side and/or
//     resolved intra-repo dependency-target side (see subjectFromResolvedPath)
//   - adj:      the resolved DepAdjacency (forward + reverse posting lists),
//     ready for ports.FactStore.PutResolved
//   - findings: FactFinding facts from the compact-time detectors (detect.go)
//
// Pure aside from ReadManifests(root) (a bounded go.mod walk — the same cost
// the legacy resolveEdgeBatch path already pays every debounce window).
func Compact(raw []ports.Fact, root string, fileSet map[string]bool) (units []ports.Fact, adj *ports.DepAdjacency, findings []ports.Fact) {
	manifests := ReadManifests(root)
	return CompactWithManifests(raw, fileSet, manifests)
}

// CompactWithManifests is Compact with pre-read manifests — the pure,
// dependency-injectable core (unit-testable without touching the
// filesystem).
func CompactWithManifests(raw []ports.Fact, fileSet map[string]bool, manifests Manifests) (units []ports.Fact, adj *ports.DepAdjacency, findings []ports.Fact) {
	dirSet := buildDirSet(fileSet)

	unitByID := make(map[string]ports.Fact)
	// bestEffort tracks which units currently carry a *provisional* source
	// (stamped only because the unit was first met as a dependency
	// *target*, never yet as the Subject of its own raw dep fact — see the
	// ensureUnit(obj, ...) call below). Raw-fact visitation order is not
	// guaranteed to meet a unit's own subject-owning fact before every fact
	// that merely references it as a target, so a later genuine subject
	// fact must be allowed to upgrade a unit's source even though the unit
	// entry already exists; only a second *best-effort* sighting is a
	// true no-op.
	bestEffort := make(map[string]bool)
	order := make([]string, 0, 64)
	ensureUnit := func(id string, src ports.FactSource, ts int64, provisional bool) {
		if id == "" || strings.HasPrefix(id, "ext:") {
			return
		}
		if existing, ok := unitByID[id]; ok {
			if bestEffort[id] && !provisional {
				// A real subject-owning source has arrived for a unit that
				// so far only had a best-effort placeholder — replace it.
				existing.Source = src
				existing.TS = ts
				unitByID[id] = existing
				bestEffort[id] = false
			}
			return
		}
		unitByID[id] = ports.Fact{
			Kind:    ports.FactUnit,
			Subject: id,
			Source:  src,
			Prov:    ports.ProvDerived,
			TS:      ts,
		}
		bestEffort[id] = provisional
		order = append(order, id)
	}

	fwdCount := make(map[string]map[string]int)
	revCount := make(map[string]map[string]int)
	var unresolved []ports.Fact

	for _, f := range raw {
		if f.Kind != ports.FactDep {
			continue // compact-time synthesis operates on dep facts only (§2.1)
		}
		subj := f.Subject
		if subj != "" {
			ensureUnit(subj, f.Source, f.TS, false)
		}

		edge := ports.ImportEdge{FromFile: f.Source.File, ImportPath: f.Attrs["spec"], StartLine: f.Source.Line}
		lang := langFromPath(edge.FromFile)
		resolved, ok := resolveOne(edge, lang, fileSet, dirSet, manifests)
		if !ok {
			unresolved = append(unresolved, f)
			continue
		}

		obj := subjectFromResolvedPath(lang, resolved.ImportPath)
		if obj == "" || obj == subj {
			continue // no phantom nodes, no self-loops
		}
		if !strings.HasPrefix(obj, "ext:") {
			// Best-effort source for a unit only ever seen as a dependency
			// target (never itself the Subject of a raw dep fact) — e.g. a
			// leaf package that imports nothing. Source is the edge's own
			// file, which is the only provenance pointer we have (§2.1
			// "no single canonical line for a unit").
			ensureUnit(obj, ports.FactSource{File: resolved.FromFile}, f.TS, true)
		}
		if subj == "" {
			continue // cannot attribute an edge without an owning unit
		}

		if fwdCount[subj] == nil {
			fwdCount[subj] = make(map[string]int)
		}
		fwdCount[subj][obj]++
		if revCount[obj] == nil {
			revCount[obj] = make(map[string]int)
		}
		revCount[obj][subj]++
	}

	sort.Strings(order)
	units = make([]ports.Fact, 0, len(order))
	for _, id := range order {
		units = append(units, unitByID[id])
	}

	adj = &ports.DepAdjacency{
		Fwd: countsToAdjacency(fwdCount),
		Rev: countsToAdjacency(revCount),
	}

	findings = Detect(units, adj, unresolved)
	return units, adj, findings
}

// countsToAdjacency turns a subject->object->count map into the sorted
// []DepEdge posting lists PutResolved/encodeDepEdges expect.
func countsToAdjacency(counts map[string]map[string]int) map[string][]ports.DepEdge {
	if len(counts) == 0 {
		return nil
	}
	out := make(map[string][]ports.DepEdge, len(counts))
	for subj, targets := range counts {
		edges := make([]ports.DepEdge, 0, len(targets))
		for obj, cnt := range targets {
			if cnt > 65535 {
				cnt = 65535 // DepEdge.Count is uint16 — defensive cap, never hit in practice
			}
			edges = append(edges, ports.DepEdge{Unit: obj, Count: uint16(cnt)})
		}
		sort.Slice(edges, func(i, j int) bool { return edges[i].Unit < edges[j].Unit })
		out[subj] = edges
	}
	return out
}

// subjectFromResolvedPath maps a resolve.go-resolved import target — a Go
// package directory, or a Python/JS/TS file path — into the same subject-ID
// namespace app.factSubjectForFile (indexer.go) uses for the importing
// file's own unit id, so a compacted edge's Object always matches the
// Subject some other raw fact would have produced for that same file. This
// duplicates factSubjectForFile's three-bucket (go/py/ts) mapping rather
// than importing internal/app: domain/facts must stay dependency-free
// (hexagonal law, CLAUDE.md) and internal/app already imports this package,
// so the reverse import is not an option.
// "ext:" targets pass through unchanged (already correctly prefixed by
// resolve.go).
func subjectFromResolvedPath(lang, resolved string) string {
	if resolved == "" || strings.HasPrefix(resolved, "ext:") {
		return resolved
	}
	switch lang {
	case "go":
		return "go:" + resolved
	case "python":
		return "py:" + strings.TrimSuffix(resolved, filepath.Ext(resolved))
	case "javascript", "typescript":
		return "ts:" + strings.TrimSuffix(resolved, filepath.Ext(resolved))
	default:
		return "file:" + resolved
	}
}
