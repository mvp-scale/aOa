// Package cmd — `aoa arch journey`, `aoa arch derive`, `aoa arch reach`, `aoa arch blast` (L19.16).
//
// derive: wraps MethodArchDerive (BFS shortest dep-path between two units).
// journey: stub — reserved for a future release.
// reach: CLI-only alias for derive (per ADR 2026-07-02).
// blast: CLI-only alias for findings scoped to the changeset (per ADR 2026-07-02).
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	archDeriveScope  string
	archDeriveK      int
	archDerivePretty bool
	archJourneyList  bool
)

// ── aoa arch journey ──────────────────────────────────────────────────────────

var archJourneyCmd = &cobra.Command{
	Use:           "journey [<id>|--list]",
	Short:         "Show an arch journey (stub — not yet implemented)",
	Long:          "arch.journey is reserved for a future release. It will list or show curated arch narratives.",
	RunE:          runArchJourney,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	archJourneyCmd.Flags().BoolVar(&archJourneyList, "list", false, "List available journeys")
}

func runArchJourney(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(os.Stderr, "arch journey: not yet implemented in this release")
	os.Exit(1)
	return nil
}

// ── aoa arch derive ───────────────────────────────────────────────────────────

var archDeriveCmd = &cobra.Command{
	Use:   "derive <from-path> <to-path> [k]",
	Short: "Find the shortest import-edge path from one unit to another",
	Long: `Derive the shortest dep-path (in unit IDs) between two units.

<from-path> and <to-path> are directory paths relative to the project root
(e.g. "internal/app", "internal/adapters/bbolt"). They are converted to unit
IDs internally.

Optional third positional arg [k] overrides the hop budget (default 10).

Exit codes:
  0 — path found
  1 — no path within k hops
  2 — operational error`,
	Args:          cobra.RangeArgs(2, 3),
	RunE:          runArchDerive,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	archDeriveCmd.Flags().StringVar(&archDeriveScope, "scope", archDefaultScope, "Scope to query")
	archDeriveCmd.Flags().IntVar(&archDeriveK, "k", 10, "Maximum hop budget")
	archDeriveCmd.Flags().BoolVar(&archDerivePretty, "pretty", false, "Pretty-print JSON output")
}

func runArchDerive(cmd *cobra.Command, args []string) error {
	fromPath := args[0]
	toPath := args[1]
	k := archDeriveK
	if len(args) == 3 {
		n, err := strconv.Atoi(args[2])
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "arch derive: invalid k %q: must be a positive integer\n", args[2])
			os.Exit(2)
		}
		k = n
	}

	fromID := archUnitSlug(fromPath)
	toID := archUnitSlug(toPath)

	scope := archDeriveScope
	if scope == "" {
		scope = archDefaultScope
	}

	return doDerive(scope, fromID, toID, k)
}

// doDerive is the shared implementation for derive/reach.
func doDerive(scope, fromID, toID string, k int) error {
	root := projectRoot()
	ctx, exitCode, err := resolveArch(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	// Daemon path.
	if ctx.socketClient != nil {
		result, err := ctx.socketClient.ArchDerive(scope, fromID, toID, k)
		ctx.closeStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "arch derive: %v\n", err)
			os.Exit(2)
		}
		if !result.Found {
			fmt.Fprintf(os.Stderr, "arch derive: no path found within %d hops\n", k)
			os.Exit(1)
		}
		out, _ := json.Marshal(result.Path)
		prettyPrintJSON(json.RawMessage(out), archDerivePretty)
		return nil
	}

	// Direct-RO path.
	path, err := ctx.querier.Derive(scope, fromID, toID, k)
	ctx.closeStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "arch derive: %v\n", err)
		os.Exit(2)
	}
	if path == nil {
		fmt.Fprintf(os.Stderr, "arch derive: no path found within %d hops\n", k)
		os.Exit(1)
	}
	out, _ := json.Marshal(path)
	prettyPrintJSON(json.RawMessage(out), archDerivePretty)
	return nil
}

// ── aoa arch reach (CLI-only alias for derive) ────────────────────────────────
// Per ADR 2026-07-02: reach is CLI sugar over MethodArchDerive. No 7th method.

var archReachCmd = &cobra.Command{
	Use:           "reach <from-path> [to-path]",
	Short:         "Alias for `arch derive` (reach from A to B)",
	Long:          "reach is a CLI-only alias for `aoa arch derive`. It maps to MethodArchDerive — no extra protocol method exists.",
	Args:          cobra.RangeArgs(1, 2),
	RunE:          runArchReach,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func runArchReach(cmd *cobra.Command, args []string) error {
	fromPath := args[0]
	toPath := ""
	if len(args) == 2 {
		toPath = args[1]
	}
	if toPath == "" {
		// With only one arg, show reachability from the source (all paths).
		// For now: require two args (to be enhanced in L19.23).
		fmt.Fprintln(os.Stderr, "arch reach: requires two paths: <from> <to>. Use `arch derive` for BFS.")
		os.Exit(2)
	}
	fromID := archUnitSlug(fromPath)
	toID := archUnitSlug(toPath)
	return doDerive(archDefaultScope, fromID, toID, 10)
}

// ── aoa arch blast (CLI-only alias for findings scoped to changeset) ──────────
// Per ADR 2026-07-02: blast maps to MethodArchFindings with a changeset scope.
// Full blast semantics (affected-set query) land in L22.6.

var archBlastCmd = &cobra.Command{
	Use:           "blast <ref>",
	Short:         "Alias for findings scoped to a changeset (stub for blast radius)",
	Long:          "blast is a CLI-only alias for `aoa arch findings` scoped to the changeset's affected set. Full semantics (affected-set query) land in L22.6; for now it prints findings for the default scope.",
	Args:          cobra.ExactArgs(1),
	RunE:          runArchBlast,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func runArchBlast(cmd *cobra.Command, args []string) error {
	// Stub: blast <ref> → findings for the default scope.
	// Full affected-set query lands in L22.6 (per ADR 2026-07-02).
	fmt.Fprintf(os.Stderr, "arch blast: full changeset-scoped blast lands in L22.6. Showing findings for scope %q.\n", archDefaultScope)

	root := projectRoot()
	ctx, exitCode, err := resolveArch(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(exitCode)
	}

	if ctx.socketClient != nil {
		result, sErr := ctx.socketClient.ArchFindings(archDefaultScope)
		ctx.closeStore()
		if sErr != nil {
			fmt.Fprintf(os.Stderr, "arch blast: %v\n", sErr)
			os.Exit(2)
		}
		prettyPrintJSON(result.Raw, false)
		return nil
	}

	data, fErr := ctx.querier.Findings(archDefaultScope)
	ctx.closeStore()
	if fErr != nil {
		fmt.Fprintf(os.Stderr, "arch blast: %v\n", fErr)
		os.Exit(2)
	}
	if data == nil {
		data = []byte("[]")
	}
	prettyPrintJSON(json.RawMessage(data), false)
	return nil
}
