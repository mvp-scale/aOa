package treesitter

import "sync"

// This file registers file extension mappings and symbol extraction rules.
// These are always included regardless of build tags — they're needed for
// both compiled-in and dynamically-loaded grammars.

// extensionMap is a lazily-initialized static map from file extension to language.
var (
	extensionMapOnce sync.Once
	extensionMap     map[string]string
)

// buildExtensionMap constructs the static extension map from the same data
// used by Parser.registerExtensions. This allows extension lookups without
// requiring a Parser instance.
func buildExtensionMap() map[string]string {
	m := make(map[string]string, 130)
	add := func(lang string, exts ...string) {
		for _, ext := range exts {
			m[ext] = lang
		}
	}

	// Core
	add("python", ".py", ".pyw")
	add("javascript", ".js", ".jsx", ".mjs", ".cjs")
	add("typescript", ".ts", ".mts")
	add("tsx", ".tsx")
	add("go", ".go")
	add("rust", ".rs")
	add("java", ".java")
	add("c", ".c", ".h")
	add("cpp", ".cpp", ".hpp", ".cc", ".cxx", ".hxx")
	add("c_sharp", ".cs")
	add("ruby", ".rb")
	add("php", ".php")
	add("swift", ".swift")
	add("kotlin", ".kt", ".kts")
	add("scala", ".scala", ".sc")

	// Scripting
	add("bash", ".sh", ".bash")
	add("lua", ".lua")
	add("perl", ".pl", ".pm")
	add("r", ".r", ".R")
	add("julia", ".jl")
	add("elixir", ".ex", ".exs")
	add("erlang", ".erl", ".hrl")

	// Functional
	add("haskell", ".hs", ".lhs")
	add("ocaml", ".ml", ".mli")
	add("gleam", ".gleam")
	add("elm", ".elm")
	add("clojure", ".clj", ".cljs", ".cljc")
	add("purescript", ".purs")
	add("fennel", ".fnl")

	// Systems
	add("zig", ".zig")
	add("d", ".d")
	add("cuda", ".cu", ".cuh")
	add("odin", ".odin")
	add("nim", ".nim")
	add("objc", ".m", ".mm")
	add("ada", ".ada", ".adb", ".ads")
	add("fortran", ".f90", ".f95", ".f03", ".f")
	add("verilog", ".sv")
	add("vhdl", ".vhd", ".vhdl")

	// Web
	add("html", ".html", ".htm")
	add("css", ".css", ".less")
	add("scss", ".scss")
	add("vue", ".vue")
	add("svelte", ".svelte")
	add("dart", ".dart")

	// Data & Config
	add("json", ".json")
	add("jsonc", ".jsonc")
	add("yaml", ".yaml", ".yml")
	add("toml", ".toml")
	add("sql", ".sql")
	add("markdown", ".md", ".mdx")
	add("graphql", ".graphql", ".gql")
	add("hcl", ".tf", ".hcl")
	add("dockerfile", "Dockerfile", ".dockerfile")
	add("nix", ".nix")

	// Build & Infra
	add("cmake", ".cmake")
	add("make", ".mk")
	add("groovy", ".groovy", ".gradle")
	add("glsl", ".glsl", ".vert", ".frag")
	add("hlsl", ".hlsl")

	return m
}

// ExtensionToLanguage returns the tree-sitter language name for a file extension
// (e.g. ".py" -> "python", ".go" -> "go"). Returns "" for unknown extensions.
// This is safe to call without a Parser instance.
func ExtensionToLanguage(ext string) string {
	extensionMapOnce.Do(func() {
		extensionMap = buildExtensionMap()
	})
	return extensionMap[ext]
}

// registerExtensions maps file extensions to language names.
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
	p.addExt("c_sharp", ".cs")
	p.addExt("ruby", ".rb")
	p.addExt("php", ".php")
	p.addExt("swift", ".swift")
	p.addExt("kotlin", ".kt", ".kts")
	p.addExt("scala", ".scala", ".sc")

	// Scripting
	p.addExt("bash", ".sh", ".bash")
	p.addExt("lua", ".lua")
	p.addExt("perl", ".pl", ".pm")
	p.addExt("r", ".r", ".R")
	p.addExt("julia", ".jl")
	p.addExt("elixir", ".ex", ".exs")
	p.addExt("erlang", ".erl", ".hrl")
	p.addExt("awk", ".awk")
	p.addExt("fish", ".fish")
	p.addExt("nu", ".nu")
	p.addExt("powershell", ".ps1", ".psm1", ".psd1")
	p.addExt("tcl", ".tcl")

	// Functional
	p.addExt("haskell", ".hs", ".lhs")
	p.addExt("ocaml", ".ml", ".mli")
	p.addExt("gleam", ".gleam")
	p.addExt("elm", ".elm")
	p.addExt("clojure", ".clj", ".cljs", ".cljc")
	p.addExt("purescript", ".purs")
	p.addExt("fennel", ".fnl")
	p.addExt("fsharp", ".fs", ".fsx", ".fsi")
	p.addExt("scheme", ".scm", ".ss")
	p.addExt("racket", ".rkt")
	p.addExt("commonlisp", ".lisp", ".cl", ".lsp")
	p.addExt("sml", ".sml", ".sig")
	p.addExt("rescript", ".res", ".resi")

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
	p.addExt("systemverilog", ".svh")
	p.addExt("vhdl", ".vhd", ".vhdl")
	p.addExt("pascal", ".pas", ".pp")
	p.addExt("crystal", ".cr")
	p.addExt("hare", ".ha")
	p.addExt("cairo", ".cairo")

	// Web & Frontend
	p.addExt("html", ".html", ".htm")
	p.addExt("css", ".css", ".less")
	p.addExt("scss", ".scss")
	p.addExt("vue", ".vue")
	p.addExt("svelte", ".svelte")
	p.addExt("dart", ".dart")
	p.addExt("astro", ".astro")
	p.addExt("pug", ".pug")
	p.addExt("slim", ".slim")
	p.addExt("haml", ".haml")
	p.addExt("embedded_template", ".erb")
	p.addExt("heex", ".heex")

	// Data & Config
	p.addExt("json", ".json")
	p.addExt("jsonc", ".jsonc")
	p.addExt("json5", ".json5")
	p.addExt("yaml", ".yaml", ".yml")
	p.addExt("toml", ".toml")
	p.addExt("sql", ".sql")
	p.addExt("markdown", ".md", ".mdx")
	p.addExt("graphql", ".graphql", ".gql")
	p.addExt("hcl", ".tf", ".hcl")
	p.addExt("dockerfile", "Dockerfile", ".dockerfile")
	p.addExt("nix", ".nix")
	p.addExt("xml", ".xml", ".xsl", ".xslt")
	p.addExt("csv", ".csv")
	p.addExt("ini", ".ini")
	p.addExt("proto", ".proto")
	p.addExt("latex", ".tex", ".latex")
	p.addExt("kdl", ".kdl")

	// Build & Infra
	p.addExt("cmake", ".cmake")
	p.addExt("make", ".mk")
	p.addExt("groovy", ".groovy", ".gradle")
	p.addExt("glsl", ".glsl", ".vert", ".frag")
	p.addExt("hlsl", ".hlsl")
	p.addExt("wgsl", ".wgsl")
	p.addExt("just", "Justfile", ".just")
	p.addExt("ninja", ".ninja")
	p.addExt("meson", ".meson")

	// Blockchain & Smart Contracts
	p.addExt("solidity", ".sol")
	p.addExt("prisma", ".prisma")

	// Game Dev
	p.addExt("gdscript", ".gd")
	p.addExt("godot_resource", ".tscn", ".tres")

	// Misc
	p.addExt("vim", ".vim")
	p.addExt("typst", ".typ")
	p.addExt("pkl", ".pkl")
	p.addExt("asm", ".asm", ".s", ".S")
	p.addExt("nasm", ".nasm")
	p.addExt("starlark", ".bzl", ".star")
	p.addExt("cobol", ".cob", ".cbl")
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
		"function_definition":  "function",
		"method_declaration":   "method",
		"class_declaration":    "class",
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
		"function_declaration":          "function",
		"function_definition_statement": "function",
	},
	"elixir": {
		"call": "function", // def/defp/defmodule are calls in elixir AST
	},
	"haskell": {
		"function":   "function",
		"data":       "type",
		"type_alias": "type",
		"class":      "class",
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
		"fn_decl":   "function",
		"test_decl": "test",
	},
	"dart": {
		"function_signature": "function",
		"method_signature":   "method",
		"class_definition":   "class",
	},
	"c_sharp": {
		"method_declaration":    "method",
		"class_declaration":     "class",
		"interface_declaration": "interface",
		"struct_declaration":    "struct",
	},
	"ocaml": {
		"let_binding":       "function",
		"type_definition":   "type",
		"module_definition": "module",
	},
	"hcl": {
		"block": "block",
	},
	"sql": {
		"create_table_statement":    "table",
		"create_function_statement": "function",
	},
	"graphql": {
		"type_definition":       "type",
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
