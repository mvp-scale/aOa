//go:build cgo

package cmd

import (
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/ports"
)

// newParser returns a tree-sitter parser when CGo is available.
func newParser() ports.Parser {
	return treesitter.NewParser()
}
