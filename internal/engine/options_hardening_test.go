package engine

import (
	"context"
	"math"
	"testing"

	"github.com/pinchtab/semantic/internal/types"
)

func TestSanitizeFindOptions_AdversarialMatrix(t *testing.T) {
	thresholds := []float64{-10, -1, -0.01, 0, 0.1, 0.5, 1, 2, 10, math.NaN(), math.Inf(1), math.Inf(-1)}
	topKs := []int{-100, -1, 0, 1, 2, 3, 99}
	elementCounts := []int{0, 1, 3, 10}

	for _, threshold := range thresholds {
		for _, topK := range topKs {
			for _, count := range elementCounts {
				opts := sanitizeFindOptions(types.FindOptions{Threshold: threshold, TopK: topK}, count, 3)

				if math.IsNaN(opts.Threshold) || math.IsInf(opts.Threshold, 0) {
					t.Fatalf("threshold not sanitized: threshold=%v topK=%d count=%d -> %v", threshold, topK, count, opts.Threshold)
				}
				if opts.Threshold < 0 || opts.Threshold > 1 {
					t.Fatalf("threshold out of [0,1]: threshold=%v topK=%d count=%d -> %v", threshold, topK, count, opts.Threshold)
				}

				if opts.TopK < 0 {
					t.Fatalf("top-k should never be negative: threshold=%v topK=%d count=%d -> %d", threshold, topK, count, opts.TopK)
				}
				if count >= 0 && opts.TopK > count {
					t.Fatalf("top-k should be capped by element count: threshold=%v topK=%d count=%d -> %d", threshold, topK, count, opts.TopK)
				}
			}
		}
	}
}

func TestMatchers_AdversarialOptions_DoNotProduceInvalidScores(t *testing.T) {
	ctx := context.Background()
	thresholds := []float64{-100, -1, -0.1, 0, 0.3, 0.8, 1, 2, math.NaN(), math.Inf(1), math.Inf(-1)}
	topKs := []int{-10, 0, 1, 2, 50}

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Sign in"},
		{Ref: "e1", Role: "link", Name: "Create account"},
		{Ref: "e2", Role: "textbox", Name: "Email"},
	}

	matchers := []types.ElementMatcher{
		NewLexicalMatcher(),
		NewEmbeddingMatcher(NewHashingEmbedder(128)),
		NewCombinedMatcher(NewHashingEmbedder(128)),
	}

	for _, m := range matchers {
		for _, threshold := range thresholds {
			for _, topK := range topKs {
				opts := types.FindOptions{Threshold: threshold, TopK: topK}
				res, err := m.Find(ctx, "sign in button", elements, opts)
				if err != nil {
					t.Fatalf("matcher %s returned error for threshold=%v topK=%d: %v", m.Strategy(), threshold, topK, err)
				}

				sanitized := sanitizeFindOptions(opts, len(elements), 3)
				if len(res.Matches) > sanitized.TopK {
					t.Fatalf("matcher %s exceeded top-k: threshold=%v topK=%d sanitizedTopK=%d got=%d", m.Strategy(), threshold, topK, sanitized.TopK, len(res.Matches))
				}

				if math.IsNaN(res.BestScore) || math.IsInf(res.BestScore, 0) {
					t.Fatalf("matcher %s produced invalid best score: %v", m.Strategy(), res.BestScore)
				}
				if res.BestScore < 0 || res.BestScore > 1 {
					t.Fatalf("matcher %s produced out-of-range best score: %v", m.Strategy(), res.BestScore)
				}

				for _, match := range res.Matches {
					if math.IsNaN(match.Score) || math.IsInf(match.Score, 0) {
						t.Fatalf("matcher %s produced invalid score for ref %s: %v", m.Strategy(), match.Ref, match.Score)
					}
					if match.Score < 0 || match.Score > 1 {
						t.Fatalf("matcher %s produced out-of-range score for ref %s: %v", m.Strategy(), match.Ref, match.Score)
					}
				}
			}
		}
	}
}
