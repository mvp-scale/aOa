// Package cmd — `aoa arch pack` stub (L19.16; body in L22.5).
//
// The pack subcommand registers now; its implementation lands in L22.5.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var archPackCmd = &cobra.Command{
	Use:   "pack <dd|pci|delta>",
	Short: "Package arch views for compliance/diff delivery (stub — body in L22.5)",
	Long: `arch pack packages arch views for structured delivery.

  dd     — data-dictionary package
  pci    — PCI/compliance scope package
  delta  — change-delta package between two fact revisions

Body lands in L22.5. This stub registers the command so help text is correct.`,
	Args:          cobra.ExactArgs(1),
	RunE:          runArchPack,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func runArchPack(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(os.Stderr, "arch pack: not yet implemented (body lands in L22.5)")
	os.Exit(1)
	return nil
}
