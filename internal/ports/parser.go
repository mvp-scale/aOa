package ports

// Parser extracts structural symbols (functions, classes, methods) from source files.
// The concrete implementation (tree-sitter) lives in internal/adapters/treesitter.
// When nil, the system operates in tokenization-only mode: file-level search works,
// but no symbol names or line ranges are available.
type Parser interface {
	// ParseFileToMeta extracts symbols from a source file and returns them as
	// SymbolMeta entries suitable for the search index. Returns nil, nil for
	// unsupported languages (not an error).
	ParseFileToMeta(path string, source []byte) ([]*SymbolMeta, error)

	// SupportsExtension returns true if the parser can handle files with this
	// extension (e.g., ".go", ".py"). Extension includes the leading dot.
	SupportsExtension(ext string) bool
}
