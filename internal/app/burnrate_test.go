package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBurnRateTracker_Empty(t *testing.T) {
	b := NewBurnRateTracker(5 * time.Minute)
	assert.Equal(t, float64(0), b.TokensPerMin())
	assert.Equal(t, int64(0), b.TotalTokens())
}

func TestBurnRateTracker_SingleSample(t *testing.T) {
	b := NewBurnRateTracker(5 * time.Minute)
	b.RecordAt(time.Now(), 1000)
	// Single sample → 0 rate (need at least 2 points for a span)
	assert.Equal(t, float64(0), b.TokensPerMin())
	assert.Equal(t, int64(1000), b.TotalTokens())
}

func TestBurnRateTracker_MultiSample(t *testing.T) {
	b := NewBurnRateTracker(5 * time.Minute)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	b.RecordAt(base, 500)
	b.RecordAt(base.Add(1*time.Minute), 500)
	b.RecordAt(base.Add(2*time.Minute), 500)

	// Rate at 2 minutes: 1500 tokens / 2 minutes = 750 tokens/min
	rate := b.TokensPerMinAt(base.Add(2 * time.Minute))
	assert.InDelta(t, 750.0, rate, 0.01)
	assert.Equal(t, int64(1500), b.TotalTokens())
}

func TestBurnRateTracker_Eviction(t *testing.T) {
	b := NewBurnRateTracker(5 * time.Minute)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	b.RecordAt(base, 1000)
	b.RecordAt(base.Add(1*time.Minute), 1000)

	// Move clock beyond window — samples should be evicted
	now := base.Add(10 * time.Minute)
	rate := b.TokensPerMinAt(now)
	assert.Equal(t, float64(0), rate, "all samples evicted → 0 rate")

	// Total is lifetime — not affected by eviction
	assert.Equal(t, int64(2000), b.TotalTokens())
}

func TestBurnRateTracker_Reset(t *testing.T) {
	b := NewBurnRateTracker(5 * time.Minute)
	b.RecordAt(time.Now(), 500)

	b.Reset()
	assert.Equal(t, float64(0), b.TokensPerMin())
	assert.Equal(t, int64(0), b.TotalTokens())
}

func TestBurnRateTracker_PartialEviction(t *testing.T) {
	b := NewBurnRateTracker(5 * time.Minute)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	b.RecordAt(base, 100)                      // will be evicted
	b.RecordAt(base.Add(3*time.Minute), 200)   // kept
	b.RecordAt(base.Add(4*time.Minute), 300)   // kept

	// At 6 minutes: base+0 is outside 5-min window, others inside
	now := base.Add(6 * time.Minute)
	rate := b.TokensPerMinAt(now)
	// 500 tokens over 3 minutes (from 3:00 to 6:00) = 166.67
	assert.InDelta(t, 166.67, rate, 0.01)
}
