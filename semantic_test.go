package semantic_test

import (
	"context"
	"testing"

	"github.com/pinchtab/semantic"
)

func negativeMatchingFixture() []semantic.ElementDescriptor {
	return []semantic.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Submit"},
		{Ref: "e1", Role: "button", Name: "Cancel"},
		{Ref: "e2", Role: "button", Name: "Sign In"},
		{Ref: "e3", Role: "link", Name: "Logout"},
		{Ref: "e4", Role: "textbox", Name: "Email"},
		{Ref: "e5", Role: "textbox", Name: "Password"},
	}
}

func findScore(matches []semantic.ElementMatch, ref string) (float64, bool) {
	for _, m := range matches {
		if m.Ref == ref {
			return m.Score, true
		}
	}
	return 0, false
}

func TestLexicalMatcher_NegativeMatching_Issue24Cases(t *testing.T) {
	m := semantic.NewLexicalMatcher()
	elements := negativeMatchingFixture()

	tests := []struct {
		name  string
		query string
		check func(t *testing.T, result semantic.FindResult)
	}{
		{
			name:  "button not submit penalizes submit",
			query: "button not submit",
			check: func(t *testing.T, result semantic.FindResult) {
				e0, ok := findScore(result.Matches, "e0")
				if !ok {
					t.Fatalf("expected e0 in matches")
				}
				e1, ok := findScore(result.Matches, "e1")
				if !ok {
					t.Fatalf("expected e1 in matches")
				}
				e2, ok := findScore(result.Matches, "e2")
				if !ok {
					t.Fatalf("expected e2 in matches")
				}
				if !(e1 > e0 || e2 > e0) {
					t.Fatalf("expected e1 or e2 to rank above penalized e0, got e0=%.4f e1=%.4f e2=%.4f", e0, e1, e2)
				}
			},
		},
		{
			name:  "button not behaves as button",
			query: "button not",
			check: func(t *testing.T, result semantic.FindResult) {
				if result.BestRef == "e4" || result.BestRef == "e5" {
					t.Fatalf("expected a button-like result, got %s", result.BestRef)
				}
			},
		},
		{
			name:  "button not cancel penalizes cancel",
			query: "button not cancel",
			check: func(t *testing.T, result semantic.FindResult) {
				e0, ok := findScore(result.Matches, "e0")
				if !ok {
					t.Fatalf("expected e0 in matches")
				}
				e1, ok := findScore(result.Matches, "e1")
				if !ok {
					t.Fatalf("expected e1 in matches")
				}
				if e0 <= e1 {
					t.Fatalf("expected e0 above penalized e1, got e0=%.4f e1=%.4f", e0, e1)
				}
			},
		},
		{
			name:  "textbox not email prefers password",
			query: "textbox not email",
			check: func(t *testing.T, result semantic.FindResult) {
				e4, ok := findScore(result.Matches, "e4")
				if !ok {
					t.Fatalf("expected e4 in matches")
				}
				e5, ok := findScore(result.Matches, "e5")
				if !ok {
					t.Fatalf("expected e5 in matches")
				}
				if e5 <= e4 {
					t.Fatalf("expected e5 above penalized e4, got e4=%.4f e5=%.4f", e4, e5)
				}
			},
		},
		{
			name:  "button not login penalizes sign in by synonym",
			query: "button not login",
			check: func(t *testing.T, result semantic.FindResult) {
				e0, ok := findScore(result.Matches, "e0")
				if !ok {
					t.Fatalf("expected e0 in matches")
				}
				e1, ok := findScore(result.Matches, "e1")
				if !ok {
					t.Fatalf("expected e1 in matches")
				}
				e2, ok := findScore(result.Matches, "e2")
				if !ok {
					t.Fatalf("expected e2 in matches")
				}
				if !(e0 > e2 && e1 > e2) {
					t.Fatalf("expected e2 to be penalized by login/sign in synonym, got e0=%.4f e1=%.4f e2=%.4f", e0, e1, e2)
				}
			},
		},
		{
			name:  "button not sign in penalizes sign in",
			query: "button not sign in",
			check: func(t *testing.T, result semantic.FindResult) {
				e0, ok := findScore(result.Matches, "e0")
				if !ok {
					t.Fatalf("expected e0 in matches")
				}
				e1, ok := findScore(result.Matches, "e1")
				if !ok {
					t.Fatalf("expected e1 in matches")
				}
				e2, ok := findScore(result.Matches, "e2")
				if !ok {
					t.Fatalf("expected e2 in matches")
				}
				if !(e0 > e2 && e1 > e2) {
					t.Fatalf("expected e2 to be penalized by multi-token negative, got e0=%.4f e1=%.4f e2=%.4f", e0, e1, e2)
				}
			},
		},
		{
			name:  "button not submit not cancel penalizes both",
			query: "button not submit not cancel",
			check: func(t *testing.T, result semantic.FindResult) {
				e0, ok := findScore(result.Matches, "e0")
				if !ok {
					t.Fatalf("expected e0 in matches")
				}
				e1, ok := findScore(result.Matches, "e1")
				if !ok {
					t.Fatalf("expected e1 in matches")
				}
				e2, ok := findScore(result.Matches, "e2")
				if !ok {
					t.Fatalf("expected e2 in matches")
				}
				if !(e2 > e0 && e2 > e1) {
					t.Fatalf("expected both submit/cancel to be penalized, got e0=%.4f e1=%.4f e2=%.4f", e0, e1, e2)
				}
			},
		},
		{
			name:  "sign in button regression",
			query: "sign in button",
			check: func(t *testing.T, result semantic.FindResult) {
				if result.BestRef != "e2" {
					t.Fatalf("expected e2 as best result, got %s", result.BestRef)
				}
			},
		},
		{
			name:  "link without logout drives logout near zero",
			query: "link without logout",
			check: func(t *testing.T, result semantic.FindResult) {
				e3, ok := findScore(result.Matches, "e3")
				if !ok {
					t.Fatalf("expected e3 in matches")
				}
				if e3 > 0.1 {
					t.Fatalf("expected e3 near-zero score, got %.4f", e3)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.Find(context.Background(), tt.query, elements, semantic.FindOptions{
				Threshold: 0,
				TopK:      len(elements),
			})
			if err != nil {
				t.Fatalf("Find returned error: %v", err)
			}
			tt.check(t, result)
		})
	}
}

func TestLexicalMatcher_NegativeOnlyQuery(t *testing.T) {
	m := semantic.NewLexicalMatcher()
	elements := negativeMatchingFixture()

	result, err := m.Find(context.Background(), "not submit", elements, semantic.FindOptions{
		Threshold: 0,
		TopK:      len(elements),
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if len(result.Matches) == 0 {
		t.Fatalf("expected non-empty matches for leading-not query")
	}
	if result.BestRef != "e0" {
		t.Fatalf("expected leading-not query to behave as positive text, got best=%s", result.BestRef)
	}
}

func TestNewCombinedMatcher_Find(t *testing.T) {
	m := semantic.NewCombinedMatcher(semantic.NewHashingEmbedder(128))

	elements := []semantic.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Sign In"},
		{Ref: "e1", Role: "textbox", Name: "Email"},
		{Ref: "e2", Role: "link", Name: "Forgot Password"},
	}

	result, err := m.Find(context.Background(), "login button", elements, semantic.FindOptions{
		Threshold: 0.1,
		TopK:      3,
	})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if result.BestRef != "e0" {
		t.Errorf("expected BestRef=e0, got %s", result.BestRef)
	}
	if result.ElementCount != 3 {
		t.Errorf("expected ElementCount=3, got %d", result.ElementCount)
	}
}

func TestNewLexicalMatcher(t *testing.T) {
	m := semantic.NewLexicalMatcher()
	if m.Strategy() != "lexical" {
		t.Errorf("expected strategy=lexical, got %s", m.Strategy())
	}
}

func TestNewEmbeddingMatcher(t *testing.T) {
	e := semantic.NewHashingEmbedder(64)
	m := semantic.NewEmbeddingMatcher(e)
	if m.Strategy() != "embedding:hashing" {
		t.Errorf("expected strategy=embedding:hashing, got %s", m.Strategy())
	}
}

func TestNewEmbeddingMatcherWithNeighborWeight(t *testing.T) {
	e := semantic.NewHashingEmbedder(64)
	m := semantic.NewEmbeddingMatcherWithNeighborWeight(e, 0.2)
	if m.Strategy() != "embedding:hashing" {
		t.Errorf("expected strategy=embedding:hashing, got %s", m.Strategy())
	}
}

func TestCalibrateConfidence(t *testing.T) {
	if semantic.CalibrateConfidence(0.9) != "high" {
		t.Error("0.9 should be high")
	}
	if semantic.CalibrateConfidence(0.7) != "medium" {
		t.Error("0.7 should be medium")
	}
	if semantic.CalibrateConfidence(0.3) != "low" {
		t.Error("0.3 should be low")
	}
}

func TestLexicalScore(t *testing.T) {
	score := semantic.LexicalScore("submit button", "button: Submit")
	if score < 0.5 {
		t.Errorf("expected high score for exact match, got %.2f", score)
	}
}

func TestCosineSimilarity(t *testing.T) {
	v := []float32{1, 0, 0}
	if semantic.CosineSimilarity(v, v) < 0.99 {
		t.Error("identical vectors should have similarity ~1.0")
	}
}

func TestElementDescriptor_Composite(t *testing.T) {
	d := semantic.ElementDescriptor{Role: "button", Name: "Submit"}
	if d.Composite() != "button: Submit" {
		t.Errorf("unexpected composite: %q", d.Composite())
	}
}

func TestElementDescriptor_Composite_IncludesSection(t *testing.T) {
	d := semantic.ElementDescriptor{Role: "button", Name: "Submit", Section: "Login form"}
	if d.Composite() != "button: Submit {Login form}" {
		t.Errorf("unexpected composite with section: %q", d.Composite())
	}
}

func TestElementDescriptor_PositionalHints(t *testing.T) {
	d := semantic.ElementDescriptor{
		Role: "textbox",
		Name: "Email",
		Positional: semantic.PositionalHints{
			Depth:        3,
			SiblingIndex: 1,
			SiblingCount: 2,
			LabelledBy:   "Work Email",
		},
	}
	if d.Positional.Depth != 3 {
		t.Errorf("unexpected depth: %d", d.Positional.Depth)
	}
	if d.Positional.LabelledBy != "Work Email" {
		t.Errorf("unexpected labelled_by: %q", d.Positional.LabelledBy)
	}
}
