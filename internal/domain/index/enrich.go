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

// generateTags returns atlas terms for a symbol by resolving its tokens
// through the keyword→term reverse lookup. Tags are terms, not raw keywords.
func (e *SearchEngine) generateTags(ref ports.TokenRef) []string {
	tokens := e.refToTokens[ref]
	return e.resolveTerms(tokens)
}

// Token exclusion set for tag generation.
var excludeTokens = map[string]bool{
	"self": true,
	"base": true,
	"case": true,
}

// resolveTerms converts raw tokens (keywords) to atlas terms via the
// keyword→term reverse lookup. Returns up to 3 unique terms, sorted
// alphabetically for deterministic output.
func (e *SearchEngine) resolveTerms(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var terms []string
	for _, tok := range tokens {
		if excludeTokens[tok] {
			continue
		}
		for _, term := range e.keywordToTerms[strings.ToLower(tok)] {
			if !seen[term] {
				seen[term] = true
				terms = append(terms, term)
			}
		}
	}

	if len(terms) == 0 {
		return nil
	}

	sort.Strings(terms)

	if len(terms) > 3 {
		terms = terms[:3]
	}
	return terms
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
