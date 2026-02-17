// Package app wires together all adapters and domain logic.
// It provides lifecycle management for the aOa daemon: create, start, stop.
package app

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

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
func (a *App) searchObserver(query string, opts ports.SearchOptions, result *index.SearchResult) {
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

	switch ev.Kind {
	case ports.EventUserInput:
		a.promptN++
		a.Learner.ProcessBigrams(ev.Text)
		a.writeStatus(nil)

	case ports.EventAIThinking, ports.EventAIResponse:
		if ev.Text != "" {
			a.Learner.ProcessBigrams(ev.Text)
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

	return nil
}
