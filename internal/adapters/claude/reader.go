// Package claude implements ports.SessionReader for Claude Code sessions.
//
// It wraps the tailer package to tail Claude Code JSONL session logs,
// decomposes raw events into canonical SessionEvents (the Session Prism),
// and tracks extraction health metrics.
//
// Architecture:
//
//	tailer.Tailer (file discovery, polling, dedup)
//	    → tailer.SessionEvent (Claude-specific)
//	        → claude.Reader.translate()
//	            → ports.SessionEvent (agent-agnostic, canonical)
//
// A single Claude assistant message may contain thinking + text + N tool_uses.
// The reader decomposes this into N+2 atomic ports.SessionEvents, all linked
// by TurnID. Consumers see simple events; grouping is opt-in via TurnID.
package claude

import (
	"fmt"
	"sync"
	"time"

	"github.com/corey/aoa/internal/adapters/tailer"
	"github.com/corey/aoa/internal/ports"
)

// Reader implements ports.SessionReader for Claude Code sessions.
type Reader struct {
	cfg    Config
	tailer *tailer.Tailer

	cb func(ports.SessionEvent)

	mu     sync.Mutex
	health ports.ExtractionHealth
}

// Config holds initialization parameters for a Claude session reader.
type Config struct {
	// ProjectRoot is the absolute path to the project.
	// Used to discover the Claude Code session directory.
	ProjectRoot string

	// SessionDir overrides automatic session directory discovery.
	// If set, ProjectRoot is ignored for directory resolution.
	SessionDir string

	// PollInterval is how often to check for new session log lines.
	// Default: 500ms.
	PollInterval time.Duration
}

// New creates a Reader. Does not start reading until Start() is called.
func New(cfg Config) *Reader {
	return &Reader{
		cfg:    cfg,
		health: freshHealth(),
	}
}

// Start begins reading session events. Events are delivered via callback.
// The reader tails the active session log incrementally.
func (r *Reader) Start(callback func(ports.SessionEvent)) {
	r.cb = callback
	r.tailer = tailer.New(tailer.Config{
		ProjectRoot:  r.cfg.ProjectRoot,
		SessionDir:   r.cfg.SessionDir,
		PollInterval: r.cfg.PollInterval,
		Callback:     r.onRawEvent,
		OnError:      r.onParseError,
	})
	r.tailer.Start()
}

// Stop terminates the reader and releases resources. Safe to call multiple times.
func (r *Reader) Stop() {
	if r.tailer != nil {
		r.tailer.Stop()
	}
}

// Health returns extraction health metrics accumulated since the last call
// to Health (or since Start). Resets internal counters for the next window.
func (r *Reader) Health() ports.ExtractionHealth {
	r.mu.Lock()
	defer r.mu.Unlock()
	h := r.health
	h.WindowEnd = time.Now()
	r.health = freshHealth()
	return h
}

// Started returns a channel that closes after the underlying tailer
// completes initial file discovery. Useful for synchronization in tests.
func (r *Reader) Started() <-chan struct{} {
	if r.tailer != nil {
		return r.tailer.Started()
	}
	// Not started yet — return a closed channel
	ch := make(chan struct{})
	close(ch)
	return ch
}

// --- Internal ---

func freshHealth() ports.ExtractionHealth {
	return ports.ExtractionHealth{
		EventCounts:  make(map[ports.EventKind]int),
		UnknownTypes: make(map[string]int),
		WindowStart:  time.Now(),
	}
}

// onRawEvent is the tailer callback. Translates raw → canonical and delivers.
func (r *Reader) onRawEvent(raw *tailer.SessionEvent) {
	r.mu.Lock()
	r.health.LinesRead++
	r.health.LinesParsed++

	// Version tracking
	if raw.Version != "" {
		if r.health.AgentVersion != "" && r.health.AgentVersion != raw.Version {
			r.health.VersionChanged = true
		}
		r.health.AgentVersion = raw.Version
	}
	r.mu.Unlock()

	events := r.translate(raw)

	// Update health metrics for each emitted event
	r.mu.Lock()
	for i := range events {
		ev := &events[i]
		r.health.EventCounts[ev.Kind]++
		if ev.Text != "" {
			r.health.TextYield++
		}
		if ev.Tool != nil {
			r.health.ToolYield++
		}
		if ev.Usage != nil {
			r.health.UsageYield++
		}
		if ev.File != nil {
			r.health.FileYield++
		}
	}
	r.mu.Unlock()

	// Deliver canonical events
	if r.cb != nil {
		for _, ev := range events {
			r.cb(ev)
		}
	}
}

// onParseError is the tailer error callback.
func (r *Reader) onParseError(_ error) {
	r.mu.Lock()
	r.health.LinesRead++
	r.health.LinesSkipped++
	r.mu.Unlock()
}

// translate decomposes one raw tailer event into zero or more canonical events.
func (r *Reader) translate(raw *tailer.SessionEvent) []ports.SessionEvent {
	switch raw.Type {
	case "user":
		return r.translateUser(raw)
	case "assistant":
		return r.translateAssistant(raw)
	case "system":
		return r.translateSystem(raw)
	default:
		// progress, file-history-snapshot, queue-operation, etc.
		r.mu.Lock()
		r.health.UnknownTypes[raw.Type]++
		r.mu.Unlock()
		return nil
	}
}

// translateUser converts a user message into EventUserInput and/or EventToolResult.
// User messages carry tool_result blocks alongside user text.
func (r *Reader) translateUser(raw *tailer.SessionEvent) []ports.SessionEvent {
	var events []ports.SessionEvent

	if raw.UserText != "" {
		events = append(events, ports.SessionEvent{
			ID:           raw.UUID,
			TurnID:       raw.UUID,
			SessionID:    raw.SessionID,
			Timestamp:    raw.Timestamp,
			Kind:         ports.EventUserInput,
			Text:         raw.UserText,
			AgentVersion: raw.Version,
		})
	}

	if len(raw.ToolResultSizes) > 0 {
		events = append(events, ports.SessionEvent{
			ID:              raw.UUID + ":toolresult",
			TurnID:          raw.UUID,
			SessionID:       raw.SessionID,
			Timestamp:       raw.Timestamp,
			Kind:            ports.EventToolResult,
			ToolResultSizes: raw.ToolResultSizes,
			AgentVersion:    raw.Version,
		})
	}

	if len(events) == 0 {
		r.mu.Lock()
		r.health.Gaps++
		r.mu.Unlock()
	}

	return events
}

// translateAssistant decomposes an assistant message into atomic events:
//   - EventAIThinking (if thinking text present)
//   - EventAIResponse (if text present, or to carry usage data)
//   - EventToolInvocation (per tool_use, with FileRef if applicable)
func (r *Reader) translateAssistant(raw *tailer.SessionEvent) []ports.SessionEvent {
	turnID := raw.UUID
	var events []ports.SessionEvent

	// 1. Thinking
	if raw.ThinkingText != "" {
		events = append(events, ports.SessionEvent{
			ID:           turnID + ":thinking",
			TurnID:       turnID,
			SessionID:    raw.SessionID,
			Timestamp:    raw.Timestamp,
			Kind:         ports.EventAIThinking,
			Text:         raw.ThinkingText,
			Model:        raw.Model,
			AgentVersion: raw.Version,
		})
	}

	// 2. Response text (also carries usage when present)
	if raw.AssistantText != "" || raw.Usage != nil {
		ev := ports.SessionEvent{
			ID:           turnID + ":response",
			TurnID:       turnID,
			SessionID:    raw.SessionID,
			Timestamp:    raw.Timestamp,
			Kind:         ports.EventAIResponse,
			Text:         raw.AssistantText,
			Model:        raw.Model,
			AgentVersion: raw.Version,
		}
		if raw.Usage != nil {
			ev.Usage = &ports.TokenUsage{
				InputTokens:      raw.Usage.InputTokens,
				OutputTokens:     raw.Usage.OutputTokens,
				CacheReadTokens:  raw.Usage.CacheReadTokens,
				CacheWriteTokens: raw.Usage.CacheWriteTokens,
				ServiceTier:      raw.Usage.ServiceTier,
			}
		}
		events = append(events, ev)
	}

	// 3. Tool invocations
	for i, tu := range raw.ToolUses {
		toolEv := ports.SessionEvent{
			ID:        fmt.Sprintf("%s:tool:%d", turnID, i),
			TurnID:    turnID,
			SessionID: raw.SessionID,
			Timestamp: raw.Timestamp,
			Kind:      ports.EventToolInvocation,
			Tool: &ports.ToolEvent{
				Name:    tu.Name,
				ToolID:  tu.ID,
				Input:   tu.Input,
				Command: tu.Command,
				Pattern: tu.Pattern,
			},
			Model:        raw.Model,
			AgentVersion: raw.Version,
		}
		if tu.FilePath != "" {
			toolEv.File = &ports.FileRef{
				Path:   tu.FilePath,
				Offset: tu.Offset,
				Limit:  tu.Limit,
				Action: toolAction(tu.Name),
			}
		}
		events = append(events, toolEv)
	}

	// Gap: assistant with nothing at all
	if len(events) == 0 {
		r.mu.Lock()
		r.health.Gaps++
		r.mu.Unlock()
	}

	return events
}

// translateSystem converts a system event to EventSystemMeta.
func (r *Reader) translateSystem(raw *tailer.SessionEvent) []ports.SessionEvent {
	return []ports.SessionEvent{{
		ID:           raw.UUID,
		TurnID:       raw.ParentUUID,
		SessionID:    raw.SessionID,
		Timestamp:    raw.Timestamp,
		Kind:         ports.EventSystemMeta,
		Text:         raw.Subtype,
		DurationMs:   raw.DurationMs,
		AgentVersion: raw.Version,
	}}
}

// toolAction maps Claude tool names to agent-agnostic file access actions.
func toolAction(name string) string {
	switch name {
	case "Read":
		return "read"
	case "Write":
		return "write"
	case "Edit", "NotebookEdit":
		return "edit"
	case "Grep", "Glob":
		return "search"
	default:
		return "access"
	}
}
