package deployfile

import (
	"bytes"
	"io"

	"gopkg.in/yaml.v3"
)

// rawK8sManifest mirrors the subset of the Kubernetes manifest schema this
// reader cares about: kind/metadata.name (every resource has these) plus the
// pod-template container image, read from whichever of the two shapes a
// workload kind uses (Deployment/StatefulSet/DaemonSet nest it under
// spec.template.spec.containers; CronJob nests an extra spec.jobTemplate
// level around the same pod-template shape). A bare Service has no pod
// template at all — Image simply stays empty, which is correct (a Service
// doesn't run a container).
type rawK8sManifest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Template    podTemplateSpec `yaml:"template"`
		JobTemplate struct {
			Spec struct {
				Template podTemplateSpec `yaml:"template"`
			} `yaml:"spec"`
		} `yaml:"jobTemplate"`
	} `yaml:"spec"`
}

type podTemplateSpec struct {
	Spec struct {
		Containers []struct {
			Image string `yaml:"image"`
		} `yaml:"containers"`
	} `yaml:"spec"`
}

// k8sKind maps a manifest's `kind:` to this view's Artifact.Kind, or "" for
// a kind this v1 reader doesn't recognize (honestly skipped by the caller,
// never guessed — 80/20: the common workload/service kinds, not full API
// coverage).
func k8sKind(raw string) string {
	switch raw {
	case "Deployment":
		return "k8s-deployment"
	case "StatefulSet":
		return "k8s-statefulset"
	case "DaemonSet":
		return "k8s-daemonset"
	case "Service":
		return "k8s-service"
	case "CronJob":
		return "k8s-cronjob"
	default:
		return ""
	}
}

// ParseK8sManifest extracts one Artifact per recognized Kubernetes resource
// document in content. Manifests are commonly multi-document ("---"-
// separated) — every document is decoded independently via yaml.Decoder.
// path is the manifest's repo-relative path (G7). Line numbers are read
// straight from each document's own root node position (yaml.Node.Line,
// 1-based) — real G7 evidence.
//
// A document missing `kind`/`metadata.name`, or carrying an unrecognized
// kind (k8sKind returns ""), is silently skipped — not every YAML document in
// a repo is a Kubernetes manifest (e.g. a CI workflow), and guessing would be
// fabrication, not derivation.
func ParseK8sManifest(path string, content []byte) ([]Artifact, error) {
	dec := yaml.NewDecoder(bytes.NewReader(content))
	var out []Artifact
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, err
		}
		if len(doc.Content) == 0 {
			continue
		}
		top := doc.Content[0]

		var m rawK8sManifest
		if err := top.Decode(&m); err != nil {
			continue // malformed document — skip, never fabricate
		}
		kind := k8sKind(m.Kind)
		if kind == "" || m.Metadata.Name == "" {
			continue
		}

		image := firstContainerImage(m.Spec.Template)
		if image == "" {
			image = firstContainerImage(m.Spec.JobTemplate.Spec.Template)
		}

		out = append(out, Artifact{
			Name:  m.Metadata.Name,
			Kind:  kind,
			Image: image,
			File:  path,
			Line:  uint32(top.Line),
		})
	}
	return out, nil
}

func firstContainerImage(t podTemplateSpec) string {
	if len(t.Spec.Containers) == 0 {
		return ""
	}
	return t.Spec.Containers[0].Image
}
