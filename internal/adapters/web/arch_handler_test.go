//go:build !lean

package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

// setupArchServerWithRevision creates a test server with a revision source set.
// Used to test that the C4 fence runs before ETag logic (PF4).
func setupArchServerWithRevision(t *testing.T, archQuerier ports.ArchQuerier, rev uint64) *httptest.Server {
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
	srv.SetRevisionSource(func() uint64 { return rev })
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

// TestArchC4_ShardWithRevisionSource_DisabledReturns404 is the PF4 regression:
// with arch disabled (Arch() == nil) AND a revision source set (so withETag would
// normally produce 304 on a matching ETag), the shard route MUST return 404.
// Previously, withETag(handleArchShard) answered 304 before the C4 fence ran,
// leaking whether arch data exists to any client holding a stale ETag.
func TestArchC4_ShardWithRevisionSource_DisabledReturns404(t *testing.T) {
	const testRev uint64 = 42
	// arch disabled (nil querier), revision source active
	ts := setupArchServerWithRevision(t, nil, testRev)
	defer ts.Close()

	// Send a request whose If-None-Match matches the current revision.
	// With the old withETag-wrapped shard route, this would return 304.
	// With PF4 (no withETag on shard), the C4 fence runs first → 404.
	req, err := http.NewRequest("GET", ts.URL+"/api/arch/local/component", nil)
	require.NoError(t, err)
	req.Header.Set("If-None-Match", "42") // matches revisionFn() = 42

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode,
		"disabled arch must return 404 even with matching ETag (C4 fence before ETag)")
	assert.NotEqual(t, http.StatusNotModified, resp.StatusCode,
		"304 here would be the PF4 bug: ETag bypass before C4 fence")
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

// TestArchManifest_LabelDerivedFromProjectRoot verifies PF7: the estate label
// is derived from the project root basename, not hardcoded to "aOa".
// The mockQueries.ProjectConfig() returns ProjectRoot="/test/project", so the
// label must be "project".
func TestArchManifest_LabelDerivedFromProjectRoot(t *testing.T) {
	q := &mockArchQuerier{} // arch enabled, no shards yet (empty manifest path)
	ts := setupArchServer(t, q)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/manifest")
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))

	estates, ok := result["estates"].(map[string]interface{})
	require.True(t, ok, "manifest must have estates")
	local, ok := estates["local"].(map[string]interface{})
	require.True(t, ok, "estates must have local key")

	// Label must be filepath.Base(ProjectRoot) = "project", NOT hardcoded "aOa"
	assert.Equal(t, "project", local["label"],
		"estate label must be derived from ProjectRoot basename (PF7)")

	scopes, ok := local["scopes"].(map[string]interface{})
	require.True(t, ok)
	localScope, ok := scopes["local"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "project", localScope["label"],
		"scope label must match estate label")
}

// --- /api/arch/{path} shard tests ---

// TestArchShard_ReturnsBytesVerbatim ensures /api/arch/{scope}/{id} returns the
// raw shard bytes exactly as stored.
//
// PF2 byte-identity AC: CLI JSON == browser shard.
// De-masked: TrimSpace removed — body must equal stored bytes without whitespace trimming.
// The HTTP handler writes stored bytes verbatim (no trailing newline); the CLI also
// writes stored bytes verbatim (prettyPrintJSON non-pretty path uses os.Stdout.Write).
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
	// PF2 de-mask: no TrimSpace — stored bytes and HTTP body must be identical.
	assert.Equal(t, shardJSON, body,
		"HTTP body must be byte-identical to stored shard (PF2 byte-parity AC)")
}

// TestArchShard_ImmutableCacheWithHash ensures /api/arch/{path}?v={correct-hash} gets
// immutable cache headers when the content hash matches the requested hash.
//
// PF5: hash is verified before serving; immutable header only on hash match.
func TestArchShard_ImmutableCacheWithHash(t *testing.T) {
	shardJSON := []byte(`{"kind":"simple","title":"DSM"}`)
	// Compute the correct content hash (same algorithm as arch.ContentHash).
	hash := contentHash(shardJSON)
	q := &mockArchQuerier{
		views: map[string][]byte{
			"local/dsm": shardJSON,
		},
	}
	ts := setupArchServer(t, q)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/arch/local/dsm?v=" + hash)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	cc := resp.Header.Get("Cache-Control")
	assert.Contains(t, cc, "immutable",
		"correct ?v=hash must get immutable cache header (PF5)")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, shardJSON, body, "body must be the shard bytes")
}

// TestArchShard_StaleHashReturns404 verifies PF5: when ?v=hash doesn't match the
// actual content hash (shard was re-derived), the server returns 404 so the client
// knows to discard the stale URL and refetch the manifest for the new hash.
// This prevents wrong bytes being cached permanently under a content-addressed URL.
func TestArchShard_StaleHashReturns404(t *testing.T) {
	shardJSON := []byte(`{"kind":"simple","title":"DSM"}`)
	q := &mockArchQuerier{
		views: map[string][]byte{
			"local/dsm": shardJSON,
		},
	}
	ts := setupArchServer(t, q)
	defer ts.Close()

	// Request with a stale/wrong hash — simulates the browser using a URL from
	// a manifest that was superseded by a re-derive.
	resp, err := http.Get(ts.URL + "/api/arch/local/dsm?v=000000000000")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode,
		"stale ?v= hash must return 404 so client refetches manifest (PF5)")
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

// --- /arch/vendor/bundle.js gzip serving test ---

// TestArchVendorBundle_ServedWithGzipEncoding ensures /arch/vendor/bundle.js is served
// with Content-Type: application/javascript and Content-Encoding: gzip from the
// pre-compressed bundle.js.gz (stored under 1 MB per-file limit).
// The browser's ES module loader handles Content-Encoding: gzip transparently.
func TestArchVendorBundle_ServedWithGzipEncoding(t *testing.T) {
	q := &mockArchQuerier{}
	ts := setupArchServer(t, q)
	defer ts.Close()

	// Raw transport (no auto-decompression) so headers/body are exactly as served.
	client := &http.Client{
		Transport: &http.Transport{DisableCompression: true},
	}

	// Client advertising gzip → pre-compressed bytes, Content-Encoding: gzip.
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/arch/vendor/bundle.js", nil)
	require.NoError(t, err)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/javascript", resp.Header.Get("Content-Type"))
	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"),
		"gzip-accepting client must get the pre-compressed bundle")
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "Accept-Encoding", resp.Header.Get("Vary"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Greater(t, len(body), 1000, "gzip bundle must be non-trivially sized")

	// Client NOT advertising gzip → decompressed JS, no Content-Encoding (RFC 7231).
	respID, err := client.Get(ts.URL + "/arch/vendor/bundle.js")
	require.NoError(t, err)
	defer respID.Body.Close()

	assert.Equal(t, http.StatusOK, respID.StatusCode)
	assert.Empty(t, respID.Header.Get("Content-Encoding"),
		"non-gzip client must get identity-encoded bytes")
	idBody, err := io.ReadAll(respID.Body)
	require.NoError(t, err)
	assert.Greater(t, len(idBody), len(body),
		"identity body must be the decompressed (larger) bundle")
}
