package cmd

import "os"

// isShimMode returns true when AOA_SHIM=1 is set in the environment.
// This signals that aoa is running as a transparent Unix shim (e.g.
// ~/.aoa/shims/grep). In shim mode:
//   - Output is pure GNU grep/find/locate compatible (no ANSI, no emoji)
//   - Exit codes follow GNU conventions
//   - Daemon-unavailable errors fall back to system grep (with correct args)
//   - No rich formatting regardless of TTY status
func isShimMode() bool {
	return os.Getenv("AOA_SHIM") == "1"
}

// isStdoutTTY returns true if stdout is connected to a terminal.
// Always returns false in shim mode — shim output must be machine-parseable.
func isStdoutTTY() bool {
	if isShimMode() {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// isStdinPipe returns true if stdin is a pipe (not a terminal).
func isStdinPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice == 0
}

// resolveColor determines whether to use color output based on flags and TTY status.
// colorFlag is the --color value: "auto", "always", or "never".
// noColorFlag is the --no-color boolean flag.
// Always returns false in shim mode — no ANSI codes in shim output.
func resolveColor(colorFlag string, noColorFlag bool) bool {
	if isShimMode() || noColorFlag {
		return false
	}
	switch colorFlag {
	case "always":
		return true
	case "never":
		return false
	default: // "auto"
		return isStdoutTTY()
	}
}
