// VL-3 (board #37): app-layer wiring for the API Contract (view id
// "api-contract") view — HTTP route-registration facts extracted from Go
// source (net/http mux + gin idioms, "Go stacks first" per the work order).
//
// Deliberately NOT threaded through the main buildIndexCore parse pass /
// FactSink dual-run path (BuildIndexWithFactsAndSink): that path is already
// unused in production — WarmCaches, Reindex, and the fsnotify-watcher
// incremental path (doFlushEdgeBatch) all build their raw FactDep facts
// straight from the []ImportEdge buildIndexCore/BuildIndexWithFacts return,
// never from a live sink. Threading a new fact kind through would mean
// touching all three call sites plus a new incremental-batch structure
// mirroring edgePendingBatch — a blast radius well beyond this slice's cap.
// Instead this mirrors vl1.go's documented deviation: read/derive directly
// at arch-derive time via a dedicated re-parse (routeFileMax bounds it,
// Go-only for v1). Recorded here rather than silently diverging from the
// work-package brief's "emit through FactSink" framing.
package app

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
)

// routeFileMax bounds VL-3's derive-time re-parse (MEASURED guard, same
// class as churnMaxCommits/churnSinceWindow): routes are extracted by
// re-parsing each Go file directly, independent of the main index-build
// parse pass, so this cap keeps one derive from re-parsing an unbounded
// number of files on a very large repo. 2000 Go files comfortably covers a
// real monorepo (v1 80/20 scope, documented rather than silently absent).
const routeFileMax = 2000

// buildRouteEntries assembles VL-3's API Contract rows for one derive pass:
// for every Go file known to the index (idx.Files, Language=="go"),
// re-parse and extract route-registration calls (net/http mux + gin
// idioms). root is the project root (source is re-read from disk since idx
// caches no file content). parser may be nil, or may not implement
// ports.RouteExtractor (e.g. tokenization-only mode) — either returns nil,
// and the API Contract view renders its honest "0 routes" empty state
// (never a phantom shard — "api-contract" is a mandatory view).
func buildRouteEntries(root string, idx *ports.Index, parser ports.Parser) []arch.RouteEntry {
	if idx == nil || parser == nil {
		return nil
	}
	re, ok := parser.(ports.RouteExtractor)
	if !ok {
		return nil
	}

	var goFiles []string
	for _, fm := range idx.Files {
		if fm == nil || fm.Language != "go" {
			continue
		}
		goFiles = append(goFiles, fm.Path)
	}
	if len(goFiles) == 0 {
		return nil
	}
	sort.Strings(goFiles)
	if len(goFiles) > routeFileMax {
		goFiles = goFiles[:routeFileMax]
	}

	var entries []arch.RouteEntry
	for _, relPath := range goFiles {
		source, err := os.ReadFile(filepath.Join(root, relPath))
		if err != nil {
			continue
		}
		edges, err := re.ExtractRoutes(relPath, source)
		if err != nil || len(edges) == 0 {
			continue
		}
		for _, e := range edges {
			entries = append(entries, arch.RouteEntry{
				Method:    e.Method,
				Path:      e.Path,
				Handler:   e.Handler,
				Framework: e.Framework,
				File:      relPath,
				Line:      e.StartLine,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Method < entries[j].Method
	})
	return entries
}
