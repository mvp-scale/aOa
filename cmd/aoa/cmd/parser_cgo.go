//go:build cgo

package cmd

import (
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/ports"
)

// newParser returns a tree-sitter parser when CGo is available.
// If root is non-empty, configures dynamic grammar loading from
// project-local and global grammar directories.
func newParser(root string) ports.Parser {
	p := treesitter.NewParser()
	if root != "" {
		p.SetGrammarPaths(treesitter.DefaultGrammarPaths(root))
	}
	return p
}
