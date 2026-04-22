package engine

import (
	"context"
	"testing"

	"github.com/pinchtab/semantic/internal/types"
)

func TestParseOrdinalConstraint_BasicPatterns(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantBase   string
		wantHasOrd bool
		wantPos    int
		wantIsLast bool
	}{
		{
			name:       "second button",
			query:      "second button",
			wantBase:   "button",
			wantHasOrd: true,
			wantPos:    2,
		},
		{
			name:       "numeric ordinal",
			query:      "3rd menu item",
			wantBase:   "menu item",
			wantHasOrd: true,
			wantPos:    3,
		},
		{
			name:       "last input field",
			query:      "last input field",
			wantBase:   "input field",
			wantHasOrd: true,
			wantIsLast: true,
		},
		{
			name:       "non ordinal content query",
			query:      "first name",
			wantBase:   "first name",
			wantHasOrd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, base := parseOrdinalConstraint(tt.query)
			if base != tt.wantBase {
				t.Fatalf("base query mismatch: want=%q got=%q", tt.wantBase, base)
			}
			if got.HasOrdinal != tt.wantHasOrd {
				t.Fatalf("HasOrdinal mismatch: want=%v got=%v", tt.wantHasOrd, got.HasOrdinal)
			}
			if got.Position != tt.wantPos {
				t.Fatalf("position mismatch: want=%d got=%d", tt.wantPos, got.Position)
			}
			if got.Last != tt.wantIsLast {
				t.Fatalf("last mismatch: want=%v got=%v", tt.wantIsLast, got.Last)
			}
		})
	}
}

func TestParseQueryContext_WithOrdinalAndNegativeScope(t *testing.T) {
	ctx := ParseQueryContext("second button not in header")
	if !ctx.Ordinal.HasOrdinal || ctx.Ordinal.Position != 2 {
		t.Fatalf("expected second ordinal, got %+v", ctx.Ordinal)
	}
	assertTokens(t, ctx.Base.Positive, []string{"button"}, "positive")
	assertTokens(t, ctx.Base.Negative, []string{}, "negative")
	assertTokens(t, ctx.Exclude, []string{"header"}, "exclude")
	if !ctx.HasScope {
		t.Fatalf("expected scope exclusion to be detected")
	}
}

func TestCombinedMatcher_OrdinalQuery_SecondButton(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "btn-1", Role: "button", Name: "Action", Positional: types.PositionalHints{SiblingIndex: 0}},
		{Ref: "btn-2", Role: "button", Name: "Action", Positional: types.PositionalHints{SiblingIndex: 1}},
		{Ref: "btn-3", Role: "button", Name: "Action", Positional: types.PositionalHints{SiblingIndex: 2}},
	}

	res, err := m.Find(context.Background(), "second button", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 1 {
		t.Fatalf("expected one ordinal-selected match, got %d", len(res.Matches))
	}
	if res.BestRef != "btn-2" {
		t.Fatalf("expected second button btn-2, got %s", res.BestRef)
	}
}

func TestCombinedMatcher_OrdinalQuery_LastInputField(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "input-1", Role: "textbox", Name: "Email", Positional: types.PositionalHints{SiblingIndex: 0}},
		{Ref: "input-2", Role: "textbox", Name: "Email", Positional: types.PositionalHints{SiblingIndex: 1}},
		{Ref: "input-3", Role: "textbox", Name: "Email", Positional: types.PositionalHints{SiblingIndex: 2}},
	}

	res, err := m.Find(context.Background(), "last input field", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 1 {
		t.Fatalf("expected one ordinal-selected match, got %d", len(res.Matches))
	}
	if res.BestRef != "input-3" {
		t.Fatalf("expected last input field input-3, got %s", res.BestRef)
	}
}

func TestCombinedMatcher_OrdinalQuery_OutOfRangeReturnsNoMatch(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "b1", Role: "button", Name: "Continue", Positional: types.PositionalHints{SiblingIndex: 0}},
		{Ref: "b2", Role: "button", Name: "Continue", Positional: types.PositionalHints{SiblingIndex: 1}},
		{Ref: "b3", Role: "button", Name: "Continue", Positional: types.PositionalHints{SiblingIndex: 2}},
	}

	res, err := m.Find(context.Background(), "fifth button", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 0 {
		t.Fatalf("expected no matches for out-of-range ordinal, got %d", len(res.Matches))
	}
	if res.BestRef != "" || res.BestScore != 0 {
		t.Fatalf("expected empty best match for out-of-range ordinal, got ref=%q score=%f", res.BestRef, res.BestScore)
	}
}

func TestCombinedMatcher_OrdinalGuard_DoesNotTreatFirstNameAsOrdinal(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "first-name", Role: "textbox", Name: "First Name", Positional: types.PositionalHints{SiblingIndex: 3}},
		{Ref: "last-name", Role: "textbox", Name: "Last Name", Positional: types.PositionalHints{SiblingIndex: 0}},
	}

	res, err := m.Find(context.Background(), "first name", elements, types.FindOptions{Threshold: 0, TopK: 2})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) == 0 {
		t.Fatalf("expected at least one match")
	}
	if res.BestRef != "first-name" {
		t.Fatalf("expected semantic match for 'first name', got %s", res.BestRef)
	}
}

func TestCombinedMatcher_OrdinalWithContextExclusion(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "header-btn", Role: "button", Name: "Submit", Section: "Header", Positional: types.PositionalHints{SiblingIndex: 0}},
		{Ref: "main-btn-1", Role: "button", Name: "Submit", Section: "Main", Positional: types.PositionalHints{SiblingIndex: 1}},
		{Ref: "main-btn-2", Role: "button", Name: "Submit", Section: "Main", Positional: types.PositionalHints{SiblingIndex: 2}},
	}

	res, err := m.Find(context.Background(), "second button not in header", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) != 1 {
		t.Fatalf("expected one ordinal-selected match, got %d", len(res.Matches))
	}
	if res.BestRef != "main-btn-2" {
		t.Fatalf("expected second non-header button, got %s", res.BestRef)
	}
}
