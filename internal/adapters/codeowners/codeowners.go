// Package codeowners parses a repository's CODEOWNERS file (the GitHub/
// GitLab convention) into ownership rules for COL-3's Ownership (view id
// "ownership") view (board M6).
//
// D30 (extensionless disk read): CODEOWNERS carries no file extension, so
// the main index walk never sees it (internal/domain/index/walk.go skips
// zero-extension files) — this package re-reads it directly at derive time,
// the same "extensionless -> dedicated disk reader" discipline as vl7.go's
// readDockerfile. Probe order mirrors GitHub's own lookup convention: repo
// root, then .github/, then docs/ — first hit wins, no merging of multiple
// files.
//
// Pattern matching is deliberately 80/20 (owner ruling: no over-
// engineering): only directory-prefix patterns (e.g. "/internal/app/",
// "internal/app") and the catch-all "*" are resolved. No gitignore-style
// glob engine (**, brace expansion, char classes, negation) is implemented.
// A repo that leans on advanced CODEOWNERS glob features gets a documented
// weaker cut — some units may go unmatched here and fall back to COL-3's
// derived git-authorship reader — never a silent misattribution of an
// owner to the wrong unit.
package codeowners

import (
	"os"
	"path/filepath"
	"strings"
)

// probePaths are CODEOWNERS' conventional locations, checked in order
// (GitHub's own probe order: repo root, .github/, docs/). Relative to root.
var probePaths = []string{
	"CODEOWNERS",
	filepath.Join(".github", "CODEOWNERS"),
	filepath.Join("docs", "CODEOWNERS"),
}

// Rule is one parsed CODEOWNERS line: a path pattern plus its declared
// owners, with a G7 source pointer back to the declaring line.
type Rule struct {
	Pattern string   // path pattern as declared (e.g. "/internal/app/", "*")
	Owners  []string // declared owner tokens (e.g. "@alice", "team@org")
	File    string   // CODEOWNERS file path, relative to root (G7)
	Line    uint32   // 1-based line within File (G7)
}

// Read probes root's conventional CODEOWNERS locations (probePaths) and
// parses the first one found. Returns (nil, nil) — not an error — when no
// CODEOWNERS file exists anywhere on the probe path: an absent CODEOWNERS is
// an honest state for COL-3 (it falls back to derived git authorship), not
// a failure.
func Read(root string) ([]Rule, error) {
	for _, rel := range probePaths {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		return parse(rel, data), nil
	}
	return nil, nil
}

// parse reads CODEOWNERS' line grammar: blank lines and "#"-comments are
// skipped; the first whitespace-separated field is the pattern, the rest
// are owner tokens. A pattern with no owner tokens declares no ownership
// (skipped — not fabricated) since CODEOWNERS treats such a line as a
// no-op.
func parse(relPath string, data []byte) []Rule {
	var rules []Rule
	lines := strings.Split(string(data), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		rules = append(rules, Rule{
			Pattern: fields[0],
			Owners:  fields[1:],
			File:    relPath,
			Line:    uint32(i + 1),
		})
	}
	return rules
}

// Match resolves unitPath (a unit's repo-relative directory, aOa's "root"
// sentinel for the top-level directory) against rules using CODEOWNERS'
// "last matching rule wins" precedence. Returns ok=false when no rule
// matches — the caller's honest signal to fall back to derived git
// authorship rather than fabricate ownership.
func Match(rules []Rule, unitPath string) (owners []string, rule Rule, ok bool) {
	for i := len(rules) - 1; i >= 0; i-- {
		r := rules[i]
		if matches(r.Pattern, unitPath) {
			return r.Owners, r, true
		}
	}
	return nil, Rule{}, false
}

// matches reports whether pattern covers unitPath under the 80/20
// directory-prefix rule documented in the package doc.
func matches(pattern, unitPath string) bool {
	if pattern == "*" {
		return true
	}
	p := strings.Trim(pattern, "/")
	if p == "" {
		// A bare "/" pattern is CODEOWNERS' repo-root rule — it covers only
		// the top-level ("root") unit, never every unit (that's "*"'s job).
		return unitPath == "root"
	}
	if unitPath == "root" {
		return false
	}
	return unitPath == p || strings.HasPrefix(unitPath, p+"/")
}
