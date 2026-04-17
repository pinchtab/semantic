package engine

import (
	"context"
	"errors"
	"fmt"
	"github.com/pinchtab/semantic/internal/types"
	"math"
	"testing"
	"time"
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
	sim := CosineSimilarity(v, v)
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("identical vectors should have similarity 1.0, got %f", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0, 0}
	b := []float32{0, 1, 0, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim) > 1e-6 {
		t.Errorf("orthogonal vectors should have similarity ~0, got %f", sim)
	}
}

func TestCosineSimilarity_Empty(t *testing.T) {
	sim := CosineSimilarity(nil, nil)
	if sim != 0 {
		t.Errorf("empty vectors should have similarity 0, got %f", sim)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0, 0}
	sim := CosineSimilarity(a, b)
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

func TestNewEmbeddingMatcher_DefaultNeighborWeight(t *testing.T) {
	m := NewEmbeddingMatcher(newDummyEmbedder(64))
	if math.Abs(float64(m.neighborWeight-defaultNeighborWeight)) > 1e-6 {
		t.Errorf("expected default neighborWeight=%f, got %f", defaultNeighborWeight, m.neighborWeight)
	}
}

func TestNewEmbeddingMatcherWithNeighborWeight_ClampsRange(t *testing.T) {
	below := NewEmbeddingMatcherWithNeighborWeight(newDummyEmbedder(64), -0.25)
	if below.neighborWeight != 0 {
		t.Errorf("expected neighborWeight to clamp to 0, got %f", below.neighborWeight)
	}

	above := NewEmbeddingMatcherWithNeighborWeight(newDummyEmbedder(64), 2)
	if above.neighborWeight != 1 {
		t.Errorf("expected neighborWeight to clamp to 1, got %f", above.neighborWeight)
	}
}

func TestEmbeddingMatcher_Find(t *testing.T) {
	m := NewEmbeddingMatcher(newDummyEmbedder(64))

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Login"},
		{Ref: "e1", Role: "textbox", Name: "Username"},
		{Ref: "e2", Role: "link", Name: "Forgot Password"},
	}

	result, err := m.Find(context.Background(), "login button", elements, types.FindOptions{
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

	elements := []types.ElementDescriptor{
		{Ref: "e0", Role: "button", Name: "Submit"},
		{Ref: "e1", Role: "link", Name: "Cancel"},
	}

	result, err := m.Find(context.Background(), "xyz completely unrelated", elements, types.FindOptions{
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

func TestEmbeddingMatcher_TieBreaksByPositionalHints(t *testing.T) {
	e := newScriptedEmbedder(map[string][]float32{
		"open button":  {1, 0, 0},
		"button: Open": {1, 0, 0},
	})
	m := NewEmbeddingMatcherWithNeighborWeight(e, 0)

	elements := []types.ElementDescriptor{
		{Ref: "shallow", Role: "button", Name: "Open", Positional: types.PositionalHints{Depth: 1, SiblingIndex: 1}},
		{Ref: "deep-left", Role: "button", Name: "Open", Positional: types.PositionalHints{Depth: 3, SiblingIndex: 0}},
		{Ref: "deep-right", Role: "button", Name: "Open", Positional: types.PositionalHints{Depth: 3, SiblingIndex: 2}},
	}

	res, err := m.Find(context.Background(), "open button", elements, types.FindOptions{Threshold: 0, TopK: 3})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if res.BestRef != "deep-left" {
		t.Fatalf("expected deep-left to win tie-break, got %s", res.BestRef)
	}
}

func TestEmbeddingMatcher_NeighborContextDisambiguatesRealWorldButtons(t *testing.T) {
	e := newScriptedEmbedder(map[string][]float32{
		"laptop add to cart":         {1, 1, 0},
		"heading: Gaming Laptop Pro": {0, 1, 0},
		"button: Add to cart":        {1, 0, 0},
		"heading: Running Shoes":     {0, 0, 1},
	})

	elements := []types.ElementDescriptor{
		{Ref: "title-laptop", Role: "heading", Name: "Gaming Laptop Pro"},
		{Ref: "add-laptop", Role: "button", Name: "Add to cart"},
		{Ref: "title-shoes", Role: "heading", Name: "Running Shoes"},
		{Ref: "add-shoes", Role: "button", Name: "Add to cart"},
	}

	noCtx := NewEmbeddingMatcherWithNeighborWeight(e, 0)
	baseRes, err := noCtx.Find(context.Background(), "laptop add to cart", elements, types.FindOptions{Threshold: 0, TopK: 4})
	if err != nil {
		t.Fatalf("Find with no context failed: %v", err)
	}
	baseLaptop, ok := findMatchScore(baseRes.Matches, "add-laptop")
	if !ok {
		t.Fatalf("expected add-laptop result in no-context run")
	}
	baseShoes, ok := findMatchScore(baseRes.Matches, "add-shoes")
	if !ok {
		t.Fatalf("expected add-shoes result in no-context run")
	}
	if math.Abs(baseLaptop-baseShoes) > 1e-9 {
		t.Fatalf("expected identical button scores without context, got laptop=%.6f shoes=%.6f", baseLaptop, baseShoes)
	}

	withCtx := NewEmbeddingMatcherWithNeighborWeight(e, 0.2)
	ctxRes, err := withCtx.Find(context.Background(), "laptop add to cart", elements, types.FindOptions{Threshold: 0, TopK: 4})
	if err != nil {
		t.Fatalf("Find with context failed: %v", err)
	}
	ctxLaptop, ok := findMatchScore(ctxRes.Matches, "add-laptop")
	if !ok {
		t.Fatalf("expected add-laptop result in contextual run")
	}
	ctxShoes, ok := findMatchScore(ctxRes.Matches, "add-shoes")
	if !ok {
		t.Fatalf("expected add-shoes result in contextual run")
	}
	if ctxLaptop <= ctxShoes {
		t.Fatalf("expected laptop button to rank higher with context, got laptop=%.6f shoes=%.6f", ctxLaptop, ctxShoes)
	}
}

func TestEmbeddingMatcher_SingleElement_WithNeighborWeight(t *testing.T) {
	e := newScriptedEmbedder(map[string][]float32{
		"open account settings":    {1, 1, 0},
		"button: Account settings": {1, 1, 0},
	})
	m := NewEmbeddingMatcherWithNeighborWeight(e, 0.5)

	res, err := m.Find(context.Background(), "open account settings", []types.ElementDescriptor{
		{Ref: "settings", Role: "button", Name: "Account settings"},
	}, types.FindOptions{Threshold: 0, TopK: 1})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	if res.BestRef != "settings" {
		t.Fatalf("expected BestRef=settings, got %s", res.BestRef)
	}
	if len(res.Matches) != 1 {
		t.Fatalf("expected one match, got %d", len(res.Matches))
	}
}

func TestEmbeddingMatcher_Find_ContextCanceledBeforeEmbed(t *testing.T) {
	e := &countingEmbedder{}
	m := NewEmbeddingMatcher(e)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := m.Find(ctx, "save button", []types.ElementDescriptor{{Ref: "e1", Role: "button", Name: "Save"}}, types.FindOptions{Threshold: 0, TopK: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	if e.calls != 0 {
		t.Fatalf("embedder should not have been invoked on canceled context, calls=%d", e.calls)
	}
}

func TestEmbeddingMatcher_Find_ContextCanceledDuringEmbedContext(t *testing.T) {
	e := &blockingContextEmbedder{started: make(chan struct{}), release: make(chan struct{})}
	defer close(e.release)

	m := NewEmbeddingMatcher(e)
	elements := []types.ElementDescriptor{{Ref: "e1", Role: "button", Name: "Save"}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := m.Find(ctx, "save button", elements, types.FindOptions{Threshold: 0, TopK: 1})
		done <- err
	}()

	select {
	case <-e.started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected embedding to start")
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled error, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Find did not return promptly after context cancellation")
	}
}

func TestEmbeddingMatcher_Find_EmbedderVectorCountMismatchReturnsError(t *testing.T) {
	m := NewEmbeddingMatcher(&malformedEmbedder{
		vectors: [][]float32{{1, 0, 0}},
	})

	_, err := m.Find(context.Background(), "query", []types.ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Save"},
		{Ref: "e2", Role: "button", Name: "Cancel"},
	}, types.FindOptions{Threshold: 0, TopK: 2})
	if err == nil {
		t.Fatal("expected embedder vector count mismatch error")
	}
}

func TestEmbeddingMatcher_Find_InconsistentVectorDimensionsReturnsError(t *testing.T) {
	m := NewEmbeddingMatcher(&malformedEmbedder{
		vectors: [][]float32{{1, 0}, {1}, {0, 1}},
	})

	_, err := m.Find(context.Background(), "query", []types.ElementDescriptor{
		{Ref: "e1", Role: "button", Name: "Save"},
		{Ref: "e2", Role: "button", Name: "Cancel"},
	}, types.FindOptions{Threshold: 0, TopK: 2})
	if err == nil {
		t.Fatal("expected inconsistent embedding dimension error")
	}
}

type countingEmbedder struct {
	calls int
}

func (e *countingEmbedder) Strategy() string { return "counting" }

func (e *countingEmbedder) Embed(texts []string) ([][]float32, error) {
	e.calls++
	return fixedVectors(len(texts), 4), nil
}

type blockingContextEmbedder struct {
	started chan struct{}
	release chan struct{}
}

func (e *blockingContextEmbedder) Strategy() string { return "blocking-context" }

func (e *blockingContextEmbedder) Embed(texts []string) ([][]float32, error) {
	return fixedVectors(len(texts), 4), nil
}

func (e *blockingContextEmbedder) EmbedContext(ctx context.Context, texts []string) ([][]float32, error) {
	close(e.started)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.release:
		return fixedVectors(len(texts), 4), nil
	}
}

type malformedEmbedder struct {
	vectors [][]float32
}

func (e *malformedEmbedder) Strategy() string { return "malformed" }

func (e *malformedEmbedder) Embed(_ []string) ([][]float32, error) {
	return e.vectors, nil
}

type scriptedEmbedder struct {
	vectors map[string][]float32
}

func newScriptedEmbedder(vectors map[string][]float32) *scriptedEmbedder {
	return &scriptedEmbedder{vectors: vectors}
}

func (s *scriptedEmbedder) Strategy() string {
	return "scripted"
}

func (s *scriptedEmbedder) Embed(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		base, ok := s.vectors[text]
		if !ok {
			return nil, fmt.Errorf("missing scripted embedding for %q", text)
		}
		vec := make([]float32, len(base))
		copy(vec, base)
		normalizeDenseVector(vec)
		out[i] = vec
	}
	return out, nil
}

func findMatchScore(matches []types.ElementMatch, ref string) (float64, bool) {
	for _, match := range matches {
		if match.Ref == ref {
			return match.Score, true
		}
	}
	return 0, false
}

// FindResult.ConfidenceLabel tests
