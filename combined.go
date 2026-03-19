package semantic

import (
	"context"
	"fmt"
	"sort"
)

// CombinedMatcher fuses results from a lexical matcher and an embedding
// matcher. The final score for each element is a weighted average:
//
//	score = lexicalWeight * lexicalScore + embeddingWeight * embeddingScore
//
// This gives the best of both worlds: exact word overlap from lexical
// matching and fuzzy / sub-word similarity from embedding matching.
type CombinedMatcher struct {
	lexical   *LexicalMatcher
	embedding *EmbeddingMatcher

	// Weight factors (should sum to 1.0 for interpretable scores).
	LexicalWeight   float64
	EmbeddingWeight float64
}

// NewCombinedMatcher creates a CombinedMatcher with default weights
// (0.6 lexical, 0.4 embedding). The lexical component has higher weight
// because it handles exact matches perfectly, while embeddings add value
// for fuzzy / partial queries.
func NewCombinedMatcher(embedder Embedder) *CombinedMatcher {
	return &CombinedMatcher{
		lexical:         NewLexicalMatcher(),
		embedding:       NewEmbeddingMatcher(embedder),
		LexicalWeight:   0.6,
		EmbeddingWeight: 0.4,
	}
}

// Strategy returns "combined:lexical+embedding:<embedder>".
func (c *CombinedMatcher) Strategy() string {
	return "combined:lexical+" + c.embedding.Strategy()
}

// Find runs both lexical and embedding matchers, merges scores by ref,
// applies weighted averaging, and returns the top-K candidates.
func (c *CombinedMatcher) Find(ctx context.Context, query string, elements []ElementDescriptor, opts FindOptions) (FindResult, error) {
	if opts.TopK <= 0 {
		opts.TopK = 3
	}

	// Per-request weight overrides; fall back to matcher defaults.
	lexW := c.LexicalWeight
	embW := c.EmbeddingWeight
	if opts.LexicalWeight > 0 || opts.EmbeddingWeight > 0 {
		lexW = opts.LexicalWeight
		embW = opts.EmbeddingWeight
	}

	// Use a lower internal threshold to capture candidates from both matchers
	// that might miss the final threshold individually but pass when combined.
	internalOpts := FindOptions{
		Threshold: opts.Threshold * 0.5,
		TopK:      len(elements), // get all candidates internally
	}

	// Run both matchers concurrently.
	type matcherResult struct {
		result FindResult
		err    error
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
		return FindResult{}, lexRes.err
	}
	if embRes.err != nil {
		return FindResult{}, embRes.err
	}

	// Build ref → score maps.
	lexScores := make(map[string]float64, len(lexRes.result.Matches))
	for _, m := range lexRes.result.Matches {
		lexScores[m.Ref] = m.Score
	}

	embScores := make(map[string]float64, len(embRes.result.Matches))
	for _, m := range embRes.result.Matches {
		embScores[m.Ref] = m.Score
	}

	// Build element lookup by ref for metadata.
	refToElem := make(map[string]ElementDescriptor, len(elements))
	for _, el := range elements {
		refToElem[el.Ref] = el
	}

	// Merge: collect all refs that appeared in either matcher.
	allRefs := make(map[string]bool)
	for ref := range lexScores {
		allRefs[ref] = true
	}
	for ref := range embScores {
		allRefs[ref] = true
	}

	type scored struct {
		ref      string
		score    float64
		el       ElementDescriptor
		lexScore float64
		embScore float64
	}

	var candidates []scored
	for ref := range allRefs {
		ls := lexScores[ref]
		es := embScores[ref]
		combined := lexW*ls + embW*es
		if combined >= opts.Threshold {
			s := scored{
				ref:   ref,
				score: combined,
				el:    refToElem[ref],
			}
			if opts.Explain {
				s.lexScore = lexW * ls
				s.embScore = embW * es
			}
			candidates = append(candidates, s)
		}
	}

	// Sort descending by combined score.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > opts.TopK {
		candidates = candidates[:opts.TopK]
	}

	result := FindResult{
		Strategy:     c.Strategy(),
		ElementCount: len(elements),
	}

	for _, cand := range candidates {
		em := ElementMatch{
			Ref:   cand.ref,
			Score: cand.score,
			Role:  cand.el.Role,
			Name:  cand.el.Name,
		}
		if opts.Explain {
			em.Explain = &MatchExplain{
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

	return result, nil
}
