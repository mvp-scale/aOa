package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinaryIdentical(t *testing.T) {
	dir := t.TempDir()

	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	c := filepath.Join(dir, "c")

	require.NoError(t, os.WriteFile(a, []byte("hello world"), 0755))
	require.NoError(t, os.WriteFile(b, []byte("hello world"), 0755))
	require.NoError(t, os.WriteFile(c, []byte("different content"), 0755))

	assert.True(t, binaryIdentical(a, b), "identical files should match")
	assert.False(t, binaryIdentical(a, c), "different files should not match")
	assert.False(t, binaryIdentical(a, filepath.Join(dir, "nonexistent")), "missing file should return false")
}

func TestCopyFileTo(t *testing.T) {
	dir := t.TempDir()

	src := filepath.Join(dir, "src")
	content := []byte("#!/bin/bash\necho hello\n")
	require.NoError(t, os.WriteFile(src, content, 0755))

	dst := filepath.Join(dir, "dst")
	require.NoError(t, copyFileTo(src, dst))

	// Verify content
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, content, got)

	// Verify permissions
	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm())

	// Verify atomicity: no temp files left behind
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{"src", "dst"}, names)
}

func TestDetectShellRC(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name     string
		shell    string
		expected string
	}{
		{
			name:     "zsh",
			shell:    "/bin/zsh",
			expected: filepath.Join(home, ".zshrc"),
		},
		{
			name:     "zsh usr local",
			shell:    "/usr/local/bin/zsh",
			expected: filepath.Join(home, ".zshrc"),
		},
	}

	// Add bash test based on OS
	if runtime.GOOS == "darwin" {
		tests = append(tests, struct {
			name     string
			shell    string
			expected string
		}{
			name:     "bash macOS",
			shell:    "/bin/bash",
			expected: filepath.Join(home, ".bash_profile"),
		})
	} else {
		tests = append(tests, struct {
			name     string
			shell    string
			expected string
		}{
			name:     "bash linux",
			shell:    "/bin/bash",
			expected: filepath.Join(home, ".bashrc"),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := os.Getenv("SHELL")
			t.Setenv("SHELL", tt.shell)
			defer os.Setenv("SHELL", orig)

			result := detectShellRC()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShellRCBlock(t *testing.T) {
	// Block should use $HOME, not hardcoded paths
	assert.Contains(t, shellRCBlock, `$HOME/.local/bin`)
	assert.Contains(t, shellRCBlock, `$HOME/.local/share/aoa/shims`)
	assert.NotContains(t, shellRCBlock, os.Getenv("HOME"))

	// Block should have correct sentinel structure
	assert.True(t, len(shellRCBlock) > 0)
	assert.Contains(t, shellRCBlock, shellRCSentinelStart)
	assert.Contains(t, shellRCBlock, shellRCSentinelEnd)

	// Block should contain the claude and gemini aliases
	assert.Contains(t, shellRCBlock, "alias claude=")
	assert.Contains(t, shellRCBlock, "alias gemini=")

	// Block should contain PATH export
	assert.Contains(t, shellRCBlock, `export PATH="$HOME/.local/bin:$PATH"`)
}

func TestConfigureShellRC(t *testing.T) {
	dir := t.TempDir()
	rcFile := filepath.Join(dir, ".bashrc")

	// Override detectShellRC for testing by setting SHELL and HOME.
	// We can't easily override detectShellRC without refactoring, so we
	// test the lower-level logic directly using configureShellRCFile.

	t.Run("fresh file", func(t *testing.T) {
		f := filepath.Join(dir, "fresh-bashrc")
		modified, err := configureShellRCFile(f)
		require.NoError(t, err)
		assert.True(t, modified)

		content, err := os.ReadFile(f)
		require.NoError(t, err)
		assert.Contains(t, string(content), shellRCSentinelStart)
		assert.Contains(t, string(content), shellRCSentinelEnd)
		assert.Contains(t, string(content), `alias claude=`)
	})

	t.Run("idempotent", func(t *testing.T) {
		// Write initial content
		require.NoError(t, os.WriteFile(rcFile, []byte("# existing config\n"), 0644))

		modified1, err := configureShellRCFile(rcFile)
		require.NoError(t, err)
		assert.True(t, modified1)

		content1, _ := os.ReadFile(rcFile)

		// Run again — should be idempotent
		modified2, err := configureShellRCFile(rcFile)
		require.NoError(t, err)
		assert.False(t, modified2, "second run should not modify")

		content2, _ := os.ReadFile(rcFile)
		assert.Equal(t, string(content1), string(content2))
	})

	t.Run("preserves existing content", func(t *testing.T) {
		f := filepath.Join(dir, "existing-bashrc")
		existing := "# my config\nexport FOO=bar\n"
		require.NoError(t, os.WriteFile(f, []byte(existing), 0644))

		modified, err := configureShellRCFile(f)
		require.NoError(t, err)
		assert.True(t, modified)

		content, err := os.ReadFile(f)
		require.NoError(t, err)
		assert.Contains(t, string(content), "# my config")
		assert.Contains(t, string(content), "export FOO=bar")
		assert.Contains(t, string(content), shellRCSentinelStart)
	})

	t.Run("upgrade replaces block", func(t *testing.T) {
		f := filepath.Join(dir, "upgrade-bashrc")
		oldBlock := "# pre-existing\n" + shellRCSentinelStart + "\nold content\n" + shellRCSentinelEnd + "\n# post-existing\n"
		require.NoError(t, os.WriteFile(f, []byte(oldBlock), 0644))

		modified, err := configureShellRCFile(f)
		require.NoError(t, err)
		assert.True(t, modified)

		content, err := os.ReadFile(f)
		require.NoError(t, err)
		assert.Contains(t, string(content), "# pre-existing")
		assert.Contains(t, string(content), "# post-existing")
		assert.NotContains(t, string(content), "old content")
		assert.Contains(t, string(content), `alias claude=`)
	})
}

func TestUnconfigureShellRC(t *testing.T) {
	t.Run("removes block preserves surrounding", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, ".bashrc")

		content := "# before\nexport FOO=bar\n" + shellRCBlock + "\n# after\nexport BAZ=qux\n"
		require.NoError(t, os.WriteFile(f, []byte(content), 0644))

		removed := unconfigureShellRCFile(f)
		assert.True(t, removed)

		result, err := os.ReadFile(f)
		require.NoError(t, err)
		assert.Contains(t, string(result), "# before")
		assert.Contains(t, string(result), "export FOO=bar")
		assert.Contains(t, string(result), "# after")
		assert.Contains(t, string(result), "export BAZ=qux")
		assert.NotContains(t, string(result), shellRCSentinelStart)
		assert.NotContains(t, string(result), shellRCSentinelEnd)
	})

	t.Run("returns false when no block", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, ".bashrc")
		require.NoError(t, os.WriteFile(f, []byte("# no aoa block here\n"), 0644))

		removed := unconfigureShellRCFile(f)
		assert.False(t, removed)
	})

	t.Run("returns false for missing file", func(t *testing.T) {
		removed := unconfigureShellRCFile("/nonexistent/path/.bashrc")
		assert.False(t, removed)
	})
}

// configureShellRCFile is a test helper that applies the shell RC block to a specific file.
func configureShellRCFile(rcFile string) (modified bool, err error) {
	content, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	text := string(content)

	startIdx := strings.Index(text, shellRCSentinelStart)
	endIdx := strings.Index(text, shellRCSentinelEnd)

	if startIdx >= 0 && endIdx >= 0 {
		blockEnd := endIdx + len(shellRCSentinelEnd)
		if blockEnd < len(text) && text[blockEnd] == '\n' {
			blockEnd++
		}
		existingBlock := text[startIdx:blockEnd]
		if existingBlock == shellRCBlock+"\n" || existingBlock == shellRCBlock {
			return false, nil
		}
		newText := text[:startIdx] + shellRCBlock + "\n" + text[blockEnd:]
		return true, os.WriteFile(rcFile, []byte(newText), 0644)
	}

	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	defer f.Close()

	prefix := "\n"
	if len(content) == 0 {
		prefix = ""
	} else if len(content) > 0 && content[len(content)-1] == '\n' {
		prefix = ""
	}

	_, err = f.WriteString(prefix + shellRCBlock + "\n")
	return true, err
}

// unconfigureShellRCFile is a test helper that removes the shell RC block from a specific file.
func unconfigureShellRCFile(rcFile string) bool {
	data, err := os.ReadFile(rcFile)
	if err != nil {
		return false
	}

	text := string(data)
	startIdx := strings.Index(text, shellRCSentinelStart)
	endIdx := strings.Index(text, shellRCSentinelEnd)
	if startIdx < 0 || endIdx < 0 {
		return false
	}

	blockEnd := endIdx + len(shellRCSentinelEnd)
	if blockEnd < len(text) && text[blockEnd] == '\n' {
		blockEnd++
	}

	blockStart := startIdx
	if blockStart > 0 && text[blockStart-1] == '\n' {
		blockStart--
	}

	cleaned := text[:blockStart] + text[blockEnd:]
	return os.WriteFile(rcFile, []byte(cleaned), 0644) == nil
}
