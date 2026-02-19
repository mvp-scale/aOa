package app

import (
	"sort"
	"time"
)

// RateTracker collects (durationMs, outputTokens) samples from completed turns
// and computes the P50 ms/token rate over a rolling window.
// Not thread-safe — caller (App.mu) must serialize access.
type RateTracker struct {
	window  time.Duration
	samples []rateSample
}

type rateSample struct {
	ts       time.Time
	msPerTok float64
}

// NewRateTracker creates a tracker with the given rolling window duration.
func NewRateTracker(window time.Duration) *RateTracker {
	return &RateTracker{window: window}
}

// Record adds a rate sample at the current time.
// Filters out noisy turns (too few tokens, too short, outlier rates).
func (r *RateTracker) Record(durationMs, outputTokens int) {
	r.RecordAt(time.Now(), durationMs, outputTokens)
}

// RecordAt adds a rate sample at a specific timestamp.
// Filters out noisy turns:
//   - outputTokens < 10 → skip
//   - durationMs < 100 → skip
//   - msPerTok > 50 → skip (tool-heavy outlier)
//   - msPerTok < 0.5 → skip (cached/unrealistic)
func (r *RateTracker) RecordAt(ts time.Time, durationMs, outputTokens int) {
	if outputTokens < 10 || durationMs < 100 {
		return
	}
	msPerTok := float64(durationMs) / float64(outputTokens)
	if msPerTok > 50 || msPerTok < 0.5 {
		return
	}
	r.samples = append(r.samples, rateSample{ts: ts, msPerTok: msPerTok})
	r.evict(ts)
}

// MsPerToken returns the P50 (median) ms/token rate from valid samples
// within the window. Returns 0 if fewer than 5 samples are available.
func (r *RateTracker) MsPerToken() float64 {
	r.evict(time.Now())
	if len(r.samples) < 5 {
		return 0
	}
	// Copy rates for sorting (don't mutate sample order)
	rates := make([]float64, len(r.samples))
	for i, s := range r.samples {
		rates[i] = s.msPerTok
	}
	sort.Float64s(rates)
	return rates[len(rates)/2]
}

// HasData returns true if there are enough valid samples to compute a rate.
func (r *RateTracker) HasData() bool {
	r.evict(time.Now())
	return len(r.samples) >= 5
}

// Reset clears all samples.
func (r *RateTracker) Reset() {
	r.samples = nil
}

// evict removes samples older than the window.
func (r *RateTracker) evict(now time.Time) {
	cutoff := now.Add(-r.window)
	i := 0
	for i < len(r.samples) && r.samples[i].ts.Before(cutoff) {
		i++
	}
	if i > 0 {
		r.samples = r.samples[i:]
	}
}
