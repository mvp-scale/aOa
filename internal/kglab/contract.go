package kglab

import (
	"fmt"

	"github.com/corey/aoa/internal/domain/arch"
)

// contract.go — the agent-agnostic contract. Plain stdlib structs with json
// tags; no Claude/MCP/SDK types. Any agent (GPT, Gemini, a bash script) drives
// the same four nouns: graph (where-we-are), target (where-to-be), drift (the
// vector), ledger (the elephant). Text captions answer in one line; JSON carries
// the evidence.

// RenderResponseJSON marshals any response envelope deterministically without
// HTML-escaping (mirrors arch.MarshalShard), so any agent/CI can diff outputs.
func RenderResponseJSON(v interface{}) ([]byte, error) { return marshalNoEscape(v) }

// GraphResponse — where-we-are.
type GraphResponse struct {
	OK      bool        `json:"ok"`
	Verb    string      `json:"verb"`
	Caption string      `json:"caption"`
	Render  string      `json:"render,omitempty"`
	Prov    string      `json:"prov,omitempty"`
	Shard   *arch.Shard `json:"shard,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// TargetResponse — where-to-be (ontology coverage).
type TargetResponse struct {
	OK      bool   `json:"ok"`
	Verb    string `json:"verb"`
	Caption string `json:"caption"`
	Total   int    `json:"total"`
	Wired   int    `json:"wired"`
	Error   string `json:"error,omitempty"`
}

// DriftResponse — the vector between real and target.
type DriftResponse struct {
	OK      bool        `json:"ok"`
	Verb    string      `json:"verb"`
	Caption string      `json:"caption"`
	Result  DriftResult `json:"result"`
	Error   string      `json:"error,omitempty"`
}

// LedgerResponse — the completeness snapshot.
type LedgerResponse struct {
	OK      bool   `json:"ok"`
	Verb    string `json:"verb"`
	Caption string `json:"caption"`
	Ledger  Ledger `json:"ledger"`
	Error   string `json:"error,omitempty"`
}

// ComputeGraph renders a blueprint over the sample REAL graph.
func ComputeGraph(render, seed, dir string) GraphResponse {
	units, deps := SampleGraph()
	q := ViewQuery{Scope: "sample", Render: RenderSpec{Kind: render}}
	if seed != "" {
		q.Traverse = &TraverseSpec{Seed: seed, Dir: dir, EdgeKind: "imports"}
	}
	shard, err := Compile(q, units, deps)
	if err != nil {
		return GraphResponse{OK: false, Verb: "graph", Error: err.Error()}
	}
	return GraphResponse{
		OK:      true,
		Verb:    "graph",
		Render:  render,
		Prov:    shard.Prov.Label,
		Caption: "WHERE-WE-ARE (import-only): " + shard.Count,
		Shard:   shard,
	}
}

// ComputeTarget reports ontology coverage — how far to where-we-need-to-be.
func ComputeTarget() TargetResponse {
	entries := DefaultEntries()
	wired := wiredEdgeCount(entries)
	return TargetResponse{
		OK:      true,
		Verb:    "target",
		Total:   len(entries),
		Wired:   wired,
		Caption: fmt.Sprintf("WHERE-TO-BE: %d of %d concepts wired as graph edges — call is recon-matched (next bite), rest stub", wired, len(entries)),
	}
}

// ComputeDrift computes the vector between the sample real graph and sample target.
func ComputeDrift() DriftResponse {
	_, deps := SampleGraph()
	real := FactSetFromDeps("real", deps)
	target, err := LoadTarget("target", SampleTarget())
	if err != nil {
		return DriftResponse{OK: false, Verb: "drift", Error: err.Error()}
	}
	res := DriftDiff(real, target)
	return DriftResponse{
		OK:      true,
		Verb:    "drift",
		Result:  res,
		Caption: fmt.Sprintf("VECTOR (import-only): %d VIOLATION, %d MISSING, %d CONFORMANT — act on file:line in --json", res.Violations, res.Missing, res.Conformant),
	}
}

// ComputeLedger builds the completeness snapshot; prevRev enables change detection.
func ComputeLedger(prevRev string) LedgerResponse {
	l := BuildLedger(DefaultEntries(), prevRev)
	if prevRev != "" {
		prev := BuildLedger(DefaultEntries(), "")
		l.Drifted = DiffLedger(prev, l)
	}
	return LedgerResponse{
		OK:      true,
		Verb:    "ledger",
		Ledger:  l,
		Caption: fmt.Sprintf("LEDGER: %d/%d concepts wired · rev=%s", wiredEdgeCount(l.Entries), len(l.Entries), l.Rev),
	}
}
