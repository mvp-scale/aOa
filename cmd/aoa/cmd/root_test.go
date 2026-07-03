package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestT36_HelpOutput_ArchAbsentWhenOff is the standing C4 help-output gate
// (checkpoint-F1 PC5 / R6). It verifies that in BOTH off modes —
//   (a) AOA_ARCH=off env var
//   (b) .aoa/config arch=off
//
// — the arch subcommand is absent from the help output (binary surfaces fully
// dark). A fresh cobra.Command is built for each sub-test so the global
// rootCmd (init-time state) is not involved.
func TestT36_HelpOutput_ArchAbsentWhenOff(t *testing.T) {
	t.Run("env_off", func(t *testing.T) {
		t.Setenv("AOA_ARCH", "off")

		enabled := archFlagEnabled()
		assert.False(t, enabled, "T36/env: AOA_ARCH=off must disable arch")

		help := helpOutputFor(t, buildTestRoot(enabled))
		assert.NotContains(t, help, "arch",
			"T36/env: 'arch' must be absent from --help when AOA_ARCH=off")
	})

	t.Run("config_off", func(t *testing.T) {
		// Clear env so the config file is the deciding factor.
		t.Setenv("AOA_ARCH", "")

		tmpDir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".aoa"), 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, ".aoa", "config"),
			[]byte("arch=off\n"),
			0644,
		))
		// archFlagEnabled() walks up from cwd — chdir to tmpDir so it finds
		// tmpDir/.aoa immediately (PC5 walk-up fix, root.go).
		t.Chdir(tmpDir)

		enabled := archFlagEnabled()
		assert.False(t, enabled, "T36/config: .aoa/config arch=off must disable arch")

		help := helpOutputFor(t, buildTestRoot(enabled))
		assert.NotContains(t, help, "arch",
			"T36/config: 'arch' must be absent from --help when .aoa/config arch=off")
	})
}

// TestT36_HelpOutput_ArchPresentByDefault verifies the default-ON behaviour:
// when neither env nor config disables arch, it appears in the help output.
func TestT36_HelpOutput_ArchPresentByDefault(t *testing.T) {
	t.Setenv("AOA_ARCH", "") // empty = default ON

	// Use a temp dir with no .aoa so the walk-up falls through to env-only.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	enabled := archFlagEnabled()
	assert.True(t, enabled, "T36/default: empty AOA_ARCH and no config → arch enabled")

	help := helpOutputFor(t, buildTestRoot(enabled))
	assert.Contains(t, help, "arch",
		"T36/default: 'arch' must appear in --help by default (emission default-ON)")
}

// buildTestRoot builds an isolated cobra root command, registering the arch
// subcommand tree only when archEnabled is true.  This mirrors the pattern in
// root.go init() without touching the package-level rootCmd.
func buildTestRoot(archEnabled bool) *cobra.Command {
	root := &cobra.Command{Use: "aoa", Short: "aOa test root"}
	if archEnabled {
		RegisterArchCommands(root)
	}
	return root
}

// helpOutputFor executes root --help and returns the combined stdout+stderr.
func helpOutputFor(t *testing.T, root *cobra.Command) string {
	t.Helper()
	var sb strings.Builder
	root.SetOut(&sb)
	root.SetErr(&sb)
	root.SetArgs([]string{"--help"})
	_ = root.Execute()
	return sb.String()
}
