//go:build !lean

package web

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/corey/aoa/internal/ports"
)

// registerArchRoutes adds the architecture viewer routes to mux.
// Called from Server.Start(). No-op in lean builds (see arch_handler_lean.go).
//
// C4: all routes return 404 when arch is disabled (queries.Arch() == nil).
// F-6: /arch/* is served via a dedicated fenced route, NOT the catch-all GET /,
// so it correctly 404s when arch is off even though static/arch is NOT in staticFS.
func (s *Server) registerArchRoutes(mux *http.ServeMux) {
	// /arch/ page — serves the viewer HTML + static assets.
	// C4: returns 404 when arch is disabled.
	//
	// The vendor bundle is stored pre-compressed as vendor/bundle.js.gz (≈537 KB)
	// to stay within the git 1 MB per-file limit; requests for vendor/bundle.js are
	// intercepted and served with Content-Encoding: gzip so browsers decode it
	// transparently as a normal ES module.
	archStatic, _ := fs.Sub(archStaticFS, "static/arch")
	archFileServer := http.FileServerFS(archStatic)

	serveArchStatic := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.queries == nil || s.queries.Arch() == nil {
			http.NotFound(w, r)
			return
		}
		// Intercept vendor/bundle.js → serve vendor/bundle.js.gz with gzip encoding.
		// The browser's ES module loader handles Content-Encoding: gzip transparently.
		// Clients that don't accept gzip get the bytes decompressed (RFC 7231 §3.1.2.2).
		if r.URL.Path == "/vendor/bundle.js" {
			data, err := archStaticFS.ReadFile("static/arch/vendor/bundle.js.gz")
			if err != nil {
				http.Error(w, "bundle unavailable", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/javascript")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Vary", "Accept-Encoding")
			if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				w.Header().Set("Content-Encoding", "gzip")
				_, _ = w.Write(data)
				return
			}
			zr, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				http.Error(w, "bundle unavailable", http.StatusInternalServerError)
				return
			}
			defer zr.Close()
			_, _ = io.Copy(w, zr)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		archFileServer.ServeHTTP(w, r)
	})

	// Serve /arch/ and all sub-paths (viewer.js, viewer.css, vendor/*)
	mux.Handle("GET /arch/", http.StripPrefix("/arch", serveArchStatic))

	// API routes
	// L19.20: manifest ETag is handled inside handleArchManifest using m.Rev (tighter than
	// global revision — 304 only when arch facts unchanged; see handleArchManifest for rationale).
	mux.HandleFunc("GET /api/arch/manifest", s.handleArchManifest)
	mux.HandleFunc("GET /api/arch/standards", s.handleArchStandards)
	// PF4: arch shard route must NOT be wrapped in withETag.
	// withETag answers 304 before the archQuerier C4 fence runs — a disabled daemon
	// would leak existence information to holders of a stale ETag.  The shard route
	// uses content-addressed ?v=hash immutable caching instead; the two strategies are
	// incompatible, so withETag is dropped here (resolves cop F1 + F4 together).
	// Terrain knowledge graph endpoint — live substrate (not a rendered shard).
	// Go 1.22 ServeMux: /api/arch/graph (literal) wins over /api/arch/{path...}
	// (wildcard) by specificity — registration order is irrelevant.
	mux.HandleFunc("GET /api/arch/graph", s.handleArchGraph)
	mux.HandleFunc("GET /api/arch/findings", s.handleArchFindings)
	mux.HandleFunc("GET /api/arch/{path...}", s.handleArchShard)
}

// handleArchManifest synthesizes the estates-shaped manifest the viewer requires.
// The Go-internal flat manifest {scope,rev,views:[]} is transformed into:
//
//	{schema,sharded,generated:{timestamp},estates:{local:{scopes:{local:{label,views:{vid:{...shard}}}}}}}
//
// Timestamp is injected at SERVE-TIME only — stored shards stay byte-stable
// so the byte-identity AC (CLI JSON == browser shard) holds on shards.
// C4: returns 404 when arch is disabled.
//
// L19.20 ETag strategy: ETag = m.Rev (12-char factsHash), NOT the global revision counter.
//   - Tighter invalidation: 304 only when arch facts are unchanged; 200 after any re-derive.
//   - Zero-symbol file change: if facts are unchanged, m.Rev is unchanged → 304 is CORRECT
//     (the view truly didn't change). Board AC "no stale 304" = no 304 when the view changed;
//     304 on an unchanged view is honest. See L19.20 commit for the recorded reading.
//   - Timestamp safety: ETag is m.Rev (not the serialised body), so the serve-time
//     generated.timestamp injection never causes spurious 200 responses.
func (s *Server) handleArchManifest(w http.ResponseWriter, r *http.Request) {
	q := s.archQuerier(w, r)
	if q == nil {
		return
	}

	// PF7: derive estate label and tech from project context (not hardcoded).
	label := s.estateLabel()
	tech := s.estateTech()

	m, err := q.Manifest("local")
	if err != nil {
		http.Error(w, `{"error":"manifest unavailable"}`, http.StatusInternalServerError)
		return
	}
	if m == nil {
		// No shards derived yet — return empty but valid estates manifest (no ETag: no Rev).
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(emptyEstatesManifest(label, tech))
		return
	}

	// ETag from m.Rev — see function-level doc for rationale.
	etag := m.Rev
	if etag != "" {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
	}

	estate := buildEstatesManifest(m, q, label, tech)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(estate)
}

// emptyEstatesManifest returns a valid estates manifest with no views yet.
// label is the project root basename (PF7); tech is the dominant language (may be "").
// No manifest has ever been derived in this state, so there is no DerivedAt
// to serve honestly — time.Now() here just marks "checked now, nothing
// derived" and carries no staleness-masking risk (T65 only concerns a
// PERSISTED manifest's age, which this path by definition does not have).
func emptyEstatesManifest(label, tech string) map[string]interface{} {
	return map[string]interface{}{
		"schema":  "aoa.archmodel/v1",
		"sharded": true,
		"generated": map[string]interface{}{
			"timestamp": time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		},
		"estates": map[string]interface{}{
			"local": map[string]interface{}{
				"label": label,
				"scopes": map[string]interface{}{
					"local": map[string]interface{}{
						"label": label,
						"tech":  tech,
						"views": map[string]interface{}{},
					},
				},
			},
		},
	}
}

// shardHeader is the minimal subset of a shard JSON blob used for manifest synthesis.
// We unmarshal only the fields needed by the viewer before the shard is lazy-loaded.
type shardHeader struct {
	Kind  string   `json:"kind"`
	Title string   `json:"title"`
	Count string   `json:"count"`
	Prov  provJSON `json:"prov"`
	// FindingsClause: A3 calm-default split — the "· ⚠ N findings" tail DeriveCaption used to
	// bake into Count. Kept separate so the viewer appends it only when its Findings lens is on.
	FindingsClause string `json:"findingsClause,omitempty"`
}

type provJSON struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

// buildEstatesManifest converts the flat Go manifest into the estates shape the viewer expects.
// For each view it reads the minimal shard header (kind, title, count, prov) from the store
// so the viewer can render the catalog and header bar before the full shard is lazy-loaded.
//
// label is the project root basename (PF7); tech is the dominant language (may be "").
// Timestamp is injected at serve-time; shards are byte-stable (T-4 ruling).
func buildEstatesManifest(m *ports.ArchManifest, q ports.ArchQuerier, label, tech string) map[string]interface{} {
	views := make(map[string]interface{})

	for _, ve := range m.Views {
		// Fetch the shard bytes to extract the header fields.
		shardBytes, err := q.View(m.Scope, ve.ID)
		var hdr shardHeader
		if err == nil && shardBytes != nil {
			_ = json.Unmarshal(shardBytes, &hdr)
		}
		if hdr.Kind == "" {
			hdr.Kind = "simple" // safe fallback
		}

		// Shard path is relative to BASE (= /api/arch/).
		// The viewer fetches: BASE + shard.path + "?v=" + shard.hash
		// → /api/arch/{scope}/{id}?v={hash}
		shardPath := m.Scope + "/" + ve.ID

		views[ve.ID] = map[string]interface{}{
			"kind":           hdr.Kind,
			"title":          hdr.Title,
			"count":          hdr.Count,
			"findingsClause": hdr.FindingsClause,
			"prov": map[string]interface{}{
				"kind":  hdr.Prov.Kind,
				"label": hdr.Prov.Label,
			},
			"shard": map[string]interface{}{
				"path": shardPath,
				"hash": ve.Hash,
			},
		}
	}

	// T65: serve the DERIVE-time stamp (captured once by deriveArch, right
	// before persisting), never a fresh time.Now() here — a serve-time stamp
	// is what let week-old shards claim "current · code as of now" (F-2).
	// Fall back to time.Now() only when a manifest predates T65 (DerivedAt
	// empty) so the chip still renders something rather than a blank field.
	ts := m.DerivedAt
	if ts == "" {
		ts = time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	}

	return map[string]interface{}{
		"schema":  "aoa.archmodel/v1",
		"sharded": true,
		"generated": map[string]interface{}{
			"timestamp": ts,
		},
		"estates": map[string]interface{}{
			"local": map[string]interface{}{
				"label": label,
				"scopes": map[string]interface{}{
					"local": map[string]interface{}{
						"label": label,
						"tech":  tech,
						"views": views,
					},
				},
			},
		},
	}
}

// handleArchStandards serves the embedded view-standards.json content.
// The viewer fetches this at boot to populate VIEW_INTENT (per-view design standard)
// and named_palettes. No ETag — content is static (embedded at build time).
// C4: returns 404 when arch is disabled.
func (s *Server) handleArchStandards(w http.ResponseWriter, r *http.Request) {
	if s.archQuerier(w, r) == nil {
		return
	}
	data, err := archStaticFS.ReadFile("static/arch/view-standards.json")
	if err != nil {
		http.Error(w, `{"error":"standards unavailable"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

// handleArchGraph serves the raw substrate knowledge graph for the Terrain tab.
// Route: GET /api/arch/graph?grain=file|unit (default: file)
//
// This is NOT a rendered shard — it is computed live from the raw import edges
// (LoadAllEdges + in-memory aggregation) and reflects the actual substrate at
// the time of the request. Numbers are real: file grain gives ~310 nodes /
// ~1,700 edges on a typical mid-size Go project.
//
// SIZE GUARD (honest): if grain=file produces >20,000 edges, the response
// switches to grain=unit and sets "downgraded" — never a silent truncation.
// C4: returns 404 when arch is disabled.
// C1: the implementation clones the index (for unit grain) outside App.mu.
func (s *Server) handleArchGraph(w http.ResponseWriter, r *http.Request) {
	q := s.archQuerier(w, r)
	if q == nil {
		return // C4: 404 already written
	}

	grain := r.URL.Query().Get("grain")
	if grain == "" {
		grain = "file"
	}
	if grain != "file" && grain != "unit" {
		http.Error(w, `{"error":"grain must be 'file' or 'unit'"}`, http.StatusBadRequest)
		return
	}

	data, err := q.Graph("local", grain)
	if err != nil {
		http.Error(w, `{"error":"graph unavailable"}`, http.StatusInternalServerError)
		return
	}
	if data == nil {
		// No edges yet — substrate empty.
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

// handleArchFindings serves the arch detector findings for a scope.
// Route: GET /api/arch/findings?scope=<scope> (default: local)
//
// Returns the JSON-encoded []ports.Finding slice computed by the daemon detectors.
// Returns "[]" (empty JSON array) when no findings have been computed yet.
// C4: returns 404 when arch is disabled.
func (s *Server) handleArchFindings(w http.ResponseWriter, r *http.Request) {
	q := s.archQuerier(w, r)
	if q == nil {
		return
	}
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "local"
	}
	data, err := q.Findings(scope)
	if err != nil {
		http.Error(w, `{"error":"findings unavailable"}`, http.StatusInternalServerError)
		return
	}
	if data == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte("[]"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(data)
}

// handleArchShard serves a single shard by scope/view path.
// Route: GET /api/arch/{path...}
// The viewer fetches: /api/arch/{scope}/{id}?v={hash}
//
// Byte-identity (PF2): the bytes returned here are IDENTICAL to `aoa arch view <id>` CLI output.
// C4 (PF4): archQuerier fence runs BEFORE any cache logic — no withETag wrapper on this route.
//
// PF5 — Immutable-cache honesty:
// When ?v=hash is present, the served bytes are verified against contentHash(data).
// If the hash doesn't match (shard was re-derived since the manifest was fetched), 404 is
// returned so the browser discards the stale URL and refetches the manifest for the new hash.
// Only when the hash matches is Cache-Control: immutable stamped.
func (s *Server) handleArchShard(w http.ResponseWriter, r *http.Request) {
	q := s.archQuerier(w, r) // PF4: C4 fence first — no withETag above
	if q == nil {
		return
	}

	// path is "{scope}/{id}" extracted from the wildcard
	path := r.PathValue("path")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		http.NotFound(w, r)
		return
	}
	scope, id := parts[0], parts[1]

	data, err := q.View(scope, id)
	if err != nil {
		http.Error(w, `{"error":"shard unavailable"}`, http.StatusInternalServerError)
		return
	}
	if data == nil {
		http.NotFound(w, r)
		return
	}

	if v := r.URL.Query().Get("v"); v != "" {
		// PF5: verify the requested hash matches actual content before serving immutable.
		// If the shard was re-derived, contentHash(data) will differ from the requested v;
		// 404 tells the client to discard the stale URL and refetch the manifest.
		if contentHash(data) != v {
			http.Error(w, `{"error":"stale hash; refetch manifest"}`, http.StatusNotFound)
			return
		}
		// Hash verified — safe to cache immutably (content-addressed URL).
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// archQuerier returns the ArchQuerier for this server, or writes a 404 and returns nil.
// Callers must check for nil and return immediately (C4 kill-switch).
// PF4: request r is passed so http.NotFound can log correctly (no nil-request).
func (s *Server) archQuerier(w http.ResponseWriter, r *http.Request) ports.ArchQuerier {
	if s.queries == nil {
		http.NotFound(w, r)
		return nil
	}
	q := s.queries.Arch()
	if q == nil {
		http.NotFound(w, r)
		return nil
	}
	return q
}

// contentHash returns the first 12 hex characters of SHA-256(b).
// Algorithm matches arch.ContentHash in internal/domain/arch/marshal.go — kept local
// to avoid importing the domain package from this adapter.
func contentHash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:12]
}

// estateLabel derives the estate label from the project root basename.
// Falls back to "local" when the project root is unavailable.
// PF7: replaces the hardcoded "aOa" label.
func (s *Server) estateLabel() string {
	if s.queries == nil {
		return "local"
	}
	root := s.queries.ProjectConfig().ProjectRoot
	if root == "" {
		return "local"
	}
	return filepath.Base(root)
}

// estateTech derives the dominant programming language from the index file metadata.
// Returns "" when the index is absent or contains no language metadata.
// PF7: replaces the hardcoded "Go" label; returns "" when unknown (honest omission).
func (s *Server) estateTech() string {
	if s.idx == nil || len(s.idx.Files) == 0 {
		return ""
	}
	count := make(map[string]int)
	for _, f := range s.idx.Files {
		if f != nil && f.Language != "" {
			count[f.Language]++
		}
	}
	if len(count) == 0 {
		return ""
	}
	best, bestN := "", 0
	for lang, n := range count {
		if n > bestN || (n == bestN && lang < best) {
			best, bestN = lang, n
		}
	}
	return best
}
