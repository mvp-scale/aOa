package analyzer

// Concept names for unified AST queries across languages.
const (
	ConceptCall          = "call"
	ConceptStringLiteral = "string_literal"
	ConceptStringConcat  = "string_concat"
	ConceptAssignment    = "assignment"
	ConceptForLoop       = "for_loop"
	ConceptDefer         = "defer"
	ConceptReturn        = "return"
	ConceptImport        = "import"
	ConceptFunction      = "function"
	ConceptClass         = "class"
)

// langMap maps (language, concept) -> list of AST node kinds.
// Each language maps unified concepts to the concrete tree-sitter node types.
var langMap = map[string]map[string][]string{
	"go": {
		ConceptCall:          {"call_expression"},
		ConceptStringLiteral: {"interpreted_string_literal", "raw_string_literal"},
		ConceptStringConcat:  {"binary_expression"},
		ConceptAssignment:    {"short_var_declaration", "assignment_statement"},
		ConceptForLoop:       {"for_statement"},
		ConceptDefer:         {"defer_statement"},
		ConceptReturn:        {"return_statement"},
		ConceptImport:        {"import_declaration"},
		ConceptFunction:      {"function_declaration", "method_declaration"},
		ConceptClass:         {"type_declaration"},
	},
	"python": {
		ConceptCall:          {"call"},
		ConceptStringLiteral: {"string", "concatenated_string"},
		ConceptStringConcat:  {"binary_operator"},
		ConceptAssignment:    {"assignment", "augmented_assignment"},
		ConceptForLoop:       {"for_statement", "while_statement"},
		ConceptReturn:        {"return_statement"},
		ConceptImport:        {"import_statement", "import_from_statement"},
		ConceptFunction:      {"function_definition"},
		ConceptClass:         {"class_definition"},
	},
	"javascript": {
		ConceptCall:          {"call_expression"},
		ConceptStringLiteral: {"string", "template_string"},
		ConceptStringConcat:  {"binary_expression", "template_string"},
		ConceptAssignment:    {"variable_declarator", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "while_statement", "for_in_statement"},
		ConceptReturn:        {"return_statement"},
		ConceptImport:        {"import_statement"},
		ConceptFunction:      {"function_declaration", "arrow_function", "method_definition"},
		ConceptClass:         {"class_declaration"},
	},
	"typescript": {
		ConceptCall:          {"call_expression"},
		ConceptStringLiteral: {"string", "template_string"},
		ConceptStringConcat:  {"binary_expression", "template_string"},
		ConceptAssignment:    {"variable_declarator", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "while_statement", "for_in_statement"},
		ConceptReturn:        {"return_statement"},
		ConceptImport:        {"import_statement"},
		ConceptFunction:      {"function_declaration", "arrow_function", "method_definition"},
		ConceptClass:         {"class_declaration"},
	},
	"tsx": {
		ConceptCall:          {"call_expression"},
		ConceptStringLiteral: {"string", "template_string"},
		ConceptStringConcat:  {"binary_expression", "template_string"},
		ConceptAssignment:    {"variable_declarator", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "while_statement", "for_in_statement"},
		ConceptReturn:        {"return_statement"},
		ConceptImport:        {"import_statement"},
		ConceptFunction:      {"function_declaration", "arrow_function", "method_definition"},
		ConceptClass:         {"class_declaration"},
	},
	"rust": {
		ConceptCall:          {"call_expression"},
		ConceptStringLiteral: {"string_literal", "raw_string_literal"},
		ConceptStringConcat:  {"binary_expression"},
		ConceptAssignment:    {"let_declaration", "assignment_expression"},
		ConceptForLoop:       {"for_expression", "while_expression", "loop_expression"},
		ConceptReturn:        {"return_expression"},
		ConceptImport:        {"use_declaration"},
		ConceptFunction:      {"function_item"},
		ConceptClass:         {"struct_item", "enum_item", "impl_item"},
	},
	"java": {
		ConceptCall:          {"method_invocation"},
		ConceptStringLiteral: {"string_literal"},
		ConceptStringConcat:  {"binary_expression"},
		ConceptAssignment:    {"local_variable_declaration", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "enhanced_for_statement", "while_statement"},
		ConceptReturn:        {"return_statement"},
		ConceptImport:        {"import_declaration"},
		ConceptFunction:      {"method_declaration", "constructor_declaration"},
		ConceptClass:         {"class_declaration", "interface_declaration"},
	},
	"c": {
		ConceptCall:          {"call_expression"},
		ConceptStringLiteral: {"string_literal"},
		ConceptStringConcat:  {"binary_expression"},
		ConceptAssignment:    {"declaration", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "while_statement", "do_statement"},
		ConceptReturn:        {"return_statement"},
		ConceptImport:        {"preproc_include"},
		ConceptFunction:      {"function_definition"},
		ConceptClass:         {"struct_specifier"},
	},
	"cpp": {
		ConceptCall:          {"call_expression"},
		ConceptStringLiteral: {"string_literal", "raw_string_literal"},
		ConceptStringConcat:  {"binary_expression"},
		ConceptAssignment:    {"declaration", "assignment_expression"},
		ConceptForLoop:       {"for_statement", "while_statement", "for_range_loop"},
		ConceptReturn:        {"return_statement"},
		ConceptImport:        {"preproc_include"},
		ConceptFunction:      {"function_definition"},
		ConceptClass:         {"class_specifier", "struct_specifier"},
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
	},
}

// Resolve returns the AST node kinds for a given language and concept.
// Returns nil for unknown languages or concepts (graceful degradation).
func Resolve(lang, concept string) []string {
	concepts, ok := langMap[lang]
	if !ok {
		return nil
	}
	return concepts[concept]
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

// SupportedLanguages returns all languages with map entries.
func SupportedLanguages() []string {
	langs := make([]string, 0, len(langMap))
	for lang := range langMap {
		langs = append(langs, lang)
	}
	return langs
}
