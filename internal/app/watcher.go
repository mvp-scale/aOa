package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/ports"
)

// contextJSONLFile is the filename for status line context snapshots.
const contextJSONLFile = "context.jsonl"

// usageTxtFile is the filename for pasted /usage output.
const usageTxtFile = "usage.txt"

// onFileChanged handles a file create/modify/delete event from the watcher.
// It updates the index in-place and rebuilds the search engine.
func (a *App) onFileChanged(absPath string) {
	aoadir := filepath.Join(a.ProjectRoot, ".aoa")
	base := filepath.Base(absPath)
	dir := filepath.Dir(absPath)

	// Intercept .aoa/context.jsonl before the extension filter
	if base == contextJSONLFile && dir == aoadir {
		a.onContextFileChanged(absPath)
		return
	}

	// Intercept .aoa/usage.txt — pasted /usage output
	if base == usageTxtFile && dir == aoadir {
		a.onUsageFileChanged(absPath)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	ext := strings.ToLower(filepath.Ext(absPath))
	if ext == "" {
		return
	}
	// When parser is nil (tokenization-only mode), use the default code extensions list.
	if a.Parser != nil && !a.Parser.SupportsExtension(ext) {
		return
	}
	if a.Parser == nil && !defaultCodeExtensions[ext] {
		return
	}

	relPath, err := filepath.Rel(a.ProjectRoot, absPath)
	if err != nil {
		return
	}

	// Find existing fileID for this path
	var existingID uint32
	for id, fm := range a.Index.Files {
		if fm.Path == relPath {
			existingID = id
			break
		}
	}

	// Check if file was deleted
	info, statErr := os.Stat(absPath)
	if statErr != nil {
		// File removed
		if existingID > 0 {
			a.removeFileFromIndex(existingID)
			a.Engine.Rebuild()
			if a.Store != nil {
				_ = a.Store.SaveIndex(a.ProjectID, a.Index)
			}
		}
		return
	}

	// Skip files > 1MB
	if info.Size() > 1<<20 {
		return
	}

	source, err := os.ReadFile(absPath)
	if err != nil {
		return
	}

	// Remove old entry if modifying existing file
	if existingID > 0 {
		a.removeFileFromIndex(existingID)
	}

	// Allocate new fileID (max existing + 1)
	var fileID uint32
	if existingID > 0 {
		fileID = existingID
	} else {
		for id := range a.Index.Files {
			if id >= fileID {
				fileID = id + 1
			}
		}
		if fileID == 0 {
			fileID = 1
		}
	}

	ext = strings.TrimPrefix(filepath.Ext(absPath), ".")
	a.Index.Files[fileID] = &ports.FileMeta{
		Path:         relPath,
		LastModified: info.ModTime().Unix(),
		Language:     ext,
		Size:         info.Size(),
	}

	// When parser is available, extract symbols; otherwise tokenize file content only.
	var metas []*ports.SymbolMeta
	if a.Parser != nil {
		var parseErr error
		metas, parseErr = a.Parser.ParseFileToMeta(absPath, source)
		if parseErr != nil {
			metas = nil
		}
	}

	if len(metas) == 0 {
		// No symbols (parser nil or no matches) — tokenize file content for file-level search.
		lines := strings.Split(string(source), "\n")
		for _, line := range lines {
			tokens := index.TokenizeContentLine(line)
			for _, tok := range tokens {
				ref := ports.TokenRef{FileID: fileID, Line: 0}
				a.Index.Tokens[tok] = append(a.Index.Tokens[tok], ref)
			}
		}
		a.Engine.Rebuild()
		if a.Store != nil {
			_ = a.Store.SaveIndex(a.ProjectID, a.Index)
		}
		return
	}

	for _, meta := range metas {
		ref := ports.TokenRef{FileID: fileID, Line: meta.StartLine}
		a.Index.Metadata[ref] = meta

		tokens := index.Tokenize(meta.Name)
		for _, tok := range tokens {
			a.Index.Tokens[tok] = append(a.Index.Tokens[tok], ref)
		}

		lower := strings.ToLower(meta.Name)
		if lower != "" {
			a.Index.Tokens[lower] = append(a.Index.Tokens[lower], ref)
		}
	}

	a.Engine.Rebuild()
	if a.Store != nil {
		_ = a.Store.SaveIndex(a.ProjectID, a.Index)
	}

	// If aoa-recon is available and parser is nil, trigger incremental enhancement.
	// When parser is non-nil, symbols are already extracted above.
	if a.Parser == nil {
		a.TriggerReconEnhanceFile(absPath)
	}
}

// onContextFileChanged reads .aoa/context.jsonl and stores the last 5 snapshots.
// The hook script is the sole writer; we are read-only.
func (a *App) onContextFileChanged(absPath string) {
	f, err := os.Open(absPath)
	if err != nil {
		return
	}
	defer f.Close()

	// Read all lines, keep last 5
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 4096)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return
	}

	// Parse last 5 lines into snapshots
	start := 0
	if len(lines) > 5 {
		start = len(lines) - 5
	}
	recent := lines[start:]

	a.mu.Lock()
	defer a.mu.Unlock()

	a.ctxSnapCount = 0
	a.ctxSnapHead = 0
	for _, line := range recent {
		var snap ContextSnapshot
		if err := json.Unmarshal([]byte(line), &snap); err != nil {
			continue
		}
		a.ctxSnapshots[a.ctxSnapHead] = snap
		a.ctxSnapHead = (a.ctxSnapHead + 1) % 5
		if a.ctxSnapCount < 5 {
			a.ctxSnapCount++
		}
	}
}

// latestContextSnapshot returns the most recent context snapshot, or nil if none.
// Must be called with a.mu held.
func (a *App) latestContextSnapshot() *ContextSnapshot {
	if a.ctxSnapCount == 0 {
		return nil
	}
	idx := (a.ctxSnapHead - 1 + 5) % 5
	snap := a.ctxSnapshots[idx]
	return &snap
}

// usagePasteMarker is the line that separates instructions from pasted content.
const usagePasteMarker = "--- PASTE /usage OUTPUT BELOW THIS LINE ---"

// usageConfirmMarker indicates the file has been processed.
const usageConfirmMarker = "--- LAST PROCESSED ---"

// usageFileHeader is the instructions written to a fresh usage.txt.
const usageFileHeader = `# aOa Usage Quota
#
# This file lets aOa track your Claude Code usage limits.
# The dashboard uses this to show session/weekly countdowns and budget pacing.
#
# HOW TO UPDATE:
#   1. In Claude Code, type:  /usage
#   2. Copy the entire output (Cmd+A, Cmd+C or select all)
#   3. Paste it below the marker line
#   4. Save the file — aOa processes it automatically
#
# You only need to do this once a day (or when you want fresh numbers).
#

`

// onUsageFileChanged reads .aoa/usage.txt (pasted /usage output) and parses it.
// After successful parsing, writes back the file with confirmation + structured data.
func (a *App) onUsageFileChanged(absPath string) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return
	}

	content := string(data)

	// If file already has confirmation and no new paste, skip
	if strings.Contains(content, usageConfirmMarker) && !hasNewPasteContent(content) {
		return
	}

	// Extract content after the paste marker (or use entire file if no marker)
	pasteContent := content
	if idx := strings.Index(content, usagePasteMarker); idx >= 0 {
		pasteContent = content[idx+len(usagePasteMarker):]
	}

	// Strip any previous confirmation block before parsing
	if idx := strings.Index(pasteContent, usageConfirmMarker); idx >= 0 {
		pasteContent = pasteContent[:idx]
	}

	quota := parseUsageOutput(pasteContent)
	if quota == nil {
		return
	}

	quota.CapturedAt = time.Now().Unix()

	a.mu.Lock()
	a.usageQuota = quota
	a.mu.Unlock()

	// Write back confirmation
	writeUsageConfirmation(absPath, quota)
}

// hasNewPasteContent checks if there's pasted text between the paste marker and the confirmation.
func hasNewPasteContent(content string) bool {
	pasteIdx := strings.Index(content, usagePasteMarker)
	confirmIdx := strings.Index(content, usageConfirmMarker)

	if pasteIdx < 0 {
		return false
	}

	between := ""
	if confirmIdx > pasteIdx {
		between = content[pasteIdx+len(usagePasteMarker) : confirmIdx]
	} else {
		between = content[pasteIdx+len(usagePasteMarker):]
	}

	// Check if there's any non-whitespace content with % signs (usage data)
	return strings.Contains(between, "% used")
}

// writeUsageConfirmation rewrites usage.txt with header, marker, and structured confirmation.
func writeUsageConfirmation(absPath string, quota *UsageQuota) {
	var b strings.Builder

	b.WriteString(usageFileHeader)
	b.WriteString(usagePasteMarker)
	b.WriteString("\n\n")
	b.WriteString(usageConfirmMarker)
	b.WriteString("\n")

	ts := time.Unix(quota.CapturedAt, 0)
	b.WriteString("# Processed: " + ts.Format("Jan 2, 2006 3:04 PM MST") + "\n")
	b.WriteString("#\n")

	if t := quota.Session; t != nil {
		resetStr := t.ResetsAt
		if t.Timezone != "" {
			resetStr += " (" + t.Timezone + ")"
		}
		b.WriteString(fmt.Sprintf("# Session:        %3d%% used  │  resets %s\n", t.UsedPct, resetStr))
	}
	if t := quota.WeeklyAll; t != nil {
		resetStr := t.ResetsAt
		if t.Timezone != "" {
			resetStr += " (" + t.Timezone + ")"
		}
		b.WriteString(fmt.Sprintf("# Weekly (all):   %3d%% used  │  resets %s\n", t.UsedPct, resetStr))
	}
	if t := quota.WeeklySonnet; t != nil {
		resetStr := t.ResetsAt
		if t.Timezone != "" {
			resetStr += " (" + t.Timezone + ")"
		}
		b.WriteString(fmt.Sprintf("# Weekly (Sonnet): %2d%% used  │  resets %s\n", t.UsedPct, resetStr))
	}

	b.WriteString("#\n")
	b.WriteString("# To update: paste new /usage output above the --- LAST PROCESSED --- line.\n")

	os.WriteFile(absPath, []byte(b.String()), 0644)
}

// SeedUsageFile creates .aoa/usage.txt with instructions if it doesn't exist.
func SeedUsageFile(projectRoot string) {
	path := filepath.Join(projectRoot, ".aoa", "usage.txt")
	if _, err := os.Stat(path); err == nil {
		return // already exists
	}

	var b strings.Builder
	b.WriteString(usageFileHeader)
	b.WriteString(usagePasteMarker)
	b.WriteString("\n")

	os.WriteFile(path, []byte(b.String()), 0644)
}

// usageQuota returns the current usage quota, or nil. Must be called with a.mu held.
func (a *App) latestUsageQuota() *UsageQuota {
	return a.usageQuota
}

// Regex patterns for parsing /usage output.
var (
	usagePctRe   = regexp.MustCompile(`(\d+)%\s+used`)
	usageResetRe = regexp.MustCompile(`Resets\s+(.+?)\s*\(([^)]+)\)`)
)

// parseUsageOutput parses the text output of Claude Code's /usage command.
// Expected format (3 blocks):
//
//	Current session
//	████                  8% used
//	Resets 3pm (America/New_York)
//
//	Current week (all models)
//	████████████████████  92% used
//	Resets Feb 22, 8pm (America/New_York)
//
//	Current week (Sonnet only)
//	█                     2% used
//	Resets Feb 25, 10pm (America/New_York)
func parseUsageOutput(text string) *UsageQuota {
	// Split into blocks by blank lines
	lines := strings.Split(text, "\n")

	type block struct {
		header string
		lines  []string
	}

	var blocks []block
	var current *block

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current != nil {
				blocks = append(blocks, *current)
				current = nil
			}
			continue
		}
		if current == nil {
			current = &block{header: trimmed}
		} else {
			current.lines = append(current.lines, trimmed)
		}
	}
	if current != nil {
		blocks = append(blocks, *current)
	}

	if len(blocks) == 0 {
		return nil
	}

	quota := &UsageQuota{}

	for _, b := range blocks {
		tier := parseTierBlock(b.header, b.lines)
		if tier == nil {
			continue
		}

		headerLower := strings.ToLower(b.header)
		switch {
		case strings.Contains(headerLower, "session"):
			tier.Label = "session"
			quota.Session = tier
		case strings.Contains(headerLower, "sonnet"):
			tier.Label = "weekly_sonnet"
			quota.WeeklySonnet = tier
		case strings.Contains(headerLower, "week"):
			tier.Label = "weekly_all"
			quota.WeeklyAll = tier
		}
	}

	if quota.Session == nil && quota.WeeklyAll == nil && quota.WeeklySonnet == nil {
		return nil
	}

	return quota
}

// parseTierBlock extracts percentage, reset time, and timezone from a usage block.
func parseTierBlock(header string, lines []string) *UsageQuotaTier {
	tier := &UsageQuotaTier{}
	joined := strings.Join(lines, "\n")

	// Extract percentage
	if m := usagePctRe.FindStringSubmatch(joined); len(m) >= 2 {
		tier.UsedPct, _ = strconv.Atoi(m[1])
	}

	// Extract reset time and timezone
	if m := usageResetRe.FindStringSubmatch(joined); len(m) >= 3 {
		tier.ResetsAt = strings.TrimSpace(m[1])
		tier.Timezone = strings.TrimSpace(m[2])
		tier.ResetEpoch = parseResetTime(tier.ResetsAt, tier.Timezone)
	}

	// Must have at least a percentage to be valid
	if tier.UsedPct == 0 && tier.ResetsAt == "" {
		return nil
	}

	return tier
}

// parseResetTime attempts to parse a reset time string like "3pm" or "Feb 22, 8pm"
// into a unix timestamp, given a timezone name. Returns 0 if unparseable.
func parseResetTime(resetStr, tzName string) int64 {
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return 0
	}

	now := time.Now().In(loc)

	// Normalize: lowercase, strip extra spaces
	s := strings.ToLower(strings.TrimSpace(resetStr))

	// Try formats: "3pm", "8pm", "3:30pm"
	// or: "feb 22, 8pm", "feb 25, 10pm"
	timeFormats := []string{"3pm", "3:04pm", "3PM", "3:04PM"}
	dateTimeFormats := []string{
		"Jan 2, 3pm",
		"Jan 2, 3:04pm",
		"Jan 02, 3pm",
		"Jan 02, 3:04pm",
	}

	// Try time-only (same day or next occurrence)
	for _, fmt := range timeFormats {
		t, err := time.ParseInLocation(fmt, s, loc)
		if err == nil {
			// Set to today's date
			result := time.Date(now.Year(), now.Month(), now.Day(),
				t.Hour(), t.Minute(), 0, 0, loc)
			// If already past, it means tomorrow
			if result.Before(now) {
				result = result.AddDate(0, 0, 1)
			}
			return result.Unix()
		}
	}

	// Try date+time
	for _, fmt := range dateTimeFormats {
		t, err := time.ParseInLocation(fmt, s, loc)
		if err == nil {
			// Set year to current year
			result := time.Date(now.Year(), t.Month(), t.Day(),
				t.Hour(), t.Minute(), 0, 0, loc)
			// If more than 6 months ago, assume next year
			if result.Before(now.AddDate(0, -6, 0)) {
				result = result.AddDate(1, 0, 0)
			}
			return result.Unix()
		}
	}

	return 0
}

// removeFileFromIndex removes all entries for a fileID from the index maps.
// Must be called with a.mu held.
func (a *App) removeFileFromIndex(fileID uint32) {
	// Remove metadata entries for this file
	for ref := range a.Index.Metadata {
		if ref.FileID == fileID {
			delete(a.Index.Metadata, ref)
		}
	}

	// Remove token refs for this file; delete empty token entries
	for tok, refs := range a.Index.Tokens {
		var kept []ports.TokenRef
		for _, ref := range refs {
			if ref.FileID != fileID {
				kept = append(kept, ref)
			}
		}
		if len(kept) == 0 {
			delete(a.Index.Tokens, tok)
		} else {
			a.Index.Tokens[tok] = kept
		}
	}

	// Remove file entry
	delete(a.Index.Files, fileID)
}
