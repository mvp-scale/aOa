// Package deployfile parses deployment manifests (Dockerfile, compose.yaml,
// Kubernetes workload/service manifests) into a small, format-neutral
// Artifact record. COL-2 (board M6): the reader half of "deploy-file parser
// -> deployment facts -> Deployment view".
//
// Deliberately line-grain/stdlib-first for Dockerfile (no dockerfile-parser
// dependency) and gopkg.in/yaml.v3-based for compose.yaml/Kubernetes
// manifests (already a direct module dependency, see
// internal/domain/analyzer/rules.go) — a well-known subset of each format
// (FROM/EXPOSE; services -> image/ports/depends_on; kind/metadata.name/
// spec.template.spec.containers[0].image), not full spec compliance — 80/20
// per the owner ruling (no over-engineering).
//
// Readers are pure: (path, content) -> (Artifact(s), error). No file I/O, no
// FactStore writes happen in this package — the app layer decides whether/how
// to read the filesystem and persist (see internal/app/vl7.go for the derive-
// time call site). This mirrors internal/adapters/lockfile's boundary: keeps
// readers trivially unit-testable and avoids feeding deploy-manifest specs
// into the code-import FactDep stream the compactor (internal/domain/facts)
// already resolves into the unit dependency graph — a different, non-code
// dependency concept that must not corrupt that graph (phantom-node law, see
// internal/app/vl1.go's package doc for the full rationale this mirrors).
package deployfile

// Artifact is one detected deploy unit from a manifest file: a Dockerfile's
// base image, one compose.yaml service, or one Kubernetes workload/service
// manifest document. Mirrors arch.DeploymentEntry field-for-field; the app
// layer converts between the two at the boundary (D25 pattern — adapt at the
// boundary rather than importing domain types into an adapter).
type Artifact struct {
	Name      string   // service/resource/image name
	Kind      string   // "dockerfile" | "compose-service" | "k8s-deployment" | "k8s-statefulset" | "k8s-daemonset" | "k8s-service" | "k8s-cronjob"
	Image     string   // container image reference, when declared
	Ports     []string // exported/published ports, when declared
	DependsOn []string // same-manifest dependency names (compose depends_on only, v1)
	File      string   // manifest file path, as passed to the reader (repo-relative)
	Line      uint32   // 1-based line in File where this artifact was declared (G7)
}
