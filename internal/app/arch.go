// Package app — L19.14 arch wiring (app/arch.go).
//
// Derivation entry point: App.deriveArch() loads all resolved import edges,
// aggregates them to UnitFact/DepFact at package/directory grain, runs the
// arch domain service (grouping + detectors + renderers), and persists shards
// and findings to bbolt — all outside App.mu (C1 law).
//
// C1 invariant: every write call (SaveShards, SaveManifest, SaveFindings) must
// happen AFTER App.mu is released.  The snapshot-release-write pattern is
// enforced throughout: state is read under mu, mu is released, then IO runs.
//
// C4 gate: all paths check a.ArchEnabled first; when false the function is a
// no-op and no buckets are touched.
//
// Compact-time trigger: App.deriveArch is fired via safeGo after every
// doFlushEdgeBatch (one render per debounce window, C2) and after Reindex.
// It is NEVER called inside onFileChanged's critical section.
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
)

// archScope is the default scope name for local derivation.
const archScope = "local"

// ── Unit ID helpers ────────────────────────────────────────────────────────

// unitSlug is an app-package shim that delegates to the canonical
// arch.UnitSlug implementation (internal/domain/arch/graph.go).
// Both app/arch.go and cmd/arch.go consume the same code path;
// copy discipline is eliminated (PC8 Finding 14).
func unitSlug(path string) string { return arch.UnitSlug(path) }

// unitLabel returns a short display label for a path.
// Strips the "ext:" prefix from external paths and returns the last meaningful
// directory segment, truncated to ≤30 chars (view-standards budget).
func unitLabel(path string) string {
	if path == "" || path == "." {
		return "."
	}
	p := path
	if strings.HasPrefix(p, "ext:std/") {
		p = strings.TrimPrefix(p, "ext:std/")
	} else if strings.HasPrefix(p, "ext:") {
		p = strings.TrimPrefix(p, "ext:")
	}
	label := filepath.Base(p)
	if label == "." || label == "/" {
		label = p
	}
	if len(label) > 30 {
		label = label[:30]
	}
	return label
}

// ── Edge aggregation ───────────────────────────────────────────────────────

// aggregateEdges converts resolved ImportEdge slices to UnitFact + DepFact.
// Unit grain is package/directory (directory of FromFile for source units;
// resolved ImportPath for target units).  Each unit appears once; deps are
// aggregated by (fromUnit, toUnit) pair with a total count.
//
// idx may be nil (acceptable per spec); when non-nil, file Domain fields from
// the index are used to populate UnitFact.Domain for rung-3 grouping.
//
// Self-loops (source == target at the directory grain) are dropped.
func aggregateEdges(edges []ports.ImportEdge, idx *ports.Index) ([]arch.UnitFact, []arch.DepFact) {
	// Build relPath → domain map for rung-3 (atlas domain).
	fileDomains := make(map[string]string)
	if idx != nil {
		for _, fm := range idx.Files {
			if fm.Domain != "" {
				fileDomains[fm.Path] = fm.Domain
			}
		}
	}

	type depKey struct{ from, to string }
	type depVal struct {
		count int
		file  string
		line  uint32
	}

	unitMap := make(map[string]*arch.UnitFact)
	depMap := make(map[depKey]*depVal)

	for _, e := range edges {
		// Source unit: directory of the file containing the import.
		fromDir := filepath.Dir(e.FromFile)
		if fromDir == "." {
			fromDir = "root"
		}
		fromID := unitSlug(fromDir)

		if _, ok := unitMap[fromID]; !ok {
			unitMap[fromID] = &arch.UnitFact{
				ID:     fromID,
				Label:  unitLabel(fromDir),
				Path:   fromDir,
				File:   e.FromFile,
				Line:   e.StartLine,
				Domain: fileDomains[e.FromFile],
			}
		} else if unitMap[fromID].Domain == "" && fileDomains[e.FromFile] != "" {
			// Unit was first seen as an import target (no domain set then); retroactively
			// assign the domain now that we have a source file for this unit.
			unitMap[fromID].Domain = fileDomains[e.FromFile]
		}

		// Target unit: the resolved ImportPath (intra-repo dir or "ext:...").
		toPath := e.ImportPath
		toID := unitSlug(toPath)

		if _, ok := unitMap[toID]; !ok {
			unitMap[toID] = &arch.UnitFact{
				ID:    toID,
				Label: unitLabel(toPath),
				Path:  toPath,
				// File/Line left zero for external targets (no source position).
			}
		}

		// Drop self-loops (same directory imports itself).
		if fromID == toID {
			continue
		}

		key := depKey{from: fromID, to: toID}
		if dv, ok := depMap[key]; ok {
			dv.count++
		} else {
			depMap[key] = &depVal{count: 1, file: e.FromFile, line: e.StartLine}
		}
	}

	// Stable sort by ID for byte-determinism.
	units := make([]arch.UnitFact, 0, len(unitMap))
	for _, u := range unitMap {
		units = append(units, *u)
	}
	sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })

	// Stable sort by (from, to) for byte-determinism.
	type depRow struct {
		from, to string
		val      *depVal
	}
	rows := make([]depRow, 0, len(depMap))
	for k, v := range depMap {
		rows = append(rows, depRow{from: k.from, to: k.to, val: v})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].from != rows[j].from {
			return rows[i].from < rows[j].from
		}
		return rows[i].to < rows[j].to
	})

	deps := make([]arch.DepFact, len(rows))
	for i, r := range rows {
		deps[i] = arch.DepFact{
			FromUnit: r.from,
			ToUnit:   r.to,
			Count:    r.val.count,
			File:     r.val.file,
			Line:     r.val.line,
		}
	}
	return units, deps
}

// ── Overlay loading ────────────────────────────────────────────────────────

// loadOverlaySpec reads and parses the overlay file for a scope.
// Returns nil, nil when no overlay file exists (valid — overlay is optional).
// Returns an error when the file exists but cannot be parsed.
// Path: {root}/.aoa/arch/overlays/{scope}.json
func loadOverlaySpec(projectRoot, scope string) (*arch.OverlaySpec, error) {
	overlayPath := filepath.Join(projectRoot, ".aoa", "arch", "overlays", scope+".json")
	data, err := os.ReadFile(overlayPath)
	if os.IsNotExist(err) {
		return nil, nil // no overlay — valid
	}
	if err != nil {
		return nil, fmt.Errorf("arch overlay: read %s: %w", overlayPath, err)
	}
	return arch.ParseOverlay(data)
}

// ── App.deriveArch — main derivation entry point ───────────────────────────

// deriveArch is the compact-time derivation entry point (L19.14 step 9).
// It is always called via safeGo — never under App.mu, never in the watcher
// critical section.
//
// Pipeline:
//  1. C4 gate — no-op when ArchEnabled is false.
//  2. Load all edges from the store (read-only, C1 safe).
//  3. Snapshot idx under mu, then release (snapshot-release pattern).
//  4. Aggregate edges → UnitFact / DepFact slices.
//  5. Load overlay spec (file I/O, all reads before RenderAll).
//  6. Apply overlay leash validation (only after units are known).
//  7. RenderAll: grouping + detectors + renderers (pure, no I/O).
//  8. Persist shards + manifest + findings outside mu (C1 compliant).
func (a *App) deriveArch() {
	if !a.ArchEnabled || a.Store == nil {
		return // C4 gate
	}

	// Serialize whole derives: without this, a stale-generation goroutine could
	// finish SaveManifest after a fresher one (torn manifest). Each serialized
	// run loads edges fresh inside the critical section, so the last completed
	// run always reflects the newest data.
	a.archDeriveMu.Lock()
	defer a.archDeriveMu.Unlock()

	// 1. Load all edges (read-only — db.View; C1 does not apply to reads).
	// BE-2 policy: _test.go imports ARE included in the fact substrate — they enter
	// via LoadAllEdges without filtering, affecting fan-in counts, road directions, and DAG claims.
	edges, err := a.Store.LoadAllEdges(a.ProjectID)
	if err != nil {
		a.debugf("deriveArch: LoadAllEdges: %v", err)
		return
	}
	if len(edges) == 0 {
		return // nothing to derive yet
	}

	// 2. Snapshot the index under mu, then release (snapshot-release pattern).
	// MUST Clone: a.Index is live — Reindex/WarmCaches/Wipe write Index.Files
	// under mu after we release it (same contract as SaveIndex, app.go).
	a.mu.Lock()
	idx := a.Index.Clone()
	projectID := a.ProjectID
	a.mu.Unlock()

	// 3. Aggregate edges → unit facts + dep facts (pure, no I/O).
	units, deps := aggregateEdges(edges, idx)
	if len(units) == 0 {
		return
	}

	// 4. Load overlay spec (all file I/O before RenderAll).
	overlaySpec, overlayErr := loadOverlaySpec(a.ProjectRoot, archScope)
	if overlayErr != nil {
		a.debugf("deriveArch: overlay: %v (continuing without overlay)", overlayErr)
	}

	// 5. Build GroupOptions from overlay (leash-validate against known units).
	var opts *arch.GroupOptions
	if overlaySpec != nil {
		approved, invalidIDs := arch.ApplyOverlay(overlaySpec, units)
		opts = &arch.GroupOptions{
			Overlays:             approved,
			OverlayHadInvalidIDs: len(invalidIDs) > 0,
		}
	}

	// 6. Compute refHits from the index (dead-candidate fuel). Non-nil whenever
	// idx is non-nil (here it always is — Clone never returns nil), so the
	// detector fires only on units with zero inbound deps AND zero index refs.
	refHits := buildRefHits(idx)

	// 6b. Build symbol index for the code renderer (②b, L19.23).
	// idx is already a Clone (snapshot-release, line 238); safe to read here.
	symIndex := buildCodeSymbolIndex(idx)

	// 7. RenderAll: pure domain computation — no I/O, no mu needed.
	svc := &arch.Service{}
	shards, manifest, findings, err := svc.RenderAll(archScope, units, deps, opts, refHits, symIndex)
	if err != nil {
		a.debugf("deriveArch: RenderAll: %v", err)
		return
	}

	// 8. Persist — all writes outside a.mu (C1 compliant).

	// Build the keyed shard map for the store: "{scope}/{view}@{hash}"
	storeShards := make(map[string][]byte, len(manifest.Views))
	for _, ve := range manifest.Views {
		if data, ok := shards[ve.ID]; ok {
			storeShards[ve.Key] = data
		}
	}
	if len(storeShards) > 0 {
		if err := a.Store.SaveShards(projectID, storeShards); err != nil {
			a.debugf("deriveArch: SaveShards: %v", err)
		}
	}

	// Persist manifest (ETag anchor for the viewer and socket layer).
	manifestData, err := arch.MarshalManifest(&manifest)
	if err == nil {
		if err := a.Store.SaveManifest(projectID, archScope, manifestData); err != nil {
			a.debugf("deriveArch: SaveManifest: %v", err)
		}
	} else {
		a.debugf("deriveArch: MarshalManifest: %v", err)
	}

	// Persist findings (pure cache; always re-derivable).
	// Convert domain findings to the ports DTO (adapter must not import domain/arch).
	if err := a.Store.SaveFindings(projectID, archScope, archFindingsToPortsFindings(findings)); err != nil {
		a.debugf("deriveArch: SaveFindings: %v", err)
	}

	a.debugf("deriveArch: scope=%s units=%d deps=%d shards=%d findings=%d rev=%s",
		archScope, len(units), len(deps), len(storeShards), len(findings), manifest.Rev)
}

// buildRefHits returns a map of unit ID → count of index token references that
// land in files belonging to that unit. It is the dead-candidate detector's
// fuel: a unit fires as a dead-code candidate only when it has zero inbound
// dependencies AND zero index references (refHits[u]==0). A unit whose files
// carry indexed symbols is suppressed — it is live code the import graph simply
// never saw an inbound edge for (an entry point, or reachable via reflection or
// build tags the extractor cannot see).
//
// Grain: package/directory, identical to aggregateEdges' source-unit slugging —
// filepath.Dir of each file's relative path ("." → "root"), then unitSlug. Both
// this map and the aggregated units share the same relative-path keyspace
// (indexer.go writes FileMeta.Path and ImportEdge.FromFile from one relPath), so
// the two agree on unit IDs by construction.
//
// A "reference" here is one TokenRef in the index — every indexed symbol
// occurrence in a file counts once toward that file's unit. This is a
// deterministic proxy for "does this unit have indexed code", not a precise
// cross-reference analysis; its only job is to separate a genuinely empty /
// unindexed directory from a real package that merely lacks an inbound import.
//
// Returns nil when idx is nil so the detector can distinguish "measured, zero
// references" (non-nil map, 0 entry) from "not measured" (nil) and word its
// message honestly (detect.go DetectDeadCandidates).
func buildRefHits(idx *ports.Index) map[string]int {
	if idx == nil {
		return nil
	}
	refHits := make(map[string]int)
	for _, refs := range idx.Tokens {
		for _, ref := range refs {
			fm, ok := idx.Files[ref.FileID]
			if !ok || fm.Path == "" {
				continue
			}
			dir := filepath.Dir(fm.Path)
			if dir == "." {
				dir = "root"
			}
			refHits[unitSlug(dir)]++
		}
	}
	return refHits
}

// buildCodeSymbolIndex converts a ports.Index snapshot into an arch.CodeSymbolIndex
// for the code renderer (②b, L19.23).
//
// The input is always a Clone of the live index (see deriveArch:238 and
// Derive:493); it is safe to iterate without holding App.mu.
//
// Returns nil when idx has no symbol metadata — the caller treats nil as
// "no symbols available" and omits the code view (never a phantom shard).
func buildCodeSymbolIndex(idx *ports.Index) *arch.CodeSymbolIndex {
	if idx == nil || len(idx.Metadata) == 0 {
		return nil
	}

	byFile := make(map[string][]arch.CodeSymbol, len(idx.Files))
	for ref, meta := range idx.Metadata {
		if meta == nil {
			continue
		}
		fm, ok := idx.Files[ref.FileID]
		if !ok || fm.Path == "" {
			continue
		}
		byFile[fm.Path] = append(byFile[fm.Path], arch.CodeSymbol{
			Name:      meta.Name,
			Signature: meta.Signature,
			Kind:      meta.Kind,
			File:      fm.Path,
			StartLine: meta.StartLine,
			EndLine:   meta.EndLine,
			Parent:    meta.Parent,
		})
	}
	if len(byFile) == 0 {
		return nil
	}

	// Sort each file's symbol list by StartLine for determinism (T4).
	for path := range byFile {
		syms := byFile[path]
		sort.Slice(syms, func(i, j int) bool {
			return syms[i].StartLine < syms[j].StartLine
		})
		byFile[path] = syms
	}

	return &arch.CodeSymbolIndex{ByFile: byFile}
}

// ── Finding conversion ────────────────────────────────────────────────────

// archFindingsToPortsFindings converts []arch.Finding (domain type) to
// []ports.Finding (ports DTO) for persistence. The adapter layer must not
// import domain/arch, so conversion happens here at the app boundary (PC8 F-1).
// JSON tags are identical between the two types — bucket bytes are byte-compatible.
func archFindingsToPortsFindings(findings []arch.Finding) []ports.Finding {
	out := make([]ports.Finding, len(findings))
	for i := range findings {
		f := &findings[i] // pointer to avoid 160-byte copy per iteration
		srcs := make([]ports.SourceRef, len(f.Sources))
		for j := range f.Sources {
			srcs[j] = ports.SourceRef{File: f.Sources[j].File, Line: f.Sources[j].Line}
		}
		out[i] = ports.Finding{
			ID:          f.ID,
			Rule:        f.Rule,
			Severity:    f.Severity,
			Scope:       f.Scope,
			Message:     f.Message,
			Subjects:    f.Subjects,
			Sources:     srcs,
			CheapestCut: f.CheapestCut,
			Attrs:       f.Attrs,
			New:         f.New,
		}
	}
	return out
}

// ── App.Arch — ArchQuerier accessor ───────────────────────────────────────

// Arch returns the arch querier for this project, or nil when the arch flag is
// off (C4).  The returned querier is safe for concurrent use (all reads go
// through bbolt db.View; no writes).
//
// Implements socket.AppQueries.Arch().
func (a *App) Arch() ports.ArchQuerier {
	if !a.ArchEnabled || a.Store == nil {
		return nil // C4: nil signals "arch not available" to callers
	}
	return &archQuerier{app: a}
}

// ── archQuerier — ports.ArchQuerier implementation ────────────────────────

// archQuerier implements ports.ArchQuerier backed by the bbolt ArchStore.
// All methods are read-only (db.View); thread-safe for concurrent callers.
type archQuerier struct {
	app *App
}

// Manifest returns the current view catalog for the given scope.
// Returns nil, nil when no shards have been derived yet.
func (q *archQuerier) Manifest(scope string) (*ports.ArchManifest, error) {
	data, err := q.app.Store.LoadManifest(q.app.ProjectID, scope)
	if err != nil || data == nil {
		return nil, err
	}
	var m ports.ArchManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("arch.Manifest: unmarshal: %w", err)
	}
	return &m, nil
}

// View returns the raw JSON bytes of a rendered shard by scope + view ID.
// Returns nil, nil when the view has not been rendered.
func (q *archQuerier) View(scope, id string) ([]byte, error) {
	// Look up the manifest to find the key for this view.
	m, err := q.Manifest(scope)
	if err != nil || m == nil {
		return nil, err
	}
	for _, ve := range m.Views {
		if ve.ID == id {
			return q.app.Store.LoadShard(q.app.ProjectID, ve.Key)
		}
	}
	return nil, nil // view not found
}

// Findings returns the JSON-encoded []ports.Finding for a scope.
// Returns nil, nil when no findings have been computed.
// The JSON shape is identical to arch.Finding (same tags) — callers and the
// viewer see the same bytes regardless of which Go type was persisted.
func (q *archQuerier) Findings(scope string) ([]byte, error) {
	findings, err := q.app.Store.LoadFindings(q.app.ProjectID, scope)
	if err != nil || findings == nil {
		return nil, err
	}
	return json.Marshal(findings)
}

// Facts returns the JSON-encoded import-edge provenance trail for a subject.
// Subject is matched as a substring against both FromFile and ImportPath.
// L19.16: backs the socket MethodArchFacts handler and `aoa arch facts` CLI.
func (q *archQuerier) Facts(_ string, subject string, limit int) ([]byte, error) {
	edges, err := q.app.Store.LoadAllEdges(q.app.ProjectID)
	if err != nil || len(edges) == 0 {
		return nil, err
	}
	type factEntry struct {
		FromFile   string `json:"from_file"`
		ImportPath string `json:"import_path"`
		StartLine  uint32 `json:"start_line"`
	}
	var result []factEntry
	for _, e := range edges {
		if strings.Contains(e.FromFile, subject) || strings.Contains(e.ImportPath, subject) {
			result = append(result, factEntry{
				FromFile:   e.FromFile,
				ImportPath: e.ImportPath,
				StartLine:  e.StartLine,
			})
		}
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	if result == nil {
		result = []factEntry{} // never null in JSON output
	}
	return json.Marshal(result)
}

// Graph returns the raw substrate knowledge graph as JSON.
// grain="file": file-level nodes (FromFile + resolved ImportPath) and edges with G7 provenance.
// grain="unit": package-directory aggregation via aggregateEdges.
// SIZE GUARD: if file grain would produce > 20,000 edges, downgrades to unit grain.
// C4: returns nil, nil when no edges exist.
// C1: index Clone under mu before release (avoids aliasing race — T20).
func (q *archQuerier) Graph(scope string, grain string) ([]byte, error) {
	edges, err := q.app.Store.LoadAllEdges(q.app.ProjectID)
	if err != nil || len(edges) == 0 {
		return nil, err
	}

	// Get manifest rev for provenance annotation (best-effort; "" on miss).
	var rev string
	if m, mErr := q.Manifest(scope); mErr == nil && m != nil {
		rev = m.Rev
	}

	const edgeBudget = 20000
	needUnitGrain := grain == "unit" || len(edges) > edgeBudget

	var idx *ports.Index
	if needUnitGrain {
		// MUST Clone: q.app.Index is live; Reindex/WarmCaches write it under mu.
		q.app.mu.Lock()
		idx = q.app.Index.Clone()
		q.app.mu.Unlock()
	}

	downgraded := ""
	if grain != "unit" && len(edges) > edgeBudget {
		downgraded = fmt.Sprintf("file→unit (%d edges over budget)", len(edges))
	}
	payload := BuildGraphPayload(edges, idx, rev, grain, downgraded)
	return json.Marshal(payload)
}

// BuildGraphPayload assembles a ports.GraphPayload from raw import edges.
// grain="file": file-level graph with G7 provenance.
// grain="unit": package-directory aggregation (idx may be nil).
// downgraded carries the server-side SIZE GUARD message when non-empty.
// Exported so cliArchQuerier (cmd/aoa) can call it without duplicating logic.
func BuildGraphPayload(edges []ports.ImportEdge, idx *ports.Index, rev, grain, downgraded string) ports.GraphPayload {
	if grain == "unit" || downgraded != "" {
		return buildUnitGrainGraph(edges, idx, rev, downgraded)
	}
	return buildFileGrainGraph(edges, rev, "")
}

// buildFileGrainGraph produces the file-grain graph payload.
// Nodes: distinct FromFile values (internal) + distinct ImportPath values (may be ext:*).
// Edges: one entry per distinct (FromFile, ImportPath) pair with G7 StartLine provenance.
// Both slices are sorted for byte-determinism.
func buildFileGrainGraph(edges []ports.ImportEdge, rev, downgraded string) ports.GraphPayload {
	nodeMap := make(map[string]*ports.GraphNode, len(edges)*2)
	edgeSeen := make(map[[2]string]bool, len(edges))
	var gEdges []ports.GraphEdge

	for _, e := range edges {
		// Source node: the importing file.
		if _, ok := nodeMap[e.FromFile]; !ok {
			nodeMap[e.FromFile] = &ports.GraphNode{
				ID:    e.FromFile,
				Label: filepath.Base(e.FromFile),
				Path:  e.FromFile,
			}
		}
		// Target node: the resolved import path (intra-repo dir or "ext:...").
		tgt := e.ImportPath
		if _, ok := nodeMap[tgt]; !ok {
			isExt := strings.HasPrefix(tgt, "ext:")
			nodeMap[tgt] = &ports.GraphNode{
				ID:    tgt,
				Label: unitLabel(tgt),
				Path:  tgt,
				Ext:   isExt,
			}
		}
		// Edge: deduplicate (FromFile, ImportPath) — keep first StartLine.
		key := [2]string{e.FromFile, tgt}
		if !edgeSeen[key] {
			edgeSeen[key] = true
			gEdges = append(gEdges, ports.GraphEdge{
				From: e.FromFile,
				To:   tgt,
				File: e.FromFile,
				Line: e.StartLine,
			})
		}
	}

	// Sort nodes by ID for byte-determinism.
	nodes := make([]ports.GraphNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, *n)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	// Sort edges by (from, to) for byte-determinism.
	sort.Slice(gEdges, func(i, j int) bool {
		if gEdges[i].From != gEdges[j].From {
			return gEdges[i].From < gEdges[j].From
		}
		return gEdges[i].To < gEdges[j].To
	})

	return ports.GraphPayload{
		Grain:      "file",
		Rev:        rev,
		Downgraded: downgraded,
		Nodes:      nodes,
		Edges:      gEdges,
	}
}

// buildUnitGrainGraph produces the unit-grain graph payload via aggregateEdges.
// idx may be nil (domain enrichment skipped — acceptable for headless or CLI use).
// Both slices are sorted for byte-determinism (aggregateEdges guarantees this).
func buildUnitGrainGraph(edges []ports.ImportEdge, idx *ports.Index, rev, downgraded string) ports.GraphPayload {
	units, deps := aggregateEdges(edges, idx)

	nodes := make([]ports.GraphNode, 0, len(units))
	for _, u := range units {
		isExt := strings.HasPrefix(u.Path, "ext:")
		nodes = append(nodes, ports.GraphNode{
			ID:     u.ID,
			Label:  u.Label,
			Path:   u.Path,
			Ext:    isExt,
			Domain: u.Domain, // atlas domain from UnitFact (populated by aggregateEdges via fileDomains)
		})
	}
	// aggregateEdges returns units sorted by ID — no re-sort needed.

	gEdges := make([]ports.GraphEdge, 0, len(deps))
	for _, d := range deps {
		gEdges = append(gEdges, ports.GraphEdge{
			From:  d.FromUnit,
			To:    d.ToUnit,
			Count: d.Count,
			File:  d.File,
			Line:  d.Line,
		})
	}
	// aggregateEdges returns deps sorted by (from, to) — no re-sort needed.

	return ports.GraphPayload{
		Grain:      "unit",
		Rev:        rev,
		Downgraded: downgraded,
		Nodes:      nodes,
		Edges:      gEdges,
	}
}

// Derive returns the shortest dep-path (unit IDs) from `from` to `to`,
// limited to k hops.  Returns nil if no path exists within the hop budget.
// Loads all edges and computes BFS at the unit-directory grain.
// BFS delegated to arch.BFSShortestPath (canonical — PC8 Finding 14).
func (q *archQuerier) Derive(scope, from, to string, k int) ([]string, error) {
	edges, err := q.app.Store.LoadAllEdges(q.app.ProjectID)
	if err != nil || len(edges) == 0 {
		return nil, err
	}

	// Snapshot idx for domain enrichment (only needed for aggregateEdges).
	// MUST Clone: q.app.Index is live after mu release (see deriveArch).
	q.app.mu.Lock()
	idx := q.app.Index.Clone()
	q.app.mu.Unlock()

	_, deps := aggregateEdges(edges, idx)

	// Build unit-level adjacency (de-duplicated).
	adj := make(map[string][]string)
	seen := make(map[[2]string]bool)
	for _, d := range deps {
		key := [2]string{d.FromUnit, d.ToUnit}
		if !seen[key] {
			seen[key] = true
			adj[d.FromUnit] = append(adj[d.FromUnit], d.ToUnit)
		}
	}

	return arch.BFSShortestPath(adj, from, to, k), nil
}
