// Package cmd — `aoa arch findings` (L19.16, honest --new baseline in PC3).
//
// Exit codes:
//   - 0: success (when --new: no findings absent from the baseline)
//   - 1: --new and at least one finding is NOT in the stored baseline
//   - 2: operational error
//
// --new semantics (PC3 / ledger T46): `--new` is an honest set-difference against
// a stored baseline of finding IDs, not "any findings exist". Write the baseline
// with `--baseline`; re-running `--new` then exits 0 until a NEW finding (an ID
// absent from the baseline) appears. With no baseline file, every finding counts
// as new (exit 1 if any) and a clear message says so. This is a lightweight CI
// gate — it does NOT build L22.4's conformance machinery.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

var (
	archFindingsScope     string
	archFindingsNew       bool
	archFindingsWriteBase bool
	archFindingsSeverity  string
	archFindingsPretty    bool
)

var archFindingsCmd = &cobra.Command{
	Use:   "findings [--new] [--baseline] [--severity level] [--scope s]",
	Short: "List architectural findings (cycle, god, orphan, dead-code, …)",
	Long: `List arch findings for the project.

Findings are produced by detectors (Tarjan SCC, god-node, orphan, budget
overflow, dead-code candidate, DSM mutual pair) and persisted at compact time.

--baseline  Record the current finding IDs as the baseline (CI anchor) and
            exit 0. Written to .aoa/arch/findings-baseline.json, keyed by scope.

--new       Exit 1 when a finding is present now but absent from the stored
            baseline (an honest set-difference — the CI drift gate); exit 0 when
            every current finding is already baselined. With no baseline file on
            disk, all findings are treated as new (exit 1 if any) and a message
            on stderr says so. --new does NOT mean "any findings exist".`,
	RunE:          runArchFindings,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	archFindingsCmd.Flags().StringVar(&archFindingsScope, "scope", archDefaultScope, "Scope to query")
	archFindingsCmd.Flags().BoolVar(&archFindingsNew, "new", false, "Exit 1 if a finding is absent from the stored baseline (CI drift gate)")
	archFindingsCmd.Flags().BoolVar(&archFindingsWriteBase, "baseline", false, "Record current findings as the baseline and exit 0")
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

	// Fetch the findings JSON from whichever substrate is live. Both paths
	// converge on the same raw []Finding bytes so the baseline diff is identical
	// daemon-side and direct-RO-side.
	var raw json.RawMessage
	if ctx.socketClient != nil {
		result, err := ctx.socketClient.ArchFindings(scope)
		ctx.closeStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "arch findings: %v\n", err)
			os.Exit(2)
		}
		raw = result.Raw
	} else {
		data, err := ctx.querier.Findings(scope)
		ctx.closeStore()
		if err != nil {
			fmt.Fprintf(os.Stderr, "arch findings: %v\n", err)
			os.Exit(2)
		}
		raw = json.RawMessage(data)
	}
	if len(raw) == 0 {
		raw = json.RawMessage("[]")
	}

	if archFindingsSeverity != "" {
		raw = filterBySeverity(raw, archFindingsSeverity)
	}

	handleFindings(root, scope, raw)
	return nil
}

// filterBySeverity keeps only findings whose severity matches. When severity is
// empty the caller skips this and raw passes through byte-identical.
func filterBySeverity(raw json.RawMessage, severity string) json.RawMessage {
	var findings []json.RawMessage
	if err := json.Unmarshal(raw, &findings); err != nil {
		return raw // malformed → leave untouched
	}
	kept := make([]json.RawMessage, 0, len(findings))
	for _, f := range findings {
		var probe struct {
			Severity string `json:"severity"`
		}
		if json.Unmarshal(f, &probe) == nil && probe.Severity == severity {
			kept = append(kept, f)
		}
	}
	out, err := json.Marshal(kept)
	if err != nil {
		return raw
	}
	return out
}

// findingIDs parses the raw []Finding JSON and returns the sorted set of IDs.
func findingIDs(raw json.RawMessage) []string {
	var findings []struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &findings) // best-effort; empty on malformed
	ids := make([]string, 0, len(findings))
	for _, f := range findings {
		if f.ID != "" {
			ids = append(ids, f.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

// newSinceBaseline returns the finding IDs present now (ids) but absent from the
// baseline set — the honest --new set-difference (pure; unit-testable).
func newSinceBaseline(base map[string]bool, ids []string) []string {
	var out []string
	for _, id := range ids {
		if !base[id] {
			out = append(out, id)
		}
	}
	return out
}

// findingsBaseline is the on-disk baseline: finding IDs keyed by scope.
type findingsBaseline struct {
	Scopes map[string][]string `json:"scopes"`
}

// baselinePath returns the baseline file location for a project root.
func baselinePath(root string) string {
	return filepath.Join(root, ".aoa", "arch", "findings-baseline.json")
}

// loadBaseline reads the baseline file. Returns (nil, false, nil) when the file
// does not exist (the "no baseline" case), (set, true, nil) when present.
func loadBaseline(root, scope string) (map[string]bool, bool, error) {
	data, err := os.ReadFile(baselinePath(root))
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var b findingsBaseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, false, fmt.Errorf("parse baseline %s: %w", baselinePath(root), err)
	}
	set := make(map[string]bool)
	for _, id := range b.Scopes[scope] {
		set[id] = true
	}
	// The file exists → treat as a baseline even if this scope has no entry
	// (an empty entry means "clean baseline for this scope").
	return set, true, nil
}

// writeBaseline records the current finding IDs for scope, merging with any
// existing per-scope entries so recording one scope never drops another.
func writeBaseline(root, scope string, ids []string) error {
	path := baselinePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b := findingsBaseline{Scopes: map[string][]string{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &b) // preserve other scopes; ignore parse errors
		if b.Scopes == nil {
			b.Scopes = map[string][]string{}
		}
	}
	b.Scopes[scope] = ids
	out, err := json.MarshalIndent(&b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// handleFindings prints the findings JSON and applies --baseline / --new.
func handleFindings(root, scope string, raw json.RawMessage) {
	ids := findingIDs(raw)

	// --baseline: record and exit 0 (do not also gate).
	if archFindingsWriteBase {
		if err := writeBaseline(root, scope, ids); err != nil {
			fmt.Fprintf(os.Stderr, "arch findings: write baseline: %v\n", err)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "arch findings: baseline recorded — %d findings for scope %q\n", len(ids), scope)
		os.Exit(0)
	}

	prettyPrintJSON(raw, archFindingsPretty)

	if !archFindingsNew {
		return // no gate requested
	}

	base, haveBaseline, err := loadBaseline(root, scope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arch findings: %v\n", err)
		os.Exit(2)
	}

	if !haveBaseline {
		// No baseline on disk → every finding is new.
		if len(ids) > 0 {
			fmt.Fprintf(os.Stderr,
				"arch findings --new: no baseline (%s) — all %d findings are new; run `aoa arch findings --baseline` to record\n",
				baselinePath(root), len(ids))
			os.Exit(1)
		}
		return // no findings at all → clean
	}

	// Honest set-difference: IDs present now but absent from the baseline.
	newIDs := newSinceBaseline(base, ids)
	if len(newIDs) > 0 {
		fmt.Fprintf(os.Stderr, "arch findings --new: %d new finding(s) not in baseline: %v\n", len(newIDs), newIDs)
		os.Exit(1)
	}
	// exit 0: every current finding is already baselined.
}
