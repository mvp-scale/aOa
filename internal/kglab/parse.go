package kglab

import (
	"bufio"
	"fmt"
	"strings"
)

// parse.go — the minimal .aoa authoring-language parser. Turns a human-written
// estate file into the structs the engine already runs (View + []TargetFact).
// This is the missing primitive: it makes the AUTHORING STRUCTURE real, instead
// of hand-authored Go struct literals. Intentionally tiny (line-based, stdlib).

// EstateSpec is a parsed .aoa authoring document.
type EstateSpec struct {
	Name    string
	View    string       // component | dsm | cycles
	Allowed []TargetFact // the declared target: edges that SHOULD exist
	Forbid  []string     // documented forbidden edges (shown, enforced implicitly)
}

// ParseEstate parses a minimal .aoa file. Grammar (one statement per line):
//
//	estate NAME
//	view   KIND
//	allow  FROM -> TO       # a target edge (part of where-we-need-to-be)
//	forbid FROM -> TO       # documented prohibition (also a VIOLATION if in real)
//	# comment
func ParseEstate(src string) (EstateSpec, error) {
	spec := EstateSpec{View: "component"}
	sc := bufio.NewScanner(strings.NewReader(src))
	ln := 0
	for sc.Scan() {
		ln++
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "estate":
			if len(fields) < 2 {
				return spec, fmt.Errorf("line %d: 'estate' needs a name", ln)
			}
			spec.Name = fields[1]
		case "view":
			if len(fields) < 2 {
				return spec, fmt.Errorf("line %d: 'view' needs a kind", ln)
			}
			spec.View = fields[1]
		case "allow", "forbid":
			from, to, err := parseArrow(strings.Join(fields[1:], " "), ln)
			if err != nil {
				return spec, err
			}
			if fields[0] == "allow" {
				spec.Allowed = append(spec.Allowed, TargetFact{Concept: "import", FromUnit: from, ToUnit: to})
			} else {
				spec.Forbid = append(spec.Forbid, from+" -> "+to)
			}
		default:
			return spec, fmt.Errorf("line %d: unknown keyword %q", ln, fields[0])
		}
	}
	return spec, sc.Err()
}

// parseArrow splits "FROM -> TO".
func parseArrow(s string, ln int) (string, string, error) {
	parts := strings.SplitN(s, "->", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("line %d: expected 'FROM -> TO'", ln)
	}
	from, to := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if from == "" || to == "" {
		return "", "", fmt.Errorf("line %d: empty side in 'FROM -> TO'", ln)
	}
	return from, to, nil
}
