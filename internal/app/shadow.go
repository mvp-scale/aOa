package app

import "time"

// ToolShadow records one counterfactual comparison between a native tool
// result and what aOa would have returned.
type ToolShadow struct {
	Timestamp   time.Time
	Source      string // "session_log" or "shim"
	ToolName    string
	ToolID      string
	Pattern     string
	FilePath    string
	ActualChars int   // chars from Claude's tool result
	ShadowChars int   // chars from aOa search
	ShadowRan   bool  // whether the shadow search completed
	ShadowMs    int64 // shadow search duration in ms
	CharsSaved  int   // ActualChars - ShadowChars (positive = aOa saved context)
}

// ShadowRing is a fixed-size ring buffer of ToolShadow entries.
// Not thread-safe — caller must hold a.mu.
type ShadowRing struct {
	entries         [100]ToolShadow
	head            int
	count           int
	totalCharsSaved int64
}

// Push adds a shadow entry to the ring.
func (r *ShadowRing) Push(s ToolShadow) {
	r.entries[r.head] = s
	r.head = (r.head + 1) % 100
	if r.count < 100 {
		r.count++
	}
	if s.CharsSaved > 0 {
		r.totalCharsSaved += int64(s.CharsSaved)
	}
}

// Entries returns all entries in the ring, oldest first.
func (r *ShadowRing) Entries() []ToolShadow {
	if r.count == 0 {
		return nil
	}
	result := make([]ToolShadow, r.count)
	for i := 0; i < r.count; i++ {
		idx := (r.head - r.count + i + 100) % 100
		result[i] = r.entries[idx]
	}
	return result
}

// TotalCharsSaved returns the cumulative chars saved across all shadow entries.
func (r *ShadowRing) TotalCharsSaved() int64 {
	return r.totalCharsSaved
}

// Count returns the number of entries in the ring.
func (r *ShadowRing) Count() int {
	return r.count
}

// FindByToolID searches the ring for an entry matching the given ToolID.
// Returns nil if not found.
func (r *ShadowRing) FindByToolID(toolID string) *ToolShadow {
	if toolID == "" || r.count == 0 {
		return nil
	}
	// Search from newest to oldest (most likely match is recent)
	for i := 0; i < r.count; i++ {
		idx := (r.head - 1 - i + 100) % 100
		if r.entries[idx].ToolID == toolID {
			entry := r.entries[idx]
			return &entry
		}
	}
	return nil
}

// Reset clears the ring buffer and counters.
func (r *ShadowRing) Reset() {
	*r = ShadowRing{}
}
