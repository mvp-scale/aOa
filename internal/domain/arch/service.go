package arch

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// viewDef pairs a view ID with the render function that produces its Shard.
type viewDef struct {
	id string
	fn func(RenderInput) (*Shard, error)
}

// mandatoryViewDefs are the views RenderAll always registers, regardless of
// optional inputs (unlike "code", which is present only when symbolIndex !=
// nil). This is the single source of truth for both RenderAll's render loop
// and MandatoryViewIDs' app-layer freshness contract below — one list, so
// adding a view here can never silently diverge from what the boot-freshness
// check expects to find.
var mandatoryViewDefs = []viewDef{
	{"capability", RenderCapability},
	{"component", RenderComponent},
	{"context", RenderContext},
	{"cycles", RenderCycles},
	{"dsm", RenderDSM},
}

// MandatoryViewIDs returns the view IDs RenderAll always renders (excludes
// the conditional "code" view). The app layer's boot-freshness check
// (hasLocalArchManifest) uses this to detect a persisted manifest that is
// missing a view the CURRENT binary would always render — e.g. right after
// a new view type is added to mandatoryViewDefs. That case does not bump
// ArchSchemaVersion (reserved for JSON *shape* changes, not view-set
// additions — see ArchSchemaVersion's doc comment), so the schema-version
// gate alone lets a stale manifest missing the new view survive indefinitely
// across restarts (root cause: labs serving pre-phase shards forever once a
// new view ships, VP-2 gap on the same class of bug T64/PA2 fixed for shape
// drift). Order matches mandatoryViewDefs; callers should compare as sets.
func MandatoryViewIDs() []string {
	ids := make([]string, len(mandatoryViewDefs))
	for i, vd := range mandatoryViewDefs {
		ids[i] = vd.id
	}
	return ids
}

// Service orchestrates the arch domain: it accepts pre-loaded unit and dep facts,
// runs detectors + grouping + renderers, and returns serialized shards + manifest.
//
// The Service is dependency-free (no I/O, no bbolt, no cobra). All inputs are
// passed in; all outputs are returned. The app layer handles file I/O and storage.
//
// Thread safety: Service itself is stateless; callers may share it across goroutines.
type Service struct{}

// RenderAll runs the full derive pipeline for one scope:
//  1. Group units via GroupWithOptions (rung-1/2/3 + overlays).
//  2. Detect cycles/gods/orphans/budget/dead-candidate/mutual (Tarjan SCC
//     shared with cycles renderer; budget/mutual need the grouping).
//  3. Render component, dsm, cycles, and (when symbolIndex != nil) code views.
//  4. Marshal each shard to JSON and compute its 12-char ContentHash.
//  5. Build a byte-stable Manifest.
//
// refHits maps unit ID → index reference hits (dead-candidate fuel); nil is
// valid when no index is available — all units are then treated as 0 hits.
//
// symbolIndex carries per-file symbol data for the code renderer (②b, L19.23).
// Nil → code view omitted (never a phantom shard). Supplied by the app layer
// from a Clone of ports.Index (never the live pointer — race gate).
//
// Returns:
//   - shards: view ID → raw JSON bytes (ready to write to arch_shards bucket)
//   - manifest: catalog of all rendered views (byte-stable)
//   - findings: combined detector + overlay-leash findings
//   - error: any render/marshal error
//
// C1 context: this method does no writes; callers snapshot-release-write per C1.
func (s *Service) RenderAll(scope string, units []UnitFact, deps []DepFact, opts *GroupOptions, refHits map[string]int, symbolIndex *CodeSymbolIndex) (
	shards map[string][]byte,
	manifest Manifest,
	findings []Finding,
	err error,
) {
	// 1. Group units (rung-1/2/3 + overlays). Collect overlay-leash warnings.
	grouping, groupProv, leashWarnings := GroupWithOptions(units, opts)

	// 2. Detect: cycles + gods + orphans + budget + dead-candidate + mutual.
	// SCCs reused by cycles renderer.
	detectedFindings, sccs := Detect(scope, units, deps, DefaultThresholds(), grouping, refHits)
	findings = append(findings, detectedFindings...)
	findings = append(findings, leashWarnings...)

	// 3. Build shared render input.
	in := RenderInput{
		Scope:       scope,
		Units:       units,
		Deps:        deps,
		Grouping:    grouping,
		GroupProv:   groupProv,
		SCCs:        sccs,
		Findings:    findings,
		CodeSymbols: symbolIndex,
	}

	// 4. Render each view.
	viewDefs := append([]viewDef(nil), mandatoryViewDefs...)
	// Code view (②b, L19.23): registered only when symbol data is present.
	// Nil symbolIndex → view absent from manifest (never a phantom shard).
	if symbolIndex != nil {
		viewDefs = append(viewDefs, viewDef{"code", RenderCode})
	}

	shards = make(map[string][]byte, len(viewDefs))
	viewEntries := make([]ViewEntry, 0, len(viewDefs))

	for _, vd := range viewDefs {
		shard, renderErr := vd.fn(in)
		if renderErr != nil {
			err = fmt.Errorf("render %s: %w", vd.id, renderErr)
			return
		}
		data, marshalErr := MarshalShard(shard)
		if marshalErr != nil {
			err = fmt.Errorf("marshal %s: %w", vd.id, marshalErr)
			return
		}
		hash := ContentHash(data)
		shards[vd.id] = data
		viewEntries = append(viewEntries, ViewEntry{
			ID:      vd.id,
			Key:     scope + "/" + vd.id + "@" + hash,
			Hash:    hash,
			Caption: shard.Count,
			Prov:    shard.Prov.Kind,
		})
	}

	// Sort view entries alphabetically by ID for byte-stability.
	sort.Slice(viewEntries, func(i, j int) bool {
		return viewEntries[i].ID < viewEntries[j].ID
	})

	// 5. Compute Rev = 12-char hash of sorted unit IDs + dep signatures.
	rev := factsHash(units, deps)

	manifest = Manifest{
		Scope:         scope,
		Rev:           rev,
		SchemaVersion: ArchSchemaVersion,
		Views:         viewEntries,
	}
	return
}

// factsHash produces a 12-char content hash of the unit+dep fact set.
// Order-independent: unit IDs are sorted; dep signatures are sorted.
// Changes when the fact set changes; stable across renders of the same input.
func factsHash(units []UnitFact, deps []DepFact) string {
	h := sha256.New()

	// Sort unit IDs for order-independence.
	ids := make([]string, len(units))
	for i, u := range units {
		ids[i] = u.ID
	}
	sort.Strings(ids)
	for _, id := range ids {
		h.Write([]byte(id))
		h.Write([]byte{'\x00'})
	}

	// Sort dep signatures for order-independence.
	type depSig struct{ from, to string; count int }
	sigs := make([]depSig, len(deps))
	for i, d := range deps {
		sigs[i] = depSig{d.FromUnit, d.ToUnit, d.Count}
	}
	sort.Slice(sigs, func(i, j int) bool {
		if sigs[i].from != sigs[j].from {
			return sigs[i].from < sigs[j].from
		}
		return sigs[i].to < sigs[j].to
	})
	for _, sig := range sigs {
		fmt.Fprintf(h, "%s\x00%s\x00%d\x00", sig.from, sig.to, sig.count)
	}

	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}

// MarshalManifest encodes a Manifest to byte-stable JSON (same rules as MarshalShard).
func MarshalManifest(m *Manifest) ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("arch.MarshalManifest: %w", err)
	}
	return data, nil
}
