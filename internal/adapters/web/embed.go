// Package web serves an embedded HTML dashboard over HTTP.
// Binds to localhost only — no network exposure, no auth needed.
package web

import "embed"

// staticFS contains the dashboard static assets (non-arch).
// The arch-specific static files live in a separately-tagged embed (embed_arch.go, !lean).
//
//go:embed static/app.js static/hero.json static/index.html static/style.css
var staticFS embed.FS
