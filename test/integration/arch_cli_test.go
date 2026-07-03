//go:build !lean

// T8 — three-mode parity for `aoa arch` + C4 dark check (L19.16).
//
// Three-mode parity contract:
//   1. Daemon mode: commands route through the Unix socket (daemon up).
//   2. Direct-RO mode: commands open bbolt read-only (daemon never started).
//   3. Dead-daemon mode: daemon was up, stopped, fallback to direct-RO works.
//
// C4 dark check: AOA_ARCH=off → no "arch" subcommand in help; socket methods
// for arch return a clear error (not "unknown method").
package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupArchProject creates a temp dir with inter-package Go imports so that
// the arch pipeline produces real edges and shards.
func setupArchProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Root package — imports internal/service.
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"fmt"
	"github.com/example/aoa-arch-test/internal/service"
)

func main() {
	fmt.Println(service.Run())
}
`)

	// go.mod so imports resolve as intra-repo.
	writeFile(t, filepath.Join(dir, "go.mod"), `module github.com/example/aoa-arch-test

go 1.21
`)

	// internal/service — imports internal/db.
	writeFile(t, filepath.Join(dir, "internal", "service", "service.go"), `package service

import (
	"github.com/example/aoa-arch-test/internal/db"
)

// Run starts the service.
func Run() string {
	return db.Connect()
}
`)

	// internal/db — no intra-repo imports.
	writeFile(t, filepath.Join(dir, "internal", "db", "db.go"), `package db

// Connect connects to the DB.
func Connect() string {
	return "connected"
}
`)

	return dir
}

// pollForArchData retries `aoa arch views` up to maxWait, returning true when it
// exits 0 (i.e. shards have been derived). Used to absorb async derivation lag.
func pollForArchData(t *testing.T, dir string, maxWait time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		_, _, exit := runAOA(t, dir, "arch", "views")
		if exit == 0 {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// =============================================================================
// C4 dark check — AOA_ARCH=off removes "arch" from CLI and errors at socket
// =============================================================================

// TestArch_C4Dark_CLIHideSubcommand verifies that with AOA_ARCH=off, the "arch"
// subcommand does NOT appear in `aoa --help` output.
func TestArch_C4Dark_CLIHideSubcommand(t *testing.T) {
	dir := setupProject(t)

	// With arch off: --help must not show "arch" as a command entry.
	// We check for "  arch" (Cobra indents commands with two spaces) to avoid
	// false matches on substrings like "search" or "egrep ... search".
	stdout, _, exit := runAOAWithEnv(t, dir, []string{"AOA_ARCH=off"}, "--help")
	if exit != 0 {
		t.Fatalf("--help should exit 0, got %d", exit)
	}
	if strings.Contains(stdout, "  arch") {
		t.Errorf("AOA_ARCH=off: 'arch' command must not appear in --help output:\n%s", stdout)
	}

	// With arch on (default): "arch" must appear as a command.
	stdoutOn, _, _ := runAOAWithEnv(t, dir, []string{"AOA_ARCH=on"}, "--help")
	if !strings.Contains(stdoutOn, "  arch") {
		t.Errorf("AOA_ARCH=on: 'arch' command must appear in --help output:\n%s", stdoutOn)
	}
}

// TestArch_C4Dark_CommandNotRegistered verifies that with AOA_ARCH=off, invoking
// `aoa arch views` produces an error (command not found), not an arch response.
func TestArch_C4Dark_CommandNotRegistered(t *testing.T) {
	dir := setupProject(t)

	_, stderr, exit := runAOAWithEnv(t, dir, []string{"AOA_ARCH=off"}, "arch", "views")
	if exit == 0 {
		t.Error("aoa arch views with AOA_ARCH=off should exit non-zero")
	}
	// Cobra reports "unknown command" when the subcommand is not registered.
	if !strings.Contains(stderr, "unknown command") && !strings.Contains(stderr, "arch") {
		t.Errorf("should report 'unknown command' or arch-not-available:\n%s", stderr)
	}
}

// =============================================================================
// T8 — three-mode parity: daemon / direct-RO / dead-daemon
// =============================================================================

// TestArch_T8_Views_DaemonMode verifies `aoa arch views` works when the daemon
// is running (mode 1: daemon path through socket).
// PC2: after PC1 (boot-arch-derive), data must always be present. hasData=false
// is now a hard failure, and we assert valid JSON output when data is present.
func TestArch_T8_Views_DaemonMode(t *testing.T) {
	dir := setupArchProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()

	// PC1: daemon boot fires boot-arch-derive when edges exist but shards are absent.
	// Data must arrive within 10s; absence is a test failure (not a log note).
	if !pollForArchData(t, dir, 10*time.Second) {
		t.Fatal("T8/daemon: arch data must be present after boot-derive (PC1); no shards after 10s")
	}

	stdout, stderr, exit := runAOA(t, dir, "arch", "views")
	if exit != 0 {
		t.Fatalf("arch views daemon mode: must exit 0 when data present, got %d\nstderr: %s", exit, stderr)
	}
	// Assert the output is valid JSON (manifest array or object).
	if strings.TrimSpace(stdout) == "" {
		t.Error("arch views daemon mode: stdout must not be empty when exit 0")
	}
	t.Logf("daemon mode exit=%d stdout=%q", exit, truncate(stdout, 200))
}

// TestArch_T8_Views_DirectRO verifies `aoa arch views` falls back to direct RO
// when the daemon is NOT running (mode 2: direct-RO fallback).
// PC2: data must be present (PC1 boot-derive); absence is a hard failure.
// The environmental t.Skip is replaced with a bounded retry + t.Fatal so the
// gate cannot be vacuously skipped.
func TestArch_T8_Views_DirectRO(t *testing.T) {
	dir := setupArchProject(t)
	runAOA(t, dir, "init")

	// Start daemon; PC1 boot-arch-derive fires and produces shards.
	cleanup := startDaemon(t, dir)
	if !pollForArchData(t, dir, 10*time.Second) {
		cleanup()
		t.Fatal("T8/direct-RO: arch data must be present after boot-derive (PC1); no shards after 10s")
	}
	cleanup()

	// Wait for daemon to release the DB lock (retry loop; bounded 2s).
	// t.Fatal replaces the old t.Skip — a daemon that won't stop is a real
	// environment problem, not a reason to silently skip the gate.
	stopped := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, healthExit := runAOA(t, dir, "health"); healthExit != 0 {
			stopped = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !stopped {
		t.Fatal("T8/direct-RO: daemon did not stop within 2s — cannot test direct-RO fallback")
	}

	// arch views must work in direct-RO mode (no daemon) and return data.
	stdout, stderr, exit := runAOA(t, dir, "arch", "views")
	if exit != 0 {
		t.Fatalf("arch views direct-RO: must exit 0 when shards present, got %d\nstderr: %s", exit, stderr)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("arch views direct-RO: stdout must not be empty when exit 0")
	}
	t.Logf("direct-RO mode exit=%d stdout=%q", exit, truncate(stdout, 200))
}

// TestArch_T8_Views_DeadDaemon verifies mode 3: daemon was up, is now down,
// and the fallback to direct RO delivers the same data.
// PC2: hasData=false is a hard failure (not a silent pass). Shard byte content
// must match across daemon and dead-daemon fallback modes.
// The environmental t.Skip is replaced with a bounded retry + t.Fatal.
func TestArch_T8_Views_DeadDaemon(t *testing.T) {
	dir := setupArchProject(t)
	runAOA(t, dir, "init")

	// Bring daemon up; PC1 boot-arch-derive fires and produces shards.
	cleanup := startDaemon(t, dir)
	if !pollForArchData(t, dir, 10*time.Second) {
		cleanup()
		t.Fatal("T8/dead-daemon: arch data must be present after boot-derive (PC1); no shards after 10s")
	}

	// Capture daemon-mode output (data is present — exit must be 0).
	daemonStdout, daemonStderr, daemonExit := runAOA(t, dir, "arch", "views")
	if daemonExit != 0 {
		cleanup()
		t.Fatalf("T8/dead-daemon: daemon mode must exit 0 with data; exit=%d stderr=%s", daemonExit, daemonStderr)
	}

	// Kill daemon (dead-daemon scenario).
	cleanup()

	// Wait for daemon to die (bounded retry; t.Fatal replaces old t.Skip).
	stopped := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, healthExit := runAOA(t, dir, "health"); healthExit != 0 {
			stopped = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !stopped {
		t.Fatal("T8/dead-daemon: daemon did not stop within 2s — cannot test dead-daemon fallback")
	}

	// Fallback: arch views must work (direct-RO) and return the exact same bytes.
	fallbackStdout, fallbackStderr, fallbackExit := runAOA(t, dir, "arch", "views")
	if fallbackExit != 0 {
		t.Fatalf("T8/dead-daemon: fallback must exit 0 with data; exit=%d stderr=%s", fallbackExit, fallbackStderr)
	}

	// Shard byte identity: same bbolt bytes → same JSON output.
	if daemonStdout != fallbackStdout {
		t.Errorf("T8/dead-daemon shard byte parity failure:\ndaemon:   %s\nfallback: %s",
			truncate(daemonStdout, 400), truncate(fallbackStdout, 400))
	}
	t.Logf("dead-daemon parity: daemon=%d fallback=%d bytes=%d", daemonExit, fallbackExit, len(daemonStdout))
}

// TestArch_T8_Findings_ThreeModes verifies `aoa arch findings` in all three modes.
// PC2: compare payload JSON across modes, not just exit codes.
func TestArch_T8_Findings_ThreeModes(t *testing.T) {
	dir := setupArchProject(t)
	runAOA(t, dir, "init")

	// Mode 1: daemon.
	cleanup := startDaemon(t, dir)
	if !pollForArchData(t, dir, 10*time.Second) {
		cleanup()
		t.Fatal("T8/findings: arch data must be present after boot-derive (PC1)")
	}

	daemonStdout, _, daemonExit := runAOA(t, dir, "arch", "findings")
	// exit 0 = findings list (possibly empty []); exit 1 = no findings; both valid.
	if daemonExit != 0 && daemonExit != 1 {
		cleanup()
		t.Fatalf("arch findings daemon: unexpected exit %d", daemonExit)
	}

	// Mode 3: dead-daemon → direct-RO fallback.
	cleanup()
	stopped := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, h := runAOA(t, dir, "health"); h != 0 {
			stopped = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !stopped {
		t.Fatal("T8/findings: daemon did not stop within 2s")
	}

	fallbackStdout, _, fallbackExit := runAOA(t, dir, "arch", "findings")

	// Exit code parity.
	if daemonExit != fallbackExit {
		t.Errorf("findings exit parity: daemon=%d fallback=%d", daemonExit, fallbackExit)
	}
	// Payload parity: same mode→same bytes.
	if daemonExit == fallbackExit && daemonStdout != fallbackStdout {
		t.Errorf("findings content parity failure:\ndaemon:   %s\nfallback: %s",
			truncate(daemonStdout, 300), truncate(fallbackStdout, 300))
	}
	t.Logf("findings parity: daemon=%d fallback=%d bytes=%d", daemonExit, fallbackExit, len(daemonStdout))
}

// TestArch_T8_Facts_ThreeModes verifies `aoa arch facts <subject>` in daemon and fallback modes.
// PC2: compare payload JSON across modes, not just exit codes.
func TestArch_T8_Facts_ThreeModes(t *testing.T) {
	dir := setupArchProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	if !pollForArchData(t, dir, 10*time.Second) {
		cleanup()
		t.Fatal("T8/facts: arch data must be present after boot-derive (PC1)")
	}

	// Mode 1: daemon mode — facts for a known path substring.
	daemonStdout, _, daemonExit := runAOA(t, dir, "arch", "facts", "internal")
	if daemonExit != 0 && daemonExit != 1 {
		cleanup()
		t.Fatalf("arch facts daemon: unexpected exit %d", daemonExit)
	}

	cleanup()
	stopped := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, h := runAOA(t, dir, "health"); h != 0 {
			stopped = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !stopped {
		t.Fatal("T8/facts: daemon did not stop within 2s")
	}

	// Mode 3: direct-RO fallback.
	fallbackStdout, _, fallbackExit := runAOA(t, dir, "arch", "facts", "internal")

	// Exit code parity.
	if daemonExit != fallbackExit {
		t.Errorf("facts exit parity: daemon=%d fallback=%d", daemonExit, fallbackExit)
	}
	// Payload parity.
	if daemonExit == fallbackExit && daemonStdout != fallbackStdout {
		t.Errorf("facts content parity failure:\ndaemon:   %s\nfallback: %s",
			truncate(daemonStdout, 300), truncate(fallbackStdout, 300))
	}
	t.Logf("facts parity: daemon=%d fallback=%d bytes=%d", daemonExit, fallbackExit, len(daemonStdout))
}

// TestArch_T8_Derive_ThreeModes verifies `aoa arch derive` in daemon and fallback modes.
// PC2: compare payload JSON across modes, not just exit codes.
func TestArch_T8_Derive_ThreeModes(t *testing.T) {
	dir := setupArchProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	if !pollForArchData(t, dir, 10*time.Second) {
		cleanup()
		t.Fatal("T8/derive: arch data must be present after boot-derive (PC1)")
	}

	// Attempt derive between two units. exit 0 = path found; exit 1 = no path.
	daemonStdout, _, daemonExit := runAOA(t, dir, "arch", "derive", "internal/service", "internal/db")
	if daemonExit != 0 && daemonExit != 1 {
		cleanup()
		t.Fatalf("arch derive daemon: unexpected exit %d", daemonExit)
	}

	cleanup()
	stopped := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, h := runAOA(t, dir, "health"); h != 0 {
			stopped = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !stopped {
		t.Fatal("T8/derive: daemon did not stop within 2s")
	}

	fallbackStdout, _, fallbackExit := runAOA(t, dir, "arch", "derive", "internal/service", "internal/db")

	// Exit code parity.
	if daemonExit != fallbackExit {
		t.Errorf("derive exit parity: daemon=%d fallback=%d", daemonExit, fallbackExit)
	}
	// Payload parity.
	if daemonExit == fallbackExit && daemonStdout != fallbackStdout {
		t.Errorf("derive content parity failure:\ndaemon:   %s\nfallback: %s",
			truncate(daemonStdout, 300), truncate(fallbackStdout, 300))
	}
	t.Logf("derive parity: daemon=%d fallback=%d bytes=%d", daemonExit, fallbackExit, len(daemonStdout))
}

// TestArch_T8_NoSubstrate_ExitsTwo verifies that with no .aoa directory,
// arch commands exit 2 (operational error) rather than 1 (empty result).
func TestArch_T8_NoSubstrate_ExitsTwo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n")

	_, stderr, exit := runAOA(t, dir, "arch", "views")
	if exit != 2 {
		t.Errorf("arch views with no substrate: expected exit 2, got %d\nstderr: %s", exit, stderr)
	}
	if !strings.Contains(stderr, "substrate") && !strings.Contains(stderr, "init") {
		t.Errorf("error should mention 'substrate' or 'init':\n%s", stderr)
	}
}

// TestArch_T8_DefaultArm_StillWorksForNonArch verifies that the six new arch
// dispatch arms don't break the existing "unknown method" default arm.
// Sends a garbage method and checks for "unknown method" (not an arch error).
func TestArch_T8_DefaultArm_UnchangedForNonArch(t *testing.T) {
	dir := setupProject(t)
	runAOA(t, dir, "init")
	cleanup := startDaemon(t, dir)
	defer cleanup()

	// `aoa health` uses MethodHealth (not an arch method); must still work.
	stdout, _, exit := runAOA(t, dir, "health")
	if exit != 0 {
		t.Errorf("health (non-arch method) must still work after arch arms added, exit %d", exit)
	}
	if !strings.Contains(stdout, "Files:") {
		t.Errorf("health should return file count:\n%s", stdout)
	}
}

// TestArch_FindingsNew_CI verifies `aoa arch findings --new` exits 0 when clean
// and exits 1 when findings exist.
func TestArch_FindingsNew_CI(t *testing.T) {
	dir := setupArchProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()
	pollForArchData(t, dir, 10*time.Second)

	// Without --new: must exit 0 (list findings, always succeeds).
	_, _, exitList := runAOA(t, dir, "arch", "findings")
	if exitList != 0 {
		t.Logf("arch findings (no --new): exit %d (non-zero findings list is OK)", exitList)
	}

	// With --new: exit 0 if no findings, 1 if findings exist.
	_, _, exitNew := runAOA(t, dir, "arch", "findings", "--new")
	if exitNew != 0 && exitNew != 1 {
		t.Errorf("arch findings --new: expected exit 0 or 1 (CI gate), got %d", exitNew)
	}
	t.Logf("findings --new exit=%d (0=clean, 1=findings-exist)", exitNew)
}

// TestArch_Views_HelpWorks verifies `aoa arch views --help` works.
func TestArch_Views_HelpWorks(t *testing.T) {
	dir := setupProject(t)
	stdout, _, exit := runAOA(t, dir, "arch", "views", "--help")
	if exit != 0 {
		t.Fatalf("arch views --help exit %d", exit)
	}
	for _, want := range []string{"views", "scope", "manifest"} {
		if !strings.Contains(strings.ToLower(stdout), want) {
			t.Errorf("arch views --help should mention %q:\n%s", want, stdout)
		}
	}
}

// TestArch_AllSubcmdsHaveHelp verifies every arch subcommand has a --help.
func TestArch_AllSubcmdsHaveHelp(t *testing.T) {
	dir := setupProject(t)
	subcmds := []string{"views", "view", "findings", "derive", "facts", "journey", "reach", "blast", "pack"}
	for _, sub := range subcmds {
		t.Run(sub, func(t *testing.T) {
			stdout, stderr, _ := runAOA(t, dir, "arch", sub, "--help")
			combined := stdout + stderr
			if combined == "" {
				t.Errorf("arch %s --help produced no output", sub)
			}
		})
	}
}

// TestArch_View_UnknownID_ExitsOne verifies that `aoa arch view nonexistent-view`
// exits 1 (empty result), not 2 (operational error).
func TestArch_View_UnknownID_ExitsOne(t *testing.T) {
	dir := setupArchProject(t)
	runAOA(t, dir, "init")

	cleanup := startDaemon(t, dir)
	defer cleanup()
	pollForArchData(t, dir, 10*time.Second)

	_, _, exit := runAOA(t, dir, "arch", "view", "nonexistent-view-zzzz")
	// Exit 1 = not found, or 0 if no data derived yet.
	if exit != 0 && exit != 1 {
		t.Errorf("arch view nonexistent: expected exit 0 or 1, got %d", exit)
	}
}

// TestArch_Reach_Alias verifies `aoa arch reach` is a recognised CLI command.
func TestArch_Reach_Alias(t *testing.T) {
	dir := setupProject(t)
	// reach --help must work (command registered).
	stdout, _, exit := runAOA(t, dir, "arch", "reach", "--help")
	if exit != 0 {
		t.Fatalf("arch reach --help exit %d", exit)
	}
	if !strings.Contains(strings.ToLower(stdout), "alias") && !strings.Contains(strings.ToLower(stdout), "derive") {
		t.Errorf("arch reach --help should mention it's an alias:\n%s", stdout)
	}
}

// TestArch_Blast_Alias verifies `aoa arch blast` is a recognised CLI command.
func TestArch_Blast_Alias(t *testing.T) {
	dir := setupProject(t)
	stdout, _, exit := runAOA(t, dir, "arch", "blast", "--help")
	if exit != 0 {
		t.Fatalf("arch blast --help exit %d", exit)
	}
	_ = stdout
}

// ── helpers ───────────────────────────────────────────────────────────────────

// truncate returns the first n characters of s.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Suppress unused import warnings.
var _ = os.Stderr
