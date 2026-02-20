package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// reconFinding represents a single detected issue in a file.
type reconFinding struct {
	Symbol   string `json:"symbol"`
	DimID    string `json:"dim_id"`
	TierID   string `json:"tier_id"`
	ID       string `json:"id"`
	Label    string `json:"label"`
	Severity string `json:"severity"`
	Line     int    `json:"line"`
}

// reconFileInfo holds findings and metadata for one file.
type reconFileInfo struct {
	Language string         `json:"language"`
	Symbols  []string       `json:"symbols"`
	Findings []reconFinding `json:"findings"`
}

// reconResult is the full API response for GET /api/recon.
type reconResult struct {
	FilesScanned  int                                `json:"files_scanned"`
	TotalFindings int                                `json:"total_findings"`
	Critical      int                                `json:"critical"`
	Warnings      int                                `json:"warnings"`
	Info          int                                `json:"info"`
	CleanFiles    int                                `json:"clean_files"`
	TierCounts    map[string]int                     `json:"tier_counts"`
	DimCounts     map[string]int                     `json:"dim_counts"`
	Tree          map[string]map[string]reconFileInfo `json:"tree"` // folder -> file -> info
}

// reconSymbolInfo holds a symbol's name and line range for enclosing-symbol lookups.
type reconSymbolInfo struct {
	name      string
	startLine uint16
	endLine   uint16
}

// reconPattern defines a text pattern to scan for.
type reconPattern struct {
	id       string
	label    string
	tierID   string
	dimID    string
	severity string
	// codeOnly: if true, skip non-code files (md, json, html, txt, yaml, etc.)
	codeOnly bool
	match    func(line string, lineNum int, isTest bool, isMain bool) bool
}

// scannable languages — only scan these for code-quality patterns.
var reconCodeExts = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	".rs": true, ".c": true, ".cpp": true, ".h": true, ".java": true, ".rb": true,
	".sh": true, ".bash": true, ".cs": true, ".swift": true, ".kt": true, ".scala": true,
	".zig": true, ".lua": true, ".php": true, ".pl": true, ".r": true,
}

var (
	reDeferInFor = regexp.MustCompile(`^\s*defer\s`)
	reForLoop    = regexp.MustCompile(`^\s*for\s`)
	rePkgVar     = regexp.MustCompile(`^var\s+\w+`)
	reTodoFixme  = regexp.MustCompile(`(?i)\b(TODO|FIXME|HACK|XXX)\b`)
	// hardcoded_secret: string literal assignment with secret-like name
	reSecretAssign = regexp.MustCompile(`(?i)(password|secret|api_key|apikey|private_key)\s*[:=].*["` + "`" + `']`)
	// ignored_error: specifically blank identifier discarding a function return
	reIgnoredErrCall = regexp.MustCompile(`_\s*[:=]=\s*\w+[\.(]`)
)

func buildReconPatterns() []reconPattern {
	return []reconPattern{
		// Security — hardcoded secrets (code files only, require string literal assignment)
		{
			id: "hardcoded_secret", label: "Potential hardcoded secret or credential",
			tierID: "security", dimID: "secrets", severity: "critical", codeOnly: true,
			match: func(line string, _ int, isTest, _ bool) bool {
				if isTest {
					return false
				}
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "*") {
					return false
				}
				return reSecretAssign.MatchString(line)
			},
		},
		// Security — command injection (code files only)
		{
			id: "command_injection", label: "Potential command injection via exec/system call",
			tierID: "security", dimID: "injection", severity: "critical", codeOnly: true,
			match: func(line string, _ int, isTest, _ bool) bool {
				if isTest {
					return false
				}
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					return false
				}
				for _, pat := range []string{"exec.Command(", "os.system(", "eval("} {
					if strings.Contains(line, pat) {
						return true
					}
				}
				return false
			},
		},
		// Security — weak hash (code files only)
		{
			id: "weak_hash", label: "MD5 or SHA1 used (weak for security purposes)",
			tierID: "security", dimID: "crypto", severity: "warning", codeOnly: true,
			match: func(line string, _ int, isTest, _ bool) bool {
				if isTest {
					return false
				}
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					return false
				}
				return strings.Contains(line, "md5.") || strings.Contains(line, "sha1.")
			},
		},
		// Security — insecure random (code files only)
		{
			id: "insecure_random", label: "math/rand used where crypto/rand may be needed",
			tierID: "security", dimID: "crypto", severity: "warning", codeOnly: true,
			match: func(line string, _ int, isTest, _ bool) bool {
				if isTest {
					return false
				}
				return strings.Contains(line, `"math/rand"`)
			},
		},
		// Performance — defer in loop (code files only, Go-specific)
		{
			id: "defer_in_loop", label: "defer inside loop body",
			tierID: "performance", dimID: "resources", severity: "warning", codeOnly: true,
			match: func(line string, _ int, _, _ bool) bool {
				return reDeferInFor.MatchString(line)
			},
		},
		// Quality — ignored error (code files only)
		{
			id: "ignored_error", label: "Error return value assigned to blank identifier",
			tierID: "quality", dimID: "errors", severity: "warning", codeOnly: true,
			match: func(line string, _ int, _, _ bool) bool {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					return false
				}
				// Must look like discarding a function call result: _ = foo( or _, _ := bar(
				// or multi-return: x, _ = foo( or x, _ := foo(
				if strings.HasPrefix(trimmed, "_ =") || strings.HasPrefix(trimmed, "_ :=") {
					// Verify it's a function call, not just `_ = true` or assignment
					return strings.Contains(trimmed, "(") || strings.Contains(trimmed, ".")
				}
				if strings.Contains(trimmed, ", _ =") || strings.Contains(trimmed, ", _ :=") {
					return strings.Contains(trimmed, "(")
				}
				return false
			},
		},
		// Quality — panic in library (code files only)
		{
			id: "panic_in_lib", label: "panic() called in library/non-main package",
			tierID: "quality", dimID: "errors", severity: "warning", codeOnly: true,
			match: func(line string, _ int, _, isMain bool) bool {
				if isMain {
					return false
				}
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					return false
				}
				return strings.Contains(trimmed, "panic(")
			},
		},
		// Observability — print statements (code files only)
		{
			id: "print_statement", label: "Debug print statement left in code",
			tierID: "observability", dimID: "debug", severity: "info", codeOnly: true,
			match: func(line string, _ int, isTest, _ bool) bool {
				if isTest {
					return false
				}
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					return false
				}
				return strings.Contains(line, "fmt.Println(") ||
					strings.Contains(line, "fmt.Printf(") ||
					strings.Contains(line, "console.log(")
			},
		},
		// Observability — TODO/FIXME markers (all file types)
		{
			id: "todo_fixme", label: "TODO/FIXME/HACK/XXX marker in source",
			tierID: "observability", dimID: "debug", severity: "info", codeOnly: false,
			match: func(line string, _ int, _, _ bool) bool {
				return reTodoFixme.MatchString(line)
			},
		},
		// Architecture — mutable global state (code files only, Go-specific)
		{
			id: "global_state", label: "Mutable global variable at package level",
			tierID: "architecture", dimID: "antipattern", severity: "warning", codeOnly: true,
			match: func(line string, _ int, isTest, isMain bool) bool {
				if isTest || isMain {
					return false
				}
				// Must start at column 0 with "var " (no indentation = package level)
				if !strings.HasPrefix(line, "var ") {
					return false
				}
				// Skip var blocks opener "var ("
				trimmed := strings.TrimSpace(line)
				if trimmed == "var (" {
					return false
				}
				// Skip embedded FS and constants-like patterns
				if strings.Contains(line, "embed.FS") || strings.Contains(line, "//go:embed") {
					return false
				}
				// Skip regexp.MustCompile — compile-time constant, not mutable state
				if strings.Contains(line, "regexp.MustCompile") || strings.Contains(line, "regexp.Compile") {
					return false
				}
				return true
			},
		},
	}
}

func (s *Server) handleRecon(w http.ResponseWriter, r *http.Request) {
	if s.idx == nil || s.engine == nil {
		http.Error(w, `{"error":"index not available"}`, http.StatusServiceUnavailable)
		return
	}

	fc := s.engine.Cache()

	patterns := buildReconPatterns()

	result := reconResult{
		TierCounts: make(map[string]int),
		DimCounts:  make(map[string]int),
		Tree:       make(map[string]map[string]reconFileInfo),
	}

	// Build per-file symbol mapping from index metadata
	fileSymbols := make(map[uint32][]reconSymbolInfo)
	for ref, meta := range s.idx.Metadata {
		if meta == nil {
			continue
		}
		fileSymbols[ref.FileID] = append(fileSymbols[ref.FileID], reconSymbolInfo{
			name:      meta.Name,
			startLine: meta.StartLine,
			endLine:   meta.EndLine,
		})
	}
	// Sort symbols by start line for enclosing-symbol lookup
	for fid := range fileSymbols {
		syms := fileSymbols[fid]
		sort.Slice(syms, func(i, j int) bool {
			return syms[i].startLine < syms[j].startLine
		})
	}

	// Pre-check long functions from symbol metadata
	type longFunc struct {
		fileID uint32
		sym    reconSymbolInfo
	}
	var longFuncs []longFunc
	for fid, syms := range fileSymbols {
		for _, sym := range syms {
			if sym.endLine > 0 && int(sym.endLine)-int(sym.startLine) > 100 {
				longFuncs = append(longFuncs, longFunc{fileID: fid, sym: sym})
			}
		}
	}

	for fileID, fileMeta := range s.idx.Files {
		result.FilesScanned++

		dir := filepath.Dir(fileMeta.Path)
		base := filepath.Base(fileMeta.Path)
		ext := filepath.Ext(base)

		isCodeFile := reconCodeExts[ext]
		isTest := strings.HasSuffix(base, "_test.go") ||
			strings.HasSuffix(base, "_test.py") ||
			strings.HasSuffix(base, ".test.js") ||
			strings.HasSuffix(base, ".test.ts") ||
			strings.HasSuffix(base, "_test.rs") ||
			strings.Contains(base, "test_")
		isMain := strings.Contains(fileMeta.Path, "cmd/") ||
			base == "main.go" || base == "main.py"

		// Get symbol names for this file
		syms := fileSymbols[fileID]
		symbolNames := make([]string, len(syms))
		for i, s := range syms {
			symbolNames[i] = s.name
		}

		var findings []reconFinding

		// Scan file content if cache is available
		if fc != nil {
			lines := fc.GetLines(fileID)
			if lines != nil {
				forLoopDepth := 0
				for lineIdx, line := range lines {
					lineNum := lineIdx + 1

					// Track for-loop depth for defer-in-loop detection
					if reForLoop.MatchString(line) {
						forLoopDepth++
					}

					for _, pat := range patterns {
						// Skip code-only patterns on non-code files
						if pat.codeOnly && !isCodeFile {
							continue
						}

						// Special handling for defer_in_loop: only flag if inside a for loop
						if pat.id == "defer_in_loop" {
							if forLoopDepth > 0 && reDeferInFor.MatchString(line) {
								sym := findEnclosingSymbol(syms, lineNum)
								findings = append(findings, reconFinding{
									Symbol:   sym,
									DimID:    pat.dimID,
									TierID:   pat.tierID,
									ID:       pat.id,
									Label:    pat.label,
									Severity: pat.severity,
									Line:     lineNum,
								})
							}
							continue
						}

						if pat.match(line, lineNum, isTest, isMain) {
							sym := findEnclosingSymbol(syms, lineNum)
							findings = append(findings, reconFinding{
								Symbol:   sym,
								DimID:    pat.dimID,
								TierID:   pat.tierID,
								ID:       pat.id,
								Label:    pat.label,
								Severity: pat.severity,
								Line:     lineNum,
							})
						}
					}

					// Reset for-loop depth on closing braces (rough heuristic)
					trimmed := strings.TrimSpace(line)
					if trimmed == "}" && !strings.Contains(line, "\t\t") {
						if forLoopDepth > 0 {
							forLoopDepth--
						}
					}
				}
			}
		}

		// Add long_function findings from symbol metadata (code files only)
		if isCodeFile {
			for _, lf := range longFuncs {
				if lf.fileID == fileID {
					findings = append(findings, reconFinding{
						Symbol:   lf.sym.name,
						DimID:    "complexity",
						TierID:   "quality",
						ID:       "long_function",
						Label:    "Function exceeds 100 lines",
						Severity: "info",
						Line:     int(lf.sym.startLine),
					})
				}
			}
		}

		// Populate tree
		if _, ok := result.Tree[dir]; !ok {
			result.Tree[dir] = make(map[string]reconFileInfo)
		}
		result.Tree[dir][base] = reconFileInfo{
			Language: fileMeta.Language,
			Symbols:  symbolNames,
			Findings: findings,
		}

		// Aggregate counts
		if len(findings) == 0 {
			result.CleanFiles++
		}
		for _, f := range findings {
			result.TotalFindings++
			result.TierCounts[f.TierID]++
			result.DimCounts[f.DimID]++
			switch f.Severity {
			case "critical":
				result.Critical++
			case "warning":
				result.Warnings++
			case "info":
				result.Info++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// findEnclosingSymbol returns the name of the symbol (function) that contains the given line.
func findEnclosingSymbol(syms []reconSymbolInfo, lineNum int) string {
	for i := len(syms) - 1; i >= 0; i-- {
		if uint16(lineNum) >= syms[i].startLine && (syms[i].endLine == 0 || uint16(lineNum) <= syms[i].endLine) {
			return syms[i].name
		}
	}
	// If no enclosing symbol, find the nearest symbol above
	for i := len(syms) - 1; i >= 0; i-- {
		if uint16(lineNum) >= syms[i].startLine {
			return syms[i].name
		}
	}
	return ""
}
