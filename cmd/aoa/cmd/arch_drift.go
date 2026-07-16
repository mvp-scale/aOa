// Package cmd — `aoa arch drift` (GOV-1, board #40): the Angle-of-Attack CLI
// verb. Parses a declared .aoa target file, diffs it against the REAL fact
// substrate via kglab.DriftDiff, converts VIOLATIONs into ports.Finding, and
// gates on them with the SAME --new/--baseline convention `aoa arch findings`
// already uses (reusing loadBaseline/writeBaseline/newSinceBaseline from
// arch_findings.go — one baseline file, one exit-code convention, keyed by
// scope) instead of inventing a parallel findings-persistence path.
//
// Direct-RO ONLY — an intentional, documented asymmetry from every other
// `aoa arch *` subcommand (which are daemon-socket-first with a direct-RO
// fallback). Graph(), the richest real-facts read path, is NOT one of the 6
// wired MethodArch* socket methods (internal/adapters/socket/protocol.go),
// and D22 forbids adding a 7th just for drift. So this verb always opens the
// bbolt DB read-only directly, whether or not a daemon happens to be running
// (same substrate resolveArch's direct-RO fallback uses).
//
// Exit codes (mirrors the documented convention, this file's header comment
// in arch.go): 0 clean (or --baseline recorded), 1 violations present (or
// --new found a violation absent from the baseline), 2 operational error.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/app"
	"github.com/corey/aoa/internal/domain/arch"
	"github.com/corey/aoa/internal/kglab"
	"github.com/corey/aoa/internal/ports"
	"github.com/spf13/cobra"
)

var (
	archDriftJSON     bool
	archDriftPretty   bool
	archDriftNew      bool
	archDriftBaseline bool
	archDriftScope    string
)

var archDriftCmd = &cobra.Command{
	Use:   "drift <target.aoa>",
	Short: "Diff a declared .aoa target against the real import graph",
	Long: `drift parses a .aoa authoring file (estate/view/allow/forbid) and computes
the vector between where-the-code-IS (REAL, read from the fact substrate) and
where-you-declared-it-should-be (TARGET) — kglab's Angle of Attack.

  VIOLATION  a real import the target does not declare (actionable — carries
             the real file:line).
  MISSING    a declared import the code has not built yet (no file:line — it
             does not exist).

--new/--baseline mirror 'aoa arch findings': VIOLATIONs convert into the same
Finding shape and gate against a stored baseline
(.aoa/arch/findings-baseline.json, scope "drift:<estate-name>" unless --scope
overrides it).

Direct-RO only: this verb does not use the daemon socket (see package doc).`,
	Args:          cobra.ExactArgs(1),
	RunE:          runArchDrift,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	archDriftCmd.Flags().BoolVar(&archDriftJSON, "json", false, "Print the full DriftResult + findings as JSON")
	archDriftCmd.Flags().BoolVar(&archDriftPretty, "pretty", false, "Pretty-print JSON output")
	archDriftCmd.Flags().BoolVar(&archDriftNew, "new", false, "Exit 1 if a VIOLATION finding is absent from the stored baseline")
	archDriftCmd.Flags().BoolVar(&archDriftBaseline, "baseline", false, "Record current VIOLATION finding IDs as the baseline and exit 0")
	archDriftCmd.Flags().StringVar(&archDriftScope, "scope", "", `Baseline scope override (default: "drift:<estate-name>")`)
}

// driftResponse is the drift verb's JSON envelope — mirrors kglab's other
// Response types (internal/kglab/contract.go) but adds Findings, the GOV-1(b)
// bridge into the findings-pipeline shape.
type driftResponse struct {
	OK       bool              `json:"ok"`
	Verb     string            `json:"verb"`
	Caption  string            `json:"caption"`
	Result   kglab.DriftResult `json:"result"`
	Findings []ports.Finding   `json:"findings"`
}

// driftCompute is the pure computation behind `aoa arch drift`: parse the
// .aoa file, open the DB read-only, bridge real facts, diff, and convert
// violations to findings. Split out from runArchDrift (which owns os.Exit)
// so it is unit-testable without spawning a process (mirrors PC3's
// arch_findings_test.go convention).
func driftCompute(root, aoaPath string) (resp driftResponse, scope string, err error) {
	src, rerr := os.ReadFile(aoaPath)
	if rerr != nil {
		return resp, "", fmt.Errorf("read %s: %w", aoaPath, rerr)
	}
	spec, perr := kglab.ParseEstate(string(src))
	if perr != nil {
		return resp, "", fmt.Errorf("parse %s: %w", aoaPath, perr)
	}
	// .aoa authors write natural unit paths ("internal/app -> internal/domain
	// /arch"), but RealDepFactsFromStore's DepFacts key on arch.UnitSlug(path)
	// IDs (the same ID mint aggregateEdges/unitFactsFromFactStore use). This
	// boundary adaptation belongs here (D25), not in kglab.LoadTarget — kglab's
	// own fixtures (SampleGraph/SampleTarget) already pass pre-slugged IDs on
	// both sides and must not be double-slugged.
	slugged := make([]kglab.TargetFact, len(spec.Allowed))
	for i, t := range spec.Allowed {
		slugged[i] = kglab.TargetFact{Concept: t.Concept, FromUnit: arch.UnitSlug(t.FromUnit), ToUnit: arch.UnitSlug(t.ToUnit)}
	}
	target, terr := kglab.LoadTarget(spec.Name, slugged)
	if terr != nil {
		return resp, "", terr
	}

	paths := app.NewPaths(root)
	if _, serr := os.Stat(paths.DB); os.IsNotExist(serr) {
		return resp, "", fmt.Errorf("no facts substrate. Run: aoa init && aoa daemon start")
	}
	store, oerr := bbolt.NewReadOnlyStore(paths.DB)
	if oerr != nil {
		return resp, "", fmt.Errorf("open DB read-only: %w", oerr)
	}
	defer store.Close()

	projectID := filepath.Base(root)
	deps, derr := app.RealDepFactsFromStore(store, projectID, nil)
	if derr != nil {
		return resp, "", fmt.Errorf("read real facts: %w", derr)
	}
	real := kglab.FactSetFromDeps("real", deps)

	result := kglab.DriftDiff(real, target)
	scope = archDriftScope
	if scope == "" {
		scope = "drift:" + spec.Name
	}
	findings := app.DriftViolationFindings(scope, result)

	resp = driftResponse{
		OK:       true,
		Verb:     "drift",
		Caption:  driftCaption(result),
		Result:   result,
		Findings: findings,
	}
	return resp, scope, nil
}

func driftCaption(r kglab.DriftResult) string {
	return fmt.Sprintf("VECTOR %s vs %s: %d VIOLATION · %d MISSING · %d CONFORMANT",
		r.RealName, r.TargetName, r.Violations, r.Missing, r.Conformant)
}

func runArchDrift(cmd *cobra.Command, args []string) error {
	root := projectRoot()

	resp, scope, err := driftCompute(root, args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "arch drift: %v\n", err)
		os.Exit(2)
	}

	if archDriftJSON {
		b, jerr := kglab.RenderResponseJSON(resp)
		if jerr != nil {
			fmt.Fprintf(os.Stderr, "arch drift: json: %v\n", jerr)
			os.Exit(2)
		}
		prettyPrintJSON(json.RawMessage(b), archDriftPretty)
		fmt.Println()
	} else {
		fmt.Println(resp.Caption)
		for _, it := range resp.Result.Items {
			if it.Alignment == kglab.AlignConformant {
				continue
			}
			line := fmt.Sprintf("  %s  %s -> %s", it.Alignment, it.Fact.FromUnit, it.Fact.ToUnit)
			if it.Fact.File != "" {
				line += fmt.Sprintf("  (%s:%d)", it.Fact.File, it.Fact.Line)
			}
			fmt.Println(line)
		}
	}

	ids := make([]string, 0, len(resp.Findings))
	for i := range resp.Findings {
		ids = append(ids, resp.Findings[i].ID)
	}
	sort.Strings(ids)

	if archDriftBaseline {
		if werr := writeBaseline(root, scope, ids); werr != nil {
			fmt.Fprintf(os.Stderr, "arch drift: write baseline: %v\n", werr)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "arch drift: baseline recorded — %d violation(s) for scope %q\n", len(ids), scope)
		os.Exit(0)
	}

	if archDriftNew {
		base, haveBaseline, lerr := loadBaseline(root, scope)
		if lerr != nil {
			fmt.Fprintf(os.Stderr, "arch drift: %v\n", lerr)
			os.Exit(2)
		}
		if !haveBaseline {
			if len(ids) > 0 {
				fmt.Fprintf(os.Stderr,
					"arch drift --new: no baseline (%s) — all %d violation(s) are new; run `aoa arch drift <file.aoa> --baseline` (scope %q)\n",
					baselinePath(root), len(ids), scope)
				os.Exit(1)
			}
			return nil
		}
		newIDs := newSinceBaseline(base, ids)
		if len(newIDs) > 0 {
			fmt.Fprintf(os.Stderr, "arch drift --new: %d new violation(s) not in baseline\n", len(newIDs))
			os.Exit(1)
		}
		return nil
	}

	if resp.Result.Violations > 0 {
		os.Exit(1)
	}
	return nil
}
