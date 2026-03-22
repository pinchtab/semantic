package engine

// stopwords that carry little semantic meaning in UI matching.
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

// semanticStopwords are context-dependent: "in" is noise alone but
// meaningful in "sign in". Removed only when absent from the other token set.
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

func isStopword(token string) bool {
	return stopwords[token]
}

func isSemanticStopword(token string) bool {
	return semanticStopwords[token]
}

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

// removeStopwordsContextAware preserves stopwords that appear in the other
// token set or form part of known synonym phrases ("sign in", "log on").
// Returns original tokens if removal would empty the list.
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
