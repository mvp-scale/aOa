package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestT6_GuidanceBlock_ArchSection asserts the aOaGuidance const contains a
// complete architecture guidance section with all required elements:
//   - Subagent instruction ("AND any subagents you spawn")
//   - Concrete output example (real command + output shape)
//   - Import-edge asymmetry explanation (importer=file:line, imported=unit grain)
//   - External ext: stamp explanation
//   - derive→peek workflow (two-step via locate/grep/peek)
//   - Fallback instruction for missing substrate
//
// L19.17 / T6 gate: guidance block live in init.go.
func TestT6_GuidanceBlock_ArchSection(t *testing.T) {
	g := aOaGuidance

	// Subagent instruction.
	assert.Contains(t, g, "AND any subagents you spawn",
		"T6: arch block must include explicit subagent instruction")

	// Concrete output example — real shapes from aoa arch views/derive.
	assert.Contains(t, g, "aoa arch views",
		"T6: arch block must show aoa arch views example command")
	assert.Contains(t, g, `"id":"component"`,
		"T6: arch block must show concrete views JSON output with id:component")
	assert.Contains(t, g, "aoa arch derive",
		"T6: arch block must show aoa arch derive example command")
	assert.Contains(t, g, `["u_internal_app","u_internal_adapters_bbolt"]`,
		"T6: arch block must show concrete derive output (unit-ID path)")

	// Import-edge asymmetry: importer side = file:line, imported = unit grain.
	assert.Contains(t, g, "file:line",
		"T6: arch block must explain importer side carries file:line")
	assert.Contains(t, g, "unit",
		"T6: arch block must explain imported side is unit/package grain")

	// External ext: stamp.
	assert.Contains(t, g, "ext:",
		"T6: arch block must document ext: stamp for external imports")
	assert.Contains(t, g, "ext:std/fmt",
		"T6: arch block must give a concrete ext: example (ext:std/fmt)")

	// derive → locate → grep → peek workflow (two-line pattern).
	assert.Contains(t, g, "aoa locate",
		"T6: arch block must show aoa locate in derive→peek workflow")
	assert.Contains(t, g, "aoa peek",
		"T6: arch block must show aoa peek in derive→peek workflow")

	// Provenance: trust derived, verify mixed.
	assert.Contains(t, g, "derived",
		"T6: arch block must mention 'derived' provenance")
	assert.Contains(t, g, "mixed",
		"T6: arch block must mention 'mixed' provenance for contrast")

	// Fallback when no substrate.
	assert.Contains(t, g, "no facts substrate",
		"T6: arch block must document fallback when substrate is absent")

	// Sentinel structure is intact.
	assert.Contains(t, g, "<!-- aOa-guidance -->",
		"T6: opening sentinel must be present in aOaGuidance")
	assert.Contains(t, g, "<!-- /aOa-guidance -->",
		"T6: closing sentinel must be present in aOaGuidance")
}

// TestT6_ShimTemplate_InvokesAoa_NotSystemGrep asserts that the shim scripts
// written by createShims call the aoa binary via exec, NOT a bare system grep
// path. This is the regression test for gotcha-3b: the dead-daemon branch must
// route through aoa's lazy-revive, not bypass it via a direct grep call.
//
// L19.17 / T6 gate: template content assertion.
func TestT6_ShimTemplate_InvokesAoa_NotSystemGrep(t *testing.T) {
	dir := t.TempDir()
	// createShims requires .aoa/shims to be creatable.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".aoa", "shims"), 0755))

	ok := createShims(dir)
	assert.True(t, ok, "createShims must succeed in a writable temp dir")

	for _, shimName := range []string{"grep", "egrep"} {
		shimPath := filepath.Join(dir, ".aoa", "shims", shimName)
		content, err := os.ReadFile(shimPath)
		require.NoError(t, err, "shim file %q must be written", shimName)
		s := string(content)

		// Must be a bash script.
		assert.True(t, strings.HasPrefix(s, "#!/usr/bin/env bash"),
			"T6/shim/%s: must start with bash shebang", shimName)

		// Must export AOA_SHIM=1 so the binary knows it's in shim mode.
		assert.Contains(t, s, "AOA_SHIM=1",
			"T6/shim/%s: must export AOA_SHIM=1", shimName)

		// Must exec into the aoa binary (exec "<aoaBin> <subcmd> "$@"").
		assert.Contains(t, s, "exec",
			"T6/shim/%s: must use exec (not fork) to hand off to aoa", shimName)
		assert.Contains(t, s, shimName+` "$@"`,
			"T6/shim/%s: must call aoa %s \"$@\"", shimName, shimName)

		// Must NOT contain any bare system grep paths (the dead-daemon branch
		// must remain inside aoa, not bypass lazy-revive by calling grep directly).
		for _, sysGrep := range []string{"/usr/bin/grep", "/bin/grep", "/usr/local/bin/grep"} {
			assert.NotContains(t, s, sysGrep,
				"T6/shim/%s: must NOT call system grep directly (gotcha-3b)", shimName)
		}

		t.Logf("T6/shim/%s content: %q", shimName, s)
	}
}
