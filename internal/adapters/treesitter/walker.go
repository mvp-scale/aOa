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
		case "checkNestingDepth":
			ctx.checkNestingDepth(n, r, result, 0)
		case "checkTooManyParams":
			ctx.checkTooManyParams(n, r, result)
		case "checkLargeSwitch":
			ctx.checkLargeSwitch(n, r, result)
		case "checkUnreachableCode":
			ctx.checkUnreachableCode(n, r, result)
		case "checkGodObject":
			ctx.checkGodObject(n, r, result)
		case "checkExcessiveImports":
			ctx.checkExcessiveImports(n, r, result)
		case "checkExportedNoDoc":
			ctx.checkExportedNoDoc(n, r, result)
		case "checkUnstableInterface":
			ctx.checkUnstableInterface(n, r, result)
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

// checkNestingDepth: nesting exceeds 4 levels of if/for/switch
func (ctx *walkContext) checkNestingDepth(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult, depth int) {
	kind := n.Kind()
	isNesting := analyzer.IsNodeKind(ctx.lang, analyzer.ConceptForLoop, kind) ||
		kind == "if_statement" || kind == "if_expression" ||
		kind == "switch_statement" || kind == "select_statement" ||
		kind == "match_expression" || kind == "case_statement"

	newDepth := depth
	if isNesting {
		newDepth++
		if newDepth > 4 {
			result.Findings = append(result.Findings, analyzer.RuleFinding{
				RuleID:   r.ID,
				Line:     int(n.StartPosition().Row + 1),
				Severity: r.Severity,
			})
			return // don't report deeper nesting within same tree
		}
	}

	for i := uint(0); i < uint(n.ChildCount()); i++ {
		ctx.checkNestingDepth(n.Child(i), r, result, newDepth)
	}
}

// checkTooManyParams: function with more than 5 parameters
func (ctx *walkContext) checkTooManyParams(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if !isSymbolNode(ctx.lang, n.Kind()) {
		return
	}

	// Look for parameter_list or formal_parameters child
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		kind := c.Kind()
		if kind == "parameter_list" || kind == "formal_parameters" || kind == "parameters" {
			// Count parameter children (exclude punctuation)
			paramCount := 0
			for j := uint(0); j < uint(c.ChildCount()); j++ {
				pk := c.Child(j).Kind()
				if pk != "," && pk != "(" && pk != ")" {
					paramCount++
				}
			}
			if paramCount > 5 {
				name := extractSymbolName(n, ctx.source)
				result.Findings = append(result.Findings, analyzer.RuleFinding{
					RuleID:   r.ID,
					Line:     int(n.StartPosition().Row + 1),
					Symbol:   name,
					Severity: r.Severity,
				})
			}
			return
		}
	}
}

// checkLargeSwitch: switch/select with more than 15 cases
func (ctx *walkContext) checkLargeSwitch(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	kind := n.Kind()
	if kind != "switch_statement" && kind != "select_statement" &&
		kind != "expression_switch_statement" && kind != "type_switch_statement" &&
		kind != "match_expression" && kind != "switch_expression" {
		return
	}

	caseCount := 0
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		ck := n.Child(i).Kind()
		if ck == "case_clause" || ck == "expression_case" || ck == "default_case" ||
			ck == "case" || ck == "communication_case" || ck == "match_arm" {
			caseCount++
		}
	}
	if caseCount > 15 {
		result.Findings = append(result.Findings, analyzer.RuleFinding{
			RuleID:   r.ID,
			Line:     int(n.StartPosition().Row + 1),
			Severity: r.Severity,
		})
	}
}

// checkUnreachableCode: statements after unconditional return/panic
func (ctx *walkContext) checkUnreachableCode(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	// Look at block bodies for statements after return/panic
	kind := n.Kind()
	if kind != "block" && kind != "statement_block" && kind != "compound_statement" {
		return
	}

	foundReturn := false
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		ck := c.Kind()
		if ck == "{" || ck == "}" {
			continue
		}
		if foundReturn {
			result.Findings = append(result.Findings, analyzer.RuleFinding{
				RuleID:   r.ID,
				Line:     int(c.StartPosition().Row + 1),
				Severity: r.Severity,
			})
			return // report only the first unreachable statement
		}
		if analyzer.IsNodeKind(ctx.lang, analyzer.ConceptReturn, ck) ||
			(ck == "expression_statement" && c.ChildCount() > 0 && nodeTextWalker(c, ctx.source) == "panic(") {
			foundReturn = true
		}
	}
}

// checkGodObject: struct/class with more than 15 fields
func (ctx *walkContext) checkGodObject(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if !analyzer.IsNodeKind(ctx.lang, analyzer.ConceptClass, n.Kind()) {
		return
	}

	// Count field declarations
	fieldCount := countFieldsRecursive(n)
	if fieldCount > 15 {
		name := extractSymbolName(n, ctx.source)
		result.Findings = append(result.Findings, analyzer.RuleFinding{
			RuleID:   r.ID,
			Line:     int(n.StartPosition().Row + 1),
			Symbol:   name,
			Severity: r.Severity,
		})
	}
}

// countFieldsRecursive counts field declarations within a struct/class node.
func countFieldsRecursive(n *tree_sitter.Node) int {
	count := 0
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		kind := c.Kind()
		if kind == "field_declaration" || kind == "field_definition" ||
			kind == "property_declaration" || kind == "field_declaration_list" {
			if kind == "field_declaration_list" {
				count += countFieldsRecursive(c)
			} else {
				count++
			}
		}
	}
	return count
}

// checkExcessiveImports: file imports more than 15 packages
func (ctx *walkContext) checkExcessiveImports(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if !analyzer.IsNodeKind(ctx.lang, analyzer.ConceptImport, n.Kind()) {
		return
	}

	// For Go: import_declaration can have an import_spec_list with multiple specs
	importCount := 0
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		if c.Kind() == "import_spec_list" {
			for j := uint(0); j < uint(c.ChildCount()); j++ {
				if c.Child(j).Kind() == "import_spec" {
					importCount++
				}
			}
		} else if c.Kind() == "import_spec" {
			importCount++
		}
	}

	if importCount > 15 {
		result.Findings = append(result.Findings, analyzer.RuleFinding{
			RuleID:   r.ID,
			Line:     int(n.StartPosition().Row + 1),
			Severity: r.Severity,
		})
	}
}

// checkExportedNoDoc: exported function/type without preceding doc comment (Go-specific)
func (ctx *walkContext) checkExportedNoDoc(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if ctx.lang != "go" {
		return
	}
	if !isSymbolNode(ctx.lang, n.Kind()) {
		return
	}

	name := extractSymbolName(n, ctx.source)
	if name == "" || name[0] < 'A' || name[0] > 'Z' {
		return // not exported
	}

	// Check for preceding comment: look at the previous sibling
	prevSib := n.PrevSibling()
	if prevSib != nil && prevSib.Kind() == "comment" {
		return // has doc comment
	}

	result.Findings = append(result.Findings, analyzer.RuleFinding{
		RuleID:   r.ID,
		Line:     int(n.StartPosition().Row + 1),
		Symbol:   name,
		Severity: r.Severity,
	})
}

// checkUnstableInterface: interface with more than 10 methods (Go-specific)
func (ctx *walkContext) checkUnstableInterface(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	if ctx.lang != "go" {
		return
	}
	if n.Kind() != "type_declaration" {
		return
	}

	// Look for interface_type child
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		if c.Kind() == "type_spec" {
			for j := uint(0); j < uint(c.ChildCount()); j++ {
				iface := c.Child(j)
				if iface.Kind() == "interface_type" {
					methodCount := 0
					for k := uint(0); k < uint(iface.ChildCount()); k++ {
						mk := iface.Child(k).Kind()
						if mk == "method_spec" || mk == "method_elem" {
							methodCount++
						}
					}
					if methodCount > 10 {
						name := extractSymbolName(n, ctx.source)
						result.Findings = append(result.Findings, analyzer.RuleFinding{
							RuleID:   r.ID,
							Line:     int(n.StartPosition().Row + 1),
							Symbol:   name,
							Severity: r.Severity,
						})
					}
					return
				}
			}
		}
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
