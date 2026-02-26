// Package tailer provides defensive parsing of Claude Code session JSONL logs.
//
// The JSONL format is NOT documented by Anthropic and changes across versions.
// This parser uses map[string]any extraction with multi-path field resolution
// to survive schema changes. It never crashes on unexpected input.
//
// Three text streams are extracted for bigram/intent analysis:
//   - UserText: what the user typed (cleaned of system tags)
//   - ThinkingText: Claude's extended thinking/reasoning
//   - AssistantText: Claude's visible response text
//
// Tool events are extracted for observe signals (file reads, edits, searches).
package tailer

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// SessionEvent is the normalized output of parsing one JSONL line.
// Fields are best-effort: any field may be empty if the source format changed.
type SessionEvent struct {
	// Identity
	Type      string    // "user", "assistant", "system", "unknown"
	UUID      string    // unique message ID (for dedup)
	Timestamp time.Time // message creation time (zero if unparseable)

	// Conversation text (for bigrams)
	UserText      string // user prompt text, cleaned of system tags
	AssistantText string // Claude's visible response text
	ThinkingText  string // Claude's extended thinking/reasoning

	// Tool events (for observe signals)
	ToolUses []ToolUse

	// Metadata
	Model     string // e.g., "claude-opus-4-6-20260101"
	Version   string // Claude Code version, e.g., "2.1.29"
	SessionID string // session identifier
	IsMeta    bool   // internal/meta message (skip for intents)

	// Token economics (from assistant message.usage)
	Usage *MessageUsage

	// Tool result sizes (tool_use_id -> content char count)
	ToolResultSizes map[string]int

	// System event details
	Subtype    string // e.g., "turn_duration"
	ParentUUID string // links system events to parent AI turn
	DurationMs int    // turn duration in milliseconds
}

// ToolUse represents a single tool invocation extracted from an assistant message.
type ToolUse struct {
	Name     string         // tool name: Read, Edit, Write, Bash, Grep, Glob, etc.
	ID       string         // tool_use_id
	Input    map[string]any // raw tool input (for future extensibility)
	FilePath string         // extracted from input (multi-path: file_path, path, notebook_path)
	Offset   int            // file read offset (0 if absent)
	Limit    int            // file read limit (0 if absent)
	Command  string         // bash command text (empty for non-Bash tools)
	Pattern  string         // grep/glob pattern (empty if absent)
}

// MessageUsage holds token economics from an AI model response.
// Extracted from the message.usage object in assistant messages.
type MessageUsage struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ServiceTier      string
}

// AllText returns all conversation text concatenated for bigram extraction.
// Order: user text, thinking text, assistant text.
func (e *SessionEvent) AllText() string {
	var parts []string
	if e.UserText != "" {
		parts = append(parts, e.UserText)
	}
	if e.ThinkingText != "" {
		parts = append(parts, e.ThinkingText)
	}
	if e.AssistantText != "" {
		parts = append(parts, e.AssistantText)
	}
	return strings.Join(parts, " ")
}

// HasText returns true if the event contains any conversation text.
func (e *SessionEvent) HasText() bool {
	return e.UserText != "" || e.ThinkingText != "" || e.AssistantText != ""
}

// ParseLine parses a single JSONL line into a SessionEvent.
// Returns nil, nil for empty lines. Returns nil, error for malformed JSON.
// Unknown fields are silently ignored. Missing fields get zero values.
func ParseLine(line []byte) (*SessionEvent, error) {
	line = trimBOM(line)
	if len(line) == 0 {
		return nil, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, err
	}

	ev := &SessionEvent{
		Type:       getString(raw, "type"),
		UUID:       getStringAny(raw, "uuid", "id"),
		Timestamp:  getTime(raw, "timestamp"),
		Version:    getString(raw, "version"),
		SessionID:  getStringAny(raw, "sessionId", "session_id"),
		IsMeta:     getBool(raw, "isMeta"),
		Subtype:    getString(raw, "subtype"),
		ParentUUID: getStringAny(raw, "parentUuid", "parent_uuid", "parentId"),
		DurationMs: getInt(raw, "durationMs"),
	}

	// Extract message content — the core of the parser
	msg := getMap(raw, "message")
	if msg != nil {
		ev.Model = getString(msg, "model")
		role := getStringAny(msg, "role")
		ev.extractContent(msg, role)

		// Extract usage (token economics)
		if usage := getMap(msg, "usage"); usage != nil {
			ev.Usage = &MessageUsage{
				InputTokens:      getInt(usage, "input_tokens"),
				OutputTokens:     getInt(usage, "output_tokens"),
				CacheReadTokens:  getInt(usage, "cache_read_input_tokens"),
				CacheWriteTokens: getInt(usage, "cache_creation_input_tokens"),
				ServiceTier:      getString(usage, "service_tier"),
			}
		}
	}

	// Fallback: some formats put content at top level
	if msg == nil && ev.Type != "" {
		ev.extractContent(raw, ev.Type)
	}

	// Normalize type
	if ev.Type == "" {
		ev.Type = "unknown"
	}

	return ev, nil
}

// extractContent extracts text and tool uses from a message-like map.
// Handles both string content (older format) and array content (current format).
func (e *SessionEvent) extractContent(msg map[string]any, role string) {
	content := msg["content"]
	if content == nil {
		return
	}

	switch c := content.(type) {
	case string:
		// Older format: content is a plain string
		text := cleanSystemTags(c)
		if role == "user" {
			e.UserText = text
		} else {
			e.AssistantText = text
		}

	case []any:
		// Current format: content is array of content blocks
		for _, item := range c {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			e.extractContentBlock(block, role)
		}
	}
}

// extractContentBlock processes a single content block from the content array.
// Block types: text, thinking, tool_use, tool_result, image, etc.
func (e *SessionEvent) extractContentBlock(block map[string]any, role string) {
	blockType := getString(block, "type")

	switch blockType {
	case "text":
		text := getString(block, "text")
		if text == "" {
			return
		}
		if role == "user" {
			e.UserText = appendText(e.UserText, cleanSystemTags(text))
		} else {
			e.AssistantText = appendText(e.AssistantText, text)
		}

	case "thinking":
		// Try multiple field names — Anthropic may change this
		text := getStringAny(block, "thinking", "text", "content")
		if text != "" {
			e.ThinkingText = appendText(e.ThinkingText, text)
		}

	case "tool_use":
		tu := ToolUse{
			Name: getStringAny(block, "name", "tool_name"),
			ID:   getStringAny(block, "id", "tool_use_id"),
		}
		if inp, ok := block["input"].(map[string]any); ok {
			tu.Input = inp
			tu.FilePath = getStringAny(inp, "file_path", "path", "notebook_path", "filePath")
			tu.Offset = getInt(inp, "offset")
			tu.Limit = getInt(inp, "limit")
			tu.Command = getStringAny(inp, "command", "cmd")
			tu.Pattern = getStringAny(inp, "pattern", "query", "q")
		}
		// Also try top-level tool_input (alternate format)
		if tu.Input == nil {
			if inp, ok := block["tool_input"].(map[string]any); ok {
				tu.Input = inp
				tu.FilePath = getStringAny(inp, "file_path", "path", "notebook_path", "filePath")
				tu.Offset = getInt(inp, "offset")
				tu.Limit = getInt(inp, "limit")
				tu.Command = getStringAny(inp, "command", "cmd")
				tu.Pattern = getStringAny(inp, "pattern", "query", "q")
			}
		}
		e.ToolUses = append(e.ToolUses, tu)

	case "tool_result":
		// Tool results sometimes contain text we want for context.
		// But they can also be huge (multi-MB). Only extract short ones.
		text := getString(block, "text")
		if len(text) > 0 && len(text) < 2000 {
			e.AssistantText = appendText(e.AssistantText, text)
		}

		// Capture tool result size for throughput tracking.
		// Correlate via tool_use_id back to the originating tool_use.
		toolUseID := getStringAny(block, "tool_use_id", "id")
		if toolUseID != "" {
			chars := 0
			// Content can be string, array of blocks, or just "text" field
			switch c := block["content"].(type) {
			case string:
				chars = len(c)
			case []any:
				for _, item := range c {
					if m, ok := item.(map[string]any); ok {
						chars += len(getString(m, "text"))
					}
				}
			default:
				// Fallback: use "text" field length if present
				chars = len(text)
			}
			if chars > 0 {
				if e.ToolResultSizes == nil {
					e.ToolResultSizes = make(map[string]int)
				}
				e.ToolResultSizes[toolUseID] = chars
			}
		}
	}
}

// --- System tag cleaning ---

// systemTagRE matches system-injected tags that should be stripped from user text.
var systemTagRE = regexp.MustCompile(
	`(?s)<system-reminder>.*?</system-reminder>` +
		`|<local-command-[^>]*>.*?</local-command-[^>]*>` +
		`|<command-[^>]+>.*?</command-[^>]+>` +
		`|<hook-[^>]+>.*?</hook-[^>]+>`,
)

// residualTagRE strips any remaining XML-like tags after system tag removal.
var residualTagRE = regexp.MustCompile(`<[^>]+>`)

// whitespaceRE collapses runs of whitespace to a single space.
var whitespaceRE = regexp.MustCompile(`\s+`)

// cleanSystemTags removes system-injected blocks and residual tags from user text.
func cleanSystemTags(text string) string {
	clean := systemTagRE.ReplaceAllString(text, "")
	clean = residualTagRE.ReplaceAllString(clean, "")
	clean = whitespaceRE.ReplaceAllString(clean, " ")
	return strings.TrimSpace(clean)
}

// --- Helper functions for defensive map access ---

// getString safely extracts a string from a map. Returns "" if missing or wrong type.
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getStringAny tries multiple keys and returns the first non-empty string found.
func getStringAny(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// getMap safely extracts a nested map. Returns nil if missing or wrong type.
func getMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

// getBool safely extracts a boolean. Returns false if missing or wrong type.
func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// getInt safely extracts an integer from a map value that may be float64 (JSON numbers).
func getInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

// getTime parses an ISO 8601 timestamp string. Returns zero time on failure.
func getTime(m map[string]any, key string) time.Time {
	s := getString(m, key)
	if s == "" {
		return time.Time{}
	}
	// Try multiple formats
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// trimBOM strips UTF-8 BOM if present.
func trimBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	return data
}

// appendText joins two text fragments with a space separator.
func appendText(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + " " + addition
}
