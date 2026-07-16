//go:build !lean

package web

// BEAU-1 (board M6, table-view-beautification): source-text regression gate over
// viewer.js/viewer.css, the same style TestViewerForkGuard/TestStandards_* already use
// for this embedded, test-runner-less frontend — there is no JS harness in this repo, so
// the shared TableView spine (sticky header, numeric-column right-align, row-hover,
// density wiring, header click-sort, per-view treatments) and the emptySelText/toolbar
// fixes are locked in as substring assertions against the live static assets instead of
// a snapshot render.

import (
	"strings"
	"testing"
)

func readArchStatic(t *testing.T, name string) string {
	t.Helper()
	data, err := archStaticFS.ReadFile("static/arch/" + name)
	if err != nil {
		t.Fatalf("cannot read static/arch/%s: %v", name, err)
	}
	return string(data)
}

// TestBEAU1_TableViewSpine_StickyHeaderAndNumericAlign asserts the TableView thead cells
// are sticky (DSM precedent :591-605) and that numeric columns are detected via the
// shared isNumCell idiom and right-aligned with tabular-nums.
func TestBEAU1_TableViewSpine_StickyHeaderAndNumericAlign(t *testing.T) {
	js := readArchStatic(t, "viewer.js")

	if !strings.Contains(js, "const isNumCell=") {
		t.Fatal("viewer.js: expected a shared isNumCell idiom (reused by TableView and DockTable)")
	}
	if !strings.Contains(js, "function TableView({view,onSel,selId,vid,den})") {
		t.Fatal("viewer.js: TableView must accept a `den` (density) prop for row-padding wiring")
	}
	// thead cell must be sticky (position:sticky,top:0) — DSM precedent, not a new pattern.
	tvStart := strings.Index(js, "function TableView(")
	tvEnd := strings.Index(js[tvStart:], "\nfunction DSMView(")
	if tvStart < 0 || tvEnd < 0 {
		t.Fatal("viewer.js: could not isolate the TableView function body")
	}
	tvBody := js[tvStart : tvStart+tvEnd]
	if !strings.Contains(tvBody, `position:"sticky",top:0`) {
		t.Fatal("TableView: thead th must be position:sticky,top:0 (sticky header)")
	}
	if !strings.Contains(tvBody, "numCols[") {
		t.Fatal("TableView: numeric columns must be detected (numCols) and drive right-align")
	}
	if !strings.Contains(tvBody, `fontVariantNumeric:"tabular-nums"`) {
		t.Fatal("TableView: numeric cells must use tabular-nums numerals")
	}
	if !strings.Contains(tvBody, `class="table-row-hover"`) {
		t.Fatal("TableView: rows must carry the table-row-hover affordance class")
	}
	if !strings.Contains(tvBody, "rowPad") {
		t.Fatal("TableView: row padding must be density-wired (compact/comfort)")
	}
}

// TestBEAU1_TableViewSpine_HeaderClickSortDefaultsToShardOrder asserts header click-sort
// exists and defaults to shard order (col:null) — the shard order IS each view's answer
// (e.g. change is already risk-desc), so sort must be able to return to it.
func TestBEAU1_TableViewSpine_HeaderClickSortDefaultsToShardOrder(t *testing.T) {
	js := readArchStatic(t, "viewer.js")
	if !strings.Contains(js, `useState({col:null,dir:1})`) {
		t.Fatal("TableView: sort state must default to shard order (col:null)")
	}
	if !strings.Contains(js, "toggleSort") {
		t.Fatal("TableView: expected a header click-sort handler (toggleSort)")
	}
}

// TestBEAU1_PerViewTreatments asserts the per-view cell treatments named in the BEAU-1
// work order: api-contract method chip, glossary owning-domain chip + 60ch wrapped
// definition, sbom unpinned chip, ownership owner(s) chips, change/techportfolio data-ink
// bar — and that the always-on red ⚠ rule (R8) is untouched.
func TestBEAU1_PerViewTreatments(t *testing.T) {
	js := readArchStatic(t, "viewer.js")

	for _, want := range []string{
		`vid==="sbom"&&cols[ci]==="unpinned"`,
		`vid==="api-contract"&&ci===0`,
		`vid==="glossary"&&ci===2`,
		`vid==="ownership"&&cols[ci]==="Owner(s)"`,
		`vid==="change"?cols.length-1:vid==="techportfolio"?cols.indexOf("count")`,
		`maxWidth:"60ch"`,
		"dataInkBar",
		"quietChip",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("viewer.js: expected per-view treatment marker %q", want)
		}
	}

	// R8: ⚠-prefixed cells stay always-on red — must not be gated behind showFindings.
	if !strings.Contains(js, `color:String(cell).startsWith("⚠")?T.red`) {
		t.Fatal("TableView: ⚠-prefixed cells must keep the always-on red rule (R8)")
	}

	// data-ink bar accent must never be red/yellow (A3/R8: reserved colors).
	if strings.Contains(js, "T.red}+\"") || strings.Contains(js, "T.yellow+\"") {
		t.Fatal("dataInkBar: must not use a reserved (red/yellow) accent color")
	}
}

// TestBEAU1_EmptySelText_TableKindSaysRow asserts the SELECTION empty-state text for
// kind==="table" reads "click a row to inspect" (rows are the only clickable element in
// a TableView — the previous cell/row-header wording was written for the DSM matrix).
func TestBEAU1_EmptySelText_TableKindSaysRow(t *testing.T) {
	js := readArchStatic(t, "viewer.js")
	if !strings.Contains(js, `?"click a row to inspect"`) {
		t.Fatal(`viewer.js: emptySelText must return "click a row to inspect" for kind==="table"`)
	}
	// matrix (DSM) keeps its own affordance text — must not be collapsed into the table text.
	if !strings.Contains(js, "click a cell to inspect · click a row header to expand") {
		t.Fatal("viewer.js: matrix/DSM empty-selection text must be preserved")
	}
}

// TestBEAU1_Toolbar_DirectionButtonsHiddenForHtmlView_DensityStaysVisible asserts the
// inert Auto/direction buttons are hidden for table/matrix (htmlView) views, while
// Compact/Comfort remain visible — they now drive TableView row padding, so they are no
// longer inert for table views.
func TestBEAU1_Toolbar_DirectionButtonsHiddenForHtmlView_DensityStaysVisible(t *testing.T) {
	js := readArchStatic(t, "viewer.js")

	compactIdx := strings.Index(js, `btn(den==="compact")`)
	autoIdx := strings.Index(js, `title="Pick the direction that best fits the viewport"`)
	guardIdx := strings.Index(js, `${!(els&&els.htmlView)?html`)
	if compactIdx < 0 || autoIdx < 0 || guardIdx < 0 {
		t.Fatal("viewer.js: expected Compact button, Auto direction button, and an htmlView guard")
	}
	if !(compactIdx < guardIdx && guardIdx < autoIdx) {
		t.Fatal("viewer.js: htmlView guard must wrap the Auto/direction buttons but not Compact/Comfort")
	}
}

// TestBEAU1_ViewerCSS_RowHoverClass asserts the shared row-hover affordance class exists
// (hover:[] is standards law — one house-wide class, no invented per-view hover metadata).
func TestBEAU1_ViewerCSS_RowHoverClass(t *testing.T) {
	css := readArchStatic(t, "viewer.css")
	if !strings.Contains(css, ".table-row-hover:hover{background:") {
		t.Fatal("viewer.css: expected a .table-row-hover:hover rule")
	}
}
