package app

import (
	"strings"

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

// stripCodeBlocks removes markdown fenced code blocks (```...```) from text,
// preserving only the prose that expresses intent. Code output is not intent
// signal — it's just code. We learn from what the user asks, what the AI
// reasons about, and what the AI explains, not from the code it produces.
func stripCodeBlocks(text string) string {
	var b strings.Builder
	for {
		start := strings.Index(text, "```")
		if start == -1 {
			b.WriteString(text)
			break
		}
		b.WriteString(text[:start])
		// Find closing fence
		rest := text[start+3:]
		end := strings.Index(rest, "```")
		if end == -1 {
			// Unclosed fence — skip everything after it
			break
		}
		text = rest[end+3:]
	}
	return b.String()
}

// conversationProseCap is the maximum prose bytes we tokenize for enricher
// resolution. Intent signal saturates after a few hundred words — the same
// domain concepts repeat. 8KB (~1200 words of prose) is well past saturation
// for any single event while preventing unbounded tokenization of unusually
// long AI responses.
const conversationProseCap = 8192

// processConversationSignal extracts learning signals from conversation text
// (user prompts, AI thinking, AI responses). Strips code blocks first — code
// output is not intent signal. Caps the remaining prose at conversationProseCap
// to avoid tokenizing past intent saturation. Tokenizes through the enricher
// for keyword/term/domain resolution. Unlike processGrepSignal, this does NOT
// call ProcessBigrams (the caller already does that). Must be called with
// a.mu held.
func (a *App) processConversationSignal(text string, observe bool) {
	if text == "" || a.Enricher == nil {
		return
	}

	// Strip code blocks — only prose carries intent signal
	text = stripCodeBlocks(text)

	// Cap prose to avoid tokenizing past intent saturation
	if len(text) > conversationProseCap {
		text = text[:conversationProseCap]
	}

	tokens := index.Tokenize(text)
	if len(tokens) == 0 {
		return
	}

	sc := newSignalCollector()
	for _, tok := range tokens {
		// signalCollector.addKeyword already deduplicates
		sc.addKeyword(tok, a.Enricher)
	}

	// Only emit an observe event if the enricher resolved something
	if len(sc.Keywords) == 0 {
		return
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

	if observe {
		tuneResult := a.Learner.ObserveAndMaybeTune(event)
		if tuneResult != nil && a.Store != nil {
			_ = a.Store.SaveLearnerState(a.ProjectID, a.Learner.State())
			a.writeStatus(tuneResult)
		}
	} else {
		a.Learner.Observe(event)
	}
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
}
