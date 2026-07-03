//go:build race

package arch

// raceDetectorEnabled is true under `go test -race`. Race instrumentation
// inflates wall-clock latency and RSS several-fold, so T15's numeric budget
// assertions are meaningless in that mode — the values are still logged, only
// the pass/fail budget check is skipped (see TestT15_30kFixture_LatencyBudget).
const raceDetectorEnabled = true
