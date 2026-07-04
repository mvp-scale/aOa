// Package web — dashboard Architecture tab integration tests.
//
// These tests assert that index.html ships the Architecture tab correctly:
// the nav button is hidden by default (so C4-off and lean builds degrade
// cleanly) and the tab pane contains the embedded viewer iframe.
//
// Both lean and standard builds share the same static/index.html, so these
// tests run unconditionally in both configurations.
package web

import (
	"os"
	"strings"
	"testing"
)

// TestArchTabInDashboard verifies that index.html contains the Architecture
// tab button and pane with the correct structure:
//
//   - nav button is present with id="navTabArch" and style="display:none"
//     (hidden by default; app.js reveals it only when /api/arch/manifest 200s)
//   - tab pane id="tab-architecture" is present
//   - iframe id="archFrame" is present (src is set lazily by app.js)
//   - "Architecture Views" card header is present
//
// This test is build-tag agnostic: the HTML ships identically in lean and
// standard builds. The hidden-by-default button + absent route = honest
// degradation in lean builds without any server templating.
func TestArchTabInDashboard(t *testing.T) {
	htmlBytes, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("cannot read static/index.html: %v", err)
	}
	html := string(htmlBytes)

	// Nav button: present with correct id and data-tab attribute.
	if !strings.Contains(html, `id="navTabArch"`) {
		t.Error("index.html missing architecture nav button (id=navTabArch)")
	}
	if !strings.Contains(html, `data-tab="architecture"`) {
		t.Error("index.html missing data-tab=\"architecture\" on nav button")
	}

	// Nav button must ship hidden — display:none means a 404 manifest probe
	// (lean build, C4 disabled) leaves the button permanently invisible.
	if !strings.Contains(html, `style="display:none"`) {
		t.Error(`architecture nav button must have style="display:none" (hidden by default for C4/lean honesty)`)
	}

	// Tab pane must be present.
	if !strings.Contains(html, `id="tab-architecture"`) {
		t.Error("index.html missing architecture tab pane (id=tab-architecture)")
	}

	// Iframe element: present (src is set lazily by app.js on first activation).
	if !strings.Contains(html, `id="archFrame"`) {
		t.Error("index.html missing arch viewer iframe (id=archFrame)")
	}
	// The iframe must NOT have a src attribute in the HTML — lazy-load is mandatory
	// so the React/xyflow bundle is not fetched on dashboard boot.
	if strings.Contains(html, `archFrame" src=`) || strings.Contains(html, `archFrame"  src=`) {
		t.Error("archFrame iframe must not have a src attribute in HTML; src is set lazily by app.js")
	}

	// Card header label.
	if !strings.Contains(html, "Architecture Views") {
		t.Error("index.html missing 'Architecture Views' card header in architecture tab")
	}

	// Hero metric cell ids must be present (updated by renderArchitecture() from manifest data).
	for _, id := range []string{"hm-arch-0", "hm-arch-1", "hm-arch-2", "hm-arch-3"} {
		if !strings.Contains(html, `id="`+id+`"`) {
			t.Errorf("index.html missing hero metric cell id=%q in architecture tab", id)
		}
	}

	t.Log("arch tab assertions PASS: nav button hidden-by-default, tab pane + iframe present, hero cells wired")
}

// TestArchTabNavPosition verifies that the Architecture tab button appears
// between Debrief and Arsenal in the nav (correct visual order).
func TestArchTabNavPosition(t *testing.T) {
	htmlBytes, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("cannot read static/index.html: %v", err)
	}
	html := string(htmlBytes)

	debriefPos := strings.Index(html, `data-tab="debrief"`)
	archPos := strings.Index(html, `id="navTabArch"`)
	arsenalPos := strings.Index(html, `data-tab="arsenal"`)

	if debriefPos < 0 {
		t.Fatal("data-tab=debrief not found in index.html")
	}
	if archPos < 0 {
		t.Fatal("id=navTabArch not found in index.html")
	}
	if arsenalPos < 0 {
		t.Fatal("data-tab=arsenal not found in index.html")
	}

	if !(debriefPos < archPos && archPos < arsenalPos) {
		t.Errorf("Architecture tab must appear between Debrief and Arsenal in the nav: debrief@%d arch@%d arsenal@%d",
			debriefPos, archPos, arsenalPos)
	}

	t.Log("arch tab nav position PASS: Debrief < Architecture < Arsenal")
}
