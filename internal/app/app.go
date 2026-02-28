// Package app wires together all adapters and domain logic.
// It provides lifecycle management for the aOa daemon: create, start, stop.
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/corey/aoa/atlas"
	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/version"
	claude "github.com/corey/aoa/internal/adapters/claude"
	fsw "github.com/corey/aoa/internal/adapters/fsnotify"
	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/adapters/web"
	"github.com/corey/aoa/internal/domain/analyzer"
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
	ModelTokens      map[string]int64 // per-model total token breakdown
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
	ToolID      string // tool_use_id for result correlation
	ResultChars int    // character count of tool result (0 = unknown)
	Pattern     string // L9.2: search pattern (Grep/Glob)
	FilePath    string // L9.2: file path (Read/Write/Edit)
	Command     string // L9.2: shell command (Bash)
}

// ConversationTurn describes a single turn in the conversation feed.
type ConversationTurn struct {
	TurnID       string
	Role         string // "user" or "assistant"
	Text         string
	ThinkingText string
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

// UsageQuotaTier holds parsed data for one usage tier (session, weekly-all, weekly-sonnet).
type UsageQuotaTier struct {
	Label      string `json:"label"`       // "session", "weekly_all", "weekly_sonnet"
	UsedPct    int    `json:"used_pct"`    // 0-100
	ResetsAt   string `json:"resets_at"`   // raw from /usage: "Feb 22, 8pm" or "3pm"
	ResetEpoch int64  `json:"reset_epoch"` // best-effort parsed to unix timestamp (0 if unparseable)
	Timezone   string `json:"timezone"`    // "America/New_York"
}

// UsageQuota holds the full parsed /usage output.
type UsageQuota struct {
	Session      *UsageQuotaTier `json:"session,omitempty"`
	WeeklyAll    *UsageQuotaTier `json:"weekly_all,omitempty"`
	WeeklySonnet *UsageQuotaTier `json:"weekly_sonnet,omitempty"`
	CapturedAt   int64           `json:"captured_at"`
}

// ContextSnapshot holds a single status line data point from Claude Code.
// Written by the hook script to .aoa/context.jsonl, read by the daemon.
type ContextSnapshot struct {
	Timestamp          int64   `json:"ts"`
	CtxUsed            int64   `json:"ctx_used"`
	CtxMax             int64   `json:"ctx_max"`
	UsedPct            float64 `json:"used_pct"`
	RemainingPct       float64 `json:"remaining_pct"`
	TotalCostUSD       float64 `json:"total_cost_usd"`
	TotalDurationMs    int64   `json:"total_duration_ms"`
	TotalApiDurationMs int64   `json:"total_api_duration_ms"`
	TotalLinesAdded    int     `json:"total_lines_added"`
	TotalLinesRemoved  int     `json:"total_lines_removed"`
	Model              string  `json:"model"`
	SessionID          string  `json:"session_id"`
	Version            string  `json:"version"`
}

// App is the top-level container wiring all components together.
type App struct {
	ProjectRoot string
	ProjectID   string
	Paths       *Paths // resolved .aoa/ directory paths

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

	dimEngine      any                    // *recon.Engine in full builds, nil in lean
	dimRules       []analyzer.Rule         // loaded from YAML at startup
	debug          bool                   // AOA_DEBUG=1 enables verbose event logging
	mu             sync.Mutex             // serializes learner access (searches are concurrent)

	reconMu        sync.RWMutex

	// Dimensional results cache (from aoa-recon via bbolt, cached in memory)
	dimCache     map[string]*socket.DimensionalFileResult
	dimCacheSet  bool // true once loaded (distinguishes nil "no data" from "not loaded")

	// Investigated files (user-triaged, excluded from active recon view)
	investigatedFiles map[string]int64 // relPath -> unix timestamp
	promptN        uint32                 // prompt counter (incremented on each user input)
	lastAutotune   *learner.AutotuneResult // most recent autotune result (for status line)
	statusLinePath string                 // project-local path for status line file
	httpPort       int                    // preferred HTTP port (0 = auto from project root)
	dbPath         string                 // path to bbolt database file
	started        time.Time              // daemon start time

	// Content meter (L9.1) — raw char counts from all content streams
	meter ContentMeter

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

	// Shadow engine (L9.5/L9.6) — counterfactual comparison ring
	shadowRing    ShadowRing             // ring buffer of shadow comparison results
	shadowPending map[string]*ToolShadow // toolID → pending shadow (awaiting result)

	// Value engine fields (L0.3)
	currentModel          string // model from most recent event
	counterfactTokensSaved int64  // lifetime tokens saved by aOa-guided reads
	sessionReadCount      int    // total range-gated reads this session
	sessionGuidedCount    int    // reads where savings >= 50%

	// Session boundary tracking (L0.5)
	currentSessionID    string // active Claude session ID
	currentSessionStart int64  // unix timestamp of session start
	sessionPrompts      int    // prompt count within current session

	// Context snapshot ring buffer (from status line hook via .aoa/context.jsonl)
	ctxSnapshots [5]ContextSnapshot // ring buffer of last 5 snapshots
	ctxSnapHead  int                // write index
	ctxSnapCount int                // fill count (0-5)

	// Usage quota (from pasted /usage output via .aoa/usage.txt)
	usageQuota *UsageQuota

	// Debounced index persistence — in-memory index is always current,
	// only the bbolt write is delayed to avoid 306MB rewrites on every file change.
	indexDirty     bool
	indexSaveTimer *time.Timer
}

// Config holds initialization parameters for the App.
type Config struct {
	ProjectRoot   string
	ProjectID     string
	DBPath        string      // path to bbolt file (default: .aoa/aoa.db)
	HTTPPort      int         // preferred HTTP port (default: computed from project root)
	CacheMaxBytes int64       // file cache memory budget (default: 250MB if 0)
	Parser        ports.Parser // optional: nil = tokenization-only mode (no tree-sitter)
	Debug         bool        // enable debug logging (AOA_DEBUG=1)
}

// debugf logs a timestamped debug message when debug mode is enabled.
// No-op when debug is off. Safe to call from any goroutine.
func (a *App) debugf(format string, args ...interface{}) {
	if !a.debug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] [debug] %s\n", time.Now().Format("15:04:05.000"), msg)
}

// New creates an App with all dependencies wired. Does not start services.
// Initialization is fast — heavy IO (index load, cache warming) is deferred
// to WarmCaches() which should be called after Start().
func New(cfg Config) (*App, error) {
	if cfg.ProjectRoot == "" {
		return nil, fmt.Errorf("project root required")
	}
	if cfg.ProjectID == "" {
		cfg.ProjectID = filepath.Base(cfg.ProjectRoot)
	}
	paths := NewPaths(cfg.ProjectRoot)
	if err := paths.EnsureDirs(); err != nil {
		return nil, fmt.Errorf("ensure .aoa dirs: %w", err)
	}
	if _, err := paths.Migrate(); err != nil {
		return nil, fmt.Errorf("migrate .aoa layout: %w", err)
	}

	if cfg.DBPath == "" {
		cfg.DBPath = paths.DB
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

	// Start with empty index — loaded in WarmCaches() after server is up
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
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

	// Create file cache and attach to search engine (not yet warmed)
	cache := index.NewFileCache(cfg.CacheMaxBytes)
	engine.SetCache(cache)

	// Start with fresh learner — state loaded in WarmCaches()
	lrn := learner.New()

	reader := claude.New(claude.Config{
		ProjectRoot: cfg.ProjectRoot,
	})

	// Status line goes alongside the DB in .aoa/
	statusPath := paths.Status

	a := &App{
		ProjectRoot:    cfg.ProjectRoot,
		ProjectID:      cfg.ProjectID,
		Paths:          paths,
		Store:          store,
		Watcher:        watcher,
		Enricher:       enr,
		debug:          cfg.Debug || os.Getenv("AOA_DEBUG") == "1",
		Engine:         engine,
		Learner:        lrn,
		Parser:         cfg.Parser, // nil = tokenization-only mode
		Reader:         reader,
		Index:          idx,
		statusLinePath: statusPath,
		httpPort:       cfg.HTTPPort,
		dbPath:         cfg.DBPath,
		toolMetrics: ToolMetrics{
			FileReads:    make(map[string]int),
			BashCommands: make(map[string]int),
			GrepPatterns: make(map[string]int),
		},
		turnBuffer:          make(map[string]*turnBuilder),
		shadowPending:       make(map[string]*ToolShadow),
		burnRate:            NewBurnRateTracker(5 * time.Minute),
		burnRateCounterfact: NewBurnRateTracker(5 * time.Minute),
		rateTracker:         NewRateTracker(30 * time.Minute),
	}

	// Create server with App as query provider (for domains, stats, etc.)
	sockPath := socket.SocketPath(cfg.ProjectRoot)
	a.Server = socket.NewServer(engine, idx, sockPath, a)

	// Create HTTP server for web dashboard
	a.WebServer = web.NewServer(a, idx, engine, paths.PortFile)

	// Wire search observer: search results → learning signals
	engine.SetObserver(a.searchObserver)

	// Load YAML-defined dimensional analysis rules and create engine
	a.initDimEngine()

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
	a.debugf("search query=%q hits=%d elapsed=%v", query, len(result.Hits), elapsed)
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

	// L9.6: Shim-level counterfactual — compute savings when results were truncated.
	// TotalMatchChars > returned chars means the search engine truncated results.
	if result.TotalMatchChars > 0 && len(result.Hits) > 0 {
		var returnedChars int
		for _, h := range result.Hits {
			returnedChars += len(h.File) + len(h.Content) + 10
		}
		saved := result.TotalMatchChars - returnedChars
		if saved > 0 {
			a.counterfactTokensSaved += int64(saved / 4)
			a.shadowRing.Push(ToolShadow{
				Timestamp:   time.Now(),
				Source:      "shim",
				Pattern:     query,
				ActualChars: result.TotalMatchChars,
				ShadowChars: returnedChars,
				CharsSaved:  saved,
				ShadowRan:   true,
			})
		}
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
	if a.debug {
		a.debugf("starting daemon — root=%s parser=%v dimEngine=%v",
			a.ProjectRoot, a.Parser != nil, a.dimEngine != nil)
	}
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
	// Watch .aoa/hook/ for context.jsonl and usage.txt (excluded by main ignore rules)
	if err := a.Watcher.WatchExtra(a.Paths.HookDir); err != nil {
		fmt.Printf("[warning] hook watcher unavailable: %v\n", err)
	}
	// Seed context snapshot from existing file (don't wait for next hook write)
	if _, err := os.Stat(a.Paths.ContextJSONL); err == nil {
		a.onContextFileChanged(a.Paths.ContextJSONL)
	}
	// Start session reader — tails Claude session logs for learning signals
	a.Reader.Start(a.onSessionEvent)
	// Write initial status line
	a.mu.Lock()
	a.writeStatus(nil)
	a.mu.Unlock()
	return nil
}

// WarmResult holds timing and stats from WarmCaches for structured logging.
type WarmResult struct {
	FileCount    int
	TokenCount   int
	PromptCount  int
	IndexTime    float64
	LearnerTime  float64
	CacheTime    float64
	ReconTime    float64
	ReconCached  int  // files loaded from bbolt
	ReconScanned int  // files re-analyzed
	FirstRun     bool // true if recon had no persisted data
	TotalTime    float64
}

// WarmCaches loads the persisted index and learner state from bbolt, warms the
// file cache from disk, and pre-computes the recon scan. This is IO-heavy and
// should be called after Start() so the daemon is already reachable.
// logFn receives progress messages during long-running phases.
// Returns a WarmResult with final stats for the caller to format a summary.
func (a *App) WarmCaches(logFn func(string)) WarmResult {
	var r WarmResult
	totalStart := time.Now()

	// 1. Load persisted index from bbolt
	logFn("loading index from database...")
	start := time.Now()
	idx, err := a.Store.LoadIndex(a.ProjectID)
	if err != nil {
		logFn(fmt.Sprintf("warning: failed to load index: %v", err))
	}
	if idx != nil {
		a.mu.Lock()
		a.Index.Tokens = idx.Tokens
		a.Index.Metadata = idx.Metadata
		a.Index.Files = idx.Files
		a.mu.Unlock()
		a.Engine.Rebuild()
		r.FileCount = len(idx.Files)
		r.TokenCount = len(idx.Tokens)
		logFn(fmt.Sprintf("index loaded: %d files, %d tokens (%.1fs)",
			r.FileCount, r.TokenCount, time.Since(start).Seconds()))
	} else {
		logFn(fmt.Sprintf("no persisted index (%.1fs)", time.Since(start).Seconds()))
	}
	r.IndexTime = time.Since(start).Seconds()

	// 2. Load persisted learner state
	start = time.Now()
	ls, err := a.Store.LoadLearnerState(a.ProjectID)
	if err == nil && ls != nil {
		a.mu.Lock()
		a.Learner = learner.NewFromState(ls)
		a.promptN = a.Learner.PromptCount()
		a.mu.Unlock()
		r.PromptCount = int(ls.PromptCount)
	}
	r.LearnerTime = time.Since(start).Seconds()

	if r.FileCount == 0 {
		logFn("no files in index, skipping cache warm")
		r.TotalTime = time.Since(totalStart).Seconds()
		return r
	}

	// 3. Warm file cache (always) and recon cache (only if recon is available).
	logFn(fmt.Sprintf("warming file cache (%d files)...", r.FileCount))
	var wg sync.WaitGroup

	wg.Add(1)
	fileCacheStart := time.Now()
	go func() {
		defer wg.Done()
		a.Engine.WarmCache()
	}()

	reconStart := time.Now()
	if a.dimEngine != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.ReconCached, r.ReconScanned, r.FirstRun = a.warmDimCache(logFn)
		}()
	}

	wg.Wait()
	r.CacheTime = time.Since(fileCacheStart).Seconds()
	r.ReconTime = time.Since(reconStart).Seconds()
	a.loadInvestigated()

	r.TotalTime = time.Since(totalStart).Seconds()
	return r
}

// RuleCount returns the number of dimensional analysis rules loaded.
func (a *App) RuleCount() int {
	return len(a.dimRules)
}

// indexSaveDebounce is the delay before flushing a dirty index to bbolt.
const indexSaveDebounce = 2 * time.Second

// markIndexDirty flags the index for deferred persistence.
// Resets the debounce timer so rapid changes coalesce into a single write.
// Must be called with a.mu held.
func (a *App) markIndexDirty() {
	a.indexDirty = true
	if a.indexSaveTimer != nil {
		a.indexSaveTimer.Stop()
	}
	a.indexSaveTimer = time.AfterFunc(indexSaveDebounce, func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.indexDirty {
			_ = a.Store.SaveIndex(a.ProjectID, a.Index)
			a.indexDirty = false
		}
	})
}

// flushIndex saves the index to bbolt immediately if dirty.
// Must be called with a.mu held.
func (a *App) flushIndex() {
	if a.indexSaveTimer != nil {
		a.indexSaveTimer.Stop()
		a.indexSaveTimer = nil
	}
	if a.indexDirty {
		_ = a.Store.SaveIndex(a.ProjectID, a.Index)
		a.indexDirty = false
	}
}

// Stop gracefully shuts down all services and persists learner state.
func (a *App) Stop() error {
	a.debugf("stopping daemon")
	a.Reader.Stop()
	a.Watcher.Stop()
	a.WebServer.Stop()
	a.Server.Stop()
	// Persist final state before shutdown
	a.mu.Lock()
	a.flushSessionSummary()
	a.flushIndex()
	_ = a.Store.SaveLearnerState(a.ProjectID, a.Learner.State())
	a.mu.Unlock()
	a.Store.Close()
	return nil
}

// onSessionEvent processes canonical session events from the Claude adapter.
// Extracts bigrams from conversation text and file access signals from tools.
// Thread-safe via mutex.
func (a *App) onSessionEvent(ev ports.SessionEvent) {
	a.debugf("session-event kind=%d", ev.Kind)
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
		// L9.4: Subagent user input — just count chars, skip turn tracking
		if ev.IsSubagent {
			a.meter.RecordSubagent(len(ev.Text))
			a.Learner.ProcessBigrams(ev.Text)
			break
		}
		// Flush the current exchange builder before starting new user input
		a.flushCurrentBuilder()
		a.promptN++
		a.sessionPrompts++
		a.sessionMetrics.TurnCount++
		a.meter.RecordUser(len(ev.Text))
		a.Learner.ProcessBigrams(ev.Text)
		a.writeStatus(nil)
		// Push user turn to ring
		a.pushTurn(ConversationTurn{
			TurnID:    ev.TurnID,
			Role:      "user",
			Text:      ev.Text,
			Timestamp: ts,
			Model:     ev.Model,
		})

	case ports.EventAIThinking:
		if ev.Text != "" {
			if ev.IsSubagent {
				a.meter.RecordSubagent(len(ev.Text))
			} else {
				a.meter.RecordThinking(len(ev.Text))
			}
			a.Learner.ProcessBigrams(ev.Text)
			tb := a.ensureTurnBuilder(ev.TurnID, ts, ev.Model)
			if tb.ThinkingText.Len() > 0 {
				tb.ThinkingText.WriteString("\n")
			}
			tb.ThinkingText.WriteString(ev.Text)
		}

	case ports.EventAIResponse:
		if ev.Text != "" {
			if ev.IsSubagent {
				a.meter.RecordSubagent(len(ev.Text))
			} else {
				a.meter.RecordAssistant(len(ev.Text))
			}
			a.Learner.ProcessBigrams(ev.Text)
		}
		if ev.IsSubagent {
			break // subagent responses don't accumulate into main session metrics
		}
		// Accumulate token usage (global + per-turn)
		tb := a.ensureTurnBuilder(ev.TurnID, ts, ev.Model)
		if ev.Usage != nil {
			a.meter.RecordAPI(ev.Usage.InputTokens, ev.Usage.OutputTokens, ev.Usage.CacheReadTokens)
			a.sessionMetrics.InputTokens += ev.Usage.InputTokens
			a.sessionMetrics.OutputTokens += ev.Usage.OutputTokens
			a.sessionMetrics.CacheReadTokens += ev.Usage.CacheReadTokens
			a.sessionMetrics.CacheWriteTokens += ev.Usage.CacheWriteTokens
			// Per-model token tracking
			total := ev.Usage.InputTokens + ev.Usage.OutputTokens + ev.Usage.CacheReadTokens + ev.Usage.CacheWriteTokens
			if model := ev.Model; model != "" && total > 0 {
				if a.sessionMetrics.ModelTokens == nil {
					a.sessionMetrics.ModelTokens = make(map[string]int64)
				}
				a.sessionMetrics.ModelTokens[model] += int64(total)
			}
			tb.InputTokens += ev.Usage.InputTokens
			tb.OutputTokens += ev.Usage.OutputTokens
			// L0.1: Record burn rate for both actual and counterfactual
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
				// tokens deferred to result_chars (actual result size, not full-codebase estimate)
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
				if ev.Tool.Input != nil {
					if desc, ok := ev.Tool.Input["description"].(string); ok && desc != "" {
						target = a.truncate(desc, 100)
					}
				}
				if target == "" && ev.Tool.Command != "" {
					target = a.truncate(ev.Tool.Command, 100)
				}
			}

			// L9.2: Extract tool detail fields
			var toolPattern, toolFilePath, toolCommand string
			if ev.Tool != nil {
				toolPattern = ev.Tool.Pattern
				toolCommand = ev.Tool.Command
			}
			if ev.File != nil {
				toolFilePath = ev.File.Path
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
				ToolID:      ev.Tool.ToolID,
				Pattern:     toolPattern,
				FilePath:    toolFilePath,
				Command:     toolCommand,
			})

			// L9.5: Create pending shadow for Grep/Glob tools
			if (action == "Grep" || action == "Glob") && ev.Tool.ToolID != "" && toolPattern != "" {
				a.shadowPending[ev.Tool.ToolID] = &ToolShadow{
					Timestamp: time.Now(),
					Source:    "session_log",
					ToolName:  action,
					ToolID:    ev.Tool.ToolID,
					Pattern:   toolPattern,
					FilePath:  toolFilePath,
				}
			}

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

	case ports.EventToolResult:
		// L9.1: Accumulate tool result chars into content meter
		if len(ev.ToolResultSizes) > 0 {
			var totalResultChars int
			for _, chars := range ev.ToolResultSizes {
				totalResultChars += chars
			}
			if totalResultChars > 0 {
				a.meter.RecordToolResult(totalResultChars)
			}
		}
		// L9.3: Accumulate persisted tool result chars
		if len(ev.ToolPersistedSizes) > 0 {
			for _, chars := range ev.ToolPersistedSizes {
				if chars > 0 {
					a.meter.RecordToolPersisted(chars)
				}
			}
		}
		// Correlate tool result sizes back to TurnActions via ToolID.
		// Tool results arrive in user messages; the matching tool_use was
		// in a prior assistant message. Walk all turn builders to find matches.
		if len(ev.ToolResultSizes) > 0 {
			// Check current in-progress builder first (most likely match)
			if a.currentBuilder != nil {
				for i := range a.currentBuilder.Actions {
					act := &a.currentBuilder.Actions[i]
					if act.ToolID != "" {
						if chars, ok := ev.ToolResultSizes[act.ToolID]; ok {
							act.ResultChars = chars
						}
					}
				}
			}
			// Also check pending turn builders (tool results may correlate
			// to actions in a different turn builder than current)
			for _, tb := range a.turnBuffer {
				for i := range tb.Actions {
					act := &tb.Actions[i]
					if act.ToolID != "" {
						if chars, ok := ev.ToolResultSizes[act.ToolID]; ok {
							act.ResultChars = chars
						}
					}
				}
			}

			// L9.5: Dispatch shadow search for pending Grep/Glob comparisons
			for toolID, chars := range ev.ToolResultSizes {
				if shadow, ok := a.shadowPending[toolID]; ok {
					shadow.ActualChars = chars
					delete(a.shadowPending, toolID)
					a.dispatchShadowSearch(shadow)
				}
			}
		}

	case ports.EventSystemMeta:
		// L9.1: Track active milliseconds
		if ev.DurationMs > 0 {
			a.meter.RecordActiveMs(ev.DurationMs)
		}
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

	var msPerToken float64
	if a.rateTracker != nil && a.rateTracker.HasData() {
		msPerToken = a.rateTracker.MsPerToken()
	}

	result := socket.RunwayResult{
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
		MsPerToken:         msPerToken,
		ReadCount:          a.sessionReadCount,
		GuidedReadCount:    a.sessionGuidedCount,
		CacheHitRate:       a.sessionMetrics.CacheHitRate(),
		// L9.7: Burst throughput from content meter
		BurstThroughput:   a.meter.BurstTokensPerSec(),
		ActiveMs:          a.meter.ActiveMs,
		TurnVelocities:    a.meter.TurnVelocities(),
		// L9.8: Shadow savings from shadow ring
		ShadowTotalSaved:  a.shadowRing.TotalCharsSaved() / 4, // chars → tokens
		ShadowSearchCount: a.shadowRing.Count(),
	}

	// Overlay real context data from status line hook if available
	if snap := a.latestContextSnapshot(); snap != nil {
		result.CtxUsed = snap.CtxUsed
		result.CtxMax = snap.CtxMax
		result.CtxUsedPct = snap.UsedPct
		result.CtxRemainingPct = snap.RemainingPct
		result.TotalCostUSD = snap.TotalCostUSD
		result.TotalDurationMs = snap.TotalDurationMs
		result.TotalApiDurationMs = snap.TotalApiDurationMs
		result.TotalLinesAdded = snap.TotalLinesAdded
		result.TotalLinesRemoved = snap.TotalLinesRemoved
		result.CtxSnapshotAge = time.Now().Unix() - snap.Timestamp
		// Override context window max with real value if available
		if snap.CtxMax > 0 {
			result.ContextWindowMax = int(snap.CtxMax)
		}
	}

	return result
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
			ModelTokens:      s.ModelTokens,
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
		Version:       version.Version,
		BuildDate:     version.BuildDate,
	}
}

// UsageQuota returns the parsed /usage output, or nil if not available.
// Implements socket.AppQueries.
func (a *App) UsageQuota() *socket.UsageQuotaResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.usageQuota == nil {
		return nil
	}

	result := &socket.UsageQuotaResult{
		CapturedAt: a.usageQuota.CapturedAt,
	}

	if t := a.usageQuota.Session; t != nil {
		result.Session = &socket.UsageQuotaTierResult{
			Label: t.Label, UsedPct: t.UsedPct,
			ResetsAt: t.ResetsAt, ResetEpoch: t.ResetEpoch, Timezone: t.Timezone,
		}
	}
	if t := a.usageQuota.WeeklyAll; t != nil {
		result.WeeklyAll = &socket.UsageQuotaTierResult{
			Label: t.Label, UsedPct: t.UsedPct,
			ResetsAt: t.ResetsAt, ResetEpoch: t.ResetEpoch, Timezone: t.Timezone,
		}
	}
	if t := a.usageQuota.WeeklySonnet; t != nil {
		result.WeeklySonnet = &socket.UsageQuotaTierResult{
			Label: t.Label, UsedPct: t.UsedPct,
			ResetsAt: t.ResetsAt, ResetEpoch: t.ResetEpoch, Timezone: t.Timezone,
		}
	}

	return result
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
		SessionStartTs:   a.currentSessionStart,
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
			actions = append(actions, a.actionToResult(act))
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
			actions = append(actions, a.actionToResult(act))
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

// actionToResult converts a TurnAction to a TurnActionResult for the wire format.
// Also looks up shadow data if available. Must be called with a.mu held.
func (a *App) actionToResult(act TurnAction) socket.TurnActionResult {
	r := socket.TurnActionResult{
		Tool:        act.Tool,
		Target:      act.Target,
		Range:       act.Range,
		Impact:      act.Impact,
		Attrib:      act.Attrib,
		Tokens:      act.Tokens,
		Savings:     act.Savings,
		TimeSavedMs: act.TimeSavedMs,
		ResultChars: act.ResultChars,
		Pattern:     act.Pattern,
		FilePath:    act.FilePath,
		Command:     act.Command,
	}
	// L9.5: Look up shadow data by ToolID
	if act.ToolID != "" {
		if shadow := a.shadowRing.FindByToolID(act.ToolID); shadow != nil {
			r.ShadowChars = shadow.ShadowChars
			r.ShadowSaved = shadow.CharsSaved
		}
	}
	return r
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
		// L9.1: Push turn snapshot to content meter ring
		a.meter.PushTurn(TurnSnapshot{
			Timestamp:       a.currentBuilder.Timestamp,
			DurationMs:      a.currentBuilder.DurationMs,
			UserChars:       0, // user chars recorded separately in EventUserInput
			AssistantChars:  a.currentBuilder.Text.Len(),
			ThinkingChars:   a.currentBuilder.ThinkingText.Len(),
			ToolResultChars: a.builderToolResultChars(a.currentBuilder),
			APIOutputTokens: a.currentBuilder.OutputTokens,
			ActionCount:     len(a.currentBuilder.Actions),
		})
		a.pushTurn(a.turnFromBuilder(a.currentBuilder))
		a.currentBuilder = nil
	}
}

// dispatchShadowSearch runs an async shadow search to compare native tool output
// with aOa's optimized search result. Must be called with a.mu held (shadow is
// already detached from shadowPending). The goroutine acquires a.mu only to push
// the result into the ring.
func (a *App) dispatchShadowSearch(shadow *ToolShadow) {
	if a.Engine == nil || shadow.Pattern == "" {
		return
	}
	engine := a.Engine
	go func(s *ToolShadow) {
		start := time.Now()
		result := engine.Search(s.Pattern, ports.SearchOptions{})
		var shadowChars int
		if result != nil {
			for _, h := range result.Hits {
				shadowChars += len(h.File) + len(h.Content) + 10
			}
		}
		s.ShadowChars = shadowChars
		s.ShadowRan = true
		s.ShadowMs = time.Since(start).Milliseconds()
		s.CharsSaved = s.ActualChars - s.ShadowChars

		a.mu.Lock()
		a.shadowRing.Push(*s)
		if s.CharsSaved > 0 {
			a.counterfactTokensSaved += int64(s.CharsSaved / 4)
		}
		a.mu.Unlock()
	}(shadow)
}

// builderToolResultChars sums ResultChars from all actions in a turn builder.
// Must be called with a.mu held.
func (a *App) builderToolResultChars(tb *turnBuilder) int {
	var total int
	for _, act := range tb.Actions {
		total += act.ResultChars
	}
	return total
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
	// L9.1: Reset content meter on session boundary
	a.meter.Reset()
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
		ModelTokens:      a.sessionMetrics.ModelTokens,
	}
	_ = a.Store.SaveSessionSummary(a.ProjectID, summary)
}

// turnFromBuilder converts a turnBuilder into a ConversationTurn.
func (a *App) turnFromBuilder(tb *turnBuilder) ConversationTurn {
	return ConversationTurn{
		TurnID:       tb.TurnID,
		Role:         "assistant",
		Text:         tb.Text.String(),
		ThinkingText: tb.ThinkingText.String(),
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
	a.debugf("reindex starting")
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

	// Re-scan dimensional analysis in the background after full reindex
	if a.dimEngine != nil {
		go a.warmDimCache(func(string) {})
	}

	elapsed := time.Since(start)
	return socket.ReindexResult{
		FileCount:   stats.FileCount,
		SymbolCount: stats.SymbolCount,
		TokenCount:  stats.TokenCount,
		ElapsedMs:   elapsed.Milliseconds(),
	}, nil
}

// updateReconOrDimForFile dispatches to the dimensional engine for file analysis.
// No-op when dimEngine is not available (pure aOa mode).
func (a *App) updateReconOrDimForFile(fileID uint32, relPath string) {
	if a.dimEngine != nil {
		a.debugf("dim-update file=%s id=%d", relPath, fileID)
		a.updateDimForFile(fileID, relPath)
	}
}

// InvestigatedFiles returns the current set of investigated file paths.
// Implements socket.AppQueries.
func (a *App) InvestigatedFiles() []string {
	a.reconMu.RLock()
	defer a.reconMu.RUnlock()
	now := time.Now().Unix()
	weekSec := int64(7 * 24 * 60 * 60)
	var files []string
	for path, ts := range a.investigatedFiles {
		if now-ts < weekSec {
			files = append(files, path)
		}
	}
	return files
}

// SetFileInvestigated marks or unmarks a file as investigated.
func (a *App) SetFileInvestigated(relPath string, investigated bool) {
	a.reconMu.Lock()
	defer a.reconMu.Unlock()
	if a.investigatedFiles == nil {
		a.investigatedFiles = make(map[string]int64)
	}
	if investigated {
		a.investigatedFiles[relPath] = time.Now().Unix()
	} else {
		delete(a.investigatedFiles, relPath)
	}
	a.saveInvestigated()
}

// ClearInvestigated removes all investigation markers.
func (a *App) ClearInvestigated() {
	a.reconMu.Lock()
	defer a.reconMu.Unlock()
	a.investigatedFiles = make(map[string]int64)
	a.saveInvestigated()
}

// saveInvestigated persists the investigated files to .aoa/recon/investigated.json.
// Must be called under reconMu.Lock().
func (a *App) saveInvestigated() {
	data, _ := json.Marshal(a.investigatedFiles)
	os.WriteFile(a.Paths.ReconInvestigated, data, 0644)
}

// loadInvestigated loads investigated files from disk, pruning expired entries.
func (a *App) loadInvestigated() {
	path := a.Paths.ReconInvestigated
	data, err := os.ReadFile(path)
	if err != nil {
		a.investigatedFiles = make(map[string]int64)
		return
	}
	var files map[string]int64
	if err := json.Unmarshal(data, &files); err != nil {
		a.investigatedFiles = make(map[string]int64)
		return
	}
	// Prune expired (>1 week)
	now := time.Now().Unix()
	weekSec := int64(7 * 24 * 60 * 60)
	for path, ts := range files {
		if now-ts >= weekSec {
			delete(files, path)
		}
	}
	a.investigatedFiles = files
}

// clearFileInvestigated removes investigation status for a file (called on file change).
func (a *App) clearFileInvestigated(relPath string) {
	a.reconMu.Lock()
	defer a.reconMu.Unlock()
	if _, ok := a.investigatedFiles[relPath]; ok {
		delete(a.investigatedFiles, relPath)
		a.saveInvestigated()
	}
}

// WipeProject deletes all persisted data and resets in-memory state.
// Implements socket.AppQueries.
func (a *App) WipeProject() error {
	a.debugf("wipe-project starting")
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

	// Reset dimensional cache
	a.reconMu.Lock()
	a.dimCache = nil
	a.dimCacheSet = false
	a.reconMu.Unlock()

	return nil
}
