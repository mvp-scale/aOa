// Package treesitter — import-edge extractors (L19.9 E1).
//
// Each extractor walks the TOP-LEVEL children of a tree-sitter root node
// (same level as extractGo/extractPython/extractJavaScript), so we never do
// a full tree descent for imports. One logical traversal per file: the
// extractors are called by ParseFileToMetaAndFacts after extractSymbols,
// sharing the already-parsed tree.
//
// Languages: Go, Python, JavaScript / TypeScript / TSX (P1 set).
// Java deferred to P2 languages batch (FQN/wildcard complexity).

package treesitter

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/corey/aoa/internal/ports"
)

// extractImports dispatches to the language-specific import extractor via
// the importExtractors registry (extractors.go, FDN-2 board #28).
// Returns nil for languages without a P1 extractor.
func extractImports(root *tree_sitter.Node, source []byte, filePath, lang string) []ports.ImportEdge {
	if fn, ok := importExtractors[lang]; ok {
		return fn(root, source, filePath)
	}
	return nil
}

// extractImportsGo extracts import edges from a Go source file.
//
// Go import AST shapes (root.Child(i) = import_declaration):
//
//	Single:  import_declaration > import_spec > interpreted_string_literal
//	Grouped: import_declaration > import_spec_list > import_spec > interpreted_string_literal
//
// Both shapes are handled in one pass over root.ChildCount(). Each import_spec
// has exactly one interpreted_string_literal child that carries the quoted path.
// Aliases (import f "pkg"), blank identifiers (_ "pkg"), and dot imports (. "pkg")
// are all supported — only the string literal is captured.
func extractImportsGo(root *tree_sitter.Node, source []byte, filePath string) []ports.ImportEdge {
	var edges []ports.ImportEdge

	for i := uint(0); i < uint(root.ChildCount()); i++ {
		child := root.Child(i)
		if child.Kind() != "import_declaration" {
			continue
		}

		for j := uint(0); j < uint(child.ChildCount()); j++ {
			c := child.Child(j)
			switch c.Kind() {
			case "import_spec":
				// Single import: import "pkg" or import alias "pkg"
				if lit := childByKind(c, "interpreted_string_literal"); lit != nil {
					if path := unquote(nodeText(lit, source)); path != "" {
						edges = append(edges, ports.ImportEdge{
							FromFile:   filePath,
							ImportPath: path,
							StartLine:  uint32(c.StartPosition().Row + 1),
						})
					}
				}

			case "import_spec_list":
				// Grouped import: import ( "pkg1"\n "pkg2" )
				for k := uint(0); k < uint(c.ChildCount()); k++ {
					spec := c.Child(k)
					if spec.Kind() != "import_spec" {
						continue
					}
					if lit := childByKind(spec, "interpreted_string_literal"); lit != nil {
						if path := unquote(nodeText(lit, source)); path != "" {
							edges = append(edges, ports.ImportEdge{
								FromFile:   filePath,
								ImportPath: path,
								StartLine:  uint32(spec.StartPosition().Row + 1),
							})
						}
					}
				}
			}
		}
	}

	return edges
}

// extractImportsPython extracts import edges from a Python source file.
//
// Python import AST shapes at top-level (root.Child(i)):
//
//	import_statement:
//	  - import os             → dotted_name["os"]
//	  - import os, sys        → dotted_name["os"], dotted_name["sys"]
//	  - import numpy as np    → aliased_import > dotted_name["numpy"]
//
//	import_from_statement:
//	  - from os.path import … → first dotted_name is the module
//	  - from . import module  → relative_import is the module
//
// Only the module being imported FROM is captured for import_from_statement;
// the names of the imported symbols (after "import") are not recorded as separate edges.
func extractImportsPython(root *tree_sitter.Node, source []byte, filePath string) []ports.ImportEdge {
	var edges []ports.ImportEdge

	for i := uint(0); i < uint(root.ChildCount()); i++ {
		child := root.Child(i)
		startLine := uint32(child.StartPosition().Row + 1)

		switch child.Kind() {
		case "import_statement":
			// Walk children: each dotted_name or aliased_import is a module.
			for j := uint(0); j < uint(child.ChildCount()); j++ {
				c := child.Child(j)
				switch c.Kind() {
				case "dotted_name":
					// import os  or  import os, sys
					edges = append(edges, ports.ImportEdge{
						FromFile:   filePath,
						ImportPath: nodeText(c, source),
						StartLine:  startLine,
					})
				case "aliased_import":
					// import numpy as np  →  take the dotted_name child
					if dn := childByKind(c, "dotted_name"); dn != nil {
						edges = append(edges, ports.ImportEdge{
							FromFile:   filePath,
							ImportPath: nodeText(dn, source),
							StartLine:  startLine,
						})
					}
				}
			}

		case "import_from_statement":
			// The FROM module is the first dotted_name or relative_import child.
			// Children: "from", <module>, "import", <what> — we want <module> only.
			for j := uint(0); j < uint(child.ChildCount()); j++ {
				c := child.Child(j)
				switch c.Kind() {
				case "dotted_name":
					edges = append(edges, ports.ImportEdge{
						FromFile:   filePath,
						ImportPath: nodeText(c, source),
						StartLine:  startLine,
					})
					goto nextChild // stop after first module; remaining names are imported symbols
				case "relative_import":
					// from . import x  or  from ..utils import y
					// Capture the full relative_import text (dots + optional dotted_name)
					edges = append(edges, ports.ImportEdge{
						FromFile:   filePath,
						ImportPath: nodeText(c, source),
						StartLine:  startLine,
					})
					goto nextChild
				}
			}
		}
	nextChild:
	}

	return edges
}

// extractImportsJS extracts import edges from JavaScript, TypeScript, or TSX files.
//
// JS/TS import AST shapes (root.Child(i) = import_statement):
//
//	import React from 'react'          → import_statement > ... > string["react"]
//	import { x } from 'react'          → import_statement > ... > string["react"]
//	import * as path from 'path'       → import_statement > ... > string["path"]
//	import './styles.css'              → import_statement > string["./styles.css"]
//	import type { T } from './types'   → import_statement > ... > string["./types"]
//
// The module specifier is always the `string` node that is a direct child of
// import_statement (TypeScript also uses `string` for the module specifier).
// One edge per import statement — multiple named imports from the same module
// are recorded as one edge.
func extractImportsJS(root *tree_sitter.Node, source []byte, filePath string) []ports.ImportEdge {
	var edges []ports.ImportEdge

	for i := uint(0); i < uint(root.ChildCount()); i++ {
		child := root.Child(i)
		if child.Kind() != "import_statement" {
			continue
		}

		startLine := uint32(child.StartPosition().Row + 1)

		// Find the string node (module specifier) — it is always a direct child
		// of import_statement, regardless of what is being imported.
		for j := uint(0); j < uint(child.ChildCount()); j++ {
			c := child.Child(j)
			if c.Kind() == "string" {
				if path := unquote(nodeText(c, source)); path != "" {
					edges = append(edges, ports.ImportEdge{
						FromFile:   filePath,
						ImportPath: path,
						StartLine:  startLine,
					})
					break // one string per import statement
				}
			}
		}
	}

	return edges
}

// unquote strips surrounding quote characters from a string literal.
// Handles double-quotes (Go, JS, Python), single-quotes (JS, Python),
// and backticks (JS template literals — uncommon in imports but safe to strip).
func unquote(s string) string {
	if len(s) < 2 {
		return s
	}
	first, last := s[0], s[len(s)-1]
	if (first == '"' && last == '"') ||
		(first == '\'' && last == '\'') ||
		(first == '`' && last == '`') {
		return s[1 : len(s)-1]
	}
	return s
}
