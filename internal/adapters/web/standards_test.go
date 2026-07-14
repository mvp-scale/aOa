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
