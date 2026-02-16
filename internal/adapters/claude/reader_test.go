package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/corey/aoa/internal/adapters/tailer"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Translation: user events
// =============================================================================

func TestTranslate_UserInput(t *testing.T) {
	r := &Reader{health: freshHealth()}
	raw := &tailer.SessionEvent{
		Type:      "user",
		UUID:      "u-001",
		SessionID: "session-1",
		Timestamp: time.Date(2026, 2, 16, 10, 0, 0, 0, time.UTC),
		Version:   "2.1.29",
		UserText:  "fix the auth bug",
	}

	events := r.translate(raw)
	require.Len(t, events, 1)

	ev := events[0]
	assert.Equal(t, ports.EventUserInput, ev.Kind)
	assert.Equal(t, "u-001", ev.ID)
	assert.Equal(t, "u-001", ev.TurnID)
	assert.Equal(t, "session-1", ev.SessionID)
	assert.Equal(t, "fix the auth bug", ev.Text)
	assert.Equal(t, "2.1.29", ev.AgentVersion)
	assert.Nil(t, ev.Tool)
	assert.Nil(t, ev.File)
	assert.Nil(t, ev.Usage)
}

func TestTranslate_UserInput_Empty_RecordsGap(t *testing.T) {
	r := &Reader{health: freshHealth()}
	raw := &tailer.SessionEvent{
		Type: "user",
		UUID: "u-002",
	}

	events := r.translate(raw)
	assert.Empty(t, events)
	assert.Equal(t, 1, r.health.Gaps)
}

// =============================================================================
// Translation: assistant events
// =============================================================================

func TestTranslate_AssistantTextOnly(t *testing.T) {
	r := &Reader{health: freshHealth()}
	raw := &tailer.SessionEvent{
		Type:          "assistant",
		UUID:          "a-001",
		SessionID:     "session-1",
		Model:         "claude-opus-4-6",
		AssistantText: "Here is the fix.",
	}

	events := r.translate(raw)
	require.Len(t, events, 1)
	assert.Equal(t, ports.EventAIResponse, events[0].Kind)
	assert.Equal(t, "Here is the fix.", events[0].Text)
	assert.Equal(t, "a-001:response", events[0].ID)
	assert.Equal(t, "a-001", events[0].TurnID)
	assert.Equal(t, "claude-opus-4-6", events[0].Model)
}

func TestTranslate_AssistantThinkingAndText(t *testing.T) {
	r := &Reader{health: freshHealth()}
	raw := &tailer.SessionEvent{
		Type:          "assistant",
		UUID:          "a-002",
		ThinkingText:  "The user wants to fix auth.",
		AssistantText: "Let me look at the handler.",
	}

	events := r.translate(raw)
	require.Len(t, events, 2)

	assert.Equal(t, ports.EventAIThinking, events[0].Kind)
	assert.Equal(t, "The user wants to fix auth.", events[0].Text)
	assert.Equal(t, "a-002:thinking", events[0].ID)
	assert.Equal(t, "a-002", events[0].TurnID)

	assert.Equal(t, ports.EventAIResponse, events[1].Kind)
	assert.Equal(t, "Let me look at the handler.", events[1].Text)
	assert.Equal(t, "a-002:response", events[1].ID)
	assert.Equal(t, "a-002", events[1].TurnID)
}

func TestTranslate_AssistantWithTools(t *testing.T) {
	r := &Reader{health: freshHealth()}
	raw := &tailer.SessionEvent{
		Type:          "assistant",
		UUID:          "a-003",
		AssistantText: "Let me read the file.",
		ToolUses: []tailer.ToolUse{
			{
				Name:     "Read",
				ID:       "toolu_001",
				FilePath: "/src/auth.py",
				Offset:   10,
				Limit:    50,
			},
			{
				Name:    "Bash",
				ID:      "toolu_002",
				Command: "aoa grep auth",
			},
		},
	}

	events := r.translate(raw)
	require.Len(t, events, 3) // response + 2 tools

	// AIResponse
	assert.Equal(t, ports.EventAIResponse, events[0].Kind)
	assert.Equal(t, "Let me read the file.", events[0].Text)

	// Tool 1: Read with file
	assert.Equal(t, ports.EventToolInvocation, events[1].Kind)
	assert.Equal(t, "a-003:tool:0", events[1].ID)
	assert.Equal(t, "a-003", events[1].TurnID)
	require.NotNil(t, events[1].Tool)
	assert.Equal(t, "Read", events[1].Tool.Name)
	assert.Equal(t, "toolu_001", events[1].Tool.ToolID)
	require.NotNil(t, events[1].File)
	assert.Equal(t, "/src/auth.py", events[1].File.Path)
	assert.Equal(t, 10, events[1].File.Offset)
	assert.Equal(t, 50, events[1].File.Limit)
	assert.Equal(t, "read", events[1].File.Action)

	// Tool 2: Bash without file
	assert.Equal(t, ports.EventToolInvocation, events[2].Kind)
	assert.Equal(t, "a-003:tool:1", events[2].ID)
	require.NotNil(t, events[2].Tool)
	assert.Equal(t, "Bash", events[2].Tool.Name)
	assert.Equal(t, "aoa grep auth", events[2].Tool.Command)
	assert.Nil(t, events[2].File)
}

func TestTranslate_AssistantFull_ThinkingTextToolsUsage(t *testing.T) {
	r := &Reader{health: freshHealth()}
	raw := &tailer.SessionEvent{
		Type:          "assistant",
		UUID:          "a-004",
		Model:         "claude-opus-4-6",
		ThinkingText:  "Analyzing the problem.",
		AssistantText: "Found the issue.",
		ToolUses: []tailer.ToolUse{
			{Name: "Read", ID: "t1", FilePath: "/src/main.go", Limit: 100},
		},
		Usage: &tailer.MessageUsage{
			InputTokens:      500,
			OutputTokens:     200,
			CacheReadTokens:  8000,
			CacheWriteTokens: 100,
			ServiceTier:      "standard",
		},
	}

	events := r.translate(raw)
	require.Len(t, events, 3) // thinking + response + tool

	// Thinking — no usage
	assert.Equal(t, ports.EventAIThinking, events[0].Kind)
	assert.Nil(t, events[0].Usage)

	// Response — carries usage
	assert.Equal(t, ports.EventAIResponse, events[1].Kind)
	assert.Equal(t, "Found the issue.", events[1].Text)
	require.NotNil(t, events[1].Usage)
	assert.Equal(t, 500, events[1].Usage.InputTokens)
	assert.Equal(t, 200, events[1].Usage.OutputTokens)
	assert.Equal(t, 8000, events[1].Usage.CacheReadTokens)
	assert.Equal(t, 100, events[1].Usage.CacheWriteTokens)
	assert.Equal(t, "standard", events[1].Usage.ServiceTier)

	// Tool
	assert.Equal(t, ports.EventToolInvocation, events[2].Kind)

	// All share TurnID
	for _, ev := range events {
		assert.Equal(t, "a-004", ev.TurnID)
	}
}

func TestTranslate_AssistantUsageOnly_NoText(t *testing.T) {
	// Tool-only turn: no visible response text, but usage data present
	r := &Reader{health: freshHealth()}
	raw := &tailer.SessionEvent{
		Type: "assistant",
		UUID: "a-005",
		ToolUses: []tailer.ToolUse{
			{Name: "Read", ID: "t1", FilePath: "/src/main.go"},
		},
		Usage: &tailer.MessageUsage{
			InputTokens:  300,
			OutputTokens: 50,
		},
	}

	events := r.translate(raw)
	require.Len(t, events, 2) // response (for usage) + tool

	// AIResponse with empty text but carries usage
	assert.Equal(t, ports.EventAIResponse, events[0].Kind)
	assert.Equal(t, "", events[0].Text)
	require.NotNil(t, events[0].Usage)
	assert.Equal(t, 300, events[0].Usage.InputTokens)
}

func TestTranslate_AssistantEmpty_RecordsGap(t *testing.T) {
	r := &Reader{health: freshHealth()}
	raw := &tailer.SessionEvent{
		Type: "assistant",
		UUID: "a-006",
	}

	events := r.translate(raw)
	assert.Empty(t, events)
	assert.Equal(t, 1, r.health.Gaps)
}

// =============================================================================
// Translation: system events
// =============================================================================

func TestTranslate_SystemEvent(t *testing.T) {
	r := &Reader{health: freshHealth()}
	raw := &tailer.SessionEvent{
		Type:       "system",
		UUID:       "sys-001",
		Subtype:    "turn_duration",
		ParentUUID: "a-001",
		DurationMs: 5250,
		SessionID:  "session-1",
	}

	events := r.translate(raw)
	require.Len(t, events, 1)

	ev := events[0]
	assert.Equal(t, ports.EventSystemMeta, ev.Kind)
	assert.Equal(t, "sys-001", ev.ID)
	assert.Equal(t, "a-001", ev.TurnID) // linked to parent assistant turn
	assert.Equal(t, "turn_duration", ev.Text)
	assert.Equal(t, 5250, ev.DurationMs)
}

// =============================================================================
// Translation: unknown event types
// =============================================================================

func TestTranslate_UnknownType_TrackedInHealth(t *testing.T) {
	r := &Reader{health: freshHealth()}

	events := r.translate(&tailer.SessionEvent{Type: "progress"})
	assert.Empty(t, events)
	assert.Equal(t, 1, r.health.UnknownTypes["progress"])

	r.translate(&tailer.SessionEvent{Type: "file-history-snapshot"})
	assert.Equal(t, 1, r.health.UnknownTypes["file-history-snapshot"])

	// Second progress
	r.translate(&tailer.SessionEvent{Type: "progress"})
	assert.Equal(t, 2, r.health.UnknownTypes["progress"])
}

// =============================================================================
// Tool action mapping
// =============================================================================

func TestToolAction(t *testing.T) {
	cases := []struct {
		tool   string
		action string
	}{
		{"Read", "read"},
		{"Write", "write"},
		{"Edit", "edit"},
		{"NotebookEdit", "edit"},
		{"Grep", "search"},
		{"Glob", "search"},
		{"Bash", "access"},
		{"WebFetch", "access"},
		{"CustomTool", "access"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.action, toolAction(tc.tool), "tool: %s", tc.tool)
	}
}

// =============================================================================
// Health tracking
// =============================================================================

func TestHealth_CountsEvents(t *testing.T) {
	r := &Reader{health: freshHealth()}

	// Simulate onRawEvent processing
	r.onRawEvent(&tailer.SessionEvent{
		Type:     "user",
		UUID:     "u-1",
		UserText: "hello",
		Version:  "2.1.29",
	})
	r.onRawEvent(&tailer.SessionEvent{
		Type:          "assistant",
		UUID:          "a-1",
		AssistantText: "hi there",
		Usage:         &tailer.MessageUsage{InputTokens: 100, OutputTokens: 50},
		Version:       "2.1.29",
	})

	h := r.Health()
	assert.Equal(t, 2, h.LinesRead)
	assert.Equal(t, 2, h.LinesParsed)
	assert.Equal(t, 0, h.LinesSkipped)
	assert.Equal(t, 1, h.EventCounts[ports.EventUserInput])
	assert.Equal(t, 1, h.EventCounts[ports.EventAIResponse])
	assert.Equal(t, 2, h.TextYield) // user text + assistant text
	assert.Equal(t, 1, h.UsageYield)
	assert.Equal(t, "2.1.29", h.AgentVersion)
	assert.False(t, h.VersionChanged)
	assert.True(t, h.IsHealthy())
}

func TestHealth_DetectsVersionChange(t *testing.T) {
	r := &Reader{health: freshHealth()}

	r.onRawEvent(&tailer.SessionEvent{
		Type: "user", UUID: "u-1", UserText: "a", Version: "2.1.29",
	})
	r.onRawEvent(&tailer.SessionEvent{
		Type: "user", UUID: "u-2", UserText: "b", Version: "2.1.30",
	})

	h := r.Health()
	assert.True(t, h.VersionChanged)
	assert.Equal(t, "2.1.30", h.AgentVersion)
}

func TestHealth_ResetOnRead(t *testing.T) {
	r := &Reader{health: freshHealth()}

	r.onRawEvent(&tailer.SessionEvent{
		Type: "user", UUID: "u-1", UserText: "a",
	})

	h1 := r.Health()
	assert.Equal(t, 1, h1.LinesRead)

	// After Health(), counters reset
	h2 := r.Health()
	assert.Equal(t, 0, h2.LinesRead)
}

func TestHealth_ErrorsTrackedAsSkipped(t *testing.T) {
	r := &Reader{health: freshHealth()}

	r.onParseError(fmt.Errorf("bad json"))
	r.onParseError(fmt.Errorf("truncated"))

	h := r.Health()
	assert.Equal(t, 2, h.LinesRead)
	assert.Equal(t, 2, h.LinesSkipped)
	assert.Equal(t, 0, h.LinesParsed)
}

func TestHealth_ToolAndFileYield(t *testing.T) {
	r := &Reader{health: freshHealth()}

	r.onRawEvent(&tailer.SessionEvent{
		Type:          "assistant",
		UUID:          "a-1",
		AssistantText: "checking",
		ToolUses: []tailer.ToolUse{
			{Name: "Read", ID: "t1", FilePath: "/src/main.go", Limit: 50},
			{Name: "Bash", ID: "t2", Command: "ls"},
		},
	})

	h := r.Health()
	assert.Equal(t, 2, h.ToolYield) // 2 tool events
	assert.Equal(t, 1, h.FileYield) // only Read has FilePath
}

// =============================================================================
// Integration: full pipeline with tailer
// =============================================================================

func TestIntegration_TailerToCanonical(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session-001.jsonl")
	require.NoError(t, os.WriteFile(sessionFile, []byte(""), 0644))

	var mu sync.Mutex
	var events []ports.SessionEvent

	reader := New(Config{
		SessionDir:   dir,
		PollInterval: 50 * time.Millisecond,
	})
	reader.Start(func(ev ports.SessionEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})
	defer reader.Stop()
	<-reader.Started()

	// Write session events
	f, err := os.OpenFile(sessionFile, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)

	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"fix the bug"}}`,
		`{"type":"assistant","uuid":"a1","message":{"role":"assistant","model":"claude-opus-4-6","content":[{"type":"thinking","thinking":"analyzing"},{"type":"text","text":"found it"},{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/src/main.go","limit":50}}],"usage":{"input_tokens":500,"output_tokens":200,"cache_read_input_tokens":8000}}}`,
		`{"type":"system","uuid":"s1","subtype":"turn_duration","parentUuid":"a1","durationMs":3200}`,
	}
	for _, line := range lines {
		_, err := f.WriteString(line + "\n")
		require.NoError(t, err)
	}
	f.Close()

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// User → 1, Assistant → 3 (thinking + response + tool), System → 1 = 5
	require.Len(t, events, 5)

	// User input
	assert.Equal(t, ports.EventUserInput, events[0].Kind)
	assert.Equal(t, "fix the bug", events[0].Text)

	// Thinking
	assert.Equal(t, ports.EventAIThinking, events[1].Kind)
	assert.Equal(t, "analyzing", events[1].Text)
	assert.Equal(t, "a1", events[1].TurnID)

	// Response with usage
	assert.Equal(t, ports.EventAIResponse, events[2].Kind)
	assert.Equal(t, "found it", events[2].Text)
	assert.Equal(t, "a1", events[2].TurnID)
	require.NotNil(t, events[2].Usage)
	assert.Equal(t, 500, events[2].Usage.InputTokens)
	assert.Equal(t, 8000, events[2].Usage.CacheReadTokens)

	// Tool invocation with file
	assert.Equal(t, ports.EventToolInvocation, events[3].Kind)
	assert.Equal(t, "a1", events[3].TurnID)
	require.NotNil(t, events[3].Tool)
	assert.Equal(t, "Read", events[3].Tool.Name)
	require.NotNil(t, events[3].File)
	assert.Equal(t, "/src/main.go", events[3].File.Path)
	assert.Equal(t, "read", events[3].File.Action)

	// System meta
	assert.Equal(t, ports.EventSystemMeta, events[4].Kind)
	assert.Equal(t, "turn_duration", events[4].Text)
	assert.Equal(t, 3200, events[4].DurationMs)
	assert.Equal(t, "a1", events[4].TurnID)

	// Health check
	h := reader.Health()
	assert.True(t, h.IsHealthy())
	assert.Equal(t, 3, h.LinesRead)
	assert.Equal(t, 3, h.LinesParsed)
}
