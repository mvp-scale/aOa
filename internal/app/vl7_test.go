package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDeploymentEntries_Dockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "Dockerfile"),
		[]byte("FROM alpine:3.19\nEXPOSE 8080\n"),
		0644,
	))

	idx, _, err := BuildIndex(tmpDir, nil)
	require.NoError(t, err)

	entries := buildDeploymentEntries(tmpDir, idx)
	require.Len(t, entries, 1)
	assert.Equal(t, "dockerfile", entries[0].Kind)
	assert.Equal(t, "alpine:3.19", entries[0].Image)
	assert.Equal(t, "Dockerfile", entries[0].File)
}

func TestBuildDeploymentEntries_Compose(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "compose.yaml"),
		[]byte("services:\n  web:\n    image: myco/web:1.0\n  db:\n    image: postgres:16\n"),
		0644,
	))

	idx, _, err := BuildIndex(tmpDir, nil)
	require.NoError(t, err)

	entries := buildDeploymentEntries(tmpDir, idx)
	require.Len(t, entries, 2)
	assert.Equal(t, "compose-service", entries[0].Kind)
}

func TestBuildDeploymentEntries_K8sManifest(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "deploy.yaml"),
		[]byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\nspec:\n  template:\n    spec:\n      containers:\n        - image: myco/web:1.0\n"),
		0644,
	))

	idx, _, err := BuildIndex(tmpDir, nil)
	require.NoError(t, err)

	entries := buildDeploymentEntries(tmpDir, idx)
	require.Len(t, entries, 1)
	assert.Equal(t, "k8s-deployment", entries[0].Kind)
	assert.Equal(t, "web", entries[0].ID)
}

func TestBuildDeploymentEntries_NilIndex_DockerfileOnlyStillWorks(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "Dockerfile"),
		[]byte("FROM alpine:3.19\n"),
		0644,
	))
	entries := buildDeploymentEntries(tmpDir, nil)
	require.Len(t, entries, 1)
	assert.Equal(t, "dockerfile", entries[0].Kind)
}

func TestBuildDeploymentEntries_NoDeployFiles_ReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n"), 0644))

	idx, _, err := BuildIndex(tmpDir, nil)
	require.NoError(t, err)

	entries := buildDeploymentEntries(tmpDir, idx)
	assert.Nil(t, entries)
}

// TestBuildDeploymentEntries_ComposeFixtureRepo exercises the committed
// compose-fixture golden (test/fixtures/col2/compose-fixture): a Dockerfile
// alongside a compose.yaml with 3 services (one declaring depends_on) —
// verifies the Dockerfile + compose readers merge correctly at the app-layer
// boundary on a real (if small) repo, not just synthetic TempDir content.
func TestBuildDeploymentEntries_ComposeFixtureRepo(t *testing.T) {
	root := filepath.Join("..", "..", "test", "fixtures", "col2", "compose-fixture")
	idx, _, err := BuildIndex(root, nil)
	require.NoError(t, err)

	entries := buildDeploymentEntries(root, idx)
	require.Len(t, entries, 4) // Dockerfile + api + cache + db

	byID := make(map[string]int, len(entries))
	for i, e := range entries {
		byID[e.ID] = i
	}

	dockerfile := entries[byID["Dockerfile"]]
	assert.Equal(t, "dockerfile", dockerfile.Kind)
	assert.Equal(t, "alpine:3.19", dockerfile.Image) // final FROM stage ships

	api := entries[byID["api"]]
	assert.Equal(t, "compose-service", api.Kind)
	assert.Equal(t, "fixture/api:1.0", api.Image)
	assert.ElementsMatch(t, []string{"db", "cache"}, api.DependsOn)

	db := entries[byID["db"]]
	assert.Equal(t, "postgres:16", db.Image)
}

// TestBuildDeploymentEntries_K8sFixtureRepo exercises the committed
// k8s-fixture golden (test/fixtures/col2/k8s-fixture): a Dockerfile alongside
// a multi-document Kubernetes manifest (Deployment + Service).
func TestBuildDeploymentEntries_K8sFixtureRepo(t *testing.T) {
	root := filepath.Join("..", "..", "test", "fixtures", "col2", "k8s-fixture")
	idx, _, err := BuildIndex(root, nil)
	require.NoError(t, err)

	entries := buildDeploymentEntries(root, idx)
	require.Len(t, entries, 3) // Dockerfile + Deployment + Service

	byID := make(map[string]int, len(entries))
	for i, e := range entries {
		byID[e.ID] = i
	}

	dep := entries[byID["fixture-web"]]
	assert.Equal(t, "k8s-deployment", dep.Kind)
	assert.Equal(t, "fixture/web:1.0", dep.Image)

	svc := entries[byID["fixture-web-svc"]]
	assert.Equal(t, "k8s-service", svc.Kind)
}

func TestBuildDeploymentEntries_SortedByKindThenID(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte("FROM alpine:3.19\n"), 0644))
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "compose.yaml"),
		[]byte("services:\n  web:\n    image: myco/web:1.0\n"),
		0644,
	))

	idx, _, err := BuildIndex(tmpDir, nil)
	require.NoError(t, err)

	entries := buildDeploymentEntries(tmpDir, idx)
	require.Len(t, entries, 2)
	assert.Equal(t, "compose-service", entries[0].Kind) // "compose-service" < "dockerfile"
	assert.Equal(t, "dockerfile", entries[1].Kind)
}
