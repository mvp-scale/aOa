// Package treesitter implements source code parsing using tree-sitter grammars.
// It extracts symbols (functions, classes, methods) from source files and produces
// ports.SymbolMeta entries for the search index.
//
// 56 languages compiled-in via CGo from official tree-sitter repos.
// Extension point for runtime .so loading via purego for additional grammars.
package treesitter

import (
	"path/filepath"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/corey/aoa/internal/ports"
)

// Symbol represents an extracted code symbol before it becomes a SymbolMeta.
type Symbol struct {
	Name      string
	Signature string
	Kind      string // "function", "class", "method", "struct", etc.
	StartLine uint16
	EndLine   uint16
	Parent    string
}

// Parser extracts symbols from source files using tree-sitter grammars.
type Parser struct {
	languages map[string]*tree_sitter.Language // lang name -> language
	extToLang map[string]string                // extension -> lang name
	loader    *DynamicLoader                   // optional: loads grammars from .so/.dylib
}

// NewParser creates a parser with all built-in grammars registered.
func NewParser() *Parser {
	p := &Parser{
		languages: make(map[string]*tree_sitter.Language),
		extToLang: make(map[string]string),
	}
	p.registerBuiltinLanguages()
	p.registerExtensions()
	return p
}

// addLang registers a language by name.
func (p *Parser) addLang(name string, lang *tree_sitter.Language) {
	if lang != nil {
		p.languages[name] = lang
	}
}

// addExt maps file extensions to a language name.
func (p *Parser) addExt(lang string, exts ...string) {
	for _, ext := range exts {
		p.extToLang[ext] = lang
	}
}

// ParseFile extracts symbols from source code given a file path and contents.
// Returns nil for unknown languages — not an error.
func (p *Parser) ParseFile(filePath string, source []byte) ([]Symbol, error) {
	langName := p.detectLanguage(filePath)
	if langName == "" {
		return nil, nil
	}

	lang, ok := p.languages[langName]
	if !ok && p.loader != nil {
		loaded, err := p.loader.LoadGrammar(langName)
		if err != nil {
			return nil, nil // dynamic loading failed, degrade gracefully
		}
		p.languages[langName] = loaded
		lang = loaded
	} else if !ok {
		return nil, nil // extension mapped but grammar not available
	}

	if len(source) == 0 {
		return nil, nil
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		return nil, err
	}

	tree := parser.Parse(source, nil)
	defer tree.Close()

	return extractSymbols(tree.RootNode(), source, langName), nil
}

// ParseFileToMeta converts symbols to ports.SymbolMeta.
func (p *Parser) ParseFileToMeta(filePath string, source []byte) ([]*ports.SymbolMeta, error) {
	symbols, err := p.ParseFile(filePath, source)
	if err != nil {
		return nil, err
	}
	metas := make([]*ports.SymbolMeta, len(symbols))
	for i, sym := range symbols {
		metas[i] = &ports.SymbolMeta{
			Name:      sym.Name,
			Signature: sym.Signature,
			Kind:      sym.Kind,
			StartLine: sym.StartLine,
			EndLine:   sym.EndLine,
			Parent:    sym.Parent,
		}
	}
	return metas, nil
}

// SupportsExtension returns true if the parser recognizes this file extension.
func (p *Parser) SupportsExtension(ext string) bool {
	_, ok := p.extToLang[strings.ToLower(ext)]
	return ok
}

// SupportedExtensions returns all registered file extensions.
func (p *Parser) SupportedExtensions() []string {
	exts := make([]string, 0, len(p.extToLang))
	for ext := range p.extToLang {
		exts = append(exts, ext)
	}
	return exts
}

// SetGrammarPaths configures the parser to load grammars dynamically from
// shared libraries found in the given directories. Project-local paths should
// come first, global paths last. This enables parsing of languages that don't
// have compiled-in grammars.
func (p *Parser) SetGrammarPaths(paths []string) {
	p.loader = NewDynamicLoader(paths)
}

// Loader returns the dynamic grammar loader, or nil if not configured.
func (p *Parser) Loader() *DynamicLoader {
	return p.loader
}

// LanguageCount returns the number of languages with compiled-in grammars.
func (p *Parser) LanguageCount() int {
	return len(p.languages)
}

// HasLanguage returns true if a grammar is available (compiled-in or dynamically loaded)
// for the given language name.
func (p *Parser) HasLanguage(lang string) bool {
	if _, ok := p.languages[lang]; ok {
		return true
	}
	if p.loader != nil {
		return p.loader.GrammarPath(lang) != ""
	}
	return false
}

// detectLanguage determines the language from the file path.
func (p *Parser) detectLanguage(filePath string) string {
	base := filepath.Base(filePath)

	// Special filenames (no extension)
	if lang, ok := p.extToLang[base]; ok {
		return lang
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if lang, ok := p.extToLang[ext]; ok {
		return lang
	}
	return ""
}

// extractSymbols dispatches to language-specific or generic extraction.
func extractSymbols(root *tree_sitter.Node, source []byte, lang string) []Symbol {
	switch lang {
	case "go":
		return extractGo(root, source)
	case "python":
		return extractPython(root, source, "")
	case "javascript", "typescript", "tsx":
		return extractJavaScript(root, source, "")
	default:
		return extractGeneric(root, source, lang)
	}
}

// extractGeneric uses the symbolRules table to extract symbols from any language.
// It walks top-level and one level of nesting (class bodies).
func extractGeneric(root *tree_sitter.Node, source []byte, lang string) []Symbol {
	rules, ok := symbolRules[lang]
	if !ok {
		return nil // no extraction rules for this language
	}

	var symbols []Symbol
	walkForSymbols(root, source, rules, "", &symbols, 0)
	return symbols
}

// walkForSymbols recursively walks the AST looking for symbol nodes.
// maxDepth prevents unbounded recursion (2 levels: top-level + class body).
func walkForSymbols(n *tree_sitter.Node, source []byte, rules map[string]string, parent string, symbols *[]Symbol, depth int) {
	if depth > 3 {
		return
	}

	for i := uint(0); i < uint(n.ChildCount()); i++ {
		child := n.Child(i)
		kind := child.Kind()

		if symKind, ok := rules[kind]; ok && symKind != "" {
			name := extractName(child, source)
			if name == "" {
				continue
			}

			actualKind := symKind
			actualParent := parent
			if parent != "" && (symKind == "function" || symKind == "method") {
				actualKind = "method"
				actualParent = parent
			}

			*symbols = append(*symbols, Symbol{
				Name:      name,
				Signature: buildGenericSignature(child, source, name),
				Kind:      actualKind,
				StartLine: uint16(child.StartPosition().Row + 1),
				EndLine:   uint16(child.EndPosition().Row + 1),
				Parent:    actualParent,
			})

			// If this is a class/struct/module, recurse into its body for methods
			if symKind == "class" || symKind == "module" || symKind == "impl" || symKind == "struct" || symKind == "interface" || symKind == "trait" || symKind == "object" {
				walkForSymbols(child, source, rules, name, symbols, depth+1)
			}
		} else {
			// Not a symbol node — recurse to find nested symbols
			walkForSymbols(child, source, rules, parent, symbols, depth+1)
		}
	}
}

// extractName finds the identifier/name node in a symbol declaration.
func extractName(n *tree_sitter.Node, source []byte) string {
	// Common name node types across languages
	nameKinds := []string{"identifier", "name", "field_identifier", "property_identifier", "type_identifier", "constant"}
	for _, kind := range nameKinds {
		if c := childByKind(n, kind); c != nil {
			return nodeText(c, source)
		}
	}
	return ""
}

// buildGenericSignature builds a signature from name + parameters if present.
func buildGenericSignature(n *tree_sitter.Node, source []byte, name string) string {
	// Look for parameter-like children
	paramKinds := []string{"parameters", "formal_parameters", "parameter_list", "arguments", "type_parameters"}
	for _, kind := range paramKinds {
		if p := childByKind(n, kind); p != nil {
			return name + nodeText(p, source)
		}
	}
	return name
}

// nodeText returns the source text for a node.
func nodeText(n *tree_sitter.Node, source []byte) string {
	return string(source[n.StartByte():n.EndByte()])
}

// childByKind finds the first child with the given kind.
func childByKind(n *tree_sitter.Node, kind string) *tree_sitter.Node {
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		if c.Kind() == kind {
			return c
		}
	}
	return nil
}

// ---------- Go extraction ----------

func extractGo(root *tree_sitter.Node, source []byte) []Symbol {
	var symbols []Symbol
	for i := uint(0); i < uint(root.ChildCount()); i++ {
		child := root.Child(i)
		switch child.Kind() {
		case "function_declaration":
			symbols = append(symbols, extractGoFunction(child, source))
		case "method_declaration":
			symbols = append(symbols, extractGoMethod(child, source))
		case "type_declaration":
			if sym, ok := extractGoType(child, source); ok {
				symbols = append(symbols, sym)
			}
		}
	}
	return symbols
}

func extractGoFunction(n *tree_sitter.Node, source []byte) Symbol {
	name := ""
	if id := childByKind(n, "identifier"); id != nil {
		name = nodeText(id, source)
	}
	return Symbol{
		Name:      name,
		Signature: buildGoSignature(n, source, name),
		Kind:      "function",
		StartLine: uint16(n.StartPosition().Row + 1),
		EndLine:   uint16(n.EndPosition().Row + 1),
	}
}

func extractGoMethod(n *tree_sitter.Node, source []byte) Symbol {
	name := ""
	if id := childByKind(n, "field_identifier"); id != nil {
		name = nodeText(id, source)
	}
	parent := ""
	if params := childByKind(n, "parameter_list"); params != nil {
		for j := uint(0); j < uint(params.ChildCount()); j++ {
			pd := params.Child(j)
			if pd.Kind() == "parameter_declaration" {
				for k := uint(0); k < uint(pd.ChildCount()); k++ {
					c := pd.Child(k)
					switch c.Kind() {
					case "type_identifier":
						parent = nodeText(c, source)
					case "pointer_type":
						if ti := childByKind(c, "type_identifier"); ti != nil {
							parent = nodeText(ti, source)
						}
					}
				}
				break
			}
		}
	}
	return Symbol{
		Name:      name,
		Signature: buildGoSignature(n, source, name),
		Kind:      "method",
		StartLine: uint16(n.StartPosition().Row + 1),
		EndLine:   uint16(n.EndPosition().Row + 1),
		Parent:    parent,
	}
}

func buildGoSignature(n *tree_sitter.Node, source []byte, name string) string {
	var paramList *tree_sitter.Node
	foundName := false
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		c := n.Child(i)
		if c.Kind() == "identifier" || c.Kind() == "field_identifier" {
			foundName = true
			continue
		}
		if foundName && c.Kind() == "parameter_list" {
			paramList = c
			break
		}
	}
	params := ""
	if paramList != nil {
		params = nodeText(paramList, source)
	}
	return name + params
}

func extractGoType(n *tree_sitter.Node, source []byte) (Symbol, bool) {
	spec := childByKind(n, "type_spec")
	if spec == nil {
		return Symbol{}, false
	}
	if childByKind(spec, "struct_type") == nil {
		return Symbol{}, false
	}
	name := ""
	if id := childByKind(spec, "type_identifier"); id != nil {
		name = nodeText(id, source)
	}
	return Symbol{
		Name:      name,
		Signature: "type " + name + " struct",
		Kind:      "struct",
		StartLine: uint16(n.StartPosition().Row + 1),
		EndLine:   uint16(n.EndPosition().Row + 1),
	}, true
}

// ---------- Python extraction ----------

func extractPython(n *tree_sitter.Node, source []byte, parent string) []Symbol {
	var symbols []Symbol
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		child := n.Child(i)
		switch child.Kind() {
		case "function_definition":
			symbols = append(symbols, extractPythonFunction(child, source, parent))
		case "class_definition":
			sym := extractPythonClass(child, source)
			symbols = append(symbols, sym)
			if body := childByKind(child, "block"); body != nil {
				symbols = append(symbols, extractPython(body, source, sym.Name)...)
			}
		case "decorated_definition":
			for j := uint(0); j < uint(child.ChildCount()); j++ {
				inner := child.Child(j)
				if inner.Kind() == "function_definition" {
					symbols = append(symbols, extractPythonFunction(inner, source, parent))
				} else if inner.Kind() == "class_definition" {
					sym := extractPythonClass(inner, source)
					symbols = append(symbols, sym)
					if body := childByKind(inner, "block"); body != nil {
						symbols = append(symbols, extractPython(body, source, sym.Name)...)
					}
				}
			}
		}
	}
	return symbols
}

func extractPythonFunction(n *tree_sitter.Node, source []byte, parent string) Symbol {
	name := ""
	if id := childByKind(n, "identifier"); id != nil {
		name = nodeText(id, source)
	}
	params := ""
	if p := childByKind(n, "parameters"); p != nil {
		params = nodeText(p, source)
	}
	kind := "function"
	if parent != "" {
		kind = "method"
	}
	return Symbol{
		Name:      name,
		Signature: name + params,
		Kind:      kind,
		StartLine: uint16(n.StartPosition().Row + 1),
		EndLine:   uint16(n.EndPosition().Row + 1),
		Parent:    parent,
	}
}

func extractPythonClass(n *tree_sitter.Node, source []byte) Symbol {
	name := ""
	if id := childByKind(n, "identifier"); id != nil {
		name = nodeText(id, source)
	}
	sig := "class " + name
	if argList := childByKind(n, "argument_list"); argList != nil {
		sig += nodeText(argList, source)
	}
	return Symbol{
		Name:      name,
		Signature: sig,
		Kind:      "class",
		StartLine: uint16(n.StartPosition().Row + 1),
		EndLine:   uint16(n.EndPosition().Row + 1),
	}
}

// ---------- JavaScript/TypeScript extraction ----------

func extractJavaScript(n *tree_sitter.Node, source []byte, parent string) []Symbol {
	var symbols []Symbol
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		child := n.Child(i)
		switch child.Kind() {
		case "function_declaration":
			symbols = append(symbols, extractJSFunction(child, source))
		case "class_declaration":
			sym := extractJSClass(child, source)
			symbols = append(symbols, sym)
			if body := childByKind(child, "class_body"); body != nil {
				symbols = append(symbols, extractJSClassBody(body, source, sym.Name)...)
			}
		case "lexical_declaration", "variable_declaration":
			symbols = append(symbols, extractJSVarDecl(child, source)...)
		}
	}
	return symbols
}

func extractJSFunction(n *tree_sitter.Node, source []byte) Symbol {
	name := ""
	if id := childByKind(n, "identifier"); id != nil {
		name = nodeText(id, source)
	}
	params := ""
	if p := childByKind(n, "formal_parameters"); p != nil {
		params = nodeText(p, source)
	}
	return Symbol{
		Name:      name,
		Signature: name + params,
		Kind:      "function",
		StartLine: uint16(n.StartPosition().Row + 1),
		EndLine:   uint16(n.EndPosition().Row + 1),
	}
}

func extractJSClass(n *tree_sitter.Node, source []byte) Symbol {
	name := ""
	if id := childByKind(n, "identifier"); id != nil {
		name = nodeText(id, source)
	}
	return Symbol{
		Name:      name,
		Signature: "class " + name,
		Kind:      "class",
		StartLine: uint16(n.StartPosition().Row + 1),
		EndLine:   uint16(n.EndPosition().Row + 1),
	}
}

func extractJSClassBody(n *tree_sitter.Node, source []byte, parent string) []Symbol {
	var symbols []Symbol
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		child := n.Child(i)
		if child.Kind() == "method_definition" {
			name := ""
			if id := childByKind(child, "property_identifier"); id != nil {
				name = nodeText(id, source)
			}
			params := ""
			if p := childByKind(child, "formal_parameters"); p != nil {
				params = nodeText(p, source)
			}
			symbols = append(symbols, Symbol{
				Name:      name,
				Signature: name + params,
				Kind:      "method",
				StartLine: uint16(child.StartPosition().Row + 1),
				EndLine:   uint16(child.EndPosition().Row + 1),
				Parent:    parent,
			})
		}
	}
	return symbols
}

func extractJSVarDecl(n *tree_sitter.Node, source []byte) []Symbol {
	var symbols []Symbol
	for i := uint(0); i < uint(n.ChildCount()); i++ {
		child := n.Child(i)
		if child.Kind() != "variable_declarator" {
			continue
		}
		nameNode := childByKind(child, "identifier")
		if nameNode == nil {
			continue
		}
		name := nodeText(nameNode, source)
		for j := uint(0); j < uint(child.ChildCount()); j++ {
			val := child.Child(j)
			if val.Kind() == "arrow_function" || val.Kind() == "function" {
				params := ""
				if p := childByKind(val, "formal_parameters"); p != nil {
					params = nodeText(p, source)
				}
				symbols = append(symbols, Symbol{
					Name:      name,
					Signature: name + params,
					Kind:      "function",
					StartLine: uint16(n.StartPosition().Row + 1),
					EndLine:   uint16(n.EndPosition().Row + 1),
				})
			}
		}
	}
	return symbols
}
