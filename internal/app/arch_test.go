//go:build !lean

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

// edgesForDeriveTest returns a minimal resolved edge slice for deriveArch tests.
func edgesForDeriveTest() []ports.ImportEdge {
	return []ports.ImportEdge{
		{FromFile: "internal/app/arch.go", ImportPath: "internal/domain/arch", StartLine: 1},
		{FromFile: "internal/app/arch.go", ImportPath: "ext:std/fmt", StartLine: 2},
		{FromFile: "internal/adapters/bbolt/store.go", ImportPath: "internal/ports", StartLine: 1},
	}
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
