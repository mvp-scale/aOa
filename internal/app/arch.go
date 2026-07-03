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

// unitSlug converts a path (directory or import path) to a stable unit ID.
// Deterministic: same path → same slug on every machine and run.
// Format: "u_" + lowercase-alphanum-with-underscores.
// Examples:
//
//	"internal/app"         → "u_internal_app"
//	"ext:std/fmt"          → "u_ext_std_fmt"
//	"ext:go.etcd.io/bbolt" → "u_ext_go_etcd_io_bbolt"
//	"" or "."              → "u_root"
func unitSlug(path string) string {
	if path == "" || path == "." {
		return "u_root"
	}
	path = strings.ToLower(path)
	var b strings.Builder
	prevUnderscore := false
	for _, r := range path {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			prevUnderscore = false
		} else {
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
			}
			prevUnderscore = true
		}
	}
	s := strings.TrimRight(b.String(), "_")
	if s == "" {
		return "u_root"
	}
	return "u_" + s
}

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

	// 1. Load all edges (read-only — db.View; C1 does not apply to reads).
	edges, err := a.Store.LoadAllEdges(a.ProjectID)
	if err != nil {
		a.debugf("deriveArch: LoadAllEdges: %v", err)
		return
	}
	if len(edges) == 0 {
		return // nothing to derive yet
	}

	// 2. Snapshot the index under mu, then release (snapshot-release pattern).
	// We only need the Domain field from FileMeta for rung-3 enrichment.
	a.mu.Lock()
	idx := a.Index // pointer snapshot; files are not mutated by watcher during derive
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

	// 6. Compute refHits from the index (dead-candidate fuel).
	// refHits is nil-safe in the domain service; nil is acceptable per spec.
	refHits := buildRefHits(idx)

	// 7. RenderAll: pure domain computation — no I/O, no mu needed.
	svc := &arch.Service{}
	shards, manifest, findings, err := svc.RenderAll(archScope, units, deps, opts, refHits)
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
	if err := a.Store.SaveFindings(projectID, archScope, findings); err != nil {
		a.debugf("deriveArch: SaveFindings: %v", err)
	}

	a.debugf("deriveArch: scope=%s units=%d deps=%d shards=%d findings=%d rev=%s",
		archScope, len(units), len(deps), len(storeShards), len(findings), manifest.Rev)
}

// buildRefHits returns a map of unit ID → index reference hit count for use
// as the dead-candidate fuel in the Detect step.
// Currently returns nil (all units treated as 0 hits) — acceptable per spec
// ("nil acceptable if the WP doesn't specify").
// TODO(L19.15): populate from index token hit counts keyed by unit directory.
func buildRefHits(_ *ports.Index) map[string]int {
	return nil
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

// Findings returns the JSON-encoded []arch.Finding for a scope.
// Returns nil, nil when no findings have been computed.
func (q *archQuerier) Findings(scope string) ([]byte, error) {
	findings, err := q.app.Store.LoadFindings(q.app.ProjectID, scope)
	if err != nil || findings == nil {
		return nil, err
	}
	return json.Marshal(findings)
}

// Derive returns the shortest dep-path (unit IDs) from `from` to `to`,
// limited to k hops.  Returns nil if no path exists within the hop budget.
// Loads all edges and computes BFS at the unit-directory grain.
func (q *archQuerier) Derive(scope, from, to string, k int) ([]string, error) {
	edges, err := q.app.Store.LoadAllEdges(q.app.ProjectID)
	if err != nil || len(edges) == 0 {
		return nil, err
	}

	// Snapshot idx for domain enrichment (only needed for aggregateEdges).
	q.app.mu.Lock()
	idx := q.app.Index
	q.app.mu.Unlock()

	_, deps := aggregateEdges(edges, idx)

	// Build unit-level adjacency.
	adj := make(map[string][]string)
	seen := make(map[[2]string]bool)
	for _, d := range deps {
		key := [2]string{d.FromUnit, d.ToUnit}
		if !seen[key] {
			seen[key] = true
			adj[d.FromUnit] = append(adj[d.FromUnit], d.ToUnit)
		}
	}

	if from == to {
		return []string{from}, nil
	}

	// BFS: breadth-first shortest-path, budget = k hops.
	type bfsState struct {
		id   string
		path []string
	}
	visited := map[string]bool{from: true}
	queue := []bfsState{{id: from, path: []string{from}}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if len(curr.path) > k {
			break // exceeded hop budget
		}
		for _, next := range adj[curr.id] {
			newPath := make([]string, len(curr.path)+1)
			copy(newPath, curr.path)
			newPath[len(curr.path)] = next
			if next == to {
				return newPath, nil // found shortest path
			}
			if !visited[next] && len(newPath) <= k {
				visited[next] = true
				queue = append(queue, bfsState{id: next, path: newPath})
			}
		}
	}
	return nil, nil // no path within k hops
}
