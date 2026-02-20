package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ReconBridge discovers and invokes the aoa-recon companion binary.
// Discovery order: exec.LookPath("aoa-recon") → .aoa/bin/aoa-recon → alongside aoa binary.
type ReconBridge struct {
	binaryPath string // empty = not found
}

// NewReconBridge probes for the aoa-recon binary.
func NewReconBridge(projectRoot string) *ReconBridge {
	rb := &ReconBridge{}

	// 1. PATH lookup (npm install -g puts it here)
	if path, err := exec.LookPath("aoa-recon"); err == nil {
		rb.binaryPath = path
		return rb
	}

	// 2. Project-local .aoa/bin/
	localBin := filepath.Join(projectRoot, ".aoa", "bin", "aoa-recon")
	if _, err := os.Stat(localBin); err == nil {
		rb.binaryPath = localBin
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

// initReconBridge sets up the recon bridge and stores it on the App.
func (a *App) initReconBridge() {
	a.reconBridge = NewReconBridge(a.ProjectRoot)
	if a.reconBridge.Available() {
		fmt.Printf("[%s] aoa-recon found at %s\n", time.Now().Format(time.RFC3339), a.reconBridge.Path())
	}
}

// ReconAvailable returns true if aoa-recon is installed and discoverable.
// Used by the dashboard API to decide whether to show the install prompt.
func (a *App) ReconAvailable() bool {
	return a.reconBridge != nil && a.reconBridge.Available()
}

// TriggerReconEnhance runs aoa-recon enhance in the background after init/reindex.
// Non-blocking: spawns a goroutine. Errors are logged but not fatal.
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
	}()
}

// TriggerReconEnhanceFile runs aoa-recon enhance-file in the background after a file change.
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
	}()
}
