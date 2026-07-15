package lockfile

import (
	"sort"
	"strings"
)

// ParseGoMod parses a go.mod file's require/replace directives into
// Components. Handles both single-line (`require X vY`) and block
// (`require (\n\tX vY\n)`) forms, plus `// indirect` trailing comments and
// `replace X => Y [vZ]` overrides (local-path targets, i.e. no version, are
// flagged Unpinned=true — a filesystem replace has no resolvable pinned
// version). Does not handle `retract`/`exclude` directives or block comments
// (80/20 — those don't affect the SBOM's component/version/supplier answer).
//
// path is recorded verbatim on every Component (repo-relative, caller's
// choice) for G7 source attribution.
func ParseGoMod(path string, content []byte) ([]Component, error) {
	lines := strings.Split(string(content), "\n")

	byName := make(map[string]*Component)
	var order []string

	add := func(name, version, supplier string, lineNo int) {
		if c, ok := byName[name]; ok {
			c.Version = version
			c.Supplier = supplier
			c.Line = uint32(lineNo)
			return
		}
		byName[name] = &Component{
			Name:     name,
			Version:  version,
			Supplier: supplier,
			Language: "go",
			File:     path,
			Line:     uint32(lineNo),
		}
		order = append(order, name)
	}

	inRequireBlock := false
	inReplaceBlock := false
	for i, raw := range lines {
		lineNo := i + 1
		line := stripGoModComment(raw)
		trimmed := strings.TrimSpace(line)

		switch {
		case inRequireBlock:
			if trimmed == ")" {
				inRequireBlock = false
				continue
			}
			if name, version, ok := parseRequireFields(trimmed); ok {
				supplier := "direct"
				if isIndirectComment(raw) {
					supplier = "indirect"
				}
				add(name, version, supplier, lineNo)
			}
			continue

		case inReplaceBlock:
			if trimmed == ")" {
				inReplaceBlock = false
				continue
			}
			applyReplace(byName, &order, trimmed, path, lineNo)
			continue
		}

		switch {
		case trimmed == "require (":
			inRequireBlock = true
		case strings.HasPrefix(trimmed, "require "):
			if name, version, ok := parseRequireFields(strings.TrimPrefix(trimmed, "require ")); ok {
				supplier := "direct"
				if isIndirectComment(raw) {
					supplier = "indirect"
				}
				add(name, version, supplier, lineNo)
			}
		case trimmed == "replace (":
			inReplaceBlock = true
		case strings.HasPrefix(trimmed, "replace "):
			applyReplace(byName, &order, strings.TrimPrefix(trimmed, "replace "), path, lineNo)
		}
	}

	out := make([]Component, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// stripGoModComment removes a trailing "//"-style comment (but not the
// "// indirect" marker's meaning — callers that need it check the raw line
// separately via isIndirectComment before this strip is applied to it).
func stripGoModComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		return line[:i]
	}
	return line
}

func isIndirectComment(rawLine string) bool {
	i := strings.Index(rawLine, "//")
	if i < 0 {
		return false
	}
	return strings.TrimSpace(rawLine[i+2:]) == "indirect"
}

// parseRequireFields splits "module/path vVersion" into (name, version, ok).
// ok is false for blank lines or malformed entries (fewer than 2 fields).
func parseRequireFields(s string) (name, version string, ok bool) {
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return "", "", false
	}
	return fields[0], fields[1], true
}

// applyReplace handles one "X [vOld] => Y [vNew]" replace directive body
// (the part after the "replace " keyword has already been stripped by the
// caller). Only the target (Y, vNew) matters for the Component record: a
// replace directive substitutes what actually gets built.
func applyReplace(byName map[string]*Component, order *[]string, body, path string, lineNo int) {
	parts := strings.SplitN(body, "=>", 2)
	if len(parts) != 2 {
		return
	}
	oldFields := strings.Fields(parts[0])
	newFields := strings.Fields(parts[1])
	if len(oldFields) == 0 || len(newFields) == 0 {
		return
	}
	oldName := oldFields[0]

	// Two shapes with different SBOM identity semantics:
	//   - "replace X => github.com/fork/y vZ" (versioned module target): the
	//     shipped code really is a different named+versioned package — report
	//     the fork's identity.
	//   - "replace X => ../local/path" (filesystem target): there is no
	//     resolvable name/version at all; the import path everywhere in code
	//     is still X, just now backed by an on-disk copy — keep X's identity,
	//     flag Unpinned (no pin exists to report), version cleared.
	name, version, unpinned := oldName, "", true
	if len(newFields) >= 2 {
		name, version, unpinned = newFields[0], newFields[1], false
	}

	if c, ok := byName[oldName]; ok {
		c.Name = name
		c.Version = version
		c.Supplier = "replace"
		c.Unpinned = unpinned
		c.Line = uint32(lineNo)
		c.File = path
		return
	}
	byName[oldName] = &Component{
		Name:     name,
		Version:  version,
		Supplier: "replace",
		Language: "go",
		Unpinned: unpinned,
		File:     path,
		Line:     uint32(lineNo),
	}
	*order = append(*order, oldName)
}
