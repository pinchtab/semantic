package semantic

import (
	"context"
	"math"
	"sort"
)

// Embedder is the interface for converting text into dense vector
// representations. The default implementation is NewHashingEmbedder
// (zero deps, feature hashing).
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

// embeddingMatcher implements ElementMatcher using vector embeddings
// and cosine similarity. It delegates text → vector conversion to the
// provided Embedder implementation.
type embeddingMatcher struct {
	embedder Embedder
}

// NewEmbeddingMatcher creates an EmbeddingMatcher backed by the given Embedder.
func NewEmbeddingMatcher(e Embedder) ElementMatcher {
	return &embeddingMatcher{embedder: e}
}

// Strategy returns "embedding:<embedder_strategy>".
func (m *embeddingMatcher) Strategy() string {
	return "embedding:" + m.embedder.Strategy()
}

// Find embeds the query and all element descriptions, ranks by cosine
// similarity, filters by threshold, and returns top-K matches.
func (m *embeddingMatcher) Find(_ context.Context, query string, elements []ElementDescriptor, opts FindOptions) (FindResult, error) {
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
