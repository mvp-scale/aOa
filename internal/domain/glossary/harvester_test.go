package glossary

import (
	"testing"

	"github.com/corey/aoa/internal/domain/enricher"
	"github.com/corey/aoa/internal/ports"
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

// idxWithGoFile builds a minimal *ports.Index with one "go" (code) file
// carrying the given tokens — the fixture shape HarvestFiltered's
// co-occurrence gate needs (it requires idx.Files[fileID].Language to be an
// actual source-code language, not just a token map).
func idxWithGoFile(fileID uint32, tokens ...string) *ports.Index {
	idx := &ports.Index{
		Files:  map[uint32]*ports.FileMeta{fileID: {Path: "f.go", Language: "go"}},
		Tokens: map[string][]ports.TokenRef{},
	}
	for _, tok := range tokens {
		idx.Tokens[tok] = append(idx.Tokens[tok], ports.TokenRef{FileID: fileID, Line: 1})
	}
	return idx
}

func TestHarvestFiltered_MajorityKeywordsCoOccurInSameFile_Survives(t *testing.T) {
	domains := []enricher.DomainDef{
		{Domain: "scheduling", Terms: map[string][]string{
			"eviction": {"evict", "taint", "drain"},
		}},
	}
	// 2 of 3 keywords (majority) present together in the same code file.
	idx := idxWithGoFile(1, "evict", "taint", "other")
	entries := HarvestFiltered(domains, idx)
	require.Len(t, entries, 1)
	assert.Equal(t, "eviction", entries[0].Term)
}

func TestHarvestFiltered_SingleKeywordAnywhere_NoLongerSufficient(t *testing.T) {
	// VL-1.p1 punch: "any ONE keyword present anywhere in the project" was
	// too weak a gate at real-project scale. A lone matching keyword, with no
	// co-occurring partner in the same file, must no longer be enough.
	domains := []enricher.DomainDef{
		{Domain: "scheduling", Terms: map[string][]string{
			"eviction": {"evict", "taint", "drain"},
		}},
	}
	idx := idxWithGoFile(1, "taint", "other")
	entries := HarvestFiltered(domains, idx)
	assert.Empty(t, entries)
}

func TestHarvestFiltered_KeywordsInDifferentFiles_DoesNotCount(t *testing.T) {
	// Co-occurrence requires the SAME file — scattering matched keywords
	// across unrelated files must not satisfy the majority gate.
	domains := []enricher.DomainDef{
		{Domain: "scheduling", Terms: map[string][]string{
			"eviction": {"evict", "taint", "drain"},
		}},
	}
	idx := &ports.Index{
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "a.go", Language: "go"},
			2: {Path: "b.go", Language: "go"},
		},
		Tokens: map[string][]ports.TokenRef{
			"evict": {{FileID: 1, Line: 1}},
			"taint": {{FileID: 2, Line: 1}},
		},
	}
	entries := HarvestFiltered(domains, idx)
	assert.Empty(t, entries)
}

func TestHarvestFiltered_ProseLanguageFile_ExcludedFromEvidence(t *testing.T) {
	// A markdown/JSON file that happens to contain the atlas's own keywords
	// (e.g. project docs, or the embedded atlas seed itself) must not count
	// as project code vocabulary — only source-code files are evidence.
	domains := []enricher.DomainDef{
		{Domain: "backup", Terms: map[string][]string{
			"unused": {"orphan", "dead", "restore"},
		}},
	}
	idx := &ports.Index{
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "README.md", Language: "md"},
		},
		Tokens: map[string][]ports.TokenRef{
			"orphan":  {{FileID: 1, Line: 1}},
			"dead":    {{FileID: 1, Line: 2}},
			"restore": {{FileID: 1, Line: 3}},
		},
	}
	entries := HarvestFiltered(domains, idx)
	assert.Empty(t, entries)
}

func TestHarvestFiltered_ExcludesTermsWithoutProjectKeywords(t *testing.T) {
	domains := []enricher.DomainDef{
		{Domain: "backup", Terms: map[string][]string{
			"unused": {"orphan", "dead"},
		}},
	}
	idx := idxWithGoFile(1, "present")
	entries := HarvestFiltered(domains, idx)
	assert.Empty(t, entries)
}

func TestHarvestFiltered_EmptyIndex_NoTerms(t *testing.T) {
	domains := []enricher.DomainDef{
		{Domain: "backup", Terms: map[string][]string{
			"unused": {"orphan", "dead"},
		}},
	}
	entries := HarvestFiltered(domains, &ports.Index{})
	assert.Empty(t, entries)
}

func TestHarvestFiltered_NilIndex_NoTerms(t *testing.T) {
	domains := []enricher.DomainDef{
		{Domain: "backup", Terms: map[string][]string{
			"unused": {"orphan", "dead"},
		}},
	}
	entries := HarvestFiltered(domains, nil)
	assert.Empty(t, entries)
}

func TestHarvestFiltered_Deterministic_SortedByDomainThenTerm(t *testing.T) {
	domains := []enricher.DomainDef{
		{Domain: "z_domain", Terms: map[string][]string{"z": {"k"}}},
		{Domain: "a_domain", Terms: map[string][]string{"b": {"k"}, "a": {"k"}}},
	}
	idx := idxWithGoFile(1, "k")
	e1 := HarvestFiltered(domains, idx)
	e2 := HarvestFiltered(domains, idx)
	require.Equal(t, e1, e2)
	require.Len(t, e1, 3)
	assert.Equal(t, "a_domain", e1[0].Domain)
	assert.Equal(t, "a", e1[0].Term)
	assert.Equal(t, "a_domain", e1[1].Domain)
	assert.Equal(t, "b", e1[1].Term)
	assert.Equal(t, "z_domain", e1[2].Domain)
}
