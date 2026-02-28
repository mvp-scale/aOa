package app

import "github.com/corey/aoa/internal/adapters/socket"

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
func (a *App) loadDimensionalFromStore() map[string]*socket.DimensionalFileResult {
	if a.Store == nil {
		return nil
	}
	analyses, err := a.Store.LoadAllDimensions(a.ProjectID)
	if err != nil || analyses == nil {
		return nil
	}

	results := make(map[string]*socket.DimensionalFileResult, len(analyses))
	for path, fa := range analyses {
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

// invalidateDimCache clears the dimensional results cache so the next call reloads from bbolt.
func (a *App) invalidateDimCache() {
	a.reconMu.Lock()
	a.dimCache = nil
	a.dimCacheSet = false
	a.reconMu.Unlock()
}
