package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/corey/aoa/internal/adapters/socket"
)

// isDBLockError returns true if the error chain contains a bbolt lock timeout.
// bbolt returns the string "timeout" when it cannot acquire the file lock
// within the configured deadline.
func isDBLockError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "timeout")
}

// diagnoseDBLock checks the daemon state and returns actionable guidance
// when a bbolt open fails due to lock contention. It distinguishes three
// scenarios: daemon running, stale socket, and unknown lock holder.
func diagnoseDBLock(root string) string {
	sockPath := socket.SocketPath(root)
	client := socket.NewClient(sockPath)

	if client.Ping() {
		return "database is locked by the running daemon\n" +
			"  → stop it first:  aoa daemon stop\n" +
			"  → then retry your command"
	}

	if _, err := os.Stat(sockPath); err == nil {
		return fmt.Sprintf("database is locked — daemon socket exists but is not responding\n"+
			"  → a previous daemon may have crashed\n"+
			"  → find the process:  ps aux | grep 'aoa daemon'\n"+
			"  → kill it:           kill <PID>\n"+
			"  → clean up socket:   rm %s", sockPath)
	}

	return "database is locked by another process\n" +
		"  → find the process:  ps aux | grep 'aoa'\n" +
		"  → kill it:           kill <PID>\n" +
		"  → then retry your command"
}
