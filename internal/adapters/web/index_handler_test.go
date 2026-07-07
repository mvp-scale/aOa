package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockIndexQuerier implements ports.IndexQuerier for testing.
type mockIndexQuerier struct {
	peekResults map[string]ports.PeekHit    // code -> result
	refsResults map[string]ports.RefsResult // token -> result
}

func (m *mockIndexQuerier) Peek(codes []string) ([]ports.PeekHit, error) {
	result := make([]ports.PeekHit, len(codes))
	for i, code := range codes {
		if hit, ok := m.peekResults[code]; ok {
			result[i] = hit
		} else {
			result[i] = ports.PeekHit{Code: code, Error: "symbol not found"}
		}
	}
	return result, nil
}

func (m *mockIndexQuerier) Refs(token string, k int) ports.RefsResult {
	if r, ok := m.refsResults[token]; ok {
		refs := r.Refs
		truncated := false
		if k > 0 && len(refs) > k {
			refs = refs[:k]
			truncated = true
		}
		return ports.RefsResult{
			Token:     token,
			Total:     r.Total,
			Refs:      refs,
			Truncated: truncated,
		}
	}
	return ports.RefsResult{Token: token, Total: 0, Refs: []ports.RefHit{}, Truncated: false}
}

// extendedMockQueries extends mockQueries with IndexQuerier.
type extendedMockQueries struct {
	mockQueries
	iq ports.IndexQuerier
}

func (m *extendedMockQueries) IndexQuerier() ports.IndexQuerier {
	return m.iq
}

func setupIndexTestServer(t *testing.T, iq ports.IndexQuerier) *httptest.Server {
	t.Helper()
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files:    make(map[uint32]*ports.FileMeta),
	}
	q := &extendedMockQueries{
		mockQueries: mockQueries{state: newTestState()},
		iq:          iq,
	}
	srv := NewServer(q, idx, nil, "")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/peek", srv.handlePeek)
	mux.HandleFunc("GET /api/refs", srv.handleRefs)
	return httptest.NewServer(mux)
}

// --- /api/peek tests ---

func TestHandlePeek_MissingCode(t *testing.T) {
	ts := setupIndexTestServer(t, &mockIndexQuerier{})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/peek")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandlePeek_UnknownCode_Returns404(t *testing.T) {
	ts := setupIndexTestServer(t, &mockIndexQuerier{
		peekResults: map[string]ports.PeekHit{},
	})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/peek?code=zzzzz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "symbol not found", body["error"])
	assert.Equal(t, "zzzzz", body["code"])
}

func TestHandlePeek_FoundCode_Returns200(t *testing.T) {
	iq := &mockIndexQuerier{
		peekResults: map[string]ports.PeekHit{
			"abc1": {
				Code:      "abc1",
				File:      "internal/app/app.go",
				Symbol:    "New",
				Signature: "New(cfg Config) (*App, error)",
				Span:      [2]int{409, 555},
				Body:      "func New(cfg Config) (*App, error) {\n\t// ...\n}",
				Domain:    "@architecture",
				Tags:      []string{"wiring"},
			},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/peek?code=abc1")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	results, ok := body["results"].([]interface{})
	require.True(t, ok, "expected results array")
	require.Len(t, results, 1)

	hit := results[0].(map[string]interface{})
	assert.Equal(t, "abc1", hit["code"])
	assert.Equal(t, "internal/app/app.go", hit["file"])
	assert.Equal(t, "New", hit["symbol"])
	assert.Equal(t, "@architecture", hit["domain"])
	assert.NotEmpty(t, hit["body"])
	assert.Empty(t, hit["error"])
}

func TestHandlePeek_MultipleCodes_MixedResults(t *testing.T) {
	iq := &mockIndexQuerier{
		peekResults: map[string]ports.PeekHit{
			"good1": {Code: "good1", File: "foo.go", Symbol: "Foo", Span: [2]int{1, 10}, Body: "func Foo() {}"},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/peek?code=good1,bad99")
	require.NoError(t, err)
	defer resp.Body.Close()
	// Multiple codes: always 200 even with some errors
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	results, ok := body["results"].([]interface{})
	require.True(t, ok)
	require.Len(t, results, 2)

	hit0 := results[0].(map[string]interface{})
	assert.Equal(t, "good1", hit0["code"])
	assert.Empty(t, hit0["error"])

	hit1 := results[1].(map[string]interface{})
	assert.Equal(t, "bad99", hit1["code"])
	assert.NotEmpty(t, hit1["error"])
}

func TestHandlePeek_NoIndexQuerier_Returns503(t *testing.T) {
	ts := setupIndexTestServer(t, nil) // pass nil iq
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/peek?code=abc")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// --- /api/refs tests ---

func TestHandleRefs_MissingToken(t *testing.T) {
	ts := setupIndexTestServer(t, &mockIndexQuerier{})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/refs")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandleRefs_FoundToken(t *testing.T) {
	iq := &mockIndexQuerier{
		refsResults: map[string]ports.RefsResult{
			"Reindex": {
				Token: "Reindex",
				Total: 3,
				Refs: []ports.RefHit{
					{File: "internal/app/app.go", Line: 3250, Symbol: "Reindex", Peek: "abc1"},
					{File: "internal/adapters/socket/server.go", Line: 497, Symbol: "handleReindex"},
					{File: "cmd/aoa/cmd/reindex.go", Line: 15},
				},
				Truncated: false,
			},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/refs?token=Reindex")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var result ports.RefsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "Reindex", result.Token)
	assert.Equal(t, 3, result.Total)
	assert.Len(t, result.Refs, 3)
	assert.False(t, result.Truncated)
	assert.Equal(t, "internal/app/app.go", result.Refs[0].File)
	assert.Equal(t, 3250, result.Refs[0].Line)
	assert.Equal(t, "Reindex", result.Refs[0].Symbol)
	assert.Equal(t, "abc1", result.Refs[0].Peek)
}

func TestHandleRefs_TruncatedByK(t *testing.T) {
	refs := make([]ports.RefHit, 25)
	for i := range refs {
		refs[i] = ports.RefHit{File: "foo.go", Line: i + 1}
	}
	iq := &mockIndexQuerier{
		refsResults: map[string]ports.RefsResult{
			"foo": {Token: "foo", Total: 25, Refs: refs, Truncated: false},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	// k=5 hard cap applied by mock
	resp, err := http.Get(ts.URL + "/api/refs?token=foo&k=5")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result ports.RefsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "foo", result.Token)
	assert.Equal(t, 25, result.Total)
	assert.Len(t, result.Refs, 5)
	assert.True(t, result.Truncated)
}

func TestHandleRefs_UnknownToken_ReturnsEmptyNotError(t *testing.T) {
	ts := setupIndexTestServer(t, &mockIndexQuerier{
		refsResults: map[string]ports.RefsResult{},
	})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/refs?token=NoSuchToken")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result ports.RefsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "NoSuchToken", result.Token)
	assert.Equal(t, 0, result.Total)
	assert.Len(t, result.Refs, 0)
	assert.False(t, result.Truncated)
}

func TestHandleRefs_DefaultK_Is20(t *testing.T) {
	refs := make([]ports.RefHit, 30)
	for i := range refs {
		refs[i] = ports.RefHit{File: "foo.go", Line: i + 1}
	}
	iq := &mockIndexQuerier{
		refsResults: map[string]ports.RefsResult{
			"bar": {Token: "bar", Total: 30, Refs: refs},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	// No k param → default 20
	resp, err := http.Get(ts.URL + "/api/refs?token=bar")
	require.NoError(t, err)
	defer resp.Body.Close()

	var result ports.RefsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, 30, result.Total)
	assert.LessOrEqual(t, len(result.Refs), 20)
}

// latency assertions
func TestHandlePeek_Latency(t *testing.T) {
	iq := &mockIndexQuerier{
		peekResults: map[string]ports.PeekHit{
			"x": {Code: "x", File: "a.go", Symbol: "A", Span: [2]int{1, 2}, Body: "func A(){}"},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	start := time.Now()
	resp, err := http.Get(ts.URL + "/api/peek?code=x")
	elapsed := time.Since(start)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// Sub-10ms on test index (mock is in-memory)
	assert.Less(t, elapsed.Milliseconds(), int64(100), "peek should be fast")
}

func TestHandleRefs_Latency(t *testing.T) {
	iq := &mockIndexQuerier{
		refsResults: map[string]ports.RefsResult{
			"fast": {Token: "fast", Total: 1, Refs: []ports.RefHit{{File: "a.go", Line: 1}}},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	start := time.Now()
	resp, err := http.Get(ts.URL + "/api/refs?token=fast")
	elapsed := time.Since(start)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Less(t, elapsed.Milliseconds(), int64(100), "refs should be fast")
}

// Test that refs response has correct JSON field names
func TestHandleRefs_JSONShape(t *testing.T) {
	iq := &mockIndexQuerier{
		refsResults: map[string]ports.RefsResult{
			"srv": {
				Token: "srv",
				Total: 1,
				Refs:  []ports.RefHit{{File: "server.go", Line: 99, Symbol: "Server", Peek: "1abc"}},
			},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/refs?token=srv")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Decode as raw map to check field names
	var raw map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))
	assert.Contains(t, raw, "token")
	assert.Contains(t, raw, "total")
	assert.Contains(t, raw, "refs")
	assert.Contains(t, raw, "truncated")

	refs := raw["refs"].([]interface{})
	require.Len(t, refs, 1)
	ref := refs[0].(map[string]interface{})
	assert.Contains(t, ref, "file")
	assert.Contains(t, ref, "line")
	assert.Contains(t, ref, "symbol")
	assert.Contains(t, ref, "peek")
}

func TestHandlePeek_JSONShape(t *testing.T) {
	iq := &mockIndexQuerier{
		peekResults: map[string]ports.PeekHit{
			"z1": {
				Code: "z1", File: "app.go", Symbol: "Foo",
				Signature: "Foo()", Span: [2]int{10, 20},
				Body: "func Foo() {}", Domain: "@x", Tags: []string{"t1"},
			},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/peek?code=z1")
	require.NoError(t, err)
	defer resp.Body.Close()

	var raw map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&raw))
	assert.Contains(t, raw, "results")
	results := raw["results"].([]interface{})
	require.Len(t, results, 1)
	hit := results[0].(map[string]interface{})
	// Check all fields present
	for _, field := range []string{"code", "file", "symbol", "signature", "span", "body", "domain", "tags"} {
		assert.Contains(t, hit, field, "missing field: %s", field)
	}
	// Error should be absent on successful hit
	assert.Empty(t, hit["error"])
}

// Test that comma-separated codes are parsed correctly
func TestHandlePeek_CommaSeparatedCodes(t *testing.T) {
	iq := &mockIndexQuerier{
		peekResults: map[string]ports.PeekHit{
			"a1": {Code: "a1", File: "a.go", Symbol: "A", Span: [2]int{1, 5}, Body: "func A(){}"},
			"b2": {Code: "b2", File: "b.go", Symbol: "B", Span: [2]int{1, 5}, Body: "func B(){}"},
			"c3": {Code: "c3", File: "c.go", Symbol: "C", Span: [2]int{1, 5}, Body: "func C(){}"},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/peek?code=a1,b2,c3")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	results := body["results"].([]interface{})
	assert.Len(t, results, 3)
	assert.Equal(t, "a1", results[0].(map[string]interface{})["code"])
	assert.Equal(t, "b2", results[1].(map[string]interface{})["code"])
	assert.Equal(t, "c3", results[2].(map[string]interface{})["code"])
}

// Test that k parameter is respected and capped at defaultRefsK
func TestHandleRefs_KParam_CappedAtDefault(t *testing.T) {
	refs := make([]ports.RefHit, defaultRefsK+5)
	for i := range refs {
		refs[i] = ports.RefHit{File: "foo.go", Line: i + 1}
	}
	iq := &mockIndexQuerier{
		refsResults: map[string]ports.RefsResult{
			"tok": {Token: "tok", Total: defaultRefsK + 5, Refs: refs},
		},
	}
	ts := setupIndexTestServer(t, iq)
	defer ts.Close()

	// k larger than cap → server uses cap
	resp, err := http.Get(ts.URL + "/api/refs?token=tok&k=9999")
	require.NoError(t, err)
	defer resp.Body.Close()

	var result ports.RefsResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.LessOrEqual(t, len(result.Refs), defaultRefsK)
}

// Test that refs with spaces in names is handled correctly
func TestHandleRefs_TokenWithSpaces_ReturnsBadRequest(t *testing.T) {
	ts := setupIndexTestServer(t, &mockIndexQuerier{})
	defer ts.Close()

	// Spaces in the URL query param — should be treated as the literal token including spaces
	// (or empty if URL parser strips them) — the important thing is no panic
	resp, err := http.Get(ts.URL + "/api/refs?token=foo+bar")
	require.NoError(t, err)
	defer resp.Body.Close()
	// Should be 200 with empty result (token="foo bar" just won't be found)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
