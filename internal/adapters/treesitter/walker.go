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
		if ctx.shouldSkipLang(r) {
			continue
		}

		if r.Structural != nil {
			ctx.evaluateStructural(n, r, result)
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

// shouldSkipLang returns true if this rule should be skipped for the current language.
func (ctx *walkContext) shouldSkipLang(r analyzer.Rule) bool {
	for _, lang := range r.SkipLangs {
		if lang == ctx.lang {
			return true
		}
	}
	return false
}

// evaluateStructural evaluates a declarative StructuralBlock against a node.
func (ctx *walkContext) evaluateStructural(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult) {
	sb := r.Structural
	kind := n.Kind()

	// Special handling for nesting_threshold: needs recursive depth tracking
	if sb.NestingThreshold > 0 {
		ctx.evalNestingDepth(n, r, result, 0)
		return
	}

	// Resolve match concept to AST node kinds
	matchKinds := ctx.resolveMatchConcept(sb.Match)
	if len(matchKinds) == 0 {
		return
	}

	// Check if node kind matches
	matched := false
	for _, mk := range matchKinds {
		if kind == mk {
			matched = true
			break
		}
	}
	if !matched {
		return
	}

	// Check inside constraint
	if sb.Inside != "" {
		if !ctx.checkInside(n, sb.Inside) {
			return
		}
	}

	// Check text_contains constraint
	if len(sb.TextContains) > 0 {
		text := strings.ToUpper(nodeTextWalker(n, ctx.source))
		found := false
		for _, pat := range sb.TextContains {
			if strings.Contains(text, strings.ToUpper(pat)) {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}

	// Check receiver_contains constraint (case-insensitive for cross-language matching)
	if len(sb.ReceiverContains) > 0 {
		text := strings.ToLower(nodeTextWalker(n, ctx.source))
		found := false
		for _, pat := range sb.ReceiverContains {
			if strings.Contains(text, strings.ToLower(pat)) {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}

	// Check name_contains constraint
	if len(sb.NameContains) > 0 {
		if !ctx.checkNameContains(n, sb.NameContains) {
			return
		}
	}

	// Check has_arg constraint
	if sb.HasArg != nil {
		if !ctx.checkHasArg(n, sb.HasArg) {
			return
		}
	}

	// Check without_sibling constraint (semantic templates)
	if sb.WithoutSibling != "" {
		if !ctx.checkWithoutSibling(n, sb.WithoutSibling) {
			return
		}
	}

	// Check child_count_threshold constraint (context-sensitive)
	if sb.ChildCountThreshold > 0 {
		if !ctx.checkChildCountThreshold(n, sb.Match, sb.ChildCountThreshold) {
			return
		}
	}

	// Check line_threshold constraint
	if sb.LineThreshold > 0 {
		startLine := int(n.StartPosition().Row + 1)
		endLine := int(n.EndPosition().Row + 1)
		if endLine-startLine <= sb.LineThreshold {
			return
		}
	}

	// All constraints passed — emit finding
	name := ""
	if isSymbolNode(ctx.lang, kind) {
		name = extractSymbolName(n, ctx.source)
	}
	result.Findings = append(result.Findings, analyzer.RuleFinding{
		RuleID:   r.ID,
		Line:     int(n.StartPosition().Row + 1),
		Symbol:   name,
		Severity: r.Severity,
	})
}

// resolveMatchConcept resolves a match concept to AST node kinds
// via the universal concept layer.
func (ctx *walkContext) resolveMatchConcept(concept string) []string {
	return analyzer.Resolve(ctx.lang, concept)
}

// checkInside verifies that the node has an ancestor matching the given concept.
func (ctx *walkContext) checkInside(n *tree_sitter.Node, concept string) bool {
	// Special fast path for for_loop using loopDepth counter
	if concept == analyzer.ConceptForLoop {
		return ctx.loopDepth > 0
	}

	// General ancestor walk
	ancestorKinds := analyzer.Resolve(ctx.lang, concept)
	if len(ancestorKinds) == 0 {
		return false
	}
	for p := n.Parent(); p != nil; p = p.Parent() {
		pk := p.Kind()
		for _, ak := range ancestorKinds {
			if pk == ak {
				return true
			}
		}
	}
	return false
}

// checkNameContains checks if any identifier child contains one of the substrings.
// Looks through expression_list wrappers (Go assignments use expression_list for LHS/RHS).
func (ctx *walkContext) checkNameContains(n *tree_sitter.Node, patterns []string) bool {
	nameKinds := []string{"identifier", "name", "field_identifier", "property_identifier", "blank_identifier"}
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		ck := c.Kind()
		// Look through expression_list wrappers (Go assignment LHS)
		if ck == "expression_list" || ck == "left_hand_side" {
			if ctx.checkNameContains(c, patterns) {
				return true
			}
			continue
		}
		for _, nk := range nameKinds {
			if ck == nk {
				text := nodeTextWalker(c, ctx.source)
				for _, pat := range patterns {
					if strings.Contains(text, pat) {
						return true
					}
				}
			}
		}
	}
	return false
}

// checkHasArg checks if a call node has arguments matching the spec.
// For assignment nodes, looks through expression_list wrappers to find the call.
func (ctx *walkContext) checkHasArg(n *tree_sitter.Node, spec *analyzer.ArgSpec) bool {
	// Walk children looking for argument_list or similar
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		ck := c.Kind()
		if ck == "argument_list" || ck == "arguments" || ck == "actual_parameters" {
			return ctx.checkArgChildren(c, spec)
		}
		// Look through expression_list wrappers (Go assignment RHS)
		if ck == "expression_list" || ck == "right_hand_side" {
			if ctx.checkHasArg(c, spec) {
				return true
			}
		}
	}
	// For some grammars, arguments are direct children of call
	return ctx.checkArgChildren(n, spec)
}

// checkArgChildren checks if any child of the argument list matches the spec.
// When both Type and TextContains are present, an arg must satisfy type match
// and then text_contains is checked on that same arg's subtree.
func (ctx *walkContext) checkArgChildren(n *tree_sitter.Node, spec *analyzer.ArgSpec) bool {
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		ck := c.Kind()
		// Skip punctuation
		if ck == "," || ck == "(" || ck == ")" {
			continue
		}

		// Check type constraint via universal concept resolution
		typeMatched := false
		if len(spec.Type) > 0 {
			for _, t := range spec.Type {
				conceptKinds := ctx.resolveMatchConcept(t)
				if len(conceptKinds) > 0 {
					for _, conceptKind := range conceptKinds {
						if ck == conceptKind {
							typeMatched = true
							break
						}
					}
				} else {
					// Literal node kind match
					if ck == t {
						typeMatched = true
					}
				}
				if typeMatched {
					break
				}
			}
		} else {
			typeMatched = true // no type constraint means any arg passes
		}

		if !typeMatched {
			continue
		}

		// If text_contains is also present, check it on the matched arg
		if len(spec.TextContains) > 0 {
			text := strings.ToUpper(nodeTextWalker(c, ctx.source))
			for _, pat := range spec.TextContains {
				if strings.Contains(text, strings.ToUpper(pat)) {
					return true
				}
			}
			// Type matched but text didn't — continue checking other args
			continue
		}

		// Type matched, no text constraint — pass
		return true
	}
	return false
}

// checkWithoutSibling implements semantic template checking.
// Returns true if the "without" condition is met (i.e., the sibling is absent).
func (ctx *walkContext) checkWithoutSibling(n *tree_sitter.Node, template string) bool {
	switch template {
	case "comma_ok":
		return ctx.checkWithoutCommaOk(n)
	case "doc_comment":
		return ctx.checkWithoutDocComment(n)
	case "after_return":
		return ctx.checkAfterReturn(n)
	case "error_check":
		return ctx.checkWithoutErrorCheck(n)
	default:
		return false
	}
}

// checkWithoutCommaOk: type assertion without comma-ok pattern.
// Returns true if there's no comma-ok (i.e., the assertion IS unchecked).
func (ctx *walkContext) checkWithoutCommaOk(n *tree_sitter.Node) bool {
	parent := n.Parent()
	if parent == nil {
		return true // used as expression statement without assignment
	}

	pKind := parent.Kind()
	if pKind == "short_var_declaration" || pKind == "assignment_statement" {
		text := nodeTextWalker(parent, ctx.source)
		parts := strings.SplitN(text, ":=", 2)
		if len(parts) < 2 {
			parts = strings.SplitN(text, "=", 2)
		}
		if len(parts) >= 1 {
			lhs := parts[0]
			if strings.Count(lhs, ",") >= 1 {
				return false // comma-ok pattern
			}
		}
	}
	return true
}

// checkWithoutDocComment: exported function/type without doc comment.
// Returns true if there's no preceding doc comment.
func (ctx *walkContext) checkWithoutDocComment(n *tree_sitter.Node) bool {
	if ctx.lang != "go" {
		return false
	}

	name := extractSymbolName(n, ctx.source)
	if name == "" || name[0] < 'A' || name[0] > 'Z' {
		return false // not exported — don't fire
	}

	prevSib := n.PrevSibling()
	if prevSib != nil && prevSib.Kind() == "comment" {
		return false // has doc comment
	}
	return true
}

// checkAfterReturn: block has statements after unconditional return.
// Returns true if there are unreachable statements.
func (ctx *walkContext) checkAfterReturn(n *tree_sitter.Node) bool {
	foundReturn := false
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		ck := c.Kind()
		if ck == "{" || ck == "}" {
			continue
		}
		if foundReturn {
			return true // found unreachable statement
		}
		if analyzer.IsNodeKind(ctx.lang, analyzer.ConceptReturn, ck) ||
			(ck == "expression_statement" && c.ChildCount() > 0 && strings.HasPrefix(nodeTextWalker(c, ctx.source), "panic(")) {
			foundReturn = true
		}
	}
	return false
}

// checkWithoutErrorCheck: call without error check.
// This is a Go-specific check. Returns true if the call lacks error handling.
func (ctx *walkContext) checkWithoutErrorCheck(n *tree_sitter.Node) bool {
	if ctx.lang != "go" {
		return false
	}

	// Check if the call is on its own (expression_statement) without error handling
	parent := n.Parent()
	if parent == nil {
		return false
	}

	// If parent is an expression_statement, the return is fully ignored
	if parent.Kind() == "expression_statement" {
		text := nodeTextWalker(n, ctx.source)
		// Skip common non-error-returning patterns
		if strings.HasPrefix(text, "fmt.") || strings.HasPrefix(text, "log.") ||
			strings.HasPrefix(text, "defer ") || strings.HasPrefix(text, "go ") {
			return false
		}
		return true
	}

	return false
}

// checkChildCountThreshold implements context-sensitive child counting.
func (ctx *walkContext) checkChildCountThreshold(n *tree_sitter.Node, matchConcept string, threshold int) bool {
	switch matchConcept {
	case analyzer.ConceptFunction:
		return ctx.countFunctionParams(n) > threshold
	case analyzer.ConceptClass:
		return countFieldsRecursive(n) > threshold
	case analyzer.ConceptSwitch:
		return ctx.countSwitchCases(n) > threshold
	case analyzer.ConceptImport:
		return ctx.countImportSpecs(n) > threshold
	case analyzer.ConceptInterface:
		return ctx.countInterfaceMethods(n) > threshold
	default:
		// Generic child count
		return int(n.ChildCount()) > threshold
	}
}

// countFunctionParams counts parameter declarations in a function node.
func (ctx *walkContext) countFunctionParams(n *tree_sitter.Node) int {
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		kind := c.Kind()
		if kind == "parameter_list" || kind == "formal_parameters" || kind == "parameters" {
			paramCount := 0
			for j := uint(0); j < uint(c.ChildCount()); j++ {
				pk := c.Child(j).Kind()
				if pk != "," && pk != "(" && pk != ")" {
					paramCount++
				}
			}
			return paramCount
		}
	}
	return 0
}

// countSwitchCases counts case clauses in a switch node.
func (ctx *walkContext) countSwitchCases(n *tree_sitter.Node) int {
	caseCount := 0
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		ck := n.Child(i).Kind()
		if ck == "case_clause" || ck == "expression_case" || ck == "default_case" ||
			ck == "case" || ck == "communication_case" || ck == "match_arm" {
			caseCount++
		}
	}
	return caseCount
}

// countImportSpecs counts import specifications in an import node.
func (ctx *walkContext) countImportSpecs(n *tree_sitter.Node) int {
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
	return importCount
}

// countInterfaceMethods counts methods in an interface node.
// For Go: walks through type_declaration > type_spec > interface_type.
func (ctx *walkContext) countInterfaceMethods(n *tree_sitter.Node) int {
	// For Go type_declaration containing interface
	if ctx.lang == "go" && n.Kind() == "type_declaration" {
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
						return methodCount
					}
				}
			}
		}
	}
	// Generic: count children
	return int(n.ChildCount())
}

// evalNestingDepth recursively checks nesting depth.
func (ctx *walkContext) evalNestingDepth(n *tree_sitter.Node, r analyzer.Rule, result *WalkResult, depth int) {
	kind := n.Kind()
	isNesting := analyzer.IsNodeKind(ctx.lang, analyzer.ConceptForLoop, kind) ||
		kind == "if_statement" || kind == "if_expression" ||
		kind == "switch_statement" || kind == "select_statement" ||
		kind == "match_expression" || kind == "case_statement"

	newDepth := depth
	if isNesting {
		newDepth++
		if newDepth > r.Structural.NestingThreshold {
			result.Findings = append(result.Findings, analyzer.RuleFinding{
				RuleID:   r.ID,
				Line:     int(n.StartPosition().Row + 1),
				Severity: r.Severity,
			})
			return // don't report deeper nesting
		}
	}

	for i := uint(0); i < uint(n.ChildCount()); i++ {
		ctx.evalNestingDepth(n.Child(i), r, result, newDepth)
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
