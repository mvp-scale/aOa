package learner

import (
	"regexp"
	"strings"
)

// BigramThreshold is the minimum accumulated count before a bigram
// is promoted from internal staging to the persistent Bigrams map.
// Matches Python's threshold gate (count >= 6).
const BigramThreshold uint32 = 6

// bigramRE matches tokens for bigram extraction.
// Matches Python: re.findall(r'\b[a-z][a-z0-9_]+\b', text.lower())
//
// Rules:
//   - First char must be [a-z]
//   - Remaining chars: [a-z0-9_]+
//   - Minimum 2 characters total
//   - Underscore is inside \w, so "auth_handler" is one token
var bigramRE = regexp.MustCompile(`\b[a-z][a-z0-9_]+\b`)

// stopWords are common English function words excluded from bigram formation.
// These carry no domain signal in conversation text. Kept conservative —
// only words that are unambiguously non-technical.
var stopWords = map[string]bool{
	// Be/have/do auxiliaries
	"are": true, "was": true, "were": true, "been": true,
	"being": true, "have": true, "has": true, "had": true,
	"does": true, "did": true,
	// Modal verbs
	"will": true, "would": true, "could": true, "should": true,
	"might": true, "shall": true, "can": true,
	// Determiners/pronouns
	"the": true, "this": true, "that": true, "these": true,
	"those": true, "which": true, "what": true, "when": true,
	"where": true, "who": true, "whom": true, "whose": true,
	"how": true,
	// Conjunctions/prepositions
	"and": true, "but": true, "not": true, "nor": true,
	"for": true, "with": true, "from": true, "into": true,
	"about": true, "than": true, "onto": true, "over": true,
	"is": true, "it": true, "in": true, "on": true, "to": true,
	"as": true, "at": true, "by": true, "or": true, "if": true,
	"so": true, "no": true, "do": true, "up": true, "an": true,
	// Adverbs
	"also": true, "just": true, "only": true, "very": true,
	"more": true, "most": true, "then": true, "here": true,
	"there": true,
	// Pronouns
	"you": true, "your": true, "they": true, "them": true,
	"their": true, "our": true, "its": true, "she": true,
	"her": true, "his": true, "him": true,
	// Quantifiers
	"some": true, "such": true, "each": true, "every": true,
	"all": true, "both": true, "few": true, "other": true,
}

// bigramTokenize extracts tokens from text for bigram formation.
// Matches Python's re.findall(r'\b[a-z][a-z0-9_]+\b', text.lower()).
// Stop words are filtered out.
func bigramTokenize(text string) []string {
	lower := strings.ToLower(text)
	matches := bigramRE.FindAllString(lower, -1)

	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if !stopWords[m] {
			result = append(result, m)
		}
	}
	return result
}

// ExtractBigrams extracts adjacent word pairs from text.
// Returns a map of "word1:word2" -> count within the text.
// Returns nil if fewer than 2 tokens remain after stop word filtering.
func ExtractBigrams(text string) map[string]uint32 {
	words := bigramTokenize(text)
	if len(words) < 2 {
		return nil
	}

	bigrams := make(map[string]uint32)
	for i := 0; i < len(words)-1; i++ {
		bg := words[i] + ":" + words[i+1]
		bigrams[bg]++
	}
	return bigrams
}

// ProcessBigrams extracts bigrams from conversation text and accumulates
// counts. Bigrams that reach BigramThreshold are promoted from internal
// staging to state.Bigrams (persistent, subject to autotune decay).
//
// Already-promoted bigrams are incremented directly in state.Bigrams.
// Staging counts are NOT persisted — they reset on restart. This is
// acceptable because the threshold is a noise filter, not critical data.
func (l *Learner) ProcessBigrams(text string) {
	extracted := ExtractBigrams(text)
	for bg, count := range extracted {
		// Already promoted — direct increment
		if _, promoted := l.state.Bigrams[bg]; promoted {
			l.state.Bigrams[bg] += count
			continue
		}
		// Stage until threshold
		l.staging[bg] += count
		if l.staging[bg] >= BigramThreshold {
			l.state.Bigrams[bg] = l.staging[bg]
			delete(l.staging, bg)
		}
	}
}
