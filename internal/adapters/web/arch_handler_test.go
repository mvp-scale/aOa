//go:build !lean

package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock ArchQuerier ---

type mockArchQuerier struct {
	manifest *ports.ArchManifest
	views    map[string][]byte
}

func (m *mockArchQuerier) Manifest(scope string) (*ports.ArchManifest, error) {
	return m.manifest, nil
}

func (m *mockArchQuerier) View(scope, id string) ([]byte, error) {
	if m.views == nil {
		return nil, nil
	}
	b, ok := m.views[scope+"/"+id]
	if !ok {
		return nil, nil
	}
	return b, nil
}

func (m *mockArchQuerier) Findings(scope string) ([]byte, error)                    { return nil, nil }
func (m *mockArchQuerier) Derive(scope, from, to string, k int) ([]string, error)   { return nil, nil }
func (m *mockArchQuerier) Facts(scope, subject string, limit int) ([]byte, error)   { return nil, nil }

// --- mock AppQueries with arch ---

type archEnabledQueries struct {
	mockQueries
	arch ports.ArchQuerier
}

func (a *archEnabledQueries) Arch() ports.ArchQuerier { return a.arch }

// --- test helpers ---

// setupArchServer creates a test HTTP server with arch routes registered.
// If archQuerier is nil, queries.Arch() returns nil (C4 kill-switch).
func setupArchServer(t *testing.T, archQuerier ports.ArchQuerier) *httptest.Server {
	t.Helper()
	queries := &archEnabledQueries{
		mockQueries: mockQueries{state: newTestState()},
		arch:        archQuerier,
	}
	srv := NewServer(queries, &ports.Index{
		Tokens:   map[string][]ports.TokenRef{},
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    map[uint32]*ports.FileMeta{},
	}, nil, "")
	mux := http.NewServeMux()
	srv.registerArchRoutes(mux)
	return httptest.NewServer(mux)
}

// --- C4 kill-switch tests ---

// TestArchC4_ManifestReturns404WhenDisabled ensures /api/arch/manifest returns 404
// when Arch() is nil (C4 kill-switch).
func TestArchC4_ManifestReturns404WhenDisabled(t *testing.T) {
	ts := setupArchServer(t, nil) // arch disabled
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/manifest")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestArchC4_StandardsReturns404WhenDisabled ensures /api/arch/standards returns 404
// when Arch() is nil (C4 kill-switch).
func TestArchC4_StandardsReturns404WhenDisabled(t *testing.T) {
	ts := setupArchServer(t, nil)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/standards")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestArchC4_ShardReturns404WhenDisabled ensures /api/arch/{path} returns 404
// when Arch() is nil (C4 kill-switch).
func TestArchC4_ShardReturns404WhenDisabled(t *testing.T) {
	ts := setupArchServer(t, nil)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/local/component")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestArchC4_ViewerStaticReturns404WhenDisabled ensures /arch/ returns 404
// when Arch() is nil (C4 kill-switch, F-6 fencing).
func TestArchC4_ViewerStaticReturns404WhenDisabled(t *testing.T) {
	ts := setupArchServer(t, nil)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/arch/")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// --- /api/arch/standards tests ---

// TestArchStandards_ServesEmbeddedJSON ensures /api/arch/standards returns valid JSON
// containing the views section from view-standards.json.
func TestArchStandards_ServesEmbeddedJSON(t *testing.T) {
	q := &mockArchQuerier{}
	ts := setupArchServer(t, q)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/standards")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Must be valid JSON
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result), "response must be valid JSON")

	// Must have a "views" section (the standards file's top-level key)
	_, hasViews := result["views"]
	assert.True(t, hasViews, "standards JSON must have a 'views' key")

	// Must have named_palettes under global.palette
	global, ok := result["global"].(map[string]interface{})
	require.True(t, ok, "standards JSON must have a 'global' section")
	palette, ok := global["palette"].(map[string]interface{})
	require.True(t, ok, "global must have a 'palette' section")
	_, hasNamedPalettes := palette["named_palettes"]
	assert.True(t, hasNamedPalettes, "palette must have 'named_palettes'")
}

// --- /api/arch/manifest tests ---

// TestArchManifest_EmptyWhenNoShards ensures /api/arch/manifest returns a valid
// estates-shaped manifest with an empty views map when no shards have been derived.
func TestArchManifest_EmptyWhenNoShards(t *testing.T) {
	q := &mockArchQuerier{manifest: nil} // no shards derived yet
	ts := setupArchServer(t, q)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/manifest")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	assert.Equal(t, "aoa.archmodel/v1", result["schema"])
	assert.Equal(t, true, result["sharded"])
	assert.NotNil(t, result["generated"])
	assert.NotNil(t, result["estates"])
}

// TestArchManifest_SynthesizesEstatesShape ensures /api/arch/manifest transforms the
// flat Go manifest into the estates-shaped structure the viewer expects.
func TestArchManifest_SynthesizesEstatesShape(t *testing.T) {
	shardJSON := []byte(`{"kind":"buckets","title":"Component","count":"12 units","prov":{"kind":"derived","label":"derived"}}`)
	q := &mockArchQuerier{
		manifest: &ports.ArchManifest{
			Scope: "local",
			Rev:   "abc123def456",
			Views: []ports.ArchViewEntry{
				{ID: "component", Key: "local/component@abc123def456", Hash: "abc123def456", Caption: "12 units", Prov: "derived"},
			},
		},
		views: map[string][]byte{
			"local/component": shardJSON,
		},
	}
	ts := setupArchServer(t, q)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/manifest")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	// Verify estates structure
	estates, ok := result["estates"].(map[string]interface{})
	require.True(t, ok)
	local, ok := estates["local"].(map[string]interface{})
	require.True(t, ok)
	scopes, ok := local["scopes"].(map[string]interface{})
	require.True(t, ok)
	localScope, ok := scopes["local"].(map[string]interface{})
	require.True(t, ok)
	views, ok := localScope["views"].(map[string]interface{})
	require.True(t, ok)

	// Verify the component view is present
	compView, ok := views["component"].(map[string]interface{})
	require.True(t, ok, "component view must be present in manifest")
	assert.Equal(t, "buckets", compView["kind"])
	assert.Equal(t, "Component", compView["title"])
	assert.Equal(t, "12 units", compView["count"])

	// Shard path must be "local/component" (for the viewer's BASE+path fetch)
	shard, ok := compView["shard"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "local/component", shard["path"])
	assert.Equal(t, "abc123def456", shard["hash"])
}

// --- /api/arch/{path} shard tests ---

// TestArchShard_ReturnsBytesVerbatim ensures /api/arch/{scope}/{id} returns the
// raw shard bytes exactly as stored (byte-identity AC: CLI JSON == browser shard).
func TestArchShard_ReturnsBytesVerbatim(t *testing.T) {
	shardJSON := []byte(`{"kind":"simple","title":"Component","count":"5 nodes"}`)
	q := &mockArchQuerier{
		views: map[string][]byte{
			"local/component": shardJSON,
		},
	}
	ts := setupArchServer(t, q)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/local/component")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, string(shardJSON), strings.TrimSpace(string(body)))
}

// TestArchShard_ImmutableCacheWithHash ensures /api/arch/{path}?v={hash} gets
// immutable cache headers (content-addressed URL → safe to cache forever).
func TestArchShard_ImmutableCacheWithHash(t *testing.T) {
	shardJSON := []byte(`{"kind":"simple","title":"DSM"}`)
	q := &mockArchQuerier{
		views: map[string][]byte{
			"local/dsm": shardJSON,
		},
	}
	ts := setupArchServer(t, q)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/local/dsm?v=abc123")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	cc := resp.Header.Get("Cache-Control")
	assert.Contains(t, cc, "immutable", "?v=hash must get immutable cache header")
}

// TestArchShard_Returns404ForMissingView ensures /api/arch/{path} returns 404
// when the view does not exist in the store.
func TestArchShard_Returns404ForMissingView(t *testing.T) {
	q := &mockArchQuerier{views: map[string][]byte{}} // no views
	ts := setupArchServer(t, q)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/local/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestArchShard_Returns404ForMissingPathSegment ensures /api/arch/ (no scope/id)
// returns 404 rather than panicking.
func TestArchShard_Returns404ForMissingPathSegment(t *testing.T) {
	q := &mockArchQuerier{}
	ts := setupArchServer(t, q)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/noslash")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
