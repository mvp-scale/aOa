package arch

import "testing"

func TestRoleFor(t *testing.T) {
	cases := []struct {
		label     string
		wantLayer string
	}{
		// core-logic (blue)
		{"domain", "core"},
		{"core", "core"},
		{"service", "core"},
		{"codec", "core"},     // default fallback
		{"binding", "core"},   // default fallback
		{"@business-logic", "core"},
		// boundary / interface (cyan)
		{"ports", "edge"},
		{"api", "edge"},
		{"controller", "edge"},
		{"@contracts", "edge"},
		// connector / adapter (orange)
		{"adapters", "integration"},
		{"httpapi", "integration"}, // http wins over api
		{"handlers", "integration"},
		{"gateway", "integration"},
		{"middleware", "integration"},
		// store / data (green)
		{"store", "data"},
		{"memstore", "data"},
		{"repository", "data"},
		{"cache", "data"},
		{"@storage", "data"},
		// external (grey)
		{"ext:std", "external"},
		{"external", "external"},
		{"vendor", "external"},
		// config / wiring (neutral)
		{"app", "supporting"},
		{"cmd", "supporting"},
		{"config", "supporting"},
	}
	for _, c := range cases {
		gotLayer, gotIco := roleFor(c.label)
		if gotLayer != c.wantLayer {
			t.Errorf("roleFor(%q) layer = %q, want %q", c.label, gotLayer, c.wantLayer)
		}
		if gotIco == "" {
			t.Errorf("roleFor(%q) returned empty icon", c.label)
		}
	}
}
