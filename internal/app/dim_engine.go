//go:build core

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
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

	cached := 0
	scanStart := time.Now()

	// Separate files into cached vs needing scan
	type scanJob struct {
		fileID uint32
		path   string
		source []byte
	}
	var jobs []scanJob

	for fileID, fm := range a.Index.Files {
		if persisted != nil {
			if fa, ok := persisted[fm.Path]; ok {
				if fm.LastModified <= fa.ScanTime/1e6 {
					results[fm.Path] = convertFileAnalysis(fa, fileID)
					analyses[fm.Path] = fa
					cached++
					continue
				}
			}
		}

		source, ok := a.getFileSource(fileID, fm.Path)
		if !ok {
			continue
		}
		jobs = append(jobs, scanJob{fileID: fileID, path: fm.Path, source: source})
	}

	// Parallel scan with worker pool
	var scanned int64
	var processed int64
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}
	if numWorkers < 2 {
		numWorkers = 2
	}

	type scanResult struct {
		path   string
		fileID uint32
		dr     *socket.DimensionalFileResult
		fa     *analyzer.FileAnalysis
	}

	resultCh := make(chan scanResult, numWorkers*4)
	var wg sync.WaitGroup

	// Fan out jobs across workers
	jobCh := make(chan scanJob, numWorkers*2)
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				fa := engine.AnalyzeFile(job.path, job.source, isTestFile(job.path), isMainFile(job.path))
				p := atomic.AddInt64(&processed, 1)
				if fa != nil {
					atomic.AddInt64(&scanned, 1)
					resultCh <- scanResult{
						path:   job.path,
						fileID: job.fileID,
						dr:     convertFileAnalysis(fa, job.fileID),
						fa:     fa,
					}
				}
				if p%500 == 0 {
					logFn(fmt.Sprintf("dim scan: %d/%d files (%.1fs elapsed, %d cached, %d scanned)...",
						int64(cached)+p, total, time.Since(scanStart).Seconds(), cached, atomic.LoadInt64(&scanned)))
				}
			}
		}()
	}

	// Collector goroutine — gathers results into maps
	done := make(chan struct{})
	go func() {
		for sr := range resultCh {
			results[sr.path] = sr.dr
			analyses[sr.path] = sr.fa
		}
		close(done)
	}()

	// Feed jobs
	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)
	wg.Wait()
	close(resultCh)
	<-done

	a.reconMu.Lock()
	a.dimCache = results
	a.dimCacheSet = true
	a.reconMu.Unlock()

	// Persist to bbolt so subsequent restarts can skip re-scanning
	if scanned > 0 {
		_ = a.Store.SaveAllDimensions(a.ProjectID, analyses)
	}

	return cached, int(scanned), firstRun
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
