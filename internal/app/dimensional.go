package app

import (
	"sync/atomic"
	"time"

	"github.com/corey/aoa/internal/adapters/socket"
)

// ReconAvailable returns true if dimensional analysis is available.
// In the core build this is true when the dimensional engine is loaded.
// In the lean build this is always false.
func (a *App) ReconAvailable() bool {
	return a.dimEngine != nil
}

// DimensionalResults returns cached dimensional analysis results, loading from
// bbolt on first access. Thread-safe via reconMu.
func (a *App) DimensionalResults() map[string]*socket.DimensionalFileResult {
	a.reconMu.RLock()
	if a.dimCacheSet {
		cached := a.dimCache
		a.reconMu.RUnlock()
		return cached
	}
	a.reconMu.RUnlock()

	results := a.loadDimensionalFromStore()

	a.reconMu.Lock()
	a.dimCache = results
	a.dimCacheSet = true
	a.reconMu.Unlock()

	return results
}

// loadDimensionalFromStore reads dimensional analysis from bbolt and converts to DTOs.
// Only includes files that exist in the current index (respects gitignore changes).
func (a *App) loadDimensionalFromStore() map[string]*socket.DimensionalFileResult {
	if a.Store == nil {
		return nil
	}
	analyses, err := a.Store.LoadAllDimensions(a.ProjectID)
	if err != nil || analyses == nil {
		return nil
	}

	// Build set of indexed paths for fast lookup
	indexedPaths := make(map[string]bool, len(a.Index.Files))
	for _, fm := range a.Index.Files {
		indexedPaths[fm.Path] = true
	}

	results := make(map[string]*socket.DimensionalFileResult, len(analyses))
	for path, fa := range analyses {
		if !indexedPaths[path] {
			continue // file no longer in index (gitignored, deleted, etc.)
		}
		methods := make([]socket.DimensionalMethodResult, len(fa.Methods))
		for i, m := range fa.Methods {
			findings := make([]socket.DimensionalFindingResult, len(m.Findings))
			for j, f := range m.Findings {
				findings[j] = socket.DimensionalFindingResult{
					RuleID:   f.RuleID,
					Line:     f.Line,
					Symbol:   f.Symbol,
					Severity: int(f.Severity),
				}
			}
			methods[i] = socket.DimensionalMethodResult{
				Name:     m.Name,
				Line:     m.Line,
				EndLine:  m.EndLine,
				Bitmask:  m.Bitmask,
				Score:    m.Score,
				Findings: findings,
			}
		}
		fileFindings := make([]socket.DimensionalFindingResult, len(fa.Findings))
		for j, f := range fa.Findings {
			fileFindings[j] = socket.DimensionalFindingResult{
				RuleID:   f.RuleID,
				Line:     f.Line,
				Symbol:   f.Symbol,
				Severity: int(f.Severity),
			}
		}
		results[path] = &socket.DimensionalFileResult{
			Path:     fa.Path,
			Language: fa.Language,
			Bitmask:  fa.Bitmask,
			Methods:  methods,
			Findings: fileFindings,
			ScanTime: fa.ScanTime,
		}
	}
	return results
}

// DimScanProgress returns the current progress of the background dimensional scan.
// All fields are read via atomics — no lock needed.
func (a *App) DimScanProgress() socket.DimScanProgress {
	running := atomic.LoadInt32(&a.dimScanRunning) == 1
	total := int(atomic.LoadInt64(&a.dimScanTotal))
	done := int(atomic.LoadInt64(&a.dimScanDone))
	cached := int(atomic.LoadInt64(&a.dimScanCached))
	startedNano := atomic.LoadInt64(&a.dimScanStarted)

	var pct float64
	if total > 0 {
		pct = float64(done) / float64(total) * 100
	}

	var elapsed, eta float64
	if startedNano > 0 {
		elapsed = time.Since(time.Unix(0, startedNano)).Seconds()
		if done > 0 && running {
			rate := elapsed / float64(done)
			eta = rate * float64(total-done)
		}
	}

	return socket.DimScanProgress{
		Running: running,
		Total:   total,
		Done:    done,
		Cached:  cached,
		Pct:     pct,
		Elapsed: elapsed,
		ETA:     eta,
	}
}

// invalidateDimCache clears the dimensional results cache so the next call reloads from bbolt.
func (a *App) invalidateDimCache() {
	a.reconMu.Lock()
	a.dimCache = nil
	a.dimCacheSet = false
	a.reconMu.Unlock()
}
