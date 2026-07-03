// Package cmd — arch subcommand skeleton (C4 gate, L19.9).
//
// This file is the construction-time C4 gate for all arch-related Cobra
// subcommands. When AOA_ARCH is off (or .aoa/config contains arch=off),
// App.ArchEnabled=false and RegisterArchCommands is never called, keeping the
// binary fully dark: no subcommands, no routes, no edge emission.
//
// Populated in L19.16 (arch service wire-up). For now the stub satisfies the
// C4 dark-check requirement: when flag is off, the cobra help output must
// contain no "arch" subcommand.

package cmd

import (
	"github.com/spf13/cobra"
)

// RegisterArchCommands adds the arch subcommand tree to the root command.
// Must only be called when App.ArchEnabled == true (C4).
// Calling this when arch is disabled is a contract violation.
func RegisterArchCommands(root *cobra.Command) {
	archCmd := &cobra.Command{
		Use:   "arch",
		Short: "Architectural graph commands (import edges, dependency views)",
		Long: `arch provides subcommands for exploring the import-edge graph
extracted during indexing. Requires AOA_ARCH=on (default).`,
		// No Run/RunE — this is a parent command; subcommands populated in L19.16.
	}

	root.AddCommand(archCmd)
}
