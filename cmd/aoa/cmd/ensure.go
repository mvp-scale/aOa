package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/spf13/cobra"
)

// L21.1 — lazy-start: revive the daemon at point of use.
//
// Design (kickoff-L21 §5-7, F4 capture doc):
//   - revive-on-connect-error only — NEVER ping ahead of a dispatch, so the
//     healthy hot path pays zero cost (G0).
//   - flock on .aoa/run/daemon.lock dedups concurrent revivers: exactly one
//     daemon per project no matter how many callers race (E-2).
//   - one bounded stall, then a semantic result (D-U2a): spawnDaemon already
//     waits for the socket (15s cap); callers retry once after revive.
//   - only initialized projects revive (.aoa exists) — a bare directory never
//     grows a daemon.
//   - never-revive commands (health, daemon stop, remove, wipe, reset) simply
//     do not call this.

// canRevive reports whether this directory is an initialized aOa project.
func canRevive(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".aoa"))
	return err == nil
}

// reviveDaemon performs a flock-guarded lazy start after a connect failure.
// Returns true when the daemon is reachable (whether we revived it or a
// concurrent caller did). Never returns before the socket answers or the
// spawn has definitively failed.
func reviveDaemon(root, sockPath string) bool {
	if !canRevive(root) {
		return false
	}
	client := socket.NewClient(sockPath)
	if client.Ping() {
		return true // raced: already back up
	}

	paths := app.NewPaths(root)
	if err := paths.EnsureDirs(); err != nil {
		return false
	}
	lockFile, err := os.OpenFile(filepath.Join(paths.RunDir, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return false
	}
	defer lockFile.Close()

	// Exclusive lock: concurrent revivers queue here; the first spawns, the
	// rest find the socket alive on the double-check below.
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return false
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	if client.Ping() {
		return true // a concurrent caller revived while we waited on the lock
	}
	if _, err := spawnDaemon(root, sockPath); err != nil {
		return false
	}
	return client.Ping()
}

var daemonEnsureCmd = &cobra.Command{
	Use:           "ensure",
	Short:         "Start the daemon only if it is not running (flock-guarded; instant when alive)",
	RunE:          runDaemonEnsure,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// runDaemonEnsure is the fire-and-forget revive target (D-U1a): the status-line
// hook backgrounds `aoa daemon ensure` so a dead daemon comes back without any
// caller waiting on it.
func runDaemonEnsure(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)

	if socket.NewClient(sockPath).Ping() {
		return nil // alive — instant no-op
	}
	if !canRevive(root) {
		fmt.Fprintln(os.Stderr, "ensure: no .aoa project here — run: aoa init")
		os.Exit(1)
	}
	if !reviveDaemon(root, sockPath) {
		fmt.Fprintf(os.Stderr, "ensure: failed to revive daemon — check %s\n", app.NewPaths(root).DaemonLog)
		os.Exit(1)
	}
	fmt.Println("⚡ daemon revived")
	return nil
}
