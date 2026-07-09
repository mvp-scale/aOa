// Package cmd — `aoa arch recon` (consensus 2026-07-08, §5 v1 cut-line).
//
// recon runs the deterministic, network-never footprint detector over the repo,
// writes {ProjectRoot}/.aoa/arch/footprint.json, and prints a one-line human
// summary. It is re-runnable (refreshes the cached footprint) and needs neither
// the daemon nor the fact DB — it is a pure single tree walk (Layers 0–1b).
//
// The footprint's anchor grain repairs the scrapy §10 failure: subpackages one
// level down (scrapy/core, scrapy/http, …) stop collapsing into a single box.
package cmd

import (
	"fmt"
	"os"

	"github.com/corey/aoa/internal/domain/arch"
	"github.com/spf13/cobra"
)

var archReconCmd = &cobra.Command{
	Use:   "recon",
	Short: "Detect the repo's architectural footprint (anchor + grouping grain)",
	Long: `recon runs a deterministic, network-never pass over the repository to find
where its architecture actually lives — the manifest/unit markers (go.mod,
setup.py, package.json, Cargo.toml, …) and the grouping grain (the path depth at
which sibling directories are the real architecture).

It writes .aoa/arch/footprint.json and prints a one-line summary. Re-run any time
to refresh. No daemon or index required. Network is never touched.`,
	RunE:          runArchRecon,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func runArchRecon(cmd *cobra.Command, args []string) error {
	root := projectRoot()

	fp, err := arch.DetectFootprint(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arch recon: %v\n", err)
		os.Exit(2)
	}

	// v1 (ruling D): the Haiku --refine layer is NOT wired. The deterministic
	// footprint is complete on its own; DeterministicRefiner is the no-op seam.
	fp, _ = arch.DeterministicRefiner{}.RefineFootprint(fp)

	if err := arch.SaveFootprint(root, fp); err != nil {
		fmt.Fprintf(os.Stderr, "arch recon: %v\n", err)
		os.Exit(2)
	}

	nGroups := ""
	if g := fp.PrimaryGrain(); g != nil && g.Mode == "descend" {
		nGroups = " (subpackages become distinct groups)"
	}
	fmt.Printf("%s%s\n", fp.Summary(), nGroups)
	fmt.Printf("wrote %s\n", arch.FootprintPath(root))
	return nil
}
