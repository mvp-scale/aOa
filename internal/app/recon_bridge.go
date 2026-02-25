package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/corey/aoa/internal/adapters/socket"
)

// ReconBridge discovers and invokes the aoa-recon companion binary.
// Discovery order: exec.LookPath("aoa-recon") → .aoa/bin/aoa-recon → alongside aoa binary.
type ReconBridge struct {
	binaryPath string // empty = not found
}

// NewReconBridge probes for the aoa-recon binary.
func NewReconBridge(paths *Paths) *ReconBridge {
	rb := &ReconBridge{}

	// 1. PATH lookup (npm install -g puts it here)
	if path, err := exec.LookPath("aoa-recon"); err == nil {
		rb.binaryPath = path
		return rb
	}

	// 2. Project-local .aoa/bin/
	if _, err := os.Stat(paths.ReconBin); err == nil {
		rb.binaryPath = paths.ReconBin
		return rb
	}

	// 3. Alongside the aoa binary
	if exePath, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(exePath), "aoa-recon")
		if _, err := os.Stat(sibling); err == nil {
			rb.binaryPath = sibling
			return rb
		}
	}

	return rb
}

// Available returns true if the aoa-recon binary was found.
func (rb *ReconBridge) Available() bool {
	return rb.binaryPath != ""
}

// Path returns the discovered binary path, or empty string if not found.
func (rb *ReconBridge) Path() string {
	return rb.binaryPath
}

// Enhance runs a full project scan: aoa-recon enhance --db <dbpath> --root <project>.
// Returns stdout output and any error.
func (rb *ReconBridge) Enhance(dbPath, projectRoot string) (string, error) {
	if rb.binaryPath == "" {
		return "", fmt.Errorf("aoa-recon not available")
	}

	cmd := exec.Command(rb.binaryPath, "enhance", "--db", dbPath, "--root", projectRoot)
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("aoa-recon enhance: %w\n%s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// EnhanceFile runs an incremental single-file update:
// aoa-recon enhance-file --db <dbpath> --file <path>.
func (rb *ReconBridge) EnhanceFile(dbPath, filePath string) (string, error) {
	if rb.binaryPath == "" {
		return "", fmt.Errorf("aoa-recon not available")
	}

	cmd := exec.Command(rb.binaryPath, "enhance-file", "--db", dbPath, "--file", filePath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("aoa-recon enhance-file: %w\n%s", err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

// reconBridge is the App-level field for the recon bridge. Set during New().
// Not exported because it's an implementation detail.

// initReconBridge sets up the recon bridge, but only if recon has been
// explicitly enabled via `aoa recon init` (which writes .aoa/recon/enabled).
// This ensures pure aOa never probes for or depends on aoa-recon.
func (a *App) initReconBridge() {
	if _, err := os.Stat(a.Paths.ReconEnabled); err != nil {
		return // recon not enabled
	}
	a.reconBridge = NewReconBridge(a.Paths)
}

// ReconAvailable returns true if aoa-recon is installed and discoverable.
// Used by the dashboard API to decide whether to show the install prompt.
func (a *App) ReconAvailable() bool {
	return a.reconBridge != nil && a.reconBridge.Available()
}

// TriggerReconEnhance runs aoa-recon enhance in the background after init/reindex.
// Non-blocking: spawns a goroutine. Errors are logged but not fatal.
// After success, invalidates the dimensional cache so the next poll picks up fresh data.
func (a *App) TriggerReconEnhance() {
	if a.reconBridge == nil || !a.reconBridge.Available() {
		return
	}
	go func() {
		output, err := a.reconBridge.Enhance(a.dbPath, a.ProjectRoot)
		if err != nil {
			fmt.Printf("[%s] recon enhance failed: %v\n", time.Now().Format(time.RFC3339), err)
			return
		}
		if output != "" {
			fmt.Printf("[%s] %s\n", time.Now().Format(time.RFC3339), output)
		}
		// Invalidate dimensional cache so next poll reloads from bbolt
		a.invalidateDimCache()
	}()
}

// DimensionalResults returns persisted dimensional analysis results from bbolt.
// Returns nil if no dimensional data exists (aoa-recon hasn't run yet).
// Results are cached in memory; cache is invalidated when aoa-recon completes.
// Implements socket.AppQueries.
func (a *App) DimensionalResults() map[string]*socket.DimensionalFileResult {
	a.reconMu.RLock()
	if a.dimCacheSet {
		cached := a.dimCache
		a.reconMu.RUnlock()
		return cached
	}
	a.reconMu.RUnlock()

	// Cache miss — load from bbolt and cache the result
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

// TriggerReconEnhanceFile runs aoa-recon enhance-file in the background after a file change.
// After success, invalidates the dimensional cache.
func (a *App) TriggerReconEnhanceFile(absPath string) {
	if a.reconBridge == nil || !a.reconBridge.Available() {
		return
	}
	go func() {
		_, err := a.reconBridge.EnhanceFile(a.dbPath, absPath)
		if err != nil {
			// Silently ignore — incremental enhancement is best-effort.
			return
		}
		// Invalidate dimensional cache so next poll reloads from bbolt
		a.invalidateDimCache()
	}()
}
