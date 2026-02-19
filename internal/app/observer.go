package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/corey/aoa/internal/domain/index"
	"github.com/corey/aoa/internal/domain/learner"
)

// hitSignals is an enricher-free signal container extracted directly from
// search result hits. Domains and terms come from the index metadata, not
// from enricher re-resolution.
type hitSignals struct {
	Domains     []string
	Terms       []string
	TermDomains [][2]string
	ContentText []string
}

// collectHitSignals iterates hits and extracts domains, terms, term-domain
// pairs, and content text without any enricher dependency. Deduplicates
// domains and terms. Processes at most topN hits.
func collectHitSignals(hits []index.Hit, topN int) hitSignals {
	if topN > len(hits) {
		topN = len(hits)
	}

	var sig hitSignals
	seenDomains := make(map[string]bool)
	seenTerms := make(map[string]bool)

	for _, hit := range hits[:topN] {
		// Extract domain (strip "@" prefix)
		domain := strings.TrimPrefix(hit.Domain, "@")
		if domain != "" && !seenDomains[domain] {
			seenDomains[domain] = true
			sig.Domains = append(sig.Domains, domain)
		}

		// Extract tags as terms, pair each with this hit's domain
		for _, term := range hit.Tags {
			if !seenTerms[term] {
				seenTerms[term] = true
				sig.Terms = append(sig.Terms, term)
			}
			if domain != "" {
				sig.TermDomains = append(sig.TermDomains, [2]string{term, domain})
			}
		}

		// Collect content hit text for bigram processing
		if hit.Kind == "content" && hit.Content != "" {
			sig.ContentText = append(sig.ContentText, hit.Content)
		}
	}

	return sig
}

// processGrepSignal generates learning signals from a session-log Grep pattern.
// Tokenizes the pattern through the enricher for keyword/term/domain resolution,
// then feeds the pattern text to ProcessBigrams. Pushes a Learn activity with
// "(grep)" suffix. Must be called with a.mu held.
func (a *App) processGrepSignal(pattern string) {
	if pattern == "" {
		return
	}

	tokens := index.Tokenize(pattern)
	if len(tokens) == 0 {
		return
	}

	sc := newSignalCollector()
	for _, tok := range tokens {
		sc.addKeyword(tok, a.Enricher)
	}

	event := learner.ObserveEvent{
		PromptNumber: a.promptN,
		Observe: learner.ObserveData{
			Keywords:     sc.Keywords,
			Terms:        sc.Terms,
			Domains:      sc.Domains,
			KeywordTerms: sc.KwTerms,
			TermDomains:  sc.TermDomains,
		},
	}

	tuneResult := a.Learner.ObserveAndMaybeTune(event)
	if tuneResult != nil && a.Store != nil {
		_ = a.Store.SaveLearnerState(a.ProjectID, a.Learner.State())
		a.writeStatus(tuneResult)
	}

	a.Learner.ProcessBigrams(pattern)

	if len(sc.Keywords) > 0 || len(sc.Terms) > 0 || len(sc.Domains) > 0 {
		a.pushActivity(ActivityEntry{
			Action:    "Learn",
			Source:    "aOa",
			Impact:    fmt.Sprintf("+%d keywords, +%d terms, +%d domains (grep)", len(sc.Keywords), len(sc.Terms), len(sc.Domains)),
			Timestamp: time.Now().Unix(),
		})
	}
}
