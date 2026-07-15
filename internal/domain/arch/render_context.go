package arch

import (
	"fmt"
	"sort"
	"strings"
)

// RenderContext produces a "simple" shard answering the System Context
// question (view-standards.json:63-76): "Who and what does this system talk
// to, and why?" One system-identity node plus every external actor/system it
// exchanges dep edges with, connected by labeled relationships.
//
// # Algorithm
//
//  1. Partition units into local vs external via isExternalUnit — a unit is
//     external when its ID/Path carries the "ext:" namespace (D7; stamped by
//     factSubjectToUnitPath/unitFactsFromFactStore in the app-layer bridge).
//  2. Walk every DepFact with exactly one external endpoint and aggregate by
//     external unit: Count summed, direction preserved (system→external for
//     imports, external→system for the rare inbound case), label taken from
//     DepFact.Kind (default "imports" per D28 — no invented verbs).
//  3. The system-identity node reuses RenderCode's documented entrypoint
//     heuristic (selectCodeEntrypoint) for its label/source — one "primary
//     entrypoint" concept shared by both renderers, not two competing ones.
//  4. Budget: cap external nodes at simple_view_nodes_max-1 (view-standards.json
//     global.budgets.simple_view_nodes_max=30, leaving room for the system
//     node), keeping the highest-count relationships first; ties broken
//     alphabetically by unit ID. Final node order is alphabetical for
//     byte-stability (T4).
//
// # Provenance
//
// Always MIXED (D2/D15): edges are real import facts, but external unit
// naming/labels are heuristic (factSubjectToUnitPath + unitLabel, upstream
// of this renderer) — never claimed as fully derived.
func RenderContext(in RenderInput) (*Shard, error) {
	unitByID := make(map[string]UnitFact, len(in.Units))
	for _, u := range in.Units {
		unitByID[u.ID] = u
	}

	// 1-2. Aggregate local<->external relationships, one per external unit.
	type rel struct {
		count   int
		label   string
		inbound bool // true: external -> system; false: system -> external
	}
	rels := make(map[string]*rel)
	for _, d := range in.Deps {
		fromExt := isExternalUnit(d.FromUnit, unitByID)
		toExt := isExternalUnit(d.ToUnit, unitByID)
		if fromExt == toExt {
			continue // both local or both external — not a context-view edge
		}
		extID := d.ToUnit
		inbound := false
		if fromExt {
			extID = d.FromUnit
			inbound = true
		}
		label := d.Kind
		if label == "" {
			label = "imports"
		}
		r, ok := rels[extID]
		if !ok {
			r = &rel{label: label, inbound: inbound}
			rels[extID] = r
		}
		r.count += d.Count
	}

	prov := Prov{Kind: "mixed", Label: "MIXED · external system naming heuristic · edges real dep facts"}

	if len(rels) == 0 {
		shard := &Shard{Kind: "simple", Title: "System Context", Prov: prov}
		_, shard.FindingsClause = DeriveCaption(shard, in.Findings)
		shard.Count = "0 external systems"
		return shard, nil
	}

	// 3. Budget: sort candidates by count desc then ID asc, cap, then re-sort
	// alphabetically for the final byte-stable node order.
	const maxNodes = 30
	trueTotal := len(rels) // pre-cap truth (VP-1.p1: caption must never claim the display budget as fact)
	extIDs := make([]string, 0, len(rels))
	for id := range rels {
		extIDs = append(extIDs, id)
	}
	sort.Slice(extIDs, func(i, j int) bool {
		if rels[extIDs[i]].count != rels[extIDs[j]].count {
			return rels[extIDs[i]].count > rels[extIDs[j]].count
		}
		return extIDs[i] < extIDs[j]
	})
	if len(extIDs) > maxNodes-1 {
		extIDs = extIDs[:maxNodes-1]
	}
	sort.Strings(extIDs)

	// 4. System identity node — same entrypoint heuristic as RenderCode.
	const sysID = "sys_self"
	sysLabel := "this system"
	var sysSources []SourceRef
	if ep, ok := unitByID[selectCodeEntrypoint(in.Units, in.Deps)]; ok {
		sysLabel = ep.Label
		if ep.File != "" {
			sysSources = []SourceRef{{File: ep.File, Line: ep.Line}}
		}
	}

	nodes := make([]Node, 0, len(extIDs)+1)
	nodes = append(nodes, Node{
		ID:      sysID,
		Type:    "sys",
		Label:   truncate(sysLabel, 30),
		Real:    true,
		Sources: sysSources,
	})
	for _, id := range extIDs {
		nodes = append(nodes, Node{
			ID:    id,
			Type:  "ext",
			Label: truncate(externalLabel(id, unitByID), 30),
			Real:  false,
		})
	}

	edges := make([]ShardEdge, 0, len(extIDs))
	for i, id := range extIDs {
		r := rels[id]
		src, dst := sysID, id
		if r.inbound {
			src, dst = id, sysID
		}
		edges = append(edges, ShardEdge{
			ID:     fmt.Sprintf("e%d", i+1),
			Source: src,
			Target: dst,
			Count:  r.count,
			Label:  r.label,
		})
	}

	shard := &Shard{
		Kind:  "simple",
		Title: "System Context",
		Prov:  prov,
		Nodes: nodes,
		Edges: edges,
	}
	_, shard.FindingsClause = DeriveCaption(shard, in.Findings)
	if trueTotal > len(extIDs) {
		// Budget truncated the drawing (D22/D23 still enforced on the canvas) —
		// the caption must say so honestly: true total + shown subset, calm
		// phrasing, no alarm glyph (D17: Count clause stays calm).
		shard.Count = fmt.Sprintf("%d external systems (showing %d) · %d relationships", trueTotal, len(extIDs), trueTotal)
	} else {
		shard.Count = fmt.Sprintf("%d external systems · %d relationships", len(extIDs), len(edges))
	}
	return shard, nil
}

// isExternalUnit reports whether unit id belongs to the "ext:" namespace
// (D7): either the raw ID carries the namespace directly (test fixtures,
// pre-slug IDs), the slugified form does (UnitSlug("ext:...") -> "u_ext_...",
// the real arch_factstore_bridge.go pipeline), or the resolved UnitFact.Path
// does (belt-and-suspenders fallback).
func isExternalUnit(id string, unitByID map[string]UnitFact) bool {
	if strings.HasPrefix(id, "ext:") || strings.HasPrefix(id, "u_ext_") {
		return true
	}
	if u, ok := unitByID[id]; ok {
		return strings.HasPrefix(u.Path, "ext:")
	}
	return false
}

// externalLabel returns the display label for an external unit ID, preferring
// the resolved UnitFact.Label (already heuristically friendly-named upstream
// by the app-layer unitLabel — e.g. "ext:go.etcd.io/bbolt" -> "bbolt"). Falls
// back to stripping the "ext:" namespace prefix directly when no UnitFact is
// present (defensive: the real bridge always mints one, but tests/synthetic
// inputs may not).
func externalLabel(id string, unitByID map[string]UnitFact) string {
	if u, ok := unitByID[id]; ok && u.Label != "" {
		return u.Label
	}
	return strings.TrimPrefix(id, "ext:")
}
