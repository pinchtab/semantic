package engine

import (
	"context"
	"testing"

	"github.com/pinchtab/semantic/internal/types"
)

func TestParseVisualQueryHints_BasicPatterns(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantBase    string
		wantTop     bool
		wantBottom  bool
		wantLeft    bool
		wantRight   bool
		wantAbove   string
		wantBelow   string
		wantHasHint bool
	}{
		{
			name:        "top right corner",
			query:       "button in top right corner",
			wantBase:    "button",
			wantTop:     true,
			wantRight:   true,
			wantHasHint: true,
		},
		{
			name:        "below anchor",
			query:       "link below the search box",
			wantBase:    "link",
			wantBelow:   "search box",
			wantHasHint: true,
		},
		{
			name:        "left side",
			query:       "sidebar on the left",
			wantBase:    "sidebar",
			wantLeft:    true,
			wantHasHint: true,
		},
		{
			name:        "plain query",
			query:       "submit button",
			wantBase:    "submit button",
			wantHasHint: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVisualQueryHints(tt.query)
			if got.baseQuery != tt.wantBase {
				t.Fatalf("base query mismatch: want=%q got=%q", tt.wantBase, got.baseQuery)
			}
			if got.top != tt.wantTop || got.bottom != tt.wantBottom || got.left != tt.wantLeft || got.right != tt.wantRight {
				t.Fatalf("directional hints mismatch: got top=%v bottom=%v left=%v right=%v", got.top, got.bottom, got.left, got.right)
			}
			if got.aboveAnchor != tt.wantAbove || got.belowAnchor != tt.wantBelow {
				t.Fatalf("anchor mismatch: got above=%q below=%q", got.aboveAnchor, got.belowAnchor)
			}
			if got.hasHints != tt.wantHasHint {
				t.Fatalf("hasHints mismatch: want=%v got=%v", tt.wantHasHint, got.hasHints)
			}
		})
	}
}

func TestCombinedMatcher_VisualHint_TopRightCorner(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "btn-left-top", Role: "button", Name: "Open", Positional: types.PositionalHints{X: 20, Y: 20, Width: 80, Height: 24}},
		{Ref: "btn-right-top", Role: "button", Name: "Open", Positional: types.PositionalHints{X: 880, Y: 30, Width: 80, Height: 24}},
		{Ref: "btn-right-bottom", Role: "button", Name: "Open", Positional: types.PositionalHints{X: 860, Y: 620, Width: 80, Height: 24}},
	}

	res, err := m.Find(context.Background(), "button in top right corner", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if res.BestRef != "btn-right-top" {
		t.Fatalf("expected top-right button, got %s", res.BestRef)
	}
}

func TestCombinedMatcher_VisualHint_BelowAnchor(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "search", Role: "searchbox", Name: "Search", Positional: types.PositionalHints{X: 120, Y: 40, Width: 320, Height: 32}},
		{Ref: "link-top", Role: "link", Name: "Help", Positional: types.PositionalHints{X: 140, Y: 10, Width: 70, Height: 20}},
		{Ref: "link-bottom", Role: "link", Name: "Help", Positional: types.PositionalHints{X: 140, Y: 160, Width: 70, Height: 20}},
	}

	res, err := m.Find(context.Background(), "link below the search box", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if res.BestRef != "link-bottom" {
		t.Fatalf("expected link below anchor, got %s", res.BestRef)
	}
}

func TestCombinedMatcher_VisualHint_LeftSidebar(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "sidebar-left", Role: "navigation", Name: "Sidebar", Positional: types.PositionalHints{X: 10, Y: 120, Width: 200, Height: 600}},
		{Ref: "sidebar-right", Role: "navigation", Name: "Sidebar", Positional: types.PositionalHints{X: 980, Y: 120, Width: 200, Height: 600}},
	}

	res, err := m.Find(context.Background(), "sidebar on the left", elements, types.FindOptions{Threshold: 0, TopK: 2})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if res.BestRef != "sidebar-left" {
		t.Fatalf("expected left sidebar, got %s", res.BestRef)
	}
}

func TestCombinedMatcher_VisualHint_BottomFallbackWithoutCoordinates(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "button-1", Role: "button", Name: "Submit", Positional: types.PositionalHints{SiblingIndex: 0, SiblingCount: 3}},
		{Ref: "button-2", Role: "button", Name: "Submit", Positional: types.PositionalHints{SiblingIndex: 1, SiblingCount: 3}},
		{Ref: "button-3", Role: "button", Name: "Submit", Positional: types.PositionalHints{SiblingIndex: 2, SiblingCount: 3}},
	}

	res, err := m.Find(context.Background(), "button at bottom of page", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if res.BestRef != "button-3" {
		t.Fatalf("expected bottom-most fallback button, got %s", res.BestRef)
	}
}
