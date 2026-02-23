package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/corey/aoa/internal/adapters/recon"
	"github.com/corey/aoa/internal/adapters/socket"
)

// fileCacheAdapter adapts the search engine's FileCache to the recon.LineGetter interface.
type fileCacheAdapter struct {
	cache interface {
		GetLines(fileID uint32) []string
	}
}

func (a *fileCacheAdapter) GetLines(fileID uint32) []string {
	if a.cache == nil {
		return nil
	}
	return a.cache.GetLines(fileID)
}

func (s *Server) handleRecon(w http.ResponseWriter, r *http.Request) {
	// Check for persisted dimensional results first (from aoa-recon engine)
	if s.queries != nil {
		dimResults := s.queries.DimensionalResults()
		if dimResults != nil && len(dimResults) > 0 {
			s.serveDimensionalResults(w, dimResults)
			return
		}
	}

	// Fall back to interim pattern scanner
	if s.idx == nil || s.engine == nil {
		http.Error(w, `{"error":"index not available"}`, http.StatusServiceUnavailable)
		return
	}

	var lines recon.LineGetter
	if fc := s.engine.Cache(); fc != nil {
		lines = &fileCacheAdapter{cache: fc}
	}

	result := recon.Scan(s.idx, lines)

	// Wrap result with recon_available field for dashboard install prompt
	reconAvailable := false
	if s.queries != nil {
		reconAvailable = s.queries.ReconAvailable()
	}
	response := struct {
		*recon.Result
		ReconAvailable bool  `json:"recon_available"`
		ScannedAt      int64 `json:"scanned_at"`
	}{
		Result:         result,
		ReconAvailable: reconAvailable,
		ScannedAt:      time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// serveDimensionalResults converts dimensional analysis results to the
// recon.Result JSON shape for backward compatibility with the dashboard.
func (s *Server) serveDimensionalResults(w http.ResponseWriter, dimResults map[string]*socket.DimensionalFileResult) {
	result := &recon.Result{
		TierCounts: make(map[string]int),
		DimCounts:  make(map[string]int),
		Tree:       make(map[string]map[string]recon.FileInfo),
	}

	// Map severity int to string
	sevNames := [4]string{"info", "warning", "high", "critical"}

	for relPath, fa := range dimResults {
		result.FilesScanned++

		dir := filepath.Dir(relPath)
		base := filepath.Base(relPath)

		var findings []recon.Finding
		for _, f := range fa.Findings {
			sevStr := "info"
			if f.Severity >= 0 && f.Severity < len(sevNames) {
				sevStr = sevNames[f.Severity]
			}
			tierID, dimID := inferTierDim(f.RuleID)
			findings = append(findings, recon.Finding{
				Symbol:   f.Symbol,
				DimID:    dimID,
				TierID:   tierID,
				ID:       f.RuleID,
				Label:    f.RuleID,
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
			result.Tree[dir] = make(map[string]recon.FileInfo)
		}
		result.Tree[dir][base] = recon.FileInfo{
			Language: fa.Language,
			Symbols:  symbols,
			Findings: findings,
		}

		if len(findings) == 0 {
			result.CleanFiles++
		}
	}

	reconAvailable := false
	if s.queries != nil {
		reconAvailable = s.queries.ReconAvailable()
	}
	response := struct {
		*recon.Result
		ReconAvailable  bool  `json:"recon_available"`
		DimensionalMode bool  `json:"dimensional_mode"`
		ScannedAt       int64 `json:"scanned_at"`
	}{
		Result:          result,
		ReconAvailable:  reconAvailable,
		DimensionalMode: true,
		ScannedAt:       time.Now().Unix(),
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
	result := make([]sourceLine, 0, endLine-startLine+1)
	for i := startLine; i <= endLine; i++ {
		result = append(result, sourceLine{
			Line:    i,
			Content: lines[i-1], // 0-indexed array, 1-indexed lines
			IsMatch: i == lineNum,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// inferTierDim maps a rule ID to its tier and dimension.
func inferTierDim(ruleID string) (tierID, dimID string) {
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
