package arch

import (
	"testing"
)

// TestPathPrefixGroup_NilGrainByteIdentical is the MUST-NOT-CUT guarantee
// (consensus §5): with NO footprint grain (nil), pathPrefixGroup must return
// exactly what the pre-recon top-level-dir grouper returned. We assert this by
// running a representative set of aOa-shaped paths through the new grain-aware
// function with nil grain and comparing to the hand-verified legacy outputs.
func TestPathPrefixGroup_NilGrainByteIdentical(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"internal/domain/arch/model.go", "domain"},
		{"internal/adapters/bbolt/store.go", "adapters"},
		{"cmd/aoa/main.go", "cmd"},
		{"ports/storage.go", "ports"},
		{"src/lib/util.go", "lib"},    // src skipped; lib is the real group
		{"pkg/controller/ctrl.go", "controller"}, // pkg skipped
		{"main.go", "other"},          // filename only → other
		{"scrapy/core/engine.py", "scrapy"}, // legacy: collapses to top-level (the bug)
		{"scrapy/http/request.py", "scrapy"},
	}
	for _, c := range cases {
		got := pathPrefixGroup(c.path, nil)
		if got != c.want {
			t.Errorf("pathPrefixGroup(%q, nil) = %q, want %q (byte-identical fallback broken)", c.path, got, c.want)
		}
	}
}

// TestPathPrefixGroup_DescendGrain is the scrapy fix: with a descend grain
// under="scrapy" depth=1, scrapy's subpackages become distinct groups instead
// of collapsing into one SCRAPY box.
func TestPathPrefixGroup_DescendGrain(t *testing.T) {
	g := &Grain{Mode: "descend", Under: "scrapy", Depth: 1}
	cases := []struct {
		path string
		want string
	}{
		{"scrapy/core/engine.py", "core"},
		{"scrapy/http/request.py", "http"},
		{"scrapy/spidermiddlewares/offsite.py", "spidermiddlewares"},
		{"scrapy/utils/misc.py", "utils"},
		{"scrapy/__init__.py", "scrapy"}, // file directly under scrapy → stays scrapy
		// Paths outside the `under` prefix are grouped by default rule.
		{"docs/conf.py", "docs"},
		{"tests/test_engine.py", "tests"},
	}
	for _, c := range cases {
		got := pathPrefixGroup(c.path, g)
		if got != c.want {
			t.Errorf("pathPrefixGroup(%q, descend) = %q, want %q", c.path, got, c.want)
		}
	}
}

// TestPathPrefixGroup_SegmentGrainEqualsNil: an explicit segment-mode grain must
// behave identically to nil (default top-level-dir grouping).
func TestPathPrefixGroup_SegmentGrainEqualsNil(t *testing.T) {
	g := &Grain{Mode: "segment"}
	paths := []string{
		"internal/domain/arch/model.go",
		"cmd/aoa/main.go",
		"scrapy/core/engine.py",
		"src/lib/util.go",
	}
	for _, p := range paths {
		if got, want := pathPrefixGroup(p, g), pathPrefixGroup(p, nil); got != want {
			t.Errorf("segment grain %q = %q, want %q (must equal nil grain)", p, got, want)
		}
	}
}

// TestGroupingResult_NilFootprintUnchanged asserts the whole Group() pipeline is
// byte-identical when grain is not supplied — Group() keeps its no-grain signature.
func TestGroupingResult_NilFootprintUnchanged(t *testing.T) {
	units := []UnitFact{
		{ID: "u1", Path: "internal/domain/arch/model.go"},
		{ID: "u2", Path: "internal/adapters/bbolt/store.go"},
		{ID: "u3", Path: "cmd/aoa/main.go"},
	}
	res := Group(units)
	if res.UnitGroup["u1"] != "g_domain" {
		t.Errorf("u1 group = %q, want g_domain", res.UnitGroup["u1"])
	}
	if res.UnitGroup["u2"] != "g_adapters" {
		t.Errorf("u2 group = %q, want g_adapters", res.UnitGroup["u2"])
	}
	if res.UnitGroup["u3"] != "g_cmd" {
		t.Errorf("u3 group = %q, want g_cmd", res.UnitGroup["u3"])
	}
}
