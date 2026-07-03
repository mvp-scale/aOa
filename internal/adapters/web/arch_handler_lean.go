//go:build lean

package web

import "net/http"

// registerArchRoutes is a no-op in lean builds (arch viewer excluded via !lean tag).
func (s *Server) registerArchRoutes(mux *http.ServeMux) {}
