package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitmask_SetAndHas(t *testing.T) {
	var b Bitmask
	b.Set(TierSecurity, 0)
	b.Set(TierSecurity, 5)
	b.Set(TierQuality, 63)

	assert.True(t, b.Has(TierSecurity, 0))
	assert.True(t, b.Has(TierSecurity, 5))
	assert.True(t, b.Has(TierQuality, 63))

	assert.False(t, b.Has(TierSecurity, 1))
	assert.False(t, b.Has(TierPerformance, 0))
	assert.False(t, b.Has(TierObservability, 0))
}

func TestBitmask_TierIsolation(t *testing.T) {
	var b Bitmask
	b.Set(TierSecurity, 3)
	// Same bit position in a different tier must not be set
	assert.False(t, b.Has(TierPerformance, 3))
	assert.False(t, b.Has(TierQuality, 3))
	assert.False(t, b.Has(TierObservability, 3))
}

func TestBitmask_OverflowGuard(t *testing.T) {
	var b Bitmask
	// Out of range bits are silently ignored
	b.Set(TierSecurity, 64)
	b.Set(TierSecurity, -1)
	b.Set(Tier(5), 0)  // invalid tier
	b.Set(Tier(-1), 0) // invalid tier
	assert.True(t, b.IsZero())
}

func TestBitmask_HasOverflowGuard(t *testing.T) {
	var b Bitmask
	assert.False(t, b.Has(TierSecurity, 64))
	assert.False(t, b.Has(TierSecurity, -1))
	assert.False(t, b.Has(Tier(5), 0))
}

func TestBitmask_Or(t *testing.T) {
	var a, b Bitmask
	a.Set(TierSecurity, 0)
	a.Set(TierQuality, 10)
	b.Set(TierSecurity, 1)
	b.Set(TierPerformance, 5)

	a.Or(b)
	assert.True(t, a.Has(TierSecurity, 0))
	assert.True(t, a.Has(TierSecurity, 1))
	assert.True(t, a.Has(TierQuality, 10))
	assert.True(t, a.Has(TierPerformance, 5))
}

func TestBitmask_PopCount(t *testing.T) {
	var b Bitmask
	assert.Equal(t, 0, b.PopCount())

	b.Set(TierSecurity, 0)
	b.Set(TierSecurity, 5)
	b.Set(TierQuality, 63)
	assert.Equal(t, 3, b.PopCount())
}

func TestBitmask_IsZero(t *testing.T) {
	var b Bitmask
	assert.True(t, b.IsZero())
	b.Set(TierSecurity, 0)
	assert.False(t, b.IsZero())
}

func TestSeverity_String(t *testing.T) {
	assert.Equal(t, "info", SevInfo.String())
	assert.Equal(t, "warning", SevWarning.String())
	assert.Equal(t, "high", SevHigh.String())
	assert.Equal(t, "critical", SevCritical.String())
	assert.Equal(t, "unknown", Severity(99).String())
}
