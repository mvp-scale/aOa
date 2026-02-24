package analyzer

import (
	"testing"

	"github.com/corey/aoa/internal/adapters/reconrules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRulesFromFS(t *testing.T) {
	rules, err := LoadRulesFromFS(reconrules.FS, "rules")
	require.NoError(t, err)
	require.NotEmpty(t, rules)

	t.Logf("loaded %d rules from YAML", len(rules))

	// Verify we got rules from multiple tiers
	tierCounts := make(map[Tier]int)
	for _, r := range rules {
		tierCounts[r.Tier]++
	}
	assert.Greater(t, tierCounts[TierSecurity], 0, "should have security rules")
	assert.Greater(t, tierCounts[TierPerformance], 0, "should have performance rules")
	assert.Greater(t, tierCounts[TierQuality], 0, "should have quality rules")
	assert.Greater(t, tierCounts[TierArchitecture], 0, "should have architecture rules")
	assert.Greater(t, tierCounts[TierObservability], 0, "should have observability rules")
	assert.Greater(t, tierCounts[TierCompliance], 0, "should have compliance rules")

	for tier, count := range tierCounts {
		t.Logf("  tier %d (%s): %d rules", tier, TierName(tier), count)
	}
}

func TestLoadRulesFromFS_UniqueIDs(t *testing.T) {
	rules, err := LoadRulesFromFS(reconrules.FS, "rules")
	require.NoError(t, err)

	seen := make(map[string]bool)
	for _, r := range rules {
		assert.False(t, seen[r.ID], "duplicate rule ID: %s", r.ID)
		seen[r.ID] = true
	}
}

func TestLoadRulesFromFS_UniqueTierBit(t *testing.T) {
	rules, err := LoadRulesFromFS(reconrules.FS, "rules")
	require.NoError(t, err)

	type tierBitKey struct {
		tier Tier
		bit  int
	}
	seen := make(map[tierBitKey]string)
	for _, r := range rules {
		key := tierBitKey{r.Tier, r.Bit}
		assert.Empty(t, seen[key], "duplicate (tier=%d, bit=%d): %s conflicts with %s",
			r.Tier, r.Bit, r.ID, seen[key])
		seen[key] = r.ID
	}
}

func TestLoadRulesFromFS_ValidBitRanges(t *testing.T) {
	rules, err := LoadRulesFromFS(reconrules.FS, "rules")
	require.NoError(t, err)

	for _, r := range rules {
		assert.GreaterOrEqual(t, r.Bit, 0, "rule %s bit must be >= 0", r.ID)
		assert.LessOrEqual(t, r.Bit, 63, "rule %s bit must be <= 63", r.ID)
	}
}

func TestLoadRulesFromFS_TextRulesHavePatterns(t *testing.T) {
	rules, err := LoadRulesFromFS(reconrules.FS, "rules")
	require.NoError(t, err)

	for _, r := range rules {
		if r.Kind == RuleText {
			assert.NotEmpty(t, r.TextPatterns, "text rule %s must have patterns", r.ID)
		}
		if r.Kind == RuleStructural {
			assert.NotEmpty(t, r.StructuralCheck, "structural rule %s must have check name", r.ID)
		}
	}
}

func TestLoadRulesFromFS_AllDimensionsNonEmpty(t *testing.T) {
	rules, err := LoadRulesFromFS(reconrules.FS, "rules")
	require.NoError(t, err)

	dimCounts := make(map[string]int)
	for _, r := range rules {
		dimCounts[r.Dimension]++
	}

	t.Logf("dimensions: %d unique", len(dimCounts))
	for dim, count := range dimCounts {
		t.Logf("  %s: %d rules", dim, count)
		assert.Greater(t, count, 0)
	}
}

func TestTierFromName(t *testing.T) {
	assert.Equal(t, TierSecurity, TierFromName("security"))
	assert.Equal(t, TierPerformance, TierFromName("performance"))
	assert.Equal(t, TierQuality, TierFromName("quality"))
	assert.Equal(t, TierObservability, TierFromName("observability"))
	assert.Equal(t, TierArchitecture, TierFromName("architecture"))
	assert.Equal(t, TierCompliance, TierFromName("compliance"))
	assert.Equal(t, Tier(-1), TierFromName("unknown"))
}

func TestSeverityFromName(t *testing.T) {
	assert.Equal(t, SevInfo, SeverityFromName("info"))
	assert.Equal(t, SevWarning, SeverityFromName("warning"))
	assert.Equal(t, SevHigh, SeverityFromName("high"))
	assert.Equal(t, SevCritical, SeverityFromName("critical"))
	assert.Equal(t, Severity(-1), SeverityFromName("unknown"))
}
