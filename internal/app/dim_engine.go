package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corey/aoa/internal/adapters/recon"
	"github.com/corey/aoa/internal/adapters/reconrules"
	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/adapters/treesitter"
	"github.com/corey/aoa/internal/adapters/web"
	"github.com/corey/aoa/internal/domain/analyzer"
)

// initDimEngine loads YAML rules and creates the dimensional analysis engine.
// Falls back to hardcoded rules if YAML loading fails.
func (a *App) initDimEngine() {
	rules, err := analyzer.LoadRulesFromFS(reconrules.FS, "rules")
	if err != nil {
		fmt.Printf("[%s] YAML rules failed, using hardcoded fallback: %v\n",
			time.Now().Format(time.RFC3339), err)
		rules = analyzer.AllRules()
	}
	a.dimRules = rules

	// Populate the web server's rule index for data-driven tier/dim resolution
	web.SetRuleIndex(rules)

	if tsParser, ok := a.Parser.(*treesitter.Parser); ok && tsParser != nil {
		a.dimEngine = recon.NewEngine(rules, tsParser)
		fmt.Printf("[%s] dimensional engine: %d rules loaded (%d text, %d structural, %d composite)\n",
			time.Now().Format(time.RFC3339), len(rules),
			countRuleKind(rules, analyzer.RuleText),
			countRuleKind(rules, analyzer.RuleStructural),
			countRuleKind(rules, analyzer.RuleComposite))
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

// warmDimCache runs the dimensional engine over all indexed files and populates
// the dimensional results cache. Falls back to old scanner if no engine.
func (a *App) warmDimCache() {
	if a.dimEngine == nil {
		return
	}

	results := make(map[string]*socket.DimensionalFileResult)

	for fileID, fm := range a.Index.Files {
		absPath := filepath.Join(a.ProjectRoot, fm.Path)
		source, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}

		isTest := strings.HasSuffix(fm.Path, "_test.go") ||
			strings.HasSuffix(fm.Path, "_test.py") ||
			strings.HasSuffix(fm.Path, ".test.js") ||
			strings.HasSuffix(fm.Path, ".test.ts") ||
			strings.HasSuffix(fm.Path, ".spec.js") ||
			strings.HasSuffix(fm.Path, ".spec.ts")
		isMain := strings.Contains(fm.Path, "cmd/") ||
			fm.Path == "main.go" ||
			strings.HasSuffix(fm.Path, "/main.go")

		fa := a.dimEngine.AnalyzeFile(fm.Path, source, isTest, isMain)
		if fa == nil {
			continue
		}

		results[fm.Path] = convertFileAnalysis(fa, fileID)
	}

	a.reconMu.Lock()
	a.dimCache = results
	a.dimCacheSet = true
	a.reconMu.Unlock()
}

// updateDimForFile incrementally updates dimensional results for a single file.
func (a *App) updateDimForFile(fileID uint32, relPath string) {
	if a.dimEngine == nil {
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

	// Read and analyze the file
	absPath := filepath.Join(a.ProjectRoot, relPath)
	source, err := os.ReadFile(absPath)
	if err != nil {
		delete(a.dimCache, relPath)
		return
	}

	isTest := strings.HasSuffix(relPath, "_test.go") ||
		strings.HasSuffix(relPath, "_test.py") ||
		strings.HasSuffix(relPath, ".test.js") ||
		strings.HasSuffix(relPath, ".test.ts") ||
		strings.HasSuffix(relPath, ".spec.js") ||
		strings.HasSuffix(relPath, ".spec.ts")
	isMain := strings.Contains(relPath, "cmd/") ||
		relPath == "main.go" ||
		strings.HasSuffix(relPath, "/main.go")

	fa := a.dimEngine.AnalyzeFile(relPath, source, isTest, isMain)
	if fa == nil {
		delete(a.dimCache, relPath)
		return
	}

	a.dimCache[relPath] = convertFileAnalysis(fa, fileID)
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
