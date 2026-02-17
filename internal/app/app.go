// Package app wires together all adapters and domain logic.
// It provides lifecycle management for the aOa daemon: create, start, stop.
package app

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/corey/aoa/atlas"
	"github.com/corey/aoa/internal/adapters/bbolt"
	claude "github.com/corey/aoa/internal/adapters/claude"
	fsw "github.com/corey/aoa/internal/adapters/fsnotify"
	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/adapters/web"
	"github.com/corey/aoa/internal/domain/enricher"
	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/domain/learner"
	"github.com/corey/aoa/internal/domain/status"
	"github.com/corey/aoa/internal/ports"
)

// SessionMetrics holds aggregated token and turn counters from session events.
type SessionMetrics struct {
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	TurnCount        int
}

// CacheHitRate returns the percentage of cache read tokens vs total input tokens.
func (s *SessionMetrics) CacheHitRate() float64 {
	total := s.InputTokens + s.CacheReadTokens
	if total == 0 {
		return 0.0
	}
	return float64(s.CacheReadTokens) / float64(total) * 100.0
}

// ToolMetrics holds per-tool counters and top targets.
type ToolMetrics struct {
	ReadCount    int
	WriteCount   int
	EditCount    int
	BashCount    int
	GrepCount    int
	GlobCount    int
	OtherCount   int
	FileReads    map[string]int // file path -> count
	BashCommands map[string]int // command -> count
	GrepPatterns map[string]int // pattern -> count
}

// TurnAction describes a single tool action within a conversation turn.
type TurnAction struct {
	Tool   string // "Read", "Edit", "Bash", "Grep", "Glob", "Task", etc.
	Target string // relative path, command, or pattern
	Range  string // ":offset-limit" for reads, empty otherwise
	Impact string // "↓62%", "4 match", "309 pass", "1 fail", etc.
}

// ConversationTurn describes a single turn in the conversation feed.
type ConversationTurn struct {
	TurnID       string
	Role         string // "user" or "assistant"
	Text         string // truncated to 500 chars
	ThinkingText string // truncated to 500 chars
	DurationMs   int
	ToolNames    []string
	Actions      []TurnAction
	Timestamp    int64
	Model        string
	InputTokens  int
	OutputTokens int
}

// ActivityEntry describes a single action for the activity ring buffer.
type ActivityEntry struct {
	Action    string
	Source    string
	Attrib    string
	Impact    string
	Tags      string
	Target    string
	Timestamp int64
}

// turnBuilder is a staging area for in-progress AI turns.
type turnBuilder struct {
	TurnID       string
	Timestamp    int64
	Model        string
	Text         strings.Builder
	ThinkingText strings.Builder
	ToolNames    []string
	Actions      []TurnAction
	DurationMs   int
	InputTokens  int
	OutputTokens int
}

// App is the top-level container wiring all components together.
type App struct {
	ProjectRoot string
	ProjectID   string

	Store     *bbolt.Store
	Watcher   *fsw.Watcher
	Enricher  *enricher.Enricher
	Engine    *index.SearchEngine
	Learner   *learner.Learner
	Server    *socket.Server
	WebServer *web.Server
	Reader    *claude.Reader
	Index     *ports.Index

	mu             sync.Mutex             // serializes learner access (searches are concurrent)
	promptN        uint32                 // prompt counter (incremented on each user input)
	lastAutotune   *learner.AutotuneResult // most recent autotune result (for status line)
	statusLinePath string                 // project-local path for status line file
	httpPort       int                    // preferred HTTP port (0 = auto from project root)

	// Session metrics accumulators (ephemeral, reset on daemon restart)
	sessionMetrics SessionMetrics
	toolMetrics    ToolMetrics
	convRing       [50]ConversationTurn   // ring buffer of last 50 turns
	convHead       int                    // write index for ring buffer
	convCount      int                    // fill count (0-50)
	turnBuffer     map[string]*turnBuilder // partial AI turns keyed by TurnID
	currentBuilder *turnBuilder            // active exchange builder (all assistant events between user inputs)

	// Activity feed (separate ring buffer for tool actions)
	activityRing  [50]ActivityEntry // ring buffer of last 50 activity entries
	activityHead  int
	activityCount int
	guidedPaths   map[string]int64 // file path -> unix timestamp of last aOa search hit
}

// Config holds initialization parameters for the App.
type Config struct {
	ProjectRoot string
	ProjectID   string
	DBPath      string // path to bbolt file (default: .aoa/aoa.db)
	HTTPPort    int    // preferred HTTP port (default: computed from project root)
}

// New creates an App with all dependencies wired. Does not start services.
func New(cfg Config) (*App, error) {
	if cfg.ProjectRoot == "" {
		return nil, fmt.Errorf("project root required")
	}
	if cfg.ProjectID == "" {
		cfg.ProjectID = filepath.Base(cfg.ProjectRoot)
	}
	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join(cfg.ProjectRoot, ".aoa", "aoa.db")
	}

	store, err := bbolt.NewStore(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	watcher, err := fsw.NewWatcher()
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	// Load existing index or create empty
	idx, err := store.LoadIndex(cfg.ProjectID)
	if err != nil {
		store.Close()
		watcher.Stop()
		return nil, fmt.Errorf("load index: %w", err)
	}
	if idx == nil {
		idx = &ports.Index{
			Tokens:   make(map[string][]ports.TokenRef),
			Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
			Files:    make(map[uint32]*ports.FileMeta),
		}
	}

	// Load universal domains from embedded atlas
	enr, err := enricher.NewFromFS(atlas.FS, "v1")
	if err != nil {
		store.Close()
		watcher.Stop()
		return nil, fmt.Errorf("load atlas: %w", err)
	}

	// Convert atlas domain defs to search engine domain map
	domains := make(map[string]index.Domain, len(enr.DomainDefs()))
	for _, d := range enr.DomainDefs() {
		domains[d.Domain] = index.Domain{Terms: d.Terms}
	}

	engine := index.NewSearchEngine(idx, domains)

	// Load existing learner state or create fresh
	ls, err := store.LoadLearnerState(cfg.ProjectID)
	if err != nil {
		store.Close()
		watcher.Stop()
		return nil, fmt.Errorf("load learner state: %w", err)
	}
	var lrn *learner.Learner
	if ls != nil {
		lrn = learner.NewFromState(ls)
	} else {
		lrn = learner.New()
	}

	reader := claude.New(claude.Config{
		ProjectRoot: cfg.ProjectRoot,
	})

	// Status line goes alongside the DB in .aoa/
	statusPath := filepath.Join(cfg.ProjectRoot, ".aoa", status.StatusFile)

	a := &App{
		ProjectRoot:    cfg.ProjectRoot,
		ProjectID:      cfg.ProjectID,
		Store:          store,
		Watcher:        watcher,
		Enricher:       enr,
		Engine:         engine,
		Learner:        lrn,
		Reader:         reader,
		Index:          idx,
		promptN:        lrn.PromptCount(),
		statusLinePath: statusPath,
		httpPort:       cfg.HTTPPort,
		toolMetrics: ToolMetrics{
			FileReads:    make(map[string]int),
			BashCommands: make(map[string]int),
			GrepPatterns: make(map[string]int),
		},
		turnBuffer:  make(map[string]*turnBuilder),
		guidedPaths: make(map[string]int64),
	}

	// Create server with App as query provider (for domains, stats, etc.)
	sockPath := socket.SocketPath(cfg.ProjectRoot)
	a.Server = socket.NewServer(engine, idx, sockPath, a)

	// Create HTTP server for web dashboard
	httpPortFile := filepath.Join(cfg.ProjectRoot, ".aoa", "http.port")
	a.WebServer = web.NewServer(a, idx, engine, httpPortFile)

	// Wire search observer: search results → learning signals
	engine.SetObserver(a.searchObserver)

	return a, nil
}

// searchObserver extracts learning signals from search queries and results.
// Called after every search. Thread-safe via mutex.
func (a *App) searchObserver(query string, opts ports.SearchOptions, result *index.SearchResult, elapsed time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.promptN++

	// Tokenize query into keywords
	tokens := index.Tokenize(query)
	if len(tokens) == 0 {
		return
	}

	var keywords []string
	var terms []string
	var domains []string
	var kwTerms [][2]string
	var termDomains [][2]string

	seenTerms := make(map[string]bool)
	seenDomains := make(map[string]bool)

	// Resolve each query token via the atlas enricher
	for _, tok := range tokens {
		keywords = append(keywords, tok)
		matches := a.Enricher.Lookup(tok)
		for _, m := range matches {
			kwTerms = append(kwTerms, [2]string{tok, m.Term})
			termDomains = append(termDomains, [2]string{m.Term, m.Domain})
			if !seenTerms[m.Term] {
				seenTerms[m.Term] = true
				terms = append(terms, m.Term)
			}
			if !seenDomains[m.Domain] {
				seenDomains[m.Domain] = true
				domains = append(domains, m.Domain)
			}
		}
	}

	// Collect domains from result hits (file-level domains)
	for _, hit := range result.Hits {
		d := strings.TrimPrefix(hit.Domain, "@")
		if d != "" && !seenDomains[d] {
			seenDomains[d] = true
			domains = append(domains, d)
		}
	}

	event := learner.ObserveEvent{
		PromptNumber: a.promptN,
		Observe: learner.ObserveData{
			Keywords:     keywords,
			Terms:        terms,
			Domains:      domains,
			KeywordTerms: kwTerms,
			TermDomains:  termDomains,
		},
	}

	tuneResult := a.Learner.ObserveAndMaybeTune(event)
	if tuneResult != nil {
		// Persist state after autotune cycle
		_ = a.Store.SaveLearnerState(a.ProjectID, a.Learner.State())
		a.writeStatus(tuneResult)
	}

	// Record activity entry for the search
	entry := ActivityEntry{
		Action:    "Search",
		Source:    "aOa",
		Attrib:    a.searchAttrib(query, opts),
		Impact:    fmt.Sprintf("%d hits | %.2fms", len(result.Hits), elapsed.Seconds()*1000),
		Target:    a.searchTarget(query, opts),
		Timestamp: time.Now().Unix(),
	}
	a.pushActivity(entry)

	// Cache result file paths in guidedPaths
	now := time.Now().Unix()
	for _, hit := range result.Hits {
		a.guidedPaths[hit.File] = now
	}

	// Prune stale guidedPaths entries (older than 5 minutes)
	for path, ts := range a.guidedPaths {
		if now-ts > 300 {
			delete(a.guidedPaths, path)
		}
	}
}

// Start begins the daemon (socket server + HTTP server + session reader).
func (a *App) Start() error {
	if err := a.Server.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}
	// Start HTTP dashboard — non-fatal if port unavailable
	httpPort := a.httpPort
	if httpPort == 0 {
		httpPort = web.DefaultPort(a.ProjectRoot)
	}
	if err := a.WebServer.Start(httpPort); err != nil {
		fmt.Printf("[warning] HTTP dashboard unavailable: %v\n", err)
	}
	// Start session reader — tails Claude session logs for learning signals
	a.Reader.Start(a.onSessionEvent)
	// Write initial status line
	a.mu.Lock()
	a.writeStatus(nil)
	a.mu.Unlock()
	return nil
}

// Stop gracefully shuts down all services and persists learner state.
func (a *App) Stop() error {
	a.Reader.Stop()
	a.Watcher.Stop()
	a.WebServer.Stop()
	a.Server.Stop()
	// Persist final learner state before shutdown
	a.mu.Lock()
	_ = a.Store.SaveLearnerState(a.ProjectID, a.Learner.State())
	a.mu.Unlock()
	a.Store.Close()
	return nil
}

// onSessionEvent processes canonical session events from the Claude adapter.
// Extracts bigrams from conversation text and file access signals from tools.
// Thread-safe via mutex.
func (a *App) onSessionEvent(ev ports.SessionEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ts := ev.Timestamp.Unix()
	if ts == 0 {
		ts = int64(a.promptN) // fallback if timestamp missing
	}

	switch ev.Kind {
	case ports.EventUserInput:
		// Flush the current exchange builder before starting new user input
		a.flushCurrentBuilder()
		a.promptN++
		a.sessionMetrics.TurnCount++
		a.Learner.ProcessBigrams(ev.Text)
		a.writeStatus(nil)
		// Push user turn to ring
		a.pushTurn(ConversationTurn{
			TurnID:    ev.TurnID,
			Role:      "user",
			Text:      a.truncate(ev.Text, 500),
			Timestamp: ts,
			Model:     ev.Model,
		})

	case ports.EventAIThinking:
		if ev.Text != "" {
			a.Learner.ProcessBigrams(ev.Text)
			tb := a.ensureTurnBuilder(ev.TurnID, ts, ev.Model)
			if tb.ThinkingText.Len() > 0 {
				tb.ThinkingText.WriteString(" ")
			}
			tb.ThinkingText.WriteString(ev.Text)
		}

	case ports.EventAIResponse:
		if ev.Text != "" {
			a.Learner.ProcessBigrams(ev.Text)
		}
		// Accumulate token usage (global + per-turn)
		tb := a.ensureTurnBuilder(ev.TurnID, ts, ev.Model)
		if ev.Usage != nil {
			a.sessionMetrics.InputTokens += ev.Usage.InputTokens
			a.sessionMetrics.OutputTokens += ev.Usage.OutputTokens
			a.sessionMetrics.CacheReadTokens += ev.Usage.CacheReadTokens
			a.sessionMetrics.CacheWriteTokens += ev.Usage.CacheWriteTokens
			tb.InputTokens += ev.Usage.InputTokens
			tb.OutputTokens += ev.Usage.OutputTokens
		}
		// Buffer AI response text
		if ev.Text != "" {
			if tb.Text.Len() > 0 {
				tb.Text.WriteString(" ")
			}
			tb.Text.WriteString(ev.Text)
		}

	case ports.EventToolInvocation:
		// Range gate: only focused file reads (limit > 0 && < 500)
		if ev.File != nil && ev.File.Action == "read" &&
			ev.File.Limit > 0 && ev.File.Limit < 500 {
			a.Learner.Observe(learner.ObserveEvent{
				PromptNumber: a.promptN,
				FileRead: &learner.FileRead{
					File: ev.File.Path,
				},
			})
		}
		// Accumulate tool metrics
		a.accumulateTool(ev)
		// Buffer tool name and action for the current turn
		if ev.Tool != nil {
			tb := a.ensureTurnBuilder(ev.TurnID, ts, ev.Model)
			tb.ToolNames = append(tb.ToolNames, ev.Tool.Name)

			// Build target, range, impact, and attrib for activity + turn action
			action := ev.Tool.Name
			attrib := "-"
			target := ""
			rangeStr := ""
			impact := ""

			switch action {
			case "Read":
				if ev.File != nil && ev.File.Path != "" {
					target = ev.File.Path
					if ev.File.Offset > 0 || ev.File.Limit > 0 {
						rangeStr = fmt.Sprintf(":%d-%d", ev.File.Offset, ev.File.Offset+ev.File.Limit)
						// Calculate savings from FileMeta.Size
						impact = a.readSavings(ev.File.Path, ev.File.Limit)
					}
					// Check if this read was guided by a recent aOa search
					if guidedTS, ok := a.guidedPaths[ev.File.Path]; ok {
						if time.Now().Unix()-guidedTS < 300 {
							attrib = "aOa guided"
							if impact == "" {
								impact = "guided"
							}
						}
					}
				}
			case "Bash":
				if ev.Tool.Command != "" {
					target = a.truncate(ev.Tool.Command, 100)
				}
			case "Grep":
				if ev.Tool.Pattern != "" {
					target = ev.Tool.Pattern
				}
			case "Write", "Edit":
				if ev.File != nil && ev.File.Path != "" {
					target = ev.File.Path
				}
			case "Glob":
				if ev.File != nil && ev.File.Path != "" {
					target = ev.File.Path
				} else if ev.Tool.Pattern != "" {
					target = ev.Tool.Pattern
				}
			case "Task":
				if ev.Tool.Command != "" {
					target = a.truncate(ev.Tool.Command, 100)
				}
			}

			// Append structured action to turn builder
			tb.Actions = append(tb.Actions, TurnAction{
				Tool:   action,
				Target: target,
				Range:  rangeStr,
				Impact: impact,
			})

			// Activity feed target includes range for reads
			actTarget := target
			if rangeStr != "" {
				actTarget = target + rangeStr
			}

			a.pushActivity(ActivityEntry{
				Action:    action,
				Source:    "claude",
				Attrib:    attrib,
				Impact:    impact,
				Target:    actTarget,
				Timestamp: time.Now().Unix(),
			})
		}

	case ports.EventSystemMeta:
		// Link DurationMs to turn builder
		if ev.DurationMs > 0 && ev.TurnID != "" {
			tb := a.ensureTurnBuilder(ev.TurnID, ts, ev.Model)
			tb.DurationMs = ev.DurationMs
		}
	}
}

// writeStatus generates and writes status JSON to the project-local file.
// Must be called with a.mu held.
func (a *App) writeStatus(autotune *learner.AutotuneResult) {
	if autotune != nil {
		a.lastAutotune = autotune
	}
	data := status.Generate(a.Learner.State(), a.lastAutotune)
	_ = status.WriteJSON(a.statusLinePath, data)
}

// pushActivity adds an entry to the activity ring buffer.
// Must be called with a.mu held.
func (a *App) pushActivity(entry ActivityEntry) {
	a.activityRing[a.activityHead] = entry
	a.activityHead = (a.activityHead + 1) % 50
	if a.activityCount < 50 {
		a.activityCount++
	}
}

// searchAttrib returns the attribution label for a search query.
// Must be called with a.mu held.
func (a *App) searchAttrib(query string, opts ports.SearchOptions) string {
	if opts.Mode == "regex" {
		return "regex"
	}
	if opts.AndMode {
		return "and"
	}
	tokens := index.Tokenize(query)
	if len(tokens) > 1 {
		return "multi-or"
	}
	return "indexed"
}

// searchTarget returns the target label for a search query.
func (a *App) searchTarget(query string, opts ports.SearchOptions) string {
	if opts.Mode == "regex" {
		return "aOa egrep " + query
	}
	return "aOa grep " + query
}

// ActivityFeed returns the most recent activity entries for the dashboard.
// Implements socket.AppQueries.
func (a *App) ActivityFeed() socket.ActivityFeedResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	var entries []socket.ActivityEntryResult

	if a.activityCount == 0 {
		return socket.ActivityFeedResult{Entries: entries, Count: 0}
	}

	limit := a.activityCount
	if limit > 20 {
		limit = 20
	}

	for i := 0; i < limit; i++ {
		idx := (a.activityHead - 1 - i + 50) % 50
		e := a.activityRing[idx]
		entries = append(entries, socket.ActivityEntryResult{
			Action:    e.Action,
			Source:    e.Source,
			Attrib:    e.Attrib,
			Impact:    e.Impact,
			Tags:      e.Tags,
			Target:    e.Target,
			Timestamp: e.Timestamp,
		})
	}

	return socket.ActivityFeedResult{
		Entries: entries,
		Count:   len(entries),
	}
}

// LearnerSnapshot returns a deep copy of the current learner state.
// Safe for concurrent use — the returned state is independent.
// Implements socket.AppQueries.
func (a *App) LearnerSnapshot() *ports.LearnerState {
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := json.Marshal(a.Learner.State())
	if err != nil {
		return &ports.LearnerState{
			KeywordHits:      make(map[string]uint32),
			TermHits:         make(map[string]uint32),
			DomainMeta:       make(map[string]*ports.DomainMeta),
			CohitKwTerm:      make(map[string]uint32),
			CohitTermDomain:  make(map[string]uint32),
			Bigrams:          make(map[string]uint32),
			FileHits:         make(map[string]uint32),
			KeywordBlocklist: make(map[string]bool),
			GapKeywords:      make(map[string]bool),
		}
	}
	var copy ports.LearnerState
	if err := json.Unmarshal(data, &copy); err != nil {
		return &ports.LearnerState{}
	}
	return &copy
}

// SessionMetricsSnapshot returns a snapshot of session token metrics.
// Implements socket.AppQueries.
func (a *App) SessionMetricsSnapshot() socket.SessionMetricsResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	return socket.SessionMetricsResult{
		InputTokens:      a.sessionMetrics.InputTokens,
		OutputTokens:     a.sessionMetrics.OutputTokens,
		CacheReadTokens:  a.sessionMetrics.CacheReadTokens,
		CacheWriteTokens: a.sessionMetrics.CacheWriteTokens,
		TurnCount:        a.sessionMetrics.TurnCount,
		CacheHitRate:     a.sessionMetrics.CacheHitRate(),
	}
}

// ToolMetricsSnapshot returns a snapshot of tool usage metrics.
// Implements socket.AppQueries.
func (a *App) ToolMetricsSnapshot() socket.ToolMetricsResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	total := a.toolMetrics.ReadCount + a.toolMetrics.WriteCount +
		a.toolMetrics.EditCount + a.toolMetrics.BashCount +
		a.toolMetrics.GrepCount + a.toolMetrics.GlobCount +
		a.toolMetrics.OtherCount

	return socket.ToolMetricsResult{
		ReadCount:    a.toolMetrics.ReadCount,
		WriteCount:   a.toolMetrics.WriteCount,
		EditCount:    a.toolMetrics.EditCount,
		BashCount:    a.toolMetrics.BashCount,
		GrepCount:    a.toolMetrics.GrepCount,
		GlobCount:    a.toolMetrics.GlobCount,
		OtherCount:   a.toolMetrics.OtherCount,
		TotalCount:   total,
		FileReads:    a.topN(a.toolMetrics.FileReads, 20),
		BashCommands: a.topN(a.toolMetrics.BashCommands, 20),
		GrepPatterns: a.topN(a.toolMetrics.GrepPatterns, 20),
	}
}

// ConversationTurns returns the last N turns from the ring buffer.
// Implements socket.AppQueries.
func (a *App) ConversationTurns() socket.ConversationFeedResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	var turns []socket.ConversationTurnResult

	// Copy ring buffer in reverse order (newest first)
	if a.convCount == 0 {
		return socket.ConversationFeedResult{Turns: turns, Count: 0}
	}

	// Include the in-progress exchange builder as the most recent turn
	if a.currentBuilder != nil {
		inProgress := a.turnFromBuilder(a.currentBuilder)
		var actions []socket.TurnActionResult
		for _, act := range inProgress.Actions {
			actions = append(actions, socket.TurnActionResult{
				Tool:   act.Tool,
				Target: act.Target,
				Range:  act.Range,
				Impact: act.Impact,
			})
		}
		turns = append(turns, socket.ConversationTurnResult{
			TurnID:       inProgress.TurnID,
			Role:         inProgress.Role,
			Text:         inProgress.Text,
			ThinkingText: inProgress.ThinkingText,
			DurationMs:   inProgress.DurationMs,
			ToolNames:    inProgress.ToolNames,
			Actions:      actions,
			Timestamp:    inProgress.Timestamp,
			Model:        inProgress.Model,
			InputTokens:  inProgress.InputTokens,
			OutputTokens: inProgress.OutputTokens,
		})
	}

	// Start from most recent (convHead - 1) and walk backwards
	for i := 0; i < a.convCount && i < 20; i++ {
		idx := (a.convHead - 1 - i + 50) % 50
		turn := a.convRing[idx]
		var actions []socket.TurnActionResult
		for _, act := range turn.Actions {
			actions = append(actions, socket.TurnActionResult{
				Tool:   act.Tool,
				Target: act.Target,
				Range:  act.Range,
				Impact: act.Impact,
			})
		}
		turns = append(turns, socket.ConversationTurnResult{
			TurnID:       turn.TurnID,
			Role:         turn.Role,
			Text:         turn.Text,
			ThinkingText: turn.ThinkingText,
			DurationMs:   turn.DurationMs,
			ToolNames:    turn.ToolNames,
			Actions:      actions,
			Timestamp:    turn.Timestamp,
			Model:        turn.Model,
			InputTokens:  turn.InputTokens,
			OutputTokens: turn.OutputTokens,
		})
	}

	return socket.ConversationFeedResult{
		Turns: turns,
		Count: len(turns),
	}
}

// TopKeywords returns the top N keywords sorted by hit count.
// Implements socket.AppQueries.
func (a *App) TopKeywords(limit int) socket.TopItemsResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	state := a.Learner.State()
	items := a.sortAndTruncate(state.KeywordHits, limit)

	return socket.TopItemsResult{
		Items: items,
		Count: len(items),
		Kind:  "keywords",
	}
}

// TopTerms returns the top N terms sorted by hit count.
// Implements socket.AppQueries.
func (a *App) TopTerms(limit int) socket.TopItemsResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	state := a.Learner.State()
	items := a.sortAndTruncate(state.TermHits, limit)

	return socket.TopItemsResult{
		Items: items,
		Count: len(items),
		Kind:  "terms",
	}
}

// TopFiles returns the top N files sorted by hit count.
// Implements socket.AppQueries.
func (a *App) TopFiles(limit int) socket.TopItemsResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	state := a.Learner.State()
	items := a.sortAndTruncate(state.FileHits, limit)

	return socket.TopItemsResult{
		Items: items,
		Count: len(items),
		Kind:  "files",
	}
}

// DomainTermNames returns the term names for a domain, sorted by keyword hit
// popularity (most popular first). Stable sort with alphabetical tiebreaker.
// Implements socket.AppQueries.
func (a *App) DomainTermNames(domain string) []string {
	terms := a.Enricher.DomainTerms(domain)
	if terms == nil {
		return nil
	}
	state := a.Learner.State()
	type termPop struct {
		name string
		hits int
	}
	ranked := make([]termPop, 0, len(terms))
	for name, keywords := range terms {
		total := 0
		for _, kw := range keywords {
			total += int(state.KeywordHits[kw])
		}
		ranked = append(ranked, termPop{name, total})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].hits != ranked[j].hits {
			return ranked[i].hits > ranked[j].hits
		}
		return ranked[i].name < ranked[j].name
	})
	names := make([]string, len(ranked))
	for i, r := range ranked {
		names[i] = r.name
	}
	return names
}

// DomainTermHitCounts returns a map of term name → total keyword hits for a domain.
// Used by the dashboard to detect which terms are "hot" and animate them.
func (a *App) DomainTermHitCounts(domain string) map[string]int {
	terms := a.Enricher.DomainTerms(domain)
	if terms == nil {
		return nil
	}
	state := a.Learner.State()
	hits := make(map[string]int, len(terms))
	for name, keywords := range terms {
		total := 0
		for _, kw := range keywords {
			total += int(state.KeywordHits[kw])
		}
		if total > 0 {
			hits[name] = total
		}
	}
	if len(hits) == 0 {
		return nil
	}
	return hits
}

// ensureTurnBuilder returns the current exchange builder.
// All assistant events between user inputs accumulate into one builder.
// Must be called with a.mu held.
func (a *App) ensureTurnBuilder(turnID string, ts int64, model string) *turnBuilder {
	if a.currentBuilder == nil {
		a.currentBuilder = &turnBuilder{
			TurnID:    turnID,
			Timestamp: ts,
			Model:     model,
		}
	}
	// Keep model updated (first event may not have it)
	if model != "" && a.currentBuilder.Model == "" {
		a.currentBuilder.Model = model
	}
	return a.currentBuilder
}

// flushCurrentBuilder pushes the current exchange builder to the ring buffer.
// Must be called with a.mu held.
func (a *App) flushCurrentBuilder() {
	if a.currentBuilder != nil {
		a.pushTurn(a.turnFromBuilder(a.currentBuilder))
		a.currentBuilder = nil
	}
}

// flushStaleTurns flushes the current builder and any legacy buffered turns.
// Must be called with a.mu held.
func (a *App) flushStaleTurns(activeTurnID string) {
	a.flushCurrentBuilder()
	for id, tb := range a.turnBuffer {
		if id != activeTurnID {
			a.pushTurn(a.turnFromBuilder(tb))
			delete(a.turnBuffer, id)
		}
	}
}

// turnFromBuilder converts a turnBuilder into a ConversationTurn.
func (a *App) turnFromBuilder(tb *turnBuilder) ConversationTurn {
	return ConversationTurn{
		TurnID:       tb.TurnID,
		Role:         "assistant",
		Text:         a.truncate(tb.Text.String(), 500),
		ThinkingText: a.truncate(tb.ThinkingText.String(), 500),
		DurationMs:   tb.DurationMs,
		ToolNames:    tb.ToolNames,
		Actions:      tb.Actions,
		Timestamp:    tb.Timestamp,
		Model:        tb.Model,
		InputTokens:  tb.InputTokens,
		OutputTokens: tb.OutputTokens,
	}
}

// pushTurn adds a turn to the ring buffer.
// Must be called with a.mu held.
func (a *App) pushTurn(turn ConversationTurn) {
	a.convRing[a.convHead] = turn
	a.convHead = (a.convHead + 1) % 50
	if a.convCount < 50 {
		a.convCount++
	}
}

// accumulateTool updates tool metrics from a session event.
// Must be called with a.mu held.
func (a *App) accumulateTool(ev ports.SessionEvent) {
	if ev.Tool == nil {
		return
	}

	// Update per-tool counters
	switch ev.Tool.Name {
	case "Read":
		a.toolMetrics.ReadCount++
		if ev.File != nil && ev.File.Path != "" {
			key := ev.File.Path
			if ev.File.Offset > 0 || ev.File.Limit > 0 {
				key = fmt.Sprintf("%s:%d-%d", ev.File.Path, ev.File.Offset, ev.File.Offset+ev.File.Limit)
			}
			a.toolMetrics.FileReads[key]++
		}
	case "Write":
		a.toolMetrics.WriteCount++
	case "Edit":
		a.toolMetrics.EditCount++
	case "Bash":
		a.toolMetrics.BashCount++
		if ev.Tool.Command != "" {
			cmd := a.truncate(ev.Tool.Command, 100)
			a.toolMetrics.BashCommands[cmd]++
		}
	case "Grep":
		a.toolMetrics.GrepCount++
		if ev.Tool.Pattern != "" {
			pattern := a.truncate(ev.Tool.Pattern, 100)
			a.toolMetrics.GrepPatterns[pattern]++
		}
	case "Glob":
		a.toolMetrics.GlobCount++
	default:
		a.toolMetrics.OtherCount++
	}
}

// readSavings computes a savings string for a range-limited read vs full file.
// Returns e.g. "↓62%" or empty string if savings can't be computed.
// Must be called with a.mu held.
func (a *App) readSavings(path string, limit int) string {
	if limit <= 0 || a.Index == nil {
		return ""
	}
	// Find the file's size from the index
	for _, fm := range a.Index.Files {
		if fm.Path == path && fm.Size > 0 {
			// Approximate lines: assume ~40 bytes per line
			totalLines := fm.Size / 40
			if totalLines <= 0 {
				totalLines = 1
			}
			if int64(limit) < totalLines {
				pct := 100 - (int64(limit)*100)/totalLines
				if pct > 0 && pct <= 99 {
					return fmt.Sprintf("\u2193%d%%", pct)
				}
			}
			break
		}
	}
	return ""
}

// truncate truncates a string to a maximum length.
func (a *App) truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// topN returns the top N items from a map, sorted by count descending.
// Used for tool metrics snapshot. Truncates to 20 items max.
func (a *App) topN(m map[string]int, n int) map[string]int {
	if len(m) == 0 {
		return make(map[string]int)
	}
	if n > 20 {
		n = 20
	}

	type item struct {
		key   string
		count int
	}
	var items []item
	for k, v := range m {
		items = append(items, item{k, v})
	}

	// Sort by count descending, then by key ascending
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].count > items[i].count ||
				(items[j].count == items[i].count && items[j].key < items[i].key) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	// Truncate to top N
	if len(items) > n {
		items = items[:n]
	}

	result := make(map[string]int, len(items))
	for _, it := range items {
		result[it.key] = it.count
	}
	return result
}

// sortAndTruncate converts a map to sorted RankedItems and truncates to limit.
// Works with learner state maps (map[string]uint32).
func (a *App) sortAndTruncate(m map[string]uint32, limit int) []socket.RankedItem {
	if len(m) == 0 {
		return []socket.RankedItem{}
	}

	type scoredItem struct {
		name  string
		score float64
	}
	var items []scoredItem
	for name, count := range m {
		items = append(items, scoredItem{name, float64(count)})
	}

	// Sort by score descending, then by name ascending
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].score > items[i].score ||
				(items[j].score == items[i].score && items[j].name < items[i].name) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	// Truncate to limit
	if len(items) > limit {
		items = items[:limit]
	}

	result := make([]socket.RankedItem, len(items))
	for i, it := range items {
		result[i] = socket.RankedItem{
			Name:  it.name,
			Count: it.score,
		}
	}
	return result
}

// WipeProject deletes all persisted data and resets in-memory state.
// Implements socket.AppQueries.
func (a *App) WipeProject() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.Store.DeleteProject(a.ProjectID); err != nil {
		return err
	}

	// Reset learner to fresh state
	a.Learner = learner.New()

	// Clear index maps (engine holds same pointer, sees the change)
	a.Index.Tokens = make(map[string][]ports.TokenRef)
	a.Index.Metadata = make(map[ports.TokenRef]*ports.SymbolMeta)
	a.Index.Files = make(map[uint32]*ports.FileMeta)

	a.promptN = 0
	a.lastAutotune = nil

	// Reset session metrics accumulators
	a.sessionMetrics = SessionMetrics{}
	a.toolMetrics = ToolMetrics{
		FileReads:    make(map[string]int),
		BashCommands: make(map[string]int),
		GrepPatterns: make(map[string]int),
	}
	a.convRing = [50]ConversationTurn{}
	a.convHead = 0
	a.convCount = 0
	for k := range a.turnBuffer {
		delete(a.turnBuffer, k)
	}

	// Reset activity feed
	a.activityRing = [50]ActivityEntry{}
	a.activityHead = 0
	a.activityCount = 0
	a.guidedPaths = make(map[string]int64)

	return nil
}
