package engine

import "github.com/pinchtab/semantic/internal/types"

// Query grammar:
//
//	<positive tokens> [NEGATIVE_TRIGGER <negative token>...]+
//
// A NEGATIVE_TRIGGER is one of:
// not, without, exclude, excluding, except, no, ignore.
// After a trigger, all following tokens are classified as negative until
// another trigger or the end of the query.
type ParsedQuery = types.ParsedQuery

var negativeTriggers = map[string]bool{
	"not":       true,
	"without":   true,
	"exclude":   true,
	"excluding": true,
	"except":    true,
	"no":        true,
	"ignore":    true,
}

// ParseQuery tokenizes and classifies tokens into positive and negative terms.
func ParseQuery(raw string) ParsedQuery {
	tokens := tokenize(raw)
	parsed := types.ParsedQuery{
		Positive: make([]string, 0, len(tokens)),
		Negative: make([]string, 0, len(tokens)),
	}

	inNegative := false
	for _, tok := range tokens {
		if negativeTriggers[tok] {
			inNegative = true
			continue
		}
		if inNegative {
			parsed.Negative = append(parsed.Negative, tok)
			continue
		}
		parsed.Positive = append(parsed.Positive, tok)
	}

	return parsed
}
