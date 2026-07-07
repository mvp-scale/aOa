// Package web — dashboard knowledge-zone integration tests.
//
// These tests assert that index.html ships the Terrain + Blueprints tabs
// correctly: both nav buttons are hidden by default (so C4-off and lean builds
// degrade cleanly to a five-tab nav), the spine divider is hidden by default,
// and each pane has the expected structure.
//
// Both lean and standard builds share the same static/index.html, so these
// tests run unconditionally in both configurations.
package web

import (
	"os"
	"strings"
	"testing"
)

// TestKnowledgeTabsInDashboard verifies that index.html contains the Terrain and
// Blueprints tab buttons and panes with the correct structure:
//
//   - Terrain nav button: id="navTabTerrain", data-tab="terrain", hidden by default
//   - Blueprints nav button: id="navTabBlueprints", data-tab="blueprints", hidden by default
//   - Nav spine: id="navSpine", class="nav-spine", hidden by default
//   - tab-terrain pane: present, contains canvas id="terrainCanvas" and status strip
//   - tab-blueprints pane: present, contains iframe id="archFrame", NO hero-row
func TestKnowledgeTabsInDashboard(t *testing.T) {
	htmlBytes, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("cannot read static/index.html: %v", err)
	}
	html := string(htmlBytes)

	// ── Terrain button ──
	if !strings.Contains(html, `id="navTabTerrain"`) {
		t.Error("index.html missing Terrain nav button (id=navTabTerrain)")
	}
	if !strings.Contains(html, `data-tab="terrain"`) {
		t.Error(`index.html missing data-tab="terrain" on Terrain nav button`)
	}

	// ── Blueprints button ──
	if !strings.Contains(html, `id="navTabBlueprints"`) {
		t.Error("index.html missing Blueprints nav button (id=navTabBlueprints)")
	}
	if !strings.Contains(html, `data-tab="blueprints"`) {
		t.Error(`index.html missing data-tab="blueprints" on Blueprints nav button`)
	}

	// ── Nav spine ──
	if !strings.Contains(html, `id="navSpine"`) {
		t.Error("index.html missing nav spine (id=navSpine)")
	}
	if !strings.Contains(html, `class="nav-spine"`) {
		t.Error(`index.html missing class="nav-spine" on spine element`)
	}

	// All three knowledge-zone elements must ship hidden (display:none).
	// Count occurrences of style="display:none" to ensure at least three are present.
	count := strings.Count(html, `style="display:none"`)
	if count < 3 {
		t.Errorf("expected at least 3 style=\"display:none\" elements (spine + terrain + blueprints); got %d", count)
	}

	// ── tab-terrain pane ──
	if !strings.Contains(html, `id="tab-terrain"`) {
		t.Error("index.html missing Terrain tab pane (id=tab-terrain)")
	}
	if !strings.Contains(html, `id="terrainCanvas"`) {
		t.Error("index.html missing terrain canvas (id=terrainCanvas)")
	}
	if !strings.Contains(html, `id="terrainStatusStrip"`) {
		t.Error("index.html missing terrain status strip (id=terrainStatusStrip)")
	}

	// ── tab-blueprints pane ──
	if !strings.Contains(html, `id="tab-blueprints"`) {
		t.Error("index.html missing Blueprints tab pane (id=tab-blueprints)")
	}
	if !strings.Contains(html, `id="archFrame"`) {
		t.Error("index.html missing arch viewer iframe (id=archFrame)")
	}
	// Iframe must NOT have a src attribute in the HTML — lazy-load is mandatory.
	if strings.Contains(html, `archFrame" src=`) || strings.Contains(html, `archFrame"  src=`) {
		t.Error("archFrame iframe must not have a src attribute in HTML; src is set lazily by app.js")
	}

	// Blueprints pane must NOT contain a hero-row (no hero on knowledge surfaces).
	bpStart := strings.Index(html, `id="tab-blueprints"`)
	if bpStart < 0 {
		t.Fatal("tab-blueprints pane not found — cannot check for hero-row absence")
	}
	// Find end of blueprints pane (next tab-content or end of main)
	bpEnd := strings.Index(html[bpStart:], `id="tab-`)
	if bpEnd < 0 {
		bpEnd = len(html) - bpStart
	}
	bpHtml := html[bpStart : bpStart+bpEnd]
	if strings.Contains(bpHtml, "hero-row") {
		t.Error("tab-blueprints must not contain a hero-row (no hero on knowledge surfaces)")
	}
	if strings.Contains(bpHtml, "Architecture Views") {
		t.Error("tab-blueprints must not contain 'Architecture Views' card chrome")
	}

	// Old architecture tab must be gone.
	if strings.Contains(html, `id="tab-architecture"`) {
		t.Error("index.html still has old id=tab-architecture pane; should have been renamed to tab-blueprints")
	}
	if strings.Contains(html, `id="navTabArch"`) {
		t.Error("index.html still has old id=navTabArch button; should have been replaced")
	}

	t.Log("knowledge-zone assertions PASS: spine + terrain + blueprints present, hidden-by-default, correct structure")
}

// TestKnowledgeTabNavPosition verifies the nav order:
// Debrief < Arsenal < navSpine < navTabTerrain < navTabBlueprints
func TestKnowledgeTabNavPosition(t *testing.T) {
	htmlBytes, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("cannot read static/index.html: %v", err)
	}
	html := string(htmlBytes)

	debriefPos := strings.Index(html, `data-tab="debrief"`)
	arsenalPos := strings.Index(html, `data-tab="arsenal"`)
	spinePos := strings.Index(html, `id="navSpine"`)
	terrainPos := strings.Index(html, `id="navTabTerrain"`)
	blueprintsPos := strings.Index(html, `id="navTabBlueprints"`)

	for name, pos := range map[string]int{
		"debrief":    debriefPos,
		"arsenal":    arsenalPos,
		"navSpine":   spinePos,
		"terrain":    terrainPos,
		"blueprints": blueprintsPos,
	} {
		if pos < 0 {
			t.Fatalf("%s not found in index.html", name)
		}
	}

	if !(debriefPos < arsenalPos) {
		t.Errorf("Debrief must appear before Arsenal: debrief@%d arsenal@%d", debriefPos, arsenalPos)
	}
	if !(arsenalPos < spinePos) {
		t.Errorf("Arsenal must appear before spine: arsenal@%d spine@%d", arsenalPos, spinePos)
	}
	if !(spinePos < terrainPos) {
		t.Errorf("spine must appear before Terrain: spine@%d terrain@%d", spinePos, terrainPos)
	}
	if !(terrainPos < blueprintsPos) {
		t.Errorf("Terrain must appear before Blueprints: terrain@%d blueprints@%d", terrainPos, blueprintsPos)
	}

	t.Log("knowledge-zone nav position PASS: Debrief < Arsenal < spine < Terrain < Blueprints")
}

// TestQNavBarInDashboard verifies that index.html ships the QNAV query bar
// with correct structure:
//   - Query wrap: id="terrainQueryWrap" (hidden until graph loads)
//   - Query input: id="terrainQInput" with placeholder text
//   - Autocomplete: id="terrainQAC" (dropdown, hidden by default)
//   - Clear button: id="terrainQClearBtn"
//   - Query bar is positioned between canvas-wrap and status strip
func TestQNavBarInDashboard(t *testing.T) {
	htmlBytes, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("cannot read static/index.html: %v", err)
	}
	html := string(htmlBytes)

	// Required IDs
	for _, id := range []string{"terrainQueryWrap", "terrainQInput", "terrainQAC", "terrainQClearBtn"} {
		if !strings.Contains(html, `id="`+id+`"`) {
			t.Errorf("index.html missing #%s", id)
		}
	}

	// Query wrap must ship hidden (shown by JS when graph loads)
	qwPos := strings.Index(html, `id="terrainQueryWrap"`)
	if qwPos < 0 {
		t.Fatal("terrainQueryWrap not found")
	}
	// Check hidden attribute near terrainQueryWrap
	qwSnip := html[qwPos:qwPos+80]
	if !strings.Contains(qwSnip, `style="display:none"`) {
		t.Error(`terrainQueryWrap must ship with style="display:none"`)
	}

	// Input must have placeholder text (invitation copy per §2.1)
	if !strings.Contains(html, `placeholder="Search a file, or pick a question"`) {
		t.Error(`terrainQInput must have placeholder="Search a file, or pick a question"`)
	}

	// Query bar must appear between canvas-wrap and status strip
	canvasWrapPos := strings.Index(html, `id="terrainCanvasWrap"`)
	queryWrapPos  := strings.Index(html, `id="terrainQueryWrap"`)
	statusStripPos := strings.Index(html, `id="terrainStatusStrip"`)
	if canvasWrapPos < 0 || queryWrapPos < 0 || statusStripPos < 0 {
		t.Fatal("one of terrainCanvasWrap/terrainQueryWrap/terrainStatusStrip not found")
	}
	if !(canvasWrapPos < queryWrapPos && queryWrapPos < statusStripPos) {
		t.Errorf("query bar must appear between canvas-wrap and status-strip: wrap@%d qbar@%d strip@%d",
			canvasWrapPos, queryWrapPos, statusStripPos)
	}

	t.Log("QNAV bar assertions PASS: terrainQueryWrap hidden, correct placeholder, correct position")
}

// TestSemanticLensInDashboard verifies that index.html ships the semantic lens
// toggle button in the terrain status strip:
//   - terrainLensBtn: Meaning button for domain-grouped territories (Commit C)
func TestSemanticLensInDashboard(t *testing.T) {
	htmlBytes, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("cannot read static/index.html: %v", err)
	}
	html := string(htmlBytes)

	if !strings.Contains(html, `id="terrainLensBtn"`) {
		t.Error("index.html missing terrainLensBtn for semantic lens toggle")
	}
	// Lens button must be inside the terrain-status-strip (not outside)
	stripPos := strings.Index(html, `id="terrainStatusStrip"`)
	lensPos  := strings.Index(html, `id="terrainLensBtn"`)
	if stripPos < 0 || lensPos < 0 {
		t.Fatal("terrainStatusStrip or terrainLensBtn not found")
	}
	if lensPos <= stripPos {
		t.Error("terrainLensBtn must appear after terrainStatusStrip opening tag")
	}

	t.Log("semantic lens assertions PASS: terrainLensBtn present inside terrain-status-strip")
}

// TestBlastWalksImporters is the direction tripwire for the blast verb
// (merge-review blocker 2026-07-07): blast X answers "what breaks if X
// changes" = reverse BFS over IMPORTERS (inAdj). A regression to outAdj
// would silently answer "what does X use" (that's `deps`).
func TestBlastWalksImporters(t *testing.T) {
	js, err := os.ReadFile("static/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	if !bytes.Contains(js, []byte("terrainBFS(node.id, adj.inAdj, 6)")) {
		t.Fatal("blast verb must reverse-BFS over adj.inAdj (importers); found no such call — direction regression?")
	}
}
