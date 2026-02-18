package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/app"
	"github.com/spf13/cobra"
)

// daemonChildEnv is set in the background child process to distinguish it
// from the parent (which spawns the child and returns).
const daemonChildEnv = "AOA_DAEMON_CHILD"

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the aOa daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon in the background",
	RunE:  runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	RunE:  runDaemonStop,
}

var daemonRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the daemon (stop + start)",
	RunE:  runDaemonRestart,
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonRestartCmd)
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)

	// Check if already running.
	client := socket.NewClient(sockPath)
	if client.Ping() {
		fmt.Println("⚡ daemon already running")
		return nil
	}

	// If this IS the background child process, run the blocking daemon loop.
	if os.Getenv(daemonChildEnv) == "1" {
		return runDaemonLoop(root, sockPath)
	}

	// Parent: spawn the daemon as a detached background process.
	return spawnDaemon(root, sockPath)
}

func runDaemonRestart(cmd *cobra.Command, args []string) error {
	// Stop the running daemon (ignores "not running" — idempotent).
	_ = runDaemonStop(cmd, args)

	// Brief pause to let the socket file be cleaned up.
	time.Sleep(200 * time.Millisecond)

	// Start a fresh daemon.
	return runDaemonStart(cmd, args)
}

// spawnDaemon re-execs the current binary as a background process with
// stdout/stderr directed to .aoa/daemon.log. It waits until the socket
// becomes reachable (or the child exits early) before returning.
func spawnDaemon(root, sockPath string) error {
	aoaDir := filepath.Join(root, ".aoa")
	if err := os.MkdirAll(aoaDir, 0755); err != nil {
		return fmt.Errorf("create .aoa dir: %w", err)
	}

	logPath := filepath.Join(aoaDir, "daemon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open daemon log: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		logFile.Close()
		return fmt.Errorf("resolve executable path: %w", err)
	}

	child := exec.Command(exePath, "daemon", "start")
	child.Dir = root
	child.Env = append(os.Environ(), daemonChildEnv+"=1")
	child.Stdout = logFile
	child.Stderr = logFile
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := child.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("spawn daemon: %w", err)
	}

	pid := child.Process.Pid

	// Write PID file so `daemon stop` can find orphaned processes.
	pidPath := filepath.Join(aoaDir, "daemon.pid")
	os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", pid)), 0644)

	// Watch for early child exit (e.g., lock contention, init failure).
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()

	// Parent no longer needs the log fd — child inherited its own copy.
	logFile.Close()

	// Poll until the socket is reachable or the child dies.
	client := socket.NewClient(sockPath)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-exited:
			// Child exited early — read the log tail so the user sees why.
			detail := readLogTail(logPath, 10)
			if detail != "" {
				return fmt.Errorf("daemon failed to start:\n%s", detail)
			}
			return fmt.Errorf("daemon failed to start\n  → check log: %s", logPath)
		default:
		}
		if client.Ping() {
			fmt.Printf("⚡ daemon started (pid %d)\n", pid)
			fmt.Printf("  log: %s\n", logPath)
			// Show dashboard URL if HTTP port file exists
			httpPortPath := filepath.Join(aoaDir, "http.port")
			if portData, err := os.ReadFile(httpPortPath); err == nil {
				fmt.Printf("  dashboard: http://localhost:%s\n", strings.TrimSpace(string(portData)))
			}
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon not responding after start\n  → check log: %s", logPath)
}

// readLogTail returns the last n lines of a file, or empty string on error.
func readLogTail(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// runDaemonLoop is the blocking entry point for the background child process.
// It opens the database, starts the socket server, and blocks until a signal
// or remote shutdown arrives. All output goes to .aoa/daemon.log.
func runDaemonLoop(root, sockPath string) error {
	fmt.Printf("[%s] daemon starting\n", time.Now().Format(time.RFC3339))

	a, err := app.New(app.Config{ProjectRoot: root})
	if err != nil {
		if isDBLockError(err) {
			fmt.Printf("[%s] error: %s\n", time.Now().Format(time.RFC3339), diagnoseDBLock(root))
			return fmt.Errorf("cannot start daemon: %s", diagnoseDBLock(root))
		}
		fmt.Printf("[%s] error: %v\n", time.Now().Format(time.RFC3339), err)
		return fmt.Errorf("init: %w", err)
	}

	if err := a.Start(); err != nil {
		fmt.Printf("[%s] error: %v\n", time.Now().Format(time.RFC3339), err)
		return err
	}

	fmt.Printf("[%s] daemon ready at %s\n", time.Now().Format(time.RFC3339), sockPath)
	if a.WebServer.Port() > 0 {
		fmt.Printf("[%s] dashboard at %s\n", time.Now().Format(time.RFC3339), a.WebServer.URL())
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		fmt.Printf("[%s] received %s, shutting down\n", time.Now().Format(time.RFC3339), sig)
	case <-a.Server.ShutdownCh():
		fmt.Printf("[%s] remote stop, shutting down\n", time.Now().Format(time.RFC3339))
	}

	err = a.Stop()

	// Clean up PID file.
	pidPath := filepath.Join(root, ".aoa", "daemon.pid")
	os.Remove(pidPath)

	fmt.Printf("[%s] daemon stopped\n", time.Now().Format(time.RFC3339))
	return err
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	sockPath := socket.SocketPath(root)
	pidPath := filepath.Join(root, ".aoa", "daemon.pid")
	client := socket.NewClient(sockPath)

	// Socket is reachable — send a graceful shutdown request.
	if client.Ping() {
		if err := client.Shutdown(); err != nil {
			return fmt.Errorf("shutdown request failed: %w", err)
		}

		// Verify the daemon actually stopped.
		for i := 0; i < 30; i++ {
			if !client.Ping() {
				fmt.Println("⚡ daemon stopped")
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}

		return fmt.Errorf("daemon did not stop within 3 seconds\n" +
			"  → find the process:  ps aux | grep 'aoa daemon'\n" +
			"  → kill it:           kill <PID>")
	}

	// Socket not reachable — try PID file to find an orphaned daemon.
	if pid, err := readPID(pidPath); err == nil {
		if processAlive(pid) {
			syscall.Kill(pid, syscall.SIGTERM)
			for i := 0; i < 30; i++ {
				if !processAlive(pid) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if processAlive(pid) {
				syscall.Kill(pid, syscall.SIGKILL)
				time.Sleep(200 * time.Millisecond)
			}
			os.Remove(pidPath)
			os.Remove(sockPath)
			fmt.Printf("⚡ daemon stopped (pid %d)\n", pid)
			return nil
		}
		// PID file exists but process is dead — clean up stale files.
		os.Remove(pidPath)
		os.Remove(sockPath)
		fmt.Println("⚡ cleaned up stale daemon files")
		return nil
	}

	// Check for stale socket with no PID file.
	if _, err := os.Stat(sockPath); err == nil {
		os.Remove(sockPath)
		fmt.Println("⚡ removed stale daemon socket")
		fmt.Println("  → if a process is still running: ps aux | grep 'aoa daemon'")
		return nil
	}

	fmt.Println("⚡ daemon is not running")
	return nil
}

// readPID reads and parses the .aoa/daemon.pid file.
func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

// processAlive checks if a process with the given PID exists.
// Uses signal 0 which tests existence without delivering a signal.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
