package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/corey/aoa/internal/adapters/socket"
	"github.com/corey/aoa/internal/domain/analyzer"
)

// Local dashboard types — field-for-field identical JSON shape to recon.Result,
// recon.Finding, recon.FileInfo. Avoids importing internal/adapters/recon (G4).

type reconFinding struct {
	Symbol   string `json:"symbol"`
	DimID    string `json:"dim_id"`
	TierID   string `json:"tier_id"`
	ID       string `json:"id"`
	Label    string `json:"label"`
	Severity string `json:"severity"`
	Line     int    `json:"line"`
}

type reconFileInfo struct {
	Language string         `json:"language"`
	Symbols  []string       `json:"symbols"`
	Findings []reconFinding `json:"findings"`
}

type reconResult struct {
	FilesScanned  int                                `json:"files_scanned"`
	TotalFindings int                                `json:"total_findings"`
	Critical      int                                `json:"critical"`
	Warnings      int                                `json:"warnings"`
	Info          int                                `json:"info"`
	CleanFiles    int                                `json:"clean_files"`
	TierCounts    map[string]int                     `json:"tier_counts"`
	DimCounts     map[string]int                     `json:"dim_counts"`
	Tree          map[string]map[string]reconFileInfo `json:"tree"`
}

// ruleIndex maps rule ID → (tier, dimension) for data-driven tier/dim resolution.
var ruleIndex map[string]ruleMeta

type ruleMeta struct {
	tier      string
	dimension string
	label     string
}

// SetRuleIndex populates the rule metadata index from loaded rules.
// Called once at startup after YAML rules are loaded.
func SetRuleIndex(rules []analyzer.Rule) {
	ruleIndex = make(map[string]ruleMeta, len(rules))
	for _, r := range rules {
		ruleIndex[r.ID] = ruleMeta{
			tier:      analyzer.TierName(r.Tier),
			dimension: r.Dimension,
			label:     r.Label,
		}
	}
}

func (s *Server) handleRecon(w http.ResponseWriter, r *http.Request) {
	// Check for persisted dimensional results (from aoa-recon engine or dim engine)
	if s.queries != nil {
		dimResults := s.queries.DimensionalResults()
		if dimResults != nil && len(dimResults) > 0 {
			s.serveDimensionalResults(w, dimResults)
			return
		}
	}

	// No recon data available — return install prompt
	w.Header().Set("Content-Type", "application/json")
	reconAvailable := s.queries != nil && s.queries.ReconAvailable()
	var invFiles []string
	if s.queries != nil {
		invFiles = s.queries.InvestigatedFiles()
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"files_scanned":      0,
		"total_findings":     0,
		"recon_available":    reconAvailable,
		"install_prompt":     "Run 'aoa recon init' to enable structural analysis",
		"tree":               map[string]interface{}{},
		"investigated_files": invFiles,
	})
}

// serveDimensionalResults converts dimensional analysis results to the
// reconResult JSON shape for backward compatibility with the dashboard.
func (s *Server) serveDimensionalResults(w http.ResponseWriter, dimResults map[string]*socket.DimensionalFileResult) {
	result := &reconResult{
		TierCounts: make(map[string]int),
		DimCounts:  make(map[string]int),
		Tree:       make(map[string]map[string]reconFileInfo),
	}

	// Map severity int to string
	sevNames := [4]string{"info", "warning", "high", "critical"}

	for relPath, fa := range dimResults {
		result.FilesScanned++

		dir := filepath.Dir(relPath)
		base := filepath.Base(relPath)

		var findings []reconFinding
		for _, f := range fa.Findings {
			sevStr := "info"
			if f.Severity >= 0 && f.Severity < len(sevNames) {
				sevStr = sevNames[f.Severity]
			}
			tierID, dimID := inferTierDim(f.RuleID)
			findings = append(findings, reconFinding{
				Symbol:   f.Symbol,
				DimID:    dimID,
				TierID:   tierID,
				ID:       f.RuleID,
				Label:    inferLabel(f.RuleID),
				Severity: sevStr,
				Line:     f.Line,
			})

			result.TotalFindings++
			result.TierCounts[tierID]++
			result.DimCounts[dimID]++
			switch sevStr {
			case "critical":
				result.Critical++
			case "warning":
				result.Warnings++
			case "high":
				result.Warnings++
			case "info":
				result.Info++
			}
		}

		symbols := make([]string, len(fa.Methods))
		for i, m := range fa.Methods {
			symbols[i] = m.Name
		}

		if _, ok := result.Tree[dir]; !ok {
			result.Tree[dir] = make(map[string]reconFileInfo)
		}
		result.Tree[dir][base] = reconFileInfo{
			Language: fa.Language,
			Symbols:  symbols,
			Findings: findings,
		}

		if len(findings) == 0 {
			result.CleanFiles++
		}
	}

	reconAvailable := false
	var invFiles []string
	if s.queries != nil {
		reconAvailable = s.queries.ReconAvailable()
		invFiles = s.queries.InvestigatedFiles()
	}
	response := struct {
		*reconResult
		ReconAvailable    bool     `json:"recon_available"`
		DimensionalMode   bool     `json:"dimensional_mode"`
		ScannedAt         int64    `json:"scanned_at"`
		InvestigatedFiles []string `json:"investigated_files"`
	}{
		reconResult:       result,
		ReconAvailable:    reconAvailable,
		DimensionalMode:   true,
		ScannedAt:         time.Now().Unix(),
		InvestigatedFiles: invFiles,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSourceLine returns source lines from the in-memory file cache.
// GET /api/source-line?file=relative/path.go&line=12&context=2
func (s *Server) handleSourceLine(w http.ResponseWriter, r *http.Request) {
	if s.idx == nil || s.engine == nil {
		http.Error(w, `{"error":"index not available"}`, http.StatusServiceUnavailable)
		return
	}

	filePath := r.URL.Query().Get("file")
	lineStr := r.URL.Query().Get("line")
	ctxStr := r.URL.Query().Get("context")

	if filePath == "" || lineStr == "" {
		http.Error(w, `{"error":"file and line required"}`, http.StatusBadRequest)
		return
	}

	lineNum, err := strconv.Atoi(lineStr)
	if err != nil || lineNum < 1 {
		http.Error(w, `{"error":"invalid line number"}`, http.StatusBadRequest)
		return
	}

	ctxLines := 0
	if ctxStr != "" {
		ctxLines, _ = strconv.Atoi(ctxStr)
	}
	if ctxLines < 0 {
		ctxLines = 0
	}
	if ctxLines > 5 {
		ctxLines = 5
	}

	// Find file ID by path
	fc := s.engine.Cache()
	if fc == nil {
		http.Error(w, `{"error":"file cache not available"}`, http.StatusServiceUnavailable)
		return
	}

	var fileID uint32
	found := false
	for id, fm := range s.idx.Files {
		if fm.Path == filePath {
			fileID = id
			found = true
			break
		}
	}
	if !found {
		http.Error(w, `{"error":"file not in index"}`, http.StatusNotFound)
		return
	}

	lines := fc.GetLines(fileID)
	if lines == nil {
		http.Error(w, `{"error":"file not in cache"}`, http.StatusNotFound)
		return
	}

	// Extract the requested line plus context
	startLine := lineNum - ctxLines
	if startLine < 1 {
		startLine = 1
	}
	endLine := lineNum + ctxLines
	if endLine > len(lines) {
		endLine = len(lines)
	}

	type sourceLine struct {
		Line    int    `json:"line"`
		Content string `json:"content"`
		IsMatch bool   `json:"is_match"`
	}
	resultLines := make([]sourceLine, 0, endLine-startLine+1)
	for i := startLine; i <= endLine; i++ {
		resultLines = append(resultLines, sourceLine{
			Line:    i,
			Content: lines[i-1], // 0-indexed array, 1-indexed lines
			IsMatch: i == lineNum,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resultLines)
}

// handleReconInvestigate handles POST /api/recon-investigate to mark/unmark files.
func (s *Server) handleReconInvestigate(w http.ResponseWriter, r *http.Request) {
	if s.queries == nil {
		http.Error(w, `{"error":"not available"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		File   string `json:"file"`
		Action string `json:"action"` // "add", "remove", "clear"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	switch req.Action {
	case "add":
		if req.File == "" {
			http.Error(w, `{"error":"file required"}`, http.StatusBadRequest)
			return
		}
		s.queries.SetFileInvestigated(req.File, true)
	case "remove":
		if req.File == "" {
			http.Error(w, `{"error":"file required"}`, http.StatusBadRequest)
			return
		}
		s.queries.SetFileInvestigated(req.File, false)
	case "clear":
		s.queries.ClearInvestigated()
	default:
		http.Error(w, `{"error":"action must be add, remove, or clear"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// inferTierDim maps a rule ID to its tier and dimension.
// Uses data-driven rule index when available (from YAML), falls back to hardcoded map.
func inferTierDim(ruleID string) (tierID, dimID string) {
	if ruleIndex != nil {
		if rm, ok := ruleIndex[ruleID]; ok {
			return rm.tier, rm.dimension
		}
	}
	// Fallback for backward compatibility
	switch ruleID {
	case "hardcoded_secret", "aws_credentials", "jwt_secret_inline", "private_key_inline":
		return "security", "secrets"
	case "command_injection", "sql_injection_keyword", "path_traversal_keyword",
		"eval_usage", "unsafe_deserialization", "open_redirect_keyword",
		"exec_with_variable", "tainted_path_join", "format_string_injection",
		"template_unescaped", "sql_string_concat":
		return "security", "injection"
	case "weak_hash", "insecure_random":
		return "security", "crypto"
	case "insecure_tls", "disabled_tls_verify", "insecure_http":
		return "security", "transport"
	case "debug_endpoint", "cors_wildcard":
		return "security", "exposure"
	case "sensitive_data_log":
		return "security", "data"
	case "hardcoded_ip", "world_readable_perms":
		return "security", "config"
	case "regex_dos":
		return "security", "denial"
	case "defer_in_loop":
		return "performance", "resources"
	case "ignored_error", "panic_in_lib", "unchecked_type_assertion", "error_not_checked":
		return "quality", "errors"
	case "long_function":
		return "quality", "complexity"
	default:
		return "security", "general"
	}
}

// inferLabel returns the label for a finding, using rule index when available.
func inferLabel(ruleID string) string {
	if ruleIndex != nil {
		if rm, ok := ruleIndex[ruleID]; ok && rm.label != "" {
			return rm.label
		}
	}
	return ruleID
}
