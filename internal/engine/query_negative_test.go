package engine

import (
	"context"
	"testing"

	"github.com/pinchtab/semantic/internal/types"
)

func TestParseNegativeConstraints_BasicPatterns(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		wantBase      string
		wantExclusion string
	}{
		{
			name:          "not in",
			query:         "submit button not in header",
			wantBase:      "submit button",
			wantExclusion: "header",
		},
		{
			name:          "comma not the one",
			query:         "login link, not the footer one",
			wantBase:      "login link",
			wantExclusion: "footer",
		},
		{
			name:          "excluding",
			query:         "search box excluding sidebar",
			wantBase:      "search box",
			wantExclusion: "sidebar",
		},
		{
			name:          "except",
			query:         "menu item except sticky footer",
			wantBase:      "menu item",
			wantExclusion: "sticky footer",
		},
		{
			name:          "no negative qualifier",
			query:         "plain submit button",
			wantBase:      "plain submit button",
			wantExclusion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNegativeConstraints(tt.query)
			if got.baseQuery != tt.wantBase {
				t.Fatalf("base query mismatch: want=%q got=%q", tt.wantBase, got.baseQuery)
			}
			if got.exclusionPhrase != tt.wantExclusion {
				t.Fatalf("exclusion mismatch: want=%q got=%q", tt.wantExclusion, got.exclusionPhrase)
			}
		})
	}
}

func TestShouldExcludeElement_ContextSignals(t *testing.T) {
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

	if !shouldExcludeElement(el, "header") {
		t.Fatalf("expected header exclusion to match")
	}
	if !shouldExcludeElement(el, "top header") {
		t.Fatalf("expected multi-token exclusion to match")
	}
	if shouldExcludeElement(el, "sidebar") {
		t.Fatalf("did not expect unrelated exclusion to match")
	}
}

func TestLexicalMatcher_NegativeQualifierFiltersExcludedSection(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "header-submit", Role: "button", Name: "Submit", Section: "Header"},
		{Ref: "main-submit", Role: "button", Name: "Submit", Section: "Checkout content"},
		{Ref: "footer-submit", Role: "button", Name: "Submit", Section: "Footer"},
	}

	res, err := m.Find(context.Background(), "submit button not in header", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if len(res.Matches) == 0 {
		t.Fatalf("expected at least one match")
	}
	for _, match := range res.Matches {
		if match.Ref == "header-submit" {
			t.Fatalf("expected excluded header element to be filtered out")
		}
	}
}

func TestEmbeddingMatcher_NegativeQualifierFiltersSidebarVariant(t *testing.T) {
	m := NewEmbeddingMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "main-search", Role: "searchbox", Name: "Search", Section: "Main content"},
		{Ref: "sidebar-search", Role: "searchbox", Name: "Search", Section: "Sidebar"},
		{Ref: "header-search", Role: "searchbox", Name: "Search", Section: "Header"},
	}

	res, err := m.Find(context.Background(), "search box excluding sidebar", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	for _, match := range res.Matches {
		if match.Ref == "sidebar-search" {
			t.Fatalf("expected excluded sidebar element to be filtered out")
		}
	}
}

func TestCombinedMatcher_NegativeQualifier_RealWorldDuplicateLinks(t *testing.T) {
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
	if len(res.Matches) == 0 {
		t.Fatalf("expected at least one match")
	}
	for _, match := range res.Matches {
		if match.Ref == "login-footer" {
			t.Fatalf("expected excluded footer variant to be filtered out")
		}
	}

	res2, err := m.Find(context.Background(), "login link except sticky sidebar quick actions", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	for _, match := range res2.Matches {
		if match.Ref == "login-sidebar" {
			t.Fatalf("expected multi-token excluded sidebar variant to be filtered out")
		}
	}
}
