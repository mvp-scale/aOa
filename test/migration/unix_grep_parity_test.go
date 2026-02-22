package migration

// Unix Grep Parity Test
//
// Tests aoa grep/egrep output against /usr/bin/grep for drop-in replacement.
// Each test case runs both system grep and defines the expected aoa behavior.
//
// This is NOT about search quality — it's about output format, flag handling,
// stdin, exit codes, and TTY behavior. An AI agent parsing aoa grep output
// must get the same structure it would get from GNU grep.
//
// Categories:
//   1. Output format (file:line:content, no decoration)
//   2. Positional file arguments (grep pattern file, grep pattern dir/)
//   3. Stdin piping (echo text | grep pattern)
//   4. Exit codes (0=found, 1=not found, 2=error)
//   5. Flag handling (every flag an agent might use)
//   6. Color/TTY behavior (no ANSI when stdout is not a terminal)
//   7. Egrep-specific (extended regex)

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFiles defines the fixture files for parity testing.
// These are real files on disk that both system grep and aoa grep will search.
var testFiles = map[string]string{
	"src/handler.py": `import os
import runpod
from config import settings

class RunpodHandler:
    """Main handler for RunPod serverless."""

    def __init__(self):
        self.client = runpod.Client()
        self.timeout = settings.TIMEOUT

    def handle(self, event):
        """Handle incoming RunPod event."""
        job_id = event.get("id")
        input_data = event.get("input", {})
        result = self.process(input_data)
        return {"status": "completed", "output": result}

    def process(self, data):
        """Process the input data."""
        return {"processed": True, "data": data}
`,
	"src/config.py": `import os

class Settings:
    """Application settings loaded from environment."""
    TIMEOUT = int(os.environ.get("TIMEOUT", "30"))
    DEBUG = os.environ.get("DEBUG", "false").lower() == "true"
    API_KEY = os.environ.get("RUNPOD_API_KEY", "")
    ENDPOINT = os.environ.get("RUNPOD_ENDPOINT", "https://api.runpod.io")

settings = Settings()
`,
	"src/utils.py": `"""Utility functions."""

def retry(fn, max_attempts=3):
    """Retry a function up to max_attempts times."""
    for attempt in range(max_attempts):
        try:
            return fn()
        except Exception as e:
            if attempt == max_attempts - 1:
                raise
            print(f"Retry {attempt + 1}/{max_attempts}: {e}")

def format_bytes(n):
    """Format bytes as human-readable string."""
    for unit in ["B", "KB", "MB", "GB"]:
        if n < 1024:
            return f"{n:.1f} {unit}"
        n /= 1024
    return f"{n:.1f} TB"
`,
	"README.md": `# Example Project

A sample project for testing grep parity.

## Setup

Install dependencies:
` + "```bash" + `
pip install runpod
` + "```" + `

## Usage

Run the handler:
` + "```bash" + `
python -m src.handler
` + "```" + `
`,
	"tests/test_handler.py": `import pytest
from src.handler import RunpodHandler

class TestHandler:
    def test_handle_returns_completed(self):
        handler = RunpodHandler()
        result = handler.handle({"id": "test-123", "input": {"key": "value"}})
        assert result["status"] == "completed"

    def test_process_returns_data(self):
        handler = RunpodHandler()
        result = handler.process({"key": "value"})
        assert result["processed"] is True

    def test_handle_missing_input(self):
        handler = RunpodHandler()
        result = handler.handle({"id": "test-456"})
        assert result["status"] == "completed"
`,
	"Dockerfile": `FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY src/ ./src/
CMD ["python", "-m", "src.handler"]
`,
	".gitignore": `__pycache__/
*.pyc
.env
node_modules/
`,
}

// setupTestDir creates the fixture files on disk and returns the temp directory path.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for relPath, content := range testFiles {
		fullPath := filepath.Join(dir, relPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}
	return dir
}

// sysGrep runs system grep and returns stdout, stderr, and exit code.
func sysGrep(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	grepPath, err := exec.LookPath("grep")
	if err != nil {
		t.Skip("system grep not found")
	}
	cmd := exec.Command(grepPath, args...)
	cmd.Dir = dir
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("grep exec error: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// sysEgrep runs system egrep (grep -E) and returns stdout, stderr, and exit code.
func sysEgrep(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	// Use grep -E since egrep is deprecated on many systems
	fullArgs := append([]string{"-E"}, args...)
	return sysGrep(t, dir, fullArgs...)
}

// sysGrepStdin runs system grep with stdin input.
func sysGrepStdin(t *testing.T, input string, args ...string) (stdout string, exitCode int) {
	t.Helper()
	grepPath, err := exec.LookPath("grep")
	if err != nil {
		t.Skip("system grep not found")
	}
	cmd := exec.Command(grepPath, args...)
	cmd.Stdin = strings.NewReader(input)
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	err = cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return outBuf.String(), exitCode
}

// =============================================================================
// 1. OUTPUT FORMAT — agents parse grep output, format must be identical
// =============================================================================

func TestUnixParity_OutputFormat_SingleFile(t *testing.T) {
	dir := setupTestDir(t)
	// grep pattern file → "line content" (no filename prefix when single file)
	stdout, _, code := sysGrep(t, dir, "runpod", "src/handler.py")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Equal(t, 2, len(lines), "should find 2 runpod lines in handler.py")
	// Single file: no filename prefix
	for _, line := range lines {
		assert.False(t, strings.HasPrefix(line, "src/handler.py:"),
			"single file grep should NOT prefix filename, got: %s", line)
	}
}

func TestUnixParity_OutputFormat_MultiFile(t *testing.T) {
	dir := setupTestDir(t)
	// grep -r pattern . → "file:content" (filename prefix when multiple files)
	stdout, _, code := sysGrep(t, dir, "-r", "runpod", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.GreaterOrEqual(t, len(lines), 3, "should find runpod across multiple files")
	for _, line := range lines {
		assert.Contains(t, line, ":", "multi-file grep must have file: prefix, got: %s", line)
	}
}

func TestUnixParity_OutputFormat_WithLineNumbers(t *testing.T) {
	dir := setupTestDir(t)
	// grep -rn pattern . → "file:linenum:content"
	stdout, _, code := sysGrep(t, dir, "-rn", "runpod", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		assert.GreaterOrEqual(t, len(parts), 3,
			"grep -rn output must be file:line:content, got: %s", line)
	}
}

func TestUnixParity_OutputFormat_NoHeaders(t *testing.T) {
	dir := setupTestDir(t)
	// GNU grep never prints summary headers — no "⚡ N hits" or elapsed time
	stdout, _, _ := sysGrep(t, dir, "-rn", "runpod", ".")
	assert.NotContains(t, stdout, "⚡", "grep output must not contain aOa headers")
	assert.NotContains(t, stdout, "hits", "grep output must not contain hit summary")
	assert.NotContains(t, stdout, "files │", "grep output must not contain file count summary")
}

func TestUnixParity_OutputFormat_NoTags(t *testing.T) {
	dir := setupTestDir(t)
	// GNU grep never prints #tags or @domain annotations
	stdout, _, _ := sysGrep(t, dir, "-rn", "runpod", ".")
	assert.NotContains(t, stdout, "@", "grep output must not contain @domain")
	assert.NotContains(t, stdout, "#", "grep output must not contain #tags")
}

func TestUnixParity_OutputFormat_NoIndentation(t *testing.T) {
	dir := setupTestDir(t)
	// GNU grep output lines start at column 0 — no leading spaces
	stdout, _, _ := sysGrep(t, dir, "-rn", "runpod", ".")
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		assert.False(t, strings.HasPrefix(line, " "),
			"grep output must not have leading spaces, got: %q", line)
	}
}

// =============================================================================
// 2. POSITIONAL FILE ARGUMENTS — agents pass file/dir paths after pattern
// =============================================================================

func TestUnixParity_FileArg_SingleFile(t *testing.T) {
	dir := setupTestDir(t)
	// grep pattern file.py → searches only that file
	stdout, _, code := sysGrep(t, dir, "import", "src/handler.py")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Equal(t, 3, len(lines), "should find 3 import lines in handler.py (import os, import runpod, from config import settings)")
}

func TestUnixParity_FileArg_MultipleFiles(t *testing.T) {
	dir := setupTestDir(t)
	// grep pattern file1 file2 → searches both, prefixes with filename
	stdout, _, code := sysGrep(t, dir, "import", "src/handler.py", "src/config.py")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.GreaterOrEqual(t, len(lines), 3, "should find imports in both files")
	// With multiple files, each line has filename prefix
	for _, line := range lines {
		assert.True(t,
			strings.HasPrefix(line, "src/handler.py:") || strings.HasPrefix(line, "src/config.py:"),
			"multi-file output must prefix filename, got: %s", line)
	}
}

func TestUnixParity_FileArg_Directory(t *testing.T) {
	dir := setupTestDir(t)
	// grep -r pattern dir/ → recursive search in subdirectory
	stdout, _, code := sysGrep(t, dir, "-r", "import", "src/")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.GreaterOrEqual(t, len(lines), 4, "should find imports across src/ files")
}

func TestUnixParity_FileArg_Nonexistent(t *testing.T) {
	dir := setupTestDir(t)
	// grep pattern nonexistent → exit code 2, error message
	_, stderr, code := sysGrep(t, dir, "pattern", "nonexistent.py")
	assert.Equal(t, 2, code, "nonexistent file should exit 2")
	assert.Contains(t, stderr, "No such file", "should report missing file")
}

func TestUnixParity_FileArg_Dot(t *testing.T) {
	dir := setupTestDir(t)
	// grep -r pattern . → "." means current directory, recursive
	stdout, _, code := sysGrep(t, dir, "-r", "runpod", ".")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "handler.py", "should find matches in handler.py")
	assert.Contains(t, stdout, "config.py", "should find matches in config.py")
}

// =============================================================================
// 3. STDIN PIPING — agents use grep as a filter
// =============================================================================

func TestUnixParity_Stdin_BasicFilter(t *testing.T) {
	input := "hello world\nfoo bar\nhello again\n"
	stdout, code := sysGrepStdin(t, input, "hello")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Equal(t, 2, len(lines), "should filter to 2 matching lines")
	assert.Equal(t, "hello world", lines[0])
	assert.Equal(t, "hello again", lines[1])
}

func TestUnixParity_Stdin_NoMatch(t *testing.T) {
	input := "hello world\nfoo bar\n"
	_, code := sysGrepStdin(t, input, "xyz")
	assert.Equal(t, 1, code, "no match on stdin should exit 1")
}

func TestUnixParity_Stdin_WithFlags(t *testing.T) {
	input := "Hello World\nhello again\nHELLO CAPS\n"
	stdout, code := sysGrepStdin(t, input, "-i", "hello")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Equal(t, 3, len(lines), "case insensitive stdin should match all 3")
}

func TestUnixParity_Stdin_Count(t *testing.T) {
	input := "aaa\nbbb\naaa\nccc\naaa\n"
	stdout, code := sysGrepStdin(t, input, "-c", "aaa")
	assert.Equal(t, 0, code)
	assert.Equal(t, "3\n", stdout, "-c on stdin should output count")
}

func TestUnixParity_Stdin_LineNumbers(t *testing.T) {
	input := "alpha\nbeta\nalpha\n"
	stdout, code := sysGrepStdin(t, input, "-n", "alpha")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Equal(t, "1:alpha", lines[0])
	assert.Equal(t, "3:alpha", lines[1])
}

// =============================================================================
// 4. EXIT CODES — agents use exit codes for control flow
// =============================================================================

func TestUnixParity_ExitCode_Found(t *testing.T) {
	dir := setupTestDir(t)
	_, _, code := sysGrep(t, dir, "runpod", "src/handler.py")
	assert.Equal(t, 0, code, "pattern found → exit 0")
}

func TestUnixParity_ExitCode_NotFound(t *testing.T) {
	dir := setupTestDir(t)
	_, _, code := sysGrep(t, dir, "zzzznothere", "src/handler.py")
	assert.Equal(t, 1, code, "pattern not found → exit 1")
}

func TestUnixParity_ExitCode_Error(t *testing.T) {
	dir := setupTestDir(t)
	_, _, code := sysGrep(t, dir, "pattern", "no_such_file.txt")
	assert.Equal(t, 2, code, "error (missing file) → exit 2")
}

func TestUnixParity_ExitCode_Quiet(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-q", "runpod", "src/handler.py")
	assert.Equal(t, 0, code, "-q with match → exit 0")
	assert.Empty(t, stdout, "-q should produce no output")
}

func TestUnixParity_ExitCode_QuietNotFound(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-q", "zzzznothere", "src/handler.py")
	assert.Equal(t, 1, code, "-q with no match → exit 1")
	assert.Empty(t, stdout, "-q should produce no output even on no match")
}

// =============================================================================
// 5. FLAG HANDLING — common flags agents use
// =============================================================================

func TestUnixParity_Flag_Recursive(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-r", "runpod", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.GreaterOrEqual(t, len(lines), 3, "-r should find runpod in multiple files")
}

func TestUnixParity_Flag_RecursiveLineNumbers(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rn", "runpod", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		assert.GreaterOrEqual(t, len(parts), 3,
			"-rn must produce file:line:content, got: %s", line)
	}
}

func TestUnixParity_Flag_CaseInsensitive(t *testing.T) {
	dir := setupTestDir(t)
	stdoutSens, _, _ := sysGrep(t, dir, "-r", "RunPod", ".")
	stdoutInsens, _, _ := sysGrep(t, dir, "-ri", "RunPod", ".")
	linesSens := strings.Split(strings.TrimSpace(stdoutSens), "\n")
	linesInsens := strings.Split(strings.TrimSpace(stdoutInsens), "\n")
	assert.Greater(t, len(linesInsens), len(linesSens),
		"-i should find more matches than case-sensitive")
}

func TestUnixParity_Flag_IncludeGlob(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rn", "--include=*.py", "runpod", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		assert.Contains(t, line, ".py",
			"--include=*.py should only return .py files, got: %s", line)
	}
}

func TestUnixParity_Flag_ExcludeGlob(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rn", "--exclude=*.md", "runpod", ".")
	assert.Equal(t, 0, code)
	assert.NotContains(t, stdout, "README.md",
		"--exclude=*.md should not return .md files")
}

func TestUnixParity_Flag_ExcludeDir(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rn", "--exclude-dir=tests", "import", ".")
	assert.Equal(t, 0, code)
	assert.NotContains(t, stdout, "tests/",
		"--exclude-dir=tests should not return files in tests/")
}

func TestUnixParity_Flag_CountSingleFile(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-c", "runpod", "src/handler.py")
	assert.Equal(t, 0, code)
	// Single file: just the count
	assert.Equal(t, "2\n", stdout, "-c on single file should return count only")
}

func TestUnixParity_Flag_CountMultiFile(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rc", "import", ".")
	assert.Equal(t, 0, code)
	// Multi file: file:count per file
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		if strings.HasSuffix(line, ":0") {
			continue // files with 0 matches
		}
		parts := strings.SplitN(line, ":", 2)
		assert.Equal(t, 2, len(parts),
			"-rc output must be file:count, got: %s", line)
	}
}

func TestUnixParity_Flag_FilesWithMatches(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rl", "runpod", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	// -l outputs filenames only, one per line
	for _, line := range lines {
		assert.NotContains(t, line, ":",
			"-l should output filenames only, not file:content, got: %s", line)
	}
	assert.GreaterOrEqual(t, len(lines), 3, "-l should list files with matches")
}

func TestUnixParity_Flag_FilesWithoutMatch(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rL", "runpod", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		// Files that contain lowercase "runpod" should NOT appear in -L output
		assert.NotEqual(t, "./src/handler.py", line,
			"-L should not list files that contain 'runpod'")
		assert.NotEqual(t, "./README.md", line,
			"-L should not list files that contain 'runpod'")
	}
	// Files without lowercase "runpod" SHOULD appear (case-sensitive matching)
	assert.Contains(t, stdout, "test_handler.py",
		"-L should list test_handler.py (contains RunpodHandler but not bare 'runpod')")
	assert.Contains(t, stdout, "utils.py",
		"-L should list utils.py (doesn't contain 'runpod')")
}

func TestUnixParity_Flag_InvertMatch(t *testing.T) {
	dir := setupTestDir(t)
	stdoutNormal, _, _ := sysGrep(t, dir, "import", "src/handler.py")
	stdoutInvert, _, _ := sysGrep(t, dir, "-v", "import", "src/handler.py")
	normalLines := strings.Count(stdoutNormal, "\n")
	invertLines := strings.Count(stdoutInvert, "\n")
	// All lines = normal + inverted
	totalLines := strings.Count(testFiles["src/handler.py"], "\n")
	assert.Equal(t, totalLines, normalLines+invertLines,
		"-v should invert: normal(%d) + inverted(%d) should equal total(%d)",
		normalLines, invertLines, totalLines)
}

func TestUnixParity_Flag_WholeWord(t *testing.T) {
	dir := setupTestDir(t)
	// "import" as whole word should not match "imported" or "importing"
	stdout, _, code := sysGrep(t, dir, "-rw", "import", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		// Each line should contain "import" as a whole word
		assert.Regexp(t, `\bimport\b`, line,
			"-w should match whole words only, got: %s", line)
	}
}

func TestUnixParity_Flag_OnlyMatching(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-o", "runpod", "src/handler.py")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		assert.Equal(t, "runpod", line,
			"-o should output only the matching text, got: %s", line)
	}
}

func TestUnixParity_Flag_MaxCount(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-m", "1", "import", "src/handler.py")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Equal(t, 1, len(lines), "-m 1 should return exactly 1 match")
}

func TestUnixParity_Flag_MultiplePatterns(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-e", "runpod", "-e", "settings", "src/handler.py")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	hasRunpod := false
	hasSettings := false
	for _, line := range lines {
		if strings.Contains(line, "runpod") {
			hasRunpod = true
		}
		if strings.Contains(line, "settings") {
			hasSettings = true
		}
	}
	assert.True(t, hasRunpod, "-e should match runpod")
	assert.True(t, hasSettings, "-e should match settings")
}

func TestUnixParity_Flag_ContextAfter(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-A", "2", "class RunpodHandler", "src/handler.py")
	assert.Equal(t, 0, code)
	// Output includes match line + 2 after lines (one may be blank)
	// Count raw lines including blanks
	rawLines := strings.Split(stdout, "\n")
	nonEmpty := 0
	for _, l := range rawLines {
		if l != "" {
			nonEmpty++
		}
	}
	assert.GreaterOrEqual(t, nonEmpty, 2,
		"-A 2 should show match + context lines, got %d non-empty lines", nonEmpty)
}

func TestUnixParity_Flag_ContextBefore(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-B", "2", "def handle", "src/handler.py")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.GreaterOrEqual(t, len(lines), 3,
		"-B 2 should show 2 context lines + match, got %d lines", len(lines))
}

func TestUnixParity_Flag_ContextBoth(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-C", "1", "def handle", "src/handler.py")
	assert.Equal(t, 0, code)
	// -C 1 shows 1 before + match + 1 after. With "def handle" matching once,
	// we get 3 lines. But the before line may be blank, so count raw lines.
	rawLines := strings.Split(stdout, "\n")
	nonEmpty := 0
	for _, l := range rawLines {
		if l != "" {
			nonEmpty++
		}
	}
	assert.GreaterOrEqual(t, nonEmpty, 2,
		"-C 1 should show context around match, got %d non-empty lines", nonEmpty)
}

// =============================================================================
// 6. COLOR / TTY BEHAVIOR — no ANSI when output is captured
// =============================================================================

func TestUnixParity_NoColor_WhenPiped(t *testing.T) {
	dir := setupTestDir(t)
	// When grep output goes to a pipe (not a TTY), no ANSI codes
	// Our test harness captures stdout, so this is already non-TTY
	stdout, _, _ := sysGrep(t, dir, "-rn", "runpod", ".")
	assert.NotContains(t, stdout, "\033[", "piped grep must not contain ANSI escape codes")
	assert.NotContains(t, stdout, "\x1b[", "piped grep must not contain ANSI escape codes")
}

func TestUnixParity_Color_Never(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, _ := sysGrep(t, dir, "-rn", "--color=never", "runpod", ".")
	assert.NotContains(t, stdout, "\033[", "--color=never must not contain ANSI codes")
}

// =============================================================================
// 7. EGREP — extended regex patterns
// =============================================================================

func TestUnixParity_Egrep_BasicRegex(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysEgrep(t, dir, "-rn", "run.od", ".")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "runpod", "egrep run.od should match runpod")
}

func TestUnixParity_Egrep_Alternation(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysEgrep(t, dir, "-rn", "import|class", "src/handler.py")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	hasImport := false
	hasClass := false
	for _, line := range lines {
		if strings.Contains(line, "import") {
			hasImport = true
		}
		if strings.Contains(line, "class") {
			hasClass = true
		}
	}
	assert.True(t, hasImport, "egrep alternation should match import")
	assert.True(t, hasClass, "egrep alternation should match class")
}

func TestUnixParity_Egrep_Quantifiers(t *testing.T) {
	dir := setupTestDir(t)
	// + quantifier (one or more)
	stdout, _, code := sysEgrep(t, dir, "-rn", "r[a-z]+d", ".")
	assert.Equal(t, 0, code)
	assert.NotEmpty(t, stdout, "egrep r[a-z]+d should match 'runpod' etc")
}

func TestUnixParity_Egrep_IncludeGlob(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysEgrep(t, dir, "-rn", "--include=*.py", "def .+\\(", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		assert.Contains(t, line, ".py",
			"egrep --include=*.py should only return .py files, got: %s", line)
	}
}

// =============================================================================
// 8. COMBINED FLAGS — real-world agent invocations
// =============================================================================

func TestUnixParity_RealWorld_ClaudeGrep(t *testing.T) {
	// Claude Code's most common grep pattern
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rn", "runpod", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Equal(t, 4, len(lines), "grep -rn runpod . should return 4 matches")
	// Verify exact format: ./file:line:content
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		assert.GreaterOrEqual(t, len(parts), 3, "expected file:line:content, got: %s", line)
	}
}

func TestUnixParity_RealWorld_ClaudeGrepInclude(t *testing.T) {
	// Claude Code searching specific file types
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rn", "--include=*.py", "runpod", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Equal(t, 3, len(lines), "--include=*.py should return 3 py-only matches")
	for _, line := range lines {
		assert.True(t,
			strings.Contains(line, ".py:"),
			"--include=*.py must only return py files, got: %s", line)
	}
}

func TestUnixParity_RealWorld_GeminiGrep(t *testing.T) {
	// Gemini tends to use grep -ri for broad search
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-ri", "runpod", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.GreaterOrEqual(t, len(lines), 5)
}

func TestUnixParity_RealWorld_PipeChain(t *testing.T) {
	// Agents sometimes chain: grep -rl pattern . | head -5
	// The grep part should still work correctly
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rl", "import", ".")
	assert.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	// Each line is a filename, no colons
	for _, line := range lines {
		assert.True(t,
			strings.HasSuffix(line, ".py") || strings.HasSuffix(line, ".md"),
			"-rl should output filenames only, got: %s", line)
	}
}

// =============================================================================
// 9. EDGE CASES — things that trip up naive implementations
// =============================================================================

func TestUnixParity_Edge_PatternWithDash(t *testing.T) {
	dir := setupTestDir(t)
	// Pattern starting with dash: need -- separator
	stdout, _, code := sysGrep(t, dir, "-rn", "--", "--no-cache-dir", ".")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "Dockerfile", "should find --no-cache-dir in Dockerfile")
}

func TestUnixParity_Edge_PatternWithSpecialChars(t *testing.T) {
	dir := setupTestDir(t)
	// Pattern with regex special chars used literally with -F
	stdout, _, code := sysGrep(t, dir, "-F", "f\"{n:.1f}", "src/utils.py")
	assert.Equal(t, 0, code)
	assert.NotEmpty(t, stdout, "-F should match literal special chars")
}

func TestUnixParity_Edge_EmptyResult(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-r", "xyznonexistent123", ".")
	assert.Equal(t, 1, code, "no matches → exit 1")
	assert.Empty(t, stdout, "no matches → empty stdout")
}

func TestUnixParity_Edge_BinaryFileSkip(t *testing.T) {
	dir := setupTestDir(t)
	// Create a binary file
	binPath := filepath.Join(dir, "data.bin")
	require.NoError(t, os.WriteFile(binPath, []byte{0x00, 0x01, 0x02, 'h', 'e', 'l', 'l', 'o', 0x00}, 0644))
	_, _, code := sysGrep(t, dir, "-r", "hello", ".")
	// GNU grep still finds it but prints "Binary file matches"
	assert.Equal(t, 0, code)
}

func TestUnixParity_Edge_SymlinkFollow(t *testing.T) {
	dir := setupTestDir(t)
	// Create a symlink
	linkPath := filepath.Join(dir, "link_handler.py")
	targetPath := filepath.Join(dir, "src", "handler.py")
	err := os.Symlink(targetPath, linkPath)
	if err != nil {
		t.Skip("symlinks not supported")
	}
	stdout, _, code := sysGrep(t, dir, "runpod", "link_handler.py")
	assert.Equal(t, 0, code)
	assert.NotEmpty(t, stdout, "grep should follow symlinks by default")
}

// =============================================================================
// REFERENCE: Expected hit counts for baseline validation
// =============================================================================

func TestUnixParity_Baseline_HitCounts(t *testing.T) {
	dir := setupTestDir(t)

	tests := []struct {
		name     string
		args     []string
		minHits  int
		maxHits  int
	}{
		// Actual GNU grep counts on fixture files (case-sensitive "runpod"):
		// handler.py:2, config.py:1 (RUNPOD_ENDPOINT line), README.md:1 = 4 total
		{"grep -r runpod .", []string{"-r", "runpod", "."}, 4, 4},
		// Case-insensitive catches RunpodHandler, RUNPOD_API_KEY, etc = 12 total
		{"grep -ri runpod .", []string{"-ri", "runpod", "."}, 10, 14},
		{"grep -rn runpod .", []string{"-rn", "runpod", "."}, 4, 4},
		{"grep runpod src/handler.py", []string{"runpod", "src/handler.py"}, 2, 2},
		{"grep -r import .", []string{"-r", "import", "."}, 5, 15},
		// --include=*.py: handler.py:2 + config.py:1 = 3
		{"grep -rn --include=*.py runpod .", []string{"-rn", "--include=*.py", "runpod", "."}, 3, 3},
		// -rc: one line per file (including 0-count files)
		{"grep -rc runpod .", []string{"-rc", "runpod", "."}, 3, 10},
		// -rl: files with matches (handler.py, config.py, README.md)
		{"grep -rl runpod .", []string{"-rl", "runpod", "."}, 3, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, code := sysGrep(t, dir, tt.args...)
			assert.Equal(t, 0, code, "expected matches for: %s", tt.name)
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			actual := len(lines)
			assert.GreaterOrEqual(t, actual, tt.minHits,
				"%s: expected >= %d lines, got %d", tt.name, tt.minHits, actual)
			assert.LessOrEqual(t, actual, tt.maxHits,
				"%s: expected <= %d lines, got %d\nOutput:\n%s", tt.name, tt.maxHits, actual, stdout)
		})
	}
}

// =============================================================================
// REFERENCE: Snapshot exact GNU grep output for key scenarios
// =============================================================================

func TestUnixParity_Snapshot_RecursiveFormat(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, code := sysGrep(t, dir, "-rn", "class RunpodHandler", ".")
	assert.Equal(t, 0, code)
	// Snapshot the exact format for implementation reference
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Equal(t, 1, len(lines), "should match exactly one line")
	// Expected: ./src/handler.py:5:class RunpodHandler:
	line := lines[0]
	assert.True(t, strings.HasPrefix(line, "./src/handler.py:5:"),
		"expected ./src/handler.py:5:..., got: %s", line)
	assert.Contains(t, line, "class RunpodHandler")
	t.Logf("GNU grep output: %q", line)
}

func TestUnixParity_Snapshot_CountFormat(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, _ := sysGrep(t, dir, "-c", "import", "src/handler.py")
	t.Logf("GNU grep -c output: %q", stdout)
	// handler.py has 3 lines containing "import": import os, import runpod, from config import settings
	assert.Equal(t, "3\n", stdout, "-c should output just the count as a number")
}

func TestUnixParity_Snapshot_FilesOnlyFormat(t *testing.T) {
	dir := setupTestDir(t)
	stdout, _, _ := sysGrep(t, dir, "-rl", "runpod", ".")
	t.Logf("GNU grep -rl output: %q", stdout)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	fmt.Println(lines)
	for _, line := range lines {
		// Each line is a relative path, no colon, no content
		assert.NotContains(t, line, ":")
	}
}
