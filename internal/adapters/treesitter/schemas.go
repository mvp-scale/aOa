// Package treesitter — Go struct-entity extractor (COL-1: schema-collector
// & datamodel view).
//
// Mirrors routes.go's shape and honesty tier: extractSchemasGo walks the
// SAME top-level shape as extractGo/extractImports (root.Child(i)), then
// descends into a top-level type_declaration up to 3 more hops —
// type_spec -> struct_type -> field_declaration_list — to read field
// names. D31 boundary (a NEW pin, sibling of the route grant
// routes.go:75-104): this is a bounded descent from a node the walk
// already visits (the top-level type_declaration), not a new unbounded
// traversal — nested struct/interface bodies inside a field's own type are
// NOT walked into (a field naming an inline anonymous struct type is out
// of v1 scope, documented rather than silently mis-extracted). Struct tags
// (raw_string_literal) are a sibling leaf of field_identifier under
// field_declaration and are never read as a field name.
//
// No relationship/FK detection here (D29 ruling: relationship verbs are
// MIXED/overlay-only, a separate concern from this REAL/derived field
// extraction).
//
// Languages: Go only (v1 scope — "Go stacks first" per the work order).
package treesitter

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/corey/aoa/internal/ports"
)

// extractSchemas dispatches to the language-specific schema extractor via
// the schemaExtractors registry (extractors.go, FDN-2 plug point). Returns
// nil for languages without a schema extractor.
func extractSchemas(root *tree_sitter.Node, source []byte, filePath, lang string) []ports.SchemaEntity {
	if fn, ok := schemaExtractors[lang]; ok {
		return fn(root, source, filePath)
	}
	return nil
}

// extractSchemasGo extracts struct-type entities from a Go source file.
// Mirrors extractGo's top-level iteration (and reuses extractGoType's
// struct-detection test), then bounded-descends into the struct's
// field_declaration_list to read field names.
func extractSchemasGo(root *tree_sitter.Node, source []byte, filePath string) []ports.SchemaEntity {
	var entities []ports.SchemaEntity

	for i := uint(0); i < uint(root.ChildCount()); i++ {
		child := root.Child(i)
		if child.Kind() != "type_declaration" {
			continue
		}
		spec := childByKind(child, "type_spec")
		if spec == nil {
			continue
		}
		structType := childByKind(spec, "struct_type")
		if structType == nil {
			continue // non-struct type_declaration (alias, interface, primitive) — honestly out of scope
		}
		name := ""
		if id := childByKind(spec, "type_identifier"); id != nil {
			name = nodeText(id, source)
		}
		if name == "" {
			continue
		}

		entities = append(entities, ports.SchemaEntity{
			FromFile:  filePath,
			Name:      name,
			Fields:    extractStructFields(structType, source),
			StartLine: uint32(child.StartPosition().Row + 1),
		})
	}

	return entities
}

// extractStructFields descends ONE hop from struct_type into its
// field_declaration_list (D31 grant), reading each field_declaration's
// name(s). A single field_declaration may carry more than one name
// (`Age, Height int` -> two field_identifier children) — every name is
// emitted. An embedded field (no field_identifier at all: `Embedded`,
// `*PtrEmbedded`, `pkg.Qualified`) promotes by its own type's identifier —
// the real Go field-name rule for embedding, not a guess.
func extractStructFields(structType *tree_sitter.Node, source []byte) []string {
	fdl := childByKind(structType, "field_declaration_list")
	if fdl == nil {
		return nil
	}

	var fields []string
	for i := uint(0); i < uint(fdl.ChildCount()); i++ {
		fd := fdl.Child(i)
		if fd.Kind() != "field_declaration" {
			continue
		}
		fields = append(fields, fieldNames(fd, source)...)
	}
	return fields
}

// fieldNames returns the field name(s) a single field_declaration
// contributes: every field_identifier child (named field(s)), or — when
// none is present — the embedded field's own promoted type name (bare,
// pointer, or package-qualified).
func fieldNames(fd *tree_sitter.Node, source []byte) []string {
	var names []string
	for i := uint(0); i < uint(fd.ChildCount()); i++ {
		if c := fd.Child(i); c.Kind() == "field_identifier" {
			names = append(names, nodeText(c, source))
		}
	}
	if len(names) > 0 {
		return names
	}

	// Embedded field: no field_identifier — the promoted name is the
	// type's own identifier (qualified_type's last component for
	// `pkg.Qualified`, else the bare/pointer type_identifier).
	if qt := childByKind(fd, "qualified_type"); qt != nil {
		if tid := childByKind(qt, "type_identifier"); tid != nil {
			return []string{nodeText(tid, source)}
		}
	}
	if tid := childByKind(fd, "type_identifier"); tid != nil {
		return []string{nodeText(tid, source)}
	}
	return nil
}
