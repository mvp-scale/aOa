package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQueries implements socket.AppQueries for testing.
type mockQueries struct {
	state *ports.LearnerState
}

func (m *mockQueries) LearnerSnapshot() *ports.LearnerState {
	return m.state
}

func (m *mockQueries) WipeProject() error {
	return nil
}

func newTestState() *ports.LearnerState {
	return &ports.LearnerState{
		PromptCount: 42,
		KeywordHits: map[string]uint32{"auth": 5, "login": 3},
		TermHits:    map[string]uint32{"authentication": 5},
		DomainMeta: map[string]*ports.DomainMeta{
			"security": {Hits: 10.5, Tier: "core", State: "active", Source: "search"},
			"web":      {Hits: 3.2, Tier: "context", State: "active", Source: "search"},
		},
		CohitKwTerm:      make(map[string]uint32),
		CohitTermDomain:  make(map[string]uint32),
		Bigrams:          map[string]uint32{"auth login": 7, "user session": 3},
		FileHits:         map[string]uint32{"auth.go": 4},
		KeywordBlocklist: make(map[string]bool),
		GapKeywords:      make(map[string]bool),
	}
}

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	idx := &ports.Index{
		Tokens:   map[string][]ports.TokenRef{"auth": {{FileID: 1, Line: 1}}},
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    map[uint32]*ports.FileMeta{1: {Path: "auth.go", Language: "go"}},
	}
	domains := make(map[string]index.Domain)
	engine := index.NewSearchEngine(idx, domains)
	queries := &mockQueries{state: newTestState()}

	srv := NewServer(queries, idx, engine, "")
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(staticFS))
	mux.HandleFunc("GET /api/health", srv.handleHealth)
	mux.HandleFunc("GET /api/stats", srv.handleStats)
	mux.HandleFunc("GET /api/domains", srv.handleDomains)
	mux.HandleFunc("GET /api/bigrams", srv.handleBigrams)

	return httptest.NewServer(mux)
}

func TestHealthEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result socket.HealthResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result.Status)
	assert.Equal(t, 1, result.FileCount)
	assert.Equal(t, 1, result.TokenCount)
}

func TestStatsEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/stats")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result socket.StatsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, uint32(42), result.PromptCount)
	assert.Equal(t, 2, result.DomainCount)
	assert.Equal(t, 1, result.CoreCount)
	assert.Equal(t, 1, result.ContextCount)
	assert.Equal(t, 2, result.KeywordCount)
	assert.Equal(t, 1, result.TermCount)
	assert.Equal(t, 2, result.BigramCount)
	assert.Equal(t, 1, result.FileHitCount)
}

func TestDomainsEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/domains")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result socket.DomainsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, 2, result.Count)
	assert.Equal(t, 1, result.CoreCount)

	// Should be sorted by hits descending
	require.Len(t, result.Domains, 2)
	assert.Equal(t, "security", result.Domains[0].Name)
	assert.Equal(t, 10.5, result.Domains[0].Hits)
	assert.Equal(t, "core", result.Domains[0].Tier)
	assert.Equal(t, "web", result.Domains[1].Name)
}

func TestBigramsEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/bigrams")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result socket.BigramsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, 2, result.Count)
	assert.Equal(t, uint32(7), result.Bigrams["auth login"])
	assert.Equal(t, uint32(3), result.Bigrams["user session"])
}

func TestDashboardHTML(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/static/index.html")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	ct := resp.Header.Get("Content-Type")
	assert.True(t, strings.HasPrefix(ct, "text/html"), "content-type should be text/html, got %s", ct)
}

func TestDefaultPort(t *testing.T) {
	port := DefaultPort("/home/user/project")
	assert.GreaterOrEqual(t, port, 19000)
	assert.Less(t, port, 20000)

	// Same path should give same port
	port2 := DefaultPort("/home/user/project")
	assert.Equal(t, port, port2)

	// Different path should (likely) give different port
	port3 := DefaultPort("/home/user/other")
	// Not guaranteed to be different but we can check range
	assert.GreaterOrEqual(t, port3, 19000)
	assert.Less(t, port3, 20000)
}
