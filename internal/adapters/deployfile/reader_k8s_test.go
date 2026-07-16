package deployfile

import "testing"

const k8sFixture = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: web
          image: myco/web:1.4
---
apiVersion: v1
kind: Service
metadata:
  name: web-svc
spec:
  selector:
    app: web
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nightly
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: nightly
              image: myco/batch:2.0
`

func TestParseK8sManifest_MultiDocument(t *testing.T) {
	artifacts, err := ParseK8sManifest("k8s.yaml", []byte(k8sFixture))
	if err != nil {
		t.Fatalf("ParseK8sManifest: %v", err)
	}
	if len(artifacts) != 3 {
		t.Fatalf("len(artifacts) = %d, want 3", len(artifacts))
	}

	byName := make(map[string]Artifact, len(artifacts))
	for _, a := range artifacts {
		byName[a.Name] = a
	}

	dep, ok := byName["web"]
	if !ok {
		t.Fatal("missing Deployment/web artifact")
	}
	if dep.Kind != "k8s-deployment" {
		t.Errorf("web.Kind = %q, want %q", dep.Kind, "k8s-deployment")
	}
	if dep.Image != "myco/web:1.4" {
		t.Errorf("web.Image = %q, want %q", dep.Image, "myco/web:1.4")
	}
	if dep.Line == 0 {
		t.Error("web.Line = 0, want a real (>0) G7 line pointer")
	}

	svc, ok := byName["web-svc"]
	if !ok {
		t.Fatal("missing Service/web-svc artifact")
	}
	if svc.Kind != "k8s-service" {
		t.Errorf("web-svc.Kind = %q, want %q", svc.Kind, "k8s-service")
	}

	cron, ok := byName["nightly"]
	if !ok {
		t.Fatal("missing CronJob/nightly artifact")
	}
	if cron.Kind != "k8s-cronjob" {
		t.Errorf("nightly.Kind = %q, want %q", cron.Kind, "k8s-cronjob")
	}
}

func TestParseK8sManifest_UnrecognizedKind_SkippedNotFabricated(t *testing.T) {
	content := []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: settings
data:
  key: value
`)
	artifacts, err := ParseK8sManifest("configmap.yaml", content)
	if err != nil {
		t.Fatalf("ParseK8sManifest: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("artifacts = %v, want empty (unrecognized kind must not be fabricated)", artifacts)
	}
}

func TestParseK8sManifest_NotAK8sManifest_ReturnsEmptyNotError(t *testing.T) {
	// A plain, non-k8s-shaped YAML file (e.g. a CI workflow) has no `kind`/
	// `metadata.name` at all — must be silently skipped, not an error and
	// not a phantom row.
	artifacts, err := ParseK8sManifest("ci.yaml", []byte("on: push\njobs: {}\n"))
	if err != nil {
		t.Fatalf("ParseK8sManifest: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("artifacts = %v, want empty", artifacts)
	}
}

func TestParseK8sManifest_MalformedYAML_ReturnsError(t *testing.T) {
	_, err := ParseK8sManifest("bad.yaml", []byte("kind: [this is not valid\n"))
	if err == nil {
		t.Fatal("expected an error for malformed YAML")
	}
}
