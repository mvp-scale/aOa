//go:build !lean

package treesitter

// This file registers all compiled-in grammars. It is included in the default
// build (go build / go install) but excluded when building with -tags lean,
// which produces a binary that loads grammars dynamically from .so/.dylib files.

import (
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	// === Official tree-sitter/* org (Tier 1) ===
	ts_bash "github.com/tree-sitter/tree-sitter-bash/bindings/go"
	ts_c "github.com/tree-sitter/tree-sitter-c/bindings/go"
	ts_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
	ts_csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	ts_css "github.com/tree-sitter/tree-sitter-css/bindings/go"
	ts_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	ts_haskell "github.com/tree-sitter/tree-sitter-haskell/bindings/go"
	ts_html "github.com/tree-sitter/tree-sitter-html/bindings/go"
	ts_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
	ts_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	ts_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
	ts_ocaml "github.com/tree-sitter/tree-sitter-ocaml/bindings/go"
	ts_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	ts_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	ts_ruby "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
	ts_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	ts_scala "github.com/tree-sitter/tree-sitter-scala/bindings/go"
	ts_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
	ts_verilog "github.com/tree-sitter/tree-sitter-verilog/bindings/go"

	// === tree-sitter-grammars/* org (Tier 2 — curated) ===
	ts_cuda "github.com/tree-sitter-grammars/tree-sitter-cuda/bindings/go"
	ts_hcl "github.com/tree-sitter-grammars/tree-sitter-hcl/bindings/go"
	ts_kotlin "github.com/tree-sitter-grammars/tree-sitter-kotlin/bindings/go"
	ts_lua "github.com/tree-sitter-grammars/tree-sitter-lua/bindings/go"
	ts_svelte "github.com/tree-sitter-grammars/tree-sitter-svelte/bindings/go"
	ts_toml "github.com/tree-sitter-grammars/tree-sitter-toml/bindings/go"
	ts_yaml "github.com/tree-sitter-grammars/tree-sitter-yaml/bindings/go"
	ts_zig "github.com/tree-sitter-grammars/tree-sitter-zig/bindings/go"
)

// langPtr wraps a Language() call that returns unsafe.Pointer.
func langPtr(p unsafe.Pointer) *tree_sitter.Language {
	return tree_sitter.NewLanguage(p)
}

// registerBuiltinLanguages adds all compiled-in grammars to the parser.
func (p *Parser) registerBuiltinLanguages() {
	// Core (14)
	p.addLang("python", langPtr(ts_python.Language()))
	p.addLang("javascript", langPtr(ts_javascript.Language()))
	p.addLang("typescript", langPtr(ts_typescript.LanguageTypescript()))
	p.addLang("tsx", langPtr(ts_typescript.LanguageTSX()))
	p.addLang("go", langPtr(ts_go.Language()))
	p.addLang("rust", langPtr(ts_rust.Language()))
	p.addLang("java", langPtr(ts_java.Language()))
	p.addLang("c", langPtr(ts_c.Language()))
	p.addLang("cpp", langPtr(ts_cpp.Language()))
	p.addLang("csharp", langPtr(ts_csharp.Language()))
	p.addLang("ruby", langPtr(ts_ruby.Language()))
	p.addLang("php", langPtr(ts_php.LanguagePHP()))
	p.addLang("kotlin", langPtr(ts_kotlin.Language()))
	p.addLang("scala", langPtr(ts_scala.Language()))

	// Scripting & Shell
	p.addLang("bash", langPtr(ts_bash.Language()))
	p.addLang("lua", langPtr(ts_lua.Language()))

	// Functional
	p.addLang("haskell", langPtr(ts_haskell.Language()))
	p.addLang("ocaml", langPtr(ts_ocaml.LanguageOCaml()))

	// Systems & Emerging
	p.addLang("zig", langPtr(ts_zig.Language()))
	p.addLang("cuda", langPtr(ts_cuda.Language()))
	p.addLang("verilog", langPtr(ts_verilog.Language()))

	// Web & Frontend
	p.addLang("html", langPtr(ts_html.Language()))
	p.addLang("css", langPtr(ts_css.Language()))
	p.addLang("svelte", langPtr(ts_svelte.Language()))

	// Data & Config
	p.addLang("json", langPtr(ts_json.Language()))
	p.addLang("yaml", langPtr(ts_yaml.Language()))
	p.addLang("toml", langPtr(ts_toml.Language()))
	p.addLang("hcl", langPtr(ts_hcl.Language()))
}
