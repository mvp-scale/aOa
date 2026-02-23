package cmd

import "os"

// isStdoutTTY returns true if stdout is connected to a terminal.
func isStdoutTTY() bool {
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
func resolveColor(colorFlag string, noColorFlag bool) bool {
	if noColorFlag {
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
