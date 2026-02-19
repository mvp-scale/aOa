package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunwayProjection_ZeroBurnRate(t *testing.T) {
	a := newTestApp(t)
	result := a.RunwayProjection()
	assert.Equal(t, float64(0), result.RunwayMinutes)
	assert.Equal(t, float64(0), result.CounterfactMinutes)
	assert.Equal(t, float64(0), result.DeltaMinutes)
	assert.Equal(t, 200000, result.ContextWindowMax)
}

func TestRunwayProjection_WithBurnRate(t *testing.T) {
	a := newTestApp(t)
	a.currentModel = "claude-opus-4-6"

	// Use recent timestamps so they stay within the 5-min window
	now := time.Now()
	a.burnRate.RecordAt(now.Add(-1*time.Minute), 10000)
	a.burnRate.RecordAt(now, 10000)

	a.burnRateCounterfact.RecordAt(now.Add(-1*time.Minute), 10000)
	a.burnRateCounterfact.RecordAt(now, 10000)

	result := a.RunwayProjection()
	assert.Equal(t, "claude-opus-4-6", result.Model)
	assert.Equal(t, 200000, result.ContextWindowMax)
	assert.Equal(t, int64(20000), result.TokensUsed)
	assert.Greater(t, result.BurnRatePerMin, float64(0))
	assert.Greater(t, result.RunwayMinutes, float64(0))
}

func TestRunwayProjection_CounterfactualDelta(t *testing.T) {
	a := newTestApp(t)
	a.currentModel = "claude-opus-4-6"

	now := time.Now()

	// Actual burn: 10k/min
	a.burnRate.RecordAt(now.Add(-1*time.Minute), 10000)
	a.burnRate.RecordAt(now, 10000)

	// Counterfactual: 15k/min (would have been higher without aOa)
	a.burnRateCounterfact.RecordAt(now.Add(-1*time.Minute), 15000)
	a.burnRateCounterfact.RecordAt(now, 15000)

	a.counterfactTokensSaved = 5000

	result := a.RunwayProjection()
	assert.Equal(t, int64(5000), result.TokensSaved)
	// Actual runway should be longer than counterfactual (lower burn rate)
	assert.Greater(t, result.RunwayMinutes, result.CounterfactMinutes)
	assert.Greater(t, result.DeltaMinutes, float64(0))
}
