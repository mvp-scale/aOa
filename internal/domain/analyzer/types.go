// Package analyzer defines the dimensional analysis engine types.
// All types are pure Go with no external dependencies.
package analyzer

import "math/bits"

// Tier represents a dimension tier (security, performance, quality, observability, architecture).
// Slot 5 is reserved (formerly compliance) to preserve bitmask storage compatibility.
type Tier int

const (
	TierSecurity      Tier = 0
	TierPerformance   Tier = 1
	TierQuality       Tier = 2
	TierObservability Tier = 3
	TierArchitecture  Tier = 4
	TierReserved      Tier = 5 // formerly compliance — slot preserved for bitmask compat
)

// TierCount is the number of slots in the bitmask (includes reserved slot 5).
const TierCount = 6

// TierName returns the string label for a tier constant.
func TierName(t Tier) string {
	switch t {
	case TierSecurity:
		return "security"
	case TierPerformance:
		return "performance"
	case TierQuality:
		return "quality"
	case TierObservability:
		return "observability"
	case TierArchitecture:
		return "architecture"
	case TierReserved:
		return "reserved"
	default:
		return "unknown"
	}
}

// TierFromName maps a string tier name to its Tier constant.
// Returns -1 for unknown names.
func TierFromName(name string) Tier {
	switch name {
	case "security":
		return TierSecurity
	case "performance":
		return TierPerformance
	case "quality":
		return TierQuality
	case "observability":
		return TierObservability
	case "architecture":
		return TierArchitecture
	default:
		return -1
	}
}

// SeverityFromName maps a string severity name to its Severity constant.
// Returns -1 for unknown names.
func SeverityFromName(name string) Severity {
	switch name {
	case "info":
		return SevInfo
	case "warning":
		return SevWarning
	case "high":
		return SevHigh
	case "critical":
		return SevCritical
	default:
		return -1
	}
}

// RuleKindFromName maps a string kind name to its RuleKind constant.
// Returns -1 for unknown names.
func RuleKindFromName(name string) RuleKind {
	switch name {
	case "text":
		return RuleText
	case "structural":
		return RuleStructural
	case "composite":
		return RuleComposite
	default:
		return -1
	}
}

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
	ID           string           // unique identifier, e.g. "hardcoded_secret"
	Label        string           // human-readable description
	Dimension    string           // dimension within the tier, e.g. "secrets", "injection"
	Structural   *StructuralBlock // declarative structural constraints
	Regex        string           // optional regex confirmation pattern
	Tier         Tier             // which tier this rule belongs to
	Bit          int              // bit position 0-63 within the tier
	Severity     Severity         // finding severity
	Kind         RuleKind         // text, structural, or composite
	TextPatterns []string         // patterns for AC automaton (text + composite rules)
	SkipTest     bool             // skip test files
	SkipMain     bool             // skip main/cmd packages
	CodeOnly     bool             // only scan code files
	SkipLangs    []string         // languages to skip this rule for
}

// StructuralBlock describes declarative AST matching constraints.
// A node must satisfy ALL present fields to trigger the rule.
type StructuralBlock struct {
	Match               string   // concept to match, e.g. "call", "defer", "function"
	ReceiverContains    []string // node text must contain one of these (case-sensitive)
	Inside              string   // ancestor concept required, e.g. "for_loop"
	HasArg              *ArgSpec // argument child constraint
	NameContains        []string // identifier child must contain one of these substrings
	ValueType           string   // not yet used — reserved
	WithoutSibling      string   // semantic template name, e.g. "comma_ok", "doc_comment"
	NestingThreshold    int      // nesting depth threshold (0 = disabled)
	ChildCountThreshold int      // child count threshold (0 = disabled)
	ParentKinds         []string // not yet used — reserved
	TextContains        []string // node text (uppercased) must contain one of these
	LineThreshold       int      // line span threshold (0 = disabled)
}

// ArgSpec constrains function/call arguments.
type ArgSpec struct {
	Type         []string // required argument node concept types, e.g. ["identifier", "call"]
	TextContains []string // argument text must contain one of these
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
