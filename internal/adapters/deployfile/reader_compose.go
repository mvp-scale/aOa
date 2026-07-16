package deployfile

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// rawComposeService mirrors the subset of the Compose Spec's service schema
// this reader cares about. Ports/DependsOn are decoded generically
// (interface{}) because the spec allows more than one shape for each: ports
// may be a string ("8080:80"), a bare int (9090), or a long-form mapping
// (target/published/protocol — not attempted in v1, 80/20 cut); depends_on
// may be a plain list of service names or a map keyed by service name with a
// condition value. normalizePorts/normalizeDependsOn below flatten both to
// plain strings.
type rawComposeService struct {
	Image     string        `yaml:"image"`
	Ports     []interface{} `yaml:"ports"`
	DependsOn interface{}   `yaml:"depends_on"`
}

// ParseCompose extracts one Artifact per service defined in a compose.yaml's
// top-level `services:` map, sorted by service name for deterministic output.
// path is the manifest's repo-relative path (G7). Line numbers are read
// straight from the YAML node's own position (yaml.Node.Line, 1-based) —
// real G7 evidence, not a heuristic text search. No `services:` key (or an
// empty one) -> (nil, nil): a compose file that declares nothing isn't an
// error, just an honest empty contribution.
func ParseCompose(path string, content []byte) ([]Artifact, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 {
		return nil, nil
	}
	top := root.Content[0]
	servicesNode := mappingValue(top, "services")
	if servicesNode == nil || servicesNode.Kind != yaml.MappingNode {
		return nil, nil
	}

	var out []Artifact
	for i := 0; i+1 < len(servicesNode.Content); i += 2 {
		nameNode := servicesNode.Content[i]
		svcNode := servicesNode.Content[i+1]

		var svc rawComposeService
		if err := svcNode.Decode(&svc); err != nil {
			continue // malformed service entry — skip, never fabricate (honest v1 cut)
		}
		out = append(out, Artifact{
			Name:      nameNode.Value,
			Kind:      "compose-service",
			Image:     svc.Image,
			Ports:     normalizePorts(svc.Ports),
			DependsOn: normalizeDependsOn(svc.DependsOn),
			File:      path,
			Line:      uint32(nameNode.Line),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// mappingValue returns the value node for key in a YAML mapping node, or nil
// if mapping isn't a mapping node or key isn't present.
func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// normalizePorts flattens the Compose Spec's short-form ports (string or
// bare int) to plain strings. Long-form (mapping) entries are silently
// skipped — 80/20 cut, documented on rawComposeService.
func normalizePorts(raw []interface{}) []string {
	var out []string
	for _, p := range raw {
		switch v := p.(type) {
		case string:
			out = append(out, v)
		case int:
			out = append(out, fmt.Sprintf("%d", v))
		}
	}
	return out
}

// normalizeDependsOn flattens the Compose Spec's two depends_on shapes (a
// plain list of service names, or a map keyed by service name with a
// condition value) down to a sorted list of service names.
func normalizeDependsOn(raw interface{}) []string {
	switch v := raw.(type) {
	case []interface{}:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case map[string]interface{}:
		out := make([]string, 0, len(v))
		for name := range v {
			out = append(out, name)
		}
		sort.Strings(out)
		return out
	default:
		return nil
	}
}
