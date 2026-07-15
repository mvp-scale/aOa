package glossary

import (
	"testing"

	"github.com/corey/aoa/internal/domain/enricher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarvest_OneEntryPerDomainTermPair(t *testing.T) {
	domains := []enricher.DomainDef{
		{Domain: "scheduling", Terms: map[string][]string{
			"eviction": {"evict", "taint", "drain"},
		}},
	}
	entries := Harvest(domains)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "eviction", e.Term)
	assert.Equal(t, "scheduling", e.Domain)
	assert.Contains(t, e.Definition, "drain")
	assert.Contains(t, e.Definition, "evict")
	assert.Contains(t, e.Definition, "taint")
}

func TestHarvest_SharedTermAcrossDomains_OneEntryEach(t *testing.T) {
	domains := []enricher.DomainDef{
		{Domain: "networking", Terms: map[string][]string{"retry": {"backoff"}}},
		{Domain: "resilience", Terms: map[string][]string{"retry": {"circuit-breaker"}}},
	}
	entries := Harvest(domains)
	require.Len(t, entries, 2)
	domainsSeen := map[string]bool{}
	for _, e := range entries {
		assert.Equal(t, "retry", e.Term)
		domainsSeen[e.Domain] = true
	}
	assert.True(t, domainsSeen["networking"])
	assert.True(t, domainsSeen["resilience"])
}

func TestHarvest_DefinitionSortedForByteStability(t *testing.T) {
	domains := []enricher.DomainDef{
		{Domain: "d", Terms: map[string][]string{"t": {"zeta", "alpha", "mid"}}},
	}
	e1 := Harvest(domains)
	e2 := Harvest(domains)
	require.Equal(t, e1, e2)
	assert.Equal(t, "alpha, mid, zeta", e1[0].Definition)
}

func TestHarvest_NoTerms_EmptyResult(t *testing.T) {
	entries := Harvest(nil)
	assert.Empty(t, entries)
}

func TestHarvest_Deterministic_SortedByDomainThenTerm(t *testing.T) {
	domains := []enricher.DomainDef{
		{Domain: "z_domain", Terms: map[string][]string{"z": {"k"}}},
		{Domain: "a_domain", Terms: map[string][]string{"b": {"k"}, "a": {"k"}}},
	}
	entries := Harvest(domains)
	require.Len(t, entries, 3)
	assert.Equal(t, "a_domain", entries[0].Domain)
	assert.Equal(t, "a", entries[0].Term)
	assert.Equal(t, "a_domain", entries[1].Domain)
	assert.Equal(t, "b", entries[1].Term)
	assert.Equal(t, "z_domain", entries[2].Domain)
}
