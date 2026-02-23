// Package recon provides code quality pattern scanning. It detects security issues,
// performance problems, and code quality concerns by scanning source files line-by-line.
//
// This is a shared package used by both the web dashboard (GET /api/recon) and
// the aoa-recon CLI binary (aoa-recon enhance).
package recon

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/corey/aoa/internal/ports"
)

// Finding represents a single detected issue in a file.
type Finding struct {
	Symbol   string `json:"symbol"`
	DimID    string `json:"dim_id"`
	TierID   string `json:"tier_id"`
	ID       string `json:"id"`
	Label    string `json:"label"`
	Severity string `json:"severity"`
	Line     int    `json:"line"`
}

// FileInfo holds findings and metadata for one file.
type FileInfo struct {
	Language string    `json:"language"`
	Symbols  []string  `json:"symbols"`
	Findings []Finding `json:"findings"`
}

// Result is the full scan result.
type Result struct {
	FilesScanned  int                            `json:"files_scanned"`
	TotalFindings int                            `json:"total_findings"`
	Critical      int                            `json:"critical"`
	Warnings      int                            `json:"warnings"`
	Info          int                            `json:"info"`
	CleanFiles    int                            `json:"clean_files"`
	TierCounts    map[string]int                 `json:"tier_counts"`
	DimCounts     map[string]int                 `json:"dim_counts"`
	Tree          map[string]map[string]FileInfo `json:"tree"` // folder -> file -> info
}

// symbolInfo holds a symbol's name and line range for enclosing-symbol lookups.
type symbolInfo struct {
	name      string
	startLine uint16
	endLine   uint16
}

// pattern defines a text pattern to scan for.
type pattern struct {
	id       string
	label    string
	tierID   string
	dimID    string
	severity string
	codeOnly bool
	match    func(line string, lineNum int, isTest bool, isMain bool) bool
}

// CodeExts is the set of extensions considered "code" for code-quality patterns.
var CodeExts = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	".rs": true, ".c": true, ".cpp": true, ".h": true, ".java": true, ".rb": true,
	".sh": true, ".bash": true, ".cs": true, ".swift": true, ".kt": true, ".scala": true,
	".zig": true, ".lua": true, ".php": true, ".pl": true, ".r": true,
}

var (
	reDeferInFor   = regexp.MustCompile(`^\s*defer\s`)
	reForLoop      = regexp.MustCompile(`^\s*for\s`)
	reTodoFixme    = regexp.MustCompile(`(?i)\b(TODO|FIXME|HACK|XXX)\b`)
	reSecretAssign = regexp.MustCompile(`(?i)(password|secret|api_key|apikey|private_key)\s*[:=].*["` + "`" + `']`)
)

func buildPatterns() []pattern {
	return []pattern{
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
		{
			id: "defer_in_loop", label: "defer inside loop body",
			tierID: "performance", dimID: "resources", severity: "warning", codeOnly: true,
			match: func(line string, _ int, _, _ bool) bool {
				return reDeferInFor.MatchString(line)
			},
		},
		{
			id: "ignored_error", label: "Error return value assigned to blank identifier",
			tierID: "quality", dimID: "errors", severity: "warning", codeOnly: true,
			match: func(line string, _ int, _, _ bool) bool {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					return false
				}
				if strings.HasPrefix(trimmed, "_ =") || strings.HasPrefix(trimmed, "_ :=") {
					return strings.Contains(trimmed, "(") || strings.Contains(trimmed, ".")
				}
				if strings.Contains(trimmed, ", _ =") || strings.Contains(trimmed, ", _ :=") {
					return strings.Contains(trimmed, "(")
				}
				return false
			},
		},
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
		{
			id: "todo_fixme", label: "TODO/FIXME/HACK/XXX marker in source",
			tierID: "observability", dimID: "debug", severity: "info", codeOnly: false,
			match: func(line string, _ int, _, _ bool) bool {
				return reTodoFixme.MatchString(line)
			},
		},
		{
			id: "global_state", label: "Mutable global variable at package level",
			tierID: "architecture", dimID: "antipattern", severity: "warning", codeOnly: true,
			match: func(line string, _ int, isTest, isMain bool) bool {
				if isTest || isMain {
					return false
				}
				if !strings.HasPrefix(line, "var ") {
					return false
				}
				trimmed := strings.TrimSpace(line)
				if trimmed == "var (" {
					return false
				}
				if strings.Contains(line, "embed.FS") || strings.Contains(line, "//go:embed") {
					return false
				}
				if strings.Contains(line, "regexp.MustCompile") || strings.Contains(line, "regexp.Compile") {
					return false
				}
				return true
			},
		},
	}
}

// LineGetter retrieves source lines for a given file ID.
// Returns nil if lines are unavailable.
type LineGetter interface {
	GetLines(fileID uint32) []string
}

// ScanFile scans a single file and returns its FileInfo.
// symbols must be sorted by startLine. lines may be nil.
func ScanFile(fileMeta *ports.FileMeta, symbols []symbolInfo, fileLines []string, pats []pattern) FileInfo {
	base := filepath.Base(fileMeta.Path)
	ext := filepath.Ext(base)

	isCodeFile := CodeExts[ext]
	isTest := strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, ".test.js") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, "_test.rs") ||
		strings.Contains(base, "test_")
	isMain := strings.Contains(fileMeta.Path, "cmd/") ||
		base == "main.go" || base == "main.py"

	symbolNames := make([]string, len(symbols))
	for i, s := range symbols {
		symbolNames[i] = s.name
	}

	var findings []Finding

	if fileLines != nil {
		forLoopDepth := 0
		for lineIdx, line := range fileLines {
			lineNum := lineIdx + 1

			if reForLoop.MatchString(line) {
				forLoopDepth++
			}

			for _, pat := range pats {
				if pat.codeOnly && !isCodeFile {
					continue
				}
				if pat.id == "defer_in_loop" {
					if forLoopDepth > 0 && reDeferInFor.MatchString(line) {
						sym := findEnclosingSymbol(symbols, lineNum)
						findings = append(findings, Finding{
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
					sym := findEnclosingSymbol(symbols, lineNum)
					findings = append(findings, Finding{
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

			trimmed := strings.TrimSpace(line)
			if trimmed == "}" && !strings.Contains(line, "\t\t") {
				if forLoopDepth > 0 {
					forLoopDepth--
				}
			}
		}
	}

	// Check for long functions
	if isCodeFile {
		for _, sym := range symbols {
			if sym.endLine > 0 && int(sym.endLine)-int(sym.startLine) > 100 {
				findings = append(findings, Finding{
					Symbol:   sym.name,
					DimID:    "complexity",
					TierID:   "quality",
					ID:       "long_function",
					Label:    "Function exceeds 100 lines",
					Severity: "info",
					Line:     int(sym.startLine),
				})
			}
		}
	}

	return FileInfo{
		Language: fileMeta.Language,
		Symbols:  symbolNames,
		Findings: findings,
	}
}

// SubtractFile removes a file's contribution from aggregate counts.
func (r *Result) SubtractFile(dir, base string) {
	files, ok := r.Tree[dir]
	if !ok {
		return
	}
	info, ok := files[base]
	if !ok {
		return
	}

	r.FilesScanned--
	if len(info.Findings) == 0 {
		r.CleanFiles--
	}
	for _, f := range info.Findings {
		r.TotalFindings--
		r.TierCounts[f.TierID]--
		r.DimCounts[f.DimID]--
		switch f.Severity {
		case "critical":
			r.Critical--
		case "warning":
			r.Warnings--
		case "info":
			r.Info--
		}
	}

	delete(files, base)
	if len(files) == 0 {
		delete(r.Tree, dir)
	}
}

// AddFile adds a file's contribution to aggregate counts.
func (r *Result) AddFile(dir, base string, info FileInfo) {
	r.FilesScanned++

	if _, ok := r.Tree[dir]; !ok {
		r.Tree[dir] = make(map[string]FileInfo)
	}
	r.Tree[dir][base] = info

	if len(info.Findings) == 0 {
		r.CleanFiles++
	}
	for _, f := range info.Findings {
		r.TotalFindings++
		r.TierCounts[f.TierID]++
		r.DimCounts[f.DimID]++
		switch f.Severity {
		case "critical":
			r.Critical++
		case "warning":
			r.Warnings++
		case "info":
			r.Info++
		}
	}
}

// BuildFileSymbols builds a per-file sorted symbol mapping from index metadata.
func BuildFileSymbols(idx *ports.Index) map[uint32][]symbolInfo {
	fileSymbols := make(map[uint32][]symbolInfo)
	for ref, meta := range idx.Metadata {
		if meta == nil {
			continue
		}
		fileSymbols[ref.FileID] = append(fileSymbols[ref.FileID], symbolInfo{
			name:      meta.Name,
			startLine: meta.StartLine,
			endLine:   meta.EndLine,
		})
	}
	for fid := range fileSymbols {
		syms := fileSymbols[fid]
		sort.Slice(syms, func(i, j int) bool {
			return syms[i].startLine < syms[j].startLine
		})
	}
	return fileSymbols
}

// Patterns returns the built-in scan patterns. Exported for incremental scanning.
func Patterns() []pattern {
	return buildPatterns()
}

// Scan performs a full recon scan over all files in the index.
func Scan(idx *ports.Index, lines LineGetter) *Result {
	pats := buildPatterns()

	result := &Result{
		TierCounts: make(map[string]int),
		DimCounts:  make(map[string]int),
		Tree:       make(map[string]map[string]FileInfo),
	}

	fileSymbols := BuildFileSymbols(idx)

	for fileID, fileMeta := range idx.Files {
		dir := filepath.Dir(fileMeta.Path)
		base := filepath.Base(fileMeta.Path)

		var fileLines []string
		if lines != nil {
			fileLines = lines.GetLines(fileID)
		}

		info := ScanFile(fileMeta, fileSymbols[fileID], fileLines, pats)
		result.AddFile(dir, base, info)
	}

	return result
}

func findEnclosingSymbol(syms []symbolInfo, lineNum int) string {
	for i := len(syms) - 1; i >= 0; i-- {
		if uint16(lineNum) >= syms[i].startLine && (syms[i].endLine == 0 || uint16(lineNum) <= syms[i].endLine) {
			return syms[i].name
		}
	}
	for i := len(syms) - 1; i >= 0; i-- {
		if uint16(lineNum) >= syms[i].startLine {
			return syms[i].name
		}
	}
	return ""
}
