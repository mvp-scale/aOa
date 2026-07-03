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
// P4 widened scope (checkpoint-F1 finding 5): the flag-on arm times the FULL
// boot delta, not parse-only — extraction + §2.4 resolve + grouping + edge
// PERSISTENCE into a real bbolt store via ReplaceAllEdges/SaveUnresolved,
// exactly the arch-only work WarmCaches/Reindex add when ArchEnabled=true.
// (SaveIndex is identical in both arms and excluded from both.)
//
// Env-gated so it never runs in the normal gauntlet:
//   AOA_T1=1 go test -run TestT1KeystoneF1 -timeout 60m ./internal/app/ -v
// Corpus: AOA_T1_CORPUS (default /home/corey/kubernetes).
// Pairs:  AOA_T1_PAIRS (default 4; 2 = quick confirm).

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/corey/aoa/internal/adapters/bbolt"
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/domain/facts"
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

	pairs := 4 // T28: default 4 interleaved (off,on) pairs
	if p := os.Getenv("AOA_T1_PAIRS"); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 {
			t.Fatalf("AOA_T1_PAIRS must be a positive integer, got %q", p)
		}
		pairs = n
	}

	type runResult struct {
		archOn    bool
		elapsed   time.Duration
		files     int
		edges     int    // extracted (parse-pass) edge count
		persisted int    // edge rows readable back from bbolt (ON arm only)
		heapInuse uint64 // bytes, index still live
		sys       uint64 // bytes obtained from OS
	}

	runOnce := func(runN int, archOn bool) runResult {
		runtime.GC()
		parser := treesitter.NewParser()

		// P4: real bbolt store per ON run (fresh DB in TempDir). Opening the DB
		// is outside the timed window — production has the DB open either way;
		// the flag delta is the arch-only WRITE work, which is timed below.
		var store *bbolt.Store
		if archOn {
			s, err := bbolt.NewStore(filepath.Join(t.TempDir(), fmt.Sprintf("t1-run%d.db", runN)))
			if err != nil {
				t.Fatalf("bbolt.NewStore: %v", err)
			}
			defer s.Close()
			store = s
		}

		start := time.Now()
		idx, res, edges, err := BuildIndexWithFacts(corpus, parser, archOn)
		if err != nil {
			t.Fatalf("BuildIndexWithFacts (arch=%v): %v", archOn, err)
		}
		// P4 widened scope: the flag-on boot path's arch-only tail, timed.
		// Mirrors WarmCaches/Reindex exactly: resolve → group → persist.
		if archOn && len(edges) > 0 {
			manifests := facts.ReadManifests(corpus)
			fileSet := buildFileSet(idx)
			rr := facts.Resolve(edges, fileSet, manifests)
			byFile := groupEdgesByFile(idx, rr.Resolved)
			if err := store.ReplaceAllEdges("t1", byFile); err != nil {
				t.Fatalf("ReplaceAllEdges: %v", err)
			}
			if len(rr.Unresolved) > 0 {
				if err := store.SaveUnresolved("t1", rr.Unresolved); err != nil {
					t.Fatalf("SaveUnresolved: %v", err)
				}
			}
		}
		el := time.Since(start)

		// Work proof for the persistence leg (outside the timed window):
		// edges must be readable back from the real store.
		persisted := 0
		if archOn {
			rows, err := store.LoadAllEdges("t1")
			if err != nil {
				t.Fatalf("LoadAllEdges: %v", err)
			}
			persisted = len(rows)
		}

		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return runResult{
			archOn:    archOn,
			elapsed:   el,
			files:     res.FileCount,
			edges:     len(edges),
			persisted: persisted,
			heapInuse: ms.HeapInuse,
			sys:       ms.Sys,
		}
	}

	var results []runResult
	// T28: interleaved per iteration. R7 (residual warm-up bias): ALTERNATE the
	// within-pair order — pair 1 (off,on), pair 2 (on,off), ... — so neither arm
	// systematically benefits from the other's page-cache warm-up.
	runN := 0
	next := func(archOn bool) {
		runN++
		results = append(results, runOnce(runN, archOn))
	}
	for i := 0; i < pairs; i++ {
		if i%2 == 0 {
			next(false)
			next(true)
		} else {
			next(true)
			next(false)
		}
	}

	fmt.Printf("\n=== T1 RE-RUN (F1 exit / T28+T29, P4-widened: ON arm incl. resolve+persist) — corpus %s, %d interleaved pairs (R7 alternating) ===\n", corpus, pairs)
	fmt.Printf("%-4s %-8s %-12s %-8s %-10s %-10s %-12s %-12s\n",
		"run", "arch", "elapsed", "files", "edges", "persisted", "heapInuse", "sys")
	var offTimes, onTimes []time.Duration
	for i, r := range results {
		mode := "OFF"
		if r.archOn {
			mode = "ON"
		}
		fmt.Printf("%-4d %-8s %-12v %-8d %-10d %-10d %-12.1f %-12.1f\n",
			i+1, mode, r.elapsed.Round(time.Millisecond), r.files, r.edges, r.persisted,
			float64(r.heapInuse)/(1<<20), float64(r.sys)/(1<<20))
		if r.archOn {
			onTimes = append(onTimes, r.elapsed)
			// T29: prove the arm did work — every ON run must extract > 50K imports.
			if r.edges <= 50000 {
				t.Errorf("T29 FAIL: flag-on run %d extracted only %d import edges (need > 50000)", i+1, r.edges)
			}
			// P4: prove the persistence leg did work — edges readable back from bbolt.
			if r.persisted == 0 {
				t.Errorf("P4 FAIL: flag-on run %d persisted 0 edge rows to the real store", i+1)
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
