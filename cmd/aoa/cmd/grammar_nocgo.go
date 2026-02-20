//go:build !cgo

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var grammarCmd = &cobra.Command{
	Use:   "grammar",
	Short: "Manage tree-sitter grammar packs (requires aoa-recon)",
	Long:  "Grammar management requires CGo. Install aoa-recon for full grammar support.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Grammar management requires tree-sitter (CGo).")
		fmt.Println("Install aoa-recon for full grammar and scanning support:")
		fmt.Println("  npm install -g aoa-recon")
		return nil
	},
}
