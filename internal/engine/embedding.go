package engine

import (
	"context"
	"github.com/pinchtab/semantic/internal/types"
	"math"
	"sort"
	"strings"
)

// Embedder converts text into dense vectors. See NewHashingEmbedder.
type Embedder interface {
	// Embed converts a batch of text strings into float32 vectors.
	// All returned vectors must have the same dimensionality.
	Embed(texts []string) ([][]float32, error)

	// Strategy returns the name of the embedding strategy (e.g. "hashing", "openai").
	Strategy() string
}

// EmbeddingMatcher scores elements using cosine similarity on dense
// vectors produced by an Embedder.
type EmbeddingMatcher struct {
	embedder       Embedder
	neighborWeight float32
}

const defaultNeighborWeight float32 = 0.10

// NewEmbeddingMatcher creates an embedding-based matcher.
func NewEmbeddingMatcher(e Embedder) *EmbeddingMatcher {
	return NewEmbeddingMatcherWithNeighborWeight(e, float64(defaultNeighborWeight))
}

// NewEmbeddingMatcherWithNeighborWeight creates an embedding matcher and sets
// how much immediate neighbors influence each element embedding.
func NewEmbeddingMatcherWithNeighborWeight(e Embedder, weight float64) *EmbeddingMatcher {
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}
	return &EmbeddingMatcher{embedder: e, neighborWeight: float32(weight)}
}

func (m *EmbeddingMatcher) Strategy() string {
	return "embedding:" + m.embedder.Strategy()
}

func (m *EmbeddingMatcher) Find(_ context.Context, query string, elements []types.ElementDescriptor, opts types.FindOptions) (types.FindResult, error) {
	ctx := ParseQueryContext(query)
	return m.findWithParsed(ctx, elements, opts)
}

func (m *EmbeddingMatcher) findWithParsed(ctx QueryContext, elements []types.ElementDescriptor, opts types.FindOptions) (types.FindResult, error) {
	parsed := ctx.Base
	if opts.TopK <= 0 {
		opts.TopK = 3
	}

	if len(parsed.Positive) == 0 && len(parsed.Negative) == 0 {
		return types.FindResult{
			Strategy:     m.Strategy(),
			ElementCount: len(elements),
		}, nil
	}

	filtered := filterContextExcludedElements(elements, ctx)
	if len(filtered) == 0 {
		return types.FindResult{Strategy: m.Strategy(), ElementCount: len(elements)}, nil
	}

	vectors, err := m.embedQueryAndElements(parsed, filtered)
	if err != nil {
		return types.FindResult{}, err
	}

	candidates := m.scoreCandidates(parsed, filtered, vectors, opts.Threshold)
	sort.Slice(candidates, func(i, j int) bool {
		return rankedMatchLess(
			candidates[i].score, candidates[i].desc, candidates[i].order,
			candidates[j].score, candidates[j].desc, candidates[j].order,
		)
	})

	if len(candidates) > opts.TopK {
		candidates = candidates[:opts.TopK]
	}

	return buildEmbeddingResult(m.Strategy(), len(elements), candidates), nil
}

func filterContextExcludedElements(elements []types.ElementDescriptor, ctx QueryContext) []types.ElementDescriptor {
	filtered := make([]types.ElementDescriptor, 0, len(elements))
	for _, el := range elements {
		if ctx.HasScope && matchesExcludedContext(el, ctx.Exclude) {
			continue
		}
		filtered = append(filtered, el)
	}
	return filtered
}

func (m *EmbeddingMatcher) embedQueryAndElements(parsed types.ParsedQuery, elements []types.ElementDescriptor) ([][]float32, error) {
	positiveQuery := strings.Join(parsed.Positive, " ")
	negativeQuery := strings.Join(parsed.Negative, " ")

	descs := make([]string, len(elements))
	for i, el := range elements {
		descs[i] = el.Composite()
	}

	texts := make([]string, 0, len(descs)+2)
	if len(parsed.Positive) > 0 {
		texts = append(texts, positiveQuery)
	}
	if len(parsed.Negative) > 0 {
		texts = append(texts, negativeQuery)
	}
	texts = append(texts, descs...)
	return m.embedder.Embed(texts)
}

type embeddingScored struct {
	desc  types.ElementDescriptor
	score float64
	order int
}

func (m *EmbeddingMatcher) scoreCandidates(parsed types.ParsedQuery, elements []types.ElementDescriptor, vectors [][]float32, threshold float64) []embeddingScored {
	negativeOnly := len(parsed.Positive) == 0 && len(parsed.Negative) > 0
	idx := 0
	var posVec []float32
	if len(parsed.Positive) > 0 {
		posVec = vectors[idx]
		idx++
	}

	var negVec []float32
	if len(parsed.Negative) > 0 {
		negVec = vectors[idx]
		idx++
	}

	elemVecs := vectors[idx:]
	contextVecs := elemVecs
	if m.neighborWeight > 0 && len(elemVecs) > 1 {
		contextVecs = m.withNeighborContext(elemVecs)
	}

	var candidates []embeddingScored
	for i, el := range elements {
		score := scoreEmbeddingCandidate(parsed, posVec, negVec, contextVecs[i], elemVecs[i])
		if negativeOnly && score == 0 {
			continue
		}
		if score >= threshold {
			candidates = append(candidates, embeddingScored{desc: el, score: score, order: i})
		}
	}
	return candidates
}

func scoreEmbeddingCandidate(parsed types.ParsedQuery, posVec, negVec, contextVec, elemVec []float32) float64 {
	score := 1.0
	if len(parsed.Positive) > 0 {
		score = CosineSimilarity(posVec, contextVec)
	}

	if len(parsed.Negative) > 0 {
		negSim := CosineSimilarity(negVec, elemVec)
		if len(parsed.Positive) == 0 {
			if negSim > 0.5 {
				score = 0
			}
		} else if negSim > 0.5 {
			score *= 1 - (negSim * 0.8)
		}
	}

	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func buildEmbeddingResult(strategy string, elementCount int, candidates []embeddingScored) types.FindResult {
	result := types.FindResult{
		Strategy:     strategy,
		ElementCount: elementCount,
	}
	for _, c := range candidates {
		result.Matches = append(result.Matches, types.ElementMatch{
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
	return result
}

func (m *EmbeddingMatcher) withNeighborContext(base [][]float32) [][]float32 {
	contextual := make([][]float32, len(base))
	for i := range base {
		vec := make([]float32, len(base[i]))
		copy(vec, base[i])

		if i > 0 {
			for d := range vec {
				vec[d] += base[i-1][d] * m.neighborWeight
			}
		}
		if i+1 < len(base) {
			for d := range vec {
				vec[d] += base[i+1][d] * m.neighborWeight
			}
		}

		normalizeDenseVector(vec)
		contextual[i] = vec
	}
	return contextual
}

func normalizeDenseVector(vec []float32) {
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm == 0 {
		return
	}
	invNorm := float32(1.0 / math.Sqrt(norm))
	for i := range vec {
		vec[i] *= invNorm
	}
}

// CosineSimilarity computes the cosine similarity between two float32 vectors.
func CosineSimilarity(a, b []float32) float64 {
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
