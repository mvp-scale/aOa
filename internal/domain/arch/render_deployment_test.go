package arch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderDeployment_Kind(t *testing.T) {
	shard, err := RenderDeployment(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "buckets", shard.Kind)
}

func TestRenderDeployment_EmptyState_HonestNotPhantom(t *testing.T) {
	shard, err := RenderDeployment(RenderInput{})
	require.NoError(t, err)
	assert.Empty(t, shard.Buckets)
	assert.Equal(t, "0 deploy artifacts", shard.Count)
}

func TestRenderDeployment_BucketsBySurface(t *testing.T) {
	in := RenderInput{Deployments: []DeploymentEntry{
		{ID: "Dockerfile", Kind: "dockerfile", Image: "alpine:3.19", File: "Dockerfile", Line: 1},
		{ID: "web", Kind: "compose-service", Image: "myco/web:1.0", DependsOn: []string{"db"}, File: "compose.yaml", Line: 2},
		{ID: "db", Kind: "compose-service", Image: "postgres:16", File: "compose.yaml", Line: 6},
		{ID: "web", Kind: "k8s-deployment", Image: "myco/web:1.0", File: "deploy.yaml", Line: 1},
	}}
	shard, err := RenderDeployment(in)
	require.NoError(t, err)
	require.Len(t, shard.Buckets, 2)

	var container, k8s *Bucket
	for i := range shard.Buckets {
		switch shard.Buckets[i].ID {
		case "b_container":
			container = &shard.Buckets[i]
		case "b_kubernetes":
			k8s = &shard.Buckets[i]
		}
	}
	require.NotNil(t, container, "expected a container/docker bucket")
	require.NotNil(t, k8s, "expected a kubernetes bucket")
	assert.Len(t, container.Members, 3) // Dockerfile + web + db
	assert.Len(t, k8s.Members, 1)       // web (k8s-deployment)
}

func TestRenderDeployment_MemberDependsOnSurfacedAsSub(t *testing.T) {
	in := RenderInput{Deployments: []DeploymentEntry{
		{ID: "web", Kind: "compose-service", DependsOn: []string{"db", "cache"}, File: "compose.yaml", Line: 2},
		{ID: "db", Kind: "compose-service", File: "compose.yaml", Line: 6},
	}}
	shard, err := RenderDeployment(in)
	require.NoError(t, err)
	require.Len(t, shard.Buckets, 1)
	var web Member
	for _, m := range shard.Buckets[0].Members {
		if m.ID == "web" {
			web = m
		}
	}
	assert.Contains(t, web.Sub, "db")
	assert.Contains(t, web.Sub, "cache")
}

func TestRenderDeployment_NoInventedCrossEnvEdges(t *testing.T) {
	in := RenderInput{Deployments: []DeploymentEntry{
		{ID: "web", Kind: "compose-service", File: "compose.yaml", Line: 2},
		{ID: "web", Kind: "k8s-deployment", File: "deploy.yaml", Line: 1},
	}}
	shard, err := RenderDeployment(in)
	require.NoError(t, err)
	assert.Empty(t, shard.Edges, "v1 must never invent a cross-surface correlation edge")
}

func TestRenderDeployment_Provenance(t *testing.T) {
	shard, err := RenderDeployment(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "derived", shard.Prov.Kind)
}

func TestRenderDeployment_CaptionStatesWhatShips(t *testing.T) {
	in := RenderInput{Deployments: []DeploymentEntry{
		{ID: "web", Kind: "compose-service", File: "compose.yaml", Line: 2},
	}}
	shard, err := RenderDeployment(in)
	require.NoError(t, err)
	assert.Contains(t, shard.Count, "ship")
}
