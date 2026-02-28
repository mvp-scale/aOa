//go:build !lean

package treesitter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// S-02: Tree-sitter Adapter — Parse files, extract symbols
// Goals: G1 (O(1) performance), G2 (grep/egrep parity)
// Expectation: Parse source files into symbols (functions, classes, methods)
// with name, signature, kind, and line range
// =============================================================================

func TestParser_ExtractGoFunctions(t *testing.T) {
	// S-02, G1: Parse a .go file, extract function declarations.
	// Each function has: name, signature, kind="function", start/end lines.
	p := NewParser()

	source := []byte(`package main

func hello(name string) string {
	return "hello " + name
}

type Server struct {
	port int
}

func (s *Server) Start() error {
	return nil
}
`)

	symbols, err := p.ParseFile("main.go", source)
	require.NoError(t, err)
	require.Equal(t, 3, len(symbols), "expected function + struct + method")

	// Function
	assert.Equal(t, "hello", symbols[0].Name)
	assert.Equal(t, "hello(name string)", symbols[0].Signature)
	assert.Equal(t, "function", symbols[0].Kind)
	assert.Equal(t, uint16(3), symbols[0].StartLine)
	assert.Equal(t, uint16(5), symbols[0].EndLine)
	assert.Equal(t, "", symbols[0].Parent)

	// Struct
	assert.Equal(t, "Server", symbols[1].Name)
	assert.Equal(t, "type Server struct", symbols[1].Signature)
	assert.Equal(t, "struct", symbols[1].Kind)

	// Method
	assert.Equal(t, "Start", symbols[2].Name)
	assert.Equal(t, "Start()", symbols[2].Signature)
	assert.Equal(t, "method", symbols[2].Kind)
	assert.Equal(t, "Server", symbols[2].Parent)
	assert.Equal(t, uint16(11), symbols[2].StartLine)
	assert.Equal(t, uint16(13), symbols[2].EndLine)
}

func TestParser_ExtractPythonFunctions(t *testing.T) {
	// S-02, G1: Parse a .py file, extract def/class declarations.
	p := NewParser()

	source := []byte(`class AuthHandler:
    def login(self, user, password):
        return True

    def logout(self):
        pass

def standalone(x):
    return x
`)

	symbols, err := p.ParseFile("auth.py", source)
	require.NoError(t, err)
	require.Equal(t, 4, len(symbols), "expected class + 2 methods + function")

	// Class
	assert.Equal(t, "AuthHandler", symbols[0].Name)
	assert.Equal(t, "class AuthHandler", symbols[0].Signature)
	assert.Equal(t, "class", symbols[0].Kind)

	// Methods
	assert.Equal(t, "login", symbols[1].Name)
	assert.Equal(t, "login(self, user, password)", symbols[1].Signature)
	assert.Equal(t, "method", symbols[1].Kind)
	assert.Equal(t, "AuthHandler", symbols[1].Parent)

	assert.Equal(t, "logout", symbols[2].Name)
	assert.Equal(t, "AuthHandler", symbols[2].Parent)

	// Standalone function
	assert.Equal(t, "standalone", symbols[3].Name)
	assert.Equal(t, "standalone(x)", symbols[3].Signature)
	assert.Equal(t, "function", symbols[3].Kind)
	assert.Equal(t, "", symbols[3].Parent)
}

func TestParser_ExtractJavaScriptFunctions(t *testing.T) {
	// S-02, G1: Parse a .js file, extract function/class/arrow declarations.
	p := NewParser()

	source := []byte(`class Router {
  handle(req, res) {
    return res.send("ok")
  }
}

function standalone(x) {
  return x
}

const arrow = (x) => x + 1
`)

	symbols, err := p.ParseFile("router.js", source)
	require.NoError(t, err)
	require.Equal(t, 4, len(symbols), "expected class + method + function + arrow")

	// Class
	assert.Equal(t, "Router", symbols[0].Name)
	assert.Equal(t, "class Router", symbols[0].Signature)
	assert.Equal(t, "class", symbols[0].Kind)

	// Method
	assert.Equal(t, "handle", symbols[1].Name)
	assert.Equal(t, "handle(req, res)", symbols[1].Signature)
	assert.Equal(t, "method", symbols[1].Kind)
	assert.Equal(t, "Router", symbols[1].Parent)

	// Standalone function
	assert.Equal(t, "standalone", symbols[2].Name)
	assert.Equal(t, "standalone(x)", symbols[2].Signature)
	assert.Equal(t, "function", symbols[2].Kind)

	// Arrow function
	assert.Equal(t, "arrow", symbols[3].Name)
	assert.Equal(t, "arrow(x)", symbols[3].Signature)
	assert.Equal(t, "function", symbols[3].Kind)
}

func TestParser_LanguageDetection(t *testing.T) {
	// S-02, G2: File extension -> correct tree-sitter grammar.
	p := NewParser()

	assert.True(t, p.SupportsExtension(".go"))
	assert.True(t, p.SupportsExtension(".py"))
	assert.True(t, p.SupportsExtension(".pyw"))
	assert.True(t, p.SupportsExtension(".js"))
	assert.True(t, p.SupportsExtension(".jsx"))
	assert.True(t, p.SupportsExtension(".mjs"))
	assert.True(t, p.SupportsExtension(".rs")) // Rust is registered
	assert.True(t, p.SupportsExtension(".ts")) // TypeScript
	assert.True(t, p.SupportsExtension(".rb")) // Ruby
	assert.False(t, p.SupportsExtension(".xyz"))
	assert.False(t, p.SupportsExtension(".unknown"))
	assert.False(t, p.SupportsExtension(""))
}

func TestParser_NestedSymbols(t *testing.T) {
	// S-02, G2: Class methods are nested: parent="ClassName".
	p := NewParser()

	// Python nested
	pySource := []byte(`class Foo:
    def bar(self):
        pass
    def baz(self, x):
        return x
`)
	symbols, err := p.ParseFile("foo.py", pySource)
	require.NoError(t, err)

	require.Equal(t, 3, len(symbols))
	assert.Equal(t, "Foo", symbols[0].Name)
	assert.Equal(t, "bar", symbols[1].Name)
	assert.Equal(t, "Foo", symbols[1].Parent)
	assert.Equal(t, "baz", symbols[2].Name)
	assert.Equal(t, "Foo", symbols[2].Parent)

	// JS nested
	jsSource := []byte(`class Widget {
  render() { return null }
  update(data) { this.data = data }
}
`)
	symbols, err = p.ParseFile("widget.js", jsSource)
	require.NoError(t, err)

	require.Equal(t, 3, len(symbols))
	assert.Equal(t, "Widget", symbols[0].Name)
	assert.Equal(t, "render", symbols[1].Name)
	assert.Equal(t, "Widget", symbols[1].Parent)
	assert.Equal(t, "update", symbols[2].Name)
	assert.Equal(t, "Widget", symbols[2].Parent)

	// Go nested (method with receiver)
	goSource := []byte(`package main

type DB struct{}

func (d *DB) Query(sql string) error {
	return nil
}
`)
	symbols, err = p.ParseFile("db.go", goSource)
	require.NoError(t, err)

	require.Equal(t, 2, len(symbols))
	assert.Equal(t, "DB", symbols[0].Name)
	assert.Equal(t, "struct", symbols[0].Kind)
	assert.Equal(t, "Query", symbols[1].Name)
	assert.Equal(t, "DB", symbols[1].Parent)
	assert.Equal(t, "method", symbols[1].Kind)
}

func TestParser_SymbolRange(t *testing.T) {
	// S-02, G1: Each symbol has [startLine, endLine] range.
	p := NewParser()

	source := []byte(`package main

func short() {
}

func longer() {
	x := 1
	y := 2
	z := x + y
	_ = z
}
`)

	symbols, err := p.ParseFile("range.go", source)
	require.NoError(t, err)
	require.Equal(t, 2, len(symbols))

	// short: lines 3-4
	assert.Equal(t, uint16(3), symbols[0].StartLine)
	assert.Equal(t, uint16(4), symbols[0].EndLine)

	// longer: lines 6-11
	assert.Equal(t, uint16(6), symbols[1].StartLine)
	assert.Equal(t, uint16(11), symbols[1].EndLine)
}

func TestParser_UnknownLanguage_NoError(t *testing.T) {
	// S-02, G5: File with unknown extension (.xyz) returns empty symbols,
	// no error. Graceful degradation.
	p := NewParser()

	symbols, err := p.ParseFile("config.xyz", []byte("some content"))
	assert.NoError(t, err)
	assert.Nil(t, symbols)

	symbols, err = p.ParseFile("data.csv", []byte("a,b,c"))
	assert.NoError(t, err)
	assert.Nil(t, symbols)
}

func TestParser_EmptyFile_NoError(t *testing.T) {
	// S-02, G5: Empty file returns empty symbols, no error.
	p := NewParser()

	symbols, err := p.ParseFile("empty.go", []byte(""))
	assert.NoError(t, err)
	assert.Nil(t, symbols)

	symbols, err = p.ParseFile("empty.py", []byte(""))
	assert.NoError(t, err)
	assert.Nil(t, symbols)

	symbols, err = p.ParseFile("empty.js", nil)
	assert.NoError(t, err)
	assert.Nil(t, symbols)
}

func BenchmarkParseFile(b *testing.B) {
	// S-02, G1: Target <20ms per file (vs 50-200ms Python).
	p := NewParser()

	source := []byte(`package main

import "fmt"

type Handler struct {
	name string
}

func NewHandler(name string) *Handler {
	return &Handler{name: name}
}

func (h *Handler) Handle(req Request) Response {
	fmt.Println(h.name, req)
	return Response{Status: 200}
}

func (h *Handler) Close() error {
	return nil
}

func standalone() {
	fmt.Println("hello")
}
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.ParseFile("handler.go", source)
	}
}

func TestParser_LanguageCount(t *testing.T) {
	p := NewParser()
	t.Logf("Languages registered: %d", p.LanguageCount())
	t.Logf("Extensions registered: %d", len(p.SupportedExtensions()))
	
	// We should have 29 languages compiled in
	assert.GreaterOrEqual(t, p.LanguageCount(), 25, "should have at least 25 languages")
}
