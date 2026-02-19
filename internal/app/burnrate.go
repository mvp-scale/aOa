package app

import "time"

// BurnRateTracker computes a rolling token burn rate over a configurable window.
// Not thread-safe — caller (App.mu) must serialize access.
type BurnRateTracker struct {
	window  time.Duration
	samples []burnSample
	total   int64
}

type burnSample struct {
	ts     time.Time
	tokens int
}

// NewBurnRateTracker creates a tracker with the given rolling window duration.
func NewBurnRateTracker(window time.Duration) *BurnRateTracker {
	return &BurnRateTracker{window: window}
}

// Record adds a token sample at the current time.
func (b *BurnRateTracker) Record(tokens int) {
	b.RecordAt(time.Now(), tokens)
}

// RecordAt adds a token sample at a specific timestamp.
func (b *BurnRateTracker) RecordAt(ts time.Time, tokens int) {
	b.samples = append(b.samples, burnSample{ts: ts, tokens: tokens})
	b.total += int64(tokens)
	b.evict(ts)
}

// TokensPerMin returns the current burn rate in tokens per minute.
func (b *BurnRateTracker) TokensPerMin() float64 {
	return b.TokensPerMinAt(time.Now())
}

// TokensPerMinAt computes the burn rate as of the given time.
func (b *BurnRateTracker) TokensPerMinAt(now time.Time) float64 {
	b.evict(now)
	if len(b.samples) < 2 {
		return 0
	}
	oldest := b.samples[0].ts
	span := now.Sub(oldest)
	if span <= 0 {
		return 0
	}

	sum := 0
	for _, s := range b.samples {
		sum += s.tokens
	}
	return float64(sum) / span.Minutes()
}

// TotalTokens returns the lifetime total of recorded tokens.
func (b *BurnRateTracker) TotalTokens() int64 {
	return b.total
}

// Reset clears all samples and the lifetime total.
func (b *BurnRateTracker) Reset() {
	b.samples = nil
	b.total = 0
}

// evict removes samples older than the window.
func (b *BurnRateTracker) evict(now time.Time) {
	cutoff := now.Add(-b.window)
	i := 0
	for i < len(b.samples) && b.samples[i].ts.Before(cutoff) {
		i++
	}
	if i > 0 {
		b.samples = b.samples[i:]
	}
}
