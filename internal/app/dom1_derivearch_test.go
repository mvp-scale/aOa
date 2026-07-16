//go:build !lean

package app

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shardCapturingArchStore records the shard bytes deriveArch persists, keyed
// by the manifest's "{scope}/{view}@{hash}" storage key.
type shardCapturingArchStore struct {
	noopStore
	mu     sync.Mutex
	shards map[string][]byte
}

func (s *shardCapturingArchStore) LoadAllEdges(_ string) ([]ports.ImportEdge, error) {
	_, edges := dom1Index()
	return edges, nil
}

func (s *shardCapturingArchStore) FactsByKind(_ string, kind ports.FactKind) ([]ports.Fact, error) {
	if kind != ports.FactUnit {
		return nil, nil
	}
	_, edges := dom1Index()
	units, _ := factsFromResolvedEdges(edges)
	return units, nil
}

func (s *shardCapturingArchStore) Dependencies(_, unit string) ([]ports.DepEdge, error) {
	_, edges := dom1Index()
	_, adj := factsFromResolvedEdges(edges)
	return adj.Fwd[unit], nil
}

func (s *shardCapturingArchStore) SaveShards(_ string, shards map[string][]byte) error {
	s.mu.Lock()
	s.shards = make(map[string][]byte, len(shards))
	for k, v := range shards {
		s.shards[k] = v
	}
	s.mu.Unlock()
	return nil
}

func (s *shardCapturingArchStore) SaveManifest(_ string, _ string, _ []byte) error { return nil }
func (s *shardCapturingArchStore) SaveFindings(_ string, _ string, _ []ports.Finding) error {
	return nil
}
func (s *shardCapturingArchStore) LoadFindings(_ string, _ string) ([]ports.Finding, error) {
	return nil, nil
}

// dom1Index builds a small real index + matching edge set: two files under
// "auth/" both scoring the "authentication" atlas domain (real token
// overlap), plus one file under "misc/" with no domain-scoring tokens at all
// (must land in the domains view's "unassigned" bucket, D36).
func dom1Index() (*ports.Index, []ports.ImportEdge) {
	idx := &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"login":    {{FileID: 1, Line: 10}},
			"logout":   {{FileID: 1, Line: 20}},
			"password": {{FileID: 2, Line: 5}},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "Login", Kind: "function", StartLine: 10, EndLine: 15},
			{FileID: 1, Line: 20}: {Name: "Logout", Kind: "function", StartLine: 20, EndLine: 25},
			{FileID: 2, Line: 5}:  {Name: "Password", Kind: "function", StartLine: 5, EndLine: 9},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "auth/login.go"},
			2: {Path: "auth/session.go"},
			3: {Path: "misc/util.go"},
		},
	}
	edges := []ports.ImportEdge{
		{FromFile: "auth/login.go", ImportPath: "misc", StartLine: 1},
		{FromFile: "misc/util.go", ImportPath: "ext:std/fmt", StartLine: 1},
	}
	return idx, edges
}

// shardFor returns the shard bytes for the given view ID, keyed
// "{archScope}/{view}@{hash}" (deriveArch's SaveShards key format).
func shardFor(t *testing.T, shards map[string][]byte, view string) []byte {
	t.Helper()
	prefix := archScope + "/" + view + "@"
	for k, v := range shards {
		if strings.HasPrefix(k, prefix) {
			return v
		}
	}
	return nil
}

// TestDOM1_DeriveArch_FileDomainsReachesDomainsViewOnly is the integration
// regression test for DOM-1 (board L22.23): deriveArch must feed
// Engine.DeriveFileDomains() into the domains view via vlIn.FileDomains, and
// that value must NEVER leak into UnitFact.Domain or idx.Files[].Domain —
// D35 requires rung-3 (grouping.go:189-194) to stay dormant so component/dsm/
// cycles are never silently regrouped by the same atlas vote.
func TestDOM1_DeriveArch_FileDomainsReachesDomainsViewOnly(t *testing.T) {
	tmpDir := t.TempDir()
	a := newWatcherTestApp(t, tmpDir)

	idx, _ := dom1Index()
	domains := map[string]index.Domain{
		"authentication": {Terms: map[string][]string{
			"login":    {"login", "logout", "authenticate"},
			"password": {"password", "bcrypt", "salt"},
		}},
	}
	a.mu.Lock()
	a.Index = idx
	a.Engine = index.NewSearchEngine(idx, domains, tmpDir)
	a.mu.Unlock()

	store := &shardCapturingArchStore{}
	a.Store = store
	a.ArchEnabled = true
	a.stopCh = make(chan struct{})

	a.deriveArch()

	store.mu.Lock()
	shards := store.shards
	store.mu.Unlock()
	require.NotEmpty(t, shards, "deriveArch must have persisted shards")

	// D35: deriveArch must never write idx.Files[].Domain — the derived
	// file->domain map reaches ONLY vlIn.FileDomains, never the live index.
	for fileID := range idx.Files {
		assert.Empty(t, idx.Files[fileID].Domain, "deriveArch must never write idx.Files[].Domain (D35 — rung-3 stays dormant)")
	}

	// domains view: auth unit (real atlas tokens) lands in a real domain
	// bucket; misc unit (zero atlas tokens) lands in "unassigned" (D36).
	domainsShard := shardFor(t, shards, "domains")
	require.NotNil(t, domainsShard, "domains view must be persisted (D37 — mandatory view)")

	var shard arch.Shard
	require.NoError(t, json.Unmarshal(domainsShard, &shard))
	assert.Equal(t, "mixed", shard.Prov.Kind, "D36: domains view is always MIXED provenance")

	foundAuthInRealDomain := false
	foundMiscUnassigned := false
	for _, b := range shard.Buckets {
		for _, m := range b.Members {
			if m.ID == "u_auth" && b.ID != "dom_unassigned" {
				foundAuthInRealDomain = true
			}
			if m.ID == "u_misc" && b.ID == "dom_unassigned" {
				foundMiscUnassigned = true
			}
		}
	}
	assert.True(t, foundAuthInRealDomain, "auth unit scored real atlas tokens -> must land in a real domain bucket, not unassigned")
	assert.True(t, foundMiscUnassigned, "misc unit scored zero atlas tokens -> must land in the explicit unassigned bucket (D36)")

	// component view must still be grouped by rung-2 path-prefix (derived),
	// never rung-3 atlas domain — confirms D35's dormancy held.
	componentShard := shardFor(t, shards, "component")
	require.NotNil(t, componentShard)
	var cshard arch.Shard
	require.NoError(t, json.Unmarshal(componentShard, &cshard))
	assert.Equal(t, "derived", cshard.Prov.Kind, "D35: component view must stay path-prefix derived, never silently regrouped by atlas domain")
}
