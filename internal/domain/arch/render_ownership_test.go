package arch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderOwnership_Kind(t *testing.T) {
	shard, err := RenderOwnership(RenderInput{})
	require.NoError(t, err)
	assert.Equal(t, "table", shard.Kind)
}

func TestRenderOwnership_EmptyState_HonestNotPhantom(t *testing.T) {
	shard, err := RenderOwnership(RenderInput{})
	require.NoError(t, err)
	assert.Empty(t, shard.Rows)
	assert.Equal(t, "0 units with defined owners", shard.Count)
}

func TestRenderOwnership_Rows_Populated(t *testing.T) {
	in := RenderInput{OwnershipEntries: []OwnershipEntry{
		{Path: "internal/app", Owners: []string{"@alice"}, Provenance: "declared", File: "CODEOWNERS", Line: 3},
		{Path: "internal/domain/arch", Owners: []string{"bob"}, Provenance: "derived", File: "internal/domain/arch/model.go", Line: 1},
	}}
	shard, err := RenderOwnership(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 2)
	assert.Equal(t, []string{"internal/app", "@alice", "CODEOWNERS"}, shard.Rows[0])
	assert.Equal(t, []string{"internal/domain/arch", "bob", "git shortlog"}, shard.Rows[1])
	assert.Equal(t, []string{"Path", "Owner(s)", "Source"}, shard.Columns)
}

func TestRenderOwnership_Provenance_MixedWhenBothSourcesPresent(t *testing.T) {
	in := RenderInput{OwnershipEntries: []OwnershipEntry{
		{Path: "internal/app", Owners: []string{"@alice"}, Provenance: "declared"},
		{Path: "internal/domain/arch", Owners: []string{"bob"}, Provenance: "derived"},
	}}
	shard, err := RenderOwnership(in)
	require.NoError(t, err)
	assert.Equal(t, "mixed", shard.Prov.Kind)
}

func TestRenderOwnership_Provenance_DerivedWhenOnlyCodeowners(t *testing.T) {
	in := RenderInput{OwnershipEntries: []OwnershipEntry{
		{Path: "internal/app", Owners: []string{"@alice"}, Provenance: "declared"},
	}}
	shard, err := RenderOwnership(in)
	require.NoError(t, err)
	assert.Equal(t, "derived", shard.Prov.Kind)
}

func TestRenderOwnership_NoOwners_RendersEmDash(t *testing.T) {
	in := RenderInput{OwnershipEntries: []OwnershipEntry{
		{Path: "internal/app", Owners: nil, Provenance: "derived"},
	}}
	shard, err := RenderOwnership(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 1)
	assert.Equal(t, "—", shard.Rows[0][1])
}

func TestRenderOwnership_SortedByPath(t *testing.T) {
	in := RenderInput{OwnershipEntries: []OwnershipEntry{
		{Path: "zebra", Owners: []string{"a"}, Provenance: "derived"},
		{Path: "apple", Owners: []string{"b"}, Provenance: "derived"},
	}}
	shard, err := RenderOwnership(in)
	require.NoError(t, err)
	require.Len(t, shard.Rows, 2)
	assert.Equal(t, "apple", shard.Rows[0][0])
	assert.Equal(t, "zebra", shard.Rows[1][0])
}

func TestRenderOwnership_CaptionStatesTruncation(t *testing.T) {
	entries := make([]OwnershipEntry, 0, 60)
	for i := 0; i < 60; i++ {
		entries = append(entries, OwnershipEntry{
			Path:       string(rune('a'+i%26)) + "_unit",
			Owners:     []string{"owner"},
			Provenance: "derived",
		})
	}
	shard, err := RenderOwnership(RenderInput{OwnershipEntries: entries})
	require.NoError(t, err)
	assert.LessOrEqual(t, len(shard.Rows), ownershipRowsMax)
	assert.Contains(t, shard.Count, "60")
	assert.Contains(t, shard.Count, "showing")
}
