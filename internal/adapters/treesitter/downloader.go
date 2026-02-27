package treesitter

import (
	"os"
	"path/filepath"
	"runtime"
)

// PlatformString returns the OS-arch string for the current platform.
// e.g. "linux-amd64", "darwin-arm64"
func PlatformString() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// GlobalGrammarDir returns the default global grammar directory: ~/.aoa/grammars/
func GlobalGrammarDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".aoa", "grammars")
}
