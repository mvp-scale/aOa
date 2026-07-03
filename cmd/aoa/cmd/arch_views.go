// Package cmd — `aoa arch views` and `aoa arch view <id>` (L19.16).
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	archViewsScope string
	archViewScope  string
	archViewPretty bool
)

var archViewsCmd = &cobra.Command{
	Use:           "views [--scope s]",
	Short:         "List all rendered arch views (manifest)",
	Long:          "List the rendered architecture views for a scope. Output is the manifest JSON with view IDs, content hashes, and captions.",
	RunE:          runArchViews,
	SilenceErrors: true,
	SilenceUsage:  true,
}

var archViewCmd = &cobra.Command{
	Use:           "view <id> [--scope s]",
	Short:         "Show a single rendered arch view as JSON",
	Long:          "Return the rendered shard JSON for a specific view ID (e.g. 'component', 'dsm', 'cycles').",
	Args:          cobra.ExactArgs(1),
	RunE:          runArchView,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	archViewsCmd.Flags().StringVar(&archViewsScope, "scope", archDefaultScope, "Scope to query")
	archViewsCmd.Flags().BoolVar(&archViewPretty, "pretty", false, "Pretty-print JSON output")

	archViewCmd.Flags().StringVar(&archViewScope, "scope", archDefaultScope, "Scope to query")
	archViewCmd.Flags().BoolVar(&archViewPretty, "pretty", false, "Pretty-print JSON output")
}

func runArchViews(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	ctx, exitCode, err := resolveArch(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	scope := archViewsScope
	if scope == "" {
		scope = archDefaultScope
	}

	// Daemon path.
	if ctx.socketClient != nil {
		result, err := ctx.socketClient.ArchViews(scope)
		ctx.closeStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "arch views: %v\n", err)
			os.Exit(2)
		}
		if !result.HasData {
			fmt.Fprintln(os.Stderr, "arch views: no shards derived yet (run: aoa daemon start)")
			os.Exit(1)
		}
		prettyPrintJSON(result.Raw, archViewPretty)
		return nil
	}

	// Direct-RO path.
	m, err := ctx.querier.Manifest(scope)
	ctx.closeStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "arch views: %v\n", err)
		os.Exit(2)
	}
	if m == nil {
		fmt.Fprintln(os.Stderr, "arch views: no shards derived yet (run: aoa daemon start)")
		os.Exit(1)
	}
	raw, _ := json.Marshal(m)
	prettyPrintJSON(json.RawMessage(raw), archViewPretty)
	return nil
}

func runArchView(cmd *cobra.Command, args []string) error {
	viewID := args[0]
	root := projectRoot()
	ctx, exitCode, err := resolveArch(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	scope := archViewScope
	if scope == "" {
		scope = archDefaultScope
	}

	// Daemon path.
	if ctx.socketClient != nil {
		result, err := ctx.socketClient.ArchView(scope, viewID)
		ctx.closeStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "arch view: %v\n", err)
			os.Exit(2)
		}
		if !result.Found {
			fmt.Fprintf(os.Stderr, "arch view: view %q not found in scope %q\n", viewID, scope)
			os.Exit(1)
		}
		prettyPrintJSON(result.Raw, archViewPretty)
		return nil
	}

	// Direct-RO path.
	data, err := ctx.querier.View(scope, viewID)
	ctx.closeStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "arch view: %v\n", err)
		os.Exit(2)
	}
	if data == nil {
		fmt.Fprintf(os.Stderr, "arch view: view %q not found in scope %q\n", viewID, scope)
		os.Exit(1)
	}
	prettyPrintJSON(json.RawMessage(data), archViewPretty)
	return nil
}
