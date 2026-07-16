// COL-2 (deployment-collector & deployment view): app-layer wiring for the
// Deployment (view id "deployment") view — deploy-surface facts extracted
// from a Dockerfile, compose.yaml, and Kubernetes manifests.
//
// Deliberate deviation from the work-order's suggested "bounded discovery:
// well-known roots (deploy/k8s/charts) + one level" for Kubernetes manifest
// discovery: rather than re-implementing a second bounded filesystem walk
// parallel to the main indexer's (D9 — walk.go stays untouched, and a second
// walk would be a second place D9's skip-dir/extension rules could drift),
// this reader reuses idx.Files — already walked, already bounded, and
// already carries Language=="yaml" for every .yaml/.yml file the project has
// (both the light default-extension set and the treesitter parser index
// .yaml/.yml — see internal/app/indexer.go:72 and
// internal/adapters/treesitter/extensions.go:84/197). Candidate manifests
// are filtered by filename convention (the compose.yaml family) or by
// content shape (a recognized Kubernetes `kind:`, honestly skipped when
// absent/unrecognized — deployfile.k8sKind's allowlist) rather than by
// directory location, so a manifest living outside deploy/k8s/charts is not
// silently missed, and a stray CI-workflow yaml is not silently mistaken for
// one (no `kind:`/`services:` shape present, no match). Recorded here per
// the vl3.go/vl6.go precedent of noting deliberate deviations rather than
// silently diverging from a work order's stated implementation route.
//
// Dockerfile (D30: extensionless, never in idx.Files — the main walk skips
// zero-extension files, internal/domain/index/walk.go) is read directly at
// the project root only (repo-root grain, mirrors vl1.go's go.mod/
// package.json scope — 80/20, no recursive Dockerfile discovery in v1).
package app

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/corey/aoa/internal/adapters/deployfile"
	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
)

// deployFileMax bounds COL-2's derive-time re-read of yaml manifests found in
// idx.Files (MEASURED guard, same class as entityFileMax/routeFileMax): keeps
// one derive from re-reading an unbounded number of yaml files on a very
// large repo. 2000 comfortably covers a real monorepo's compose/k8s manifest
// count (v1 80/20 scope, documented rather than silently absent).
const deployFileMax = 2000

// composeFileNames is the Compose Spec's conventional manifest names, matched
// by base filename only (any directory) — the reader's way of telling a
// compose manifest apart from an arbitrary Kubernetes/other yaml file without
// guessing from directory location.
var composeFileNames = map[string]bool{
	"compose.yaml":        true,
	"compose.yml":         true,
	"docker-compose.yaml": true,
	"docker-compose.yml":  true,
}

// buildDeploymentEntries assembles COL-2's Deployment view rows for one
// derive pass: the project root's Dockerfile (image + exposed ports), every
// compose.yaml-family manifest known to idx.Files (one row per service), and
// every other .yaml/.yml file known to idx.Files whose content resolves to a
// recognized Kubernetes workload/service kind (one row per manifest
// document). root is the project root; idx may be nil (mirrors
// buildRefHits/buildEntityEntries's contract) — the Dockerfile check still
// runs since it needs no index.
func buildDeploymentEntries(root string, idx *ports.Index) []arch.DeploymentEntry {
	var out []arch.DeploymentEntry
	if e, ok := readDockerfile(root); ok {
		out = append(out, e)
	}

	if idx != nil {
		var composeFiles, k8sFiles []string
		for _, fm := range idx.Files {
			if fm == nil || fm.Language != "yaml" {
				continue
			}
			if composeFileNames[filepath.Base(fm.Path)] {
				composeFiles = append(composeFiles, fm.Path)
			} else {
				k8sFiles = append(k8sFiles, fm.Path)
			}
		}
		sort.Strings(composeFiles)
		sort.Strings(k8sFiles)
		if len(composeFiles) > deployFileMax {
			composeFiles = composeFiles[:deployFileMax]
		}
		if len(k8sFiles) > deployFileMax {
			k8sFiles = k8sFiles[:deployFileMax]
		}

		for _, relPath := range composeFiles {
			source, err := os.ReadFile(filepath.Join(root, relPath))
			if err != nil {
				continue
			}
			artifacts, err := deployfile.ParseCompose(relPath, source)
			if err != nil || len(artifacts) == 0 {
				continue
			}
			out = append(out, convertArtifacts(artifacts)...)
		}

		for _, relPath := range k8sFiles {
			source, err := os.ReadFile(filepath.Join(root, relPath))
			if err != nil {
				continue
			}
			artifacts, err := deployfile.ParseK8sManifest(relPath, source)
			if err != nil || len(artifacts) == 0 {
				continue
			}
			out = append(out, convertArtifacts(artifacts)...)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// readDockerfile reads root's top-level Dockerfile (D30: extensionless,
// never in idx.Files) and extracts a single arch.DeploymentEntry. A missing
// file or a Dockerfile with no FROM line -> (zero value, false) — honest
// absence, not an error.
func readDockerfile(root string) (arch.DeploymentEntry, bool) {
	source, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		return arch.DeploymentEntry{}, false
	}
	a, ok := deployfile.ParseDockerfile("Dockerfile", source)
	if !ok {
		return arch.DeploymentEntry{}, false
	}
	return convertArtifact(a), true
}

// convertArtifacts adapts deployfile.Artifact (I/O-layer shape) to
// arch.DeploymentEntry (dependency-free domain shape) at the boundary — D25
// pattern, same as convertComponents (vl1.go).
func convertArtifacts(in []deployfile.Artifact) []arch.DeploymentEntry {
	out := make([]arch.DeploymentEntry, len(in))
	for i, a := range in {
		out[i] = convertArtifact(a)
	}
	return out
}

func convertArtifact(a deployfile.Artifact) arch.DeploymentEntry {
	return arch.DeploymentEntry{
		ID:        a.Name,
		Kind:      a.Kind,
		Image:     a.Image,
		Ports:     a.Ports,
		DependsOn: a.DependsOn,
		File:      a.File,
		Line:      a.Line,
	}
}
