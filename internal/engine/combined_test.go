package engine

import (
	"context"
	"fmt"
	"github.com/pinchtab/semantic/internal/types"
	"testing"
)

// CATEGORY 6: Role Boost Accumulation Test (Bug Fix)

// Phase 3: CombinedMatcher tests

func TestCombinedMatcher_Strategy(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	want := "combined:lexical+embedding:hashing"
	if m.Strategy() != want {
		t.Errorf("expected strategy=%s, got %s", want, m.Strategy())
	}
}

func TestCombinedMatcher_Find(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Log In"},
		{Ref: "e1", Role: "link", Name: "Sign Up"},
		{Ref: "e2", Role: "textbox", Name: "Email Address"},
	}

	result, err := m.Find(context.Background(), "log in button", elements, types.FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if result.ElementCount != 3 {
		t.Errorf("expected ElementCount=3, got %d", result.ElementCount)
	}
	if result.BestRef != "e0" {
		t.Errorf("expected BestRef=e0, got %s", result.BestRef)
	}
	if result.BestScore <= 0 {
		t.Errorf("expected positive BestScore, got %f", result.BestScore)
	}
	if result.Strategy != "combined:lexical+embedding:hashing" {
		t.Errorf("expected combined strategy, got %s", result.Strategy)
	}
}

func TestCombinedMatcher_ThresholdFiltering(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Submit"},
		{Ref: "e1", Role: "link", Name: "Home"},
	}

	result, err := m.Find(context.Background(), "submit button", elements, types.FindOptions{
		Threshold: 0.99,
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	for _, match := range result.Matches {
		if match.Score < 0.99 {
			t.Errorf("match %s has score %f below threshold", match.Ref, match.Score)
		}
	}
}

func TestCombinedMatcher_TopK(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Submit"},
		{Ref: "e1", Role: "button", Name: "Cancel"},
		{Ref: "e2", Role: "button", Name: "Reset"},
		{Ref: "e3", Role: "link", Name: "Home"},
		{Ref: "e4", Role: "textbox", Name: "Name"},
	}

	result, err := m.Find(context.Background(), "button", elements, types.FindOptions{
		Threshold: 0.01,
		TopK:      2,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(result.Matches) > 2 {
		t.Errorf("expected at most 2 matches (TopK=2), got %d", len(result.Matches))
	}
}

func TestCombinedMatcher_DeterministicTieBreak(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "first", Role: "button", Name: "Open", Positional: types.PositionalHints{Depth: 2, SiblingIndex: 0}},
		{Ref: "second", Role: "button", Name: "Open", Positional: types.PositionalHints{Depth: 2, SiblingIndex: 0}},
	}

	for i := 0; i < 100; i++ {
		result, err := m.Find(context.Background(), "open button", elements, types.FindOptions{Threshold: 0, TopK: 2})
		if err != nil {
			t.Fatalf("Find returned error: %v", err)
		}
		if result.BestRef != "first" {
			t.Fatalf("run %d: expected BestRef=first, got %s", i, result.BestRef)
		}
	}
}

func TestCombinedMatcher_ScoresDescending(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Login"},
		{Ref: "e1", Role: "textbox", Name: "Username"},
		{Ref: "e2", Role: "link", Name: "Forgot Password"},
		{Ref: "e3", Role: "heading", Name: "Welcome Page"},
	}

	result, err := m.Find(context.Background(), "login button", elements, types.FindOptions{
		Threshold: 0.01,
		TopK:      10,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	for i := 1; i < len(result.Matches); i++ {
		if result.Matches[i].Score > result.Matches[i-1].Score {
			t.Errorf("matches not sorted descending: [%d]=%f > [%d]=%f",
				i, result.Matches[i].Score, i-1, result.Matches[i-1].Score)
		}
	}
}

func TestCombinedMatcher_FusesBothStrategies(t *testing.T) {
	combined := NewCombinedMatcher(NewHashingEmbedder(128))
	lexical := NewLexicalMatcher()
	embedding := NewEmbeddingMatcher(NewHashingEmbedder(128))

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Log In"},
		{Ref: "e1", Role: "link", Name: "Sign Up"},
	}

	ctx := context.Background()
	opts := types.FindOptions{Threshold: 0.01, TopK: 3}

	combResult, err := combined.Find(ctx, "log in", elements, opts)
	if err != nil {
		t.Fatalf("Combined.Find error: %v", err)
	}
	lexResult, err := lexical.Find(ctx, "log in", elements, opts)
	if err != nil {
		t.Fatalf("Lexical.Find error: %v", err)
	}
	embResult, err := embedding.Find(ctx, "log in", elements, opts)
	if err != nil {
		t.Fatalf("Embedding.Find error: %v", err)
	}

	// Combined score should differ from both pure strategies (it's a weighted fusion).
	if combResult.BestScore == lexResult.BestScore && combResult.BestScore == embResult.BestScore { //nolint:gocritic // intentional equality check
		t.Errorf("combined score (%.4f) identical to both lexical (%.4f) and embedding (%.4f) — fusion not working",
			combResult.BestScore, lexResult.BestScore, embResult.BestScore)
	}

	// All three should identify the same best ref.
	if combResult.BestRef != "e0" {
		t.Errorf("expected BestRef=e0, got %s", combResult.BestRef)
	}
}

func TestCombinedMatcher_NoElements(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	result, err := m.Find(context.Background(), "anything", nil, types.FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(result.Matches) != 0 {
		t.Errorf("expected no matches for empty elements, got %d", len(result.Matches))
	}
	if result.BestRef != "" {
		t.Errorf("expected empty BestRef, got %s", result.BestRef)
	}
}

// Phase 3: Complex UI test scenarios

// complexFormElements returns a realistic form page with 15+ elements.
func complexFormElements() []types.ElementDescriptor {
	return []types.ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Registration Form"},
		{Ref: "e1", Role: "textbox", Name: "First Name"},
		{Ref: "e2", Role: "textbox", Name: "Last Name"},
		{Ref: "e3", Role: "textbox", Name: "Email Address"},
		{Ref: "e4", Role: "textbox", Name: "Password", Value: ""},
		{Ref: "e5", Role: "textbox", Name: "Confirm Password"},
		{Ref: "e6", Role: "combobox", Name: "Country"},
		{Ref: "e7", Role: "checkbox", Name: "I agree to the Terms of Service"},
		{Ref: "e8", Role: "checkbox", Name: "Subscribe to newsletter"},
		{Ref: "e9", Role: "button", Name: "Submit Registration"},
		{Ref: "e10", Role: "button", Name: "Cancel"},
		{Ref: "e11", Role: "link", Name: "Already have an account? Log in"},
		{Ref: "e12", Role: "link", Name: "Privacy Policy"},
		{Ref: "e13", Role: "link", Name: "Terms of Service"},
		{Ref: "e14", Role: "img", Name: "Company Logo"},
		{Ref: "e15", Role: "navigation", Name: "Main Navigation"},
	}
}

// complexTableElements returns a data table with columns and actions.
func complexTableElements() []types.ElementDescriptor {
	return []types.ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "User Management"},
		{Ref: "e1", Role: "search", Name: "Search Users"},
		{Ref: "e2", Role: "button", Name: "Add New User"},
		{Ref: "e3", Role: "button", Name: "Export CSV"},
		{Ref: "e4", Role: "table", Name: "Users Table"},
		{Ref: "e5", Role: "columnheader", Name: "Name"},
		{Ref: "e6", Role: "columnheader", Name: "Email"},
		{Ref: "e7", Role: "columnheader", Name: "Role"},
		{Ref: "e8", Role: "columnheader", Name: "Status"},
		{Ref: "e9", Role: "columnheader", Name: "Actions"},
		{Ref: "e10", Role: "cell", Name: "John Doe", Value: "john@pinchtab.com"},
		{Ref: "e11", Role: "button", Name: "Edit", Value: "John Doe"},
		{Ref: "e12", Role: "button", Name: "Delete", Value: "John Doe"},
		{Ref: "e13", Role: "cell", Name: "Jane Smith", Value: "jane@pinchtab.com"},
		{Ref: "e14", Role: "button", Name: "Edit", Value: "Jane Smith"},
		{Ref: "e15", Role: "button", Name: "Delete", Value: "Jane Smith"},
		{Ref: "e16", Role: "button", Name: "Previous Page"},
		{Ref: "e17", Role: "button", Name: "Next Page"},
		{Ref: "e18", Role: "combobox", Name: "Rows per page", Value: "10"},
	}
}

// complexModalElements returns a page with a modal dialog overlay.
func complexModalElements() []types.ElementDescriptor {
	return []types.ElementDescriptor{
		{Ref: "e0", Role: "heading", Name: "Dashboard"},
		{Ref: "e1", Role: "button", Name: "Settings"},
		{Ref: "e2", Role: "button", Name: "Notifications"},
		{Ref: "e3", Role: "dialog", Name: "Confirm Delete"},
		{Ref: "e4", Role: "heading", Name: "Are you sure?"},
		{Ref: "e5", Role: "text", Name: "This action cannot be undone. The item will be permanently deleted."},
		{Ref: "e6", Role: "button", Name: "Yes, Delete"},
		{Ref: "e7", Role: "button", Name: "Cancel"},
		{Ref: "e8", Role: "button", Name: "Close Dialog"},
		{Ref: "e9", Role: "navigation", Name: "Sidebar Menu"},
		{Ref: "e10", Role: "link", Name: "Home"},
		{Ref: "e11", Role: "link", Name: "Reports"},
		{Ref: "e12", Role: "link", Name: "Settings"},
	}
}

func TestCombinedMatcher_ComplexForm(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := complexFormElements()

	tests := []struct {
		query   string
		wantRef string
		desc    string
	}{
		{"submit registration", "e9", "should find the submit button"},
		{"email field", "e3", "should find email textbox"},
		{"terms checkbox", "e7", "should find terms of service checkbox"},
		{"password input", "e4", "should find password field"},
		{"cancel button", "e10", "should find cancel button"},
		{"log in link", "e11", "should find the login link"},
		{"country dropdown", "e6", "should find country combobox"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := m.Find(context.Background(), tt.query, elements, types.FindOptions{
				Threshold: 0.01,
				TopK:      3,
			})
			if err != nil {
				t.Fatalf("Find error: %v", err)
			}
			if result.BestRef != tt.wantRef {
				t.Errorf("query=%q: expected BestRef=%s, got %s (score=%f)",
					tt.query, tt.wantRef, result.BestRef, result.BestScore)
				for _, m := range result.Matches {
					t.Logf("  match: ref=%s score=%f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
				}
			}
		})
	}
}

func TestCombinedMatcher_ComplexTable(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := complexTableElements()

	tests := []struct {
		query   string
		wantRef string
		desc    string
	}{
		{"search users", "e1", "should find the search box"},
		{"add new user", "e2", "should find the add button"},
		{"export csv", "e3", "should find the export button"},
		{"next page", "e17", "should find the next page button"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := m.Find(context.Background(), tt.query, elements, types.FindOptions{
				Threshold: 0.01,
				TopK:      3,
			})
			if err != nil {
				t.Fatalf("Find error: %v", err)
			}
			if result.BestRef != tt.wantRef {
				t.Errorf("query=%q: expected BestRef=%s, got %s (score=%f)",
					tt.query, tt.wantRef, result.BestRef, result.BestScore)
				for _, m := range result.Matches {
					t.Logf("  match: ref=%s score=%f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
				}
			}
		})
	}
}

func TestCombinedMatcher_ComplexModal(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := complexModalElements()

	tests := []struct {
		query   string
		wantRef string
		desc    string
	}{
		{"delete button", "e6", "should find the yes delete button in modal"},
		{"close dialog", "e8", "should find the close dialog button"},
		{"cancel", "e7", "should find the cancel button in modal"},
		{"settings button", "e1", "should find the settings button"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result, err := m.Find(context.Background(), tt.query, elements, types.FindOptions{
				Threshold: 0.01,
				TopK:      3,
			})
			if err != nil {
				t.Fatalf("Find error: %v", err)
			}
			if result.BestRef != tt.wantRef {
				t.Errorf("query=%q: expected BestRef=%s, got %s (score=%f)",
					tt.query, tt.wantRef, result.BestRef, result.BestScore)
				for _, m := range result.Matches {
					t.Logf("  match: ref=%s score=%f role=%s name=%s", m.Ref, m.Score, m.Role, m.Name)
				}
			}
		})
	}
}

// Phase 3: Benchmark tests

func BenchmarkLexicalMatcher_Find(b *testing.B) {
	m := NewLexicalMatcher()
	elements := complexFormElements()
	opts := types.FindOptions{Threshold: 0.1, TopK: 3}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "submit registration button", elements, opts)
	}
}

func BenchmarkHashingEmbedder_Embed(b *testing.B) {
	e := NewHashingEmbedder(128)
	texts := []string{"submit registration button"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = e.Embed(texts)
	}
}

func BenchmarkHashingEmbedder_EmbedBatch(b *testing.B) {
	e := NewHashingEmbedder(128)
	elements := complexFormElements()
	texts := make([]string, len(elements))
	for i, el := range elements {
		texts[i] = el.Composite()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = e.Embed(texts)
	}
}

func BenchmarkEmbeddingMatcher_Find(b *testing.B) {
	m := NewEmbeddingMatcher(NewHashingEmbedder(128))
	elements := complexFormElements()
	opts := types.FindOptions{Threshold: 0.1, TopK: 3}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "submit registration button", elements, opts)
	}
}

func BenchmarkCombinedMatcher_Find(b *testing.B) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := complexFormElements()
	opts := types.FindOptions{Threshold: 0.1, TopK: 3}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "submit registration button", elements, opts)
	}
}

func BenchmarkCombinedMatcher_LargeElementSet(b *testing.B) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	// Build a large element set (100 elements) simulating a complex page.
	elements := make([]types.ElementDescriptor, 100)
	roles := []string{"button", "link", "textbox", "heading", "img", "checkbox", "combobox"}
	for i := 0; i < 100; i++ {
		elements[i] = types.ElementDescriptor{
			Ref:  fmt.Sprintf("e%d", i),
			Role: roles[i%len(roles)],
			Name: fmt.Sprintf("Element %d action item", i),
		}
	}
	opts := types.FindOptions{Threshold: 0.1, TopK: 5}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.Find(ctx, "click the action button number 42", elements, opts)
	}
}

func TestCombinedMatcher_WeightsApplied(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))

	// Override weights to emphasize embedding.
	m.LexicalWeight = 0.2
	m.EmbeddingWeight = 0.8

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Log In"},
		{Ref: "e1", Role: "link", Name: "Sign Up"},
	}

	result, err := m.Find(context.Background(), "log in", elements, types.FindOptions{
		Threshold: 0.01,
		TopK:      3,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if result.ElementCount != 2 {
		t.Errorf("expected ElementCount=2, got %d", result.ElementCount)
	}
	if result.BestRef != "e0" {
		t.Errorf("expected BestRef=e0, got %s", result.BestRef)
	}
}
