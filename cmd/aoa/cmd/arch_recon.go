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
	"path/filepath"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/corey/aoa/internal/domain/arch"
	"github.com/spf13/cobra"
)

// reconDaemon is the slice of socket.Client that recon's invalidation needs.
// *socket.Client satisfies it; tests inject a fake.
type reconDaemon interface {
	Ping() bool
	Reindex() (*socket.ReindexResult, error)
}

// invalidateArchShards drops derived arch views so the next derive rebuilds
// with the fresh footprint grain. Returns a one-line human summary.
//
// Daemon-first (mirrors resolveArch): a running daemon holds the bbolt lock,
// so it must do the rebuild itself — Reindex re-extracts edges and fires
// deriveArch, which reads footprint.json fresh from disk and bumps the
// revision (open viewers ETag-refresh). Reindex is an existing admin method;
// the six-MethodArch* protocol surface (ADR 2026-07-02) is untouched.
// Daemon down: delete the derived views directly; the boot-derive trigger
// (manifest absent) rebuilds them on the next daemon start or lazy revive.
// No DB: nothing to invalidate — recon stays DB-less and never creates one.
func invalidateArchShards(root string, daemon reconDaemon) (string, error) {
	if daemon.Ping() {
		if _, err := daemon.Reindex(); err != nil {
			return "", fmt.Errorf("daemon reindex: %w", err)
		}
		return "daemon reindexing — views re-derive with the new grain", nil
	}

	dbPath := app.NewPaths(root).DB
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return "no fact database yet — first derive will use the new grain", nil
	}
	store, err := bbolt.NewStore(dbPath)
	if err != nil {
		return "", fmt.Errorf("open DB to invalidate views: %w", err)
	}
	defer store.Close()
	if err := store.DeleteShardsForScope(filepath.Base(root), archDefaultScope); err != nil {
		return "", fmt.Errorf("invalidate views: %w", err)
	}
	return "stale views invalidated — next daemon start re-derives with the new grain", nil
}

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

	// The footprint changed (or may have): derived views built with the old
	// grain are now stale and must not keep serving. Failure here is surfaced
	// loudly but does not fail recon — the footprint (the primary output) is
	// already written; the user is told exactly what did NOT happen.
	msg, err := invalidateArchShards(root, socket.NewClient(socket.SocketPath(root)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "arch recon: views NOT refreshed (%v) — run 'aoa reindex' or restart the daemon\n", err)
		return nil
	}
	fmt.Println(msg)
	return nil
}
