// Package treesitter — HTTP route extractors (VL-3, board #37).
//
// Each extractor walks the SAME top-level shape as extractGo/extractImports
// (root.Child(i)), then descends into a top-level function/method's body up
// to 3 more hops — block -> statement -> call_expression — to catch a
// route-registration call written directly in a setup function (the real-
// world idiom: gin/net-http routes are registered inside func main() or a
// setupRouter()-style helper, never at file scope). D9 boundary: this is a
// bounded descent from a node the walk already visits (the top-level
// function_declaration), not a new unbounded traversal — nested control-flow
// bodies (if/for/switch) are NOT walked into; a route call inside one is out
// of v1 scope (documented, not silently mis-extracted).
//
// Detection is a syntactic method-name match against known HTTP-verb/
// registration method names (gin: GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS/
// Any; net/http: HandleFunc/Handle) with a string-literal first argument —
// the same honesty tier as extractImportsGo's literal spec: no type
// resolution, so a same-named method on an unrelated receiver also matches.
//
// Languages: Go only (v1 scope — "Go stacks first" per the work order).
package treesitter

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/corey/aoa/internal/ports"
)

// extractRoutes dispatches to the language-specific route extractor via the
// routeExtractors registry (extractors.go, FDN-2 plug point). Returns nil
// for languages without a route extractor.
func extractRoutes(root *tree_sitter.Node, source []byte, filePath, lang string) []ports.RouteEdge {
	if fn, ok := routeExtractors[lang]; ok {
		return fn(root, source, filePath)
	}
	return nil
}

// ginRouteVerbs maps a gin router method name to its HTTP verb. "Any"
// registers a handler for every verb — surfaced as the literal verb "ANY"
// rather than guessing/expanding to all seven (honest: that is what the
// source says).
var ginRouteVerbs = map[string]string{
	"GET":     "GET",
	"POST":    "POST",
	"PUT":     "PUT",
	"DELETE":  "DELETE",
	"PATCH":   "PATCH",
	"HEAD":    "HEAD",
	"OPTIONS": "OPTIONS",
	"Any":     "ANY",
}

// netHTTPRouteMethods is the set of net/http (and net/http-compatible mux)
// method names that register a route. Neither carries an explicit verb in
// the call itself (the handler dispatches on r.Method internally) — Method
// is left "" rather than guessed.
var netHTTPRouteMethods = map[string]bool{
	"HandleFunc": true,
	"Handle":     true,
}

// classifyRouteMethod reports whether methodName is a known route-
// registration method, and if so which framework/verb it belongs to.
func classifyRouteMethod(methodName string) (framework, verb string, ok bool) {
	if v, found := ginRouteVerbs[methodName]; found {
		return "gin", v, true
	}
	if netHTTPRouteMethods[methodName] {
		return "net/http", "", true
	}
	return "", "", false
}

// extractRoutesGo extracts route-registration calls from a Go source file.
// Mirrors extractGo's top-level iteration, then bounded-descends into each
// top-level function/method's body (function_declaration/method_declaration
// -> block -> statement -> call_expression).
func extractRoutesGo(root *tree_sitter.Node, source []byte, filePath string) []ports.RouteEdge {
	var routes []ports.RouteEdge

	for i := uint(0); i < uint(root.ChildCount()); i++ {
		fn := root.Child(i)
		if fn.Kind() != "function_declaration" && fn.Kind() != "method_declaration" {
			continue
		}
		body := childByKind(fn, "block")
		if body == nil {
			continue
		}
		for j := uint(0); j < uint(body.ChildCount()); j++ {
			stmt := body.Child(j)
			call := callExpressionInStatement(stmt)
			if call == nil {
				continue
			}
			if edge, ok := routeFromCall(call, source, filePath); ok {
				routes = append(routes, edge)
			}
		}
	}

	return routes
}

// callExpressionInStatement returns the call_expression a single body
// statement wraps, or nil. Only unwraps ONE level (expression_statement's
// sole child) — matches the "route call written as its own statement"
// idiom (`r.GET(...)`); assignments/short-var-decls that merely construct a
// router (`r := gin.Default()`) are not calls we walk into further, keeping
// the descent bounded (D9).
func callExpressionInStatement(stmt *tree_sitter.Node) *tree_sitter.Node {
	if stmt.Kind() == "call_expression" {
		return stmt
	}
	if stmt.Kind() == "expression_statement" && stmt.NamedChildCount() > 0 {
		if child := stmt.NamedChild(0); child != nil && child.Kind() == "call_expression" {
			return child
		}
	}
	return nil
}

// routeFromCall inspects one call_expression and, if its function is a
// selector whose field matches a known route-registration method name with
// a string-literal first argument (the route path), returns the RouteEdge.
func routeFromCall(call *tree_sitter.Node, source []byte, filePath string) (ports.RouteEdge, bool) {
	fnNode := call.ChildByFieldName("function")
	if fnNode == nil || fnNode.Kind() != "selector_expression" {
		return ports.RouteEdge{}, false
	}
	fieldNode := fnNode.ChildByFieldName("field")
	if fieldNode == nil {
		return ports.RouteEdge{}, false
	}
	framework, verb, ok := classifyRouteMethod(nodeText(fieldNode, source))
	if !ok {
		return ports.RouteEdge{}, false
	}

	args := call.ChildByFieldName("arguments")
	if args == nil || args.NamedChildCount() == 0 {
		return ports.RouteEdge{}, false
	}

	pathNode := args.NamedChild(0)
	if pathNode.Kind() != "interpreted_string_literal" && pathNode.Kind() != "raw_string_literal" {
		return ports.RouteEdge{}, false
	}
	path := unquote(nodeText(pathNode, source))
	if path == "" {
		return ports.RouteEdge{}, false
	}

	var handler string
	if args.NamedChildCount() > 1 {
		handler = nodeText(args.NamedChild(1), source)
	}

	return ports.RouteEdge{
		FromFile:  filePath,
		Framework: framework,
		Method:    verb,
		Path:      path,
		Handler:   handler,
		StartLine: uint32(call.StartPosition().Row + 1),
	}, true
}
