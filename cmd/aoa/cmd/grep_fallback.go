package cmd

import (
	"fmt"
	"os"
	"os/exec"
)

// systemGrepPaths are checked in order to find the real system grep.
// Absolute paths only to avoid finding our own shim via PATH.
var systemGrepPaths = []string{
	"/usr/bin/grep",
	"/bin/grep",
	"/usr/local/bin/grep",
}

// findSystemGrep locates the real system grep binary.
func findSystemGrep() string {
	for _, p := range systemGrepPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// fallbackSystemGrep executes the system grep with the given arguments,
// passing through stdin/stdout/stderr and propagating the exit code.
func fallbackSystemGrep(args []string) error {
	grepPath := findSystemGrep()
	if grepPath == "" {
		return fmt.Errorf("no system grep found and daemon not running. Start with: aoa daemon start")
	}

	cmd := exec.Command(grepPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return grepExit{exitErr.ExitCode()}
		}
		return err
	}
	return nil
}
