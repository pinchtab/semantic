package engine

import (
	"context"
	"fmt"
	"github.com/pinchtab/semantic/internal/types"
	"math"
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

	parsed := ParseQueryContext(query)
	mergeOpts := opts
	internalOpts := opts
	if parsed.Ordinal.HasOrdinal {
		mergeOpts.TopK = len(elements)
		internalOpts.TopK = len(elements)
	}

	lexW, embW := c.weights(opts)

	lexResult, embResult, err := c.runBothParsed(ctx, parsed, elements, internalOpts)
	if err != nil {
		return types.FindResult{}, err
	}

	merged := c.mergeResults(lexResult, embResult, elements, mergeOpts, lexW, embW)
	return selectOrdinalMatchInOrder(merged, parsed.Ordinal, elements), nil
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

func (c *CombinedMatcher) runBothParsed(ctx context.Context, parsed QueryContext, elements []types.ElementDescriptor, opts types.FindOptions) (types.FindResult, types.FindResult, error) {
	internalOpts := types.FindOptions{
		// Lower threshold allows both strategies to contribute to fusion
		// before final filtering at the caller's requested threshold.
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
		r := c.lexical.findWithParsed(parsed, elements, internalOpts)
		lexCh <- matcherResult{result: r}
	}()
	go func() {
		defer func() {
			if p := recover(); p != nil {
				embCh <- matcherResult{err: fmt.Errorf("embedding matcher panic: %v", p)}
			}
		}()
		r, err := c.embedding.findWithParsed(parsed, elements, internalOpts)
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
	for ref := range allRefs {
		combined := lexW*lexScores[ref] + embW*embScores[ref]
		if combined < 0 {
			combined = 0
		}
		if combined > 1 {
			combined = 1
		}
		if combined >= opts.Threshold {
			s := scored{ref: ref, score: combined, el: refToElem[ref]}
			if opts.Explain {
				s.lexScore = lexW * lexScores[ref]
				s.embScore = embW * embScores[ref]
			}
			candidates = append(candidates, s)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		scoreDiff := candidates[i].score - candidates[j].score
		if math.Abs(scoreDiff) > 1e-9 {
			return scoreDiff > 0
		}

		idxI := documentOrderIndex(candidates[i].el, i)
		idxJ := documentOrderIndex(candidates[j].el, j)
		if idxI != idxJ {
			return idxI < idxJ
		}

		return candidates[i].ref < candidates[j].ref
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

func documentOrderIndex(el types.ElementDescriptor, fallback int) int {
	if el.DocumentIdx > 0 {
		return el.DocumentIdx
	}
	if el.Positional.SiblingIndex > 0 {
		return el.Positional.SiblingIndex
	}
	return fallback
}

func scoreMap(matches []types.ElementMatch) map[string]float64 {
	m := make(map[string]float64, len(matches))
	for _, match := range matches {
		m[match.Ref] = match.Score
	}
	return m
}
