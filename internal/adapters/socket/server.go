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

	"github.com/corey/aoa/internal/domain/index"
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
}

// Server is the daemon that listens on a Unix socket and serves search requests.
type Server struct {
	engine   *index.SearchEngine
	idx      *ports.Index
	queries  AppQueries
	listener net.Listener
	sockPath string
	started  time.Time

	done         chan struct{}
	shutdownCh   chan struct{} // closed when a remote shutdown request is received
	shutdownOnce sync.Once
	stopOnce     sync.Once
	wg           sync.WaitGroup
}

// NewServer creates a daemon server backed by the given search engine.
// The queries parameter may be nil if learner/wipe features are not needed.
func NewServer(engine *index.SearchEngine, idx *ports.Index, sockPath string, queries AppQueries) *Server {
	return &Server{
		engine:     engine,
		idx:        idx,
		queries:    queries,
		sockPath:   sockPath,
		done:       make(chan struct{}),
		shutdownCh: make(chan struct{}),
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

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB max message

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeResponse(conn, Response{Error: "invalid request JSON"})
			continue
		}

		resp := s.handleRequest(req)
		s.writeResponse(conn, resp)

		if req.Method == MethodShutdown {
			s.shutdownOnce.Do(func() { close(s.shutdownCh) })
			return
		}
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
	result := s.engine.Search(params.Query, params.Options)
	elapsed := time.Since(start)

	hits := make([]SearchHit, len(result.Hits))
	for i, h := range result.Hits {
		hits[i] = SearchHit{
			File:    h.File,
			Line:    h.Line,
			Symbol:  h.Symbol,
			Range:   h.Range,
			Domain:  h.Domain,
			Tags:    h.Tags,
			Kind:    h.Kind,
			Content: h.Content,
		}
	}

	return Response{
		ID: req.ID,
		Result: SearchResult{
			Hits:     hits,
			Count:    result.Count,
			ExitCode: result.ExitCode,
			Elapsed:  elapsed.String(),
		},
	}
}

func (s *Server) handleHealth(req Request) Response {
	fileCount := 0
	tokenCount := 0
	if s.idx != nil {
		fileCount = len(s.idx.Files)
		tokenCount = len(s.idx.Tokens)
	}

	return Response{
		ID: req.ID,
		Result: HealthResult{
			Status:     "ok",
			FileCount:  fileCount,
			TokenCount: tokenCount,
			Uptime:     time.Since(s.started).Round(time.Second).String(),
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

func (s *Server) handleWipe(req Request) Response {
	if s.queries == nil {
		return Response{ID: req.ID, Error: "wipe not available"}
	}

	if err := s.queries.WipeProject(); err != nil {
		return Response{ID: req.ID, Error: err.Error()}
	}
	return Response{ID: req.ID, Result: struct{}{}}
}

func (s *Server) writeResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	data = append(data, '\n')
	conn.Write(data)
}
