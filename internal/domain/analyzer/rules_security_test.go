package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllRules_Count(t *testing.T) {
	rules := AllRules()
	assert.GreaterOrEqual(t, len(rules), 30, "expected at least 30 rules")
}

func TestAllRules_UniqueIDs(t *testing.T) {
	rules := AllRules()
	seen := make(map[string]bool)
	for _, r := range rules {
		require.False(t, seen[r.ID], "duplicate rule ID: %s", r.ID)
		seen[r.ID] = true
	}
}

func TestAllRules_UniqueBits(t *testing.T) {
	rules := AllRules()
	type tierBitKey struct {
		tier Tier
		bit  int
	}
	seen := make(map[tierBitKey]string)
	for _, r := range rules {
		key := tierBitKey{r.Tier, r.Bit}
		if prev, exists := seen[key]; exists {
			t.Errorf("bit collision: rule %q and %q both use tier=%d bit=%d",
				r.ID, prev, r.Tier, r.Bit)
		}
		seen[key] = r.ID
	}
}

func TestAllRules_BitsInRange(t *testing.T) {
	rules := AllRules()
	for _, r := range rules {
		assert.GreaterOrEqual(t, r.Bit, 0, "rule %s has negative bit", r.ID)
		assert.LessOrEqual(t, r.Bit, 63, "rule %s has bit > 63", r.ID)
	}
}

func TestAllRules_FieldsPopulated(t *testing.T) {
	rules := AllRules()
	for _, r := range rules {
		assert.NotEmpty(t, r.ID, "rule has empty ID")
		assert.NotEmpty(t, r.Label, "rule %s has empty label", r.ID)
		assert.NotEmpty(t, r.Dimension, "rule %s has empty dimension", r.ID)

		switch r.Kind {
		case RuleText:
			assert.NotEmpty(t, r.TextPatterns, "text rule %s has no patterns", r.ID)
		case RuleStructural:
			assert.NotEmpty(t, r.StructuralCheck, "structural rule %s has no check", r.ID)
		case RuleComposite:
			assert.NotEmpty(t, r.TextPatterns, "composite rule %s has no patterns", r.ID)
			assert.NotEmpty(t, r.StructuralCheck, "composite rule %s has no check", r.ID)
		}
	}
}

func TestTextRules_AllHavePatterns(t *testing.T) {
	rules := TextRules()
	for _, r := range rules {
		assert.NotEmpty(t, r.TextPatterns, "text rule %s has no patterns", r.ID)
	}
	assert.Greater(t, len(rules), 15, "expected at least 15 text rules")
}

func TestStructuralRules_AllHaveChecks(t *testing.T) {
	rules := StructuralRules()
	for _, r := range rules {
		assert.NotEmpty(t, r.StructuralCheck, "structural rule %s has no check", r.ID)
	}
	assert.Greater(t, len(rules), 8, "expected at least 8 structural rules")
}
