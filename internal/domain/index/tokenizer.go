package index

import (
	"regexp"
	"strings"
	"unicode"
)

// separatorRe splits on slash, underscore, hyphen, dot, whitespace.
var separatorRe = regexp.MustCompile(`[/_\-.\s]+`)

// Tokenize splits a query or symbol name into normalized tokens.
// Rules from SPEC.md:
//  1. Split on [/_\-.\s]+ (slash, underscore, hyphen, dot, whitespace)
//  2. CamelCase split
//  3. Lowercase all
//  4. Discard tokens < 2 chars
func Tokenize(input string) []string {
	if len(input) == 0 {
		return nil
	}

	// Strip non-ASCII characters (graceful handling of unicode like "résumé")
	cleaned := stripNonASCII(input)
	if len(cleaned) == 0 {
		return nil
	}

	// Step 1: Split on separators
	parts := separatorRe.Split(cleaned, -1)

	var tokens []string
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		// Step 2: CamelCase split each part
		subTokens := splitCamelCase(part)
		for _, tok := range subTokens {
			// Step 3: Lowercase
			tok = strings.ToLower(tok)
			// Step 4: Discard < 2 chars
			if len(tok) >= 2 {
				tokens = append(tokens, tok)
			}
		}
	}

	if len(tokens) == 0 {
		return nil
	}
	return tokens
}

// splitCamelCase splits a string on CamelCase boundaries.
// Examples:
//
//	"getUserToken"  -> ["get", "User", "Token"]
//	"APIKey"        -> ["API", "Key"]
//	"LOGIN"         -> ["LOGIN"]
//	"handler404Resp" -> ["handler", "404", "Resp"]
func splitCamelCase(s string) []string {
	if len(s) == 0 {
		return nil
	}

	runes := []rune(s)
	var parts []string
	start := 0

	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		cur := runes[i]

		split := false

		switch {
		// lowercase -> uppercase: "getUser" splits before 'U'
		case unicode.IsLower(prev) && unicode.IsUpper(cur):
			split = true
		// letter -> digit: "handler404" splits before '4'
		case unicode.IsLetter(prev) && unicode.IsDigit(cur):
			split = true
		// digit -> letter: "404Response" splits before 'R'
		case unicode.IsDigit(prev) && unicode.IsLetter(cur):
			split = true
		// uppercase -> uppercase+lowercase: "APIKey" splits before 'K'
		// (only if there's a run of 2+ uppercase before a lowercase)
		case unicode.IsUpper(prev) && unicode.IsUpper(cur):
			// Look ahead: if next char is lowercase, split before current
			if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				split = true
			}
		}

		if split {
			parts = append(parts, string(runes[start:i]))
			start = i
		}
	}

	// Add the remaining part
	parts = append(parts, string(runes[start:]))
	return parts
}

// stripNonASCII removes non-ASCII runes, keeping only printable ASCII.
func stripNonASCII(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r <= unicode.MaxASCII && r >= ' ' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
