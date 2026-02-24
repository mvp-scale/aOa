package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolve_GoCallExpression(t *testing.T) {
	kinds := Resolve("go", ConceptCall)
	assert.Equal(t, []string{"call_expression"}, kinds)
}

func TestResolve_PythonCall(t *testing.T) {
	kinds := Resolve("python", ConceptCall)
	assert.Equal(t, []string{"call"}, kinds)
}

func TestResolve_JSStringLiteral(t *testing.T) {
	kinds := Resolve("javascript", ConceptStringLiteral)
	assert.Contains(t, kinds, "string")
	assert.Contains(t, kinds, "template_string")
}

func TestResolve_UnknownLanguage_ReturnsDefaults(t *testing.T) {
	// Universal defaults: unknown languages get conceptDefaults instead of nil
	kinds := Resolve("cobol", ConceptCall)
	assert.Equal(t, []string{"call_expression"}, kinds, "unknown language should get default call concept")
}

func TestResolve_UnknownConcept(t *testing.T) {
	kinds := Resolve("go", "nonexistent_concept")
	assert.Nil(t, kinds)
}

func TestResolve_UniversalDefaults(t *testing.T) {
	// Languages without explicit overrides should get universal defaults
	testCases := []struct {
		lang    string
		concept string
		want    []string
	}{
		{"php", ConceptCall, []string{"call_expression"}},
		{"php", ConceptFunction, []string{"function_declaration", "function_definition"}},
		{"kotlin", ConceptAssignment, []string{"assignment_expression", "variable_declarator"}},
		{"swift", ConceptForLoop, []string{"for_statement", "while_statement"}},
		{"elixir", ConceptReturn, []string{"return_statement"}},
		{"dart", ConceptClass, []string{"class_declaration", "struct_specifier"}},
	}
	for _, tc := range testCases {
		kinds := Resolve(tc.lang, tc.concept)
		assert.Equal(t, tc.want, kinds, "%s/%s should return universal default", tc.lang, tc.concept)
	}
}

func TestResolve_OverridesPrecedeDefaults(t *testing.T) {
	// Python overrides call to "call", not the default "call_expression"
	assert.Equal(t, []string{"call"}, Resolve("python", ConceptCall))

	// Go overrides assignment, not the default
	assert.Equal(t, []string{"short_var_declaration", "assignment_statement"}, Resolve("go", ConceptAssignment))

	// Java overrides call to method_invocation
	assert.Equal(t, []string{"method_invocation"}, Resolve("java", ConceptCall))

	// Go falls through to default for call (no call override in Go)
	assert.Equal(t, []string{"call_expression"}, Resolve("go", ConceptCall))
}

func TestResolve_OverrideLangFallsToDefault(t *testing.T) {
	// Go has overrides but NOT for ConceptDefer — should get default
	assert.Equal(t, []string{"defer_statement"}, Resolve("go", ConceptDefer))

	// Python has overrides but NOT for ConceptDefer — should get default
	assert.Equal(t, []string{"defer_statement"}, Resolve("python", ConceptDefer))
}

func TestResolve_AllConceptsForGo(t *testing.T) {
	concepts := []string{
		ConceptCall, ConceptStringLiteral, ConceptStringConcat,
		ConceptAssignment, ConceptForLoop, ConceptDefer,
		ConceptReturn, ConceptImport, ConceptFunction, ConceptClass,
	}
	for _, c := range concepts {
		kinds := Resolve("go", c)
		assert.NotNil(t, kinds, "Go should have kinds for concept %s", c)
	}
}

func TestResolve_AllConceptsForPython(t *testing.T) {
	concepts := []string{
		ConceptCall, ConceptStringLiteral, ConceptStringConcat,
		ConceptAssignment, ConceptForLoop,
		ConceptReturn, ConceptImport, ConceptFunction, ConceptClass,
	}
	for _, c := range concepts {
		kinds := Resolve("python", c)
		assert.NotNil(t, kinds, "Python should have kinds for concept %s", c)
	}
}

func TestResolve_AllConceptsForJS(t *testing.T) {
	concepts := []string{
		ConceptCall, ConceptStringLiteral, ConceptStringConcat,
		ConceptAssignment, ConceptForLoop,
		ConceptReturn, ConceptImport, ConceptFunction, ConceptClass,
	}
	for _, c := range concepts {
		kinds := Resolve("javascript", c)
		assert.NotNil(t, kinds, "JavaScript should have kinds for concept %s", c)
	}
}

func TestIsNodeKind_Match(t *testing.T) {
	assert.True(t, IsNodeKind("go", ConceptCall, "call_expression"))
	assert.True(t, IsNodeKind("python", ConceptCall, "call"))
	assert.True(t, IsNodeKind("go", ConceptDefer, "defer_statement"))
}

func TestIsNodeKind_NoMatch(t *testing.T) {
	assert.False(t, IsNodeKind("go", ConceptCall, "string_literal"))
	assert.False(t, IsNodeKind("python", ConceptCall, "call_expression")) // Python overrides call to "call"
}

func TestIsNodeKind_UniversalDefault(t *testing.T) {
	// Unknown languages should still match via defaults
	assert.True(t, IsNodeKind("php", ConceptCall, "call_expression"))
	assert.True(t, IsNodeKind("haskell", ConceptFunction, "function_declaration"))
}

func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()
	assert.GreaterOrEqual(t, len(langs), 10)
	expected := []string{"go", "python", "javascript", "typescript", "tsx", "rust", "java", "c", "cpp", "ruby"}
	for _, e := range expected {
		assert.Contains(t, langs, e)
	}
}
