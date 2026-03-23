package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/ports"
)

// Server serves the web dashboard and JSON API over HTTP.
type Server struct {
	queries   socket.AppQueries
	idx       *ports.Index
	lineCache ports.LineCache
	listener  net.Listener
	httpSrv   *http.Server
	port      int
	started   time.Time
	stopOnce  sync.Once

	portFilePath string          // .aoa/http.port
	revisionFn   func() uint64  // L17.6: returns current global state revision
}

// NewServer creates an HTTP server for the dashboard.
// The portFilePath is where the bound port is written for discovery.
// lineCache may be nil if source line serving is not needed.
func NewServer(queries socket.AppQueries, idx *ports.Index, lineCache ports.LineCache, portFilePath string) *Server {
	return &Server{
		queries:      queries,
		idx:          idx,
		lineCache:    lineCache,
		portFilePath: portFilePath,
	}
}

// SetRevisionSource configures the global state revision function for ETag gating (L17.6).
// If not set, ETag middleware is a no-op.
func (s *Server) SetRevisionSource(fn func() uint64) {
	s.revisionFn = fn
}

// DefaultPort computes a project-specific port: 19000 + (hash(abs_path) % 1000).
func DefaultPort(projectRoot string) int {
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		abs = projectRoot
	}
	h := sha256.Sum256([]byte(abs))
	// Use first 4 bytes as uint32
	n := uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
	return 19000 + int(n%1000)
}

// Start begins listening on the preferred port. Writes the port to .aoa/http.port.
func (s *Server) Start(preferredPort int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferredPort)) // localhost only
	if err != nil {
		return fmt.Errorf("listen 127.0.0.1:%d: %w", preferredPort, err)
	}
	s.listener = ln
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.started = time.Now()

	mux := http.NewServeMux()

	// Serve static files from embedded FS (strip "static/" prefix so / → static/index.html).
	// no-cache ensures the browser revalidates on every load — after a daemon restart
	// with new embedded files, the user gets them without a hard refresh.
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("create static sub-fs: %w", err)
	}
	staticHandler := http.FileServerFS(staticSub)
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		staticHandler.ServeHTTP(w, r)
	}))

	mux.HandleFunc("GET /api/telemetry", s.withETag(s.handleTelemetry))
	mux.HandleFunc("GET /api/health", s.handleHealth) // no ETag — liveness check
	mux.HandleFunc("GET /api/stats", s.withETag(s.handleStats))
	mux.HandleFunc("GET /api/domains", s.withETag(s.handleDomains))
	mux.HandleFunc("GET /api/bigrams", s.withETag(s.handleBigrams))
	mux.HandleFunc("GET /api/conversation/metrics", s.withETag(s.handleConvMetrics))
	mux.HandleFunc("GET /api/conversation/tools", s.withETag(s.handleConvTools))
	mux.HandleFunc("GET /api/conversation/feed", s.withETag(s.handleConvFeed))
	mux.HandleFunc("GET /api/top-keywords", s.withETag(s.handleTopKeywords))
	mux.HandleFunc("GET /api/top-terms", s.withETag(s.handleTopTerms))
	mux.HandleFunc("GET /api/top-files", s.withETag(s.handleTopFiles))
	mux.HandleFunc("GET /api/activity/feed", s.withETag(s.handleActivityFeed))
	mux.HandleFunc("GET /api/runway", s.withETag(s.handleRunway))
	mux.HandleFunc("GET /api/sessions", s.withETag(s.handleSessions))
	mux.HandleFunc("GET /api/config", s.withETag(s.handleConfig))
	mux.HandleFunc("GET /api/recon", s.withETag(s.handleRecon))
	mux.HandleFunc("GET /api/recon/summary", s.withETag(s.handleReconSummary))
	mux.HandleFunc("GET /api/recon/tree", s.withETag(s.handleReconTree))
	mux.HandleFunc("GET /api/recon/findings", s.withETag(s.handleReconFindings))
	mux.HandleFunc("POST /api/recon-investigate", s.handleReconInvestigate) // POST — no ETag
	mux.HandleFunc("GET /api/source-line", s.handleSourceLine) // no ETag — file content
	mux.HandleFunc("GET /api/usage", s.withETag(s.handleUsage))

	s.httpSrv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Write port file for discovery
	if s.portFilePath != "" {
		os.WriteFile(s.portFilePath, []byte(fmt.Sprintf("%d", s.port)), 0644)
	}

	go s.httpSrv.Serve(ln)
	return nil
}

// Stop gracefully shuts down the HTTP server. Idempotent.
func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		if s.httpSrv != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			s.httpSrv.Shutdown(ctx)
		}
		if s.portFilePath != "" {
			os.Remove(s.portFilePath)
		}
	})
}

// Port returns the bound port number.
func (s *Server) Port() int {
	return s.port
}

// URL returns the dashboard URL.
func (s *Server) URL() string {
	return fmt.Sprintf("http://localhost:%d", s.port)
}

// withETag wraps a handler with ETag/revision gating (L17.6).
// If the client's If-None-Match header matches the current revision, returns 304.
// Otherwise, sets the ETag header and calls the underlying handler.
func (s *Server) withETag(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.revisionFn == nil {
			next(w, r)
			return
		}
		rev := strconv.FormatUint(s.revisionFn(), 10)
		if r.Header.Get("If-None-Match") == rev {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", rev)
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	fileCount := 0
	tokenCount := 0
	if s.idx != nil {
		fileCount = len(s.idx.Files)
		tokenCount = len(s.idx.Tokens)
	}

	result := socket.HealthResult{
		Status:     "ok",
		FileCount:  fileCount,
		TokenCount: tokenCount,
		Uptime:     time.Since(s.started).Round(time.Second).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"learner not available"}`, http.StatusServiceUnavailable)
		return
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

	result := socket.StatsResult{
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
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleDomains(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"learner not available"}`, http.StatusServiceUnavailable)
		return
	}

	state := s.queries.LearnerSnapshot()

	var domains []socket.DomainInfo
	var coreCount int
	for name, dm := range state.DomainMeta {
		domains = append(domains, socket.DomainInfo{
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

	result := socket.DomainsResult{
		Domains:   domains,
		Count:     len(domains),
		CoreCount: coreCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleBigrams(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"learner not available"}`, http.StatusServiceUnavailable)
		return
	}

	state := s.queries.LearnerSnapshot()

	result := socket.BigramsResult{
		Bigrams:         state.Bigrams,
		Count:           len(state.Bigrams),
		CohitKwTerm:     state.CohitKwTerm,
		CohitTermDomain: state.CohitTermDomain,
		CohitKwCount:    len(state.CohitKwTerm),
		CohitTdCount:    len(state.CohitTermDomain),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleConvMetrics(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.SessionMetricsSnapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleConvTools(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.ToolMetricsSnapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleConvFeed(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.ConversationTurns()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleTopKeywords(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.TopKeywords(15)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleTopTerms(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.TopTerms(15)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleTopFiles(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.TopFiles(15)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleActivityFeed(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.ActivityFeed()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleRunway(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.RunwayProjection()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.SessionList()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.TelemetrySnapshot()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.ProjectConfig()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}
	result := s.queries.UsageQuota()
	w.Header().Set("Content-Type", "application/json")
	if result == nil {
		w.Write([]byte("null"))
		return
	}
	json.NewEncoder(w).Encode(result)
}
