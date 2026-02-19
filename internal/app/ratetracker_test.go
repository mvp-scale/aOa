package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateTracker_Empty(t *testing.T) {
	rt := NewRateTracker(30 * time.Minute)
	assert.False(t, rt.HasData(), "empty tracker should not have data")
	assert.Equal(t, 0.0, rt.MsPerToken(), "empty tracker should return 0")
}

func TestRateTracker_InsufficientSamples(t *testing.T) {
	rt := NewRateTracker(30 * time.Minute)
	now := time.Now()

	// Add 3 valid samples — below the 5-sample minimum
	for i := 0; i < 3; i++ {
		rt.RecordAt(now.Add(time.Duration(i)*time.Second), 5000, 500) // 10 ms/tok
	}

	assert.False(t, rt.HasData(), "3 samples should not be enough")
	assert.Equal(t, 0.0, rt.MsPerToken(), "insufficient samples should return 0")
}

func TestRateTracker_SufficientSamples(t *testing.T) {
	rt := NewRateTracker(30 * time.Minute)
	now := time.Now()

	// 5 samples with known ms/tok rates: 5, 8, 10, 12, 15
	data := []struct {
		durationMs   int
		outputTokens int
		msPerTok     float64
	}{
		{5000, 1000, 5.0},
		{4000, 500, 8.0},
		{5000, 500, 10.0},
		{6000, 500, 12.0},
		{7500, 500, 15.0},
	}

	for i, d := range data {
		rt.RecordAt(now.Add(time.Duration(i)*time.Second), d.durationMs, d.outputTokens)
	}

	assert.True(t, rt.HasData(), "5 valid samples should be enough")
	// P50 of [5, 8, 10, 12, 15] → index 2 → 10.0
	assert.Equal(t, 10.0, rt.MsPerToken(), "P50 should be the median value")
}

func TestRateTracker_FilterOutliers(t *testing.T) {
	rt := NewRateTracker(30 * time.Minute)
	now := time.Now()

	// Too few output tokens (< 10) → filtered
	rt.RecordAt(now, 1000, 5)
	assert.False(t, rt.HasData())

	// Too short duration (< 100ms) → filtered
	rt.RecordAt(now.Add(time.Second), 50, 500)
	assert.False(t, rt.HasData())

	// Rate too high (> 50 ms/tok) → filtered
	// 10000ms / 100 tokens = 100 ms/tok
	rt.RecordAt(now.Add(2*time.Second), 10000, 100)
	assert.False(t, rt.HasData())

	// Rate too low (< 0.5 ms/tok) → filtered
	// 100ms / 1000 tokens = 0.1 ms/tok
	rt.RecordAt(now.Add(3*time.Second), 100, 1000)
	assert.False(t, rt.HasData())

	// No valid samples should have been recorded
	assert.Equal(t, 0.0, rt.MsPerToken())

	// Now add 5 valid ones
	for i := 0; i < 5; i++ {
		rt.RecordAt(now.Add(time.Duration(4+i)*time.Second), 5000, 500) // 10 ms/tok
	}
	assert.True(t, rt.HasData(), "valid samples after outliers should count")
	assert.Equal(t, 10.0, rt.MsPerToken())
}

func TestRateTracker_WindowEviction(t *testing.T) {
	window := 30 * time.Minute
	rt := NewRateTracker(window)
	base := time.Now()

	// Add 5 samples at base time
	for i := 0; i < 5; i++ {
		rt.RecordAt(base.Add(time.Duration(i)*time.Second), 5000, 500) // 10 ms/tok
	}
	assert.True(t, rt.HasData())

	// Fast-forward past the window — all samples should be evicted
	future := base.Add(window + time.Minute)
	// Add fewer than 5 samples at the future time
	for i := 0; i < 3; i++ {
		rt.RecordAt(future.Add(time.Duration(i)*time.Second), 3000, 500) // 6 ms/tok
	}

	assert.False(t, rt.HasData(), "old samples should be evicted, leaving only 3")
	assert.Equal(t, 0.0, rt.MsPerToken())
}

func TestRateTracker_Reset(t *testing.T) {
	rt := NewRateTracker(30 * time.Minute)
	now := time.Now()

	for i := 0; i < 5; i++ {
		rt.RecordAt(now.Add(time.Duration(i)*time.Second), 5000, 500)
	}
	assert.True(t, rt.HasData())

	rt.Reset()
	assert.False(t, rt.HasData(), "Reset should clear all data")
	assert.Equal(t, 0.0, rt.MsPerToken(), "Reset should return 0")
}
