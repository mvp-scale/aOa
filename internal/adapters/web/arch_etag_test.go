//go:build !lean

package web

// L19.20: Arch manifest ETag correctness tests — extend ETag coverage for /api/arch/manifest.
//
// Decision: manifest ETag = m.Rev (factsHash), NOT the global revision counter.
//   • Tighter invalidation: 304 only when arch facts unchanged, 200 on any re-derive.
//   • Zero-symbol file change: if facts are unchanged, m.Rev is unchanged → 304 is CORRECT
//     and honest (the view truly didn't change). Board AC "no stale 304" means no 304 when
//     the view changed; 304 on an unchanged view is not stale, it's accurate.
//   • Timestamp safety: ETag is m.Rev, not the serialised body, so the serve-time
//     generated.timestamp injection never causes a spurious 200.
//
// See also: internal/adapters/web/etag_test.go (global revision middleware tests).

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// swappableQuerier is a mockArchQuerier whose manifest can be replaced between requests,
// simulating a re-derive that changes m.Rev.
type swappableQuerier struct {
	manifest *ports.ArchManifest
	views    map[string][]byte
}

func (m *swappableQuerier) Manifest(scope string) (*ports.ArchManifest, error) {
	return m.manifest, nil
}

func (m *swappableQuerier) View(scope, id string) ([]byte, error) {
	if m.views == nil {
		return nil, nil
	}
	b, ok := m.views[scope+"/"+id]
	if !ok {
		return nil, nil
	}
	return b, nil
}

func (m *swappableQuerier) Findings(scope string) ([]byte, error)                    { return nil, nil }
func (m *swappableQuerier) Derive(scope, from, to string, k int) ([]string, error)   { return nil, nil }
func (m *swappableQuerier) Facts(scope, subject string, limit int) ([]byte, error)   { return nil, nil }
func (m *swappableQuerier) Graph(scope, grain string) ([]byte, error)                { return nil, nil }

// =============================================================================
// L19.20 (item d): 304 round-trip on unchanged Rev
// =============================================================================

// TestArchManifestETag_SameRev_Returns304 verifies the 304 round-trip:
// first GET returns 200 + ETag = m.Rev; second GET with If-None-Match = ETag → 304.
func TestArchManifestETag_SameRev_Returns304(t *testing.T) {
	q := &swappableQuerier{
		manifest: &ports.ArchManifest{
			Scope: "local",
			Rev:   "abc123def456",
			Views: []ports.ArchViewEntry{},
		},
	}
	ts := setupArchServer(t, q)
	defer ts.Close()

	// First GET — no If-None-Match → 200 with ETag = m.Rev
	resp, err := http.Get(ts.URL + "/api/arch/manifest")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	etag := resp.Header.Get("ETag")
	assert.Equal(t, "abc123def456", etag, "ETag must equal m.Rev")
	_, _ = io.ReadAll(resp.Body)

	// Second GET — send ETag back, Rev unchanged → 304
	req, _ := http.NewRequest("GET", ts.URL+"/api/arch/manifest", nil)
	req.Header.Set("If-None-Match", etag)
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusNotModified, resp2.StatusCode)
	body2, _ := io.ReadAll(resp2.Body)
	assert.Empty(t, body2, "304 must have empty body")
}

// =============================================================================
// L19.20 (item d): Rev change → new ETag + 200
// =============================================================================

// TestArchManifestETag_RevChange_Returns200 verifies that after m.Rev changes
// (simulating a re-derive), a poll with the old ETag gets 200 with the new ETag.
func TestArchManifestETag_RevChange_Returns200(t *testing.T) {
	q := &swappableQuerier{
		manifest: &ports.ArchManifest{
			Scope: "local",
			Rev:   "oldrev000001",
			Views: []ports.ArchViewEntry{},
		},
	}
	ts := setupArchServer(t, q)
	defer ts.Close()

	// First GET → 200, ETag = old Rev
	resp, err := http.Get(ts.URL + "/api/arch/manifest")
	require.NoError(t, err)
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	oldEtag := resp.Header.Get("ETag")
	assert.Equal(t, "oldrev000001", oldEtag)

	// Simulate re-derive: swap manifest with new Rev
	q.manifest = &ports.ArchManifest{
		Scope: "local",
		Rev:   "newrev999999",
		Views: []ports.ArchViewEntry{},
	}

	// Poll with old ETag → MUST 200 (not 304), new ETag in response
	req, _ := http.NewRequest("GET", ts.URL+"/api/arch/manifest", nil)
	req.Header.Set("If-None-Match", oldEtag)
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode, "changed Rev must yield 200, not 304")
	assert.Equal(t, "newrev999999", resp2.Header.Get("ETag"), "response must carry new ETag")
}

// =============================================================================
// L19.20 (item d): no-stale-304 regression
// Simulates: file edit → edge change → re-derive with new Rev → poll sees 200.
// =============================================================================

// TestArchManifestETag_NoStale304_RedriveNewRev is the no-stale-304 regression test.
// Sequence: boot poll (200, ETag1) → re-derive fires (new manifest, Rev changes) →
// next poll with If-None-Match=ETag1 → MUST 200 with ETag2.
func TestArchManifestETag_NoStale304_RedriveNewRev(t *testing.T) {
	shardBefore := []byte(`{"kind":"simple","title":"Component","count":"3 nodes"}`)
	shardAfter := []byte(`{"kind":"simple","title":"Component","count":"5 nodes","nodes":[]}`)

	q := &swappableQuerier{
		manifest: &ports.ArchManifest{
			Scope: "local",
			Rev:   "rev_before_1",
			Views: []ports.ArchViewEntry{
				{ID: "component", Hash: "sha_before_1"},
			},
		},
		views: map[string][]byte{"local/component": shardBefore},
	}
	ts := setupArchServer(t, q)
	defer ts.Close()

	// Boot poll: no ETag → 200 + ETag = rev_before_1
	resp, err := http.Get(ts.URL + "/api/arch/manifest")
	require.NoError(t, err)
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	etagBefore := resp.Header.Get("ETag")
	require.Equal(t, "rev_before_1", etagBefore)

	// Re-derive: file edit changed edges → new Rev + new shard hash
	q.manifest = &ports.ArchManifest{
		Scope: "local",
		Rev:   "rev_after_2",
		Views: []ports.ArchViewEntry{
			{ID: "component", Hash: "sha_after_2"},
		},
	}
	q.views = map[string][]byte{"local/component": shardAfter}

	// Next poll with stale ETag → MUST 200 (no stale 304)
	req, _ := http.NewRequest("GET", ts.URL+"/api/arch/manifest", nil)
	req.Header.Set("If-None-Match", etagBefore)
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode,
		"re-derive changes m.Rev → stale ETag must not get 304")
	assert.Equal(t, "rev_after_2", resp2.Header.Get("ETag"))

	// Verify the body reflects the new manifest (new shard hash in the manifest)
	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp2.Body).Decode(&result))
	estates := result["estates"].(map[string]interface{})
	local := estates["local"].(map[string]interface{})
	scopes := local["scopes"].(map[string]interface{})
	localScope := scopes["local"].(map[string]interface{})
	views := localScope["views"].(map[string]interface{})
	comp := views["component"].(map[string]interface{})
	shard := comp["shard"].(map[string]interface{})
	assert.Equal(t, "sha_after_2", shard["hash"], "manifest body must reflect new shard hash")
}

// =============================================================================
// L19.20 (item c): ETag computed from Rev, not body-with-timestamp
// Same Rev → same ETag + 304, even though serve-time timestamp differs.
// =============================================================================

// TestArchManifestETag_TimestampDoesNotAffectETag verifies that the serve-time
// generated.timestamp injection does NOT affect the ETag: two sequential GETs
// with the same m.Rev must yield the same ETag, and the second (with If-None-Match)
// must 304 even though the response body would have a different timestamp.
func TestArchManifestETag_TimestampDoesNotAffectETag(t *testing.T) {
	q := &swappableQuerier{
		manifest: &ports.ArchManifest{
			Scope: "local",
			Rev:   "stable_rev_xyz",
			Views: []ports.ArchViewEntry{},
		},
	}
	ts := setupArchServer(t, q)
	defer ts.Close()

	// First GET → 200, record ETag
	resp1, err := http.Get(ts.URL + "/api/arch/manifest")
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp1.StatusCode)
	etag1 := resp1.Header.Get("ETag")
	assert.Equal(t, "stable_rev_xyz", etag1, "ETag must be m.Rev")

	// Parse body1: record timestamp
	var body1 map[string]interface{}
	require.NoError(t, json.NewDecoder(resp1.Body).Decode(&body1))
	resp1.Body.Close()
	gen1 := body1["generated"].(map[string]interface{})
	ts1 := gen1["timestamp"].(string)
	assert.NotEmpty(t, ts1)

	// Second GET with If-None-Match = ETag1 → MUST 304
	// (Even though serve-time would produce a new timestamp, ETag = m.Rev unchanged)
	req, _ := http.NewRequest("GET", ts.URL+"/api/arch/manifest", nil)
	req.Header.Set("If-None-Match", etag1)
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusNotModified, resp2.StatusCode,
		"same m.Rev → 304 regardless of serve-time timestamp")
	body2, _ := io.ReadAll(resp2.Body)
	assert.Empty(t, body2, "304 must have empty body (timestamp irrelevant)")
}
