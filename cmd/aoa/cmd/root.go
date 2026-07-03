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
// predicate — same logic as App.New). It calls app.ReadArchFlag with the
// .aoa directory derived from the current working directory so that both
// the env var and the .aoa/config file are consulted at startup.
//
// This eliminates the split-brain found in checkpoint-F1 finding 8:
// previously only the env var was checked here, allowing .aoa/config arch=off
// to leave `aoa arch` registered in the help output.
func archFlagEnabled() bool {
	cwd, err := os.Getwd()
	if err != nil {
		// Cannot determine project root; fall back to env-only (safe default).
		switch strings.ToLower(strings.TrimSpace(os.Getenv("AOA_ARCH"))) {
		case "off", "0", "false":
			return false
		}
		return true
	}
	return app.ReadArchFlag(filepath.Join(cwd, ".aoa"))
}
