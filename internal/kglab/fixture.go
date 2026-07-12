package kglab

import "github.com/corey/aoa/internal/domain/arch"

// SampleGraph returns a deterministic, in-memory test knowledge graph modelling
// a tiny hexagonal codebase. No daemon, no filesystem, no parser — pure struct
// literals so every render is byte-stable across machines.
//
// Shape (paths chosen so arch.Group yields exactly 5 groups):
//
//	cmd      : cmd/aoa                                    (1)
//	app      : internal/app                               (1)
//	domain   : searcher, learner, enricher, indexer       (4)
//	adapters : bbolt, socket                              (2)
//	ports    : storage                                    (1)
//
// A cycle is planted inside the domain group:
//
//	searcher -> learner -> enricher -> searcher
//
// so the cycles blueprint has exactly one SCC to report.
func SampleGraph() ([]arch.UnitFact, []arch.DepFact) {
	u := func(id, label, path string) arch.UnitFact {
		return arch.UnitFact{ID: id, Label: label, Path: path, File: path + "/mod.go", Line: 1}
	}
	units := []arch.UnitFact{
		u("m_cmd_aoa", "cmd/aoa", "cmd/aoa"),
		u("m_app", "app", "internal/app"),
		u("m_domain_searcher", "domain/searcher", "internal/domain/searcher"),
		u("m_domain_learner", "domain/learner", "internal/domain/learner"),
		u("m_domain_enricher", "domain/enricher", "internal/domain/enricher"),
		u("m_domain_indexer", "domain/indexer", "internal/domain/indexer"),
		u("m_adapters_bbolt", "adapters/bbolt", "internal/adapters/bbolt"),
		u("m_adapters_socket", "adapters/socket", "internal/adapters/socket"),
		u("m_ports_storage", "ports/storage", "internal/ports/storage"),
	}

	d := func(from, to string, n int) arch.DepFact {
		return arch.DepFact{FromUnit: from, ToUnit: to, Count: n, File: from + "/mod.go", Line: 7}
	}
	deps := []arch.DepFact{
		d("m_cmd_aoa", "m_app", 2),            // cmd -> app
		d("m_cmd_aoa", "m_adapters_socket", 1), // cmd -> adapters
		d("m_app", "m_domain_searcher", 3),    // app -> domain
		d("m_app", "m_domain_indexer", 1),
		d("m_app", "m_adapters_bbolt", 2),    // app -> adapters
		d("m_app", "m_ports_storage", 1),     // app -> ports
		d("m_domain_searcher", "m_domain_learner", 1),  // cycle
		d("m_domain_learner", "m_domain_enricher", 1),  // cycle
		d("m_domain_enricher", "m_domain_searcher", 1), // cycle closes
		d("m_domain_searcher", "m_ports_storage", 2),  // domain -> ports
		d("m_domain_indexer", "m_ports_storage", 1),
		d("m_adapters_bbolt", "m_ports_storage", 2),  // adapters -> ports
		d("m_adapters_socket", "m_ports_storage", 1),
	}
	return units, deps
}

// cycleEdges are the 3 planted domain import edges that form the SCC. The target
// architecture forbids them, so they show up as drift VIOLATIONs.
var cycleEdges = map[string]bool{
	"m_domain_searcher|m_domain_learner":  true,
	"m_domain_learner|m_domain_enricher":  true,
	"m_domain_enricher|m_domain_searcher": true,
}

// SampleTarget is the authored TARGET world (where-we-need-to-be): every real
// import edge EXCEPT the forbidden cycle, PLUS one declared edge reality hasn't
// built yet. Diffing real against this yields 3 VIOLATIONs (the cycle) and 1
// MISSING (the not-yet-built edge).
func SampleTarget() []TargetFact {
	_, deps := SampleGraph()
	var out []TargetFact
	for _, d := range deps {
		if cycleEdges[d.FromUnit+"|"+d.ToUnit] {
			continue // architect forbids the cycle → real will VIOLATE
		}
		out = append(out, TargetFact{Concept: "import", FromUnit: d.FromUnit, ToUnit: d.ToUnit})
	}
	// A declared edge the code has not built yet → MISSING.
	out = append(out, TargetFact{Concept: "import", FromUnit: "m_cmd_aoa", ToUnit: "m_ports_storage"})
	return out
}

// SampleGraphWithCalls returns the sample graph plus two illustrative :calls
// edges (Kind="calls"), used to prove buildAdj filters by concept. NOTE: these
// call edges are FIXTURE data, not extracted — the ontology marks call as
// recon-not-wired, and Compile refuses to traverse them.
func SampleGraphWithCalls() ([]arch.UnitFact, []arch.DepFact) {
	units, deps := SampleGraph()
	deps = append(deps,
		arch.DepFact{FromUnit: "m_app", ToUnit: "m_domain_searcher", Count: 1, Kind: "calls"},
		arch.DepFact{FromUnit: "m_cmd_aoa", ToUnit: "m_app", Count: 1, Kind: "calls"},
	)
	return units, deps
}
