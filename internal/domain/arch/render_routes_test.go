package arch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderAPIContract_Kind(t *testing.T) {
	shard, err := RenderAPIContract(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "table", shard.Kind)
}

func TestRenderAPIContract_EmptyState_HonestNotPhantom(t *testing.T) {
	shard, err := RenderAPIContract(RenderInput{})
	require.NoError(t, err)
	assert.Empty(t, shard.Rows)
	assert.Equal(t, "0 routes", shard.Count)
}

func TestRenderAPIContract_ColumnsAndRows(t *testing.T) {
	in := RenderInput{Routes: []RouteEntry{
		{Method: "GET", Path: "/ping", Handler: "pingHandler", Framework: "gin", File: "main.go", Line: 8},
	}}
	shard, err := RenderAPIContract(in)
	require.NoError(t, err)
	assert.Equal(t, []string{"method", "path", "handler", "framework"}, shard.Columns)
	require.Len(t, shard.Rows, 1)
	assert.Equal(t, []string{"GET", "/ping", "pingHandler", "gin"}, shard.Rows[0])
}

func TestRenderAPIContract_NoVerbShownAsDash(t *testing.T) {
	in := RenderInput{Routes: []RouteEntry{
		{Method: "", Path: "/static/", Handler: "fileServer", Framework: "net/http"},
	}}
	shard, err := RenderAPIContract(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 1)
	assert.Equal(t, "—", shard.Rows[0][0])
}

func TestRenderAPIContract_SortedByPathThenMethod(t *testing.T) {
	in := RenderInput{Routes: []RouteEntry{
		{Method: "POST", Path: "/users"},
		{Method: "GET", Path: "/ping"},
		{Method: "GET", Path: "/users"},
	}}
	shard, err := RenderAPIContract(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 3)
	assert.Equal(t, "/ping", shard.Rows[0][1])
	assert.Equal(t, "/users", shard.Rows[1][1])
	assert.Equal(t, "GET", shard.Rows[1][0])
	assert.Equal(t, "/users", shard.Rows[2][1])
	assert.Equal(t, "POST", shard.Rows[2][0])
}

func TestRenderAPIContract_Provenance(t *testing.T) {
	shard, err := RenderAPIContract(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "derived", shard.Prov.Kind)
}

func TestRenderAPIContract_TruncatesWithHonestCaption(t *testing.T) {
	entries := make([]RouteEntry, 0, 40)
	for i := 0; i < 40; i++ {
		entries = append(entries, RouteEntry{Method: "GET", Path: string(rune('a'+i%26)) + "_route"})
	}
	shard, err := RenderAPIContract(RenderInput{Routes: entries})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(shard.Rows), apiContractRowsMax)
	assert.Contains(t, shard.Count, "40 routes")
	assert.Contains(t, shard.Count, "showing")
}
