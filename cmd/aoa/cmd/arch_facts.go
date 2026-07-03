// Package cmd — `aoa arch facts` (L19.16).
//
// Provenance audit trail: import edge facts with file:line stamps.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	archFactsScope  string
	archFactsLimit  int
	archFactsPretty bool
)

var archFactsCmd = &cobra.Command{
	Use:   "facts <subject>",
	Short: "Show import edge facts for a subject (provenance audit trail)",
	Long: `Show the raw import edge facts for a subject (file path or import path substring).

Each fact carries file:line provenance (G7). Subject is matched as a substring
against both the source file path and the resolved import path.

Output is a JSON array of { from_file, import_path, start_line } objects.`,
	Args:          cobra.ExactArgs(1),
	RunE:          runArchFacts,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	archFactsCmd.Flags().StringVar(&archFactsScope, "scope", archDefaultScope, "Scope to query")
	archFactsCmd.Flags().IntVar(&archFactsLimit, "limit", 0, "Max facts to return (0 = unlimited)")
	archFactsCmd.Flags().BoolVar(&archFactsPretty, "pretty", false, "Pretty-print JSON output")
}

func runArchFacts(cmd *cobra.Command, args []string) error {
	subject := args[0]
	root := projectRoot()
	ctx, exitCode, err := resolveArch(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	scope := archFactsScope
	if scope == "" {
		scope = archDefaultScope
	}

	// Daemon path.
	if ctx.socketClient != nil {
		result, err := ctx.socketClient.ArchFacts(scope, subject, archFactsLimit)
		ctx.closeStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "arch facts: %v\n", err)
			os.Exit(2)
		}
		if result.Count == 0 {
			fmt.Fprintf(os.Stderr, "arch facts: no facts found for subject %q\n", subject)
			os.Exit(1)
		}
		prettyPrintJSON(result.Facts, archFactsPretty)
		return nil
	}

	// Direct-RO path.
	data, err := ctx.querier.Facts(scope, subject, archFactsLimit)
	ctx.closeStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "arch facts: %v\n", err)
		os.Exit(2)
	}
	var count int
	if data != nil {
		var entries []json.RawMessage
		_ = json.Unmarshal(data, &entries)
		count = len(entries)
	}
	if count == 0 {
		fmt.Fprintf(os.Stderr, "arch facts: no facts found for subject %q\n", subject)
		os.Exit(1)
	}
	prettyPrintJSON(json.RawMessage(data), archFactsPretty)
	return nil
}
