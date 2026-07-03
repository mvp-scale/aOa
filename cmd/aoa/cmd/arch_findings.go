// Package cmd — `aoa arch findings` (L19.16).
//
// Exit codes:
//   - 0: success (when --new: no new findings)
//   - 1: --new flag and findings exist (CI gate)
//   - 2: operational error
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	archFindingsScope    string
	archFindingsNew      bool
	archFindingsSeverity string
	archFindingsPretty   bool
)

var archFindingsCmd = &cobra.Command{
	Use:   "findings [--new] [--severity level] [--scope s]",
	Short: "List architectural findings (cycle, god, orphan, dead-code, …)",
	Long: `List arch findings for the project.

With --new: exit 1 when any findings exist (CI gate). Exit 0 when clean.

Findings are produced by detectors (Tarjan SCC, god-node, orphan, budget
overflow, dead-code candidate, DSM mutual pair) and persisted at compact time.`,
	RunE:          runArchFindings,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	archFindingsCmd.Flags().StringVar(&archFindingsScope, "scope", archDefaultScope, "Scope to query")
	archFindingsCmd.Flags().BoolVar(&archFindingsNew, "new", false, "Exit 1 if any findings exist (CI gate); exit 0 when clean")
	archFindingsCmd.Flags().StringVar(&archFindingsSeverity, "severity", "", "Filter by severity: error|warn|info")
	archFindingsCmd.Flags().BoolVar(&archFindingsPretty, "pretty", false, "Pretty-print JSON output")
}

func runArchFindings(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	ctx, exitCode, err := resolveArch(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	scope := archFindingsScope
	if scope == "" {
		scope = archDefaultScope
	}

	// Daemon path.
	if ctx.socketClient != nil {
		result, err := ctx.socketClient.ArchFindings(scope)
		ctx.closeStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "arch findings: %v\n", err)
			os.Exit(2)
		}
		printFindings(result.Raw, result.HasNew)
		return nil
	}

	// Direct-RO path.
	data, err := ctx.querier.Findings(scope)
	ctx.closeStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "arch findings: %v\n", err)
		os.Exit(2)
	}
	if data == nil {
		data = []byte("[]")
	}
	hasNew := len(data) > 2 // non-empty JSON array
	printFindings(json.RawMessage(data), hasNew)
	return nil
}

// printFindings prints findings JSON and handles --new exit code.
func printFindings(raw json.RawMessage, hasNew bool) {
	prettyPrintJSON(raw, archFindingsPretty)
	if archFindingsNew && hasNew {
		os.Exit(1) // CI gate: findings found
	}
	// exit 0: clean (or --new not specified)
}
