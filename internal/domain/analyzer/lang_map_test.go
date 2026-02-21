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

func TestResolve_UnknownLanguage(t *testing.T) {
	kinds := Resolve("cobol", ConceptCall)
	assert.Nil(t, kinds)
}

func TestResolve_UnknownConcept(t *testing.T) {
	kinds := Resolve("go", "nonexistent_concept")
	assert.Nil(t, kinds)
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
	assert.False(t, IsNodeKind("python", ConceptDefer, "defer_statement")) // Python has no defer
}

func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()
	assert.GreaterOrEqual(t, len(langs), 10)
	expected := []string{"go", "python", "javascript", "typescript", "tsx", "rust", "java", "c", "cpp", "ruby"}
	for _, e := range expected {
		assert.Contains(t, langs, e)
	}
}
