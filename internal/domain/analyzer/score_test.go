package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScore_SingleRule(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Tier: TierSecurity, Bit: 0, Severity: SevCritical},
	}
	var mask Bitmask
	mask.Set(TierSecurity, 0)
	assert.Equal(t, 10, Score(mask, rules)) // critical = 10
}

func TestScore_MultipleRules(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Tier: TierSecurity, Bit: 0, Severity: SevCritical},
		{ID: "r2", Tier: TierSecurity, Bit: 1, Severity: SevWarning},
		{ID: "r3", Tier: TierQuality, Bit: 0, Severity: SevInfo},
	}
	var mask Bitmask
	mask.Set(TierSecurity, 0)
	mask.Set(TierSecurity, 1)
	mask.Set(TierQuality, 0)
	assert.Equal(t, 14, Score(mask, rules)) // 10 + 3 + 1
}

func TestScore_EmptyMask(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Tier: TierSecurity, Bit: 0, Severity: SevCritical},
	}
	var mask Bitmask
	assert.Equal(t, 0, Score(mask, rules))
}

func TestCompose_BasicAttribution(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Tier: TierSecurity, Bit: 0, Severity: SevCritical},
		{ID: "r2", Tier: TierQuality, Bit: 0, Severity: SevWarning},
	}
	symbols := []SymbolSpan{
		{Name: "funcA", StartLine: 1, EndLine: 10},
		{Name: "funcB", StartLine: 12, EndLine: 20},
	}
	findings := []RuleFinding{
		{RuleID: "r1", Line: 5, Severity: SevCritical},
		{RuleID: "r2", Line: 15, Severity: SevWarning},
	}

	methods := Compose(findings, symbols, rules)
	assert.Len(t, methods, 2)
	assert.Equal(t, "funcA", methods[0].Name)
	assert.Equal(t, "funcB", methods[1].Name)
	assert.True(t, methods[0].Bitmask.Has(TierSecurity, 0))
	assert.False(t, methods[0].Bitmask.Has(TierQuality, 0))
	assert.True(t, methods[1].Bitmask.Has(TierQuality, 0))
}

func TestCompose_FileLevelFinding(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Tier: TierSecurity, Bit: 0, Severity: SevCritical},
	}
	// No symbols — finding falls to <file>
	findings := []RuleFinding{
		{RuleID: "r1", Line: 42, Severity: SevCritical},
	}

	methods := Compose(findings, nil, rules)
	assert.Len(t, methods, 1)
	assert.Equal(t, "<file>", methods[0].Name)
	assert.Equal(t, 10, methods[0].Score)
}

func TestCompose_DedupSameRuleSameLine(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Tier: TierSecurity, Bit: 0, Severity: SevCritical},
	}
	symbols := []SymbolSpan{
		{Name: "funcA", StartLine: 1, EndLine: 10},
	}
	// Same rule on same line — should deduplicate
	findings := []RuleFinding{
		{RuleID: "r1", Line: 5, Severity: SevCritical},
		{RuleID: "r1", Line: 5, Severity: SevCritical},
	}

	methods := Compose(findings, symbols, rules)
	assert.Len(t, methods, 1)
	assert.Len(t, methods[0].Findings, 1)
}

func TestCompose_MultipleMethodsScored(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Tier: TierSecurity, Bit: 0, Severity: SevCritical},
		{ID: "r2", Tier: TierSecurity, Bit: 1, Severity: SevHigh},
	}
	symbols := []SymbolSpan{
		{Name: "handler", StartLine: 1, EndLine: 20},
	}
	findings := []RuleFinding{
		{RuleID: "r1", Line: 5, Severity: SevCritical},
		{RuleID: "r2", Line: 10, Severity: SevHigh},
	}

	methods := Compose(findings, symbols, rules)
	assert.Len(t, methods, 1)
	assert.Equal(t, 17, methods[0].Score) // critical(10) + high(7)
	assert.Equal(t, 2, methods[0].Bitmask.PopCount())
}

func TestCompose_EmptyFindings(t *testing.T) {
	rules := []Rule{
		{ID: "r1", Tier: TierSecurity, Bit: 0, Severity: SevCritical},
	}
	methods := Compose(nil, nil, rules)
	assert.Nil(t, methods)
}
