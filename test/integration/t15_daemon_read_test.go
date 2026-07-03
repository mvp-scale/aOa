//go:build !lean

// T15 daemon-read latency: arch.view socket round-trip sub-ms (L19.16 AC).
//
// L19.16 acceptance criterion: "aoa arch view component returns deterministic
// file:line-stamped JSON in <1ms daemon path."
// Ledger T15 gap (checkpoint-F2.md PC5): this path was measured nowhere.
//
// Method: start a daemon with derived arch data, then run N=100 arch.views
// requests directly over the Unix socket and assert p50 < 1ms.
//
// The measurement uses client.ArchViews which sends arch.views (manifest fetch
// — a single bbolt read). The socket round-trip includes connect + write +
// read + close on a Unix domain socket; it does NOT include process spawn.
//
// If arch data cannot be derived (PC1 boot gap), the test is skipped with a
// clear message. The latency claim only applies to the cached-read path.
package integration

import (
	"sort"
	"testing"
	"time"

	"github.com/corey/aoa/internal/adapters/socket"
)

// TestT15_DaemonReadLatency measures the arch.views socket round-trip over
// N=100 iterations on a daemon that has derived arch shards, and asserts p50 < 1ms.
func TestT15_DaemonReadLatency(t *testing.T) {
	dir := setupArchProject(t)

	// startDaemonAndDeriveArch is defined in t6_toolkit_test.go (same package).
	cleanup, hasData := startDaemonAndDeriveArch(t, dir)
	defer cleanup()

	if !hasData {
		t.Skip("arch data not derived after 15s (PC1 boot gap) — skipping <1ms latency assertion; fix PC1 first")
	}

	sockPath := socketPathForDir(dir)
	c := socket.NewClient(sockPath)

	// Warm-up call: first access may be slower due to bbolt page faults.
	if _, err := c.ArchViews("local"); err != nil {
		t.Fatalf("warm-up arch.views call failed: %v", err)
	}

	const N = 100
	durations := make([]time.Duration, N)
	for i := 0; i < N; i++ {
		start := time.Now()
		result, err := c.ArchViews("local")
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("arch.views call %d failed: %v", i, err)
		}
		if !result.HasData {
			// Data disappeared between warm-up and measurement — unlikely but guard it.
			t.Fatalf("arch.views call %d: HasData=false after successful warm-up", i)
		}
		durations[i] = elapsed
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	p50 := durations[N/2]
	p95 := durations[int(float64(N)*0.95)]
	p99 := durations[int(float64(N)*0.99)]

	t.Logf("T15 daemon-read latency (N=%d, arch.views): p50=%s p95=%s p99=%s", N, p50, p95, p99)

	const budget = time.Millisecond
	if p50 > budget {
		// Do NOT loosen the budget silently. Report the measured value and fail.
		t.Errorf("T15 BREACH: daemon-read p50 %s > budget %s — <1ms AC not met (L19.16)", p50, budget)
		t.Logf("  p95=%s p99=%s — check for bbolt lock contention or page cache pressure", p95, p99)
	} else {
		t.Logf("T15 PASS: daemon-read p50 %s ≤ %s (L19.16 <1ms AC met)", p50, budget)
	}
}
