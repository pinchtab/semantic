package engine

import (
	"context"
	"github.com/pinchtab/semantic/internal/types"
	"sort"
	"strings"
	"unicode"
)

const (
	// roleBoostPerMatch is added per overlapping role keyword (capped).
	roleBoostPerMatch = 0.12
	// roleBoostCap prevents role boost from dominating the score.
	roleBoostCap = 0.25
	// synonymBoostWeight controls how much synonym matches contribute.
	synonymBoostWeight = 0.30
	// prefixMatchWeight controls how much prefix matches contribute.
	prefixMatchWeight = 0.20
)

// LexicalMatcher scores elements using Jaccard similarity with synonym
// expansion, context-aware stopwords, role boosting, and prefix matching.
type LexicalMatcher struct{}

// NewLexicalMatcher creates a stateless lexical matcher.
func NewLexicalMatcher() *LexicalMatcher {
	return &LexicalMatcher{}
}

func (m *LexicalMatcher) Strategy() string { return "lexical" }

func (m *LexicalMatcher) Find(_ context.Context, query string, elements []types.ElementDescriptor, opts types.FindOptions) (types.FindResult, error) {
	if opts.TopK <= 0 {
		opts.TopK = 3
	}

	type scored struct {
		desc  types.ElementDescriptor
		score float64
	}

	var candidates []scored
	for _, el := range elements {
		composite := el.Composite()
		score := LexicalScore(query, composite)
		if score >= opts.Threshold {
			candidates = append(candidates, scored{desc: el, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > opts.TopK {
		candidates = candidates[:opts.TopK]
	}

	result := types.FindResult{
		Strategy:     "lexical",
		ElementCount: len(elements),
	}

	for _, c := range candidates {
		result.Matches = append(result.Matches, types.ElementMatch{
			Ref:   c.desc.Ref,
			Score: c.score,
			Role:  c.desc.Role,
			Name:  c.desc.Name,
		})
	}

	if len(result.Matches) > 0 {
		result.BestRef = result.Matches[0].Ref
		result.BestScore = result.Matches[0].Score
	}

	return result, nil
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func tokenFreq(tokens []string) map[string]int {
	m := make(map[string]int, len(tokens))
	for _, t := range tokens {
		m[t]++
	}
	return m
}

func tokenSet(tokens []string) map[string]bool {
	m := make(map[string]bool, len(tokens))
	for _, t := range tokens {
		m[t] = true
	}
	return m
}

var roleKeywords = map[string]bool{
	"button":   true,
	"input":    true,
	"link":     true,
	"submit":   true,
	"form":     true,
	"textbox":  true,
	"checkbox": true,
	"radio":    true,
	"select":   true,
	"option":   true,
	"tab":      true,
	"menu":     true,
	"search":   true,
}

// LexicalScore computes Jaccard similarity with synonym expansion,
// context-aware stopwords, role boosting, and prefix matching.
// Returns [0, 1].
func LexicalScore(query, desc string) float64 {
	rawQTokens := tokenize(query)
	rawDTokens := tokenize(desc)

	qTokens := removeStopwordsContextAware(rawQTokens, rawDTokens)
	dTokens := removeStopwordsContextAware(rawDTokens, rawQTokens)

	if len(qTokens) == 0 || len(dTokens) == 0 {
		return 0
	}

	// --- 1. Base Jaccard with frequency weighting ---
	qFreq := tokenFreq(qTokens)
	dFreq := tokenFreq(dTokens)

	var intersectW float64
	for t, qc := range qFreq {
		if dc, ok := dFreq[t]; ok {
			minC := qc
			if dc < minC {
				minC = dc
			}
			intersectW += float64(minC)
		}
	}

	allTokens := tokenSet(append(qTokens, dTokens...))
	var unionW float64
	for t := range allTokens {
		qc := qFreq[t]
		dc := dFreq[t]
		maxC := qc
		if dc > maxC {
			maxC = dc
		}
		unionW += float64(maxC)
	}

	if unionW == 0 {
		return 0
	}

	jaccard := intersectW / unionW

	// --- 2. Synonym boost ---
	synScore := synonymScore(qTokens, dTokens) * synonymBoostWeight

	// --- 3. Prefix matching boost ---
	prefixScore := tokenPrefixScore(qTokens, dTokens) * prefixMatchWeight

	// --- 4. Role keyword boost (cumulative, capped) ---
	roleBoost := 0.0
	qSet := tokenSet(qTokens)
	dSet := tokenSet(dTokens)
	for t := range qSet {
		if roleKeywords[t] && dSet[t] {
			roleBoost += roleBoostPerMatch
		}
	}
	for t := range qSet {
		if roleKeywords[t] {
			continue // already checked direct match
		}
		if syns, ok := synonymIndex[t]; ok {
			for syn := range syns {
				if roleKeywords[syn] && dSet[syn] {
					roleBoost += roleBoostPerMatch * 0.8
					break
				}
			}
		}
	}
	if roleBoost > roleBoostCap {
		roleBoost = roleBoostCap
	}

	score := jaccard + synScore + prefixScore + roleBoost
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func tokenPrefixScore(qTokens, dTokens []string) float64 {
	if len(qTokens) == 0 {
		return 0
	}

	var total float64
	for _, qt := range qTokens {
		if len(qt) < 2 {
			continue
		}
		bestMatch := 0.0
		for _, dt := range dTokens {
			if qt == dt {
				continue // already counted by Jaccard
			}
			if len(dt) > len(qt) && strings.HasPrefix(dt, qt) {
				ratio := float64(len(qt)) / float64(len(dt))
				if ratio > bestMatch {
					bestMatch = ratio
				}
			}
			if len(qt) > len(dt) && strings.HasPrefix(qt, dt) {
				ratio := float64(len(dt)) / float64(len(qt))
				if ratio*0.7 > bestMatch { // penalize reverse prefix slightly
					bestMatch = ratio * 0.7
				}
			}
		}
		total += bestMatch
	}

	return total / float64(len(qTokens))
}
