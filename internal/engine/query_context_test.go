package engine

import (
	"context"
	"testing"

	"github.com/pinchtab/semantic/internal/types"
)

func TestParseQueryContext_BasicPatterns(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantPositive []string
		wantNegative []string
		wantExclude  []string
		wantHasScope bool
	}{
		{
			name:         "plain negative tokens",
			query:        "button not submit",
			wantPositive: []string{"button"},
			wantNegative: []string{"submit"},
		},
		{
			name:         "context exclusion in header",
			query:        "submit button not in header",
			wantPositive: []string{"submit", "button"},
			wantExclude:  []string{"header"},
			wantHasScope: true,
		},
		{
			name:         "context exclusion with filler tail",
			query:        "login link, not the footer one",
			wantPositive: []string{"login", "link"},
			wantExclude:  []string{"footer"},
			wantHasScope: true,
		},
		{
			name:         "excluding sidebar",
			query:        "search box excluding sidebar",
			wantPositive: []string{"search", "box"},
			wantExclude:  []string{"sidebar"},
			wantHasScope: true,
		},
		{
			name:         "leading not stays literal",
			query:        "not now button",
			wantPositive: []string{"not", "now", "button"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseQueryContext(tt.query)
			assertTokens(t, got.Base.Positive, tt.wantPositive, "positive")
			assertTokens(t, got.Base.Negative, tt.wantNegative, "negative")
			assertTokens(t, got.Exclude, tt.wantExclude, "exclude")
			if got.HasScope != tt.wantHasScope {
				t.Fatalf("HasScope mismatch: got=%v want=%v", got.HasScope, tt.wantHasScope)
			}
		})
	}
}

func TestMatchesExcludedContext(t *testing.T) {
	el := types.ElementDescriptor{
		Ref:     "e1",
		Role:    "button",
		Name:    "Submit",
		Parent:  "Account Header",
		Section: "Top Header Actions",
		Positional: types.PositionalHints{
			LabelledBy: "Header controls",
		},
	}

	if !matchesExcludedContext(el, []string{"header"}) {
		t.Fatalf("expected header exclusion to match")
	}
	if !matchesExcludedContext(el, []string{"top", "header"}) {
		t.Fatalf("expected multi-token exclusion to match")
	}
	if matchesExcludedContext(el, []string{"sidebar"}) {
		t.Fatalf("did not expect unrelated exclusion to match")
	}
}

func TestNegativeContextAcrossMatchers(t *testing.T) {
	elements := []types.ElementDescriptor{
		{Ref: "header-submit", Role: "button", Name: "Submit", Section: "Header"},
		{Ref: "main-submit", Role: "button", Name: "Submit", Section: "Checkout content"},
		{Ref: "footer-submit", Role: "button", Name: "Submit", Section: "Footer"},
	}

	queries := []string{
		"submit button not in header",
		"submit button except footer",
	}

	matchers := []types.ElementMatcher{
		NewLexicalMatcher(),
		NewEmbeddingMatcher(NewHashingEmbedder(128)),
		NewCombinedMatcher(NewHashingEmbedder(128)),
	}

	for _, matcher := range matchers {
		for _, query := range queries {
			res, err := matcher.Find(context.Background(), query, elements, types.FindOptions{Threshold: 0, TopK: 3})
			if err != nil {
				t.Fatalf("%s Find failed for %q: %v", matcher.Strategy(), query, err)
			}
			for _, match := range res.Matches {
				if query == "submit button not in header" && match.Ref == "header-submit" {
					t.Fatalf("%s should exclude header match for %q", matcher.Strategy(), query)
				}
				if query == "submit button except footer" && match.Ref == "footer-submit" {
					t.Fatalf("%s should exclude footer match for %q", matcher.Strategy(), query)
				}
			}
		}
	}
}

func TestDuplicateRegionDisambiguation(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "login-header", Role: "link", Name: "Log in", Section: "Header"},
		{Ref: "login-footer", Role: "link", Name: "Log in", Section: "Footer"},
		{Ref: "login-sidebar", Role: "link", Name: "Log in", Section: "Sticky Sidebar Quick Actions"},
	}

	res, err := m.Find(context.Background(), "login link, not the footer one", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	for _, match := range res.Matches {
		if match.Ref == "login-footer" {
			t.Fatalf("expected footer variant to be excluded")
		}
	}

	res2, err := m.Find(context.Background(), "login link except sticky sidebar quick actions", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	for _, match := range res2.Matches {
		if match.Ref == "login-sidebar" {
			t.Fatalf("expected sidebar variant to be excluded")
		}
	}
}

func assertTokens(t *testing.T, got, want []string, label string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s length mismatch: got=%v want=%v", label, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s mismatch: got=%v want=%v", label, got, want)
		}
	}
}
