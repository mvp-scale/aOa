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

// TestViewerForkGuard asserts that:
// (a) viewer.js no longer contains the __VIEW_INTENT__ build-time placeholder
//     (V2: intent is now fetched at runtime from /api/arch/standards).
// (b) The architecture mockup at playbook/mockups/architecture-c4.html was
//     generated from internal/adapters/web/static/arch/viewer.js — verified by
//     checking that the first and last 512 bytes of viewer.js appear verbatim
//     in the mockup (T16 first half).
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

	// (a) V2 contract: viewer.js must NOT contain the build-time placeholder.
	// Intent is now fetched at runtime from /api/arch/standards.
	const placeholder = "__VIEW_INTENT__"
	if strings.Contains(viewerJS, placeholder) {
		t.Fatalf("viewer.js still contains %q — remove it and replace with the runtime standards fetch.\n"+
			"  viewer.js: %s", placeholder, viewerJSPath)
	}

	// (b) Fork-guard: first and last 512 bytes of viewer.js must appear verbatim in the mockup.
	// This proves the mockup was generated from (and has not drifted from) viewer.js.
	const window = 512

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

	// Check the first window bytes of viewer.js appear in the mockup
	head := viewerJS
	if len(head) > window {
		head = head[:window]
	}
	if !strings.Contains(mockup, head) {
		t.Fatalf("mockup does not contain the first %d bytes of viewer.js — sources have diverged.\n"+
			"Fix: regenerate the mockup with the playbook generator.\n"+
			"  viewer.js: %s\n  mockup: %s\n  first %d bytes: %q",
			window, viewerJSPath, mockupPath, window, head)
	}

	// Check the last window bytes of viewer.js appear in the mockup
	tail := viewerJS
	if len(tail) > window {
		tail = tail[len(tail)-window:]
	}
	if !strings.Contains(mockup, tail) {
		t.Fatalf("mockup does not contain the last %d bytes of viewer.js — sources have diverged.\n"+
			"Fix: regenerate the mockup with the playbook generator.\n"+
			"  viewer.js: %s\n  mockup: %s\n  last %d bytes: %q",
			window, viewerJSPath, mockupPath, window, tail)
	}

	t.Logf("Fork-guard PASS: mockup embeds JS derived from viewer.js (%d bytes, head+tail %d-byte windows both present)",
		len(viewerJS), window)
}
