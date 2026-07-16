package arch

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// domainBucketMembersMax caps members per domain bucket (view-standards.json
// global.budgets.bucket_members_max=40 — same shared canvas budget every
// other buckets-kind renderer respects).
const domainBucketMembersMax = 40

// unassignedDomainBucketID is the explicit, always-present bucket for units
// that scored no atlas domain vote at all (D36: never invented, never
// silently dropped).
const unassignedDomainBucketID = "dom_unassigned"

// RenderDomains produces a "buckets" shard answering the Domain map question
// (view-standards.json id "domains"): "What are the business domains, what
// lives in each, and which domains depend on which?"
//
// # Algorithm
//
//  1. Aggregate in.FileDomains (file-grain atlas votes, DOM-1) up to a
//     unit-grain modal vote: for every file under a unit's directory
//     (exact match on UnitFact.Path), tally its domain, then pick the
//     majority; ties broken lexicographically. This is deliberately NOT the
//     bridge's one-file shortcut (aggregateEdges/unitFactsFromFactStore,
//     which credits a unit with whichever single file happened to define it
//     first) — every file in the directory gets a vote.
//  2. Every local (non-"ext:") unit becomes a member of its modal domain's
//     bucket, or the explicit "unassigned" bucket when it scored no vote
//     (D36 — never invented, never dropped).
//  3. Cross-domain dep edges are aggregated by (source domain, dest domain)
//     pair, counts summed; external endpoints are excluded entirely (they
//     are not business domains).
//  4. Each bucket is capped at domainBucketMembersMax (view-standards
//     bucket_members_max), highest fan-in first; the caption states the
//     TRUE total member count alongside the shown count whenever any bucket
//     was truncated (VP-1.p1 truncation-honest pattern — never claim the
//     display budget as fact).
//
// # Provenance
//
// Always MIXED (D36): domain membership is an atlas keyword-scoring
// heuristic, never claimed as a fully derived fact.
func RenderDomains(in RenderInput) (*Shard, error) {
	unitByID := make(map[string]UnitFact, len(in.Units))
	pathToUnit := make(map[string]string, len(in.Units))
	for _, u := range in.Units {
		unitByID[u.ID] = u
		pathToUnit[u.Path] = u.ID
	}

	// 1. Unit-grain modal domain vote — see doc comment above.
	votes := make(map[string]map[string]int)
	for filePath, domain := range in.FileDomains {
		if domain == "" {
			continue
		}
		dir := filepath.Dir(filePath)
		if dir == "." || dir == "" {
			dir = "root"
		}
		id, ok := pathToUnit[dir]
		if !ok {
			continue // file's directory isn't a known unit in this scope — skip, never invent
		}
		if votes[id] == nil {
			votes[id] = make(map[string]int)
		}
		votes[id][domain]++
	}
	unitDomain := make(map[string]string, len(votes))
	for id, domainVotes := range votes {
		best, bestCount := "", 0
		for d, c := range domainVotes {
			if c > bestCount || (c == bestCount && (best == "" || d < best)) {
				best, bestCount = d, c
			}
		}
		// DeriveFileDomains stamps its winner "@domain" (atlas convention,
		// enrich.go:assignDomainByKeywords); strip it here so bucket
		// IDs/labels are clean everywhere downstream (matches roleFor's own
		// TrimPrefix(label, "@") convention).
		unitDomain[id] = strings.TrimPrefix(best, "@")
	}

	// Fan-in per unit (for member sub: "in N", matches RenderComponent).
	fanIn := make(map[string]int, len(in.Units))
	for _, d := range in.Deps {
		fanIn[d.ToUnit]++
	}

	// 2. Bucket every local unit by its modal domain (or "unassigned").
	unitBucket := make(map[string]string, len(in.Units)) // unitID -> bucketID
	membersByBucket := make(map[string][]Member)
	labelByBucket := make(map[string]string)
	for _, u := range in.Units {
		if isExternalUnit(u.ID, unitByID) {
			continue
		}
		bucketID := unassignedDomainBucketID
		domain := unitDomain[u.ID]
		if domain != "" {
			bucketID = "dom_" + slugify(domain)
			labelByBucket[bucketID] = domain
		}
		unitBucket[u.ID] = bucketID

		sub := ""
		if fi := fanIn[u.ID]; fi > 0 {
			sub = fmt.Sprintf("in %d", fi)
		}
		membersByBucket[bucketID] = append(membersByBucket[bucketID], Member{
			ID:      u.ID,
			Label:   truncate(u.Label, 26), // member budget: ≤26 chars
			Sub:     sub,
			Sources: []SourceRef{{File: u.File, Line: u.Line}},
		})
	}

	// 3. Cross-domain edges, external endpoints excluded.
	type edgeKey struct{ src, dst string }
	edgeCounts := make(map[edgeKey]int)
	for _, d := range in.Deps {
		srcBucket, srcOK := unitBucket[d.FromUnit]
		dstBucket, dstOK := unitBucket[d.ToUnit]
		if !srcOK || !dstOK || srcBucket == dstBucket {
			continue
		}
		edgeCounts[edgeKey{srcBucket, dstBucket}] += d.Count
	}
	var edges []ShardEdge
	for k, cnt := range edgeCounts {
		edges = append(edges, ShardEdge{
			ID:     fmt.Sprintf("e_%s_%s", k.src, k.dst),
			Source: k.src,
			Target: k.dst,
			Count:  cnt,
		})
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })

	// 4. Assemble buckets: domains sorted alphabetically by label, unassigned
	// always last. Members sorted fan-in desc then label asc, capped at
	// domainBucketMembersMax (truncation tracked for the honest caption).
	bucketIDs := make([]string, 0, len(membersByBucket))
	for id := range membersByBucket {
		if id != unassignedDomainBucketID {
			bucketIDs = append(bucketIDs, id)
		}
	}
	sort.Strings(bucketIDs)
	if _, ok := membersByBucket[unassignedDomainBucketID]; ok {
		bucketIDs = append(bucketIDs, unassignedDomainBucketID)
	}

	trueMemberTotal, shownMemberTotal := 0, 0
	buckets := make([]Bucket, 0, len(bucketIDs))
	for part, id := range bucketIDs {
		members := membersByBucket[id]
		sort.Slice(members, func(i, j int) bool {
			fi, fj := fanIn[members[i].ID], fanIn[members[j].ID]
			if fi != fj {
				return fi > fj
			}
			return members[i].Label < members[j].Label
		})
		trueMemberTotal += len(members)
		if len(members) > domainBucketMembersMax {
			members = members[:domainBucketMembersMax]
		}
		shownMemberTotal += len(members)

		label := "Unassigned"
		layer, ico := "", ""
		if id != unassignedDomainBucketID {
			label = domainDisplayLabel(labelByBucket[id])
			layer, ico = roleFor(labelByBucket[id])
		}
		buckets = append(buckets, Bucket{
			ID:       id,
			Layer:    layer,
			Ico:      ico,
			Label:    truncate(label, 30), // node budget: ≤30 chars
			Part:     part,
			Inferred: true, // atlas keyword vote (INFERRED) — never a declared contract (D36)
			Members:  members,
		})
	}

	prov := Prov{Kind: "mixed", Label: "MIXED · domains seeded from atlas token scoring (modal vote per unit)"}
	shard := &Shard{
		Kind:    "buckets",
		Title:   "Domain map",
		Dir:     "DOWN",
		Prov:    prov,
		Buckets: buckets,
		Edges:   edges,
	}
	_, shard.FindingsClause = DeriveCaption(shard, in.Findings)

	nDomains := len(buckets)
	if _, ok := membersByBucket[unassignedDomainBucketID]; ok {
		nDomains-- // "unassigned" is a bookkeeping bucket, not a business domain
	}
	if trueMemberTotal > shownMemberTotal {
		// Budget truncated the drawing — the caption must say so honestly:
		// true total + shown subset, never claim the shown count as fact
		// (VP-1.p1 truncation-honest pattern).
		shard.Count = fmt.Sprintf("%d domains · %d members (showing %d)", nDomains, trueMemberTotal, shownMemberTotal)
	} else {
		shard.Count = fmt.Sprintf("%d domains · %d members", nDomains, trueMemberTotal)
	}
	return shard, nil
}

// domainDisplayLabel turns an atlas domain slug (e.g. "build_system") into a
// display label (e.g. "Build system") — underscores become spaces, first
// word capitalized.
func domainDisplayLabel(domain string) string {
	words := strings.Split(strings.ReplaceAll(domain, "_", " "), " ")
	if len(words) > 0 && words[0] != "" {
		r := []rune(words[0])
		r[0] = unicode.ToUpper(r[0])
		words[0] = string(r)
	}
	return strings.Join(words, " ")
}
