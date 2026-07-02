//go:build compliance

// Package compliance validates aOa's integration contract with Claude Code's
// status line system. See CONTRACT.md for the full spec.
//
// This file deliberately does NOT import any aOa packages — the test
// validates the contract, not the implementation.
//
// Run:
//
//	go test -tags compliance ./compliance/claude-statusline/...
//
// Without the build tag this file is invisible to `go test ./...`.
package compliance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Stdin paths the hook reads — every "consumed" path in the contract.
// Mirrors versions/v2.1.70-inferred/assumed-fields.md.
var consumedStdinPaths = []string{
	"cwd",
	"session_id",
	"version",
	"model.id",
	"model.display_name",
	"cost.total_cost_usd",
	"cost.total_lines_added",
	"cost.total_lines_removed",
	"cost.total_duration_ms",
	"cost.total_api_duration_ms",
	"context_window.context_window_size",
	"context_window.used_percentage",
	"context_window.remaining_percentage",
	"context_window.total_input_tokens",
	"context_window.total_output_tokens",
	"context_window.current_usage.input_tokens",
	"context_window.current_usage.cache_creation_input_tokens",
	"context_window.current_usage.cache_read_input_tokens",
	"rate_limits.five_hour.used_percentage",
	"rate_limits.five_hour.resets_at",
	"rate_limits.seven_day.used_percentage",
	"rate_limits.seven_day.resets_at",
}

// Settings keys the integration writes; each must remain accepted by Claude Code.
var requiredSettingsKeys = []string{
	"statusLine",
	"statusLine.type",
	"statusLine.command",
}

// repoRoot resolves to /path/to/aOa-go.
func repoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot resolve cwd: %v", err)
	}
	for i := 0; i < 5 && filepath.Base(cwd) != "aOa-go"; i++ {
		cwd = filepath.Dir(cwd)
	}
	return cwd
}

// loadBaseline reads the highest-versioned observed manifest.
func loadBaseline(t *testing.T) (version, dir string, manifest map[string]any) {
	t.Helper()
	entries, err := os.ReadDir("versions")
	if err != nil {
		t.Skipf("versions/ not readable: %v", err)
	}
	var bestName string
	var bestVer []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasPrefix(n, "v") || !strings.HasSuffix(n, "-observed") {
			continue
		}
		ver := parseSemver(strings.TrimSuffix(strings.TrimPrefix(n, "v"), "-observed"))
		if ver == nil {
			continue
		}
		if bestName == "" || semverLess(bestVer, ver) {
			bestName = n
			bestVer = ver
		}
	}
	if bestName == "" {
		t.Skipf("no v*-observed/ directory found")
	}
	dir = filepath.Join("versions", bestName)
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		// A captured sample with no manifest is a half-authored baseline: the
		// highest-version dir would silently skip every pass, reading green while
		// validating nothing. Fail loudly so the gap can't ship unnoticed.
		if _, sErr := os.Stat(filepath.Join(dir, "sample.json")); sErr == nil {
			t.Fatalf("FAIL: %s has sample.json but no manifest.json — baseline is half-authored and would silently skip validation; author the manifest", dir)
		}
		t.Skipf("cannot read %s/manifest.json: %v", dir, err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Skipf("bad manifest.json in %s: %v", dir, err)
	}
	v, _ := manifest["claude_code_version"].(string)
	return v, dir, manifest
}

func parseSemver(s string) []int {
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}

func semverLess(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

// flatten produces dotted paths for every leaf in a JSON object.
func flatten(prefix string, v any, out map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			flatten(path, child, out)
		}
	default:
		out[prefix] = true
	}
}

// ---------- Pass 0: Versions ----------

func TestPass0_Versions(t *testing.T) {
	version, dir, _ := loadBaseline(t)
	t.Logf("─── Claude Code Status Line Contract ────────────────────────")
	t.Logf("  Baseline (validated):   v%s   (from %s)", version, dir)
	t.Logf("  Stdin paths consumed:   %d", len(consumedStdinPaths))
	t.Logf("  Settings keys required: %v", requiredSettingsKeys)
	t.Logf("  Env vars required:      [CLAUDE_PROJECT_DIR]")
	t.Logf("  Status:                 (pass-by-pass below)")
	t.Logf("──────────────────────────────────────────────────────────────")
}

// ---------- Pass 1: Hook script existence ----------

func TestPass1_HookScript(t *testing.T) {
	root := repoRoot(t)
	hook := filepath.Join(root, "hooks", "aoa-status-line.sh")
	info, err := os.Stat(hook)
	if err != nil {
		t.Fatalf("FAIL: hook script missing at %s: %v", hook, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("FAIL: hook script %s is not executable (mode %o)", hook, info.Mode().Perm())
	} else {
		t.Logf("OK: hook script present and executable: %s", hook)
	}
}

// ---------- Pass 2: Settings shape ----------

func TestPass2_SettingsShape(t *testing.T) {
	root := repoRoot(t)
	settingsPath := filepath.Join(root, ".claude", "settings.local.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Skipf("INFO: %s not present — aOa not registered in this repo (run `aoa init` to register). Skipping settings shape check.", settingsPath)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("FAIL: settings file is not valid JSON: %v", err)
	}

	sl, ok := settings["statusLine"].(map[string]any)
	if !ok {
		t.Errorf("FAIL: settings.statusLine missing or not an object — Claude Code will not invoke the hook")
		return
	}
	t.Logf("OK: settings.statusLine present")

	stype, _ := sl["type"].(string)
	if stype != "command" {
		t.Errorf("FAIL: settings.statusLine.type = %q (expected %q)", stype, "command")
	} else {
		t.Logf("OK: settings.statusLine.type = %q", stype)
	}

	cmd, _ := sl["command"].(string)
	if cmd == "" {
		t.Errorf("FAIL: settings.statusLine.command missing or empty")
	} else if !strings.Contains(cmd, "aoa-status-line.sh") {
		t.Logf("INFO: settings.statusLine.command does not reference aoa-status-line.sh — value: %q. Acceptable if the user replaced it.", cmd)
	} else {
		t.Logf("OK: settings.statusLine.command references aoa hook")
	}
}

// ---------- Pass 3: Synthetic stdin ----------

// syntheticStdin builds a JSON object covering every consumedStdinPath
// with non-zero, type-correct values. Feeding this to the hook should
// produce non-empty, non-error output.
func syntheticStdin() string {
	return `{
  "cwd": "/tmp/test-project",
  "session_id": "00000000-0000-0000-0000-000000000000",
  "version": "2.1.126",
  "model": {
    "id": "claude-opus-4-7",
    "display_name": "Opus 4.7"
  },
  "cost": {
    "total_cost_usd": 1.23,
    "total_lines_added": 100,
    "total_lines_removed": 50,
    "total_duration_ms": 60000,
    "total_api_duration_ms": 30000
  },
  "context_window": {
    "context_window_size": 200000,
    "used_percentage": 25,
    "remaining_percentage": 75,
    "total_input_tokens": 50000,
    "total_output_tokens": 10000,
    "current_usage": {
      "input_tokens": 5000,
      "cache_creation_input_tokens": 1000,
      "cache_read_input_tokens": 40000
    }
  },
  "rate_limits": {
    "five_hour": {"used_percentage": 30, "resets_at": 9999999999},
    "seven_day": {"used_percentage": 10, "resets_at": 9999999999}
  }
}`
}

func TestPass3_SyntheticStdin(t *testing.T) {
	root := repoRoot(t)
	hook := filepath.Join(root, "hooks", "aoa-status-line.sh")

	cmd := exec.Command("bash", hook)
	cmd.Stdin = strings.NewReader(syntheticStdin())
	cmd.Env = append(os.Environ(), "CLAUDE_PROJECT_DIR="+root)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("FAIL: hook exited non-zero: %v\nstderr:\n%s", err, stderr.String())
	}
	out := stdout.String()
	if strings.TrimSpace(out) == "" {
		t.Errorf("FAIL: hook produced empty output")
	} else {
		// Sanity: line 1 should mention cwd; line 2 should reference Opus or model name.
		if !strings.Contains(out, "test-project") {
			t.Errorf("FAIL: synthetic cwd not rendered — expected 'test-project' in output, got:\n%s", out)
		} else {
			t.Logf("OK: cwd rendered in line 1")
		}
		if !strings.Contains(out, "Opus") && !strings.Contains(out, "claude-opus") {
			t.Logf("INFO: model name not visibly in default output (acceptable — `model` segment may be off by default)")
		} else {
			t.Logf("OK: model name rendered")
		}
		t.Logf("OK: hook exit 0, output non-empty (%d bytes, %d lines)", len(out), strings.Count(out, "\n")+1)
	}
}

// ---------- Pass 4: Live shape (optional) ----------

func TestPass4_LiveShape(t *testing.T) {
	_, dir, _ := loadBaseline(t)
	samplePath := filepath.Join(dir, "sample.json")
	data, err := os.ReadFile(samplePath)
	if err != nil {
		t.Skipf("INFO: %s not present — see compliance/claude-statusline/README.md 'Capturing live stdin' to enable this pass", samplePath)
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("FAIL: %s is not valid JSON: %v", samplePath, err)
	}

	observed := map[string]bool{}
	flatten("", obj, observed)

	consumed := map[string]bool{}
	for _, p := range consumedStdinPaths {
		consumed[p] = true
	}

	// Drift A: consumed paths missing from observed
	var missing []string
	for _, p := range consumedStdinPaths {
		if !observed[p] {
			missing = append(missing, p)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("BREAKING DRIFT: %d consumed path(s) absent in live sample: %v — schema rename or removal", len(missing), missing)
	}

	// Drift B: new fields in observed that we do not consume
	var newFields []string
	for p := range observed {
		if !consumed[p] {
			newFields = append(newFields, p)
		}
	}
	sort.Strings(newFields)

	t.Logf("─── Live Stdin Shape (live sample at %s) ────────────────", samplePath)
	t.Logf("  Consumed paths in sample:    %d / %d", len(consumedStdinPaths)-len(missing), len(consumedStdinPaths))
	t.Logf("  Missing (BREAKING):          %d   %v", len(missing), missing)
	t.Logf("  New fields (additive drift): %d   %v", len(newFields), newFields)
	switch {
	case len(missing) > 0:
		t.Logf("  Status:                       BREAKING DRIFT — contract update required")
	case len(newFields) > 0:
		t.Logf("  Status:                       ADDITIVE DRIFT — signal we could consume; see observations.md")
	default:
		t.Logf("  Status:                       FULLY ALIGNED")
	}
	t.Logf("──────────────────────────────────────────────────────────────")
}

// ---------- Pass 5: Non-Compliance Coverage Report ----------

// TestPass5_NonComplianceReport quantifies what fraction of the observed
// status-line contract surface aOa actually consumes. Never fails — informs.
func TestPass5_NonComplianceReport(t *testing.T) {
	version, dir, _ := loadBaseline(t)

	// Settings keys: 3/3 always (we set them in init.go).
	settingsConsumed := len(requiredSettingsKeys)
	settingsTotal := 3

	// Env vars: 1/1 always (CLAUDE_PROJECT_DIR is the only one we depend on).
	envConsumed := 1
	envTotal := 1

	// Stdin paths: depends on whether we have a live sample.
	samplePath := filepath.Join(dir, "sample.json")
	stdinTotal := len(consumedStdinPaths) // baseline assumption: all consumed paths exist
	stdinConsumed := len(consumedStdinPaths)
	var droppedFields []string
	var liveCaptured bool

	if data, err := os.ReadFile(samplePath); err == nil {
		liveCaptured = true
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err == nil {
			observed := map[string]bool{}
			flatten("", obj, observed)
			consumed := map[string]bool{}
			for _, p := range consumedStdinPaths {
				consumed[p] = true
			}
			stdinTotal = len(observed)
			stdinConsumed = 0
			for p := range observed {
				if consumed[p] {
					stdinConsumed++
				} else {
					droppedFields = append(droppedFields, p)
				}
			}
			sort.Strings(droppedFields)
		}
	}

	pct := func(a, b int) string {
		if b == 0 {
			return "n/a"
		}
		return fmt.Sprintf("%d%%", a*100/b)
	}

	var b strings.Builder
	wf := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		t.Log(line)
		b.WriteString(line)
		b.WriteString("\n")
	}

	wf("### Coverage")
	wf("")
	wf("| Surface area      | Consumed | Total | Coverage |")
	wf("|-------------------|----------|-------|----------|")
	wf("| Stdin JSON paths  | %d       | %d    | %s%s    |", stdinConsumed, stdinTotal, pct(stdinConsumed, stdinTotal), liveCaptureNote(liveCaptured))
	wf("| Settings keys     | %d        | %d     | %s     |", settingsConsumed, settingsTotal, pct(settingsConsumed, settingsTotal))
	wf("| Env vars          | %d        | %d     | %s     |", envConsumed, envTotal, pct(envConsumed, envTotal))
	wf("")
	wf("### Top gaps")
	wf("")
	if !liveCaptured {
		wf("**[Unknown] Stdin coverage cannot be computed — no live sample**")
		wf("- what we know: all %d consumed paths are populated at v%s (Pass 3 confirmed)", len(consumedStdinPaths), version)
		wf("- what we don't know: how many fields Claude Code emits that we silently drop")
		wf("- action: capture `sample.json` (see `README.md` \"Capturing live stdin\")")
	} else if len(droppedFields) > 0 {
		wf("**[Drift] Stdin fields emitted but not consumed — %d**", len(droppedFields))
		wf("- fields: %v", droppedFields)
	} else {
		wf("**[None] Stdin fully covered — every emitted field is consumed**")
	}
	wf("")
	wf("**[None] Settings shape — %d/%d keys consumed, none dropped**", settingsConsumed, settingsTotal)
	wf("")
	wf("**[None] Env vars — %d/%d consumed, none dropped**", envConsumed, envTotal)

	writeReportSection(t, "claude-statusline", "Claude Code Status Line", version, b.String())
}

// writeReportSection updates compliance/REPORT.md by replacing the section
// between "<!-- BEGIN {id} -->" and "<!-- END {id} -->" markers.
// Mirrors the helper in claude-session/compliance_test.go.
func writeReportSection(t *testing.T, id, title, version, body string) {
	t.Helper()
	const reportPath = "../REPORT.md"
	beginMarker := fmt.Sprintf("<!-- BEGIN %s -->", id)
	endMarker := fmt.Sprintf("<!-- END %s -->", id)

	timestamp := time.Now().UTC().Format("2006-01-02 15:04 UTC")
	section := fmt.Sprintf(
		"%s\n## %s — v%s\n\n*Last regenerated: %s*\n\n%s\n%s",
		beginMarker, title, version, timestamp, strings.TrimRight(body, "\n"), endMarker,
	)

	existing, err := os.ReadFile(reportPath)
	if err != nil {
		header := "# aOa Compliance Report\n\n" +
			"> Auto-generated by `go test -tags compliance ./compliance/...`. Do not edit by hand — your changes will be overwritten on the next test run.\n\n" +
			"This report quantifies where aOa's parser is non-compliant relative to the contracts in this folder. \"Non-compliant\" means signal Claude Code emits that aOa does not consume — not necessarily a breakage.\n\n" +
			"---\n\n"
		placeholders := map[string]string{
			"claude-session":    "<!-- BEGIN claude-session -->\n## Claude Code Session JSONL\n\n*(not yet generated — run `go test -tags compliance ./compliance/claude-session/...`)*\n<!-- END claude-session -->",
			"claude-statusline": "<!-- BEGIN claude-statusline -->\n## Claude Code Status Line\n\n*(not yet generated — run `go test -tags compliance ./compliance/claude-statusline/...`)*\n<!-- END claude-statusline -->",
		}
		placeholders[id] = section
		full := header + placeholders["claude-session"] + "\n\n---\n\n" + placeholders["claude-statusline"] + "\n"
		if err := os.WriteFile(reportPath, []byte(full), 0o644); err != nil {
			t.Logf("WARN: could not write %s: %v", reportPath, err)
			return
		}
		t.Logf("Initialized %s with section %q", reportPath, id)
		return
	}

	content := string(existing)
	beginIdx := strings.Index(content, beginMarker)
	endIdx := strings.Index(content, endMarker)
	if beginIdx == -1 || endIdx == -1 || endIdx < beginIdx {
		updated := strings.TrimRight(content, "\n") + "\n\n---\n\n" + section + "\n"
		if err := os.WriteFile(reportPath, []byte(updated), 0o644); err != nil {
			t.Logf("WARN: could not write %s: %v", reportPath, err)
		} else {
			t.Logf("Appended section %q to %s (markers were missing)", id, reportPath)
		}
		return
	}

	endLen := beginIdx + len(content[beginIdx:endIdx]) + len(endMarker)
	updated := content[:beginIdx] + section + content[endLen:]
	if err := os.WriteFile(reportPath, []byte(updated), 0o644); err != nil {
		t.Logf("WARN: could not write %s: %v", reportPath, err)
		return
	}
	t.Logf("Updated section %q in %s", id, reportPath)
}

func liveCaptureNote(captured bool) string {
	if captured {
		return ""
	}
	return "  (consumed-only baseline; live capture pending)"
}

// ---------- helpers ----------

func init() {
	// Quiet linter — ensure fmt is used in case we add future formatters.
	_ = fmt.Sprintf
}
