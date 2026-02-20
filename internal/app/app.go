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

// CacheHitRate returns the fraction of cache read tokens vs total input tokens (0.0–1.0).
func (s *SessionMetrics) CacheHitRate() float64 {
	total := s.InputTokens + s.CacheReadTokens
	if total == 0 {
		return 0.0
	}
	return float64(s.CacheReadTokens) / float64(total)
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
	Tool        string // "Read", "Edit", "Bash", "Grep", "Glob", "Task", etc.
	Target      string // relative path, command, or pattern
	Range       string // ":offset-limit" for reads, empty otherwise
	Impact      string // "↓62%", "4 match", "309 pass", "1 fail", etc.
	Attrib      string // "aOa guided", "unguided", "crafted"/"authored"/"forged"/"innovated", "-"
	Tokens      int    // estimated token cost of this action (0 = unknown)
	Savings     int    // savings percentage when guided (0-99), 0 = none
	TimeSavedMs int64  // estimated time saved in ms (only on guided)
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
	Learned   string
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
	Parser    ports.Parser // nil = tokenization-only mode (no tree-sitter)
	Server    *socket.Server
	WebServer *web.Server
	Reader    *claude.Reader
	Index     *ports.Index

	reconBridge    *ReconBridge            // discovers and invokes aoa-recon companion binary
	mu             sync.Mutex             // serializes learner access (searches are concurrent)
	promptN        uint32                 // prompt counter (incremented on each user input)
	lastAutotune   *learner.AutotuneResult // most recent autotune result (for status line)
	statusLinePath string                 // project-local path for status line file
	httpPort       int                    // preferred HTTP port (0 = auto from project root)
	dbPath         string                 // path to bbolt database file
	started        time.Time              // daemon start time

	// Session metrics accumulators (ephemeral, reset on daemon restart)
	sessionMetrics SessionMetrics
	toolMetrics    ToolMetrics
	convRing       [50]ConversationTurn   // ring buffer of last 50 turns
	convHead       int                    // write index for ring buffer
	convCount      int                    // fill count (0-50)
	turnBuffer     map[string]*turnBuilder // partial AI turns keyed by TurnID
	currentBuilder *turnBuilder            // active exchange builder (all assistant events between user inputs)

	// Activity feed (separate ring buffer for tool actions)
	activityRing    [50]ActivityEntry // ring buffer of last 50 activity entries
	activityHead    int
	activityCount   int
	creativeWordIdx  int  // cycles through creative attrib words for Write/Edit
	learnWordIdx     int  // cycles through learn attrib words for Search/Read signals
	lastReadLearned  bool // set by range-gated read, consumed by Read activity row

	// Burn rate tracking (L0.1)
	burnRate            *BurnRateTracker // actual token burn rate
	burnRateCounterfact *BurnRateTracker // counterfactual (what burn would be without aOa)

	// Time savings tracking (L0.55)
	rateTracker        *RateTracker // dynamic ms/token rate from completed turns
	sessionTimeSavedMs int64        // cumulative time saved this session (ms)

	// Value engine fields (L0.3)
	currentModel          string // model from most recent event
	counterfactTokensSaved int64  // lifetime tokens saved by aOa-guided reads
	sessionReadCount      int    // total range-gated reads this session
	sessionGuidedCount    int    // reads where savings >= 50%

	// Session boundary tracking (L0.5)
	currentSessionID    string // active Claude session ID
	currentSessionStart int64  // unix timestamp of session start
	sessionPrompts      int    // prompt count within current session
}

// Config holds initialization parameters for the App.
type Config struct {
	ProjectRoot   string
	ProjectID     string
	DBPath        string      // path to bbolt file (default: .aoa/aoa.db)
	HTTPPort      int         // preferred HTTP port (default: computed from project root)
	CacheMaxBytes int64       // file cache memory budget (default: 250MB if 0)
	Parser        ports.Parser // optional: nil = tokenization-only mode (no tree-sitter)
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

	engine := index.NewSearchEngine(idx, domains, cfg.ProjectRoot)

	// Create file cache and attach to search engine
	cache := index.NewFileCache(cfg.CacheMaxBytes)
	engine.SetCache(cache)

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
		Parser:         cfg.Parser, // nil = tokenization-only mode
		Reader:         reader,
		Index:          idx,
		promptN:        lrn.PromptCount(),
		statusLinePath: statusPath,
		httpPort:       cfg.HTTPPort,
		dbPath:         cfg.DBPath,
		toolMetrics: ToolMetrics{
			FileReads:    make(map[string]int),
			BashCommands: make(map[string]int),
			GrepPatterns: make(map[string]int),
		},
		turnBuffer:          make(map[string]*turnBuilder),
		burnRate:            NewBurnRateTracker(5 * time.Minute),
		burnRateCounterfact: NewBurnRateTracker(5 * time.Minute),
		rateTracker:         NewRateTracker(30 * time.Minute),
	}

	// Create server with App as query provider (for domains, stats, etc.)
	sockPath := socket.SocketPath(cfg.ProjectRoot)
	a.Server = socket.NewServer(engine, idx, sockPath, a)

	// Create HTTP server for web dashboard
	httpPortFile := filepath.Join(cfg.ProjectRoot, ".aoa", "http.port")
	a.WebServer = web.NewServer(a, idx, engine, httpPortFile)

	// Wire search observer: search results → learning signals
	engine.SetObserver(a.searchObserver)

	// Discover aoa-recon companion binary (non-fatal if not found)
	a.initReconBridge()

	return a, nil
}

// signalCollector accumulates unique keywords, terms, domains, and their
// relationship pairs from search queries and results. Each add method
// deduplicates so the caller doesn't need to track what's been seen.
type signalCollector struct {
	Keywords    []string
	Terms       []string
	Domains     []string
	KwTerms     [][2]string
	TermDomains [][2]string

	seenKw      map[string]bool
	seenTerms   map[string]bool
	seenDomains map[string]bool
}

func newSignalCollector() *signalCollector {
	return &signalCollector{
		seenKw:      make(map[string]bool),
		seenTerms:   make(map[string]bool),
		seenDomains: make(map[string]bool),
	}
}

// addKeyword adds a keyword and resolves it through the enricher to
// discover terms and domains. Skips duplicates.
func (sc *signalCollector) addKeyword(kw string, enr *enricher.Enricher) {
	if sc.seenKw[kw] {
		return
	}
	sc.seenKw[kw] = true
	sc.Keywords = append(sc.Keywords, kw)

	for _, m := range enr.Lookup(kw) {
		sc.KwTerms = append(sc.KwTerms, [2]string{kw, m.Term})
		sc.addTermDomain(m.Term, m.Domain)
	}
}

// addTerm adds a term and resolves it to its owning domain(s). Skips duplicates.
func (sc *signalCollector) addTerm(term string, enr *enricher.Enricher) {
	if sc.seenTerms[term] {
		return
	}
	sc.seenTerms[term] = true
	sc.Terms = append(sc.Terms, term)

	for _, domain := range enr.LookupTerm(term) {
		sc.addTermDomain(term, domain)
	}
}

// addDomain adds a domain directly. Skips duplicates.
func (sc *signalCollector) addDomain(domain string) {
	d := strings.TrimPrefix(domain, "@")
	if d == "" || sc.seenDomains[d] {
		return
	}
	sc.seenDomains[d] = true
	sc.Domains = append(sc.Domains, d)
}

// addTermDomain records a term→domain pair and ensures both are tracked.
func (sc *signalCollector) addTermDomain(term, domain string) {
	sc.TermDomains = append(sc.TermDomains, [2]string{term, domain})
	if !sc.seenTerms[term] {
		sc.seenTerms[term] = true
		sc.Terms = append(sc.Terms, term)
	}
	if !sc.seenDomains[domain] {
		sc.seenDomains[domain] = true
		sc.Domains = append(sc.Domains, domain)
	}
}

// searchObserver extracts learning signals from search queries and results.
// Called after every search. Thread-safe via mutex.
func (a *App) searchObserver(query string, opts ports.SearchOptions, result *index.SearchResult, elapsed time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.promptN++

	tokens := index.Tokenize(query)
	if len(tokens) == 0 {
		return
	}

	sc := newSignalCollector()

	// 1. Query tokens → keywords → terms → domains
	for _, tok := range tokens {
		sc.addKeyword(tok, a.Enricher)
	}

	// 2. Top result hits → direct-increment domains, terms, term-domain pairs
	//    No enricher re-resolution — signals come directly from index metadata.
	sig := collectHitSignals(result.Hits, 10)
	for _, d := range sig.Domains {
		sc.addDomain(d)
	}
	for _, term := range sig.Terms {
		if !sc.seenTerms[term] {
			sc.seenTerms[term] = true
			sc.Terms = append(sc.Terms, term)
		}
	}
	for _, td := range sig.TermDomains {
		sc.addTermDomain(td[0], td[1])
	}
	// Feed content hit text through bigram extraction
	for _, text := range sig.ContentText {
		a.Learner.ProcessBigrams(text)
	}

	event := learner.ObserveEvent{
		PromptNumber: a.promptN,
		Observe: learner.ObserveData{
			Keywords:     sc.Keywords,
			Terms:        sc.Terms,
			Domains:      sc.Domains,
			KeywordTerms: sc.KwTerms,
			TermDomains:  sc.TermDomains,
		},
	}

	tuneResult := a.Learner.ObserveAndMaybeTune(event)
	if tuneResult != nil {
		_ = a.Store.SaveLearnerState(a.ProjectID, a.Learner.State())
		a.writeStatus(tuneResult)
		// L0.7: Autotune activity event
		a.pushActivity(ActivityEntry{
			Action:    "Autotune",
			Source:    "aOa",
			Attrib:    fmt.Sprintf("cycle %d", a.promptN/50),
			Impact:    fmt.Sprintf("+%d promoted, -%d demoted, ~%d decayed", tuneResult.Promoted, tuneResult.Demoted, tuneResult.Decayed),
			Timestamp: time.Now().Unix(),
		})
	}

	// Count unique files in results
	fileSet := make(map[string]bool)
	for _, hit := range result.Hits {
		fileSet[hit.File] = true
	}

	entry := ActivityEntry{
		Action:    "Search",
		Source:    "aOa",
		Attrib:    a.searchAttrib(query, opts),
		Impact:    fmt.Sprintf("%d hits, %d files | %.2fms", len(result.Hits), len(fileSet), elapsed.Seconds()*1000),
		Target:    a.searchTarget(query, opts),
		Timestamp: time.Now().Unix(),
	}
	if len(sc.Keywords) > 0 || len(sc.Terms) > 0 || len(sc.Domains) > 0 {
		entry.Learned = a.nextLearnWord()
	}
	a.pushActivity(entry)
}

// Start begins the daemon (socket server + HTTP server + session reader).
func (a *App) Start() error {
	a.started = time.Now()
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
	// Start file watcher — non-fatal if setup fails
	if err := a.Watcher.Watch(a.ProjectRoot, a.onFileChanged); err != nil {
		fmt.Printf("[warning] file watcher unavailable: %v\n", err)
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
	// Persist final state before shutdown
	a.mu.Lock()
	a.flushSessionSummary()
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

	// Track current model from every event
	if ev.Model != "" {
		a.currentModel = ev.Model
	}

	// Session boundary detection (L0.5)
	a.handleSessionBoundary(ev)

	ts := ev.Timestamp.Unix()
	if ts == 0 {
		ts = int64(a.promptN) // fallback if timestamp missing
	}

	switch ev.Kind {
	case ports.EventUserInput:
		// Flush the current exchange builder before starting new user input
		a.flushCurrentBuilder()
		a.promptN++
		a.sessionPrompts++
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
				tb.ThinkingText.WriteString("\n")
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
			// L0.1: Record burn rate for both actual and counterfactual
			total := ev.Usage.InputTokens + ev.Usage.OutputTokens + ev.Usage.CacheReadTokens + ev.Usage.CacheWriteTokens
			a.burnRate.Record(total)
			a.burnRateCounterfact.Record(total)
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
			// L0.3: Track read counts and counterfactual savings
			a.sessionReadCount++
			s := a.readSavings(ev.File.Path, ev.File.Limit)
			if s.pct >= 50 {
				a.sessionGuidedCount++
				delta := int(s.fileTokens - s.readTokens)
				a.counterfactTokensSaved += int64(delta)
				a.burnRateCounterfact.Record(delta)
				_, tsMs := a.timeSavedForTokens(delta)
				a.sessionTimeSavedMs += tsMs
			}
			// L0.11: Mark that learning occurred (attrib pill added to Read row below)
			a.lastReadLearned = true
		}
		// Accumulate tool metrics
		a.accumulateTool(ev)
		// Buffer tool name and action for the current turn
		if ev.Tool != nil {
			tb := a.ensureTurnBuilder(ev.TurnID, ts, ev.Model)
			tb.ToolNames = append(tb.ToolNames, ev.Tool.Name)

			// Build target, range, impact, attrib, tokens, savings for activity + turn action
			action := ev.Tool.Name
			attrib := "-"
			target := ""
			rangeStr := ""
			impact := "-"
			tokens := 0
			savings := 0
			var timeSavedMs int64
			skipActivity := false

			switch action {
			case "Read":
				if ev.File != nil && ev.File.Path != "" {
					target = a.relativePath(ev.File.Path)
					if ev.File.Offset > 0 || ev.File.Limit > 0 {
						// Partial read — check for guided savings
						rangeStr = fmt.Sprintf(":%d-%d", ev.File.Offset, ev.File.Offset+ev.File.Limit)
						s := a.readSavings(ev.File.Path, ev.File.Limit)
						if s.display != "" {
							impact = s.display
							tokens = int(s.readTokens)
							if s.pct >= 50 {
								attrib = "aOa guided"
								savings = s.pct
								_, tsMs := a.timeSavedForTokens(int(s.fileTokens - s.readTokens))
								timeSavedMs = tsMs
							}
						} else {
							// Partial read but no savings data — estimate from lines
							tokens = ev.File.Limit * 20 // ~80 bytes/line ÷ 4
							attrib = "unguided"
							if tokens > 0 {
								impact = fmt.Sprintf("~%s tokens", formatTokens(int64(tokens)))
							}
						}
					} else {
						// Full file read — estimate cost from index
						attrib = "unguided"
						tokens = a.estimateFileTokens(ev.File.Path)
						if tokens > 0 {
							impact = fmt.Sprintf("~%s tokens", formatTokens(int64(tokens)))
						}
					}
					// lastReadLearned consumed below when building activity entry
				}
			case "Bash":
				if ev.Tool.Command != "" {
					target = a.truncate(ev.Tool.Command, 100)
				}
				// A-03: Filter aOa commands (already captured as Search by observer)
				// A-04: Filter Bash commands without file context
				if strings.HasPrefix(ev.Tool.Command, "./aoa ") ||
					strings.HasPrefix(ev.Tool.Command, "aoa ") ||
					ev.File == nil {
					skipActivity = true
				}
			case "Grep":
				if ev.Tool.Pattern != "" {
					target = ev.Tool.Pattern
				}
				attrib = "unguided"
				tokens = a.estimateGrepTokens()
				impact = a.estimateGrepCost(ev.Tool.Pattern)
				// Session-log Grep → learning signals
				if ev.Tool.Pattern != "" {
					a.processGrepSignal(ev.Tool.Pattern)
				}
			case "Write", "Edit":
				if ev.File != nil && ev.File.Path != "" {
					target = a.relativePath(ev.File.Path)
				}
				creativeWords := [4]string{"crafted", "authored", "forged", "innovated"}
				attrib = creativeWords[a.creativeWordIdx%4]
				a.creativeWordIdx++
			case "Glob":
				// Prefer pattern for display (more descriptive than directory)
				if ev.Tool.Pattern != "" {
					target = ev.Tool.Pattern
				} else if ev.File != nil && ev.File.Path != "" {
					target = a.relativePath(ev.File.Path)
				}
				attrib = "unguided"
				globPattern := ev.Tool.Pattern
				globDir := ""
				if ev.File != nil {
					globDir = ev.File.Path
				}
				if globPattern != "" || globDir != "" {
					impact = a.estimateGlobCost(globDir, globPattern)
					tokens = a.estimateGlobTokens(globDir, globPattern)
				}
			case "Task":
				if ev.Tool.Command != "" {
					target = a.truncate(ev.Tool.Command, 100)
				}
			}

			// Append structured action to turn builder
			tb.Actions = append(tb.Actions, TurnAction{
				Tool:        action,
				Target:      target,
				Range:       rangeStr,
				Impact:      impact,
				Attrib:      attrib,
				Tokens:      tokens,
				Savings:     savings,
				TimeSavedMs: timeSavedMs,
			})

			// Activity feed target includes range for reads
			actTarget := target
			if rangeStr != "" {
				actTarget = target + rangeStr
			}

			if !skipActivity {
				entry := ActivityEntry{
					Action:    action,
					Source:    "Claude",
					Attrib:    attrib,
					Impact:    impact,
					Target:    actTarget,
					Timestamp: time.Now().Unix(),
				}
				if action == "Read" && a.lastReadLearned {
					entry.Learned = a.nextLearnWord()
					a.lastReadLearned = false
				}
				a.pushActivity(entry)
			}
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
		return "multi-and"
	}
	tokens := index.Tokenize(query)
	if len(tokens) > 1 {
		return "multi-or"
	}
	return "indexed"
}

// learnWords are cycling attrib words that signal learning occurred.
var learnWords = [8]string{
	"trained", "fine-tuned", "calibrated", "converged",
	"reinforced", "optimized", "weighted", "adapted",
}

// nextLearnWord returns the next cycling learn word for attrib pills.
// Must be called with a.mu held.
func (a *App) nextLearnWord() string {
	w := learnWords[a.learnWordIdx%len(learnWords)]
	a.learnWordIdx++
	return w
}

// searchTarget returns the target label for a search query.
// Preserves the actual command with flags as the user typed it.
func (a *App) searchTarget(query string, opts ports.SearchOptions) string {
	var parts []string
	if opts.Mode == "regex" {
		parts = append(parts, "aOa egrep")
	} else {
		parts = append(parts, "aOa grep")
	}
	if opts.Mode == "case_insensitive" {
		parts = append(parts, "-i")
	}
	if opts.InvertMatch {
		parts = append(parts, "-v")
	}
	if opts.AndMode {
		parts = append(parts, "-a")
	}
	if opts.WordBoundary {
		parts = append(parts, "-w")
	}
	if opts.CountOnly {
		parts = append(parts, "-c")
	}
	if opts.OnlyMatching {
		parts = append(parts, "-o")
	}
	if opts.FilesWithoutMatch {
		parts = append(parts, "-L")
	}
	if opts.IncludeGlob != "" {
		parts = append(parts, "--include", opts.IncludeGlob)
	}
	if opts.ExcludeGlob != "" {
		parts = append(parts, "--exclude", opts.ExcludeGlob)
	}
	if opts.ExcludeDirGlob != "" {
		parts = append(parts, "--exclude-dir", opts.ExcludeDirGlob)
	}
	if opts.Context > 0 {
		parts = append(parts, fmt.Sprintf("-C %d", opts.Context))
	} else {
		if opts.BeforeContext > 0 {
			parts = append(parts, fmt.Sprintf("-B %d", opts.BeforeContext))
		}
		if opts.AfterContext > 0 {
			parts = append(parts, fmt.Sprintf("-A %d", opts.AfterContext))
		}
	}
	parts = append(parts, query)
	return strings.Join(parts, " ")
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
			Learned:   e.Learned,
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

// RunwayProjection computes dual context-runway projections.
// Implements socket.AppQueries.
func (a *App) RunwayProjection() socket.RunwayResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	model := a.currentModel
	windowMax := ContextWindowSize(model)
	tokensUsed := a.burnRate.TotalTokens()
	burnRate := a.burnRate.TokensPerMin()
	counterfactRate := a.burnRateCounterfact.TokensPerMin()

	remaining := int64(windowMax) - tokensUsed
	if remaining < 0 {
		remaining = 0
	}

	var runwayMin, counterfactMin float64
	if burnRate > 0 {
		runwayMin = float64(remaining) / burnRate
	}
	if counterfactRate > 0 {
		counterfactMin = float64(remaining) / counterfactRate
	}

	return socket.RunwayResult{
		Model:              model,
		ContextWindowMax:   windowMax,
		TokensUsed:         tokensUsed,
		BurnRatePerMin:     burnRate,
		CounterfactPerMin:  counterfactRate,
		RunwayMinutes:      runwayMin,
		CounterfactMinutes: counterfactMin,
		DeltaMinutes:       runwayMin - counterfactMin,
		TokensSaved:        a.counterfactTokensSaved,
		TimeSavedMs:        a.sessionTimeSavedMs,
	}
}

// SessionList returns all persisted session summaries, sorted by start time descending.
// Implements socket.AppQueries.
func (a *App) SessionList() socket.SessionListResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Store == nil {
		return socket.SessionListResult{Sessions: []socket.SessionSummaryResult{}}
	}

	summaries, err := a.Store.ListSessionSummaries(a.ProjectID)
	if err != nil || len(summaries) == 0 {
		return socket.SessionListResult{Sessions: []socket.SessionSummaryResult{}}
	}

	// Sort by start time descending (most recent first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].StartTime > summaries[j].StartTime
	})

	results := make([]socket.SessionSummaryResult, len(summaries))
	for i, s := range summaries {
		results[i] = socket.SessionSummaryResult{
			SessionID:        s.SessionID,
			StartTime:        s.StartTime,
			EndTime:          s.EndTime,
			PromptCount:      s.PromptCount,
			ReadCount:        s.ReadCount,
			GuidedReadCount:  s.GuidedReadCount,
			GuidedRatio:      s.GuidedRatio,
			TokensSaved:      s.TokensSaved,
			TimeSavedMs:      s.TimeSavedMs,
			InputTokens:      s.InputTokens,
			OutputTokens:     s.OutputTokens,
			CacheReadTokens:  s.CacheReadTokens,
			CacheWriteTokens: s.CacheWriteTokens,
			Model:            s.Model,
		}
	}

	return socket.SessionListResult{
		Sessions: results,
		Count:    len(results),
	}
}

// ProjectConfig returns project metadata and runtime configuration.
// Implements socket.AppQueries.
func (a *App) ProjectConfig() socket.ProjectConfigResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	indexFiles := 0
	indexTokens := 0
	if a.Index != nil {
		indexFiles = len(a.Index.Files)
		indexTokens = len(a.Index.Tokens)
	}

	var uptimeSeconds int64
	if !a.started.IsZero() {
		uptimeSeconds = int64(time.Since(a.started).Seconds())
	}

	return socket.ProjectConfigResult{
		ProjectRoot:   a.ProjectRoot,
		ProjectID:     a.ProjectID,
		DBPath:        a.dbPath,
		SocketPath:    socket.SocketPath(a.ProjectRoot),
		IndexFiles:    indexFiles,
		IndexTokens:   indexTokens,
		UptimeSeconds: uptimeSeconds,
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
				Tool:        act.Tool,
				Target:      act.Target,
				Range:       act.Range,
				Impact:      act.Impact,
				Attrib:      act.Attrib,
				Tokens:      act.Tokens,
				Savings:     act.Savings,
				TimeSavedMs: act.TimeSavedMs,
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
				Tool:        act.Tool,
				Target:      act.Target,
				Range:       act.Range,
				Impact:      act.Impact,
				Attrib:      act.Attrib,
				Tokens:      act.Tokens,
				Savings:     act.Savings,
				TimeSavedMs: act.TimeSavedMs,
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
		// Record ms/token rate sample for time savings computation
		if a.currentBuilder.DurationMs > 0 && a.currentBuilder.OutputTokens > 0 {
			a.rateTracker.Record(a.currentBuilder.DurationMs, a.currentBuilder.OutputTokens)
		}
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

// handleSessionBoundary detects Claude session changes and flushes/loads session summaries.
// Must be called with a.mu held.
func (a *App) handleSessionBoundary(ev ports.SessionEvent) {
	if ev.SessionID == "" || ev.SessionID == a.currentSessionID {
		return
	}
	// Flush old session if we had one
	if a.currentSessionID != "" {
		a.flushSessionSummary()
	}
	// Check if revisiting an existing session
	if a.Store != nil {
		existing, _ := a.Store.LoadSessionSummary(a.ProjectID, ev.SessionID)
		if existing != nil {
			// Restore counters from persisted session
			a.sessionPrompts = existing.PromptCount
			a.sessionReadCount = existing.ReadCount
			a.sessionGuidedCount = existing.GuidedReadCount
			a.counterfactTokensSaved = existing.TokensSaved
			a.sessionTimeSavedMs = existing.TimeSavedMs
			a.currentSessionStart = existing.StartTime
		} else {
			a.sessionPrompts = 0
			a.sessionReadCount = 0
			a.sessionGuidedCount = 0
			a.sessionTimeSavedMs = 0
			a.currentSessionStart = ev.Timestamp.Unix()
		}
	} else {
		a.sessionPrompts = 0
		a.sessionReadCount = 0
		a.sessionGuidedCount = 0
		a.sessionTimeSavedMs = 0
		a.currentSessionStart = ev.Timestamp.Unix()
	}
	a.currentSessionID = ev.SessionID
}

// flushSessionSummary builds a SessionSummary from current counters and persists it.
// Must be called with a.mu held.
func (a *App) flushSessionSummary() {
	if a.currentSessionID == "" || a.Store == nil {
		return
	}
	var guidedRatio float64
	if a.sessionReadCount > 0 {
		guidedRatio = float64(a.sessionGuidedCount) / float64(a.sessionReadCount)
	}
	summary := &ports.SessionSummary{
		SessionID:        a.currentSessionID,
		StartTime:        a.currentSessionStart,
		EndTime:          time.Now().Unix(),
		PromptCount:      a.sessionPrompts,
		ReadCount:        a.sessionReadCount,
		GuidedReadCount:  a.sessionGuidedCount,
		GuidedRatio:      guidedRatio,
		TokensSaved:      a.counterfactTokensSaved,
		TimeSavedMs:      a.sessionTimeSavedMs,
		InputTokens:      a.sessionMetrics.InputTokens,
		OutputTokens:     a.sessionMetrics.OutputTokens,
		CacheReadTokens:  a.sessionMetrics.CacheReadTokens,
		CacheWriteTokens: a.sessionMetrics.CacheWriteTokens,
		Model:            a.currentModel,
	}
	_ = a.Store.SaveSessionSummary(a.ProjectID, summary)
}

// turnFromBuilder converts a turnBuilder into a ConversationTurn.
func (a *App) turnFromBuilder(tb *turnBuilder) ConversationTurn {
	return ConversationTurn{
		TurnID:       tb.TurnID,
		Role:         "assistant",
		Text:         a.truncate(tb.Text.String(), 500),
		ThinkingText: a.truncate(tb.ThinkingText.String(), 2000),
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

// savingsInfo holds the result of a token-savings calculation for a range-limited read.
type savingsInfo struct {
	pct        int    // savings percentage (0-100)
	fileTokens int64  // full file estimated tokens
	readTokens int64  // read portion estimated tokens
	display    string // formatted "↓N% (Xk → Yk)" or ""
}

// readSavings computes token savings for a range-limited read vs full file.
// Token approximation: bytes / 4. Read output: limit lines × ~80 bytes/line / 4.
// Must be called with a.mu held.
func (a *App) readSavings(path string, limit int) savingsInfo {
	if limit <= 0 || a.Index == nil {
		return savingsInfo{}
	}
	// Normalize to relative path for index lookup
	relPath := path
	if a.ProjectRoot != "" && strings.HasPrefix(path, a.ProjectRoot) {
		relPath = strings.TrimPrefix(path, a.ProjectRoot)
		relPath = strings.TrimPrefix(relPath, "/")
	}
	for _, fm := range a.Index.Files {
		if (fm.Path == path || fm.Path == relPath) && fm.Size > 0 {
			fileTokens := fm.Size / 4
			if fileTokens <= 0 {
				fileTokens = 1
			}
			readTokens := int64(limit) * 20 // ~80 bytes/line ÷ 4 bytes/token
			if readTokens >= fileTokens {
				return savingsInfo{}
			}
			pct := int((fileTokens - readTokens) * 100 / fileTokens)
			if pct <= 0 || pct > 99 {
				return savingsInfo{}
			}
			display := fmt.Sprintf("↓%d%% (%s → %s)", pct, formatTokens(fileTokens), formatTokens(readTokens))
			return savingsInfo{pct: pct, fileTokens: fileTokens, readTokens: readTokens, display: display}
		}
	}
	return savingsInfo{}
}

// relativePath strips the project root prefix from an absolute path.
func (a *App) relativePath(path string) string {
	if a.ProjectRoot != "" && strings.HasPrefix(path, a.ProjectRoot) {
		rel := strings.TrimPrefix(path, a.ProjectRoot)
		rel = strings.TrimPrefix(rel, "/")
		if rel != "" {
			return rel
		}
	}
	return path
}

// formatTokens formats a token count with k/M suffixes.
func formatTokens(tokens int64) string {
	if tokens >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	}
	if tokens >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1_000)
	}
	return fmt.Sprintf("%d", tokens)
}

// formatTimeSaved formats milliseconds into a human-readable "saved ~Ns" or "saved ~N.Nmin" string.
// Returns "" for values ≤ 0.
func formatTimeSaved(ms int64) string {
	if ms <= 0 {
		return ""
	}
	if ms >= 60000 {
		return fmt.Sprintf("saved ~%.1fmin", float64(ms)/60000)
	}
	return fmt.Sprintf("saved ~%ds", ms/1000)
}

// timeSavedForTokens converts token savings into a time estimate.
// Uses dynamic rate from rateTracker if available, otherwise falls back to 7.5 ms/token.
// Returns the display string and raw milliseconds.
// Must be called with a.mu held.
func (a *App) timeSavedForTokens(tokensSaved int) (string, int64) {
	if tokensSaved <= 0 {
		return "", 0
	}
	rate := 7.5 // fallback constant
	if a.rateTracker != nil && a.rateTracker.HasData() {
		rate = a.rateTracker.MsPerToken()
	}
	ms := int64(float64(tokensSaved) * rate)
	return formatTimeSaved(ms), ms
}

// estimateGlobCost estimates the token cost of a glob operation by scanning
// the index for files matching the given pattern within a directory.
// Must be called with a.mu held.
func (a *App) estimateGlobCost(dir, pattern string) string {
	totalBytes := a.matchGlobFiles(dir, pattern)
	if totalBytes == 0 {
		return "-"
	}
	tokens := totalBytes / 4
	return fmt.Sprintf("~%s tokens", formatTokens(tokens))
}

// matchGlobFiles sums the size of indexed files matching a glob pattern.
// Handles both: pattern-only (e.g., "internal/**/*.go") and dir+pattern combos.
func (a *App) matchGlobFiles(dir, pattern string) int64 {
	if a.Index == nil {
		return 0
	}
	// Build the effective glob by combining dir and pattern
	effectiveGlob := pattern
	if effectiveGlob == "" && dir != "" {
		// Directory-only: match all files under it
		relDir := a.relativePath(dir)
		var totalBytes int64
		for _, fm := range a.Index.Files {
			fmRel := a.relativePath(fm.Path)
			if strings.HasPrefix(fmRel, relDir) || strings.HasPrefix(fm.Path, dir) {
				totalBytes += fm.Size
			}
		}
		return totalBytes
	}
	// Try filepath.Match against relative paths in the index
	var totalBytes int64
	for _, fm := range a.Index.Files {
		fmRel := a.relativePath(fm.Path)
		if matched, _ := filepath.Match(effectiveGlob, fmRel); matched {
			totalBytes += fm.Size
			continue
		}
		// Also try with absolute path
		if matched, _ := filepath.Match(effectiveGlob, fm.Path); matched {
			totalBytes += fm.Size
		}
	}
	return totalBytes
}

// estimateGrepCost estimates the token cost of a grep scan across indexed files.
// Must be called with a.mu held.
func (a *App) estimateGrepCost(pattern string) string {
	if a.Index == nil || pattern == "" {
		return "-"
	}
	var totalBytes int64
	for _, fm := range a.Index.Files {
		totalBytes += fm.Size
	}
	if totalBytes == 0 {
		return "-"
	}
	tokens := totalBytes / 4
	return fmt.Sprintf("~%s tokens", formatTokens(tokens))
}

// estimateFileTokens estimates the token cost of reading an entire file.
// Returns 0 if the file is not in the index.
// Must be called with a.mu held.
func (a *App) estimateFileTokens(path string) int {
	if a.Index == nil {
		return 0
	}
	relPath := a.relativePath(path)
	for _, fm := range a.Index.Files {
		if fm.Path == path || fm.Path == relPath {
			return int(fm.Size / 4)
		}
	}
	return 0
}

// estimateGrepTokens returns the estimated token count for a grep scan.
// Must be called with a.mu held.
func (a *App) estimateGrepTokens() int {
	if a.Index == nil {
		return 0
	}
	var totalBytes int64
	for _, fm := range a.Index.Files {
		totalBytes += fm.Size
	}
	return int(totalBytes / 4)
}

// estimateGlobTokens returns the estimated token count for a glob match.
// Must be called with a.mu held.
func (a *App) estimateGlobTokens(dir, pattern string) int {
	return int(a.matchGlobFiles(dir, pattern) / 4)
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

// Reindex performs a full project walk + parse + index rebuild.
// The IO-heavy walk/parse runs outside the mutex; only the swap is locked.
// Implements socket.AppQueries.
func (a *App) Reindex() (socket.ReindexResult, error) {
	start := time.Now()

	// Build new index outside the mutex (IO-heavy)
	idx, stats, err := BuildIndex(a.ProjectRoot, a.Parser)
	if err != nil {
		return socket.ReindexResult{}, fmt.Errorf("build index: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Swap index maps in-place (engine/server hold pointer to a.Index struct)
	a.Index.Tokens = idx.Tokens
	a.Index.Metadata = idx.Metadata
	a.Index.Files = idx.Files

	a.Engine.Rebuild()

	if a.Store != nil {
		if err := a.Store.SaveIndex(a.ProjectID, a.Index); err != nil {
			return socket.ReindexResult{}, fmt.Errorf("save index: %w", err)
		}
	}

	// If parser is nil and aoa-recon is available, trigger enhancement in background.
	if a.Parser == nil {
		a.TriggerReconEnhance()
	}

	elapsed := time.Since(start)
	return socket.ReindexResult{
		FileCount:   stats.FileCount,
		SymbolCount: stats.SymbolCount,
		TokenCount:  stats.TokenCount,
		ElapsedMs:   elapsed.Milliseconds(),
	}, nil
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

	// Reset burn rate trackers (L0.1)
	a.burnRate.Reset()
	a.burnRateCounterfact.Reset()

	// Reset time savings tracking (L0.55)
	a.sessionTimeSavedMs = 0
	a.rateTracker.Reset()

	// Reset value engine fields (L0.3)
	a.currentModel = ""
	a.counterfactTokensSaved = 0
	a.sessionReadCount = 0
	a.sessionGuidedCount = 0

	// Reset session tracking (L0.5)
	a.currentSessionID = ""
	a.currentSessionStart = 0
	a.sessionPrompts = 0

	return nil
}
