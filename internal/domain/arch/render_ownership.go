package arch

import (
	"fmt"
	"sort"
	"strings"
)

// ownershipRowsMax caps the Ownership table at the shared "table" budget
// (same shared-constant rationale as sbomRowsMax/changeRowsMax — no
// dedicated budget key exists for this view yet, and 50 comfortably covers
// a v1 80/20 repo's unit count while staying truncation-honest above it).
const ownershipRowsMax = 50

// RenderOwnership produces a "table" shard answering the Ownership (view id
// "ownership") question (view-standards.json: "Who owns each part of this
// system, and where is ownership thin?"). One row per in.OwnershipEntries
// element (COL-3, board M6): CODEOWNERS-declared owners where a rule
// matches, bounded-git-authorship-derived owners as the fallback.
//
// Provenance: per-row Provenance is carried straight through to the "Source"
// column ("CODEOWNERS" | "git shortlog"). The shard-level Prov follows the
// same "min over contributors" mixing rule every other renderer here uses:
// CODEOWNERS-only rows are REAL/derived (a syntactic parse of a declared
// file, same honesty tier as RenderDeployment's manifest extraction);
// git-authorship-only rows are MIXED (a heuristic top-author/unit-path
// join, same tier as RenderChangeMap); when both sources contribute, the
// shard is MIXED.
func RenderOwnership(in RenderInput) (*Shard, error) {
	columns := []string{"Path", "Owner(s)", "Source"}

	sorted := make([]OwnershipEntry, len(in.OwnershipEntries))
	copy(sorted, in.OwnershipEntries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	trueTotal := len(sorted)

	shown := sorted
	if len(shown) > ownershipRowsMax {
		shown = shown[:ownershipRowsMax]
	}

	var hasDeclared, hasDerived bool
	rows := make([][]string, 0, len(shown))
	for _, e := range shown {
		owners := "—"
		if len(e.Owners) > 0 {
			owners = strings.Join(e.Owners, ", ")
		}
		source := "git shortlog"
		if e.Provenance == "declared" {
			source = "CODEOWNERS"
			hasDeclared = true
		} else {
			hasDerived = true
		}
		rows = append(rows, []string{e.Path, owners, source})
	}
	if rows == nil {
		rows = [][]string{}
	}

	var prov Prov
	switch {
	case hasDeclared && hasDerived:
		prov = Prov{Kind: "mixed", Label: "MIXED · CODEOWNERS (declared) + bounded git-authorship fallback (derived)"}
	case hasDeclared:
		prov = Prov{Kind: "derived", Label: "REAL · CODEOWNERS parse"}
	default:
		prov = Prov{Kind: "mixed", Label: "MIXED · bounded git-authorship join (no CODEOWNERS present)"}
	}

	shard := &Shard{
		Kind:    "table",
		Title:   "Ownership",
		Prov:    prov,
		Columns: columns,
		Rows:    rows,
	}
	_, shard.FindingsClause = DeriveCaption(shard, in.Findings)

	switch {
	case trueTotal == 0:
		shard.Count = "0 units with defined owners"
	case trueTotal > len(shown):
		shard.Count = fmt.Sprintf("%d units with owners — showing %d", trueTotal, len(shown))
	default:
		shard.Count = fmt.Sprintf("%d units with owners", trueTotal)
	}
	return shard, nil
}
