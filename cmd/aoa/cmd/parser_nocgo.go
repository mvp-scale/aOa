//go:build !cgo

package cmd

import "github.com/corey/aoa/internal/ports"

// newParser returns nil when CGo is unavailable (lean build).
// The system operates in tokenization-only mode: file-level search works,
// symbol-level search requires the core build with tree-sitter.
func newParser(_ string) ports.Parser {
	return nil
}
