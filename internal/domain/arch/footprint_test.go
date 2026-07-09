package arch

import (
	"os"
	"path/filepath"
	"testing"
)

// mkTree creates a set of empty files under root (dirs auto-created).
func mkTree(t *testing.T, root string, files []string) {
	t.Helper()
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
}

// TestDetectFootprint_Scrapy is the headline case: a setup.py package whose
// architecture lives one level down under scrapy/. The detector must produce a
// single anchor rooted at scrapy/ with a descend grain so the 25 subpackages
// become 25 groups (not one SCRAPY box).
func TestDetectFootprint_Scrapy(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root, []string{
		"setup.py",
		"scrapy/__init__.py",
		"scrapy/core/engine.py",
		"scrapy/core/scheduler.py",
		"scrapy/http/request.py",
		"scrapy/http/response.py",
		"scrapy/spidermiddlewares/offsite.py",
		"scrapy/utils/misc.py",
		"docs/conf.py",
		"tests/test_engine.py",
		"extras/qps.py",
	})

	fp, err := DetectFootprint(root)
	if err != nil {
		t.Fatalf("DetectFootprint: %v", err)
	}
	if fp.Kind != "python-app" {
		t.Errorf("kind = %q, want python-app", fp.Kind)
	}
	if fp.Engine != "deterministic" {
		t.Errorf("engine = %q, want deterministic", fp.Engine)
	}
	if fp.Prov != "derived" {
		t.Errorf("prov = %q, want derived (ruling A)", fp.Prov)
	}
	if len(fp.Anchors) != 1 {
		t.Fatalf("anchors = %d, want 1 (v1 single-anchor, ruling B)", len(fp.Anchors))
	}
	a := fp.Anchors[0]
	if a.Path != "scrapy" {
		t.Errorf("anchor path = %q, want scrapy", a.Path)
	}
	if a.Grain == nil || a.Grain.Mode != "descend" || a.Grain.Under != "scrapy" || a.Grain.Depth != 1 {
		t.Errorf("grain = %+v, want descend under=scrapy depth=1", a.Grain)
	}
	// docs/tests/extras are satellites → excluded, never anchors.
	if !containsStr(fp.Excluded, "docs") || !containsStr(fp.Excluded, "tests") {
		t.Errorf("excluded = %v, want to contain docs+tests", fp.Excluded)
	}
}

// TestDetectFootprint_Aoa is the no-regression case: aOa's own hexagonal layout
// (go.mod at root, code in internal/, cmd/). The detector must NOT invent a
// descend grain that reshapes the existing views — a root go.mod anchors at the
// repo root with default (segment) grain, so grouping stays byte-identical.
func TestDetectFootprint_Aoa(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root, []string{
		"go.mod",
		"cmd/aoa/main.go",
		"internal/app/app.go",
		"internal/domain/arch/model.go",
		"internal/ports/storage.go",
	})

	fp, err := DetectFootprint(root)
	if err != nil {
		t.Fatalf("DetectFootprint: %v", err)
	}
	if fp.Kind != "go-app" {
		t.Errorf("kind = %q, want go-app", fp.Kind)
	}
	if len(fp.Anchors) != 1 {
		t.Fatalf("anchors = %d, want 1", len(fp.Anchors))
	}
	a := fp.Anchors[0]
	if a.Path != "" {
		t.Errorf("anchor path = %q, want \"\" (repo root)", a.Path)
	}
	// Root anchor → default segment grain (nil or segment). The `internal`
	// descent stays a code-level default, NOT a footprint descend rule.
	if a.Grain != nil && a.Grain.Mode == "descend" {
		t.Errorf("grain = %+v, want segment/nil for root-anchored go repo", a.Grain)
	}
}

// TestDetectFootprint_GoModAtDepth exercises 1b: a single go.mod one level down
// (e.g. a repo where the module lives under app/). Anchor roots there, and grain
// descends into it.
func TestDetectFootprint_GoModAtDepth(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root, []string{
		"README.md",
		"app/go.mod",
		"app/internal/server/server.go",
		"app/cmd/main.go",
	})

	fp, err := DetectFootprint(root)
	if err != nil {
		t.Fatalf("DetectFootprint: %v", err)
	}
	if len(fp.Anchors) != 1 {
		t.Fatalf("anchors = %d, want 1", len(fp.Anchors))
	}
	a := fp.Anchors[0]
	if a.Path != "app" {
		t.Errorf("anchor path = %q, want app", a.Path)
	}
	if a.Grain == nil || a.Grain.Mode != "descend" || a.Grain.Under != "app" || a.Grain.Depth != 1 {
		t.Errorf("grain = %+v, want descend under=app depth=1", a.Grain)
	}
}

// TestDetectFootprint_NoMarker: a repo with no manifest at all. v1 ships Layers
// 0–1b only (no shape fallback), so a no-marker repo yields a single root anchor
// with default grain → byte-identical to today.
func TestDetectFootprint_NoMarker(t *testing.T) {
	root := t.TempDir()
	mkTree(t, root, []string{
		"main.py",
		"lib/util.py",
	})

	fp, err := DetectFootprint(root)
	if err != nil {
		t.Fatalf("DetectFootprint: %v", err)
	}
	if len(fp.Anchors) != 1 {
		t.Fatalf("anchors = %d, want 1", len(fp.Anchors))
	}
	if fp.Anchors[0].Path != "" {
		t.Errorf("anchor path = %q, want \"\" (root, default grain)", fp.Anchors[0].Path)
	}
	if fp.Anchors[0].Grain != nil && fp.Anchors[0].Grain.Mode == "descend" {
		t.Errorf("grain = %+v, want default for no-marker repo", fp.Anchors[0].Grain)
	}
}

// TestFootprint_MarshalRoundTrip verifies byte-stable JSON with no timestamps.
func TestFootprint_MarshalRoundTrip(t *testing.T) {
	fp := &Footprint{
		Schema:  FootprintSchema,
		Kind:    "python-app",
		Engine:  "deterministic",
		Prov:    "derived",
		Anchors: []Anchor{{Path: "scrapy", Kind: "python-app", Marker: "setup.py", Confidence: "high", Grain: &Grain{Mode: "descend", Under: "scrapy", Depth: 1}}},
		Excluded: []string{"docs", "tests"},
	}
	data, err := MarshalFootprint(fp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// No timestamp field in output (byte-stability requirement).
	if containsSub(string(data), "generatedAt") || containsSub(string(data), "\"time\"") {
		t.Errorf("footprint JSON must not carry a timestamp: %s", data)
	}
	got, err := ParseFootprint(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Kind != fp.Kind || len(got.Anchors) != 1 || got.Anchors[0].Grain.Under != "scrapy" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	// Marshal is deterministic (same bytes twice).
	data2, _ := MarshalFootprint(fp)
	if string(data) != string(data2) {
		t.Errorf("marshal not byte-stable")
	}
}

// TestRefineFootprint_Seam verifies the Haiku seam (ruling D): the default
// refiner is a no-op that returns the deterministic footprint unchanged, with
// no network access. This is the interface point --refine drops into later.
func TestRefineFootprint_Seam(t *testing.T) {
	fp := &Footprint{Schema: FootprintSchema, Kind: "go-app", Engine: "deterministic", Prov: "derived"}
	var r FootprintRefiner = DeterministicRefiner{}
	out, err := r.RefineFootprint(fp)
	if err != nil {
		t.Fatalf("refine: %v", err)
	}
	if out != fp {
		t.Errorf("no-op refiner must return the same footprint unchanged")
	}
	if out.Engine != "deterministic" {
		t.Errorf("engine must stay deterministic when refinement is a no-op")
	}
}

// TestDetectFootprint_AoaSelf_NoDescend is the byte-identical guarantee at the
// footprint level: detecting aOa's OWN repo must NOT produce a descend grain.
// aOa's architecture is spread across internal/, cmd/, ports/, atlas/ — no
// single dominant subtree — so the root anchor keeps default (segment) grain and
// every existing arch view (goldens) is unchanged. This runs against the live
// worktree, not a synthetic tree.
func TestDetectFootprint_AoaSelf_NoDescend(t *testing.T) {
	// Walk up from CWD to the repo root (dir containing go.mod).
	root := findRepoRoot(t)
	fp, err := DetectFootprint(root)
	if err != nil {
		t.Fatalf("DetectFootprint(aoa): %v", err)
	}
	if fp.Kind != "go-app" {
		t.Errorf("aoa kind = %q, want go-app", fp.Kind)
	}
	if len(fp.Anchors) != 1 {
		t.Fatalf("aoa anchors = %d, want 1", len(fp.Anchors))
	}
	a := fp.Anchors[0]
	if a.Path != "" {
		t.Errorf("aoa anchor path = %q, want \"\" (repo root — no re-root)", a.Path)
	}
	if a.Grain != nil && a.Grain.Mode == "descend" {
		t.Errorf("aoa grain = %+v, want segment/nil — a descend grain would reshape aOa's own views (byte-identical guarantee broken)", a.Grain)
	}
	if g := fp.PrimaryGrain(); g != nil && g.Mode == "descend" {
		t.Errorf("aoa PrimaryGrain = %+v, want nil/segment", g)
	}
}

// findRepoRoot walks up from the test's working dir to the dir holding go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find repo root (go.mod) from test cwd")
	return ""
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func containsSub(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
