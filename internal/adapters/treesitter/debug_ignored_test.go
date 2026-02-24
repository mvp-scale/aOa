//go:build !lean

package treesitter

import (
	"testing"
	"github.com/corey/aoa/internal/domain/analyzer"
)

func TestDebugIgnoredError(t *testing.T) {
	source := []byte(`package main
func handler() {
	_ = doSomething()
}
func doSomething() error { return nil }
`)
	p := NewParser()
	tree, lang, err := p.ParseToTree("main.go", source)
	if err != nil { t.Fatal(err) }
	defer tree.Close()

	root := tree.RootNode()
	// Find assignment_statement and dump ALL its children
	funcDecl := root.Child(1) // function_declaration: handler
	block := funcDecl.Child(3) // block
	assign := block.Child(1) // assignment_statement
	t.Logf("Assignment node: kind=%s text='%s'", assign.Kind(), string(source[assign.StartByte():assign.EndByte()]))
	t.Logf("Assignment children: %d", assign.ChildCount())
	for i := uint(0); i < uint(assign.ChildCount()); i++ {
		c := assign.Child(i)
		cText := string(source[c.StartByte():c.EndByte()])
		t.Logf("  [%d]: kind=%s text='%s'", i, c.Kind(), cText)
		for j := uint(0); j < uint(c.ChildCount()); j++ {
			cc := c.Child(j)
			ccText := string(source[cc.StartByte():cc.EndByte()])
			t.Logf("    [%d]: kind=%s text='%s'", j, cc.Kind(), ccText)
		}
	}

	// Now manually test name_contains
	nameKinds := []string{"identifier", "name", "field_identifier", "property_identifier", "blank_identifier"}
	t.Log("\nManual name_contains check:")
	for i := uint(0); i < uint(assign.ChildCount()); i++ {
		c := assign.Child(i)
		for _, nk := range nameKinds {
			if c.Kind() == nk {
				t.Logf("  Found name child: kind=%s text='%s'", c.Kind(), string(source[c.StartByte():c.EndByte()]))
			}
		}
	}

	// Check has_arg: look for call_expression child
	t.Log("\nManual has_arg check:")
	callKinds := analyzer.Resolve(lang, "call")
	t.Logf("call resolves to: %v", callKinds)
	for i := uint(0); i < uint(assign.ChildCount()); i++ {
		c := assign.Child(i)
		for _, ck := range callKinds {
			if c.Kind() == ck {
				t.Logf("  Found call child: kind=%s text='%s'", c.Kind(), string(source[c.StartByte():c.EndByte()]))
			}
		}
	}
}
