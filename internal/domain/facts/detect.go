// FDN-3 (board #29), D27: compact-time detectors (§4.4). Run once per
// compaction, over the DepAdjacency Compact just built — never at render
// time (ENHANCEMENT-GUIDE §4). Output is FactFinding facts, keyed by
// (rule, subject) at the store layer (facts_findings, bbolt/facts.go
// PutFindings).
//
// These detectors are independent of internal/domain/arch's TarjanSCC/
// DetectGods/DetectOrphans/DetectDeadCandidates (which operate on arch's own
// UnitFact/DepFact/Finding types, not ports.Fact/DepAdjacency): domain/facts
// must stay dependency-free per hexagonal law (CLAUDE.md — "depend on ports
// interfaces only"), and importing internal/domain/arch here would risk a
// future import cycle once FDN-4 reconciles arch's provisional UnitFact/
// DepFact with ports.Fact (arch's own TODO comment, model.go:6-8). The
// Tarjan implementation below is intentionally a fresh, small copy of the
// same well-tested algorithm, not a shared abstraction.
package facts

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/corey/aoa/internal/ports"
)

// Rule names for FactFinding facts (§4.4 table).
const (
	RuleCycle         = "cycle"
	RuleGodUnit       = "god_unit"
	RuleOrphan        = "orphan"
	RuleDeadCandidate = "dead_candidate"
	RuleBrokenImport  = "broken_import"
)

// DefaultGodThreshold is the default fan-in AND fan-out floor for the
// god_unit detector (§4.4: "fan-in + fan-out > threshold (default 12/12,
// configurable)" — read as two thresholds, matching arch.DetectGods' prior
// art: both fan-in and fan-out must clear the bar).
const DefaultGodThreshold = 12

// maxBrokenImportSpecs bounds how many distinct broken specs are listed per
// subject in one finding's attrs (Attrs is small and bounded, §1.1).
const maxBrokenImportSpecs = 5

// Detect runs every compact-time detector over units/adj (Compact's output)
// plus the raw dep facts that failed resolution (unresolved, §2.4). refHits
// is not measured at compact time in the current wiring (symbol-reference
// counting lives with the search index, an app-layer concern facts cannot
// reach) so dead_candidate is computed via DetectWithRefs(..., nil), which
// honestly emits no dead_candidate findings rather than mislabeling every
// orphan (scope law: never fabricate a measurement that wasn't taken).
func Detect(units []ports.Fact, adj *ports.DepAdjacency, unresolved []ports.Fact) []ports.Fact {
	return DetectWithRefs(units, adj, unresolved, nil)
}

// DetectWithRefs is Detect with an optional unit -> index-reference-count
// map for the dead_candidate detector. refHits == nil means "not measured"
// (dead_candidate emits nothing); a non-nil map (even if empty) means
// "measured, zero is a real zero" — same honesty contract as
// arch.DetectDeadCandidates.
func DetectWithRefs(units []ports.Fact, adj *ports.DepAdjacency, unresolved []ports.Fact, refHits map[string]int) []ports.Fact {
	var findings []ports.Fact
	findings = append(findings, detectCycles(units, adj)...)
	findings = append(findings, detectGodUnits(units, adj, DefaultGodThreshold)...)
	orphans := detectOrphans(units, adj)
	findings = append(findings, orphans...)
	findings = append(findings, detectDeadCandidates(orphans, refHits)...)
	findings = append(findings, detectBrokenImports(unresolved)...)
	return findings
}

// tarjanSCC computes strongly connected components of a directed graph.
// adj maps node ID -> neighbor IDs; nodes absent from adj (referenced only
// as a neighbor, e.g. an "ext:" leaf) are treated as having no out-edges.
// Deterministic: node/neighbor traversal and SCC membership are sorted.
func tarjanSCC(adj map[string][]string) [][]string {
	index := 0
	var stack []string
	onStack := map[string]bool{}
	indices := map[string]int{}
	lowlink := map[string]int{}
	var sccs [][]string

	nodes := make([]string, 0, len(adj))
	for n := range adj {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)

	var strongconnect func(v string)
	strongconnect = func(v string) {
		indices[v] = index
		lowlink[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		neighbors := append([]string(nil), adj[v]...)
		sort.Strings(neighbors)
		for _, w := range neighbors {
			if _, seen := indices[w]; !seen {
				strongconnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] {
				if indices[w] < lowlink[v] {
					lowlink[v] = indices[w]
				}
			}
		}

		if lowlink[v] == indices[v] {
			var scc []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			if len(scc) > 1 {
				sort.Strings(scc)
				sccs = append(sccs, scc)
			}
		}
	}

	for _, n := range nodes {
		if _, seen := indices[n]; !seen {
			strongconnect(n)
		}
	}
	sort.Slice(sccs, func(i, j int) bool { return sccs[i][0] < sccs[j][0] })
	return sccs
}

// unitPath strips the "<ns>:" prefix from a canonical unit ID.
func unitPath(id string) string {
	if i := strings.Index(id, ":"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// isEntrypoint reports whether a unit looks like a program entrypoint or a
// test-only unit — both are expected to have zero inbound deps and must not
// be flagged orphan (§4.4: "not an entrypoint glob (cmd/**, main.*, tests)").
func isEntrypoint(unitID string) bool {
	p := unitPath(unitID)
	if p == "cmd" || strings.HasPrefix(p, "cmd/") {
		return true
	}
	base := p
	if i := strings.LastIndex(p, "/"); i >= 0 {
		base = p[i+1:]
	}
	if base == "main" || strings.HasPrefix(base, "main_") || strings.HasSuffix(base, "_main") {
		return true
	}
	if strings.HasSuffix(base, "_test") || base == "test" || base == "tests" ||
		strings.HasPrefix(p, "test/") || strings.Contains(p, "/test/") ||
		strings.HasPrefix(p, "tests/") || strings.Contains(p, "/tests/") {
		return true
	}
	return false
}

func sortedUnits(units []ports.Fact) []ports.Fact {
	sorted := append([]ports.Fact(nil), units...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Subject < sorted[j].Subject })
	return sorted
}

func detectCycles(units []ports.Fact, adj *ports.DepAdjacency) []ports.Fact {
	if adj == nil {
		return nil
	}
	unitSrc := make(map[string]ports.FactSource, len(units))
	graph := make(map[string][]string, len(units))
	for _, u := range units {
		unitSrc[u.Subject] = u.Source
		graph[u.Subject] = nil
	}
	for subj, edges := range adj.Fwd {
		for _, e := range edges {
			graph[subj] = append(graph[subj], e.Unit)
		}
	}

	var findings []ports.Fact
	for _, scc := range tarjanSCC(graph) {
		members := append([]string(nil), scc...)
		sort.Strings(members)
		findings = append(findings, ports.Fact{
			Kind:    ports.FactFinding,
			Subject: members[0],
			Attrs: map[string]string{
				"rule":    RuleCycle,
				"members": strings.Join(members, ","),
				"size":    strconv.Itoa(len(members)),
			},
			Source: unitSrc[members[0]],
			Prov:   ports.ProvDerived,
			TS:     time.Now().Unix(),
		})
	}
	return findings
}

func detectGodUnits(units []ports.Fact, adj *ports.DepAdjacency, threshold int) []ports.Fact {
	fanIn := map[string]int{}
	fanOut := map[string]int{}
	if adj != nil {
		for subj, edges := range adj.Fwd {
			fanOut[subj] = len(edges)
		}
		for subj, edges := range adj.Rev {
			fanIn[subj] = len(edges)
		}
	}

	var findings []ports.Fact
	for _, u := range sortedUnits(units) {
		in, out := fanIn[u.Subject], fanOut[u.Subject]
		if in < threshold || out < threshold {
			continue
		}
		findings = append(findings, ports.Fact{
			Kind:    ports.FactFinding,
			Subject: u.Subject,
			Attrs: map[string]string{
				"rule":      RuleGodUnit,
				"fan_in":    strconv.Itoa(in),
				"fan_out":   strconv.Itoa(out),
				"threshold": strconv.Itoa(threshold),
			},
			Source: u.Source,
			Prov:   ports.ProvDerived,
			TS:     time.Now().Unix(),
		})
	}
	return findings
}

func detectOrphans(units []ports.Fact, adj *ports.DepAdjacency) []ports.Fact {
	hasInbound := map[string]bool{}
	if adj != nil {
		for subj, edges := range adj.Rev {
			if len(edges) > 0 {
				hasInbound[subj] = true
			}
		}
	}

	var findings []ports.Fact
	for _, u := range sortedUnits(units) {
		if hasInbound[u.Subject] || isEntrypoint(u.Subject) {
			continue
		}
		findings = append(findings, ports.Fact{
			Kind:    ports.FactFinding,
			Subject: u.Subject,
			Attrs:   map[string]string{"rule": RuleOrphan},
			Source:  u.Source,
			Prov:    ports.ProvDerived,
			TS:      time.Now().Unix(),
		})
	}
	return findings
}

// detectDeadCandidates narrows orphans to units with zero measured symbol
// references. refHits == nil ("not measured") yields no findings — see
// DetectWithRefs' doc comment on why silence beats a mislabeled duplicate.
func detectDeadCandidates(orphans []ports.Fact, refHits map[string]int) []ports.Fact {
	if refHits == nil {
		return nil
	}
	var findings []ports.Fact
	for _, o := range orphans {
		if refHits[o.Subject] > 0 {
			continue
		}
		findings = append(findings, ports.Fact{
			Kind:    ports.FactFinding,
			Subject: o.Subject,
			Attrs:   map[string]string{"rule": RuleDeadCandidate, "refs": "0"},
			Source:  o.Source,
			Prov:    ports.ProvDerived,
			TS:      time.Now().Unix(),
		})
	}
	return findings
}

// detectBrokenImports groups unresolved raw dep facts by importing subject —
// one finding per subject (avoids a facts_findings key collision, since the
// store keys findings by rule\x00subject; multiple broken specs from the
// same file collapse into one finding listing up to maxBrokenImportSpecs).
func detectBrokenImports(unresolved []ports.Fact) []ports.Fact {
	if len(unresolved) == 0 {
		return nil
	}
	bySubject := make(map[string][]ports.Fact)
	order := make([]string, 0, len(unresolved))
	for _, f := range unresolved {
		if _, ok := bySubject[f.Subject]; !ok {
			order = append(order, f.Subject)
		}
		bySubject[f.Subject] = append(bySubject[f.Subject], f)
	}
	sort.Strings(order)

	var findings []ports.Fact
	for _, subj := range order {
		items := append([]ports.Fact(nil), bySubject[subj]...)
		sort.Slice(items, func(i, j int) bool {
			if items[i].Source.File != items[j].Source.File {
				return items[i].Source.File < items[j].Source.File
			}
			return items[i].Source.Line < items[j].Source.Line
		})
		specs := make([]string, 0, len(items))
		for _, it := range items {
			specs = append(specs, it.Attrs["spec"])
		}
		count := len(specs)
		if len(specs) > maxBrokenImportSpecs {
			specs = specs[:maxBrokenImportSpecs]
		}
		findings = append(findings, ports.Fact{
			Kind:    ports.FactFinding,
			Subject: subj,
			Attrs: map[string]string{
				"rule":  RuleBrokenImport,
				"specs": strings.Join(specs, ","),
				"count": strconv.Itoa(count),
			},
			Source: items[0].Source,
			Prov:   ports.ProvDerived,
			TS:     time.Now().Unix(),
		})
	}
	return findings
}
