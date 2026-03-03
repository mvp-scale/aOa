package cmd

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/corey/aoa/internal/version"
)

// globalBinDir is where the aoa symlink lives.
const globalBinDir = ".local/bin"

// globalDataDir is the base for versioned binaries and shims.
const globalDataDir = ".local/share/aoa"

// globalShimDir is where global shims (grep, egrep) are installed.
const globalShimDir = ".local/share/aoa/shims"

// globalVersionsDir is where versioned binaries are stored.
const globalVersionsDir = ".local/share/aoa/versions"

// shellRCSentinelStart marks the beginning of the aOa block in shell rc files.
const shellRCSentinelStart = "# >>> aOa >>>"

// shellRCSentinelEnd marks the end of the aOa block in shell rc files.
const shellRCSentinelEnd = "# <<< aOa <<<"

// shellRCBlock is the content written between sentinels in the shell rc file.
// Uses $HOME so the block is portable across systems.
const shellRCBlock = shellRCSentinelStart + `
export PATH="$HOME/.local/bin:$PATH"
alias claude='PATH="$HOME/.local/share/aoa/shims:$PATH" claude'
alias gemini='PATH="$HOME/.local/share/aoa/shims:$PATH" gemini'
` + shellRCSentinelEnd

// selfInstall copies the running binary to ~/.local/share/aoa/versions/{version}
// and symlinks ~/.local/bin/aoa to it.
// Returns the symlink path, whether a new version was installed, and any error.
func selfInstall() (path string, installed bool, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, fmt.Errorf("resolve home directory: %w", err)
	}

	ver := version.Version
	if ver == "" {
		ver = "dev"
	}

	versionDir := filepath.Join(home, globalVersionsDir, ver)
	versionedBin := filepath.Join(versionDir, "aoa")
	symlink := filepath.Join(home, globalBinDir, "aoa")

	src, err := os.Executable()
	if err != nil {
		return "", false, fmt.Errorf("resolve executable: %w", err)
	}
	src, err = filepath.EvalSymlinks(src)
	if err != nil {
		return "", false, fmt.Errorf("resolve symlinks: %w", err)
	}

	// Already running from the versioned location — just ensure symlink.
	if src == versionedBin {
		ensureSymlink(symlink, versionedBin)
		return symlink, false, nil
	}

	// Copy binary to versioned path if not identical.
	if !binaryIdentical(src, versionedBin) {
		if err := os.MkdirAll(versionDir, 0755); err != nil {
			return "", false, fmt.Errorf("create %s: %w", versionDir, err)
		}
		if err := copyFileTo(src, versionedBin); err != nil {
			return "", false, fmt.Errorf("copy binary: %w", err)
		}
		installed = true
	}

	// Ensure ~/.local/bin/aoa -> versioned binary.
	if err := os.MkdirAll(filepath.Dir(symlink), 0755); err != nil {
		return "", false, fmt.Errorf("create %s: %w", filepath.Dir(symlink), err)
	}
	ensureSymlink(symlink, versionedBin)

	return symlink, installed, nil
}

// ensureSymlink creates or updates a symlink to point at target.
func ensureSymlink(symlink, target string) {
	current, err := os.Readlink(symlink)
	if err == nil && current == target {
		return // already correct
	}
	os.Remove(symlink)
	os.Symlink(target, symlink)
}

// selfInstalledBinaryPath returns ~/.local/bin/aoa if it exists and resolves
// to an executable file. Returns empty string otherwise.
func selfInstalledBinaryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(home, globalBinDir, "aoa")
	// Stat follows symlinks — checks the actual target is executable.
	info, err := os.Stat(p)
	if err != nil {
		return ""
	}
	if info.Mode()&0111 == 0 {
		return "" // not executable
	}
	return p
}

// createGlobalShims writes grep and egrep shims to ~/.local/share/aoa/shims/.
// Uses aoaBin as the target binary path in the shims.
// Returns true if shims are in place (whether freshly written or already present).
func createGlobalShims(aoaBin string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	shimDir := filepath.Join(home, globalShimDir)
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not create global shims directory: %v\n", err)
		return false
	}

	shims := map[string]string{
		"grep":  fmt.Sprintf("#!/usr/bin/env bash\nexport AOA_SHIM=1\nexec %q grep \"$@\"\n", aoaBin),
		"egrep": fmt.Sprintf("#!/usr/bin/env bash\nexport AOA_SHIM=1\nexec %q egrep \"$@\"\n", aoaBin),
	}

	ok := true
	for name, content := range shims {
		path := filepath.Join(shimDir, name)

		// Skip if content is identical.
		existing, err := os.ReadFile(path)
		if err == nil && string(existing) == content {
			continue
		}

		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write global shim %s: %v\n", name, err)
			ok = false
		}
	}

	return ok
}

// configureShellRC writes the aOa sentinel block to the user's shell rc file.
// If the sentinel block already exists, it replaces it (handles upgrades).
// Returns the rc file path, whether the file was modified, and any error.
func configureShellRC() (rcFile string, modified bool, err error) {
	rcFile = detectShellRC()
	if rcFile == "" {
		return "", false, fmt.Errorf("could not determine shell rc file")
	}

	content, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return rcFile, false, fmt.Errorf("read %s: %w", rcFile, err)
	}

	text := string(content)

	// Check if sentinel block already exists.
	startIdx := strings.Index(text, shellRCSentinelStart)
	endIdx := strings.Index(text, shellRCSentinelEnd)

	if startIdx >= 0 && endIdx >= 0 {
		// Extract existing block for comparison.
		blockEnd := endIdx + len(shellRCSentinelEnd)
		if blockEnd < len(text) && text[blockEnd] == '\n' {
			blockEnd++
		}
		existingBlock := text[startIdx:blockEnd]
		if existingBlock == shellRCBlock+"\n" || existingBlock == shellRCBlock {
			return rcFile, false, nil // already up to date
		}

		// Replace the existing block (upgrade path).
		newText := text[:startIdx] + shellRCBlock + "\n" + text[blockEnd:]
		if err := os.WriteFile(rcFile, []byte(newText), 0644); err != nil {
			return rcFile, false, fmt.Errorf("write %s: %w", rcFile, err)
		}
		return rcFile, true, nil
	}

	// Append the block.
	f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return rcFile, false, fmt.Errorf("open %s: %w", rcFile, err)
	}
	defer f.Close()

	// Ensure there's a newline before our block if file has content.
	prefix := "\n"
	if len(content) == 0 {
		prefix = ""
	} else if len(content) > 0 && content[len(content)-1] == '\n' {
		prefix = ""
	}

	if _, err := f.WriteString(prefix + shellRCBlock + "\n"); err != nil {
		return rcFile, false, fmt.Errorf("write %s: %w", rcFile, err)
	}

	return rcFile, true, nil
}

// unconfigureShellRC removes the aOa sentinel block from the user's shell rc file.
// Returns true if the block was found and removed.
func unconfigureShellRC() bool {
	rcFile := detectShellRC()
	if rcFile == "" {
		return false
	}

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

	// Trim one leading newline before the block if present.
	blockStart := startIdx
	if blockStart > 0 && text[blockStart-1] == '\n' {
		blockStart--
	}

	cleaned := text[:blockStart] + text[blockEnd:]

	if err := os.WriteFile(rcFile, []byte(cleaned), 0644); err != nil {
		return false
	}
	return true
}

// detectShellRC determines the appropriate shell rc file for the current user.
// Returns the absolute path or empty string if indeterminate.
func detectShellRC() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	shell := os.Getenv("SHELL")

	if strings.HasSuffix(shell, "/zsh") {
		return filepath.Join(home, ".zshrc")
	}

	// Bash: macOS uses .bash_profile, Linux uses .bashrc
	if strings.HasSuffix(shell, "/bash") {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".bash_profile")
		}
		return filepath.Join(home, ".bashrc")
	}

	// Fallback: try zsh first (common default), then bashrc.
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); err == nil {
		return filepath.Join(home, ".zshrc")
	}
	return filepath.Join(home, ".bashrc")
}

// binaryIdentical returns true if two files exist and have identical SHA-256 hashes.
func binaryIdentical(a, b string) bool {
	ha, err := fileHash(a)
	if err != nil {
		return false
	}
	hb, err := fileHash(b)
	if err != nil {
		return false
	}
	return ha == hb
}

// fileHash computes the SHA-256 hash of a file.
func fileHash(path string) ([32]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return [32]byte{}, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return [32]byte{}, err
	}

	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

// copyFileTo copies src to dst atomically via a temp file + rename.
// The destination file is set to mode 0755.
func copyFileTo(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Write to temp file in the same directory (same filesystem for atomic rename).
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".aoa-install-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, srcFile); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}
