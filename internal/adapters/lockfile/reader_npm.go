package lockfile

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// packageJSON is the subset of package.json fields this reader cares about.
// Everything else (scripts, bin, keywords, ...) is ignored.
type packageJSON struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
}

// npmSection pairs one dependency map with its supplier label, in the fixed
// iteration order that keeps output deterministic before the final sort.
type npmSection struct {
	deps     map[string]string
	supplier string
}

// ParsePackageJSON parses the four standard npm dependency maps
// (dependencies, devDependencies, optionalDependencies, peerDependencies)
// into Components. A package appearing in more than one section is recorded
// once, per the first section below it appears in (dependencies takes
// priority — matches npm's own "a direct dep wins over a peer/optional
// listing of the same name" resolution intent).
//
// Unpinned: true unless the version string is an exact semver-shaped pin
// (digits.digits.digits, optionally with a pre-release/build suffix) — any
// range operator (^, ~, >=, <, ||), wildcard (*, x), tag ("latest"), or
// non-registry spec (git url, "file:", "workspace:") is not a single
// resolvable version (D2/D17 honesty: real ambiguity, not invented risk).
func ParsePackageJSON(path string, content []byte) ([]Component, error) {
	var pj packageJSON
	if err := json.Unmarshal(content, &pj); err != nil {
		return nil, fmt.Errorf("lockfile.ParsePackageJSON: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	sections := []npmSection{
		{pj.Dependencies, "direct"},
		{pj.DevDependencies, "dev"},
		{pj.OptionalDependencies, "optional"},
		{pj.PeerDependencies, "peer"},
	}

	seen := make(map[string]bool)
	var out []Component
	for _, sec := range sections {
		names := make([]string, 0, len(sec.deps))
		for name := range sec.deps {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if seen[name] {
				continue
			}
			seen[name] = true
			version := sec.deps[name]
			out = append(out, Component{
				Name:     name,
				Version:  version,
				Supplier: sec.supplier,
				Language: "js",
				Unpinned: !isPinnedNpmVersion(version),
				File:     path,
				Line:     findLine(lines, `"`+name+`"`, 1),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// isPinnedNpmVersion reports whether v names exactly one resolvable version:
// digits.digits.digits with an optional -prerelease/+build suffix, no range
// operator or wildcard prefix.
func isPinnedNpmVersion(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	if v[0] < '0' || v[0] > '9' {
		return false // ^, ~, >=, <, ||, *, x, "latest", "git+...", "file:", "workspace:", ...
	}
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return false
	}
	for i, p := range parts {
		if i == 2 {
			// Trailing segment may carry -prerelease/+build; only the
			// leading digit run needs to be numeric.
			p = strings.FieldsFunc(p, func(r rune) bool { return r == '-' || r == '+' })[0]
		}
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
