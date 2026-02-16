package index

import (
	"sort"
	"strings"

	"github.com/corey/aoa/internal/ports"
)

// assignDomain returns the domain for a symbol.
// Uses the file-level domain if set, otherwise falls back to keyword overlap scoring.
func (e *SearchEngine) assignDomain(ref ports.TokenRef) string {
	file := e.idx.Files[ref.FileID]
	if file != nil && file.Domain != "" {
		return file.Domain
	}

	// Fallback: keyword overlap scoring
	return e.assignDomainByKeywords(ref)
}

// assignDomainByKeywords picks the best-matching domain based on token overlap.
// Returns the domain name with "@" prefix (matching file-level domain convention).
func (e *SearchEngine) assignDomainByKeywords(ref ports.TokenRef) string {
	symTokens := e.refTokenSet(ref)
	if len(symTokens) == 0 {
		return ""
	}

	bestDomain := ""
	bestScore := 0

	domainNames := make([]string, 0, len(e.domains))
	for name := range e.domains {
		domainNames = append(domainNames, name)
	}
	sort.Strings(domainNames)

	for _, domainName := range domainNames {
		domain := e.domains[domainName]
		score := 0
		for _, keywords := range domain.Terms {
			for _, kw := range keywords {
				if symTokens[strings.ToLower(kw)] {
					score++
				}
			}
		}
		if score > bestScore {
			bestScore = score
			bestDomain = domainName
		}
	}

	if bestDomain != "" && !strings.HasPrefix(bestDomain, "@") {
		bestDomain = "@" + bestDomain
	}

	return bestDomain
}

// generateTags returns the tags for a symbol.
// Uses pre-computed tags from SymbolMeta if available,
// otherwise falls back to token-based generation.
func (e *SearchEngine) generateTags(ref ports.TokenRef) []string {
	sym := e.idx.Metadata[ref]
	if sym != nil && len(sym.Tags) > 0 {
		return sym.Tags
	}

	// Fallback: use tokens from inverted index
	return e.generateTagsFromTokens(ref)
}

// Token exclusion set for fallback tag generation.
var excludeTokens = map[string]bool{
	"self": true,
	"base": true,
	"case": true,
}

// generateTagsFromTokens generates tags from the inverted index tokens.
func (e *SearchEngine) generateTagsFromTokens(ref ports.TokenRef) []string {
	tokens := e.refToTokens[ref]
	if len(tokens) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var candidates []string
	for _, tok := range tokens {
		if excludeTokens[tok] || seen[tok] {
			continue
		}
		seen[tok] = true
		candidates = append(candidates, tok)
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		fi := e.tokenDocFreq[candidates[i]]
		fj := e.tokenDocFreq[candidates[j]]
		if fi != fj {
			return fi < fj
		}
		return candidates[i] < candidates[j]
	})

	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return candidates
}

// refTokenSet returns the set of tokens for a ref from the reverse map.
func (e *SearchEngine) refTokenSet(ref ports.TokenRef) map[string]bool {
	tokens := e.refToTokens[ref]
	set := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		set[t] = true
	}
	return set
}
