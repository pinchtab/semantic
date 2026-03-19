package semantic

// stopwords is a set of common English words that carry little semantic
// meaning and should be excluded from lexical matching to improve
// signal-to-noise ratio.
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"to": true, "of": true, "for": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"into": true, "through": true, "about": true, "above": true,
	"after": true, "before": true, "between": true, "under": true,
	"and": true, "but": true, "nor": true,
	"so": true, "yet": true, "both": true, "either": true, "neither": true,
	"this": true, "that": true, "these": true, "those": true,
	"it": true, "its": true, "i": true, "me": true, "my": true,
	"we": true, "our": true, "you": true, "your": true, "he": true,
	"she": true, "his": true, "her": true, "they": true, "their": true,
}

// semanticStopwords are words that are normally stopwords but carry
// meaningful signal in certain UI contexts (e.g. "in" in "sign in",
// "not" in "do not"). They are removed ONLY if they don't appear in
// the other side's token set (context-aware removal).
var semanticStopwords = map[string]bool{
	"in":  true, // "sign in", "log in"
	"up":  true, // "sign up", "look up"
	"out": true, // "log out", "sign out"
	"on":  true, // "log on"
	"off": true, // "log off"
	"not": true, // "do not", "not now"
	"no":  true, // negation carries meaning
	"or":  true, // disjunction in UI labels
	"ok":  true, // acceptance button
}

// isStopword returns true if the token is a common English stopword.
func isStopword(token string) bool {
	return stopwords[token]
}

// isSemanticStopword returns true for words that are semi-stopwords:
// normally low-value but can carry meaning in UI context.
func isSemanticStopword(token string) bool {
	return semanticStopwords[token]
}

// removeStopwords filters out stopwords from a token list.
// If removal would empty the list, the original tokens are returned
// to avoid zero-signal matching.
func removeStopwords(tokens []string) []string {
	filtered := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if !isStopword(t) {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return tokens
	}
	return filtered
}

// removeStopwordsContextAware performs context-aware stopword removal.
// A word is preserved if:
//  1. It is not a stopword, OR
//  2. It IS a semantic stopword AND it appears in the other set of
//     tokens (meaning it carries matching signal), OR
//  3. It forms part of a known synonym phrase with adjacent tokens
//     (e.g. "sign" + "in" → "sign in" is a synonym entry).
//
// Falls back to returning original tokens if everything would be removed.
func removeStopwordsContextAware(tokens []string, otherTokens []string) []string {
	otherSet := make(map[string]bool, len(otherTokens))
	for _, t := range otherTokens {
		otherSet[t] = true
	}

	phraseTokens := make(map[int]bool)
	for n := 2; n <= 3 && n <= len(tokens); n++ {
		for i := 0; i <= len(tokens)-n; i++ {
			joined := ""
			for j := i; j < i+n; j++ {
				if j > i {
					joined += " "
				}
				joined += tokens[j]
			}
			if _, ok := synonymIndex[joined]; ok {
				for j := i; j < i+n; j++ {
					phraseTokens[j] = true
				}
			}
		}
	}

	filtered := make([]string, 0, len(tokens))
	for i, t := range tokens {
		switch {
		case !isStopword(t) && !isSemanticStopword(t):
			// Not a stopword at all — always keep.
			filtered = append(filtered, t)
		case phraseTokens[i]:
			// Part of a known synonym phrase — keep it.
			filtered = append(filtered, t)
		case isSemanticStopword(t) && otherSet[t]:
			// Semantic stopword that appears in the other side — keep.
			filtered = append(filtered, t)
		case isSemanticStopword(t) && !isStopword(t):
			// Semantic-only word not in the hard stopword list — keep.
			filtered = append(filtered, t)
			// Pure stopwords are dropped (default case).
		}
	}

	if len(filtered) == 0 {
		return tokens
	}
	return filtered
}
