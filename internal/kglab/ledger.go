package kglab

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ledger.go — the eat-the-elephant completeness ledger. It IS the ontology
// snapshot: which of the 16 concepts are wired as graph substrate, with a
// deterministic rev hash. Re-running shows what changed (drift over time).
//
// The ledger is a CATALOGUE + trip-wire, not a test runner. `go test` is the
// real enforcement; TestLedger_ExactlyOneWired asserts the authored baseline so
// a status flip without a real pipeline breaks CI.

// Ledger is a completeness snapshot of the ontology.
type Ledger struct {
	Rev     string       `json:"rev"`
	PrevRev string       `json:"prev_rev,omitempty"`
	Entries []ConceptDef `json:"entries"`
	Drifted []string     `json:"drifted,omitempty"`
}

// DefaultEntries returns a copy of the ontology (the ledger IS the ontology).
func DefaultEntries() []ConceptDef {
	out := make([]ConceptDef, len(Ontology))
	copy(out, Ontology)
	return out
}

// ledgerHash is a deterministic 12-hex fingerprint over the wiring state.
// (Reimplements the arch.factsHash recipe, which is unexported.)
func ledgerHash(entries []ConceptDef) string {
	rows := make([]string, len(entries))
	for i, e := range entries {
		rows[i] = e.Name + "\x00" + string(e.Role) + "\x00" + string(e.Status) + "\x00" + e.Evidence
	}
	sort.Strings(rows)
	sum := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(sum[:])[:12]
}

// BuildLedger computes the rev over the given entries.
func BuildLedger(entries []ConceptDef, prevRev string) Ledger {
	return Ledger{Rev: ledgerHash(entries), PrevRev: prevRev, Entries: entries}
}

// DiffLedger returns the concept names whose Role/Status/Evidence changed.
func DiffLedger(prev, next Ledger) []string {
	old := make(map[string]ConceptDef, len(prev.Entries))
	for _, e := range prev.Entries {
		old[e.Name] = e
	}
	var drifted []string
	for _, e := range next.Entries {
		if p, ok := old[e.Name]; !ok || p.Role != e.Role || p.Status != e.Status || p.Evidence != e.Evidence {
			drifted = append(drifted, e.Name)
		}
	}
	sort.Strings(drifted)
	return drifted
}

// wiredEdgeCount counts concepts that are real graph substrate today.
func wiredEdgeCount(entries []ConceptDef) int {
	n := 0
	for _, e := range entries {
		if e.Role == RoleEdge && e.Status == StatusWired {
			n++
		}
	}
	return n
}

// RenderTable renders the ledger as a human-readable table.
func RenderTable(l Ledger) string {
	var b strings.Builder
	fmt.Fprintf(&b, "LEDGER rev=%s  %d/%d concepts wired as graph edges\n", l.Rev, wiredEdgeCount(l.Entries), len(l.Entries))
	for _, e := range l.Entries {
		fmt.Fprintf(&b, "  %-15s %-9s %-6s %s\n", e.Name, e.Role, e.Status, e.Evidence)
	}
	if len(l.Drifted) > 0 {
		fmt.Fprintf(&b, "  drifted since %s: %s\n", l.PrevRev, strings.Join(l.Drifted, ", "))
	}
	return b.String()
}

// RenderJSON renders the ledger as deterministic JSON (no HTML escaping).
func RenderJSON(l Ledger) ([]byte, error) { return marshalNoEscape(l) }

// marshalNoEscape marshals compactly without HTML-escaping (mirrors arch.MarshalShard).
func marshalNoEscape(v interface{}) ([]byte, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimRight(b.String(), "\n")), nil
}
