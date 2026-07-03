package socket

// C4 socket probe — PC6 (checkpoint-F2.md).
//
// Ruling R2: flag-off socket answer = explicit "arch not available (C4)" error,
// never silent unknown-method, never a hang. This file adds the standing probe
// that exercises the guard at server.go:587-598 directly — no process spawning,
// no env-var or config-file propagation required.
//
// Approach: create a Server with an AppQueries mock where Arch() returns nil
// (exactly the condition that fires when ArchEnabled=false, i.e. AOA_ARCH=off or
// .aoa/config sets AOA_ARCH=off). The mock satisfies the full AppQueries interface
// while only meaningfully implementing Arch() — all other methods return zero
// values because the arch handlers short-circuit before calling any other method.

import (
	"strings"
	"testing"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nilArchQueries implements AppQueries with Arch() returning nil (arch-off state).
// All other methods return zero values; they are not called for arch requests
// because the archQuerier / handleArchJourney guards fire first.
type nilArchQueries struct{}

func (nilArchQueries) Arch() ports.ArchQuerier                    { return nil }
func (nilArchQueries) LearnerSnapshot() *ports.LearnerState       { return nil }
func (nilArchQueries) WipeProject() error                         { return nil }
func (nilArchQueries) Reindex() (ReindexResult, error)            { return ReindexResult{}, nil }
func (nilArchQueries) SessionMetricsSnapshot() SessionMetricsResult { return SessionMetricsResult{} }
func (nilArchQueries) ToolMetricsSnapshot() ToolMetricsResult       { return ToolMetricsResult{} }
func (nilArchQueries) ConversationTurns() ConversationFeedResult    { return ConversationFeedResult{} }
func (nilArchQueries) ActivityFeed() ActivityFeedResult             { return ActivityFeedResult{} }
func (nilArchQueries) TopKeywords(limit int) TopItemsResult         { return TopItemsResult{} }
func (nilArchQueries) TopTerms(limit int) TopItemsResult            { return TopItemsResult{} }
func (nilArchQueries) TopFiles(limit int) TopItemsResult            { return TopItemsResult{} }
func (nilArchQueries) DomainTermNames(domain string) []string       { return nil }
func (nilArchQueries) DomainTermHitCounts(domain string) map[string]int { return nil }
func (nilArchQueries) RunwayProjection() RunwayResult               { return RunwayResult{} }
func (nilArchQueries) SessionList() SessionListResult               { return SessionListResult{} }
func (nilArchQueries) ProjectConfig() ProjectConfigResult           { return ProjectConfigResult{} }
func (nilArchQueries) ReconAvailable() bool                         { return false }
func (nilArchQueries) DimensionalResults() map[string]*DimensionalFileResult { return nil }
func (nilArchQueries) InvestigatedFiles() []string                  { return nil }
func (nilArchQueries) SetFileInvestigated(relPath string, investigated bool) {}
func (nilArchQueries) ClearInvestigated()                           {}
func (nilArchQueries) UsageQuota() *UsageQuotaResult                { return nil }
func (nilArchQueries) DimScanProgress() DimScanProgress             { return DimScanProgress{} }
func (nilArchQueries) GenerateHints(query string, opts ports.SearchOptions) []string { return nil }
func (nilArchQueries) TelemetrySnapshot() TelemetryResult           { return TelemetryResult{} }

// assertC4Resp verifies that err is non-nil, contains "C4", and does NOT
// contain "unknown method" (which would indicate the guard was bypassed).
func assertC4Resp(t *testing.T, method string, err error) {
	t.Helper()
	require.Error(t, err, "C4 probe %s: expected error, got nil (guard not reached)", method)
	msg := err.Error()
	assert.True(t, strings.Contains(msg, "C4"),
		"C4 probe %s: error must contain 'C4':\n  got: %q", method, msg)
	assert.False(t, strings.Contains(msg, "unknown method") || strings.Contains(msg, "unknown-method"),
		"C4 probe %s: must not be 'unknown method' (guard bypassed):\n  got: %q", method, msg)
	t.Logf("C4 probe %s: OK — %q", method, msg)
}

// TestC4_SocketProbe_AllArchMethodsReturnC4Error creates a server with a
// nil-arch AppQueries (simulating AOA_ARCH=off / ArchEnabled=false) and verifies
// every one of the six MethodArch* socket methods returns the C4 error —
// not "unknown method" (server.go:264 default arm), not a hang. Asserts R2.
func TestC4_SocketProbe_AllArchMethodsReturnC4Error(t *testing.T) {
	engine, idx := testFixtures()
	sockPath := testSocketPath(t)

	// Wire a server with Arch()=nil queries (the C4 condition).
	srv := NewServer(index.NewSearchAdapter(engine), idx, sockPath, nilArchQueries{})
	require.NoError(t, srv.Start())
	defer srv.Stop()

	c := NewClient(sockPath)

	// Sanity: health still works (arch-off must not affect non-arch methods).
	health, err := c.Health()
	require.NoError(t, err)
	assert.Equal(t, "ok", health.Status)

	// ── Six MethodArch* methods — each must return the C4 error ──────────────

	// 1. arch.views
	_, err = c.ArchViews("local")
	assertC4Resp(t, "arch.views", err)

	// 2. arch.view
	_, err = c.ArchView("local", "component")
	assertC4Resp(t, "arch.view", err)

	// 3. arch.findings
	_, err = c.ArchFindings("local")
	assertC4Resp(t, "arch.findings", err)

	// 4. arch.journey  — handleArchJourney uses its own nil check (server.go:702)
	//    rather than archQuerier, but the outcome is the same C4 error when Arch()=nil.
	err = c.ArchJourney("", false)
	assertC4Resp(t, "arch.journey", err)

	// 5. arch.derive — C4 check in archQuerier precedes param validation
	_, err = c.ArchDerive("local", "u_main", "u_internal_service", 10)
	assertC4Resp(t, "arch.derive", err)

	// 6. arch.facts
	_, err = c.ArchFacts("local", "main", 0)
	assertC4Resp(t, "arch.facts", err)
}

// TestC4_SocketProbe_NonArchMethodsUnaffected verifies that on the same
// arch-disabled server (Arch()=nil), the non-arch socket methods (health, search)
// still work normally — the server.go:264 default arm is untouched.
func TestC4_SocketProbe_NonArchMethodsUnaffected(t *testing.T) {
	engine, idx := testFixtures()
	sockPath := testSocketPath(t)

	srv := NewServer(index.NewSearchAdapter(engine), idx, sockPath, nilArchQueries{})
	require.NoError(t, srv.Start())
	defer srv.Stop()

	c := NewClient(sockPath)

	// Health must succeed.
	health, err := c.Health()
	require.NoError(t, err)
	assert.Equal(t, "ok", health.Status, "health unaffected by arch-off")
	t.Logf("health on nil-arch server: status=%q files=%d", health.Status, health.FileCount)

	// Search must succeed (uses searcher, not arch querier).
	// Note: Count is only populated in CountOnly mode; use len(Hits) for normal mode.
	result, err := c.Search("login", ports.SearchOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, result.ExitCode, "search exit code on nil-arch server")
	assert.Greater(t, len(result.Hits), 0, "search must return hits for known term 'login'")
	t.Logf("search on nil-arch server: hits=%d", len(result.Hits))
}
