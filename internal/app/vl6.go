// COL-1 (schema-collector & datamodel view): app-layer wiring for the Data
// Model / ER (view id "datamodel") view — Go struct-entity facts extracted
// from Go source ("Go stacks first" per the work order, mirrors VL-3's
// route extraction).
//
// Same deliberate deviation as vl3.go (see its package doc for the full
// rationale): rather than threading a new fact kind through
// BuildIndexWithFactsAndSink/FactSink (already-unused in production; would
// touch WarmCaches, Reindex, and the fsnotify incremental path plus a new
// batch structure — a blast radius beyond this slice's cap), entities are
// read/derived directly at arch-derive time via a dedicated re-parse
// (entityFileMax bounds it, Go-only for v1).
package app

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
)

// entityFileMax bounds COL-1's derive-time re-parse (MEASURED guard, same
// class as routeFileMax/churnMaxCommits): entities are extracted by
// re-parsing each Go file directly, independent of the main index-build
// parse pass, so this cap keeps one derive from re-parsing an unbounded
// number of files on a very large repo. 2000 Go files comfortably covers a
// real monorepo (v1 80/20 scope, documented rather than silently absent).
const entityFileMax = 2000

// buildEntityEntries assembles COL-1's Data Model / ER rows for one derive
// pass: for every Go file known to the index (idx.Files, Language=="go"),
// re-parse and extract struct-entity declarations (field names, no
// FK/relationship detection — D29 ruling: overlay-only, out of this
// slice). root is the project root (source is re-read from disk since idx
// caches no file content). parser may be nil, or may not implement
// ports.SchemaExtractor (e.g. tokenization-only mode) — either returns nil,
// and the Data Model / ER view renders its honest "0 entities" empty state
// (never a phantom shard — "datamodel" is a mandatory view).
func buildEntityEntries(root string, idx *ports.Index, parser ports.Parser) []arch.EntityEntry {
	if idx == nil || parser == nil {
		return nil
	}
	se, ok := parser.(ports.SchemaExtractor)
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
	if len(goFiles) > entityFileMax {
		goFiles = goFiles[:entityFileMax]
	}

	var entries []arch.EntityEntry
	for _, relPath := range goFiles {
		source, err := os.ReadFile(filepath.Join(root, relPath))
		if err != nil {
			continue
		}
		schemas, err := se.ExtractSchemas(relPath, source)
		if err != nil || len(schemas) == 0 {
			continue
		}
		for _, s := range schemas {
			entries = append(entries, arch.EntityEntry{
				Name:   s.Name,
				Fields: s.Fields,
				Tech:   "Go struct",
				File:   relPath,
				Line:   s.StartLine,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}
