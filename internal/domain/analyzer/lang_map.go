package analyzer

// Concept names for unified AST queries across languages.
const (
	ConceptCall            = "call"
	ConceptStringLiteral   = "string_literal"
	ConceptStringConcat    = "string_concat"
	ConceptAssignment      = "assignment"
	ConceptForLoop         = "for_loop"
	ConceptDefer           = "defer"
	ConceptReturn          = "return"
	ConceptImport          = "import"
	ConceptFunction        = "function"
	ConceptClass           = "class"
	ConceptBlock           = "block"
	ConceptSwitch          = "switch"
	ConceptFormatCall      = "format_call"
	ConceptTypeAssertion   = "type_assertion"
	ConceptInterface       = "interface"
)

// conceptDefaults provides universal AST node kinds for all 509 tree-sitter languages.
// These defaults cover the most common grammar conventions. Languages that diverge
// from these defaults are listed in langOverrides.
var conceptDefaults = map[string][]string{
	ConceptCall:          {"call_expression"},
	ConceptStringLiteral: {"string_literal"},
	ConceptStringConcat:  {"binary_expression"},
	ConceptAssignment:    {"assignment_expression", "variable_declarator"},
	ConceptForLoop:       {"for_statement", "while_statement"},
	ConceptDefer:         {"defer_statement"},
	ConceptReturn:        {"return_statement"},
	ConceptImport:        {"import_statement", "import_declaration"},
	ConceptFunction:      {"function_declaration", "function_definition"},
	ConceptClass:         {"class_declaration", "struct_specifier"},
	ConceptBlock:         {"block"},
	ConceptSwitch:        {"switch_statement"},
	ConceptFormatCall:    {"macro_invocation"},
	ConceptTypeAssertion: {"type_assertion_expression"},
	ConceptInterface:     {"interface_declaration"},
}

// langOverrides contains only the languages that diverge from conceptDefaults.
// If a language is not listed here, it uses the universal defaults for all concepts.
// If a language IS listed but a specific concept is not overridden, the default is used.
var langOverrides = map[string]map[string][]string{
	"go": {
		ConceptStringLiteral: {"interpreted_string_literal", "raw_string_literal"},
		ConceptAssignment:    {"short_var_declaration", "assignment_statement"},
		ConceptForLoop:       {"for_statement"},
		ConceptImport:        {"import_declaration"},
		ConceptFunction:      {"function_declaration", "method_declaration"},
		ConceptClass:         {"type_declaration"},
		ConceptSwitch:        {"switch_statement", "select_statement", "expression_switch_statement", "type_switch_statement"},
		ConceptInterface:     {"interface_type"},
	},
	"python": {
		ConceptCall:          {"call"},
		ConceptStringLiteral: {"string", "concatenated_string"},
		ConceptStringConcat:  {"binary_operator"},
		ConceptAssignment:    {"assignment", "augmented_assignment"},
		ConceptImport:        {"import_statement", "import_from_statement"},
		ConceptFunction:      {"function_definition"},
		ConceptClass:         {"class_definition"},
		ConceptSwitch:        {"match_statement"},
	},
	"javascript": {
		ConceptStringLiteral: {"string", "template_string"},
		ConceptStringConcat:  {"binary_expression", "template_string"},
		ConceptAssignment:    {"variable_declarator", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "while_statement", "for_in_statement"},
		ConceptFunction:      {"function_declaration", "arrow_function", "method_definition"},
		ConceptBlock:         {"statement_block"},
	},
	"typescript": {
		ConceptStringLiteral: {"string", "template_string"},
		ConceptStringConcat:  {"binary_expression", "template_string"},
		ConceptAssignment:    {"variable_declarator", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "while_statement", "for_in_statement"},
		ConceptFunction:      {"function_declaration", "arrow_function", "method_definition"},
		ConceptBlock:         {"statement_block"},
	},
	"tsx": {
		ConceptStringLiteral: {"string", "template_string"},
		ConceptStringConcat:  {"binary_expression", "template_string"},
		ConceptAssignment:    {"variable_declarator", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "while_statement", "for_in_statement"},
		ConceptFunction:      {"function_declaration", "arrow_function", "method_definition"},
		ConceptBlock:         {"statement_block"},
	},
	"rust": {
		ConceptStringLiteral: {"string_literal", "raw_string_literal"},
		ConceptAssignment:    {"let_declaration", "assignment_expression"},
		ConceptForLoop:       {"for_expression", "while_expression", "loop_expression"},
		ConceptReturn:        {"return_expression"},
		ConceptImport:        {"use_declaration"},
		ConceptFunction:      {"function_item"},
		ConceptClass:         {"struct_item", "enum_item", "impl_item"},
		ConceptSwitch:        {"match_expression"},
	},
	"java": {
		ConceptCall:          {"method_invocation"},
		ConceptAssignment:    {"local_variable_declaration", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "enhanced_for_statement", "while_statement"},
		ConceptFunction:      {"method_declaration", "constructor_declaration"},
		ConceptClass:         {"class_declaration", "interface_declaration"},
		ConceptSwitch:        {"switch_expression", "switch_statement"},
		ConceptInterface:     {"interface_declaration"},
	},
	"c": {
		ConceptAssignment: {"declaration", "assignment_expression"},
		ConceptForLoop:    {"for_statement", "while_statement", "do_statement"},
		ConceptImport:     {"preproc_include"},
		ConceptFunction:   {"function_definition"},
		ConceptBlock:      {"compound_statement"},
	},
	"cpp": {
		ConceptStringLiteral: {"string_literal", "raw_string_literal"},
		ConceptAssignment:    {"declaration", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "while_statement", "for_range_loop"},
		ConceptImport:        {"preproc_include"},
		ConceptFunction:      {"function_definition"},
		ConceptClass:         {"class_specifier", "struct_specifier"},
		ConceptBlock:         {"compound_statement"},
	},
	"ruby": {
		ConceptCall:          {"call", "method_call"},
		ConceptStringLiteral: {"string", "string_content"},
		ConceptStringConcat:  {"binary"},
		ConceptAssignment:    {"assignment"},
		ConceptForLoop:       {"for", "while", "until"},
		ConceptReturn:        {"return"},
		ConceptImport:        {"call"}, // require/require_relative are calls
		ConceptFunction:      {"method", "singleton_method"},
		ConceptClass:         {"class", "module"},
		ConceptBlock:         {"body_statement"},
		ConceptSwitch:        {"case"},
	},
}

// Resolve returns the AST node kinds for a given language and concept.
// Checks per-language overrides first, then falls back to universal defaults.
// Returns nil only for unknown concepts (not for unknown languages).
func Resolve(lang, concept string) []string {
	if overrides, ok := langOverrides[lang]; ok {
		if kinds, ok := overrides[concept]; ok {
			return kinds
		}
	}
	return conceptDefaults[concept]
}

// IsNodeKind returns true if the given AST node kind matches the concept
// for the specified language.
func IsNodeKind(lang, concept, kind string) bool {
	kinds := Resolve(lang, concept)
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// SupportedLanguages returns languages with explicit overrides.
// All 509 tree-sitter languages are supported via universal defaults;
// this returns only the ones with language-specific overrides.
func SupportedLanguages() []string {
	langs := make([]string, 0, len(langOverrides))
	for lang := range langOverrides {
		langs = append(langs, lang)
	}
	return langs
}
