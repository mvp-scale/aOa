// Package hooks provides the embedded status line hook script.
package hooks

import "embed"

//go:embed aoa-status-line.sh
var FS embed.FS
