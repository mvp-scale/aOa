package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corey/aoa/internal/app"
	"github.com/corey/aoa/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aoa",
	Short: "aOa — semantic code search engine",
	Long:  "Fast symbol lookup, regex search, and domain-aware results for codebases.",
}

// projectRoot returns the project root (cwd by default).
func projectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return dir
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Version = version.String()

	// Search & query — always available
	rootCmd.AddCommand(grepCmd)
	rootCmd.AddCommand(egrepCmd)
	rootCmd.AddCommand(findCmd)
	rootCmd.AddCommand(locateCmd)
	rootCmd.AddCommand(treeCmd)
	rootCmd.AddCommand(peekCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(openCmd)

	// Admin — hidden in shim mode to prevent accidental state changes
	if !isShimMode() {
		rootCmd.AddCommand(initCmd)
		rootCmd.AddCommand(daemonCmd)
		rootCmd.AddCommand(resetCmd)
		rootCmd.AddCommand(removeCmd)
		rootCmd.AddCommand(configCmd)
		rootCmd.AddCommand(grammarCmd)
	}

	// C4: arch subcommands registered only when AOA_ARCH is on (default ON).
	// When AOA_ARCH=off, the binary stays fully dark — no "arch" in help output,
	// no edge emission, no /api/arch/* routes.
	if archFlagEnabled() {
		RegisterArchCommands(rootCmd)
	}
}

// archFlagEnabled is the C4 predicate for Cobra registration (T36: unified
// predicate — same logic as App.New). It walks up from the current working
// directory to find the project root (the first ancestor that contains a
// .aoa/ directory), then delegates to app.ReadArchFlag so that both the env
// var and the .aoa/config file are consulted at startup.
//
// Walking up (rather than using cwd directly) ensures that running aoa from
// a subdirectory reads the same .aoa/config as running it from the root —
// eliminating the split-brain found in checkpoint-F1 R6 / PC5.
func archFlagEnabled() bool {
	dir, err := os.Getwd()
	if err != nil {
		// Cannot determine cwd; env-only fallback.
		switch strings.ToLower(strings.TrimSpace(os.Getenv("AOA_ARCH"))) {
		case "off", "0", "false":
			return false
		}
		return true
	}
	// Walk up from cwd to find the nearest .aoa directory (project root).
	for {
		dotAOA := filepath.Join(dir, ".aoa")
		if _, statErr := os.Stat(dotAOA); statErr == nil {
			return app.ReadArchFlag(dotAOA)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root without finding .aoa
		}
		dir = parent
	}
	// No .aoa found — env-only (ReadArchFlag on absent path: env takes priority,
	// config read fails gracefully, falls through to default ON).
	return app.ReadArchFlag(filepath.Join(dir, ".aoa"))
}
