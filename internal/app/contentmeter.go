package app

// ContentMeter accumulates raw character counts and timing data from all
// content streams in a Claude Code session. Raw chars + timestamps only;
// token conversions happen at display time (chars/4).
//
// Thread safety: ContentMeter itself is NOT thread-safe. The caller (App)
// must hold a.mu before calling any method.
type ContentMeter struct {
	// Conversation streams (visible dialogue)
	UserChars      int64
	AssistantChars int64
	ThinkingChars  int64

	// Tool streams (infrastructure)
	ToolResultChars    int64
	ToolResultCount    int
	ToolPersistedChars int64 // L9.3: chars resolved from tool-results/ files

	// Agent streams (subagent forks)
	SubagentChars int64 // L9.4: chars from subagent JSONL
	SubagentCount int

	// API-reported tokens (real tokenizer, not char-based estimate)
	APIOutputTokens    int
	APIInputTokens     int
	APICacheReadTokens int

	// Timing
	ActiveMs  int64 // sum of turn_duration events (excludes idle)
	TurnCount int

	// Per-turn ring buffer (last 50 turns)
	Turns    [50]TurnSnapshot
	TurnHead int
	TurnFill int
}

// TurnSnapshot captures per-turn metrics for velocity and burst analysis.
type TurnSnapshot struct {
	Timestamp       int64
	DurationMs      int
	UserChars       int
	AssistantChars  int
	ThinkingChars   int
	ToolResultChars int
	APIOutputTokens int
	ActionCount     int
}

// RecordUser adds user input characters.
func (m *ContentMeter) RecordUser(n int) {
	m.UserChars += int64(n)
}

// RecordAssistant adds assistant response characters.
func (m *ContentMeter) RecordAssistant(n int) {
	m.AssistantChars += int64(n)
}

// RecordThinking adds thinking/reasoning characters.
func (m *ContentMeter) RecordThinking(n int) {
	m.ThinkingChars += int64(n)
}

// RecordToolResult adds tool result characters.
func (m *ContentMeter) RecordToolResult(n int) {
	m.ToolResultChars += int64(n)
	m.ToolResultCount++
}

// RecordToolPersisted adds persisted tool result characters (from disk files).
func (m *ContentMeter) RecordToolPersisted(n int) {
	m.ToolPersistedChars += int64(n)
}

// RecordSubagent adds subagent characters.
func (m *ContentMeter) RecordSubagent(n int) {
	m.SubagentChars += int64(n)
	m.SubagentCount++
}

// RecordAPI adds API-reported token counts.
func (m *ContentMeter) RecordAPI(input, output, cacheRead int) {
	m.APIInputTokens += input
	m.APIOutputTokens += output
	m.APICacheReadTokens += cacheRead
}

// RecordActiveMs adds active (non-idle) milliseconds from a turn duration.
func (m *ContentMeter) RecordActiveMs(ms int) {
	m.ActiveMs += int64(ms)
}

// PushTurn records a TurnSnapshot into the ring buffer.
func (m *ContentMeter) PushTurn(snap TurnSnapshot) {
	m.Turns[m.TurnHead] = snap
	m.TurnHead = (m.TurnHead + 1) % 50
	if m.TurnFill < 50 {
		m.TurnFill++
	}
	m.TurnCount++
}

// ConversationChars returns total chars from visible dialogue streams.
func (m *ContentMeter) ConversationChars() int64 {
	return m.UserChars + m.AssistantChars + m.ThinkingChars
}

// TotalChars returns total chars from all streams including tools and subagents.
func (m *ContentMeter) TotalChars() int64 {
	return m.ConversationChars() + m.ToolResultChars + m.ToolPersistedChars + m.SubagentChars
}

// BurstTokensPerSec returns the undiluted active-work token generation rate.
// Uses chars/4 as token estimate. Returns 0 if no active time recorded.
func (m *ContentMeter) BurstTokensPerSec() float64 {
	if m.ActiveMs <= 0 {
		return 0
	}
	tokens := float64(m.TotalChars()) / 4.0
	seconds := float64(m.ActiveMs) / 1000.0
	return tokens / seconds
}

// TurnVelocities returns per-turn token/sec values from the ring buffer.
// Ordered oldest to newest.
func (m *ContentMeter) TurnVelocities() []float64 {
	if m.TurnFill == 0 {
		return nil
	}
	vels := make([]float64, 0, m.TurnFill)
	for i := 0; i < m.TurnFill; i++ {
		idx := (m.TurnHead - m.TurnFill + i + 50) % 50
		snap := m.Turns[idx]
		if snap.DurationMs > 0 {
			chars := snap.AssistantChars + snap.ThinkingChars + snap.ToolResultChars
			tokens := float64(chars) / 4.0
			seconds := float64(snap.DurationMs) / 1000.0
			vels = append(vels, tokens/seconds)
		}
	}
	return vels
}

// Reset clears all counters and the ring buffer.
func (m *ContentMeter) Reset() {
	*m = ContentMeter{}
}
