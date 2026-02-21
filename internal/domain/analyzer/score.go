package analyzer

import "sort"

// severityWeight maps severity levels to scoring weights.
var severityWeight = [4]int{1, 3, 7, 10} // info=1, warning=3, high=7, critical=10

// Score computes a weighted severity sum from the set bits in the bitmask.
// Each set bit is looked up in the rules slice to find its severity weight.
func Score(mask Bitmask, rules []Rule) int {
	ruleIndex := buildRuleIndex(rules)
	total := 0
	for tier := 0; tier < TierCount; tier++ {
		bits := mask[tier]
		for bits != 0 {
			// Find lowest set bit
			bit := bits & (-bits)
			pos := bitPos(bit)
			if r, ok := ruleIndex[tierBit{Tier(tier), pos}]; ok {
				if int(r.Severity) < len(severityWeight) {
					total += severityWeight[r.Severity]
				}
			}
			bits &^= bit // clear lowest set bit
		}
	}
	return total
}

// bitPos returns the position of a single set bit (0-63).
func bitPos(v uint64) int {
	n := 0
	if v&0xFFFFFFFF00000000 != 0 {
		n += 32
		v >>= 32
	}
	if v&0xFFFF0000 != 0 {
		n += 16
		v >>= 16
	}
	if v&0xFF00 != 0 {
		n += 8
		v >>= 8
	}
	if v&0xF0 != 0 {
		n += 4
		v >>= 4
	}
	if v&0xC != 0 {
		n += 2
		v >>= 2
	}
	if v&0x2 != 0 {
		n++
	}
	return n
}

type tierBit struct {
	tier Tier
	bit  int
}

func buildRuleIndex(rules []Rule) map[tierBit]Rule {
	idx := make(map[tierBit]Rule, len(rules))
	for _, r := range rules {
		idx[tierBit{r.Tier, r.Bit}] = r
	}
	return idx
}

// Compose attributes findings to enclosing methods and builds per-method bitmasks.
// Findings outside any symbol go to a synthetic "<file>" method.
// Returns method analyses sorted by line number.
func Compose(findings []RuleFinding, symbols []SymbolSpan, rules []Rule) []MethodAnalysis {
	if len(findings) == 0 {
		return nil
	}

	ruleMap := make(map[string]Rule, len(rules))
	for _, r := range rules {
		ruleMap[r.ID] = r
	}

	// Sort symbols by start line for binary search
	sortedSyms := make([]SymbolSpan, len(symbols))
	copy(sortedSyms, symbols)
	sort.Slice(sortedSyms, func(i, j int) bool {
		return sortedSyms[i].StartLine < sortedSyms[j].StartLine
	})

	// Group findings by enclosing method
	type methodKey struct {
		name    string
		line    int
		endLine int
	}
	methodFindings := make(map[methodKey][]RuleFinding)
	methodOrder := make(map[methodKey]int) // track insertion order

	// Dedup: one finding per (ruleID, line)
	type dedupKey struct {
		ruleID string
		line   int
	}
	seen := make(map[dedupKey]bool)

	orderCounter := 0
	for _, f := range findings {
		dk := dedupKey{f.RuleID, f.Line}
		if seen[dk] {
			continue
		}
		seen[dk] = true

		sym := findEnclosingSymbol(sortedSyms, f.Line)
		mk := methodKey{name: sym.Name, line: sym.StartLine, endLine: sym.EndLine}
		if _, exists := methodOrder[mk]; !exists {
			methodOrder[mk] = orderCounter
			orderCounter++
		}
		methodFindings[mk] = append(methodFindings[mk], f)
	}

	// Build method analyses
	methods := make([]MethodAnalysis, 0, len(methodFindings))
	for mk, mf := range methodFindings {
		var mask Bitmask
		for _, f := range mf {
			if r, ok := ruleMap[f.RuleID]; ok {
				mask.Set(r.Tier, r.Bit)
			}
		}
		methods = append(methods, MethodAnalysis{
			Name:     mk.name,
			Line:     mk.line,
			EndLine:  mk.endLine,
			Bitmask:  mask,
			Score:    Score(mask, rules),
			Findings: mf,
		})
	}

	// Sort by line number
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Line < methods[j].Line
	})

	return methods
}

// findEnclosingSymbol returns the symbol that contains the given line.
// If no symbol encloses the line, returns a synthetic "<file>" span.
func findEnclosingSymbol(symbols []SymbolSpan, line int) SymbolSpan {
	// Walk backward to find the innermost enclosing symbol
	for i := len(symbols) - 1; i >= 0; i-- {
		s := symbols[i]
		if line >= s.StartLine && (s.EndLine == 0 || line <= s.EndLine) {
			return s
		}
	}
	// Fallback: closest preceding symbol
	for i := len(symbols) - 1; i >= 0; i-- {
		if line >= symbols[i].StartLine {
			return symbols[i]
		}
	}
	return SymbolSpan{Name: "<file>", StartLine: 0, EndLine: 0}
}
