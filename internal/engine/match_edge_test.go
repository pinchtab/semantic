package engine

// Edge case tests.
// Verify graceful handling of empty queries, gibberish, all-stopword input,
// very long queries, and single-character queries.

import (
	"github.com/pinchtab/semantic/internal/types"
	"context"
	"testing"
)

// CATEGORY 5: Edge Cases

// CATEGORY 5: Edge Cases

func TestCombined_EdgeCase_EmptyQuery(t *testing.T) {
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{{Ref: "e1", Role: "button", Name: "Submit"}}

	result, err := matcher.Find(context.Background(), "", elements, types.FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) > 0 {
		t.Errorf("expected no matches for empty query, got %d", len(result.Matches))
	}
}

func TestCombined_EdgeCase_GibberishQuery(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "xyzzy plugh qwerty", sites["wikipedia"], types.FindOptions{
		Threshold: 0.3,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Gibberish query: matches=%d best_score=%.3f", len(result.Matches), result.BestScore)
	// Should return no matches at threshold 0.3
	if len(result.Matches) > 0 {
		t.Errorf("expected no matches for gibberish query at threshold 0.3, got %d", len(result.Matches))
	}
}

func TestCombined_EdgeCase_AllStopwords(t *testing.T) {
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Submit"},
		{Ref: "e2", Role: "link", Name: "The"},
	}

	result, err := matcher.Find(context.Background(), "the a is", elements, types.FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("All-stopwords query: matches=%d", len(result.Matches))
}

func TestCombined_EdgeCase_VeryLongQuery(t *testing.T) {
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Submit"},
	}

	longQuery := "I want to find the submit button that is located on the bottom right of the page and click on it to submit the form"
	result, err := matcher.Find(context.Background(), longQuery, elements, types.FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Long query: matches=%d best_score=%.3f", len(result.Matches), result.BestScore)
	if result.BestRef != "e1" {
		t.Errorf("expected long query to still find 'Submit' button, got %s", result.BestRef)
	}
}

func TestCombined_EdgeCase_SingleCharQuery(t *testing.T) {
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "e1", Role: "link", Name: "X"},
		{Ref: "e2", Role: "button", Name: "Close"},
	}

	result, err := matcher.Find(context.Background(), "x", elements, types.FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Single char 'x': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
}

// CATEGORY 6: Role Boost Accumulation Test (Bug Fix)


// Phase 3: CombinedMatcher tests
