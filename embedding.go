package semantic

import (
	"context"
	"math"
	"sort"
)

// Embedder is the interface for converting text into dense vector
// representations. Implementations include HashingEmbedder (zero deps,
// feature hashing) and DummyEmbedder (deterministic, for testing).
//
// Custom implementations can be provided for real ML models
// (sentence-transformers, OpenAI, etc.).
type Embedder interface {
	// Embed converts a batch of text strings into float32 vectors.
	// All returned vectors must have the same dimensionality.
	Embed(texts []string) ([][]float32, error)

	// Strategy returns the name of the embedding strategy (e.g. "hashing", "openai").
	Strategy() string
}

// DummyEmbedder generates deterministic fixed-dimension vectors using a
// simple hash of each input string. Useful for testing without real ML
// dependencies. For production use, prefer HashingEmbedder.
type DummyEmbedder struct {
	Dim int // vector dimensionality (default 64)
}

// NewDummyEmbedder creates a DummyEmbedder with the given dimensionality.
func NewDummyEmbedder(dim int) *DummyEmbedder {
	if dim <= 0 {
		dim = 64
	}
	return &DummyEmbedder{Dim: dim}
}

// Strategy returns "test".
func (d *DummyEmbedder) Strategy() string { return "test" }

// Embed generates deterministic pseudo-vectors by hashing each character
// of the input string into the vector dimensions.
func (d *DummyEmbedder) Embed(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = d.hashVec(text)
	}
	return result, nil
}

func (d *DummyEmbedder) hashVec(s string) []float32 {
	vec := make([]float32, d.Dim)
	for i, c := range s {
		idx := (i*31 + int(c)) % d.Dim
		if idx < 0 {
			idx = -idx
		}
		vec[idx] += float32(c) / 128.0
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		invNorm := float32(1.0 / math.Sqrt(norm))
		for j := range vec {
			vec[j] *= invNorm
		}
	}
	return vec
}

// EmbeddingMatcher implements ElementMatcher using vector embeddings
// and cosine similarity. It delegates text → vector conversion to the
// provided Embedder implementation.
type EmbeddingMatcher struct {
	embedder Embedder
}

// NewEmbeddingMatcher creates an EmbeddingMatcher backed by the given Embedder.
func NewEmbeddingMatcher(e Embedder) *EmbeddingMatcher {
	return &EmbeddingMatcher{embedder: e}
}

// Strategy returns "embedding:<embedder_strategy>".
func (m *EmbeddingMatcher) Strategy() string {
	return "embedding:" + m.embedder.Strategy()
}

// Find embeds the query and all element descriptions, ranks by cosine
// similarity, filters by threshold, and returns top-K matches.
func (m *EmbeddingMatcher) Find(_ context.Context, query string, elements []ElementDescriptor, opts FindOptions) (FindResult, error) {
	if opts.TopK <= 0 {
		opts.TopK = 3
	}

	// Build composite descriptions.
	descs := make([]string, len(elements))
	for i, el := range elements {
		descs[i] = el.Composite()
	}

	// Embed query + all descriptions in a single batch.
	texts := append([]string{query}, descs...)
	vectors, err := m.embedder.Embed(texts)
	if err != nil {
		return FindResult{}, err
	}

	queryVec := vectors[0]
	elemVecs := vectors[1:]

	type scored struct {
		desc  ElementDescriptor
		score float64
	}

	var candidates []scored
	for i, el := range elements {
		sim := cosineSimilarity(queryVec, elemVecs[i])
		if sim >= opts.Threshold {
			candidates = append(candidates, scored{desc: el, score: sim})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > opts.TopK {
		candidates = candidates[:opts.TopK]
	}

	result := FindResult{
		Strategy:     m.Strategy(),
		ElementCount: len(elements),
	}

	for _, c := range candidates {
		result.Matches = append(result.Matches, ElementMatch{
			Ref:   c.desc.Ref,
			Score: c.score,
			Role:  c.desc.Role,
			Name:  c.desc.Name,
		})
	}

	if len(result.Matches) > 0 {
		result.BestRef = result.Matches[0].Ref
		result.BestScore = result.Matches[0].Score
	}

	return result, nil
}

// cosineSimilarity computes cosine similarity between two float32 vectors.
// Returns a value in [-1, 1]; for normalized vectors this is in [0, 1].
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
