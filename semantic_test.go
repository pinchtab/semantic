package semantic_test

import (
	"context"
	"testing"

	"github.com/pinchtab/semantic"
)

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
