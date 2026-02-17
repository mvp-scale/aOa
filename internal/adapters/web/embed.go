// Package web serves an embedded HTML dashboard over HTTP.
// Binds to localhost only — no network exposure, no auth needed.
package web

import "embed"

//go:embed static/index.html
var staticFS embed.FS
