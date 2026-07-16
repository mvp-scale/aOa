//go:build !lean

package web

// A1: reserved-color regression gate. Red and yellow are RESERVED for
// violations/faults and warnings/simulated-provenance decoration
// (view-standards.json global.palette.reserved) — no named_palettes entry
// may claim either color for a layer, or a data-file edit could silently
// repaint a layer with a meaning-carrying reserved color.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStandards_NamedPalettesNeverUseReservedColors(t *testing.T) {
	data, err := archStaticFS.ReadFile("static/arch/view-standards.json")
	require.NoError(t, err, "view-standards.json must be embedded and readable")

	var doc struct {
		Global struct {
			Palette struct {
				NamedPalettes map[string]map[string]string `json:"named_palettes"`
			} `json:"palette"`
		} `json:"global"`
	}
	require.NoError(t, json.Unmarshal(data, &doc), "view-standards.json must be valid JSON")
	require.NotEmpty(t, doc.Global.Palette.NamedPalettes, "named_palettes must be present")

	reserved := map[string]bool{"red": true, "yellow": true}
	for paletteID, mapping := range doc.Global.Palette.NamedPalettes {
		for layer, color := range mapping {
			require.Falsef(t, reserved[color],
				"named_palettes.%s.%s = %q uses a RESERVED color (red/yellow are violations/warnings only)",
				paletteID, layer, color)
		}
	}
}

// SUR-2/R2: container/capability twin (D39) — the kickoff ruling (option B,
// M6-completion/tasks.md:101) formalizes viewer.js:68's informal vid-merge
// fallback (["capability","container"]) as an explicit alias recorded on the
// standards schema, instead of re-keying the sim-estate JSONs (option A) that
// still author their mockup views under the legacy "container" id.
func TestStandards_CapabilityRecordsContainerAlias(t *testing.T) {
	data, err := archStaticFS.ReadFile("static/arch/view-standards.json")
	require.NoError(t, err, "view-standards.json must be embedded and readable")

	var doc struct {
		Views map[string]struct {
			Alias string `json:"alias"`
		} `json:"views"`
	}
	require.NoError(t, json.Unmarshal(data, &doc), "view-standards.json must be valid JSON")

	cap, ok := doc.Views["capability"]
	require.True(t, ok, "views.capability must be present")
	require.Equal(t, "container", cap.Alias,
		"views.capability.alias must record 'container' as its formal alias (R2/D39 ruling)")
}
