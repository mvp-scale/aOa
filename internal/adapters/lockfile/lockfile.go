// Package lockfile parses dependency manifests (go.mod/go.sum, package.json)
// into a small, ecosystem-neutral Component record. VL-1a (board #35): the
// reader half of "lockfile parser -> dep facts -> SBOM table view".
//
// Deliberately text-first and stdlib-only (no golang.org/x/mod, no npm SDK):
// the grammars parsed here are a well-known subset (require/replace blocks;
// dependencies/devDependencies/optionalDependencies/peerDependencies maps),
// not full spec compliance — 80/20 per the owner ruling (no over-engineering).
//
// Readers are pure: (path, content) -> ([]Component, error). No file I/O, no
// FactStore writes happen in this package — the app layer decides whether/how
// to persist (see internal/app for the derive-time call site). This keeps the
// readers trivially unit-testable and avoids feeding SBOM component specs
// into the code-import FactDep stream the compactor (internal/domain/facts)
// already resolves into the unit dependency graph — a different, non-code
// dependency concept that must not corrude that graph (D1-D6: no side-channel
// writes, and just as importantly, no cross-contamination of an existing
// substrate with a differently-shaped fact).
package lockfile

import "strings"

// Component is one detected dependency/component entry from a manifest file.
// Mirrors arch.Component (internal/domain/arch/model.go) field-for-field;
// the app layer converts between the two at the boundary (D25 pattern —
// adapt at the boundary rather than importing domain types into an adapter).
type Component struct {
	Name     string // module import path (go) or package name (npm)
	Version  string // as declared; empty for unversioned local replace targets
	Supplier string // "direct" | "indirect" | "replace" | "dev" | "optional" | "peer"
	Language string // "go" | "js"
	Unpinned bool   // true when the manifest does not pin a single resolvable version
	File     string // manifest file path, as passed to the reader (repo-relative)
	Line     uint32 // 1-based line in File where this component was declared (G7)
}

// findLine returns the 1-based line number of the first line in content that
// contains needle, or fallback if not found. Cheap, honest G7 provenance for
// formats (JSON) that don't carry line numbers through encoding/json — a real
// search of the actual file text, not a fabricated pointer.
func findLine(lines []string, needle string, fallback uint32) uint32 {
	for i, l := range lines {
		if strings.Contains(l, needle) {
			return uint32(i + 1)
		}
	}
	return fallback
}
