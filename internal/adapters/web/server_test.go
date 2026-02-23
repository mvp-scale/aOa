package web

import (
	"encoding/json"
	"io/fs"
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

func (m *mockQueries) Reindex() (socket.ReindexResult, error) {
	return socket.ReindexResult{}, nil
}

func (m *mockQueries) SessionMetricsSnapshot() socket.SessionMetricsResult {
	return socket.SessionMetricsResult{}
}

func (m *mockQueries) ToolMetricsSnapshot() socket.ToolMetricsResult {
	return socket.ToolMetricsResult{}
}

func (m *mockQueries) ConversationTurns() socket.ConversationFeedResult {
	return socket.ConversationFeedResult{}
}

func (m *mockQueries) ActivityFeed() socket.ActivityFeedResult {
	return socket.ActivityFeedResult{Entries: []socket.ActivityEntryResult{}}
}

func (m *mockQueries) TopKeywords(limit int) socket.TopItemsResult {
	return socket.TopItemsResult{}
}

func (m *mockQueries) TopTerms(limit int) socket.TopItemsResult {
	return socket.TopItemsResult{}
}

func (m *mockQueries) TopFiles(limit int) socket.TopItemsResult {
	return socket.TopItemsResult{}
}

func (m *mockQueries) DomainTermNames(domain string) []string {
	return nil
}

func (m *mockQueries) DomainTermHitCounts(domain string) map[string]int {
	return nil
}

func (m *mockQueries) RunwayProjection() socket.RunwayResult {
	return socket.RunwayResult{
		Model:            "claude-opus-4-6",
		ContextWindowMax: 200000,
		BurnRatePerMin:   1500.0,
	}
}

func (m *mockQueries) SessionList() socket.SessionListResult {
	return socket.SessionListResult{
		Sessions: []socket.SessionSummaryResult{
			{
				SessionID:   "abc123",
				StartTime:   1700000000,
				EndTime:     1700003600,
				PromptCount: 15,
				ReadCount:   10,
				GuidedReadCount: 7,
				GuidedRatio: 0.7,
				TokensSaved: 5000,
				Model:       "claude-opus-4-6",
			},
		},
		Count: 1,
	}
}

func (m *mockQueries) ReconAvailable() bool {
	return false
}

func (m *mockQueries) DimensionalResults() map[string]*socket.DimensionalFileResult {
	return nil
}

func (m *mockQueries) CachedReconResult() (interface{}, int64) {
	return nil, 0
}

func (m *mockQueries) ProjectConfig() socket.ProjectConfigResult {
	return socket.ProjectConfigResult{
		ProjectRoot:   "/test/project",
		ProjectID:     "project",
		DBPath:        "/test/project/.aoa/aoa.db",
		SocketPath:    "/tmp/aoa-test.sock",
		IndexFiles:    42,
		IndexTokens:   100,
		UptimeSeconds: 3600,
	}
}

func (m *mockQueries) UsageQuota() *socket.UsageQuotaResult {
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
	engine := index.NewSearchEngine(idx, domains, "")
	queries := &mockQueries{state: newTestState()}

	srv := NewServer(queries, idx, engine, "")
	mux := http.NewServeMux()
	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("GET /", http.FileServerFS(staticSub))
	mux.HandleFunc("GET /api/health", srv.handleHealth)
	mux.HandleFunc("GET /api/stats", srv.handleStats)
	mux.HandleFunc("GET /api/domains", srv.handleDomains)
	mux.HandleFunc("GET /api/bigrams", srv.handleBigrams)
	mux.HandleFunc("GET /api/conversation/metrics", srv.handleConvMetrics)
	mux.HandleFunc("GET /api/conversation/tools", srv.handleConvTools)
	mux.HandleFunc("GET /api/conversation/feed", srv.handleConvFeed)
	mux.HandleFunc("GET /api/top-keywords", srv.handleTopKeywords)
	mux.HandleFunc("GET /api/top-terms", srv.handleTopTerms)
	mux.HandleFunc("GET /api/top-files", srv.handleTopFiles)
	mux.HandleFunc("GET /api/activity/feed", srv.handleActivityFeed)
	mux.HandleFunc("GET /api/runway", srv.handleRunway)
	mux.HandleFunc("GET /api/sessions", srv.handleSessions)
	mux.HandleFunc("GET /api/config", srv.handleConfig)

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

	// Test root path serves index.html
	resp, err := http.Get(ts.URL + "/")
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

func TestConvMetricsEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/conversation/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result socket.SessionMetricsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	// Zero values expected from mock
}

func TestConvToolsEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/conversation/tools")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result socket.ToolMetricsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
}

func TestConvFeedEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/conversation/feed")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result socket.ConversationFeedResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
}

func TestTopKeywordsEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/top-keywords")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result socket.TopItemsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
}

func TestTopTermsEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/top-terms")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result socket.TopItemsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
}

func TestTopFilesEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/top-files")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	var result socket.TopItemsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
}

func TestRunwayEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/runway")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result socket.RunwayResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "claude-opus-4-6", result.Model)
	assert.Equal(t, 200000, result.ContextWindowMax)
	assert.InDelta(t, 1500.0, result.BurnRatePerMin, 0.01)
}

func TestHandleActivityFeed(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/activity/feed")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result socket.ActivityFeedResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotNil(t, result.Entries)
	assert.Equal(t, 0, result.Count)
}

func TestSessionsEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/sessions")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result socket.SessionListResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, 1, result.Count)
	require.Len(t, result.Sessions, 1)
	assert.Equal(t, "abc123", result.Sessions[0].SessionID)
	assert.Equal(t, 15, result.Sessions[0].PromptCount)
	assert.Equal(t, int64(5000), result.Sessions[0].TokensSaved)
	assert.InDelta(t, 0.7, result.Sessions[0].GuidedRatio, 0.01)
}

func TestConfigEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/config")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result socket.ProjectConfigResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "/test/project", result.ProjectRoot)
	assert.Equal(t, "project", result.ProjectID)
	assert.Equal(t, 42, result.IndexFiles)
	assert.Equal(t, int64(3600), result.UptimeSeconds)
}
