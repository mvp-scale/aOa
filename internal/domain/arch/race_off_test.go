//go:build !race

package arch

// raceDetectorEnabled is false in normal builds. Wall-clock latency and RSS
// budgets (T15) are only meaningful without race instrumentation; the race
// variant of this const gates those numeric assertions off under `-race`.
const raceDetectorEnabled = false
