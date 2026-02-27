//go:build !cgo

package cmd

// scanAndDownloadGrammars is a no-op without CGo.
// Tree-sitter grammars require CGo to load.
func scanAndDownloadGrammars(_ string) {}
