package learner

import (
	"sort"
	"strings"
)

// runDedup performs cohit dedup on a map (cohit_kw_term or cohit_term_domain).
// Matches Python's run_dedup() and the DEDUP_LUA behavior.
//
// Algorithm:
//  1. Group entries by entity (left side of colon)
//  2. Filter to entities appearing in 2+ containers
//  3. Filter to entities with total cohits >= DedupMinTotal (100)
//  4. For each qualifying entity: winner = highest count, losers removed
func runDedup(cohitMap map[string]uint32) {
	// Group by entity (left side of colon)
	type containerEntry struct {
		name  string
		count uint32
	}
	entityContainers := make(map[string][]containerEntry)

	for key, count := range cohitMap {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		entity := parts[0]
		container := parts[1]
		entityContainers[entity] = append(entityContainers[entity], containerEntry{container, count})
	}

	// Process entities with 2+ containers and total >= DedupMinTotal
	for entity, containers := range entityContainers {
		if len(containers) < 2 {
			continue
		}

		var total uint32
		for _, c := range containers {
			total += c.count
		}
		if total < DedupMinTotal {
			continue
		}

		// Sort by count desc, then alphabetically for tie-breaking
		sort.Slice(containers, func(i, j int) bool {
			if containers[i].count != containers[j].count {
				return containers[i].count > containers[j].count
			}
			return containers[i].name < containers[j].name
		})

		// Winner is first (highest count). Remove all losers.
		for _, loser := range containers[1:] {
			loserKey := entity + ":" + loser.name
			delete(cohitMap, loserKey)
		}
	}
}
