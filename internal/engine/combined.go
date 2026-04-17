package engine

import (
	"context"
	"fmt"
	"github.com/pinchtab/semantic/internal/types"
	"sort"
)

// combinedMatcher fuses lexical and embedding scores:
//
//	score = 0.6 * lexical + 0.4 * embedding
//
// CombinedMatcher fuses lexical and embedding scores:
//
//	score = LexicalWeight * lexical + EmbeddingWeight * embedding
type CombinedMatcher struct {
	lexical   *LexicalMatcher
	embedding *EmbeddingMatcher

	// LexicalWeight and EmbeddingWeight should sum to 1.0 for
	// interpretable scores. Defaults: 0.6 / 0.4.
	LexicalWeight   float64
	EmbeddingWeight float64
}

// NewCombinedMatcher creates a matcher that fuses lexical and embedding
// strategies with default weights (0.6 lexical, 0.4 embedding).
func NewCombinedMatcher(embedder Embedder) *CombinedMatcher {
	return &CombinedMatcher{
		lexical:         NewLexicalMatcher(),
		embedding:       NewEmbeddingMatcher(embedder),
		LexicalWeight:   0.6,
		EmbeddingWeight: 0.4,
	}
}

func (c *CombinedMatcher) Strategy() string {
	return "combined:lexical+" + c.embedding.Strategy()
}

func (c *CombinedMatcher) Find(ctx context.Context, query string, elements []types.ElementDescriptor, opts types.FindOptions) (types.FindResult, error) {
	if opts.TopK <= 0 {
		opts.TopK = 3
	}

	visualHints := parseVisualQueryHints(query)
	effectiveQuery := query
	if visualHints.baseQuery != "" {
		effectiveQuery = visualHints.baseQuery
	}

	mergeOpts := opts
	if visualHints.hasHints {
		mergeOpts.TopK = len(elements)
	}

	lexW, embW := c.weights(opts)

	lexResult, embResult, err := c.runBoth(ctx, effectiveQuery, elements, opts)
	if err != nil {
		return types.FindResult{}, err
	}

	merged := c.mergeResults(lexResult, embResult, elements, mergeOpts, lexW, embW)
	return applyVisualHintBoost(merged, visualHints, elements, opts.TopK), nil
}

func (c *CombinedMatcher) weights(opts types.FindOptions) (float64, float64) {
	if opts.LexicalWeight > 0 || opts.EmbeddingWeight > 0 {
		return opts.LexicalWeight, opts.EmbeddingWeight
	}
	return c.LexicalWeight, c.EmbeddingWeight
}

type matcherResult struct {
	result types.FindResult
	err    error
}

func (c *CombinedMatcher) runBoth(ctx context.Context, query string, elements []types.ElementDescriptor, opts types.FindOptions) (types.FindResult, types.FindResult, error) {
	internalOpts := types.FindOptions{
		Threshold: opts.Threshold * 0.5,
		TopK:      len(elements),
	}

	lexCh := make(chan matcherResult, 1)
	embCh := make(chan matcherResult, 1)

	go func() {
		defer func() {
			if p := recover(); p != nil {
				lexCh <- matcherResult{err: fmt.Errorf("lexical matcher panic: %v", p)}
			}
		}()
		r, err := c.lexical.Find(ctx, query, elements, internalOpts)
		lexCh <- matcherResult{r, err}
	}()
	go func() {
		defer func() {
			if p := recover(); p != nil {
				embCh <- matcherResult{err: fmt.Errorf("embedding matcher panic: %v", p)}
			}
		}()
		r, err := c.embedding.Find(ctx, query, elements, internalOpts)
		embCh <- matcherResult{r, err}
	}()

	lexRes := <-lexCh
	embRes := <-embCh

	if lexRes.err != nil {
		return types.FindResult{}, types.FindResult{}, lexRes.err
	}
	if embRes.err != nil {
		return types.FindResult{}, types.FindResult{}, embRes.err
	}
	return lexRes.result, embRes.result, nil
}

type scored struct {
	ref      string
	score    float64
	el       types.ElementDescriptor
	order    int
	lexScore float64
	embScore float64
}

func (c *CombinedMatcher) mergeResults(lexResult, embResult types.FindResult, elements []types.ElementDescriptor, opts types.FindOptions, lexW, embW float64) types.FindResult {
	lexScores := scoreMap(lexResult.Matches)
	embScores := scoreMap(embResult.Matches)

	refToElem := make(map[string]types.ElementDescriptor, len(elements))
	for _, el := range elements {
		refToElem[el.Ref] = el
	}

	// Collect all refs from either matcher.
	allRefs := make(map[string]bool, len(lexScores)+len(embScores))
	for ref := range lexScores {
		allRefs[ref] = true
	}
	for ref := range embScores {
		allRefs[ref] = true
	}

	candidates := make([]scored, 0, len(allRefs))
	appendCandidate := func(ref string, el types.ElementDescriptor, order int) {
		combined := lexW*lexScores[ref] + embW*embScores[ref]
		if combined < opts.Threshold {
			return
		}

		s := scored{ref: ref, score: combined, el: el, order: order}
		if opts.Explain {
			s.lexScore = lexW * lexScores[ref]
			s.embScore = embW * embScores[ref]
		}
		candidates = append(candidates, s)
	}

	for i, el := range elements {
		if !allRefs[el.Ref] {
			continue
		}
		appendCandidate(el.Ref, el, i)
		delete(allRefs, el.Ref)
	}

	if len(allRefs) > 0 {
		extraRefs := make([]string, 0, len(allRefs))
		for ref := range allRefs {
			extraRefs = append(extraRefs, ref)
		}
		sort.Strings(extraRefs)
		for i, ref := range extraRefs {
			appendCandidate(ref, refToElem[ref], len(elements)+i)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return rankedMatchLess(
			candidates[i].score,
			candidates[i].el,
			candidates[i].order,
			candidates[j].score,
			candidates[j].el,
			candidates[j].order,
		)
	})
	if len(candidates) > opts.TopK {
		candidates = candidates[:opts.TopK]
	}

	result := types.FindResult{
		Strategy:     c.Strategy(),
		ElementCount: len(elements),
	}
	for _, cand := range candidates {
		em := types.ElementMatch{
			Ref:   cand.ref,
			Score: cand.score,
			Role:  cand.el.Role,
			Name:  cand.el.Name,
		}
		if opts.Explain {
			em.Explain = &types.MatchExplain{
				LexicalScore:   cand.lexScore,
				EmbeddingScore: cand.embScore,
				Composite:      cand.el.Composite(),
			}
		}
		result.Matches = append(result.Matches, em)
	}
	if len(result.Matches) > 0 {
		result.BestRef = result.Matches[0].Ref
		result.BestScore = result.Matches[0].Score
	}
	return result
}

func scoreMap(matches []types.ElementMatch) map[string]float64 {
	m := make(map[string]float64, len(matches))
	for _, match := range matches {
		m[match.Ref] = match.Score
	}
	return m
}
