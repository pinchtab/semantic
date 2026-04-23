package engine

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// hashingEmbedder uses the hashing trick (Weinberger et al. 2009) to produce
// fixed-dimension vectors from word unigrams and character n-grams.
// No vocabulary construction needed.
// HashingEmbedder uses the hashing trick (Weinberger et al. 2009) to produce
// fixed-dimension vectors from word unigrams and character n-grams.
// Zero external dependencies.
type HashingEmbedder struct {
	dim         int     // vector dimensionality
	ngramMin    int     // minimum character n-gram length
	ngramMax    int     // maximum character n-gram length
	wordWeight  float32 // weight factor for word-level features
	ngramWeight float32 // weight factor for n-gram features
}

// NewHashingEmbedder creates a hashing-based embedder with the given
// vector dimensionality. Default: 128.
func NewHashingEmbedder(dim int) *HashingEmbedder {
	if dim <= 0 {
		dim = 128
	}
	return &HashingEmbedder{
		dim:         dim,
		ngramMin:    2,
		ngramMax:    4,
		wordWeight:  1.0,
		ngramWeight: 0.5,
	}
}

func (h *HashingEmbedder) Strategy() string { return "hashing" }

func (h *HashingEmbedder) Embed(texts []string) ([][]float32, error) {
	return h.EmbedContext(context.Background(), texts)
}

func (h *HashingEmbedder) EmbedContext(ctx context.Context, texts []string) ([][]float32, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	result := make([][]float32, len(texts))
	for i, text := range texts {
		if i%64 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		result[i] = h.vectorize(text)
	}
	return result, nil
}

func (h *HashingEmbedder) vectorize(text string) []float32 {
	vec := make([]float32, h.dim)

	// Normalize text
	text = strings.ToLower(text)

	// 1. Word-level features (captures exact word overlap)
	words := tokenizeForEmbedding(text)
	for _, word := range words {
		idx, sign := h.hashFeature("w:" + word)
		vec[idx] += sign * h.wordWeight
	}

	// 2. Character n-gram features (captures sub-word similarity)
	//    e.g. "button" → "bu", "ut", "tt", "to", "on", "but", "utt", "tto", "ton"
	for _, word := range words {
		padded := "^" + word + "$" // boundary markers
		for n := h.ngramMin; n <= h.ngramMax; n++ {
			for i := 0; i <= len(padded)-n; i++ {
				ngram := padded[i : i+n]
				idx, sign := h.hashFeature("n:" + ngram)
				vec[idx] += sign * h.ngramWeight
			}
		}
	}

	// 3. Role-aware features: if a word is a known UI role, add an
	//    extra feature to boost role-based matching
	for _, word := range words {
		if roleKeywords[word] {
			idx, sign := h.hashFeature("role:" + word)
			vec[idx] += sign * 0.8
		}
	}

	// 4. Synonym features: inject word-level features for known synonyms
	//    at a reduced weight so "sign in" and "log in" share vector space.
	for _, word := range words {
		if syns, ok := synonymIndex[word]; ok {
			for syn := range syns {
				synTokens := strings.Fields(syn)
				for _, st := range synTokens {
					idx, sign := h.hashFeature("w:" + st)
					vec[idx] += sign * h.wordWeight * 0.3
				}
			}
		}
	}

	// 5. Multi-word synonym phrases: check consecutive word pairs/triples
	//    so "look up" → "search" gets injected at the embedding level.
	for n := 2; n <= 3 && n <= len(words); n++ {
		for i := 0; i <= len(words)-n; i++ {
			phrase := strings.Join(words[i:i+n], " ")
			if syns, ok := synonymIndex[phrase]; ok {
				for syn := range syns {
					synTokens := strings.Fields(syn)
					for _, st := range synTokens {
						idx, sign := h.hashFeature("w:" + st)
						vec[idx] += sign * h.wordWeight * 0.3
					}
				}
			}
		}
	}

	// L2-normalize for cosine similarity
	h.normalize(vec)
	return vec
}

func (h *HashingEmbedder) hashFeature(feature string) (int, float32) {
	// Index hash
	hasher := fnv.New32a()
	hasher.Write([]byte(feature))
	idx := int(hasher.Sum32()) % h.dim
	if idx < 0 {
		idx = -idx
	}

	// Sign hash (use different seed by prepending marker)
	signHasher := fnv.New32()
	signHasher.Write([]byte("s:" + feature))
	sign := float32(1.0)
	if signHasher.Sum32()%2 == 1 {
		sign = -1.0
	}

	return idx, sign
}

func (h *HashingEmbedder) normalize(vec []float32) {
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm > 0 {
		invNorm := float32(1.0 / math.Sqrt(norm))
		for i := range vec {
			vec[i] *= invNorm
		}
	}
}

func tokenizeForEmbedding(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}
