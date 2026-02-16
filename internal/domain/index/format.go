package index

import (
	"strings"

	"github.com/corey/aoa-go/internal/ports"
)

// FormatSymbol creates the display scope string for a symbol.
// Rules:
//   - Method (has parent): Parent.signature  e.g. "AuthHandler.login(self, user, password)"
//   - Class: strip "class " prefix from signature  e.g. "TestAuth(TestCase)"
//   - Top-level function: signature as-is  e.g. "create_app(config)"
//   - Directive: name only  e.g. "EXPOSE"
func FormatSymbol(sym *ports.SymbolMeta) string {
	if sym == nil {
		return ""
	}

	switch {
	case sym.Parent != "":
		// Method: Parent.signature
		return sym.Parent + "." + sym.Signature

	case sym.Kind == "class":
		// Class: strip "class " prefix
		return strings.TrimPrefix(sym.Signature, "class ")

	case sym.Kind == "directive":
		// Directive: just the name
		return sym.Name

	default:
		// Top-level function: signature as-is
		return sym.Signature
	}
}
