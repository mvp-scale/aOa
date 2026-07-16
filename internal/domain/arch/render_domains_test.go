package arch

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DOM-1 (board L22.23) fixture
//
// Three local units + one external unit:
//   - u_internal_domain_auth:    2 files, both vote "authentication"
//   - u_internal_domain_billing: 3 files, 1 votes "billing", 2 vote "payments"
//     (modal winner is "payments" — proves the per-directory MAJORITY vote,
//     not the bridge's one-file shortcut which would pick the first file's
//     "billing")
//   - u_internal_domain_misc:    no FileDomains entries at all -> unassigned
//   - ext:redis: external, excluded from domain membership entirely
// ---------------------------------------------------------------------------

func makeDomainsFixture() RenderInput {
	auth := UnitSlug("internal/domain/auth")
	billing := UnitSlug("internal/domain/billing")
	misc := UnitSlug("internal/domain/misc")

	units := []UnitFact{
		{ID: auth, Label: "domain/auth", Path: "internal/domain/auth", File: "internal/domain/auth/login.go", Line: 1},
		{ID: billing, Label: "domain/billing", Path: "internal/domain/billing", File: "internal/domain/billing/invoice.go", Line: 1},
		{ID: misc, Label: "domain/misc", Path: "internal/domain/misc", File: "internal/domain/misc/util.go", Line: 1},
		{ID: "ext:redis", Label: "redis", Path: "ext:redis"},
	}

	deps := []DepFact{
		// auth -> billing: cross-domain (authentication -> payments)
		{FromUnit: auth, ToUnit: billing, Count: 3, File: "internal/domain/auth/login.go", Line: 10},
		// billing -> misc: cross-domain (payments -> unassigned)
		{FromUnit: billing, ToUnit: misc, Count: 1, File: "internal/domain/billing/invoice.go", Line: 5},
		// auth -> ext:redis: excluded entirely (external endpoint)
		{FromUnit: auth, ToUnit: "ext:redis", Count: 2, File: "internal/domain/auth/login.go", Line: 20},
	}

	fileDomains := map[string]string{
		"internal/domain/auth/login.go":      "authentication",
		"internal/domain/auth/session.go":    "authentication",
		"internal/domain/billing/invoice.go": "billing",  // minority (1 vote)
		"internal/domain/billing/charge.go":  "payments", // majority (2 votes)
		"internal/domain/billing/refund.go":  "payments",
	}

	return RenderInput{
		Scope:       "test",
		Units:       units,
		Deps:        deps,
		FileDomains: fileDomains,
	}
}

func TestRenderDomains_Kind(t *testing.T) {
	in := makeDomainsFixture()

	s, err := RenderDomains(in)
	require.NoError(t, err)

	assert.Equal(t, "buckets", s.Kind)
	assert.Equal(t, "Domain map", s.Title)
}

func TestRenderDomains_Provenance_AlwaysMixed(t *testing.T) {
	in := makeDomainsFixture()

	s, err := RenderDomains(in)
	require.NoError(t, err)

	assert.Equal(t, "mixed", s.Prov.Kind, "D36: domain votes are an atlas heuristic, never claimed REAL")
	assert.Contains(t, s.Prov.Label, "MIXED")
}

func TestRenderDomains_ModalVotePerDirectory_NotOneFileShortcut(t *testing.T) {
	in := makeDomainsFixture()
	billing := UnitSlug("internal/domain/billing")

	s, err := RenderDomains(in)
	require.NoError(t, err)

	var billingBucket *Bucket
	for i := range s.Buckets {
		for _, m := range s.Buckets[i].Members {
			if m.ID == billing {
				billingBucket = &s.Buckets[i]
			}
		}
	}
	require.NotNil(t, billingBucket, "billing unit must appear in some bucket")
	assert.Equal(t, "dom_payments", billingBucket.ID,
		"majority vote (2 files: payments) must win over the first-seen file (billing) — not the bridge's one-file shortcut")
}

func TestRenderDomains_UnassignedBucket_NeverDropped(t *testing.T) {
	in := makeDomainsFixture()
	misc := UnitSlug("internal/domain/misc")

	s, err := RenderDomains(in)
	require.NoError(t, err)

	var unassigned *Bucket
	for i := range s.Buckets {
		if s.Buckets[i].ID == "dom_unassigned" {
			unassigned = &s.Buckets[i]
		}
	}
	require.NotNil(t, unassigned, "D36: units with no domain vote must land in an explicit 'unassigned' bucket, never be dropped")
	found := false
	for _, m := range unassigned.Members {
		if m.ID == misc {
			found = true
		}
	}
	assert.True(t, found, "misc unit has zero FileDomains entries -> unassigned")
}

func TestRenderDomains_ExternalUnitsExcluded(t *testing.T) {
	in := makeDomainsFixture()

	s, err := RenderDomains(in)
	require.NoError(t, err)

	for _, b := range s.Buckets {
		for _, m := range b.Members {
			assert.NotContains(t, m.ID, "ext:", "external units are not business domains")
		}
	}
}

func TestRenderDomains_CrossDomainEdgesWithCounts(t *testing.T) {
	in := makeDomainsFixture()

	s, err := RenderDomains(in)
	require.NoError(t, err)

	var found *ShardEdge
	for i := range s.Edges {
		if s.Edges[i].Source == "dom_authentication" && s.Edges[i].Target == "dom_payments" {
			found = &s.Edges[i]
		}
	}
	require.NotNil(t, found, "auth -> billing dep must surface as a cross-domain edge (authentication -> payments)")
	assert.Equal(t, 3, found.Count)

	// The ext:redis edge must never appear — external endpoints are excluded.
	for _, e := range s.Edges {
		assert.NotContains(t, e.Source, "ext")
		assert.NotContains(t, e.Target, "ext")
	}
}

func TestRenderDomains_EmptyState_HonestNotPhantom(t *testing.T) {
	in := RenderInput{Scope: "test"}

	s, err := RenderDomains(in)
	require.NoError(t, err)

	assert.Equal(t, "buckets", s.Kind)
	assert.Equal(t, "0 domains · 0 members", s.Count)
}

func TestDeterminism_Domains(t *testing.T) {
	in := makeDomainsFixture()

	s1, err := RenderDomains(in)
	require.NoError(t, err)
	b1, err := MarshalShard(s1)
	require.NoError(t, err)

	s2, err := RenderDomains(in)
	require.NoError(t, err)
	b2, err := MarshalShard(s2)
	require.NoError(t, err)

	require.Equal(t, string(b1), string(b2), "domains shard must be byte-identical across two independent renders (T4)")

	checkAndUpdateGolden(t, "testdata/golden_domains.json", b1)
}

func TestRenderDomains_StripsAtlasAtPrefix(t *testing.T) {
	// enrich.go's assignDomainByKeywords stamps its winner "@domain" (the
	// atlas convention seen throughout aoa grep's @domain tagging) — the real
	// DeriveFileDomains() always returns "@"-prefixed values. RenderDomains
	// must strip it so bucket IDs/labels stay clean.
	auth := UnitSlug("internal/domain/auth")
	units := []UnitFact{
		{ID: auth, Label: "domain/auth", Path: "internal/domain/auth", File: "internal/domain/auth/login.go", Line: 1},
	}
	in := RenderInput{
		Scope: "test",
		Units: units,
		FileDomains: map[string]string{
			"internal/domain/auth/login.go": "@authentication",
		},
	}

	s, err := RenderDomains(in)
	require.NoError(t, err)

	require.Len(t, s.Buckets, 1)
	assert.Equal(t, "dom_authentication", s.Buckets[0].ID)
	assert.Equal(t, "Authentication", s.Buckets[0].Label)
}

func TestRenderDomains_TruncationHonestCaption(t *testing.T) {
	// One domain bucket with 41 members (over the 40 bucket_members_max
	// budget) — caption must state the true total, never claim the shown
	// subset as fact (VP-1.p1 truncation-honest pattern).
	units := make([]UnitFact, 0, 41)
	fileDomains := make(map[string]string, 41)
	for i := 0; i < 41; i++ {
		path := fmt.Sprintf("internal/domain/big%02d", i)
		id := UnitSlug(path)
		units = append(units, UnitFact{ID: id, Label: path, Path: path, File: path + "/f.go", Line: 1})
		fileDomains[path+"/f.go"] = "commerce"
	}
	in := RenderInput{Scope: "test", Units: units, FileDomains: fileDomains}

	s, err := RenderDomains(in)
	require.NoError(t, err)

	require.Len(t, s.Buckets, 1)
	assert.LessOrEqual(t, len(s.Buckets[0].Members), 40, "bucket_members_max budget")
	assert.Contains(t, s.Count, "41 members", "caption must state the TRUE total, not just the shown/capped count")
	assert.Contains(t, s.Count, "showing 40")
}
