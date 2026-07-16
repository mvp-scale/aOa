// Package treesitter — extractor registry (FDN-2, board #28).
//
// Replaces the two hardcoded dispatch switches (extractSymbols in parser.go,
// extractImports in imports.go) with a package-level lookup table. Each
// language extractor registers itself here; the existing walk keeps calling
// into the SAME node it already visits (the root's children) — no deepened
// traversal, no descent into method/function bodies (D9/D21 boundary is
// unchanged by this file).
//
// This is a pure indirection swap: the registry maps 1:1 onto the previous
// switch arms and every extractor function is untouched. Two of the three
// P1 symbol extractors (extractPython, extractJavaScript) take an extra
// "parent" argument used only by their own internal recursion — the
// top-level dispatch always calls them with parent="" — so they are wrapped
// in adapter closures to present a uniform two-arg shape to the registry.
package treesitter

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/corey/aoa/internal/ports"
)

// symbolExtractorFunc is the uniform shape every registered symbol extractor
// presents to extractSymbols, regardless of the underlying function's own
// signature.
type symbolExtractorFunc func(root *tree_sitter.Node, source []byte) []Symbol

// importExtractorFunc is the uniform shape every registered import extractor
// presents to extractImports. All three P1 language extractors already share
// this exact signature — no adapter closures needed on the import side.
type importExtractorFunc func(root *tree_sitter.Node, source []byte, filePath string) []ports.ImportEdge

// routeExtractorFunc is the uniform shape every registered route extractor
// presents to extractRoutes (VL-3, board #37, routes.go).
type routeExtractorFunc func(root *tree_sitter.Node, source []byte, filePath string) []ports.RouteEdge

// schemaExtractorFunc is the uniform shape every registered schema extractor
// presents to extractSchemas (COL-1, schemas.go).
type schemaExtractorFunc func(root *tree_sitter.Node, source []byte, filePath string) []ports.SchemaEntity

// symbolExtractors maps a detected language name to its symbol extractor.
// Languages absent from this map fall back to extractGeneric (rule-table
// driven) — unchanged behavior from the previous switch's default arm.
var symbolExtractors = map[string]symbolExtractorFunc{
	"go": extractGo,
	"python": func(root *tree_sitter.Node, source []byte) []Symbol {
		return extractPython(root, source, "")
	},
	"javascript": func(root *tree_sitter.Node, source []byte) []Symbol {
		return extractJavaScript(root, source, "")
	},
	"typescript": func(root *tree_sitter.Node, source []byte) []Symbol {
		return extractJavaScript(root, source, "")
	},
	"tsx": func(root *tree_sitter.Node, source []byte) []Symbol {
		return extractJavaScript(root, source, "")
	},
}

// importExtractors maps a detected language name to its import-edge
// extractor. Languages absent from this map produce no edges — unchanged
// behavior from the previous switch's default arm (nil).
var importExtractors = map[string]importExtractorFunc{
	"go":         extractImportsGo,
	"python":     extractImportsPython,
	"javascript": extractImportsJS,
	"typescript": extractImportsJS,
	"tsx":        extractImportsJS,
}

// routeExtractors maps a detected language name to its route extractor
// (VL-3, board #37). Languages absent from this map produce no routes.
// Go only for v1 ("Go stacks first" per the work order: net/http mux + gin
// idioms, both handled by extractRoutesGo's method-name classification).
var routeExtractors = map[string]routeExtractorFunc{
	"go": extractRoutesGo,
}

// schemaExtractors maps a detected language name to its schema extractor
// (COL-1, schema-collector). Languages absent from this map produce no
// entities. Go only for v1 ("Go stacks first" per the work order).
var schemaExtractors = map[string]schemaExtractorFunc{
	"go": extractSchemasGo,
}
