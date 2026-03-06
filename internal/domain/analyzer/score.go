// Copyright 2025-2026 Corey Gorman / MVP-Scale.com
// Licensed under the Apache License, Version 2.0
//
// Patent Pending: The SignalScore algorithm (multi-signal scoring from
// per-line bitmask topology) is Patent Pending. See NOTICE for details.

package analyzer

import (
	"math"
	"sort"
)

// severityWeight maps severity levels to scoring weights.
var severityWeight = [4]int{1, 3, 7, 10} // info=1, warning=3, high=7, critical=10

// SignalScore computes a method's score from its per-line bitmask topology.
// It combines six signals into a single score:
//
//  1. Severity floor — critical/high always contribute full weight
//  2. Density modulation — warning/info scaled by hit density within the method
//  3. Amplifier — priority rules bypass density (floor from amplifier, density adds)
//  4. Co-occurrence — bits on the same line amplify each other (+2 per pair)
//  5. Clustering — adjacent signal lines form compound blocks (+1 per unique bit)
//  6. Breadth — many distinct warning+ bits = systematic debt (+1 per bit beyond 2)
//
// Gate: methods with score >= 6 surface as findings. Below that is noise.
func SignalScore(findings []RuleFinding, rules []Rule, totalLines int) int {
	if len(findings) == 0 || totalLines <= 0 {
		return 0
	}

	ruleIndex := buildRuleIndex(rules)

	// ── Analyze per-line topology ──────────────────────────────────
	type bitInfo struct {
		lines    map[int]bool
		severity Severity
		rule     Rule
	}
	perBit := make(map[tierBit]*bitInfo)
	lineSignals := make(map[int]map[tierBit]bool)

	for _, f := range findings {
		r, ok := ruleIndex[tierBit{ruleIndex[tierBitFromID(f.RuleID, ruleIndex)].Tier, ruleIndex[tierBitFromID(f.RuleID, ruleIndex)].Bit}]
		if !ok {
			// Look up by ID instead
			for _, rule := range rules {
				if rule.ID == f.RuleID {
					r = rule
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		tb := tierBit{r.Tier, r.Bit}
		bi, exists := perBit[tb]
		if !exists {
			bi = &bitInfo{lines: make(map[int]bool), severity: r.Severity, rule: r}
			perBit[tb] = bi
		}
		bi.lines[f.Line] = true

		if lineSignals[f.Line] == nil {
			lineSignals[f.Line] = make(map[tierBit]bool)
		}
		lineSignals[f.Line][tb] = true
	}

	tl := float64(totalLines)

	// ── Component 1: Severity-anchored base with amplifier ────────
	base := 0.0
	warnBits := 0
	for _, bi := range perBit {
		w := float64(severityWeight[bi.severity])
		density := float64(len(bi.lines)) / tl

		if bi.rule.Amplifier > 0 {
			// Floor from amplifier + density bonus on top
			base += w*bi.rule.Amplifier + density*w*10
			if bi.severity >= SevWarning {
				warnBits++
			}
			continue
		}

		switch {
		case bi.severity >= SevCritical:
			base += w
		case bi.severity >= SevHigh:
			base += w * math.Max(0.5, math.Min(1.0, density*10))
		case bi.severity >= SevWarning:
			base += w * math.Min(1.0, density*10)
			warnBits++
		default:
			base += w * math.Min(1.0, density*5)
		}
	}

	// ── Component 2: Co-occurrence ────────────────────────────────
	coBonus := 0.0
	for _, bits := range lineSignals {
		if n := len(bits); n >= 2 {
			coBonus += float64(n*(n-1)/2) * 2.0
		}
	}

	// ── Component 3: Clustering ───────────────────────────────────
	clusterBonus := 0.0
	if len(lineSignals) >= 2 {
		clusterBits := countClusterBitsFromLines(lineSignals)
		if clusterBits >= 2 {
			clusterBonus = float64(clusterBits) * 1.0
		}
	}

	// ── Component 4: Breadth ──────────────────────────────────────
	breadthBonus := 0.0
	if warnBits >= 3 {
		breadthBonus = float64(warnBits-2) * 1.0
	}
	if len(perBit) >= 4 && breadthBonus == 0 {
		breadthBonus = float64(len(perBit)-3) * 0.5
	}

	return int(base + coBonus + clusterBonus + breadthBonus)
}

// countClusterBitsFromLines finds the largest cluster of adjacent signal lines
// (±2 gap tolerance) and returns the number of unique bits in that cluster.
func countClusterBitsFromLines(lineSignals map[int]map[tierBit]bool) int {
	lines := make([]int, 0, len(lineSignals))
	for l := range lineSignals {
		lines = append(lines, l)
	}
	sort.Ints(lines)

	bestBits := make(map[tierBit]bool)
	curBits := make(map[tierBit]bool)

	// Seed with first line
	for tb := range lineSignals[lines[0]] {
		curBits[tb] = true
	}

	for i := 1; i < len(lines); i++ {
		if lines[i] <= lines[i-1]+2 {
			for tb := range lineSignals[lines[i]] {
				curBits[tb] = true
			}
		} else {
			if len(curBits) > len(bestBits) {
				bestBits = curBits
			}
			curBits = make(map[tierBit]bool)
			for tb := range lineSignals[lines[i]] {
				curBits[tb] = true
			}
		}
	}
	if len(curBits) > len(bestBits) {
		bestBits = curBits
	}

	return len(bestBits)
}

// tierBitFromID looks up a rule by ID and returns its tierBit key.
// This is a helper for the common case where we have a RuleID string.
func tierBitFromID(ruleID string, idx map[tierBit]Rule) tierBit {
	for tb, r := range idx {
		if r.ID == ruleID {
			return tb
		}
	}
	return tierBit{}
}

// Score computes a weighted severity sum from the set bits in the bitmask.
// Each set bit is looked up in the rules slice to find its severity weight.
// Deprecated: use SignalScore for method-level scoring with full topology.
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
		// Compute method line span for density calculations.
		// For synthetic <file> methods (line=0), use count of distinct
		// finding lines as a proxy for method size.
		methodLines := mk.endLine - mk.line + 1
		if methodLines <= 0 {
			lineSet := make(map[int]bool, len(mf))
			for _, f := range mf {
				lineSet[f.Line] = true
			}
			methodLines = len(lineSet)
			if methodLines == 0 {
				methodLines = 1
			}
		}

		methods = append(methods, MethodAnalysis{
			Name:     mk.name,
			Line:     mk.line,
			EndLine:  mk.endLine,
			Bitmask:  mask,
			Score:    SignalScore(mf, rules, methodLines),
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
