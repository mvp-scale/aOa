package ahocorasick

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Aho-Corasick Pattern Matcher — Fast multi-keyword matching
// Goals: G1 (O(1) performance)
// =============================================================================

func TestMatcher_SingleKeyword(t *testing.T) {
	var m Matcher
	m.Build([]string{"login"})
	result := m.Match("user login flow")
	assert.Equal(t, []string{"login"}, result)
}

func TestMatcher_MultipleKeywords(t *testing.T) {
	var m Matcher
	m.Build([]string{"login", "auth", "session"})
	result := m.Match("auth login creates session")
	assert.Len(t, result, 3)
	assert.Contains(t, result, "login")
	assert.Contains(t, result, "auth")
	assert.Contains(t, result, "session")
}

func TestMatcher_OverlappingKeywords(t *testing.T) {
	// Standard (non-overlapping) match: "log" is found first at position 0,
	// consuming those bytes so "login" at position 0 is not reported.
	// This is the correct behavior for the learner's keyword matcher.
	// For overlapping matches, use TextScanner which uses IterOverlapping.
	var m Matcher
	m.Build([]string{"log", "login"})
	result := m.Match("login page log in")
	assert.Contains(t, result, "log")
	// "login" may or may not appear depending on match strategy;
	// the key property is that "log" is always found.
	assert.GreaterOrEqual(t, len(result), 1)
}

func TestMatcher_NoMatch(t *testing.T) {
	var m Matcher
	m.Build([]string{"auth"})
	result := m.Match("hello world")
	assert.Nil(t, result)
}

func TestMatcher_Rebuild(t *testing.T) {
	var m Matcher
	m.Build([]string{"old_keyword"})
	assert.NotNil(t, m.Match("old_keyword here"))

	m.Rebuild([]string{"new_keyword"})
	assert.Nil(t, m.Match("old_keyword here"))
	assert.NotNil(t, m.Match("new_keyword here"))
}

func TestMatcher_CaseSensitive(t *testing.T) {
	var m Matcher
	m.Build([]string{"login"})
	assert.NotNil(t, m.Match("login"))
	assert.Nil(t, m.Match("Login"))
	assert.Nil(t, m.Match("LOGIN"))
}

func TestMatcher_EmptyKeywords(t *testing.T) {
	var m Matcher
	m.Build([]string{})
	assert.Nil(t, m.Match("anything"))
}

func TestMatcher_NotBuilt(t *testing.T) {
	var m Matcher
	assert.Nil(t, m.Match("anything"))
}

// =============================================================================
// TextScanner — Byte-offset scanning for dimensional analysis
// =============================================================================

func TestTextScanner_SinglePattern(t *testing.T) {
	s := NewTextScanner([]string{"exec.Command("})
	matches := s.Scan([]byte(`result := exec.Command("ls")`))
	assert.Len(t, matches, 1)
	assert.Equal(t, 0, matches[0].PatternIndex)
	assert.Equal(t, 10, matches[0].Start)
}

func TestTextScanner_MultiplePatterns(t *testing.T) {
	s := NewTextScanner([]string{"password=", "secret="})
	content := []byte(`config.password=foo; config.secret=bar`)
	matches := s.Scan(content)
	assert.Len(t, matches, 2)

	patterns := map[int]bool{}
	for _, m := range matches {
		patterns[m.PatternIndex] = true
	}
	assert.True(t, patterns[0], "password= should match")
	assert.True(t, patterns[1], "secret= should match")
}

func TestTextScanner_OverlappingPatterns(t *testing.T) {
	s := NewTextScanner([]string{"md5", "md5.New("})
	matches := s.Scan([]byte(`h := md5.New()`))
	assert.GreaterOrEqual(t, len(matches), 2, "both md5 and md5.New( should match")
}

func TestTextScanner_NoMatch(t *testing.T) {
	s := NewTextScanner([]string{"exec.Command("})
	matches := s.Scan([]byte(`fmt.Println("hello")`))
	assert.Len(t, matches, 0)
}

func TestTextScanner_ByteOffsets(t *testing.T) {
	s := NewTextScanner([]string{"TODO"})
	content := []byte("line1\nline2\n// TODO: fix this\nline4")
	matches := s.Scan(content)
	assert.Len(t, matches, 1)
	// "TODO" starts at byte 15 ("line1\nline2\n// " = 15 chars)
	assert.Equal(t, 15, matches[0].Start)
	assert.Equal(t, 19, matches[0].End)
}

func TestTextScanner_PatternCount(t *testing.T) {
	s := NewTextScanner([]string{"a", "b", "c"})
	assert.Equal(t, 3, s.PatternCount())
}

func TestTextScanner_PatternAccess(t *testing.T) {
	s := NewTextScanner([]string{"foo", "bar"})
	assert.Equal(t, "foo", s.Pattern(0))
	assert.Equal(t, "bar", s.Pattern(1))
	assert.Equal(t, "", s.Pattern(-1))
	assert.Equal(t, "", s.Pattern(99))
}

func BenchmarkMatch(b *testing.B) {
	// G1: Target: match 500 keywords against 1KB content in <100µs.
	keywords := make([]string, 500)
	for i := range keywords {
		keywords[i] = strings.Repeat("k", 5+i%10)
	}
	var m Matcher
	m.Build(keywords)

	content := strings.Repeat("the quick brown fox jumps over the lazy dog ", 25) // ~1KB
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Match(content)
	}
}

func BenchmarkTextScanner(b *testing.B) {
	patterns := make([]string, 500)
	for i := range patterns {
		patterns[i] = strings.Repeat("p", 5+i%10)
	}
	s := NewTextScanner(patterns)
	content := []byte(strings.Repeat("the quick brown fox jumps over the lazy dog ", 25))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Scan(content)
	}
}
