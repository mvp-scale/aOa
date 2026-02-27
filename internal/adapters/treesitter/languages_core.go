//go:build core

package treesitter

// This file is included only when building with -tags core.
// The tree-sitter C runtime compiles in (via go-tree-sitter CGo), but no
// grammar packages are imported — all grammars are loaded dynamically from
// .so/.dylib files via the DynamicLoader (purego).
//
// Build with: ./build.sh --core

// registerBuiltinLanguages is a no-op in core builds.
// All grammar loading happens through the DynamicLoader.
func (p *Parser) registerBuiltinLanguages() {}
