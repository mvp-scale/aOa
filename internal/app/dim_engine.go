//go:build !lean

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corey/aoa/internal/adapters/recon"
	"github.com/corey/aoa/internal/adapters/socket"
	reconfs "github.com/corey/aoa/recon"
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/adapters/web"
	"github.com/corey/aoa/internal/domain/analyzer"
)

// initDimEngine loads YAML rules and creates the dimensional analysis engine.
// Falls back to hardcoded rules if YAML loading fails.
func (a *App) initDimEngine() {
	rules, err := analyzer.LoadRulesFromFS(reconfs.FS, "rules")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] YAML rules failed, using hardcoded fallback: %v\n",
			time.Now().Format(time.RFC3339), err)
		rules = analyzer.AllRules()
	}
	a.dimRules = rules

	// Populate the web server's rule index for data-driven tier/dim resolution
	web.SetRuleIndex(rules)

	if tsParser, ok := a.Parser.(*treesitter.Parser); ok && tsParser != nil {
		a.dimEngine = recon.NewEngine(rules, tsParser)
	}
}

func countRuleKind(rules []analyzer.Rule, kind analyzer.RuleKind) int {
	n := 0
	for _, r := range rules {
		if r.Kind == kind {
			n++
		}
	}
	return n
}

// warmDimCache loads persisted dimensional analysis results from bbolt, then
// incrementally rescans only files that are new or modified since last scan.
// Persists the final results to bbolt for fast subsequent restarts.
// Returns (cached, scanned, firstRun) counts for the caller to log.
// logFn receives progress messages during scanning.
func (a *App) warmDimCache(logFn func(string)) (int, int, bool) {
	engine, _ := a.dimEngine.(*recon.Engine)
	if engine == nil {
		return 0, 0, false
	}

	total := len(a.Index.Files)
	results := make(map[string]*socket.DimensionalFileResult, total)
	analyses := make(map[string]*analyzer.FileAnalysis, total)

	// Try to load persisted results from bbolt
	persisted, err := a.Store.LoadAllDimensions(a.ProjectID)
	if err != nil {
		persisted = nil
	}
	firstRun := persisted == nil

	if firstRun {
		logFn(fmt.Sprintf("building dimensional cache (first run) — %d files to scan...", total))
	}

	scanned := 0
	cached := 0
	processed := 0
	scanStart := time.Now()
	for fileID, fm := range a.Index.Files {
		// Check if persisted result exists and is still fresh
		if persisted != nil {
			if fa, ok := persisted[fm.Path]; ok {
				// ScanTime is in microseconds; LastModified is in seconds
				if fm.LastModified <= fa.ScanTime/1e6 {
					results[fm.Path] = convertFileAnalysis(fa, fileID)
					analyses[fm.Path] = fa
					cached++
					processed++
					continue
				}
			}
		}

		// File is new or stale — try file cache first, fall back to disk
		source, ok := a.getFileSource(fileID, fm.Path)
		if !ok {
			processed++
			continue
		}

		fa := engine.AnalyzeFile(fm.Path, source, isTestFile(fm.Path), isMainFile(fm.Path))
		if fa == nil {
			processed++
			continue
		}

		results[fm.Path] = convertFileAnalysis(fa, fileID)
		analyses[fm.Path] = fa
		scanned++
		processed++

		// Report progress every 500 files when actively scanning
		if scanned > 0 && processed%500 == 0 {
			logFn(fmt.Sprintf("dim scan: %d/%d files (%.1fs elapsed, %d cached, %d scanned)...",
				processed, total, time.Since(scanStart).Seconds(), cached, scanned))
		}
	}

	a.reconMu.Lock()
	a.dimCache = results
	a.dimCacheSet = true
	a.reconMu.Unlock()

	// Persist to bbolt so subsequent restarts can skip re-scanning
	if scanned > 0 {
		_ = a.Store.SaveAllDimensions(a.ProjectID, analyses)
	}

	return cached, scanned, firstRun
}

func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go") ||
		strings.HasSuffix(path, "_test.py") ||
		strings.HasSuffix(path, ".test.js") ||
		strings.HasSuffix(path, ".test.ts") ||
		strings.HasSuffix(path, ".spec.js") ||
		strings.HasSuffix(path, ".spec.ts")
}

func isMainFile(path string) bool {
	return strings.Contains(path, "cmd/") ||
		path == "main.go" ||
		strings.HasSuffix(path, "/main.go")
}

// updateDimForFile incrementally updates dimensional results for a single file.
func (a *App) updateDimForFile(fileID uint32, relPath string) {
	engine, _ := a.dimEngine.(*recon.Engine)
	if engine == nil {
		return
	}

	a.reconMu.Lock()
	defer a.reconMu.Unlock()

	if a.dimCache == nil {
		a.dimCache = make(map[string]*socket.DimensionalFileResult)
		a.dimCacheSet = true
	}

	// Check if file was deleted
	fm := a.Index.Files[fileID]
	if fm == nil {
		delete(a.dimCache, relPath)
		return
	}

	// Read file — try cache first, fall back to disk
	source, ok := a.getFileSource(fileID, relPath)
	if !ok {
		delete(a.dimCache, relPath)
		return
	}

	fa := engine.AnalyzeFile(relPath, source, isTestFile(relPath), isMainFile(relPath))
	if fa == nil {
		delete(a.dimCache, relPath)
		return
	}

	a.dimCache[relPath] = convertFileAnalysis(fa, fileID)
}

// getFileSource returns file content, checking the file cache first and falling
// back to os.ReadFile. Returns nil, false if the file cannot be read.
func (a *App) getFileSource(fileID uint32, relPath string) ([]byte, bool) {
	if cache := a.Engine.Cache(); cache != nil {
		if content, ok := cache.GetContent(fileID); ok {
			return content, true
		}
	}
	absPath := filepath.Join(a.ProjectRoot, relPath)
	source, err := os.ReadFile(absPath)
	if err != nil {
		return nil, false
	}
	return source, true
}

// convertFileAnalysis converts analyzer.FileAnalysis to socket.DimensionalFileResult.
func convertFileAnalysis(fa *analyzer.FileAnalysis, fileID uint32) *socket.DimensionalFileResult {
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
	return &socket.DimensionalFileResult{
		Path:     fa.Path,
		Language: fa.Language,
		Bitmask:  fa.Bitmask,
		Methods:  methods,
		Findings: fileFindings,
		ScanTime: fa.ScanTime,
	}
}
