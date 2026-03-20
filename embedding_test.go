package semantic

import (
	"context"
	"math"
	"testing"
)



// dummyEmbedder tests

func TestDummyEmbedder_Deterministic(t *testing.T) {
	e := newDummyEmbedder(64)

	v1, err := e.Embed([]string{"hello world"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}
	v2, err := e.Embed([]string{"hello world"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	if len(v1[0]) != 64 {
		t.Errorf("expected dim=64, got %d", len(v1[0]))
	}

	for i := range v1[0] {
		if v1[0][i] != v2[0][i] {
			t.Fatalf("dummyEmbedder is not deterministic at dim %d", i)
		}
	}
}

func TestDummyEmbedder_Strategy(t *testing.T) {
	e := newDummyEmbedder(32)
	if e.Strategy() != "test" {
		t.Errorf("expected strategy=dummy, got %s", e.Strategy())
	}
}

func TestDummyEmbedder_DefaultDim(t *testing.T) {
	e := newDummyEmbedder(0)
	if e.dim != 64 {
		t.Errorf("expected default dim=64, got %d", e.dim)
	}
}

func TestDummyEmbedder_NormalizedOutput(t *testing.T) {
	e := newDummyEmbedder(64)
	vecs, err := e.Embed([]string{"test string"})
	if err != nil {
		t.Fatalf("Embed error: %v", err)
	}

	var norm float64
	for _, v := range vecs[0] {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 0.01 {
		t.Errorf("expected unit-norm vector, got norm=%f", norm)
	}
}

// CosineSimilarity tests

// CosineSimilarity tests

func TestCosineSimilarity_Identical(t *testing.T) {
	v := []float32{1, 0, 0, 0}
	sim := cosineSimilarity(v, v)
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("identical vectors should have similarity 1.0, got %f", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0, 0}
	b := []float32{0, 1, 0, 0}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim) > 1e-6 {
		t.Errorf("orthogonal vectors should have similarity ~0, got %f", sim)
	}
}

func TestCosineSimilarity_Empty(t *testing.T) {
	sim := cosineSimilarity(nil, nil)
	if sim != 0 {
		t.Errorf("empty vectors should have similarity 0, got %f", sim)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0, 0}
	sim := cosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("different-length vectors should return 0, got %f", sim)
	}
}

// EmbeddingMatcher tests

// EmbeddingMatcher tests

func TestEmbeddingMatcher_Strategy(t *testing.T) {
	m := NewEmbeddingMatcher(newDummyEmbedder(64))
	want := "embedding:test"
	if m.Strategy() != want {
		t.Errorf("expected strategy=%s, got %s", want, m.Strategy())
	}
}

func TestEmbeddingMatcher_Find(t *testing.T) {
	m := NewEmbeddingMatcher(newDummyEmbedder(64))

	elements := []ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Login"},
		{Ref: "e1", Role: "textbox", Name: "Username"},
		{Ref: "e2", Role: "link", Name: "Forgot Password"},
	}

	result, err := m.Find(context.Background(), "login button", elements, FindOptions{
		Threshold: 0.0,
		TopK:      3,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	if result.ElementCount != 3 {
		t.Errorf("expected ElementCount=3, got %d", result.ElementCount)
	}
	if result.Strategy != "embedding:test" {
		t.Errorf("expected strategy=embedding:test, got %s", result.Strategy)
	}
	if len(result.Matches) == 0 {
		t.Error("expected at least one match")
	}
	// BestScore should be in valid range
	if result.BestScore < 0 || result.BestScore > 1 {
		t.Errorf("BestScore out of [0,1] range: %f", result.BestScore)
	}
}

func TestEmbeddingMatcher_ThresholdFiltering(t *testing.T) {
	m := NewEmbeddingMatcher(newDummyEmbedder(64))

	elements := []ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Submit"},
		{Ref: "e1", Role: "link", Name: "Cancel"},
	}

	result, err := m.Find(context.Background(), "xyz completely unrelated", elements, FindOptions{
		Threshold: 0.99,
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("Find returned error: %v", err)
	}

	for _, m := range result.Matches {
		if m.Score < 0.99 {
			t.Errorf("match %s score %f below threshold 0.99", m.Ref, m.Score)
		}
	}
}

// FindResult.ConfidenceLabel tests
