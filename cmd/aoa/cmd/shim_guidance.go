package cmd

import "math/rand"

// zeroResultHints are shown when a shim search returns no matches.
// These guide the agent to use aOa's capabilities instead of falling
// back to /usr/bin/grep.
var zeroResultHints = []string{
	"aOa: No matches in the index. Try OR mode: grep 'term1 term2' to broaden. Results are ranked by your session intent.",
	"aOa: No results. This is an O(1) index — try broader terms or check spelling. Use grep 'auth login' for OR across multiple keywords.",
	"aOa: Zero hits. The index respects .gitignore and ranks by recent work. Try synonyms or partial terms — tokenization splits camelCase and snake_case automatically.",
	"aOa: No matches found. Try: grep -a 'term1 term2' for AND mode, or grep 'term' for single keyword. All results are pre-ranked by your coding intent.",
	"aOa: Nothing matched. Remember: this index covers every symbol, function, and method. Try a shorter keyword — even 'auth' will find AuthHandler, authenticate, auth_token.",
}

// pickZeroResultHint returns a random hint for when shim search finds nothing.
func pickZeroResultHint() string {
	return zeroResultHints[rand.Intn(len(zeroResultHints))]
}
