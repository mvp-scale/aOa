package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentMeter_Accumulation(t *testing.T) {
	var m ContentMeter

	m.RecordUser(100)
	m.RecordAssistant(200)
	m.RecordThinking(50)
	m.RecordToolResult(300)
	m.RecordToolPersisted(150)
	m.RecordSubagent(400)

	assert.Equal(t, int64(100), m.UserChars)
	assert.Equal(t, int64(200), m.AssistantChars)
	assert.Equal(t, int64(50), m.ThinkingChars)
	assert.Equal(t, int64(300), m.ToolResultChars)
	assert.Equal(t, 1, m.ToolResultCount)
	assert.Equal(t, int64(150), m.ToolPersistedChars)
	assert.Equal(t, int64(400), m.SubagentChars)
	assert.Equal(t, 1, m.SubagentCount)

	assert.Equal(t, int64(350), m.ConversationChars()) // 100+200+50
	assert.Equal(t, int64(1200), m.TotalChars())       // 350+300+150+400
}

func TestContentMeter_API(t *testing.T) {
	var m ContentMeter

	m.RecordAPI(1000, 500, 200)
	m.RecordAPI(800, 300, 100)

	assert.Equal(t, 1800, m.APIInputTokens)
	assert.Equal(t, 800, m.APIOutputTokens)
	assert.Equal(t, 300, m.APICacheReadTokens)
}

func TestContentMeter_ActiveMs(t *testing.T) {
	var m ContentMeter

	m.RecordActiveMs(1500)
	m.RecordActiveMs(2500)
	assert.Equal(t, int64(4000), m.ActiveMs)
}

func TestContentMeter_BurstTokensPerSec(t *testing.T) {
	var m ContentMeter

	// No active time → 0
	assert.Equal(t, 0.0, m.BurstTokensPerSec())

	// 4000 chars, 2000ms → 4000/4 = 1000 tokens, 2s → 500 tok/s
	m.RecordAssistant(4000)
	m.RecordActiveMs(2000)
	assert.InDelta(t, 500.0, m.BurstTokensPerSec(), 0.01)
}

func TestContentMeter_TurnRing(t *testing.T) {
	var m ContentMeter

	// Push 3 turns
	for i := 0; i < 3; i++ {
		m.PushTurn(TurnSnapshot{
			Timestamp:      int64(i),
			DurationMs:     1000,
			AssistantChars: 400,
		})
	}

	assert.Equal(t, 3, m.TurnFill)
	assert.Equal(t, 3, m.TurnCount)

	vels := m.TurnVelocities()
	assert.Len(t, vels, 3)
	// 400 chars / 4 = 100 tokens, 1s → 100 tok/s
	for _, v := range vels {
		assert.InDelta(t, 100.0, v, 0.01)
	}
}

func TestContentMeter_TurnRingWrap(t *testing.T) {
	var m ContentMeter

	// Push 55 turns — ring wraps at 50
	for i := 0; i < 55; i++ {
		m.PushTurn(TurnSnapshot{
			Timestamp:      int64(i),
			DurationMs:     1000,
			AssistantChars: 400 * (i + 1),
		})
	}

	assert.Equal(t, 50, m.TurnFill) // capped at 50
	assert.Equal(t, 55, m.TurnCount)

	vels := m.TurnVelocities()
	assert.Len(t, vels, 50)

	// Oldest surviving turn should be turn index 5 (0-indexed)
	// Turn 5: 400*6=2400 chars → 2400/4/1 = 600 tok/s
	assert.InDelta(t, 600.0, vels[0], 0.01)

	// Newest turn: turn 54: 400*55=22000 chars → 22000/4/1 = 5500 tok/s
	assert.InDelta(t, 5500.0, vels[49], 0.01)
}

func TestContentMeter_Reset(t *testing.T) {
	var m ContentMeter

	m.RecordUser(100)
	m.RecordAssistant(200)
	m.RecordToolResult(300)
	m.RecordAPI(1000, 500, 200)
	m.RecordActiveMs(5000)
	m.PushTurn(TurnSnapshot{Timestamp: 1, DurationMs: 1000})

	m.Reset()

	assert.Equal(t, int64(0), m.UserChars)
	assert.Equal(t, int64(0), m.AssistantChars)
	assert.Equal(t, int64(0), m.ToolResultChars)
	assert.Equal(t, 0, m.APIInputTokens)
	assert.Equal(t, int64(0), m.ActiveMs)
	assert.Equal(t, 0, m.TurnFill)
	assert.Equal(t, 0, m.TurnCount)
	assert.Equal(t, int64(0), m.TotalChars())
	assert.Equal(t, 0.0, m.BurstTokensPerSec())
	assert.Nil(t, m.TurnVelocities())
}

func TestContentMeter_TurnVelocities_SkipsZeroDuration(t *testing.T) {
	var m ContentMeter

	m.PushTurn(TurnSnapshot{Timestamp: 1, DurationMs: 0, AssistantChars: 400})
	m.PushTurn(TurnSnapshot{Timestamp: 2, DurationMs: 1000, AssistantChars: 400})

	vels := m.TurnVelocities()
	// Only the turn with DurationMs > 0 should appear
	assert.Len(t, vels, 1)
	assert.InDelta(t, 100.0, vels[0], 0.01)
}
