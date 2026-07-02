package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:           "health",
	Short:         "Check daemon status (daemon/db/web reported independently)",
	RunE:          runHealth,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// staleStatusAge is how old status.json may be (with the daemon down) before
// it is flagged as stale — the 2026-06-21 failure mode: a dead daemon leaving
// plausible-looking metrics behind for days.
const staleStatusAge = 60 * time.Second

// runHealth reports tri-state health (L21.2). It NEVER revives the daemon —
// health must observe the dead state truthfully (never-revive list, kickoff §4).
// Exit codes: 0 = daemon up; 1 = daemon down (deliberate contract change from
// the old exit-0-on-dead behavior).
func runHealth(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if !client.Ping() {
		fmt.Print(formatHealthDown(root))
		os.Exit(1)
	}

	health, err := client.Health()
	if err != nil {
		return err
	}

	// Version-skew guard: an older daemon omits the tri-state fields, so
	// zero-value false would misreport "down". A response proves the daemon;
	// derive db/web from the Status it does carry.
	if !health.DaemonOK {
		health.DaemonOK = true
		derived := health.Status == "ok" || health.Status == "recovered"
		health.DBOK = derived
		health.WebOK = derived
	}

	fmt.Print(formatHealth(health))
	return nil
}

// formatHealthDown reports the dead-daemon state with independent best-effort
// probes: DB = file presence (unverified — no daemon to ask), Web = port-file
// + dial, plus the stale-status.json flag.
func formatHealthDown(root string) string {
	paths := app.NewPaths(root)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s⚡ aOa daemon%s\n", colorBold, colorReset))
	sb.WriteString(fmt.Sprintf("  Daemon:  %sdown%s\n", colorRed, colorReset))

	// DB: without the daemon we can only attest the file, not a transaction.
	if _, err := os.Stat(paths.DB); err == nil {
		sb.WriteString(fmt.Sprintf("  DB:      %spresent (unverified — daemon down)%s\n", colorYellow, colorReset))
	} else {
		sb.WriteString(fmt.Sprintf("  DB:      %smissing%s\n", colorRed, colorReset))
	}

	// Web: stale port file or dead listener both report down.
	webUp := false
	if portData, err := os.ReadFile(paths.PortFile); err == nil {
		addr := net.JoinHostPort("127.0.0.1", strings.TrimSpace(string(portData)))
		if conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
			conn.Close()
			webUp = true
		}
	}
	if webUp {
		sb.WriteString(fmt.Sprintf("  Web:     %sup (orphaned? daemon socket dead)%s\n", colorYellow, colorReset))
	} else {
		sb.WriteString(fmt.Sprintf("  Web:     %sdown%s\n", colorRed, colorReset))
	}

	// Stale status.json: the silent-outage tell.
	if info, err := os.Stat(filepath.Join(root, ".aoa", "status.json")); err == nil {
		if age := time.Since(info.ModTime()); age > staleStatusAge {
			sb.WriteString(fmt.Sprintf("  %s⚠ status.json is stale (%s old) — metrics shown elsewhere are from a dead daemon%s\n",
				colorYellow, age.Round(time.Minute), colorReset))
		}
	}

	sb.WriteString(fmt.Sprintf("  → start with: %saoa daemon start%s\n", colorCyan, colorReset))
	return sb.String()
}
