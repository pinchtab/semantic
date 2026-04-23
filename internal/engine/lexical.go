package engine

import (
	"context"
	"github.com/pinchtab/semantic/internal/types"
	"math"
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
	// phraseExactBonus rewards full multi-word phrase containment.
	phraseExactBonus = 0.15
	// phrasePartialBonus rewards partial phrase containment (bigrams/trigrams).
	phrasePartialBonus = 0.08
	// interactiveActionBoost is applied when action verbs imply intent to interact.
	interactiveActionBoost = 0.10
	// interactiveBaseBoost lightly favors interactive elements for generic queries.
	interactiveBaseBoost = 0.05
	// positionalLabelBoost rewards label associations matching the query.
	positionalLabelBoost = 0.15
	// positionalUniqueSiblingBoost rewards singleton role elements in a group.
	positionalUniqueSiblingBoost = 0.03
	// typoBonusPerToken rewards one-edit typos on short tokens.
	typoBonusPerToken = 0.08
	// typoBonusCap prevents typo matching from dominating lexical score.
	typoBonusCap = 0.15
)

// LexicalMatcher scores elements using Jaccard similarity with synonym
// expansion, context-aware stopwords, role boosting, and prefix matching.
type LexicalMatcher struct{}

// NewLexicalMatcher creates a stateless lexical matcher.
func NewLexicalMatcher() *LexicalMatcher {
	return &LexicalMatcher{}
}

func (m *LexicalMatcher) Strategy() string { return "lexical" }

func (m *LexicalMatcher) Find(ctx context.Context, query string, elements []types.ElementDescriptor, opts types.FindOptions) (types.FindResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return types.FindResult{}, err
	}

	queryCtx := ParseQueryContext(query)
	return m.findWithParsedContext(ctx, queryCtx, elements, opts), nil
}

func (m *LexicalMatcher) findWithParsed(queryCtx QueryContext, elements []types.ElementDescriptor, opts types.FindOptions) types.FindResult {
	return m.findWithParsedContext(context.Background(), queryCtx, elements, opts)
}

func (m *LexicalMatcher) findWithParsedContext(ctx context.Context, queryCtx QueryContext, elements []types.ElementDescriptor, opts types.FindOptions) types.FindResult {
	parsed := queryCtx.Base
	opts = sanitizeFindOptions(opts, len(elements), 3)

	if len(parsed.Positive) == 0 && len(parsed.Negative) == 0 {
		return types.FindResult{
			Strategy:     "lexical",
			ElementCount: len(elements),
		}
	}

	negativeOnly := len(parsed.Positive) == 0 && len(parsed.Negative) > 0
	positiveQuery := strings.Join(parsed.Positive, " ")

	ef := BuildElementFrequency(elements)

	type scored struct {
		desc  types.ElementDescriptor
		score float64
	}

	var candidates []scored
	for i, el := range elements {
		if i%64 == 0 {
			if ctx.Err() != nil {
				break
			}
		}

		if queryCtx.HasScope && matchesExcludedContext(el, queryCtx.Exclude) {
			continue
		}

		composite := el.Composite()
		descTokens := tokenize(composite)
		score := 0.0
		if len(parsed.Positive) == 0 {
			score = 1.0
		} else {
			score = lexicalScoreTokens(parsed.Positive, descTokens, el.Interactive, ef)
			score += positionalBoost(positiveQuery, el.Positional)
		}

		if len(parsed.Negative) > 0 {
			negativeScore := lexicalScoreTokens(parsed.Negative, descTokens, el.Interactive, ef)
			switch {
			case hasStrongNegativeHit(parsed.Negative, descTokens) || negativeScore > 0.7:
				score = 0
			case negativeScore > 0.4:
				score *= 1 - negativeScore
			}
		}

		if score < 0 {
			score = 0
		}
		if score > 1.0 {
			score = 1.0
		}
		if negativeOnly && score == 0 {
			continue
		}
		if score >= opts.Threshold {
			candidates = append(candidates, scored{desc: el, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		scoreDiff := candidates[i].score - candidates[j].score
		if math.Abs(scoreDiff) > 1e-9 {
			return scoreDiff > 0
		}

		depthI := candidates[i].desc.Positional.Depth
		depthJ := candidates[j].desc.Positional.Depth
		if depthI != depthJ {
			return depthI > depthJ
		}

		idxI := candidates[i].desc.Positional.SiblingIndex
		idxJ := candidates[j].desc.Positional.SiblingIndex
		if idxI != idxJ {
			return idxI < idxJ
		}

		return candidates[i].desc.Ref < candidates[j].desc.Ref
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

	return result
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

var actionVerbs = map[string]bool{
	"click":  true,
	"press":  true,
	"tap":    true,
	"type":   true,
	"enter":  true,
	"select": true,
	"check":  true,
	"toggle": true,
	"submit": true,
	"fill":   true,
}

// ElementFrequency holds per-snapshot token document frequencies.
// It is used to compute inverse element frequency (IEF) token weights.
type ElementFrequency struct {
	tokenDF map[string]int
	total   int
}

// BuildElementFrequency creates and fills frequency statistics for one snapshot.
func BuildElementFrequency(elements []types.ElementDescriptor) *ElementFrequency {
	ef := &ElementFrequency{}
	ef.Build(elements)
	return ef
}

// Build recomputes token frequencies from a snapshot.
func (ef *ElementFrequency) Build(elements []types.ElementDescriptor) {
	ef.tokenDF = make(map[string]int)
	ef.total = len(elements)

	for _, el := range elements {
		seen := tokenSet(tokenize(el.Composite()))
		for t := range seen {
			ef.tokenDF[t]++
		}
	}
}

// IEF returns inverse element frequency for a token.
func (ef *ElementFrequency) IEF(token string) float64 {
	if ef == nil || ef.total <= 0 {
		return 0
	}
	df := ef.tokenDF[token]
	if df <= 0 {
		return 0
	}
	return math.Log(1 + float64(ef.total)/float64(df))
}

// LexicalScore computes Jaccard similarity with synonym expansion,
// context-aware stopwords, role boosting, and prefix matching.
// Returns [0, 1].
func LexicalScore(query, desc string) float64 {
	return lexicalScore(query, desc, false, nil)
}

// LexicalScoreWithFrequency computes lexical similarity with optional
// snapshot-level IEF weighting (nil keeps default equal-weight behavior).
func LexicalScoreWithFrequency(query, desc string, ef *ElementFrequency) float64 {
	return lexicalScore(query, desc, false, ef)
}

func lexicalScore(query, desc string, interactive bool, ef *ElementFrequency) float64 {
	rawQTokens := tokenize(query)
	rawDTokens := tokenize(desc)
	return lexicalScoreTokens(rawQTokens, rawDTokens, interactive, ef)
}

func lexicalScoreTokens(rawQTokens, rawDTokens []string, interactive bool, ef *ElementFrequency) float64 {
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
			intersectW += float64(minC) * tokenWeight(t, ef)
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
		unionW += float64(maxC) * tokenWeight(t, ef)
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

	// --- 5. Phrase bonus for preserving multi-word intent ---
	phraseBoost := phraseBonus(qTokens, dTokens)

	// --- 6. Interactive boost for action-oriented queries ---
	interactiveScore := interactiveBoost(qTokens, interactive)

	// --- 7. Typo tolerance bonus (Levenshtein edit distance = 1) ---
	typoScore := typoBonus(qTokens, dTokens)

	score := jaccard + synScore + prefixScore + roleBoost + phraseBoost + interactiveScore + typoScore
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func phraseBonus(qTokens, dTokens []string) float64 {
	if len(qTokens) < 2 || len(dTokens) < 2 {
		return 0
	}

	qPhrase := strings.Join(qTokens, " ")
	dPhrase := strings.Join(dTokens, " ")
	if strings.Contains(dPhrase, qPhrase) {
		return phraseExactBonus
	}

	// Fallback: reward any matching significant query sub-phrase.
	for n := minInt(3, len(qTokens)); n >= 2; n-- {
		for i := 0; i+n <= len(qTokens); i++ {
			sub := strings.Join(qTokens[i:i+n], " ")
			if strings.Contains(dPhrase, sub) {
				return phrasePartialBonus
			}
		}
	}

	return 0
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func interactiveBoost(qTokens []string, isInteractive bool) float64 {
	if !isInteractive {
		return 0
	}
	if containsActionVerb(qTokens) {
		return interactiveActionBoost
	}
	return interactiveBaseBoost
}

func positionalBoost(query string, hints types.PositionalHints) float64 {
	boost := 0.0
	if hints.SiblingCount == 1 {
		boost += positionalUniqueSiblingBoost
	}
	if hints.LabelledBy != "" && labelMatchesQuery(query, hints.LabelledBy) {
		boost += positionalLabelBoost
	}
	return boost
}

func labelMatchesQuery(query, labelledBy string) bool {
	qTokens := tokenize(query)
	lTokens := tokenize(labelledBy)

	qTokens = removeStopwordsContextAware(qTokens, lTokens)
	lTokens = removeStopwordsContextAware(lTokens, qTokens)
	if len(qTokens) == 0 || len(lTokens) == 0 {
		return false
	}

	qSet := tokenSet(qTokens)
	lSet := tokenSet(lTokens)
	matched := 0
	for t := range qSet {
		if lSet[t] {
			matched++
		}
	}
	if matched == 0 {
		return false
	}

	return matched*2 >= len(qSet)
}

func containsActionVerb(tokens []string) bool {
	for _, t := range tokens {
		if actionVerbs[t] {
			return true
		}
	}
	return false
}

func typoBonus(qTokens, dTokens []string) float64 {
	bonus := 0.0

	qSeen := tokenSet(qTokens)
	for qt := range qSeen {
		ql := len([]rune(qt))
		if ql < 3 || ql > 10 {
			continue
		}

		for _, dt := range dTokens {
			if qt == dt {
				continue
			}
			dl := len([]rune(dt))
			if dl < 3 || dl > 10 {
				continue
			}
			if absInt(ql-dl) > 1 {
				continue
			}
			if levenshtein(qt, dt) == 1 {
				bonus += typoBonusPerToken
				break
			}
		}

		if bonus >= typoBonusCap {
			return typoBonusCap
		}
	}

	if bonus > typoBonusCap {
		return typoBonusCap
	}
	return bonus
}

func levenshtein(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}

	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := 0; j <= len(br); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			ins := cur[j-1] + 1
			del := prev[j] + 1
			sub := prev[j-1] + cost
			cur[j] = minInt(ins, minInt(del, sub))
		}
		prev, cur = cur, prev
	}

	return prev[len(br)]
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func tokenWeight(token string, ef *ElementFrequency) float64 {
	if ef == nil {
		return 1.0
	}
	w := ef.IEF(token)
	if w <= 0 {
		return 1.0
	}
	return w
}

// weightedJaccard computes set-based Jaccard similarity with optional
// per-token weights (e.g., IDF). Missing or non-positive token weights default
// to 1.0 so callers can pass partial maps safely.
func weightedJaccard(qTokens, dTokens []string, idf map[string]float64) float64 {
	qSet := tokenSet(qTokens)
	dSet := tokenSet(dTokens)

	all := make(map[string]bool, len(qSet)+len(dSet))
	for t := range qSet {
		all[t] = true
	}
	for t := range dSet {
		all[t] = true
	}

	var intersectW float64
	var unionW float64
	for t := range all {
		w := 1.0
		if idf != nil {
			if iw, ok := idf[t]; ok && iw > 0 {
				w = iw
			}
		}

		if qSet[t] && dSet[t] {
			intersectW += w
		}
		unionW += w
	}

	if unionW == 0 {
		return 0
	}
	return intersectW / unionW
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

func hasStrongNegativeHit(negativeTokens, descTokens []string) bool {
	if len(negativeTokens) == 0 || len(descTokens) == 0 {
		return false
	}

	dSet := tokenSet(descTokens)
	for _, nt := range negativeTokens {
		if isStopword(nt) || isSemanticStopword(nt) {
			continue
		}
		if dSet[nt] {
			return true
		}
		if syns, ok := synonymIndex[nt]; ok {
			for syn := range syns {
				synTokens := strings.Fields(syn)
				if len(synTokens) == 0 {
					continue
				}
				allPresent := true
				hasMeaningfulToken := false
				for _, st := range synTokens {
					if isStopword(st) || isSemanticStopword(st) {
						continue
					}
					hasMeaningfulToken = true
					if !dSet[st] {
						allPresent = false
						break
					}
				}
				if hasMeaningfulToken && allPresent {
					return true
				}
			}
		}
	}

	return false
}
