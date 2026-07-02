//go:build !lean

package app

// T1 re-run (F1 exit, L19.13) under the T28/T29 binding conditions from
// checkpoint-F0 P4:
//   - T28: INTERLEAVED A/B per iteration (off,on,off,on,...) — not sequential
//     blocks — with the per-run raw table preserved and memory headroom recorded.
//   - T29: the harness must PROVE the arm did work — assert extracted-import
//     count > 50K on every flag-on run.
//
// Unlike the L19.6 throwaway spike (branch spike/f0-keystone-bench), this
// measures the PRODUCTION E1 path: BuildIndexWithFacts with archEnabled
// true/false — the exact C4 construction-time switch the daemon uses.
//
// Env-gated so it never runs in the normal gauntlet:
//   AOA_T1=1 go test -run TestT1KeystoneF1 -timeout 60m ./internal/app/ -v
// Corpus: AOA_T1_CORPUS (default /home/corey/kubernetes).

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/corey/aoa/internal/adapters/treesitter"
)

func TestT1KeystoneF1(t *testing.T) {
	if os.Getenv("AOA_T1") == "" {
		t.Skip("T1 benchmark: set AOA_T1=1 to run (11+ min on the kubernetes corpus)")
	}
	corpus := os.Getenv("AOA_T1_CORPUS")
	if corpus == "" {
		corpus = "/home/corey/kubernetes"
	}
	if _, err := os.Stat(corpus); err != nil {
		t.Skipf("corpus not present: %s", corpus)
	}

	const pairs = 4 // T28: 4 interleaved (off,on) pairs

	type runResult struct {
		archOn    bool
		elapsed   time.Duration
		files     int
		edges     int
		heapInuse uint64 // bytes, index still live
		sys       uint64 // bytes obtained from OS
	}

	runOnce := func(archOn bool) runResult {
		runtime.GC()
		parser := treesitter.NewParser()
		start := time.Now()
		_, res, edges, err := BuildIndexWithFacts(corpus, parser, archOn)
		el := time.Since(start)
		if err != nil {
			t.Fatalf("BuildIndexWithFacts (arch=%v): %v", archOn, err)
		}
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return runResult{
			archOn:    archOn,
			elapsed:   el,
			files:     res.FileCount,
			edges:     len(edges),
			heapInuse: ms.HeapInuse,
			sys:       ms.Sys,
		}
	}

	var results []runResult
	// T28: interleaved per iteration — off,on,off,on,off,on,off,on.
	for i := 0; i < pairs; i++ {
		results = append(results, runOnce(false))
		results = append(results, runOnce(true))
	}

	fmt.Printf("\n=== T1 RE-RUN (F1 exit / T28+T29) — corpus %s, %d interleaved pairs ===\n", corpus, pairs)
	fmt.Printf("%-4s %-8s %-12s %-8s %-10s %-12s %-12s\n",
		"run", "arch", "elapsed", "files", "edges", "heapInuse", "sys")
	var offTimes, onTimes []time.Duration
	for i, r := range results {
		mode := "OFF"
		if r.archOn {
			mode = "ON"
		}
		fmt.Printf("%-4d %-8s %-12v %-8d %-10d %-12.1f %-12.1f\n",
			i+1, mode, r.elapsed.Round(time.Millisecond), r.files, r.edges,
			float64(r.heapInuse)/(1<<20), float64(r.sys)/(1<<20))
		if r.archOn {
			onTimes = append(onTimes, r.elapsed)
			// T29: prove the arm did work — every ON run must extract > 50K imports.
			if r.edges <= 50000 {
				t.Errorf("T29 FAIL: flag-on run %d extracted only %d import edges (need > 50000)", i+1, r.edges)
			}
		} else {
			offTimes = append(offTimes, r.elapsed)
			// C4 sanity in the same harness: flag-off must emit zero edges.
			if r.edges != 0 {
				t.Errorf("C4 FAIL: flag-off run %d emitted %d edges (must be 0)", i+1, r.edges)
			}
		}
	}

	// p50 for even n: mean of the two middle values.
	p50 := func(ds []time.Duration) time.Duration {
		s := append([]time.Duration(nil), ds...)
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
		n := len(s)
		if n%2 == 1 {
			return s[n/2]
		}
		return (s[n/2-1] + s[n/2]) / 2
	}

	offP50, onP50 := p50(offTimes), p50(onTimes)
	delta := (float64(onP50) - float64(offP50)) / float64(offP50) * 100

	fmt.Printf("\nOFF p50: %v   ON p50: %v   delta: %+.2f%%   (gate: <= +3%%)\n",
		offP50.Round(time.Millisecond), onP50.Round(time.Millisecond), delta)
	fmt.Printf("OFF raw: %v\nON  raw: %v\n", offTimes, onTimes)

	if delta > 3.0 {
		t.Errorf("T1 RED: delta %+.2f%% exceeds the +3%% G0 budget — L19.6 fallback plan required", delta)
	} else {
		fmt.Printf("T1 VERDICT: GREEN — production E1 rides the always-on parse within budget\n")
	}
}
