package arch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// T4 — Golden determinism: same input -> byte-identical shard across runs
// ---------------------------------------------------------------------------

func TestDeterminism_Capability(t *testing.T) {
	in := makeContextFixture() // reuse VP-1's fixture (adds "ext:" units/deps)

	s1, err := RenderCapability(in)
	require.NoError(t, err)
	b1, err := MarshalShard(s1)
	require.NoError(t, err)

	s2, err := RenderCapability(in)
	require.NoError(t, err)
	b2, err := MarshalShard(s2)
	require.NoError(t, err)

	require.Equal(t, string(b1), string(b2),
		"capability shard must be byte-identical across two independent renders (T4)")

	checkAndUpdateGolden(t, "testdata/golden_capability.json", b1)
}

func TestRenderCapability_Provenance(t *testing.T) {
	in := makeFixture()

	s, err := RenderCapability(in)
	require.NoError(t, err)

	assert.Equal(t, "mixed", s.Prov.Kind,
		"capability seeding is a heuristic over footprint app-bases, never REAL (D2 honesty)")
}

func TestRenderCapability_BucketsInferredTrue(t *testing.T) {
	in := makeFixture()

	s, err := RenderCapability(in)
	require.NoError(t, err)

	require.NotEmpty(t, s.Buckets)
	for _, b := range s.Buckets {
		assert.True(t, b.Inferred, "bucket %q must be stamped Inferred=true — owner relabels DECLARED later (D18/D24)", b.ID)
	}
}

func TestRenderCapability_SingleAnchorCollapsesToOneCapability(t *testing.T) {
	in := makeFixture() // 12 local units, no "ext:" units, single implicit anchor

	s, err := RenderCapability(in)
	require.NoError(t, err)

	require.Len(t, s.Buckets, 1, "v1 footprint always has exactly one anchor (ruling B) -> exactly one capability")
	assert.Len(t, s.Buckets[0].Members, len(in.Units), "every local unit belongs to the single v1 capability")
	assert.Empty(t, s.Edges, "one capability -> no cross-capability edges possible")
}

func TestRenderCapability_ExternalUnitsExcludedFromMembership(t *testing.T) {
	in := makeContextFixture() // adds 3 "ext:" units on top of the 12 local units

	s, err := RenderCapability(in)
	require.NoError(t, err)

	require.Len(t, s.Buckets, 1)
	for _, m := range s.Buckets[0].Members {
		assert.NotContains(t, m.ID, "ext:", "external units are not part of this system's own capability")
	}
	assert.Len(t, s.Buckets[0].Members, 12, "external units must not inflate local capability membership")
}

func TestRenderCapability_Caption(t *testing.T) {
	in := makeFixture()

	s, err := RenderCapability(in)
	require.NoError(t, err)

	assert.Equal(t, "1 groups · 12 members", s.Count)
	assert.NotContains(t, s.Count, "⚠", "A3: calm caption must never carry a findings glyph")
}

func TestRenderCapability_Kind(t *testing.T) {
	in := makeFixture()

	s, err := RenderCapability(in)
	require.NoError(t, err)

	assert.Equal(t, "buckets", s.Kind)
	assert.Equal(t, "Capabilities", s.Title)
}
