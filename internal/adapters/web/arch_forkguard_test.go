// Package web — fork-guard for the arch viewer (T16).
// Two halves:
//   (a) viewer fork-guard: ensures playbook/mockups/architecture-c4.html embeds JS derived
//       from internal/adapters/web/static/arch/viewer.js, so the two can never silently drift.
//   (b) bundle budget: vendored bundle ≤2.2 MB raw / ≤650 KB gz; zero CDN/esm.sh imports.
package web

import (
	"bytes"
	"compress/gzip"
	"io"
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

// TestT16BundleBudget asserts the vendor bundle budget and no-CDN-import constraints (T16 second half).
//
// Pass criteria (from kickoff §6 / board T16):
//   - static/arch/vendor/bundle.js exists
//   - raw size ≤ 2.2 MB (2,306,867 bytes)
//   - gzip size ≤ 650 KB (665,600 bytes)
//   - static/arch/vendor/xyflow.css exists
//   - viewer.js contains zero esm.sh or CDN URLs
//   - index.html contains zero esm.sh or CDN URLs for JS/CSS
func TestT16BundleBudget(t *testing.T) {
	root := repoRoot(t)
	archDir := filepath.Join(root, "internal", "adapters", "web", "static", "arch")

	// ── vendor/bundle.js.gz exists and within size budget ────────────────────
	// The bundle is stored pre-compressed (gzip) to stay under the repo 1 MB
	// per-file limit. The arch handler serves it with Content-Encoding: gzip.
	bundlePath := filepath.Join(archDir, "vendor", "bundle.js.gz")
	bundleGzData, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatalf("vendor/bundle.js.gz missing — run the vendor step:\n"+
			"  cd /tmp/vendor_build && npm install react@18.3.1 react-dom@18.3.1 @xyflow/react@12.3.5 elkjs@0.11.1 htm@3.1.1 esbuild\n"+
			"  npx esbuild entry.js --bundle --format=esm --platform=browser --outfile=bundle.js --minify\n"+
			"  gzip -9 -c bundle.js > static/arch/vendor/bundle.js.gz\n"+
			"  path: %s\n  err: %v", bundlePath, err)
	}

	// The stored file IS the gzip — check its size (≤ 650 KB)
	const maxGz = 665_600 // 650 KB
	if len(bundleGzData) > maxGz {
		t.Errorf("bundle.js.gz size %d bytes exceeds %d bytes (650 KB budget)", len(bundleGzData), maxGz)
	} else {
		t.Logf("bundle.js.gz: %d bytes (%.0f KB, budget %.0f KB)", len(bundleGzData),
			float64(len(bundleGzData))/1024, float64(maxGz)/1024)
	}

	// Decompress and check raw size (≤ 2.2 MB)
	gr, err := gzip.NewReader(bytes.NewReader(bundleGzData))
	if err != nil {
		t.Fatalf("open gzip reader for bundle.js.gz: %v", err)
	}
	rawData, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("decompress bundle.js.gz: %v", err)
	}
	_ = gr.Close()

	const maxRaw = 2_306_867 // 2.2 MB
	if len(rawData) > maxRaw {
		t.Errorf("bundle.js raw (decompressed) size %d bytes exceeds %d bytes (2.2 MB budget)", len(rawData), maxRaw)
	} else {
		t.Logf("bundle.js raw (decompressed): %d bytes (%.1f MB, budget %.1f MB)", len(rawData),
			float64(len(rawData))/1e6, float64(maxRaw)/1e6)
	}

	// ── vendor/xyflow.css exists ──────────────────────────────────────────────
	cssPath := filepath.Join(archDir, "vendor", "xyflow.css")
	if _, err := os.Stat(cssPath); err != nil {
		t.Errorf("vendor/xyflow.css missing (expected at %s): %v", cssPath, err)
	}

	// ── viewer.js: zero esm.sh / CDN imports ─────────────────────────────────
	viewerJSPath := filepath.Join(archDir, "viewer.js")
	viewerData, err := os.ReadFile(viewerJSPath)
	if err != nil {
		t.Fatalf("cannot read viewer.js: %v", err)
	}
	viewerJS := string(viewerData)

	cdnPatterns := []string{"esm.sh", "cdn.skypack", "unpkg.com", "jsdelivr.net"}
	for _, pat := range cdnPatterns {
		if strings.Contains(viewerJS, pat) {
			t.Errorf("viewer.js still contains CDN import %q — replace with ./vendor/bundle.js", pat)
		}
	}

	// ── index.html: zero esm.sh / CDN links ──────────────────────────────────
	indexPath := filepath.Join(archDir, "index.html")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("cannot read index.html: %v", err)
	}
	indexHTML := string(indexData)
	for _, pat := range cdnPatterns {
		if strings.Contains(indexHTML, pat) {
			t.Errorf("index.html still contains CDN reference %q — replace with vendor/xyflow.css", pat)
		}
	}

	if !t.Failed() {
		t.Logf("T16 bundle budget PASS: no CDN imports, bundle within budget")
	}
}

// TestStandardsCopiesInSync guards the dual copy of view-standards.json:
// playbook/standards/ is the source of truth; static/arch/ is the embedded
// serving copy. They must stay byte-identical (L19.19 review nit N2).
func TestStandardsCopiesInSync(t *testing.T) {
	served, err := os.ReadFile("static/arch/view-standards.json")
	if err != nil {
		t.Fatalf("read served copy: %v", err)
	}
	canonical, err := os.ReadFile("../../../playbook/standards/view-standards.json")
	if err != nil {
		t.Fatalf("read canonical copy: %v", err)
	}
	if !bytes.Equal(served, canonical) {
		t.Fatal("static/arch/view-standards.json has drifted from playbook/standards/view-standards.json — copy the canonical file over the served one")
	}
}
