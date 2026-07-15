package arch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderTechStack_Kind(t *testing.T) {
	shard, err := RenderTechStack(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "table", shard.Kind)
}

func TestRenderTechStack_EmptyState(t *testing.T) {
	shard, err := RenderTechStack(RenderInput{})
	require.NoError(t, err)
	assert.Empty(t, shard.Rows)
	assert.Equal(t, "0 technologies", shard.Count)
}

func TestRenderTechStack_ColumnsAndRows(t *testing.T) {
	in := RenderInput{Technologies: []TechEntry{
		{Name: "go", Kind: "language", Count: 42, File: "internal/app/app.go", Line: 1},
	}}
	shard, err := RenderTechStack(in)
	require.NoError(t, err)
	assert.Equal(t, []string{"technology", "kind", "count", "where used"}, shard.Columns)
	require.Len(t, shard.Rows, 1)
	assert.Equal(t, []string{"go", "language", "42", "internal/app/app.go"}, shard.Rows[0])
}

func TestRenderTechStack_UnpinnedDependencyFlagged(t *testing.T) {
	in := RenderInput{Technologies: []TechEntry{
		{Name: "some/fork", Kind: "dependency", Count: 1, Unpinned: true, File: "go.mod"},
	}}
	shard, err := RenderTechStack(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 1)
	assert.Equal(t, "⚠ some/fork", shard.Rows[0][0])
}

func TestRenderTechStack_SortedByCountDescThenName(t *testing.T) {
	in := RenderInput{Technologies: []TechEntry{
		{Name: "js", Kind: "language", Count: 3},
		{Name: "go", Kind: "language", Count: 42},
	}}
	shard, err := RenderTechStack(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 2)
	assert.Equal(t, "go", shard.Rows[0][0])
	assert.Equal(t, "js", shard.Rows[1][0])
}

func TestRenderTechStack_Provenance(t *testing.T) {
	shard, err := RenderTechStack(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "mixed", shard.Prov.Kind)
}

func TestRenderTechStack_TruncatesWithHonestCaption(t *testing.T) {
	techs := make([]TechEntry, 0, 40)
	for i := 0; i < 40; i++ {
		techs = append(techs, TechEntry{Name: string(rune('a' + i%26)), Kind: "language", Count: i + 1})
	}
	shard, err := RenderTechStack(RenderInput{Technologies: techs})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(shard.Rows), techStackRowsMax)
	assert.Contains(t, shard.Count, "40 technologies")
	assert.Contains(t, shard.Count, "showing")
}
