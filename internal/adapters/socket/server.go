package socket

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/corey/aoa/internal/peek"
	"github.com/corey/aoa/internal/ports"
)

// AppQueries provides read access to app state for server handlers.
// Thread safety is the implementor's responsibility.
type AppQueries interface {
	LearnerSnapshot() *ports.LearnerState
	WipeProject() error
	Reindex() (ReindexResult, error)
	SessionMetricsSnapshot() SessionMetricsResult
	ToolMetricsSnapshot() ToolMetricsResult
	ConversationTurns() ConversationFeedResult
	ActivityFeed() ActivityFeedResult
	TopKeywords(limit int) TopItemsResult
	TopTerms(limit int) TopItemsResult
	TopFiles(limit int) TopItemsResult
	DomainTermNames(domain string) []string
	DomainTermHitCounts(domain string) map[string]int
	RunwayProjection() RunwayResult
	SessionList() SessionListResult
	ProjectConfig() ProjectConfigResult
	ReconAvailable() bool
	DimensionalResults() map[string]*DimensionalFileResult
	InvestigatedFiles() []string
	SetFileInvestigated(relPath string, investigated bool)
	ClearInvestigated()
	UsageQuota() *UsageQuotaResult
	DimScanProgress() DimScanProgress
	GenerateHints(query string, opts ports.SearchOptions) []string
	TelemetrySnapshot() TelemetryResult
	// Arch returns the arch querier for the project (L19.14 / L19.16 prep).
	// Returns nil when the arch flag is off (C4) or no substrate is available.
	// L19.16 adds dispatch arms — this accessor enables them without further
	// interface changes.
	Arch() ports.ArchQuerier
	// IndexQuerier returns the index querier for peek and refs HTTP routes.
	// Returns nil when the index is not yet populated.
	// Routes backed by this interface work in all builds (lean and full).
	IndexQuerier() ports.IndexQuerier
}

// Server is the daemon that listens on a Unix socket and serves search requests.
type Server struct {
	searcher ports.Searcher
	idx      *ports.Index
	queries  AppQueries
	listener net.Listener
	sockPath string
	started  time.Time
	logFn    func(string, ...interface{}) // optional error logger

	done         chan struct{}
	shutdownCh   chan struct{} // closed when a remote shutdown request is received
	shutdownOnce sync.Once
	stopOnce     sync.Once
	wg           sync.WaitGroup

	healthFn func() string       // optional real status source; nil => "ok"
	probesFn func() (bool, bool) // optional (dbOK, webOK) probe source; nil => derive from status
}

// SetHealthFn supplies a status provider for the health check so the daemon can
// report a real status (e.g. "recovered", "unhealthy") derived from store state
// instead of a hardcoded literal. Pass nil to keep the default "ok".
func (s *Server) SetHealthFn(fn func() string) {
	s.healthFn = fn
}

// SetProbesFn supplies the (dbOK, webOK) probe source for tri-state health
// (L21.2). Same closure-injection pattern as SetHealthFn: the app wires real
// probes; the socket server never holds a store or HTTP handle (G4). Pass nil
// to derive both from the status string.
func (s *Server) SetProbesFn(fn func() (bool, bool)) {
	s.probesFn = fn
}

// NewServer creates a daemon server backed by the given searcher.
// The queries parameter may be nil if learner/wipe features are not needed.
func NewServer(searcher ports.Searcher, idx *ports.Index, sockPath string, queries AppQueries) *Server {
	return &Server{
		searcher:   searcher,
		idx:        idx,
		queries:    queries,
		sockPath:   sockPath,
		done:       make(chan struct{}),
		shutdownCh: make(chan struct{}),
	}
}

// SetLogFn sets a logger for server errors (writeResponse failures, connection errors).
// Lazy formatting — no allocations on success paths.
func (s *Server) SetLogFn(fn func(string, ...interface{})) {
	s.logFn = fn
}

func (s *Server) logErr(format string, args ...interface{}) {
	if s.logFn != nil {
		s.logFn(format, args...)
	}
}

// Start begins listening on the Unix socket. It handles stale sockets by
// attempting a connection first — if the connection fails, the stale socket
// is removed before binding.
func (s *Server) Start() error {
	// Handle stale socket
	if _, err := os.Stat(s.sockPath); err == nil {
		conn, err := net.DialTimeout("unix", s.sockPath, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return fmt.Errorf("daemon already running at %s", s.sockPath)
		}
		// Stale socket — remove it
		os.Remove(s.sockPath)
	}

	ln, err := net.Listen("unix", s.sockPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = ln
	s.started = time.Now()

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop gracefully shuts down the server, closing the listener and removing the socket file.
// Idempotent — safe to call multiple times (e.g., after remote shutdown + signal).
func (s *Server) Stop() error {
	s.stopOnce.Do(func() {
		close(s.done)
		if s.listener != nil {
			s.listener.Close()
		}
		s.wg.Wait()
		os.Remove(s.sockPath)
	})
	return nil
}

// ShutdownCh returns a channel that is closed when a remote shutdown request
// is received. The daemon's main goroutine should select on this alongside
// OS signals so the process actually exits after a remote stop.
func (s *Server) ShutdownCh() <-chan struct{} {
	return s.shutdownCh
}

// Addr returns the socket path the server is listening on.
func (s *Server) Addr() string {
	return s.sockPath
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				continue
			}
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Prevent goroutine leak from clients that connect but never send.
	// 10s is 100x the normal case (<100ms round-trip).
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB max message

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Refresh deadline after each successful read.
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(conn, Response{Error: "invalid request JSON"})
			continue
		}

		resp := s.handleRequest(req)

		// Close shutdown channel before writing the response so that
		// ShutdownCh() is readable by the time the client returns.
		if req.Method == MethodShutdown {
			s.shutdownOnce.Do(func() { close(s.shutdownCh) })
		}

		s.writeResponse(conn, resp)

		if req.Method == MethodShutdown {
			return
		}
	}
	if err := scanner.Err(); err != nil {
		s.logErr("connection read: %v", err)
	}
}

func (s *Server) handleRequest(req Request) Response {
	switch req.Method {
	case MethodSearch:
		return s.handleSearch(req)
	case MethodHealth:
		return s.handleHealth(req)
	case MethodShutdown:
		return Response{ID: req.ID, Result: struct{}{}}
	case MethodFiles:
		return s.handleFiles(req)
	case MethodDomains:
		return s.handleDomains(req)
	case MethodBigrams:
		return s.handleBigrams(req)
	case MethodStats:
		return s.handleStats(req)
	case MethodWipe:
		return s.handleWipe(req)
	case MethodReindex:
		return s.handleReindex(req)
	case MethodPeek:
		return s.handlePeek(req)
	// L19.16: six arch dispatch arms (before default — each nil-checks Arch() for C4).
	case MethodArchViews:
		return s.handleArchViews(req)
	case MethodArchView:
		return s.handleArchView(req)
	case MethodArchFindings:
		return s.handleArchFindings(req)
	case MethodArchJourney:
		return s.handleArchJourney(req)
	case MethodArchDerive:
		return s.handleArchDerive(req)
	case MethodArchFacts:
		return s.handleArchFacts(req)
	default:
		return Response{ID: req.ID, Error: fmt.Sprintf("unknown method: %s", req.Method)}
	}
}

func (s *Server) handleSearch(req Request) Response {
	// Re-marshal params to decode into SearchParams
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return Response{ID: req.ID, Error: "invalid search params"}
	}
	var params SearchParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return Response{ID: req.ID, Error: "invalid search params"}
	}

	start := time.Now()
	result := s.searcher.Search(params.Query, params.Options)
	elapsed := time.Since(start)

	hits := make([]SearchHit, len(result.Hits))
	for i, h := range result.Hits {
		hits[i] = SearchHit{
			File:         h.File,
			Line:         h.Line,
			Symbol:       h.Symbol,
			Range:        h.Range,
			Domain:       h.Domain,
			Tags:         h.Tags,
			Kind:         h.Kind,
			Content:      h.Content,
			ContextLines: h.ContextLines,
			PeekCode:     h.PeekCode,
		}
	}

	sr := SearchResult{
		Hits:     hits,
		Count:    result.Count,
		ExitCode: result.ExitCode,
		Elapsed:  elapsed.String(),
	}
	if len(hits) == 0 && s.queries != nil {
		sr.Hints = s.queries.GenerateHints(params.Query, params.Options)
	}
	return Response{ID: req.ID, Result: sr}
}

func (s *Server) handleHealth(req Request) Response {
	fileCount := 0
	tokenCount := 0
	if s.idx != nil {
		fileCount = len(s.idx.Files)
		tokenCount = len(s.idx.Tokens)
	}

	status := "ok"
	if s.healthFn != nil {
		status = s.healthFn()
	}

	// Tri-state probes (L21.2). No probe source wired => derive from status so
	// a healthy daemon never misreports "down" as zero-values (version skew).
	dbOK, webOK := status == "ok" || status == "recovered", status == "ok" || status == "recovered"
	if s.probesFn != nil {
		dbOK, webOK = s.probesFn()
	}

	return Response{
		ID: req.ID,
		Result: HealthResult{
			Status:     status,
			FileCount:  fileCount,
			TokenCount: tokenCount,
			Uptime:     time.Since(s.started).Round(time.Second).String(),
			DaemonOK:   true, // a served response proves the daemon
			DBOK:       dbOK,
			WebOK:      webOK,
		},
	}
}

func (s *Server) handleFiles(req Request) Response {
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return Response{ID: req.ID, Error: "invalid files params"}
	}
	var params FilesParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return Response{ID: req.ID, Error: "invalid files params"}
	}

	var files []FileInfo
	for _, fm := range s.idx.Files {
		match := false
		if params.Glob != "" {
			base := filepath.Base(fm.Path)
			if ok, _ := filepath.Match(params.Glob, base); ok {
				match = true
			}
			if !match {
				if ok, _ := filepath.Match(params.Glob, fm.Path); ok {
					match = true
				}
			}
		}
		if params.Name != "" {
			base := filepath.Base(fm.Path)
			if strings.Contains(strings.ToLower(base), strings.ToLower(params.Name)) {
				match = true
			}
		}
		if params.Glob == "" && params.Name == "" {
			match = true
		}
		if match {
			files = append(files, FileInfo{
				Path:     fm.Path,
				Language: fm.Language,
				Domain:   fm.Domain,
			})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return Response{
		ID: req.ID,
		Result: FilesResult{
			Files: files,
			Count: len(files),
		},
	}
}

func (s *Server) handleDomains(req Request) Response {
	if s.queries == nil {
		return Response{ID: req.ID, Error: "learner not available"}
	}

	state := s.queries.LearnerSnapshot()

	var domains []DomainInfo
	var coreCount int
	for name, dm := range state.DomainMeta {
		domains = append(domains, DomainInfo{
			Name:     name,
			Hits:     dm.Hits,
			Tier:     dm.Tier,
			State:    dm.State,
			Source:   dm.Source,
			Terms:    s.queries.DomainTermNames(name),
			TermHits: s.queries.DomainTermHitCounts(name),
		})
		if dm.Tier == "core" {
			coreCount++
		}
	}

	sort.Slice(domains, func(i, j int) bool {
		if domains[i].Hits != domains[j].Hits {
			return domains[i].Hits > domains[j].Hits
		}
		return domains[i].Name < domains[j].Name
	})

	return Response{
		ID: req.ID,
		Result: DomainsResult{
			Domains:   domains,
			Count:     len(domains),
			CoreCount: coreCount,
		},
	}
}

func (s *Server) handleBigrams(req Request) Response {
	if s.queries == nil {
		return Response{ID: req.ID, Error: "learner not available"}
	}

	state := s.queries.LearnerSnapshot()

	return Response{
		ID: req.ID,
		Result: BigramsResult{
			Bigrams:         state.Bigrams,
			Count:           len(state.Bigrams),
			CohitKwTerm:     state.CohitKwTerm,
			CohitTermDomain: state.CohitTermDomain,
			CohitKwCount:    len(state.CohitKwTerm),
			CohitTdCount:    len(state.CohitTermDomain),
		},
	}
}

func (s *Server) handleStats(req Request) Response {
	if s.queries == nil {
		return Response{ID: req.ID, Error: "learner not available"}
	}

	state := s.queries.LearnerSnapshot()

	coreCount := 0
	contextCount := 0
	for _, dm := range state.DomainMeta {
		switch dm.Tier {
		case "core":
			coreCount++
		case "context":
			contextCount++
		}
	}

	return Response{
		ID: req.ID,
		Result: StatsResult{
			PromptCount:  state.PromptCount,
			DomainCount:  len(state.DomainMeta),
			CoreCount:    coreCount,
			ContextCount: contextCount,
			KeywordCount: len(state.KeywordHits),
			TermCount:    len(state.TermHits),
			BigramCount:  len(state.Bigrams),
			FileHitCount: len(state.FileHits),
			IndexFiles:   len(s.idx.Files),
			IndexTokens:  len(s.idx.Tokens),
		},
	}
}

func (s *Server) handleReindex(req Request) Response {
	if s.queries == nil {
		return Response{ID: req.ID, Error: "reindex not available"}
	}

	result, err := s.queries.Reindex()
	if err != nil {
		return Response{ID: req.ID, Error: err.Error()}
	}
	return Response{ID: req.ID, Result: result}
}

func (s *Server) handlePeek(req Request) Response {
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return Response{ID: req.ID, Error: "invalid peek params"}
	}
	var params PeekParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return Response{ID: req.ID, Error: "invalid peek params"}
	}

	root := s.searcher.ProjectRoot()
	symbols := make([]PeekSymbol, len(params.Codes))

	for i, code := range params.Codes {
		symbols[i].Code = code

		fileID, startLine, err := peek.Decode(code)
		if err != nil {
			symbols[i].Error = err.Error()
			continue
		}

		ref := ports.TokenRef{FileID: fileID, Line: startLine}
		sym := s.idx.Metadata[ref]
		if sym == nil {
			symbols[i].Error = "symbol not found"
			continue
		}
		file := s.idx.Files[fileID]
		if file == nil {
			symbols[i].Error = "file not found"
			continue
		}

		domain, tags := s.searcher.EnrichRef(ref)
		symbols[i].File = file.Path
		symbols[i].Symbol = s.searcher.FormatSymbol(sym)
		symbols[i].Range = [2]int{int(sym.StartLine), int(sym.EndLine)}
		symbols[i].Domain = domain
		symbols[i].Tags = tags

		// Read source lines from disk
		absPath := filepath.Join(root, file.Path)
		data, err := os.ReadFile(absPath)
		if err != nil {
			symbols[i].Error = fmt.Sprintf("read file: %v", err)
			continue
		}
		allLines := strings.Split(string(data), "\n")
		start := int(sym.StartLine) - 1 // 0-indexed
		end := int(sym.EndLine)          // exclusive
		if start < 0 {
			start = 0
		}
		if end > len(allLines) {
			end = len(allLines)
		}
		symbols[i].Lines = allLines[start:end]
	}

	return Response{ID: req.ID, Result: PeekResult{Symbols: symbols}}
}

func (s *Server) handleWipe(req Request) Response {
	if s.queries == nil {
		return Response{ID: req.ID, Error: "wipe not available"}
	}

	if err := s.queries.WipeProject(); err != nil {
		return Response{ID: req.ID, Error: err.Error()}
	}
	return Response{ID: req.ID, Result: struct{}{}}
}

// ── L19.16 arch handlers ─────────────────────────────────────────────────────
// Each handler nil-checks s.queries.Arch() first (C4: when arch is off, Arch()
// returns nil and we return a clear error — the default arm is NOT hit).

// archQuerier returns the arch querier or nil with a ready error Response.
func (s *Server) archQuerier(id string) (ports.ArchQuerier, *Response) {
	if s.queries == nil {
		r := Response{ID: id, Error: "arch not available: no queries"}
		return nil, &r
	}
	q := s.queries.Arch()
	if q == nil {
		r := Response{ID: id, Error: "arch not available (C4: arch flag off or no substrate)"}
		return nil, &r
	}
	return q, nil
}

func (s *Server) handleArchViews(req Request) Response {
	q, errResp := s.archQuerier(req.ID)
	if errResp != nil {
		return *errResp
	}

	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return Response{ID: req.ID, Error: "invalid arch.views params"}
	}
	var params ArchViewsParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return Response{ID: req.ID, Error: "invalid arch.views params"}
	}
	scope := params.Scope
	if scope == "" {
		scope = "local"
	}

	m, err := q.Manifest(scope)
	if err != nil {
		return Response{ID: req.ID, Error: fmt.Sprintf("arch.views: %v", err)}
	}
	if m == nil {
		return Response{ID: req.ID, Result: ArchViewsResult{HasData: false}}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return Response{ID: req.ID, Error: fmt.Sprintf("arch.views: marshal: %v", err)}
	}
	return Response{ID: req.ID, Result: ArchViewsResult{Raw: raw, HasData: true}}
}

func (s *Server) handleArchView(req Request) Response {
	q, errResp := s.archQuerier(req.ID)
	if errResp != nil {
		return *errResp
	}

	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return Response{ID: req.ID, Error: "invalid arch.view params"}
	}
	var params ArchViewParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return Response{ID: req.ID, Error: "invalid arch.view params"}
	}
	if params.View == "" {
		return Response{ID: req.ID, Error: "arch.view: 'view' param required"}
	}
	scope := params.Scope
	if scope == "" {
		scope = "local"
	}

	data, err := q.View(scope, params.View)
	if err != nil {
		return Response{ID: req.ID, Error: fmt.Sprintf("arch.view: %v", err)}
	}
	if data == nil {
		return Response{ID: req.ID, Result: ArchViewResult{Found: false}}
	}
	return Response{ID: req.ID, Result: ArchViewResult{Raw: json.RawMessage(data), Found: true}}
}

func (s *Server) handleArchFindings(req Request) Response {
	q, errResp := s.archQuerier(req.ID)
	if errResp != nil {
		return *errResp
	}

	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return Response{ID: req.ID, Error: "invalid arch.findings params"}
	}
	var params ArchFindingsParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return Response{ID: req.ID, Error: "invalid arch.findings params"}
	}
	scope := params.Scope
	if scope == "" {
		scope = "local"
	}

	data, err := q.Findings(scope)
	if err != nil {
		return Response{ID: req.ID, Error: fmt.Sprintf("arch.findings: %v", err)}
	}
	hasNew := len(data) > 2 // non-empty JSON array
	if data == nil {
		data = []byte("[]")
	}
	return Response{ID: req.ID, Result: ArchFindingsResult{
		Raw:    json.RawMessage(data),
		HasNew: hasNew,
	}}
}

func (s *Server) handleArchJourney(req Request) Response {
	// MethodArchJourney exists as a protocol slot; full journey semantics land
	// in a future task. Return a clear "not yet" rather than "unknown method".
	if s.queries != nil && s.queries.Arch() == nil {
		return Response{ID: req.ID, Error: "arch not available (C4: arch flag off or no substrate)"}
	}
	return Response{ID: req.ID, Error: "arch.journey: not yet implemented in this release"}
}

func (s *Server) handleArchDerive(req Request) Response {
	q, errResp := s.archQuerier(req.ID)
	if errResp != nil {
		return *errResp
	}

	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return Response{ID: req.ID, Error: "invalid arch.derive params"}
	}
	var params ArchDeriveParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return Response{ID: req.ID, Error: "invalid arch.derive params"}
	}
	if params.From == "" || params.To == "" {
		return Response{ID: req.ID, Error: "arch.derive: 'from' and 'to' params required"}
	}
	scope := params.Scope
	if scope == "" {
		scope = "local"
	}
	k := params.K
	if k <= 0 {
		k = 10 // default hop budget
	}

	path, err := q.Derive(scope, params.From, params.To, k)
	if err != nil {
		return Response{ID: req.ID, Error: fmt.Sprintf("arch.derive: %v", err)}
	}
	if path == nil {
		return Response{ID: req.ID, Result: ArchDeriveResult{Found: false}}
	}
	return Response{ID: req.ID, Result: ArchDeriveResult{Path: path, Found: true}}
}

func (s *Server) handleArchFacts(req Request) Response {
	q, errResp := s.archQuerier(req.ID)
	if errResp != nil {
		return *errResp
	}

	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return Response{ID: req.ID, Error: "invalid arch.facts params"}
	}
	var params ArchFactsParams
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return Response{ID: req.ID, Error: "invalid arch.facts params"}
	}
	if params.Subject == "" {
		return Response{ID: req.ID, Error: "arch.facts: 'subject' param required"}
	}
	scope := params.Scope
	if scope == "" {
		scope = "local"
	}

	data, err := q.Facts(scope, params.Subject, params.Limit)
	if err != nil {
		return Response{ID: req.ID, Error: fmt.Sprintf("arch.facts: %v", err)}
	}

	// Count entries from raw JSON array.
	var entries []json.RawMessage
	if data != nil {
		_ = json.Unmarshal(data, &entries) // count-only; ignore error
	}
	if data == nil {
		data = []byte("[]")
	}
	return Response{ID: req.ID, Result: ArchFactsResult{
		Facts: json.RawMessage(data),
		Count: len(entries),
	}}
}

func (s *Server) writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		s.logErr("marshal response: %v", err)
		return
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		s.logErr("write response: %v", err)
	}
}
