package arch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderDataModel_Kind(t *testing.T) {
	shard, err := RenderDataModel(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "entity", shard.Kind)
}

func TestRenderDataModel_EmptyState_HonestNotPhantom(t *testing.T) {
	shard, err := RenderDataModel(RenderInput{})
	require.NoError(t, err)
	assert.Empty(t, shard.Nodes)
	assert.Equal(t, "0 entities", shard.Count)
}

func TestRenderDataModel_NodeShape(t *testing.T) {
	in := RenderInput{Entities: []EntityEntry{
		{Name: "User", Fields: []string{"ID", "Name"}, Tech: "Go struct", File: "user.go", Line: 5},
	}}
	shard, err := RenderDataModel(in)
	require.NoError(t, err)
	require.Len(t, shard.Nodes, 1)
	n := shard.Nodes[0]
	assert.Equal(t, "User", n.Label)
	assert.Equal(t, "entity", n.Type)
	assert.Equal(t, "Go struct", n.Tech)
	assert.Equal(t, []string{"ID", "Name"}, n.Fields)
	assert.True(t, n.Real)
	require.Len(t, n.Sources, 1)
	assert.Equal(t, "user.go", n.Sources[0].File)
	assert.Equal(t, uint32(5), n.Sources[0].Line)
}

func TestRenderDataModel_SortedByName(t *testing.T) {
	in := RenderInput{Entities: []EntityEntry{
		{Name: "Zebra", Fields: []string{"A"}},
		{Name: "Apple", Fields: []string{"A"}},
	}}
	shard, err := RenderDataModel(in)
	require.NoError(t, err)
	require.Len(t, shard.Nodes, 2)
	assert.Equal(t, "Apple", shard.Nodes[0].Label)
	assert.Equal(t, "Zebra", shard.Nodes[1].Label)
}

// TestRenderDataModel_SkipsZeroFieldEntities: the viewer's entity layout
// unconditionally reads n.fields.length (viewer.js:799) — a zero-field
// struct (Fields omitempty -> absent from JSON) would crash that read.
// Documented v1 cut: a marker struct with no fields isn't a meaningful row
// for "what are the core entities, their key fields" anyway.
func TestRenderDataModel_SkipsZeroFieldEntities(t *testing.T) {
	in := RenderInput{Entities: []EntityEntry{
		{Name: "Empty", Fields: nil},
		{Name: "User", Fields: []string{"ID"}},
	}}
	shard, err := RenderDataModel(in)
	require.NoError(t, err)
	require.Len(t, shard.Nodes, 1)
	assert.Equal(t, "User", shard.Nodes[0].Label)
}

func TestRenderDataModel_Provenance(t *testing.T) {
	shard, err := RenderDataModel(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "derived", shard.Prov.Kind)
}

func TestRenderDataModel_TruncatesWithHonestCaption(t *testing.T) {
	entries := make([]EntityEntry, 0, 40)
	for i := 0; i < 40; i++ {
		entries = append(entries, EntityEntry{Name: string(rune('a'+i%26)) + "_entity", Fields: []string{"ID"}})
	}
	shard, err := RenderDataModel(RenderInput{Entities: entries})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(shard.Nodes), dataModelNodesMax)
	assert.Contains(t, shard.Count, "40 entities")
	assert.Contains(t, shard.Count, "showing")
}
