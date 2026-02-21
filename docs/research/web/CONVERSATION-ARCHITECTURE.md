# Conversation Dashboard Architecture — Data Model & Implementation Plan

> **Focus:** Conversation tab design with 2-3 column layout (Feed | Tools | Agents)
> **Goal:** Show everything happening in Claude Code in a viable, beautiful way

---

## Visual Layout Concept

```
┌───────────────────────────────────────────────────────────────────┐
│ CONVERSATION TAB                                                  │
│                                                                   │
│ ┌──────────────────────────────────────────────────────────────┐ │
│ │ Token Stats Grid (5-7 cards)                                  │ │
│ │ Input | Output | Cache Read | Cache Hit % | Opus | Sonnet    │ │
│ └──────────────────────────────────────────────────────────────┘ │
│                                                                   │
│ ┌──────────────────────┬──────────────┬─────────────────────┐   │
│ │ Conversation Feed    │ Tools        │ Agents (optional)   │   │
│ │ (2fr, scrolls)       │ (1fr)        │ (1fr, can combine)  │   │
│ │ ──────────────────── │ ──────────── │ ─────────────────── │   │
│ │ [Turn Card 1]        │ Read: 47     │ Task invocations: 7 │   │
│ │  ┌ User: "Fix bug"   │ Write: 12    │                     │   │
│ │  ├ Thinking (fold)   │ Edit: 8      │ Top agents:         │   │
│ │  ├ Response          │ Bash: 23     │ • Explore (4x)      │   │
│ │  ├ Tools (expand):   │ Grep: 15     │ • Plan (2x)         │   │
│ │  │  • Read auth.py   │              │ • Test (1x)         │   │
│ │  │  • Bash: go test  │ Top Files:   │                     │   │
│ │  └ 5.2s · 8.5K tok   │ 1. learner   │ Recent Tasks:       │   │
│ │                      │ 2. server    │ • "Run tests" (2m)  │   │
│ │ [Turn Card 2]        │ 3. store     │ • "Fix bug" (5m)    │   │
│ │ ...                  │              │                     │   │
│ │                      │ Top Patterns:│                     │   │
│ │                      │ 1. autotune  │                     │   │
│ │                      │ 2. DecayRate │                     │   │
│ └──────────────────────┴──────────────┴─────────────────────┘   │
│                                                                   │
│ ┌──────────────────────────────────────────────────────────────┐ │
│ │ Token Economics (Period Breakdown)                            │ │
│ │ Today | 7 days | 30 days | All-time                          │ │
│ └──────────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────────────┘
```

---

## Data Model: The Turn

### Turn Structure (Canonical Representation)

A **turn** is the fundamental unit of conversation. It groups:
1. User input (1 event)
2. AI thinking (0-1 event, optional)
3. AI response (1 event)
4. Tool invocations (0-N events)
5. System metadata (0-1 event, timing)

**All events share a common `TurnID`.**

```go
type ConversationTurn struct {
    // Identity
    TurnID       string    `json:"turn_id"`
    SessionID    string    `json:"session_id"`
    Timestamp    time.Time `json:"timestamp"`

    // Content
    UserPrompt   string    `json:"user_prompt"`
    Thinking     string    `json:"thinking,omitempty"`
    Response     string    `json:"response"`

    // Tools
    Tools        []ToolSummary `json:"tools,omitempty"`

    // Metrics
    TokenUsage   *TokenUsage `json:"token_usage,omitempty"`
    DurationMs   int         `json:"duration_ms,omitempty"`
    Model        string      `json:"model"`
    AgentVersion string      `json:"agent_version,omitempty"`
}

type ToolSummary struct {
    Name      string `json:"name"`        // Read, Write, Edit, Bash, Grep, Glob, Task
    Target    string `json:"target"`      // File path OR command OR pattern
    Impact    string `json:"impact"`      // "↓94% (40K→2.4K)" OR "7 hits" OR "exit 0"
    Timestamp string `json:"timestamp"`   // Relative: "2s ago"
}
```

### Source Data Mapping

| Turn Field | SessionEvent Source | Processing |
|------------|---------------------|------------|
| `TurnID` | `ev.TurnID` (same across all events in turn) | Group events |
| `UserPrompt` | `EventUserInput.Text` | Extract from Kind==UserInput |
| `Thinking` | `EventAIThinking.Text` | Extract from Kind==AIThinking |
| `Response` | `EventAIResponse.Text` | Extract from Kind==AIResponse |
| `Tools` | `EventToolInvocation.Tool + File` | Build ToolSummary from each tool event |
| `TokenUsage` | `EventAIResponse.Usage` | Extract from AIResponse |
| `DurationMs` | `EventSystemMeta.DurationMs` | Extract from SystemMeta linked by TurnID |
| `Model` | `EventAIResponse.Model` | From AI response event |
| `Timestamp` | `EventUserInput.Timestamp` OR first event timestamp | Use user input time as turn time |

---

## Implementation: Turn Buffer in App

### Step 1: Add Turn Accumulator

```go
// In internal/app/app.go, App struct:

type App struct {
    // ... existing fields ...

    // Conversation tracking
    turnBuffer      map[string]*TurnBuilder  // TurnID → partial turn
    conversationRing []ConversationTurn       // Last 50 completed turns
    ringIndex       int                       // Ring buffer write position
}

type TurnBuilder struct {
    TurnID       string
    SessionID    string
    Timestamp    time.Time
    UserPrompt   string
    Thinking     string
    Response     string
    Tools        []ToolSummary
    TokenUsage   *ports.TokenUsage
    DurationMs   int
    Model        string
    AgentVersion string
    Complete     bool  // Has both UserInput and AIResponse
}
```

### Step 2: Populate in onSessionEvent

```go
func (a *App) onSessionEvent(ev ports.SessionEvent) {
    a.mu.Lock()
    defer a.mu.Unlock()

    // Get or create turn builder
    builder := a.turnBuffer[ev.TurnID]
    if builder == nil {
        builder = &TurnBuilder{
            TurnID:    ev.TurnID,
            SessionID: ev.SessionID,
            Timestamp: ev.Timestamp,
        }
        a.turnBuffer[ev.TurnID] = builder
    }

    switch ev.Kind {
    case ports.EventUserInput:
        a.promptN++
        a.Learner.ProcessBigrams(ev.Text)

        // NEW: Capture user prompt
        builder.UserPrompt = ev.Text
        builder.Timestamp = ev.Timestamp
        a.writeStatus(nil)

    case ports.EventAIThinking:
        a.Learner.ProcessBigrams(ev.Text)

        // NEW: Capture thinking
        builder.Thinking = ev.Text
        builder.Model = ev.Model
        builder.AgentVersion = ev.AgentVersion

    case ports.EventAIResponse:
        if ev.Text != "" {
            a.Learner.ProcessBigrams(ev.Text)
        }

        // NEW: Capture response and token usage
        builder.Response = ev.Text
        builder.Model = ev.Model
        builder.AgentVersion = ev.AgentVersion
        if ev.Usage != nil {
            builder.TokenUsage = ev.Usage
        }
        builder.Complete = true  // Turn is ready

    case ports.EventToolInvocation:
        // Existing: File read observation
        if ev.File != nil && ev.File.Action == "read" &&
            ev.File.Limit > 0 && ev.File.Limit < 500 {
            a.Learner.Observe(learner.ObserveEvent{
                PromptNumber: a.promptN,
                FileRead: &learner.FileRead{File: ev.File.Path},
            })
        }

        // NEW: Capture tool summary
        tool := ToolSummary{
            Name:   ev.Tool.Name,
            Target: formatToolTarget(ev),
            Impact: formatToolImpact(ev),
        }
        builder.Tools = append(builder.Tools, tool)

    case ports.EventSystemMeta:
        // NEW: Capture turn duration
        if ev.DurationMs > 0 {
            builder.DurationMs = ev.DurationMs
        }
    }

    // Flush complete turns to ring buffer
    if builder.Complete && len(builder.UserPrompt) > 0 {
        a.flushTurnToRing(builder)
        delete(a.turnBuffer, ev.TurnID)
    }
}
```

### Step 3: Helper Functions

```go
func formatToolTarget(ev ports.SessionEvent) string {
    if ev.File != nil {
        if ev.File.Offset > 0 || ev.File.Limit > 0 {
            return fmt.Sprintf("%s:%d-%d",
                filepath.Base(ev.File.Path),
                ev.File.Offset,
                ev.File.Offset + ev.File.Limit)
        }
        return filepath.Base(ev.File.Path)
    }
    if ev.Tool != nil {
        if ev.Tool.Command != "" {
            return ev.Tool.Command
        }
        if ev.Tool.Pattern != "" {
            return ev.Tool.Pattern
        }
    }
    return ev.Tool.Name
}

func formatToolImpact(ev ports.SessionEvent) string {
    switch ev.Tool.Name {
    case "Read":
        if ev.File != nil && ev.File.Limit > 0 && ev.File.Limit < 500 {
            // Guided read — estimate savings
            // (requires file size metadata)
            return "guided"
        }
        return "full file"
    case "Bash":
        return "executed"
    case "Grep", "Glob":
        return "searching..."  // Could populate from search results
    default:
        return ""
    }
}

func (a *App) flushTurnToRing(builder *TurnBuilder) {
    turn := ConversationTurn{
        TurnID:       builder.TurnID,
        SessionID:    builder.SessionID,
        Timestamp:    builder.Timestamp,
        UserPrompt:   builder.UserPrompt,
        Thinking:     builder.Thinking,
        Response:     builder.Response,
        Tools:        builder.Tools,
        TokenUsage:   builder.TokenUsage,
        DurationMs:   builder.DurationMs,
        Model:        builder.Model,
        AgentVersion: builder.AgentVersion,
    }

    // Ring buffer (FIFO, cap at 50)
    if len(a.conversationRing) < 50 {
        a.conversationRing = append(a.conversationRing, turn)
    } else {
        a.conversationRing[a.ringIndex] = turn
        a.ringIndex = (a.ringIndex + 1) % 50
    }
}
```

---

## API Endpoints for Conversation Tab

### `/api/conversation/feed` (New)

**Handler:**
```go
func (s *Server) handleConversationFeed(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    turns := s.app.ConversationTurns()  // Returns []ConversationTurn

    respondJSON(w, map[string]any{
        "turns": turns,
        "count": len(turns),
    })
}
```

**App method:**
```go
func (a *App) ConversationTurns() []ConversationTurn {
    a.mu.Lock()
    defer a.mu.Unlock()

    // Return copy of ring buffer
    result := make([]ConversationTurn, len(a.conversationRing))
    copy(result, a.conversationRing)

    // Sort by timestamp descending (newest first)
    sort.Slice(result, func(i, j int) bool {
        return result[i].Timestamp.After(result[j].Timestamp)
    })

    return result
}
```

---

### `/api/conversation/tools` (New)

**Response:**
```json
{
  "read_count": 47,
  "write_count": 12,
  "edit_count": 8,
  "bash_count": 23,
  "grep_count": 15,
  "glob_count": 4,
  "task_count": 7,
  "total_tools": 116,
  "guided_read_pct": 100.0,
  "top_files": [
    {"path": "internal/domain/learner/learner.go", "reads": 12, "edits": 2},
    {"path": "internal/adapters/socket/server.go", "reads": 8, "edits": 1}
  ],
  "top_bash_commands": [
    {"command": "go test ./...", "count": 5},
    {"command": "go build ./cmd/aoa/", "count": 4}
  ],
  "top_search_patterns": [
    {"pattern": "autotune.*threshold", "count": 3},
    {"pattern": "DecayRate", "count": 2}
  ]
}
```

**App accumulator:**
```go
type ToolMetrics struct {
    ReadCount   int
    WriteCount  int
    EditCount   int
    BashCount   int
    GrepCount   int
    GlobCount   int
    TaskCount   int
    SkillCount  int

    GuidedReadCount   int  // Reads with 0 < limit < 500
    UnfocusedReadCount int  // Reads with limit == 0 or >= 500

    FileReads  map[string]int  // Path → read count
    FileWrites map[string]int  // Path → write count
    FileEdits  map[string]int  // Path → edit count

    BashCommands   map[string]int  // Command → count
    SearchPatterns map[string]int  // Pattern → count
    SkillNames     map[string]int  // Skill → count
}
```

**Population in onSessionEvent:**
```go
case ports.EventToolInvocation:
    if ev.Tool == nil {
        break
    }

    switch ev.Tool.Name {
    case "Read":
        a.toolMetrics.ReadCount++
        if ev.File != nil {
            a.toolMetrics.FileReads[ev.File.Path]++

            // Guided vs unfocused
            if ev.File.Limit > 0 && ev.File.Limit < 500 {
                a.toolMetrics.GuidedReadCount++
            } else {
                a.toolMetrics.UnfocusedReadCount++
            }
        }

    case "Write":
        a.toolMetrics.WriteCount++
        if ev.File != nil {
            a.toolMetrics.FileWrites[ev.File.Path]++
        }

    case "Edit", "NotebookEdit":
        a.toolMetrics.EditCount++
        if ev.File != nil {
            a.toolMetrics.FileEdits[ev.File.Path]++
        }

    case "Bash":
        a.toolMetrics.BashCount++
        if ev.Tool.Command != "" {
            a.toolMetrics.BashCommands[ev.Tool.Command]++
        }

    case "Grep", "Glob":
        if ev.Tool.Name == "Grep" {
            a.toolMetrics.GrepCount++
        } else {
            a.toolMetrics.GlobCount++
        }
        if ev.Tool.Pattern != "" {
            a.toolMetrics.SearchPatterns[ev.Tool.Pattern]++
        }

    case "Task":
        a.toolMetrics.TaskCount++

    case "Skill":
        a.toolMetrics.SkillCount++
        // Extract skill name from input if needed
    }
```

---

### `/api/conversation/metrics` (New)

**Response:**
```json
{
  "session_id": "session-abc",
  "session_start": "2026-02-17T09:00:00Z",
  "duration_s": 7820,
  "turns_count": 142,

  "input_tokens": 1820000,
  "output_tokens": 412000,
  "cache_read_tokens": 14600000,
  "cache_write_tokens": 280000,

  "cache_hit_rate": 0.874,
  "effective_cost_ratio": 0.143,

  "turns_with_usage": 140,
  "avg_input_per_turn": 13000,
  "avg_output_per_turn": 2943,

  "model_breakdown": {
    "claude-opus-4-6": {
      "turns": 120,
      "output_tokens": 380000,
      "avg_tok_per_sec": 38.4
    },
    "claude-sonnet-4": {
      "turns": 22,
      "output_tokens": 32000,
      "avg_tok_per_sec": 82.1
    }
  }
}
```

**App accumulator:**
```go
type SessionMetrics struct {
    SessionID    string
    StartTime    time.Time

    // Token totals
    InputTokens      int64
    OutputTokens     int64
    CacheReadTokens  int64
    CacheWriteTokens int64

    // Turn tracking
    TurnsCount       int
    TurnsWithUsage   int

    // Per-model tracking
    ModelStats map[string]*ModelMetrics
}

type ModelMetrics struct {
    Model        string
    TurnsCount   int
    OutputTokens int64
    TotalDurationMs int64
}
```

**Velocity calculation:**
```go
func (m *ModelMetrics) AvgTokensPerSecond() float64 {
    if m.TotalDurationMs == 0 {
        return 0
    }
    return float64(m.OutputTokens) / (float64(m.TotalDurationMs) / 1000.0)
}
```

---

## Dashboard Implementation

### Conversation Feed Card (Left Column, 2fr)

**HTML Structure:**
```html
<div class="card conversation-feed">
  <div class="card-header">
    <div class="card-title">Conversation</div>
    <span class="badge badge-live">live</span>
  </div>
  <div class="feed-body" id="conversationFeed">
    <!-- Turn cards inserted here via JS -->
  </div>
</div>
```

**Turn Card Template:**
```html
<div class="turn-card" data-turn-id="{turnID}">
  <div class="turn-header">
    <div class="turn-icon user">U</div>
    <div class="turn-text">{userPrompt}</div>
    <div class="turn-time">{timestamp}</div>
  </div>

  <!-- Thinking (collapsible) -->
  <div class="turn-thinking collapsed">
    <div class="thinking-toggle">💭 Show thinking</div>
    <div class="thinking-content">{thinking}</div>
  </div>

  <!-- Response -->
  <div class="turn-response">
    <div class="turn-icon ai">A</div>
    <div class="turn-text">{response}</div>
  </div>

  <!-- Tools (expandable) -->
  <div class="turn-tools">
    <div class="tools-summary">
      {toolCount} tools · {durationMs}ms
      <span class="expand-icon">▼</span>
    </div>
    <div class="tools-list collapsed">
      <div class="tool-item">
        <span class="tool-pill read">Read</span>
        <span class="tool-target">learner.go:142-287</span>
        <span class="tool-impact">↓94%</span>
      </div>
      <div class="tool-item">
        <span class="tool-pill bash">Bash</span>
        <span class="tool-target">go test ./...</span>
        <span class="tool-impact">exit 0</span>
      </div>
    </div>
  </div>

  <!-- Token footer -->
  <div class="turn-footer">
    <span class="model-badge">{model}</span>
    <span class="token-usage">{inputTokens}i + {outputTokens}o + {cacheReadTokens}c</span>
    <span class="cache-hit">{cacheHitRate}% cache</span>
  </div>
</div>
```

**JS Rendering:**
```js
function renderConversationFeed(data) {
  var container = document.getElementById('conversationFeed');
  container.innerHTML = '';

  (data.turns || []).forEach(function(turn) {
    var card = createTurnCard(turn);
    container.appendChild(card);
  });
}

function createTurnCard(turn) {
  var card = document.createElement('div');
  card.className = 'turn-card';
  card.dataset.turnId = turn.turn_id;

  // Build HTML from template with turn data
  // Add click handlers for collapsible sections

  return card;
}
```

---

### Tools Panel (Right Column, 1fr)

**HTML Structure:**
```html
<div class="card tools-panel">
  <div class="card-header">
    <div class="card-title">Tools &amp; Agents</div>
  </div>
  <div class="tools-body">
    <!-- Tool distribution pie/bar chart -->
    <div class="tool-dist">
      <div class="dist-item">
        <span class="dist-label">Read</span>
        <div class="dist-bar">
          <div class="dist-fill green" style="width:40%"></div>
        </div>
        <span class="dist-count">47</span>
      </div>
      <div class="dist-item">
        <span class="dist-label">Bash</span>
        <div class="dist-bar">
          <div class="dist-fill yellow" style="width:20%"></div>
        </div>
        <span class="dist-count">23</span>
      </div>
      <!-- ... -->
    </div>

    <!-- Top files -->
    <div class="tool-section">
      <div class="section-title">Top Files</div>
      <div class="top-list">
        <div class="top-item">
          <span class="rank">1</span>
          <span class="name mono">learner.go</span>
          <span class="count">12</span>
        </div>
        <!-- ... -->
      </div>
    </div>

    <!-- Top bash commands -->
    <div class="tool-section">
      <div class="section-title">Top Commands</div>
      <div class="top-list">
        <div class="top-item">
          <span class="rank">1</span>
          <span class="name mono">go test ./...</span>
          <span class="count">5</span>
        </div>
        <!-- ... -->
      </div>
    </div>
  </div>
</div>
```

**JS Rendering:**
```js
function renderToolsPanel(data) {
  // Update tool distribution bars
  var total = data.total_tools || 1;
  updateToolBar('read', data.read_count, total);
  updateToolBar('write', data.write_count, total);
  updateToolBar('bash', data.bash_count, total);
  // ...

  // Render top files list
  renderTopList('topFilesList', data.top_files, 'path', 'reads');

  // Render top bash commands
  renderTopList('topBashList', data.top_bash_commands, 'command', 'count');
}
```

---

## Token Economics Panel (Full Width)

**HTML Structure:**
```html
<div class="card token-economics">
  <div class="card-header">
    <div class="card-title">Token Economics</div>
    <div class="period-tabs">
      <button class="period-tab active" data-period="today">Today</button>
      <button class="period-tab" data-period="7d">7 Days</button>
      <button class="period-tab" data-period="30d">30 Days</button>
    </div>
  </div>
  <div class="econ-body">
    <table class="econ-table">
      <thead>
        <tr>
          <th></th>
          <th>Input</th>
          <th>Output</th>
          <th>Cache Read</th>
          <th>Cache Write</th>
          <th>Total</th>
          <th>Cost Est.</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>Opus</td>
          <td class="mono">{opusInput}</td>
          <td class="mono">{opusOutput}</td>
          <td class="mono green">{opusCacheRead}</td>
          <td class="mono yellow">{opusCacheWrite}</td>
          <td class="mono">{opusTotal}</td>
          <td class="mono">${opusCost}</td>
        </tr>
        <tr>
          <td>Sonnet</td>
          <td class="mono">{sonnetInput}</td>
          <td class="mono">{sonnetOutput}</td>
          <td class="mono green">{sonnetCacheRead}</td>
          <td class="mono yellow">{sonnetCacheWrite}</td>
          <td class="mono">{sonnetTotal}</td>
          <td class="mono">${sonnetCost}</td>
        </tr>
        <tr class="total-row">
          <td><strong>Total</strong></td>
          <td class="mono"><strong>{totalInput}</strong></td>
          <td class="mono"><strong>{totalOutput}</strong></td>
          <td class="mono green"><strong>{totalCacheRead}</strong></td>
          <td class="mono yellow"><strong>{totalCacheWrite}</strong></td>
          <td class="mono"><strong>{grandTotal}</strong></td>
          <td class="mono"><strong>${totalCost}</strong></td>
        </tr>
      </tbody>
    </table>
  </div>
</div>
```

**Data source:** `/api/conversation/metrics` with period parameter: `?period=today|7d|30d`

**Period accumulation:** Requires bbolt persistence of daily rollups.

---

## Agents Column (Optional 3rd Column)

If Task/Skill tools are used heavily, add a 3rd column showing:

```html
<div class="card agents-panel">
  <div class="card-header">
    <div class="card-title">Agents</div>
  </div>
  <div class="agents-body">
    <div class="agent-stat">
      <div class="stat-value">7</div>
      <div class="stat-label">Task invocations</div>
    </div>

    <div class="agent-section">
      <div class="section-title">Top Agents</div>
      <div class="agent-list">
        <div class="agent-item">
          <span class="agent-name">Explore</span>
          <div class="agent-bar">
            <div class="agent-fill cyan" style="width:60%"></div>
          </div>
          <span class="agent-count">4</span>
        </div>
        <div class="agent-item">
          <span class="agent-name">Plan</span>
          <div class="agent-bar">
            <div class="agent-fill purple" style="width:30%"></div>
          </div>
          <span class="agent-count">2</span>
        </div>
      </div>
    </div>

    <div class="agent-section">
      <div class="section-title">Recent Tasks</div>
      <div class="task-list">
        <div class="task-item">
          <div class="task-desc">"Run integration tests"</div>
          <div class="task-time">2m ago</div>
        </div>
        <div class="task-item">
          <div class="task-desc">"Fix daemon shutdown bug"</div>
          <div class="task-time">15m ago</div>
        </div>
      </div>
    </div>
  </div>
</div>
```

**Data:** Extract from `Task` tool invocations:
- `Tool.Input["description"]` → task description
- `Tool.Input["subagent_type"]` → agent name (Explore, Plan, Test, etc.)

---

## Implementation Phases

### Phase A: Token Metrics (4-6 hours)
1. Add `SessionMetrics` struct to App
2. Increment in `onSessionEvent` for EventAIResponse with Usage
3. Add `/api/conversation/metrics` endpoint
4. Wire 5 stat cards in Conversation tab

**Result:** Input/Output/Cache Read/Cache Write/Cache Hit % all show real numbers.

---

### Phase B: Tool Metrics (4-6 hours)
1. Add `ToolMetrics` struct to App
2. Increment in `onSessionEvent` for all EventToolInvocation by tool type
3. Accumulate top files, bash commands, search patterns (truncate at 50 each)
4. Add `/api/conversation/tools` endpoint
5. Build Tools panel with distribution bars + top lists

**Result:** Tools & Agents panel shows real Read/Write/Bash counts, top files, top commands.

---

### Phase C: Conversation Feed (8-12 hours)
1. Add `TurnBuilder` buffering in `onSessionEvent`
2. Build `ConversationTurn` from grouped events
3. Ring buffer last 50 turns
4. Add `/api/conversation/feed` endpoint
5. Render turn cards in dashboard with collapsible sections

**Result:** Live conversation feed with user prompts, AI responses, thinking, tools.

---

### Phase D: Velocity & Economics (8-12 hours)
1. Link DurationMs to Usage via TurnID buffering
2. Compute tok/s per model
3. Add model breakdown to SessionMetrics
4. Create period rollups (daily/weekly/monthly) with bbolt persistence
5. Add `/api/conversation/velocity` and `/api/conversation/periods` endpoints
6. Build Token Economics panel with period tabs

**Result:** Opus/Sonnet tok/s cards show real velocity. Token Economics table shows period breakdown.

---

## CSS Classes & Styling

### Turn Cards
```css
.turn-card {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 12px;
  background: var(--card);
}

.turn-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.turn-icon {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 12px;
}
.turn-icon.user { background: var(--cyan-dim); color: var(--cyan); }
.turn-icon.ai { background: var(--purple-dim); color: var(--purple); }

.turn-thinking {
  margin: 8px 0;
  padding: 10px 12px;
  background: var(--yellow-dim);
  border-left: 2px solid var(--yellow);
  border-radius: 6px;
  font-size: 12px;
  color: var(--dim);
  font-style: italic;
}
.turn-thinking.collapsed .thinking-content { display: none; }

.turn-tools {
  margin-top: 10px;
  padding: 8px 0;
  border-top: 1px solid var(--border-subtle);
}

.tool-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
  font-size: 12px;
}

.tool-pill {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 5px;
  font-family: var(--font-mono);
}
.tool-pill.read { background: var(--green-dim); color: var(--green); }
.tool-pill.bash { background: var(--yellow-dim); color: var(--yellow); }
.tool-pill.grep { background: var(--cyan-dim); color: var(--cyan); }

.turn-footer {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--border-subtle);
  display: flex;
  gap: 12px;
  font-size: 11px;
  color: var(--mute);
}

.model-badge {
  font-family: var(--font-mono);
  padding: 2px 6px;
  background: var(--purple-dim);
  color: var(--purple);
  border-radius: 4px;
}

.token-usage {
  font-family: var(--font-mono);
}
```

---

## Data Persistence Strategy

### In-Memory (App struct)
- `conversationRing` — Last 50 turns (no persistence)
- `sessionMetrics` — Current session totals (reset on restart)
- `toolMetrics` — Current session tool counts (reset on restart)
- `turnBuffer` — Partial turns awaiting completion (transient)

### bbolt (Persisted)
- `metrics:{projectID}:session:{sessionID}` — Per-session totals (if needed for multi-session analysis)
- `metrics:{projectID}:daily:{YYYY-MM-DD}` — Daily rollups (for 7d/30d queries)
- `metrics:{projectID}:weekly:{YYYY-Www}` — Weekly rollups (optional)

### Reset Behavior
- Session metrics reset on: `aoa wipe`, daemon restart
- Period rollups persist across restarts (queried from bbolt)

---

## Summary

**What we need for Conversation tab:**
1. **Turn buffer** — Group SessionEvents by TurnID → ConversationTurn structs
2. **Token accumulator** — Sum Usage fields → SessionMetrics
3. **Tool accumulator** — Count tool invocations → ToolMetrics
4. **3 new API endpoints** — `/feed`, `/metrics`, `/tools`
5. **Dashboard JS rendering** — Turn cards, tool charts, token tables

**All data is already captured.** No new session parsing needed. Just accumulation + API + UI.

**Estimated effort:** 16-30 hours for full implementation (Phases A-D).
