package deployfile

import "strings"

// ParseDockerfile extracts a single Artifact summarizing a Dockerfile's final
// base image and exposed ports (line-grain, stdlib text parsing only — D30:
// Dockerfile is extensionless and the main index walk never sees it, so this
// reader is the only path to that content; see internal/app/vl7.go's package
// doc for the full rationale).
//
// A multi-stage Dockerfile declares more than one FROM line ("FROM ... AS
// build", then a final "FROM ..."); the LAST FROM line is the image that
// actually ships (earlier stages are build-time only), so it wins over the
// first. EXPOSE ports are collected from every stage (a later stage's EXPOSE
// is still a real, declared port). No content -> (Artifact{}, false): a
// Dockerfile with no FROM line at all isn't a meaningful row for this view's
// question (honest v1 cut, mirrors RenderDataModel's zero-field-struct skip).
func ParseDockerfile(path string, content []byte) (Artifact, bool) {
	var image string
	var imageLine uint32
	var ports []string

	lines := strings.Split(string(content), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "FROM "):
			rest := strings.TrimSpace(line[len("FROM "):])
			// Strip a trailing "AS <stage>" alias — keep the base image ref only.
			if fields := strings.Fields(rest); len(fields) > 0 {
				image = fields[0]
			}
			imageLine = uint32(i + 1)
		case strings.HasPrefix(upper, "EXPOSE "):
			ports = append(ports, strings.Fields(line[len("EXPOSE "):])...)
		}
	}

	if image == "" {
		return Artifact{}, false
	}
	return Artifact{
		Name:  path,
		Kind:  "dockerfile",
		Image: image,
		Ports: ports,
		File:  path,
		Line:  imageLine,
	}, true
}
