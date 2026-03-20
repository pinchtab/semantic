package semantic

// Partial match and abbreviation tests.
// Verify that Find() handles shortened forms ("btn" → "button", "nav" → "navigation").

import (
	"context"
	"testing"
)

// CATEGORY 4: Partial Match / Abbreviation Tests

// CATEGORY 4: Partial Match / Abbreviation Tests

func TestCombined_Partial_Btn(t *testing.T) {
	elements := []ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Submit"},
		{Ref: "e2", Role: "link", Name: "Home"},
		{Ref: "e3", Role: "textbox", Name: "Email"},
	}
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "submit btn", elements, FindOptions{
		Threshold: 0.15,
		TopK:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='submit btn': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	if result.BestRef != "e1" {
		t.Errorf("expected 'submit btn' to match 'Submit' button (e1), got %s", result.BestRef)
	}
}

func TestCombined_Partial_Nav(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "nav menu", sites["ecommerce"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='nav menu': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "nav menu" should match "Main navigation" (e13) via prefix + synonym
	if result.BestRef != "e13" {
		t.Errorf("expected 'nav menu' to match 'Main navigation' (e13), got %s", result.BestRef)
	}
}

func TestCombined_Partial_Qty(t *testing.T) {
	sites := buildRealWorldElements()
	matcher := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := matcher.Find(context.Background(), "qty", sites["ecommerce"], FindOptions{
		Threshold: 0.15,
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Query='qty': BestRef=%s Score=%.3f", result.BestRef, result.BestScore)
	for _, m := range result.Matches {
		t.Logf("  match: ref=%s score=%.3f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
	}
	// "qty" should match "Quantity" (e11) via synonym
	if result.BestRef != "e11" {
		t.Errorf("expected 'qty' to match 'Quantity' (e11), got %s", result.BestRef)
	}
}

// CATEGORY 5: Edge Cases

// CATEGORY 5: Edge Cases
