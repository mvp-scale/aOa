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
	match    func(line string, lineNum int, isTest bool, isMain bool) bool
}

var (
	reDeferInFor  = regexp.MustCompile(`^\s*defer\s`)
	reForLoop     = regexp.MustCompile(`^\s*for\s`)
	reIgnoredErr  = regexp.MustCompile(`_\s*[:=]=?\s*`)
	rePkgVar      = regexp.MustCompile(`^var\s+\w+`)
	reTodoFixme   = regexp.MustCompile(`(?i)\b(TODO|FIXME|HACK|XXX)\b`)
)

func buildReconPatterns() []reconPattern {
	return []reconPattern{
		// Security — secrets
		{
			id: "hardcoded_secret", label: "Potential hardcoded secret or credential",
			tierID: "security", dimID: "secrets", severity: "critical",
			match: func(line string, _ int, isTest, _ bool) bool {
				if isTest {
					return false
				}
				lower := strings.ToLower(line)
				// Look for assignment patterns with secret-like variable names
				for _, kw := range []string{"password", "secret", "api_key", "apikey", "private_key"} {
					if strings.Contains(lower, kw) && (strings.Contains(line, "=") || strings.Contains(line, ":")) {
						// Skip comments
						trimmed := strings.TrimSpace(line)
						if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
							return false
						}
						return true
					}
				}
				return false
			},
		},
		// Security — command injection
		{
			id: "command_injection", label: "Potential command injection via exec/system call",
			tierID: "security", dimID: "injection", severity: "critical",
			match: func(line string, _ int, isTest, _ bool) bool {
				if isTest {
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
		// Security — weak hash
		{
			id: "weak_hash", label: "MD5 or SHA1 used (weak for security purposes)",
			tierID: "security", dimID: "crypto", severity: "warning",
			match: func(line string, _ int, isTest, _ bool) bool {
				if isTest {
					return false
				}
				return strings.Contains(line, "md5.") || strings.Contains(line, "sha1.")
			},
		},
		// Security — insecure random
		{
			id: "insecure_random", label: "math/rand used where crypto/rand may be needed",
			tierID: "security", dimID: "crypto", severity: "warning",
			match: func(line string, _ int, isTest, _ bool) bool {
				if isTest {
					return false
				}
				return strings.Contains(line, `"math/rand"`)
			},
		},
		// Performance — defer in loop
		{
			id: "defer_in_loop", label: "defer inside loop body",
			tierID: "performance", dimID: "resources", severity: "warning",
			match: func(line string, _ int, _, _ bool) bool {
				// Heuristic: line has defer — the for-loop context check
				// would require multi-line analysis; we flag defer statements
				// and let the user review.
				return reDeferInFor.MatchString(line)
			},
		},
		// Quality — ignored error
		{
			id: "ignored_error", label: "Error return value assigned to blank identifier",
			tierID: "quality", dimID: "errors", severity: "warning",
			match: func(line string, _ int, _, _ bool) bool {
				trimmed := strings.TrimSpace(line)
				// Match "_ = something" or "_ := something" patterns
				return strings.HasPrefix(trimmed, "_ =") || strings.HasPrefix(trimmed, "_ :=") ||
					strings.Contains(trimmed, ", _ =") || strings.Contains(trimmed, ", _ :=")
			},
		},
		// Quality — panic in library
		{
			id: "panic_in_lib", label: "panic() called in library/non-main package",
			tierID: "quality", dimID: "errors", severity: "warning",
			match: func(line string, _ int, _, isMain bool) bool {
				if isMain {
					return false
				}
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					return false
				}
				return strings.Contains(line, "panic(")
			},
		},
		// Observability — print statements
		{
			id: "print_statement", label: "Debug print statement left in code",
			tierID: "observability", dimID: "debug", severity: "info",
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
		// Observability — TODO/FIXME markers
		{
			id: "todo_fixme", label: "TODO/FIXME/HACK/XXX marker in source",
			tierID: "observability", dimID: "debug", severity: "info",
			match: func(line string, _ int, _, _ bool) bool {
				return reTodoFixme.MatchString(line)
			},
		},
		// Architecture — mutable global state
		{
			id: "global_state", label: "Mutable global variable at package level",
			tierID: "architecture", dimID: "antipattern", severity: "warning",
			match: func(line string, _ int, isTest, isMain bool) bool {
				if isTest || isMain {
					return false
				}
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					return false
				}
				// Only match package-level var declarations (no leading whitespace)
				return rePkgVar.MatchString(line) && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ")
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
	// Sort symbols by start line for binary search
	for fid := range fileSymbols {
		syms := fileSymbols[fid]
		sort.Slice(syms, func(i, j int) bool {
			return syms[i].startLine < syms[j].startLine
		})
	}

	// Check for long functions from symbol metadata
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
				// Track if we're inside a for loop for defer detection
				forLoopDepth := 0
				for lineIdx, line := range lines {
					lineNum := lineIdx + 1 // 1-based

					// Track for-loop depth for defer-in-loop detection
					if reForLoop.MatchString(line) {
						forLoopDepth++
					}

					for _, pat := range patterns {
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

					// Reset for-loop tracking at function boundaries (rough heuristic)
					trimmed := strings.TrimSpace(line)
					if trimmed == "}" && !strings.Contains(line, "\t\t") {
						if forLoopDepth > 0 {
							forLoopDepth--
						}
					}
				}
			}
		}

		// Add long_function findings from symbol metadata
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
	// If no enclosing symbol, try to find the nearest symbol above
	for i := len(syms) - 1; i >= 0; i-- {
		if uint16(lineNum) >= syms[i].startLine {
			return syms[i].name
		}
	}
	return ""
}
