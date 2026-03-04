// Package analyzer defines the dimensional analysis engine types.
// Type definitions live in ports for hexagonal architecture compliance.
// This file re-exports them for backward compatibility within the domain.
package analyzer

import "github.com/corey/aoa/internal/ports"

// Re-export port types as aliases (zero cost, full backward compatibility).
type Tier = ports.Tier
type Bitmask = ports.Bitmask
type Severity = ports.Severity
type RuleKind = ports.RuleKind
type Rule = ports.Rule
type StructuralBlock = ports.StructuralBlock
type ArgSpec = ports.ArgSpec
type RuleFinding = ports.RuleFinding
type SymbolSpan = ports.SymbolSpan
type MethodAnalysis = ports.MethodAnalysis
type FileAnalysis = ports.FileAnalysis

// Re-export constants.
const (
	TierSecurity      = ports.TierSecurity
	TierPerformance   = ports.TierPerformance
	TierQuality       = ports.TierQuality
	TierObservability = ports.TierObservability
	TierArchitecture  = ports.TierArchitecture
	TierReserved      = ports.TierReserved
	TierCount         = ports.TierCount

	SevInfo     = ports.SevInfo
	SevWarning  = ports.SevWarning
	SevHigh     = ports.SevHigh
	SevCritical = ports.SevCritical

	RuleText       = ports.RuleText
	RuleStructural = ports.RuleStructural
	RuleComposite  = ports.RuleComposite
)

// Re-export functions.
var (
	TierName         = ports.TierName
	TierFromName     = ports.TierFromName
	SeverityFromName = ports.SeverityFromName
	RuleKindFromName = ports.RuleKindFromName
)
