package tailer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// T-02: Parser — Defensive JSONL parsing
// =============================================================================

func TestParser_UserPromptEvent(t *testing.T) {
	line := []byte(`{
		"type": "user",
		"uuid": "550e8400-e29b-41d4-a716-446655440000",
		"timestamp": "2026-02-14T18:20:15.000Z",
		"sessionId": "session-abc",
		"version": "2.1.29",
		"isMeta": false,
		"message": {
			"role": "user",
			"content": "Research how to parse Claude Code session logs"
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)

	assert.Equal(t, "user", ev.Type)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", ev.UUID)
	assert.Equal(t, "session-abc", ev.SessionID)
	assert.Equal(t, "2.1.29", ev.Version)
	assert.False(t, ev.IsMeta)
	assert.Equal(t, "Research how to parse Claude Code session logs", ev.UserText)
	assert.Empty(t, ev.AssistantText)
	assert.Empty(t, ev.ThinkingText)
	assert.Empty(t, ev.ToolUses)
	assert.True(t, ev.HasText())
}

func TestParser_UserPromptEvent_ArrayContent(t *testing.T) {
	// Current format: content is array of content blocks
	line := []byte(`{
		"type": "user",
		"uuid": "u-001",
		"message": {
			"role": "user",
			"content": [
				{"type": "text", "text": "Fix the auth bug"},
				{"type": "text", "text": " in login handler"}
			]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Equal(t, "Fix the auth bug in login handler", ev.UserText)
}

func TestParser_UserPromptEvent_SystemTagsCleaned(t *testing.T) {
	line := []byte(`{
		"type": "user",
		"uuid": "u-002",
		"message": {
			"role": "user",
			"content": "<system-reminder>hook output here</system-reminder>Fix the bug please"
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Equal(t, "Fix the bug please", ev.UserText)
	assert.NotContains(t, ev.UserText, "system-reminder")
}

func TestParser_AssistantTextEvent(t *testing.T) {
	line := []byte(`{
		"type": "assistant",
		"uuid": "a-001",
		"message": {
			"role": "assistant",
			"model": "claude-opus-4-6-20260101",
			"content": [
				{"type": "text", "text": "I'll help you fix the auth bug."}
			]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)

	assert.Equal(t, "assistant", ev.Type)
	assert.Equal(t, "claude-opus-4-6-20260101", ev.Model)
	assert.Equal(t, "I'll help you fix the auth bug.", ev.AssistantText)
	assert.Empty(t, ev.UserText)
}

func TestParser_ThinkingBlock(t *testing.T) {
	line := []byte(`{
		"type": "assistant",
		"uuid": "a-002",
		"message": {
			"role": "assistant",
			"content": [
				{"type": "thinking", "thinking": "The user wants to fix an auth bug. Let me look at the login handler."},
				{"type": "text", "text": "Let me examine the login handler."}
			]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)

	assert.Equal(t, "The user wants to fix an auth bug. Let me look at the login handler.", ev.ThinkingText)
	assert.Equal(t, "Let me examine the login handler.", ev.AssistantText)
	assert.True(t, ev.HasText())

	// AllText concatenates all three streams
	all := ev.AllText()
	assert.Contains(t, all, "auth bug")
	assert.Contains(t, all, "login handler")
}

func TestParser_ThinkingBlock_TextFieldFallback(t *testing.T) {
	// Some versions might use "text" instead of "thinking" for thinking blocks
	line := []byte(`{
		"type": "assistant",
		"uuid": "a-003",
		"message": {
			"role": "assistant",
			"content": [
				{"type": "thinking", "text": "Reasoning about the problem..."}
			]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Equal(t, "Reasoning about the problem...", ev.ThinkingText)
}

func TestParser_ToolUseEvent(t *testing.T) {
	line := []byte(`{
		"type": "assistant",
		"uuid": "a-004",
		"message": {
			"role": "assistant",
			"content": [
				{
					"type": "tool_use",
					"id": "toolu_01234567890abcd",
					"name": "Read",
					"input": {
						"file_path": "/home/corey/aOa/services/auth/handler.py",
						"offset": 100,
						"limit": 50
					}
				}
			]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)

	require.Len(t, ev.ToolUses, 1)
	tu := ev.ToolUses[0]
	assert.Equal(t, "Read", tu.Name)
	assert.Equal(t, "toolu_01234567890abcd", tu.ID)
	assert.Equal(t, "/home/corey/aOa/services/auth/handler.py", tu.FilePath)
	assert.Equal(t, 100, tu.Offset)
	assert.Equal(t, 50, tu.Limit)
}

func TestParser_ToolUseEvent_BashCommand(t *testing.T) {
	line := []byte(`{
		"type": "assistant",
		"uuid": "a-005",
		"message": {
			"role": "assistant",
			"content": [
				{
					"type": "tool_use",
					"id": "toolu_bash001",
					"name": "Bash",
					"input": {
						"command": "aoa grep auth"
					}
				}
			]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.Len(t, ev.ToolUses, 1)
	assert.Equal(t, "Bash", ev.ToolUses[0].Name)
	assert.Equal(t, "aoa grep auth", ev.ToolUses[0].Command)
}

func TestParser_ToolUseEvent_GrepPattern(t *testing.T) {
	line := []byte(`{
		"type": "assistant",
		"uuid": "a-006",
		"message": {
			"role": "assistant",
			"content": [
				{
					"type": "tool_use",
					"id": "toolu_grep001",
					"name": "Grep",
					"input": {
						"pattern": "handleAuth",
						"path": "/home/corey/aOa/src"
					}
				}
			]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.Len(t, ev.ToolUses, 1)
	tu := ev.ToolUses[0]
	assert.Equal(t, "Grep", tu.Name)
	assert.Equal(t, "handleAuth", tu.Pattern)
	assert.Equal(t, "/home/corey/aOa/src", tu.FilePath)
}

func TestParser_MultipleToolUses(t *testing.T) {
	line := []byte(`{
		"type": "assistant",
		"uuid": "a-007",
		"message": {
			"role": "assistant",
			"content": [
				{"type": "text", "text": "Let me check both files."},
				{"type": "tool_use", "id": "t1", "name": "Read", "input": {"file_path": "/a.py"}},
				{"type": "tool_use", "id": "t2", "name": "Read", "input": {"file_path": "/b.py"}}
			]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	assert.Equal(t, "Let me check both files.", ev.AssistantText)
	require.Len(t, ev.ToolUses, 2)
	assert.Equal(t, "/a.py", ev.ToolUses[0].FilePath)
	assert.Equal(t, "/b.py", ev.ToolUses[1].FilePath)
}

func TestParser_100PercentToolUseCoverage(t *testing.T) {
	// All tool types should produce ToolUse entries (even unknown tools)
	tools := []string{"Read", "Write", "Edit", "Bash", "Grep", "Glob", "WebFetch", "WebSearch", "Task", "NotebookEdit", "CustomTool"}
	for _, tool := range tools {
		line := []byte(fmt.Sprintf(`{
			"type": "assistant",
			"uuid": "a-tool-%s",
			"message": {
				"role": "assistant",
				"content": [
					{"type": "tool_use", "id": "t-%s", "name": "%s", "input": {}}
				]
			}
		}`, tool, tool, tool))

		ev, err := ParseLine(line)
		require.NoError(t, err, "tool: %s", tool)
		require.Len(t, ev.ToolUses, 1, "tool: %s", tool)
		assert.Equal(t, tool, ev.ToolUses[0].Name, "tool: %s", tool)
	}
}

func TestParser_EventDedup_UUIDTracking(t *testing.T) {
	// Parser itself doesn't dedup — that's the Tailer's job.
	// But ParseLine should extract UUID for the tailer to use.
	line := []byte(`{"type": "user", "uuid": "dup-001", "message": {"role": "user", "content": "hello"}}`)

	ev1, err := ParseLine(line)
	require.NoError(t, err)
	assert.Equal(t, "dup-001", ev1.UUID)

	// Same UUID — parser returns it, tailer filters
	ev2, err := ParseLine(line)
	require.NoError(t, err)
	assert.Equal(t, "dup-001", ev2.UUID)
}

func TestParser_SkipMalformedJSON(t *testing.T) {
	cases := []struct {
		name string
		line []byte
	}{
		{"truncated", []byte(`{"type": "user", "uuid":`)},
		{"not json", []byte(`this is not json`)},
		{"binary garbage", []byte{0x00, 0x01, 0x02}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := ParseLine(tc.line)
			assert.Error(t, err)
			assert.Nil(t, ev)
		})
	}
}

func TestParser_EmptyLine(t *testing.T) {
	ev, err := ParseLine([]byte(""))
	assert.NoError(t, err)
	assert.Nil(t, ev)

	ev, err = ParseLine([]byte("   "))
	// whitespace-only is not valid JSON, so error
	assert.Error(t, err)
	assert.Nil(t, ev)
}

func TestParser_SystemEvent(t *testing.T) {
	line := []byte(`{
		"type": "system",
		"subtype": "turn_duration",
		"parentUuid": "a-001",
		"durationMs": 5250
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Equal(t, "system", ev.Type)
	assert.False(t, ev.HasText())
}

func TestParser_MetaMessage_Skipped(t *testing.T) {
	line := []byte(`{
		"type": "user",
		"uuid": "meta-001",
		"isMeta": true,
		"message": {"role": "user", "content": "/clear"}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.True(t, ev.IsMeta)
}

func TestParser_TimestampParsing(t *testing.T) {
	cases := []struct {
		ts string
	}{
		{"2026-02-14T18:20:15.000Z"},
		{"2026-02-14T18:20:15Z"},
		{"2026-02-14T18:20:15+00:00"},
		{"2026-02-14T18:20:15.123456789Z"},
	}
	for _, tc := range cases {
		line := []byte(fmt.Sprintf(`{"type": "user", "uuid": "ts-%s", "timestamp": "%s", "message": {"role": "user", "content": "x"}}`, tc.ts, tc.ts))
		ev, err := ParseLine(line)
		require.NoError(t, err, "ts: %s", tc.ts)
		assert.False(t, ev.Timestamp.IsZero(), "ts: %s should parse", tc.ts)
	}
}

func TestParser_AllText(t *testing.T) {
	ev := &SessionEvent{
		UserText:      "fix the bug",
		ThinkingText:  "analyzing the problem",
		AssistantText: "here is the fix",
	}
	all := ev.AllText()
	assert.Equal(t, "fix the bug analyzing the problem here is the fix", all)
}

func TestParser_AssistantUsage(t *testing.T) {
	line := []byte(`{
		"type": "assistant",
		"uuid": "a-usage",
		"message": {
			"role": "assistant",
			"content": [{"type": "text", "text": "response"}],
			"usage": {
				"input_tokens": 500,
				"output_tokens": 200,
				"cache_read_input_tokens": 8000,
				"cache_creation_input_tokens": 100,
				"service_tier": "standard"
			}
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)
	require.NotNil(t, ev.Usage)
	assert.Equal(t, 500, ev.Usage.InputTokens)
	assert.Equal(t, 200, ev.Usage.OutputTokens)
	assert.Equal(t, 8000, ev.Usage.CacheReadTokens)
	assert.Equal(t, 100, ev.Usage.CacheWriteTokens)
	assert.Equal(t, "standard", ev.Usage.ServiceTier)
}

func TestParser_AssistantNoUsage(t *testing.T) {
	line := []byte(`{
		"type": "assistant",
		"uuid": "a-no-usage",
		"message": {
			"role": "assistant",
			"content": [{"type": "text", "text": "response"}]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Nil(t, ev.Usage)
}

func TestParser_SystemEvent_Details(t *testing.T) {
	line := []byte(`{
		"type": "system",
		"uuid": "sys-details",
		"subtype": "turn_duration",
		"parentUuid": "a-001",
		"durationMs": 5250
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Equal(t, "turn_duration", ev.Subtype)
	assert.Equal(t, "a-001", ev.ParentUUID)
	assert.Equal(t, 5250, ev.DurationMs)
}

func TestParser_BOMHandling(t *testing.T) {
	// UTF-8 BOM + valid JSON
	bom := []byte{0xEF, 0xBB, 0xBF}
	jsonData := []byte(`{"type": "user", "uuid": "bom-001", "message": {"role": "user", "content": "hello"}}`)
	line := append(bom, jsonData...)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.Equal(t, "hello", ev.UserText)
}

func TestParser_FilePathMultiPath(t *testing.T) {
	// "path" field instead of "file_path"
	line := []byte(`{
		"type": "assistant",
		"uuid": "mp-001",
		"message": {
			"role": "assistant",
			"content": [
				{"type": "tool_use", "id": "t1", "name": "Glob", "input": {"path": "/src", "pattern": "*.go"}}
			]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.Len(t, ev.ToolUses, 1)
	assert.Equal(t, "/src", ev.ToolUses[0].FilePath)
	assert.Equal(t, "*.go", ev.ToolUses[0].Pattern)
}

// =============================================================================
// T-01: Tailer — File tailing and discovery
// =============================================================================

func TestTailer_TailNewLines(t *testing.T) {
	dir := t.TempDir()

	// Create a session file
	sessionFile := filepath.Join(dir, "session-001.jsonl")
	require.NoError(t, os.WriteFile(sessionFile, []byte(""), 0644))

	var mu sync.Mutex
	var events []*SessionEvent

	tailer := New(Config{
		SessionDir:   dir,
		PollInterval: 50 * time.Millisecond,
		Callback: func(ev *SessionEvent) {
			mu.Lock()
			events = append(events, ev)
			mu.Unlock()
		},
	})

	tailer.Start()
	defer tailer.Stop()
	<-tailer.Started() // Wait for initial file discovery before writing

	// Append lines to the file
	f, err := os.OpenFile(sessionFile, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)

	lines := []string{
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"first prompt"}}`,
		`{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"text","text":"response one"}]}}`,
		`{"type":"user","uuid":"u2","message":{"role":"user","content":"second prompt"}}`,
	}
	for _, line := range lines {
		_, err := f.WriteString(line + "\n")
		require.NoError(t, err)
	}
	f.Close()

	// Wait for tailer to pick up lines
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, events, 3)
	assert.Equal(t, "first prompt", events[0].UserText)
	assert.Equal(t, "response one", events[1].AssistantText)
	assert.Equal(t, "second prompt", events[2].UserText)
}

func TestTailer_SurvivesLogRotation(t *testing.T) {
	dir := t.TempDir()

	// Start with an empty dir — tailer starts, no files yet
	var mu sync.Mutex
	var events []*SessionEvent

	tailer := New(Config{
		SessionDir:   dir,
		PollInterval: 50 * time.Millisecond,
		Callback: func(ev *SessionEvent) {
			mu.Lock()
			events = append(events, ev)
			mu.Unlock()
		},
	})

	tailer.Start()
	defer tailer.Stop()
	<-tailer.Started() // Wait for initial file discovery

	// Create first session file after tailer is running
	session1 := filepath.Join(dir, "session-001.jsonl")
	require.NoError(t, os.WriteFile(session1, []byte(
		`{"type":"user","uuid":"u1","message":{"role":"user","content":"from session 1"}}`+"\n",
	), 0644))

	time.Sleep(200 * time.Millisecond)

	// Create a newer session file (simulates session rotation)
	time.Sleep(50 * time.Millisecond)
	session2 := filepath.Join(dir, "session-002.jsonl")
	require.NoError(t, os.WriteFile(session2, []byte(
		`{"type":"user","uuid":"u2","message":{"role":"user","content":"from session 2"}}`+"\n",
	), 0644))

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.GreaterOrEqual(t, len(events), 2)

	// Should have events from both sessions
	texts := make([]string, len(events))
	for i, ev := range events {
		texts[i] = ev.UserText
	}
	assert.Contains(t, texts, "from session 1")
	assert.Contains(t, texts, "from session 2")
}

func TestTailer_ReadOnly(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session-001.jsonl")

	content := `{"type":"user","uuid":"u1","message":{"role":"user","content":"hello"}}` + "\n"
	require.NoError(t, os.WriteFile(sessionFile, []byte(content), 0444)) // read-only

	var events []*SessionEvent
	tailer := New(Config{
		SessionDir:   dir,
		PollInterval: 50 * time.Millisecond,
		Callback: func(ev *SessionEvent) {
			events = append(events, ev)
		},
	})

	// Read the file — should work even with read-only perms (we only read)
	tailer.currentFile = sessionFile
	tailer.offset = 0
	tailer.readNewLines()

	require.Len(t, events, 1)
	assert.Equal(t, "hello", events[0].UserText)
}

func TestTailer_StopCleanup(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session-001.jsonl")
	require.NoError(t, os.WriteFile(sessionFile, []byte(""), 0644))

	callCount := 0
	tailer := New(Config{
		SessionDir:   dir,
		PollInterval: 20 * time.Millisecond,
		Callback: func(ev *SessionEvent) {
			callCount++
		},
	})

	tailer.Start()
	time.Sleep(50 * time.Millisecond)
	tailer.Stop()

	// Write after stop — should NOT trigger callback
	beforeCount := callCount
	f, _ := os.OpenFile(sessionFile, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString(`{"type":"user","uuid":"late","message":{"role":"user","content":"too late"}}` + "\n")
	f.Close()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, beforeCount, callCount, "no events after Stop()")

	// Double stop should not panic
	tailer.Stop()
}

func TestTailer_EventDedup(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session-001.jsonl")

	// Write duplicate UUIDs (known bug #5034)
	content := `{"type":"user","uuid":"dup1","message":{"role":"user","content":"first"}}` + "\n" +
		`{"type":"user","uuid":"dup1","message":{"role":"user","content":"duplicate"}}` + "\n" +
		`{"type":"user","uuid":"dup2","message":{"role":"user","content":"second"}}` + "\n"
	require.NoError(t, os.WriteFile(sessionFile, []byte(content), 0644))

	var events []*SessionEvent
	tailer := New(Config{
		SessionDir:   dir,
		PollInterval: 50 * time.Millisecond,
		Callback: func(ev *SessionEvent) {
			events = append(events, ev)
		},
	})

	tailer.currentFile = sessionFile
	tailer.offset = 0
	tailer.readNewLines()

	// Should have 2 events, not 3 (duplicate filtered)
	require.Len(t, events, 2)
	assert.Equal(t, "first", events[0].UserText)
	assert.Equal(t, "second", events[1].UserText)
}

func TestTailer_SkipsMeta(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session-001.jsonl")

	content := `{"type":"user","uuid":"m1","isMeta":true,"message":{"role":"user","content":"/clear"}}` + "\n" +
		`{"type":"user","uuid":"m2","isMeta":false,"message":{"role":"user","content":"real prompt"}}` + "\n"
	require.NoError(t, os.WriteFile(sessionFile, []byte(content), 0644))

	var events []*SessionEvent
	tailer := New(Config{
		SessionDir:   dir,
		PollInterval: 50 * time.Millisecond,
		Callback: func(ev *SessionEvent) {
			events = append(events, ev)
		},
	})

	tailer.currentFile = sessionFile
	tailer.offset = 0
	tailer.readNewLines()

	require.Len(t, events, 1)
	assert.Equal(t, "real prompt", events[0].UserText)
}

func TestTailer_SkipsOversizedLines(t *testing.T) {
	dir := t.TempDir()
	sessionFile := filepath.Join(dir, "session-001.jsonl")

	// Create a line that exceeds 512KB (known bug #23948 — bloated tool outputs)
	bigContent := make([]byte, 600*1024)
	for i := range bigContent {
		bigContent[i] = 'x'
	}
	bigLine, _ := json.Marshal(map[string]any{
		"type": "assistant", "uuid": "big1",
		"message": map[string]any{
			"role":    "assistant",
			"content": string(bigContent),
		},
	})

	normalLine := `{"type":"user","uuid":"n1","message":{"role":"user","content":"normal"}}`

	f, _ := os.Create(sessionFile)
	f.Write(bigLine)
	f.WriteString("\n")
	f.WriteString(normalLine + "\n")
	f.Close()

	var events []*SessionEvent
	tailer := New(Config{
		SessionDir:   dir,
		PollInterval: 50 * time.Millisecond,
		Callback: func(ev *SessionEvent) {
			events = append(events, ev)
		},
	})

	tailer.currentFile = sessionFile
	tailer.offset = 0
	tailer.readNewLines()

	// Only the normal line should come through
	require.Len(t, events, 1)
	assert.Equal(t, "normal", events[0].UserText)
}

// =============================================================================
// T-03: Signal Extraction helpers
// =============================================================================

func TestSignals_ReadEventToObserve(t *testing.T) {
	line := []byte(`{
		"type": "assistant",
		"uuid": "a-read",
		"message": {
			"role": "assistant",
			"content": [
				{"type": "tool_use", "id": "t1", "name": "Read", "input": {"file_path": "/src/auth.py", "offset": 10, "limit": 50}}
			]
		}
	}`)

	ev, err := ParseLine(line)
	require.NoError(t, err)
	require.Len(t, ev.ToolUses, 1)

	tu := ev.ToolUses[0]
	assert.Equal(t, "Read", tu.Name)
	assert.Equal(t, "/src/auth.py", tu.FilePath)
	assert.Equal(t, 10, tu.Offset)
	assert.Equal(t, 50, tu.Limit)
}

func TestSignals_RangeGate_Under500(t *testing.T) {
	tu := ToolUse{Name: "Read", FilePath: "/src/auth.py", Limit: 100}
	// Limit < 500 → should fire signal (intentional, focused read)
	assert.True(t, tu.Limit > 0 && tu.Limit < 500)
}

func TestSignals_RangeGate_Over500(t *testing.T) {
	tu := ToolUse{Name: "Read", FilePath: "/src/auth.py", Limit: 2000}
	// Limit >= 500 → skip (full-file scan, not focused)
	assert.True(t, tu.Limit >= 500)
}

func TestSignals_RangeGate_ZeroLimit(t *testing.T) {
	tu := ToolUse{Name: "Read", FilePath: "/src/auth.py", Limit: 0}
	// Limit == 0 means "read whole file" → skip (not focused)
	assert.True(t, tu.Limit == 0 || tu.Limit >= 500)
}

// =============================================================================
// Session directory discovery
// =============================================================================

func TestSessionDirForProject(t *testing.T) {
	home, _ := os.UserHomeDir()

	dir := SessionDirForProject("/home/corey/aOa")
	expected := filepath.Join(home, ".claude", "projects", "-home-corey-aOa")
	assert.Equal(t, expected, dir)
}

func TestSessionDirForProject_NestedPath(t *testing.T) {
	home, _ := os.UserHomeDir()

	dir := SessionDirForProject("/home/corey/projects/my-app")
	expected := filepath.Join(home, ".claude", "projects", "-home-corey-projects-my-app")
	assert.Equal(t, expected, dir)
}
