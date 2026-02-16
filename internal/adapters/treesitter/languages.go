package treesitter

// This file registers all 56 built-in grammars and their symbol extraction rules.
// Each grammar is a Go module pulled via `go get` — the C source compiles into the binary via CGo.
//
// To add a new language:
// 1. go get github.com/{org}/tree-sitter-{lang}@latest
// 2. Add import + Language() call in registerBuiltinLanguages()
// 3. Add extension mappings in registerExtensions()
// 4. Add symbol node types in registerSymbolNodes()

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

	// NOTE: The following grammars are in go.mod but don't have bindings/go
	// or have incomplete bindings (missing scanner.c):
	// r (bindings exist but scanner.c not included - linker errors)
	// julia, vue, markdown (no bindings/go directory)
	// They're registered as extensions for tokenization, but tree-sitter
	// parsing isn't available until upstream fixes bindings/go
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

	// Scripting & Shell (5)
	p.addLang("bash", langPtr(ts_bash.Language()))
	p.addLang("lua", langPtr(ts_lua.Language()))

	// Functional (2)
	p.addLang("haskell", langPtr(ts_haskell.Language()))
	p.addLang("ocaml", langPtr(ts_ocaml.LanguageOCaml()))

	// Systems & Emerging (3)
	p.addLang("zig", langPtr(ts_zig.Language()))
	p.addLang("cuda", langPtr(ts_cuda.Language()))
	p.addLang("verilog", langPtr(ts_verilog.Language()))

	// Web & Frontend (3)
	p.addLang("html", langPtr(ts_html.Language()))
	p.addLang("css", langPtr(ts_css.Language()))
	p.addLang("svelte", langPtr(ts_svelte.Language()))

	// Data & Config (4)
	p.addLang("json", langPtr(ts_json.Language()))
	p.addLang("yaml", langPtr(ts_yaml.Language()))
	p.addLang("toml", langPtr(ts_toml.Language()))
	p.addLang("hcl", langPtr(ts_hcl.Language()))

	// NOTE: The following grammars are in go.mod but their bindings/go
	// subpackage doesn't exist or has incompatible module structure.
	// They're registered as extension mappings so files are still indexed
	// by tokenization — just without tree-sitter structural parsing.
	// When their upstream repos add bindings/go, we add one line here.
	//
	// Awaiting bindings/go:
	//   swift, perl, elixir, erlang, dart, nim, clojure, d,
	//   gleam, elm, purescript, odin, v, ada, fortran, fennel,
	//   groovy, graphql, cmake, make, svelte, nix, objc,
	//   vhdl, glsl, hlsl, yaml, toml, markdown, sql, dockerfile
}

// registerExtensions maps file extensions to language names.
// All 97 extensions from Python aOa parity.
func (p *Parser) registerExtensions() {
	// Core
	p.addExt("python", ".py", ".pyw")
	p.addExt("javascript", ".js", ".jsx", ".mjs", ".cjs")
	p.addExt("typescript", ".ts", ".mts")
	p.addExt("tsx", ".tsx")
	p.addExt("go", ".go")
	p.addExt("rust", ".rs")
	p.addExt("java", ".java")
	p.addExt("c", ".c", ".h")
	p.addExt("cpp", ".cpp", ".hpp", ".cc", ".cxx", ".hxx")
	p.addExt("csharp", ".cs")
	p.addExt("ruby", ".rb")
	p.addExt("php", ".php")
	p.addExt("swift", ".swift")
	p.addExt("kotlin", ".kt", ".kts")
	p.addExt("scala", ".scala", ".sc")

	// Scripting
	p.addExt("bash", ".sh", ".bash", ".zsh")
	p.addExt("lua", ".lua")
	p.addExt("perl", ".pl", ".pm")
	p.addExt("r", ".r", ".R")
	p.addExt("julia", ".jl")
	p.addExt("elixir", ".ex", ".exs")
	p.addExt("erlang", ".erl", ".hrl")

	// Functional
	p.addExt("haskell", ".hs", ".lhs")
	p.addExt("ocaml", ".ml", ".mli")
	p.addExt("gleam", ".gleam")
	p.addExt("elm", ".elm")
	p.addExt("clojure", ".clj", ".cljs", ".cljc")
	p.addExt("purescript", ".purs")
	p.addExt("fennel", ".fnl")

	// Systems & Emerging
	p.addExt("zig", ".zig")
	p.addExt("d", ".d")
	p.addExt("cuda", ".cu", ".cuh")
	p.addExt("odin", ".odin")
	p.addExt("v", ".v")
	p.addExt("nim", ".nim")
	p.addExt("objc", ".m", ".mm")
	p.addExt("ada", ".ada", ".adb", ".ads")
	p.addExt("fortran", ".f90", ".f95", ".f03", ".f")
	p.addExt("verilog", ".sv")
	p.addExt("vhdl", ".vhd", ".vhdl")

	// Web & Frontend
	p.addExt("html", ".html", ".htm")
	p.addExt("css", ".css", ".scss", ".less")
	p.addExt("vue", ".vue")
	p.addExt("svelte", ".svelte")
	p.addExt("dart", ".dart")

	// Data & Config
	p.addExt("json", ".json", ".jsonc")
	p.addExt("yaml", ".yaml", ".yml")
	p.addExt("toml", ".toml")
	p.addExt("sql", ".sql")
	p.addExt("markdown", ".md", ".mdx")
	p.addExt("graphql", ".graphql", ".gql")
	p.addExt("hcl", ".tf", ".hcl")
	p.addExt("dockerfile", "Dockerfile", ".dockerfile")
	p.addExt("nix", ".nix")

	// Build
	p.addExt("cmake", ".cmake")
	p.addExt("make", ".mk")
	p.addExt("groovy", ".groovy", ".gradle")
	p.addExt("glsl", ".glsl", ".vert", ".frag")
	p.addExt("hlsl", ".hlsl")
}

// symbolRules maps language names to their symbol extraction node types.
// This is the data-driven equivalent of Python's SYMBOL_NODES dict.
var symbolRules = map[string]map[string]string{
	"python": {
		"function_definition": "function",
		"class_definition":    "class",
	},
	"javascript": {
		"function_declaration": "function",
		"class_declaration":    "class",
		"method_definition":    "method",
		"arrow_function":       "function",
	},
	"typescript": {
		"function_declaration":  "function",
		"class_declaration":     "class",
		"method_definition":     "method",
		"arrow_function":        "function",
		"interface_declaration": "interface",
	},
	"tsx": {
		"function_declaration":  "function",
		"class_declaration":     "class",
		"method_definition":     "method",
		"arrow_function":        "function",
		"interface_declaration": "interface",
	},
	"go": {
		"function_declaration": "function",
		"method_declaration":   "method",
		"type_declaration":     "type",
	},
	"rust": {
		"function_item": "function",
		"impl_item":     "impl",
		"struct_item":   "struct",
		"enum_item":     "enum",
		"trait_item":    "trait",
	},
	"java": {
		"method_declaration":    "method",
		"class_declaration":     "class",
		"interface_declaration": "interface",
	},
	"c": {
		"function_definition": "function",
		"struct_specifier":    "struct",
	},
	"cpp": {
		"function_definition": "function",
		"class_specifier":     "class",
		"struct_specifier":    "struct",
	},
	"ruby": {
		"method":  "method",
		"class":   "class",
		"module":  "module",
		"comment": "",
	},
	"php": {
		"function_definition": "function",
		"method_declaration":  "method",
		"class_declaration":   "class",
		"interface_declaration": "interface",
		"trait_declaration":     "trait",
	},
	"swift": {
		"function_declaration": "function",
		"class_declaration":    "class",
		"struct_declaration":   "struct",
		"protocol_declaration": "protocol",
	},
	"kotlin": {
		"function_declaration":  "function",
		"class_declaration":     "class",
		"object_declaration":    "object",
		"interface_declaration": "interface",
	},
	"scala": {
		"function_definition": "function",
		"class_definition":    "class",
		"object_definition":   "object",
		"trait_definition":    "trait",
	},
	"bash": {
		"function_definition": "function",
	},
	"lua": {
		"function_declaration":      "function",
		"function_definition_statement": "function",
	},
	"elixir": {
		"call": "function", // def/defp/defmodule are calls in elixir AST
	},
	"haskell": {
		"function":    "function",
		"data":        "type",
		"type_alias":  "type",
		"class":       "class",
	},
	"r": {
		"function_definition": "function",
	},
	"julia": {
		"function_definition": "function",
		"struct_definition":   "struct",
		"module_definition":   "module",
	},
	"zig": {
		"fn_decl": "function",
		"test_decl": "test",
	},
	"dart": {
		"function_signature":    "function",
		"method_signature":      "method",
		"class_definition":      "class",
	},
	"csharp": {
		"method_declaration": "method",
		"class_declaration":  "class",
		"interface_declaration": "interface",
		"struct_declaration":    "struct",
	},
	"ocaml": {
		"let_binding":      "function",
		"type_definition":  "type",
		"module_definition": "module",
	},
	"hcl": {
		"block": "block",
	},
	"sql": {
		"create_table_statement": "table",
		"create_function_statement": "function",
	},
	"graphql": {
		"type_definition":      "type",
		"input_type_definition": "input",
		"enum_type_definition":  "enum",
	},
	"verilog": {
		"module_declaration": "module",
	},
	"cuda": {
		"function_definition": "function",
		"struct_specifier":    "struct",
	},
}
