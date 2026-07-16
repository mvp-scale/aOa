package arch

import (
	"fmt"
	"sort"
	"strings"
)

// deploymentBucketMembersMax caps members per deployment bucket
// (view-standards.json global.budgets.bucket_members_max=40 — same shared
// canvas budget every other buckets-kind renderer respects, e.g.
// domainBucketMembersMax).
const deploymentBucketMembersMax = 40

// RenderDeployment produces a "buckets" shard answering the Deployment (view
// id "deployment") question (view-standards.json: "What runs where, and what
// crosses environment boundaries?").
//
// V1 honestly answers a weaker question than the one asked: no reader
// derives environment/zone grain (a compose.yaml or a Kubernetes manifest
// names services/workloads, never "staging" vs "prod") — so buckets group by
// DEPLOY SURFACE instead (container/Docker vs Kubernetes), and the caption
// says plainly what ships rather than claiming an environment topology this
// v1 cannot see. This is the same "documented weaker cut, never a silent
// fabrication" discipline as RenderDataModel's no-FK-detection scope.
//
// Cross-bucket (cross-surface) edges are never invented: nothing here tries
// to correlate a compose service with a same-named Kubernetes workload as
// "the same thing" — that would be a guess, not a derivation, so v1 emits no
// deployment edges at all. The one real, literal relationship this reader
// does have — a compose service's depends_on — is same-surface information,
// not a cross-environment flow, so it is surfaced as member hover detail
// (Member.Sub) rather than fabricating a bucket-to-bucket edge that isn't
// the relationship it would visually claim to be.
//
// Provenance: always "derived"/REAL (D2/D15) — kind/name/image/ports/
// depends_on are read straight off the manifest, same honesty tier as route
// and entity extraction (a syntactic/schema match, no cross-file resolution).
func RenderDeployment(in RenderInput) (*Shard, error) {
	type bucketDef struct {
		id, label string
	}
	defs := []bucketDef{
		{"b_container", "Container (Docker)"},
		{"b_kubernetes", "Kubernetes"},
	}

	membersByBucket := make(map[string][]Member, len(defs))
	trueTotal := 0
	for _, e := range in.Deployments {
		bucketID := deploymentBucketFor(e.Kind)
		trueTotal++

		sub := ""
		if len(e.DependsOn) > 0 {
			sub = "depends on " + strings.Join(e.DependsOn, ", ")
		}
		membersByBucket[bucketID] = append(membersByBucket[bucketID], Member{
			ID:      e.ID,
			Label:   truncate(e.ID, 26), // member budget: ≤26 chars
			Sub:     sub,
			Sources: []SourceRef{{File: e.File, Line: e.Line}},
		})
	}

	shownTotal := 0
	var buckets []Bucket
	for part, def := range defs {
		members, ok := membersByBucket[def.id]
		if !ok {
			continue // no artifacts on this surface — no phantom bucket
		}
		sort.Slice(members, func(i, j int) bool { return members[i].Label < members[j].Label })
		if len(members) > deploymentBucketMembersMax {
			members = members[:deploymentBucketMembersMax]
		}
		shownTotal += len(members)
		buckets = append(buckets, Bucket{
			ID:      def.id,
			Label:   def.label,
			Part:    part,
			Members: members,
		})
	}

	prov := Prov{Kind: "derived", Label: "REAL · Dockerfile/compose.yaml/Kubernetes-manifest extraction"}
	shard := &Shard{
		Kind:  "buckets",
		Title: "Deployment",
		Dir:   "DOWN",
		Prov:  prov,
		// Buckets/Edges left nil-safe (omitempty): a repo with no IaC gets an
		// honest empty shard, not a fabricated one.
		Buckets: buckets,
	}
	_, shard.FindingsClause = DeriveCaption(shard, in.Findings)

	switch {
	case trueTotal == 0:
		shard.Count = "0 deploy artifacts"
	case trueTotal > shownTotal:
		shard.Count = fmt.Sprintf("%d artifacts ship (showing %d)", trueTotal, shownTotal)
	default:
		shard.Count = fmt.Sprintf("%d artifacts ship", trueTotal)
	}
	return shard, nil
}

// deploymentBucketFor buckets a DeploymentEntry by deploy surface: every
// "k8s-*" kind groups under Kubernetes, everything else (dockerfile,
// compose-service) groups under Container/Docker.
func deploymentBucketFor(kind string) string {
	if strings.HasPrefix(kind, "k8s-") {
		return "b_kubernetes"
	}
	return "b_container"
}
