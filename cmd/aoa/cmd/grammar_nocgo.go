//go:build !cgo

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var grammarCmd = &cobra.Command{
	Use:   "grammar",
	Short: "Manage tree-sitter grammar packs (requires core build)",
	Long:  "Grammar management requires the core build with CGo/tree-sitter. This is the lean build.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Grammar management requires the core build (tree-sitter + CGo).")
		fmt.Println("This is the lean build. Install the standard aOa package:")
		fmt.Println("  npm install -g @mvpscale/aoa")
		return nil
	},
}
