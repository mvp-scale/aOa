package ports

import "time"

// =============================================================================
// Universal Session Port — Agent-Agnostic Canonical Model
//
// This defines what an AI coding session IS, regardless of which AI agent
// produced it (Claude, Gemini, Cursor, etc.). Adapters translate agent-specific
// formats into this canonical representation.
//
// Four pillars:
//   1. Conversation  — text streams for bigrams and intent
//   2. Tool Activity — what the AI did, what files it touched
//   3. Economics     — tokens, cache, cost, throughput
//   4. Health        — extraction yield, gaps, version tracking
// =============================================================================

// SessionReader is the port for consuming AI coding session events.
// Each AI agent gets its own adapter that implements this interface.
type SessionReader interface {
	// Start begins reading session events. New events are delivered via
	// the callback. The reader tails the active session incrementally.
	// Call Stop() to terminate.
	Start(callback func(SessionEvent))

	// Stop terminates the reader and releases resources.
	// Safe to call multiple times. Blocks until cleanup completes.
	Stop()

	// Health returns extraction health metrics accumulated since the
	// last call to Health (or since Start). Calling Health resets the
	// internal counters for the next reporting window.
	Health() ExtractionHealth
}

// SessionEvent is the canonical representation of one atomic event
// in an AI coding session. Every event has exactly one Kind.
//
// Agent adapters decompose compound messages (e.g., one Claude message
// with thinking + text + 3 tool uses) into multiple atomic SessionEvents,
// linked by TurnID.
type SessionEvent struct {
	// --- Identity ---

	// ID is a unique identifier for this event (from the source, e.g., UUID).
	ID string

	// TurnID groups events that originated from the same AI turn.
	// Multiple events with the same TurnID happened together
	// (e.g., thinking + response + tool calls in one assistant message).
	TurnID string

	// SessionID identifies the session this event belongs to.
	SessionID string

	// Timestamp is when the event occurred. Zero if unparseable.
	Timestamp time.Time

	// --- Classification ---

	// Kind identifies what this event represents.
	Kind EventKind

	// --- Content ---

	// Text is the primary text content. Meaning depends on Kind:
	//   UserInput:   cleaned user prompt (system tags stripped)
	//   AIThinking:  reasoning/thinking text
	//   AIResponse:  visible response text
	//   ToolResult:  short tool output text (if available)
	//   SystemMeta:  subtype or description
	Text string

	// Tool carries tool invocation details. Non-nil only when Kind == ToolInvocation.
	Tool *ToolEvent

	// File carries file access details. Non-nil when a file was referenced.
	// Can be present on ToolInvocation, FileAccess, or ToolResult events.
	File *FileRef

	// Usage carries token economics. Non-nil when the source provided usage data.
	// Typically present on AIResponse events (one per AI turn).
	Usage *TokenUsage

	// ToolResultSizes maps tool_use_id to content character count.
	// Non-nil only on EventToolResult events.
	ToolResultSizes map[string]int

	// DurationMs is the turn duration in milliseconds.
	// Non-zero only on SystemMeta events with timing data.
	// Linked to the AI turn via TurnID.
	DurationMs int

	// --- Agent Metadata ---

	// Model identifies which AI model handled this turn (e.g., "claude-opus-4-6").
	Model string

	// AgentVersion is the AI agent's version string (e.g., "2.1.42").
	AgentVersion string
}

// EventKind classifies a SessionEvent. Each event is exactly one kind.
type EventKind int

const (
	// EventUserInput is text the human typed.
	EventUserInput EventKind = iota

	// EventAIThinking is the AI's internal reasoning (not shown to user).
	EventAIThinking

	// EventAIResponse is the AI's visible response text.
	EventAIResponse

	// EventToolInvocation is the AI requesting a tool execution.
	EventToolInvocation

	// EventToolResult is the output from a tool execution.
	EventToolResult

	// EventFileAccess is a file being read, written, or edited.
	// Derived from tool invocations that reference files.
	EventFileAccess

	// EventSystemMeta is a session lifecycle or metadata event
	// (turn duration, session start/stop, etc.).
	EventSystemMeta
)

// String returns the event kind name.
func (k EventKind) String() string {
	switch k {
	case EventUserInput:
		return "user_input"
	case EventAIThinking:
		return "ai_thinking"
	case EventAIResponse:
		return "ai_response"
	case EventToolInvocation:
		return "tool_invocation"
	case EventToolResult:
		return "tool_result"
	case EventFileAccess:
		return "file_access"
	case EventSystemMeta:
		return "system_meta"
	default:
		return "unknown"
	}
}

// ToolEvent describes a tool invocation. Agent-agnostic.
type ToolEvent struct {
	// Name is the tool identifier (e.g., "Read", "Bash", "Grep").
	Name string

	// ToolID is the unique ID for this invocation (for correlating with results).
	ToolID string

	// Input is the raw tool input map (for extensibility and future adapters).
	Input map[string]any

	// Command is the shell command text (for shell/bash tools). Empty otherwise.
	Command string

	// Pattern is the search pattern (for grep/glob tools). Empty otherwise.
	Pattern string
}

// FileRef describes a file access event. Agent-agnostic.
type FileRef struct {
	// Path is the absolute file path.
	Path string

	// Offset is the starting line number (0 if reading from start or whole file).
	Offset int

	// Limit is the number of lines read (0 if reading whole file).
	Limit int

	// Action describes what was done: "read", "write", "edit", "search", "glob".
	Action string
}

// TokenUsage captures token economics for one AI turn.
// All fields are optional — adapters populate what the source provides.
type TokenUsage struct {
	// InputTokens is the number of non-cached input tokens.
	InputTokens int

	// OutputTokens is the number of output tokens generated.
	OutputTokens int

	// CacheReadTokens is input tokens served from cache (cheaper).
	CacheReadTokens int

	// CacheWriteTokens is input tokens written to cache for future turns.
	CacheWriteTokens int

	// ServiceTier indicates the pricing/priority tier (e.g., "standard").
	ServiceTier string
}

// CacheHitRate returns the fraction of context tokens served from cache.
// Returns 0 if no context tokens were used.
func (u *TokenUsage) CacheHitRate() float64 {
	total := u.InputTokens + u.CacheReadTokens + u.CacheWriteTokens
	if total == 0 {
		return 0
	}
	return float64(u.CacheReadTokens) / float64(total)
}

// TotalContextTokens returns the total context window size for this turn.
func (u *TokenUsage) TotalContextTokens() int {
	return u.InputTokens + u.CacheReadTokens + u.CacheWriteTokens
}

// =============================================================================
// Extraction Health — The Validation Layer
//
// Adapters accumulate these metrics as they parse. Consumers check them
// to detect format degradation, silent failures, and version changes.
// If yield drops to zero for a category that should have data, something broke.
// =============================================================================

// ExtractionHealth reports how well an adapter is extracting data from
// its source format. A healthy adapter has high yield and zero gaps.
// A degraded adapter has low yield or increasing gaps.
type ExtractionHealth struct {
	// --- Volume ---

	// LinesRead is the total number of source lines/records processed.
	LinesRead int

	// LinesParsed is the number of lines that produced valid events.
	LinesParsed int

	// LinesSkipped is lines that were malformed, oversized, or deduped.
	LinesSkipped int

	// --- Event Yield ---

	// EventCounts tracks how many events of each kind were produced.
	EventCounts map[EventKind]int

	// TextYield is the number of events that had non-empty Text.
	// For a healthy session, this should be > 0 for user + assistant events.
	TextYield int

	// ToolYield is the number of events with non-nil Tool data.
	ToolYield int

	// UsageYield is the number of events with non-nil Usage data.
	UsageYield int

	// FileYield is the number of events with non-nil File data.
	FileYield int

	// --- Gap Detection ---

	// Gaps is the count of events where expected content was missing.
	// A user event with no text, or an assistant event with no text
	// AND no tools, increments this counter.
	Gaps int

	// UnknownTypes tracks source event types the adapter couldn't classify.
	// Keys are the raw type strings from the source format.
	// Non-zero counts here mean the format has new event types we're not handling.
	UnknownTypes map[string]int

	// --- Version Tracking ---

	// AgentVersion is the most recent version string seen in the source.
	AgentVersion string

	// VersionChanged is true if the version string changed during this window.
	// A version change is the most likely moment for format breaking changes.
	VersionChanged bool

	// --- Timing ---

	// WindowStart is when this health window began.
	WindowStart time.Time

	// WindowEnd is when this health window ended (set on Health() call).
	WindowEnd time.Time
}

// IsHealthy returns true if extraction appears to be working correctly.
// Checks: at least some lines parsed, non-zero text yield if events exist,
// no excessive gaps.
func (h *ExtractionHealth) IsHealthy() bool {
	if h.LinesRead == 0 {
		return true // no data yet, can't judge
	}
	if h.LinesParsed == 0 {
		return false // read lines but parsed nothing — format broke
	}
	// If we have user or assistant events, we should have text
	userCount := h.EventCounts[EventUserInput]
	aiCount := h.EventCounts[EventAIResponse] + h.EventCounts[EventAIThinking]
	if (userCount+aiCount) > 10 && h.TextYield == 0 {
		return false // many conversation events but no text extracted
	}
	// Gap rate > 50% is unhealthy
	totalEvents := h.LinesParsed
	if totalEvents > 10 && h.Gaps > totalEvents/2 {
		return false
	}
	return true
}

// Summary returns a one-line description of extraction health.
func (h *ExtractionHealth) Summary() string {
	if h.LinesRead == 0 {
		return "no data"
	}
	if !h.IsHealthy() {
		return "degraded"
	}
	unknownCount := 0
	for _, c := range h.UnknownTypes {
		unknownCount += c
	}
	if unknownCount > 0 || h.VersionChanged {
		return "ok (format changes detected)"
	}
	return "healthy"
}
