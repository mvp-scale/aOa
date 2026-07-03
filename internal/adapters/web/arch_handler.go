//go:build !lean

package web

import (
	"encoding/json"
	"io/fs"
	"net/http"
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
	archStatic, _ := fs.Sub(archStaticFS, "static/arch")
	archFileServer := http.FileServerFS(archStatic)

	serveArchStatic := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.queries == nil || s.queries.Arch() == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		archFileServer.ServeHTTP(w, r)
	})

	// Serve /arch/ and all sub-paths (viewer.js, viewer.css, vendor/*)
	mux.Handle("GET /arch/", http.StripPrefix("/arch", serveArchStatic))

	// API routes
	mux.HandleFunc("GET /api/arch/manifest", s.withETag(s.handleArchManifest))
	mux.HandleFunc("GET /api/arch/standards", s.handleArchStandards)
	mux.HandleFunc("GET /api/arch/{path...}", s.withETag(s.handleArchShard))
}

// handleArchManifest synthesizes the estates-shaped manifest the viewer requires.
// The Go-internal flat manifest {scope,rev,views:[]} is transformed into:
//
//	{schema,sharded,generated:{timestamp},estates:{local:{scopes:{local:{label,views:{vid:{...shard}}}}}}}
//
// Timestamp is injected at SERVE-TIME only — stored shards stay byte-stable
// so the byte-identity AC (CLI JSON == browser shard) holds on shards.
// C4: returns 404 when arch is disabled.
func (s *Server) handleArchManifest(w http.ResponseWriter, r *http.Request) {
	q := s.archQuerier(w)
	if q == nil {
		return
	}

	m, err := q.Manifest("local")
	if err != nil {
		http.Error(w, `{"error":"manifest unavailable"}`, http.StatusInternalServerError)
		return
	}
	if m == nil {
		// No shards derived yet — return empty but valid estates manifest
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(emptyEstatesManifest())
		return
	}

	estate := buildEstatesManifest(m, q)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(estate)
}

// emptyEstatesManifest returns a valid estates manifest with no views yet.
func emptyEstatesManifest() map[string]interface{} {
	return map[string]interface{}{
		"schema":  "aoa.archmodel/v1",
		"sharded": true,
		"generated": map[string]interface{}{
			"timestamp": time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		},
		"estates": map[string]interface{}{
			"local": map[string]interface{}{
				"label": "aOa",
				"scopes": map[string]interface{}{
					"local": map[string]interface{}{
						"label": "aOa",
						"tech":  "Go",
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
}

type provJSON struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

// buildEstatesManifest converts the flat Go manifest into the estates shape the viewer expects.
// For each view it reads the minimal shard header (kind, title, count, prov) from the store
// so the viewer can render the catalog and header bar before the full shard is lazy-loaded.
//
// Timestamp is injected at serve-time; shards are byte-stable (T-4 ruling).
func buildEstatesManifest(m *ports.ArchManifest, q ports.ArchQuerier) map[string]interface{} {
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
			"kind":  hdr.Kind,
			"title": hdr.Title,
			"count": hdr.Count,
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

	return map[string]interface{}{
		"schema":  "aoa.archmodel/v1",
		"sharded": true,
		"generated": map[string]interface{}{
			"timestamp": time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		},
		"estates": map[string]interface{}{
			"local": map[string]interface{}{
				"label": "aOa",
				"scopes": map[string]interface{}{
					"local": map[string]interface{}{
						"label": "aOa",
						"tech":  "Go",
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
	if s.archQuerier(w) == nil {
		return
	}
	data, err := archStaticFS.ReadFile("static/arch/view-standards.json")
	if err != nil {
		http.Error(w, `{"error":"standards unavailable"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(data)
}

// handleArchShard serves a single shard by scope/view path.
// Route: GET /api/arch/{path...}
// The viewer fetches: /api/arch/{scope}/{id}?v={hash}
// Byte-identity: the bytes returned here are IDENTICAL to `aoa arch view <id>` CLI output.
// C4: returns 404 when arch is disabled.
func (s *Server) handleArchShard(w http.ResponseWriter, r *http.Request) {
	q := s.archQuerier(w)
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

	// Immutable cache: the ?v=hash query param means this exact hash is content-addressed.
	// Cache-Control: immutable is safe; the URL changes when the content changes.
	if r.URL.Query().Get("v") != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// archQuerier returns the ArchQuerier for this server, or writes a 404 and returns nil.
// Callers must check for nil and return immediately (C4 kill-switch).
func (s *Server) archQuerier(w http.ResponseWriter) ports.ArchQuerier {
	if s.queries == nil {
		http.NotFound(w, nil)
		return nil
	}
	q := s.queries.Arch()
	if q == nil {
		http.NotFound(w, nil)
		return nil
	}
	return q
}
