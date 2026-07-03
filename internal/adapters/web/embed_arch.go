//go:build !lean

package web

import "embed"

// archStaticFS contains the architecture viewer static assets.
// Excluded from lean builds via the !lean build tag (T-6).
//
//go:embed static/arch
var archStaticFS embed.FS
