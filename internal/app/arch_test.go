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
	// Helper to set and restore env var
	setEnv := func(val string) func() {
		old := os.Getenv("AOA_ARCH")
		os.Setenv("AOA_ARCH", val)
		return func() {
			if old == "" {
				os.Unsetenv("AOA_ARCH")
			} else {
				os.Setenv("AOA_ARCH", old)
			}
		}
	}

	tmpDir := t.TempDir()

	// Default: ON
	restore := setEnv("")
	assert.True(t, readArchFlag(tmpDir), "empty AOA_ARCH → default ON")
	restore()

	// Explicit off
	for _, val := range []string{"off", "0", "false", "OFF", "False"} {
		restore = setEnv(val)
		assert.False(t, readArchFlag(tmpDir), "AOA_ARCH=%q → OFF", val)
		restore()
	}

	// Explicit on
	for _, val := range []string{"on", "1", "true", "ON"} {
		restore = setEnv(val)
		assert.True(t, readArchFlag(tmpDir), "AOA_ARCH=%q → ON", val)
		restore()
	}
}

// TestC4FlagOff_Config verifies readArchFlag reads .aoa/config for arch=off.
func TestC4FlagOff_Config(t *testing.T) {
	tmpDir := t.TempDir()

	// Ensure env var is unset so config file takes effect
	old := os.Getenv("AOA_ARCH")
	os.Unsetenv("AOA_ARCH")
	defer func() {
		if old != "" {
			os.Setenv("AOA_ARCH", old)
		}
	}()

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
