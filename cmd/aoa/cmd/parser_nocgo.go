//go:build !cgo

package cmd

import "github.com/corey/aoa/internal/ports"

// newParser returns nil when CGo is unavailable (pure Go build).
// The system operates in tokenization-only mode: file-level search works,
// symbol-level search requires aoa-recon to enhance the index.
func newParser(_ string) ports.Parser {
	return nil
}
