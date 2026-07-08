package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/corey/aoa/internal/ports"
)

// defaultRefsK is the default and hard server cap for /api/refs result count.
// Law 3: the tool never dumps the corpus. C10: ranked cursors, never full-posting materialization.
const defaultRefsK = 20

// handlePeek serves GET /api/peek?code=<code>[,<code>...]
//
// Returns resolved source bodies for one or more peek codes.
// Always-on (no lean guard): peek is L1 index data, not L5 arch data — works in all builds.
//
// Response shapes:
//   - Single unknown code: 404 {"error":"...", "code":"..."}
//   - Single found code:   200 {"results":[{file, symbol, signature, span, body, domain?, tags?}]}
//   - Multiple codes:      200 {"results":[...]} (errors embedded per item)
//
// C4: returns 503 when the index querier is unavailable.
func (s *Server) handlePeek(w http.ResponseWriter, r *http.Request) {
	q := s.indexQuerier(w, r)
	if q == nil {
		return
	}

	raw := r.URL.Query().Get("code")
	if raw == "" {
		http.Error(w, `{"error":"code parameter required"}`, http.StatusBadRequest)
		return
	}

	// Split comma-separated codes; trim whitespace from each.
	parts := strings.Split(raw, ",")
	codes := make([]string, 0, len(parts))
	for _, p := range parts {
		if c := strings.TrimSpace(p); c != "" {
			codes = append(codes, c)
		}
	}
	if len(codes) == 0 {
		http.Error(w, `{"error":"code parameter required"}`, http.StatusBadRequest)
		return
	}
	// Law 3 (answers are O(answer), never O(corpus)): each code costs a map
	// lookup + a disk read — cap the batch so a single request stays bounded.
	const maxCodes = 50
	if len(codes) > maxCodes {
		http.Error(w, `{"error":"too many codes: max 50 per request"}`, http.StatusBadRequest)
		return
	}

	hits, err := q.Peek(codes)
	if err != nil {
		http.Error(w, `{"error":"peek failed"}`, http.StatusInternalServerError)
		return
	}

	// For a single code that errored, return 404 with machine-readable reason.
	if len(codes) == 1 && len(hits) == 1 && hits[0].Error != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": hits[0].Error,
			"code":  codes[0],
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"results": hits,
	})
}

// handleRefs serves GET /api/refs?token=<name>&k=<K>
//
// Returns ranked reference sites for a token. Contract C10: "name references (textual)".
// Always-on (no lean guard): refs is L1 index data.
//
// K default 20, hard server cap 20 (Law 3: no corpus dumps).
// O(K) not O(corpus): the posting list is never fully materialized.
//
// Response: {token, total, refs:[{file, line, symbol?, peek?}], truncated:bool}
// Unknown token returns 200 with total=0 and empty refs (honest absence, not an error).
func (s *Server) handleRefs(w http.ResponseWriter, r *http.Request) {
	q := s.indexQuerier(w, r)
	if q == nil {
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, `{"error":"token parameter required"}`, http.StatusBadRequest)
		return
	}

	// Parse k param; enforce hard server cap.
	k := defaultRefsK
	if ks := r.URL.Query().Get("k"); ks != "" {
		if n, err := strconv.Atoi(ks); err == nil && n > 0 {
			if n < defaultRefsK {
				k = n
			}
			// else: keep defaultRefsK as the hard cap
		}
	}

	result := q.Refs(token, k)

	// Ensure refs is never null in JSON (empty array, not null)
	if result.Refs == nil {
		result.Refs = []ports.RefHit{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	_ = json.NewEncoder(w).Encode(result)
}

// indexQuerier returns the IndexQuerier or writes a 503 and returns nil.
// Callers must check for nil and return immediately.
// This is the always-on equivalent of archQuerier — no C4 kill-switch.
func (s *Server) indexQuerier(w http.ResponseWriter, r *http.Request) ports.IndexQuerier {
	if s.queries == nil {
		http.Error(w, `{"error":"index not available"}`, http.StatusServiceUnavailable)
		return nil
	}
	q := s.queries.IndexQuerier()
	if q == nil {
		http.Error(w, `{"error":"index not available"}`, http.StatusServiceUnavailable)
		return nil
	}
	return q
}
