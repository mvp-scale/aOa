package arch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderGlossary_Kind(t *testing.T) {
	shard, err := RenderGlossary(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "table", shard.Kind)
}

func TestRenderGlossary_EmptyState(t *testing.T) {
	shard, err := RenderGlossary(RenderInput{})
	require.NoError(t, err)
	assert.Empty(t, shard.Rows)
	assert.Equal(t, "0 terms", shard.Count)
}

func TestRenderGlossary_ColumnsAndRows(t *testing.T) {
	in := RenderInput{GlossaryTerms: []GlossaryEntry{
		{Term: "eviction", Domain: "scheduling", Definition: "drain, evict, taint"},
	}}
	shard, err := RenderGlossary(in)
	require.NoError(t, err)
	assert.Equal(t, []string{"term", "definition", "owning domain"}, shard.Columns)
	require.Len(t, shard.Rows, 1)
	assert.Equal(t, []string{"eviction", "drain, evict, taint", "scheduling"}, shard.Rows[0])
}

func TestRenderGlossary_SortedAlphabeticallyByTerm(t *testing.T) {
	in := RenderInput{GlossaryTerms: []GlossaryEntry{
		{Term: "zeta", Domain: "d"},
		{Term: "alpha", Domain: "d"},
	}}
	shard, err := RenderGlossary(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 2)
	assert.Equal(t, "alpha", shard.Rows[0][0])
	assert.Equal(t, "zeta", shard.Rows[1][0])
}

func TestRenderGlossary_Provenance_MixedCandidatesNotRatified(t *testing.T) {
	shard, err := RenderGlossary(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "mixed", shard.Prov.Kind)
	assert.Contains(t, shard.Prov.Label, "not ratified")
}

func TestRenderGlossary_TruncatesWithHonestCaption(t *testing.T) {
	terms := make([]GlossaryEntry, 0, 40)
	for i := 0; i < 40; i++ {
		terms = append(terms, GlossaryEntry{Term: string(rune('a'+i%26)) + "term", Domain: "d"})
	}
	shard, err := RenderGlossary(RenderInput{GlossaryTerms: terms})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(shard.Rows), glossaryRowsMax)
	assert.Contains(t, shard.Count, "40 terms")
	assert.Contains(t, shard.Count, "showing")
}
