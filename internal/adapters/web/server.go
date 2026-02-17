package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
)

// Server serves the web dashboard and JSON API over HTTP.
type Server struct {
	queries  socket.AppQueries
	idx      *ports.Index
	engine   *index.SearchEngine
	listener net.Listener
	httpSrv  *http.Server
	port     int
	started  time.Time
	stopOnce sync.Once

	portFilePath string // .aoa/http.port
}

// NewServer creates an HTTP server for the dashboard.
// The portFilePath is where the bound port is written for discovery.
func NewServer(queries socket.AppQueries, idx *ports.Index, engine *index.SearchEngine, portFilePath string) *Server {
	return &Server{
		queries:      queries,
		idx:          idx,
		engine:       engine,
		portFilePath: portFilePath,
	}
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
	addr := fmt.Sprintf("127.0.0.1:%d", preferredPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	s.listener = ln
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.started = time.Now()

	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(staticFS))
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/domains", s.handleDomains)
	mux.HandleFunc("GET /api/bigrams", s.handleBigrams)

	s.httpSrv = &http.Server{Handler: mux}

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
			Name:   name,
			Hits:   dm.Hits,
			Tier:   dm.Tier,
			State:  dm.State,
			Source: dm.Source,
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
