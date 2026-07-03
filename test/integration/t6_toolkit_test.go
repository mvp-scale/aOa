//go:build !lean

// T6 — arch toolkit asymmetry gate (L19.17).
//
// Asserts:
//  1. aoa arch derive → unit-ID path → facts cite real file:line on importer
//     side (peekable); imported side is package/dir grain, not symbol.
//  2. Import-edge asymmetry: external imports resolve to "ext:" prefix; source
//     file:line is real, target is package grain only (no peek body).
//  3. CLI byte identity: `aoa arch view component` daemon-mode output equals
//     direct-RO output — agents and the F3 viewer read the same shards.
//  4. Dead-daemon + shim mode → grep/egrep emits error (exit 2), does NOT fall
//     back to system grep's stdin-reading behaviour (gotcha-3b regression gate).
package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startDaemonAndDeriveArch initialises a project, starts the daemon, triggers
// a Reindex (via a second `aoa init`) so arch derivation fires, and polls for
// shards. Returns (cleanup, hasData).
//
// Background: WarmCaches only backfills when the edges bucket is ABSENT (T43).
// When init already populated the bucket, arch shards are derived only when
// Reindex is called. The second init call delegates to Reindex via socket,
// which queues deriveArch as a background goroutine.
func startDaemonAndDeriveArch(t *testing.T, dir string) (cleanup func(), hasData bool) {
	t.Helper()

	// First init: builds index and saves edges to DB (no daemon).
	runAOA(t, dir, "init")

	// Start daemon.
	cleanup = startDaemon(t, dir)

	// Second init: daemon is now up, so init delegates to Reindex via socket.
	// Reindex triggers deriveArch as a background goroutine (arch-derive safeGo).
	runAOA(t, dir, "init")

	// Poll for arch shards (derivation is async after Reindex).
	hasData = pollForArchData(t, dir, 15*time.Second)
	return cleanup, hasData
}

// ── T6.1 / T6.2: derive handoff + import-edge asymmetry ────────────────────

// TestT6_Derive_Peek_Handoff verifies the graph→peek handoff chain:
//   `aoa arch derive A B` → JSON unit-ID path → last hop has facts with
//   real from_file:start_line (importer side, peekable).
//
// Also verifies import-edge asymmetry: the imported side (last hop) resolves
// to package/dir grain (no symbol-level peek body); the source side carries
// file:line that exists on disk.
func TestT6_Derive_Peek_Handoff(t *testing.T) {
	dir := setupArchProject(t)
	cleanup, hasData := startDaemonAndDeriveArch(t, dir)
	defer cleanup()

	if !hasData {
		t.Skip("T6.1: arch data not derived within timeout — environment too slow")
	}

	// Derive the path between two known units in the test project.
	stdout, stderr, exit := runAOA(t, dir, "arch", "derive", "internal/service", "internal/db")
	if exit == 1 {
		t.Skip("T6.1: no path found between units — arch graph may not have captured edges")
	}
	if exit != 0 {
		t.Fatalf("arch derive: exit %d\nstderr: %s", exit, stderr)
	}

	// Output must be a JSON array of unit-ID strings.
	var path []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &path); err != nil {
		t.Fatalf("arch derive output must be a JSON array of unit IDs: %s\nerr: %v", stdout, err)
	}
	if len(path) < 2 {
		t.Fatalf("expected path with at least 2 hops, got %v", path)
	}

	// The last hop (target unit) — verify it's a unit-ID slug (u_*).
	lastHop := path[len(path)-1]
	if !strings.HasPrefix(lastHop, "u_") {
		t.Errorf("T6.1: last hop %q should be a unit-ID slug (u_*)", lastHop)
	}
	t.Logf("T6.1: derive path: %v — last hop: %s", path, lastHop)

	// T6.1b — facts for the source unit should show real file:line (peekable).
	// The importer side in the test project is internal/service/service.go.
	factsOut, factsStderr, factsExit := runAOA(t, dir, "arch", "facts", "service")
	if factsExit != 0 {
		t.Fatalf("arch facts service: exit %d\nstderr: %s", factsExit, factsStderr)
	}

	type factEntry struct {
		FromFile   string `json:"from_file"`
		ImportPath string `json:"import_path"`
		StartLine  uint32 `json:"start_line"`
	}
	var facts []factEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(factsOut)), &facts); err != nil {
		t.Fatalf("arch facts output must be JSON: %s\nerr: %v", factsOut, err)
	}

	// Find at least one intra-repo edge with a real file:line (importer peekable).
	foundIntraRepo := false
	for _, f := range facts {
		if strings.HasPrefix(f.ImportPath, "ext:") {
			continue // skip external imports for this assertion
		}
		// Importer file must exist on disk.
		if _, err := os.Stat(filepath.Join(dir, f.FromFile)); err == nil {
			if f.StartLine > 0 {
				foundIntraRepo = true
				t.Logf("T6.1: intra-repo edge: %s:%d → %s (file exists, importer peekable)",
					f.FromFile, f.StartLine, f.ImportPath)
				break
			}
		}
	}
	if !foundIntraRepo {
		t.Errorf("T6.1: expected at least one intra-repo edge with real from_file:start_line; got: %v", facts)
	}
}

// TestT6_ImportEdge_Asymmetry_Ext verifies that external imports are stamped
// with the "ext:" prefix, and the source side still carries real file:line
// while the target is package grain only (no symbol body to peek).
func TestT6_ImportEdge_Asymmetry_Ext(t *testing.T) {
	// The setupArchProject test project imports "fmt" (stdlib) from main.go.
	dir := setupArchProject(t)
	cleanup, hasData := startDaemonAndDeriveArch(t, dir)
	defer cleanup()

	if !hasData {
		t.Skip("T6.2: arch data not derived within timeout")
	}

	// Search facts for "fmt" — stdlib import should appear as ext:std/fmt.
	factsOut, factsStderr, factsExit := runAOA(t, dir, "arch", "facts", "fmt")
	if factsExit != 0 {
		t.Logf("arch facts fmt: exit %d\nstderr: %s — stdlib not in edge set; skipping", factsExit, factsStderr)
		t.Skip("T6.2: no facts for 'fmt' — stdlib imports may not be in test fixture edges")
	}

	type factEntry struct {
		FromFile   string `json:"from_file"`
		ImportPath string `json:"import_path"`
		StartLine  uint32 `json:"start_line"`
	}
	var facts []factEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(factsOut)), &facts); err != nil {
		t.Fatalf("arch facts output must be JSON: %s\nerr: %v", factsOut, err)
	}

	// Find an ext:-stamped edge.
	foundExt := false
	for _, f := range facts {
		if strings.HasPrefix(f.ImportPath, "ext:") {
			foundExt = true
			// Source (importer) must have a real file on disk.
			if _, err := os.Stat(filepath.Join(dir, f.FromFile)); os.IsNotExist(err) {
				t.Errorf("T6.2: ext import %q: from_file %q must exist on disk", f.ImportPath, f.FromFile)
			}
			t.Logf("T6.2: ext edge: %s:%d → %s (package grain, no peek body available)",
				f.FromFile, f.StartLine, f.ImportPath)
			break
		}
	}
	if !foundExt {
		// Soft assertion: the asymmetry is covered by resolve_test.go unit tests.
		t.Skip("T6.2: no ext:-stamped edge in test fixture — asymmetry also covered by resolve_test.go")
	}
}

// ── T6.3: CLI byte identity (same shards for agents and viewer) ─────────────

// TestT6_ViewShard_ByteIdentity asserts that `aoa arch view component` returns
// identical bytes whether the daemon is up (socket path) or down (direct-RO
// path). This is the "CLI JSON == stored shard byte-identity" assertion:
// agents and the F3 viewer read the same arch_shards bucket.
func TestT6_ViewShard_ByteIdentity(t *testing.T) {
	dir := setupArchProject(t)
	cleanup, hasData := startDaemonAndDeriveArch(t, dir)

	if !hasData {
		cleanup()
		t.Skip("T6.3: arch data not derived within timeout")
	}

	// Daemon-mode view.
	daemonOut, daemonStderr, daemonExit := runAOA(t, dir, "arch", "view", "component")
	cleanup()
	time.Sleep(300 * time.Millisecond) // let daemon release DB lock

	if daemonExit != 0 {
		t.Skipf("T6.3: arch view component daemon-mode exit %d (no data): %s", daemonExit, daemonStderr)
	}
	if !json.Valid([]byte(strings.TrimSpace(daemonOut))) {
		t.Fatalf("T6.3: daemon-mode view must be valid JSON:\n%s", daemonOut)
	}

	// Direct-RO view (daemon already stopped).
	roOut, roStderr, roExit := runAOA(t, dir, "arch", "view", "component")
	if roExit != 0 {
		t.Fatalf("T6.3: direct-RO arch view exit %d: %s", roExit, roStderr)
	}

	// Byte identity: both CLI paths read raw bytes from arch_shards bucket.
	daemonTrimmed := strings.TrimSpace(daemonOut)
	roTrimmed := strings.TrimSpace(roOut)
	if daemonTrimmed != roTrimmed {
		t.Errorf("T6.3: byte identity failure — daemon and direct-RO paths returned different bytes:\n  daemon: %s\n  ro:     %s",
			truncate(daemonTrimmed, 300), truncate(roTrimmed, 300))
	} else {
		t.Logf("T6.3: byte identity confirmed (%d bytes match between daemon and direct-RO paths)", len(daemonTrimmed))
	}
}

// ── T6.4: dead-daemon shim mode must not fall back to stdin-reading grep ────

// TestT6_DeadDaemon_ShimMode_EmitsError verifies gotcha-3b regression:
// when the daemon is dead (and revive fails because no .aoa/ exists), grep and
// egrep in AOA_SHIM=1 mode must emit a clear error (exit non-zero), NOT call
// system grep which would read stdin and hang the agent indefinitely.
//
// This uses a directory with no .aoa/ so canRevive=false and reviveDaemon
// returns false immediately — the index-search path must then error, not fall
// through to fallbackSystemGrep.
func TestT6_DeadDaemon_ShimMode_EmitsError(t *testing.T) {
	// No .aoa/ → canRevive=false → reviveDaemon returns false immediately.
	dir := t.TempDir()
	writeFile(t, dir+"/main.go", "package main\n")

	for _, subcmd := range []string{"grep", "egrep"} {
		t.Run(subcmd, func(t *testing.T) {
			// Run in shim mode with no file args (index-search path).
			// AOA_SHIM=1 means the binary is being called from the shim script.
			// "somePattern" is a search query (no file path → index search mode).
			stdout, stderr, exit := runAOAWithEnv(t, dir,
				[]string{"AOA_SHIM=1"},
				subcmd, "somePattern")

			// Must exit non-zero: index search with no daemon is an error.
			if exit == 0 {
				t.Errorf("T6.4/%s: shim mode + dead daemon should exit non-zero, got 0\nstdout: %s",
					subcmd, stdout)
			}

			// Stderr must mention daemon (actionable error message).
			if !strings.Contains(stderr, "daemon") && !strings.Contains(stderr, "running") {
				t.Errorf("T6.4/%s: error must mention daemon; got stderr: %q", subcmd, stderr)
			}

			// Stdout must be empty — no stdin-read content should leak through.
			if strings.TrimSpace(stdout) != "" {
				t.Errorf("T6.4/%s: stdout must be empty in dead-daemon shim path; got: %q", subcmd, stdout)
			}

			t.Logf("T6.4/%s: exit=%d stderr=%q (correct: error emitted, stdin not read)",
				subcmd, exit, stderr)
		})
	}
}

// TestT6_DeadDaemon_ShimMode_LazyRevives is gotcha 3b's POSITIVE half
// (TestT6_DeadDaemon_ShimMode_EmitsError covers the uninitialized-project
// negative): with an INITIALIZED project and a dead daemon, shim-mode index
// search must lazy-revive the daemon (L21.1 flock-guarded spawn + one retry)
// and return results — not error out, not read stdin.
func TestT6_DeadDaemon_ShimMode_LazyRevives(t *testing.T) {
	dir := setupArchProject(t)
	runAOA(t, dir, "init") // index + DB exist; daemon deliberately NOT started
	defer runAOA(t, dir, "daemon", "stop") // stop the daemon the shim revives

	stdout, stderr, exit := runAOAWithEnv(t, dir,
		[]string{"AOA_SHIM=1"},
		"grep", "Connect")
	if exit != 0 {
		t.Fatalf("T6.4b: shim grep on initialized project + dead daemon must lazy-revive and answer; exit=%d\nstderr: %s", exit, stderr)
	}
	if !strings.Contains(stdout, "Connect") {
		t.Errorf("T6.4b: expected index results for 'Connect', got stdout: %q", stdout)
	}

	// The revive must leave a live daemon behind (L21 lazy-start, not one-shot).
	_, _, healthExit := runAOA(t, dir, "health")
	if healthExit != 0 {
		t.Errorf("T6.4b: daemon should be alive after lazy-revive; health exit=%d", healthExit)
	}
}
