// VL-1 (board #35): app-layer wiring for the SBOM/Tech Stack/Glossary views.
//
// Deliberately separate from arch_factstore_bridge.go: lockfile-derived
// Components are NOT written through FactStore.ReplaceFactsForFile using
// FactKind=FactDep, even though that would be the most literal reading of
// "reader emits through FactStore" (D4-D6). The compactor
// (internal/domain/facts/compactor.go:105) folds every FactDep fact — from
// any file — into the SAME unit-adjacency graph that feeds cycles/dsm/
// component/context/capability. A go.mod's ~140 requires stamped as FactDep
// would silently inject phantom nodes into that graph (a real correctness
// bug, not a hypothetical — verified by reading compactor.go before wiring
// this). So this file reads the manifests directly at derive time (same
// "snapshot-release" shape as buildRefHits/buildCodeSymbolIndex reading idx
// directly) and threads the result through the new, additive
// arch.VLInputs bundle instead of the shared facts_raw stream. Recorded here
// rather than silently deviating from the work-package brief.
package app

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/corey/aoa/internal/adapters/lockfile"
	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/domain/enricher"
	"github.com/corey/aoa/internal/domain/glossary"
	"github.com/corey/aoa/internal/ports"
)

// buildVLInputs assembles the VL-1 view-library data (Components/
// Technologies/GlossaryTerms) for one derive pass. root is the project root
// (manifests are read at repo-root grain only — V1 scope, 80/20: no
// recursive monorepo lockfile discovery yet). idx may be nil (mirrors
// buildRefHits/buildCodeSymbolIndex's contract); enr may be nil (headless /
// no atlas loaded).
func buildVLInputs(root string, idx *ports.Index, enr *enricher.Enricher) *arch.VLInputs {
	components := discoverComponents(root)
	return &arch.VLInputs{
		Components:    components,
		Technologies:  buildTechnologies(idx, components),
		GlossaryTerms: harvestGlossary(enr),
	}
}

// discoverComponents reads the project root's go.mod and package.json (each
// optional — missing files are silently skipped, not an error) and returns
// their combined Components, go.mod first then package.json, each internally
// sorted by name (ParseGoMod/ParsePackageJSON's own determinism contract).
func discoverComponents(root string) []arch.Component {
	var out []arch.Component
	if data, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		if comps, perr := lockfile.ParseGoMod("go.mod", data); perr == nil {
			out = append(out, convertComponents(comps)...)
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		if comps, perr := lockfile.ParsePackageJSON("package.json", data); perr == nil {
			out = append(out, convertComponents(comps)...)
		}
	}
	return out
}

// convertComponents adapts internal/adapters/lockfile.Component (I/O-layer
// shape) to arch.Component (dependency-free domain shape) at the boundary —
// D25 pattern, same as unitFactsFromFactStore adapting ports.Fact to
// arch.UnitFact/DepFact.
func convertComponents(in []lockfile.Component) []arch.Component {
	out := make([]arch.Component, len(in))
	for i, c := range in {
		out[i] = arch.Component{
			Name:     c.Name,
			Version:  c.Version,
			Supplier: c.Supplier,
			Language: c.Language,
			Unpinned: c.Unpinned,
			File:     c.File,
			Line:     c.Line,
		}
	}
	return out
}

// buildTechnologies joins FileMeta.Language (idx, aggregated per language:
// file count + lowest-path representative source pointer) with the lockfile
// Components (one dependency row each) into the Tech Stack view's rows.
// idx may be nil → language rows are simply absent (dependency rows still
// render); components may be empty → dependency rows absent.
func buildTechnologies(idx *ports.Index, components []arch.Component) []arch.TechEntry {
	type langAgg struct {
		count int
		file  string
	}
	langs := make(map[string]*langAgg)
	if idx != nil {
		for _, fm := range idx.Files {
			if fm == nil || fm.Language == "" {
				continue
			}
			a, ok := langs[fm.Language]
			if !ok {
				a = &langAgg{}
				langs[fm.Language] = a
			}
			a.count++
			// Lowest path wins — arbitrary but deterministic representative
			// source pointer (G7: real, just not privileged over any other).
			if a.file == "" || fm.Path < a.file {
				a.file = fm.Path
			}
		}
	}

	names := make([]string, 0, len(langs))
	for name := range langs {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]arch.TechEntry, 0, len(names)+len(components))
	for _, name := range names {
		a := langs[name]
		out = append(out, arch.TechEntry{Name: name, Kind: "language", Count: a.count, File: a.file, Line: 1})
	}
	for _, c := range components {
		out = append(out, arch.TechEntry{
			Name:     c.Name,
			Kind:     "dependency",
			Count:    1,
			Unpinned: c.Unpinned,
			File:     c.File,
			Line:     c.Line,
		})
	}
	return out
}

// harvestGlossary converts internal/domain/glossary.Harvest's atlas-term
// candidates into arch.GlossaryEntry. Returns nil when enr is nil (no atlas
// loaded — headless/test contexts) rather than erroring; the Glossary view
// renders its honest "0 terms" empty state in that case.
func harvestGlossary(enr *enricher.Enricher) []arch.GlossaryEntry {
	if enr == nil {
		return nil
	}
	harvested := glossary.Harvest(enr.DomainDefs())
	out := make([]arch.GlossaryEntry, len(harvested))
	for i, e := range harvested {
		out[i] = arch.GlossaryEntry{Term: e.Term, Domain: e.Domain, Definition: e.Definition}
	}
	return out
}
