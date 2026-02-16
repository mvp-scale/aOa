package ports

// PatternMatcher finds keywords in content using multi-pattern matching (Aho-Corasick).
// A single pass over the content finds all matching keywords simultaneously,
// regardless of how many keywords are in the set. This is O(n + m + z) where
// n=content length, m=total pattern length, z=number of matches.
//
// The matcher must be rebuilt when the keyword set changes (e.g., after autotune
// adds/removes keywords). Rebuild is expected to be infrequent (<1/minute).
type PatternMatcher interface {
	// Match returns all keywords found in content. Results may contain
	// duplicates if a keyword appears multiple times. Returns nil if
	// no keywords match. Content is matched as-is (caller normalizes case).
	Match(content string) []string

	// Rebuild replaces the entire keyword set and reconstructs the automaton.
	// Previous keywords are discarded. Returns an error if the keyword set
	// is invalid (e.g., empty keyword string).
	Rebuild(keywords map[string]Mapping) error
}

// Mapping represents the resolution chain for a matched keyword:
// keyword -> Term -> Domain. Used by the enricher to tag search results.
type Mapping struct {
	Term   string // Intermediate grouping (e.g., "auth" for keyword "login")
	Domain string // Final domain (e.g., "@authentication" for term "auth")
}
