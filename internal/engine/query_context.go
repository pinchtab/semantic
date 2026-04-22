package engine

import (
	"regexp"
	"strings"

	"github.com/pinchtab/semantic/internal/types"
)

var negativeContextPattern = regexp.MustCompile(`(?i)\b(not|without|exclude|excluding|except|ignore)\b`)

type QueryContext struct {
	Base     ParsedQuery
	Exclude  []string
	HasScope bool
}

func ParseQueryContext(raw string) QueryContext {
	parsed := ParseQuery(raw)
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return QueryContext{Base: parsed}
	}

	loc := negativeContextPattern.FindStringIndex(cleaned)
	if loc == nil {
		return QueryContext{Base: parsed}
	}

	baseRaw := strings.TrimSpace(cleaned[:loc[0]])
	remainder := strings.TrimSpace(cleaned[loc[1]:])
	if baseRaw == "" || remainder == "" {
		return QueryContext{Base: parsed}
	}

	baseParsed := ParseQuery(baseRaw)
	if len(baseParsed.Positive) == 0 {
		return QueryContext{Base: parsed}
	}
	if len(parsed.Negative) == 0 {
		return QueryContext{Base: parsed}
	}

	exclude := normalizeContextPhrase(remainder)
	if len(exclude) == 0 {
		return QueryContext{Base: parsed}
	}

	if !looksLikeContextPhrase(exclude) {
		return QueryContext{Base: parsed}
	}

	return QueryContext{
		Base:     baseParsed,
		Exclude:  exclude,
		HasScope: true,
	}
}

func normalizeContextPhrase(raw string) []string {
	words := tokenize(strings.Trim(raw, ",.;:- "))
	if len(words) == 0 {
		return nil
	}

	for len(words) > 0 && contextLeadingFillers[words[0]] {
		words = words[1:]
	}
	for len(words) > 0 && contextTrailingFillers[words[len(words)-1]] {
		words = words[:len(words)-1]
	}
	if len(words) == 0 {
		return nil
	}
	return words
}

func matchesExcludedContext(el types.ElementDescriptor, excludeTokens []string) bool {
	if len(excludeTokens) == 0 {
		return false
	}

	ctxTokens := tokenize(strings.Join([]string{
		el.Parent,
		el.Section,
		el.Positional.LabelledBy,
		el.Role,
		el.Name,
		el.Value,
	}, " "))
	if len(ctxTokens) == 0 {
		return false
	}

	ctxSet := tokenSet(ctxTokens)
	matched := 0
	meaningful := 0
	for _, tok := range excludeTokens {
		if isStopword(tok) || isSemanticStopword(tok) {
			continue
		}
		meaningful++
		if ctxSet[tok] {
			matched++
		}
	}
	if meaningful == 0 {
		return false
	}
	if matched == meaningful {
		return true
	}
	if meaningful > 1 && float64(matched)/float64(meaningful) >= 0.7 {
		return true
	}
	return false
}

var contextLeadingFillers = map[string]bool{
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

var contextTrailingFillers = map[string]bool{
	"one": true,
}

var contextHintTokens = map[string]bool{
	"header":     true,
	"footer":     true,
	"sidebar":    true,
	"nav":        true,
	"navigation": true,
	"menu":       true,
	"toolbar":    true,
	"dialog":     true,
	"modal":      true,
	"form":       true,
	"panel":      true,
	"section":    true,
	"content":    true,
	"main":       true,
	"top":        true,
	"bottom":     true,
	"left":       true,
	"right":      true,
	"sticky":     true,
	"primary":    true,
	"secondary":  true,
}

func looksLikeContextPhrase(tokens []string) bool {
	for _, tok := range tokens {
		if contextHintTokens[tok] {
			return true
		}
	}
	return false
}
