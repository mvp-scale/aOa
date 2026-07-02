package ports

// ImportEdge records a single import statement found in a source file.
// Every field is mandatory — no zero-value edge may be emitted (G7: truth is stamped).
//
// FromFile is a project-relative path (e.g., "internal/app/app.go").
// ImportPath is the raw import spec string as it appears in the source
// (e.g., "fmt", "os/exec", "react", "numpy"). It is NOT resolved or classified
// at extraction time; resolution to intra-repo unit ID, "ext:" facts, or
// facts_unresolved happens in the EdgeStore write path (L19.10).
// StartLine is the 1-based line number of the import statement in FromFile (G7 provenance).
type ImportEdge struct {
	FromFile   string // relative file path — never absolute (G7)
	ImportPath string // raw import spec, unresolved
	StartLine  uint32 // 1-based line number in FromFile (G7)
}

// FactParser is the extended parser interface that extracts both symbols and
// import edges from a source file in a single tree-sitter parse pass (G0:
// one traversal per file). Adapters that implement this interface register
// themselves for the C4 arch extraction path.
//
// ParseFileToMetaAndFacts parses the file once and returns:
//   - []*SymbolMeta: same output as ParseFileToMeta (zero regression)
//   - []ImportEdge:  one entry per top-level import statement, with G7 provenance
//   - error:         parse-level error; returns nil, nil, nil for unknown languages
//
// Callers MUST type-assert before using: `if fp, ok := p.(FactParser); ok { ... }`.
// Parsers that do not implement FactParser continue to work via ParseFileToMeta.
type FactParser interface {
	ParseFileToMetaAndFacts(path string, source []byte) ([]*SymbolMeta, []ImportEdge, error)
}
