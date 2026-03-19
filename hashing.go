package semantic

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// HashingEmbedder implements Embedder using a feature-hashing (hashing trick)
// approach. It produces fixed-dimension vectors by hashing word unigrams and
// character n-grams into a compact vector space. No vocabulary construction
// is required, making each Embed call fully independent.
//
// Properties:
//   - Fixed vector dimensionality regardless of vocabulary size
//   - Captures sub-word similarity (e.g. "btn" ↔ "button")
//   - L2-normalized output for cosine similarity compatibility
//   - Zero external dependencies — pure Go
type HashingEmbedder struct {
	dim         int     // vector dimensionality
	ngramMin    int     // minimum character n-gram length
	ngramMax    int     // maximum character n-gram length
	wordWeight  float32 // weight factor for word-level features
	ngramWeight float32 // weight factor for n-gram features
}

// NewHashingEmbedder creates a HashingEmbedder with the given dimension.
// Higher dimensions reduce hash collisions but use more memory.
// Recommended: 128 for speed, 256 for accuracy.
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

// Strategy returns "hashing".
func (h *HashingEmbedder) Strategy() string { return "hashing" }

// Embed converts a batch of texts into hashed feature vectors.
func (h *HashingEmbedder) Embed(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, text := range texts {
		result[i] = h.vectorize(text)
	}
	return result, nil
}

// vectorize converts a single text into a hashed feature vector combining
// word-level, character n-gram, role-aware, and synonym features.
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

// hashFeature hashes a feature string into an index [0, dim) and a sign
// (+1 or -1). The sign hash preserves inner-product properties (the
// "signed hashing trick" per Weinberger et al. 2009).
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

// normalize L2-normalizes a vector in-place.
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

// tokenizeForEmbedding splits text into lowercase tokens for embedding.
func tokenizeForEmbedding(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}
