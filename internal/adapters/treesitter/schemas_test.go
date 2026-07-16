//go:build !lean

package treesitter

import (
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractSchemasGo_Fields verifies struct field extraction: named
// fields (including a comma-grouped declaration) and a struct tag, which
// must NOT leak into the field list (D31: descent stops at
// field_declaration_list, tags are a sibling leaf ignored by design).
func TestExtractSchemasGo_Fields(t *testing.T) {
	p := NewParser()

	source := []byte(`package model

type User struct {
	ID   int    ` + "`json:\"id\"`" + `
	Name string
	Age, Height int
}
`)

	tree, langName, err := p.ParseToTree("user.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()
	assert.Equal(t, "go", langName)

	entities := extractSchemasGo(tree.RootNode(), source, "user.go")
	require.Len(t, entities, 1)

	e := entities[0]
	assert.Equal(t, "User", e.Name)
	assert.Equal(t, "user.go", e.FromFile)
	assert.Greater(t, e.StartLine, uint32(0))
	assert.Equal(t, []string{"ID", "Name", "Age", "Height"}, e.Fields)
}

// TestExtractSchemasGo_EmbeddedFields verifies bare/pointer/qualified
// embedded fields promote by their type's own name (Go's real field-name
// rule), no field_identifier present in the AST for any of them.
func TestExtractSchemasGo_EmbeddedFields(t *testing.T) {
	p := NewParser()

	source := []byte(`package model

import "pkg"

type Wrapper struct {
	Embedded
	*PtrEmbedded
	pkg.Qualified
}
`)

	tree, langName, err := p.ParseToTree("wrapper.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()
	assert.Equal(t, "go", langName)

	entities := extractSchemasGo(tree.RootNode(), source, "wrapper.go")
	require.Len(t, entities, 1)
	assert.Equal(t, []string{"Embedded", "PtrEmbedded", "Qualified"}, entities[0].Fields)
}

// TestExtractSchemasGo_EmptyStruct verifies a zero-field struct produces an
// entity with an empty (not nil-panicking) field list.
func TestExtractSchemasGo_EmptyStruct(t *testing.T) {
	p := NewParser()

	source := []byte(`package model

type Empty struct{}
`)

	tree, langName, err := p.ParseToTree("empty.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()
	assert.Equal(t, "go", langName)

	entities := extractSchemasGo(tree.RootNode(), source, "empty.go")
	require.Len(t, entities, 1)
	assert.Equal(t, "Empty", entities[0].Name)
	assert.Empty(t, entities[0].Fields)
}

// TestExtractSchemasGo_NonStructType verifies non-struct type declarations
// (interfaces, aliases, primitives) are honestly skipped, not
// misclassified as zero-field entities.
func TestExtractSchemasGo_NonStructType(t *testing.T) {
	p := NewParser()

	source := []byte(`package model

type ID int

type Reader interface {
	Read() error
}
`)

	tree, langName, err := p.ParseToTree("types.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()
	assert.Equal(t, "go", langName)

	entities := extractSchemasGo(tree.RootNode(), source, "types.go")
	assert.Empty(t, entities)
}

// TestExtractSchemasGo_EmptyFile verifies no panic/entities on a
// struct-less file.
func TestExtractSchemasGo_EmptyFile(t *testing.T) {
	p := NewParser()

	source := []byte(`package model
`)

	tree, langName, err := p.ParseToTree("empty.go", source)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer tree.Close()
	assert.Equal(t, "go", langName)

	entities := extractSchemasGo(tree.RootNode(), source, "empty.go")
	assert.Empty(t, entities)
}

// TestParser_ExtractSchemas_EndToEnd exercises the public
// Parser.ExtractSchemas entry point (its own parse pass, mirrors
// Parser.ExtractRoutes).
func TestParser_ExtractSchemas_EndToEnd(t *testing.T) {
	p := NewParser()

	source := []byte(`package model

type User struct {
	Name string
}
`)

	entities, err := p.ExtractSchemas("user.go", source)
	require.NoError(t, err)
	require.Len(t, entities, 1)
	assert.Equal(t, "User", entities[0].Name)
	assert.Equal(t, []string{"Name"}, entities[0].Fields)
}

// TestParser_ExtractSchemas_UnknownLang verifies no error/panic for an
// unsupported language (schema extraction is Go-only for v1).
func TestParser_ExtractSchemas_UnknownLang(t *testing.T) {
	p := NewParser()
	entities, err := p.ExtractSchemas("app.py", []byte("class User:\n    pass\n"))
	require.NoError(t, err)
	assert.Nil(t, entities)
}

var _ ports.SchemaExtractor = (*Parser)(nil)
