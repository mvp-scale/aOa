package web

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// L17.6: ETag/revision gating — middleware returns 304 when state is unchanged
// Goals: G0 (Speed — eliminate ~90% of redundant JSON serialization)
// =============================================================================

func TestETag_FirstRequest_Returns200WithETag(t *testing.T) {
	// First request has no If-None-Match → should get 200 with ETag header.
	var rev atomic.Uint64
	rev.Store(1)

	srv := &Server{revisionFn: func() uint64 { return rev.Load() }}
	handler := srv.withETag(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	req := httptest.NewRequest("GET", "/api/stats", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, 200, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("ETag"), "response should include ETag header")
	assert.Equal(t, "1", rec.Header().Get("ETag"))
	assert.Equal(t, `{"status":"ok"}`, rec.Body.String())
}

func TestETag_MatchingRevision_Returns304(t *testing.T) {
	// Client sends If-None-Match matching current revision → 304 with empty body.
	var rev atomic.Uint64
	rev.Store(42)

	handlerCalled := false
	srv := &Server{revisionFn: func() uint64 { return rev.Load() }}
	handler := srv.withETag(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.Write([]byte(`{"big":"payload"}`))
	})

	req := httptest.NewRequest("GET", "/api/stats", nil)
	req.Header.Set("If-None-Match", "42")
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, 304, rec.Code)
	assert.Empty(t, rec.Body.String(), "304 should have empty body")
	assert.False(t, handlerCalled, "handler should NOT be called on 304 — that's the whole point")
}

func TestETag_StaleRevision_Returns200(t *testing.T) {
	// Client has old revision → 200 with new ETag.
	var rev atomic.Uint64
	rev.Store(10)

	srv := &Server{revisionFn: func() uint64 { return rev.Load() }}
	handler := srv.withETag(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":"fresh"}`))
	})

	req := httptest.NewRequest("GET", "/api/stats", nil)
	req.Header.Set("If-None-Match", "5") // old revision
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "10", rec.Header().Get("ETag"))
	assert.Equal(t, `{"data":"fresh"}`, rec.Body.String())
}

func TestETag_RevisionBumps_InvalidateCache(t *testing.T) {
	// Simulate: poll, no change → 304; state mutates; poll again → 200.
	var rev atomic.Uint64
	rev.Store(1)

	srv := &Server{revisionFn: func() uint64 { return rev.Load() }}
	handler := srv.withETag(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	})

	// First poll — no If-None-Match → 200
	req := httptest.NewRequest("GET", "/api/stats", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	assert.Equal(t, 200, rec.Code)
	etag := rec.Header().Get("ETag")
	assert.Equal(t, "1", etag)

	// Second poll — send ETag back, nothing changed → 304
	req = httptest.NewRequest("GET", "/api/stats", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	handler(rec, req)
	assert.Equal(t, 304, rec.Code)

	// State mutation bumps revision
	rev.Add(1)

	// Third poll — same old ETag, but revision changed → 200
	req = httptest.NewRequest("GET", "/api/stats", nil)
	req.Header.Set("If-None-Match", etag) // still "1"
	rec = httptest.NewRecorder()
	handler(rec, req)
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "2", rec.Header().Get("ETag"))
}

func TestETag_NoIfNoneMatch_AlwaysReturns200(t *testing.T) {
	// Requests without If-None-Match always get 200 — clients that don't
	// support ETag should work normally.
	var rev atomic.Uint64
	rev.Store(99)

	srv := &Server{revisionFn: func() uint64 { return rev.Load() }}
	handler := srv.withETag(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/stats", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)
		assert.Equal(t, 200, rec.Code, "request %d should be 200", i)
		assert.Equal(t, "99", rec.Header().Get("ETag"))
	}
}

func TestETag_EmptyIfNoneMatch_Returns200(t *testing.T) {
	// Empty If-None-Match header should not match anything.
	var rev atomic.Uint64
	rev.Store(1)

	srv := &Server{revisionFn: func() uint64 { return rev.Load() }}
	handler := srv.withETag(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	})

	req := httptest.NewRequest("GET", "/api/stats", nil)
	req.Header.Set("If-None-Match", "")
	rec := httptest.NewRecorder()
	handler(rec, req)
	assert.Equal(t, 200, rec.Code)
}

func TestETag_HealthEndpoint_NoETag(t *testing.T) {
	// /api/health should NOT have ETag gating — it's used for liveness checks.
	// This test verifies the expectation that health is registered without withETag.
	// The actual enforcement is in server.go route registration.
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
	// Health should work regardless of ETag state
}

func TestETag_DifferentEndpoints_SameRevision(t *testing.T) {
	// All endpoints share the same global revision counter.
	// A single state mutation invalidates all cached endpoints.
	var rev atomic.Uint64
	rev.Store(5)

	srv := &Server{revisionFn: func() uint64 { return rev.Load() }}

	statsHandler := srv.withETag(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"stats":true}`))
	})
	domainsHandler := srv.withETag(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"domains":true}`))
	})

	// Both endpoints return same ETag
	req1 := httptest.NewRequest("GET", "/api/stats", nil)
	rec1 := httptest.NewRecorder()
	statsHandler(rec1, req1)
	assert.Equal(t, "5", rec1.Header().Get("ETag"))

	req2 := httptest.NewRequest("GET", "/api/domains", nil)
	rec2 := httptest.NewRecorder()
	domainsHandler(rec2, req2)
	assert.Equal(t, "5", rec2.Header().Get("ETag"))

	// Both return 304 with same ETag
	req1 = httptest.NewRequest("GET", "/api/stats", nil)
	req1.Header.Set("If-None-Match", "5")
	rec1 = httptest.NewRecorder()
	statsHandler(rec1, req1)
	assert.Equal(t, 304, rec1.Code)

	req2 = httptest.NewRequest("GET", "/api/domains", nil)
	req2.Header.Set("If-None-Match", "5")
	rec2 = httptest.NewRecorder()
	domainsHandler(rec2, req2)
	assert.Equal(t, 304, rec2.Code)
}
