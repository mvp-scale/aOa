package ports

// IndexQuerier provides read-only access to the search index for HTTP handlers.
// These routes are always-on (no lean guard): peek and refs are L1 structure data,
// not L5 arch data. They work in all builds whenever the index is populated.
type IndexQuerier interface {
	// Peek resolves one or more peek codes to source bodies.
	// Returns one PeekHit per code (in order). Unknown codes have Error set.
	Peek(codes []string) ([]PeekHit, error)

	// Refs looks up all posting-list entries for a token string.
	// Returns at most k results. k=0 uses the server default cap.
	// O(k) not O(corpus): posting list is never fully materialized.
	Refs(token string, k int) RefsResult
}

// PeekHit is a single resolved peek code result for /api/peek.
type PeekHit struct {
	Code      string   `json:"code"`
	File      string   `json:"file,omitempty"`
	Symbol    string   `json:"symbol,omitempty"`
	Signature string   `json:"signature,omitempty"`
	Span      [2]int   `json:"span,omitempty"`
	Body      string   `json:"body,omitempty"`
	Domain    string   `json:"domain,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// RefHit is a single reference location for /api/refs.
type RefHit struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Symbol string `json:"symbol,omitempty"`
	Peek   string `json:"peek,omitempty"`
}

// RefsResult is the result of a Refs lookup.
type RefsResult struct {
	Token     string   `json:"token"`
	Total     int      `json:"total"`
	Refs      []RefHit `json:"refs"`
	Truncated bool     `json:"truncated"`
}
