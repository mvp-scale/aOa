package arch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderChangeMap_Kind(t *testing.T) {
	shard, err := RenderChangeMap(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "table", shard.Kind)
}

func TestRenderChangeMap_EmptyState_HonestNotPhantom(t *testing.T) {
	shard, err := RenderChangeMap(RenderInput{})
	require.NoError(t, err)
	assert.Empty(t, shard.Rows)
	assert.Equal(t, "0 units changed", shard.Count)
}

func TestRenderChangeMap_ColumnsAndRows(t *testing.T) {
	in := RenderInput{ChurnEntries: []ChurnEntry{
		{Path: "internal/domain/arch", ChangedFiles: 3, Commits: 5, Complexity: 10, Risk: 30, File: "internal/domain/arch/model.go", Line: 1},
	}}
	shard, err := RenderChangeMap(in)
	require.NoError(t, err)
	assert.Equal(t, []string{"unit", "changed files", "commits", "complexity", "risk"}, shard.Columns)
	require.Len(t, shard.Rows, 1)
	assert.Equal(t, []string{"internal/domain/arch", "3", "5", "10", "30"}, shard.Rows[0])
}

func TestRenderChangeMap_SortedByRiskDescending(t *testing.T) {
	in := RenderInput{ChurnEntries: []ChurnEntry{
		{Path: "low/risk", ChangedFiles: 1, Complexity: 1, Risk: 1},
		{Path: "high/risk", ChangedFiles: 10, Complexity: 10, Risk: 100},
	}}
	shard, err := RenderChangeMap(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 2)
	assert.Equal(t, "high/risk", shard.Rows[0][0])
	assert.Equal(t, "low/risk", shard.Rows[1][0])
}

func TestRenderChangeMap_TieBreaksByPath(t *testing.T) {
	in := RenderInput{ChurnEntries: []ChurnEntry{
		{Path: "zeta", Risk: 5},
		{Path: "alpha", Risk: 5},
	}}
	shard, err := RenderChangeMap(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 2)
	assert.Equal(t, "alpha", shard.Rows[0][0])
	assert.Equal(t, "zeta", shard.Rows[1][0])
}

func TestRenderChangeMap_Provenance(t *testing.T) {
	shard, err := RenderChangeMap(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "mixed", shard.Prov.Kind)
}

func TestRenderChangeMap_TruncatesWithHonestCaption(t *testing.T) {
	entries := make([]ChurnEntry, 0, 40)
	for i := 0; i < 40; i++ {
		entries = append(entries, ChurnEntry{Path: string(rune('a'+i%26)) + "_unit", Risk: i})
	}
	shard, err := RenderChangeMap(RenderInput{ChurnEntries: entries})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(shard.Rows), changeRowsMax)
	assert.Contains(t, shard.Count, "40 units changed")
	assert.Contains(t, shard.Count, "showing")
}
