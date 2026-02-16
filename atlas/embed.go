// Package atlas embeds the universal domain structure for compile-time inclusion.
// The atlas is a set of JSON files defining 134 domains across 15 focus areas.
// Each domain has terms, and each term has keywords — all post-tokenization code tokens.
//
// Usage:
//
//	enricher.LoadAtlas(atlas.FS, "v1")
package atlas

import "embed"

//go:embed v1/*.json
var FS embed.FS
