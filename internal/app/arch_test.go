//go:build !lean

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corey/aoa/internal/adapters/treesitter"
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
