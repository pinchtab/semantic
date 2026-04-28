package engine

import (
	"context"
	"testing"

	"github.com/pinchtab/semantic/internal/types"
)

func TestParseStructuredLocator_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   structuredLocator
		wantOK bool
	}{
		{
			name:   "role only",
			raw:    "role:button",
			want:   structuredLocator{kind: locatorRole, role: "button"},
			wantOK: true,
		},
		{
			name:   "role with name",
			raw:    "role:button Sign In",
			want:   structuredLocator{kind: locatorRole, role: "button", value: "Sign In"},
			wantOK: true,
		},
		{
			name:   "role with bracketed name",
			raw:    "role:button [Sign In]",
			want:   structuredLocator{kind: locatorRole, role: "button", value: "Sign In"},
			wantOK: true,
		},
		{
			name:   "text",
			raw:    "text:Welcome back",
			want:   structuredLocator{kind: locatorText, value: "Welcome back"},
			wantOK: true,
		},
		{
			name:   "quoted placeholder",
			raw:    `placeholder:"Search products"`,
			want:   structuredLocator{kind: locatorPlaceholder, value: "Search products"},
			wantOK: true,
		},
		{
			name:   "first wrapper",
			raw:    "first:role:button",
			want:   structuredLocator{kind: locatorRole, role: "button", wrapper: locatorFirst},
			wantOK: true,
		},
		{
			name:   "last wrapper",
			raw:    "last:text:Submit",
			want:   structuredLocator{kind: locatorText, value: "Submit", wrapper: locatorLast},
			wantOK: true,
		},
		{
			name:   "nth wrapper",
			raw:    "nth:2:role:link Privacy",
			want:   structuredLocator{kind: locatorRole, role: "link", value: "Privacy", wrapper: locatorNth, nth: 2},
			wantOK: true,
		},
		{
			name:   "nth zero parses as structured locator",
			raw:    "nth:0:role:button",
			want:   structuredLocator{kind: locatorRole, role: "button", wrapper: locatorNth, nth: 0},
			wantOK: true,
		},
		{
			name:   "invalid wrapper base",
			raw:    "first:button",
			wantOK: false,
		},
		{
			name:   "natural language",
			raw:    "first button",
			wantOK: false,
		},
		{
			name:   "empty field",
			raw:    "testid:",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseStructuredLocator(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got != tt.want {
				t.Fatalf("locator=%+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestStructuredLocator_FieldSpecificMatches(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "name", Role: "button", Name: "Search"},
		{Ref: "placeholder", Role: "textbox", Name: "Query", Placeholder: "Search"},
		{Ref: "label", Role: "textbox", Name: "Email", Label: "Work Email"},
		{Ref: "alt", Role: "img", Name: "Decorative", Alt: "Company Logo"},
		{Ref: "title", Role: "button", Name: "Help", Title: "Open Help Center"},
		{Ref: "testid", Role: "button", Name: "Submit", TestID: "checkout-submit"},
	}

	tests := []struct {
		query string
		want  string
	}{
		{"placeholder:Search", "placeholder"},
		{"label:Work Email", "label"},
		{"alt:Company Logo", "alt"},
		{"title:Help Center", "title"},
		{"testid:checkout-submit", "testid"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			res, err := m.Find(context.Background(), tt.query, elements, types.FindOptions{Threshold: 0, TopK: len(elements)})
			if err != nil {
				t.Fatalf("Find returned error: %v", err)
			}
			if res.BestRef != tt.want {
				t.Fatalf("BestRef=%q, want %q; matches=%v", res.BestRef, tt.want, res.Matches)
			}
			if len(res.Matches) != 1 {
				t.Fatalf("expected field-specific locator to match one element, got %d", len(res.Matches))
			}
		})
	}
}

func TestStructuredLocator_ExactOutranksSubstring(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "substring", Role: "button", Text: "Submit order", DocumentIdx: 0},
		{Ref: "exact", Role: "button", Text: "Submit", DocumentIdx: 1},
	}

	res, err := m.Find(context.Background(), "text:Submit", elements, types.FindOptions{Threshold: 0, TopK: 2})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if res.BestRef != "exact" {
		t.Fatalf("BestRef=%q, want exact; matches=%v", res.BestRef, res.Matches)
	}
	if len(res.Matches) != 2 {
		t.Fatalf("expected two matches, got %d", len(res.Matches))
	}
	if res.Matches[0].Score <= res.Matches[1].Score {
		t.Fatalf("expected exact score above substring, got %.2f <= %.2f", res.Matches[0].Score, res.Matches[1].Score)
	}
}

func TestStructuredLocator_RoleUsesExplicitBeforeImplicitTagRole(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "implicit", Tag: "button", Name: "Save", DocumentIdx: 0},
		{Ref: "explicit", Role: "button", Tag: "div", Name: "Save", DocumentIdx: 1},
		{Ref: "overridden", Role: "link", Tag: "button", Name: "Save", DocumentIdx: 2},
	}

	res, err := m.Find(context.Background(), "role:button Save", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if res.BestRef != "explicit" {
		t.Fatalf("BestRef=%q, want explicit; matches=%v", res.BestRef, res.Matches)
	}
	if len(res.Matches) != 2 {
		t.Fatalf("expected explicit and implicit matches only, got %d: %v", len(res.Matches), res.Matches)
	}
	if _, ok := findInternalScore(res.Matches, "overridden"); ok {
		t.Fatalf("explicit role should override implicit tag role; matches=%v", res.Matches)
	}
}

func TestStructuredLocator_WrappersSelectFromOrderedCandidates(t *testing.T) {
	m := NewCombinedMatcher(NewHashingEmbedder(128))
	elements := []types.ElementDescriptor{
		{Ref: "first", Role: "button", Name: "Save", DocumentIdx: 0},
		{Ref: "second", Role: "button", Name: "Save", DocumentIdx: 1},
		{Ref: "third", Role: "button", Name: "Save", DocumentIdx: 2},
	}

	tests := []struct {
		query string
		want  string
	}{
		{"first:role:button Save", "first"},
		{"last:role:button Save", "third"},
		{"nth:2:role:button Save", "second"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			res, err := m.Find(context.Background(), tt.query, elements, types.FindOptions{Threshold: 0, TopK: 1})
			if err != nil {
				t.Fatalf("Find returned error: %v", err)
			}
			if res.BestRef != tt.want {
				t.Fatalf("BestRef=%q, want %q; matches=%v", res.BestRef, tt.want, res.Matches)
			}
			if len(res.Matches) != 1 {
				t.Fatalf("expected wrapper to select one match, got %d", len(res.Matches))
			}
		})
	}
}

func TestStructuredLocator_WrappersUseDocumentOrderNotScoreRank(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "first-partial", Role: "button", Text: "Submit order", DocumentIdx: 0},
		{Ref: "second-exact", Role: "button", Text: "Submit", DocumentIdx: 1},
	}

	tests := []struct {
		query string
		want  string
	}{
		{"first:text:Submit", "first-partial"},
		{"last:text:Submit", "second-exact"},
		{"nth:1:text:Submit", "first-partial"},
		{"nth:2:text:Submit", "second-exact"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			res, err := m.Find(context.Background(), tt.query, elements, types.FindOptions{Threshold: 0, TopK: 1})
			if err != nil {
				t.Fatalf("Find returned error: %v", err)
			}
			if res.BestRef != tt.want {
				t.Fatalf("BestRef=%q, want %q; matches=%v", res.BestRef, tt.want, res.Matches)
			}
		})
	}
}

func TestStructuredLocator_NthZeroReturnsNoMatch(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "submit", Role: "button", Text: "Submit"},
	}

	res, err := m.Find(context.Background(), "nth:0:text:Submit", elements, types.FindOptions{Threshold: 0, TopK: 1})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if len(res.Matches) != 0 {
		t.Fatalf("expected nth:0 to return no structured matches, got %v", res.Matches)
	}
	if res.BestRef != "" {
		t.Fatalf("expected empty BestRef, got %q", res.BestRef)
	}
}

func TestStructuredLocator_RoleNameUsesAccessibleNameCandidates(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "label", Role: "textbox", Name: "Input", Label: "Work Email"},
		{Ref: "labelled-by", Role: "textbox", Name: "Input", Positional: types.PositionalHints{LabelledBy: "Billing ZIP"}},
		{Ref: "text", Role: "button", Name: "Icon only", Text: "Save changes"},
		{Ref: "alt", Role: "img", Name: "Graphic", Alt: "Company Logo"},
		{Ref: "title", Role: "button", Name: "?", Title: "Open Help Center"},
		{Ref: "placeholder", Role: "textbox", Name: "", Placeholder: "Search products"},
		{Ref: "value", Role: "textbox", Name: "Promo", Value: "SUMMER25"},
	}

	tests := []struct {
		query string
		want  string
	}{
		{"role:textbox Work Email", "label"},
		{"role:textbox Billing ZIP", "labelled-by"},
		{"role:button Save changes", "text"},
		{"role:img Company Logo", "alt"},
		{"role:button Help Center", "title"},
		{"role:textbox Search products", "placeholder"},
		{"role:textbox SUMMER25", "value"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			res, err := m.Find(context.Background(), tt.query, elements, types.FindOptions{Threshold: 0, TopK: len(elements)})
			if err != nil {
				t.Fatalf("Find returned error: %v", err)
			}
			if res.BestRef != tt.want {
				t.Fatalf("BestRef=%q, want %q; matches=%v", res.BestRef, tt.want, res.Matches)
			}
		})
	}
}

func TestStructuredLocator_FindAndSemanticPrefixesForceNaturalMatching(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "button", Role: "button", Name: "Save"},
		{Ref: "literal", Role: "heading", Name: "role button"},
	}

	tests := []string{
		"find:role:button",
		"semantic:role:button",
	}

	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			res, err := m.Find(context.Background(), query, elements, types.FindOptions{Threshold: 0, TopK: 2})
			if err != nil {
				t.Fatalf("Find returned error: %v", err)
			}
			if res.BestRef != "literal" {
				t.Fatalf("BestRef=%q, want literal natural-language match; matches=%v", res.BestRef, res.Matches)
			}
		})
	}
}

func TestStructuredLocator_TextFallsBackToNameForLegacyDescriptors(t *testing.T) {
	m := NewLexicalMatcher()
	elements := []types.ElementDescriptor{
		{Ref: "button", Role: "button", Name: "Submit"},
	}

	res, err := m.Find(context.Background(), "text:Submit", elements, types.FindOptions{Threshold: 0, TopK: 1})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if res.BestRef != "button" {
		t.Fatalf("BestRef=%q, want button", res.BestRef)
	}
}

func TestStructuredLocator_EmbeddingMatcherBypassesEmbedding(t *testing.T) {
	m := NewEmbeddingMatcher(forbiddenEmbedder{})
	elements := []types.ElementDescriptor{
		{Ref: "button", Role: "button", Name: "Submit"},
	}

	res, err := m.Find(context.Background(), "role:button Submit", elements, types.FindOptions{Threshold: 0, TopK: 1})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}
	if res.BestRef != "button" {
		t.Fatalf("BestRef=%q, want button", res.BestRef)
	}
}

func findInternalScore(matches []types.ElementMatch, ref string) (float64, bool) {
	for _, match := range matches {
		if match.Ref == ref {
			return match.Score, true
		}
	}
	return 0, false
}

type forbiddenEmbedder struct{}

func (forbiddenEmbedder) Strategy() string { return "forbidden" }

func (forbiddenEmbedder) Embed([]string) ([][]float32, error) {
	panic("structured locator should not call Embed")
}
