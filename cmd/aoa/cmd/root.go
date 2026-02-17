package cmd

import (
	"fmt"
	"os"

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
	rootCmd.AddCommand(grepCmd)
	rootCmd.AddCommand(egrepCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(findCmd)
	rootCmd.AddCommand(locateCmd)
	rootCmd.AddCommand(treeCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(wipeCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(openCmd)
}
