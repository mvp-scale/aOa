// Package recon embeds the YAML rule definitions for dimensional analysis.
// This is a standalone package with no imports to avoid circular dependencies.
//
// Usage:
//
//	analyzer.LoadRulesFromFS(recon.FS, "rules")
package recon

import "embed"

//go:embed rules/*.yaml
var FS embed.FS
