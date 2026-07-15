package arch

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSBOM_Kind(t *testing.T) {
	shard, err := RenderSBOM(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "table", shard.Kind)
}

func TestRenderSBOM_EmptyState_HonestNotPhantom(t *testing.T) {
	shard, err := RenderSBOM(RenderInput{})
	require.NoError(t, err)
	assert.Empty(t, shard.Rows)
	assert.Equal(t, "0 components", shard.Count)
}

func TestRenderSBOM_ColumnsAndRows(t *testing.T) {
	in := RenderInput{Components: []Component{
		{Name: "github.com/spf13/cobra", Version: "v1.10.2", Supplier: "direct", Language: "go", File: "go.mod", Line: 5},
	}}
	shard, err := RenderSBOM(in)
	require.NoError(t, err)
	assert.Equal(t, []string{"component", "version", "supplier", "language", "unpinned"}, shard.Columns)
	require.Len(t, shard.Rows, 1)
	assert.Equal(t, []string{"github.com/spf13/cobra", "v1.10.2", "direct", "go", ""}, shard.Rows[0])
}

func TestRenderSBOM_UnpinnedFlaggedHonestly(t *testing.T) {
	in := RenderInput{Components: []Component{
		{Name: "pinned/pkg", Version: "v1.0.0", Supplier: "direct", Language: "go"},
		{Name: "local/pkg", Version: "", Supplier: "replace", Language: "go", Unpinned: true},
	}}
	shard, err := RenderSBOM(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 2)

	var flagged, clean [][]string
	for _, row := range shard.Rows {
		if strings.HasPrefix(row[0], "⚠ ") {
			flagged = append(flagged, row)
		} else {
			clean = append(clean, row)
		}
	}
	require.Len(t, flagged, 1)
	require.Len(t, clean, 1)
	assert.Equal(t, "⚠ local/pkg", flagged[0][0])
	assert.Equal(t, "true", flagged[0][4])
	assert.Equal(t, "2 components (1 unpinned)", shard.Count)
}

func TestRenderSBOM_Deterministic_SortedByName(t *testing.T) {
	in := RenderInput{Components: []Component{
		{Name: "zeta", Version: "v1", Language: "go"},
		{Name: "alpha", Version: "v1", Language: "go"},
	}}
	shard, err := RenderSBOM(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 2)
	assert.Equal(t, "alpha", shard.Rows[0][0])
	assert.Equal(t, "zeta", shard.Rows[1][0])
}

func TestRenderSBOM_Provenance(t *testing.T) {
	shard, err := RenderSBOM(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "mixed", shard.Prov.Kind)
}

func TestRenderSBOM_TruncatesWithHonestCaption(t *testing.T) {
	comps := make([]Component, 0, 40)
	for i := 0; i < 40; i++ {
		comps = append(comps, Component{Name: string(rune('a'+i%26)) + "_pkg", Version: "v1", Language: "go"})
	}
	shard, err := RenderSBOM(RenderInput{Components: comps})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(shard.Rows), sbomRowsMax)
	assert.Contains(t, shard.Count, "40 components")
	assert.Contains(t, shard.Count, "showing")
}
