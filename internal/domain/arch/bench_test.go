package arch

// T15 — compact-time budget on a synthetic 30k-file fixture.
//
// This file provides:
//   - gen30kFixture: generates a synthetic 30k-unit, ~90k-dep fixture in memory
//   - BenchmarkRenderAll_30k: measures full derive latency (Detect + Group + 3 renders)
//   - TestT15_30kFixture_LatencyBudget: asserts budget ≤ 10s latency for a fresh derive
//     pass (sub-ms is the cached socket-read budget, not the fresh-derive budget — see
//     T15 daemon-read test in test/integration/t15_daemon_read_test.go)
//
// RSS measurement uses /proc/self/status on Linux; falls back to 0 on other platforms.
// Numbers are always printed so CI logs capture them even when the assertion passes.
//
// Budget rationale (WP T15):
//   - 30k files ≈ largest realistic Go mono-repo
//   - Each file → 1 unit; avg 3 deps per unit → 90k DepFacts
//   - Derive pass (Detect + Group + 3 renders) must complete in ≤ 10s wall-clock
//     (measured baseline 2.6–2.8s; ×3.7 headroom; this is a ceiling, not a target)
//   - RSS growth budget: ≤ 256 MB (measured baseline 71.8–93.9 MB; ×2.7 headroom)
//     WP T15 names no explicit RSS number; 256 MB is the recorded budget (PC5)

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	// t15UnitCount is the number of synthetic units in the 30k fixture.
	t15UnitCount = 30_000
	// t15DepsPerUnit is the average number of dep edges per unit.
	t15DepsPerUnit = 3
	// t15LatencyBudget is the maximum acceptable derive latency for the full pass.
	// Measured baseline: 2.6–2.8s (BenchmarkRenderAll_30k). Budget = 10s gives ×3.7
	// headroom. WP T15 names no explicit second value; 10s is the recorded budget (PC5).
	t15LatencyBudget = 10 * time.Second
	// t15RSSGrowthBudgetKB is the maximum acceptable RSS growth (in KB) for a single
	// RenderAll pass on the 30k fixture. Measured baseline: 71.8–93.9 MB growth
	// (checkpoint-F2.md finding 5 / PC5). Budget = 256 MB gives ×2.7 headroom.
	// Only asserted on Linux (requires /proc/self/status); silently skipped elsewhere.
	t15RSSGrowthBudgetKB = 256 * 1024 // 256 MB in KB
)

// gen30kFixture creates a synthetic 30k-unit, ~90k-dep fixture.
// Units are spread across 20 groups (simulating a large mono-repo).
// Deps form a sparse but realistic cross-group dependency graph.
// A small number of cycles are planted to exercise Tarjan.
func gen30kFixture() ([]UnitFact, []DepFact) {
	const numGroups = 20
	groupNames := make([]string, numGroups)
	for i := range groupNames {
		groupNames[i] = fmt.Sprintf("group%02d", i)
	}

	units := make([]UnitFact, t15UnitCount)
	for i := range units {
		groupIdx := i % numGroups
		g := groupNames[groupIdx]
		units[i] = UnitFact{
			ID:    fmt.Sprintf("u%06d", i),
			Label: fmt.Sprintf("unit-%d", i),
			Path:  fmt.Sprintf("internal/%s/file%d.go", g, i),
			File:  fmt.Sprintf("internal/%s/file%d.go", g, i),
			Line:  1,
		}
	}

	// Build deps: each unit imports the next 3 units in different groups.
	deps := make([]DepFact, 0, t15UnitCount*t15DepsPerUnit)
	for i := 0; i < t15UnitCount; i++ {
		for d := 1; d <= t15DepsPerUnit; d++ {
			toIdx := (i + d*numGroups) % t15UnitCount
			if toIdx == i {
				continue
			}
			deps = append(deps, DepFact{
				FromUnit: fmt.Sprintf("u%06d", i),
				ToUnit:   fmt.Sprintf("u%06d", toIdx),
				Count:    1,
				File:     units[i].File,
				Line:     uint32(d),
			})
		}
	}

	// Plant 3 explicit cycles to exercise Tarjan.
	// Cycle 1: u000000 → u001000 → u002000 → u000000
	deps = append(deps,
		DepFact{FromUnit: "u000000", ToUnit: "u001000", Count: 1, File: "cycle1.go", Line: 1},
		DepFact{FromUnit: "u001000", ToUnit: "u002000", Count: 1, File: "cycle1.go", Line: 2},
		DepFact{FromUnit: "u002000", ToUnit: "u000000", Count: 1, File: "cycle1.go", Line: 3},
	)

	return units, deps
}

// maxRSSKilobytes reads the current process's peak RSS from /proc/self/status.
// Returns 0 on non-Linux platforms or if the file can't be read.
func maxRSSKilobytes() int64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseInt(fields[1], 10, 64)
				return v
			}
		}
	}
	return 0
}

// TestT15_30kFixture_LatencyBudget measures full RenderAll latency on a 30k-unit
// synthetic fixture and asserts it is within t15LatencyBudget.
// This validates that compact-time rendering is not on the critical path of a user action.
func TestT15_30kFixture_LatencyBudget(t *testing.T) {
	units, deps := gen30kFixture()
	require.Len(t, units, t15UnitCount, "fixture must have expected unit count")
	t.Logf("T15: generated %d units, %d deps", len(units), len(deps))

	rssBefore := maxRSSKilobytes()

	svc := &Service{}
	start := time.Now()
	shards, manifest, findings, err := svc.RenderAll("local", units, deps, nil, nil)
	elapsed := time.Since(start)

	require.NoError(t, err, "RenderAll must not error on 30k fixture")

	rssAfter := maxRSSKilobytes()
	rssGrowth := rssAfter - rssBefore

	t.Logf("T15 results: latency=%s views=%d findings=%d RSS_before=%dKB RSS_after=%dKB RSS_growth=%dKB",
		elapsed, len(manifest.Views), len(findings), rssBefore, rssAfter, rssGrowth)
	t.Logf("T15 shard sizes: component=%dB dsm=%dB cycles=%dB",
		len(shards["component"]), len(shards["dsm"]), len(shards["cycles"]))

	// Assert latency budget.
	if elapsed > t15LatencyBudget {
		t.Errorf("T15 BREACH: RenderAll latency %s > budget %s on 30k fixture", elapsed, t15LatencyBudget)
	} else {
		t.Logf("T15 PASS: latency %s ≤ budget %s", elapsed, t15LatencyBudget)
	}

	// Assert RSS growth budget (Linux only; skipped on non-Linux where maxRSSKilobytes returns 0).
	if rssAfter > 0 {
		if rssGrowth > t15RSSGrowthBudgetKB {
			t.Errorf("T15 RSS BREACH: growth %dKB (%dMB) > budget %dKB (%dMB) on 30k fixture",
				rssGrowth, rssGrowth/1024, t15RSSGrowthBudgetKB, t15RSSGrowthBudgetKB/1024)
		} else {
			t.Logf("T15 PASS RSS: growth %dKB (%dMB) ≤ budget %dKB (%dMB)",
				rssGrowth, rssGrowth/1024, t15RSSGrowthBudgetKB, t15RSSGrowthBudgetKB/1024)
		}
	}
}

// BenchmarkRenderAll_30k benchmarks the full derive pipeline on a 30k-unit fixture.
// Run with: go test ./internal/domain/arch/ -run=^$ -bench=BenchmarkRenderAll_30k -benchmem
func BenchmarkRenderAll_30k(b *testing.B) {
	units, deps := gen30kFixture()
	svc := &Service{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, err := svc.RenderAll("local", units, deps, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}
