package treesitter

// This file registers file extension mappings and symbol extraction rules.
// These are always included regardless of build tags — they're needed for
// both compiled-in and dynamically-loaded grammars.

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
	"csharp": {
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
