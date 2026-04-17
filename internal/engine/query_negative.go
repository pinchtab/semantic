package engine

import (
	"regexp"
	"strings"

	"github.com/pinchtab/semantic/internal/types"
)

var negativeQualifierPattern = regexp.MustCompile(`(?i)\b(not|excluding|except)\b`)

type queryNegativeConstraints struct {
	baseQuery       string
	exclusionPhrase string
}

func parseNegativeConstraints(query string) queryNegativeConstraints {
	cleaned := strings.TrimSpace(query)
	if cleaned == "" {
		return queryNegativeConstraints{}
	}

	loc := negativeQualifierPattern.FindStringIndex(cleaned)
	if loc == nil {
		return queryNegativeConstraints{baseQuery: cleaned}
	}

	base := strings.Trim(strings.TrimSpace(cleaned[:loc[0]]), ",.;:-")
	remainder := strings.TrimSpace(cleaned[loc[1]:])
	if base == "" || remainder == "" {
		return queryNegativeConstraints{baseQuery: cleaned}
	}

	exclusion := normalizeExclusionPhrase(remainder)
	if exclusion == "" {
		return queryNegativeConstraints{baseQuery: cleaned}
	}

	return queryNegativeConstraints{baseQuery: base, exclusionPhrase: exclusion}
}

func normalizeExclusionPhrase(s string) string {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(strings.Trim(s, ",.;:-"))))
	if len(words) == 0 {
		return ""
	}

	for len(words) > 0 && exclusionFillerWords[words[0]] {
		words = words[1:]
	}
	for len(words) > 0 && exclusionTailWords[words[len(words)-1]] {
		words = words[:len(words)-1]
	}

	if len(words) == 0 {
		return ""
	}
	return strings.Join(words, " ")
}

func shouldExcludeElement(el types.ElementDescriptor, exclusion string) bool {
	if exclusion == "" {
		return false
	}

	context := strings.ToLower(strings.Join([]string{
		el.Parent,
		el.Section,
		el.Role,
		el.Name,
		el.Value,
		el.Positional.LabelledBy,
	}, " "))
	if context == "" {
		return false
	}

	exclusion = strings.ToLower(strings.TrimSpace(exclusion))
	if exclusion == "" {
		return false
	}

	if strings.Contains(context, exclusion) {
		return true
	}

	exSet := tokenSet(tokenize(exclusion))
	if len(exSet) == 0 {
		return false
	}
	ctxSet := tokenSet(tokenize(context))

	matched := 0
	for tok := range exSet {
		if ctxSet[tok] {
			matched++
		}
	}
	if matched == 0 {
		return false
	}

	if float64(matched)/float64(len(exSet)) >= 0.5 {
		return true
	}

	return lexicalScore(exclusion, context, false, nil) >= 0.55
}

var exclusionFillerWords = map[string]bool{
	"in":     true,
	"on":     true,
	"at":     true,
	"of":     true,
	"to":     true,
	"from":   true,
	"inside": true,
	"within": true,
	"the":    true,
	"a":      true,
	"an":     true,
}

var exclusionTailWords = map[string]bool{
	"one": true,
}
