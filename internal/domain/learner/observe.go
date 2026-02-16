package learner

import (
	"fmt"

	"github.com/corey/aoa-go/internal/ports"
)

// ObserveEvent is a single observe signal from an intent.
// Matches the JSON fixture format: keywords, terms, domains, keyword_terms, term_domains.
type ObserveEvent struct {
	PromptNumber uint32     `json:"prompt_number"`
	Observe      ObserveData `json:"observe"`
	FileRead     *FileRead  `json:"file_read,omitempty"`
}

// ObserveData contains the signal data within an observe event.
type ObserveData struct {
	Keywords     []string    `json:"keywords"`
	Terms        []string    `json:"terms"`
	Domains      []string    `json:"domains"`
	KeywordTerms [][2]string `json:"keyword_terms"`
	TermDomains  [][2]string `json:"term_domains"`
}

// FileRead describes a file read event.
type FileRead struct {
	File   string `json:"file"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

// Observe applies a single observe event to the learner state.
// This matches Python's apply_observe() exactly.
//
// Signal processing order:
//  1. keywords → keyword_hits[kw] += 1
//  2. terms → term_hits[term] += 1
//  3. domains → domain_meta[d].hits += 1, total_hits += 1, last_hit_at = prompt
//  4. keyword_terms → keyword_hits[kw] += 1, term_hits[term] += 1, cohit_kw_term[kw:term] += 1
//  5. term_domains → cohit_term_domain[term:domain] += 1
//  6. file_read → file_hits[file] += 1
//  7. prompt_count = prompt_number
//
// Note: keyword_terms ALSO increments keyword_hits and term_hits (double counting per Python spec).
func (l *Learner) Observe(event ObserveEvent) {
	obs := event.Observe
	prompt := event.PromptNumber

	// 1. Increment keyword_hits
	for _, kw := range obs.Keywords {
		l.state.KeywordHits[kw]++
	}

	// 2. Increment term_hits
	for _, term := range obs.Terms {
		l.state.TermHits[term]++
	}

	// 3. Increment domain hits
	for _, domain := range obs.Domains {
		dm := l.state.DomainMeta[domain]
		if dm == nil {
			// Create learned domain on first encounter
			dm = &ports.DomainMeta{
				Tier:      "context",
				Source:    "learned",
				State:     "active",
				CreatedAt: 1739500000 + int64(prompt), // Offset for learned domains
			}
			l.state.DomainMeta[domain] = dm
		}
		dm.Hits += 1.0
		dm.TotalHits++
		dm.LastHitAt = prompt
	}

	// 4. Increment cohit_kw_term (also increments keyword_hits and term_hits)
	for _, pair := range obs.KeywordTerms {
		kw, term := pair[0], pair[1]
		key := fmt.Sprintf("%s:%s", kw, term)
		l.state.CohitKwTerm[key]++
		l.state.KeywordHits[kw]++
		l.state.TermHits[term]++
	}

	// 5. Increment cohit_term_domain
	for _, pair := range obs.TermDomains {
		term, domain := pair[0], pair[1]
		key := fmt.Sprintf("%s:%s", term, domain)
		l.state.CohitTermDomain[key]++
	}

	// 6. Increment file_hits
	if event.FileRead != nil {
		l.state.FileHits[event.FileRead.File]++
	}

	// 7. Update prompt count
	l.state.PromptCount = prompt
}

// ObserveAndMaybeTune applies an observe event and runs autotune if
// the prompt count hits the autotune interval boundary.
// Returns non-nil AutotuneResult if autotune ran (caller should persist state).
func (l *Learner) ObserveAndMaybeTune(event ObserveEvent) *AutotuneResult {
	l.Observe(event)
	if l.state.PromptCount > 0 && l.state.PromptCount%AutotuneInterval == 0 {
		result := l.RunMathTune()
		return &result
	}
	return nil
}
