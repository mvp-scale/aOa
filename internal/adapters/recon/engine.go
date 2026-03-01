//go:build core

package recon

import (
	"regexp"
	"time"

	"github.com/corey/aoa/internal/adapters/ahocorasick"
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/domain/analyzer"
)

// textRuleEntry maps an AC pattern index back to its owning rule.
type textRuleEntry struct {
	ruleID       string
	patternIndex int // index within the rule's TextPatterns
}

// Engine orchestrates AC text scanning + AST structural walking + bitmask composition.
type Engine struct {
	rules       []analyzer.Rule
	scanner     *ahocorasick.TextScanner
	parser      *treesitter.Parser
	textRuleMap []textRuleEntry            // AC global pattern index → rule attribution
	regexCache  map[string]*regexp.Regexp  // rule ID → compiled regex (Layer 3)
}

// NewEngine creates a dimensional analysis engine.
// It builds the AC automaton from all text patterns across all rules.
func NewEngine(rules []analyzer.Rule, parser *treesitter.Parser) *Engine {
	// Collect all text patterns and build the mapping
	var allPatterns []string
	var ruleMap []textRuleEntry

	for _, r := range rules {
		if len(r.TextPatterns) == 0 {
			continue
		}
		for i, pat := range r.TextPatterns {
			ruleMap = append(ruleMap, textRuleEntry{
				ruleID:       r.ID,
				patternIndex: i,
			})
			allPatterns = append(allPatterns, pat)
		}
	}

	var scanner *ahocorasick.TextScanner
	if len(allPatterns) > 0 {
		scanner = ahocorasick.NewTextScanner(allPatterns)
	}

	// Build regex cache for rules with regex confirmation (Layer 3)
	regexCache := make(map[string]*regexp.Regexp)
	for _, r := range rules {
		if r.Regex != "" {
			if compiled, err := regexp.Compile(r.Regex); err == nil {
				regexCache[r.ID] = compiled
			}
		}
	}

	return &Engine{
		rules:       rules,
		scanner:     scanner,
		parser:      parser,
		textRuleMap: ruleMap,
		regexCache:  regexCache,
	}
}

// AnalyzeFile runs the full dimensional analysis pipeline on a single file.
// Returns nil if no findings are detected.
func (e *Engine) AnalyzeFile(filePath string, source []byte, isTest, isMain bool) *analyzer.FileAnalysis {
	start := time.Now()

	if len(source) == 0 {
		return nil
	}

	// Build line offset table for byte offset → line number conversion
	lineOffsets := buildLineOffsets(source)

	// Build rule lookup
	ruleMap := make(map[string]analyzer.Rule, len(e.rules))
	for _, r := range e.rules {
		ruleMap[r.ID] = r
	}

	var allFindings []analyzer.RuleFinding
	var symbols []analyzer.SymbolSpan

	// Step 1: AC text scan
	if e.scanner != nil {
		textMatches := e.scanner.Scan(source)
		// Dedup: one finding per (ruleID, line)
		type dedupKey struct {
			ruleID string
			line   int
		}
		seen := make(map[dedupKey]bool)

		for _, m := range textMatches {
			if m.PatternIndex >= len(e.textRuleMap) {
				continue
			}
			entry := e.textRuleMap[m.PatternIndex]
			rule, ok := ruleMap[entry.ruleID]
			if !ok {
				continue
			}

			// Skip filters
			if rule.SkipTest && isTest {
				continue
			}
			if rule.SkipMain && isMain {
				continue
			}

			line := offsetToLine(lineOffsets, m.Start)

			// Comment-line filtering
			if isCommentLine(source, lineOffsets, line) {
				continue
			}

			dk := dedupKey{entry.ruleID, line}
			if seen[dk] {
				continue
			}
			seen[dk] = true

			// For composite rules, only mark as a text hit; AST must confirm
			if rule.Kind == analyzer.RuleComposite {
				continue // composites handled separately after AST walk
			}

			// Regex confirmation (Layer 3): if rule has regex, extract line text and confirm
			if re, ok := e.regexCache[rule.ID]; ok {
				lineText := extractLineText(source, lineOffsets, line)
				if !re.Match(lineText) {
					continue
				}
			}

			allFindings = append(allFindings, analyzer.RuleFinding{
				RuleID:   rule.ID,
				Line:     line,
				Severity: rule.Severity,
			})
		}
	}

	// Step 2: AST parse + structural walk
	var structuralFindings map[string][]int // rule ID → list of lines with structural findings
	if e.parser != nil {
		tree, lang, err := e.parser.ParseToTree(filePath, source)
		if err == nil && tree != nil {
			defer tree.Close()

			walkResult := treesitter.WalkForDimensions(tree.RootNode(), source, lang, e.rules, isMain)

			// Build structural findings index for composite resolution
			structuralFindings = make(map[string][]int)

			// Add structural findings (with skip filters)
			for _, f := range walkResult.Findings {
				rule, ok := ruleMap[f.RuleID]
				if !ok {
					continue
				}
				if rule.SkipTest && isTest {
					continue
				}

				structuralFindings[f.RuleID] = append(structuralFindings[f.RuleID], f.Line)

				// Only add pure structural findings directly
				// Composite findings go through resolveComposites
				if rule.Kind == analyzer.RuleStructural {
					allFindings = append(allFindings, f)
				}
			}

			// Use walker's symbol spans
			symbols = walkResult.Symbols

			// Step 3: Composite rule intersection
			// For composite rules: all present layers must agree (ADR constraint 6)
			if e.scanner != nil {
				compositeFindings := e.resolveComposites(source, lineOffsets, isTest, isMain, ruleMap, structuralFindings)
				allFindings = append(allFindings, compositeFindings...)
			}
		}
	}

	if len(allFindings) == 0 {
		return nil
	}

	// Step 4: Compose per-method bitmasks
	methods := analyzer.Compose(allFindings, symbols, e.rules)

	// Build file-level bitmask from all method bitmasks
	var fileMask analyzer.Bitmask
	for _, m := range methods {
		fileMask.Or(m.Bitmask)
	}

	// Detect language from extension
	lang := ""
	if e.parser != nil {
		_, detectedLang, _ := e.parser.ParseToTree(filePath, nil)
		lang = detectedLang
	}

	return &analyzer.FileAnalysis{
		Path:     filePath,
		Language: lang,
		Bitmask:  fileMask,
		Methods:  methods,
		Findings: allFindings,
		ScanTime: time.Since(start).Microseconds(),
	}
}

// resolveComposites checks composite rules: AC text hit + structural confirmation.
// For composite rules with a Structural block, verifies the structural finding
// exists on the same or nearby line as the text hit. All present layers must agree.
func (e *Engine) resolveComposites(source []byte, lineOffsets []int, isTest, isMain bool, ruleMap map[string]analyzer.Rule, structuralFindings map[string][]int) []analyzer.RuleFinding {
	var findings []analyzer.RuleFinding

	// Get text matches for composite rules
	textMatches := e.scanner.Scan(source)

	type dedupKey struct {
		ruleID string
		line   int
	}
	seen := make(map[dedupKey]bool)

	for _, m := range textMatches {
		if m.PatternIndex >= len(e.textRuleMap) {
			continue
		}
		entry := e.textRuleMap[m.PatternIndex]
		rule, ok := ruleMap[entry.ruleID]
		if !ok || rule.Kind != analyzer.RuleComposite {
			continue
		}
		if rule.SkipTest && isTest {
			continue
		}
		if rule.SkipMain && isMain {
			continue
		}

		line := offsetToLine(lineOffsets, m.Start)
		if isCommentLine(source, lineOffsets, line) {
			continue
		}

		dk := dedupKey{entry.ruleID, line}
		if seen[dk] {
			continue
		}

		// If rule has a Structural block, verify structural finding exists nearby
		if rule.Structural != nil && structuralFindings != nil {
			structLines := structuralFindings[rule.ID]
			if !hasNearbyLine(structLines, line, 3) {
				continue // structural layer didn't confirm — skip
			}
		}

		// Regex confirmation (Layer 3)
		if re, ok := e.regexCache[rule.ID]; ok {
			lineText := extractLineText(source, lineOffsets, line)
			if !re.Match(lineText) {
				continue
			}
		}

		seen[dk] = true
		findings = append(findings, analyzer.RuleFinding{
			RuleID:   rule.ID,
			Line:     line,
			Severity: rule.Severity,
		})
	}

	return findings
}

// hasNearbyLine checks if any line in the slice is within distance of target.
func hasNearbyLine(lines []int, target, distance int) bool {
	for _, l := range lines {
		diff := l - target
		if diff < 0 {
			diff = -diff
		}
		if diff <= distance {
			return true
		}
	}
	return false
}

// extractLineText extracts the raw bytes for a given 1-indexed line.
func extractLineText(source []byte, lineOffsets []int, line int) []byte {
	if line < 1 || line > len(lineOffsets) {
		return nil
	}
	start := lineOffsets[line-1]
	end := len(source)
	if line < len(lineOffsets) {
		end = lineOffsets[line]
	}
	return source[start:end]
}

// buildLineOffsets returns a slice where lineOffsets[i] is the byte offset
// of the start of line i+1 (1-indexed lines). lineOffsets[0] = 0 (line 1 starts at byte 0).
func buildLineOffsets(source []byte) []int {
	offsets := []int{0}
	for i, b := range source {
		if b == '\n' && i+1 < len(source) {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

// offsetToLine converts a byte offset to a 1-indexed line number.
func offsetToLine(offsets []int, byteOffset int) int {
	// Binary search for the largest offset <= byteOffset
	lo, hi := 0, len(offsets)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if offsets[mid] <= byteOffset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo + 1 // 1-indexed
}

// isCommentLine checks if the given line starts with a comment marker.
func isCommentLine(source []byte, lineOffsets []int, line int) bool {
	if line < 1 || line > len(lineOffsets) {
		return false
	}
	start := lineOffsets[line-1]
	// Find the trimmed start
	for start < len(source) && (source[start] == ' ' || source[start] == '\t') {
		start++
	}
	if start >= len(source) {
		return false
	}
	rest := source[start:]
	return len(rest) >= 2 && (string(rest[:2]) == "//" ||
		string(rest[:2]) == "/*" ||
		rest[0] == '#' ||
		rest[0] == '*')
}

// RuleCount returns the number of rules configured in this engine.
func (e *Engine) RuleCount() int {
	return len(e.rules)
}
