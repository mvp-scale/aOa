package web

import (
	"encoding/json"
	"net/http"

	"github.com/corey/aoa/internal/adapters/recon"
)

// fileCacheAdapter adapts the search engine's FileCache to the recon.LineGetter interface.
type fileCacheAdapter struct {
	cache interface {
		GetLines(fileID uint32) []string
	}
}

func (a *fileCacheAdapter) GetLines(fileID uint32) []string {
	if a.cache == nil {
		return nil
	}
	return a.cache.GetLines(fileID)
}

func (s *Server) handleRecon(w http.ResponseWriter, r *http.Request) {
	if s.idx == nil || s.engine == nil {
		http.Error(w, `{"error":"index not available"}`, http.StatusServiceUnavailable)
		return
	}

	var lines recon.LineGetter
	if fc := s.engine.Cache(); fc != nil {
		lines = &fileCacheAdapter{cache: fc}
	}

	result := recon.Scan(s.idx, lines)

	// Wrap result with recon_available field for dashboard install prompt
	reconAvailable := false
	if s.queries != nil {
		reconAvailable = s.queries.ReconAvailable()
	}
	response := struct {
		*recon.Result
		ReconAvailable bool `json:"recon_available"`
	}{
		Result:         result,
		ReconAvailable: reconAvailable,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
