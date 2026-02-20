//go:build lean

package treesitter

// This file is included only when building with -tags lean.
// It provides an empty registerBuiltinLanguages() — all grammars are loaded
// dynamically from .so/.dylib files via the DynamicLoader (purego).
//
// Build with: go build -tags lean ./cmd/aoa/

// registerBuiltinLanguages is a no-op in lean builds.
// All grammar loading happens through the DynamicLoader.
func (p *Parser) registerBuiltinLanguages() {}
