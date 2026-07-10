package learner

import "strings"

// BuildWinnerMap is the single, shared election the products consume: for each
// term, the one domain this project binds it to. It re-applies an EXPLICIT per-term
// total-count floor — necessary because runDedup only contests terms that appear in
// 2+ domains, so a single-domain cohit key survives at count=1 and would otherwise
// look like a "winner". The floor keeps weak/noise terms out of what grep and the
// graph read. Pure and deterministic (highest count wins; ties break lexicographically).
//
func BuildWinnerMap(cohitTermDomain map[string]uint32, floor uint32) map[string]string {
	type domCount struct {
		domain string
		count  uint32
	}
	byTerm := make(map[string][]domCount)
	total := make(map[string]uint32)
	for key, count := range cohitTermDomain {
		i := strings.Index(key, ":")
		if i <= 0 || i >= len(key)-1 {
			continue
		}
		term, domain := key[:i], key[i+1:]
		byTerm[term] = append(byTerm[term], domCount{domain, count})
		total[term] += count
	}

	winners := make(map[string]string, len(byTerm))
	for term, dcs := range byTerm {
		if total[term] < floor {
			continue // explicit per-term floor — closes the runDedup single-domain gap
		}
		best := dcs[0]
		for _, d := range dcs[1:] {
			if d.count > best.count || (d.count == best.count && d.domain < best.domain) {
				best = d
			}
		}
		winners[term] = best.domain
	}
	return winners
}
