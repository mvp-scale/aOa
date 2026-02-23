package cmd

import "fmt"

// grepExit is returned by grep/egrep to signal a specific exit code.
// GNU grep: 0=found, 1=not found, 2=error.
type grepExit struct{ code int }

func (e grepExit) Error() string {
	switch e.code {
	case 0:
		return ""
	case 1:
		return "no match"
	default:
		return fmt.Sprintf("grep error (exit %d)", e.code)
	}
}

// GrepExitCode extracts the exit code from a grepExit error.
// Returns -1 if the error is not a grepExit.
func GrepExitCode(err error) int {
	if ge, ok := err.(grepExit); ok {
		return ge.code
	}
	return -1
}
