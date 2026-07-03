//go:build !lean

package app

import (
	"sync"
	"testing"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// PC3 / ledger T46 — findings honesty: buildRefHits is real fuel, and the
// dead-candidate detector collapses to only genuinely-unreferenced units.
// =============================================================================

// pc3Index builds a small real index + matching edge set. Three units have zero
// inbound deps: u_internal_app and u_internal_live both have indexed files (real
// code that just lacks an inbound import); u_internal_ghost has NO file in the
// index (a genuinely unreferenced/unindexed unit). Only the ghost is a true
// dead-code candidate once refHits is measured.
func pc3Index() (*ports.Index, []ports.ImportEdge) {
	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "internal/app/a.go", Language: "go"},
			2: {Path: "internal/ports/p.go", Language: "go"},
			3: {Path: "internal/live/l.go", Language: "go"},
			// NOTE: internal/ghost/g.go is deliberately absent from the index.
		},
	}
	// Tokens land in files 1, 2, 3 → those units get non-zero refHits.
	idx.Tokens["App"] = []ports.TokenRef{{FileID: 1, Line: 3}, {FileID: 1, Line: 9}}
	idx.Tokens["Store"] = []ports.TokenRef{{FileID: 2, Line: 4}}
	idx.Tokens["Live"] = []ports.TokenRef{{FileID: 3, Line: 2}, {FileID: 3, Line: 7}}

	edges := []ports.ImportEdge{
		{FromFile: "internal/app/a.go", ImportPath: "internal/ports", StartLine: 1},
		{FromFile: "internal/live/l.go", ImportPath: "internal/ports", StartLine: 1},
		// ghost/g.go imports ports but is not indexed → zero refHits for its unit.
		{FromFile: "internal/ghost/g.go", ImportPath: "internal/ports", StartLine: 1},
	}
	return idx, edges
}

// TestPC3_BuildRefHits_RealCounts asserts buildRefHits returns real per-unit
// counts at directory grain and is nil only when the index is nil.
func TestPC3_BuildRefHits_RealCounts(t *testing.T) {
	idx, _ := pc3Index()
	refHits := buildRefHits(idx)

	require.NotNil(t, refHits, "buildRefHits must return a measured (non-nil) map for a real index")
	assert.Equal(t, 2, refHits["u_internal_app"], "two tokens land in internal/app")
	assert.Equal(t, 1, refHits["u_internal_ports"], "one token lands in internal/ports")
	assert.Equal(t, 2, refHits["u_internal_live"], "two tokens land in internal/live")
	assert.Equal(t, 0, refHits["u_internal_ghost"], "ghost has no indexed file → zero references")

	assert.Nil(t, buildRefHits(nil), "nil index → nil (not measured) so the message can stay honest")
}

// TestPC3_DeadCandidateNoiseCollapses is the core T46 assertion: with nil fuel
// (the shipped defect) every zero-inbound unit fires; with real buildRefHits fuel
// only the genuinely-unreferenced unit survives.
func TestPC3_DeadCandidateNoiseCollapses(t *testing.T) {
	idx, edges := pc3Index()
	units, deps := aggregateEdges(edges, idx)

	// Before (defect): nil fuel → all three zero-inbound units fire as noise.
	noisy := arch.DetectDeadCandidates(archScope, units, deps, nil)
	require.Len(t, noisy, 3, "nil fuel fires on every zero-inbound unit (the 30k-noise defect)")
	for _, f := range noisy {
		assert.Contains(t, f.Message, "not measured",
			"nil fuel must NOT claim index references were checked")
	}

	// After (fix): real fuel → only u_internal_ghost survives.
	refHits := buildRefHits(idx)
	collapsed := arch.DetectDeadCandidates(archScope, units, deps, refHits)
	require.Len(t, collapsed, 1, "real fuel collapses noise to only genuinely-unreferenced units")
	assert.Equal(t, []string{"u_internal_ghost"}, collapsed[0].Subjects)
	assert.Contains(t, collapsed[0].Message, "0 index references",
		"measured fuel must state 0 index references, not an unverified 'no index references'")
	assert.NotContains(t, collapsed[0].Message, "not measured")
}

// capturingArchStore records the findings deriveArch persists so the wiring
// (deriveArch → buildRefHits → RenderAll → SaveFindings) can be asserted with a
// real index, not just the domain function in isolation.
type capturingArchStore struct {
	noopStore
	mu       sync.Mutex
	findings []ports.Finding
}

func (s *capturingArchStore) LoadAllEdges(_ string) ([]ports.ImportEdge, error) {
	_, edges := pc3Index()
	return edges, nil
}

func (s *capturingArchStore) SaveShards(_ string, _ map[string][]byte) error  { return nil }
func (s *capturingArchStore) SaveManifest(_ string, _ string, _ []byte) error { return nil }
func (s *capturingArchStore) LoadFindings(_ string, _ string) ([]ports.Finding, error) {
	return nil, nil
}
func (s *capturingArchStore) SaveFindings(_ string, _ string, f []ports.Finding) error {
	s.mu.Lock()
	s.findings = append([]ports.Finding(nil), f...)
	s.mu.Unlock()
	return nil
}

// TestPC3_DeriveArch_UsesRealFuel drives the full app wiring: deriveArch must
// feed buildRefHits(index) into the detector so the persisted findings contain
// exactly one dead-candidate (the ghost), not one per zero-inbound unit.
func TestPC3_DeriveArch_UsesRealFuel(t *testing.T) {
	tmpDir := t.TempDir()
	a := newWatcherTestApp(t, tmpDir)
	store := &capturingArchStore{}
	a.Store = store
	a.ArchEnabled = true
	a.stopCh = make(chan struct{})

	// Seed the live index so buildRefHits has real fuel.
	idx, _ := pc3Index()
	a.mu.Lock()
	a.Index = idx
	a.mu.Unlock()

	a.deriveArch()

	store.mu.Lock()
	findings := store.findings
	store.mu.Unlock()

	var dead []ports.Finding
	for _, f := range findings {
		if f.Rule == "dead-candidate" {
			dead = append(dead, f)
		}
	}
	require.Len(t, dead, 1, "deriveArch must feed real refHits → exactly one dead-candidate (ghost)")
	assert.Equal(t, []string{"u_internal_ghost"}, dead[0].Subjects)
	assert.Contains(t, dead[0].Message, "0 index references")
}
