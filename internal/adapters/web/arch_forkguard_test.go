// Package web — fork-guard for the arch viewer.
// T16 precursor: ensures playbook/mockups/architecture-c4.html embeds JS derived
// from internal/adapters/web/static/arch/viewer.js, so the two can never silently drift.
package web

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot walks up from this test file's directory to find the repo root
// (identified by go.mod presence).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}

// TestViewerForkGuard asserts that the architecture mockup at
// playbook/mockups/architecture-c4.html was generated from
// internal/adapters/web/static/arch/viewer.js (T16 first half).
//
// Strategy: split viewer.js on "__VIEW_INTENT__" — the generator replaces
// this placeholder with intent JSON before inlining. Verify that the mockup
// contains both the prefix (all JS before the placeholder) and the suffix
// (all JS after it). If either is missing, the sources have diverged.
func TestViewerForkGuard(t *testing.T) {
	root := repoRoot(t)

	viewerJSPath := filepath.Join(root, "internal", "adapters", "web", "static", "arch", "viewer.js")
	mockupPath := filepath.Join(root, "playbook", "mockups", "architecture-c4.html")

	// Read viewer.js
	viewerJSBytes, err := os.ReadFile(viewerJSPath)
	if err != nil {
		t.Fatalf("cannot read viewer.js: %v\n  path: %s", err, viewerJSPath)
	}
	viewerJS := string(viewerJSBytes)

	// Verify viewer.js contains the placeholder exactly once
	const placeholder = "__VIEW_INTENT__"
	count := strings.Count(viewerJS, placeholder)
	if count != 1 {
		t.Fatalf("viewer.js must contain %q exactly once, got %d occurrences", placeholder, count)
	}

	// Split on the placeholder to get prefix and suffix
	parts := strings.SplitN(viewerJS, placeholder, 2)
	prefix := parts[0]
	suffix := parts[1]

	// Read the mockup
	mockupBytes, err := os.ReadFile(mockupPath)
	if err != nil {
		t.Skipf("mockup not yet generated (run the generator once to create it): %v", err)
	}
	mockup := string(mockupBytes)

	// The mockup must contain <script type="module"> wrapping the inlined JS
	if !strings.Contains(mockup, `<script type="module">`) {
		t.Fatal("mockup does not contain <script type=\"module\"> — unexpected format")
	}

	// The prefix (JS before __VIEW_INTENT__) must appear verbatim in the mockup
	if !strings.Contains(mockup, prefix) {
		// Find where they diverge for a useful error message
		mockupIdx := strings.Index(mockup, `<script type="module">`)
		if mockupIdx >= 0 {
			inlined := mockup[mockupIdx+len(`<script type="module">`):]
			// Find first divergence
			minLen := len(prefix)
			if len(inlined) < minLen {
				minLen = len(inlined)
			}
			divergeAt := minLen
			for i := 0; i < minLen; i++ {
				if prefix[i] != inlined[i] {
					divergeAt = i
					break
				}
			}
			t.Fatalf("mockup JS diverges from viewer.js at byte %d\n"+
				"viewer.js[%d:%d]: %q\n"+
				"mockup  [%d:%d]: %q\n\n"+
				"Fix: regenerate the mockup with the playbook generator.",
				divergeAt,
				fgMax(0, divergeAt-40), fgMin(len(prefix), divergeAt+40), safeSlice(prefix, divergeAt-40, divergeAt+40),
				fgMax(0, divergeAt-40), fgMin(len(inlined), divergeAt+40), safeSlice(inlined, divergeAt-40, divergeAt+40),
			)
		}
		t.Fatalf("mockup does not contain the viewer.js JS prefix — sources have diverged.\n"+
			"Fix: regenerate the mockup with the playbook generator.\n"+
			"  viewer.js: %s\n  mockup: %s", viewerJSPath, mockupPath)
	}

	// The suffix (JS after __VIEW_INTENT__) must also appear verbatim in the mockup
	if !strings.Contains(mockup, suffix) {
		t.Fatalf("mockup does not contain the viewer.js JS suffix (post-__VIEW_INTENT__ section) — sources have diverged.\n"+
			"Fix: regenerate the mockup with the playbook generator.\n"+
			"  viewer.js: %s\n  mockup: %s", viewerJSPath, mockupPath)
	}

	t.Logf("Fork-guard PASS: mockup embeds JS derived from viewer.js (%d bytes, placeholder at byte %d)",
		len(viewerJS), len(prefix))
}

func safeSlice(s string, lo, hi int) string {
	if lo < 0 {
		lo = 0
	}
	if hi > len(s) {
		hi = len(s)
	}
	if lo >= hi {
		return ""
	}
	return s[lo:hi]
}

func fgMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fgMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
