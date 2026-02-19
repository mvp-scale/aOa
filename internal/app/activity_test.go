package app

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/corey/aoa/atlas"
	"github.com/corey/aoa/internal/domain/enricher"
	"github.com/corey/aoa/internal/domain/learner"
	"github.com/corey/aoa/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestApp creates a minimal App suitable for activity/rubric tests.
// No bbolt, no socket server, no watcher, no web server — just the fields
// needed by onSessionEvent, searchAttrib, searchTarget, readSavings, and pushActivity.
func newTestApp(t *testing.T) *App {
	t.Helper()

	enr, err := enricher.NewFromFS(atlas.FS, "v1")
	require.NoError(t, err, "load atlas enricher")

	idx := &ports.Index{
		Tokens:   make(map[string][]ports.TokenRef),
		Metadata: make(map[ports.TokenRef]*ports.SymbolMeta),
		Files: map[uint32]*ports.FileMeta{
			1: {Path: "/home/user/project/src/big.go", Size: 176000, Language: "go"},    // 176KB → 44k tokens
			2: {Path: "/home/user/project/src/small.go", Size: 10000, Language: "go"},   // 10KB → 2500 tokens
			3: {Path: "/home/user/project/src/foo.go", Size: 2000, Language: "go"},      // 2KB → 500 tokens
			4: {Path: "/home/user/project/src/bar.go", Size: 1000, Language: "go"},      // 1KB → 250 tokens
			5: {Path: "/home/user/project/src/baz.go", Size: 800, Language: "go"},       // 0.8KB → 200 tokens
			6: {Path: "/home/user/project/src/handlers/auth.go", Size: 500, Language: "go"},
		},
	}

	a := &App{
		ProjectRoot: "/home/user/project",
		ProjectID:   "project",
		Enricher:    enr,
		Learner:     learner.New(),
		Index:       idx,
		toolMetrics: ToolMetrics{
			FileReads:    make(map[string]int),
			BashCommands: make(map[string]int),
			GrepPatterns: make(map[string]int),
		},
		turnBuffer:          make(map[string]*turnBuilder),
		burnRate:            NewBurnRateTracker(5 * time.Minute),
		burnRateCounterfact: NewBurnRateTracker(5 * time.Minute),
	}
	return a
}

// lastActivity reads the most recent entry from the activity ring buffer.
// Returns nil if the ring is empty.
func lastActivity(a *App) *ActivityEntry {
	if a.activityCount == 0 {
		return nil
	}
	idx := (a.activityHead - 1 + 50) % 50
	e := a.activityRing[idx]
	return &e
}

// --- searchAttrib tests ---

func TestSearchAttrib(t *testing.T) {
	a := newTestApp(t)

	t.Run("single_token_literal", func(t *testing.T) {
		got := a.searchAttrib("auth", ports.SearchOptions{})
		assert.Equal(t, "indexed", got)
	})

	t.Run("multi_token_literal", func(t *testing.T) {
		got := a.searchAttrib("auth login", ports.SearchOptions{})
		assert.Equal(t, "multi-or", got)
	})

	t.Run("and_mode", func(t *testing.T) {
		// A-02: current code returns "and"; correct value is "multi-and"
		got := a.searchAttrib("auth,login", ports.SearchOptions{AndMode: true})
		assert.Equal(t, "multi-and", got, "A-02: AND mode should produce 'multi-and', not 'and'")
	})

	t.Run("regex_mode", func(t *testing.T) {
		got := a.searchAttrib("auth.*", ports.SearchOptions{Mode: "regex"})
		assert.Equal(t, "regex", got)
	})
}

// --- searchTarget tests ---

func TestSearchTarget(t *testing.T) {
	a := newTestApp(t)

	t.Run("literal_query", func(t *testing.T) {
		got := a.searchTarget("auth", ports.SearchOptions{})
		assert.Equal(t, "aOa grep auth", got)
	})

	t.Run("regex_query", func(t *testing.T) {
		got := a.searchTarget("auth.*", ports.SearchOptions{Mode: "regex"})
		assert.Equal(t, "aOa egrep auth.*", got)
	})
}

// --- Activity source casing (A-01) ---

func TestActivitySourceCasing(t *testing.T) {
	a := newTestApp(t)

	ev := ports.SessionEvent{
		Kind:      ports.EventToolInvocation,
		TurnID:    "turn-1",
		Timestamp: time.Now(),
		Tool:      &ports.ToolEvent{Name: "Read"},
		File: &ports.FileRef{
			Path:   "/home/user/project/src/foo.go",
			Action: "read",
		},
	}
	a.onSessionEvent(ev)

	entry := lastActivity(a)
	require.NotNil(t, entry, "activity ring should have an entry after Read event")
	assert.Equal(t, "Claude", entry.Source, "A-01: Source should be 'Claude' (capital C), not 'claude'")
}

// --- Activity impact default (A-10 related) ---

func TestActivityImpactDefault(t *testing.T) {
	a := newTestApp(t)

	ev := ports.SessionEvent{
		Kind:      ports.EventToolInvocation,
		TurnID:    "turn-1",
		Timestamp: time.Now(),
		Tool:      &ports.ToolEvent{Name: "Write"},
		File: &ports.FileRef{
			Path:   "/home/user/project/src/bar.go",
			Action: "write",
		},
	}
	a.onSessionEvent(ev)

	entry := lastActivity(a)
	require.NotNil(t, entry, "activity ring should have an entry after Write event")
	assert.Equal(t, "-", entry.Impact, "default impact for tools without savings should be '-'")
}

// --- Read savings with guided attribution (A-07/A-08/A-09) ---

func TestActivityReadSavingsGuided(t *testing.T) {
	a := newTestApp(t)

	// 176KB file → 44000 tokens. Reading 200 lines → ~4000 tokens (200×20).
	// Savings: (44000-4000)/44000 = 90% → well above 50% threshold → "aOa guided"
	ev := ports.SessionEvent{
		Kind:      ports.EventToolInvocation,
		TurnID:    "turn-1",
		Timestamp: time.Now(),
		Tool:      &ports.ToolEvent{Name: "Read"},
		File: &ports.FileRef{
			Path:   "/home/user/project/src/big.go",
			Action: "read",
			Offset: 200,
			Limit:  200,
		},
	}
	a.onSessionEvent(ev)

	entry := lastActivity(a)
	require.NotNil(t, entry, "activity ring should have an entry")

	// A-09: reads with ≥50% savings should get "aOa guided" attrib
	assert.Equal(t, "aOa guided", entry.Attrib,
		"A-09: read with ≥50%% savings should have attrib 'aOa guided'")

	// A-10: impact should show savings in "↓N% (Xk → Yk)" format
	savingsRe := regexp.MustCompile(`↓\d+%\s*\(\d+.*→.*\d+.*\)`)
	assert.Regexp(t, savingsRe, entry.Impact,
		"A-10: impact should match '↓N%% (Xk → Yk)' format, got %q", entry.Impact)
}

// --- Read whole file (no savings) ---

func TestActivityReadNoSavings(t *testing.T) {
	a := newTestApp(t)

	// Read entire file — no offset/limit
	ev := ports.SessionEvent{
		Kind:      ports.EventToolInvocation,
		TurnID:    "turn-1",
		Timestamp: time.Now(),
		Tool:      &ports.ToolEvent{Name: "Read"},
		File: &ports.FileRef{
			Path:   "/home/user/project/src/foo.go",
			Action: "read",
		},
	}
	a.onSessionEvent(ev)

	entry := lastActivity(a)
	require.NotNil(t, entry, "activity ring should have an entry")
	assert.Equal(t, "unguided", entry.Attrib, "whole-file read should have attrib 'unguided'")
	assert.Equal(t, "-", entry.Impact, "whole-file read should have impact '-'")
}

// --- Path stripping (A-05, A-06) ---

func TestActivityPathStripping(t *testing.T) {
	a := newTestApp(t)

	tests := []struct {
		name     string
		ev       ports.SessionEvent
		wantTgt  string
	}{
		{
			name: "Read_strips_project_root",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-read",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Read"},
				File: &ports.FileRef{
					Path:   "/home/user/project/src/foo.go",
					Action: "read",
				},
			},
			wantTgt: "src/foo.go",
		},
		{
			name: "Write_strips_project_root",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-write",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Write"},
				File: &ports.FileRef{
					Path:   "/home/user/project/src/bar.go",
					Action: "write",
				},
			},
			wantTgt: "src/bar.go",
		},
		{
			name: "Edit_strips_project_root",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-edit",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Edit"},
				File: &ports.FileRef{
					Path:   "/home/user/project/src/baz.go",
					Action: "edit",
				},
			},
			wantTgt: "src/baz.go",
		},
		{
			name: "Glob_strips_project_root",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-glob",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Glob"},
				File: &ports.FileRef{
					Path:   "/home/user/project/src/handlers",
					Action: "glob",
				},
			},
			wantTgt: "src/handlers",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset ring for clean assertion
			a.activityRing = [50]ActivityEntry{}
			a.activityHead = 0
			a.activityCount = 0

			a.onSessionEvent(tc.ev)
			entry := lastActivity(a)
			require.NotNil(t, entry)

			assert.Equal(t, tc.wantTgt, entry.Target,
				"A-05/A-06: target should be relative path (no project root prefix)")
			assert.False(t, strings.HasPrefix(entry.Target, a.ProjectRoot),
				"target should not start with ProjectRoot %q", a.ProjectRoot)
		})
	}
}

// --- Bash filtering (A-03, A-04) ---

func TestActivityBashFiltering(t *testing.T) {
	a := newTestApp(t)

	t.Run("aoa_grep_filtered", func(t *testing.T) {
		// A-03: Bash running "./aoa grep foo" should be filtered out
		// (the search is already captured by searchObserver)
		a.activityRing = [50]ActivityEntry{}
		a.activityHead = 0
		a.activityCount = 0

		ev := ports.SessionEvent{
			Kind:      ports.EventToolInvocation,
			TurnID:    "turn-bash-aoa",
			Timestamp: time.Now(),
			Tool:      &ports.ToolEvent{Name: "Bash", Command: "./aoa grep foo"},
		}
		a.onSessionEvent(ev)

		assert.Equal(t, 0, a.activityCount,
			"A-03: Bash './aoa grep' should be filtered from activity ring")
	})

	t.Run("git_status_no_file_filtered", func(t *testing.T) {
		// A-04: Bash without file context should be filtered
		a.activityRing = [50]ActivityEntry{}
		a.activityHead = 0
		a.activityCount = 0

		ev := ports.SessionEvent{
			Kind:      ports.EventToolInvocation,
			TurnID:    "turn-bash-git",
			Timestamp: time.Now(),
			Tool:      &ports.ToolEvent{Name: "Bash", Command: "git status"},
		}
		a.onSessionEvent(ev)

		assert.Equal(t, 0, a.activityCount,
			"A-04: Bash without ev.File should be filtered from activity ring")
	})

	t.Run("bash_with_file_kept", func(t *testing.T) {
		// Bash with a File reference should be kept in the activity ring
		a.activityRing = [50]ActivityEntry{}
		a.activityHead = 0
		a.activityCount = 0

		ev := ports.SessionEvent{
			Kind:      ports.EventToolInvocation,
			TurnID:    "turn-bash-file",
			Timestamp: time.Now(),
			Tool:      &ports.ToolEvent{Name: "Bash", Command: "cat /home/user/project/src/foo.go"},
			File: &ports.FileRef{
				Path:   "/home/user/project/src/foo.go",
				Action: "read",
			},
		}
		a.onSessionEvent(ev)

		assert.Equal(t, 1, a.activityCount,
			"Bash with ev.File set should remain in activity ring")
	})
}

// --- Full rubric (table-driven integration test) ---

func TestActivityRubric(t *testing.T) {
	a := newTestApp(t)

	// ---------------------------------------------------------------
	// Search rows (1-4): test searchAttrib/searchTarget directly,
	// since the full searchObserver needs a working search engine.
	// ---------------------------------------------------------------
	searchTests := []struct {
		name       string
		query      string
		opts       ports.SearchOptions
		wantAttrib string
		wantTarget string
	}{
		{
			name:       "search_single_token",
			query:      "auth",
			opts:       ports.SearchOptions{},
			wantAttrib: "indexed",
			wantTarget: "aOa grep auth",
		},
		{
			name:       "search_multi_or",
			query:      "auth login",
			opts:       ports.SearchOptions{},
			wantAttrib: "multi-or",
			wantTarget: "aOa grep auth login",
		},
		{
			name:       "search_and_mode",
			query:      "auth,login",
			opts:       ports.SearchOptions{AndMode: true},
			wantAttrib: "multi-and",
			wantTarget: "aOa grep -a auth,login",
		},
		{
			name:       "search_regex",
			query:      "auth.*",
			opts:       ports.SearchOptions{Mode: "regex"},
			wantAttrib: "regex",
			wantTarget: "aOa egrep auth.*",
		},
	}
	for _, tc := range searchTests {
		t.Run(tc.name, func(t *testing.T) {
			gotAttrib := a.searchAttrib(tc.query, tc.opts)
			gotTarget := a.searchTarget(tc.query, tc.opts)
			assert.Equal(t, tc.wantAttrib, gotAttrib, "attrib mismatch")
			assert.Equal(t, tc.wantTarget, gotTarget, "target mismatch")
		})
	}

	// ---------------------------------------------------------------
	// Tool rows (5-13): fire SessionEvents via onSessionEvent and
	// assert the resulting ActivityEntry fields.
	// ---------------------------------------------------------------
	type toolCase struct {
		name       string
		ev         ports.SessionEvent
		filtered   bool   // if true, expect no activity entry
		wantAction string // expected Action
		wantSource string // expected Source
		wantAttrib string // expected Attrib (exact or "re:" prefix for regex)
		wantImpact string // expected Impact (exact or "re:" prefix for regex)
		wantTarget string // expected Target (exact or "re:" prefix for regex)
	}

	toolTests := []toolCase{
		{
			// Row 5: Read, guided (≥50% savings) — big file, small range
			name: "read_guided_savings",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-5",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Read"},
				File: &ports.FileRef{
					Path:   "/home/user/project/src/big.go",
					Action: "read",
					Offset: 200,
					Limit:  200,
				},
			},
			wantAction: "Read",
			wantSource: "Claude",
			wantAttrib: "aOa guided",
			wantImpact: "re:↓\\d+%",
			wantTarget: "src/big.go:200-400",
		},
		{
			// Row 6: Read, not guided (<50% savings) — small file, big range
			name: "read_not_guided",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-6",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Read"},
				File: &ports.FileRef{
					Path:   "/home/user/project/src/small.go",
					Action: "read",
					Offset: 10,
					Limit:  70,
				},
			},
			wantAction: "Read",
			wantSource: "Claude",
			wantAttrib: "-",
			wantImpact: "re:↓\\d+%",
			wantTarget: "src/small.go:10-80",
		},
		{
			// Row 7: Read, whole file (no offset/limit) — now classified as unguided
			name: "read_whole_file",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-7",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Read"},
				File: &ports.FileRef{
					Path:   "/home/user/project/src/foo.go",
					Action: "read",
				},
			},
			wantAction: "Read",
			wantSource: "Claude",
			wantAttrib: "unguided",
			wantImpact: "-",
			wantTarget: "src/foo.go",
		},
		{
			// Row 8: Write — L0.8: attrib = "productive"
			name: "write",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-8",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Write"},
				File:      &ports.FileRef{Path: "/home/user/project/src/bar.go", Action: "write"},
			},
			wantAction: "Write",
			wantSource: "Claude",
			wantAttrib: "productive",
			wantImpact: "-",
			wantTarget: "src/bar.go",
		},
		{
			// Row 9: Edit — L0.8: attrib = "productive"
			name: "edit",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-9",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Edit"},
				File:      &ports.FileRef{Path: "/home/user/project/src/baz.go", Action: "edit"},
			},
			wantAction: "Edit",
			wantSource: "Claude",
			wantAttrib: "productive",
			wantImpact: "-",
			wantTarget: "src/baz.go",
		},
		{
			// Row 10: Grep — L0.10: impact = token cost
			name: "grep",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-10",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Grep", Pattern: "auth.*handler"},
			},
			wantAction: "Grep",
			wantSource: "Claude",
			wantAttrib: "unguided",
			wantImpact: "re:~.*tokens",
			wantTarget: "auth.*handler",
		},
		{
			// Row 11: Glob — L0.9: attrib = "unguided", impact = token cost
			name: "glob",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-11",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Glob"},
				File:      &ports.FileRef{Path: "/home/user/project/src/handlers", Action: "glob"},
			},
			wantAction: "Glob",
			wantSource: "Claude",
			wantAttrib: "unguided",
			wantImpact: "re:~.*tokens",
			wantTarget: "src/handlers",
		},
		{
			// Row 12: Bash with ./aoa grep — should be FILTERED (A-03)
			name: "bash_aoa_grep_filtered",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-12",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Bash", Command: "./aoa grep foo"},
			},
			filtered: true,
		},
		{
			// Row 13: Bash with no file context — should be FILTERED (A-04)
			name: "bash_no_file_filtered",
			ev: ports.SessionEvent{
				Kind:      ports.EventToolInvocation,
				TurnID:    "turn-13",
				Timestamp: time.Now(),
				Tool:      &ports.ToolEvent{Name: "Bash", Command: "git status"},
			},
			filtered: true,
		},
	}

	for _, tc := range toolTests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset ring
			a.activityRing = [50]ActivityEntry{}
			a.activityHead = 0
			a.activityCount = 0

			a.onSessionEvent(tc.ev)

			if tc.filtered {
				assert.Equal(t, 0, a.activityCount,
					"expected event to be filtered from activity ring")
				return
			}

			entry := lastActivity(a)
			require.NotNil(t, entry, "activity ring should have an entry")

			assert.Equal(t, tc.wantAction, entry.Action, "Action mismatch")
			assert.Equal(t, tc.wantSource, entry.Source, "Source mismatch")
			assertFieldMatch(t, "Attrib", tc.wantAttrib, entry.Attrib)
			assertFieldMatch(t, "Impact", tc.wantImpact, entry.Impact)
			assertFieldMatch(t, "Target", tc.wantTarget, entry.Target)
		})
	}
}

// assertFieldMatch checks an ActivityEntry field against an expected value.
// If expected starts with "re:", the remainder is used as a regexp.
// Otherwise an exact match is required.
func assertFieldMatch(t *testing.T, field, expected, actual string) {
	t.Helper()
	if strings.HasPrefix(expected, "re:") {
		pattern := expected[3:]
		matched, err := regexp.MatchString(pattern, actual)
		if err != nil {
			t.Errorf("%s: invalid regex %q: %v", field, pattern, err)
			return
		}
		if !matched {
			t.Errorf("%s: %q does not match pattern %q", field, actual, pattern)
		}
	} else {
		if actual != expected {
			t.Errorf("%s: got %q, want %q", field, actual, expected)
		}
	}
}

// --- Verify readSavings directly ---

func TestReadSavings(t *testing.T) {
	a := newTestApp(t)

	t.Run("big_file_small_read", func(t *testing.T) {
		// 176KB → 44000 tokens. Reading 200 lines → 4000 tokens. Savings: 90%
		got := a.readSavings("/home/user/project/src/big.go", 200)
		assert.NotEmpty(t, got.display, "should compute savings for large file with small read")
		assert.Equal(t, 90, got.pct)
		assert.Equal(t, int64(44000), got.fileTokens)
		assert.Equal(t, int64(4000), got.readTokens)
		assert.True(t, strings.HasPrefix(got.display, "↓"),
			"savings display should start with ↓, got %q", got.display)
		assert.Contains(t, got.display, "44.0k")
		assert.Contains(t, got.display, "4.0k")
	})

	t.Run("small_file_moderate_read", func(t *testing.T) {
		// 10KB → 2500 tokens. Reading 70 lines → 1400 tokens. Savings: 44%
		got := a.readSavings("/home/user/project/src/small.go", 70)
		assert.NotEmpty(t, got.display, "should compute savings")
		assert.Equal(t, 44, got.pct)
		assert.True(t, got.pct < 50, "should be below 50%% guided threshold")
	})

	t.Run("whole_file_read", func(t *testing.T) {
		got := a.readSavings("/home/user/project/src/foo.go", 0)
		assert.Empty(t, got.display, "limit=0 should return empty savings")
	})

	t.Run("unknown_file", func(t *testing.T) {
		got := a.readSavings("/nonexistent/file.go", 100)
		assert.Empty(t, got.display, "unknown file should return empty savings")
	})
}

// --- Verify activity ring buffer mechanics ---

func TestActivityRingBuffer(t *testing.T) {
	a := newTestApp(t)

	// Push 55 entries — exceeds ring size of 50
	for i := 0; i < 55; i++ {
		a.pushActivity(ActivityEntry{
			Action: fmt.Sprintf("action-%d", i),
			Target: fmt.Sprintf("target-%d", i),
		})
	}

	assert.Equal(t, 50, a.activityCount, "count should cap at 50")

	// Most recent entry should be the last one pushed
	entry := lastActivity(a)
	require.NotNil(t, entry)
	assert.Equal(t, "action-54", entry.Action)
	assert.Equal(t, "target-54", entry.Target)
}

// --- L0.12: Target capture preserves full flag syntax ---

func TestSearchTargetPreservesFlags(t *testing.T) {
	a := newTestApp(t)

	tests := []struct {
		name       string
		query      string
		opts       ports.SearchOptions
		wantTarget string
	}{
		{
			name:       "all_flags",
			query:      "auth,login",
			opts:       ports.SearchOptions{AndMode: true, WordBoundary: true, CountOnly: true, IncludeGlob: "*.go", ExcludeGlob: "*_test.go"},
			wantTarget: "aOa grep -a -w -c --include *.go --exclude *_test.go auth,login",
		},
		{
			name:       "regex_with_word_boundary",
			query:      "func.*Handler",
			opts:       ports.SearchOptions{Mode: "regex", WordBoundary: true},
			wantTarget: "aOa egrep -w func.*Handler",
		},
		{
			name:       "simple_query_no_flags",
			query:      "auth",
			opts:       ports.SearchOptions{},
			wantTarget: "aOa grep auth",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := a.searchTarget(tc.query, tc.opts)
			assert.Equal(t, tc.wantTarget, got,
				"L0.12: searchTarget must preserve full flag syntax verbatim")
		})
	}
}

// --- L0.8: Write/Edit productive attrib ---

func TestActivityWriteEditProductive(t *testing.T) {
	a := newTestApp(t)

	for _, toolName := range []string{"Write", "Edit"} {
		a.activityRing = [50]ActivityEntry{}
		a.activityHead = 0
		a.activityCount = 0

		ev := ports.SessionEvent{
			Kind:      ports.EventToolInvocation,
			TurnID:    fmt.Sprintf("turn-%s", toolName),
			Timestamp: time.Now(),
			Tool:      &ports.ToolEvent{Name: toolName},
			File:      &ports.FileRef{Path: "/home/user/project/src/foo.go", Action: "write"},
		}
		a.onSessionEvent(ev)

		entry := lastActivity(a)
		require.NotNil(t, entry)
		assert.Equal(t, "productive", entry.Attrib,
			"L0.8: %s should have attrib 'productive'", toolName)
	}
}

// --- L0.9: Glob unguided + cost ---

func TestActivityGlobUnguided(t *testing.T) {
	a := newTestApp(t)
	a.activityRing = [50]ActivityEntry{}
	a.activityHead = 0
	a.activityCount = 0

	ev := ports.SessionEvent{
		Kind:      ports.EventToolInvocation,
		TurnID:    "turn-glob",
		Timestamp: time.Now(),
		Tool:      &ports.ToolEvent{Name: "Glob"},
		File:      &ports.FileRef{Path: "/home/user/project/src/handlers", Action: "glob"},
	}
	a.onSessionEvent(ev)

	entry := lastActivity(a)
	require.NotNil(t, entry)
	assert.Equal(t, "unguided", entry.Attrib, "L0.9: Glob should have attrib 'unguided'")
	// The index has auth.go in src/handlers/ (500 bytes → 125 tokens)
	assert.Contains(t, entry.Impact, "tokens", "L0.9: Glob impact should contain token estimate")
}

// --- L0.10: Grep token cost ---

func TestActivityGrepTokenCost(t *testing.T) {
	a := newTestApp(t)
	a.activityRing = [50]ActivityEntry{}
	a.activityHead = 0
	a.activityCount = 0

	ev := ports.SessionEvent{
		Kind:      ports.EventToolInvocation,
		TurnID:    "turn-grep",
		Timestamp: time.Now(),
		Tool:      &ports.ToolEvent{Name: "Grep", Pattern: "auth.*handler"},
	}
	a.onSessionEvent(ev)

	entry := lastActivity(a)
	require.NotNil(t, entry)
	assert.Equal(t, "unguided", entry.Attrib)
	assert.Contains(t, entry.Impact, "tokens", "L0.10: Grep impact should contain token estimate")
}

// --- L0.11: Learn activity from file read ---

func TestActivityLearnFromFileRead(t *testing.T) {
	a := newTestApp(t)
	a.activityRing = [50]ActivityEntry{}
	a.activityHead = 0
	a.activityCount = 0

	// Send a range-gated read that triggers learning
	ev := ports.SessionEvent{
		Kind:      ports.EventToolInvocation,
		TurnID:    "turn-learn",
		Timestamp: time.Now(),
		Tool:      &ports.ToolEvent{Name: "Read"},
		File: &ports.FileRef{
			Path:   "/home/user/project/src/big.go",
			Action: "read",
			Offset: 100,
			Limit:  50,
		},
	}
	a.onSessionEvent(ev)

	// Should have both a Learn entry and a Read entry
	found := false
	for i := 0; i < a.activityCount; i++ {
		idx := (a.activityHead - 1 - i + 50) % 50
		e := a.activityRing[idx]
		if e.Action == "Learn" && e.Source == "aOa" {
			assert.Contains(t, e.Impact, "+1 file:")
			found = true
			break
		}
	}
	assert.True(t, found, "L0.11: expected a Learn activity entry from range-gated read")
}
