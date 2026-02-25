package integration

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// aoaBin is the path to the compiled binary, set by TestMain.
var aoaBin string

func TestMain(m *testing.M) {
	// Build binary once for all tests.
	tmp, err := os.MkdirTemp("", "aoa-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	aoaBin = filepath.Join(tmp, "aoa")
	cmd := exec.Command("go", "build", "-o", aoaBin, "./cmd/aoa/")
	cmd.Dir = findModuleRoot()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// =============================================================================
// Helpers
// =============================================================================

// findModuleRoot walks up from cwd to find go.mod.
func findModuleRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("go.mod not found")
		}
		dir = parent
	}
}

// setupProject creates a temp dir with small .go files for tree-sitter to parse.
func setupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "main.go"), `package main

func main() {
	hello()
}
`)
	writeFile(t, filepath.Join(dir, "hello.go"), `package main

import "fmt"

func hello() {
	fmt.Println("hello world")
}

type Config struct {
	Name    string
	Value   int
}
`)
	writeFile(t, filepath.Join(dir, "util.go"), `package main

func add(a, b int) int {
	return a + b
}

func multiply(x, y int) int {
	return x * y
}
`)
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// runAOA executes the aoa binary in the given dir with args, returns stdout, stderr, exit code.
func runAOA(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(aoaBin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1")

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("exec error (not ExitError): %v", err)
		}
	}
	return
}

// startDaemon runs `aoa daemon start` which daemonizes and returns once ready.
// Returns a cleanup func that stops the daemon.
func startDaemon(t *testing.T, dir string) func() {
	t.Helper()

	stdout, stderr, exit := runAOA(t, dir, "daemon", "start")
	if exit != 0 {
		t.Fatalf("daemon start failed: exit %d\nstdout: %s\nstderr: %s", exit, stdout, stderr)
	}

	return func() {
		// Graceful stop.
		runAOA(t, dir, "daemon", "stop")
		// Safety net: force-kill via PID file if still alive.
		pidFile := filepath.Join(dir, ".aoa", "run", "daemon.pid")
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	}
}

// socketPathForDir computes the expected socket path for a directory.
// Replicates internal/adapters/socket.SocketPath logic.
func socketPathForDir(dir string) string {
	abs, _ := filepath.Abs(dir)
	h := sha256.Sum256([]byte(abs))
	return fmt.Sprintf("/tmp/aoa-%x.sock", h[:6])
}

// holdDBLock uses flock(1) to hold an exclusive lock on the bbolt file,
// simulating an orphaned process. Returns cleanup func.
func holdDBLock(t *testing.T, dbPath string) func() {
	t.Helper()
	cmd := exec.Command("flock", "-x", dbPath, "-c", "sleep 60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("flock: %v", err)
	}
	// Give flock time to acquire the lock.
	time.Sleep(200 * time.Millisecond)
	return func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}
}

// =============================================================================
// V-05: Standalone commands (no daemon needed)
// =============================================================================

func TestTree_Basic(t *testing.T) {
	dir := setupProject(t)
	stdout, _, exit := runAOA(t, dir, "tree")
	if exit != 0 {
		t.Fatalf("exit %d", exit)
	}
	for _, want := range []string{"main.go", "hello.go", "util.go"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("tree output missing %q:\n%s", want, stdout)
		}
	}
}

func TestTree_Depth(t *testing.T) {
	dir := setupProject(t)
	writeFile(t, filepath.Join(dir, "sub", "deep", "nested.go"), "package deep\n")

	stdout, _, exit := runAOA(t, dir, "tree", "--depth", "1")
	if exit != 0 {
		t.Fatalf("exit %d", exit)
	}
	if !strings.Contains(stdout, "sub/") {
		t.Errorf("should show sub/ at depth 1:\n%s", stdout)
	}
	if strings.Contains(stdout, "deep/") {
		t.Errorf("should NOT show deep/ at depth 1:\n%s", stdout)
	}
}

func TestConfig_Basic(t *testing.T) {
	dir := setupProject(t)
	stdout, _, exit := runAOA(t, dir, "config")
	if exit != 0 {
		t.Fatalf("exit %d", exit)
	}
	for _, want := range []string{"Project:", "Root:", "DB:", "Socket:", "Daemon:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("config missing %q:\n%s", want, stdout)
		}
	}
}

func TestConfig_NoDaemon(t *testing.T) {
	dir := setupProject(t)
	stdout, _, _ := runAOA(t, dir, "config")
	if !strings.Contains(stdout, "not running") {
		t.Errorf("should show 'not running':\n%s", stdout)
	}
}

// =============================================================================
// V-06: Init command — happy path, edge cases, lock detection
// =============================================================================

func TestInit_HappyPath(t *testing.T) {
	dir := setupProject(t)
	stdout, _, exit := runAOA(t, dir, "init")
	if exit != 0 {
		t.Fatalf("init exit %d, stdout: %s", exit, stdout)
	}
	if !strings.Contains(stdout, "indexed") {
		t.Errorf("should say 'indexed':\n%s", stdout)
	}
	// DB must exist.
	if _, err := os.Stat(filepath.Join(dir, ".aoa", "aoa.db")); os.IsNotExist(err) {
		t.Error(".aoa/aoa.db not created")
	}
}

func TestInit_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exit := runAOA(t, dir, "init")
	if exit != 0 {
		t.Fatalf("init on empty dir exit %d", exit)
	}
	if !strings.Contains(stdout, "0 files") {
		t.Errorf("should mention '0 files':\n%s", stdout)
	}
}

func TestInit_Reinit(t *testing.T) {
	dir := setupProject(t)
	// First init.
	_, _, exit1 := runAOA(t, dir, "init")
	if exit1 != 0 {
		t.Fatalf("first init failed")
	}
	// Second init should succeed (re-index).
	stdout, _, exit2 := runAOA(t, dir, "init")
	if exit2 != 0 {
		t.Fatalf("second init exit %d", exit2)
	}
	if !strings.Contains(stdout, "indexed") {
		t.Errorf("re-init should say 'indexed':\n%s", stdout)
	}
}

func TestInit_DaemonRunning_DelegatesToReindex(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// Init while daemon is running should succeed by delegating reindex via socket.
	stdout, _, exit := runAOA(t, dir, "init")
	if exit != 0 {
		t.Fatalf("init should succeed when daemon is running (delegates via socket), exit %d", exit)
	}
	if !strings.Contains(stdout, "indexed") {
		t.Errorf("should say 'indexed':\n%s", stdout)
	}
	if !strings.Contains(stdout, "Daemon running") {
		t.Errorf("should indicate delegation to daemon:\n%s", stdout)
	}
}

func TestInit_LockedDB_OrphanedProcess(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	// Start daemon, then remove socket to simulate orphaned process.
	// The daemon child still holds the bbolt lock.
	cleanup := startDaemon(t, dir)
	defer cleanup()
	sockPath := socketPathForDir(dir)
	os.Remove(sockPath)

	start := time.Now()
	_, stderr, exit := runAOA(t, dir, "init")
	elapsed := time.Since(start)

	if exit == 0 {
		t.Fatal("init should fail when DB is locked")
	}
	if elapsed > 3*time.Second {
		t.Errorf("should fail fast (<3s), took %v", elapsed)
	}
	// Error should mention "another process" since socket is gone.
	if !strings.Contains(stderr, "locked") {
		t.Errorf("error should mention 'locked':\n%s", stderr)
	}
	if !strings.Contains(stderr, "process") {
		t.Errorf("error should mention 'process':\n%s", stderr)
	}
}

func TestInit_LockedDB_ExternalProcess(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	// Hold the DB lock via flock (simulates any external process).
	dbPath := filepath.Join(dir, ".aoa", "aoa.db")
	release := holdDBLock(t, dbPath)
	defer release()

	start := time.Now()
	_, stderr, exit := runAOA(t, dir, "init")
	elapsed := time.Since(start)

	if exit == 0 {
		t.Fatal("init should fail when DB is locked by external process")
	}
	if elapsed > 3*time.Second {
		t.Errorf("should fail fast (<3s), took %v", elapsed)
	}
	if !strings.Contains(stderr, "locked") {
		t.Errorf("error should mention 'locked':\n%s", stderr)
	}
}

// =============================================================================
// V-07: Wipe command — direct, via daemon, locked, no data
// =============================================================================

func TestWipe_Direct(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	stdout, _, exit := runAOA(t, dir, "wipe", "--force")
	if exit != 0 {
		t.Fatalf("wipe exit %d", exit)
	}
	if !strings.Contains(stdout, "wiped") {
		t.Errorf("should say 'wiped':\n%s", stdout)
	}
}

func TestWipe_ViaDaemon(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	stdout, _, exit := runAOA(t, dir, "wipe", "--force")
	if exit != 0 {
		t.Fatalf("wipe via daemon exit %d", exit)
	}
	if !strings.Contains(stdout, "wiped") {
		t.Errorf("should say 'wiped':\n%s", stdout)
	}
	if !strings.Contains(stdout, "daemon") {
		t.Errorf("should indicate wipe went via daemon:\n%s", stdout)
	}
}

func TestWipe_NoData(t *testing.T) {
	dir := t.TempDir()
	stdout, _, exit := runAOA(t, dir, "wipe", "--force")
	if exit != 0 {
		t.Fatalf("wipe on fresh dir exit %d", exit)
	}
	if !strings.Contains(stdout, "no data to wipe") {
		t.Errorf("should say 'no data to wipe':\n%s", stdout)
	}
}

func TestWipe_LockedDB_OrphanedProcess(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	// Start daemon, remove socket → orphaned lock.
	cleanup := startDaemon(t, dir)
	defer cleanup()
	os.Remove(socketPathForDir(dir))

	start := time.Now()
	_, stderr, exit := runAOA(t, dir, "wipe", "--force")
	elapsed := time.Since(start)

	if exit == 0 {
		t.Fatal("wipe should fail when DB is locked")
	}
	if elapsed > 3*time.Second {
		t.Errorf("should fail fast, took %v", elapsed)
	}
	if !strings.Contains(stderr, "locked") {
		t.Errorf("error should mention 'locked':\n%s", stderr)
	}
}

// =============================================================================
// V-08: Daemon lifecycle — thorough state machine testing
// =============================================================================

func TestDaemon_StartStop(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)

	// Socket should exist.
	sockPath := socketPathForDir(dir)
	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		t.Error("socket file not created after start")
	}

	// PID file should exist.
	if _, err := os.Stat(filepath.Join(dir, ".aoa", "run", "daemon.pid")); os.IsNotExist(err) {
		t.Error("PID file not created after start")
	}

	// Health should work.
	stdout, _, exit := runAOA(t, dir, "health")
	if exit != 0 {
		t.Fatalf("health exit %d", exit)
	}
	if !strings.Contains(stdout, "Files:") {
		t.Errorf("health should show file count:\n%s", stdout)
	}

	// Stop via cleanup.
	cleanup()

	// After stop, socket should be gone.
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(sockPath); err == nil {
		t.Error("socket file should be removed after stop")
	}

	// Health should say not running.
	stdout, _, _ = runAOA(t, dir, "health")
	if !strings.Contains(stdout, "not running") {
		t.Errorf("health should say 'not running' after stop:\n%s", stdout)
	}
}

func TestDaemon_RemoteStop(t *testing.T) {
	// Verify `daemon stop` terminates the process, releases the DB lock,
	// and cleans up the PID file.
	dir := setupProject(t)
	runAOA(t, dir, "init")

	startOut, _, _ := runAOA(t, dir, "daemon", "start")
	if !strings.Contains(startOut, "daemon started") {
		t.Fatalf("start should succeed:\n%s", startOut)
	}

	// Read PID to verify the process actually dies.
	pidData, err := os.ReadFile(filepath.Join(dir, ".aoa", "run", "daemon.pid"))
	if err != nil {
		t.Fatalf("read PID file: %v", err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(pidData)))

	// Stop via remote command.
	stopOut, _, stopExit := runAOA(t, dir, "daemon", "stop")
	if stopExit != 0 {
		t.Fatalf("daemon stop exit %d", stopExit)
	}
	if !strings.Contains(stopOut, "stopped") {
		t.Errorf("should say 'stopped':\n%s", stopOut)
	}

	// Process should be dead.
	time.Sleep(500 * time.Millisecond)
	proc, _ := os.FindProcess(pid)
	if proc.Signal(syscall.Signal(0)) == nil {
		t.Error("daemon process should be dead after remote stop")
	}

	// Socket and PID file should be gone.
	sockPath := socketPathForDir(dir)
	if _, err := os.Stat(sockPath); err == nil {
		t.Error("socket should be removed after remote stop")
	}
	if _, err := os.Stat(filepath.Join(dir, ".aoa", "run", "daemon.pid")); err == nil {
		t.Error("PID file should be removed after remote stop")
	}

	// DB should be unlocked — init should work.
	stdout, _, exit := runAOA(t, dir, "init")
	if exit != 0 {
		t.Fatalf("init after remote stop should succeed, exit %d: %s", exit, stdout)
	}
}

func TestDaemon_DoubleStart(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	stdout, _, exit := runAOA(t, dir, "daemon", "start")
	if exit != 0 {
		t.Logf("double start exit %d (non-fatal)", exit)
	}
	if !strings.Contains(stdout, "already running") {
		t.Errorf("should say 'already running':\n%s", stdout)
	}
}

func TestDaemon_StopNotRunning(t *testing.T) {
	dir := setupProject(t)
	stdout, _, exit := runAOA(t, dir, "daemon", "stop")
	if exit != 0 {
		t.Fatalf("stop (not running) exit %d", exit)
	}
	if !strings.Contains(stdout, "not running") {
		t.Errorf("should say 'not running':\n%s", stdout)
	}
}

func TestDaemon_StopStaleSocket(t *testing.T) {
	dir := setupProject(t)
	sockPath := socketPathForDir(dir)

	// Create a stale socket file (no actual listener).
	if err := os.WriteFile(sockPath, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(sockPath)

	stdout, _, exit := runAOA(t, dir, "daemon", "stop")
	if exit != 0 {
		t.Fatalf("stop (stale socket) exit %d", exit)
	}
	if !strings.Contains(stdout, "stale") {
		t.Errorf("should mention 'stale':\n%s", stdout)
	}
	// Stale socket should be removed.
	if _, err := os.Stat(sockPath); err == nil {
		t.Error("stale socket should be removed")
	}
}

func TestDaemon_StartStopStart(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	// First start/stop cycle.
	startDaemon(t, dir)
	runAOA(t, dir, "daemon", "stop")

	// Wait for full cleanup.
	time.Sleep(500 * time.Millisecond)

	// Second start should succeed — DB lock is released, PID file cleaned up.
	cleanup2 := startDaemon(t, dir)
	defer cleanup2()

	stdout, _, exit := runAOA(t, dir, "health")
	if exit != 0 {
		t.Fatalf("health after restart exit %d", exit)
	}
	if !strings.Contains(stdout, "Files:") {
		t.Errorf("health should work after restart:\n%s", stdout)
	}
}

func TestDaemon_StartLockedDB(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	// Hold the lock externally.
	dbPath := filepath.Join(dir, ".aoa", "aoa.db")
	release := holdDBLock(t, dbPath)
	defer release()

	start := time.Now()
	_, stderr, exit := runAOA(t, dir, "daemon", "start")
	elapsed := time.Since(start)

	if exit == 0 {
		t.Fatal("daemon start should fail when DB is locked")
	}
	if elapsed > 3*time.Second {
		t.Errorf("should fail fast, took %v", elapsed)
	}
	if !strings.Contains(stderr, "locked") {
		t.Errorf("error should mention 'locked':\n%s", stderr)
	}
}

// =============================================================================
// V-09: Search commands (grep, egrep)
// =============================================================================

func TestGrep_Basic(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	stdout, _, exit := runAOA(t, dir, "grep", "hello")
	if exit != 0 {
		t.Fatalf("grep exit %d", exit)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("should contain 'hello' in results:\n%s", stdout)
	}
}

func TestGrep_NoDaemon(t *testing.T) {
	dir := setupProject(t)
	_, stderr, exit := runAOA(t, dir, "grep", "test")
	if exit == 0 {
		t.Error("should exit non-zero without daemon")
	}
	if !strings.Contains(stderr, "daemon not running") {
		t.Errorf("error should mention 'daemon not running':\n%s", stderr)
	}
}

func TestGrep_NoQuery(t *testing.T) {
	dir := setupProject(t)
	_, stderr, exit := runAOA(t, dir, "grep")
	if exit == 0 {
		t.Error("should exit non-zero with no query")
	}
	if !strings.Contains(stderr, "no search query") {
		t.Errorf("error should mention 'no search query':\n%s", stderr)
	}
}

func TestEgrep_Basic(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	stdout, _, exit := runAOA(t, dir, "egrep", "hel.*")
	if exit != 0 {
		t.Fatalf("egrep exit %d", exit)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("should contain 'hello' in results:\n%s", stdout)
	}
}

func TestEgrep_NoDaemon(t *testing.T) {
	dir := setupProject(t)
	_, stderr, exit := runAOA(t, dir, "egrep", "test")
	if exit == 0 {
		t.Error("should exit non-zero without daemon")
	}
	if !strings.Contains(stderr, "daemon not running") {
		t.Errorf("error should mention 'daemon not running':\n%s", stderr)
	}
}

// =============================================================================
// V-10: Query commands (health, find, locate, open)
// =============================================================================

func TestHealth_Running(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	stdout, _, exit := runAOA(t, dir, "health")
	if exit != 0 {
		t.Fatalf("health exit %d", exit)
	}
	for _, want := range []string{"Files:", "Tokens:", "Uptime:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("health missing %q:\n%s", want, stdout)
		}
	}
}

func TestHealth_NotRunning(t *testing.T) {
	dir := setupProject(t)
	stdout, _, exit := runAOA(t, dir, "health")
	// health exits 0 even when not running.
	if exit != 0 {
		t.Fatalf("health (no daemon) should exit 0, got %d", exit)
	}
	if !strings.Contains(stdout, "not running") {
		t.Errorf("should say 'not running':\n%s", stdout)
	}
}

func TestFind_Glob(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	stdout, _, exit := runAOA(t, dir, "find", "*.go")
	if exit != 0 {
		t.Fatalf("find exit %d", exit)
	}
	if !strings.Contains(stdout, ".go") {
		t.Errorf("should return .go files:\n%s", stdout)
	}
	// Should find all 3 files.
	for _, name := range []string{"main.go", "hello.go", "util.go"} {
		if !strings.Contains(stdout, name) {
			t.Errorf("find *.go should include %s:\n%s", name, stdout)
		}
	}
}

func TestFind_NoArg(t *testing.T) {
	dir := setupProject(t)
	_, _, exit := runAOA(t, dir, "find")
	if exit == 0 {
		t.Error("find with no arg should exit non-zero")
	}
}

func TestLocate_Name(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	stdout, _, exit := runAOA(t, dir, "locate", "main")
	if exit != 0 {
		t.Fatalf("locate exit %d", exit)
	}
	if !strings.Contains(stdout, "main") {
		t.Errorf("should find 'main':\n%s", stdout)
	}
}

func TestLocate_NoArg(t *testing.T) {
	dir := setupProject(t)
	_, _, exit := runAOA(t, dir, "locate")
	if exit == 0 {
		t.Error("locate with no arg should exit non-zero")
	}
}

func TestOpen_NoDaemon(t *testing.T) {
	dir := setupProject(t)
	_, stderr, exit := runAOA(t, dir, "open")
	if exit == 0 {
		t.Error("open without daemon should exit non-zero")
	}
	if !strings.Contains(stderr, "daemon not running") {
		t.Errorf("error should mention 'daemon not running':\n%s", stderr)
	}
}

func TestDashboard_HealthEndpoint(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// Read the HTTP port file
	portData, err := os.ReadFile(filepath.Join(dir, ".aoa", "run", "http.port"))
	if err != nil {
		t.Fatalf("read http.port: %v", err)
	}
	port := strings.TrimSpace(string(portData))
	url := fmt.Sprintf("http://localhost:%s/api/health", port)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", result["status"])
	}
}

func TestDashboard_HTMLServed(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	portData, err := os.ReadFile(filepath.Join(dir, ".aoa", "run", "http.port"))
	if err != nil {
		t.Fatalf("read http.port: %v", err)
	}
	port := strings.TrimSpace(string(portData))
	url := fmt.Sprintf("http://localhost:%s/index.html", port)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected text/html, got %s", ct)
	}
}

func TestDashboard_PortCleanedOnStop(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	portFile := filepath.Join(dir, ".aoa", "run", "http.port")

	// Port file should exist while running.
	if _, err := os.Stat(portFile); os.IsNotExist(err) {
		t.Error("http.port file should exist while daemon is running")
	}

	cleanup()
	time.Sleep(200 * time.Millisecond)

	// Port file should be removed after stop.
	if _, err := os.Stat(portFile); err == nil {
		t.Error("http.port file should be removed after daemon stop")
	}
}

func TestDaemonStart_ShowsDashboardURL(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	stdout, _, exit := runAOA(t, dir, "daemon", "start")
	if exit != 0 {
		t.Fatalf("daemon start failed: exit %d, output: %s", exit, stdout)
	}
	defer func() { runAOA(t, dir, "daemon", "stop") }()

	if !strings.Contains(stdout, "dashboard:") {
		t.Errorf("daemon start should show dashboard URL:\n%s", stdout)
	}
	if !strings.Contains(stdout, "http://localhost:") {
		t.Errorf("daemon start should include http://localhost:\n%s", stdout)
	}
}

// =============================================================================
// V-11: No-daemon error cases — table-driven
// =============================================================================

func TestNoDaemon_AllCommands(t *testing.T) {
	dir := setupProject(t)

	cases := []struct {
		name string
		args []string
	}{
		{"grep", []string{"grep", "test"}},
		{"egrep", []string{"egrep", "test"}},
		{"find", []string{"find", "*.go"}},
		{"locate", []string{"locate", "main"}},
		{"open", []string{"open"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, exit := runAOA(t, dir, tc.args...)
			if exit == 0 {
				t.Errorf("%s without daemon should exit non-zero", tc.name)
			}
			if !strings.Contains(stderr, "daemon not running") {
				t.Errorf("%s error should mention 'daemon not running', got:\n%s", tc.name, stderr)
			}
		})
	}
}

// =============================================================================
// V-12: Error message quality — verify actionable remediation
// =============================================================================

func TestInit_DaemonRunning_ReportsStats(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// Init delegates to daemon; output should include file/symbol/token counts.
	stdout, _, exit := runAOA(t, dir, "init")
	if exit != 0 {
		t.Fatalf("init should succeed via daemon, exit %d", exit)
	}

	// Must report indexing stats (files, symbols, tokens).
	for _, want := range []string{"files", "symbols", "tokens"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("init output should contain %q:\n%s", want, stdout)
		}
	}
}

func TestErrorMsg_WipeLocked(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	dbPath := filepath.Join(dir, ".aoa", "aoa.db")
	release := holdDBLock(t, dbPath)
	defer release()

	_, stderr, exit := runAOA(t, dir, "wipe", "--force")
	if exit == 0 {
		t.Fatal("should fail")
	}

	// Must mention it's locked and suggest how to fix.
	if !strings.Contains(stderr, "locked") {
		t.Errorf("wipe error should mention 'locked':\n%s", stderr)
	}
	if !strings.Contains(stderr, "process") {
		t.Errorf("wipe error should mention 'process':\n%s", stderr)
	}
}

func TestErrorMsg_DaemonStartLocked(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	dbPath := filepath.Join(dir, ".aoa", "aoa.db")
	release := holdDBLock(t, dbPath)
	defer release()

	_, stderr, exit := runAOA(t, dir, "daemon", "start")
	if exit == 0 {
		t.Fatal("should fail")
	}

	if !strings.Contains(stderr, "locked") {
		t.Errorf("daemon start error should mention 'locked':\n%s", stderr)
	}
}

// =============================================================================
// Timing guarantees — nothing should ever hang
// =============================================================================

func TestTiming_AllLockedOperations_FastFail(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	dbPath := filepath.Join(dir, ".aoa", "aoa.db")
	release := holdDBLock(t, dbPath)
	defer release()

	ops := []struct {
		name string
		args []string
	}{
		{"init", []string{"init"}},
		{"wipe", []string{"wipe", "--force"}},
		{"daemon start", []string{"daemon", "start"}},
	}

	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			start := time.Now()
			_, _, exit := runAOA(t, dir, op.args...)
			elapsed := time.Since(start)

			if exit == 0 {
				t.Errorf("%s should fail when DB is locked", op.name)
			}
			if elapsed > 3*time.Second {
				t.Errorf("%s took %v — should fail within 3 seconds (1s bbolt timeout + overhead)", op.name, elapsed)
			}
			if elapsed < 800*time.Millisecond {
				t.Errorf("%s completed in %v — suspiciously fast, timeout may not be working", op.name, elapsed)
			}
		})
	}
}

// =============================================================================
// V-13: L2 feature integration tests
// =============================================================================

func TestGrep_InvertMatch(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// Normal search for "hello" should find it.
	stdout, _, exit := runAOA(t, dir, "grep", "hello")
	if exit != 0 {
		t.Fatalf("grep exit %d", exit)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("grep should find 'hello':\n%s", stdout)
	}

	// Inverted: -v hello should return hits that do NOT include "hello".
	stdout, _, exit = runAOA(t, dir, "grep", "-v", "hello")
	if exit != 0 {
		t.Fatalf("grep -v exit %d", exit)
	}
	// In grep-compat mode (non-TTY), output is file:content lines.
	// Inverted results should contain non-hello symbols like "main" or "add".
	if stdout == "" {
		t.Errorf("grep -v should produce output")
	}
	if strings.Contains(stdout, "hello()") {
		t.Errorf("grep -v should NOT contain 'hello()':\n%s", stdout)
	}
}

func TestGrep_InvertMatch_CountOnly(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// -v -c should count non-matching symbols.
	// In grep-compat mode (non-TTY), output is just a number.
	stdout, _, exit := runAOA(t, dir, "grep", "-v", "-c", "hello")
	if exit != 0 {
		t.Fatalf("grep -v -c exit %d", exit)
	}
	count := strings.TrimSpace(stdout)
	if count == "" || count == "0" {
		t.Errorf("grep -v -c should show non-zero count, got: %q", count)
	}
}

func TestEgrep_InvertMatch(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// egrep -v with regex.
	// In grep-compat mode (non-TTY), output is file:content lines.
	stdout, _, exit := runAOA(t, dir, "egrep", "-v", "hel.*")
	if exit != 0 {
		t.Fatalf("egrep -v exit %d", exit)
	}
	if stdout == "" {
		t.Errorf("egrep -v should produce output")
	}
}

func TestInit_DaemonRunning_ThenSearchFindsNewSymbol(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// Add a new file with a unique symbol.
	writeFile(t, filepath.Join(dir, "newfile.go"), `package main

func UniqueNewSymbol() {
	return
}
`)

	// Reindex via init (delegates to daemon).
	stdout, _, exit := runAOA(t, dir, "init")
	if exit != 0 {
		t.Fatalf("init reindex exit %d: %s", exit, stdout)
	}

	// Search should now find the new symbol.
	stdout, _, exit = runAOA(t, dir, "grep", "uniquenewsymbol")
	if exit != 0 {
		t.Fatalf("grep exit %d", exit)
	}
	if !strings.Contains(strings.ToLower(stdout), "uniquenewsymbol") {
		t.Errorf("grep should find UniqueNewSymbol after reindex:\n%s", stdout)
	}
}

// =============================================================================
// L2.1: File watcher auto-reindex — no init needed
// =============================================================================

func TestFileWatcher_NewFile_AutoReindex(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// Baseline: "watcherprobe" should not exist yet.
	stdout, _, _ := runAOA(t, dir, "grep", "watcherprobe")
	if strings.Contains(strings.ToLower(stdout), "watcherprobe") {
		t.Fatal("watcherprobe should not exist before file creation")
	}

	// Create a new file — the daemon's file watcher should auto-detect it.
	writeFile(t, filepath.Join(dir, "probe.go"), `package main

func WatcherProbe() {
	return
}
`)

	// Wait for fsnotify detection (50ms debounce) + parse + rebuild.
	// Poll up to 3s to avoid flaky timing.
	var found bool
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		stdout, _, _ = runAOA(t, dir, "grep", "watcherprobe")
		if strings.Contains(strings.ToLower(stdout), "watcherprobe") {
			found = true
			break
		}
	}
	if !found {
		t.Error("file watcher should auto-index new file — WatcherProbe not found after 3s")
	}
}

func TestFileWatcher_ModifyFile_AutoReindex(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// Baseline: search for "multiply" (exists in util.go).
	stdout, _, exit := runAOA(t, dir, "grep", "multiply")
	if exit != 0 {
		t.Fatalf("baseline grep exit %d", exit)
	}
	if !strings.Contains(strings.ToLower(stdout), "multiply") {
		t.Fatal("multiply should exist in baseline index")
	}

	// Modify util.go: replace multiply with a new unique symbol.
	writeFile(t, filepath.Join(dir, "util.go"), `package main

func add(a, b int) int {
	return a + b
}

func WatcherModified(x, y int) int {
	return x * y
}
`)

	// Poll for the new symbol to appear.
	var found bool
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		stdout, _, _ = runAOA(t, dir, "grep", "watchermodified")
		if strings.Contains(strings.ToLower(stdout), "watchermodified") {
			found = true
			break
		}
	}
	if !found {
		t.Error("file watcher should auto-reindex modified file — WatcherModified not found after 3s")
	}
}

func TestFileWatcher_DeleteFile_AutoReindex(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// Baseline: "multiply" exists (in util.go).
	stdout, _, exit := runAOA(t, dir, "grep", "multiply")
	if exit != 0 {
		t.Fatalf("baseline grep exit %d", exit)
	}
	if !strings.Contains(strings.ToLower(stdout), "multiply") {
		t.Fatal("multiply should exist in baseline index")
	}

	// Delete util.go — watcher should remove its symbols.
	os.Remove(filepath.Join(dir, "util.go"))

	// Poll for the symbol to disappear.
	var gone bool
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		stdout, _, _ = runAOA(t, dir, "grep", "multiply")
		if !strings.Contains(strings.ToLower(stdout), "multiply") {
			gone = true
			break
		}
	}
	if !gone {
		t.Error("file watcher should remove deleted file's symbols — multiply still found after 3s")
	}
}
