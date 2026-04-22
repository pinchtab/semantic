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
	parsed := ParseQuery(query)
	return m.findWithParsed(parsed, elements, opts)
}

func (m *EmbeddingMatcher) findWithParsed(parsed types.ParsedQuery, elements []types.ElementDescriptor, opts types.FindOptions) (types.FindResult, error) {
	if opts.TopK <= 0 {
		opts.TopK = 3
	}

	if len(parsed.Positive) == 0 && len(parsed.Negative) == 0 {
		return types.FindResult{
			Strategy:     m.Strategy(),
			ElementCount: len(elements),
		}, nil
	}

	positiveQuery := strings.Join(parsed.Positive, " ")
	negativeQuery := strings.Join(parsed.Negative, " ")
	negativeOnly := len(parsed.Positive) == 0 && len(parsed.Negative) > 0

	// Build composite descriptions.
	descs := make([]string, len(elements))
	for i, el := range elements {
		descs[i] = el.Composite()
	}

	// Embed positive/negative query components and all descriptions in one batch.
	texts := make([]string, 0, len(descs)+2)
	if len(parsed.Positive) > 0 {
		texts = append(texts, positiveQuery)
	}
	if len(parsed.Negative) > 0 {
		texts = append(texts, negativeQuery)
	}
	texts = append(texts, descs...)
	vectors, err := m.embedder.Embed(texts)
	if err != nil {
		return types.FindResult{}, err
	}

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

	type scored struct {
		desc  types.ElementDescriptor
		score float64
	}

	var candidates []scored
	for i, el := range elements {
		score := 1.0
		if len(parsed.Positive) > 0 {
			score = CosineSimilarity(posVec, contextVecs[i])
		}

		if len(parsed.Negative) > 0 {
			// Debug note: negSim is the negative-token similarity used to apply
			// exclusion/down-weight penalties for this candidate.
			// Negatives should compare against the element vector itself.
			negSim := CosineSimilarity(negVec, elemVecs[i])
			if len(parsed.Positive) == 0 {
				if negSim > 0.5 {
					score = 0
				}
			} else if negSim > 0.5 {
				score *= 1 - (negSim * 0.8)
			}
		}

		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}
		if negativeOnly && score == 0 {
			continue
		}

		if score >= opts.Threshold {
			candidates = append(candidates, scored{desc: el, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		scoreDiff := candidates[i].score - candidates[j].score
		if math.Abs(scoreDiff) > 1e-9 {
			return scoreDiff > 0
		}
		return candidates[i].desc.Ref < candidates[j].desc.Ref
	})

	if len(candidates) > opts.TopK {
		candidates = candidates[:opts.TopK]
	}

	result := types.FindResult{
		Strategy:     m.Strategy(),
		ElementCount: len(elements),
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

	return result, nil
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
