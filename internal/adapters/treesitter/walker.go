//go:build !lean

package treesitter

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/corey/aoa/internal/domain/analyzer"
)

// WalkResult holds the findings from a structural AST walk.
type WalkResult struct {
	Findings []analyzer.RuleFinding
	Symbols  []analyzer.SymbolSpan
}

// WalkForDimensions walks the AST tree looking for structural rule violations.
// It returns findings with line numbers and a list of symbol spans.
func WalkForDimensions(root *tree_sitter.Node, source []byte, lang string, rules []analyzer.Rule, isMain bool) WalkResult {
	var result WalkResult

	// Build structural rule set
	structRules := make([]analyzer.Rule, 0)
	for _, r := range rules {
		if r.Kind == analyzer.RuleStructural || r.Kind == analyzer.RuleComposite {
			structRules = append(structRules, r)
		}
	}
	if len(structRules) == 0 {
		return result
	}

	ctx := &walkContext{
		source:    source,
		lang:      lang,
		rules:     structRules,
		isMain:    isMain,
		loopDepth: 0,
	}
	ctx.walk(root, &result)
	return result
}

type walkContext struct {
	source    []byte
	lang      string
	rules     []analyzer.Rule
	isMain    bool
	loopDepth int
}

func (ctx *walkContext) walk(n *tree_sitter.Node, result *WalkResult) {
	kind := n.Kind()

	// Track symbol spans
	if isSymbolNode(ctx.lang, kind) {
		name := extractSymbolName(n, ctx.source)
		if name != "" {
			result.Symbols = append(result.Symbols, analyzer.SymbolSpan{
				Name:      name,
				StartLine: int(n.StartPosition().Row + 1),
				EndLine:   int(n.EndPosition().Row + 1),
			})
		}
	}

	// Track loop depth
	isLoop := analyzer.IsNodeKind(ctx.lang, analyzer.ConceptForLoop, kind)
	if isLoop {
		ctx.loopDepth++
	}

	// Check structural rules
	for _, r := range ctx.rules {
		if r.SkipMain && ctx.isMain {
			continue
		}
		switch r.StructuralCheck {
		case "checkDeferInLoop":
			ctx.checkDeferInLoop(n, r, result)
		case "checkIgnoredError":
			ctx.checkIgnoredError(n, r, result)
		case "checkPanicInLib":
			ctx.checkPanicInLib(n, r, result)
		case "checkUncheckedTypeAssert":
			ctx.checkUncheckedTypeAssert(n, r, result)
		case "checkSQLStringConcat":
			ctx.checkSQLStringConcat(n, r, result)
		case "checkLongFunction":
			ctx.checkLongFunction(n, r, result)
		}
	}

	// Recurse into children
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		child := n.Child(i)
		ctx.walk(child, result)
	}

	if isLoop {
		ctx.loopDepth--
	}
}

// checkDeferInLoop: defer statement inside a loop body
func (ctx *walkContext) checkDeferInLoop(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if ctx.loopDepth == 0 {
		return
	}
	if !analyzer.IsNodeKind(ctx.lang, analyzer.ConceptDefer, n.Kind()) {
		return
	}
	result.Findings = append(result.Findings, analyzer.RuleFinding{
		RuleID:   r.ID,
		Line:     int(n.StartPosition().Row + 1),
		Severity: r.Severity,
	})
}

// checkIgnoredError: blank identifier assigned from a call returning error
// Matches patterns like `_ = someFunc()` or `_, _ := someFunc()`
func (ctx *walkContext) checkIgnoredError(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if ctx.lang != "go" {
		return
	}
	kind := n.Kind()
	if kind != "short_var_declaration" && kind != "assignment_statement" {
		return
	}

	text := nodeTextWalker(n, ctx.source)
	trimmed := strings.TrimSpace(text)

	// Check for blank identifier pattern
	hasBlank := strings.HasPrefix(trimmed, "_ =") ||
		strings.HasPrefix(trimmed, "_ :=") ||
		strings.Contains(trimmed, ", _ =") ||
		strings.Contains(trimmed, ", _ :=")
	if !hasBlank {
		return
	}
	// Must involve a function call
	if !strings.Contains(trimmed, "(") {
		return
	}

	result.Findings = append(result.Findings, analyzer.RuleFinding{
		RuleID:   r.ID,
		Line:     int(n.StartPosition().Row + 1),
		Severity: r.Severity,
	})
}

// checkPanicInLib: panic() called outside main package
func (ctx *walkContext) checkPanicInLib(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if ctx.isMain {
		return
	}
	if !analyzer.IsNodeKind(ctx.lang, analyzer.ConceptCall, n.Kind()) {
		return
	}
	text := nodeTextWalker(n, ctx.source)
	if !strings.HasPrefix(text, "panic(") {
		return
	}

	result.Findings = append(result.Findings, analyzer.RuleFinding{
		RuleID:   r.ID,
		Line:     int(n.StartPosition().Row + 1),
		Severity: r.Severity,
	})
}

// checkUncheckedTypeAssert: type assertion without comma-ok pattern
func (ctx *walkContext) checkUncheckedTypeAssert(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if ctx.lang != "go" {
		return
	}
	if n.Kind() != "type_assertion_expression" {
		return
	}
	// If parent is an assignment with two LHS identifiers, it's comma-ok
	// The type assertion itself is just `x.(Type)`. We check if it appears
	// as the sole RHS of an assignment with only one LHS variable.
	parent := n.Parent()
	if parent == nil {
		// Used as expression statement without assignment — unchecked
		result.Findings = append(result.Findings, analyzer.RuleFinding{
			RuleID:   r.ID,
			Line:     int(n.StartPosition().Row + 1),
			Severity: r.Severity,
		})
		return
	}

	// Check if parent is assignment/short_var_declaration
	pKind := parent.Kind()
	if pKind == "short_var_declaration" || pKind == "assignment_statement" {
		// Count the LHS identifiers by looking at expression_list before :=
		text := nodeTextWalker(parent, ctx.source)
		parts := strings.SplitN(text, ":=", 2)
		if len(parts) < 2 {
			parts = strings.SplitN(text, "=", 2)
		}
		if len(parts) >= 1 {
			lhs := parts[0]
			commaCount := strings.Count(lhs, ",")
			if commaCount >= 1 {
				return // comma-ok pattern, e.g. `v, ok := x.(Type)`
			}
		}
	}

	result.Findings = append(result.Findings, analyzer.RuleFinding{
		RuleID:   r.ID,
		Line:     int(n.StartPosition().Row + 1),
		Severity: r.Severity,
	})
}

// checkSQLStringConcat: string concatenation containing SQL keywords
func (ctx *walkContext) checkSQLStringConcat(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if !analyzer.IsNodeKind(ctx.lang, analyzer.ConceptStringConcat, n.Kind()) {
		return
	}
	text := strings.ToUpper(nodeTextWalker(n, ctx.source))
	sqlKeywords := []string{"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "DROP ", "CREATE TABLE"}
	hasSQLKeyword := false
	for _, kw := range sqlKeywords {
		if strings.Contains(text, kw) {
			hasSQLKeyword = true
			break
		}
	}
	if !hasSQLKeyword {
		return
	}
	// Must have concatenation operator or variable interpolation
	if !strings.Contains(text, "+") && !strings.Contains(text, "%") {
		return
	}

	result.Findings = append(result.Findings, analyzer.RuleFinding{
		RuleID:   r.ID,
		Line:     int(n.StartPosition().Row + 1),
		Severity: r.Severity,
	})
}

// checkLongFunction: function/method exceeds 100 lines
func (ctx *walkContext) checkLongFunction(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if !isSymbolNode(ctx.lang, n.Kind()) {
		return
	}
	startLine := int(n.StartPosition().Row + 1)
	endLine := int(n.EndPosition().Row + 1)
	if endLine-startLine > 100 {
		name := extractSymbolName(n, ctx.source)
		result.Findings = append(result.Findings, analyzer.RuleFinding{
			RuleID:   r.ID,
			Line:     startLine,
			Symbol:   name,
			Severity: r.Severity,
		})
	}
}

// isSymbolNode returns true if this node kind represents a function/method/class.
func isSymbolNode(lang, kind string) bool {
	return analyzer.IsNodeKind(lang, analyzer.ConceptFunction, kind) ||
		analyzer.IsNodeKind(lang, analyzer.ConceptClass, kind)
}

// extractSymbolName extracts the identifier name from a symbol node.
func extractSymbolName(n *tree_sitter.Node, source []byte) string {
	nameKinds := []string{"identifier", "name", "field_identifier", "property_identifier", "type_identifier"}
	for _, nk := range nameKinds {
		for i := uint(0); i < uint(n.ChildCount()); i++ {
			c := n.Child(i)
			if c.Kind() == nk {
				return string(source[c.StartByte():c.EndByte()])
			}
		}
	}
	return ""
}

// nodeTextWalker returns the source text for a node.
func nodeTextWalker(n *tree_sitter.Node, source []byte) string {
	start := n.StartByte()
	end := n.EndByte()
	if int(start) >= len(source) || int(end) > len(source) {
		return ""
	}
	return string(source[start:end])
}
