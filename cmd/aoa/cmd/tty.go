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

// isAgentMode returns true when aoa is being driven by an AI agent host.
// Explicit AOA_AGENT=1/0 always wins; otherwise the agent host is
// auto-detected (Claude Code sets CLAUDECODE=1 in its Bash tool).
// Agent mode turns on the semantic agent grammar (header, peek codes,
// [start-end] boundaries, @domains, hints) and disables implicit stdin
// reads. It is DISTINCT from shim mode: the shim's GNU-compatible output
// contract is load-bearing and unchanged (pipe-repair consensus F6a).
func isAgentMode() bool {
	switch os.Getenv("AOA_AGENT") {
	case "1":
		return true
	case "0":
		return false
	}
	return os.Getenv("CLAUDECODE") == "1"
}

// isStdinPipe returns true if stdin is a pipe (not a terminal).
func isStdinPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice == 0
}

// stdinAllowedByMode reports whether the current mode permits implicit
// stdin consumption. Shim mode ALWAYS permits it — shims fire inside agent
// Bash subshells (CLAUDECODE=1 present), and their GNU stdin-filter contract
// is binding (F6a: shim wins over agent mode). Outside shim mode, agent mode
// forbids it: agent hosts hand tools an open pipe or /dev/null, and a tool
// blocking on stdin hangs the agent.
func stdinAllowedByMode() bool {
	return isShimMode() || !isAgentMode()
}

// shouldReadStdin reports whether a no-file-arg search may consume stdin
// (GNU grep filter behavior).
func shouldReadStdin() bool {
	return isStdinPipe() && stdinAllowedByMode()
}

// useSemanticFormat reports whether search output uses the semantic agent
// grammar (header, peek codes, [start-end], @domains) instead of GNU-compat.
// One selector for grep AND egrep — the guidance teaches both.
func useSemanticFormat() bool {
	return isShimMode() || showPeekCodes() || showHints()
}

// showPeekCodes returns true when peek codes should appear in search output.
// Enabled by AOA_PEEK=1, or automatically in shim or agent mode.
// Explicitly disabled by AOA_PEEK=0 (overrides both defaults).
func showPeekCodes() bool {
	if v := os.Getenv("AOA_PEEK"); v != "" {
		return v == "1"
	}
	return isShimMode() || isAgentMode()
}

// showHints returns true when guidance hints should appear in search output.
// Enabled by AOA_HINTS=1, or automatically in shim or agent mode.
// Explicitly disabled by AOA_HINTS=0 (overrides both defaults).
func showHints() bool {
	if v := os.Getenv("AOA_HINTS"); v != "" {
		return v == "1"
	}
	return isShimMode() || isAgentMode()
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
