//go:build !lean

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// GraphPayload domain field tests (QNAV COMMIT A)
// =============================================================================

// TestBuildGraphPayload_Domain verifies that BuildGraphPayload (unit grain)
// propagates the atlas domain from UnitFact.Domain onto GraphNode.Domain when
// an index with domain-enriched files is provided.
func TestBuildGraphPayload_Domain(t *testing.T) {
	edges := []ports.ImportEdge{
		{FromFile: "internal/auth/login.go", ImportPath: "internal/ports", StartLine: 3},
		{FromFile: "internal/ports/search.go", ImportPath: "ext:std/fmt", StartLine: 1},
	}

	// Build an index that maps internal/auth/login.go → domain "authentication"
	idx := &ports.Index{
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "internal/auth/login.go", Domain: "authentication"},
			2: {Path: "internal/ports/search.go", Domain: "search"},
		},
	}

	payload := BuildGraphPayload(edges, idx, "abc123", "unit", "")

	// Find nodes by path prefix
	domainByPath := make(map[string]string)
	for _, n := range payload.Nodes {
		domainByPath[n.Path] = n.Domain
	}

	assert.Equal(t, "authentication", domainByPath["internal/auth"],
		"unit node for internal/auth/login.go should carry the atlas domain from fileDomains")
	assert.Equal(t, "search", domainByPath["internal/ports"],
		"unit node for internal/ports should carry its domain")

	// ext nodes have no domain (UnitFact.Domain is not set for ext targets)
	extDomain, ok := domainByPath["ext:std/fmt"]
	if ok {
		assert.Empty(t, extDomain, "ext node must not carry a domain")
	}
}

// TestBuildGraphPayload_DomainNilIdx verifies that nil idx (C4/headless) still
// produces a valid payload — nodes carry empty Domain, no panic.
func TestBuildGraphPayload_DomainNilIdx(t *testing.T) {
	edges := []ports.ImportEdge{
		{FromFile: "internal/app/arch.go", ImportPath: "internal/domain/arch", StartLine: 1},
	}
	payload := BuildGraphPayload(edges, nil, "test", "unit", "")
	require.NotEmpty(t, payload.Nodes, "nil idx must still produce nodes")
	for _, n := range payload.Nodes {
		assert.Empty(t, n.Domain, "nil idx: no domain expected on any node")
	}
}

// TestBuildGraphPayload_DomainDeterminism verifies that two calls with identical
// input produce byte-identical payloads (sort stability / determinism).
func TestBuildGraphPayload_DomainDeterminism(t *testing.T) {
	edges := []ports.ImportEdge{
		{FromFile: "pkg/a/a.go", ImportPath: "pkg/b", StartLine: 1},
		{FromFile: "pkg/b/b.go", ImportPath: "ext:std/fmt", StartLine: 1},
		{FromFile: "pkg/c/c.go", ImportPath: "pkg/a", StartLine: 5},
	}
	idx := &ports.Index{
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "pkg/a/a.go", Domain: "alpha"},
			2: {Path: "pkg/b/b.go", Domain: "beta"},
			3: {Path: "pkg/c/c.go", Domain: "gamma"},
		},
	}
	p1 := BuildGraphPayload(edges, idx, "rev1", "unit", "")
	p2 := BuildGraphPayload(edges, idx, "rev1", "unit", "")
	require.Equal(t, len(p1.Nodes), len(p2.Nodes), "node count must be stable")
	for i := range p1.Nodes {
		assert.Equal(t, p1.Nodes[i].ID, p2.Nodes[i].ID, "node order must be stable")
		assert.Equal(t, p1.Nodes[i].Domain, p2.Nodes[i].Domain, "domain must be stable")
	}
}

// =============================================================================
// QNAV COMMIT D — DeriveFileDomains + Graph() wiring tests
// =============================================================================

// buildDeriveArchTestEngine creates a SearchEngine whose refToTokens maps
// tokens for "auth/login.go" (FileID 1) to authentication-domain keywords.
// Used to test that DeriveFileDomains → BuildGraphPayload produces non-empty
// Domain fields on GraphNode.
func buildDeriveArchTestEngine() *index.SearchEngine {
	idx := &ports.Index{
		Tokens: map[string][]ports.TokenRef{
			"login":    {{FileID: 1, Line: 10}},
			"logout":   {{FileID: 1, Line: 20}},
			"password": {{FileID: 1, Line: 30}},
		},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{
			{FileID: 1, Line: 10}: {Name: "Login", Kind: "function", StartLine: 10, EndLine: 15},
			{FileID: 1, Line: 20}: {Name: "Logout", Kind: "function", StartLine: 20, EndLine: 25},
			{FileID: 1, Line: 30}: {Name: "Password", Kind: "function", StartLine: 30, EndLine: 35},
		},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "auth/login.go"},
		},
	}
	domains := map[string]index.Domain{
		"authentication": {Terms: map[string][]string{
			"login":    {"login", "logout", "authenticate"},
			"password": {"password", "bcrypt", "salt"},
		}},
	}
	return index.NewSearchEngine(idx, domains, "")
}

// TestDeriveFileDomains_GraphChain verifies the full chain:
// DeriveFileDomains() → populate cloned idx.Files → BuildGraphPayload → GraphNode.Domain.
// This is the Commit D integration path: token scoring produces non-empty domain fields
// on unit-grain graph nodes when the Engine is wired.
func TestDeriveFileDomains_GraphChain(t *testing.T) {
	engine := buildDeriveArchTestEngine()
	edges := []ports.ImportEdge{
		{FromFile: "auth/login.go", ImportPath: "ext:std/fmt", StartLine: 1},
	}

	// Simulate what Graph() does: clone idx, derive domains, populate clone.
	idx := &ports.Index{
		Tokens:   map[string][]ports.TokenRef{},
		Metadata: map[ports.TokenRef]*ports.SymbolMeta{},
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "auth/login.go"},
		},
	}
	derived := engine.DeriveFileDomains()
	for fileID, fm := range idx.Files {
		if d, ok := derived[fm.Path]; ok {
			idx.Files[fileID].Domain = d
		}
	}

	payload := BuildGraphPayload(edges, idx, "test", "unit", "")
	domainByPath := make(map[string]string)
	for _, n := range payload.Nodes {
		domainByPath[n.Path] = n.Domain
	}

	assert.Equal(t, "@authentication", domainByPath["auth"],
		"unit node for auth/ must carry @authentication from DeriveFileDomains")
	// ext nodes must never carry a domain.
	if d, ok := domainByPath["ext:std/fmt"]; ok {
		assert.Empty(t, d, "ext node must not carry a domain")
	}
}

// TestDeriveFileDomains_GraphChain_Determinism verifies that two successive
// DeriveFileDomains calls followed by BuildGraphPayload produce identical node ordering
// and identical Domain fields (byte-determinism requirement).
func TestDeriveFileDomains_GraphChain_Determinism(t *testing.T) {
	engine := buildDeriveArchTestEngine()
	edges := []ports.ImportEdge{
		{FromFile: "auth/login.go", ImportPath: "ext:std/fmt", StartLine: 1},
	}

	runOnce := func() []ports.GraphNode {
		idx := &ports.Index{
			Tokens:   map[string][]ports.TokenRef{},
			Metadata: map[ports.TokenRef]*ports.SymbolMeta{},
			Files: map[uint32]*ports.FileMeta{
				1: {Path: "auth/login.go"},
			},
		}
		derived := engine.DeriveFileDomains()
		for fileID, fm := range idx.Files {
			if d, ok := derived[fm.Path]; ok {
				idx.Files[fileID].Domain = d
			}
		}
		return BuildGraphPayload(edges, idx, "rev", "unit", "").Nodes
	}

	n1 := runOnce()
	n2 := runOnce()
	require.Equal(t, len(n1), len(n2), "node count must be stable")
	for i := range n1 {
		assert.Equal(t, n1[i].ID, n2[i].ID, "node ID[%d] must be stable", i)
		assert.Equal(t, n1[i].Domain, n2[i].Domain, "node Domain[%d] must be stable", i)
	}
}

// =============================================================================
// L19.9 C4 dark test — flag-off = zero edges ("fully dark")
// =============================================================================

// TestC4FlagOff_Dark_BuildIndexWithFacts verifies that BuildIndexWithFacts
// returns nil edges when archEnabled=false, regardless of parser capability.
// This is the "flag-off = fully dark" assertion for the C4 kill switch.
func TestC4FlagOff_Dark_BuildIndexWithFacts(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	// Create a Go file with imports that WOULD produce edges if arch were on
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "main.go"),
		[]byte(`package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func Hello() {
	fmt.Println(os.Args, filepath.Separator)
}
`),
		0644,
	))

	// C4 OFF — must return zero edges
	_, result, edges, err := BuildIndexWithFacts(tmpDir, parser, false)
	require.NoError(t, err)
	assert.Nil(t, edges, "C4 off: edges must be nil (fully dark)")
	assert.Equal(t, 0, result.EdgeCount, "C4 off: EdgeCount must be zero")

	// C4 ON — must return non-zero edges (3 imports in the file)
	_, resultOn, edgesOn, err := BuildIndexWithFacts(tmpDir, parser, true)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(edgesOn), 3, "C4 on: edges must include fmt, os, filepath")
	assert.GreaterOrEqual(t, resultOn.EdgeCount, 3, "C4 on: EdgeCount must reflect extracted edges")
}

// TestC4FlagOff_ReadArchFlag verifies readArchFlag correctly reads the env var.
func TestC4FlagOff_ReadArchFlag(t *testing.T) {
	tmpDir := t.TempDir()

	// Default: ON — t.Setenv registers cleanup to restore original value at test end.
	t.Setenv("AOA_ARCH", "")
	assert.True(t, readArchFlag(tmpDir), "empty AOA_ARCH → default ON")

	// Explicit off — each t.Setenv call saves the current value and restores in LIFO order.
	for _, val := range []string{"off", "0", "false", "OFF", "False"} {
		t.Setenv("AOA_ARCH", val)
		assert.False(t, readArchFlag(tmpDir), "AOA_ARCH=%q → OFF", val)
	}

	// Explicit on
	for _, val := range []string{"on", "1", "true", "ON"} {
		t.Setenv("AOA_ARCH", val)
		assert.True(t, readArchFlag(tmpDir), "AOA_ARCH=%q → ON", val)
	}
}

// TestC4FlagOff_Config verifies readArchFlag reads .aoa/config for arch=off.
func TestC4FlagOff_Config(t *testing.T) {
	tmpDir := t.TempDir()

	// Ensure env var is unset so config file takes effect.
	t.Setenv("AOA_ARCH", "")

	// Without config: default ON
	assert.True(t, readArchFlag(tmpDir))

	// Write config with arch=off
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "config"),
		[]byte("# aOa config\narch=off\n"),
		0644,
	))
	assert.False(t, readArchFlag(tmpDir), ".aoa/config arch=off → disabled")

	// Write config with AOA_ARCH=off
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "config"),
		[]byte("AOA_ARCH=off\n"),
		0644,
	))
	assert.False(t, readArchFlag(tmpDir), ".aoa/config AOA_ARCH=off → disabled")
}

// TestT36_ReadArchFlagExported verifies that the exported ReadArchFlag function
// is the unified C4 predicate: it reads env AND .aoa/config identically to the
// internal readArchFlag used by App.New, eliminating the split-brain found in
// checkpoint-F1 finding 8 (root.go checked only env; App.New checked both).
func TestT36_ReadArchFlagExported(t *testing.T) {
	tmpDir := t.TempDir()

	// Unset AOA_ARCH so config-file path is exercised.
	t.Setenv("AOA_ARCH", "")

	// Default: ON when no env and no config file.
	assert.True(t, ReadArchFlag(tmpDir), "T36: exported ReadArchFlag default must be ON")
	assert.True(t, readArchFlag(tmpDir), "T36: internal readArchFlag default must be ON")
	assert.Equal(t, ReadArchFlag(tmpDir), readArchFlag(tmpDir),
		"T36: exported and internal predicates must agree")

	// Write config with arch=off — both must return false.
	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "config"),
		[]byte("# aOa config\narch=off\n"),
		0644,
	))
	assert.False(t, ReadArchFlag(tmpDir), "T36: config arch=off must disable via exported ReadArchFlag")
	assert.False(t, readArchFlag(tmpDir), "T36: config arch=off must disable via internal readArchFlag")
	assert.Equal(t, ReadArchFlag(tmpDir), readArchFlag(tmpDir),
		"T36: exported and internal predicates must agree on config-off")
}

// TestBuildIndexWithFacts_EdgeProvenance verifies that edges have correct
// relative FromFile paths and non-zero StartLine (G7 provenance).
func TestBuildIndexWithFacts_EdgeProvenance(t *testing.T) {
	tmpDir := t.TempDir()
	parser := treesitter.NewParser()

	require.NoError(t, os.WriteFile(
		filepath.Join(tmpDir, "server.go"),
		[]byte(`package main

import "fmt"
import "net/http"

func Serve() {
	fmt.Println("listening")
	http.ListenAndServe(":8080", nil)
}
`),
		0644,
	))

	_, _, edges, err := BuildIndexWithFacts(tmpDir, parser, true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(edges), 2, "expected edges for fmt and net/http")

	for _, e := range edges {
		assert.Equal(t, "server.go", e.FromFile, "FromFile must be relative path")
		assert.Greater(t, e.StartLine, uint32(0), "StartLine must be non-zero (G7)")
		assert.NotEmpty(t, e.ImportPath, "ImportPath must not be empty")
	}
}

// =============================================================================
// L19.14 app-wiring tests — C4 dark check + C2 burst debounce
// =============================================================================

// countingArchStore embeds noopStore and counts SaveShards calls so tests can
// assert that N file events coalesce into a bounded number of arch renders.
type countingArchStore struct {
	noopStore
	shardCalls    int64 // number of SaveShards calls
	manifestCalls int64 // number of SaveManifest calls
	findingCalls  int64 // number of SaveFindings calls
	mu            sync.Mutex
}

func (s *countingArchStore) SaveShards(_ string, _ map[string][]byte) error {
	s.mu.Lock()
	s.shardCalls++
	s.mu.Unlock()
	return nil
}
func (s *countingArchStore) SaveManifest(_ string, _ string, _ []byte) error {
	s.mu.Lock()
	s.manifestCalls++
	s.mu.Unlock()
	return nil
}
func (s *countingArchStore) LoadFindings(_ string, _ string) ([]ports.Finding, error) {
	return nil, nil
}
func (s *countingArchStore) SaveFindings(_ string, _ string, _ []ports.Finding) error {
	s.mu.Lock()
	s.findingCalls++
	s.mu.Unlock()
	return nil
}
// countingArchStore needs to return non-empty edges so deriveArch proceeds past
// the early-return guard. It uses edgesForDeriveTest to supply them.
func (s *countingArchStore) LoadAllEdges(_ string) ([]ports.ImportEdge, error) {
	return edgesForDeriveTest(), nil
}

// FDN-4: deriveArch now reads the FactStore query plane (FactsByKind(FactUnit)
// + Dependencies) instead of LoadAllEdges. countingArchStore must serve the
// SAME edge set through that plane too, or deriveArch's "nothing to derive
// yet" guard fires and no SaveShards call is ever observed.
func (s *countingArchStore) FactsByKind(_ string, kind ports.FactKind) ([]ports.Fact, error) {
	if kind != ports.FactUnit {
		return nil, nil
	}
	units, _ := factsFromResolvedEdges(edgesForDeriveTest())
	return units, nil
}
func (s *countingArchStore) Dependencies(_, unit string) ([]ports.DepEdge, error) {
	_, adj := factsFromResolvedEdges(edgesForDeriveTest())
	return adj.Fwd[unit], nil
}

// edgesForDeriveTest returns a minimal resolved edge slice for deriveArch tests.
func edgesForDeriveTest() []ports.ImportEdge {
	return []ports.ImportEdge{
		{FromFile: "internal/app/arch.go", ImportPath: "internal/domain/arch", StartLine: 1},
		{FromFile: "internal/app/arch.go", ImportPath: "ext:std/fmt", StartLine: 2},
		{FromFile: "internal/adapters/bbolt/store.go", ImportPath: "internal/ports", StartLine: 1},
	}
}

// factsFromResolvedEdges builds synthetic FactStore-shaped fixtures (unit
// facts + forward adjacency) from an already-RESOLVED edge set — the exact
// shape aggregateEdges consumes (ImportPath is a directory or "ext:...", not
// a raw unresolved spec). This lets test doubles drive deriveArch's new
// FactStore-based path without standing up a real bbolt-backed compaction.
// Mirrors factSubjectForFile's "go:" namespace (directory grain) since every
// caller here seeds Go-only fixtures.
func factsFromResolvedEdges(edges []ports.ImportEdge) ([]ports.Fact, *ports.DepAdjacency) {
	toSubject := func(dir string) string {
		if dir == "." {
			dir = ""
		}
		return "go:" + dir
	}

	unitByID := make(map[string]ports.Fact)
	order := make([]string, 0, len(edges))
	fwdCount := make(map[string]map[string]int)

	for _, e := range edges {
		fromSubj := toSubject(filepath.Dir(e.FromFile))
		if _, ok := unitByID[fromSubj]; !ok {
			unitByID[fromSubj] = ports.Fact{
				Kind: ports.FactUnit, Subject: fromSubj,
				Source: ports.FactSource{File: e.FromFile, Line: e.StartLine},
			}
			order = append(order, fromSubj)
		}

		toSubj := e.ImportPath
		if !strings.HasPrefix(toSubj, "ext:") {
			toSubj = toSubject(e.ImportPath)
			if _, ok := unitByID[toSubj]; !ok {
				unitByID[toSubj] = ports.Fact{
					Kind: ports.FactUnit, Subject: toSubj,
					Source: ports.FactSource{File: e.FromFile},
				}
				order = append(order, toSubj)
			}
		}

		if fwdCount[fromSubj] == nil {
			fwdCount[fromSubj] = make(map[string]int)
		}
		fwdCount[fromSubj][toSubj]++
	}

	units := make([]ports.Fact, 0, len(order))
	for _, id := range order {
		units = append(units, unitByID[id])
	}
	fwd := make(map[string][]ports.DepEdge, len(fwdCount))
	for subj, targets := range fwdCount {
		edgeList := make([]ports.DepEdge, 0, len(targets))
		for obj, cnt := range targets {
			edgeList = append(edgeList, ports.DepEdge{Unit: obj, Count: uint16(cnt)})
		}
		fwd[subj] = edgeList
	}
	return units, &ports.DepAdjacency{Fwd: fwd}
}

// TestC4FlagOff_DeriveArch verifies the C4 gate: when ArchEnabled is false,
// deriveArch is a no-op and no store writes are triggered.
func TestC4FlagOff_DeriveArch(t *testing.T) {
	tmpDir := t.TempDir()
	a := newWatcherTestApp(t, tmpDir)
	counter := &countingArchStore{}
	a.Store = counter
	a.stopCh = make(chan struct{})

	// C4 OFF — deriveArch must not write anything.
	a.ArchEnabled = false
	a.deriveArch()

	counter.mu.Lock()
	shards := counter.shardCalls
	counter.mu.Unlock()

	assert.Equal(t, int64(0), shards,
		"C4 off: deriveArch must not call SaveShards")
}

// TestArchDerive_C2BurstDebounce verifies that N file events within one debounce
// window produce exactly one arch render (one SaveShards call), not N.
// This is the C2 burst-coalescing assertion for the arch derive path.
func TestArchDerive_C2BurstDebounce(t *testing.T) {
	const N = 10

	tmpDir := t.TempDir()
	a := newWatcherTestApp(t, tmpDir)
	counter := &countingArchStore{}
	a.Store = counter
	a.ArchEnabled = true
	a.stopCh = make(chan struct{})

	// Create N Go files that will be batched into one debounce window.
	for i := 0; i < N; i++ {
		f := filepath.Join(tmpDir, fmt.Sprintf("c2_%d.go", i))
		require.NoError(t, os.WriteFile(f,
			[]byte(fmt.Sprintf("package main\nimport \"fmt\"\nfunc F%d() { fmt.Println(%d) }\n", i, i)),
			0644))
		a.onFileChanged(f) // accumulates into edgePendingBatch
	}

	// No flush yet — no renders should have fired.
	counter.mu.Lock()
	preShard := counter.shardCalls
	counter.mu.Unlock()
	assert.Equal(t, int64(0), preShard,
		"C2: no SaveShards should fire before doFlushEdgeBatch")

	// Single flush — triggers exactly one safeGo("arch-derive").
	a.doFlushEdgeBatch()
	a.bgWg.Wait() // wait for arch-derive to finish

	counter.mu.Lock()
	postShard := counter.shardCalls
	counter.mu.Unlock()
	assert.Equal(t, int64(1), postShard,
		"C2: %d file events in one debounce window must produce exactly 1 SaveShards call", N)
}

// TestArchDerive_Querier verifies that App.Arch() returns a non-nil querier
// when ArchEnabled=true, and nil when ArchEnabled=false.
func TestArchDerive_Querier(t *testing.T) {
	tmpDir := t.TempDir()
	a := newWatcherTestApp(t, tmpDir)
	counter := &countingArchStore{}
	a.Store = counter
	a.stopCh = make(chan struct{})

	a.ArchEnabled = false
	assert.Nil(t, a.Arch(), "C4 off: Arch() must return nil")

	a.ArchEnabled = true
	assert.NotNil(t, a.Arch(), "C4 on: Arch() must return a non-nil querier")
}

// TestArchDerive_RaceWithIndexSwap is the regression test for the 5980b6a
// review blocker: deriveArch (and archQuerier.Derive) must Clone the index
// under mu — capturing the live *ports.Index pointer races with the
// Index.Files swaps that Reindex/WarmCaches/Wipe perform under mu after the
// snapshot is released. Meaningful only under -race: with an aliased pointer
// the writer goroutine below trips the detector inside aggregateEdges.
func TestArchDerive_RaceWithIndexSwap(t *testing.T) {
	tmpDir := t.TempDir()
	a := newWatcherTestApp(t, tmpDir)
	counter := &countingArchStore{}
	a.Store = counter
	a.ArchEnabled = true
	a.stopCh = make(chan struct{})

	// Seed Files so aggregateEdges has something to read.
	a.mu.Lock()
	a.Index.Files = map[uint32]*ports.FileMeta{
		1: {Path: "internal/app/arch.go"},
		2: {Path: "internal/adapters/bbolt/store.go"},
	}
	a.mu.Unlock()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	// Writer: swap Index.Files under mu, exactly as Reindex's in-memory swap does.
	go func() {
		defer wg.Done()
		for i := uint32(0); ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			a.mu.Lock()
			a.Index.Files = map[uint32]*ports.FileMeta{
				i % 8: {Path: fmt.Sprintf("internal/gen/f%d.go", i)},
			}
			a.mu.Unlock()
		}
	}()

	// Deriver + querier: both snapshot the index; both must be race-free.
	q := a.Arch()
	require.NotNil(t, q)
	for i := 0; i < 50; i++ {
		a.deriveArch()
		_, _ = q.Derive("local", "u_internal_app", "u_internal_ports", 4)
	}
	close(stop)
	wg.Wait()
}
