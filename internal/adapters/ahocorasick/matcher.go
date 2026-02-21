// Package ahocorasick provides multi-pattern string matching using an Aho-Corasick automaton.
// It wraps the petar-dambovaliev/aho-corasick library for O(n + m + z) matching.
package ahocorasick

import (
	aho "github.com/petar-dambovaliev/aho-corasick"
)

// Matcher implements fast multi-keyword matching for the learner.
// Build() compiles an automaton; Match() returns matching keywords.
type Matcher struct {
	automaton aho.AhoCorasick
	keywords  []string
	built     bool
}

// Build compiles the Aho-Corasick automaton from the given keywords.
func (m *Matcher) Build(keywords []string) {
	m.keywords = make([]string, len(keywords))
	copy(m.keywords, keywords)

	builder := aho.NewAhoCorasickBuilder(aho.Opts{
		DFA: true,
	})
	m.automaton = builder.Build(keywords)
	m.built = true
}

// Match returns all keywords found in content.
func (m *Matcher) Match(content string) []string {
	if !m.built || len(m.keywords) == 0 {
		return nil
	}
	matches := m.automaton.FindAll(content)
	if len(matches) == 0 {
		return nil
	}

	// Deduplicate by keyword
	seen := make(map[string]bool, len(matches))
	var result []string
	for i := range matches {
		kw := m.keywords[matches[i].Pattern()]
		if !seen[kw] {
			seen[kw] = true
			result = append(result, kw)
		}
	}
	return result
}

// Rebuild replaces the automaton with a new set of keywords.
func (m *Matcher) Rebuild(keywords []string) {
	m.Build(keywords)
}

// TextMatch represents a match from the TextScanner with byte offsets.
type TextMatch struct {
	PatternIndex int // index into the original patterns slice
	Start        int // byte offset start (inclusive)
	End          int // byte offset end (exclusive)
}

// TextScanner wraps an Aho-Corasick automaton for dimensional analysis text scanning.
// It returns byte offsets for each match, suitable for line-number computation.
type TextScanner struct {
	automaton aho.AhoCorasick
	patterns  []string
}

// NewTextScanner builds a text scanner from the given patterns.
func NewTextScanner(patterns []string) *TextScanner {
	builder := aho.NewAhoCorasickBuilder(aho.Opts{
		DFA: true,
	})
	p := make([]string, len(patterns))
	copy(p, patterns)
	return &TextScanner{
		automaton: builder.Build(p),
		patterns:  p,
	}
}

// Scan finds all pattern matches in content and returns them with byte offsets.
func (s *TextScanner) Scan(content []byte) []TextMatch {
	iter := s.automaton.IterOverlappingByte(content)
	var matches []TextMatch
	for next := iter.Next(); next != nil; next = iter.Next() {
		m := *next
		matches = append(matches, TextMatch{
			PatternIndex: m.Pattern(),
			Start:        m.Start(),
			End:          m.End(),
		})
	}
	return matches
}

// PatternCount returns the number of patterns in the automaton.
func (s *TextScanner) PatternCount() int {
	return len(s.patterns)
}

// Pattern returns the pattern string at the given index.
func (s *TextScanner) Pattern(idx int) string {
	if idx < 0 || idx >= len(s.patterns) {
		return ""
	}
	return s.patterns[idx]
}
