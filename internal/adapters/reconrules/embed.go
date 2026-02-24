// Package reconrules embeds the YAML rule definitions for dimensional analysis.
// This is a standalone package with no imports to avoid circular dependencies.
package reconrules

import "embed"

//go:embed rules/*.yaml
var FS embed.FS
