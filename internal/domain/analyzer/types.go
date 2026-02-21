// Package analyzer defines the dimensional analysis engine types.
// All types are pure Go with no external dependencies.
package analyzer

import "math/bits"

// Tier represents a dimension tier (security, performance, quality, observability).
type Tier int

const (
	TierSecurity      Tier = 0
	TierPerformance   Tier = 1
	TierQuality       Tier = 2
	TierObservability Tier = 3
)

// TierCount is the number of tiers in the bitmask.
const TierCount = 4

// Bitmask holds one uint64 per tier, with 64 bit positions each.
// Bit positions map 1:1 with rule definitions.
type Bitmask [TierCount]uint64

// Set sets the bit for the given tier and position.
// Positions must be 0-63; out-of-range is silently ignored.
func (b *Bitmask) Set(tier Tier, bit int) {
	if bit < 0 || bit > 63 || int(tier) < 0 || int(tier) >= TierCount {
		return
	}
	b[tier] |= 1 << uint(bit)
}

// Has returns true if the bit at the given tier and position is set.
func (b *Bitmask) Has(tier Tier, bit int) bool {
	if bit < 0 || bit > 63 || int(tier) < 0 || int(tier) >= TierCount {
		return false
	}
	return b[tier]&(1<<uint(bit)) != 0
}

// Or merges another bitmask into this one (bitwise OR).
func (b *Bitmask) Or(other Bitmask) {
	for i := 0; i < TierCount; i++ {
		b[i] |= other[i]
	}
}

// PopCount returns the total number of set bits across all tiers.
func (b *Bitmask) PopCount() int {
	n := 0
	for i := 0; i < TierCount; i++ {
		n += bits.OnesCount64(b[i])
	}
	return n
}

// IsZero returns true if no bits are set in any tier.
func (b *Bitmask) IsZero() bool {
	for i := 0; i < TierCount; i++ {
		if b[i] != 0 {
			return false
		}
	}
	return true
}

// Severity represents the severity level of a finding.
type Severity int

const (
	SevInfo     Severity = 0
	SevWarning  Severity = 1
	SevHigh     Severity = 2
	SevCritical Severity = 3
)

// String returns the human-readable severity label.
func (s Severity) String() string {
	switch s {
	case SevInfo:
		return "info"
	case SevWarning:
		return "warning"
	case SevHigh:
		return "high"
	case SevCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// RuleKind indicates how a rule matches: text patterns, AST structure, or both.
type RuleKind int

const (
	RuleText       RuleKind = iota // AC text matching only
	RuleStructural                 // AST walker only
	RuleComposite                  // AC hit confirmed by AST
)

// Rule defines a single dimensional analysis rule.
type Rule struct {
	ID              string   // unique identifier, e.g. "hardcoded_secret"
	Label           string   // human-readable description
	Dimension       string   // dimension within the tier, e.g. "secrets", "injection"
	StructuralCheck string   // check function name for structural/composite rules
	Tier            Tier     // which tier this rule belongs to
	Bit             int      // bit position 0-63 within the tier
	Severity        Severity // finding severity
	Kind            RuleKind // text, structural, or composite
	TextPatterns    []string // patterns for AC automaton (text + composite rules)
	SkipTest        bool     // skip test files
	SkipMain        bool     // skip main/cmd packages
	CodeOnly        bool     // only scan code files
}

// RuleFinding records a single rule match at a specific location.
type RuleFinding struct {
	RuleID   string   `json:"rule_id"`
	Line     int      `json:"line"`
	Symbol   string   `json:"symbol"`
	Severity Severity `json:"severity"`
}

// SymbolSpan describes a named code symbol's line range.
type SymbolSpan struct {
	Name      string
	StartLine int
	EndLine   int
}

// MethodAnalysis holds dimensional results for a single method/function.
type MethodAnalysis struct {
	Name     string        `json:"name"`
	Line     int           `json:"line"`
	EndLine  int           `json:"end_line"`
	Bitmask  Bitmask       `json:"bitmask"`
	Score    int           `json:"score"`
	Findings []RuleFinding `json:"findings"`
}

// FileAnalysis holds dimensional results for a single file.
type FileAnalysis struct {
	Path     string           `json:"path"`
	Language string           `json:"language"`
	Bitmask  Bitmask          `json:"bitmask"`
	Methods  []MethodAnalysis `json:"methods"`
	Findings []RuleFinding    `json:"findings"`
	ScanTime int64            `json:"scan_time_us"` // microseconds
}
